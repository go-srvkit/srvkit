// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package datetime

import (
	"math"
	"sync/atomic"
	"time"
)

const (
	DateLayout      = "2006-01-02T15:04:05"
	MsTimeLayout    = "2006-01-02T15:04:05.000"
	MicroTimeLayout = "2006-01-02T15:04:05.000000"

	MsPerSecond    = 1000
	MsPerMinute    = 60 * MsPerSecond
	MsPerHour      = 60 * MsPerMinute
	MsPerDay       = 24 * MsPerHour
	MsPerWeek      = 7 * MsPerDay
	SecondsPerMin  = 60
	SecondsPerHour = 60 * SecondsPerMin
	SecondsPerDay  = 24 * SecondsPerHour
)

var (
	// FirstDayIsMonday 只能在启动阶段、并发调用日期函数之前配置。
	FirstDayIsMonday = false

	timeOffset atomic.Int64 // 毫秒时间偏移
)

// ResetOffset 重置时间偏移，可并发调用。
func ResetOffset() {
	timeOffset.Store(0)
}

// AddOffset 增加毫秒时间偏移，可并发调用；溢出时钳制到 int64 边界。
func AddOffset(ms int64) {
	for {
		old := timeOffset.Load()
		if timeOffset.CompareAndSwap(old, addMilliseconds(old, ms)) {
			return
		}
	}
}

// GetOffset 获取毫秒时间偏移，可并发调用。
func GetOffset() int64 {
	return timeOffset.Load()
}

// Now 获取 UTC 当前时间（包含时间偏移），可并发调用。
// 偏移后的 Unix 毫秒时间戳溢出时钳制到 int64 边界，保留亚毫秒精度。
func Now() time.Time {
	now := time.Now()
	ms := addMilliseconds(now.UnixMilli(), timeOffset.Load())
	return time.UnixMilli(ms).Add(time.Duration(now.Nanosecond() % int(time.Millisecond))).UTC()
}

// NowMS 获取当前 Unix 毫秒时间戳（包含时间偏移），可并发调用。
// 溢出时钳制到 int64 边界。
func NowMS() int64 {
	return addMilliseconds(time.Now().UnixMilli(), timeOffset.Load())
}

func addMilliseconds(a, b int64) int64 {
	if b > 0 && a > math.MaxInt64-b {
		return math.MaxInt64
	}
	if b < 0 && a < math.MinInt64-b {
		return math.MinInt64
	}
	return a + b
}

func FormatTime(t time.Time) string {
	return t.Format(MsTimeLayout)
}

func FormatTimeUTC(t time.Time) string {
	return t.UTC().Format(MsTimeLayout)
}

func FormatMsTime(ms int64) string {
	return TimeOfMS(ms).Format(MsTimeLayout)
}

func FormatMsTimeUTC(ms int64) string {
	return UTCTimeOfMS(ms).Format(MsTimeLayout)
}

func FormatNanoTime(nano int64) string {
	return time.Unix(0, nano).Format(MicroTimeLayout)
}

func FormatNanoTimeUTC(nano int64) string {
	return time.Unix(0, nano).UTC().Format(MicroTimeLayout)
}

// TimeOfMS 将 Unix 毫秒时间戳转换为 UTC 时间。
// 为兼容现有调用，0 表示未设置，返回 time.Time{}，而非 Unix epoch。
func TimeOfMS(ms int64) time.Time {
	if ms == 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms).UTC()
}

func UTCTimeOfMS(ms int64) time.Time {
	return TimeOfMS(ms).UTC()
}

// IsLeapYear 是否闰年
func IsLeapYear(year int) bool {
	return (year%4 == 0 && year%100 != 0) || year%400 == 0
}

// MidnightTimeOf UTC 当日零点。
func MidnightTimeOf(t time.Time) time.Time {
	year, month, day := t.UTC().Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// NextMidnight 下一个 UTC 零点的 Unix 毫秒时间戳。
// 此函数的输入 0 表示 Unix epoch；超出 int64 范围时返回 math.MaxInt64。
func NextMidnight(ts int64) int64 {
	elapsed := ts % MsPerDay
	if elapsed < 0 {
		elapsed += MsPerDay
	}
	return addMilliseconds(ts, MsPerDay-elapsed)
}

// 各个星期到其下周一的天数
var weekdayOffsetDays = [7]int32{1, 7, 6, 5, 4, 3, 2}

// GetNextMonday 计算 UTC 时间的下周一零点；周一输入返回七天后的零点。
func GetNextMonday(now time.Time) time.Time {
	now = now.UTC()
	var elapsedDays = weekdayOffsetDays[now.Weekday()]
	return MidnightTimeOf(now).AddDate(0, 0, int(elapsedDays))
}

// ThisMomentAfterDays 按 UTC 日历计算 N 天后的这个时候，返回 UTC 时间。
func ThisMomentAfterDays(this time.Time, days int) time.Time {
	if days == 0 {
		return this.UTC()
	}
	return this.UTC().AddDate(0, 0, days)
}

// StartingOfWeek UTC 本周零点，周首由 FirstDayIsMonday 决定。
func StartingOfWeek(t time.Time) time.Time {
	t2 := MidnightTimeOf(t)
	weekday := int(t2.Weekday())
	if FirstDayIsMonday {
		if weekday == 0 {
			weekday = 7
		}
		weekday = weekday - 1
	}
	return t2.AddDate(0, 0, -weekday)
}

// EndOfWeek UTC 本周最后一天的 23:59:59，保留秒级精度。
func EndOfWeek(t time.Time) time.Time {
	begin := StartingOfWeek(t)
	end := ThisMomentAfterDays(begin, 7)
	return end.Add(-time.Second) // 23:59:59
}

// FirstDayOfYear 年度第一天
func FirstDayOfYear(year int) time.Time {
	return time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
}

// LastDayOfYear 年度最后一天
func LastDayOfYear(year int) time.Time {
	return time.Date(year, 12, 31, 0, 0, 0, 0, time.UTC)
}

// ElapsedDaysBetween 获取两个时间的 UTC 日历日期差，忽略日内时间。
// 返回值为 end 日期减 start 日期；不按 24 小时的经过时长计算。
func ElapsedDaysBetween(start, end time.Time) int {
	// UTC 零点的秒数是整日的倍数；先除后减避免大跨度的 Duration 溢出。
	startDay := MidnightTimeOf(start).Unix() / SecondsPerDay
	endDay := MidnightTimeOf(end).Unix() / SecondsPerDay
	return int(endDay - startDay)
}

// DaysCountOfMonth 一个月的天数
func DaysCountOfMonth(year, month int) int {
	switch time.Month(month) {
	case time.January:
		return 31
	case time.February:
		if year > 0 && IsLeapYear(year) {
			return 29
		}
		return 28
	case time.March:
		return 31
	case time.April:
		return 30
	case time.May:
		return 31
	case time.June:
		return 30
	case time.July:
		return 31
	case time.August:
		return 31
	case time.September:
		return 30
	case time.October:
		return 31
	case time.November:
		return 30
	case time.December:
		return 31
	}
	return 0
}
