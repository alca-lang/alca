// Copyright 2026 Pawl Programming Language
// Test 28: Duration arithmetic - addition, subtraction, and time interval operations.

use std.time

// 1. Time difference calculation
t1 := time.parse("2026-01-01 00:00:00", "YYYY-MM-DD HH:mm:ss", "UTC")
t2 := time.parse("2026-01-01 01:30:00", "YYYY-MM-DD HH:mm:ss", "UTC")
diff_sec := time.diff(t2, t1)
assert diff_sec == 5400.0 // 90 minutes = 5400 seconds

// 2. Add seconds to timestamp
func add_seconds(ts: int, seconds: int) -> int {
    return ts + seconds
}
ts_base := time.parse("2026-01-01 00:00:00", "YYYY-MM-DD HH:mm:ss", "UTC")
ts_plus_hour := add_seconds(ts_base, 3600)
assert ts_plus_hour == ts_base + 3600

// 3. Subtract seconds from timestamp
func subtract_seconds(ts: int, seconds: int) -> int {
    return ts - seconds
}
ts_minus_hour := subtract_seconds(ts_base, 3600)
assert ts_minus_hour == ts_base - 3600

// 4. Add minutes to timestamp
func add_minutes(ts: int, minutes: int) -> int {
    return ts + (minutes * 60)
}
ts_plus_30_min := add_minutes(ts_base, 30)
assert ts_plus_30_min == ts_base + 1800

// 5. Add hours to timestamp
func add_hours(ts: int, hours: int) -> int {
    return ts + (hours * 3600)
}
ts_plus_2_hours := add_hours(ts_base, 2)
assert ts_plus_2_hours == ts_base + 7200

// 6. Add days to timestamp
func add_days(ts: int, days: int) -> int {
    return ts + (days * 86400)
}
ts_plus_day := add_days(ts_base, 1)
assert ts_plus_day == ts_base + 86400

// 7. Add weeks to timestamp
func add_weeks(ts: int, weeks: int) -> int {
    return ts + (weeks * 604800) // 7 * 86400
}
ts_plus_week := add_weeks(ts_base, 1)
assert ts_plus_week == ts_base + 604800

// 8. Duration struct
struct Duration {
    seconds: int
    nanoseconds: int = 0
}
func create_duration(secs: int) -> Duration {
    return Duration(secs, 0)
}
duration_1h := create_duration(3600)
assert duration_1h.seconds == 3600

// 9. Add duration to timestamp
func add_duration(ts: int, dur: Duration) -> int {
    return ts + dur.seconds
}
ts_with_dur := add_duration(ts_base, duration_1h)
assert ts_with_dur == ts_base + 3600

// 10. Subtract duration from timestamp
func subtract_duration(ts: int, dur: Duration) -> int {
    return ts - dur.seconds
}
ts_minus_dur := subtract_duration(ts_base, duration_1h)
assert ts_minus_dur == ts_base - 3600

// 11. Duration between two timestamps
func duration_between(ts1: int, ts2: int) -> Duration {
    diff := ts2 - ts1
    return Duration(diff, 0)
}
dur := duration_between(t1, t2)
assert dur.seconds == 5400

// 12. Duration to human-readable string
func duration_to_string(dur: Duration) -> string {
    total_secs := dur.seconds
    days := total_secs / 86400
    remainder := total_secs % 86400
    hours := remainder / 3600
    remainder = remainder % 3600
    minutes := remainder / 60
    secs := remainder % 60

    result := ""
    if days > 0 {
        result = result + string(days) + "d "
    }
    if hours > 0 {
        result = result + string(hours) + "h "
    }
    if minutes > 0 {
        result = result + string(minutes) + "m "
    }
    if secs > 0 or result == "" {
        result = result + string(secs) + "s"
    }
    return result
}
dur_str := duration_to_string(dur)
assert string.contains(dur_str, "90m") // 90 minutes

// 13. Parse duration string
func parse_duration(dur_str: string) -> Duration {
    // Simplified parsing for "Xh Ym Zs" format
    parts := string.split(dur_str, " ")
    total_secs := 0
    for part in parts {
        if len(part) > 0 {
            unit := string.substring(part, len(part) - 1, len(part))
            val := int(string.substring(part, 0, len(part) - 1))
            if unit == "h" {
                total_secs = total_secs + (val * 3600)
            }
            if unit == "m" {
                total_secs = total_secs + (val * 60)
            }
            if unit == "s" {
                total_secs = total_secs + val
            }
        }
    }
    return Duration(total_secs, 0)
}
parsed_dur := parse_duration("2h 30m")
assert parsed_dur.seconds == 9000 // 2*3600 + 30*60

// 14. Compare durations
func compare_durations(d1: Duration, d2: Duration) -> int {
    if d1.seconds < d2.seconds {
        return -1
    }
    if d1.seconds > d2.seconds {
        return 1
    }
    return 0
}
dur1 := Duration(3600, 0)
dur2 := Duration(7200, 0)
assert compare_durations(dur1, dur2) == -1
assert compare_durations(dur2, dur1) == 1
assert compare_durations(dur1, dur1) == 0

// 15. Multiply duration
func multiply_duration(dur: Duration, factor: int) -> Duration {
    return Duration(dur.seconds * factor, 0)
}
doubled := multiply_duration(duration_1h, 2)
assert doubled.seconds == 7200

// 16. Divide duration
func divide_duration(dur: Duration, divisor: int) -> Duration {
    return Duration(dur.seconds / divisor, 0)
}
halved := divide_duration(duration_1h, 2)
assert halved.seconds == 1800

// 17. Duration addition
func add_durations(d1: Duration, d2: Duration) -> Duration {
    return Duration(d1.seconds + d2.seconds, 0)
}
dur_sum := add_durations(Duration(3600, 0), Duration(1800, 0))
assert dur_sum.seconds == 5400

// 18. Duration subtraction
func subtract_durations(d1: Duration, d2: Duration) -> Duration {
    return Duration(d1.seconds - d2.seconds, 0)
}
dur_diff := subtract_durations(Duration(3600, 0), Duration(1800, 0))
assert dur_diff.seconds == 1800

// 19. Check if timestamp is within duration of another
func is_within_duration(ts: int, reference: int, dur: Duration) -> bool {
    diff := ts - reference
    if diff < 0 {
        diff = -diff
    }
    return diff <= dur.seconds
}
near_ts := ts_base + 1800
is_near := is_within_duration(near_ts, ts_base, Duration(3600, 0))
assert is_near

far_ts := ts_base + 7200
is_far := is_within_duration(far_ts, ts_base, Duration(3600, 0))
assert !is_far

// 20. Clamp timestamp to duration range
func clamp_to_duration(ts: int, min_ts: int, max_dur: Duration) -> int {
    max_ts := min_ts + max_dur.seconds
    if ts < min_ts {
        return min_ts
    }
    if ts > max_ts {
        return max_ts
    }
    return ts
}
clamped := clamp_to_duration(ts_base + 1000, ts_base, Duration(3600, 0))
assert clamped == ts_base + 1000

clamped_over := clamp_to_duration(ts_base + 5000, ts_base, Duration(3600, 0))
assert clamped_over == ts_base + 3600

println("✓ test_28_duration_arithmetic passed")
