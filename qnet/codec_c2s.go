// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package qnet

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"

	"gopkg.in/srvkit.v1/logger"
)

const (
	C2SHeaderLength   = 16              // 消息头大小
	MaxC2SPacketSize  = 1024 * 1024 * 4 // 最大消息大小，~4MB
	MaxC2SPayloadSize = MaxC2SPacketSize - C2SHeaderLength
)

var (
	C2SCompressThreshold = 1 << 12 // 压缩阈值，4KB
	C2SMaxUpStreamSize   = 1 << 16 // 最大client上行消息大小, 64KB
)

// C2SHeader Client <-> Server协议头
// 要求强校验、强防御，在功能扩展性和实现简单性上取得平衡
//
// 字段说明：
// length	3字节，整个消息长度
// flag		1字节，标记位（压缩、加密等）
// msg_id	4字节，消息ID
// seq		4字节，序列号（匹配请求响应，防止包重放）
// crc      4字节，对上行消息是crc校验，对下行消息是错误码(若开启FlagError标记)
//|-------|--------|----------|-------|---------|--------|
//|-<len>-|-<flag>-|-<msg_id>-|--seq--|--<crc>--|-<data>-|
//|---3---|---1----|----4-----|---4---|----4----|---N/A--|
//|-------|--------|----------|-------|---------|--------|

type C2SHeader []byte

func NewC2SHeader() C2SHeader {
	return make(C2SHeader, C2SHeaderLength)
}

func (h C2SHeader) Len() uint32 {
	return partialBytesToInt(h[0:3])
}

func (h C2SHeader) Flag() MsgFlag {
	return MsgFlag(h[3])
}

func (h C2SHeader) MsgID() uint32 {
	return binary.LittleEndian.Uint32(h[4:8])
}

func (h C2SHeader) Seq() uint32 {
	return binary.LittleEndian.Uint32(h[8:12])
}

func (h C2SHeader) Checksum() uint32 {
	return binary.LittleEndian.Uint32(h[12:16])
}

func (h C2SHeader) Reset() {
	for i := range h {
		h[i] = 0
	}
}

// Pack 打包消息header的字段
func (h C2SHeader) Pack(size uint32, flag MsgFlag, msgId, seq, checksum uint32) {
	intToPartialBytes(size, h[0:3])
	h[3] = uint8(flag)
	binary.LittleEndian.PutUint32(h[4:8], msgId)
	binary.LittleEndian.PutUint32(h[8:12], seq)
	binary.LittleEndian.PutUint32(h[12:16], checksum)
}

type C2SCodec struct {
}

var _ ICodec = (*C2SCodec)(nil)

// ReadHeadBody 读取消息头和消息体
func (c *C2SCodec) ReadHeadBody(rd io.Reader, headbuf []byte, maxSize uint32) ([]byte, error) {
	var head = C2SHeader(headbuf[:C2SHeaderLength])
	if _, err := io.ReadFull(rd, head[:]); err != nil {
		return nil, err
	}
	var nLen = head.Len() // packet size，取值范围是[header size, maxSize]
	if nLen < C2SHeaderLength || nLen > maxSize {
		logger.Errorf("C2SCodec.ReadHeadBody: msg size %d out of range", nLen)
		return nil, ErrPktSizeOutOfRange
	}
	var body []byte
	if nLen > C2SHeaderLength {
		body = make([]byte, nLen-C2SHeaderLength)
		if _, err := io.ReadFull(rd, body); err != nil {
			return nil, err
		}
		var flag = head.Flag()
		if flag.Has(MsgFlagCRC) {
			var checksum = crc32.ChecksumIEEE(body)
			if crc := head.Checksum(); crc != checksum {
				logger.Errorf("C2SCodec.ReadHeadBody: msg %d checksum mismatch %x != %x", head.MsgID(), checksum, crc)
				return nil, ErrPktChecksumMismatch
			}
		}
	}
	return body, nil
}

// ProcessDecodeBody 处理消息体的解码（解密、解压缩）
func ProcessDecodeBody(flags MsgFlag, body []byte, decrypt IEncryptor) ([]byte, error) {
	if flags.Has(MsgFlagEncrypt) {
		if decrypt == nil {
			return nil, ErrCannotDecryptPkt
		}
		if decrypted, err := decrypt.Decrypt(body); err != nil {
			return nil, err
		} else {
			body = decrypted
		}
	}
	if flags.Has(MsgFlagCompress) {
		if originData, err := decompress(body, MaxC2SPayloadSize); err != nil {
			return nil, err
		} else {
			body = originData
		}
	}
	return body, nil
}

// DecodeMsgTo 把head和data表示的消息解码到NetMessage
func (c *C2SCodec) DecodeMsgTo(headbuf []byte, data []byte, decrypt IEncryptor, netMsg *NetMessage) error {
	var head = C2SHeader(headbuf[:C2SHeaderLength])
	body, err := ProcessDecodeBody(head.Flag(), data, decrypt)
	if err != nil {
		return err
	}
	netMsg.Seq = head.Seq()
	netMsg.MsgId = head.MsgID()
	netMsg.Body = body // 默认都延迟解码
	return nil
}

// EncodeNetMsg 编码消息到writer
func (c *C2SCodec) EncodeNetMsg(netMsg *NetMessage, encrypt IEncryptor, signCrc bool, w io.Writer) error {
	body, err := netMsg.Encode()
	if err != nil {
		return err
	}
	var flags MsgFlag
	if len(body) > C2SCompressThreshold {
		if compressed, err := compress(body); err != nil {
			logger.Errorf("C2SCodec.EncodeNetMsg: compress msg=%d failed: %v", netMsg.MsgId, err)
			// 不压缩，plain data
		} else {
			if len(compressed) < MaxC2SPayloadSize {
				body = compressed
				flags |= MsgFlagCompress
			}
		}
	}
	if encrypt != nil {
		if encrypted, err := encrypt.Encrypt(body); err != nil {
			return fmt.Errorf("C2SCodec.EncodeNetMsg: encrypt msg=%d %w", netMsg.MsgId, err)
		} else {
			body = encrypted
			flags |= MsgFlagEncrypt
		}
	}

	var bodySize = len(body)
	if bodySize > MaxC2SPayloadSize {
		return fmt.Errorf("C2SCodec.EncodeNetMsg: msg=%d size %d/%d overflow", netMsg.MsgId, bodySize, MaxC2SPacketSize)
	}

	// 可选的crc校验
	var checksum uint32
	if bodySize > 0 && signCrc {
		checksum = crc32.ChecksumIEEE(body)
		flags |= MsgFlagCRC
	}

	var head = make(C2SHeader, C2SHeaderLength)
	head.Pack(uint32(bodySize+C2SHeaderLength), flags, netMsg.MsgId, netMsg.Seq, checksum)
	if _, err = w.Write(head[:]); err != nil {
		return err
	}
	if bodySize > 0 {
		if _, err = w.Write(body); err != nil {
			return err
		}
	}
	return nil
}

// DecodeNetMsg 从reader解码消息到NetMessage
func (c *C2SCodec) DecodeNetMsg(rd io.Reader, maxSize uint32, decrypt IEncryptor, netMsg *NetMessage) error {
	var head [C2SHeaderLength]byte
	body, err := c.ReadHeadBody(rd, head[:], maxSize)
	if err != nil {
		return err
	}
	return c.DecodeMsgTo(head[:], body, decrypt, netMsg)
}
