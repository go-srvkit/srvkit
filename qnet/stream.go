// Copyright © 2023-2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package qnet

import (
	"sync/atomic"

	"github.com/golang/protobuf/proto"
)

// StreamBase base stream implementation
type StreamBase struct {
	node       NodeID           //
	closing    atomic.Int32     //
	sendQ      chan *NetMessage // 发消息队列
	recvQ      chan *NetMessage // 收消息队列
	errQ       chan *Error      // error signal
	remoteAddr string           // 缓存的远端地址
	userdata   any              // user data
}

func (c *StreamBase) Init(sendQSize int, recvQ chan *NetMessage, errQ chan *Error) {
	c.recvQ = recvQ
	c.errQ = errQ
	if sendQSize > 0 {
		c.sendQ = make(chan *NetMessage, sendQSize)
	}
}

func (c *StreamBase) IsClosing() bool {
	return c.closing.Load() > 0
}

func (c *StreamBase) SetClosing() bool {
	return c.closing.CompareAndSwap(0, 1)
}

func (c *StreamBase) Node() NodeID {
	return c.node
}

func (c *StreamBase) SetNode(node NodeID) {
	c.node = node
}

func (c *StreamBase) RemoteAddr() string {
	return c.remoteAddr
}

func (c *StreamBase) SetRemoteAddr(v string) {
	c.remoteAddr = v
}

func (c *StreamBase) UserData() any {
	return c.userdata
}

func (c *StreamBase) SetUserData(val any) {
	c.userdata = val
}

func (c *StreamBase) RecvNetMsg(netMsg *NetMessage) {
	c.recvQ <- netMsg
}

func (c *StreamBase) RecvQueue() <-chan *NetMessage {
	return c.recvQ
}

func (c *StreamBase) SetRecvQueue(recvQ chan *NetMessage) {
	c.recvQ = recvQ
}

func (c *StreamBase) SendQueue() <-chan *NetMessage {
	return c.sendQ
}

func (c *StreamBase) SetSendQueue(sendQ chan *NetMessage) {
	c.sendQ = sendQ
}

func (c *StreamBase) ErrQueue() <-chan *Error {
	return c.errQ
}

func (c *StreamBase) SetErrQueue(errQ chan *Error) {
	c.errQ = errQ
}

// AppendErr 投递错误消息到错误队列
func (c *StreamBase) AppendErr(reason error, endpoint Endpoint) bool {
	if c.errQ == nil {
		return false
	}
	var err = NewError(reason, endpoint)
	select {
	case c.errQ <- err:
		return true
	default:
		return false
	}
}

func (c *StreamBase) Reset() {
	c.recvQ = nil
	c.errQ = nil
	c.userdata = nil
}

// SendNetMsg 发送NetMessage消息
func (c *StreamBase) SendNetMsg(msg *NetMessage, mode int) error {
	if c.sendQ == nil {
		return ErrSendChanIsNil
	}
	switch mode {
	case SendBlock:
		c.sendQ <- msg // blocking send
		return nil
	}
	if !c.SendNetMsgNonBlock(msg) {
		return ErrConnOutboundOverflow
	}
	return nil
}

// SendNetMsgNonBlock 发送NetMessage消息
func (c *StreamBase) SendNetMsgNonBlock(msg *NetMessage) bool {

	select {
	case c.sendQ <- msg:
		return true
	default:
		return false
	}
}

// SendMessage 发送带ID的proto消息
func (c *StreamBase) SendMessage(msgId uint32, msg proto.Message) error {
	if c.IsClosing() {
		return ErrConnNotRunning
	}
	var netMsg = AllocNetMessage()
	netMsg.MsgId = msgId
	netMsg.Body = msg
	return c.SendNetMsg(netMsg, SendNonblock)
}
