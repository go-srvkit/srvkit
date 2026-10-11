// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package types

import (
	"cmp"
	"fmt"
)

// Pair is a type that provides a way to store two heterogeneous objects as a single unit.
type Pair[T1, T2 any] struct {
	First  T1
	Second T2
}

// MakePair creates a Pair object, deducing the target type from the types of arguments
func MakePair[T1, T2 any](a T1, b T2) Pair[T1, T2] {
	return Pair[T1, T2]{
		First:  a,
		Second: b,
	}
}

// Compare compares two pairs lexicographically (first by First, then by Second).
// Both T1 and T2 must be ordered types.
// Returns a negative integer, zero, or a positive integer as p is less than, equal to, or greater than other.
func Compare[T1 cmp.Ordered, T2 cmp.Ordered](a, b Pair[T1, T2]) int {
	if c := cmp.Compare(a.First, b.First); c == 0 {
		return cmp.Compare(a.Second, b.Second)
	} else {
		return c
	}
}

// String returns a string representation of the pair.
func (p Pair[T1, T2]) String() string {
	return fmt.Sprintf("(%v, %v)", p.First, p.Second)
}

// Swap exchanges the values of two pairs.
func (p *Pair[T1, T2]) Swap(other *Pair[T1, T2]) {
	p.First, other.First = other.First, p.First
	p.Second, other.Second = other.Second, p.Second
}

// EqualPair returns true if both pairs have the same values.
func EqualPair[T1 comparable, T2 comparable](p, other Pair[T1, T2]) bool {
	return p.First == other.First && p.Second == other.Second
}
