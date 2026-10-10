// Copyright © 2022 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package idgen

import (
	"context"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCounter 基于redis INCR命令实现的计数器
// 注意：Redis默认是异步复制，并非强一致系统，某些故障条件下已确认写入仍可能丢失key，导致分配ID错误
type RedisCounter struct {
	guard  sync.Mutex    //
	addr   string        // redis服务器地址
	key    string        // 使用的key
	client *redis.Client //
}

func NewRedisCounter(addr, key string) CounterStore {
	var client = redis.NewClient(&redis.Options{
		Addr:        addr,
		DialTimeout: 3 * time.Second,
		PoolSize:    5,
	})
	return &RedisCounter{
		addr:   addr,
		key:    key,
		client: client,
	}
}

func (s *RedisCounter) Init(ctx context.Context) error {
	err := s.client.Ping(ctx).Err()
	if err != nil {
		return err
	}
	if _, err = s.Incr(ctx); err != nil {
		return err
	}
	// 使用 PERSIST 禁止 TTL
	if _, err = s.client.Persist(ctx, s.key).Result(); err != nil {
		return err
	}
	return nil
}

func (s *RedisCounter) Close() error {
	s.guard.Lock()
	defer s.guard.Unlock()

	if s.client != nil {
		err := s.client.Close()
		s.client = nil
		return err
	}
	return nil
}

func (s *RedisCounter) Incr(ctx context.Context) (int64, error) {
	s.guard.Lock()
	defer s.guard.Unlock()

	if s.client == nil {
		return 0, ErrStoreClosed
	}
	ctr, err := s.doIncr(ctx)
	if err != nil {
		return 0, err
	}
	return ctr, nil
}

// 生产级的部署要持久化与高可用，需要开启AOF和半同步复制
func (s *RedisCounter) doIncr(ctx context.Context) (int64, error) {
	counter, err := s.client.Incr(ctx, s.key).Uint64()
	if err != nil {
		return 0, err
	}
	return int64(counter), nil
}
