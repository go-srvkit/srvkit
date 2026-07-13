// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package secure

import (
	"crypto/pbkdf2"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"unsafe"
)

const RandomStringChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

const (
	DefaultAlgorithm = "pbkdf2_sha256"
	DefaultIteration = 10_000
	DefaultSaltBits  = 128
)

func GetRandomString(length int) string {
	var buf = make([]byte, length)
	for i := 0; i < length; i++ {
		idx := rand.Int() % len(RandomStringChars)
		buf[i] = RandomStringChars[idx]
	}
	return unsafe.String(unsafe.SliceData(buf), len(buf))
}

func EncodePassword(rawPasswd string, salt []byte, iteration int) string {
	dk, err := pbkdf2.Key(sha256.New, rawPasswd, salt, iteration, 32)
	if err != nil {
		panic(err)
	}
	var hash = base64.StdEncoding.EncodeToString(dk)
	return fmt.Sprintf("%s$%d$%s$%s", DefaultAlgorithm, iteration, salt, hash)
}

// MakePassword Turn a plain-text password into a hash for database storage
func MakePassword(rawPasswd string) string {
	if rawPasswd == "" {
		return ""
	}
	var charCount = math.Ceil(DefaultSaltBits / math.Log2(float64(len(RandomStringChars))))
	var saltString = GetRandomString(int(charCount))
	return EncodePassword(rawPasswd, []byte(saltString), DefaultIteration)
}

// VerifyPassword Check if the given password is correct.
func VerifyPassword(passwd, inputEncoded string) bool {
	var parts = strings.Split(inputEncoded, "$")
	if len(parts) != 4 {
		return false
	}
	if algorithm := parts[0]; algorithm != DefaultAlgorithm {
		return false
	}
	iterations, _ := strconv.Atoi(parts[1])
	if iterations <= 0 {
		return false
	}
	salt := parts[2]
	var encoded = EncodePassword(passwd, []byte(salt), iterations)
	return encoded == inputEncoded
}
