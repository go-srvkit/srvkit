// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package types

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMakePair(t *testing.T) {
	tests := []struct {
		input1 int
		input  string
		want   Pair[int, string]
	}{
		{0, "", Pair[int, string]{0, ""}},
		{123, "456", Pair[int, string]{123, "456"}},
	}
	for i, tt := range tests {
		var name = fmt.Sprintf("case-%d", i+1)
		t.Run(name, func(t *testing.T) {
			var pair = MakePair(tt.input1, tt.input)
			assert.Equalf(t, tt.want, pair, "MakePair(%v, %v)", tt.input1, tt.input)
		})
	}
}

func TestPair_String(t *testing.T) {
	tests := []struct {
		name string
		pair Pair[int, string]
		want string
	}{
		{"zero values", Pair[int, string]{}, "(0, )"},
		{"normal values", MakePair(123, "456"), "(123, 456)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.pair.String())
		})
	}
}

func TestCompare(t *testing.T) {
	tests := []struct {
		name string
		a    Pair[int, string]
		b    Pair[int, string]
		want int
	}{
		{"equal", MakePair(1, "a"), MakePair(1, "a"), 0},
		{"first less", MakePair(1, "b"), MakePair(2, "a"), -1},
		{"first greater", MakePair(2, "a"), MakePair(1, "b"), 1},
		{"first equal second less", MakePair(1, "a"), MakePair(1, "b"), -1},
		{"first equal second greater", MakePair(1, "b"), MakePair(1, "a"), 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Compare(tt.a, tt.b)
			if tt.want < 0 {
				assert.True(t, result < 0)
			} else if tt.want > 0 {
				assert.True(t, result > 0)
			} else {
				assert.Zero(t, result)
			}
		})
	}
}

func TestPair_Swap(t *testing.T) {
	a := MakePair(1, "a")
	b := MakePair(2, "b")
	a.Swap(&b)
	assert.Equal(t, MakePair(2, "b"), a)
	assert.Equal(t, MakePair(1, "a"), b)
}

func TestEqualPair(t *testing.T) {
	tests := []struct {
		name string
		a    Pair[int, string]
		b    Pair[int, string]
		want bool
	}{
		{"equal", MakePair(1, "a"), MakePair(1, "a"), true},
		{"not equal first", MakePair(1, "a"), MakePair(2, "a"), false},
		{"not equal second", MakePair(1, "a"), MakePair(1, "b"), false},
		{"zero values", Pair[int, string]{}, Pair[int, string]{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, EqualPair(tt.a, tt.b))
		})
	}
}

func TestMakePairNonComparableValues(t *testing.T) {
	first := []int{1, 2}
	second := map[string]int{"count": 3}
	pair := MakePair(first, second)

	assert.Equal(t, first, pair.First)
	assert.Equal(t, second, pair.Second)
	first[0] = 4
	second["count"] = 5
	assert.Equal(t, 4, pair.First[0], "MakePair preserves slice references")
	assert.Equal(t, 5, pair.Second["count"], "MakePair preserves map references")

	var emptySlice []int
	var emptyMap map[string]int
	empty := MakePair(emptySlice, emptyMap)
	assert.Nil(t, empty.First)
	assert.Nil(t, empty.Second)
}

func TestPairStringFormatting(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"negative and empty", MakePair(-1, "").String(), "(-1, )"},
		{"boolean and slice", MakePair(true, []int{1, 2}).String(), "(true, [1 2])"},
		{"nil pointer", MakePair((*int)(nil), "nil").String(), "(<nil>, nil)"},
		{"nested pair", MakePair(MakePair(1, "a"), false).String(), "((1, a), false)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.got)
		})
	}
}

func TestCompareIntegerBoundaries(t *testing.T) {
	a := MakePair(int64(math.MinInt64), int64(math.MaxInt64))
	b := MakePair(int64(math.MaxInt64), int64(math.MinInt64))
	assert.Equal(t, -1, Compare(a, b))
	assert.Equal(t, 1, Compare(b, a))
	assert.Equal(t, 0, Compare(a, a))

	x := MakePair(uint64(math.MaxUint64), uint64(0))
	y := MakePair(uint64(math.MaxUint64), uint64(math.MaxUint64))
	assert.Equal(t, -1, Compare(x, y))
	assert.Equal(t, 1, Compare(y, x))
	assert.Equal(t, 0, Compare(y, y))
}

func TestCompareFloatingPoint(t *testing.T) {
	nan := math.NaN()
	tests := []struct {
		name string
		a, b Pair[float64, float64]
		want int
	}{
		{"NaN first precedes finite", MakePair(nan, 2.0), MakePair(0.0, 1.0), -1},
		{"finite first follows NaN", MakePair(0.0, 1.0), MakePair(nan, 2.0), 1},
		{"equal NaN first compares second", MakePair(nan, 1.0), MakePair(nan, 2.0), -1},
		{"NaN second precedes finite", MakePair(1.0, nan), MakePair(1.0, 0.0), -1},
		{"both fields NaN", MakePair(nan, nan), MakePair(nan, nan), 0},
		{"negative infinity", MakePair(math.Inf(-1), 2.0), MakePair(0.0, 1.0), -1},
		{"positive infinity", MakePair(math.Inf(1), 1.0), MakePair(0.0, 2.0), 1},
		{"signed zero", MakePair(math.Copysign(0, -1), 1.0), MakePair(0.0, 1.0), 0},
		{"first field takes precedence", MakePair(1.0, nan), MakePair(2.0, math.Inf(-1)), -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Compare(tt.a, tt.b))
			assert.Equal(t, -tt.want, Compare(tt.b, tt.a))
		})
	}
}

func TestPairSwapNonComparableValues(t *testing.T) {
	a := MakePair([]int{1, 2}, map[string]int{"count": 3})
	var b Pair[[]int, map[string]int]
	first, second := a.First, a.Second

	a.Swap(&b)

	assert.Nil(t, a.First)
	assert.Nil(t, a.Second)
	assert.Equal(t, first, b.First)
	assert.Equal(t, second, b.Second)
	first[0] = 4
	second["count"] = 5
	assert.Equal(t, 4, b.First[0])
	assert.Equal(t, 5, b.Second["count"])

	b.Swap(&b)
	assert.Equal(t, first, b.First, "swapping with itself preserves values")
	assert.Equal(t, second, b.Second)
	b.Swap(&a)
	assert.Equal(t, first, a.First)
	assert.Equal(t, second, a.Second)
	assert.Nil(t, b.First)
	assert.Nil(t, b.Second)
}

func TestEqualPairPointerIdentity(t *testing.T) {
	x, y := 1, 1
	a := MakePair(&x, [2]int{1, 2})
	assert.True(t, EqualPair(a, MakePair(&x, [2]int{1, 2})))
	assert.False(t, EqualPair(a, MakePair(&y, [2]int{1, 2})))
	assert.False(t, EqualPair(a, MakePair(&x, [2]int{2, 1})))
	assert.False(t, EqualPair(a, MakePair((*int)(nil), [2]int{1, 2})))
	assert.True(t, EqualPair(Pair[*int, [2]int]{}, Pair[*int, [2]int]{}))
}

func TestEqualPairFloatingPoint(t *testing.T) {
	nan := math.NaN()
	assert.False(t, EqualPair(MakePair(nan, 1.0), MakePair(nan, 1.0)))
	assert.False(t, EqualPair(MakePair(1.0, nan), MakePair(1.0, nan)))
	assert.True(t, EqualPair(MakePair(math.Copysign(0, -1), math.Inf(1)), MakePair(0.0, math.Inf(1))))
}
