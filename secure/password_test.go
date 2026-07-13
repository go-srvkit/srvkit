// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package secure

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var rawPasswd = "hell@w0rld"

func TestMakePassword(t *testing.T) {
	var encoded = MakePassword(rawPasswd)
	var ok = VerifyPassword(rawPasswd, encoded)
	assert.Equal(t, ok, true)
}

func TestVerifyPassword(t *testing.T) {
	var input = "pbkdf2_sha256$1000000$hLLPdKQ725OtvguB21znKG$iqGpdESnn6wRa+3qSB/jZyk9p6DRQSyeLRKjjgFP8Uk="
	var ok = VerifyPassword(rawPasswd, input)
	assert.Equal(t, ok, true)
}
