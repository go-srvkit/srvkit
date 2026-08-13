// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package cluster

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unsafe"

	clientv3 "go.etcd.io/etcd/client/v3"

	"gopkg.in/srvkit.v1/conv"
	"gopkg.in/srvkit.v1/logger"
)

var (
	ErrEmptyLease    = errors.New("empty lease")
	ErrNodeKeyExist  = errors.New("node key exist")
	ErrNoKeyDeleted  = errors.New("no key deleted")
	ErrEmptyEndpoint = errors.New("empty endpoint")
)

const (
	VerboseLv1 = 1
	VerboseLv2 = 2
)

var (
	EventChanCapacity = 1024
	OpTimeout         = 5
)

// EtcdClient 基于etcd的服务发现
type EtcdClient struct {
	closing   atomic.Bool      //
	verbose   int32            //
	endpoints []string         // etcd server address
	root      string           // name space of key
	username  string           //
	passwd    string           //
	client    *clientv3.Client // etcd client
}

func NewEtcdClient(endpoints, root, username, passwd string) *EtcdClient {
	d := &EtcdClient{
		endpoints: strings.Split(endpoints, ","),
		root:      root,
		username:  username,
		passwd:    passwd,
		verbose:   VerboseLv1,
	}
	return d
}

// Init 初始化etcd客户端
func (c *EtcdClient) Init(parentCtx context.Context) error {
	if len(c.endpoints) == 0 {
		return ErrEmptyEndpoint
	}
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   c.endpoints,
		DialTimeout: 5 * time.Second,
		Username:    c.username,
		Password:    c.passwd,
	})
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(parentCtx, time.Second*5)
	defer cancel()

	if _, err := cli.Status(ctx, c.endpoints[0]); err != nil {
		return fmt.Errorf("cannot get status of etcd %s: %w", c.endpoints[0], err)
	}
	c.client = cli
	return nil
}

// Close 关闭etcd链接
func (c *EtcdClient) Close() {
	if !c.closing.CompareAndSwap(false, true) {
		return
	}
	if c.verbose >= VerboseLv1 {
		logger.Debugf("etcd client closing")
	}
	if c.client != nil {
		c.client.Close()
		c.client = nil
	}
}

func (c *EtcdClient) SetVerbose(v int32) {
	c.verbose = v
}

func (c *EtcdClient) FormatKey(name string) string {
	return path.Join(c.root, name)
}

// IsNodeExist 节点是否存在
func (c *EtcdClient) IsNodeExist(ctx context.Context, name string) (bool, error) {
	var key = c.FormatKey(name)
	resp, err := c.client.Get(ctx, key, clientv3.WithCountOnly())
	if err != nil {
		return false, err
	}
	return resp.Count > 0, nil
}

// GetKey 获取key的值
func (c *EtcdClient) GetKey(ctx context.Context, name string) ([]byte, error) {
	var key = c.FormatKey(name)
	resp, err := c.client.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if resp.Count == 0 {
		return nil, nil
	}
	return resp.Kvs[0].Value, nil
}

// PutKey 设置一个key
func (c *EtcdClient) PutKey(ctx context.Context, name, value string, leaseId int64) error {
	var key = c.FormatKey(name)
	var resp *clientv3.PutResponse
	var err error
	if leaseId <= 0 {
		resp, err = c.client.Put(ctx, key, value)
	} else {
		resp, err = c.client.Put(ctx, key, value, clientv3.WithLease(clientv3.LeaseID(leaseId)))
	}
	if err != nil {
		return err
	}
	if c.verbose >= VerboseLv1 {
		logger.Infof("put key [%s] at rev %d", key, resp.Header.Revision)
	}
	return nil
}

// DelKey 删除一个key
func (c *EtcdClient) DelKey(ctx context.Context, name string) error {
	var key = c.FormatKey(name)
	resp, err := c.client.Delete(ctx, key)
	if err != nil {
		return err
	}
	if resp.Deleted == 0 {
		return ErrNoKeyDeleted
	}
	return nil
}

// DelDir 删除目录
func (c *EtcdClient) DelDir(ctx context.Context, dir string) (int64, error) {
	var key = c.FormatKey(dir)
	resp, err := c.client.Delete(ctx, key, clientv3.WithPrefix())
	if err != nil {
		return 0, err
	}
	return resp.Deleted, nil
}

// GetNode 获取节点信息
func (c *EtcdClient) GetNode(ctx context.Context, name string) (NodeInfo, error) {
	var key = c.FormatKey(name)
	resp, err := c.client.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if resp.Count == 0 {
		return nil, nil
	}
	var node = make(NodeInfo)
	if err = unmarshalNode(resp.Kvs[0].Value, &node); err != nil {
		return nil, err
	}
	return node, nil
}

// PutNode 设置节点信息
func (c *EtcdClient) PutNode(ctx context.Context, name string, value any, leaseId int64) error {
	var key = c.FormatKey(name)
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	var strVal = unsafe.String(unsafe.SliceData(data), len(data))
	var resp *clientv3.PutResponse
	if leaseId <= 0 {
		resp, err = c.client.Put(ctx, key, strVal)
	} else {
		resp, err = c.client.Put(ctx, key, strVal, clientv3.WithLease(clientv3.LeaseID(leaseId)))
	}
	if err != nil {
		return err
	}
	if c.verbose >= VerboseLv1 {
		logger.Debugf("put node [%s] at rev %d", key, resp.Header.Revision)
	}
	return nil
}

// ListNodes 列出目录下的所有节点
func (c *EtcdClient) ListNodes(ctx context.Context, dir string) ([]NodeInfo, int64, error) {
	var key = c.FormatKey(dir)
	resp, err := c.client.Get(ctx, key, clientv3.WithPrefix())
	if err != nil {
		return nil, 0, err
	}
	if resp.Count == 0 {
		return nil, 0, nil
	}
	var rev int64
	if resp.Header != nil {
		rev = resp.Header.Revision
	}
	var nodes = make([]NodeInfo, 0, resp.Count)
	for _, kv := range resp.Kvs {
		var node NodeInfo
		if err = unmarshalNode(kv.Value, &node); err != nil {
			return nil, 0, fmt.Errorf("unmarshal node %s: %w", kv.Key, err)
		}
		nodes = append(nodes, node)
	}
	return nodes, rev, nil
}

// ListNodesWith 列出目录下的所有指定类型节点
func (c *EtcdClient) ListNodesWith(ctx context.Context, dir string, dependency []string) ([]NodeInfo, int64, error) {
	nodes, rev, err := c.ListNodes(ctx, dir)
	if err != nil {
		return nil, 0, err
	}
	var result []NodeInfo
	for _, info := range nodes {
		var nodeType = info.GetTypeName()
		if slices.Contains(dependency, nodeType) {
			result = append(result, info)
		}
	}
	return result, rev, nil
}

// GrantLease 申请一个lease
func (c *EtcdClient) GrantLease(ctx context.Context, ttl int64) (int64, error) {
	lease, err := c.client.Grant(ctx, ttl)
	if err != nil {
		return 0, err
	}
	if lease == nil {
		return 0, ErrEmptyLease
	}
	return int64(lease.ID), nil
}

// GetLeaseTTL 获取lease的剩余时间
func (c *EtcdClient) GetLeaseTTL(ctx context.Context, leaseId int64) (int, error) {
	resp, err := c.client.TimeToLive(ctx, clientv3.LeaseID(leaseId))
	if err != nil {
		return 0, err
	}
	return int(resp.TTL), nil
}

// RevokeLease 撤销一个lease
func (c *EtcdClient) RevokeLease(ctx context.Context, leaseId int64) error {
	if c.verbose >= VerboseLv1 {
		logger.Debugf("revoke lease %d", leaseId)
	}
	_, err := c.client.Revoke(ctx, clientv3.LeaseID(leaseId))
	return err
}

// NodeAliveKeeper 用于注册并保活节点
type NodeAliveKeeper struct {
	kaChan  <-chan *clientv3.LeaseKeepAliveResponse
	leaseId int64
	alive   atomic.Bool
	ttl     int64
	name    string
	value   any
}

func NewNodeAliveKeeper(name string, value any, ttl int64) *NodeAliveKeeper {
	return &NodeAliveKeeper{
		name:  name,
		value: value,
		ttl:   ttl,
	}
}

func (k *NodeAliveKeeper) worker(ctx context.Context, client *EtcdClient) {
	var duration = time.Millisecond * 2333
	var ticker = time.NewTicker(duration)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if !k.alive.Load() || k.leaseId == 0 {
				if err := k.RegisterNode(ctx, client); err != nil {
					logger.Infof("register node %s failed: %v", k.name, err)
				}
			}

		case ka, ok := <-k.kaChan:
			if ka != nil {
				//fmt.Printf("lease=%d keep alive ttl=%d at rev=%d\n", ka.ID, ka.TTL, ka.Revision)
				continue
			} else if !ok || ka == nil {
				var leaseId = k.leaseId
				k.alive.Store(false)
				k.leaseId = 0
				k.kaChan = nil
				if client.verbose >= VerboseLv1 {
					logger.Infof("node %s lease=%d is not alive, try register later", k.name, leaseId)
				}
			}

		case <-ctx.Done():
			if client.verbose >= VerboseLv1 {
				logger.Infof("node %s alive keeper with lease=%d stopped", k.name, k.leaseId)
			}
			return
		}
	}
}

func (k *NodeAliveKeeper) RegisterNode(ctx context.Context, c *EtcdClient) error {
	if c.verbose >= VerboseLv1 {
		logger.Infof("try register key: %s", c.FormatKey(k.name))
	}
	leaseId, err := c.RegisterNode(ctx, k.name, k.value, k.ttl)
	if err != nil {
		return err
	}
	kaChan, err := c.client.KeepAlive(ctx, clientv3.LeaseID(leaseId))
	if err != nil {
		return err
	}
	k.kaChan = kaChan
	k.alive.Store(true)
	k.leaseId = leaseId
	if c.verbose >= VerboseLv1 {
		logger.Infof("register key [%s] with lease %d done", c.FormatKey(k.name), k.leaseId)
	}
	return nil
}

func (k *NodeAliveKeeper) Revoke(ctx context.Context, c *EtcdClient) error {
	if c.verbose >= VerboseLv1 {
		logger.Infof("try revoke node %s lease %d", k.name, k.leaseId)
	}
	if k.leaseId == 0 || !k.alive.Load() {
		if c.verbose >= VerboseLv1 {
			logger.Infof("node %s lease=%d is not alive", k.name, k.leaseId)
		}
		return nil
	}
	if err := c.RevokeLease(ctx, k.leaseId); err != nil {
		logger.Warnf("revoke node %s lease %d failed: %v", k.name, k.leaseId, err)
		return err
	} else {
		if c.verbose >= VerboseLv1 {
			logger.Infof("revoke node %s lease=%d done", k.name, k.leaseId)
		}
		k.leaseId = 0
		k.alive.Store(false)
	}
	return nil
}

// RegisterNode 注册一个节点信息，并返回一个ttl秒的lease
// 要保证 etcd Put-If-Absent的原子性，需要使用事务API
func (c *EtcdClient) RegisterNode(ctx context.Context, name string, value any, ttl int64) (leaseId int64, err error) {
	leaseId, err = c.GrantLease(ctx, ttl)
	if err != nil {
		return 0, err
	}
	defer func() {
		if err != nil {
			if er := c.RevokeLease(ctx, leaseId); er != nil {
				logger.Errorf("revoke node %s lease %d failed: %v", name, leaseId, er)
			}
		}
	}()

	var data []byte
	data, err = json.Marshal(value)
	if err != nil {
		return
	}
	var strVal = unsafe.String(unsafe.SliceData(data), len(data))

	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	var key = c.FormatKey(name)
	txn := c.client.Txn(ctx).
		If(
			clientv3.Compare(clientv3.CreateRevision(key), "=", 0)).
		Then(
			clientv3.OpPut(key, strVal, clientv3.WithLease(clientv3.LeaseID(leaseId))))
	var resp *clientv3.TxnResponse
	resp, err = txn.Commit()
	if err != nil {
		return
	}
	if !resp.Succeeded {
		err = ErrNodeKeyExist
	}
	return
}

// RegisterAndKeepAlive 注册一个节点并保活
func (c *EtcdClient) RegisterAndKeepAlive(ctx context.Context, name string, value any, ttl int64) (*NodeAliveKeeper, error) {
	var keeper = NewNodeAliveKeeper(name, value, ttl)
	if err := keeper.RegisterNode(ctx, c); err != nil {
		return nil, err
	}
	go keeper.worker(ctx, c)
	return keeper, nil
}

func propagateWatchEvent(eventChan chan<- *NodeEvent, ev *clientv3.Event) {
	var key = conv.BytesAsStr(ev.Kv.Key)
	nodeType, nodeId := parseNodeTypeAndID(key)
	if nodeType == "" || nodeId == 0 {
		logger.Warnf("watch event: invalid event key=%s", ev.Kv.Key)
		return
	}
	var event = &NodeEvent{
		EvType:   EventUnknown,
		NodeType: nodeType,
		NodeID:   nodeId,
	}
	switch ev.Type {
	case 0: // PUT
		if ev.IsCreate() {
			event.EvType = EventCreate
		} else {
			event.EvType = EventUpdate
		}
	case 1: // DELETE
		event.EvType = EventDelete
	}
	if len(ev.Kv.Value) > 0 {
		if err := unmarshalNode(ev.Kv.Value, &event.Info); err != nil {
			logger.Errorf("unmarshal node %s: %v", key, err)
			return
		}
	}

	// CREATE/DELETE事件不能丢弃
	switch event.EvType {
	case EventCreate, EventDelete:
		eventChan <- event
		return
	}

	// UPDATE事件可丢弃
	select {
	case eventChan <- event:
	default:
		logger.Warnf("watch event: channel full, drop update event for key=%s", ev.Kv.Key)
	}
}

func (c *EtcdClient) Watch(ctx context.Context, name string, rev int64, eventChan chan *NodeEvent) {
	var key = c.FormatKey(name)
	var opts = []clientv3.OpOption{clientv3.WithPrefix()}
	if rev > 0 {
		opts = append(opts, clientv3.WithRev(rev))
	}
	var watchCh = c.client.Watch(clientv3.WithRequireLeader(ctx), key, opts...)

	go func() {
		for {
			select {
			case resp, ok := <-watchCh:
				if !ok {
					return
				}
				if resp.Err() != nil {
					logger.Warnf("watch key %s canceled: %v", key, resp.Err())
					return
				}
				for _, ev := range resp.Events {
					propagateWatchEvent(eventChan, ev)
				}

			case <-ctx.Done():
				return
			}
		}
	}()
}

// WatchDir 订阅目录下的节点变化
func (c *EtcdClient) WatchDir(ctx context.Context, directory string, rev int64) <-chan *NodeEvent {
	var eventChan = make(chan *NodeEvent, EventChanCapacity)
	c.Watch(ctx, directory, rev, eventChan)
	return eventChan
}

// WatchDirTo 订阅目录下的所有节点变化, 并把节点变化更新到nodeMap
func (c *EtcdClient) WatchDirTo(ctx context.Context, directory string, dependency []string, nodeMap *NodeMap) error {
	slices.Sort(dependency)
	nodes, rev, err := c.ListNodesWith(ctx, directory, dependency)
	if err != nil {
		return err
	}
	for _, node := range nodes {
		nodeMap.AddNode(node)
	}
	// 开启多个watch, 每个watch会增加goroutine和channel的开销
	var eventChan = make(chan *NodeEvent, EventChanCapacity)
	c.Watch(ctx, directory, rev+1, eventChan)

	go func() {
		for {
			select {
			case ev := <-eventChan:
				if ev != nil {
					if ev.NodeType != "" && ev.NodeID > 0 && slices.Contains(dependency, ev.NodeType) {
						switch ev.EvType {
						case EventCreate, EventUpdate:
							nodeMap.UpsertNode(ev.Info)
						case EventDelete:
							nodeMap.DeleteNode(ev.NodeType, ev.NodeID)
						}
					}
				}

			case <-ctx.Done():
				close(eventChan)
				return
			}
		}
	}()
	return nil
}

// `/SLG/service/GATE/1` => {"Type": "GATE", "ID": 1, "Interface": "127.0.0.1:9981"}
func parseNodeTypeAndID(key string) (string, uint32) {
	var idx = strings.LastIndex(key, "/")
	if idx > 0 {
		var strId = key[idx+1:]
		var subkey = key[:idx]
		idx = strings.LastIndex(subkey, "/")
		if idx > 0 {
			var nodeType = subkey[idx+1:]
			id, _ := strconv.Atoi(strId)
			return nodeType, uint32(id)
		}
	}
	return "", 0
}
