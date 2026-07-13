// Copyright © 2023-2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package mapx

import (
	"cmp"
	"maps"
	"slices"
	"sort"

	"gopkg.in/srvkit.v1/collections/types"
)

// MapKeys 返回map的key列表
func MapKeys[M ~map[K]V, K comparable, V any](m M) []K {
	var keys = make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// MapValues 返回map的value列表
func MapValues[M ~map[K]V, K comparable, V any](m M) []V {
	var values = make([]V, 0, len(m))
	for _, val := range m {
		values = append(values, val)
	}
	return values
}

// MapKeyValues 返回map的key和value列表
func MapKeyValues[M ~map[K]V, K comparable, V any](m M) ([]K, []V) {
	var keys = make([]K, 0, len(m))
	var values = make([]V, 0, len(m))
	for k, val := range m {
		keys = append(keys, k)
		values = append(values, val)
	}
	return keys, values
}

// MapOrderedKeys 返回map里已排序的key列表
func MapOrderedKeys[M ~map[K]V, K cmp.Ordered, V any](m M) []K {
	var keys = MapKeys(m)
	slices.Sort(keys)
	return keys
}

// MapOrderedValues 返回map里按key排序的value列表
func MapOrderedValues[M ~map[K]V, K cmp.Ordered, V any](m M) []V {
	var keys = MapKeys(m)
	slices.Sort(keys)
	var values = make([]V, 0, len(m))
	for _, k := range keys {
		values = append(values, m[k])
	}
	return values
}

// MapOrderedKeyValues 返回map里已排序的key和value列表
func MapOrderedKeyValues[M ~map[K]V, K cmp.Ordered, V any](m M) ([]K, []V) {
	var keys = MapKeys(m)
	slices.Sort(keys)
	var values = make([]V, 0, len(m))
	for _, k := range keys {
		values = append(values, m[k])
	}
	return keys, values
}

// MapKeysSortedByValue 返回map里按value排序的key列表
func MapKeysSortedByValue[M ~map[K]V, K cmp.Ordered, V cmp.Ordered](m M) []K {
	var keys = MapKeys(m)
	sort.Slice(keys, func(i, j int) bool {
		ki, kj := keys[i], keys[j]
		if m[ki] == m[kj] {
			return ki < kj // 值相同时按key排序
		}
		return m[ki] < m[kj]
	})
	return keys
}

// MapUnion 返回两个map的并集, copy of a ∪ b
func MapUnion[M ~map[K]V, K comparable, V any](a, b M) M {
	var result = make(M, len(a)+len(b))
	maps.Copy(result, a)
	maps.Copy(result, b)
	return result
}

// MapIntersect 返回两个map的交集, a ∩ b
func MapIntersect[M ~map[K]V, K, V comparable](a, b M) M {
	if a == nil && b == nil {
		return nil
	}
	var result = make(M)
	for k, v := range a {
		if val, ok := b[k]; ok && val == v {
			result[k] = v
		}
	}
	return result
}

// MapDifference 返回两个map的差集, a - b
func MapDifference[M ~map[K]V, K, V comparable](a, b M) M {
	if a == nil {
		return nil
	}
	var result = make(M)
	for k, v := range a {
		if val, ok := b[k]; !ok || val != v {
			result[k] = v
		}
	}
	return result
}

// MapAddToValues 累加`src`的所有值到`dst`里
func MapAddToValues[M ~map[K]V, K comparable, V types.Number](dst, src M) M {
	if dst != nil {
		for k, v := range src {
			dst[k] += v
		}
	}
	return dst
}

// MapSumKeys 对map的所有key求和
func MapSumKeys[M ~map[K]V, K types.Number, V any](m M) K {
	var sum K
	for key := range m {
		sum += key
	}
	return sum
}

// MapSumValues 对map的所有值求和
func MapSumValues[M ~map[K]V, K comparable, V types.Number](m M) V {
	var sum V
	for _, val := range m {
		sum += val
	}
	return sum
}
