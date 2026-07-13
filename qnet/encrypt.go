// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package qnet

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	mrand "math/rand"
	"time"

	"golang.org/x/crypto/chacha20poly1305"
	"gopkg.in/srvkit.v1/protos"
)

const (
	AES_128_GCM       = 101
	AES_192_GCM       = 102
	AES_256_GCM       = 103
	CHACHA20_POLY1305 = 201
)

// CipherSuites 支持的加密套件列表
var CipherSuites = []int32{
	AES_128_GCM,
	AES_192_GCM,
	AES_256_GCM,
	CHACHA20_POLY1305,
}

// IEncryptor 定义加密解密接口
type IEncryptor interface {
	Encrypt([]byte) ([]byte, error)
	Decrypt([]byte) ([]byte, error)
}

// CreateEncryptor 随机选择一种加密器
func CreateEncryptor(suit int32, key []byte, session uint32, random uint64) (*AEADEncryptor, error) {
	switch suit {
	case AES_128_GCM:
		if len(key) < 16 {
			return nil, fmt.Errorf("invalid encrypt key length %d for AES-128-GCM", len(key))
		}
		return NewAESGCMEncryptor(key[:16], session, random)

	case AES_192_GCM:
		if len(key) < 24 {
			return nil, fmt.Errorf("invalid encrypt key length %d for AES-192-GCM", len(key))
		}
		return NewAESGCMEncryptor(key[:24], session, random)

	case AES_256_GCM:
		if len(key) < 32 {
			return nil, fmt.Errorf("invalid encrypt key length %d for AES-256-GCM", len(key))
		}
		return NewAESGCMEncryptor(key[:32], session, random)

	case CHACHA20_POLY1305:
		if len(key) < 32 {
			return nil, fmt.Errorf("invalid encrypt key length %d for CHACHA20-POLY1305", len(key))
		}
		return NewChaCha20Poly1305Encryptor(key[:32], session, random)
	}
	return nil, fmt.Errorf("unknown cipher suite %d", suit)
}

type AEADEncryptor struct {
	key     []byte
	session uint32
	random  uint64
	aead    cipher.AEAD
}

func NewAESGCMEncryptor(key []byte, session uint32, random uint64) (*AEADEncryptor, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &AEADEncryptor{
		key:     key,
		session: session,
		random:  random,
		aead:    aead,
	}, nil
}

func NewChaCha20Poly1305Encryptor(key []byte, session uint32, random uint64) (*AEADEncryptor, error) {
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return nil, err
	}
	return &AEADEncryptor{
		key:     key,
		session: session,
		random:  random,
		aead:    aead,
	}, nil
}

// nextNonce 用以生成每次加密的nonce，使用会话ID+自增的random
func (c *AEADEncryptor) nextNonce(buf []byte) {
	c.random++
	binary.LittleEndian.PutUint32(buf, c.session)
	binary.LittleEndian.PutUint64(buf[4:], c.random)
}

// Encrypt 加密
func (c *AEADEncryptor) Encrypt(plainData []byte) ([]byte, error) {
	var nonce [12]byte // c.aead.NonceSize()
	c.nextNonce(nonce[:])
	if n := c.aead.NonceSize(); len(nonce) != n {
		return nil, fmt.Errorf("AEAD nonce size mismatch %d != %d", len(nonce), n)
	}
	var encrypted = c.aead.Seal(nil, nonce[:], plainData, nil)
	return encrypted, nil
}

// Decrypt 解密
func (c *AEADEncryptor) Decrypt(encrypted []byte) ([]byte, error) {
	if overheadLen := c.aead.Overhead(); len(encrypted) < overheadLen {
		return nil, fmt.Errorf("AEAD encrypted data too short %d < %d", len(encrypted), overheadLen)
	}
	var nonce [12]byte
	c.nextNonce(nonce[:]) // c.aead.NonceSize()
	return c.aead.Open(nil, nonce[:], encrypted, nil)
}

// ServerKeyExchange 服务端层面与客户端密钥交换
func ServerKeyExchange(req *protos.ClientHello, ack *protos.ServerHello) (decKey, encKey []byte, err error) {
	var serverRandom = make([]byte, 8) // uint64
	if _, err = rand.Read(serverRandom[:]); err != nil {
		return
	}
	var serverNonce = make([]byte, 32)
	if _, err = rand.Read(serverNonce[:]); err != nil {
		return
	}
	// ECDH 密钥交换
	var serverPriKey *ecdh.PrivateKey
	if serverPriKey, err = ecdh.P256().GenerateKey(rand.Reader); err != nil {
		return
	}
	serverPubKey := serverPriKey.PublicKey().Bytes()
	var clientPubKey *ecdh.PublicKey
	if clientPubKey, err = ecdh.P256().NewPublicKey(req.ClientPubKey); err != nil {
		return
	}
	var sharedKey []byte
	if sharedKey, err = serverPriKey.ECDH(clientPubKey); err != nil {
		return
	}
	// 组合生成AES密钥
	var buf bytes.Buffer
	buf.Grow(len(sharedKey) + len(req.ClientNonce) + len(serverNonce))
	buf.Write(sharedKey)
	buf.Write(req.ClientNonce)
	buf.Write(serverNonce)

	// 生成解密客户端上行消息所用的key
	var clientRandom = make([]byte, 8)
	binary.LittleEndian.PutUint64(clientRandom, req.ClientRandom)
	if decKey, err = hkdf.Key(sha256.New, buf.Bytes(), clientRandom, "", sha256.Size); err != nil {
		return
	}
	// 生成加密服务端下行消息所用的key
	if encKey, err = hkdf.Key(sha256.New, buf.Bytes(), serverRandom, "", sha256.Size); err != nil {
		return
	}
	ack.ProtocolVersion = C2SProtocolVersion
	ack.Timestamp = time.Now().Unix()
	ack.ServerNonce = serverNonce
	ack.ServerPubKey = serverPubKey
	ack.ServerRandom = binary.LittleEndian.Uint64(serverRandom[:])

	return
}

// GenerateClientHello 生成ClientHello消息及对应的私钥
func GenerateClientHello(cipherSuits ...int32) (hello *protos.ClientHello, priKey *ecdh.PrivateKey, err error) {
	// 生成clientNonce
	var clientNonce = make([]byte, 32)
	if _, err = rand.Read(clientNonce); err != nil {
		return
	}
	// 生成clientRandom
	var clientRandom = make([]byte, 8) // uint64
	if _, err = rand.Read(clientRandom); err != nil {
		return
	}
	if priKey, err = ecdh.P256().GenerateKey(rand.Reader); err != nil {
		return
	}
	hello = &protos.ClientHello{
		ProtocolVersion:       C2SProtocolVersion,
		Timestamp:             time.Now().Unix(),
		SupportedCipherSuites: cipherSuits,
		ClientNonce:           clientNonce,
		ClientPubKey:          priKey.PublicKey().Bytes(),
		ClientRandom:          binary.LittleEndian.Uint64(clientRandom[:8]),
	}
	return
}

// ClientKeyExchange 客户端层面与服务端密钥交换
func ClientKeyExchange(clientPriKey *ecdh.PrivateKey, req *protos.ClientHello, ack *protos.ServerHello) (decKey, encKey []byte, err error) {
	// 计算共享密钥
	var serverPubKey *ecdh.PublicKey
	if serverPubKey, err = ecdh.P256().NewPublicKey(ack.ServerPubKey); err != nil {
		return
	}
	var sharedKey []byte
	if sharedKey, err = clientPriKey.ECDH(serverPubKey); err != nil {
		return
	}
	var serverRandom = make([]byte, 8)
	binary.LittleEndian.PutUint64(serverRandom, ack.ServerRandom)

	// 组合生成AES密钥
	var buf bytes.Buffer
	buf.Write(sharedKey)
	buf.Write(req.ClientNonce)
	buf.Write(ack.ServerNonce)
	var secretKey = buf.Bytes()

	// 生成解密服务器下行消息所用的key
	if decKey, err = hkdf.Key(sha256.New, buf.Bytes(), serverRandom, "", sha256.Size); err != nil {
		return
	}
	// 生成加密客户端上行消息所用的key
	var clientRandom = make([]byte, 8)
	binary.LittleEndian.PutUint64(clientRandom, req.ClientRandom)
	if encKey, err = hkdf.Key(sha256.New, secretKey, clientRandom, "", sha256.Size); err != nil {
		return
	}
	return
}

// RandomCipherSuite 随机选择一种加密套件
func RandomCipherSuite() int32 {
	if len(CipherSuites) > 0 {
		idx := mrand.Intn(len(CipherSuites))
		return CipherSuites[idx]
	}
	return AES_192_GCM // fallback
}
