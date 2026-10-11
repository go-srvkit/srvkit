package timers

import (
	"math"
	"math/rand"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type timerTestClock struct {
	wall     atomic.Int64
	internal atomic.Int64
}

func (c *timerTestClock) read() (int64, int64) {
	return c.wall.Load(), c.internal.Load()
}

func (c *timerTestClock) set(now int64) {
	c.wall.Store(now)
	c.internal.Store(now)
}

func newTestWheels(t *testing.T, now int64) (*TimerWheels, *timerTestClock) {
	t.Helper()
	tm := NewTimerWheels(64)
	clock := new(timerTestClock)
	clock.set(now)
	tm.clock = clock.read
	tm.now = align(now, TickInterval)
	tm.ticks = tm.now / TickInterval
	tm.stable = false
	tm.debug = false
	tm.setCurrent()
	t.Cleanup(tm.Shutdown)
	return tm, clock
}

func assertIDs(t *testing.T, got, want []int64) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Fatalf("expired IDs = %v, want %v", got, want)
	}
}

func checkWheelState(t *testing.T, tm *TimerWheels) {
	t.Helper()
	tm.guard1.RLock()
	defer tm.guard1.RUnlock()
	total := 0
	for level := range tm.wheels {
		wheel := &tm.wheels[level]
		count := 0
		for slot, bucket := range wheel.buckets {
			if testBit(wheel.bitmap, slot) != (len(bucket) != 0) {
				t.Fatalf("wheel %d slot %d: inconsistent bitmap", level+1, slot)
			}
			for index, node := range bucket {
				if node == nil || node.wheel != int32(level+1) || node.slot != int32(slot) || node.index != index || tm.ref[node.tid] != node {
					t.Fatalf("wheel %d slot %d index %d: invalid node %+v", level+1, slot, index, node)
				}
			}
			for _, node := range bucket[len(bucket):cap(bucket)] {
				if node != nil {
					t.Fatal("unused bucket capacity retains a node")
				}
			}
			count += len(bucket)
		}
		if count != wheel.size {
			t.Fatalf("wheel %d size = %d, actual %d", level+1, wheel.size, count)
		}
		total += count
	}
	if total != len(tm.ref) {
		t.Fatalf("wheel count = %d, registry count = %d", total, len(tm.ref))
	}
}

func waitTimerCondition(t *testing.T, predicate func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !predicate() {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for scheduler state")
		}
		runtime.Gosched()
	}
}

func TestTimerSched_Start(t *testing.T) {
	tm := NewTimerWheels(64)
	defer tm.Shutdown()
	tm.Start()
	tm.Start()
	tm.guard1.RLock()
	running := tm.running
	tm.guard1.RUnlock()
	if !running {
		t.Fatal("worker did not start")
	}
}

func TestTimerSched_Basic(t *testing.T) {
	tm := NewTimerWheels(64)
	defer tm.Shutdown()
	tm.AddTimer(1001, 100)
	if !tm.IsPending(1001) || tm.Size() != 1 {
		t.Fatal("timer must be pending immediately, before Start")
	}
	tm.Start()
	select {
	case id := <-tm.TimedOutChan():
		if id != 1001 {
			t.Fatalf("expired ID = %d", id)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timer did not fire")
	}
	if tm.IsPending(1001) || tm.Size() != 0 {
		t.Fatal("expired timer is still pending")
	}
}

func TestTimerSched_IsPending(t *testing.T) {
	tm, clock := newTestWheels(t, 0)
	for id, delay := range map[int64]int64{1: 0, 2: 100, 3: 800, 4: 1000000000000} {
		tm.AddTimer(id, delay)
		if !tm.IsPending(id) {
			t.Fatalf("timer %d not pending", id)
		}
	}
	if tm.IsPending(12345) || tm.Size() != 4 {
		t.Fatal("incorrect registry state")
	}
	clock.set(200)
	assertIDs(t, tm.update(200), []int64{1, 2})
	if tm.IsPending(1) || tm.IsPending(2) || !tm.IsPending(3) || !tm.IsPending(4) || tm.Size() != 2 {
		t.Fatal("incorrect registry state after expiry")
	}
	checkWheelState(t, tm)
}

func TestTimerSched_AddTimerAt(t *testing.T) {
	tm, clock := newTestWheels(t, 1000)
	tm.AddTimerAt(1, 900)
	tm.AddTimerAt(2, 1100)
	tm.AddTimerAt(3, 1600)
	clock.set(1050)
	assertIDs(t, tm.update(1050), []int64{1})
	clock.set(1700)
	assertIDs(t, tm.update(1700), []int64{2, 3})
	checkWheelState(t, tm)
}

func TestTimerSched_Cancel(t *testing.T) {
	tm, clock := newTestWheels(t, 0)
	tm.AddTimer(1, 0)
	tm.AddTimer(2, 150)
	tm.AddTimer(3, 500)
	if !tm.Cancel(2) || tm.Cancel(2) || tm.Size() != 2 {
		t.Fatal("Cancel did not synchronously remove the timer")
	}
	checkWheelState(t, tm)
	clock.set(50)
	assertIDs(t, tm.update(50), []int64{1})
	if tm.Cancel(1) || !tm.Cancel(3) || tm.Size() != 0 {
		t.Fatal("incorrect cancellation result")
	}
	assertIDs(t, tm.update(1000), nil)
	checkWheelState(t, tm)
}

func TestTimerSched_TimePast(t *testing.T) {
	tm, clock := newTestWheels(t, 0)
	const n = 100000
	for i := 0; i < n; i++ {
		tm.AddTimer(int64(i+1), int64(i%5000000))
	}
	// Keep a future timer across the rebuild, including a nonzero wheel phase.
	jump := int64(100*time.Hour/time.Millisecond) + 12350
	tm.AddTimerAt(n+1, jump+LVL2_ITV)
	clock.set(jump)
	got := tm.update(jump)
	if len(got) != n || tm.Size() != 1 || !tm.IsPending(n+1) {
		t.Fatalf("fired=%d pending=%d", len(got), tm.Size())
	}
	slices.Sort(got)
	for i, id := range got {
		if id != int64(i+1) {
			t.Fatalf("lost or duplicate ID at %d: %d", i, id)
		}
	}
	checkWheelState(t, tm)
	clock.set(jump + LVL2_ITV)
	assertIDs(t, tm.update(clock.internal.Load()), []int64{n + 1})
	checkWheelState(t, tm)
}

func TestTimerSched_StableTrigger(t *testing.T) {
	for _, jump := range []int64{1000, 3600000} {
		tm, clock := newTestWheels(t, 0)
		tm.stable = true
		const n = 100
		for i := n; i > 0; i-- {
			tm.AddTimer(int64(i), 1000)
		}
		clock.set(jump)
		got := tm.update(jump)
		if len(got) != n || !slices.IsSorted(got) {
			t.Fatalf("stable trigger produced %d IDs: %v", len(got), got)
		}
		for i, id := range got {
			if id != int64(i+1) {
				t.Fatalf("missing/duplicate ID at %d: %d", i, id)
			}
		}
	}
}

func TestTimerWheelsTickBoundaries(t *testing.T) {
	cases := []struct{ start, delay int64 }{
		{0, 0}, {0, 1}, {0, 49}, {0, 50}, {0, 51}, {0, 100},
		{0, LVL2_ITV - 1}, {0, LVL2_ITV}, {0, LVL2_ITV + 1},
		{50, LVL2_ITV}, {10000, 30000}, {25550, LVL2_ITV},
		{25600, LVL2_ITV}, {30000, 30000},
		{0, LVL3_ITV - 1}, {0, LVL3_ITV}, {0, LVL3_ITV + 1},
		{10000, LVL3_ITV},
	}
	for _, tc := range cases {
		tm, clock := newTestWheels(t, tc.start)
		tm.AddTimer(1, tc.delay)
		deadline := tc.start + tc.delay
		due := max(tc.start/TickInterval+1, (deadline+TickInterval-1)/TickInterval) * TickInterval
		// Stay within the catch-up budget to exercise cascading, not rebuilding.
		for now := tc.start + TickInterval; now < due; now += TickInterval {
			clock.set(now)
			if got := tm.update(now); len(got) != 0 {
				t.Fatalf("start=%d delay=%d: early expiry at %d: %v", tc.start, tc.delay, now, got)
			}
		}
		clock.set(due)
		got := tm.update(due)
		if !slices.Equal(got, []int64{1}) {
			t.Fatalf("start=%d delay=%d: missing expiry at %d: %v", tc.start, tc.delay, due, got)
		}
		checkWheelState(t, tm)
	}
}

func TestTimerWheelsCatchUpOrder(t *testing.T) {
	for _, stable := range []bool{false, true} {
		tm, _ := newTestWheels(t, 0)
		tm.stable = stable
		tm.AddTimer(3, 50)
		tm.AddTimer(2, 100)
		tm.AddTimer(1, 100000)
		assertIDs(t, tm.update(100000), []int64{3, 2, 1})
	}
}

func TestTimerWheelsUpperRotations(t *testing.T) {
	tm, clock := newTestWheels(t, 0)
	horizon := int64(LVL4_ITV) * LVL4_BUCKET_SIZE
	deadline := horizon + LVL4_ITV + 50
	tm.AddTimerAt(1, deadline)
	tm.AddTimerAt(2, deadline+50)
	// Skip empty lower-level ticks. The first L4 visit is still before expiry,
	// so reinserting into the popped L4 slot must preserve both nodes.
	tm.ticks = LVL4_ITV/TickInterval - 1
	tm.now = tm.ticks * TickInterval
	tm.setCurrent()
	clock.set(LVL4_ITV)
	assertIDs(t, tm.update(LVL4_ITV), nil)
	checkWheelState(t, tm)
	if tm.wheels[3].size != 2 {
		t.Fatal("distant timers were lost during top-wheel rotation")
	}
	tm.ticks = (horizon+LVL4_ITV)/TickInterval - 1
	tm.now = tm.ticks * TickInterval
	tm.setCurrent()
	assertIDs(t, tm.update(horizon+LVL4_ITV), nil)
	checkWheelState(t, tm)
	assertIDs(t, tm.update(deadline), []int64{1})
	assertIDs(t, tm.update(deadline+50), []int64{2})
	checkWheelState(t, tm)
}

func TestTimerWheelsDuplicateID(t *testing.T) {
	tm, clock := newTestWheels(t, 0)
	tm.AddTimer(7, 0)
	tm.AddTimer(7, 1000)
	if tm.Size() != 1 {
		t.Fatal("replacement created two pending timers")
	}
	assertIDs(t, tm.update(50), nil)
	if !tm.IsPending(7) {
		t.Fatal("old deadline removed the replacement")
	}
	clock.set(1000)
	assertIDs(t, tm.update(1000), []int64{7})
	tm.AddTimer(7, 100)
	if !tm.Cancel(7) {
		t.Fatal("replacement could not be cancelled")
	}
	checkWheelState(t, tm)
	assertIDs(t, tm.update(2000), nil)
}

func TestTimerWheelsClockSemantics(t *testing.T) {
	tm, clock := newTestWheels(t, 1000)
	tm.AddTimer(1, 100)
	tm.AddTimerAt(2, 1200)
	clock.wall.Store(-3600000) // wall clock rolls back; internal time keeps moving
	clock.internal.Store(1100)
	assertIDs(t, tm.update(1100), []int64{1})
	clock.wall.Store(3600000)
	clock.internal.Store(1200)
	assertIDs(t, tm.update(1200), []int64{2})
	tm.AddTimerAt(3, 3600100)
	if tm.ref[3].tick != 1300/TickInterval {
		t.Fatalf("absolute deadline was not converted: tick %d", tm.ref[3].tick)
	}
	assertIDs(t, tm.update(1250), nil)
	assertIDs(t, tm.update(1300), []int64{3})
	// Logical-clock mode can move backwards; it must not move wheel cursors back.
	assertIDs(t, tm.update(0), nil)
	if tm.now != 1300 {
		t.Fatal("scheduler moved backwards")
	}
}

func TestTimerWheelsOverflow(t *testing.T) {
	tm, _ := newTestWheels(t, 1000)
	tm.AddTimer(1, math.MaxInt64)
	tm.AddTimerAt(2, math.MinInt64)
	tm.AddTimer(3, math.MinInt64)
	got := tm.update(1050)
	slices.Sort(got)
	assertIDs(t, got, []int64{2, 3})
	if !tm.IsPending(1) || tm.ref[1].tick != math.MaxInt64/TickInterval+1 {
		t.Fatal("large positive delay wrapped around")
	}
	checkWheelState(t, tm)
	for _, tc := range []struct{ a, b, sum, diff int64 }{
		{math.MaxInt64, 1, math.MaxInt64, math.MaxInt64 - 1},
		{math.MinInt64, -1, math.MinInt64, math.MinInt64 + 1},
		{0, math.MinInt64, math.MinInt64, math.MaxInt64},
		{0, math.MaxInt64, math.MaxInt64, -math.MaxInt64},
	} {
		if saturatedAdd(tc.a, tc.b) != tc.sum || saturatedSub(tc.a, tc.b) != tc.diff {
			t.Fatalf("incorrect saturated arithmetic for %d, %d", tc.a, tc.b)
		}
	}
}

func TestTimerWheelsFullOutputShutdown(t *testing.T) {
	tm, clock := newTestWheels(t, 0)
	tm.C = make(chan int64, 1) // set before Start, never mutate a running output
	tm.AddTimer(1, 0)
	tm.AddTimer(2, 0)
	tm.AddTimer(3, 1000)
	clock.set(50)
	tm.Start()
	waitTimerCondition(t, func() bool { return len(tm.C) == 1 && tm.Size() == 1 })
	// Cancel must remain synchronous and available while delivery is blocked.
	if !tm.Cancel(3) {
		t.Fatal("could not cancel while output was blocked")
	}
	stopped := make(chan struct{})
	go func() { tm.Shutdown(); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(3 * time.Second):
		t.Fatal("Shutdown blocked behind the full output channel")
	}
	if tm.Size() != 0 {
		t.Fatal("Shutdown retained pending timers")
	}
	if _, ok := <-tm.C; !ok {
		t.Fatal("buffered ID was discarded")
	}
	if _, ok := <-tm.C; ok {
		t.Fatal("output channel was not closed")
	}
	for _, wheel := range tm.wheels {
		if wheel.buckets != nil || wheel.bitmap != nil || wheel.size != 0 {
			t.Fatal("Shutdown retained wheel storage")
		}
	}
}

func TestTimerWheelsLifecycle(t *testing.T) {
	var zero TimerWheels
	zero.Shutdown()
	for _, start := range []bool{false, true} {
		tm := NewTimerWheels(64)
		tm.AddTimer(1, 1000000000000)
		if start {
			tm.Start()
		}
		tm.Shutdown()
		tm.Shutdown()
		tm.Start()
		tm.AddTimer(2, 0)
		tm.AddTimerAt(3, 0)
		if tm.Cancel(1) || tm.Size() != 0 || tm.IsPending(2) {
			t.Fatal("stopped scheduler accepted a timer")
		}
		if _, ok := <-tm.C; ok {
			t.Fatal("stopped channel remains open")
		}
		tm.Init(8)
		tm.AddTimer(4, 0)
		tm.Start()
		select {
		case id := <-tm.C:
			if id != 4 {
				t.Fatalf("reinitialized scheduler fired %d", id)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("reinitialized scheduler failed to fire")
		}
		tm.Shutdown()
	}
}

func TestTimerWheelsConcurrentLifecycle(t *testing.T) {
	for i := 0; i < 200; i++ {
		tm := NewTimerWheels(1)
		var wg sync.WaitGroup
		for g := 0; g < 4; g++ {
			wg.Add(1)
			go func(g int) {
				defer wg.Done()
				if g%2 == 0 {
					tm.Start()
				} else {
					tm.Shutdown()
				}
				tm.AddTimer(int64(g), 0)
				tm.Cancel(int64(g))
				tm.Shutdown()
			}(g)
		}
		wg.Wait()
		if tm.Size() != 0 {
			t.Fatal("concurrent shutdown retained timers")
		}
	}
}

func TestTimerWheelsConcurrentAddCancel(t *testing.T) {
	tm, _ := newTestWheels(t, 0)
	tm.Start()
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 2000; i++ {
				id := int64(g*2000 + i)
				tm.AddTimer(id, 1000000000000)
				if !tm.IsPending(id) || !tm.Cancel(id) || tm.Cancel(id) {
					t.Errorf("inconsistent synchronous cancellation for %d", id)
					return
				}
				tm.Size()
			}
		}(g)
	}
	wg.Wait()
	checkWheelState(t, tm)
	if tm.Size() != 0 {
		t.Fatal("cancelled timers remain pending")
	}
}

func TestTimerWheelsCancelRacesExpiry(t *testing.T) {
	const n = 20000
	tm := NewTimerWheels(n)
	tm.Start()
	var cancelled sync.Map
	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < n/4; i++ {
				id := int64(g*(n/4) + i + 1)
				tm.AddTimer(id, 0)
				if tm.Cancel(id) {
					cancelled.Store(id, true)
				}
			}
		}(g)
	}
	wg.Wait()
	tm.Shutdown()
	seen := make(map[int64]bool)
	for id := range tm.C {
		if _, ok := cancelled.Load(id); ok {
			t.Fatalf("successfully cancelled timer %d fired", id)
		}
		if seen[id] {
			t.Fatalf("timer %d fired twice", id)
		}
		seen[id] = true
	}
}

func TestTimerWheelsCancelLinearization(t *testing.T) {
	tm, clock := newTestWheels(t, 0)
	for i := int64(1); i <= 1000; i++ {
		deadline := i * TickInterval
		tm.AddTimerAt(i, deadline)
		start := make(chan struct{})
		cancelled := make(chan bool, 1)
		expired := make(chan []int64, 1)
		go func() { <-start; cancelled <- tm.Cancel(i) }()
		go func() { <-start; expired <- tm.update(deadline) }()
		close(start)
		ok, got := <-cancelled, <-expired
		if ok {
			assertIDs(t, got, nil)
		} else {
			assertIDs(t, got, []int64{i})
		}
		clock.set(deadline)
	}
	checkWheelState(t, tm)
}

func TestTimerWheelsPoolReuseDuringExpiry(t *testing.T) {
	tm, _ := newTestWheels(t, 0)
	other, _ := newTestWheels(t, 0)
	const n = 10000
	for id := int64(1); id <= n; id++ {
		tm.AddTimerAt(id, 50)
	}
	start := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		<-start
		for i := 0; i < n*2; i++ {
			other.AddTimer(-1, 1000)
			other.Cancel(-1)
		}
	}()
	close(start)
	got := tm.update(50)
	<-done
	slices.Sort(got)
	if len(got) != n {
		t.Fatalf("fired %d timers, want %d", len(got), n)
	}
	for i, id := range got {
		if id != int64(i+1) {
			t.Fatalf("pooled node corrupted expiration ID at %d: %d", i, id)
		}
	}
	checkWheelState(t, tm)
	checkWheelState(t, other)
}

func TestTimerWheelsModel(t *testing.T) {
	tm, clock := newTestWheels(t, 10000)
	rng := rand.New(rand.NewSource(42))
	pending := make(map[int64]int64)
	now := int64(10000)
	for i := 0; i < 10000; i++ {
		id := int64(rng.Intn(512))
		switch rng.Intn(4) {
		case 0, 1:
			delay := int64(rng.Intn(3*LVL3_ITV)) - 100
			if i%5 == 0 {
				delay = int64(rng.Intn(1000)) - 100
			}
			tm.AddTimerAt(id, now+delay)
			deadline := max(now, now+delay)
			pending[id] = max(now/TickInterval+1, (deadline+TickInterval-1)/TickInterval)
		case 2:
			_, exists := pending[id]
			if tm.Cancel(id) != exists {
				t.Fatal("Cancel disagrees with reference model")
			}
			delete(pending, id)
		case 3:
			now += int64(rng.Intn(1000))
			if i%100 == 0 {
				now += LVL3_ITV
			}
			clock.set(now)
			var want []int64
			for id, due := range pending {
				if due <= now/TickInterval {
					want = append(want, id)
					delete(pending, id)
				}
			}
			got := tm.update(now)
			slices.Sort(got)
			slices.Sort(want)
			assertIDs(t, got, want)
		}
		if tm.Size() != len(pending) {
			t.Fatal("Size disagrees with reference model")
		}
		if i%100 == 0 {
			checkWheelState(t, tm)
		}
	}
	checkWheelState(t, tm)
}

func TestTimerWheelPopOwnership(t *testing.T) {
	var wheel TimerWheel
	wheel.Init(50, 512)
	old := &TimerNode{tid: 1}
	wheel.buckets[0] = TimerBucket{old}
	wheel.size = 1
	setBit(wheel.bitmap, 0)
	nodes := wheel.Pop(0)
	wheel.buckets[0] = append(wheel.buckets[0], &TimerNode{tid: 2})
	if nodes[0] != old || wheel.size != 0 || testBit(wheel.bitmap, 0) {
		t.Fatal("Pop did not detach its bucket")
	}
	wheel.recycle(0, nodes)
	if wheel.buckets[0][0].tid != 2 || nodes[0] != nil {
		t.Fatal("recycling overwrote a reinserted bucket")
	}
	wheel.Init(50, 512)
	if wheel.current != 0 || wheel.size != 0 {
		t.Fatal("Init retained counters")
	}
}
