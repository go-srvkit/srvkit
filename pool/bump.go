// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package pool

import (
	"sync"
)

const (
	RuntimePageSize = 8192 // RuntimePageSize is _PageSize at runtime/malloc.go
)

// BumpAlloc
// 一次申请一个block（N个元素的数组），block分配完了就丢掉（交给GC)，再申请另一个block；
// 对runtime来说每次malloc都是以N个元素大小的单位，可以减缓GC的压力;
// 如果外部代码长期持有单个element为会导致block得不到释放；
type BumpAlloc[T any] struct {
	guard sync.Mutex
	idx   int
	block []T
}

// NewBumpAlloc `blockSize`应该 = RuntimePageSize / sizeof(T)
func NewBumpAlloc[T any](blockSize int) *BumpAlloc[T] {
	return &BumpAlloc[T]{
		block: make([]T, blockSize),
	}
}

func (a *BumpAlloc[T]) Alloc() *T {
	a.guard.Lock()
	var size = len(a.block)
	var ret = &a.block[a.idx]
	a.idx++
	if a.idx >= size {
		a.block = make([]T, size)
		a.idx = 0
	}
	a.guard.Unlock()
	return ret
}
