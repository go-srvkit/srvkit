// Copyright © 2023 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package treemap

import (
	"cmp"
	"fmt"
	"math"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- Color.String ---

func TestColorString(t *testing.T) {
	tests := []struct {
		name  string
		color Color
		want  string
	}{
		{"red", RED, "red"},
		{"black", BLACK, "black"},
		{"unknown", Color(99), "??"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.color.String())
		})
	}
}

// --- Entry ---

func TestNewEntry(t *testing.T) {
	e := NewEntry("k", "v", nil)
	assert.Equal(t, "k", e.GetKey())
	assert.Equal(t, "v", e.GetValue())
	assert.Equal(t, BLACK, e.color)
	assert.Nil(t, e.parent)
}

func TestEntrySetValue(t *testing.T) {
	tests := []struct {
		name    string
		oldVal  string
		newVal  string
		wantOld string
	}{
		{"normal update", "v1", "v2", "v1"},
		{"set same value", "v1", "v1", "v1"},
		{"set empty string", "v1", "", "v1"},
		{"from empty", "", "v2", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewEntry("k", tt.oldVal, nil)
			got := e.SetValue(tt.newVal)
			assert.Equal(t, tt.wantOld, got)
			assert.Equal(t, tt.newVal, e.GetValue())
		})
	}
}

func TestEntryHeight(t *testing.T) {
	tests := []struct {
		name string
		tree func() *Entry[int, int]
		want int
	}{
		{
			"nil entry",
			func() *Entry[int, int] { return nil },
			0,
		},
		{
			"single node",
			func() *Entry[int, int] { return NewEntry(1, 1, nil) },
			1,
		},
		{
			"node with left child only",
			func() *Entry[int, int] {
				root := NewEntry(2, 2, nil)
				left := NewEntry(1, 1, root)
				root.left = left
				return root
			},
			2,
		},
		{
			"node with right child only",
			func() *Entry[int, int] {
				root := NewEntry(1, 1, nil)
				right := NewEntry(2, 2, root)
				root.right = right
				return root
			},
			2,
		},
		{
			"balanced tree",
			func() *Entry[int, int] {
				root := NewEntry(2, 2, nil)
				root.left = NewEntry(1, 1, root)
				root.right = NewEntry(3, 3, root)
				return root
			},
			2,
		},
		{
			"deep tree",
			func() *Entry[int, int] {
				root := NewEntry(3, 3, nil)
				root.left = NewEntry(2, 2, root)
				root.left.left = NewEntry(1, 1, root.left)
				return root
			},
			3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.tree().Height())
		})
	}
}

// --- Constructors ---

func TestNew(t *testing.T) {
	m := New[int, string](func(a, b int) int { return a - b })
	assert.NotNil(t, m)
	assert.Equal(t, 0, m.Size())
	assert.True(t, m.IsEmpty())
}

func TestNewOrderedTreeMap(t *testing.T) {
	m := NewOrderedTreeMap[int, string]()
	assert.NotNil(t, m)
	assert.Equal(t, 0, m.Size())
	assert.True(t, m.IsEmpty())
}

func TestNewTreeMapOf(t *testing.T) {
	tests := []struct {
		name     string
		input    map[int]string
		wantSize int
		wantKeys []int
	}{
		{"empty map", map[int]string{}, 0, nil},
		{"single entry", map[int]string{1: "a"}, 1, []int{1}},
		{"multiple entries", map[int]string{3: "c", 1: "a", 2: "b"}, 3, []int{1, 2, 3}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewTreeMapOf(tt.input)
			assert.Equal(t, tt.wantSize, m.Size())
			if tt.wantKeys != nil {
				assert.Equal(t, tt.wantKeys, m.Keys())
			}
		})
	}
}

// --- Size / IsEmpty ---

func TestSizeAndIsEmpty(t *testing.T) {
	m := NewOrderedTreeMap[int, string]()
	assert.Equal(t, 0, m.Size())
	assert.True(t, m.IsEmpty())

	m.Put(1, "a")
	assert.Equal(t, 1, m.Size())
	assert.False(t, m.IsEmpty())

	m.Remove(1)
	assert.Equal(t, 0, m.Size())
	assert.True(t, m.IsEmpty())
}

// --- ContainsKey ---

func TestContainsKey(t *testing.T) {
	tests := []struct {
		name   string
		setup  func() *Map[int, string]
		key    int
		expect bool
	}{
		{
			"empty map",
			func() *Map[int, string] { return NewOrderedTreeMap[int, string]() },
			1, false,
		},
		{
			"key exists",
			func() *Map[int, string] {
				m := NewOrderedTreeMap[int, string]()
				m.Put(1, "a")
				return m
			},
			1, true,
		},
		{
			"key not exists",
			func() *Map[int, string] {
				m := NewOrderedTreeMap[int, string]()
				m.Put(1, "a")
				return m
			},
			2, false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.setup()
			assert.Equal(t, tt.expect, m.ContainsKey(tt.key))
		})
	}
}

// --- Get / Load ---

func TestGet(t *testing.T) {
	tests := []struct {
		name      string
		setup     func() *Map[int, string]
		key       int
		wantVal   string
		wantFound bool
	}{
		{
			"empty map",
			func() *Map[int, string] { return NewOrderedTreeMap[int, string]() },
			1, "", false,
		},
		{
			"key exists",
			func() *Map[int, string] {
				m := NewOrderedTreeMap[int, string]()
				m.Put(1, "a")
				return m
			},
			1, "a", true,
		},
		{
			"key not exists",
			func() *Map[int, string] {
				m := NewOrderedTreeMap[int, string]()
				m.Put(1, "a")
				return m
			},
			2, "", false,
		},
		{
			"zero value stored",
			func() *Map[int, string] {
				m := NewOrderedTreeMap[int, string]()
				m.Put(1, "")
				return m
			},
			1, "", true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.setup()
			val, found := m.Get(tt.key)
			assert.Equal(t, tt.wantFound, found)
			assert.Equal(t, tt.wantVal, val)
		})
	}
}

func TestLoad(t *testing.T) {
	m := NewOrderedTreeMap[int, string]()
	m.Put(1, "a")

	val, found := m.Load(1)
	assert.True(t, found)
	assert.Equal(t, "a", val)

	val, found = m.Load(99)
	assert.False(t, found)
	assert.Equal(t, "", val)
}

// --- GetOrDefault ---

func TestGetOrDefault(t *testing.T) {
	tests := []struct {
		name   string
		setup  func() *Map[int, string]
		key    int
		defVal string
		want   string
	}{
		{
			"key exists returns stored value",
			func() *Map[int, string] {
				m := NewOrderedTreeMap[int, string]()
				m.Put(1, "a")
				return m
			},
			1, "default", "a",
		},
		{
			"key not exists returns default",
			func() *Map[int, string] {
				m := NewOrderedTreeMap[int, string]()
				m.Put(1, "a")
				return m
			},
			2, "default", "default",
		},
		{
			"empty map returns default",
			func() *Map[int, string] { return NewOrderedTreeMap[int, string]() },
			1, "fallback", "fallback",
		},
		{
			"stored zero value vs default",
			func() *Map[int, string] {
				m := NewOrderedTreeMap[int, string]()
				m.Put(1, "")
				return m
			},
			1, "default", "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.setup()
			assert.Equal(t, tt.want, m.GetOrDefault(tt.key, tt.defVal))
		})
	}
}

// --- FirstKey / LastKey ---

func TestFirstKeyLastKey(t *testing.T) {
	tests := []struct {
		name        string
		setup       func() *Map[int, string]
		wantFirst   int
		wantFirstOk bool
		wantLast    int
		wantLastOk  bool
	}{
		{
			"empty map",
			func() *Map[int, string] { return NewOrderedTreeMap[int, string]() },
			0, false, 0, false,
		},
		{
			"single element",
			func() *Map[int, string] {
				m := NewOrderedTreeMap[int, string]()
				m.Put(5, "e")
				return m
			},
			5, true, 5, true,
		},
		{
			"multiple elements",
			func() *Map[int, string] {
				m := NewOrderedTreeMap[int, string]()
				m.Put(3, "c")
				m.Put(1, "a")
				m.Put(5, "e")
				m.Put(2, "b")
				m.Put(4, "d")
				return m
			},
			1, true, 5, true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.setup()
			fk, fok := m.FirstKey()
			lk, lok := m.LastKey()
			assert.Equal(t, tt.wantFirstOk, fok)
			assert.Equal(t, tt.wantLastOk, lok)
			if fok {
				assert.Equal(t, tt.wantFirst, fk)
			}
			if lok {
				assert.Equal(t, tt.wantLast, lk)
			}
		})
	}
}

// --- FirstEntry / LastEntry / RootEntry ---

func TestFirstEntryLastEntryRootEntry(t *testing.T) {
	m := NewOrderedTreeMap[int, string]()
	assert.Nil(t, m.FirstEntry())
	assert.Nil(t, m.LastEntry())
	assert.Nil(t, m.RootEntry())

	m.Put(2, "b")
	m.Put(1, "a")
	m.Put(3, "c")

	assert.NotNil(t, m.RootEntry())
	assert.Equal(t, 2, m.RootEntry().GetKey())

	assert.Equal(t, 1, m.FirstEntry().GetKey())
	assert.Equal(t, "a", m.FirstEntry().GetValue())

	assert.Equal(t, 3, m.LastEntry().GetKey())
	assert.Equal(t, "c", m.LastEntry().GetValue())
}

// --- FloorEntry / FloorKey ---

func TestFloorEntryFloorKey(t *testing.T) {
	m := NewOrderedTreeMap[int, string]()
	for _, k := range []int{10, 20, 30, 40, 50} {
		m.Put(k, fmt.Sprintf("v%d", k))
	}

	tests := []struct {
		name      string
		key       int
		wantKey   int
		wantFound bool
	}{
		{"exact match", 20, 20, true},
		{"between keys", 25, 20, true},
		{"below all keys", 5, 0, false},
		{"above all keys", 55, 50, true},
		{"at first key", 10, 10, true},
		{"at last key", 50, 50, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := m.FloorEntry(tt.key)
			fk, fok := m.FloorKey(tt.key)
			if tt.wantFound {
				assert.NotNil(t, entry)
				assert.Equal(t, tt.wantKey, entry.GetKey())
				assert.True(t, fok)
				assert.Equal(t, tt.wantKey, fk)
			} else {
				assert.Nil(t, entry)
				assert.False(t, fok)
			}
		})
	}
}

func TestFloorKeyEmptyMap(t *testing.T) {
	m := NewOrderedTreeMap[int, string]()
	_, ok := m.FloorKey(1)
	assert.False(t, ok)
	assert.Nil(t, m.FloorEntry(1))
}

// --- CeilingEntry / CeilingKey ---

func TestCeilingEntryCeilingKey(t *testing.T) {
	m := NewOrderedTreeMap[int, string]()
	for _, k := range []int{10, 20, 30, 40, 50} {
		m.Put(k, fmt.Sprintf("v%d", k))
	}

	tests := []struct {
		name      string
		key       int
		wantKey   int
		wantFound bool
	}{
		{"exact match", 20, 20, true},
		{"between keys", 25, 30, true},
		{"below all keys", 5, 10, true},
		{"above all keys", 55, 0, false},
		{"at first key", 10, 10, true},
		{"at last key", 50, 50, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := m.CeilingEntry(tt.key)
			ck, cok := m.CeilingKey(tt.key)
			if tt.wantFound {
				assert.NotNil(t, entry)
				assert.Equal(t, tt.wantKey, entry.GetKey())
				assert.True(t, cok)
				assert.Equal(t, tt.wantKey, ck)
			} else {
				assert.Nil(t, entry)
				assert.False(t, cok)
			}
		})
	}
}

func TestCeilingKeyEmptyMap(t *testing.T) {
	m := NewOrderedTreeMap[int, string]()
	_, ok := m.CeilingKey(1)
	assert.False(t, ok)
	assert.Nil(t, m.CeilingEntry(1))
}

// --- HigherEntry / HigherKey ---

func TestHigherEntryHigherKey(t *testing.T) {
	m := NewOrderedTreeMap[int, string]()
	for _, k := range []int{10, 20, 30, 40, 50} {
		m.Put(k, fmt.Sprintf("v%d", k))
	}

	tests := []struct {
		name      string
		key       int
		wantKey   int
		wantFound bool
	}{
		{"exact match returns next", 20, 30, true},
		{"between keys", 25, 30, true},
		{"below all keys", 5, 10, true},
		{"above all keys", 55, 0, false},
		{"at last key", 50, 0, false},
		{"one below last", 40, 50, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := m.HigherEntry(tt.key)
			hk, hok := m.HigherKey(tt.key)
			if tt.wantFound {
				assert.NotNil(t, entry)
				assert.Equal(t, tt.wantKey, entry.GetKey())
				assert.True(t, hok)
				assert.Equal(t, tt.wantKey, hk)
			} else {
				assert.Nil(t, entry)
				assert.False(t, hok)
			}
		})
	}
}

func TestHigherKeyEmptyMap(t *testing.T) {
	m := NewOrderedTreeMap[int, string]()
	_, ok := m.HigherKey(1)
	assert.False(t, ok)
	assert.Nil(t, m.HigherEntry(1))
}

// --- Put ---

func TestPut(t *testing.T) {
	tests := []struct {
		name       string
		operations func(t *testing.T, m *Map[int, string])
	}{
		{
			"put new key returns zero value",
			func(t *testing.T, m *Map[int, string]) {
				old := m.Put(1, "a")
				assert.Equal(t, "", old)
				assert.Equal(t, 1, m.Size())
			},
		},
		{
			"put existing key returns old value",
			func(t *testing.T, m *Map[int, string]) {
				m.Put(1, "a")
				old := m.Put(1, "b")
				assert.Equal(t, "a", old)
				assert.Equal(t, 1, m.Size())
				val, _ := m.Get(1)
				assert.Equal(t, "b", val)
			},
		},
		{
			"put multiple keys in order",
			func(t *testing.T, m *Map[int, string]) {
				for i := 1; i <= 5; i++ {
					m.Put(i, fmt.Sprintf("v%d", i))
				}
				assert.Equal(t, 5, m.Size())
				assert.Equal(t, []int{1, 2, 3, 4, 5}, m.Keys())
			},
		},
		{
			"put multiple keys in reverse order",
			func(t *testing.T, m *Map[int, string]) {
				for i := 5; i >= 1; i-- {
					m.Put(i, fmt.Sprintf("v%d", i))
				}
				assert.Equal(t, 5, m.Size())
				assert.Equal(t, []int{1, 2, 3, 4, 5}, m.Keys())
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewOrderedTreeMap[int, string]()
			tt.operations(t, m)
		})
	}
}

func TestPutResult(t *testing.T) {
	tests := []struct {
		name        string
		initial     map[int]string
		key         int
		replaceOld  bool
		wantPrev    string
		wantExisted bool
		wantValue   string
	}{
		{"insert into empty map", nil, 2, true, "", false, "new"},
		{"insert if absent into empty map", nil, 2, false, "", false, "new"},
		{"insert left child", map[int]string{2: "old"}, 1, true, "", false, "new"},
		{"insert right child if absent", map[int]string{2: "old"}, 3, false, "", false, "new"},
		{"replace existing value", map[int]string{2: "old"}, 2, true, "old", true, "new"},
		{"keep existing value", map[int]string{2: "old"}, 2, false, "old", true, "old"},
		{"replace existing zero value", map[int]string{2: ""}, 2, true, "", true, "new"},
		{"keep existing zero value", map[int]string{2: ""}, 2, false, "", true, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewTreeMapOf(tt.initial)
			version := m.version
			previous, existed := m.put(tt.key, "new", tt.replaceOld)
			assert.Equal(t, tt.wantPrev, previous)
			assert.Equal(t, tt.wantExisted, existed)
			value, found := m.Get(tt.key)
			assert.True(t, found)
			assert.Equal(t, tt.wantValue, value)
			wantSize := len(tt.initial)
			if !tt.wantExisted {
				wantSize++
				version++
			}
			assert.Equal(t, wantSize, m.Size())
			assert.Equal(t, version, m.version)
		})
	}
}

// --- Store ---

func TestStore(t *testing.T) {
	m := NewOrderedTreeMap[int, string]()
	m.Store(1, "a")
	assert.Equal(t, 1, m.Size())
	val, found := m.Get(1)
	assert.True(t, found)
	assert.Equal(t, "a", val)

	m.Store(1, "b")
	val, found = m.Get(1)
	assert.True(t, found)
	assert.Equal(t, "b", val)
	assert.Equal(t, 1, m.Size())
}

// --- Swap ---

func TestSwap(t *testing.T) {
	tests := []struct {
		name       string
		setup      func() *Map[int, string]
		key        int
		newVal     string
		wantPrev   string
		wantLoaded bool
		wantNewVal string
	}{
		{
			"swap existing key",
			func() *Map[int, string] {
				m := NewOrderedTreeMap[int, string]()
				m.Put(1, "a")
				return m
			},
			1, "b", "a", true, "b",
		},
		{
			"swap non-existing key",
			func() *Map[int, string] {
				m := NewOrderedTreeMap[int, string]()
				m.Put(1, "a")
				return m
			},
			2, "b", "", false, "b",
		},
		{
			"swap on empty map",
			func() *Map[int, string] { return NewOrderedTreeMap[int, string]() },
			1, "a", "", false, "a",
		},
		{
			"swap existing zero value",
			func() *Map[int, string] {
				m := NewOrderedTreeMap[int, string]()
				m.Put(1, "")
				return m
			},
			1, "b", "", true, "b",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.setup()
			prev, loaded := m.Swap(tt.key, tt.newVal)
			assert.Equal(t, tt.wantPrev, prev)
			assert.Equal(t, tt.wantLoaded, loaded)
			val, found := m.Get(tt.key)
			assert.True(t, found)
			assert.Equal(t, tt.wantNewVal, val)
		})
	}
}

func TestSwapSingleSearch(t *testing.T) {
	for _, key := range []int{0, 1, 3, 4} {
		t.Run(fmt.Sprintf("key=%d", key), func(t *testing.T) {
			comparisons := 0
			m := New[int, string](func(a, b int) int {
				comparisons++
				return cmp.Compare(a, b)
			})
			for _, k := range []int{2, 1, 3} {
				m.Put(k, "old")
			}
			comparisons = 0
			m.Get(key)
			wantComparisons := comparisons
			comparisons = 0
			m.Swap(key, "new")
			assert.Equal(t, wantComparisons, comparisons, "Swap should search the tree only once")
		})
	}
}

// --- PutIfAbsent ---

func TestPutIfAbsent(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() *Map[int, string]
		key     int
		value   string
		want    string
		wantVal string
	}{
		{
			"key not present inserts new",
			func() *Map[int, string] { return NewOrderedTreeMap[int, string]() },
			1, "a", "", "a",
		},
		{
			"key present keeps old",
			func() *Map[int, string] {
				m := NewOrderedTreeMap[int, string]()
				m.Put(1, "old")
				return m
			},
			1, "new", "old", "old",
		},
		{
			"key present with zero value",
			func() *Map[int, string] {
				m := NewOrderedTreeMap[int, string]()
				m.Put(1, "")
				return m
			},
			1, "new", "", "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.setup()
			got := m.PutIfAbsent(tt.key, tt.value)
			assert.Equal(t, tt.want, got)
			val, _ := m.Get(tt.key)
			assert.Equal(t, tt.wantVal, val)
		})
	}
}

// --- Remove / Delete ---

func TestRemove(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() *Map[int, string]
		key      int
		want     bool
		wantSize int
	}{
		{
			"remove existing key",
			func() *Map[int, string] {
				m := NewOrderedTreeMap[int, string]()
				m.Put(1, "a")
				return m
			},
			1, true, 0,
		},
		{
			"remove non-existing key",
			func() *Map[int, string] {
				m := NewOrderedTreeMap[int, string]()
				m.Put(1, "a")
				return m
			},
			2, false, 1,
		},
		{
			"remove from empty map",
			func() *Map[int, string] { return NewOrderedTreeMap[int, string]() },
			1, false, 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.setup()
			got := m.Remove(tt.key)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantSize, m.Size())
		})
	}
}

func TestDelete(t *testing.T) {
	m := NewOrderedTreeMap[int, string]()
	m.Put(1, "a")
	m.Delete(1)
	assert.Equal(t, 0, m.Size())
	assert.False(t, m.ContainsKey(1))

	m.Delete(99)
	assert.Equal(t, 0, m.Size())
}

// --- Clear ---

func TestClear(t *testing.T) {
	m := NewOrderedTreeMap[int, string]()
	m.Put(1, "a")
	m.Put(2, "b")
	m.Put(3, "c")
	assert.Equal(t, 3, m.Size())

	m.Clear()
	assert.Equal(t, 0, m.Size())
	assert.True(t, m.IsEmpty())
	assert.Nil(t, m.RootEntry())
	assert.False(t, m.ContainsKey(1))
}

func TestClearEmptyMap(t *testing.T) {
	m := NewOrderedTreeMap[int, string]()
	m.Clear()
	assert.Equal(t, 0, m.Size())
	assert.True(t, m.IsEmpty())
}

// --- Keys / Values ---

func TestKeys(t *testing.T) {
	tests := []struct {
		name  string
		setup func() *Map[int, string]
		want  []int
	}{
		{
			"empty map",
			func() *Map[int, string] { return NewOrderedTreeMap[int, string]() },
			[]int{},
		},
		{
			"sorted order",
			func() *Map[int, string] {
				m := NewOrderedTreeMap[int, string]()
				m.Put(3, "c")
				m.Put(1, "a")
				m.Put(2, "b")
				return m
			},
			[]int{1, 2, 3},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.setup().Keys())
		})
	}
}

func TestValues(t *testing.T) {
	tests := []struct {
		name  string
		setup func() *Map[int, string]
		want  []string
	}{
		{
			"empty map",
			func() *Map[int, string] { return NewOrderedTreeMap[int, string]() },
			[]string{},
		},
		{
			"values in key order",
			func() *Map[int, string] {
				m := NewOrderedTreeMap[int, string]()
				m.Put(3, "c")
				m.Put(1, "a")
				m.Put(2, "b")
				return m
			},
			[]string{"a", "b", "c"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.setup().Values())
		})
	}
}

// --- ToSysMap ---

func TestToSysMap(t *testing.T) {
	tests := []struct {
		name  string
		setup func() *Map[int, string]
		want  map[int]string
	}{
		{
			"empty map",
			func() *Map[int, string] { return NewOrderedTreeMap[int, string]() },
			map[int]string{},
		},
		{
			"multiple entries",
			func() *Map[int, string] {
				m := NewOrderedTreeMap[int, string]()
				m.Put(1, "a")
				m.Put(2, "b")
				m.Put(3, "c")
				return m
			},
			map[int]string{1: "a", 2: "b", 3: "c"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.setup().ToSysMap())
		})
	}
}

// --- Foreach ---

func TestForeach(t *testing.T) {
	m := NewOrderedTreeMap[int, string]()
	m.Put(1, "a")
	m.Put(2, "b")
	m.Put(3, "c")

	var keys []int
	var vals []string
	m.Foreach(func(k int, v string) bool {
		keys = append(keys, k)
		vals = append(vals, v)
		return true
	})
	assert.Equal(t, []int{1, 2, 3}, keys)
	assert.Equal(t, []string{"a", "b", "c"}, vals)
}

func TestForeachEarlyTermination(t *testing.T) {
	m := NewOrderedTreeMap[int, string]()
	m.Put(1, "a")
	m.Put(2, "b")
	m.Put(3, "c")

	var keys []int
	m.Foreach(func(k int, v string) bool {
		keys = append(keys, k)
		return k < 2
	})
	assert.Equal(t, []int{1, 2}, keys)
}

func TestForeachEmptyMap(t *testing.T) {
	m := NewOrderedTreeMap[int, string]()
	called := false
	m.Foreach(func(k int, v string) bool {
		called = true
		return true
	})
	assert.False(t, called)
}

func TestForeachPanicsOnConcurrentModification(t *testing.T) {
	m := NewOrderedTreeMap[int, string]()
	m.Put(1, "a")
	m.Put(2, "b")
	m.Put(3, "c")

	assert.PanicsWithValue(t, "concurrent map modification", func() {
		m.Foreach(func(k int, v string) bool {
			if k == 2 {
				m.Put(4, "d")
			}
			return true
		})
	})
}

// --- Range ---

func TestRange(t *testing.T) {
	m := NewOrderedTreeMap[int, string]()
	m.Put(1, "a")
	m.Put(2, "b")
	m.Put(3, "c")

	var keys []int
	m.Range(func(k int, v string) bool {
		keys = append(keys, k)
		return true
	})
	assert.Equal(t, []int{1, 2, 3}, keys)
}

func TestRangeEarlyTermination(t *testing.T) {
	m := NewOrderedTreeMap[int, string]()
	m.Put(1, "a")
	m.Put(2, "b")
	m.Put(3, "c")

	var keys []int
	m.Range(func(k int, v string) bool {
		keys = append(keys, k)
		return k < 2
	})
	assert.Equal(t, []int{1, 2}, keys)
}

func TestRangeEmptyMap(t *testing.T) {
	m := NewOrderedTreeMap[int, string]()
	called := false
	m.Range(func(k int, v string) bool {
		called = true
		return true
	})
	assert.False(t, called)
}

func TestRangePanicsOnConcurrentModification(t *testing.T) {
	m := NewOrderedTreeMap[int, string]()
	m.Put(1, "a")
	m.Put(2, "b")

	assert.PanicsWithValue(t, "concurrent map modification", func() {
		m.Range(func(k int, v string) bool {
			if k == 1 {
				m.Remove(2)
			}
			return true
		})
	})
}

// --- Custom Comparator ---

func TestCustomComparator(t *testing.T) {
	reverseCmp := func(a, b int) int {
		return b - a
	}
	m := New[int, string](reverseCmp)
	m.Put(1, "a")
	m.Put(3, "c")
	m.Put(2, "b")

	assert.Equal(t, []int{3, 2, 1}, m.Keys())
	assert.Equal(t, 3, m.FirstEntry().GetKey())
	assert.Equal(t, 1, m.LastEntry().GetKey())
}

// --- Red-Black Tree Integrity ---

func TestRedBlackTreeIntegrity(t *testing.T) {
	m := NewOrderedTreeMap[int, string]()
	requireRedBlackTree(t, m)
	for i := 0; i < 100; i++ {
		m.Put(i, fmt.Sprintf("v%d", i))
		requireRedBlackTree(t, m)
	}
	assert.Equal(t, 100, m.Size())

	for i := 0; i < 100; i++ {
		val, found := m.Get(i)
		assert.True(t, found)
		assert.Equal(t, fmt.Sprintf("v%d", i), val)
	}

	assert.Equal(t, 0, m.FirstEntry().GetKey())
	assert.Equal(t, 99, m.LastEntry().GetKey())
}

func TestInsertDeleteMixed(t *testing.T) {
	m := NewOrderedTreeMap[int, string]()
	for i := 0; i < 50; i++ {
		m.Put(i, fmt.Sprintf("v%d", i))
	}
	for i := 0; i < 50; i += 2 {
		m.Remove(i)
	}
	assert.Equal(t, 25, m.Size())

	for i := 0; i < 50; i++ {
		if i%2 == 0 {
			assert.False(t, m.ContainsKey(i))
		} else {
			assert.True(t, m.ContainsKey(i))
		}
	}

	keys := m.Keys()
	assert.Equal(t, 25, len(keys))
	for i, k := range keys {
		assert.Equal(t, i*2+1, k)
	}
}

func TestDeleteAllElements(t *testing.T) {
	m := NewOrderedTreeMap[int, string]()
	for i := 0; i < 20; i++ {
		m.Put(i, fmt.Sprintf("v%d", i))
	}
	for i := 0; i < 20; i++ {
		assert.True(t, m.Remove(i))
	}
	assert.Equal(t, 0, m.Size())
	assert.True(t, m.IsEmpty())
	assert.Nil(t, m.RootEntry())
}

func TestDeleteInReverseOrder(t *testing.T) {
	m := NewOrderedTreeMap[int, string]()
	for i := 0; i < 20; i++ {
		m.Put(i, fmt.Sprintf("v%d", i))
	}
	for i := 19; i >= 0; i-- {
		assert.True(t, m.Remove(i))
		assert.Equal(t, i, m.Size())
	}
	assert.True(t, m.IsEmpty())
}

func TestDeleteRootRepeatedly(t *testing.T) {
	m := NewOrderedTreeMap[int, string]()
	for i := 0; i < 20; i++ {
		m.Put(i, fmt.Sprintf("v%d", i))
	}
	for m.Size() > 0 {
		requireRedBlackTree(t, m)
		rootKey := m.RootEntry().GetKey()
		assert.True(t, m.Remove(rootKey))
		requireRedBlackTree(t, m)
	}
	assert.True(t, m.IsEmpty())
}

// --- Large Scale with Random Operations ---

func TestLargeScaleRandomOps(t *testing.T) {
	m := NewOrderedTreeMap[int, int]()
	ref := make(map[int]int)
	rng := rand.New(rand.NewSource(42))

	for i := 0; i < 1000; i++ {
		op := rng.Intn(3)
		key := rng.Intn(200)
		switch op {
		case 0:
			val := rng.Int()
			m.Put(key, val)
			ref[key] = val
		case 1:
			m.Remove(key)
			delete(ref, key)
		case 2:
			v1, ok1 := m.Get(key)
			v2, ok2 := ref[key]
			assert.Equal(t, ok2, ok1)
			if ok1 {
				assert.Equal(t, v2, v1)
			}
		}
		requireMapMatches(t, m, ref)
	}
	assert.Equal(t, len(ref), m.Size())

	treeKeys := m.Keys()
	for i := 1; i < len(treeKeys); i++ {
		assert.True(t, treeKeys[i-1] < treeKeys[i], "keys should be sorted")
	}
}

// --- Floor / Ceiling / Higher with Single Element ---

func TestFloorCeilingHigherSingleElement(t *testing.T) {
	m := NewOrderedTreeMap[int, string]()
	m.Put(10, "ten")

	fk, fok := m.FloorKey(10)
	assert.True(t, fok)
	assert.Equal(t, 10, fk)

	fk, fok = m.FloorKey(15)
	assert.True(t, fok)
	assert.Equal(t, 10, fk)

	fk, fok = m.FloorKey(5)
	assert.False(t, fok)

	ck, cok := m.CeilingKey(10)
	assert.True(t, cok)
	assert.Equal(t, 10, ck)

	ck, cok = m.CeilingKey(5)
	assert.True(t, cok)
	assert.Equal(t, 10, ck)

	ck, cok = m.CeilingKey(15)
	assert.False(t, cok)

	hk, hok := m.HigherKey(10)
	assert.False(t, hok)

	hk, hok = m.HigherKey(5)
	assert.True(t, hok)
	assert.Equal(t, 10, hk)
}

// --- Put after Remove (reuse) ---

func TestPutAfterRemove(t *testing.T) {
	m := NewOrderedTreeMap[int, string]()
	m.Put(1, "a")
	m.Put(2, "b")
	m.Put(3, "c")
	m.Remove(2)

	m.Put(2, "B")
	assert.Equal(t, 3, m.Size())
	val, found := m.Get(2)
	assert.True(t, found)
	assert.Equal(t, "B", val)

	assert.Equal(t, []int{1, 2, 3}, m.Keys())
}

// --- String keys ---

func TestStringKeys(t *testing.T) {
	m := NewOrderedTreeMap[string, int]()
	m.Put("banana", 2)
	m.Put("apple", 1)
	m.Put("cherry", 3)

	assert.Equal(t, []string{"apple", "banana", "cherry"}, m.Keys())
	assert.Equal(t, 1, m.GetOrDefault("apple", 0))
	assert.Equal(t, 2, m.GetOrDefault("banana", 0))
	assert.Equal(t, 3, m.GetOrDefault("cherry", 0))
	assert.Equal(t, 0, m.GetOrDefault("durian", 0))
}

// --- Negative / boundary keys ---

func TestNegativeAndBoundaryKeys(t *testing.T) {
	m := NewOrderedTreeMap[int, string]()
	m.Put(math.MinInt, "min")
	m.Put(0, "zero")
	m.Put(math.MaxInt, "max")
	m.Put(-1, "neg1")
	m.Put(1, "pos1")

	assert.Equal(t, 5, m.Size())
	assert.Equal(t, math.MinInt, m.FirstEntry().GetKey())
	assert.Equal(t, math.MaxInt, m.LastEntry().GetKey())

	val, found := m.Get(math.MinInt)
	assert.True(t, found)
	assert.Equal(t, "min", val)

	val, found = m.Get(math.MaxInt)
	assert.True(t, found)
	assert.Equal(t, "max", val)
}

// --- Overwrite value with zero value ---

func TestOverwriteWithZeroValue(t *testing.T) {
	m := NewOrderedTreeMap[int, int]()
	m.Put(1, 100)
	old := m.Put(1, 0)
	assert.Equal(t, 100, old)

	val, found := m.Get(1)
	assert.True(t, found)
	assert.Equal(t, 0, val)
}

// --- Foreach / Range on single element ---

func TestForeachRangeSingleElement(t *testing.T) {
	m := NewOrderedTreeMap[int, string]()
	m.Put(1, "only")

	var keys []int
	m.Foreach(func(k int, v string) bool {
		keys = append(keys, k)
		return true
	})
	assert.Equal(t, []int{1}, keys)

	keys = nil
	m.Range(func(k int, v string) bool {
		keys = append(keys, k)
		return true
	})
	assert.Equal(t, []int{1}, keys)
}

// --- Clear then reuse ---

func TestClearThenReuse(t *testing.T) {
	m := NewOrderedTreeMap[int, string]()
	m.Put(1, "a")
	m.Put(2, "b")
	m.Clear()

	assert.Equal(t, 0, m.Size())
	assert.True(t, m.IsEmpty())

	m.Put(10, "x")
	assert.Equal(t, 1, m.Size())
	val, found := m.Get(10)
	assert.True(t, found)
	assert.Equal(t, "x", val)

	assert.Equal(t, []int{10}, m.Keys())
}

// --- Ordered map with cmp.Ordered types ---

func TestNewOrderedTreeMapWithFloat(t *testing.T) {
	m := NewOrderedTreeMap[float64, string]()
	m.Put(3.14, "pi")
	m.Put(1.41, "sqrt2")
	m.Put(2.72, "e")

	assert.Equal(t, []float64{1.41, 2.72, 3.14}, m.Keys())
}

// --- NewTreeMapOf with custom string-keyed map ---

func TestNewTreeMapOfStrings(t *testing.T) {
	input := map[string]int{"b": 2, "a": 1, "c": 3}
	m := NewTreeMapOf(input)
	assert.Equal(t, 3, m.Size())
	assert.Equal(t, []string{"a", "b", "c"}, m.Keys())
	assert.Equal(t, []int{1, 2, 3}, m.Values())
}

// --- Verify sorted order after complex operations ---

func TestSortedOrderAfterComplexOps(t *testing.T) {
	m := NewOrderedTreeMap[int, string]()
	for i := 0; i < 100; i++ {
		m.Put(i, fmt.Sprintf("v%d", i))
	}
	for i := 0; i < 50; i++ {
		m.Remove(i * 2)
	}
	for i := 100; i < 150; i++ {
		m.Put(i, fmt.Sprintf("v%d", i))
	}

	keys := m.Keys()
	for i := 1; i < len(keys); i++ {
		assert.True(t, keys[i-1] < keys[i], "keys should be strictly sorted")
	}
}

// --- Load alias ---

func TestLoadAlias(t *testing.T) {
	m := NewOrderedTreeMap[int, string]()
	m.Put(1, "a")

	v1, ok1 := m.Load(1)
	v2, ok2 := m.Get(1)
	assert.Equal(t, v2, v1)
	assert.Equal(t, ok2, ok1)

	v1, ok1 = m.Load(999)
	v2, ok2 = m.Get(999)
	assert.Equal(t, v2, v1)
	assert.Equal(t, ok2, ok1)
}

// --- Verify comparator is used ---

func TestComparatorUsedForStringOrdering(t *testing.T) {
	lengthCmp := func(a, b string) int {
		if len(a) != len(b) {
			return len(a) - len(b)
		}
		return cmp.Compare(a, b)
	}
	m := New[string, int](lengthCmp)
	m.Put("bb", 2)
	m.Put("aaa", 3)
	m.Put("c", 1)

	assert.Equal(t, []string{"c", "bb", "aaa"}, m.Keys())
}
