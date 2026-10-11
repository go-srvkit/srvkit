// Copyright © 2023-2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package mapx

import (
	"cmp"
	"maps"
	"slices"

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
// 浮点key中的NaN排在其他key之前，多个NaN的相对顺序未定义。
func MapOrderedKeys[M ~map[K]V, K cmp.Ordered, V any](m M) []K {
	var keys = MapKeys(m)
	slices.Sort(keys)
	return keys
}

// MapOrderedValues 返回map里按key排序的value列表
// 浮点key中的NaN排在其他key之前，多个NaN对应value的相对顺序未定义。
func MapOrderedValues[M ~map[K]V, K cmp.Ordered, V any](m M) []V {
	var entries = mapEntries(m)
	slices.SortFunc(entries, func(a, b types.Pair[K, V]) int {
		return cmp.Compare(a.First, b.First)
	})
	var values = make([]V, 0, len(m))
	for _, entry := range entries {
		values = append(values, entry.Second)
	}
	return values
}

// MapOrderedKeyValues 返回map里已排序的key和value列表
// 浮点key中的NaN排在其他key之前，多个NaN的相对顺序未定义，但键值配对保持不变。
func MapOrderedKeyValues[M ~map[K]V, K cmp.Ordered, V any](m M) ([]K, []V) {
	var entries = mapEntries(m)
	slices.SortFunc(entries, func(a, b types.Pair[K, V]) int {
		return cmp.Compare(a.First, b.First)
	})
	var keys = make([]K, 0, len(m))
	var values = make([]V, 0, len(m))
	for _, entry := range entries {
		keys = append(keys, entry.First)
		values = append(values, entry.Second)
	}
	return keys, values
}

// MapKeysSortedByValue 返回map里按value排序的key列表
// value相同时按key排序；浮点key和value均按cmp.Compare的规则将NaN视为相等且小于其他值。
func MapKeysSortedByValue[M ~map[K]V, K cmp.Ordered, V cmp.Ordered](m M) []K {
	var entries = mapEntries(m)
	slices.SortFunc(entries, func(a, b types.Pair[K, V]) int {
		if order := cmp.Compare(a.Second, b.Second); order != 0 {
			return order
		}
		return cmp.Compare(a.First, b.First)
	})
	var keys = make([]K, 0, len(m))
	for _, entry := range entries {
		keys = append(keys, entry.First)
	}
	return keys
}

// mapEntries preserves values for non-reflexive keys (NaN), which cannot be looked up with m[k].
func mapEntries[M ~map[K]V, K comparable, V any](m M) []types.Pair[K, V] {
	var entries = make([]types.Pair[K, V], 0, len(m))
	for k, v := range m {
		entries = append(entries, types.MakePair(k, v))
	}
	return entries
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
