package timers

import (
	"testing"
	"time"
)

// A public-API workload representative of connection timeouts that are usually
// cancelled before expiry. It is also runnable against the previous implementation.
func BenchmarkTimerWheelsAddCancel(b *testing.B) {
	tm := NewTimerWheels(64)
	tm.Start()
	defer tm.Shutdown()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tm.AddTimer(int64(i), int64(time.Hour/time.Millisecond))
		if !tm.Cancel(int64(i)) {
			b.Fatal("timer unexpectedly expired")
		}
	}
	b.StopTimer()
}

func BenchmarkTimerWheelsSameTick(b *testing.B) {
	tm := NewTimerWheels(64)
	defer tm.Shutdown()
	tm.clock = func() (int64, int64) { return 0, 0 }
	tm.ticks = 0
	tm.now = 0
	tm.setCurrent()
	const n = 1024
	var fired []int64
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Match the synthetic clock to the last tick without allocating a closure.
		tm.guard1.Lock()
		for id := 0; id < n; id++ {
			tm.schedule(int64(id), tm.now+TickInterval)
		}
		fired = tm.advance(tm.now+TickInterval, fired[:0])
		tm.guard1.Unlock()
		if len(fired) != n {
			b.Fatal("lost expiration IDs")
		}
	}
	b.StopTimer()
}
