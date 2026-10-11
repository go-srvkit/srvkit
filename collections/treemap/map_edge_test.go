package treemap

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNilComparatorAndNodeHelpers(t *testing.T) {
	require.Nil(t, New[int, int](nil))
	var node *Entry[int, int]
	require.Nil(t, parentOf(node))
	require.Nil(t, leftOf(node))
	require.Nil(t, rightOf(node))
	require.Nil(t, successor(node))
	require.Nil(t, predecessor(node))
	require.Equal(t, BLACK, colorOf(node))
	require.NotPanics(t, func() { setColor(node, RED) })
	m := NewOrderedTreeMap[int, int]()
	m.Put(1, 1)
	m.rotateLeft(nil)
	m.rotateRight(nil)
	requireRedBlackTree(t, m)
	require.Equal(t, []int{1}, m.Keys())
}

func requireEntryAt(t *testing.T, entry *Entry[int, int], keys []int, index int) {
	t.Helper()
	if index < 0 || index >= len(keys) {
		require.Nil(t, entry)
		return
	}
	require.NotNil(t, entry)
	require.Equal(t, keys[index], entry.GetKey())
	require.Equal(t, keys[index]*10, entry.GetValue())
}

func TestNavigationAgainstSortedKeys(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		for _, input := range [][]int{{}, {16}, {16, 8, 24, 4, 12, 20, 28, 2, 6, 10, 14, 18, 22, 26, 30}} {
			t.Run(fmt.Sprintf("reverse=%t/size=%d", reverse, len(input)), func(t *testing.T) {
				comparison := cmp.Compare[int]
				if reverse {
					comparison = func(a, b int) int { return cmp.Compare(b, a) }
				}
				m := New[int, int](comparison)
				for _, key := range input {
					m.Put(key, key*10)
				}
				requireRedBlackTree(t, m)
				keys := slices.Clone(input)
				slices.SortFunc(keys, comparison)
				for query := 0; query <= 32; query++ {
					lower, floor, ceiling, higher := -1, -1, -1, -1
					for i, key := range keys {
						c := comparison(key, query)
						if c < 0 {
							lower = i
						}
						if c <= 0 {
							floor = i
						}
						if c >= 0 && ceiling == -1 {
							ceiling = i
						}
						if c > 0 && higher == -1 {
							higher = i
						}
					}
					requireEntryAt(t, m.getLowerEntry(query), keys, lower)
					requireEntryAt(t, m.FloorEntry(query), keys, floor)
					requireEntryAt(t, m.CeilingEntry(query), keys, ceiling)
					requireEntryAt(t, m.HigherEntry(query), keys, higher)
					for _, bound := range []struct {
						index int
						get   func(int) (int, bool)
					}{{floor, m.FloorKey}, {ceiling, m.CeilingKey}, {higher, m.HigherKey}} {
						key, found := bound.get(query)
						require.Equal(t, bound.index >= 0, found)
						if found {
							require.Equal(t, keys[bound.index], key)
						} else {
							require.Zero(t, key)
						}
					}
				}
				entry := m.FirstEntry()
				for i := range keys {
					requireEntryAt(t, entry, keys, i)
					entry = successor(entry)
				}
				require.Nil(t, entry)
				entry = m.LastEntry()
				for i := len(keys) - 1; i >= 0; i-- {
					requireEntryAt(t, entry, keys, i)
					entry = predecessor(entry)
				}
				require.Nil(t, entry)
			})
		}
	}
}

func TestComparatorEquivalentKeys(t *testing.T) {
	m := New[int, string](func(a, b int) int { return cmp.Compare(a/10, b/10) })
	m.Put(11, "old")
	previous, existed := m.Swap(19, "new")
	require.Equal(t, "old", previous)
	require.True(t, existed)
	require.Equal(t, "new", m.PutIfAbsent(12, "ignored"))
	require.Equal(t, []int{11}, m.Keys())
	require.Equal(t, 1, m.Size())
	value, found := m.Get(15)
	require.True(t, found)
	require.Equal(t, "new", value)
	requireRedBlackTree(t, m)
	require.True(t, m.Remove(18))
	requireRedBlackTree(t, m)
	require.True(t, m.IsEmpty())
}

func TestFloatSpecialKeys(t *testing.T) {
	m := NewOrderedTreeMap[float64, int]()
	for i, key := range []float64{1, math.NaN(), math.Inf(1), 0, -1, math.Inf(-1)} {
		m.Put(key, i)
		requireRedBlackTree(t, m)
	}
	want := []float64{math.NaN(), math.Inf(-1), -1, 0, 1, math.Inf(1)}
	require.True(t, slices.EqualFunc(want, m.Keys(), func(a, b float64) bool { return cmp.Compare(a, b) == 0 }))
	previous, existed := m.Swap(math.NaN(), 99)
	require.True(t, existed)
	require.Equal(t, 1, previous)
	value, found := m.Get(math.NaN())
	require.True(t, found)
	require.Equal(t, 99, value)
	require.Equal(t, 3, m.Put(math.Copysign(0, -1), 100))
	require.Equal(t, 6, m.Size())
	key, found := m.FirstKey()
	require.True(t, found)
	require.True(t, math.IsNaN(key))
	requireRedBlackTree(t, m)
	for _, key := range want {
		require.True(t, m.Remove(key))
		requireRedBlackTree(t, m)
	}
	require.True(t, m.IsEmpty())
}

func TestTraversalMutationRules(t *testing.T) {
	for _, method := range []string{"Foreach", "Range"} {
		for _, operation := range []string{"replace", "remove missing", "insert", "remove current", "clear"} {
			t.Run(method+"/"+operation, func(t *testing.T) {
				m := NewTreeMapOf(map[int]int{1: 10, 2: 20, 3: 30})
				visit := func(key, value int) bool {
					if key == 1 {
						switch operation {
						case "replace":
							m.Put(key, value+1)
						case "remove missing":
							require.False(t, m.Remove(99))
						case "insert":
							m.Put(4, 40)
						case "remove current":
							m.Remove(key)
						case "clear":
							m.Clear()
						}
					}
					return false // A structural modification is checked even when stopping.
				}
				run := func() {
					if method == "Foreach" {
						m.Foreach(visit)
					} else {
						m.Range(visit)
					}
				}
				if operation == "replace" || operation == "remove missing" {
					require.NotPanics(t, run)
				} else {
					require.PanicsWithValue(t, "concurrent map modification", run)
				}
				requireRedBlackTree(t, m)
			})
		}
	}
}
