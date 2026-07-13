// Copyright © 2022 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package idgen

import (
	"context"
	"testing"
	"time"
)

func TestSeqIDRedisSimple(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	runSeqIDTestSimple(t, ctx, "redis", "/uuid/counter1")
	// Output:
	//  QPS 5558769.22/s
}

func TestSeqIDRedisConcurrent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	runSeqIDTestConcurrent(t, ctx, "redis", "/uuid/counter2")
	// Output:
	//  QPS 2086800.36/s
}

func TestSeqIDRedisDistributed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	runSeqIDTestDistributed(t, ctx, "redis", "/uuid/counter3")
	// Output:
	//  QPS 2315360.85/s
}

func TestSeqIDMongoSimple(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	runSeqIDTestSimple(t, ctx, "mongo", "uuid_counter1")
	// Output:
	//  QPS 406465.45/s
}

func TestSeqIDMongoConcurrent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	runSeqIDTestConcurrent(t, ctx, "mongo", "uuid_counter2")
	// Output:
	//  QPS 1880948.45/s
}

func TestSeqIDMongoDistributed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	runSeqIDTestDistributed(t, ctx, "mongo", "uuid_counter3")
	// Output:
	//  QPS 2380477.59/s
}
