// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package slice

import (
	"math/rand/v2"
	"slices"
)

// 常用的数组操作函数

// InsertAt 把`v`插入到第`i`个位置
func InsertAt[E any](s []E, i int, v E) []E {
	if i >= 0 && i < len(s) {
		return slices.Insert(s, i, v)
	}
	return append(s, v) // 超出区间的位置append到末尾
}

// RemoveAt 删除第`i`个元素，不保证原来元素的顺序
func RemoveAt[E any](s []E, i int) []E {
	if n := len(s); i >= 0 && i < n {
		s[i] = s[n-1]
		clear(s[n-1:]) // zero/nil out the obsolete elements, GC friendly
		return s[:n-1]
	}
	return s
}

// RemoveFirst 删除第一个查询到的元素
func RemoveFirst[E comparable](s []E, elem E) []E {
	var i = slices.Index(s, elem)
	if i >= 0 {
		return slices.Delete(s, i, i+1)
	}
	return s
}

// RemoveElements 原地删除`s`里所有值为`elem`的元素
func RemoveElements[E comparable](s []E, elem E) []E {
	var left = 0
	for _, v := range s {
		if v != elem {
			s[left] = v
			left++
		}
	}
	clear(s[left:])
	return s[:left]
}

// RemoveElementsFunc 原地删除`s`里所有满足`pred`的元素
func RemoveElementsFunc[T any](s []T, pred func(T) bool) []T {
	var left = 0
	for _, v := range s {
		if !pred(v) {
			s[left] = v
			left++
		}
	}
	clear(s[left:])
	return s[:left]
}

// Shuffle 随机打乱数组顺序
func Shuffle[E any](s []E) {
	rand.Shuffle(len(s), func(i, j int) {
		s[i], s[j] = s[j], s[i]
	})
}

// Shrink 缩减切片容量到等于长度，释放多余内存
func Shrink[E any](s []E) []E {
	if len(s) == 0 {
		return nil
	}
	if len(s) == cap(s) {
		return s
	}
	var a = make([]E, len(s))
	copy(a, s)
	return a
}
