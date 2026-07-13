// Copyright © 2021 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package qnet

import (
	"fmt"
	"strconv"
	"strings"
)

// 与 protos/api.go 中的 ServiceType 定义同步
const (
	NodeServiceShift = 20
	NodeServiceMask  = 1<<(32-NodeServiceShift) - 1
	NodeInstanceMask = 1<<NodeServiceShift - 1
)

// NodeID是一个32位整数表示的节点号，用以标识一个唯一的后端服务
// 二进制布局：
// 高12位表示服务类型，最大值为4095
// 低20位表示服务实例编号，最大值为1048575

type NodeID uint32

// MakeNode 根据服务类型和实例号创建一个节点ID，服务类型限制为16位
func MakeNode(service, instance uint32) NodeID {
	return NodeID((uint32(service&NodeServiceMask) << NodeServiceShift) | (instance & NodeInstanceMask))
}

// MakeNodeFrom `hexStr`为 0x0501形式的字符串
func MakeNodeFrom(hexStr string) NodeID {
	if strings.HasPrefix(hexStr, "0x") {
		hexStr = hexStr[2:]
	}
	var service uint32 = 0
	var instance uint32 = 0
	if len(hexStr) >= 4 {
		if n, err := strconv.ParseInt(hexStr[:2], 16, 32); err != nil {
			return 0
		} else {
			service = uint32(n)
		}
		if n, err := strconv.ParseInt(hexStr[2:], 16, 32); err != nil {
			return 0
		} else {
			instance = uint32(n)
		}
	}
	return MakeNode(service, instance)
}

// Service 服务型
func (n NodeID) Service() uint32 {
	return uint32((n >> NodeServiceShift) & NodeServiceMask)
}

// Instance 节点的实例编号
func (n NodeID) Instance() uint32 {
	return uint32(n) & NodeInstanceMask
}

func (n NodeID) String() string {
	return fmt.Sprintf("0x%04x%04x", n.Service(), n.Instance())
}

// NodeIDSet 没有重复ID的有序集合
type NodeIDSet = []NodeID
