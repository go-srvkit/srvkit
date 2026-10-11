// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package datetime

import (
	"errors"
	"time"
)

const (
	// Day 和 Week 表示固定经过时长，不代表本地时区的日历日或日历周。
	Day  = 24 * time.Hour
	Week = 7 * Day
)

var unitMap = map[string]uint64{
	"ns": uint64(time.Nanosecond),
	"us": uint64(time.Microsecond),
	"µs": uint64(time.Microsecond), // U+00B5 = micro symbol
	"μs": uint64(time.Microsecond), // U+03BC = Greek letter mu
	"ms": uint64(time.Millisecond),
	"s":  uint64(time.Second),
	"m":  uint64(time.Minute),
	"h":  uint64(time.Hour),
	"d":  uint64(Day),
}

// ParseDuration parses a duration string, this extends `time.ParseDuration` with days
// A duration string is a possibly signed sequence of
// decimal numbers, each with optional fraction and a unit suffix,
// such as "300ms", "-1.5h" or "2h45m".
// Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h", "d".
func ParseDuration(s string) (time.Duration, error) {
	// [-+]?([0-9]*(\.[0-9]*)?[a-z]+)+
	orig := s
	var d uint64
	neg := false

	// Consume [-+]?
	if s != "" {
		c := s[0]
		if c == '-' || c == '+' {
			neg = c == '-'
			s = s[1:]
		}
	}
	// Special case: if all that is left is "0", this is zero.
	if s == "0" {
		return 0, nil
	}
	if s == "" {
		return 0, errors.New("ParseDuration: invalid duration " + quote(orig))
	}
	for s != "" {
		var (
			v, f  uint64      // integers before, after decimal point
			scale float64 = 1 // value = v + f/scale
		)

		var err error

		// The next character must be [0-9.]
		if !(s[0] == '.' || '0' <= s[0] && s[0] <= '9') {
			return 0, errors.New("ParseDuration: invalid duration " + quote(orig))
		}
		// Consume [0-9]*
		pl := len(s)
		v, s, err = leadingInt(s)
		if err != nil {
			return 0, errors.New("ParseDuration: invalid duration " + quote(orig))
		}
		pre := pl != len(s) // whether we consumed anything before a period

		// Consume (\.[0-9]*)?
		post := false
		if s != "" && s[0] == '.' {
			s = s[1:]
			pl := len(s)
			f, scale, s = leadingFraction(s)
			post = pl != len(s)
		}
		if !pre && !post {
			// no digits (e.g. ".s" or "-.s")
			return 0, errors.New("ParseDuration: invalid duration " + quote(orig))
		}

		// Consume unit.
		i := 0
		for ; i < len(s); i++ {
			c := s[i]
			if c == '.' || '0' <= c && c <= '9' {
				break
			}
		}
		if i == 0 {
			return 0, errors.New("ParseDuration: missing unit in duration " + quote(orig))
		}
		u := s[:i]
		s = s[i:]
		unit, ok := unitMap[u]
		if !ok {
			return 0, errors.New("ParseDuration: unknown unit " + quote(u) + " in duration " + quote(orig))
		}
		if v > (1<<63)/unit {
			// overflow
			return 0, errors.New("ParseDuration: invalid duration " + quote(orig))
		}
		v *= unit
		if f > 0 {
			// float64 is needed to be nanosecond accurate for fractions of hours.
			// The largest unit is a fixed 24-hour day.
			v += uint64(float64(f) * (float64(unit) / scale))
			if v > 1<<63 {
				// overflow
				return 0, errors.New("ParseDuration: invalid duration " + quote(orig))
			}
		}
		if v > (1<<63)-d {
			// overflow
			return 0, errors.New("ParseDuration: invalid duration " + quote(orig))
		}
		d += v
	}

	if neg {
		return -time.Duration(d), nil
	}
	if d > 1<<63-1 {
		return 0, errors.New("ParseDuration: invalid duration " + quote(orig))
	}
	return time.Duration(d), nil
}

func quote(s string) string {
	return "\"" + s + "\""
}

var errLeadingInt = errors.New("ParseDuration: bad [0-9]*") // never printed

// leadingInt consumes the leading [0-9]* from s.
func leadingInt(s string) (x uint64, rem string, err error) {
	i := 0
	for ; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			break
		}
		if x > (1<<63)/10 {
			// overflow
			return 0, "", errLeadingInt
		}
		x = x*10 + uint64(c) - '0'
		if x > 1<<63 {
			// overflow
			return 0, "", errLeadingInt
		}
	}
	return x, s[i:], nil
}

// leadingFraction consumes the leading [0-9]* from s.
// It is used only for fractions, so does not return an error on overflow,
// it just stops accumulating precision.
func leadingFraction(s string) (x uint64, scale float64, rem string) {
	i := 0
	scale = 1
	overflow := false
	for ; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			break
		}
		if overflow {
			continue
		}
		if x > (1<<63-1)/10 {
			// It's possible for overflow to give a positive number, so take care.
			overflow = true
			continue
		}
		y := x*10 + uint64(c) - '0'
		if y > 1<<63 {
			overflow = true
			continue
		}
		x = y
		scale *= 10
	}
	return x, scale, s[i:]
}

// PrettyDuration returns a string representing the duration in the form of "4d2h3m5s".
func PrettyDuration(d time.Duration) string {
	u := uint64(d)
	if d < 0 {
		u = -u
	}
	if u < uint64(time.Second) {
		return d.String()
	}
	return prettyDuration(u/uint64(time.Second), u%uint64(time.Second), d < 0)
}

// prettyDuration formats an unsigned duration of at least one second.
// Keeping seconds and fractional nanoseconds separate also supports int64 milliseconds.
func prettyDuration(seconds, nanos uint64, neg bool) string {
	var buf [32]byte
	w := len(buf)
	if seconds%60 != 0 || nanos != 0 {
		w--
		buf[w] = 's'
		w, _ = fmtFrac(buf[:w], nanos, 9)
		w = fmtInt(buf[:w], seconds%60)
	}
	minutes := seconds / 60
	if minutes%60 != 0 {
		w--
		buf[w] = 'm'
		w = fmtInt(buf[:w], minutes%60)
	}
	hours := minutes / 60
	if hours%24 != 0 {
		w--
		buf[w] = 'h'
		w = fmtInt(buf[:w], hours%24)
	}
	if days := hours / 24; days != 0 {
		w--
		buf[w] = 'd'
		w = fmtInt(buf[:w], days)
	}
	if neg {
		w--
		buf[w] = '-'
	}
	return string(buf[w:])
}

// fmtFrac formats the fraction of v/10**prec (e.g., ".12345") into the
// tail of buf, omitting trailing zeros. It omits the decimal
// point too when the fraction is 0. It returns the index where the
// output bytes begin and the value v/10**prec.
func fmtFrac(buf []byte, v uint64, prec int) (nw int, nv uint64) {
	// Omit trailing zeros up to and including decimal point.
	w := len(buf)
	print := false
	for i := 0; i < prec; i++ {
		digit := v % 10
		print = print || digit != 0
		if print {
			w--
			buf[w] = byte(digit) + '0'
		}
		v /= 10
	}
	if print {
		w--
		buf[w] = '.'
	}
	return w, v
}

// fmtInt formats v into the tail of buf.
// It returns the index where the output begins.
func fmtInt(buf []byte, v uint64) int {
	w := len(buf)
	if v == 0 {
		w--
		buf[w] = '0'
	} else {
		for v > 0 {
			w--
			buf[w] = byte(v%10) + '0'
			v /= 10
		}
	}
	return w
}

// PrettyTime 返回毫秒时长的字符串显示，100000 => 1m40s。支持完整 int64 范围。
func PrettyTime(ms int64) string {
	u := uint64(ms)
	if ms < 0 {
		u = -u
	}
	if u < MsPerSecond {
		return PrettyDuration(time.Duration(ms) * time.Millisecond)
	}
	return prettyDuration(u/MsPerSecond, u%MsPerSecond*uint64(time.Millisecond), ms < 0)
}
