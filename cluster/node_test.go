// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package cluster

import (
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
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
