// Copyright © 2023 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package types

import (
	"cmp"
	"math/rand/v2"
)

// Range contains a min value and a max value.
type Range[T Integer] struct {
	Min T
	Max T
}

// NewRange creates a Range from two values, ensuring Min <= Max.
func NewRange[T Integer](a, b T) Range[T] {
	if cmp.Less(a, b) {
		return Range[T]{Min: a, Max: b}
	}
	return Range[T]{Min: b, Max: a}
}

// In returns true if the value is in the range [Min, Max].
func (r Range[T]) In(val T) bool {
	return r.Min <= val && val <= r.Max
}

// Contains returns true if the range [a, b] is fully contained in [Min, Max].
func (r Range[T]) Contains(a, b T) bool {
	return a >= r.Min && b <= r.Max
}

// Overlaps returns true if two ranges overlap.
func (r Range[T]) Overlaps(other Range[T]) bool {
	return !cmp.Less(r.Max, other.Min) && !cmp.Less(other.Max, r.Min)
}

// IsZero returns true if the range is zero.
func (r Range[T]) IsZero() bool {
	return r.Min == 0 && r.Max == 0
}

// Mid returns the middle value of the range.
func (r Range[T]) Mid() T {
	return r.Min + T((uint64(r.Max)-uint64(r.Min))/2)
}

// Rand returns a random value in the range [Min, Max].
func (r Range[T]) Rand() T {
	if r.Min == r.Max {
		return r.Min
	}
	if r.Max < r.Min {
		panic("Range.Rand: Min > Max")
	}
	var width = uint64(r.Max) - uint64(r.Min) + 1
	var offset uint64
	if width == 0 {
		offset = rand.Uint64() // 恰好覆盖 2^64 个值
	} else {
		offset = rand.Uint64N(width)
	}
	return T(uint64(r.Min) + offset)
}
