package datetime

import (
	"math"
	"sync"
	"testing"
	"time"
)

func parseTestTime(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestTimeOfMSBounds(t *testing.T) {
	if !TimeOfMS(0).IsZero() || !UTCTimeOfMS(0).IsZero() {
		t.Fatal("zero must retain the unset-time convention")
	}
	for _, ms := range []int64{-1, 1, math.MinInt64, math.MaxInt64,
		time.Date(1600, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		time.Date(2300, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()} {
		for _, got := range []time.Time{TimeOfMS(ms), UTCTimeOfMS(ms)} {
			if got.UnixMilli() != ms || got.Location() != time.UTC {
				t.Errorf("TimeOfMS(%d) = %v", ms, got)
			}
		}
	}
	if got := FormatMsTime(time.Date(2300, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()); got != "2300-01-01T00:00:00.000" {
		t.Errorf("FormatMsTime = %q", got)
	}
}

func TestNextMidnightBounds(t *testing.T) {
	for _, ts := range []int64{math.MinInt64, -MsPerDay, -1, 0, 1, MsPerDay,
		time.Date(2300, 1, 1, 12, 0, 0, 0, time.UTC).UnixMilli()} {
		got := NextMidnight(ts)
		if got <= ts || got%MsPerDay != 0 || got-ts > MsPerDay {
			t.Errorf("NextMidnight(%d) = %d, want the first later UTC midnight", ts, got)
		}
	}
	if got := NextMidnight(0); got != MsPerDay {
		t.Errorf("NextMidnight(epoch) = %d", got)
	}
	if got := NextMidnight(math.MaxInt64); got != math.MaxInt64 {
		t.Errorf("NextMidnight(MaxInt64) = %d", got)
	}
}

func TestCalendarUTC(t *testing.T) {
	oldMonday := FirstDayIsMonday
	t.Cleanup(func() { FirstDayIsMonday = oldMonday })
	for _, tc := range []struct {
		name string
		got  time.Time
		want string
	}{
		{"midnight", MidnightTimeOf(parseTestTime(t, "2026-01-01T01:00:00+08:00")), "2025-12-31T00:00:00Z"},
		{"nextMonday", GetNextMonday(parseTestTime(t, "2026-03-08T23:30:00-08:00")), "2026-03-16T00:00:00Z"},
		{"yearStart", FirstDayOfYear(2026), "2026-01-01T00:00:00Z"},
		{"yearLastDay", LastDayOfYear(2026), "2026-12-31T00:00:00Z"},
		{"afterDays", ThisMomentAfterDays(parseTestTime(t, "2026-03-07T12:00:00-05:00"), 1), "2026-03-08T17:00:00Z"},
		{"zeroDays", ThisMomentAfterDays(parseTestTime(t, "2026-03-07T12:00:00-05:00"), 0), "2026-03-07T17:00:00Z"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if !tc.got.Equal(parseTestTime(t, tc.want)) || tc.got.Location() != time.UTC {
				t.Errorf("got %v, want %s in UTC", tc.got, tc.want)
			}
		})
	}
	input := parseTestTime(t, "2026-03-07T23:30:00-05:00") // Sunday in UTC.
	for _, monday := range []bool{false, true} {
		FirstDayIsMonday = monday
		wantStart, wantEnd := "2026-03-08T00:00:00Z", "2026-03-14T23:59:59Z"
		if monday {
			wantStart, wantEnd = "2026-03-02T00:00:00Z", "2026-03-08T23:59:59Z"
		}
		if got := StartingOfWeek(input); !got.Equal(parseTestTime(t, wantStart)) || got.Location() != time.UTC {
			t.Errorf("StartingOfWeek, monday=%v: %v", monday, got)
		}
		if got := EndOfWeek(input); !got.Equal(parseTestTime(t, wantEnd)) || got.Location() != time.UTC {
			t.Errorf("EndOfWeek, monday=%v: %v", monday, got)
		}
	}
	start := time.Date(1, 1, 1, 12, 0, 0, 123, time.UTC)
	want := time.Date(274, 10, 17, 12, 0, 0, 123, time.UTC)
	if got := ThisMomentAfterDays(start, 100000); !got.Equal(want) {
		t.Errorf("100000 days later = %v, want %v", got, want)
	}
}

func TestElapsedDaysUTC(t *testing.T) {
	for _, tc := range []struct {
		start, end string
		want       int
	}{
		{"2026-01-02T01:00:00Z", "2026-01-01T20:00:00-08:00", 0},
		{"2026-01-01T23:00:00Z", "2026-01-02T07:00:00+08:00", 0},
		{"2026-01-01T23:30:00Z", "2026-01-02T00:30:00Z", 1},
		{"2026-03-07T23:30:00-05:00", "2026-03-08T23:30:00-04:00", 1},
		{"0001-01-01T00:00:00Z", "9999-12-31T23:59:59Z", 3652058},
		{"1969-12-31T23:59:59Z", "1970-01-01T00:00:00Z", 1},
	} {
		start, end := parseTestTime(t, tc.start), parseTestTime(t, tc.end)
		if got := ElapsedDaysBetween(start, end); got != tc.want {
			t.Errorf("ElapsedDaysBetween(%s, %s) = %d, want %d", tc.start, tc.end, got, tc.want)
		}
		if got := ElapsedDaysBetween(end, start); got != -tc.want {
			t.Errorf("reverse date difference = %d, want %d", got, -tc.want)
		}
	}
}

func TestOffsetBounds(t *testing.T) {
	old := GetOffset()
	t.Cleanup(func() { timeOffset.Store(old) })
	ResetOffset()
	for _, tc := range []struct{ add, want int64 }{
		{math.MaxInt64, math.MaxInt64}, {1, math.MaxInt64},
		{math.MinInt64, -1}, {math.MinInt64, math.MinInt64}, {-1, math.MinInt64},
		{math.MaxInt64, -1},
	} {
		AddOffset(tc.add)
		if got := GetOffset(); got != tc.want {
			t.Fatalf("after adding %d: got %d, want %d", tc.add, got, tc.want)
		}
	}
	for _, offset := range []int64{0, -1, 1, 9223372036855, -9223372036855, math.MinInt64, math.MaxInt64} {
		timeOffset.Store(offset)
		before := time.Now().UnixMilli()
		got, gotMS := Now(), NowMS()
		after := time.Now().UnixMilli()
		low, high := int64(math.MaxInt64), int64(math.MaxInt64)
		if offset != math.MaxInt64 {
			low, high = before+offset, after+offset
		}
		if got.Location() != time.UTC || got.UnixMilli() < low || got.UnixMilli() > high || gotMS < low || gotMS > high {
			t.Errorf("offset %d: Now=%v (%dms), NowMS=%d, want [%d,%d] in UTC", offset, got, got.UnixMilli(), gotMS, low, high)
		}
	}
}

func TestOffsetConcurrent(t *testing.T) {
	old := GetOffset()
	t.Cleanup(func() { timeOffset.Store(old) })
	ResetOffset()
	const workers, increments = 8, 1000
	var wg sync.WaitGroup
	start := make(chan struct{})
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for range increments {
				AddOffset(1)
				_ = GetOffset()
				_ = Now()
				_ = NowMS()
			}
		}()
	}
	close(start)
	wg.Wait()
	if got := GetOffset(); got != workers*increments {
		t.Fatalf("concurrent additions lost: got %d", got)
	}
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range increments {
				ResetOffset()
				AddOffset(1)
				_ = Now()
				_ = NowMS()
			}
		}()
	}
	wg.Wait()
}
