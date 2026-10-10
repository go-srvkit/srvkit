// Copyright © 2022 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package idgen

import (
	"context"
	"errors"
)

type StorageType int8

const (
	StorageMongoDB StorageType = 1
	StorageEtcd    StorageType = 2
	StorageRedis   StorageType = 3 // 更推荐使用 mongodb和 etcd这类强一致性系统
)

var (
	ErrStoreClosed       = errors.New("store not initialized or is closed")
	ErrCounterOutOfRange = errors.New("counter out of range")
	ErrStepMismatch      = errors.New("counter step mismatch")
)

// CounterStore 表示一个存储组件，维持一个持续递增（不一定连续）的counter
type CounterStore interface {
	Init(ctx context.Context) error
	Close() error
	Incr(ctx context.Context) (int64, error)
}

// IDGenerator ID生成器
type IDGenerator interface {
	Next(context.Context) (int64, error)
}
