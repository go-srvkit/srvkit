package datetime

import (
	"strconv"
	"testing"
	"time"
)

var (
	benchmarkDays int
	benchmarkText string
)

func BenchmarkElapsedDaysBetween(b *testing.B) {
	start := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, years := range []int{1, 100, 10000} {
		end := start.AddDate(years, 0, 0)
		b.Run(strconv.Itoa(years), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				benchmarkDays = ElapsedDaysBetween(start, end)
			}
		})
	}
}

func BenchmarkPrettyDuration(b *testing.B) {
	for _, d := range []time.Duration{123 * time.Microsecond, 4*Day + 123*time.Millisecond,
		time.Duration(1<<63 - 1)} {
		b.Run(d.String(), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				benchmarkText = PrettyDuration(d)
			}
		})
	}
}

func BenchmarkPrettyTime(b *testing.B) {
	for _, ms := range []int64{123, 4*MsPerDay + 123, 9223372036855} {
		b.Run(strconv.FormatInt(ms, 10), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				benchmarkText = PrettyTime(ms)
			}
		})
	}
}
