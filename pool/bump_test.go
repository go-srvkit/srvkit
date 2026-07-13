// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package pool

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewBumpAlloc(t *testing.T) {
	tests := []struct {
		name      string
		blockSize int
		wantPanic bool
	}{
		{"normal blockSize", 128, false},
		{"blockSize 1", 1, false},
		{"large blockSize", 1024, false},
		{"zero blockSize", 0, false},
		{"negative blockSize", -1, true},
		{"large negative blockSize", -100, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantPanic {
				assert.Panics(t, func() {
					NewBumpAlloc[int](tt.blockSize)
				})
			} else {
				alloc := NewBumpAlloc[int](tt.blockSize)
				assert.NotNil(t, alloc)
			}
		})
	}
}

func TestBumpAlloc_Alloc(t *testing.T) {
	tests := []struct {
		name       string
		blockSize  int
		allocCount int
	}{
		{"single alloc", 4, 1},
		{"exact block boundary", 4, 4},
		{"cross block boundary", 4, 5},
		{"multiple blocks", 4, 12},
		{"blockSize 1 multiple allocs", 1, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			alloc := NewBumpAlloc[int](tt.blockSize)
			// Act
			ptrs := make([]*int, tt.allocCount)
			for i := 0; i < tt.allocCount; i++ {
				ptrs[i] = alloc.Alloc()
			}
			// Assert - all pointers are non-nil and distinct
			for i := 0; i < tt.allocCount; i++ {
				assert.NotNil(t, ptrs[i])
				for j := i + 1; j < tt.allocCount; j++ {
					assert.NotSame(t, ptrs[i], ptrs[j])
				}
			}
		})
	}
}

func TestBumpAlloc_Alloc_ZeroValue(t *testing.T) {
	// Arrange
	alloc := NewBumpAlloc[int](4)
	// Act
	p := alloc.Alloc()
	// Assert
	assert.Equal(t, 0, *p)
}

func TestBumpAlloc_Alloc_WriteAndRead(t *testing.T) {
	tests := []struct {
		name      string
		blockSize int
		values    []int
	}{
		{"within single block", 8, []int{10, 20, 30}},
		{"across block boundary", 2, []int{1, 2, 3, 4, 5}},
		{"exact block fill", 3, []int{100, 200, 300}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			alloc := NewBumpAlloc[int](tt.blockSize)
			ptrs := make([]*int, len(tt.values))
			// Act
			for i, v := range tt.values {
				ptrs[i] = alloc.Alloc()
				*ptrs[i] = v
			}
			// Assert - values are preserved after all allocations
			for i, want := range tt.values {
				assert.Equal(t, want, *ptrs[i])
			}
		})
	}
}

func TestBumpAlloc_Alloc_StructType(t *testing.T) {
	type sample struct {
		Name string
		Age  int
	}

	// Arrange
	alloc := NewBumpAlloc[sample](4)
	// Act
	p1 := alloc.Alloc()
	p2 := alloc.Alloc()
	*p1 = sample{"alice", 30}
	*p2 = sample{"bob", 25}
	// Assert
	assert.Equal(t, sample{"alice", 30}, *p1)
	assert.Equal(t, sample{"bob", 25}, *p2)
}

func TestBumpAlloc_Alloc_Concurrent(t *testing.T) {
	// Arrange
	alloc := NewBumpAlloc[int64](64)
	const goroutines = 100
	const allocsPerGoroutine = 50

	// Act
	var wg sync.WaitGroup
	results := make([][]*int64, goroutines)
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ptrs := make([]*int64, allocsPerGoroutine)
			for i := 0; i < allocsPerGoroutine; i++ {
				ptrs[i] = alloc.Alloc()
				*ptrs[i] = int64(idx*allocsPerGoroutine + i)
			}
			results[idx] = ptrs
		}(g)
	}
	wg.Wait()

	// Assert - all pointers are unique and values are preserved
	seen := make(map[*int64]struct{}, goroutines*allocsPerGoroutine)
	for g := 0; g < goroutines; g++ {
		for i := 0; i < allocsPerGoroutine; i++ {
			p := results[g][i]
			assert.NotNil(t, p)
			assert.NotContains(t, seen, p, "duplicate pointer detected")
			seen[p] = struct{}{}
			assert.Equal(t, int64(g*allocsPerGoroutine+i), *p)
		}
	}
}

func TestBumpAlloc_Alloc_BlockRollover(t *testing.T) {
	// Arrange - blockSize=2, allocate 6 times to trigger 2 rollovers
	alloc := NewBumpAlloc[int](2)
	// Act
	ptrs := make([]*int, 6)
	for i := 0; i < 6; i++ {
		ptrs[i] = alloc.Alloc()
		*ptrs[i] = i * 10
	}
	// Assert - all values preserved across block boundaries
	for i := 0; i < 6; i++ {
		assert.Equal(t, i*10, *ptrs[i])
	}
}
