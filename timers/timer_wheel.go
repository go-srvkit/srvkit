// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package timers

import (
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gopkg.in/srvkit.v1/datetime"
	"gopkg.in/srvkit.v1/logger"
)

const MaxReuseBufferSize = 2048

// wheel, slots, interval, range
// L1, 512，           50ms, [50ms, 25.6s]
// L2, 256，       50ms*512, [25.6s, ~1.82h]
// L3, 128，   50ms*512*256, [~1.82h, ~9.7d]
// L4, 32，	50ms*512*256*32, [~9.7d, ~310d]
const (
	TickInterval     = 50 // 20帧
	LVL1_BUCKET_SIZE = 512
	LVL2_BUCKET_SIZE = 256
	LVL3_BUCKET_SIZE = 128
	LVL4_BUCKET_SIZE = 32
	LVL1_ITV         = TickInterval
	LVL2_ITV         = LVL1_ITV * LVL1_BUCKET_SIZE
	LVL3_ITV         = LVL2_ITV * LVL2_BUCKET_SIZE
	LVL4_ITV         = LVL3_ITV * LVL3_BUCKET_SIZE
)

var (
	stableTrigger  = envBool("APP_TIMER_STABLE_TRIGGER") // 全局稳定触发开关
	enableTimePast = envBool("APP_ENABLE_TIME_PAST")     // 是否允许调逻辑时钟(时间跳变）
	isDebugTimer   = envBool("APP_DEBUG_TIMER")
)

// TimerNode 定时器节点
type TimerNode struct {
	tid    int64 // 唯一ID
	expire int64 // 到期时间 (毫秒)
	wheel  int32 // wheel从1开始
	slot   int32 // 处于哪个slot
	index  int   // 在bucket中的索引
}

// TimerBucket 定时器槽
type TimerBucket []*TimerNode

// TimerWheel 单个时间轮
type TimerWheel struct {
	interval int64         // wheel的刻度
	slots    int32         // slot数量
	current  int32         // 当前指向哪个slot
	size     int           // 记录timer数量
	bitmap   []uint64      // 标记哪些槽有定时器
	buckets  []TimerBucket // 所有定时器slot
}

// Init 初始化
func (tw *TimerWheel) Init(interval int64, size int32) {
	tw.interval = interval
	tw.slots = size
	tw.bitmap = make([]uint64, (size+63)/64)
	tw.buckets = make([]TimerBucket, size)
}

// Pop 返回并清空指定slot的定时器槽
func (tw *TimerWheel) Pop(slot int) TimerBucket {
	var bucket = tw.buckets[slot]
	if cap(bucket) > MaxReuseBufferSize {
		tw.buckets[slot] = nil
	} else {
		tw.buckets[slot] = bucket[:0]
	}
	clearBit(tw.bitmap, slot)
	return bucket
}

// TimerWheels 多级时间轮定时器
type TimerWheels struct {
	done    chan struct{}
	wg      sync.WaitGroup
	running atomic.Int32
	ticks   int64
	now     int64                // 当前时钟（毫秒）
	wheels  [4]TimerWheel        // level 1~4
	C       chan int64           // 到期的定时器
	guard1  sync.RWMutex         // guard ref map
	ref     map[int64]*TimerNode // O(1)查找
	bus     chan timerOp         // queue of timer operations
	fired   []int64              // fired timers in current tick
}

func NewTimerWheels(capacity int) *TimerWheels {
	return new(TimerWheels).Init(capacity)
}

func (tm *TimerWheels) Init(capacity int) *TimerWheels {
	if capacity <= 0 {
		capacity = 1 << 15
	}
	tm.done = make(chan struct{})
	tm.ref = make(map[int64]*TimerNode, 1024)
	tm.C = make(chan int64, capacity)
	tm.bus = make(chan timerOp, 10240)
	tm.fired = make([]int64, 0, 64)
	tm.now = align(time.Now().UnixMilli(), TickInterval)
	tm.wheels[0].Init(LVL1_ITV, LVL1_BUCKET_SIZE)
	tm.wheels[1].Init(LVL2_ITV, LVL2_BUCKET_SIZE)
	tm.wheels[2].Init(LVL3_ITV, LVL3_BUCKET_SIZE)
	tm.wheels[3].Init(LVL4_ITV, LVL4_BUCKET_SIZE)
	return tm
}

// TimedOutChan 外部接受超时触发的channel
func (tm *TimerWheels) TimedOutChan() <-chan int64 {
	return tm.C
}

// Size 获取当前等待触发的定时器数量
func (tm *TimerWheels) Size() int {
	tm.guard1.RLock()
	var n = len(tm.ref)
	tm.guard1.RUnlock()
	return n
}

// IsPending 判断定时器是否在等待触发
func (tm *TimerWheels) IsPending(tid int64) bool {
	tm.guard1.RLock()
	var node = tm.ref[tid]
	var ok = node != nil && node.wheel > 0
	tm.guard1.RUnlock()
	return ok
}

// Cancel 取消一个定时器
func (tm *TimerWheels) Cancel(tid int64) bool {
	tm.guard1.Lock()
	defer tm.guard1.Unlock()

	node, found := tm.ref[tid]
	if found {
		delete(tm.ref, tid)                 // 立即删除ref
		tm.bus <- timerOp{OpDelTimer, node} // 异步删除wheel
	}
	return found
}

// AddTimer 创建一个定时器，在`delayMs`毫秒后过期
func (tm *TimerWheels) AddTimer(tid int64, delayMs int64) {
	var deadlineMs = currentTimeMillis() + delayMs
	tm.AddTimerAt(tid, deadlineMs)
}

// AddTimerAt 创建一个定时器，在指定时间（毫秒）过期
func (tm *TimerWheels) AddTimerAt(tid int64, deadlineMs int64) {
	var node = allocTimerNode()
	node.tid = tid
	node.expire = deadlineMs
	node.wheel = 0
	node.index = -1
	node.slot = -1

	// 先放入ref，等待worker线程处理
	tm.guard1.Lock()
	tm.ref[node.tid] = node
	tm.bus <- timerOp{OpAddTimer, node}
	tm.guard1.Unlock()
}

// delRef delete timer in ref
func (tm *TimerWheels) delRef(tid int64) {
	tm.guard1.Lock()
	delete(tm.ref, tid)
	tm.guard1.Unlock()
}

// Shutdown 关闭定时器调度器
func (tm *TimerWheels) Shutdown() {
	if !tm.running.CompareAndSwap(1, 0) {
		return
	}
	close(tm.done)
	tm.wg.Wait()

	close(tm.C)
	close(tm.bus)
	tm.guard1.Lock()
	tm.ref = nil
	tm.guard1.Unlock()
}

// Start 启动定时器调度器
func (tm *TimerWheels) Start() {
	if !tm.running.CompareAndSwap(0, 1) {
		return
	}
	tm.wg.Add(1)
	var ready = make(chan struct{})
	go tm.worker(ready)
	<-ready
}

// worker 定时器调度协程
func (tm *TimerWheels) worker(ready chan struct{}) {
	defer tm.wg.Done()
	var ticker = time.NewTicker(TickInterval * time.Millisecond)
	defer ticker.Stop()
	ready <- struct{}{}
	for {
		select {
		case <-ticker.C:
			tm.update(currentTimeMillis())

		case op := <-tm.bus:
			tm.handleTimerOp(op)

		case <-tm.done:
			return
		}
	}
}

func (tm *TimerWheels) handleTimerOp(op timerOp) {
	if op.node == nil {
		return
	}
	switch op.type_ {
	case OpAddTimer:
		tm.addTimer(op.node)

	case OpDelTimer:
		if !tm.delNode(op.node) {
			logger.Errorf("TimerWheels: failed to delete timer %v", op.node)
		}
	default:
		logger.Errorf("TimerWheels: unknown timer type: %v", op.type_)
	}
}

// calcWheel 计算定时器应该放在哪个层级(1~4)
func (tm *TimerWheels) calcWheel(expire int64) int32 {
	var delta = expire - tm.now
	if delta < LVL1_ITV*int64(LVL1_BUCKET_SIZE) {
		return 1
	} else if delta < LVL2_ITV*int64(LVL2_BUCKET_SIZE) {
		return 2
	} else if delta < LVL3_ITV*int64(LVL3_BUCKET_SIZE) {
		return 3
	} else {
		return 4
	}
}

// addTimer with guard1 locked
func (tm *TimerWheels) addTimer(node *TimerNode) {
	var wheel = tm.calcWheel(node.expire)
	tm.insertTo(node, wheel)
}

// insertTo with guard1 locked
func (tm *TimerWheels) insertTo(node *TimerNode, w int32) {
	var wheel = &tm.wheels[w-1]
	var diff = max(0, node.expire-tm.now) / wheel.interval
	var slot = (int(wheel.current) + int(diff)) % int(wheel.slots)
	var index = len(wheel.buckets[slot])
	wheel.buckets[slot] = append(wheel.buckets[slot], node)
	setBit(wheel.bitmap, slot)
	node.slot = int32(slot)
	node.index = index
	node.wheel = w
	wheel.size++
}

func (tm *TimerWheels) delNode(node *TimerNode) bool {
	defer func() {
		node.wheel = 0
		node.slot = -1
		node.index = -1
	}()

	if node.wheel <= 0 || int(node.wheel) > len(tm.wheels) {
		logger.Errorf("TimerWheels: invalid wheel=%d for timer=%d", node.wheel, node.tid)
		return false
	}
	var wheel = &tm.wheels[node.wheel-1]
	if node.slot < 0 || node.slot >= wheel.slots {
		logger.Errorf("TimerWheels: invalid slot=%d for timer=%d in wheel=%d", node.slot, node.tid, node.wheel)
		return false
	}
	var bucket = wheel.buckets[node.slot]
	if node.index < 0 || node.index >= len(bucket) {
		logger.Errorf("TimerWheels: invalid index=%d for timer=%d in slot=%d of wheel=%d", node.index, node.tid,
			node.slot, node.wheel)
		return false
	}
	var item = bucket[node.index]
	if item == nil || item.tid != node.tid {
		logger.Errorf("TimerWheels: timer mismatch tid=%d in slot=%d of wheel=%d", node.tid, node.slot, node.wheel)
		return false
	}
	// 删除时间轮中的定时器
	var N = len(bucket)
	var last = bucket[N-1]
	bucket[node.index] = last // swap with last item in bucket
	last.index = node.index   // update index
	bucket[N-1] = nil         // GC friendly
	wheel.buckets[node.slot] = bucket[:N-1]
	wheel.size--
	return true
}

func (tm *TimerWheels) fire() {
	if stableTrigger && len(tm.fired) > 1 {
		slices.Sort(tm.fired) // 对ID排序以保证触发顺序一致
	}
	for _, tid := range tm.fired {
		tm.C <- tid
	}
	if cap(tm.fired) > MaxReuseBufferSize {
		tm.fired = nil
	} else {
		tm.fired = tm.fired[:0]
	}
}

func (tm *TimerWheels) update(nowMs int64) {
	var elapsed = nowMs - tm.now
	if elapsed < TickInterval {
		return
	}
	// 逻辑时钟跳变了超过1h后直接全层级触发，避免CPU耗费在大量的for循环里
	if enableTimePast && elapsed >= 3600_000 {
		if isDebugTimer {
			logger.Infof("TimerWheels: time past detected elapsed=%s", datetime.PrettyTime(elapsed))
		}
		var ticks = elapsed / TickInterval
		tm.now += ticks * TickInterval
		tm.ticks += ticks
		tm.cascadeAll() // 调整所有定时器的层级
		tm.trigger0()   // 触发第一级定时器
		tm.fire()       // 投递已触发的定时器
		return
	}

	for tm.now+TickInterval <= nowMs {
		tm.now += TickInterval
		tm.ticks++
		tm.trigger0()
		tm.fire()
	}
}

// trigger0 触发第一级的定时器
func (tm *TimerWheels) trigger0() int {
	var wheel = &tm.wheels[0] // L1 wheel
	var timers = wheel.Pop(int(wheel.current))
	for _, node := range timers {
		if node.expire <= tm.now {
			tm.delRef(node.tid)
			freeTimerNode(node)
			tm.fired = append(tm.fired, node.tid)
		} else {
			logger.Errorf("TimerWheels: timer %d unexpected timeout=%d", node.tid, node.expire)
			tm.addTimer(node)
		}
	}
	wheel.current = (wheel.current + 1) % wheel.slots
	if wheel.current == 0 {
		return tm.cascade(1)
	}
	return 0
}

// cascade 级联更高层的时间轮
func (tm *TimerWheels) cascade(idx int) int {
	var wheel = &tm.wheels[idx]
	if isDebugTimer {
		logger.Debugf("TimerWheels: cascade wheel %d of slots=%d at ticks=%d", idx+1, wheel.slots, tm.ticks)
	}
	if testBit(wheel.bitmap, int(wheel.current)) {
		var timers = wheel.Pop(int(wheel.current))
		for _, node := range timers {
			tm.addTimer(node)
		}
	}
	wheel.current = (wheel.current + 1) % wheel.slots
	if wheel.current == 0 && idx+1 < len(tm.wheels) {
		return tm.cascade(idx + 1)
	}
	return idx
}

// cascadeAll 检查所有定时器（用于调试环境下的时间跳变场景）
func (tm *TimerWheels) cascadeAll() {
	for lv := 0; lv < len(tm.wheels); lv++ {
		var wheel = &tm.wheels[lv]
		for slot := 0; slot < int(wheel.slots); slot++ {
			if !testBit(wheel.bitmap, slot) {
				continue
			}
			var timers = wheel.Pop(slot)
			for _, node := range timers {
				tm.addTimer(node)
			}
		}
	}
}

const (
	OpAddTimer = iota + 1
	OpDelTimer
)

// 实现timer在worker线程里的添加、删除操作
type timerOp struct {
	type_ int
	node  *TimerNode
}

// easy to mock clock source
var currentTimeMillis = func() int64 {
	return time.Now().UnixMilli()
}

func align(ts, tick int64) int64 {
	return (ts / tick) * tick
}

// 优化TimerNode分配的对象池
var nodePool = sync.Pool{
	New: func() any {
		return new(TimerNode)
	},
}

// allocTimerNode 分配TimerNode对象
func allocTimerNode() *TimerNode {
	return nodePool.Get().(*TimerNode)
}

// freeTimerNode 归还TimerNode到对象池
func freeTimerNode(node *TimerNode) {
	nodePool.Put(node)
}

func setBit(mask []uint64, i int) {
	var v = uint64(1) << (i & 63)
	mask[i>>6] |= v
}

func testBit(mask []uint64, i int) bool {
	return mask[i>>6]&(1<<(i&63)) != 0
}

func clearBit(mask []uint64, i int) {
	var v = uint64(1) << (i & 63)
	mask[i>>6] &= ^v // 这里不进行边界检查
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
