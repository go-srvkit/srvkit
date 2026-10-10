// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package gutil

import (
	"math/rand"
	"unicode"
	"unicode/utf8"
	"unsafe"
)

const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-=~!@#$%^&*()_+[]\\;',./{}|:<>?"

// RandString 随机长度的字符串
func RandString(length int) string {
	if length <= 0 {
		return ""
	}
	var buf = make([]byte, length)
	for i := 0; i < length; i++ {
		idx := rand.Int() % len(alphabet)
		buf[i] = alphabet[idx]
	}
	return unsafe.String(unsafe.SliceData(buf), len(buf))
}

// RandBytes 随机长度的字节数组
func RandBytes(length int) []byte {
	if length <= 0 {
		return nil
	}
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		ch := byte(rand.Intn(256))
		result[i] = ch
	}
	return result
}

// FindFirstDigit 查找第一个数字的位置
func FindFirstDigit(s string) int {
	for i, r := range s {
		if unicode.IsDigit(r) {
			return i
		}
	}
	return -1
}

// FindFirstNonDigit 查找第一个非数字的位置
func FindFirstNonDigit(s string) int {
	for i, r := range s {
		if !unicode.IsDigit(r) {
			return i
		}
	}
	return -1
}

// Reverse 反转字符串
func Reverse(str string) string {
	runes := []rune(str)
	l := len(runes)
	for i := 0; i < l/2; i++ {
		runes[i], runes[l-i-1] = runes[l-i-1], runes[i]
	}
	return string(runes)
}

// LongestCommonPrefix 按 Unicode 码点比较字符串，返回 s1 中的最长共同前缀。
func LongestCommonPrefix(s1, s2 string) string {
	j := 0
	for i, r1 := range s1 {
		if j >= len(s2) {
			return s1[:i]
		}
		r2, size := utf8.DecodeRuneInString(s2[j:])
		if r1 != r2 {
			return s1[:i]
		}
		j += size
	}
	return s1
}
