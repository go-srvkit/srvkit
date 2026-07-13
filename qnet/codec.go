// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package qnet

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"net"
	"time"

	"github.com/golang/protobuf/proto"
)

const (
	C2SProtocolVersion = 1
)

type ICodec interface {
	ReadHeadBody(rd io.Reader, head []byte, maxSize uint32) ([]byte, error)
	DecodeMsgTo(head, data []byte, decrypt IEncryptor, netMsg *NetMessage) error
	EncodeNetMsg(netMsg *NetMessage, encrypt IEncryptor, signCrc bool, w io.Writer) error
	DecodeNetMsg(rd io.Reader, maxSize uint32, decrypt IEncryptor, netMsg *NetMessage) error
}

type MsgFlag uint8

const (
	MsgFlagCRC      MsgFlag = 0x01
	MsgFlagCompress MsgFlag = 0x02
	MsgFlagEncrypt  MsgFlag = 0x04
	MsgFlagError    MsgFlag = 0x10
	MsgFlagBatch    MsgFlag = 0x20
)

func (g MsgFlag) Has(n MsgFlag) bool {
	return g&n != 0
}

func (g MsgFlag) Clear(n MsgFlag) MsgFlag {
	return g &^ n
}

// ReadLenData 读取长度开头的数据，长度用uint16表示，指代整个消息的大小
func ReadLenData(r io.Reader, maxSize uint16) ([]byte, error) {
	var tmp [2]byte
	if _, err := io.ReadFull(r, tmp[:]); err != nil {
		return nil, fmt.Errorf("ReadLenData: read len: %v", err)
	}
	var nLen = int(binary.LittleEndian.Uint16(tmp[:]))
	if nLen <= 2 || nLen > int(maxSize) {
		return nil, fmt.Errorf("ReadLenData: msg size %d out of range", nLen)
	}
	var data = make([]byte, nLen-2)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, fmt.Errorf("ReadLenData: read body of len %d: %v", nLen, err)
	}
	return data, nil
}

// WriteLenData 写入长度开头的数据，长度用uint16表示，指代整个消息的大小
func WriteLenData(w io.Writer, body []byte) error {
	if len(body) == 0 {
		return ErrEmptyDataBody
	}
	var nLen = len(body) + 2
	if nLen > math.MaxUint16 {
		return fmt.Errorf("WriteLenData: msg size %d out of range", nLen)
	}
	var buf [2]byte
	binary.LittleEndian.PutUint16(buf[:2], uint16(nLen))
	if _, err := w.Write(buf[:]); err != nil {
		return err
	}
	if _, err := w.Write(body); err != nil {
		return err
	}
	return nil
}

// ReadProtoMessage 从网络连接中读取proto消息
func ReadProtoMessage(conn net.Conn, msg proto.Message) error {
	conn.SetReadDeadline(time.Now().Add(time.Second * 60))
	body, err := ReadLenData(conn, math.MaxUint16)
	if err != nil {
		return err
	}
	if err = proto.Unmarshal(body, msg); err != nil {
		return err
	}
	return nil
}

func WriteProtoMessage(w io.Writer, msg proto.Message) error {
	body, err := proto.Marshal(msg)
	if err != nil {
		return err
	}
	return WriteLenData(w, body)
}

// RequestProtoMessage send req and wait for ack
func RequestProtoMessage(conn net.Conn, req, ack proto.Message) error {
	if err := WriteProtoMessage(conn, req); err != nil {
		return err
	}
	return ReadProtoMessage(conn, ack)
}
