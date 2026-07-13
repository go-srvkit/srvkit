// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package zset

import (
	"cmp"
	"fmt"
	"math/rand"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/srvkit.v1/collections/types"
)

func uniqueScoreGen() func() int64 {
	var dict = map[int64]bool{}
	return func() int64 {
		for {
			var score = rand.Int63() % 1000000
			if _, found := dict[score]; !found {
				return score
			}
		}
	}
}

func Test_zslRandLevel(t *testing.T) {
	for i := 0; i < 100; i++ {
		var level = zslRandLevel()
		assert.GreaterOrEqual(t, level, 1)
		assert.LessOrEqual(t, level, ZSKIPLIST_MAXLEVEL)
	}
}

func testSkipList(t *testing.T, sl *ZSkipList[string]) {
	var pairs = make([]types.Pair[int64, string], 0, sl.Len())
	sl.Range(func(score int64, ele string) bool {
		pairs = append(pairs, types.MakePair(score, ele))
		return true
	})
	assert.Equal(t, len(pairs), sl.Len())
	var isSorted = slices.IsSortedFunc(pairs, func(a, b types.Pair[int64, string]) int {
		return cmp.Compare(a.First, b.First)
	})
	assert.True(t, isSorted)

	var head = sl.HeadNode()
	var tail = sl.TailNode()
	assert.Equal(t, pairs[0].Second, head.Ele)
	assert.Equal(t, pairs[len(pairs)-1].Second, tail.Ele)
}

func TestZSkipList_Insert(t *testing.T) {
	// 顺序插入
	var sl = NewZSkipList[string](types.OrderedCmp[string])
	for i := 100; i > 0; i-- {
		var score = 100 + int64(i)
		sl.Insert(score, fmt.Sprintf("test%d", i))
	}
	testSkipList(t, sl)

	//t.Logf("%s", sl.String())
	// 随机插入
	sl = NewZSkipList[string](types.OrderedCmp[string])
	var g = uniqueScoreGen()
	for i := 0; i < 100; i++ {
		var score = g()
		sl.Insert(score, fmt.Sprintf("test%d", i))
	}
	testSkipList(t, sl)
}

func TestZSkipList_Delete(t *testing.T) {
	var sl = NewZSkipList[string](types.OrderedCmp[string])
	sl.Insert(10, "test01")
	sl.Insert(20, "test02")
	sl.Insert(30, "test03")
	assert.Equal(t, 3, sl.Len())
	var s = "elem=test01, score=10,elem=test02, score=20,elem=test03, score=30,"
	assert.Equal(t, s, sl.String())

	// 删除1个已有的节点
	var node = sl.Delete(10, "test01")
	assert.NotNil(t, node)
	assert.Equal(t, 2, sl.Len())
	assert.Equal(t, "test01", node.Ele)
	assert.Equal(t, int64(10), node.Score)

	// 删除不存在的节点
	node = sl.Delete(40, "test04")
	assert.Nil(t, node)
	assert.Equal(t, 2, sl.Len())
}

func TestZSkipList_UpdateScore(t *testing.T) {
	var sl = NewZSkipList[string](types.OrderedCmp[string])
	sl.Insert(10, "test01")
	sl.Insert(20, "test02")
	sl.Insert(30, "test03")
	sl.Insert(40, "test04")
	assert.Equal(t, 4, sl.Len())
	var s = "elem=test01, score=10,elem=test02, score=20,elem=test03, score=30,elem=test04, score=40,"
	assert.Equal(t, s, sl.String())

	var node = sl.UpdateScore("test02", 20, 40)
	assert.NotNil(t, node)
	assert.Equal(t, int64(40), node.Score)
	s = "elem=test01, score=10,elem=test03, score=30,elem=test02, score=40,elem=test04, score=40,"
	assert.Equal(t, s, sl.String())
}

func TestZSkipList_DeleteRangeByRank(t *testing.T) {
	var dict = make(map[string]int64)
	var sl = NewZSkipList[string](types.OrderedCmp[string])
	for i := 1; i <= 10; i++ {
		var score = 10 * int64(i)
		var elem = fmt.Sprintf("test%02d", i)
		sl.Insert(score, elem)
		dict[elem] = score
	}
	assert.Equal(t, 10, sl.Len())
	var deleter = func(score int64, elem string) {
		delete(dict, elem)
	}
	sl.DeleteRangeByRank(1, 5, deleter)
	assert.Equal(t, 5, sl.Len())
	var s = "elem=test06, score=60,elem=test07, score=70,elem=test08, score=80,elem=test09, score=90,elem=test10, score=100,"
	assert.Equal(t, s, sl.String())

	sl.DeleteRangeByRank(4, 5, deleter)
	assert.Equal(t, 3, sl.Len())
	s = "elem=test06, score=60,elem=test07, score=70,elem=test08, score=80,"
	assert.Equal(t, s, sl.String())

	sl.DeleteRangeByRank(1, 1, deleter)
	assert.Equal(t, 2, sl.Len())
	assert.Equal(t, "elem=test07, score=70,elem=test08, score=80,", sl.String())

	sl.DeleteRangeByRank(1, 10, deleter)
	assert.Equal(t, 0, sl.Len())
}

func TestZSkipList_DeleteRangeByScore(t *testing.T) {
	var dict = make(map[string]int64)
	var sl = NewZSkipList[string](types.OrderedCmp[string])
	for i := 1; i <= 10; i++ {
		var score = 10 * int64(i)
		var elem = fmt.Sprintf("test%02d", i)
		sl.Insert(score, elem)
		dict[elem] = score
	}
	assert.Equal(t, 10, sl.Len())
	var deleter = func(score int64, elem string) {
		delete(dict, elem)
	}
	sl.DeleteRangeByScore(20, 90, deleter)
	assert.Equal(t, 2, sl.Len())
	assert.Equal(t, "elem=test01, score=10,elem=test10, score=100,", sl.String())
}

func TestZSkipList_GetRank(t *testing.T) {
	var sl = NewZSkipList[string](types.OrderedCmp[string])
	for i := 1; i <= 100; i++ {
		var score = 10 * int64(i)
		var elem = fmt.Sprintf("test%02d", i)
		sl.Insert(score, elem)
	}
	assert.Equal(t, 100, sl.Len())

	for i := 1; i <= 100; i++ {
		var score = 10 * int64(i)
		var elem = fmt.Sprintf("test%02d", i)
		var rank = sl.GetRank(score, elem)
		assert.Equal(t, i, rank)
	}
}

func TestZSkipList_GetElementByRank(t *testing.T) {
	var sl = NewZSkipList[string](types.OrderedCmp[string])
	for i := 1; i <= 100; i++ {
		var score = 10 * int64(i)
		var elem = fmt.Sprintf("test%02d", i)
		sl.Insert(score, elem)
	}
	assert.Equal(t, 100, sl.Len())

	for i := 1; i <= 100; i++ {
		var score = 10 * int64(i)
		var elem = fmt.Sprintf("test%02d", i)
		var node = sl.GetElementByRank(i)
		assert.NotNil(t, node)
		assert.Equal(t, node.Score, score)
		assert.Equal(t, node.Ele, elem)
	}
}
