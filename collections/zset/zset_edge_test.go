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

func newTestSortedSet() *SortedSet[int] {
	s := NewSortedSet(cmp.Compare[int])
	for i, score := range []int64{-10, 0, 0, 10, 20} {
		s.Add(i+1, score)
	}
	return s
}

func TestSortedSet_GetRange_Boundaries(t *testing.T) {
	tests := []struct {
		name         string
		start, end   int
		forward, rev []int
	}{
		{"all", 0, -1, []int{1, 2, 3, 4, 5}, []int{5, 4, 3, 2, 1}},
		{"negative indices", -2, -1, []int{4, 5}, []int{2, 1}},
		{"clamp start", math.MinInt, 1, []int{1, 2}, []int{5, 4}},
		{"clamp end", 3, math.MaxInt, []int{4, 5}, []int{2, 1}},
		{"start after end", 3, 1, nil, nil},
		{"start beyond length", 5, 10, nil, nil},
		{"end before first", 0, -6, nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestSortedSet()
			assert.Equal(t, tt.forward, s.GetRange(tt.start, tt.end, false))
			assert.Equal(t, tt.rev, s.GetRange(tt.start, tt.end, true))
		})
	}
	for _, reverse := range []bool{false, true} {
		assert.Nil(t, NewSortedSet(cmp.Compare[int]).GetRange(0, -1, reverse))
	}
}

func TestSortedSet_RemoveRangeByRank_Boundaries(t *testing.T) {
	tests := []struct {
		name       string
		start, end int
		remaining  []int
	}{
		{"first", 0, 0, []int{2, 3, 4, 5}},
		{"last", -1, -1, []int{1, 2, 3, 4}},
		{"negative indices", -3, -2, []int{1, 2, 5}},
		{"clamp start", math.MinInt, 1, []int{3, 4, 5}},
		{"clamp end", 3, math.MaxInt, []int{1, 2, 3}},
		{"all", 0, -1, nil},
		{"start after end", 3, 1, []int{1, 2, 3, 4, 5}},
		{"start beyond length", 5, 10, []int{1, 2, 3, 4, 5}},
		{"end before first", 0, -6, []int{1, 2, 3, 4, 5}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestSortedSet()
			assert.Equal(t, 5-len(tt.remaining), s.RemoveRangeByRank(tt.start, tt.end))
			assertSortedSetRemaining(t, s, tt.remaining)
		})
	}
	assert.Zero(t, NewSortedSet(cmp.Compare[int]).RemoveRangeByRank(0, -1))
}

func TestSortedSet_RemoveRangeByScore_Boundaries(t *testing.T) {
	tests := []struct {
		name      string
		min, max  int64
		remaining []int
	}{
		{"equal scores", 0, 0, []int{1, 4, 5}},
		{"inclusive endpoints", -10, 10, []int{5}},
		{"gap", 1, 9, []int{1, 2, 3, 4, 5}},
		{"below range", -100, -20, []int{1, 2, 3, 4, 5}},
		{"above range", 30, 100, []int{1, 2, 3, 4, 5}},
		{"reversed", 20, -10, []int{1, 2, 3, 4, 5}},
		{"all", math.MinInt64, math.MaxInt64, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestSortedSet()
			assert.Equal(t, 5-len(tt.remaining), s.RemoveRangeByScore(tt.min, tt.max))
			assertSortedSetRemaining(t, s, tt.remaining)
			assert.Zero(t, s.RemoveRangeByScore(tt.min, tt.max))
		})
	}
	assert.Zero(t, NewSortedSet(cmp.Compare[int]).RemoveRangeByScore(-1, 1))
}

func assertSortedSetRemaining(t *testing.T, s *SortedSet[int], remaining []int) {
	t.Helper()
	require.Equal(t, remaining, s.GetRange(0, -1, false))
	assert.Equal(t, len(remaining), s.Len())
	want := make(map[int]int64, len(remaining))
	scores := []int64{-10, 0, 0, 10, 20}
	for rank, ele := range remaining {
		want[ele] = scores[ele-1]
		assert.Equal(t, rank, s.GetRank(ele, false))
		assert.Equal(t, len(remaining)-rank-1, s.GetRank(ele, true))
	}
	assert.Equal(t, want, s.dict)
}

func TestSortedSet_GetRangeByScore_Boundaries(t *testing.T) {
	s := newTestSortedSet()
	assert.Equal(t, []int{2, 3}, s.GetRangeByScore(0, 0, false))
	assert.Equal(t, []int{3, 2}, s.GetRangeByScore(0, 0, true))
	for _, reverse := range []bool{false, true} {
		assert.Nil(t, s.GetRangeByScore(1, 9, reverse))
		assert.Nil(t, s.GetRangeByScore(10, -10, reverse))
		assert.Nil(t, NewSortedSet(cmp.Compare[int]).GetRangeByScore(-1, 1, reverse))
	}
}

func TestSortedSet_Add_UnchangedScore(t *testing.T) {
	s := NewSortedSet(cmp.Compare[int])
	assert.True(t, s.Add(0, 0))
	assert.True(t, s.Add(0, 0))
	assert.Equal(t, 1, s.Len())
	assert.Equal(t, []int{0}, s.GetRange(0, -1, false))
	assert.Zero(t, s.GetScore(0))
	assert.Zero(t, s.GetScore(1))
}
