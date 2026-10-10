// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package gutil

import (
	"encoding/json"
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/typepb"
	"gopkg.in/srvkit.v1/protos"
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
	cycle := make(map[string]any)
	cycle["self"] = cycle

	tests := []struct {
		name    string
		input   any
		want    string
		wantErr bool
	}{
		{"integer", 1, "1", false},
		{"boolean", true, "true", false},
		{"zero", 0, "0", false},
		{"max int64", int64(math.MaxInt64), "9223372036854775807", false},
		{"struct", image.Point{X: 12, Y: 34}, `{"X":12,"Y":34}`, false},
		{"map", map[string]int{"X": 12, "Y": 34}, `{"X":12,"Y":34}`, false},
		{"large unsigned integers", []uint64{math.MaxUint32, math.MaxUint64}, `[4294967295,18446744073709551615]`, false},
		{"nil", nil, "null", false},
		{"nil pointer", (*image.Point)(nil), "null", false},
		{"nil slice", []int(nil), "null", false},
		{"empty slice", []int{}, "[]", false},
		{"nil map", map[string]int(nil), "null", false},
		{"empty map", map[string]int{}, "{}", false},
		{"Unicode", "你好😀", `"你好😀"`, false},
		{"escaped string", "line\n\"quote\"\\", `"line\n\"quote\"\\"`, false},
		{"bytes", []byte{0, 1, 255}, `"AAH/"`, false},
		{"channel", make(chan int), "", true},
		{"function", func() {}, "", true},
		{"NaN", math.NaN(), "", true},
		{"positive infinity", math.Inf(1), "", true},
		{"negative infinity", math.Inf(-1), "", true},
		{"invalid raw JSON", json.RawMessage(`{"a":`), "", true},
		{"cycle", cycle, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := JSONStringify(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("JSONStringify() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("JSONStringify() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestJSONMustStringify(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  string
	}{
		{"nil", nil, "null"},
		{"boolean", false, "false"},
		{"max uint64", uint64(math.MaxUint64), "18446744073709551615"},
		{"Unicode", "你好😀", `"你好😀"`},
		{"struct", image.Point{X: 12, Y: 34}, `{"X":12,"Y":34}`},
		{"empty slice", []int{}, "[]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := JSONMustStringify(tt.input); got != tt.want {
				t.Errorf("JSONMustStringify() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestJSONMustStringifyPanics(t *testing.T) {
	cycle := make(map[string]any)
	cycle["self"] = cycle

	tests := []struct {
		name  string
		input any
	}{
		{"channel", make(chan int)},
		{"function", func() {}},
		{"NaN", math.NaN()},
		{"infinity", math.Inf(1)},
		{"invalid raw JSON", json.RawMessage(`{"a":`)},
		{"cycle", cycle},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Panics(t, func() { JSONMustStringify(tt.input) })
		})
	}
}

func TestJSONStringifyOf(t *testing.T) {
	tests := []struct {
		name    string
		msg     proto.Message
		want    string
		wantErr bool
	}{
		{
			name: "default fields",
			msg:  &protos.ClientHello{},
			want: `{"ProtocolVersion":0,"Extension":0,"Timestamp":"0","ClientNonce":null,"ClientPubKey":null,"ClientRandom":"0","SupportedCipherSuites":[]}`,
		},
		{
			name: "populated fields",
			msg: &protos.ClientHello{
				ProtocolVersion:       1,
				Extension:             2,
				Timestamp:             math.MaxInt64,
				ClientNonce:           []byte{0, 1, 255},
				ClientPubKey:          []byte{2, 3},
				ClientRandom:          math.MaxUint64,
				SupportedCipherSuites: []int32{1, 2},
			},
			want: `{"ProtocolVersion":1,"Extension":2,"Timestamp":"9223372036854775807","ClientNonce":"AAH/","ClientPubKey":"AgM=","ClientRandom":"18446744073709551615","SupportedCipherSuites":[1,2]}`,
		},
		{
			name: "enum as integer",
			msg:  &typepb.Type{Name: "测试😀", Syntax: typepb.Syntax_SYNTAX_PROTO3},
			want: `{"name":"测试😀","fields":[],"oneofs":[],"options":[],"sourceContext":null,"syntax":1,"edition":""}`,
		},
		{name: "nil message", wantErr: true},
		{name: "typed nil message", msg: (*protos.ClientHello)(nil), wantErr: true},
		{name: "missing required fields", msg: &descriptorpb.UninterpretedOption_NamePart{}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := JSONStringifyOf(tt.msg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("JSONStringifyOf() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				if got != "" {
					t.Errorf("JSONStringifyOf() = %q on error, want empty string", got)
				}
				return
			}
			assert.JSONEq(t, tt.want, got)
		})
	}
}

func TestIsFileNotExist(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "existing.txt")
	if err := os.WriteFile(file, []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		filename string
		want     bool
	}{
		{"existing file", file, false},
		{"existing directory", dir, false},
		{"missing file", filepath.Join(dir, "missing.txt"), true},
		{"missing parent directory", filepath.Join(dir, "missing", "file.txt"), true},
		{"empty path", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsFileNotExist(tt.filename); got != tt.want {
				t.Errorf("IsFileNotExist(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

func TestIsFileNotExistSymlink(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "existing.txt")
	if err := os.WriteFile(file, []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		target string
	}{
		{"file", file},
		{"directory", dir},
		{"dangling", filepath.Join(dir, "missing.txt")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			link := filepath.Join(dir, tt.name+".link")
			if err := os.Symlink(tt.target, link); err != nil {
				t.Skipf("symbolic link creation is unavailable: %v", err)
			}
			// Lstat checks the link itself, including when its target is missing.
			if IsFileNotExist(link) {
				t.Errorf("IsFileNotExist(%q) = true, want false", link)
			}
		})
	}
}

func TestIsFileNotExistOtherError(t *testing.T) {
	// A NUL byte produces a path error without relying on filesystem permissions.
	filename := filepath.Join(t.TempDir(), "invalid\x00name")
	_, err := os.Lstat(filename)
	if err == nil || os.IsNotExist(err) {
		t.Fatalf("Lstat(%q) error = %v, want an error other than not-exist", filename, err)
	}
	if IsFileNotExist(filename) {
		t.Errorf("IsFileNotExist(%q) = true, want false for a different error", filename)
	}
}
