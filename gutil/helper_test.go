// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package gutil

import (
	"fmt"
	"image"
	"math"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestZeroOf(t *testing.T) {
	assert.Equal(t, false, ZeroOf[bool]())
	assert.Equal(t, "", ZeroOf[string]())
	assert.Equal(t, 0, ZeroOf[int]())
	assert.Equal(t, uint(0), ZeroOf[uint]())
	assert.Equal(t, int8(0), ZeroOf[int8]())
	assert.Equal(t, uint8(0), ZeroOf[uint8]())
	assert.Equal(t, int16(0), ZeroOf[int16]())
	assert.Equal(t, uint16(0), ZeroOf[uint16]())
	assert.Equal(t, int32(0), ZeroOf[int32]())
	assert.Equal(t, uint32(0), ZeroOf[uint32]())
	assert.Equal(t, float32(0), ZeroOf[float32]())
	assert.Equal(t, float64(0), ZeroOf[float64]())
	assert.Equal(t, complex64(0), ZeroOf[complex64]())
	assert.Equal(t, image.Point{}, ZeroOf[image.Point]())
	assert.Equal(t, []int(nil), ZeroOf[[]int]())
	assert.Equal(t, map[int]int(nil), ZeroOf[map[int]int]())
}

func TestJSONParse(t *testing.T) {
	tests := []struct {
		input  string
		want   interface{}
		hasErr bool
	}{
		{"xx", 0, true},
		{"1", 1, false},
		{"true", true, false},
		{"127", int8(127), false},
		{"65535", uint16(65535), false},
		{"2147483647", int32(math.MaxInt32), false},
		{"9223372036854775807", int64(math.MaxInt64), false},
		{"3.14", float32(3.14), false},
		{`{"X":12, "Y":34}`, image.Point{X: 12, Y: 34}, false},
		{`{"X":12,"Y":34}`, map[string]int{"X": 12, "Y": 34}, false},
		{"[4294967295,18446744073709551615]", []uint64{math.MaxUint32, math.MaxUint64}, false},
	}
	for i, tt := range tests {
		var name = fmt.Sprintf("case-%d", i+1)
		t.Run(name, func(t *testing.T) {
			var rval = reflect.New(reflect.TypeOf(tt.want))
			var err = JSONParse(tt.input, rval.Interface())
			if err != nil {
				if tt.hasErr {
					return
				}
				t.Fatalf("JSONParse() error = %v, wantErr %v", err, tt.hasErr)
			}
			var got = rval.Elem().Interface()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("PrettyBytes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestJSONStringify(t *testing.T) {
	tests := []struct {
		input interface{}
		want  string
	}{
		{1, "1"},
		{true, "true"},
		{math.MaxInt64, "9223372036854775807"},
		{image.Point{X: 12, Y: 34}, `{"X":12,"Y":34}`},
		{map[string]int{"X": 12, "Y": 34}, `{"X":12,"Y":34}`},
		{[]uint64{math.MaxUint32, math.MaxUint64}, `[4294967295,18446744073709551615]`},
	}
	for i, tt := range tests {
		var name = fmt.Sprintf("case-%d", i+1)
		t.Run(name, func(t *testing.T) {
			if got := JSONStringify(tt.input); got != tt.want {
				t.Errorf("case %d JSONStringify: %v, want %v", i+1, got, tt.want)
			}
		})
	}
}
