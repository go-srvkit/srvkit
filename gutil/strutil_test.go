// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package gutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRandString(t *testing.T) {
	var s = RandString(12)
	assert.Equal(t, 12, len(s))
}

func TestRandBytes(t *testing.T) {
	tests := []struct {
		name   string
		length int
	}{
		{"negative", -1},
		{"zero", 0},
		{"single", 1},
		{"small", 16},
		{"large", 4096},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RandBytes(tt.length)
			if tt.length <= 0 {
				if got != nil {
					t.Fatalf("RandBytes(%d) = %v, want nil", tt.length, got)
				}
				return
			}
			if len(got) != tt.length {
				t.Fatalf("len(RandBytes(%d)) = %d, want %d", tt.length, len(got), tt.length)
			}
		})
	}
}

func TestRandBytesFullRange(t *testing.T) {
	// With uniform bytes, the probability of missing any value in this
	// sample is less than 1e-100. This catches the former exclusion of 0xff.
	var seen [256]bool
	for _, b := range RandBytes(65536) {
		seen[b] = true
	}
	for b, found := range seen {
		if !found {
			t.Errorf("RandBytes did not generate byte 0x%02x", b)
		}
	}
}

func TestRandBytesIndependentBuffers(t *testing.T) {
	first := RandBytes(16)
	second := RandBytes(16)
	for i := range first {
		want := second[i]
		first[i] = want ^ 0xff
		if second[i] != want {
			t.Fatalf("modifying the first buffer changed the second at index %d", i)
		}
	}
}

func TestFindFirstDigit(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"", -1},
		{"abc", -1},
		{"123", 0},
		{"abc123", 3},
	}
	for i, test := range tests {
		output := FindFirstDigit(test.input)
		if test.expected != output {
			t.Fatalf("Test case %d failed, expect %d, got %d", i, test.expected, output)
		}
	}
}

func TestFindFirstNonDigit(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"", -1},
		{"123", -1},
		{"abc", 0},
		{"123abc", 3},
	}
	for i, test := range tests {
		output := FindFirstNonDigit(test.input)
		if test.expected != output {
			t.Fatalf("Test case %d failed, expect %d, got %d", i, test.expected, output)
		}
	}
}

func TestReverse(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"abc", "cba"},
		{"a", "a"},
		{"çınar", "ranıç"},
		{"    yağmur", "rumğay    "},
		{"επαγγελματίες", "ςείταμλεγγαπε"},
	}

	for i, test := range tests {
		output := Reverse(test.input)
		if test.expected != output {
			t.Fatalf("Test case %d failed, expect %s, got %s", i, test.expected, output)
		}
	}
}

func TestLongestCommonPrefix(t *testing.T) {
	tests := []struct {
		name     string
		input1   string
		input2   string
		expected string
	}{
		{"both empty", "", "", ""},
		{"first empty", "", "a", ""},
		{"second empty", "a", "", ""},
		{"no common prefix", "ab", "cd", ""},
		{"ASCII prefix", "ab123", "abc456", "ab"},
		{"first is prefix", "ab", "abc", "ab"},
		{"second is prefix", "abc", "ab", "ab"},
		{"identical", "中国😀", "中国😀", "中国😀"},
		{"Chinese prefix", "中国", "中华", "中"},
		{"different Chinese runes", "中", "丰", ""},
		{"different accented runes", "éx", "êy", ""},
		{"different emoji", "😀a", "😁b", ""},
		{"emoji prefix", "😀😃", "😀😄", "😀"},
		{"mixed prefix", "Go😀中国", "Go😀中华", "Go😀中"},
		{"first Unicode prefix", "你😀", "你😀好", "你😀"},
		{"second Unicode prefix", "你😀好", "你😀", "你😀"},
		{"combining runes", "e\u0301a", "e\u0301b", "e\u0301"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := LongestCommonPrefix(tt.input1, tt.input2); got != tt.expected {
				t.Fatalf("LongestCommonPrefix(%q, %q) = %q, want %q", tt.input1, tt.input2, got, tt.expected)
			}
		})
	}
}
