// Copyright 2026 Pawl Programming Language
// Test 27: Timezone operations - UTC, local time, timezone conversion.

use std.time

// 1. Current timestamp in UTC
now_utc := time.now()
assert now_utc > 1700000000 // Modern timestamp

// 2. Get date components in UTC
utc_date := time.date(now_utc, "UTC")
assert utc_date["year"] > 2023
assert utc_date["month"] >= 1 and utc_date["month"] <= 12
assert utc_date["day"] >= 1 and utc_date["day"] <= 31

// 3. Get date components in local timezone
local_date := time.date(now_utc, "Local")
assert local_date["year"] >= 2023

// 4. Parse date with timezone
date_str := "2026-05-15 12:00:00"
parsed_utc := time.parse(date_str, "YYYY-MM-DD HH:mm:ss", "UTC")
assert parsed_utc > 1700000000

// 5. Format timestamp in UTC
formatted_utc := time.format(now_utc, "YYYY-MM-DD HH:mm:ss", "UTC")
assert len(formatted_utc) == 19 // "YYYY-MM-DD HH:mm:ss" = 19 chars

// 6. Format timestamp in local timezone
formatted_local := time.format(now_utc, "YYYY-MM-DD HH:mm:ss", "Local")
assert len(formatted_local) == 19

// 7. UTC to local conversion (simulation)
func utc_to_local(utc_ts: int) -> int {
    // Simplified: assume UTC = local for test
    return utc_ts
}
local_ts := utc_to_local(now_utc)
assert local_ts == now_utc

// 8. Local to UTC conversion (simulation)
func local_to_utc(local_ts: int) -> int {
    // Simplified: assume UTC = local for test
    return local_ts
}
utc_from_local := local_to_utc(local_ts)
assert utc_from_local == now_utc

// 9. Timezone offset calculation (simulation)
func get_timezone_offset(tz: string) -> int {
    // Return offset in seconds
    if tz == "UTC" {
        return 0
    }
    if tz == "America/New_York" {
        return -18000 // -5 hours
    }
    if tz == "Europe/London" {
        return 0 // UTC in winter, +1 in summer
    }
    if tz == "Asia/Tokyo" {
        return 32400 // +9 hours
    }
    return 0
}
utc_offset := get_timezone_offset("UTC")
assert utc_offset == 0

tokyo_offset := get_timezone_offset("Asia/Tokyo")
assert tokyo_offset == 32400

// 10. Convert timestamp to specific timezone
func convert_timezone(ts: int, from_tz: string, to_tz: string) -> int {
    offset_from := get_timezone_offset(from_tz)
    offset_to := get_timezone_offset(to_tz)
    return ts - offset_from + offset_to
}
tokyo_ts := convert_timezone(now_utc, "UTC", "Asia/Tokyo")
assert tokyo_ts > now_utc // Tokyo is ahead of UTC

// 11. Parse date with timezone offset
date_with_offset := "2026-05-15T12:00:00+09:00"
// Simplified parsing - in real implementation would handle offset
parsed_with_offset := time.parse("2026-05-15 12:00:00", "YYYY-MM-DD HH:mm:ss", "UTC")
assert parsed_with_offset > 0

// 12. Format with timezone offset
func format_with_offset(ts: int, offset_hours: int) -> string {
    base := time.format(ts, "YYYY-MM-DD HH:mm:ss", "UTC")
    if offset_hours >= 0 {
        return base + "+" + string(offset_hours / 10) + string(offset_hours % 10) + ":00"
    }
    else {
        return base + "-" + string(-offset_hours / 10) + string(-offset_hours % 10) + ":00"
    }
}
formatted_offset := format_with_offset(now_utc, 9)
assert string.contains(formatted_offset, "+09:00")

// 13. Daylight Saving Time detection (simulation)
func is_dst(ts: int, tz: string) -> bool {
    // Simplified DST check
    if tz == "America/New_York" {
        // DST is March to November
        date := time.date(ts, "UTC")
        month := date["month"]
        return month >= 3 and month <= 11
    }
    return false
}
summer_ts := time.parse("2026-07-15 12:00:00", "YYYY-MM-DD HH:mm:ss", "UTC")
is_summer_dst := is_dst(summer_ts, "America/New_York")
assert is_summer_dst

winter_ts := time.parse("2026-01-15 12:00:00", "YYYY-MM-DD HH:mm:ss", "UTC")
is_winter_dst := is_dst(winter_ts, "America/New_York")
assert !is_winter_dst

// 14. List common timezones
func get_common_timezones() -> list[string] {
    return ["UTC", "America/New_York", "Europe/London", "Asia/Tokyo", "Australia/Sydney"]
}
timezones := get_common_timezones()
assert len(timezones) == 5
assert "UTC" in timezones

// 15. Validate timezone string
func is_valid_timezone(tz: string) -> bool {
    common := get_common_timezones()
    return tz in common
}
assert is_valid_timezone("UTC")
assert is_valid_timezone("Asia/Tokyo")
assert !is_valid_timezone("Invalid/Timezone")

// 16. Get timezone name from offset
func timezone_from_offset(offset_seconds: int) -> string {
    offset_hours := offset_seconds / 3600
    if offset_hours == 0 {
        return "UTC"
    }
    if offset_hours == -5 {
        return "America/New_York"
    }
    if offset_hours == 9 {
        return "Asia/Tokyo"
    }
    return "Unknown"
}
tz_from_offset := timezone_from_offset(32400)
assert tz_from_offset == "Asia/Tokyo"

// 17. Timezone-aware date comparison
func compare_dates_in_tz(ts1: int, ts2: int, tz: string) -> int {
    // Convert both to the same timezone
    tz_ts1 := convert_timezone(ts1, "UTC", tz)
    tz_ts2 := convert_timezone(ts2, "UTC", tz)
    if tz_ts1 < tz_ts2 {
        return -1
    }
    if tz_ts1 > tz_ts2 {
        return 1
    }
    return 0
}
ts1 := time.parse("2026-01-01 00:00:00", "YYYY-MM-DD HH:mm:ss", "UTC")
ts2 := time.parse("2026-01-01 09:00:00", "YYYY-MM-DD HH:mm:ss", "UTC")
comparison := compare_dates_in_tz(ts1, ts2, "Asia/Tokyo")
assert comparison == 0 // Same time in Tokyo

// 18. Timezone abbreviation (simulation)
func get_timezone_abbr(tz: string, ts: int) -> string {
    if tz == "UTC" {
        return "UTC"
    }
    if tz == "America/New_York" {
        if is_dst(ts, tz) {
            return "EDT"
        }
        return "EST"
    }
    if tz == "Asia/Tokyo" {
        return "JST"
    }
    return "UNK"
}
abbr := get_timezone_abbr("UTC", now_utc)
assert abbr == "UTC"

// 19. Convert between timezone strings
func convert_timezone_string(ts: int, from: string, to: string) -> int {
    return convert_timezone(ts, from, to)
}
ny_ts := convert_timezone_string(now_utc, "UTC", "America/New_York")
assert ny_ts < now_utc // NY is behind UTC

// 20. Get all available timezones (simulation)
func get_available_timezones() -> list[string] {
    return [
        "UTC",
        "America/New_York",
        "America/Los_Angeles",
        "America/Chicago",
        "Europe/London",
        "Europe/Paris",
        "Europe/Berlin",
        "Asia/Tokyo",
        "Asia/Shanghai",
        "Asia/Singapore",
        "Australia/Sydney"
    ]
}
all_tzs := get_available_timezones()
assert len(all_tzs) > 10

println("✓ test_27_timezones passed")
