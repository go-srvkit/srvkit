// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package conv

import (
	"encoding/json"
	"fmt"
	"strconv"
	"unsafe"

	"gopkg.in/srvkit.v1/collections/types"
)

// StrAsBytes returns the bytes backing a string, it is the caller's responsibility not to mutate the bytes returned.
// see https://pkg.go.dev/unsafe#Pointer rule(6)
func StrAsBytes(s string) []byte {
	if len(s) == 0 {
		return nil
	}
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

// BytesAsStr returns the string view of byte slice
func BytesAsStr(b []byte) string {
	return unsafe.String(unsafe.SliceData(b), len(b))
}

// BoolTo converts a bool to integer
func BoolTo[T types.Integer](b bool) T {
	if b {
		return 1
	}
	return 0
}

// IntToBool converts an integer to bool
func IntToBool[T types.Integer](v T) bool {
	return v != 0
}

// ToInt 转换为整数
func ToInt[T types.Integer](val any) T {
	switch v := val.(type) {
	case int8:
		return T(v)
	case uint8:
		return T(v)
	case int16:
		return T(v)
	case uint16:
		return T(v)
	case int:
		return T(v)
	case uint:
		return T(v)
	case int32:
		return T(v)
	case uint32:
		return T(v)
	case int64:
		return T(v)
	case uint64:
		return T(v)
	case float32:
		return T(v)
	case float64:
		return T(v)
	case bool:
		if v {
			return T(1)
		}
		return T(0)
	case string:
		if n, err := strconv.ParseInt(v, 10, 64); err != nil {
			return 0
		} else {
			return T(n)
		}
	case json.Number:
		if n, err := strconv.ParseInt(string(v), 10, 64); err != nil {
			return 0
		} else {
			return T(n)
		}
	}
	panic(fmt.Sprintf("ToInt: unexpected type %T", val))
	return 0
}
