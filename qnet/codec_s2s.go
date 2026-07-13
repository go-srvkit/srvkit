// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package qnet

import (
	"encoding/binary"
	"fmt"
	"io"

	"gopkg.in/srvkit.v1/logger"
)

const (
	S2SHeaderLength   = 20         // 消息头大小
	MaxS2SPacketSize  = 0x00FFFFFF // 最大消息大小，~16MB
	MaxS2SPayloadSize = MaxS2SPacketSize - S2SHeaderLength
)

var (
	S2SCompressThreshold = 1 << 13 // 压缩阈值，8KB
)

// S2SHeader 服务端协议头，假设通信环境可信，主要强调效率
//
// 字段说明：
// length	3字节，整个消息长度
// flag     1字节，标记位
// msg_id	4字节，消息ID
// seq		4字节，序列号
// node  	4字节，目标/源服务节点
// refer    4字节，关联的客户端会话
//|-------|--------|----------|-------|----------|---------|--------|
//|-<len>-|-<flag>-|-<msg_id>-|--seq--|--<node>--|-<refer>-|-<data>-|
//|---3---|---1----|----4-----|---4---|----4----|----4-----|---N/A--|
//|-------|--------|----------|-------|---------|----------|--------|

type S2SHeader []byte

func NewS2SHeader() S2SHeader {
	return make(S2SHeader, S2SHeaderLength)
}

func (h S2SHeader) Len() uint32 {
	return partialBytesToInt(h[0:3])
}

func (h S2SHeader) Flag() MsgFlag {
	return MsgFlag(h[3])
}

func (h S2SHeader) MsgID() uint32 {
	return binary.LittleEndian.Uint32(h[4:8])
}

func (h S2SHeader) Seq() uint32 {
	return binary.LittleEndian.Uint32(h[8:12])
}

func (h S2SHeader) Node() NodeID {
	return NodeID(binary.LittleEndian.Uint32(h[12:16]))
}

func (h S2SHeader) Refer() uint32 {
	return binary.LittleEndian.Uint32(h[16:20])
}

func (h S2SHeader) Reset() {
	for i := range h {
		h[i] = 0
	}
}

// Pack 打包消息header的字段
func (h S2SHeader) Pack(size uint32, flag MsgFlag, msgId, seq uint32, node NodeID, refer uint32) {
	intToPartialBytes(size, h[0:3])
	h[3] = uint8(flag)
	binary.LittleEndian.PutUint32(h[4:8], msgId)
	binary.LittleEndian.PutUint32(h[8:12], seq)
	binary.LittleEndian.PutUint32(h[12:16], uint32(node))
	binary.LittleEndian.PutUint32(h[16:20], refer)
}

type S2SCodec struct {
}

var _ ICodec = (*S2SCodec)(nil)

// ReadHeadBody 读取消息头和消息体
func (c *S2SCodec) ReadHeadBody(rd io.Reader, headbuf []byte, maxSize uint32) ([]byte, error) {
	var head = S2SHeader(headbuf[:S2SHeaderLength])
	if _, err := io.ReadFull(rd, head); err != nil {
		return nil, err
	}
	var nLen = head.Len() // packet size，取值范围是[header size, maxSize]
	if nLen < S2SHeaderLength || nLen > maxSize {
		logger.Errorf("S2SCodec.ReadHeadBody: msg size %d out of range", nLen)
		return nil, ErrPktSizeOutOfRange
	}
	var body []byte
	if nLen > S2SHeaderLength {
		body = make([]byte, nLen-S2SHeaderLength)
		if _, err := io.ReadFull(rd, body); err != nil {
			return nil, err
		}
	}
	return body, nil
}

// DecodeMsgTo 把head和data表示的消息解码到NetMessage
func (c *S2SCodec) DecodeMsgTo(headbuf []byte, body []byte, decrypt IEncryptor, netMsg *NetMessage) error {
	var head = S2SHeader(headbuf[:S2SHeaderLength])
	var flags = head.Flag()
	if flags.Has(MsgFlagCompress) {
		if originData, err := decompress(body, MaxS2SPayloadSize); err != nil {
			return err
		} else {
			body = originData
		}
	}
	netMsg.MsgId = head.MsgID()
	netMsg.Seq = head.Seq()
	netMsg.Node = head.Node()
	netMsg.Refer = head.Refer()
	netMsg.Body = body // 默认都延迟解码
	return nil
}

// EncodeNetMsg 编码消息到writer
func (c *S2SCodec) EncodeNetMsg(netMsg *NetMessage, _ IEncryptor, _ bool, w io.Writer) error {
	body, err := netMsg.Encode()
	if err != nil {
		return err
	}
	var flags MsgFlag
	if len(body) > S2SCompressThreshold {
		if compressed, err := compress(body); err != nil {
			logger.Errorf("S2SCodec.EncodeNetMsg: compress msg=%d failed: %v", netMsg.MsgId, err)
			// 不压缩，plain data
		} else {
			if len(compressed) < MaxS2SPayloadSize {
				body = compressed
				flags |= MsgFlagCompress
			}
		}
	}

	var bodySize = len(body)
	if bodySize+S2SHeaderLength > MaxS2SPacketSize {
		return fmt.Errorf("S2SCodec.EncodeNetMsg: msg=%d size %d/%d overflow", netMsg.MsgId, bodySize, MaxS2SPacketSize)
	}

	var head = make(S2SHeader, S2SHeaderLength)
	head.Pack(uint32(bodySize+S2SHeaderLength), flags, netMsg.MsgId, netMsg.Seq, netMsg.Node, netMsg.Refer)
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
func (c *S2SCodec) DecodeNetMsg(rd io.Reader, maxSize uint32, _ IEncryptor, netMsg *NetMessage) error {
	var head [S2SHeaderLength]byte
	body, err := c.ReadHeadBody(rd, head[:], maxSize)
	if err != nil {
		return err
	}
	return c.DecodeMsgTo(head[:], body, nil, netMsg)
}
