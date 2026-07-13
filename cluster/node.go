// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package cluster

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"strings"
	"sync"

	"gopkg.in/srvkit.v1/collections/slice"
	"gopkg.in/srvkit.v1/conv"
	"gopkg.in/srvkit.v1/gutil"
)

const (
	NodeTypeMongo = "MONGO"
	NodeTypeRedis = "REDIS"
)

const (
	NodeKeyPID       = "PID"
	NodeKeyHost      = "Host"
	NodeKeyID        = "ID"
	NodeKeyType      = "Type"
	NodeKeyInterface = "Interface"
	NodeKeyURI       = "URI"
	NodeKeyMongoID   = "MongoID"
	NodeKeyRedisID   = "RedisID"
	NodeKeyStatus    = "Status"
)

// NodeInfo 节点基本信息
type NodeInfo map[string]any

func NewNodeInfo() NodeInfo {
	return make(NodeInfo)
}

// NewNodeFrom 从节点类型和服务ID创建节点信息
func NewNodeFrom(nodeType string, serviceId uint32) NodeInfo {
	var info = make(NodeInfo)
	info.Init(nodeType, serviceId)
	return info
}

// Init 初始化节点信息
func (n *NodeInfo) Init(nodeType string, serviceId uint32) {
	n.SetString(NodeKeyType, nodeType)
	n.SetInt(NodeKeyID, int64(serviceId))
	n.SetInt(NodeKeyPID, int64(os.Getpid()))
	if hostname, err := os.Hostname(); err == nil {
		n.SetString(NodeKeyHost, hostname)
	}
}

func (n NodeInfo) Size() int {
	return len(n)
}

func (n NodeInfo) SetString(key, val string) {
	n[key] = val
}

func (n NodeInfo) GetString(key string) string {
	if val, ok := n[key]; ok {
		return conv.FormatAny(val)
	}
	return ""
}

func (n NodeInfo) GetStrings(key string) []string {
	var content = n.GetString(key)
	if content != "" {
		return strings.Split(content, ",")
	}
	return nil
}

func (n NodeInfo) GetInt(key string) int64 {
	if val, ok := n[key]; ok {
		return conv.ToInt[int64](val)
	}
	return 0
}

func (n NodeInfo) SetInt(key string, val int64) {
	n[key] = val
}

func (n NodeInfo) SetBool(key string, val bool) {
	n[key] = val
}

func (n NodeInfo) GetBool(key string) bool {
	var text = n.GetString(key)
	if text != "" {
		return conv.ParseBool(text)
	}
	return false
}

func (n NodeInfo) Clone() NodeInfo {
	return maps.Clone(n)
}

func (n NodeInfo) Equal(other NodeInfo) bool {
	return maps.Equal(n, other)
}

func (n NodeInfo) String() string {
	return gutil.JSONStringify(n)
}

func (n NodeInfo) GetID() uint32 {
	return uint32(n.GetInt(NodeKeyID))
}

func (n NodeInfo) GetTypeName() string {
	return n.GetString(NodeKeyType)
}

func (n NodeInfo) GetType() string {
	var name = n.GetString(NodeKeyType)
	return name
}

func (n NodeInfo) GetURI() string {
	return n.GetString(NodeKeyURI)
}

func (n NodeInfo) SetInterface(val string) {
	n.SetString(NodeKeyInterface, val)
}

func (n NodeInfo) GetInterface() string {
	return n.GetString(NodeKeyInterface)
}

// 使用bigint序列化大整数
func unmarshalNode(data []byte, node *NodeInfo) error {
	if len(data) > 0 {
		var dec = json.NewDecoder(bytes.NewReader(data))
		dec.UseNumber()
		return dec.Decode(node)
	}
	return nil
}

// NodeMap 按服务类型区分的节点信息
type NodeMap struct {
	mut   sync.RWMutex
	nodes map[string][]NodeInfo
}

func NewNodeMap() *NodeMap {
	return new(NodeMap).Init()
}

// Init 初始化
func (m *NodeMap) Init() *NodeMap {
	if m.nodes == nil {
		m.nodes = make(map[string][]NodeInfo)
	}
	return m
}

// TotalCount 所有节点数量
func (m *NodeMap) TotalCount() int {
	m.mut.RLock()
	var count = 0
	for _, nodes := range m.nodes {
		count += len(nodes)
	}
	m.mut.RUnlock()
	return count
}

func (m *NodeMap) GetCountOf(nodeType string) int {
	m.mut.RLock()
	v := m.nodes[nodeType]
	m.mut.RUnlock()
	return len(v)
}

func (m *NodeMap) GetTypeList() []string {
	m.mut.RLock()
	var typeList = make([]string, 0, len(m.nodes))
	for nodeType := range m.nodes {
		typeList = append(typeList, nodeType)
	}
	m.mut.RUnlock()
	return typeList
}

func (m *NodeMap) HasType(nodeType string) bool {
	m.mut.RLock()
	_, found := m.nodes[nodeType]
	m.mut.RUnlock()
	return found
}

// GetNodes 所有本类型的节点，不要修改返回值
func (m *NodeMap) GetNodes(nodeType string) []NodeInfo {
	m.mut.RLock()
	v := m.nodes[nodeType]
	m.mut.RUnlock()
	return v
}

// findNode find node index, with lock holding
func (m *NodeMap) findNode(nodeType string, id uint32) int {
	var nodes = m.nodes[nodeType]
	for i := 0; i < len(nodes); i++ {
		var nodeId = nodes[i].GetID()
		if nodeId == id {
			return i
		}
	}
	return -1
}

// AddNode 添加一个节点
func (m *NodeMap) AddNode(node NodeInfo) {
	m.UpsertNode(node)
}

// FillNodeTypes 填充节点类型，用于判断节点类型是否存在
func (m *NodeMap) FillNodeTypes(dep []string) {
	m.mut.Lock()
	defer m.mut.Unlock()
	for _, nodeType := range dep {
		if _, found := m.nodes[nodeType]; !found {
			m.nodes[nodeType] = nil
		}
	}
}

// UpsertNode 添加一个节点
func (m *NodeMap) UpsertNode(node NodeInfo) bool {
	m.mut.Lock()
	defer m.mut.Unlock()

	var nodeType = node.GetTypeName()
	var nodeId = node.GetID()
	var nodes = m.nodes[nodeType]
	for i, v := range nodes {
		var nid = v.GetID()
		if nid == nodeId {
			nodes[i] = node
			return false
		}
	}
	m.nodes[nodeType] = append(nodes, node)
	return true
}

func (m *NodeMap) Clear() {
	m.mut.Lock()
	clear(m.nodes)
	m.mut.Unlock()
}

// DeleteNodes 删除某一类型的所有节点
func (m *NodeMap) DeleteNodes(nodeType string) {
	m.mut.Lock()
	m.nodes[nodeType] = nil
	m.mut.Unlock()
}

// DeleteNode 删除一个节点
func (m *NodeMap) DeleteNode(nodeType string, id uint32) {
	m.mut.Lock()
	defer m.mut.Unlock()

	var idx = m.findNode(nodeType, id)
	if idx >= 0 {
		var nodes = m.nodes[nodeType]
		nodes = slice.RemoveAt(nodes, idx)
		if len(nodes) == 0 {
			delete(m.nodes, nodeType)
		} else {
			m.nodes[nodeType] = nodes
		}
	}
}

func (m *NodeMap) String() string {
	var sb strings.Builder
	for name, set := range m.nodes {
		fmt.Fprintf(&sb, "%s: %v,\n", name, set)
	}
	return sb.String()
}
