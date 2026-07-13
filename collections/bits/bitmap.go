// Copyright © 2023 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package bits

import (
	"math/bits"
	"strings"
)

const (
	bitsPerWord = 64 // 用uint64表示
	bitsPerByte = 8  // 用uint8表示
)

func BitWordsCount(bitSize int) int {
	return (bitSize + bitsPerWord - 1) / bitsPerWord
}

func BitBytesCount(bitSize int) int {
	return (bitSize + bitsPerByte - 1) / bitsPerByte
}

// TestBit checks if the bit at the given index which starting from LSB is set.
func TestBit(bm []uint64, i int) bool {
	if i >= 0 && i < len(bm)*bitsPerWord {
		return bm[i/bitsPerWord]&(1<<(i%bitsPerWord)) != 0
	}
	return false
}

// SetBit sets the bit at the given index to 1.
func SetBit(bm []uint64, i int) bool {
	if i >= 0 && i < len(bm)*bitsPerWord {
		var v = uint64(1) << (i % bitsPerWord)
		bm[i/bitsPerWord] |= v
		return true
	}
	return false
}

// MustSetBit 指定位是否为1，并且自动增长数组，注意使用返回值赋值原数组
func MustSetBit(bm []uint64, i int) []uint64 {
	var n = BitWordsCount(i + 1)
	if n > len(bm) {
		var newb = make([]uint64, n)
		copy(newb, bm)
		bm = newb
	}
	SetBit(bm, i)
	return bm
}

// ClearBit clears the bit at the given index.
func ClearBit(bm []uint64, i int) bool {
	if i >= 0 && i < len(bm)*bitsPerWord {
		var v = uint64(1) << (i % bitsPerWord)
		bm[i/bitsPerWord] &= ^v
		return true
	}
	return false
}

// FlipBit flips the bit at the given index.
func FlipBit(bm []uint64, i int) bool {
	if i >= 0 && i < len(bm)*bitsPerWord {
		bm[i/bitsPerWord] ^= uint64(1) << (i % bitsPerWord)
		return true
	}
	return false
}

// TestAndSetBit returns the old value of the bit at the given index and sets it to 1.
func TestAndSetBit(bm []uint64, i int) bool {
	if i < 0 || i >= len(bm)*bitsPerWord {
		return false
	}
	var v = uint64(1) << (i % bitsPerWord)
	var index = i / bitsPerWord
	var old = bm[index]
	bm[index] |= v
	return old&v != 0
}

// TestAndClearBit returns the old value of the bit at the given index and clears it.
func TestAndClearBit(bm []uint64, i int) bool {
	if i < 0 || i >= len(bm)*bitsPerWord {
		return false
	}
	var v = uint64(1) << (i % bitsPerWord)
	var index = i / bitsPerWord
	var old = bm[index]
	bm[index] &= ^v
	return old&v != 0
}

// OnesCount returns the number of bits set to 1.
func OnesCount(bm []uint64) int {
	var count int
	for i := 0; i < len(bm); i++ {
		if bm[i] != 0 {
			count += bits.OnesCount64(bm[i])
		}
	}
	return count
}

// TrailingZeros returns the number of trailing zero bits in the bitmap.
// Returns the total bit size if all bits are zero.
func TrailingZeros(bm []uint64) int {
	for i := 0; i < len(bm); i++ {
		if bm[i] != 0 {
			return i*bitsPerWord + bits.TrailingZeros64(bm[i])
		}
	}
	return len(bm) * bitsPerWord
}

// IsAllZero 是否所有位都是0
func IsAllZero(bm []uint64) bool {
	for i := 0; i < len(bm); i++ {
		if bm[i] != 0 {
			return false
		}
	}
	return true
}

// FindFirst returns the index of the lowest set bit, or -1 if no bit is set.
func FindFirst(bm []uint64) int {
	for i := 0; i < len(bm); i++ {
		if bm[i] != 0 {
			return i*bitsPerWord + bits.TrailingZeros64(bm[i])
		}
	}
	return -1
}

// Format returns a string representation of the bitmap from LSB to MSB.
func Format(bm []uint64) string {
	var size = len(bm) * bitsPerWord
	var sb strings.Builder
	sb.Grow(size)
	for i := 0; i < size; i++ {
		if TestBit(bm, i) {
			sb.WriteByte('1')
		} else {
			sb.WriteByte('0')
		}
	}
	return sb.String()
}

// FormatWidth 按指定宽度对齐
func FormatWidth(bm []uint64, width int) string {
	var size = len(bm) * bitsPerWord
	var sb strings.Builder
	var n = 0
	for i := 0; i < size; i++ {
		if n%width == 0 {
			sb.WriteByte('\n')
		}
		n++
		if TestBit(bm, i) {
			sb.WriteByte('1')
		} else {
			sb.WriteByte('0')
		}
	}
	return sb.String()
}
