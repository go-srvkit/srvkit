// Copyright © 2023 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package types

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRange(t *testing.T) {
	tests := []struct {
		name    string
		a, b    int
		wantMin int
		wantMax int
	}{
		{"a < b", 1, 5, 1, 5},
		{"a > b", 5, 1, 1, 5},
		{"a == b", 3, 3, 3, 3},
		{"negative values", -10, -1, -10, -1},
		{"cross zero", -5, 5, -5, 5},
		{"zero and positive", 0, 10, 0, 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRange(tt.a, tt.b)
			assert.Equal(t, tt.wantMin, r.Min)
			assert.Equal(t, tt.wantMax, r.Max)
		})
	}
}

func TestNewRange_Types(t *testing.T) {
	t.Run("int8", func(t *testing.T) {
		r := NewRange[int8](10, 1)
		assert.Equal(t, int8(1), r.Min)
		assert.Equal(t, int8(10), r.Max)
	})
	t.Run("int64", func(t *testing.T) {
		r := NewRange[int64](100, 200)
		assert.Equal(t, int64(100), r.Min)
		assert.Equal(t, int64(200), r.Max)
	})
	t.Run("uint", func(t *testing.T) {
		r := NewRange[uint](20, 5)
		assert.Equal(t, uint(5), r.Min)
		assert.Equal(t, uint(20), r.Max)
	})
}

func TestRange_In(t *testing.T) {
	r := NewRange(1, 10)
	tests := []struct {
		name string
		val  int
		want bool
	}{
		{"below range", 0, false},
		{"at min boundary", 1, true},
		{"in middle", 5, true},
		{"at max boundary", 10, true},
		{"above range", 11, false},
		{"negative value", -1, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, r.In(tt.val))
		})
	}
}

func TestRange_In_ZeroRange(t *testing.T) {
	r := NewRange(5, 5)
	assert.True(t, r.In(5))
	assert.False(t, r.In(4))
	assert.False(t, r.In(6))
}

func TestRange_In_NegativeRange(t *testing.T) {
	r := NewRange(-10, -1)
	assert.True(t, r.In(-5))
	assert.True(t, r.In(-10))
	assert.True(t, r.In(-1))
	assert.False(t, r.In(0))
	assert.False(t, r.In(-11))
}

func TestRange_Contains(t *testing.T) {
	r := NewRange(1, 10)
	tests := []struct {
		name string
		a, b int
		want bool
	}{
		{"fully contained", 3, 7, true},
		{"exact match", 1, 10, true},
		{"left boundary match", 1, 5, true},
		{"right boundary match", 5, 10, true},
		{"extends below", 0, 5, false},
		{"extends above", 5, 11, false},
		{"extends both sides", 0, 11, false},
		{"completely outside", 20, 30, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, r.Contains(tt.a, tt.b))
		})
	}
}

func TestRange_Overlaps(t *testing.T) {
	r := NewRange(5, 10)
	tests := []struct {
		name  string
		other Range[int]
		want  bool
	}{
		{"overlap left", NewRange(1, 7), true},
		{"overlap right", NewRange(8, 15), true},
		{"fully contained", NewRange(6, 9), true},
		{"contains other", NewRange(1, 20), true},
		{"exact match", NewRange(5, 10), true},
		{"adjacent left", NewRange(1, 4), false},
		{"adjacent right", NewRange(11, 15), false},
		{"completely left", NewRange(-5, 3), false},
		{"completely right", NewRange(15, 20), false},
		{"touch at min", NewRange(5, 5), true},
		{"touch at max", NewRange(10, 10), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, r.Overlaps(tt.other))
		})
	}
}

func TestRange_IsZero(t *testing.T) {
	tests := []struct {
		name string
		r    Range[int]
		want bool
	}{
		{"zero range", Range[int]{}, true},
		{"explicit zero", NewRange(0, 0), true},
		{"non-zero", NewRange(1, 5), false},
		{"zero min non-zero max", NewRange(0, 5), false},
		{"non-zero min zero max", NewRange(-5, 0), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.r.IsZero())
		})
	}
}

func TestRange_Mid(t *testing.T) {
	tests := []struct {
		name string
		r    Range[int]
		want int
	}{
		{"normal range", NewRange(0, 10), 5},
		{"odd range", NewRange(0, 9), 4},
		{"negative range", NewRange(-10, 0), -5},
		{"cross zero", NewRange(-10, 10), 0},
		{"single value", NewRange(5, 5), 5},
		{"large values", Range[int]{Min: 1_000_000_000, Max: 2_000_000_000}, 1_500_000_000},
		{"same large values avoid overflow", Range[int]{Min: math.MaxInt - 2, Max: math.MaxInt}, math.MaxInt - 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.r.Mid())
		})
	}
}

func TestRange_Rand(t *testing.T) {
	t.Run("result within range", func(t *testing.T) {
		r := NewRange(1, 100)
		for i := 0; i < 1000; i++ {
			v := r.Rand()
			assert.True(t, v >= 1 && v <= 100, "Rand() = %d, want in [1, 100]", v)
		}
	})

	t.Run("single value range", func(t *testing.T) {
		r := NewRange(42, 42)
		for i := 0; i < 100; i++ {
			assert.Equal(t, 42, r.Rand())
		}
	})

	t.Run("negative range", func(t *testing.T) {
		r := NewRange(-100, -1)
		for i := 0; i < 1000; i++ {
			v := r.Rand()
			assert.True(t, v >= -100 && v <= -1, "Rand() = %d, want in [-100, -1]", v)
		}
	})

	t.Run("cross zero range", func(t *testing.T) {
		r := NewRange(-50, 50)
		for i := 0; i < 1000; i++ {
			v := r.Rand()
			assert.True(t, v >= -50 && v <= 50, "Rand() = %d, want in [-50, 50]", v)
		}
	})

	t.Run("Min > Max panics", func(t *testing.T) {
		r := Range[int]{Min: 10, Max: 5}
		assert.Panics(t, func() { r.Rand() })
	})
}

func checkRangeBoundaryValues[T Integer](t *testing.T, min, max, wantMid T) {
	t.Helper()
	r := NewRange(max, min)
	assert.Equal(t, min, r.Min)
	assert.Equal(t, max, r.Max)
	assert.Equal(t, wantMid, r.Mid())
	assert.True(t, r.In(min))
	assert.True(t, r.In(max))
	assert.True(t, r.Contains(min, max))
	assert.True(t, r.Overlaps(Range[T]{Min: min, Max: min}))
	assert.True(t, r.Overlaps(Range[T]{Min: max, Max: max}))
	for i := 0; i < 128; i++ {
		value := r.Rand()
		if value < min || value > max {
			t.Fatalf("Rand() = %d, want in [%d, %d]", value, min, max)
		}
	}
}

func TestRangeIntegerBoundaries(t *testing.T) {
	t.Run("int", func(t *testing.T) {
		checkRangeBoundaryValues[int](t, math.MinInt, math.MaxInt, -1)
	})
	t.Run("int8", func(t *testing.T) {
		checkRangeBoundaryValues[int8](t, math.MinInt8, math.MaxInt8, -1)
	})
	t.Run("int16", func(t *testing.T) {
		checkRangeBoundaryValues[int16](t, math.MinInt16, math.MaxInt16, -1)
	})
	t.Run("int32", func(t *testing.T) {
		checkRangeBoundaryValues[int32](t, math.MinInt32, math.MaxInt32, -1)
	})
	t.Run("int64", func(t *testing.T) {
		checkRangeBoundaryValues[int64](t, math.MinInt64, math.MaxInt64, -1)
	})
	t.Run("uint", func(t *testing.T) {
		checkRangeBoundaryValues[uint](t, 0, math.MaxUint, math.MaxUint/2)
	})
	t.Run("uint8", func(t *testing.T) {
		checkRangeBoundaryValues[uint8](t, 0, math.MaxUint8, math.MaxUint8/2)
	})
	t.Run("uint16", func(t *testing.T) {
		checkRangeBoundaryValues[uint16](t, 0, math.MaxUint16, math.MaxUint16/2)
	})
	t.Run("uint32", func(t *testing.T) {
		checkRangeBoundaryValues[uint32](t, 0, math.MaxUint32, math.MaxUint32/2)
	})
	t.Run("uint64", func(t *testing.T) {
		checkRangeBoundaryValues[uint64](t, 0, math.MaxUint64, math.MaxUint64/2)
	})
	t.Run("uintptr", func(t *testing.T) {
		max := ^uintptr(0)
		checkRangeBoundaryValues[uintptr](t, 0, max, max/2)
	})
	t.Run("named signed type", func(t *testing.T) {
		type score int16
		checkRangeBoundaryValues[score](t, math.MinInt16, math.MaxInt16, -1)
	})
	t.Run("named unsigned type", func(t *testing.T) {
		type memberID uint64
		checkRangeBoundaryValues[memberID](t, 0, math.MaxUint64, math.MaxUint64/2)
	})
}

func TestRangeWideAndNarrowBoundaries(t *testing.T) {
	t.Run("signed wide interval", func(t *testing.T) {
		checkRangeBoundaryValues[int64](t, math.MinInt64, 0, math.MinInt64/2)
	})
	t.Run("unsigned interval wider than max int64", func(t *testing.T) {
		checkRangeBoundaryValues[uint64](t, 0, uint64(1)<<63, uint64(1)<<62)
	})
	t.Run("near minimum int64", func(t *testing.T) {
		checkRangeBoundaryValues[int64](t, math.MinInt64, math.MinInt64+100, math.MinInt64+50)
	})
	t.Run("near maximum int64", func(t *testing.T) {
		checkRangeBoundaryValues[int64](t, math.MaxInt64-100, math.MaxInt64, math.MaxInt64-50)
	})
	t.Run("near maximum uint64", func(t *testing.T) {
		checkRangeBoundaryValues[uint64](t, math.MaxUint64-100, math.MaxUint64, math.MaxUint64-50)
	})
	t.Run("single minimum int64", func(t *testing.T) {
		checkRangeBoundaryValues[int64](t, math.MinInt64, math.MinInt64, math.MinInt64)
	})
	t.Run("single maximum int64", func(t *testing.T) {
		checkRangeBoundaryValues[int64](t, math.MaxInt64, math.MaxInt64, math.MaxInt64)
	})
	t.Run("single maximum uint64", func(t *testing.T) {
		checkRangeBoundaryValues[uint64](t, math.MaxUint64, math.MaxUint64, math.MaxUint64)
	})
	t.Run("zero value", func(t *testing.T) {
		var r Range[uint64]
		assert.True(t, r.IsZero())
		checkRangeBoundaryValues[uint64](t, 0, 0, 0)
	})
}

func TestRangeMidAll8BitIntervals(t *testing.T) {
	t.Run("int8", func(t *testing.T) {
		for min := math.MinInt8; min <= math.MaxInt8; min++ {
			for max := min; max <= math.MaxInt8; max++ {
				r := NewRange(int8(max), int8(min))
				want := min + (max-min)/2 // Wider int arithmetic cannot overflow here.
				if got := int(r.Mid()); got != want {
					t.Fatalf("[%d, %d].Mid() = %d, want %d", min, max, got, want)
				}
			}
		}
	})
	t.Run("uint8", func(t *testing.T) {
		for min := 0; min <= math.MaxUint8; min++ {
			for max := min; max <= math.MaxUint8; max++ {
				r := NewRange(uint8(max), uint8(min))
				want := min + (max-min)/2
				if got := int(r.Mid()); got != want {
					t.Fatalf("[%d, %d].Mid() = %d, want %d", min, max, got, want)
				}
			}
		}
	})
}

func TestRangeRelationsSmallIntervals(t *testing.T) {
	var intervals []Range[int]
	for min := -3; min <= 3; min++ {
		for max := min; max <= 3; max++ {
			intervals = append(intervals, NewRange(min, max))
		}
	}
	for _, outer := range intervals {
		for value := -4; value <= 4; value++ {
			want := value >= outer.Min && value <= outer.Max
			if outer.In(value) != want {
				t.Fatalf("%v.In(%d) != %t", outer, value, want)
			}
		}
		for _, inner := range intervals {
			contains, overlaps := true, false
			for value := inner.Min; value <= inner.Max; value++ {
				inside := outer.In(value)
				contains = contains && inside
				overlaps = overlaps || inside
			}
			if outer.Contains(inner.Min, inner.Max) != contains {
				t.Fatalf("%v.Contains(%d, %d) != %t", outer, inner.Min, inner.Max, contains)
			}
			if outer.Overlaps(inner) != overlaps || inner.Overlaps(outer) != overlaps {
				t.Fatalf("overlap between %v and %v should be %t", outer, inner, overlaps)
			}
		}
	}
}

func TestRangeRandInvalidBoundaries(t *testing.T) {
	t.Run("negative signed bounds", func(t *testing.T) {
		r := Range[int64]{Min: -1, Max: math.MinInt64}
		assert.Panics(t, func() { r.Rand() })
	})
	t.Run("unsigned bounds", func(t *testing.T) {
		r := Range[uint64]{Min: math.MaxUint64, Max: 0}
		assert.Panics(t, func() { r.Rand() })
	})
}
