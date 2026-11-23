// Copyright 2026 Pawl Programming Language
// Test 30: Timestamp formats - ISO 8601, RFC 3339, Unix timestamp, and custom formats.

use std.time

// 1. Unix timestamp (seconds since epoch)
unix_ts := time.now()
assert unix_ts > 1700000000

// 2. Unix timestamp in milliseconds
unix_ms := time.now_ms()
assert unix_ms > unix_ts * 1000

// 3. Convert milliseconds to seconds
func ms_to_seconds(ms: int) -> int {
    return ms / 1000
}
seconds_from_ms := ms_to_seconds(unix_ms)
assert seconds_from_ms == unix_ts or seconds_from_ms == unix_ts + 1

// 4. ISO 8601 format (YYYY-MM-DDTHH:MM:SSZ)
func format_iso8601(ts: int) -> string {
    base := time.format(ts, "YYYY-MM-DD HH:mm:ss", "UTC")
    return string.replace(base, " ", "T") + "Z"
}
iso_str := format_iso8601(unix_ts)
assert string.contains(iso_str, "T")
assert string.ends_with(iso_str, "Z")

// 5. Parse ISO 8601
func parse_iso8601(iso_str: string) -> int {
    // Remove T and Z
    formatted := string.replace(iso_str, "T", " ")
    formatted = string.replace(formatted, "Z", "")
    return time.parse(formatted, "YYYY-MM-DD HH:mm:ss", "UTC")
}
iso_example := "2026-05-15T12:00:00Z"
parsed_iso := parse_iso8601(iso_example)
assert parsed_iso > 0

// 6. RFC 3339 format (similar to ISO 8601 with timezone)
func format_rfc3339(ts: int, tz: string) -> string {
    if tz == "UTC" {
        return format_iso8601(ts)
    }
    base := time.format(ts, "YYYY-MM-DD HH:mm:ss", tz)
    return string.replace(base, " ", "T")
}
rfc_str := format_rfc3339(unix_ts, "UTC")
assert len(rfc_str) == 20 // "YYYY-MM-DDTHH:MM:SSZ"

// 7. Custom date format
func format_custom(ts: int, format: string) -> string {
    return time.format(ts, format, "UTC")
}
custom_str := format_custom(unix_ts, "DD/MM/YYYY")
assert len(custom_str) == 10 // "DD/MM/YYYY"

// 8. Parse custom format
func parse_custom(date_str: string, format: string) -> int {
    return time.parse(date_str, format, "UTC")
}
custom_date := "15/05/2026"
parsed_custom := parse_custom(custom_date, "DD/MM/YYYY")
assert parsed_custom > 0

// 9. HTTP date format (RFC 1123)
func format_http_date(ts: int) -> string {
    // Simplified HTTP date format
    return time.format(ts, "DDD, DD MMM YYYY HH:mm:ss", "UTC")
}
http_date := format_http_date(unix_ts)
assert len(http_date) > 20

// 10. Database timestamp format (YYYY-MM-DD HH:MM:SS)
func format_db_timestamp(ts: int) -> string {
    return time.format(ts, "YYYY-MM-DD HH:mm:ss", "UTC")
}
db_ts := format_db_timestamp(unix_ts)
assert len(db_ts) == 19

// 11. Parse database timestamp
func parse_db_timestamp(db_str: string) -> int {
    return time.parse(db_str, "YYYY-MM-DD HH:mm:ss", "UTC")
}
db_str := "2026-05-15 12:00:00"
parsed_db := parse_db_timestamp(db_str)
assert parsed_db > 0

// 12. Time-only format (HH:MM:SS)
func format_time_only(ts: int) -> string {
    return time.format(ts, "HH:mm:ss", "UTC")
}
time_only := format_time_only(unix_ts)
assert len(time_only) == 8

// 13. Date-only format (YYYY-MM-DD)
func format_date_only(ts: int) -> string {
    return time.format(ts, "YYYY-MM-DD", "UTC")
}
date_only := format_date_only(unix_ts)
assert len(date_only) == 10

// 14. Compact format (YYYYMMDDHHMMSS)
func format_compact(ts: int) -> string {
    return time.format(ts, "YYYYMMDDHHmmss", "UTC")
}
compact := format_compact(unix_ts)
assert len(compact) == 14

// 15. Parse compact format
func parse_compact(compact_str: string) -> int {
    return time.parse(compact_str, "YYYYMMDDHHmmss", "UTC")
}
compact_str := "20260515120000"
parsed_compact := parse_compact(compact_str)
assert parsed_compact > 0

// 16. Format with timezone offset
func format_with_offset(ts: int, offset_hours: int) -> string {
    base := time.format(ts, "YYYY-MM-DD HH:mm:ss", "UTC")
    if offset_hours >= 0 {
        offset_str := "+" + string(offset_hours / 10) + string(offset_hours % 10) + ":00"
    }
    else {
        offset_str := "-" + string(-offset_hours / 10) + string(-offset_hours % 10) + ":00"
    }
    return base + offset_str
}
offset_format := format_with_offset(unix_ts, 5)
assert string.contains(offset_format, "+05:00")

// 17. Unix timestamp to string (human readable)
func unix_to_string(ts: int) -> string {
    return format_db_timestamp(ts)
}
unix_str := unix_to_string(unix_ts)
assert len(unix_str) == 19

// 18. String to Unix timestamp
func string_to_unix(date_str: string) -> int {
    return parse_db_timestamp(date_str)
}
date_str := "2026-05-15 12:00:00"
unix_from_str := string_to_unix(date_str)
assert unix_from_str > 0

// 19. Relative time format (e.g., "2 hours ago")
func format_relative(ts: int, reference: int) -> string {
    diff := reference - ts
    if diff < 0 {
        diff = -diff
    }
    seconds := diff
    minutes := seconds / 60
    hours := minutes / 60
    days := hours / 24

    if days > 0 {
        return string(days) + " days ago"
    }
    if hours > 0 {
        return string(hours) + " hours ago"
    }
    if minutes > 0 {
        return string(minutes) + " minutes ago"
    }
    return "just now"
}
past_ts := unix_ts - 7200 // 2 hours ago
relative := format_relative(past_ts, unix_ts)
assert string.contains(relative, "hours")

// 20. Format duration as timestamp
func duration_to_timestamp(dur_seconds: int) -> string {
    hours := dur_seconds / 3600
    remainder := dur_seconds % 3600
    minutes := remainder / 60
    seconds := remainder % 60

    return string(hours) + ":" + string(minutes / 10) + string(minutes % 10) + ":" +
           string(seconds / 10) + string(seconds % 10)
}
dur_ts := duration_to_timestamp(3661) // 1:01:01
assert dur_ts == "1:01:01"

println("✓ test_30_timestamp_formats passed")
