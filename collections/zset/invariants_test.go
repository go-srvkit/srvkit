// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package zset

import (
	"cmp"
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSortedSet_Model(t *testing.T) {
	for _, seed := range []uint64{1, 7, 42} {
		t.Run(fmt.Sprint(seed), func(t *testing.T) {
			rng := rand.New(rand.NewPCG(seed, seed+1))
			s := NewSortedSet(cmp.Compare[int])
			model := make(map[int]int64)
			for step := 0; step < 1000; step++ {
				ele, score := rng.IntN(64), int64(rng.IntN(21)-10)
				switch rng.IntN(10) {
				case 0, 1, 2, 3, 4:
					require.True(t, s.Add(ele, score))
					model[ele] = score
				case 5:
					_, exists := model[ele]
					require.Equal(t, exists, s.Remove(ele))
					delete(model, ele)
				case 6:
					if old, exists := model[ele]; exists {
						node := s.zsl.UpdateScore(ele, old, score)
						require.NotNil(t, node)
						require.Equal(t, ele, node.Ele)
						require.Equal(t, score, node.Score)
						// UpdateScore is a low-level API; its caller owns the dictionary.
						s.dict[ele], model[ele] = score, score
					}
				case 7:
					ordered := sortedModelElements(model)
					if len(ordered) > 0 {
						start := rng.IntN(len(ordered))
						end := min(start+2, len(ordered)-1)
						require.Equal(t, end-start+1, s.RemoveRangeByRank(start, end))
						for _, key := range ordered[start : end+1] {
							delete(model, key)
						}
					}
				case 8:
					removed := 0
					for key, value := range model {
						if value >= score && value <= score+2 {
							delete(model, key)
							removed++
						}
					}
					require.Equal(t, removed, s.RemoveRangeByScore(score, score+2))
				}
				assertSortedSetModel(t, s, model, step)
			}
		})
	}
}

func sortedModelElements(model map[int]int64) []int {
	var ordered []int
	for ele := range model {
		ordered = append(ordered, ele)
	}
	slices.SortFunc(ordered, func(a, b int) int {
		if c := cmp.Compare(model[a], model[b]); c != 0 {
			return c
		}
		return cmp.Compare(a, b)
	})
	return ordered
}

func assertSortedSetModel(t *testing.T, s *SortedSet[int], model map[int]int64, step int) {
	t.Helper()
	ordered := sortedModelElements(model)
	require.Equal(t, model, s.dict, "step %d dictionary", step)
	require.Equal(t, len(ordered), s.Len(), "step %d length", step)
	require.Equal(t, ordered, s.GetRange(0, -1, false), "step %d forward range", step)
	reversed := slices.Clone(ordered)
	slices.Reverse(reversed)
	require.Equal(t, reversed, s.GetRange(0, -1, true), "step %d reverse range", step)

	sl := s.zsl
	positions := map[*ZSkipListNode[int]]int{sl.head: 0}
	var previous *ZSkipListNode[int]
	node := sl.HeadNode()
	for i, ele := range ordered {
		require.NotNil(t, node, "step %d node %d", step, i)
		require.Equal(t, ele, node.Ele)
		require.Equal(t, model[ele], node.Score)
		require.True(t, previous == node.Before(), "step %d backward link", step)
		require.Same(t, node, sl.GetElementByRank(i+1))
		require.Equal(t, i, s.GetRank(ele, false))
		require.Equal(t, len(ordered)-i-1, s.GetRank(ele, true))
		require.Equal(t, model[ele], s.GetScore(ele))
		positions[node] = i + 1
		previous, node = node, node.Next()
	}
	require.Nil(t, node, "step %d unexpected trailing node", step)
	require.True(t, previous == sl.TailNode(), "step %d tail", step)
	require.GreaterOrEqual(t, sl.Height(), 1)
	require.LessOrEqual(t, sl.Height(), ZSKIPLIST_MAXLEVEL)
	if len(ordered) > 0 {
		require.NotNil(t, sl.head.level[sl.Height()-1].forward)
	} else {
		require.Equal(t, 1, sl.Height())
	}
	for current, rank := range positions {
		for level := 0; level < min(len(current.level), sl.Height()); level++ {
			link := current.level[level]
			endRank := len(ordered)
			if link.forward != nil {
				var found bool
				endRank, found = positions[link.forward]
				require.True(t, found, "step %d forward link to missing node", step)
				require.Greater(t, endRank, rank)
			}
			require.Equal(t, endRank-rank, link.span, "step %d rank %d level %d span", step, rank, level)
		}
	}

	var inRange []int
	for _, ele := range ordered {
		if model[ele] >= -3 && model[ele] <= 3 {
			inRange = append(inRange, ele)
		}
	}
	require.Equal(t, len(inRange), s.Count(-3, 3))
	require.Equal(t, inRange, s.GetRangeByScore(-3, 3, false))
	slices.Reverse(inRange)
	require.Equal(t, inRange, s.GetRangeByScore(-3, 3, true))
}
