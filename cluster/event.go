// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package cluster

import (
	"fmt"
)

type NodeEventType int8

const (
	EventCreate NodeEventType = 0
	EventUpdate NodeEventType = 1
	EventDelete NodeEventType = 2
)

func (e NodeEventType) String() string {
	switch e {
	case EventCreate:
		return "create"
	case EventUpdate:
		return "update"
	case EventDelete:
		return "delete"
	}
	return "???"
}

// NodeEvent 节点变化事件
type NodeEvent struct {
	EvType   NodeEventType
	NodeType string
	NodeID   uint32
	Info     NodeInfo
}

func (e NodeEvent) String() string {
	return fmt.Sprintf("%s %s-%d: %v", e.EvType, e.NodeType, e.NodeID, e.Info)
}
