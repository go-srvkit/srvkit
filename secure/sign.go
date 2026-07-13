// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package secure

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// SignAccessToken 注册签名
func SignAccessToken(secretKey string, args ...string) string {
	mac := hmac.New(sha256.New, []byte(secretKey))
	for i := 0; i < len(args); i++ {
		mac.Write([]byte(args[i]))
	}
	return hex.EncodeToString(mac.Sum(nil))
}
