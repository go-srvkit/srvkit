// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package qnet

import (
	"bufio"
	"crypto/sha1"
	"fmt"
	"log"
	"math/rand"
	"net"
	"sync"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/assert"

	"gopkg.in/srvkit.v1/protos"
)

func sha1Sum(data []byte) string {
	return fmt.Sprintf("%x", sha1.Sum(data))
}

func testEchoServer(t *testing.T, ln net.Listener) {
	var sessionId uint32
	for {
		conn, err := ln.Accept()
		if err != nil {
			break
		}
		sessionId++
		go handleTestSessionConn(t, sessionId, conn)
	}
}

func handleTestSessionConn(t *testing.T, sessionId uint32, conn net.Conn) {
	defer conn.Close()

	var req protos.ClientHello
	if err := ReadProtoMessage(conn, &req); err != nil {
		t.Fatalf("Failed to receive handshake req: %v", err)
		return
	}

	var ack = &protos.ServerHello{
		ServerSessionId: sessionId,
		CipherSuite:     RandomCipherSuite(), // 随机选一种加密,
	}

	deckey, enckey, err := ServerKeyExchange(&req, ack)
	assert.NoError(t, err)
	assert.Len(t, enckey, 32)
	assert.Len(t, deckey, 32)

	// 生成解密客户端上行消息所用的key
	decrypt, err := CreateEncryptor(ack.CipherSuite, deckey, sessionId, req.ClientRandom)
	assert.NoError(t, err)
	t.Logf("server session=%d decrypt key=%s  random=%d\n", sessionId, sha1Sum(deckey), req.ClientRandom)

	// 生成加密上行发给客户端的消息用的key
	encrypt, err := CreateEncryptor(ack.CipherSuite, enckey, sessionId, ack.ServerRandom)
	assert.NoError(t, err)
	t.Logf("server session=%d encrypt key=%s  random=%d\n", sessionId, sha1Sum(enckey), ack.ServerRandom)

	if err := WriteProtoMessage(conn, ack); err != nil {
		t.Fatalf("Failed to receive handshake req: %v", err)
		return
	}

	rd := bufio.NewReader(conn)
	for {
		encData, err := ReadLenData(rd, 8192)
		if err != nil {
			t.Logf("session %d ReadLenData error: %v\n", sessionId, err)
			break
		}
		t.Logf("Session %d recv encrypted data checksum=%s, aes key=%s random=%d\n", sessionId,
			sha1Sum(encData), sha1Sum(decrypt.key), decrypt.random)

		decrypted, err := decrypt.Decrypt(encData)
		if err != nil {
			t.Fatalf("session %d decrypt error: %v\n", sessionId, err)
			continue
		}
		t.Logf("Session %d received data: len=%d\n", sessionId, len(decrypted))

		t.Logf("Session %d try encrypt data len=%d, aes key=%s random=%d\n", sessionId,
			len(decrypted), sha1Sum(encrypt.key), encrypt.random)
		encrypted, err := encrypt.Encrypt(decrypted)
		if err != nil {
			t.Fatalf("session %d encrypt error: %v\n", sessionId, err)
			continue
		}
		t.Logf("Session %d write encrypted data checksum=%s\n", sessionId, sha1Sum(encrypted))
		if err := WriteLenData(conn, encrypted); err != nil {
			t.Logf("session %d WriteLenData error: %v\n", sessionId, err)
			continue
		}
	}
}

func runTestClient(t *testing.T, wg *sync.WaitGroup, addr string) {
	defer wg.Done()

	time.Sleep(time.Millisecond * 100)
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		log.Fatalf("dial err: %v", err)
		return
	}
	defer conn.Close()

	req, clientPriKey, err := GenerateClientHello(CipherSuites...)
	assert.NoError(t, err)

	if err := WriteProtoMessage(conn, req); err != nil {
		t.Fatalf("Failed to send handshake request: %v", err)
		return
	}
	var ack protos.ServerHello
	if err := ReadProtoMessage(conn, &ack); err != nil {
		t.Fatalf("Failed to receive handshake ack: %v", err)
		return
	}
	t.Logf("client got handshake: %s\n", ack.String())

	decKey, encKey, err := ClientKeyExchange(clientPriKey, req, &ack)
	assert.NoError(t, err)
	assert.Len(t, encKey, 32)
	assert.Len(t, decKey, 32)

	var sessionId = ack.ServerSessionId

	encryptor, err := CreateEncryptor(ack.CipherSuite, encKey, sessionId, req.ClientRandom)
	assert.NoError(t, err)
	t.Logf("client encrypt key=%s session=%d random=%d\n", sha1Sum(encKey), sessionId, req.ClientRandom)

	// 生成解密下行消息用的key
	decryptor, err := CreateEncryptor(ack.CipherSuite, decKey, sessionId, ack.ServerRandom)
	assert.NoError(t, err)
	t.Logf("client decrypt key=%s session=%d random=%d\n", sha1Sum(decKey), sessionId, ack.ServerRandom)

	if err != nil {
		log.Fatalf("handshakeServer err: %v", err)
		return
	}

	for i := 0; i < 3; i++ {
		{
			var size = 32 + rand.Intn(1024)
			var s = randString(size)
			var data = unsafe.Slice(unsafe.StringData(s), len(s))

			t.Logf("client try encrypt data len=%d, aes key=%s random=%d\n",
				len(data), sha1Sum(encryptor.key), encryptor.random)

			encrypted, err := encryptor.Encrypt(data)
			if err != nil {
				t.Logf("client encrypt err: %v", err)
				break
			}
			t.Logf("client write encrypted data len=%d checksum=%s\n", len(encrypted), sha1Sum(encrypted))
			if err := WriteLenData(conn, encrypted); err != nil {
				t.Logf("client WriteText err: %v\n", err)
				break
			}
		}
		time.Sleep(time.Second)
		{
			encData, err := ReadLenData(conn, 8192)
			if err != nil {
				t.Logf("client ReadLenData err: %v\n", err)
				break
			}
			t.Logf("client recv encrypted data len=%d checksum=%s, aes key=%s random=%d\n", len(encData),
				sha1Sum(encData), sha1Sum(decryptor.key), decryptor.random)
			decrypted, err := decryptor.Decrypt(encData)
			if err != nil {
				t.Logf("client decrypt error: %v\n", err)
				continue
			}
			t.Logf("client decrypt data: %v\n", string(decrypted))
		}
	}

	return
}

func TestAEADEncryptor(t *testing.T) {
	ln, addr := createTestRandListener(t)
	assert.NotNil(t, ln)

	go testEchoServer(t, ln)

	var wg sync.WaitGroup
	wg.Add(1)
	go runTestClient(t, &wg, addr)
	wg.Wait()
}
