// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.etcd.io/etcd/api/v3/mvccpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"

	"gopkg.in/srvkit.v1/logger"
)

var (
	ErrEmptyLease    = errors.New("empty lease")
	ErrNodeKeyExist  = errors.New("node key exist")
	ErrNoKeyDeleted  = errors.New("no key deleted")
	ErrEmptyEndpoint = errors.New("empty endpoint")
	// ErrClientClosed 表示客户端已关闭，需创建新客户端后再使用。
	ErrClientClosed = errors.New("etcd client closed")
	// ErrClientNotInitialized 表示客户端尚未成功初始化。
	ErrClientNotInitialized = errors.New("etcd client not initialized")
)

const (
	VerboseLv1 = 1
	VerboseLv2 = 2
)

var (
	EventChanCapacity = 8192
	OpTimeout         = 5 // 单次普通 RPC 的超时秒数
)

// EtcdClient 基于 etcd 的服务发现。公开方法可并发调用，首次使用后不可复制。
// mu 保护客户端生命周期；后台任务在 Close 时取消并等待退出。
type EtcdClient struct {
	mu        sync.Mutex
	closed    bool
	closeDone chan struct{}
	workers   sync.WaitGroup
	verbose   int32            //
	endpoints []string         // etcd server address
	root      string           // name space of key
	username  string           //
	passwd    string           //
	client    *clientv3.Client // etcd client
}

func NewEtcdClient(endpoints, root, username, passwd string) *EtcdClient {
	var addresses []string
	for _, endpoint := range strings.Split(endpoints, ",") {
		if endpoint = strings.TrimSpace(endpoint); endpoint != "" {
			addresses = append(addresses, endpoint)
		}
	}
	d := &EtcdClient{
		endpoints: addresses,
		root:      root,
		username:  username,
		passwd:    passwd,
		verbose:   VerboseLv1,
	}
	return d
}

// Init 初始化客户端。重复调用不会替换已有连接；Close 后不可重新初始化。
func (c *EtcdClient) Init(parentCtx context.Context) error {
	return c.initClient(parentCtx, clientv3.New)
}

func (c *EtcdClient) initClient(parentCtx context.Context, newClient func(clientv3.Config) (*clientv3.Client, error)) error {
	c.mu.Lock()
	closed, initialized := c.closed, c.client != nil
	c.mu.Unlock()
	if closed {
		return ErrClientClosed
	}
	if initialized {
		return nil
	}
	if err := parentCtx.Err(); err != nil {
		return err
	}
	if len(c.endpoints) == 0 {
		return ErrEmptyEndpoint
	}
	cli, err := newClient(clientv3.Config{
		Endpoints:   c.endpoints,
		DialTimeout: 5 * time.Second,
		Username:    c.username,
		Password:    c.passwd,
	})
	if err != nil {
		return err
	}

	ctx, cancel := operationContext(parentCtx)
	statusErr := checkEndpoints(ctx, cli, c.endpoints)
	cancel()
	if err := parentCtx.Err(); err != nil {
		statusErr = err
	}
	if statusErr != nil {
		closeEtcdClient(cli)
		return fmt.Errorf("cannot get status of etcd endpoints: %w", statusErr)
	}
	c.mu.Lock()
	if c.closed || c.client != nil {
		closed = c.closed
		c.mu.Unlock()
		closeEtcdClient(cli)
		if closed {
			return ErrClientClosed
		}
		return nil
	}
	c.client = cli
	c.mu.Unlock()
	return nil
}

// checkEndpoints 最多并发探测四个成员，避免不可达成员耗尽整个初始化时限。
// 首次成功或取消后等待所有探测结束，不留下初始化 goroutine。
func checkEndpoints(ctx context.Context, cli *clientv3.Client, endpoints []string) error {
	ctx, cancel := context.WithCancel(ctx)
	jobs := make(chan string, len(endpoints))
	for _, endpoint := range endpoints {
		jobs <- endpoint
	}
	close(jobs)
	results := make(chan error, min(len(endpoints), 4))
	var workers sync.WaitGroup
	defer func() { cancel(); workers.Wait() }()
	for i := 0; i < cap(results); i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for endpoint := range jobs {
				if ctx.Err() != nil {
					return
				}
				_, err := cli.Status(ctx, endpoint)
				if err != nil {
					err = fmt.Errorf("status %s: %w", endpoint, err)
				}
				select {
				case results <- err:
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	var failures []error
	for range endpoints {
		select {
		case err := <-results:
			if err == nil {
				return nil
			}
			failures = append(failures, err)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return errors.Join(failures...)
}

// Close 关闭连接并等待后台任务退出。可重复、并发调用；不会主动撤销节点 lease。
func (c *EtcdClient) Close() {
	c.mu.Lock()
	if c.closed {
		done := c.closeDone
		c.mu.Unlock()
		<-done
		return
	}
	c.closed = true
	c.closeDone = make(chan struct{})
	cli := c.client
	c.client = nil
	c.mu.Unlock()
	if cli != nil {
		closeEtcdClient(cli)
	}
	c.workers.Wait()
	close(c.closeDone)
}

func (c *EtcdClient) SetVerbose(v int32) {
	atomic.StoreInt32(&c.verbose, v)
}

func closeEtcdClient(cli *clientv3.Client) {
	if err := cli.Close(); err != nil && !errors.Is(err, context.Canceled) {
		logger.Warnf("close etcd client: %v", err)
	}
}

func operationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	timeout := time.Duration(OpTimeout) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return context.WithTimeout(ctx, timeout)
}

func (c *EtcdClient) getClient() (*clientv3.Client, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, ErrClientClosed
	}
	if c.client == nil {
		return nil, ErrClientNotInitialized
	}
	return c.client, nil
}

// launch 在关闭开始前登记任务，避免 Wait 与 Add 并发。
func (c *EtcdClient) launch(run func(*clientv3.Client)) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return ErrClientClosed
	}
	cli := c.client
	if cli == nil {
		c.mu.Unlock()
		return ErrClientNotInitialized
	}
	c.workers.Add(1)
	c.mu.Unlock()
	go func() {
		defer c.workers.Done()
		run(cli)
	}()
	return nil
}

func (c *EtcdClient) FormatKey(name string) string {
	return path.Join(c.root, name)
}

func (c *EtcdClient) checkedKey(name string) (string, error) {
	for _, part := range strings.Split(name, "/") {
		if part == ".." {
			return "", fmt.Errorf("invalid etcd key name %q: parent traversal", name)
		}
	}
	return c.FormatKey(name), nil
}

func (c *EtcdClient) clientFor(name string, directory bool) (*clientv3.Client, string, error) {
	key, err := c.checkedKey(name)
	if err != nil {
		return nil, "", err
	}
	if directory {
		if key == "." {
			key = ""
		} else {
			key = strings.TrimRight(key, "/") + "/"
		}
	}
	cli, err := c.getClient()
	return cli, key, err
}

// IsNodeExist 节点是否存在
func (c *EtcdClient) IsNodeExist(ctx context.Context, name string) (bool, error) {
	cli, key, err := c.clientFor(name, false)
	if err != nil {
		return false, err
	}
	ctx, cancel := operationContext(ctx)
	defer cancel()
	resp, err := cli.Get(ctx, key, clientv3.WithCountOnly())
	if err != nil {
		return false, err
	}
	return resp.Count > 0, nil
}

// GetKey 获取key的值
func (c *EtcdClient) GetKey(ctx context.Context, name string) ([]byte, error) {
	cli, key, err := c.clientFor(name, false)
	if err != nil {
		return nil, err
	}
	ctx, cancel := operationContext(ctx)
	defer cancel()
	resp, err := cli.Get(ctx, key)
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
	cli, key, err := c.clientFor(name, false)
	if err != nil {
		return err
	}
	ctx, cancel := operationContext(ctx)
	defer cancel()
	var resp *clientv3.PutResponse
	if leaseId <= 0 {
		resp, err = cli.Put(ctx, key, value)
	} else {
		resp, err = cli.Put(ctx, key, value, clientv3.WithLease(clientv3.LeaseID(leaseId)))
	}
	if err != nil {
		return err
	}
	if atomic.LoadInt32(&c.verbose) >= VerboseLv1 {
		logger.Infof("put key [%s] at rev %d", key, resp.Header.GetRevision())
	}
	return nil
}

// DelKey 删除一个key
func (c *EtcdClient) DelKey(ctx context.Context, name string) error {
	cli, key, err := c.clientFor(name, false)
	if err != nil {
		return err
	}
	ctx, cancel := operationContext(ctx)
	defer cancel()
	resp, err := cli.Delete(ctx, key)
	if err != nil {
		return err
	}
	if resp.Deleted == 0 {
		return ErrNoKeyDeleted
	}
	return nil
}

// DelDir 删除目录下的 key，不包含目录同名 key 或相邻前缀目录。
func (c *EtcdClient) DelDir(ctx context.Context, dir string) (int64, error) {
	cli, key, err := c.clientFor(dir, true)
	if err != nil {
		return 0, err
	}
	ctx, cancel := operationContext(ctx)
	defer cancel()
	resp, err := cli.Delete(ctx, key, clientv3.WithPrefix())
	if err != nil {
		return 0, err
	}
	return resp.Deleted, nil
}

// GetNode 获取节点信息
func (c *EtcdClient) GetNode(ctx context.Context, name string) (NodeInfo, error) {
	cli, key, err := c.clientFor(name, false)
	if err != nil {
		return nil, err
	}
	ctx, cancel := operationContext(ctx)
	defer cancel()
	resp, err := cli.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if resp.Count == 0 {
		return nil, nil
	}
	return decodeEtcdNode(resp.Kvs[0].Value)
}

// PutNode 设置节点信息
func (c *EtcdClient) PutNode(ctx context.Context, name string, value any, leaseId int64) error {
	cli, key, err := c.clientFor(name, false)
	if err != nil {
		return err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	var strVal = string(data)
	ctx, cancel := operationContext(ctx)
	defer cancel()
	var resp *clientv3.PutResponse
	if leaseId <= 0 {
		resp, err = cli.Put(ctx, key, strVal)
	} else {
		resp, err = cli.Put(ctx, key, strVal, clientv3.WithLease(clientv3.LeaseID(leaseId)))
	}
	if err != nil {
		return err
	}
	if atomic.LoadInt32(&c.verbose) >= VerboseLv1 {
		logger.Debugf("put node [%s] at rev %d", key, resp.Header.GetRevision())
	}
	return nil
}

// ListNodes 列出目录下的所有节点
func (c *EtcdClient) ListNodes(ctx context.Context, dir string) ([]NodeInfo, int64, error) {
	cli, key, err := c.clientFor(dir, true)
	if err != nil {
		return nil, 0, err
	}
	ctx, cancel := operationContext(ctx)
	defer cancel()
	resp, err := cli.Get(ctx, key, clientv3.WithPrefix())
	if err != nil {
		return nil, 0, err
	}
	var rev int64
	if resp.Header != nil {
		rev = resp.Header.Revision
	}
	if len(resp.Kvs) == 0 {
		return nil, rev, nil
	}
	var nodes = make([]NodeInfo, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		node, err := decodeEtcdNode(kv.Value)
		if err != nil {
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
	if ttl <= 0 {
		return 0, fmt.Errorf("invalid lease TTL %d", ttl)
	}
	cli, err := c.getClient()
	if err != nil {
		return 0, err
	}
	ctx, cancel := operationContext(ctx)
	defer cancel()
	lease, err := cli.Grant(ctx, ttl)
	if err != nil {
		return 0, err
	}
	if lease == nil || lease.ID == 0 {
		return 0, ErrEmptyLease
	}
	return int64(lease.ID), nil
}

// GetLeaseTTL 获取lease的剩余时间
func (c *EtcdClient) GetLeaseTTL(ctx context.Context, leaseId int64) (int, error) {
	cli, err := c.getClient()
	if err != nil {
		return 0, err
	}
	ctx, cancel := operationContext(ctx)
	defer cancel()
	resp, err := cli.TimeToLive(ctx, clientv3.LeaseID(leaseId))
	if err != nil {
		return 0, err
	}
	return int(resp.TTL), nil
}

// RevokeLease 撤销一个lease
func (c *EtcdClient) RevokeLease(ctx context.Context, leaseId int64) error {
	cli, err := c.getClient()
	if err != nil {
		return err
	}
	ctx, cancel := operationContext(ctx)
	defer cancel()
	if atomic.LoadInt32(&c.verbose) >= VerboseLv1 {
		logger.Debugf("revoke lease %d", leaseId)
	}
	_, err = cli.Revoke(ctx, clientv3.LeaseID(leaseId))
	return err
}

// NodeAliveKeeper 注册并保活一个节点。公开方法可并发调用，不可复制。
// mu 保护 lease 和保活状态；RPC 在锁外执行，registerDone 串行化注册与注销。
// 首次注册时序列化并固定 value，此后调用方修改原始 value 不影响自动重注册。
type NodeAliveKeeper struct {
	mu             sync.Mutex
	kaChan         <-chan *clientv3.LeaseKeepAliveResponse
	leaseId        int64
	alive          atomic.Bool
	ttl            int64
	name           string
	value          any
	payload        []byte
	stopped        bool
	registerDone   chan struct{}
	registerCancel context.CancelFunc
	keepCancel     context.CancelFunc
	done           chan struct{}
	stop           chan struct{}
}

var errKeeperStopped = errors.New("node alive keeper stopped")

func NewNodeAliveKeeper(name string, value any, ttl int64) *NodeAliveKeeper {
	return &NodeAliveKeeper{name: name, value: value, ttl: ttl, stop: make(chan struct{})}
}

func (k *NodeAliveKeeper) worker(ctx context.Context, c *EtcdClient, cli *clientv3.Client) {
	ticker := time.NewTicker(2333 * time.Millisecond)
	defer ticker.Stop()
	k.run(ctx, c, cli, ticker.C)
}

// run 单独接收时钟通道，使重注册和退出测试无需依赖真实时间。
func (k *NodeAliveKeeper) run(ctx context.Context, c *EtcdClient, cli *clientv3.Client, ticks <-chan time.Time) {
	defer func() {
		k.mu.Lock()
		k.stopLocked()
		cancel, registerCancel := k.keepCancel, k.registerCancel
		k.keepCancel = nil
		k.kaChan = nil
		k.alive.Store(false)
		k.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		if registerCancel != nil {
			registerCancel()
		}
		close(k.done)
	}()
	for {
		k.mu.Lock()
		ch, stopped := k.kaChan, k.stopped
		k.mu.Unlock()
		if stopped {
			return
		}
		select {
		case <-ticks:
			if !k.alive.Load() {
				if err := k.RegisterNode(ctx, c); err != nil {
					if errors.Is(err, ErrClientClosed) || errors.Is(err, errKeeperStopped) {
						return
					}
					logger.Warnf("register node %s failed: %v", k.name, err)
				}
			}
		case response, ok := <-ch:
			if ok && response != nil && response.TTL > 0 {
				continue
			}
			k.mu.Lock()
			var cancel context.CancelFunc
			if k.kaChan == ch {
				// 保留 lease ID，以便重注册或 Revoke 时回收尚未过期的 lease。
				k.kaChan = nil
				k.alive.Store(false)
				cancel, k.keepCancel = k.keepCancel, nil
			}
			k.mu.Unlock()
			if cancel != nil {
				cancel()
			}
		case <-ctx.Done():
			return
		case <-cli.Ctx().Done():
			return
		case <-k.stop:
			return
		}
	}
}

// stopLocked 使退出不可逆，防止并发注册在 worker 退出后发布新的保活状态。
func (k *NodeAliveKeeper) stopLocked() {
	if !k.stopped {
		k.stopped = true
		if k.stop != nil {
			close(k.stop)
		}
	}
}

// RegisterNode 注册并开始保活，成功注册的 keeper 重复调用不会分配新 lease。
// 首次调用期间不可并发修改 value。Revoke 后需创建新的 keeper 才能重新注册。
func (k *NodeAliveKeeper) RegisterNode(ctx context.Context, c *EtcdClient) error {
	cli, err := c.getClient()
	if err != nil {
		return err
	}
	runCtx, cancel := context.WithCancel(ctx)
	stopClient := context.AfterFunc(cli.Ctx(), cancel)
	cancelRun := func() { stopClient(); cancel() }

	k.mu.Lock()
	if k.stopped {
		k.mu.Unlock()
		cancelRun()
		return errKeeperStopped
	}
	if k.alive.Load() {
		k.mu.Unlock()
		cancelRun()
		return nil
	}
	if k.registerDone != nil {
		k.mu.Unlock()
		cancelRun()
		return errors.New("node registration already in progress")
	}
	done := make(chan struct{})
	k.registerDone, k.registerCancel = done, cancelRun
	oldLease, payload, value := k.leaseId, k.payload, k.value
	k.mu.Unlock()

	keepRunning := false
	defer func() {
		if !keepRunning {
			cancelRun()
		}
		k.mu.Lock()
		k.registerDone, k.registerCancel = nil, nil
		close(done)
		k.mu.Unlock()
	}()
	if payload == nil {
		payload, err = json.Marshal(value)
		if err != nil {
			return err
		}
		k.mu.Lock()
		k.payload, k.value = payload, nil
		k.mu.Unlock()
	}
	if oldLease != 0 {
		revokeCtx, revokeCancel := operationContext(runCtx)
		_, err = cli.Revoke(revokeCtx, clientv3.LeaseID(oldLease))
		revokeCancel()
		if err != nil && !errors.Is(err, rpctypes.ErrLeaseNotFound) {
			return err
		}
		k.mu.Lock()
		k.leaseId = 0
		k.mu.Unlock()
	}
	leaseID, err := c.registerPayload(runCtx, cli, k.name, payload, k.ttl)
	if err != nil {
		return err
	}
	ch, err := cli.KeepAlive(runCtx, clientv3.LeaseID(leaseID))
	if err == nil && ch == nil {
		err = errors.New("empty keepalive channel")
	}
	k.mu.Lock()
	stopped := k.stopped
	k.mu.Unlock()
	if err == nil {
		if stopped {
			err = errKeeperStopped
		} else {
			err = runCtx.Err()
		}
	}
	if err != nil {
		return cleanupLease(ctx, cli, leaseID, err)
	}
	k.mu.Lock()
	// Revoke 可能在前一次检查之后开始；不得在注销开始后发布新 lease。
	if k.stopped {
		k.mu.Unlock()
		return cleanupLease(ctx, cli, leaseID, errKeeperStopped)
	}
	k.leaseId, k.kaChan, k.keepCancel = leaseID, ch, cancelRun
	k.alive.Store(true)
	keepRunning = true
	k.mu.Unlock()
	return nil
}

// Revoke 永久停止自动注册和保活，再撤销当前 lease。可重复、并发调用。
// ctx 限制等待及撤销时间；失败后可再次调用以重试撤销。
// 仅取消注册时的 context 或关闭客户端时，lease 会自然过期。
func (k *NodeAliveKeeper) Revoke(ctx context.Context, c *EtcdClient) error {
	k.mu.Lock()
	k.stopLocked()
	registerCancel, keepCancel := k.registerCancel, k.keepCancel
	registerDone, workerDone := k.registerDone, k.done
	k.alive.Store(false)
	k.mu.Unlock()
	if registerCancel != nil {
		registerCancel()
	}
	if keepCancel != nil {
		keepCancel()
	}
	for _, done := range []chan struct{}{registerDone, workerDone} {
		if done != nil {
			select {
			case <-done:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	k.mu.Lock()
	leaseID := k.leaseId
	k.mu.Unlock()
	if leaseID == 0 {
		return nil
	}
	err := c.RevokeLease(ctx, leaseID)
	if err != nil && !errors.Is(err, rpctypes.ErrLeaseNotFound) {
		return err
	}
	k.mu.Lock()
	k.leaseId, k.kaChan, k.keepCancel = 0, nil, nil
	k.mu.Unlock()
	return nil
}

// RegisterNode 原子地注册不存在的节点，并返回 ttl 秒的 lease。
// 注册失败返回零 lease ID；已申请的 lease 使用独立的有界 context 回收。
func (c *EtcdClient) RegisterNode(ctx context.Context, name string, value any, ttl int64) (int64, error) {
	cli, err := c.getClient()
	if err != nil {
		return 0, err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return 0, err
	}
	return c.registerPayload(ctx, cli, name, data, ttl)
}

func (c *EtcdClient) registerPayload(ctx context.Context, cli *clientv3.Client, name string, data []byte, ttl int64) (int64, error) {
	key, err := c.checkedKey(name)
	if err != nil {
		return 0, err
	}
	if ttl <= 0 {
		return 0, fmt.Errorf("invalid lease TTL %d", ttl)
	}
	opCtx, cancel := operationContext(ctx)
	defer cancel()
	lease, err := cli.Grant(opCtx, ttl)
	if err != nil {
		return 0, err
	}
	if lease == nil || lease.ID == 0 {
		return 0, ErrEmptyLease
	}
	response, err := cli.Txn(opCtx).
		If(clientv3.Compare(clientv3.CreateRevision(key), "=", 0)).
		Then(clientv3.OpPut(key, string(data), clientv3.WithLease(lease.ID))).
		Commit()
	if err == nil && !response.Succeeded {
		err = ErrNodeKeyExist
	}
	if err != nil {
		return 0, cleanupLease(ctx, cli, int64(lease.ID), err)
	}
	return int64(lease.ID), nil
}

func cleanupLease(ctx context.Context, cli *clientv3.Client, leaseID int64, cause error) error {
	cleanupCtx, cancel := operationContext(context.WithoutCancel(ctx))
	defer cancel()
	_, err := cli.Revoke(cleanupCtx, clientv3.LeaseID(leaseID))
	if err != nil && !errors.Is(err, rpctypes.ErrLeaseNotFound) {
		return errors.Join(cause, fmt.Errorf("revoke lease %d: %w", leaseID, err))
	}
	return cause
}

// RegisterAndKeepAlive 注册一个节点，并在 context 取消、客户端关闭或 Revoke 时停止 worker。
func (c *EtcdClient) RegisterAndKeepAlive(ctx context.Context, name string, value any, ttl int64) (*NodeAliveKeeper, error) {
	keeper := NewNodeAliveKeeper(name, value, ttl)
	if err := keeper.RegisterNode(ctx, c); err != nil {
		return nil, err
	}
	keeper.done = make(chan struct{})
	if err := c.launch(func(cli *clientv3.Client) { keeper.worker(ctx, c, cli) }); err != nil {
		keeper.mu.Lock()
		cancel := keeper.keepCancel
		keeper.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		close(keeper.done)
		// Close 已经开始时底层连接可能已关闭，lease 将自然过期。
		return nil, err
	}
	return keeper, nil
}

// decodeEtcdNode 保留 json.Number，并拒绝尾随数据及会导致 GetID panic 的字段类型。
// 为兼容普通 NodeInfo 存取，Type 和 ID 可以省略；watch 事件会进一步校验节点身份。
func decodeEtcdNode(data []byte) (NodeInfo, error) {
	var node NodeInfo
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&node); err != nil {
		return nil, err
	}
	if node == nil {
		return nil, errors.New("node must be a JSON object")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			err = errors.New("multiple JSON values")
		}
		return nil, err
	}
	if value, ok := node[NodeKeyType]; ok {
		if _, ok := value.(string); !ok {
			return nil, errors.New("node Type must be a string")
		}
	}
	if value, ok := node[NodeKeyID]; ok {
		var text string
		switch value := value.(type) {
		case json.Number:
			text = value.String()
		case string:
			text = value
		default:
			return nil, errors.New("node ID must be an unsigned 32-bit integer")
		}
		if _, err := strconv.ParseUint(text, 10, 32); err != nil {
			return nil, fmt.Errorf("invalid node ID %q: %w", text, err)
		}
	}
	return node, nil
}

// propagateWatchEvent 保序投递所有事件；取消时释放背压阻塞，不丢弃 UPDATE。
func propagateWatchEvent(ctx context.Context, eventChan chan<- *NodeEvent, ev *clientv3.Event) error {
	if ev == nil || ev.Kv == nil {
		return errors.New("empty watch event")
	}
	nodeType, nodeID := parseNodeTypeAndID(string(ev.Kv.Key))
	if nodeType == "" || nodeID == 0 {
		// 同一命名空间中可以存在普通 key，这些 key 不属于节点订阅。
		return nil
	}
	event := &NodeEvent{NodeType: nodeType, NodeID: nodeID}
	switch ev.Type {
	case mvccpb.PUT:
		event.EvType = EventUpdate
		if ev.IsCreate() {
			event.EvType = EventCreate
		}
		info, err := decodeEtcdNode(ev.Kv.Value)
		if err != nil {
			return fmt.Errorf("unmarshal node %s: %w", ev.Kv.Key, err)
		}
		if info.GetTypeName() != nodeType || info.GetID() != nodeID {
			return fmt.Errorf("node identity does not match key %s", ev.Kv.Key)
		}
		event.Info = info
	case mvccpb.DELETE:
		event.EvType = EventDelete
	default:
		return fmt.Errorf("unknown watch event type %d", ev.Type)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case eventChan <- event:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Watch 订阅原始 key 前缀，rev 为包含在内的起始 revision。
// eventChan 归调用方所有，本方法不关闭它；使用期间调用方不得关闭 channel。
// 调用方必须取消 ctx 释放订阅。需要终止通知时使用 WatchDir。
func (c *EtcdClient) Watch(ctx context.Context, name string, rev int64, eventChan chan *NodeEvent) {
	c.watch(ctx, name, rev, eventChan, false, false)
}

func (c *EtcdClient) watch(ctx context.Context, name string, rev int64, eventChan chan *NodeEvent, directory, owned bool) <-chan struct{} {
	done := make(chan struct{})
	finish := func() {
		if owned {
			close(eventChan)
		}
		close(done)
	}
	cli, key, err := c.clientFor(name, directory)
	if err == nil && eventChan == nil {
		err = errors.New("nil watch event channel")
	}
	if err == nil && rev < 0 {
		err = errors.New("negative watch revision")
	}
	if err != nil {
		logger.Warnf("start etcd watch: %v", err)
		finish()
		return done
	}
	opts := []clientv3.OpOption{clientv3.WithPrefix()}
	if rev > 0 {
		opts = append(opts, clientv3.WithRev(rev))
	}
	err = c.launch(func(_ *clientv3.Client) {
		defer finish()
		watchCtx, cancel := context.WithCancel(ctx)
		stopClient := context.AfterFunc(cli.Ctx(), cancel)
		defer cancel()
		defer stopClient()
		watchCh := cli.Watch(clientv3.WithRequireLeader(watchCtx), key, opts...)
		for {
			select {
			case response, ok := <-watchCh:
				if !ok {
					return
				}
				if err := response.Err(); err != nil {
					logger.Warnf("watch key %s terminated; relist before restarting: %v", key, err)
					return
				}
				for _, event := range response.Events {
					if err := propagateWatchEvent(watchCtx, eventChan, event); err != nil {
						if watchCtx.Err() == nil {
							logger.Warnf("watch key %s: %v", key, err)
						}
						return
					}
				}
			case <-watchCtx.Done():
				return
			}
		}
	})
	if err != nil {
		logger.Warnf("start etcd watch: %v", err)
		finish()
	}
	return done
}

// WatchDir 订阅目录下的节点变化，不包含目录同名 key 或相邻前缀目录。
// 返回的 channel 由生产端关闭：ctx 取消、Close、watch 错误或非法节点数据均会结束订阅。
// 终止后调用方应重新 List 获取快照和 revision，再从 revision+1 订阅；
// 历史被压缩时不可直接跳到最新 revision，否则可能漏掉节点变化。
func (c *EtcdClient) WatchDir(ctx context.Context, directory string, rev int64) <-chan *NodeEvent {
	capacity := max(EventChanCapacity, 0)
	eventChan := make(chan *NodeEvent, capacity)
	c.watch(ctx, directory, rev, eventChan, true, true)
	return eventChan
}

// /SLG/service/GATE/1 => GATE, 1。拒绝负数、溢出、零 ID 和空类型。
func parseNodeTypeAndID(key string) (string, uint32) {
	idx := strings.LastIndex(key, "/")
	if idx < 0 {
		return "", 0
	}
	id, err := strconv.ParseUint(key[idx+1:], 10, 32)
	if err != nil || id == 0 {
		return "", 0
	}
	parent := key[:idx]
	nodeType := parent[strings.LastIndex(parent, "/")+1:]
	if nodeType == "" || nodeType == "." || nodeType == ".." {
		return "", 0
	}
	return nodeType, uint32(id)
}
