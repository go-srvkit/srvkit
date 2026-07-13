// Copyright © 2021 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package qnet

import (
	"errors"
	"fmt"
)

var (
	ErrPktSizeOutOfRange    = errors.New("packet size out of range")
	ErrPktChecksumMismatch  = errors.New("packet checksum mismatch")
	ErrEmptyDataBody        = errors.New("empty data body")
	ErrDataCompress         = errors.New("data is incompressible")
	ErrCannotDecryptPkt     = errors.New("cannot decrypt packet")
	ErrNotEncryptedPkt      = errors.New("not encrypted packet")
	ErrConnNotRunning       = errors.New("connection not running")
	ErrConnOutboundOverflow = errors.New("connection outbound queue overflow")
	ErrConnForceClose       = errors.New("connection forced to close")
	ErrSendChanIsNil        = errors.New("send channel is nil")
	ErrConnClosed           = errors.New("connection is closed")
	ErrInvalidWSMsgType     = errors.New("invalid websocket message type")
)

type Error struct {
	Err      error
	Endpoint Endpoint
}

func NewError(err error, endpoint Endpoint) *Error {
	return &Error{
		Err:      err,
		Endpoint: endpoint,
	}
}

func (e Error) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("node %v(%s) EOF", e.Endpoint.Node(), e.Endpoint.RemoteAddr())
	}
	return fmt.Sprintf("node %v(%s) %s", e.Endpoint.Node(), e.Endpoint.RemoteAddr(), e.Err.Error())
}
