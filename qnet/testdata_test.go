// Copyright © 2023-2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package qnet

import (
	"github.com/golang/protobuf/proto"
)

func init() {
	proto.RegisterType((*BuildReq)(nil), "BuildReq")
	proto.RegisterType((*BuildAck)(nil), "BuildAck")
}

type BuildReq struct {
	PosX int32 `protobuf:"varint,2,opt,name=PosX,proto3" json:"PosX,omitempty"`
	PosY int32 `protobuf:"varint,3,opt,name=PosZ,proto3" json:"PosZ,omitempty"`
}

func (m *BuildReq) Reset()         { *m = BuildReq{} }
func (m *BuildReq) String() string { return proto.CompactTextString(m) }
func (*BuildReq) ProtoMessage()    {}

type BuildAck struct {
	Code int32 `protobuf:"varint,1,opt,name=Code,proto3,enum=protocol.ErrCode" json:"Code,omitempty"`
}

func (m *BuildAck) Reset()         { *m = BuildAck{} }
func (m *BuildAck) String() string { return proto.CompactTextString(m) }
func (*BuildAck) ProtoMessage()    {}
