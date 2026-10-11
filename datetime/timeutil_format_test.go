package datetime

import (
	"testing"
	"time"
)

func TestFormatTimeLayouts(t *testing.T) {
	for _, tc := range []struct {
		name, input, local, utc string
	}{
		{"positiveOffset", "2026-01-01T01:02:03.123456789+08:00", "2026-01-01T01:02:03.123", "2025-12-31T17:02:03.123"},
		{"negativeOffset", "2026-01-01T23:02:03.999999999-05:00", "2026-01-01T23:02:03.999", "2026-01-02T04:02:03.999"},
		{"epoch", "1970-01-01T00:00:00Z", "1970-01-01T00:00:00.000", "1970-01-01T00:00:00.000"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := parseTestTime(t, tc.input)
			if got := FormatTime(input); got != tc.local {
				t.Errorf("FormatTime = %q, want %q", got, tc.local)
			}
			if got := FormatTimeUTC(input); got != tc.utc {
				t.Errorf("FormatTimeUTC = %q, want %q", got, tc.utc)
			}
		})
	}
	if got := FormatTime(time.Time{}); got != "0001-01-01T00:00:00.000" {
		t.Errorf("FormatTime(zero) = %q", got)
	}
}

func TestFormatMilliseconds(t *testing.T) {
	for _, tc := range []struct {
		ms   int64
		want string
	}{
		{0, "0001-01-01T00:00:00.000"},
		{1, "1970-01-01T00:00:00.001"},
		{-1, "1969-12-31T23:59:59.999"},
		{946684800123, "2000-01-01T00:00:00.123"},
	} {
		for _, format := range []struct {
			name string
			fn   func(int64) string
		}{{"FormatMsTime", FormatMsTime}, {"FormatMsTimeUTC", FormatMsTimeUTC}} {
			if got := format.fn(tc.ms); got != tc.want {
				t.Errorf("%s(%d) = %q, want %q", format.name, tc.ms, got, tc.want)
			}
		}
	}
}

func TestFormatNanoseconds(t *testing.T) {
	// FormatNanoTime intentionally uses time.Local; make that zone deterministic.
	oldLocal := time.Local
	time.Local = time.FixedZone("test", 8*3600)
	t.Cleanup(func() { time.Local = oldLocal })
	for _, tc := range []struct {
		ns         int64
		local, utc string
	}{
		{0, "1970-01-01T08:00:00.000000", "1970-01-01T00:00:00.000000"},
		{123456789, "1970-01-01T08:00:00.123456", "1970-01-01T00:00:00.123456"},
		{-1, "1970-01-01T07:59:59.999999", "1969-12-31T23:59:59.999999"},
	} {
		if got := FormatNanoTime(tc.ns); got != tc.local {
			t.Errorf("FormatNanoTime(%d) = %q, want %q", tc.ns, got, tc.local)
		}
		if got := FormatNanoTimeUTC(tc.ns); got != tc.utc {
			t.Errorf("FormatNanoTimeUTC(%d) = %q, want %q", tc.ns, got, tc.utc)
		}
	}
}

func TestLeapYears(t *testing.T) {
	for _, tc := range []struct {
		year int
		want bool
	}{{1600, true}, {1900, false}, {2000, true}, {2023, false}, {2024, true}, {2100, false}, {2400, true}} {
		if got := IsLeapYear(tc.year); got != tc.want {
			t.Errorf("IsLeapYear(%d) = %v, want %v", tc.year, got, tc.want)
		}
	}
}

func TestDaysCountOfMonthCalendar(t *testing.T) {
	for _, year := range []int{1900, 2000, 2023, 2024, 2100} {
		for month := 1; month <= 12; month++ {
			// The last day of each month is an independent standard-library oracle.
			want := time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC).Day()
			if got := DaysCountOfMonth(year, month); got != want {
				t.Errorf("DaysCountOfMonth(%d, %d) = %d, want %d", year, month, got, want)
			}
		}
	}
	for _, month := range []int{-1, 0, 13} {
		if got := DaysCountOfMonth(2024, month); got != 0 {
			t.Errorf("invalid month %d returned %d days", month, got)
		}
	}
	// Preserve the existing non-positive-year convention for February.
	for _, year := range []int{-4, 0} {
		if got := DaysCountOfMonth(year, 2); got != 28 {
			t.Errorf("DaysCountOfMonth(%d, 2) = %d, want 28", year, got)
		}
	}
}
