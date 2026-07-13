// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package conv

import (
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatAny(t *testing.T) {
	assert.Equal(t, "123", FormatAny("123"))
	assert.Equal(t, "true", FormatAny(true))
	assert.Equal(t, "false", FormatAny(false))
	assert.Equal(t, "0", FormatAny(0))
	assert.Equal(t, "1", FormatAny(1))
	assert.Equal(t, "2147483647", FormatAny(2147483647))
	assert.Equal(t, "-2147483648", FormatAny(-2147483648))
	assert.Equal(t, "127", FormatAny(int8(127)))
	assert.Equal(t, "-128", FormatAny(int8(-128)))
	assert.Equal(t, "32767", FormatAny(int16(32767)))
	assert.Equal(t, "-32768", FormatAny(int16(-32768)))
	assert.Equal(t, "9223372036854775807", FormatAny(int64(9223372036854775807)))
	assert.Equal(t, "-9223372036854775808", FormatAny(int64(-9223372036854775808)))
	assert.Equal(t, "2147483647", FormatAny(int32(2147483647)))
	assert.Equal(t, "-2147483648", FormatAny(int32(-2147483648)))
	assert.Equal(t, "0", FormatAny(uint8(0)))
	assert.Equal(t, "255", FormatAny(uint8(255)))
	assert.Equal(t, "0", FormatAny(uint16(0)))
	assert.Equal(t, "65535", FormatAny(uint16(65535)))
	assert.Equal(t, "0", FormatAny(uint32(0)))
	assert.Equal(t, "4294967295", FormatAny(uint32(4294967295)))
	assert.Equal(t, "0", FormatAny(uint64(0)))
	assert.Equal(t, "18446744073709551615", FormatAny(uint64(18446744073709551615)))
	assert.Equal(t, "78187493520", FormatAny(uintptr(0x1234567890)))
	assert.Equal(t, "3.14", FormatAny(float32(3.14)))
	assert.Equal(t, "3.14159", FormatAny(3.14159))
}

func TestParseSlice(t *testing.T) {
	const sep = "|"
	assert.Equal(t, len(ParseSlice[int]("", sep)), 0)
	assert.Equal(t, len(ParseSlice[int](sep, sep)), 0)
	assert.True(t, slices.Equal([]int{1}, ParseSlice[int]("1", sep)))
	assert.True(t, slices.Equal([]int{1}, ParseSlice[int]("1||", sep)))
	assert.True(t, slices.Equal([]int{1}, ParseSlice[int]("||1||", sep)))
	assert.True(t, slices.Equal([]int{1, 2, 3}, ParseSlice[int]("|1|2|3|", sep)))

	assert.True(t, slices.Equal([]string{"usr", "local", "bin"}, ParseSlice[string]("/usr/local/bin", "/")))
}

func TestParseMap(t *testing.T) {
	tests := []struct {
		input    string
		expected map[int]int
	}{
		{"", map[int]int{}},
		{"1:2", map[int]int{1: 2}},
		{"|1:2|", map[int]int{1: 2}},
		{"||1:2||||", map[int]int{1: 2}},
		{"1:2|3:4", map[int]int{1: 2, 3: 4}},
		{"  1 : 2 | 3 : 4|  ", map[int]int{1: 2, 3: 4}},
	}
	for i, tc := range tests {
		var out = ParseMap[int, int](tc.input, "|", ":")
		if !reflect.DeepEqual(out, tc.expected) {
			t.Fatalf("unexpected ParseMap(%s) case %d result: %v != %v", tc.input, i+1, out, tc.expected)
		}
	}
}

func TestFormatIntSlice(t *testing.T) {
	var tests = []struct {
		input    []int32
		delim    string
		expected string
	}{
		{nil, ",", ""},
		{nil, "", ""},
		{[]int32{1}, "", "1"},
		{[]int32{1}, ",", "1"},
		{[]int32{1, 2}, "|", "1|2"},
		{[]int32{1, 2, 3}, ",", "1,2,3"},
	}
	for i, tc := range tests {
		output := FormatSlice(tc.input, tc.delim)
		if tc.expected != output {
			t.Fatalf("Test case %d failed, expect %s, got %s", i+1, tc.expected, output)
		}
	}
}

func TestFormatMap(t *testing.T) {
	assert.Equal(t, "1:10|2:20|3:30", FormatMap(map[int]int{2: 20, 1: 10, 3: 30}))
	assert.Equal(t, "1:10|2:20|3:30", FormatMap(map[int8]int8{2: 20, 1: 10, 3: 30}))
	assert.Equal(t, "1:10|2:20|3:30", FormatMap(map[int16]int16{2: 20, 1: 10, 3: 30}))
	assert.Equal(t, "1:10|2:20|3:30", FormatMap(map[int32]int32{2: 20, 1: 10, 3: 30}))
	assert.Equal(t, "1:10|2:20|3:30", FormatMap(map[int64]int64{2: 20, 1: 10, 3: 30}))
	assert.Equal(t, "1:10|2:20|3:30", FormatMap(map[uint]uint{2: 20, 1: 10, 3: 30}))
	assert.Equal(t, "1:10|2:20|3:30", FormatMap(map[uint8]uint8{2: 20, 1: 10, 3: 30}))
	assert.Equal(t, "1:10|2:20|3:30", FormatMap(map[uint16]uint16{2: 20, 1: 10, 3: 30}))
	assert.Equal(t, "1:10|2:20|3:30", FormatMap(map[uint32]uint32{2: 20, 1: 10, 3: 30}))
	assert.Equal(t, "1:10|2:20|3:30", FormatMap(map[uint64]uint64{2: 20, 1: 10, 3: 30}))
}

func TestPrettyBytes(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{0, "0B"},
		{KiB, "1KiB"},
		{-KiB, "-1KiB"},
		{KiB + 100, "1.1KiB"},
		{MiB, "1MiB"},
		{MiB + 10*KiB, "1.01MiB"},
		{GiB, "1GiB"},
		{GiB + 100*MiB, "1.098GiB"},
	}
	for i, tt := range tests {
		var name = fmt.Sprintf("case-%d", i+1)
		t.Run(name, func(t *testing.T) {
			if got := PrettyBytes(tt.input); got != tt.want {
				t.Errorf("PrettyBytes() = %v, want %v", got, tt.want)
			}
		})
	}
}
