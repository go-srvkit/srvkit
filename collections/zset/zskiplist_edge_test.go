// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package zset

import (
	"cmp"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestZSkipList_NodeTraversal(t *testing.T) {
	sl := newSkipListWithZeroElement()
	assert.Equal(t, 2, sl.Height())
	first, last := sl.HeadNode(), sl.TailNode()
	require.NotNil(t, first)
	require.NotNil(t, last)
	assert.Nil(t, first.Before())
	assert.Same(t, last, first.Next())
	assert.Same(t, first, last.Before())
	assert.Nil(t, last.Next())
	assert.Nil(t, sl.GetElementByRank(-1))
	assert.Nil(t, sl.GetElementByRank(3))
	assert.NotNil(t, sl.Delete(10, 1))
	assert.Equal(t, 1, sl.Height())
	assert.Same(t, first, sl.HeadNode())
	assert.Same(t, first, sl.TailNode())
	assert.Nil(t, first.Next())
	assert.NotNil(t, sl.Delete(0, 0))
	assert.Nil(t, sl.HeadNode())
	assert.Nil(t, sl.TailNode())
	assert.Zero(t, sl.Len())
	assert.Equal(t, 1, sl.Height())
}

func TestZSkipList_Range_StopEarly(t *testing.T) {
	sl := newSkipListWithZeroElement()
	var visited []int
	sl.Range(func(score int64, ele int) bool {
		visited = append(visited, ele)
		assert.Equal(t, int64(0), score)
		return false
	})
	assert.Equal(t, []int{0}, visited)
	assert.Equal(t, 2, sl.Len())
	empty := NewZSkipList(cmp.Compare[int])
	empty.Range(func(int64, int) bool {
		t.Fatal("empty list must not invoke callback")
		return true
	})
	assert.Empty(t, empty.String())
}

func TestZSkipList_ScoreRange_Boundaries(t *testing.T) {
	tests := []struct {
		name        string
		min, max    int64
		intersects  bool
		first, last int
	}{
		{"all", math.MinInt64, math.MaxInt64, true, 1, 4},
		{"equal scores", 0, 0, true, 2, 3},
		{"first", -10, -10, true, 1, 1},
		{"last", 10, 10, true, 4, 4},
		{"below range", -30, -20, false, 0, 0},
		{"above range", 20, 30, false, 0, 0},
		{"gap", 1, 9, true, 0, 0},
		{"reversed", 10, -10, false, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sl := NewZSkipList(cmp.Compare[int])
			for i, score := range []int64{-10, 0, 0, 10} {
				sl.Insert(score, i+1)
			}
			// IsInRange is a bounding-box check; a gap is rejected by First/LastInRange.
			assert.Equal(t, tt.intersects, sl.IsInRange(tt.min, tt.max))
			first, last := sl.FirstInRange(tt.min, tt.max), sl.LastInRange(tt.min, tt.max)
			if tt.first == 0 {
				assert.Nil(t, first)
				assert.Nil(t, last)
			} else {
				require.NotNil(t, first)
				require.NotNil(t, last)
				assert.Equal(t, tt.first, first.Ele)
				assert.Equal(t, tt.last, last.Ele)
			}
		})
	}
	empty := NewZSkipList(cmp.Compare[int])
	assert.False(t, empty.IsInRange(-1, 1))
	assert.Nil(t, empty.FirstInRange(-1, 1))
	assert.Nil(t, empty.LastInRange(-1, 1))
}

func TestZSkipList_UpdateScore_Boundaries(t *testing.T) {
	tests := []struct {
		name      string
		ele       int
		old, next int64
		found     bool
		reuse     bool
		order     []int
	}{
		{"unchanged", 2, 20, 20, true, true, []int{1, 2, 3}},
		{"in place", 2, 20, 25, true, true, []int{1, 2, 3}},
		{"move first", 2, 20, math.MinInt64, true, false, []int{2, 1, 3}},
		{"move last", 2, 20, math.MaxInt64, true, false, []int{1, 3, 2}},
		{"tie predecessor", 2, 20, 10, true, false, []int{1, 2, 3}},
		{"tie successor", 2, 20, 30, true, false, []int{1, 2, 3}},
		{"missing element", 4, 20, 25, false, false, []int{1, 2, 3}},
		{"missing score", 2, 15, 25, false, false, []int{1, 2, 3}},
		{"past tail", 4, 40, 25, false, false, []int{1, 2, 3}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sl := NewZSkipList(cmp.Compare[int])
			sl.Insert(10, 1)
			original := sl.Insert(20, 2)
			sl.Insert(30, 3)
			node := sl.UpdateScore(tt.ele, tt.old, tt.next)
			if tt.found {
				require.NotNil(t, node)
				assert.Equal(t, tt.ele, node.Ele)
				assert.Equal(t, tt.next, node.Score)
				if tt.reuse {
					assert.Same(t, original, node)
				}
			} else {
				assert.Nil(t, node)
			}
			var order []int
			sl.Range(func(score int64, ele int) bool {
				order = append(order, ele)
				return true
			})
			assert.Equal(t, tt.order, order)
			assert.Equal(t, 3, sl.Len())
			for i, ele := range tt.order {
				n := sl.GetElementByRank(i + 1)
				require.NotNil(t, n)
				assert.Equal(t, ele, n.Ele)
				assert.Equal(t, i+1, sl.GetRank(n.Score, ele))
			}
		})
	}
	assert.Nil(t, NewZSkipList(cmp.Compare[int]).UpdateScore(1, 0, 1))
}

func TestZSkipList_DeleteRange_NilCallback(t *testing.T) {
	for _, byRank := range []bool{false, true} {
		sl := newSkipListWithZeroElement()
		if byRank {
			assert.Equal(t, 2, sl.DeleteRangeByRank(1, 2, nil))
			assert.Zero(t, sl.DeleteRangeByRank(1, 2, nil))
		} else {
			assert.Equal(t, 2, sl.DeleteRangeByScore(0, 10, nil))
			assert.Zero(t, sl.DeleteRangeByScore(0, 10, nil))
		}
		assert.Zero(t, sl.Len())
		assert.Equal(t, 1, sl.Height())
		assert.Nil(t, sl.HeadNode())
		assert.Nil(t, sl.TailNode())
	}
}
