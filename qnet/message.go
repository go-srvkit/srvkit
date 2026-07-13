// Copyright © 2023 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package qnet

import (
	"fmt"
	"time"

	"github.com/gogo/protobuf/proto"
	"gopkg.in/srvkit.v1/pool"
)

type NetMsgHandler func(message *NetMessage) error

type NetMessage struct {
	CreatedAt int64    `json:"created_at,omitempty"`
	MsgId     uint32   `json:"cmd"`
	Seq       uint32   `json:"seq,omitempty"`
	Node      NodeID   `json:"node,omitempty"`
	Refer     uint32   `json:"refer,omitempty"`
	Body      any      `json:"body,omitempty"` // []byte or proto.Message
	Session   Endpoint `json:"-"`
}

func NewNetMessage(msgId, seq uint32, data []byte) *NetMessage {
	var msg = AllocNetMessage()
	msg.MsgId = msgId
	msg.Seq = seq
	msg.Body = data
	return msg
}

func CreateNetMessage(msgId, seq uint32, body proto.Message) *NetMessage {
	var msg = AllocNetMessage()
	msg.MsgId = msgId
	msg.Seq = seq
	msg.Body = body
	return msg
}

func (m *NetMessage) Reset() {
	m.MsgId = 0
	m.Seq = 0
	m.Node = 0
	m.Refer = 0
	m.Body = nil
	m.Session = nil
}

func (m *NetMessage) Clone() *NetMessage {
	return &NetMessage{
		CreatedAt: time.Now().UnixMicro(),
		Seq:       m.Seq,
		MsgId:     m.MsgId,
		Node:      m.Node,
		Refer:     m.Refer,
		Body:      m.Body,
	}
}

// Encode encode `Payload` to []byte
func (m *NetMessage) Encode() ([]byte, error) {
	switch val := m.Body.(type) {
	case []byte:
		m.Body = nil // GC-friendly
		return val, nil
	case proto.Message:
		data, err := proto.Marshal(val)
		if err != nil {
			return nil, err
		}
		m.Body = nil // GC-friendly
		return data, nil
	case nil:
		return nil, nil
	default:
		return nil, fmt.Errorf("NetMessage.Encode: unsupported body type %T", val)
	}
}

// DecodeTo decode `Payload` to `msg`
func (m *NetMessage) DecodeTo(msg proto.Message) error {
	data, ok := m.Body.([]byte)
	if !ok {
		return fmt.Errorf("NetMessage.DecodeTo: unsupported body type %T", m.Body)
	}
	if len(data) > 0 {
		if err := proto.Unmarshal(data, msg); err != nil {
			return err
		}
		m.Body = nil // GC-friendly
	}
	return nil
}

func (m *NetMessage) AutoDecode() error {
	var msg = factory.CreateMessageByID(m.MsgId)
	if msg != nil {
		if err := m.DecodeTo(msg); err != nil {
			return err
		}
		m.Body = msg
		return nil
	}
	return fmt.Errorf("AutoDecode: unknown message id %d", m.MsgId)
}

func (m *NetMessage) Ack(msgId uint32, ack proto.Message) error {
	var netMsg = AllocNetMessage()
	netMsg.MsgId = msgId
	netMsg.Body = ack
	netMsg.Seq = m.Seq
	return m.Session.SendNetMsg(netMsg, SendNonblock)
}

func (m *NetMessage) Reply(ack proto.Message) error {
	var netMsg = AllocNetMessage()
	netMsg.MsgId = factory.GetMessageID(ack)
	netMsg.Body = ack
	netMsg.Seq = m.Seq
	return m.Session.SendNetMsg(netMsg, SendNonblock)
}

// Refuse 返回一个带错误码的Ack
func (m *NetMessage) Refuse(ec int32) error {
	// TODO:
	return nil
}

var msgPool = pool.NewObjectPool[NetMessage]()

func AllocNetMessage() *NetMessage {
	return msgPool.Get()
}

func FreeNetMessage(netMsg *NetMessage) {
	netMsg.Reset()
	msgPool.Put(netMsg)
}

// TryEnqueueMsg 尝试将消息放入队列，如果队列已满返回false
func TryEnqueueMsg(queue chan<- *NetMessage, msg *NetMessage) bool {
	select {
	case queue <- msg:
		return true
	default:
		return false // queue is full
	}
}

// TryDequeueMsg 尝试从队列中取出消息，如果队列为空返回nil
func TryDequeueMsg(queue <-chan *NetMessage) *NetMessage {
	select {
	case msg := <-queue:
		return msg
	default:
		return nil
	}
}
