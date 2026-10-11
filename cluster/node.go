// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package cluster

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"slices"
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

func (n NodeInfo) GetAny(key string) any {
	if val, ok := n[key]; ok {
		return val
	}
	return nil
}

func (n NodeInfo) GetString(key string) string {
	if val, ok := n[key]; ok {
		return conv.FormatAny(val)
	}
	return ""
}

func (n NodeInfo) GetJSONString(key string) string {
	if val, ok := n[key]; ok {
		data, _ := json.Marshal(val)
		return string(data)
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
	s, _ := gutil.JSONStringify(n)
	return s
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

// NodeMap 按服务类型保存节点信息，零值可直接使用，公开方法可并发调用。
// 首次使用后不可复制。mut 保护索引和切片，存入的 NodeInfo 不再被内部修改。
// 输入和快照只复制 NodeInfo 第一层；嵌套 map、slice、指针等值仍共享，
// 调用方应保持这些值只读，或自行同步访问。传入节点在方法返回前不可修改。
type NodeMap struct {
	mut   sync.RWMutex
	nodes map[string][]NodeInfo
}

func NewNodeMap() *NodeMap {
	return new(NodeMap).Init()
}

// Init 初始化，保留已有节点。零值也可直接调用其他方法。
func (m *NodeMap) Init() *NodeMap {
	m.mut.Lock()
	m.initLocked()
	m.mut.Unlock()
	return m
}

// initLocked 在持有写锁时初始化存储。
func (m *NodeMap) initLocked() {
	if m.nodes == nil {
		m.nodes = make(map[string][]NodeInfo)
	}
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

// GetNodesRef 返回本类型的节点
func (m *NodeMap) GetNodesRef(nodeType string) []NodeInfo {
	m.mut.RLock()
	v := m.nodes[nodeType]
	m.mut.RUnlock()
	return v
}

// GetNodesCopy 返回独立的切片及 NodeInfo 第一层副本，保留节点顺序。
// 无节点时返回 nil；修改返回的切片和节点字段不影响内部存储。
// 嵌套引用值仍然共享，参见 NodeMap 的说明。
func (m *NodeMap) GetNodesCopy(nodeType string) []NodeInfo {
	m.mut.RLock()
	defer m.mut.RUnlock()
	nodes := m.nodes[nodeType]
	if len(nodes) == 0 {
		return nil
	}
	result := make([]NodeInfo, len(nodes))
	for i, node := range nodes {
		result[i] = node.Clone()
	}
	return result
}

// GetRefNodes 返回独立的切片，其中的 NodeInfo 引用仅供只读访问。
// 不可修改返回的 NodeInfo 或其嵌套值；需要修改时使用 GetNodesCopy。
// 后续增删或替换节点不会修改已返回的切片和 NodeInfo。
func (m *NodeMap) GetRefNodes(nodeType string) []NodeInfo {
	m.mut.RLock()
	defer m.mut.RUnlock()
	return slices.Clone(m.nodes[nodeType])
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
	m.initLocked()
	for _, nodeType := range dep {
		if _, found := m.nodes[nodeType]; !found {
			m.nodes[nodeType] = nil
		}
	}
}

// UpsertNode 按类型和 ID 插入或替换节点，保存 NodeInfo 第一层副本。
// 插入返回 true，替换返回 false；替换保留原有位置。
func (m *NodeMap) UpsertNode(node NodeInfo) bool {
	node = node.Clone()
	nodeType, nodeId := node.GetTypeName(), node.GetID()
	m.mut.Lock()
	defer m.mut.Unlock()
	m.initLocked()
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

// DeleteNodes 删除某一类型的所有节点，保留该类型，包含原先不存在的类型。
func (m *NodeMap) DeleteNodes(nodeType string) {
	m.mut.Lock()
	m.initLocked()
	m.nodes[nodeType] = nil
	m.mut.Unlock()
}

// DeleteNode 删除一个节点，不保证剩余节点顺序。删除最后一个节点时移除该类型。
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
	// 先获取切片快照，再格式化不可变的节点；自定义 JSON 序列化可能回调 NodeMap。
	m.mut.RLock()
	snapshot := make(map[string][]NodeInfo, len(m.nodes))
	for name, nodes := range m.nodes {
		snapshot[name] = slices.Clone(nodes)
	}
	m.mut.RUnlock()
	var sb strings.Builder
	for name, set := range snapshot {
		fmt.Fprintf(&sb, "%s: %v,\n", name, set)
	}
	return sb.String()
}
