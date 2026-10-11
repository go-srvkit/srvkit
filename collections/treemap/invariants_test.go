package treemap

import (
	"cmp"
	"fmt"
	"math/rand"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

// validateRedBlackTree checks structure independently of the tree's accessors.
// Bounds come from all ancestors, not just the immediate parent.
func validateRedBlackTree[K, V comparable](m *Map[K, V]) error {
	if m.root != nil && m.root.color != BLACK {
		return fmt.Errorf("root must be black")
	}
	seen := make(map[*Entry[K, V]]bool)
	var walk func(node, parent, lower, upper *Entry[K, V]) (int, error)
	walk = func(node, parent, lower, upper *Entry[K, V]) (int, error) {
		if node == nil {
			return 1, nil // Nil leaves are black.
		}
		if seen[node] {
			return 0, fmt.Errorf("key %v visited more than once", node.key)
		}
		seen[node] = true
		if node.parent != parent {
			return 0, fmt.Errorf("key %v has incorrect parent", node.key)
		}
		if node.color != RED && node.color != BLACK {
			return 0, fmt.Errorf("key %v has invalid color %d", node.key, node.color)
		}
		if lower != nil && m.comparator(node.key, lower.key) <= 0 {
			return 0, fmt.Errorf("key %v violates lower bound %v", node.key, lower.key)
		}
		if upper != nil && m.comparator(node.key, upper.key) >= 0 {
			return 0, fmt.Errorf("key %v violates upper bound %v", node.key, upper.key)
		}
		if node.color == RED {
			if node.left != nil && node.left.color == RED || node.right != nil && node.right.color == RED {
				return 0, fmt.Errorf("red key %v has a red child", node.key)
			}
		}
		leftHeight, err := walk(node.left, node, lower, node)
		if err != nil {
			return 0, err
		}
		rightHeight, err := walk(node.right, node, node, upper)
		if err != nil {
			return 0, err
		}
		if leftHeight != rightHeight {
			return 0, fmt.Errorf("key %v has unequal black height: left=%d right=%d", node.key, leftHeight, rightHeight)
		}
		if node.color == BLACK {
			leftHeight++
		}
		return leftHeight, nil
	}
	if _, err := walk(m.root, nil, nil, nil); err != nil {
		return err
	}
	if len(seen) != m.size {
		return fmt.Errorf("size=%d, reachable nodes=%d", m.size, len(seen))
	}
	return nil
}

func requireRedBlackTree[K, V comparable](t *testing.T, m *Map[K, V]) {
	t.Helper()
	require.NoError(t, validateRedBlackTree(m))
}

func TestInvariantValidatorRejectsInvalidTrees(t *testing.T) {
	for _, tt := range []struct {
		name   string
		mutate func(*Map[int, int])
		want   string
	}{
		{"red root", func(m *Map[int, int]) { m.root.color = RED }, "root must be black"},
		{"root parent", func(m *Map[int, int]) { m.root.parent = m.root.left }, "incorrect parent"},
		{"child parent", func(m *Map[int, int]) { m.root.left.parent = nil }, "incorrect parent"},
		{"red child of red parent", func(m *Map[int, int]) { m.root.left.color = RED }, "red child"},
		{"unequal black height", func(m *Map[int, int]) { m.root.left.left.color = BLACK }, "unequal black height"},
		{"invalid color", func(m *Map[int, int]) { m.root.left.left.color = Color(2) }, "invalid color"},
		{"ancestor upper bound", func(m *Map[int, int]) { m.root.left.right.key = 5 }, "upper bound"},
		{"ancestor lower bound", func(m *Map[int, int]) { m.root.right.left.key = 3 }, "lower bound"},
		{"duplicate key", func(m *Map[int, int]) { m.root.left.key = m.root.key }, "upper bound"},
		{"cycle", func(m *Map[int, int]) { m.root.left.left.left = m.root.left }, "visited more than once"},
		{"shared child", func(m *Map[int, int]) { m.root.right.left = m.root.left.left }, "visited more than once"},
		{"wrong size", func(m *Map[int, int]) { m.size++ }, "size="},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := NewOrderedTreeMap[int, int]()
			for _, key := range []int{4, 2, 6, 1, 3, 5, 7} {
				m.Put(key, key)
			}
			requireRedBlackTree(t, m)
			tt.mutate(m)
			require.ErrorContains(t, validateRedBlackTree(m), tt.want)
		})
	}
}

func TestInsertionAndDeletionInvariants(t *testing.T) {
	for _, tt := range []struct {
		name string
		keys []int
	}{
		{"left left", []int{3, 2, 1}},
		{"left right", []int{3, 1, 2}},
		{"right right", []int{1, 2, 3}},
		{"right left", []int{1, 3, 2}},
		{"recolor and delete", []int{11, 2, 14, 1, 7, 15, 5, 8, 4}},
		{"successor with right child", []int{20, 10, 30, 25, 40, 27}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := NewOrderedTreeMap[int, int]()
			requireRedBlackTree(t, m)
			for _, key := range tt.keys {
				m.Put(key, key)
				requireRedBlackTree(t, m)
			}
			for i, key := range tt.keys {
				require.True(t, m.Remove(key), "key=%d", key)
				requireRedBlackTree(t, m)
				require.Equal(t, len(tt.keys)-i-1, m.Size())
				require.False(t, m.ContainsKey(key))
			}
		})
	}
}

func requireMapMatches(t *testing.T, m *Map[int, int], ref map[int]int) {
	t.Helper()
	requireRedBlackTree(t, m)
	require.Equal(t, len(ref), m.Size())
	require.Equal(t, len(ref) == 0, m.IsEmpty())
	require.Equal(t, ref, m.ToSysMap())
	keys := make([]int, 0, len(ref))
	for key, value := range ref {
		keys = append(keys, key)
		got, found := m.Get(key)
		require.True(t, found, "key=%d", key)
		require.Equal(t, value, got, "key=%d", key)
	}
	slices.SortFunc(keys, m.comparator)
	require.Equal(t, keys, m.Keys())
}

func TestMixedOperationsAgainstReference(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		for _, seed := range []int64{1, 7, 42, 2026} {
			t.Run(fmt.Sprintf("reverse=%t/seed=%d", reverse, seed), func(t *testing.T) {
				comparison := cmp.Compare[int]
				if reverse {
					comparison = func(a, b int) int { return cmp.Compare(b, a) }
				}
				m := New[int, int](comparison)
				ref := make(map[int]int)
				rng := rand.New(rand.NewSource(seed))
				for step := 0; step < 1000; step++ {
					key, value := rng.Intn(128)-64, rng.Intn(10)
					old, exists := ref[key]
					switch rng.Intn(7) {
					case 0:
						require.Equal(t, old, m.Put(key, value))
						ref[key] = value
					case 1:
						require.Equal(t, exists, m.Remove(key))
						delete(ref, key)
					case 2:
						previous, loaded := m.Swap(key, value)
						require.Equal(t, old, previous)
						require.Equal(t, exists, loaded)
						ref[key] = value
					case 3:
						require.Equal(t, old, m.PutIfAbsent(key, value))
						if !exists {
							ref[key] = value
						}
					case 4:
						got, found := m.Get(key)
						require.Equal(t, old, got)
						require.Equal(t, exists, found)
					case 5:
						m.Store(key, value)
						ref[key] = value
					case 6:
						if step%97 == 0 {
							m.Clear()
							clear(ref)
						} else {
							m.Delete(key)
							delete(ref, key)
						}
					}
					requireMapMatches(t, m, ref)
				}
				for m.root != nil {
					key := m.root.key
					require.True(t, m.Remove(key))
					delete(ref, key)
					requireMapMatches(t, m, ref)
				}
			})
		}
	}
}
