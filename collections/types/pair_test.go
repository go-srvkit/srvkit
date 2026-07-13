// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package types

import (
	"fmt"
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
