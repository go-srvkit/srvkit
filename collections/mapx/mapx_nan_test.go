// Copyright © 2026 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package mapx

import (
	"cmp"
	"math"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMapOrderedNaNKeys(t *testing.T) {
	t.Run("float32", testMapOrderedNaNKeys[float32])
	t.Run("float64", testMapOrderedNaNKeys[float64])
	type namedFloat float64
	t.Run("namedFloat", testMapOrderedNaNKeys[namedFloat])
}

func testMapOrderedNaNKeys[T ~float32 | ~float64](t *testing.T) {
	nan := T(math.NaN())
	for _, name := range []string{"onlyNaNs", "mixed"} {
		t.Run(name, func(t *testing.T) {
			// Reusing the same NaN still inserts distinct entries in a Go map.
			m := make(map[T][]int)
			m[nan] = []int{1}
			m[nan] = []int{2}
			m[T(math.Float32frombits(0x7fc00002))] = []int{3}
			wantKeys := []T{nan, nan, nan}
			wantTail := [][]int{}
			if name == "mixed" {
				m[T(math.Inf(-1))] = []int{4}
				m[-1] = []int{5}
				m[0] = []int{6}
				m[T(math.Inf(1))] = []int{7}
				wantKeys = append(wantKeys, T(math.Inf(-1)), -1, 0, T(math.Inf(1)))
				wantTail = [][]int{{4}, {5}, {6}, {7}}
			}

			checkKeys := func(keys []T) {
				t.Helper()
				assert.True(t, slices.EqualFunc(wantKeys, keys, func(a, b T) bool {
					return cmp.Compare(a, b) == 0
				}), "keys = %v, want %v", keys, wantKeys)
			}
			checkValues := func(values [][]int) {
				t.Helper()
				if !assert.Len(t, values, len(m)) {
					return
				}
				// NaN keys compare equal, so their relative order is unspecified.
				assert.ElementsMatch(t, [][]int{{1}, {2}, {3}}, values[:3])
				assert.Equal(t, wantTail, values[3:])
			}
			checkKeys(MapOrderedKeys(m))
			checkValues(MapOrderedValues(m))
			keys, values := MapOrderedKeyValues(m)
			checkKeys(keys)
			checkValues(values)
			assert.Len(t, m, len(wantKeys))
			assert.ElementsMatch(t, append([][]int{{1}, {2}, {3}}, wantTail...), MapValues(m))
		})
	}
}

func TestMapKeysSortedByNaNValue(t *testing.T) {
	t.Run("float32", testMapKeysSortedByNaNValue[float32])
	t.Run("float64", testMapKeysSortedByNaNValue[float64])
	type namedFloat float64
	t.Run("namedFloat", testMapKeysSortedByNaNValue[namedFloat])
}

func testMapKeysSortedByNaNValue[T ~float32 | ~float64](t *testing.T) {
	m := map[int]T{
		9: T(math.NaN()),
		2: T(math.NaN()),
		8: T(math.Inf(-1)),
		1: -1,
		7: T(math.Copysign(0, -1)),
		3: 0,
		6: 1,
		4: T(math.Inf(1)),
	}
	assert.Equal(t, []int{2, 9, 8, 1, 3, 7, 6, 4}, MapKeysSortedByValue(m))
}

func TestMapKeysSortedByValueNaNKeys(t *testing.T) {
	nan1 := math.Float64frombits(0x7ff8000000000001)
	nan2 := math.Float64frombits(0x7ff8000000000002)
	tests := []struct {
		name string
		m    map[float64]float64
		want []float64
	}{
		{"actualValue", map[float64]float64{nan1: 2, -1: 1}, []float64{-1, nan1}},
		{"keyTieBreak", map[float64]float64{nan1: 1, -1: 1}, []float64{nan1, -1}},
		{"mixed", map[float64]float64{nan1: 5, nan2: 1, 2: nan1, 1: nan2, 0: 3}, []float64{1, 2, nan2, 0, nan1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MapKeysSortedByValue(tt.m)
			// Comparing bit patterns also verifies distinct NaN keys retain their values.
			assert.True(t, slices.EqualFunc(tt.want, got, func(a, b float64) bool {
				return math.Float64bits(a) == math.Float64bits(b)
			}), "keys = %v, want %v", got, tt.want)
		})
	}
}

func TestMapOrderedKeyValuesNaNPairing(t *testing.T) {
	nan1 := math.Float64frombits(0x7ff8000000000001)
	nan2 := math.Float64frombits(0x7ff8000000000002)
	m := map[float64]int{nan1: 1, nan2: 2}
	keys, values := MapOrderedKeyValues(m)
	if !assert.Len(t, keys, 2) || !assert.Len(t, values, 2) {
		return
	}
	wantBits := map[int]uint64{1: math.Float64bits(nan1), 2: math.Float64bits(nan2)}
	for i, key := range keys {
		assert.Equal(t, wantBits[values[i]], math.Float64bits(key))
	}
}
