package qnet

import (
	"encoding/json"

	"github.com/golang/protobuf/proto"
)

type WsRecvMsg struct {
	Cmd  string          `json:"cmd"`
	Seq  uint32          `json:"seq,omitempty"`
	Body json.RawMessage `json:"body,omitempty"`
}

type WsWriteMsg struct {
	Cmd  string        `json:"cmd"`
	Seq  uint32        `json:"seq,omitempty"`
	Body proto.Message `json:"body,omitempty"`
}

type MessageFactory interface {
	Register(fullname string, msgId uint32) error
	RegisterByHash(fullname string) error
	GetMessageFullName(msgId uint32) string
	GetMessageShortName(msgId uint32) string
	GetMessageIDByName(fullName string) uint32
	GetMessageID(msg proto.Message) uint32
	CreateMessageByID(msgId uint32) proto.Message
	CreateMessageByFullName(fullName string) proto.Message
	CreatePairingAck(reqMsgId uint32) proto.Message
	CreateMessageFromJSON(content []byte) (proto.Message, error)
	CreateJSONFromProto(msgId uint32, data []byte) (*WsWriteMsg, error)
}

var factory MessageFactory

func SetMessageFactory(f MessageFactory) {
	factory = f
}
