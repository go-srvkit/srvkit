package datetime

import (
	"math"
	"strings"
	"testing"
	"time"
)

func TestParseDurationBounds(t *testing.T) {
	for _, tc := range []struct {
		text string
		want time.Duration
	}{
		{"-9223372036854775808ns", time.Duration(math.MinInt64)},
		{"-2562047h47m16.854775808s", time.Duration(math.MinInt64)},
		{"-106751d23h47m16.854775808s", time.Duration(math.MinInt64)},
		{"9223372036854775807ns", time.Duration(math.MaxInt64)},
		{"106751d23h47m16.854775807s", time.Duration(math.MaxInt64)},
		{"0", 0}, {"-0", 0}, {"+0", 0}, {".5d", 12 * time.Hour},
		{"1μs", time.Microsecond}, {"0.0000000000000000000001d", 0},
		// Long fractions truncate excess precision without wrapping the accumulator.
		{"0.100000000000000000000000000000s", 100 * time.Millisecond},
		{"0.922337203685477580900000000000s", 922337203 * time.Nanosecond},
		{"+1.d", Day},
	} {
		got, err := ParseDuration(tc.text)
		if err != nil || got != tc.want {
			t.Errorf("ParseDuration(%q) = (%v, %v), want %v", tc.text, got, err, tc.want)
		}
	}
	for _, text := range []string{
		"", "-", "+", "s", ".s", "1", "1w", "1h-1m", "1..2s", "1 s", "1d\n",
		"9223372036854775808ns", "-9223372036854775809ns",
		"106751d23h47m16.854775808s", "-106751d23h47m16.854775809s",
		"9223372036854775807ns1ns", "-9223372036854775808ns1ns",
		"9223372036854775808ns9223372036854775808ns",
		"-9223372036854775808ns9223372036854775808ns",
		"999999999999999999999999999999999999ns", "106752d",
		// Fractional overflow inside one component, before total accumulation.
		"9223372036.854775809s",
	} {
		if got, err := ParseDuration(text); err == nil {
			t.Errorf("ParseDuration(%q) = %v without an error", text, got)
		}
	}
}

func TestPrettyTimeBounds(t *testing.T) {
	for _, tc := range []struct {
		ms   int64
		want string
	}{
		{0, "0s"}, {1, "1ms"}, {-1, "-1ms"}, {999, "999ms"},
		{1000, "1s"}, {1001, "1.001s"}, {100000, "1m40s"},
		{MsPerDay, "1d"}, {MsPerDay + 1, "1d0.001s"},
		{MsPerDay + MsPerMinute, "1d1m"},
		{9223372036855, "106751d23h47m16.855s"},
		{-9223372036855, "-106751d23h47m16.855s"},
		{math.MaxInt64, "106751991167d7h12m55.807s"},
		{math.MinInt64, "-106751991167d7h12m55.808s"},
	} {
		if got := PrettyTime(tc.ms); got != tc.want {
			t.Errorf("PrettyTime(%d) = %q, want %q", tc.ms, got, tc.want)
		}
	}
}

func FuzzDurationRoundTrip(f *testing.F) {
	for _, ns := range []int64{0, 1, -1, 999, 1000, 999999999, 1000000000,
		int64(Day), int64(Day + time.Millisecond), math.MinInt64, math.MaxInt64} {
		f.Add(ns)
	}
	f.Fuzz(func(t *testing.T, ns int64) {
		d := time.Duration(ns)
		for _, text := range []string{PrettyDuration(d), d.String()} {
			got, err := ParseDuration(text)
			if err != nil || got != d {
				t.Fatalf("round trip of %d through %q = (%v, %v)", ns, text, got, err)
			}
		}
		ms := ns / int64(time.Millisecond)
		if got, want := PrettyTime(ms), PrettyDuration(time.Duration(ms)*time.Millisecond); got != want {
			t.Fatalf("PrettyTime(%d) = %q, want %q", ms, got, want)
		}
	})
}

func FuzzParseDuration(f *testing.F) {
	for _, text := range []string{"", "1h30m", "1μs", "0.333333333333333333h",
		"-9223372036854775808ns", "9223372036854775808ns", "1h-1m"} {
		f.Add(text)
	}
	f.Fuzz(func(t *testing.T, text string) {
		if len(text) > 4096 || strings.Contains(text, "d") {
			return // The standard library does not support the day extension.
		}
		got, err := ParseDuration(text)
		want, wantErr := time.ParseDuration(text)
		if (err == nil) != (wantErr == nil) || err == nil && got != want {
			t.Fatalf("ParseDuration(%q) = (%v, %v), stdlib = (%v, %v)", text, got, err, want, wantErr)
		}
	})
}
