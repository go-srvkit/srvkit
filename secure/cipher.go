// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package secure

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/chacha20poly1305"
)

type ModeType uint8

const (
	ModeChaCha20 ModeType = 1 + iota
	ModeAESGCM            // AES-GCM
	ModeAESCTR            // AES-CTR
	ModeAESCBC            // AES-CBC
)

var eHMACVerifyFailed = errors.New("HMAC-SHA256 verification failed")

// Encrypter 定义加密解密接口
type Encrypter interface {
	Encrypt(plainData, nonce []byte) ([]byte, error)
	Decrypt(encrypted, nonce []byte) ([]byte, error)
}

// GenerateRandomKey 生成随机密钥，keySize 必须是 16、24、32 之一
func GenerateRandomKey(keySize int) ([]byte, error) {
	var key = make([]byte, keySize)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	return key, nil
}

func ParseCipherName(cipherName string) (ModeType, int, int, error) {
	cipherName = strings.ToLower(strings.TrimSpace(cipherName))
	if cipherName == "chacha20-poly1305" {
		return ModeChaCha20, chacha20poly1305.KeySize, chacha20poly1305.NonceSize, nil
	}
	if strings.HasPrefix(cipherName, "aes-") {
		var idx = strings.IndexByte(cipherName[4:], '-')
		if idx > 0 {
			var mode ModeType
			var modeStr = cipherName[4+idx+1:]
			switch modeStr {
			case "cbc":
				mode = ModeAESCBC
			case "ctr":
				mode = ModeAESCTR
			case "gcm":
				mode = ModeAESGCM
			default:
				return 0, 0, 0, fmt.Errorf("invalid AES mode: %s", modeStr)
			}
			keyLen, _ := strconv.Atoi(cipherName[4 : 4+idx])
			keyLen /= 8
			switch keyLen {
			case 16, 24, 32:
			default:
				return 0, 0, 0, fmt.Errorf("invalid AES block size %d", keyLen)
			}
			return mode, keyLen, 16, nil
		}
	}
	return 0, 0, 0, fmt.Errorf("invalid cipher name: %s", cipherName)
}

func CreateCryptorOf(name string, key, hmacKey []byte) (Encrypter, error) {
	mode, keyLen, _, err := ParseCipherName(name)
	if err != nil {
		return nil, err
	}
	if len(key) < keyLen {
		return nil, fmt.Errorf("key length mismatch %d != %d", len(key), keyLen)
	}
	return NewCryptor(mode, key[:keyLen], hmacKey)
}

// NewCryptor 创建AES加密器，仅支持CBC、CTR、GCM模式
func NewCryptor(mode ModeType, key, hmacKey []byte) (Encrypter, error) {
	switch mode {
	case ModeAESCBC:
		return CreateBlockCryptor(key, hmacKey)

	case ModeAESCTR:
		return CreateStreamCryptor(key, hmacKey)

	case ModeAESGCM:
		return NewAESGCMCryptor(key)

	case ModeChaCha20:
		return NewChaCha20Poly1305Cryptor(key)
	}
	return nil, fmt.Errorf("unsupported cryptor mode %d", mode)
}

func pkcs7Padding(data []byte, blockSize int) []byte {
	var padding = blockSize - len(data)%blockSize
	var padded = bytes.Repeat([]byte{byte(padding)}, padding)
	return append(data, padded...)
}

func pkcs7Unpadding(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var unpadding = int(data[len(data)-1])
	if unpadding > len(data) {
		return nil, fmt.Errorf("invalid padding size %d: data length %d", unpadding, len(data))
	}
	return data[:(len(data) - unpadding)], nil
}

func hmacSHA256Sum(key, data []byte) []byte {
	var h = hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func hmacSHA256Verify(key, data, macValue []byte) bool {
	var calculated = hmacSHA256Sum(key, data)
	return hmac.Equal(calculated, macValue)
}

// BlockCryptor implements AES encryption in CBC mode with PKCS7 padding
type BlockCryptor struct {
	key     []byte
	hmacKey []byte
	block   cipher.Block
}

func CreateBlockCryptor(key, hmacKey []byte) (*BlockCryptor, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return &BlockCryptor{
		key:     key,
		hmacKey: hmacKey,
		block:   block,
	}, nil
}

func (c *BlockCryptor) Encrypt(plainData, nonce []byte) ([]byte, error) {
	var blockSize = c.block.BlockSize()
	if len(nonce) != blockSize {
		return nil, fmt.Errorf("nonce length mismatch %d != %d", len(nonce), blockSize)
	}
	var padded = pkcs7Padding(plainData, blockSize)
	var mode = cipher.NewCBCEncrypter(c.block, nonce)
	var encrypted []byte
	// add HMAC-SHA256 for integrity check
	if c.hmacKey != nil {
		encrypted = make([]byte, len(padded), len(padded)+sha256.Size)
		mode.CryptBlocks(encrypted, padded)
		var macValue = hmacSHA256Sum(c.hmacKey, encrypted)
		encrypted = append(encrypted, macValue...)
	} else {
		encrypted = make([]byte, len(padded), len(padded))
		mode.CryptBlocks(encrypted, padded)
	}
	return encrypted, nil
}

func (c *BlockCryptor) Decrypt(encrypted, nonce []byte) ([]byte, error) {
	var blockSize = c.block.BlockSize()
	if len(nonce) != blockSize {
		return nil, fmt.Errorf("nonce length mismatch %d != %d", len(nonce), blockSize)
	}
	// verify HMAC-SHA256 for integrity check
	if c.hmacKey != nil {
		if len(encrypted) < sha256.Size {
			return nil, fmt.Errorf("invalid encrypted size: %d", len(encrypted))
		}
		var macValue = encrypted[len(encrypted)-sha256.Size:]
		encrypted = encrypted[:len(encrypted)-sha256.Size]
		if !hmacSHA256Verify(c.hmacKey, encrypted, macValue) {
			return nil, eHMACVerifyFailed
		}
	}
	var mode = cipher.NewCBCDecrypter(c.block, nonce)
	var decrypted = make([]byte, len(encrypted))
	mode.CryptBlocks(decrypted, encrypted)
	return pkcs7Unpadding(decrypted)
}

// StreamCryptor implements AES encryption in CTR mode
type StreamCryptor struct {
	key     []byte
	hmacKey []byte
	block   cipher.Block
}

func CreateStreamCryptor(key, hmacKey []byte) (*StreamCryptor, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return &StreamCryptor{
		key:     key,
		hmacKey: hmacKey,
		block:   block,
	}, nil
}

func (c *StreamCryptor) Encrypt(plainData, nonce []byte) ([]byte, error) {
	if len(nonce) != c.block.BlockSize() {
		return nil, fmt.Errorf("nonce length mismatch %d != %d", len(nonce), c.block.BlockSize())
	}
	var stream = cipher.NewCTR(c.block, nonce)
	var encrypted []byte
	// add HMAC-SHA256 for integrity check
	if c.hmacKey != nil {
		encrypted = make([]byte, len(plainData), len(plainData)+sha256.Size)
		stream.XORKeyStream(encrypted, plainData)
		var macValue = hmacSHA256Sum(c.hmacKey, encrypted)
		encrypted = append(encrypted, macValue...)
	} else {
		encrypted = make([]byte, len(plainData))
		stream.XORKeyStream(encrypted, plainData)
	}
	return encrypted, nil
}

func (c *StreamCryptor) Decrypt(encrypted, nonce []byte) ([]byte, error) {
	if len(nonce) != c.block.BlockSize() {
		return nil, fmt.Errorf("nonce length mismatch %d != %d", len(nonce), c.block.BlockSize())
	}
	// verify HMAC-SHA256 for integrity check
	if c.hmacKey != nil {
		if len(encrypted) < sha256.Size {
			return nil, fmt.Errorf("invalid encrypted size: %d", len(encrypted))
		}
		var macValue = encrypted[len(encrypted)-sha256.Size:]
		encrypted = encrypted[:len(encrypted)-sha256.Size]
		if !hmacSHA256Verify(c.hmacKey, encrypted, macValue) {
			return nil, eHMACVerifyFailed
		}
	}
	var stream = cipher.NewCTR(c.block, nonce)
	var decrypted = make([]byte, len(encrypted))
	stream.XORKeyStream(decrypted, encrypted)
	return decrypted, nil
}

// AEADCryptor implements AEAD encryption (AES-GCM, ChaCha20-Poly1305)
type AEADCryptor struct {
	key  []byte
	aead cipher.AEAD
}

func (c *AEADCryptor) Encrypt(plainData, nonce []byte) ([]byte, error) {
	if n := c.aead.NonceSize(); len(nonce) != n {
		return nil, fmt.Errorf("AEAD nonce size mismatch %d != %d", len(nonce), n)
	}
	var encrypted = c.aead.Seal(nil, nonce, plainData, nil)
	return encrypted, nil
}

func (c *AEADCryptor) Decrypt(encrypted, nonce []byte) ([]byte, error) {
	if n := c.aead.NonceSize(); len(nonce) != n {
		return nil, fmt.Errorf("AEAD nonce size mismatch %d != %d", len(nonce), n)
	}
	return c.aead.Open(nil, nonce, encrypted, nil)
}

func NewAESGCMCryptor(key []byte) (*AEADCryptor, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &AEADCryptor{
		key:  key,
		aead: aead,
	}, nil
}

func NewChaCha20Poly1305Cryptor(key []byte) (*AEADCryptor, error) {
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return nil, err
	}
	return &AEADCryptor{
		key:  key,
		aead: aead,
	}, nil
}
