package timers

import (
	"context"
	"math/rand"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func pollExpiredTimers(sched *TimerWheels) (expired []int64) {
	for {
		select {
		case tid := <-sched.TimedOutChan():
			expired = append(expired, tid)
		default:
			return
		}
	}
}

func TestTimerSched_Start(t *testing.T) {
	var sched = NewTimerWheels(64)
	defer sched.Shutdown()

	assert.True(t, sched.running.Load() == 0)

	sched.Start()

	assert.True(t, sched.running.Load() == 1)
}

func TestTimerSched_Basic(t *testing.T) {
	var sched = NewTimerWheels(64)
	defer sched.Shutdown()
	sched.Start()

	var tid int64 = 1001
	sched.AddTimer(tid, 100)

	<-time.NewTimer(time.Millisecond).C // wait worker

	assert.Equal(t, 1, sched.Size())
	assert.True(t, sched.IsPending(tid))

	// 等待触发
	select {
	case expiredID := <-sched.TimedOutChan():
		assert.Equal(t, tid, expiredID)
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Timer failed to trigger in time")
	}

	assert.Equal(t, 0, sched.Size())
	assert.False(t, sched.IsPending(tid))
}

func TestTimerSched_IsPending(t *testing.T) {
	var sched = NewTimerWheels(64)
	defer sched.Shutdown()
	sched.Start()

	sched.AddTimer(1, 0)
	sched.AddTimer(2, 100)
	sched.AddTimer(3, 800)
	sched.AddTimer(4, 1000000000000)

	<-time.NewTimer(time.Millisecond).C // wait worker

	assert.Equal(t, 4, sched.Size())
	assert.True(t, sched.IsPending(3))
	assert.True(t, sched.IsPending(4))
	assert.False(t, sched.IsPending(12345))

	<-time.NewTimer(200 * time.Millisecond).C

	assert.False(t, sched.IsPending(1))
	assert.False(t, sched.IsPending(2))
	assert.True(t, sched.IsPending(3))
	assert.True(t, sched.IsPending(4))
	assert.Equal(t, 2, sched.Size())
}

func TestTimerSched_AddTimerAt(t *testing.T) {
	var sched = NewTimerWheels(64)
	defer sched.Shutdown()
	sched.Start()

	var now = time.Now().UnixMilli()
	sched.AddTimerAt(1, now)
	sched.AddTimerAt(2, now+100)
	sched.AddTimerAt(3, now+600)

	<-time.NewTimer(time.Millisecond).C

	assert.True(t, sched.IsPending(1))
	assert.True(t, sched.IsPending(2))
	assert.True(t, sched.IsPending(3))
	assert.Equal(t, 3, sched.Size())

	<-time.NewTimer(50 * time.Millisecond).C
	var timedOut = pollExpiredTimers(sched)
	require.Equal(t, 1, len(timedOut))
	require.Equal(t, int64(1), timedOut[0])

	assert.Equal(t, 2, sched.Size())
	assert.False(t, sched.IsPending(1))
	assert.True(t, sched.IsPending(2))
	assert.True(t, sched.IsPending(3))

	<-time.NewTimer(700 * time.Millisecond).C
	timedOut = pollExpiredTimers(sched)
	require.Equal(t, 2, len(timedOut))
	require.Equal(t, int64(2), timedOut[0])
	require.Equal(t, int64(3), timedOut[1])

	assert.Equal(t, 0, sched.Size())
	assert.False(t, sched.IsPending(1))
	assert.False(t, sched.IsPending(2))
	assert.False(t, sched.IsPending(3))
}

func TestTimerSched_Cancel(t *testing.T) {
	var sched = NewTimerWheels(64)
	defer sched.Shutdown()
	sched.Start()

	sched.AddTimer(1, 0)
	sched.AddTimer(2, 150)
	sched.AddTimer(3, 500)

	<-time.NewTimer(time.Millisecond).C

	assert.True(t, sched.IsPending(1))
	assert.True(t, sched.IsPending(2))
	assert.True(t, sched.IsPending(3))
	assert.Equal(t, 3, sched.Size())

	assert.True(t, sched.Cancel(2))
	var timeouts = pollExpiredTimers(sched)
	assert.Equal(t, 2, sched.Size())
	assert.Equal(t, 0, len(timeouts))

	<-time.NewTimer(50 * time.Millisecond).C
	assert.True(t, sched.IsPending(3))
	assert.False(t, sched.Cancel(2))
	assert.True(t, sched.Cancel(3))
	assert.Equal(t, 0, sched.Size())
}

func TestTimerSched_TimePast(t *testing.T) {
	var clock = time.Now()
	currentTimeMillis = func() int64 {
		return clock.UnixMilli()
	}
	enableTimePast = true
	var sched = NewTimerWheels(10240)
	defer sched.Shutdown()
	sched.Start()

	var startId int64 = 1
	const N = 100000
	for i := 0; i < N; i++ {
		var interval = rand.Int() % 5000000
		sched.AddTimer(startId+int64(i), int64(interval))
	}

	<-time.NewTimer(time.Millisecond).C
	assert.Equal(t, N, sched.Size())

	// fast pass clock
	clock = clock.Add(time.Hour * 100)

	var fired = 0
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	for {
		select {
		case tid := <-sched.TimedOutChan():
			if tid > 0 {
				fired++
			}
		case <-ctx.Done():
			assert.Equal(t, N, fired)
			return
		}
	}
}

func TestTimerSched_StableTrigger(t *testing.T) {
	var clock = time.Now()
	currentTimeMillis = func() int64 {
		return clock.UnixMilli()
	}
	stableTrigger = true
	var sched = NewTimerWheels(64)
	defer sched.Shutdown()
	sched.Start()

	const N = 100
	var startId int64 = 1
	for i := 0; i < N; i++ {
		sched.AddTimer(startId+int64(i), 1000)
	}

	<-time.NewTimer(time.Millisecond).C
	assert.Equal(t, N, sched.Size())

	clock = clock.Add(time.Hour)

	var fired []int64
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	for {
		select {
		case tid := <-sched.TimedOutChan():
			if tid > 0 {
				fired = append(fired, tid)
			}
		case <-ctx.Done():
			assert.True(t, slices.IsSorted(fired))
			return
		}
	}
}
