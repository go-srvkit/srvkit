// Copyright © 2026 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package idgen

import (
	"context"
	"fmt"
	"sync"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// EtcdCounter 使用每次独立 Put 的集群 revision 作为号段编号，编号不一定连续。
// 同一 ID 空间必须使用同一 etcd 集群和固定步长。从旧快照恢复或更换集群时，
// 必须保证新的编号超过所有已分配编号，不能直接恢复发号。
//
// EtcdCounter 拥有客户端，其公共方法支持并发调用，不能在使用后复制。
// guard 只保护客户端指针和初始化状态，网络请求不持有锁。
// Close 关闭客户端并取消未完成请求；已经开始的请求也可能成功完成。
type EtcdCounter struct {
	guard       sync.Mutex
	key         string
	step        int32
	client      *clientv3.Client
	initialized bool
}

var _ CounterStore = (*EtcdCounter)(nil)

// NewEtcdCounter 创建并拥有一个 etcd 客户端，config 支持多 endpoint、认证和 TLS。
// 调用 Init 确认可用后才能领取号段；不再使用时应调用 Close，即使 Init 失败。
func NewEtcdCounter(config clientv3.Config, key string) (*EtcdCounter, error) {
	if key == "" {
		return nil, fmt.Errorf("etcd counter key is empty")
	}
	client, err := clientv3.New(config)
	if err != nil {
		return nil, fmt.Errorf("create etcd counter client: %w", err)
	}
	return &EtcdCounter{key: key, client: client}, nil
}

// Init 使用线性一致读检查 key 的可访问性，不创建 key 或领取号段。
// key 不存在也可初始化成功。成功后重复调用不执行请求；关闭后不能重新初始化。
// ctx 应设置截止时间，以限制集群不可用时的等待时间。
func (s *EtcdCounter) Init(ctx context.Context) error {
	s.guard.Lock()
	defer s.guard.Unlock()

	client := s.client
	initialized := s.initialized

	if client == nil {
		return ErrStoreClosed
	}
	if initialized {
		return nil
	}
	if _, err := client.Get(ctx, s.key, clientv3.WithCountOnly()); err != nil {
		return fmt.Errorf("initialize etcd counter: %w", err)
	}

	if s.client == nil {
		return ErrStoreClosed
	}
	s.initialized = true
	return nil
}

// Close 释放客户端及其后台资源。重复关闭或关闭零值安全。
// 关闭后 Init 和 Incr 返回 ErrStoreClosed。
func (s *EtcdCounter) Close() error {
	s.guard.Lock()
	defer s.guard.Unlock()

	client := s.client
	s.client = nil
	s.initialized = false

	if client == nil {
		return nil
	}
	if err := client.Close(); err != nil {
		return fmt.Errorf("close etcd counter: %w", err)
	}
	return nil
}

// Incr 独立写入一次 key，返回该次 Put 响应的 revision。key 的值不用于计数。
// ctx 应设置截止时间；失败时返回 0，已提交但未确认的写入只会浪费号段。
func (s *EtcdCounter) Incr(ctx context.Context) (int64, error) {
	s.guard.Lock()
	defer s.guard.Unlock()

	client := s.client
	initialized := s.initialized

	if client == nil || !initialized {
		return 0, ErrStoreClosed
	}
	resp, err := client.Put(ctx, s.key, "allocate")
	if err != nil {
		return 0, fmt.Errorf("allocate etcd segment: %w", err)
	}
	if resp.Header == nil || resp.Header.Revision <= 0 {
		return 0, fmt.Errorf("invalid etcd counter revision: %w", ErrCounterOutOfRange)
	}
	return resp.Header.Revision, nil
}
