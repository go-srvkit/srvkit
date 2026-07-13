// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package qnet

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMsgFlag_Has(t *testing.T) {
	var flag MsgFlag
	assert.False(t, flag.Has(MsgFlagCompress))
	assert.False(t, flag.Has(MsgFlagEncrypt))
	flag |= MsgFlagCompress
	assert.True(t, flag.Has(MsgFlagCompress))
	assert.False(t, flag.Has(MsgFlagEncrypt))
	flag |= MsgFlagEncrypt
	assert.True(t, flag.Has(MsgFlagEncrypt))
}

func TestMsgFlag_Clear(t *testing.T) {
	var flag = MsgFlagCompress | MsgFlagEncrypt
	assert.True(t, flag.Has(MsgFlagCompress))
	assert.True(t, flag.Has(MsgFlagEncrypt))
	flag = flag.Clear(MsgFlagEncrypt)
	assert.True(t, flag.Has(MsgFlagCompress))
	assert.False(t, flag.Has(MsgFlagEncrypt))
	flag = flag.Clear(MsgFlagCompress)
	assert.False(t, flag.Has(MsgFlagCompress))
}

func TestReadLenData(t *testing.T) {
	tests := []struct {
		data   []byte
		expect []byte
		err    bool
	}{
		{[]byte{0x12}, nil, true},
		{[]byte{0x0, 0x0}, nil, true},
		{[]byte{0x12, 0x34}, nil, true},
		{[]byte{0xFF, 0xFF}, nil, true},
		{[]byte{0x0, 0x2}, nil, false},
		{[]byte{0x0, 0x6, 0x1, 0x2, 0x3, 0x4}, []byte{0x1, 0x2, 0x3, 0x4}, false},
	}
	for _, tc := range tests {
		out, err := ReadLenData(bytes.NewReader(tc.data), 0x0FFF)
		if tc.err {
			assert.NotNil(t, err)
		} else {
			assert.Nil(t, err)
			assert.True(t, bytes.Equal(out, tc.expect))
		}
	}
}

func TestWriteLenData(t *testing.T) {
	tests := []struct {
		data   []byte
		expect []byte
		err    bool
	}{
		{nil, nil, false},
		{[]byte{0x1}, []byte{0x0, 0x3, 0x1}, false},
		{[]byte{0x1, 0x2, 0x3}, []byte{0x0, 0x5, 0x1, 0x2, 0x3}, false},
	}
	for _, tc := range tests {
		var buf bytes.Buffer
		err := WriteLenData(&buf, tc.data)
		var out = buf.Bytes()
		if tc.err {
			assert.NotNil(t, err)
		} else {
			assert.Nil(t, err)
			assert.True(t, bytes.Equal(out, tc.expect))
		}
	}
}

func TestReadProtoMessage(t *testing.T) {
	local, remote := createTestPipedConn(t)
	defer func() {
		local.Close()
		remote.Close()
	}()

	var req = &BuildReq{PosX: 1234, PosY: 5678}
	assert.Nil(t, WriteProtoMessage(local, req))

	var req2 BuildReq
	assert.Nil(t, ReadProtoMessage(remote, &req2))

	assert.True(t, reflect.DeepEqual(req, &req2))
}
