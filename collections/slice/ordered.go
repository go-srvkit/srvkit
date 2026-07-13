// Copyright © 2023-2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package slice

import (
	"cmp"
	"slices"
)

// 有序数组常用算法

// OrderedIndexOf `target`在有序数组`ordered`中的索引，如果不存在返回-1
func OrderedIndexOf[T cmp.Ordered](ordered []T, target T) int {
	i, found := slices.BinarySearch(ordered, target)
	if found {
		return i
	}
	return -1
}

// OrderedContains `target`是否在有序数组`ordered`中
func OrderedContains[T cmp.Ordered](ordered []T, target T) bool {
	_, found := slices.BinarySearch(ordered, target)
	return found
}

// OrderedPutIfAbsent 插入`n`到有序数组
func OrderedPutIfAbsent[T cmp.Ordered](ordered []T, target T) []T {
	i, found := slices.BinarySearch(ordered, target)
	if found {
		return ordered
	}
	return InsertAt(ordered, i, target)
}

// OrderedDelete 把`n`从有序数组中删除
func OrderedDelete[T cmp.Ordered](ordered []T, target T) []T {
	i, found := slices.BinarySearch(ordered, target)
	if found {
		return slices.Delete(ordered, i, i+1)
	}
	return ordered
}

// OrderedUnion 有序数组并集, A ∪ B，假设A和B都是有序
func OrderedUnion[T cmp.Ordered](a, b []T) []T {
	if len(a) == 0 {
		return append([]T{}, b...) // slice clone
	}
	if len(b) == 0 {
		return append([]T{}, a...) // slice clone
	}
	var c = make([]T, 0, len(a)+len(b))
	var i, j int
	for i < len(a) && j < len(b) {
		if a[i] < b[j] {
			c = append(c, a[i])
			i++
		} else if a[i] > b[j] {
			c = append(c, b[j])
			j++
		} else {
			c = append(c, a[i])
			i++
			j++
		}
	}
	for i < len(a) {
		c = append(c, a[i])
		i++
	}
	for j < len(b) {
		c = append(c, b[j])
		j++
	}
	return c
}

// OrderedIntersect 有序数组交集, A ∩ B，假设A和B都是有序
func OrderedIntersect[T cmp.Ordered](a, b []T) []T {
	if len(a) == 0 || len(b) == 0 {
		return nil
	}
	var c = make([]T, 0, min(len(a), len(b)))
	var i, j int
	for i < len(a) && j < len(b) {
		if a[i] < b[j] {
			i++
		} else if a[i] > b[j] {
			j++
		} else {
			c = append(c, a[i])
			i++
			j++
		}
	}
	return c
}
