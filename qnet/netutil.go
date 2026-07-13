// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package qnet

import (
	"encoding/binary"
	"fmt"
	"net"

	"github.com/pierrec/lz4/v4"

	"gopkg.in/srvkit.v1/logger"
)

// 3-bytes little endian to uint32
func partialBytesToInt(b []byte) uint32 {
	_ = b[2] // bounds check hint to compiler; see golang.org/issue/14808
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16
}

// uint32 to little endian 3-bytes
func intToPartialBytes(v uint32, b []byte) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
}

// 压缩算法选择：
// 1，可接受的CPU开销，
// 2，压缩率与解压速度的平衡
// 3，协议兼容性与移动端友好性；

func compress(input []byte) ([]byte, error) {
	var nLen = len(input)
	if nLen == 0 {
		return nil, nil
	}
	var size = lz4.CompressBlockBound(4 + nLen) // with original size prefix
	var output = make([]byte, size)
	binary.LittleEndian.PutUint32(output[:4], uint32(nLen))
	n, err := lz4.CompressBlock(input, output[4:], nil)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, ErrDataCompress
	}
	return output[:n+4], nil
}

func decompress(compressed []byte, maxSize uint32) ([]byte, error) {
	var nLen = len(compressed)
	if nLen < 4 || nLen > int(maxSize) {
		return nil, lz4.ErrInvalidSourceShortBuffer
	}
	// with original size prefix
	var originSize = binary.LittleEndian.Uint32(compressed[:4])
	if originSize == 0 || originSize > maxSize {
		return nil, fmt.Errorf("uncompress: origin size=%d out of range", originSize)
	}
	var origin = make([]byte, originSize)
	n, err := lz4.UncompressBlock(compressed[4:], origin)
	if err != nil {
		return nil, err
	}
	return origin[:n], nil
}

// GetLocalIPList 获取本地IP列表
func GetLocalIPList() []net.IP {
	ifaceList, err := net.Interfaces()
	if err != nil {
		logger.Errorf("cannot fetch net interfaces: %v", err)
		return nil
	}
	var ipList []net.IP
	for _, iface := range ifaceList {
		addrs, err := iface.Addrs()
		if err != nil {
			logger.Errorf("cannot fetch net address: %v", err)
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if len(ip) > 0 && !ip.IsLoopback() {
				ipList = append(ipList, ip)
			}
		}
	}
	return ipList
}
