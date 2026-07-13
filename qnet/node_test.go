// Copyright © 2021 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package qnet

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMakeNode(t *testing.T) {
	tests := []struct {
		service  uint32
		instance uint32
	}{
		{0, 0},
		{1, 1},
		{math.MaxUint8, math.MaxInt16},
		{math.MaxUint8, 41943038},
		{1023, 4194303},
	}
	for _, tt := range tests {
		var node = MakeNode(tt.service, tt.instance)
		assert.Equal(t, tt.service, node.Service())
		assert.Equal(t, tt.instance, node.Instance())
	}
}

func TestMakeNodeFrom(t *testing.T) {
	tests := []struct {
		input        string
		wantService  uint32
		wantInstance uint32
	}{
		{"", 0, 0},
		{"0x0501", 5, 1},
		{"0xffff", 0xff, 0xff},
	}
	for _, tt := range tests {
		var node = MakeNodeFrom(tt.input)
		assert.Equal(t, tt.wantService, node.Service())
		assert.Equal(t, tt.wantInstance, node.Instance())
	}
}

func TestNodeID_String(t *testing.T) {
	tests := []struct {
		service  uint32
		instance uint32
		expected string
	}{
		{1, 1, "GATE-1"},
		{math.MaxUint8, 0x3fffff, "255-4194303"},
	}
	for _, tt := range tests {
		var node = MakeNode(tt.service, tt.instance)
		assert.Equal(t, node.String(), tt.expected)
	}
}
