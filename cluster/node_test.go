// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package cluster

import (
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewNode(t *testing.T) {
	var node = NewNodeFrom("GATE", 1)
	assert.Equal(t, "GATE", node.GetType())
	assert.Equal(t, uint32(1), node.GetID())
	assert.Equal(t, "", node.GetString("not_exist"))
}

func TestNodeInfo_SetString(t *testing.T) {
	var node = NewNodeFrom("GATE", 1)
	node.SetString("hello", "world")
	assert.Equal(t, "world", node.GetString("hello"))
	assert.Equal(t, "", node.GetString("not_exist"))
}

func TestNodeInfo_SetInt(t *testing.T) {
	var node = NewNodeFrom("GATE", 1)
	node.SetInt("foo", int64(123))
	node.SetInt("bar", int64(0))
	assert.Equal(t, int64(123), node.GetInt("foo"))
	assert.Equal(t, int64(0), node.GetInt("bar"))
	assert.Equal(t, int64(0), node.GetInt("not_exist"))
}

func TestNodeInfo_SetBool(t *testing.T) {
	var node = NewNodeFrom("GATE", 1)
	node.SetBool("key1", true)
	node.SetBool("key2", false)
	assert.Equal(t, true, node.GetBool("key1"))
	assert.Equal(t, false, node.GetBool("key2"))
	assert.Equal(t, false, node.GetBool("not_exist"))
}

func TestNodeInfo_Clone(t *testing.T) {
	var node = NewNodeFrom("GATE", 1)
	var ref = node
	var clone = node.Clone()
	for k, v := range node {
		assert.Equal(t, v, clone[k])
	}
	node.SetString("hello", "world")
	assert.Equal(t, "world", ref.GetString("hello"))
	assert.Equal(t, "", clone.GetString("hello"))
	clone.SetString("foo", "bar")
	assert.Equal(t, "", node.GetString("foo"))
}

func createNodesMap(text string) *NodeMap {
	var nm = NewNodeMap()
	var parts = strings.Split(text, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		var pair = strings.Split(part, "=")
		if len(pair) == 2 {
			var strKey = strings.TrimSpace(pair[0])
			var strId = strings.TrimSpace(pair[1])
			n, _ := strconv.Atoi(strId)
			nm.UpsertNode(NewNodeFrom(strKey, uint32(n)))
		}
	}
	return nm
}

func TestNodeMap_Count(t *testing.T) {
	tests := []struct {
		input string
		want1 int
		ntype string
		want2 int
	}{
		{"", 0, "", 0},
		{"GATE=1, GATE=2, GAME=1, GAME=2, GAME=3, LOGIN=1", 6, "GATE", 2},
		{"GATE=1, GATE=2, GAME=1, GAME=2, GAME=3, LOGIN=1", 6, "GAME", 3},
		{"GATE=1, GATE=2, GAME=1, GAME=2, GAME=3, LOGIN=1", 6, "LOGIN", 1},
		{"GATE=1, GATE=2, GAME=1, GAME=2, GAME=3, LOGIN=1", 6, "MAIL", 0},
	}
	for i, tt := range tests {
		var name = fmt.Sprintf("case-%d", i+1)
		t.Run(name, func(t *testing.T) {
			var nm = createNodesMap(tt.input)
			var got1 = nm.TotalCount()
			var got2 = nm.GetCountOf(tt.ntype)
			if got1 != tt.want1 {
				t.Fatalf("Count() = %v, want %v", got1, tt.want1)
			}
			if got2 != tt.want2 {
				t.Fatalf("CountOf(%s) = %v, want %v", tt.ntype, got2, tt.want2)
			}
		})
	}
}

func TestNodeMap_GetTypeList(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", []string{}},
		{"GATE=1, GATE=2, GAME=1, GAME=2, GAME=3, LOGIN=1, LOGIN=2", []string{"GATE", "GAME", "LOGIN"}},
	}
	for i, tt := range tests {
		var name = fmt.Sprintf("case-%d", i+1)
		t.Run(name, func(t *testing.T) {
			var nm = createNodesMap(tt.input)
			var got = nm.GetTypeList()
			slices.Sort(got)
			slices.Sort(tt.want)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("GetTypeList() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNodeMap_findNode(t *testing.T) {
	tests := []struct {
		input string
		ntype string
		id    uint32
		want  int
	}{
		{"GATE=1, GATE=2, GAME=1, GAME=2, GAME=3, LOGIN=1, LOGIN=2", "MAIL", 1, -1},
		{"GATE=1, GATE=2, GAME=1, GAME=2, GAME=3, LOGIN=1, LOGIN=2", "GATE", 1, 0},
		{"GATE=1, GATE=2, GAME=1, GAME=2, GAME=3, LOGIN=1, LOGIN=2", "GATE", 2, 1},
		{"GATE=1, GATE=2, GAME=1, GAME=2, GAME=3, LOGIN=1, LOGIN=2", "LOGIN", 2, 1},
	}
	for i, tt := range tests {
		var name = fmt.Sprintf("case-%d", i+1)
		t.Run(name, func(t *testing.T) {
			var nm = createNodesMap(tt.input)
			var got = nm.findNode(tt.ntype, tt.id)
			if got != tt.want {
				t.Fatalf("FindNode(%s) = %v, want %v", tt.ntype, got, tt.want)
			}
		})
	}
}

func TestNodeMap_UpsertNode(t *testing.T) {
	var nm = createNodesMap("")
	nm.AddNode(NewNodeFrom("LOGIN", 1))
	nm.AddNode(NewNodeFrom("GATE", 1))
	nm.AddNode(NewNodeFrom("GATE", 1))
	assert.Equal(t, 1, nm.GetCountOf("GATE"))
	assert.Equal(t, 1, nm.GetCountOf("LOGIN"))
	nm.AddNode(NewNodeFrom("GATE", 2))
	assert.Equal(t, 2, nm.GetCountOf("GATE"))
}

func TestNodeMap_DeleteNodes(t *testing.T) {
	var nm = createNodesMap("")
	nm.AddNode(NewNodeFrom("GATE", 1))
	nm.AddNode(NewNodeFrom("GATE", 2))
	nm.AddNode(NewNodeFrom("LOGIN", 1))
	assert.Equal(t, 2, nm.GetCountOf("GATE"))
	assert.Equal(t, 1, nm.GetCountOf("LOGIN"))
	nm.DeleteNodes("GATE")
	assert.Equal(t, 0, nm.GetCountOf("GATE"))
	assert.Equal(t, 1, nm.GetCountOf("LOGIN"))
}

func TestNodeMap_DeleteNode(t *testing.T) {
	var nm = createNodesMap("")
	nm.AddNode(NewNodeFrom("GATE", 1))
	nm.AddNode(NewNodeFrom("GATE", 2))
	nm.AddNode(NewNodeFrom("LOGIN", 1))
	assert.Equal(t, 2, nm.GetCountOf("GATE"))
	nm.DeleteNode("GATE", 2)
	assert.Equal(t, 1, nm.GetCountOf("GATE"))
	assert.Equal(t, 1, nm.GetCountOf("LOGIN"))
}

func TestNodeMap_ZeroValue(t *testing.T) {
	t.Run("read", func(t *testing.T) {
		var m NodeMap
		if m.TotalCount() != 0 || m.GetCountOf("GATE") != 0 || m.HasType("GATE") {
			t.Fatal("zero value must be empty")
		}
		if m.GetNodesCopy("GATE") != nil || len(m.GetTypeList()) != 0 || m.String() != "" {
			t.Fatal("zero value must return empty snapshots")
		}
		m.DeleteNode("GATE", 1)
		m.Clear()
	})
	t.Run("upsert", func(t *testing.T) {
		var m NodeMap
		if !m.UpsertNode(NodeInfo{NodeKeyType: "GATE", NodeKeyID: int64(1)}) {
			t.Fatal("first upsert must insert")
		}
		if m.Init() != &m || m.Init().TotalCount() != 1 {
			t.Fatal("Init must preserve existing nodes")
		}
	})
	t.Run("fill", func(t *testing.T) {
		var m NodeMap
		m.FillNodeTypes([]string{"GATE", "GATE"})
		if !m.HasType("GATE") || m.GetNodesCopy("GATE") != nil {
			t.Fatal("FillNodeTypes must create an empty type")
		}
	})
	t.Run("delete type", func(t *testing.T) {
		var m NodeMap
		m.DeleteNodes("GATE")
		if !m.HasType("GATE") || m.TotalCount() != 0 {
			t.Fatal("DeleteNodes must preserve an empty type")
		}
	})
}

func TestNodeMap_InputOwnership(t *testing.T) {
	for _, method := range []string{"AddNode", "UpsertNode"} {
		t.Run(method, func(t *testing.T) {
			m := NewNodeMap()
			node := NodeInfo{NodeKeyType: "GATE", NodeKeyID: int64(1), NodeKeyURI: "old"}
			if method == "AddNode" {
				m.AddNode(node)
			} else {
				m.UpsertNode(node)
			}
			node.SetString(NodeKeyURI, "changed")
			node.SetString(NodeKeyType, "GAME")
			node.SetInt(NodeKeyID, 2)
			stored := m.GetNodesCopy("GATE")[0]
			if stored.GetURI() != "old" || stored.GetType() != "GATE" || stored.GetID() != 1 {
				t.Fatalf("input mutation changed stored node: %v", stored)
			}
			m.DeleteNode("GATE", 1)
			if m.HasType("GATE") {
				t.Fatal("input mutation must not change the stored identity")
			}
		})
	}
}

func TestNodeMap_GetNodesSnapshot(t *testing.T) {
	m := NewNodeMap()
	m.AddNode(NodeInfo{NodeKeyType: "GATE", NodeKeyID: int64(1), NodeKeyURI: "old"})
	m.AddNode(NodeInfo{NodeKeyType: "GATE", NodeKeyID: int64(2)})
	snapshot := m.GetNodesCopy("GATE")
	if m.UpsertNode(NodeInfo{NodeKeyType: "GATE", NodeKeyID: int64(1), NodeKeyURI: "new"}) {
		t.Fatal("upsert of an existing identity must replace")
	}
	if snapshot[0].GetURI() != "old" || m.GetNodesCopy("GATE")[0].GetURI() != "new" {
		t.Fatal("replacement must preserve order and leave earlier snapshots unchanged")
	}
	m.DeleteNode("GATE", 1)
	if snapshot[0].GetID() != 1 || snapshot[1].GetID() != 2 {
		t.Fatal("deletion must not overwrite or clear earlier snapshots")
	}
	current := m.GetNodesCopy("GATE")
	current[0].SetString(NodeKeyType, "changed")
	current[0] = nil
	if m.GetNodesCopy("GATE")[0].GetType() != "GATE" {
		t.Fatal("snapshot mutation must not change stored nodes")
	}
	m.Clear()
	if snapshot[0].GetURI() != "old" || m.TotalCount() != 0 {
		t.Fatal("Clear must not invalidate earlier snapshots")
	}
}

func TestNodeMap_GetNodesCompatibility(t *testing.T) {
	m := NewNodeMap()
	if m.GetNodesRef("missing") != nil {
		t.Fatal("missing type must return nil")
	}
	m.AddNode(NodeInfo{NodeKeyType: "GATE", NodeKeyID: int64(1)})
	nodes := m.GetNodesRef("GATE")
	nodes[0].SetInt(NodeKeyID, 99)
	nodes[0] = nil
	if got := m.GetNodesRef("GATE")[0].GetID(); got != 1 {
		t.Fatalf("GetNodes must return an independent snapshot, got ID %d", got)
	}
}

func TestNodeMap_GetRefNodes(t *testing.T) {
	var m NodeMap
	if m.GetRefNodes("missing") != nil {
		t.Fatal("missing type must return nil")
	}
	m.AddNode(NodeInfo{NodeKeyType: "GATE", NodeKeyID: int64(1), NodeKeyURI: "old"})
	m.AddNode(NodeInfo{NodeKeyType: "GATE", NodeKeyID: int64(2)})
	nodes := m.GetRefNodes("GATE")
	first, second := nodes[0], nodes[1]
	nodes[0] = nil // The slice belongs to the caller; the NodeInfo maps are read-only.
	if m.GetRefNodes("GATE")[0].GetID() != 1 {
		t.Fatal("returned slice must not alias internal storage")
	}
	nodes[0] = first
	replacement := NodeInfo{NodeKeyType: "GATE", NodeKeyID: int64(1), NodeKeyURI: "new"}
	m.UpsertNode(replacement)
	replacement.SetString(NodeKeyURI, "changed input")
	if first.GetURI() != "old" || m.GetRefNodes("GATE")[0].GetURI() != "new" {
		t.Fatal("upsert must preserve earlier references and copy replacement input")
	}
	m.DeleteNode("GATE", 1)
	m.Clear()
	if nodes[0].GetURI() != "old" || nodes[1].GetID() != second.GetID() {
		t.Fatal("deletion and Clear must preserve earlier references")
	}
}

func TestNodeMap_TypeLifecycle(t *testing.T) {
	m := NewNodeMap()
	m.FillNodeTypes([]string{"GATE", "GAME"})
	m.AddNode(NodeInfo{NodeKeyType: "GATE", NodeKeyID: int64(1)})
	m.FillNodeTypes([]string{"GATE"})
	if m.GetCountOf("GATE") != 1 {
		t.Fatal("filling an existing type must preserve its nodes")
	}
	m.DeleteNode("GAME", 1)
	if !m.HasType("GAME") {
		t.Fatal("deleting a missing node must preserve an empty type")
	}
	m.DeleteNodes("GATE")
	if !m.HasType("GATE") || m.GetCountOf("GATE") != 0 {
		t.Fatal("DeleteNodes must preserve the type")
	}
	m.AddNode(NodeInfo{NodeKeyType: "GATE", NodeKeyID: int64(1)})
	m.DeleteNode("GATE", 99)
	if m.TotalCount() != 1 {
		t.Fatal("deleting a missing ID must preserve existing nodes")
	}
	m.DeleteNode("GATE", 1)
	if m.HasType("GATE") {
		t.Fatal("deleting the last node must remove the type")
	}
	m.Clear()
	if m.HasType("GAME") || len(m.GetTypeList()) != 0 {
		t.Fatal("Clear must also remove empty types")
	}
}

// nodeMapStringValue verifies that JSON formatting may safely call back into NodeMap.
type nodeMapStringValue struct{ nodes *NodeMap }

func (v nodeMapStringValue) MarshalJSON() ([]byte, error) {
	v.nodes.UpsertNode(NodeInfo{NodeKeyType: "GAME", NodeKeyID: int64(2)})
	return []byte(`"value"`), nil
}

func TestNodeMap_StringSnapshot(t *testing.T) {
	m := NewNodeMap()
	m.AddNode(NodeInfo{
		NodeKeyType: "GATE", NodeKeyID: int64(1),
		"custom": nodeMapStringValue{nodes: m},
	})
	text := m.String()
	if !strings.Contains(text, "GATE: ") || !strings.Contains(text, `"custom":"value"`) {
		t.Fatalf("unexpected formatting: %s", text)
	}
	if strings.Contains(text, "GAME: ") || m.TotalCount() != 2 {
		t.Fatal("String must format its snapshot without holding the NodeMap lock")
	}
}

func TestNodeMap_ConcurrentAccess(t *testing.T) {
	var m NodeMap
	start := make(chan struct{})
	var wg sync.WaitGroup
	for worker := range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := range 256 {
				switch worker {
				case 0, 1:
					node := NodeInfo{NodeKeyType: "GATE", NodeKeyID: int64(i%8 + 1)}
					m.UpsertNode(node)
					node.SetString(NodeKeyType, "changed input")
				case 2:
					for _, node := range m.GetNodesCopy("GATE") {
						if node.GetType() != "GATE" || node.GetID() == 0 {
							t.Errorf("invalid snapshot: %v", node)
						}
						node.SetString(NodeKeyType, "changed output")
					}
				case 3:
					m.DeleteNode("GATE", uint32(i%8+1))
					m.FillNodeTypes([]string{"GAME"})
					m.DeleteNodes("GAME")
				case 4:
					for _, node := range m.GetRefNodes("GATE") {
						if node.GetType() != "GATE" || node.GetID() == 0 {
							t.Errorf("invalid read-only reference: %v", node)
						}
					}
					_ = m.GetNodesRef("GATE")
					_ = m.String()
					_ = m.GetTypeList()
					_ = m.TotalCount()
					_ = m.GetCountOf("GATE")
					_ = m.HasType("GATE")
				case 5:
					m.Init()
					m.Clear()
				}
			}
		}()
	}
	close(start)
	wg.Wait()
	var count int
	for _, nodeType := range m.GetTypeList() {
		count += len(m.GetNodesCopy(nodeType))
	}
	if count != m.TotalCount() {
		t.Fatal("total count must match the final snapshots")
	}
}

func BenchmarkNodeMap_GetNodes(b *testing.B) {
	for _, count := range []int{8, 128} {
		b.Run(fmt.Sprintf("nodes=%d", count), func(b *testing.B) {
			m := NewNodeMap()
			for i := range count {
				m.AddNode(NodeInfo{NodeKeyType: "GATE", NodeKeyID: int64(i + 1)})
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				_ = m.GetNodesCopy("GATE")
			}
		})
	}
}

func BenchmarkNodeMap_GetRefNodes(b *testing.B) {
	for _, count := range []int{8, 128} {
		b.Run(fmt.Sprintf("nodes=%d", count), func(b *testing.B) {
			m := NewNodeMap()
			for i := range count {
				m.AddNode(NodeInfo{NodeKeyType: "GATE", NodeKeyID: int64(i + 1)})
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				_ = m.GetRefNodes("GATE")
			}
		})
	}
}
