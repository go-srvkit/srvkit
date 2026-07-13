// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package qnet

import (
	"math/rand"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-=@#$%?"

func randString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = alphabet[rand.Intn(len(alphabet))]
	}
	return unsafe.String(unsafe.SliceData(b), len(b))
}

func createTestRandListener(t *testing.T) (net.Listener, string) {
	var err error
	var addr string
	var ln net.Listener
	for i := 0; i < 10; i++ {
		addr = "127.0.0.1:" + strconv.Itoa(24678+i)
		if ln, err = net.Listen("tcp", addr); err != nil {
			time.Sleep(time.Millisecond * 100)
			continue
		}
		break
	}
	require.NotNil(t, ln)
	return ln, addr
}

func createTestPipedConn(t *testing.T) (local, remote net.Conn) {
	var bus = make(chan net.Conn, 1)
	ln, addr := createTestRandListener(t)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				break
			}
			bus <- conn
		}
	}()

	var err error
	local, err = net.Dial("tcp", addr)
	require.Nil(t, err)
	require.NotNil(t, ln)

	remote = <-bus
	assert.NotNil(t, remote)
	return
}
func TestGetLocalIPList(t *testing.T) {
	var list = GetLocalIPList()
	assert.True(t, len(list) > 0)
}

func TestCompress(t *testing.T) {
	tests := []struct {
		input  string
		repeat int
	}{
		{"hell world", 1},
		{"aaabbbcccdddeeefffggg", 8},
		{"a quick brown fox jumps over the lazy dog", 12},
		{"It was the best of times, it was the worst of times, it was the age of wisdom, it was the age of foolishness, it was the epoch of belief, it was the epoch of incredulity, it was the season of Light, it was the season of Darkness, it was the spring of hope, it was the winter of despair, we had everything before us, we had nothing before us, we were all going direct to Heaven, we were all going direct the other way—in short, the period was so far like the present period, that some of its noisiest authorities insisted on its being received, for good or for evil, in the superlative degree of comparison only.", 100},
	}
	for i, tc := range tests {
		var input = strings.Repeat(tc.input, tc.repeat)
		encoded, err := compress([]byte(input))
		if err != nil {
			t.Fatalf("compress: %v", err)
		}
		decoded, err := decompress(encoded, MaxS2SPacketSize)
		if err != nil {
			t.Fatalf("uncompress: %v", err)
		}
		var out = string(decoded)
		if out != input {
			t.Logf("case %d: compress %s -> %s", i+1, input, out)
		}
	}
}
