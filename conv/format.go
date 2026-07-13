// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package conv

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/srvkit.v1/collections/mapx"
	"gopkg.in/srvkit.v1/collections/types"
)

const (
	KiB = 1 << 10
	MiB = 1 << 20
	GiB = 1 << 30
	TiB = 1 << 40
)

// FormatAny 格式化为字符串
func FormatAny(v any) string {
	switch val := v.(type) {
	case bool:
		return strconv.FormatBool(val)
	case int:
		return strconv.Itoa(val)
	case int8:
		return strconv.FormatInt(int64(val), 10)
	case int16:
		return strconv.FormatInt(int64(val), 10)
	case int32:
		return strconv.FormatInt(int64(val), 10)
	case int64:
		return strconv.FormatInt(val, 10)
	case uint:
		return strconv.FormatUint(uint64(val), 10)
	case uint8:
		return strconv.FormatUint(uint64(val), 10)
	case uint16:
		return strconv.FormatUint(uint64(val), 10)
	case uint32:
		return strconv.FormatUint(uint64(val), 10)
	case uint64:
		return strconv.FormatUint(val, 10)
	case uintptr:
		return strconv.FormatUint(uint64(val), 10)
	case float32:
		return strconv.FormatFloat(float64(val), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case string:
		return val
	}
	if iface, ok := v.(fmt.Stringer); ok {
		return iface.String()
	}
	return fmt.Sprintf("%v", v)
}

// FormatSlice 格式化slice为字符串
func FormatSlice[E any](a []E, delim string) string {
	var sb strings.Builder
	for i := 0; i < len(a); i++ {
		fmt.Fprintf(&sb, "%v", a[i])
		if i+1 < len(a) {
			sb.WriteString(delim)
		}
	}
	return sb.String()
}

// FormatMap 格式化数值kv类型的map为字符串
func FormatMap[M ~map[K]V, K, V types.Number](m M) string {
	var sb strings.Builder
	var keys = mapx.MapKeys(m)
	slices.Sort(keys)
	for i, k := range keys {
		var strKey = FormatAny(k)
		var strVal = FormatAny(m[k])
		sb.WriteString(strKey + ":" + strVal)
		if i+1 < len(keys) {
			sb.WriteByte('|')
		}
	}
	return sb.String()
}

func abs64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

// PrettyBytes 打印容量大小
func PrettyBytes(nbytes int64) string {
	if nbytes == 0 {
		return "0B"
	}
	var unit = 1
	var prec = 1
	var suffix string
	var absBytes = abs64(nbytes)
	switch {
	case absBytes < KiB:
		suffix = "B"
	case absBytes < MiB:
		unit = KiB
		suffix = "KiB"
	case absBytes < GiB:
		prec = 2
		unit = MiB
		suffix = "MiB"
	case absBytes < TiB:
		prec = 3
		unit = GiB
		suffix = "GiB"
	default:
		prec = 4
		unit = TiB
		suffix = "TiB"
	}
	var s = strconv.FormatFloat(float64(nbytes)/float64(unit), 'f', prec, 64)
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	return s + suffix
}
