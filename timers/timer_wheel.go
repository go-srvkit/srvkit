// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package timers

import (
	"cmp"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/srvkit.v1/logger"
)

const MaxReuseBufferSize = 2048

// wheel, slots, interval, range
// L1, 512,                 50ms, [50ms, 25.6s)
// L2, 256,             50ms*512, [25.6s, ~1.82h)
// L3, 128,         50ms*512*256, [~1.82h, ~9.7d)
// L4, 32,      50ms*512*256*128, [~9.7d, ~310d)
// More distant timers stay in L4 and are reconsidered on subsequent rotations.
const (
	TickInterval     = 50 // milliseconds
	LVL1_BUCKET_SIZE = 512
	LVL2_BUCKET_SIZE = 256
	LVL3_BUCKET_SIZE = 128
	LVL4_BUCKET_SIZE = 32
	LVL1_ITV         = TickInterval
	LVL2_ITV         = LVL1_ITV * LVL1_BUCKET_SIZE
	LVL3_ITV         = LVL2_ITV * LVL2_BUCKET_SIZE
	LVL4_ITV         = LVL3_ITV * LVL3_BUCKET_SIZE

	// Bound empty-tick catch-up work even when logical time jumps are disabled.
	maxCatchUpTicks = LVL1_BUCKET_SIZE
)

var (
	stableTrigger  = envBool("APP_TIMER_STABLE_TRIGGER")
	enableTimePast = envBool("APP_ENABLE_TIME_PAST")
	isDebugTimer   = envBool("APP_DEBUG_TIMER")
)

// TimerNode 定时器节点。All fields are protected by the owning TimerWheels.guard1.
type TimerNode struct {
	tid   int64
	tick  int64 // deadline rounded up to a wheel tick
	wheel int32 // 1-based wheel number
	slot  int32
	index int
}

// TimerBucket 定时器槽。
type TimerBucket []*TimerNode

// TimerWheel 单个时间轮。It is not safe for concurrent use on its own.
type TimerWheel struct {
	interval int64
	slots    int32
	current  int32 // slot corresponding to the last processed tick
	size     int
	bitmap   []uint64
	buckets  []TimerBucket
}

// Init 初始化。interval and size must be positive.
func (tw *TimerWheel) Init(interval int64, size int32) {
	tw.interval = interval
	tw.slots = size
	tw.current = 0
	tw.size = 0
	tw.bitmap = make([]uint64, (size+63)/64)
	tw.buckets = make([]TimerBucket, size)
}

// Pop 返回并移除指定槽。The caller owns the returned slice and its node references.
// Its backing array is detached so reinsertion cannot overwrite the popped nodes.
func (tw *TimerWheel) Pop(slot int) TimerBucket {
	bucket := tw.buckets[slot]
	tw.buckets[slot] = nil
	tw.size -= len(bucket)
	clearBit(tw.bitmap, slot)
	return bucket
}

func (tw *TimerWheel) recycle(slot int, bucket TimerBucket) {
	clear(bucket)
	if cap(bucket) <= MaxReuseBufferSize && len(tw.buckets[slot]) == 0 {
		tw.buckets[slot] = bucket[:0]
	}
}

// TimerWheels 多级时间轮定时器。
// Initialize it with NewTimerWheels or Init before use.
// AddTimer, AddTimerAt, Cancel, Size, IsPending, Start and Shutdown are safe for
// concurrent use. guard1 protects the wheels, nodes, registry and lifecycle.
// Only the worker sends expiration IDs, always outside guard1. Callers must not
// close or replace C. A slow receiver backpressures delivery but not Cancel or
// Shutdown. Expired IDs already claimed by the worker cannot be cancelled.
// The worker terminates on Shutdown; no goroutine is started until Start.
type TimerWheels struct {
	guard1  sync.RWMutex
	done    chan struct{}
	stopped chan struct{}
	running bool
	closed  bool
	ticks   int64 // last processed absolute tick in the internal clock
	now     int64 // ticks * TickInterval
	wheels  [4]TimerWheel
	C       chan int64
	ref     map[int64]*TimerNode
	// clock returns wall time and internal time from one observation. Internal
	// time is monotonic unless APP_ENABLE_TIME_PAST was enabled at Init.
	clock  func() (wallMs, internalMs int64)
	stable bool
	debug  bool
}

// NewTimerWheels creates an initialized scheduler; call Start to run it.
func NewTimerWheels(capacity int) *TimerWheels {
	return new(TimerWheels).Init(capacity)
}

// Init initializes a zero scheduler or reinitializes one after Shutdown returns.
// It must not run concurrently with other methods. An active instance is left
// unchanged. A nonpositive capacity selects the default output buffer size.
func (tm *TimerWheels) Init(capacity int) *TimerWheels {
	tm.guard1.Lock()
	defer tm.guard1.Unlock()
	if tm.done != nil {
		if !tm.closed {
			return tm
		}
		select {
		case <-tm.stopped:
		default:
			return tm
		}
	}
	if capacity <= 0 {
		capacity = 1 << 15
	}
	tm.done = make(chan struct{})
	tm.stopped = make(chan struct{})
	tm.running = false
	tm.closed = false
	tm.ref = make(map[int64]*TimerNode, 1024)
	tm.C = make(chan int64, capacity)
	origin := time.Now()
	epochMs := origin.UnixMilli()
	logical := enableTimePast
	tm.clock = func() (int64, int64) {
		now := time.Now()
		wallMs := now.UnixMilli()
		if logical {
			return wallMs, wallMs
		}
		return wallMs, saturatedAdd(epochMs, now.Sub(origin).Milliseconds())
	}
	_, now := tm.clock()
	tm.now = align(now, TickInterval)
	tm.ticks = tm.now / TickInterval
	tm.stable = stableTrigger
	tm.debug = isDebugTimer
	tm.wheels[0].Init(LVL1_ITV, LVL1_BUCKET_SIZE)
	tm.wheels[1].Init(LVL2_ITV, LVL2_BUCKET_SIZE)
	tm.wheels[2].Init(LVL3_ITV, LVL3_BUCKET_SIZE)
	tm.wheels[3].Init(LVL4_ITV, LVL4_BUCKET_SIZE)
	tm.setCurrent()
	return tm
}

// TimedOutChan returns expiration IDs. Shutdown closes it after the worker exits;
// previously buffered IDs remain readable. Pending or undelivered IDs may be
// discarded during shutdown.
func (tm *TimerWheels) TimedOutChan() <-chan int64 {
	return tm.C
}

// Size returns the number of timers that have not yet been claimed for firing.
func (tm *TimerWheels) Size() int {
	tm.guard1.RLock()
	defer tm.guard1.RUnlock()
	return len(tm.ref)
}

// IsPending reports whether tid can still be cancelled, including before Start.
func (tm *TimerWheels) IsPending(tid int64) bool {
	tm.guard1.RLock()
	defer tm.guard1.RUnlock()
	return tm.ref[tid] != nil
}

// Cancel synchronously removes tid under the wheel mutex. A true return means
// this timer will not fire. False means it is absent or already claimed for
// firing; a claimed ID may still be awaiting channel delivery.
func (tm *TimerWheels) Cancel(tid int64) bool {
	tm.guard1.Lock()
	defer tm.guard1.Unlock()
	node := tm.ref[tid]
	if node == nil {
		return false
	}
	tm.delNode(node)
	delete(tm.ref, tid)
	freeTimerNode(node)
	return true
}

// AddTimer schedules tid after delayMs milliseconds using the internal clock.
// Nonpositive delays fire on the next processed tick. Reusing tid replaces its
// pending timer. Calls after Shutdown are ignored until reinitialized with Init.
func (tm *TimerWheels) AddTimer(tid int64, delayMs int64) {
	tm.guard1.Lock()
	defer tm.guard1.Unlock()
	if tm.closed {
		return
	}
	_, now := tm.clock()
	tm.schedule(tid, saturatedAdd(now, max(0, delayMs)))
}

// AddTimerAt schedules a Unix-millisecond deadline, converted to the internal
// clock at the time of this call. It otherwise has the same semantics as AddTimer.
// In ordinary mode, later wall-clock corrections do not move existing deadlines.
func (tm *TimerWheels) AddTimerAt(tid int64, deadlineMs int64) bool {
	tm.guard1.Lock()
	defer tm.guard1.Unlock()
	if tm.closed {
		return false
	}
	wallMs, now := tm.clock()
	delay := max(0, saturatedSub(deadlineMs, wallMs))
	tm.schedule(tid, saturatedAdd(now, delay))
	return true
}

// schedule and all node/wheel helpers require guard1 to be held exclusively.
func (tm *TimerWheels) schedule(tid, expire int64) {
	node := tm.ref[tid]
	if node != nil {
		tm.delNode(node)
	} else {
		node = allocTimerNode()
	}
	node.tid = tid
	node.tick = expire / TickInterval
	if expire%TickInterval > 0 {
		node.tick++
	}
	node.tick = max(node.tick, tm.ticks+1)
	tm.ref[tid] = node
	tm.addTimer(node)
}

// Shutdown permanently stops this run, cancels pending timers and waits for
// resource cleanup. It is safe before Start and on repeated/concurrent calls.
// Start cannot restart a stopped run; Init can be used after Shutdown returns.
func (tm *TimerWheels) Shutdown() {
	tm.guard1.Lock()
	if tm.done == nil {
		tm.guard1.Unlock()
		return
	}
	if !tm.closed {
		tm.closed = true
		close(tm.done)
		if !tm.running {
			tm.finish()
		}
	}
	stopped := tm.stopped
	tm.guard1.Unlock()
	<-stopped
}

// Start starts at most one worker and is a no-op after Shutdown.
func (tm *TimerWheels) Start() {
	tm.guard1.Lock()
	if tm.closed || tm.running {
		tm.guard1.Unlock()
		return
	}
	tm.running = true
	ready := make(chan struct{})
	go tm.worker(ready)
	tm.guard1.Unlock()
	<-ready
}

func (tm *TimerWheels) worker(ready chan struct{}) {
	ticker := time.NewTicker(TickInterval * time.Millisecond)
	defer func() {
		ticker.Stop()
		tm.guard1.Lock()
		defer tm.guard1.Unlock()
		tm.finish()
	}()
	close(ready)
	var fired []int64 // owned exclusively by this worker, including during sends
	for {
		select {
		case <-tm.done:
			return
		case <-ticker.C:
			tm.guard1.Lock()
			if tm.closed {
				tm.guard1.Unlock()
				return
			}
			_, now := tm.clock()
			jumped := now/TickInterval-tm.ticks > maxCatchUpTicks
			fired = tm.advance(now, fired[:0])
			tm.guard1.Unlock()
			if jumped && tm.debug {
				logger.Debugf("TimerWheels: rebuilt wheels after clock catch-up")
			}
			if !tm.fire(fired) {
				return
			}
			if cap(fired) > MaxReuseBufferSize {
				fired = nil
			}
		}
	}
}

// finish requires guard1. It runs exactly once, after the worker's last send.
func (tm *TimerWheels) finish() {
	for _, node := range tm.ref {
		freeTimerNode(node)
	}
	tm.ref = nil
	tm.wheels = [4]TimerWheel{}
	tm.running = false
	close(tm.C)
	close(tm.stopped)
}

func (tm *TimerWheels) calcWheel(tick int64) int32 {
	delta := tick - tm.ticks
	switch {
	case delta < LVL1_BUCKET_SIZE:
		return 1
	case delta < LVL1_BUCKET_SIZE*LVL2_BUCKET_SIZE:
		return 2
	case delta < LVL1_BUCKET_SIZE*LVL2_BUCKET_SIZE*LVL3_BUCKET_SIZE:
		return 3
	default:
		return 4
	}
}

func wheelSlot(tick, stride int64, slots int32) int {
	slot := (tick / stride) % int64(slots)
	if slot < 0 {
		slot += int64(slots)
	}
	return int(slot)
}

func (tm *TimerWheels) setCurrent() {
	for i := range tm.wheels {
		wheel := &tm.wheels[i]
		wheel.current = int32(wheelSlot(tm.ticks, wheel.interval/TickInterval, wheel.slots))
	}
}

func (tm *TimerWheels) addTimer(node *TimerNode) {
	w := tm.calcWheel(node.tick)
	wheel := &tm.wheels[w-1]
	slot := wheelSlot(node.tick, wheel.interval/TickInterval, wheel.slots)
	node.wheel = w
	node.slot = int32(slot)
	node.index = len(wheel.buckets[slot])
	wheel.buckets[slot] = append(wheel.buckets[slot], node)
	wheel.size++
	setBit(wheel.bitmap, slot)
}

func (tm *TimerWheels) delNode(node *TimerNode) {
	wheel := &tm.wheels[node.wheel-1]
	slot := int(node.slot)
	bucket := wheel.buckets[slot]
	last := len(bucket) - 1
	bucket[node.index] = bucket[last]
	bucket[node.index].index = node.index
	bucket[last] = nil
	bucket = bucket[:last]
	wheel.size--
	if len(bucket) == 0 {
		clearBit(wheel.bitmap, slot)
		if cap(bucket) > MaxReuseBufferSize {
			bucket = nil
		}
	}
	wheel.buckets[slot] = bucket
	node.wheel = 0
	node.slot = -1
	node.index = -1
}

func (tm *TimerWheels) fire(fired []int64) bool {
	for _, tid := range fired {
		// Prefer stopping even when the output channel also happens to be ready.
		select {
		case <-tm.done:
			return false
		default:
		}
		select {
		case tm.C <- tid:
		case <-tm.done:
			return false
		}
	}
	return true
}

// update claims due IDs under the mutex without delivering them to C.
func (tm *TimerWheels) update(nowMs int64) []int64 {
	tm.guard1.Lock()
	defer tm.guard1.Unlock()
	return tm.advance(nowMs, nil)
}

func (tm *TimerWheels) advance(nowMs int64, fired []int64) []int64 {
	target := nowMs / TickInterval
	if tm.closed || target <= tm.ticks {
		return fired
	}
	if target-tm.ticks > maxCatchUpTicks {
		tm.ticks = target
		tm.now = target * TickInterval
		tm.cascadeAll()
		var expired []*TimerNode
		for tid, node := range tm.ref {
			if node.tick <= target {
				delete(tm.ref, tid)
				expired = append(expired, node)
			} else {
				tm.addTimer(node)
			}
		}
		// Preserve tick order when skipping empty ticks. Stable mode additionally
		// orders equal-tick deadlines by ID, just like the ordinary advance path.
		slices.SortFunc(expired, func(a, b *TimerNode) int {
			if order := cmp.Compare(a.tick, b.tick); order != 0 {
				return order
			}
			if tm.stable {
				return cmp.Compare(a.tid, b.tid)
			}
			return 0
		})
		for _, node := range expired {
			fired = append(fired, node.tid)
			freeTimerNode(node)
		}
		return fired
	}
	for tm.ticks < target {
		tm.ticks++
		tm.now = tm.ticks * TickInterval
		tm.setCurrent()
		// At shared boundaries, higher levels must move first so lower levels
		// can cascade the newly inserted nodes during this same tick.
		for i := len(tm.wheels) - 1; i > 0; i-- {
			if tm.ticks%(tm.wheels[i].interval/TickInterval) == 0 {
				tm.cascade(i)
			}
		}
		start := len(fired)
		fired = tm.trigger0(fired)
		if tm.stable {
			slices.Sort(fired[start:])
		}
	}
	return fired
}

func (tm *TimerWheels) trigger0(fired []int64) []int64 {
	wheel := &tm.wheels[0]
	slot := int(wheel.current)
	if !testBit(wheel.bitmap, slot) {
		return fired
	}
	nodes := wheel.Pop(slot)
	for _, node := range nodes {
		if node.tick <= tm.ticks {
			tid := node.tid
			delete(tm.ref, tid)
			fired = append(fired, tid)
			freeTimerNode(node)
		} else {
			tm.addTimer(node)
		}
	}
	wheel.recycle(slot, nodes)
	return fired
}

func (tm *TimerWheels) cascade(idx int) {
	wheel := &tm.wheels[idx]
	slot := int(wheel.current)
	if !testBit(wheel.bitmap, slot) {
		return
	}
	nodes := wheel.Pop(slot)
	for _, node := range nodes {
		tm.addTimer(node)
	}
	wheel.recycle(slot, nodes)
}

// cascadeAll detaches all old buckets before rebuilding from the live registry.
// This prevents nodes from being revisited or overwritten during reinsertion.
func (tm *TimerWheels) cascadeAll() {
	for i := range tm.wheels {
		wheel := &tm.wheels[i]
		for slot, bucket := range wheel.buckets {
			clear(bucket)
			if cap(bucket) > MaxReuseBufferSize {
				wheel.buckets[slot] = nil
			} else {
				wheel.buckets[slot] = bucket[:0]
			}
		}
		clear(wheel.bitmap)
		wheel.size = 0
	}
	tm.setCurrent()
}

func align(ts, tick int64) int64 {
	return (ts / tick) * tick
}

func saturatedAdd(a, b int64) int64 {
	if b > 0 && a > math.MaxInt64-b {
		return math.MaxInt64
	}
	if b < 0 && a < math.MinInt64-b {
		return math.MinInt64
	}
	return a + b
}

func saturatedSub(a, b int64) int64 {
	if b > 0 && a < math.MinInt64+b {
		return math.MinInt64
	}
	if b < 0 && a > math.MaxInt64+b {
		return math.MaxInt64
	}
	return a - b
}

var nodePool = sync.Pool{New: func() any { return new(TimerNode) }}

func allocTimerNode() *TimerNode {
	return nodePool.Get().(*TimerNode)
}

// No references may be used after this call.
func freeTimerNode(node *TimerNode) {
	*node = TimerNode{}
	nodePool.Put(node)
}

func setBit(mask []uint64, i int) {
	mask[i>>6] |= uint64(1) << (i & 63)
}

func testBit(mask []uint64, i int) bool {
	return mask[i>>6]&(uint64(1)<<(i&63)) != 0
}

func clearBit(mask []uint64, i int) {
	mask[i>>6] &^= uint64(1) << (i & 63)
}

func envBool(key string) bool {
	if s := os.Getenv(key); s != "" {
		if strings.EqualFold(s, "ON") {
			return true
		}
		b, _ := strconv.ParseBool(s)
		return b
	}
	return false
}
