// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

// Package stdlib provides the Time standard library module for the Alca programming language.
// It supports timestamp queries, formatting, parsing, measurements, intervals, and conversions.
package stdlib

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/alca-lang/alca/pkg/interpreter"
)

// normalizeLayout converts common format strings (like YYYY-MM-DD HH:mm:ss) to Go time layouts.
func normalizeLayout(layout string) string {
	if layout == "" {
		return time.RFC3339
	}
	switch layout {
	case "iso", "iso8601", "rfc3339":
		return time.RFC3339
	case "datetime", "standard":
		return "2006-01-02 15:04:05"
	case "date":
		return "2006-01-02"
	case "time":
		return "15:04:05"
	case "timestamp":
		return "2006-01-02 15:04:05.000"
	}

	// Replace human-friendly tokens with Go reference date (2006-01-02 15:04:05 MST)
	r := layout
	r = strings.ReplaceAll(r, "YYYY", "2006")
	r = strings.ReplaceAll(r, "yyyy", "2006")
	r = strings.ReplaceAll(r, "MM", "01")
	r = strings.ReplaceAll(r, "DD", "02")
	r = strings.ReplaceAll(r, "dd", "02")
	r = strings.ReplaceAll(r, "HH", "15")
	r = strings.ReplaceAll(r, "hh", "03")
	r = strings.ReplaceAll(r, "mm", "04")
	r = strings.ReplaceAll(r, "ss", "05")
	r = strings.ReplaceAll(r, "SSS", "000")
	return r
}

// RegisterTimeBuiltins registers the "std.time" module and its functions into the environment.
func RegisterTimeBuiltins(env *interpreter.Environment) {
	timeModule := interpreter.NewMap()

	register := func(name string, fn func(args ...interpreter.Value) interpreter.Value) {
		bf := &interpreter.BuiltinFunction{Name: name, Fn: fn}
		timeModule.Set(name, bf)
	}

	// now() -> unix epoch timestamp in seconds
	register("now", func(args ...interpreter.Value) interpreter.Value {
		return &interpreter.IntegerValue{Val: time.Now().Unix()}
	})

	// now_ms() -> unix epoch timestamp in milliseconds
	register("now_ms", func(args ...interpreter.Value) interpreter.Value {
		return &interpreter.IntegerValue{Val: time.Now().UnixMilli()}
	})

	// now_ns() -> unix epoch timestamp in nanoseconds
	register("now_ns", func(args ...interpreter.Value) interpreter.Value {
		return &interpreter.IntegerValue{Val: time.Now().UnixNano()}
	})

	// now_str(layout?) -> formatted string of the current time
	register("now_str", func(args ...interpreter.Value) interpreter.Value {
		layout := "2006-01-02 15:04:05"
		if len(args) > 0 {
			layout = normalizeLayout(interpreter.CoerceToString(args[0]))
		}
		now := time.Now()
		if len(args) > 1 {
			loc, err := time.LoadLocation(interpreter.CoerceToString(args[1]))
			if err != nil {
				return interpreter.NewError(400, "invalid time zone: "+err.Error())
			}
			now = now.In(loc)
		}
		return &interpreter.StringValue{Val: now.Format(layout)}
	})

	// format(timestamp, layout?) -> string
	register("format", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "time.format requires at least a timestamp argument")
		}
		var sec int64
		switch v := args[0].(type) {
		case *interpreter.IntegerValue:
			sec = v.Val
		case *interpreter.FloatValue:
			sec = int64(v.Val)
		default:
			return interpreter.NewError(400, "timestamp must be an integer or float")
		}

		layout := "2006-01-02 15:04:05"
		if len(args) > 1 {
			layout = normalizeLayout(interpreter.CoerceToString(args[1]))
		}

		t := time.Unix(sec, 0)
		if len(args) > 2 {
			loc, err := time.LoadLocation(interpreter.CoerceToString(args[2]))
			if err != nil {
				return interpreter.NewError(400, "invalid time zone: "+err.Error())
			}
			t = t.In(loc)
		}
		return &interpreter.StringValue{Val: t.Format(layout)}
	})

	// parse(timeStr, layout?) -> unix timestamp in seconds
	register("parse", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "time.parse requires a time string argument")
		}
		timeStr := interpreter.CoerceToString(args[0])
		layout := time.RFC3339
		if len(args) > 1 {
			layout = normalizeLayout(interpreter.CoerceToString(args[1]))
		} else {
			// Try auto-detecting common layouts
			if !strings.Contains(timeStr, "T") && strings.Contains(timeStr, " ") {
				layout = "2006-01-02 15:04:05"
			} else if len(timeStr) == 10 && strings.Count(timeStr, "-") == 2 {
				layout = "2006-01-02"
			}
		}

		var t time.Time
		var err error
		if len(args) > 2 {
			loc, locErr := time.LoadLocation(interpreter.CoerceToString(args[2]))
			if locErr != nil {
				return interpreter.NewError(400, "invalid time zone: "+locErr.Error())
			}
			t, err = time.ParseInLocation(layout, timeStr, loc)
		} else {
			t, err = time.Parse(layout, timeStr)
		}
		if err != nil {
			return interpreter.NewError(400, fmt.Sprintf("failed to parse time %q: %s", timeStr, err.Error()))
		}
		return &interpreter.IntegerValue{Val: t.Unix()}
	})

	// sleep(ms) -> pauses execution for given milliseconds
	register("sleep", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "time.sleep requires a duration argument (ms)")
		}
		ms := int64(0)
		switch v := args[0].(type) {
		case *interpreter.IntegerValue:
			ms = v.Val
		case *interpreter.FloatValue:
			ms = int64(v.Val)
		default:
			str := interpreter.CoerceToString(args[0])
			d, err := time.ParseDuration(str)
			if err == nil {
				if d < 0 {
					return interpreter.NewError(400, "sleep duration must not be negative")
				}
				time.Sleep(d)
				return interpreter.NULL
			}
			return interpreter.NewError(400, "invalid sleep duration: "+str)
		}

		if ms < 0 {
			return interpreter.NewError(400, "sleep duration must not be negative")
		}
		if ms > math.MaxInt64/int64(time.Millisecond) {
			return interpreter.NewError(400, "sleep duration is too large")
		}
		if ms > 0 {
			time.Sleep(time.Duration(ms) * time.Millisecond)
		}
		return interpreter.NULL
	})

	register("parse_duration", func(args ...interpreter.Value) interpreter.Value {
		if len(args) != 1 {
			return interpreter.NewError(400, "time.parse_duration requires a duration string")
		}
		duration, err := time.ParseDuration(interpreter.CoerceToString(args[0]))
		if err != nil {
			return interpreter.NewError(400, "invalid duration: "+err.Error())
		}
		return &interpreter.IntegerValue{Val: int64(duration)}
	})

	register("timer", func(args ...interpreter.Value) interpreter.Value {
		if len(args) != 1 {
			return interpreter.NewError(400, "time.timer requires a duration string")
		}
		duration, err := time.ParseDuration(interpreter.CoerceToString(args[0]))
		if err != nil || duration < 0 {
			return interpreter.NewError(400, "time.timer requires a non-negative Go duration")
		}
		timer := time.NewTimer(duration)
		cancelled := make(chan struct{})
		var cancelOnce sync.Once
		object := interpreter.NewMap()
		object.Set("wait", &interpreter.BuiltinFunction{Name: "wait", Fn: func(args ...interpreter.Value) interpreter.Value {
			select {
			case <-timer.C:
				return interpreter.TRUE
			case <-cancelled:
				return interpreter.FALSE
			}
		}})
		object.Set("cancel", &interpreter.BuiltinFunction{Name: "cancel", Fn: func(args ...interpreter.Value) interpreter.Value {
			stopped := timer.Stop()
			cancelOnce.Do(func() { close(cancelled) })
			return &interpreter.BoolValue{Val: stopped}
		}})
		return object
	})

	// since(timestamp_seconds_or_ms) -> elapsed float seconds
	register("since", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "time.since requires a start timestamp")
		}
		var start time.Time
		switch v := args[0].(type) {
		case *interpreter.IntegerValue:
			start = time.Unix(v.Val, 0)
		case *interpreter.FloatValue:
			start = time.Unix(int64(v.Val), 0)
		default:
			return interpreter.NewError(400, "timestamp must be numeric")
		}
		elapsed := time.Since(start).Seconds()
		return &interpreter.FloatValue{Val: elapsed}
	})

	// date(timestamp?) -> map { year, month, day, hour, minute, second, weekday }
	register("date", func(args ...interpreter.Value) interpreter.Value {
		t := time.Now()
		if len(args) > 0 {
			switch v := args[0].(type) {
			case *interpreter.IntegerValue:
				t = time.Unix(v.Val, 0)
			case *interpreter.FloatValue:
				t = time.Unix(int64(v.Val), 0)
			default:
				return interpreter.NewError(400, "timestamp must be numeric")
			}
		}
		if len(args) > 1 {
			loc, err := time.LoadLocation(interpreter.CoerceToString(args[1]))
			if err != nil {
				return interpreter.NewError(400, "invalid time zone: "+err.Error())
			}
			t = t.In(loc)
		}

		m := interpreter.NewMap()
		m.Set("year", &interpreter.IntegerValue{Val: int64(t.Year())})
		m.Set("month", &interpreter.IntegerValue{Val: int64(t.Month())})
		m.Set("day", &interpreter.IntegerValue{Val: int64(t.Day())})
		m.Set("hour", &interpreter.IntegerValue{Val: int64(t.Hour())})
		m.Set("minute", &interpreter.IntegerValue{Val: int64(t.Minute())})
		m.Set("second", &interpreter.IntegerValue{Val: int64(t.Second())})
		m.Set("weekday", &interpreter.StringValue{Val: t.Weekday().String()})
		m.Set("day_of_year", &interpreter.IntegerValue{Val: int64(t.YearDay())})
		m.Set("is_leap_year", &interpreter.BoolValue{Val: (t.Year()%4 == 0 && t.Year()%100 != 0) || (t.Year()%400 == 0)})
		return m
	})

	// diff(t1, t2) -> float difference in seconds (t1 - t2)
	register("diff", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "time.diff requires two timestamps")
		}
		var s1, s2 int64
		if iv, ok := args[0].(*interpreter.IntegerValue); ok {
			s1 = iv.Val
		} else if fv, ok := args[0].(*interpreter.FloatValue); ok {
			s1 = int64(fv.Val)
		}
		if iv, ok := args[1].(*interpreter.IntegerValue); ok {
			s2 = iv.Val
		} else if fv, ok := args[1].(*interpreter.FloatValue); ok {
			s2 = int64(fv.Val)
		}
		return &interpreter.FloatValue{Val: float64(s1 - s2)}
	})

	// measure(callable) -> map { result, elapsed_ms, elapsed_sec }
	register("measure", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "time.measure requires a callable function argument")
		}
		fn := args[0]
		start := time.Now()
		res, err := interpreter.Invoke(fn, nil)
		if err != nil {
			return interpreter.NewError(500, "time.measure callback failed: "+err.Error())
		}
		if res != nil && res.Type() == interpreter.ERROR_VAL {
			return res
		}
		elapsed := time.Since(start)

		m := interpreter.NewMap()
		if res != nil {
			m.Set("result", res)
		} else {
			m.Set("result", interpreter.NULL)
		}
		m.Set("elapsed_ms", &interpreter.FloatValue{Val: float64(elapsed.Microseconds()) / 1000.0})
		m.Set("elapsed_sec", &interpreter.FloatValue{Val: elapsed.Seconds()})
		return m
	})

	register("unix_ms_to_seconds", func(args ...interpreter.Value) interpreter.Value {
		if len(args) != 1 {
			return interpreter.NewError(400, "time.unix_ms_to_seconds requires an integer timestamp")
		}
		value, ok := args[0].(*interpreter.IntegerValue)
		if !ok {
			return interpreter.NewError(400, "timestamp must be an integer")
		}
		return &interpreter.FloatValue{Val: float64(value.Val) / 1000}
	})

	register("unix_seconds_to_ms", func(args ...interpreter.Value) interpreter.Value {
		if len(args) != 1 {
			return interpreter.NewError(400, "time.unix_seconds_to_ms requires a numeric timestamp")
		}
		var seconds float64
		switch value := args[0].(type) {
		case *interpreter.IntegerValue:
			seconds = float64(value.Val)
		case *interpreter.FloatValue:
			seconds = value.Val
		default:
			return interpreter.NewError(400, "timestamp must be numeric")
		}
		ms := seconds * 1000
		if math.IsNaN(ms) || math.IsInf(ms, 0) || ms > math.MaxInt64 || ms < math.MinInt64 {
			return interpreter.NewError(400, "timestamp is outside the supported range")
		}
		return &interpreter.IntegerValue{Val: int64(ms)}
	})

	env.Set("time", timeModule)
	env.Set("std.time", timeModule)
}
