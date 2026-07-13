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
	return s.client.Ping(ctx).Err()
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

	ctr, err := s.doIncr(ctx)
	if err != nil {
		return 0, err
	}
	return ctr, nil
}

// 生产级的部署要持久化与高可用，需要开启AOF和半同步复制
func (s *RedisCounter) doIncr(ctx context.Context) (int64, error) {
	counter, err := s.client.Do(ctx, "INCR", s.key).Int64()
	if err != nil {
		return 0, err
	}
	return counter, nil
}
