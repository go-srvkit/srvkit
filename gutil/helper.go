// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package gutil

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"runtime"
	"strings"
	"unsafe"

	"gopkg.in/srvkit.v1/logger"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
)

// ZeroOf returns the zero value of the type T
func ZeroOf[T any]() T {
	var zero T
	return zero
}

// JSONParse 避免大数值被解析为float导致的精度丢失
func JSONParse(s string, v any) error {
	var dec = json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	if err := dec.Decode(v); err != nil {
		return err
	}
	return nil
}

// JSONParseTo 解析json字符串到proto消息
func JSONParseTo(s string, dst proto.Message) error {
	return jsonpb.Unmarshal(strings.NewReader(s), dst)
}

// JSONStringify 序列化为json字符串
func JSONStringify(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		logger.Errorf("JSONStringify %T: %v", v, err)
		return ""
	}
	return unsafe.String(unsafe.SliceData(data), len(data))
}

// JSONStringifyOf 序列化proto消息为json格式
func JSONStringifyOf(msg proto.Message) (string, error) {
	var buf bytes.Buffer
	var jm = jsonpb.Marshaler{
		EnumsAsInts:  true,
		EmitDefaults: true,
	}
	if err := jm.Marshal(&buf, msg); err != nil {
		return "", err
	}
	var data = buf.Bytes()
	return unsafe.String(unsafe.SliceData(data), len(data)), nil
}

func MD5Sum(data []byte) string {
	var hash = md5.New()
	hash.Write(data)
	return hex.EncodeToString(hash.Sum(nil))
}

func SHA1Sum(data []byte) string {
	var hash = sha1.New()
	hash.Write(data)
	return hex.EncodeToString(hash.Sum(nil))
}

func SHA256Sum(data []byte) string {
	var hash = sha256.New()
	hash.Write(data)
	return hex.EncodeToString(hash.Sum(nil))
}

// RuntimeMemBytes bytes of allocated heap objects
func RuntimeMemBytes() uint64 {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats) // here will STW
	return memStats.HeapAlloc
}

// IsFileExist 文件是否存在
func IsFileExist(filename string) bool {
	_, err := os.Lstat(filename)
	return !os.IsNotExist(err)
}
