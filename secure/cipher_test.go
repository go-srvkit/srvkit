// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package secure

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-=~!@#$%^&*()_+[]\\;',./{}|:<>?"

func randBytes(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = alphabet[rand.Intn(len(alphabet))]
	}
	return b
}

func Test_GenerateRandomKey(t *testing.T) {
	for _, n := range []int{0, 1, 15, 25, 33} {
		key, err := GenerateRandomKey(n)
		assert.Nil(t, err)
		assert.Len(t, key, n)
	}

	for _, n := range []int{16, 24, 32} {
		key, err := GenerateRandomKey(n)
		assert.Nil(t, err)
		assert.Len(t, key, n)
	}
}

func Test_ParseCipherName(t *testing.T) {
	tests := []struct {
		name      string
		want1     ModeType
		want2     int
		want3     int
		expectErr bool
	}{
		{"invalid-cipher", 0, 0, 0, true},
		{"aes-100-cfb", 0, 0, 0, true},
		{"aes-256-xyz", 0, 0, 0, true},
		{"aes--xyz", 0, 0, 0, true},
		{"aes-1024-cfb", 0, 0, 0, true},
		{"chacha20-poly1305", ModeChaCha20, 32, 12, false},
		{"aes-128-gcm", ModeAESGCM, 16, 12, false},
		{"aes-192-gcm", ModeAESGCM, 24, 12, false},
		{"aes-256-gcm", ModeAESGCM, 32, 12, false},
		{"aes-128-ctr", ModeAESCTR, 16, 16, false},
		{"aes-192-ctr", ModeAESCTR, 24, 16, false},
		{"aes-256-ctr", ModeAESCTR, 32, 16, false},
		{"aes-128-cbc", ModeAESCBC, 16, 16, false},
		{"aes-192-cbc", ModeAESCBC, 24, 16, false},
		{"aes-256-cbc", ModeAESCBC, 32, 16, false},
	}
	for _, tt := range tests {
		got1, got2, got3, err := ParseCipherName(tt.name)
		if tt.expectErr {
			assert.NotNilf(t, err, "ParseCipherName(%v)", tt.name)
			continue
		}
		assert.Equalf(t, tt.want1, got1, "ParseCipherName(%v)", tt.name)
		assert.Equalf(t, tt.want2, got2, "ParseCipherName(%v)", tt.name)
		assert.Equalf(t, tt.want3, got3, "ParseCipherName(%v)", tt.name)
	}
}

func testSimpleEncryption(t *testing.T, mode ModeType, keySize, nonceSize int, macVerify bool) {
	key, err := GenerateRandomKey(keySize)
	assert.Nil(t, err)
	assert.Len(t, key, keySize)

	var hmacKey []byte
	if macVerify {
		hmacKey, err = GenerateRandomKey(keySize)
		assert.Nil(t, err)
		assert.Len(t, hmacKey, keySize)
	}

	encryptor, err := NewCryptor(mode, key, hmacKey)
	assert.Nil(t, err)
	assert.NotNil(t, encryptor)

	decryptor, err := NewCryptor(mode, key, hmacKey)
	assert.Nil(t, err)
	assert.NotNil(t, decryptor)

	for i := 0; i < 10; i++ {
		nonce, err := GenerateRandomKey(nonceSize)
		assert.Nil(t, err)
		assert.Len(t, nonce, nonceSize)

		var size = 32 + rand.Intn(1024)
		var plainData = randBytes(size)

		encryptedData, err := encryptor.Encrypt(plainData, nonce)
		assert.Nilf(t, err, "mode=%v, keySize=%d", mode, keySize)
		assert.NotNil(t, encryptedData)

		decrypted, err := decryptor.Decrypt(encryptedData, nonce)
		assert.Nilf(t, err, "mode=%v, keySize=%d", mode, keySize)
		assert.NotNil(t, encryptedData)
		assert.True(t, bytes.Equal(plainData, decrypted))
	}
}

func Test_AES_CBC_Encrypt(t *testing.T) {
	testSimpleEncryption(t, ModeAESCBC, 16, 16, false)
	testSimpleEncryption(t, ModeAESCBC, 24, 16, true)
	testSimpleEncryption(t, ModeAESCBC, 32, 16, true)
}

func Test_AES_CTR_Encrypt(t *testing.T) {
	testSimpleEncryption(t, ModeAESCTR, 16, 16, false)
	testSimpleEncryption(t, ModeAESCTR, 24, 16, false)
	testSimpleEncryption(t, ModeAESCTR, 32, 16, false)
}

func Test_AES_GCM_Encrypt(t *testing.T) {
	testSimpleEncryption(t, ModeAESGCM, 16, 12, false)
	testSimpleEncryption(t, ModeAESGCM, 24, 12, false)
	testSimpleEncryption(t, ModeAESGCM, 32, 12, false)
}

func Test_AES_Chacha20_Encrypt(t *testing.T) {
	testSimpleEncryption(t, ModeChaCha20, 32, 12, true)
}

func benchAESCryptor(b *testing.B, cryptor Encrypter, nonceSize int) {
	var size = 64 + rand.Intn(2048)
	var plainData = randBytes(size)
	for i := 0; i < b.N; i++ {
		nonce, _ := GenerateRandomKey(nonceSize)
		encrypted, err := cryptor.Encrypt(plainData, nonce)
		if err != nil {
			b.Fatalf("encrypt failed: %v", err)
		}
		_, err = cryptor.Decrypt(encrypted, nonce)
		if err != nil {
			b.Fatalf("decrypt failed: %v", err)
		}
	}
}

func Benchmark_AES_CBC(b *testing.B) {
	key, _ := GenerateRandomKey(24)
	hmacHey, _ := GenerateRandomKey(24)
	cryptor, err := CreateCryptorOf("aes-192-cbc", key, hmacHey)
	if err != nil {
		b.Fatalf("create cryptor failed: %v", err)
	}
	benchAESCryptor(b, cryptor, 16)
}

func Benchmark_AES_CTR(b *testing.B) {
	key, _ := GenerateRandomKey(24)
	hmacHey, _ := GenerateRandomKey(24)
	cryptor, err := CreateCryptorOf("aes-192-ctr", key, hmacHey)
	if err != nil {
		b.Fatalf("create cryptor failed: %v", err)
	}
	benchAESCryptor(b, cryptor, 16)
}

func Benchmark_AES_GCM(b *testing.B) {
	key, _ := GenerateRandomKey(24)
	cryptor, err := CreateCryptorOf("aes-192-gcm", key, nil)
	if err != nil {
		b.Fatalf("create cryptor failed: %v", err)
	}
	benchAESCryptor(b, cryptor, 12)
}

func Benchmark_AES_ChaCha(b *testing.B) {
	key, _ := GenerateRandomKey(32)
	cryptor, err := CreateCryptorOf("chacha20-poly1305", key, nil)
	if err != nil {
		b.Fatalf("create cryptor failed: %v", err)
	}
	benchAESCryptor(b, cryptor, 12)
}
