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
