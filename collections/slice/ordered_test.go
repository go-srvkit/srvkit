// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package slice

import (
	"cmp"
	"math"
	"math/rand"
	"slices"
	"sort"
	"testing"
)

func TestOrderedPutIfAbsent(t *testing.T) {
	var idSet []int

	for i := 50; i > 0; i-- {
		idSet = OrderedPutIfAbsent(idSet, i)
	}
	if !sort.IsSorted(IntSlice(idSet)) {
		t.Error("OrderedIDSet is not sorted")
	}
	for i := 1; i < 50; i++ {
		if !OrderedContains(idSet, i) {
			t.Error("OrderedIDSet does not contain", i)
		}
	}
	var n = len(idSet)
	for i := 1; i < 50; i++ {
		idSet = OrderedPutIfAbsent(idSet, i)
	}
	if len(idSet) != n {
		t.Error("OrderedIDSet does not contain duplicates")
	}
}

func TestOrderedDelete(t *testing.T) {
	var idSet []int
	for i := 1; i <= 100; i++ {
		idSet = OrderedPutIfAbsent(idSet, i)
	}
	for i := 1; i <= 90; i++ {
		idSet = OrderedDelete(idSet, i)
	}
	if len(idSet) > 10 {
		t.Error("OrderedIDSet size error")
	}
	idSet = nil
	for i := 1; i <= 100; i++ {
		idSet = OrderedPutIfAbsent(idSet, i)
	}

	var deleted []int
	for i := 0; i < 80; i++ {
		var idx = rand.Int() % len(idSet)
		deleted = append(deleted, idSet[idx])
		idSet = slices.Delete(idSet, idx, idx+1)
	}
	for _, n := range deleted {
		if OrderedContains(idSet, n) {
			t.Error("OrderedIDSet does not contain", n)
		}
	}
}

func TestOrderedContains(t *testing.T) {
	var idSet Int32Slice
	for i := 1; i <= 100; i++ {
		idSet = append(idSet, int32(i))
	}

	var deleted []int32
	for i := 0; i < 80; i++ {
		var idx = rand.Int() % len(idSet)
		deleted = append(deleted, idSet[idx])
		idSet = slices.Delete(idSet, idx, idx+1)
	}
	for _, n := range deleted {
		if OrderedContains(idSet, n) {
			t.Error("OrderedIDSet does not contain", n)
		}
	}
}

func TestOrderedUnion(t *testing.T) {
	tests := []struct {
		input1   []int32
		input2   []int32
		expected []int32
	}{
		{[]int32{1, 2, 3}, nil, []int32{1, 2, 3}},
		{[]int32{1, 2, 3}, []int32{1, 2, 3}, []int32{1, 2, 3}},
		{[]int32{1, 2, 3, 4, 5}, []int32{4, 5, 6, 7, 8}, []int32{1, 2, 3, 4, 5, 6, 7, 8}},
		{[]int32{4, 5, 6, 7, 8}, []int32{1, 2, 3, 4, 5}, []int32{1, 2, 3, 4, 5, 6, 7, 8}},
		{[]int32{1, 3, 5, 7}, []int32{2, 4, 6, 8}, []int32{1, 2, 3, 4, 5, 6, 7, 8}},
		{[]int32{2, 4, 6, 8}, []int32{1, 3, 5, 7}, []int32{1, 2, 3, 4, 5, 6, 7, 8}},
	}
	for _, tc := range tests {
		var result = OrderedUnion(tc.input1, tc.input2)
		if len(result) != len(tc.expected) {
			t.Errorf("Expected %v, got %v", tc.expected, result)
		}
		for i := 0; i < len(result); i++ {
			if result[i] != tc.expected[i] {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		}
	}
}

func TestOrderedIntersect(t *testing.T) {
	tests := []struct {
		input1   []int32
		input2   []int32
		expected []int32
	}{
		{[]int32{1, 2, 3}, nil, nil},
		{[]int32{1, 2, 3}, []int32{4, 5, 6}, nil},
		{[]int32{1, 2, 3, 4, 5}, []int32{4, 5, 6, 7, 8}, []int32{4, 5}},
		{[]int32{4, 5, 6, 7, 8}, []int32{1, 2, 3, 4, 5}, []int32{4, 5}},
	}
	for _, tc := range tests {
		var result = OrderedIntersect(tc.input1, tc.input2)
		if len(result) != len(tc.expected) {
			t.Errorf("Expected %v, got %v", tc.expected, result)
		}
		for i := 0; i < len(result); i++ {
			if result[i] != tc.expected[i] {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		}
	}
}

func equalOrderedFloats[T ~float32 | ~float64](a, b []T) bool {
	return slices.EqualFunc(a, b, func(x, y T) bool { return cmp.Compare(x, y) == 0 })
}

func testOrderedFloatSetOperations[T ~float32 | ~float64](t *testing.T) {
	t.Helper()
	nan := T(math.NaN())
	otherNaN := T(math.Float64frombits(0x7ff8000020000001))
	negInf, posInf := T(math.Inf(-1)), T(math.Inf(1))
	tests := []struct {
		name                      string
		a, b, union, intersection []T
	}{
		{"NaN and finite", []T{nan}, []T{1}, []T{nan, 1}, nil},
		{"NaN and negative infinity", []T{nan}, []T{negInf}, []T{nan, negInf}, nil},
		{"different NaNs compare equal", []T{nan}, []T{otherNaN}, []T{nan}, []T{nan}},
		{"NaN in both inputs", []T{nan, negInf, 0, 2}, []T{otherNaN, -1, 0, posInf}, []T{nan, negInf, -1, 0, 2, posInf}, []T{nan, 0}},
		{"NaN in one input", []T{nan, -1, 1}, []T{-1, 2}, []T{nan, -1, 1, 2}, []T{-1}},
		{"duplicates", []T{nan, nan, -1, 2, 2}, []T{otherNaN, 2}, []T{nan, nan, -1, 2, 2}, []T{nan, 2}},
		{"only NaN duplicates", []T{nan, nan}, []T{otherNaN, otherNaN, otherNaN}, []T{nan, nan, nan}, []T{nan, nan}},
		{"empty input", nil, []T{nan, 1}, []T{nan, 1}, nil},
		{"signed zero", []T{nan, T(math.Copysign(0, -1))}, []T{otherNaN, 0}, []T{nan, 0}, []T{nan, 0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			beforeA, beforeB := slices.Clone(tt.a), slices.Clone(tt.b)
			for _, input := range []struct {
				name string
				a, b []T
			}{
				{"forward", tt.a, tt.b},
				{"reverse", tt.b, tt.a},
			} {
				t.Run(input.name, func(t *testing.T) {
					union := OrderedUnion(input.a, input.b)
					if !equalOrderedFloats(union, tt.union) {
						t.Errorf("OrderedUnion(%v, %v) = %v, want %v", input.a, input.b, union, tt.union)
					}
					intersection := OrderedIntersect(input.a, input.b)
					if !equalOrderedFloats(intersection, tt.intersection) {
						t.Errorf("OrderedIntersect(%v, %v) = %v, want %v", input.a, input.b, intersection, tt.intersection)
					}
					if !slices.IsSorted(union) || !slices.IsSorted(intersection) {
						t.Error("set operation returned an unsorted result")
					}
				})
			}
			if !equalOrderedFloats(tt.a, beforeA) || !equalOrderedFloats(tt.b, beforeB) {
				t.Error("set operation modified an input slice")
			}
		})
	}
}

func TestOrderedSetOperationsNaN(t *testing.T) {
	t.Run("float32", testOrderedFloatSetOperations[float32])
	t.Run("float64", testOrderedFloatSetOperations[float64])
	t.Run("named float", func(t *testing.T) {
		type score float64
		testOrderedFloatSetOperations[score](t)
	})
}

func testOrderedFloatNaNLifecycle[T ~float32 | ~float64](t *testing.T) {
	t.Helper()
	nan := T(math.NaN())
	otherNaN := T(math.Float64frombits(0x7ff8000020000001))
	finite := []T{T(math.Inf(-1)), -1, 0, T(math.Inf(1))}
	if OrderedIndexOf(finite, nan) != -1 || OrderedContains(finite, nan) {
		t.Fatal("NaN unexpectedly found in a slice without NaN")
	}
	if OrderedIndexOf([]T(nil), nan) != -1 || OrderedContains([]T(nil), nan) {
		t.Fatal("NaN unexpectedly found in an empty slice")
	}

	ordered := OrderedPutIfAbsent(slices.Clone(finite), nan)
	want := append([]T{nan}, finite...)
	if !equalOrderedFloats(ordered, want) || OrderedIndexOf(ordered, otherNaN) != 0 || !OrderedContains(ordered, otherNaN) {
		t.Fatalf("inserted NaN cannot be found at the front: %v", ordered)
	}
	ordered = OrderedPutIfAbsent(ordered, otherNaN)
	if !equalOrderedFloats(ordered, want) {
		t.Fatalf("inserting another NaN created a duplicate: %v", ordered)
	}
	ordered = OrderedDelete(ordered, otherNaN)
	if !equalOrderedFloats(ordered, finite) || OrderedContains(ordered, nan) {
		t.Fatalf("deleting NaN changed other elements or left NaN: %v", ordered)
	}
	ordered = OrderedDelete(ordered, nan)
	if !equalOrderedFloats(ordered, finite) {
		t.Fatalf("deleting missing NaN changed the slice: %v", ordered)
	}

	duplicates := OrderedDelete([]T{nan, otherNaN, 1}, nan)
	if !equalOrderedFloats(duplicates, []T{otherNaN, 1}) || OrderedIndexOf(duplicates, nan) != 0 {
		t.Fatalf("OrderedDelete must remove only the first NaN: %v", duplicates)
	}
	inserted := OrderedPutIfAbsent([]T(nil), nan)
	if !equalOrderedFloats(inserted, []T{nan}) {
		t.Fatalf("inserting NaN into an empty slice returned %v", inserted)
	}
}

func TestOrderedNaNLifecycle(t *testing.T) {
	t.Run("float32", testOrderedFloatNaNLifecycle[float32])
	t.Run("float64", testOrderedFloatNaNLifecycle[float64])
}
