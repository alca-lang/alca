// Copyright 2026 Pawl Programming Language
// Test 29: Calendar operations - days of week, month boundaries, year calculations.

use std.time

// 1. Get day of week (0 = Sunday, 6 = Saturday)
func get_day_of_week(ts: int) -> int {
    // Simplified calculation
    // Unix epoch (1970-01-01) was Thursday (4)
    return ((ts / 86400) + 4) % 7
}
known_thursday := time.parse("2026-05-14 00:00:00", "YYYY-MM-DD HH:mm:ss", "UTC")
dow := get_day_of_week(known_thursday)
assert dow == 4 // Thursday

// 2. Check if date is weekend
func is_weekend(ts: int) -> bool {
    dow := get_day_of_week(ts)
    return dow == 0 or dow == 6 // Sunday or Saturday
}
saturday := time.parse("2026-05-16 00:00:00", "YYYY-MM-DD HH:mm:ss", "UTC")
assert is_weekend(saturday)

monday := time.parse("2026-05-18 00:00:00", "YYYY-MM-DD HH:mm:ss", "UTC")
assert !is_weekend(monday)

// 3. Get day of month
func get_day_of_month(ts: int) -> int {
    date := time.date(ts, "UTC")
    return date["day"]
}
day_15 := time.parse("2026-05-15 00:00:00", "YYYY-MM-DD HH:mm:ss", "UTC")
assert get_day_of_month(day_15) == 15

// 4. Get month
func get_month(ts: int) -> int {
    date := time.date(ts, "UTC")
    return date["month"]
}
may_date := time.parse("2026-05-15 00:00:00", "YYYY-MM-DD HH:mm:ss", "UTC")
assert get_month(may_date) == 5

// 5. Get year
func get_year(ts: int) -> int {
    date := time.date(ts, "UTC")
    return date["year"]
}
year_2026 := time.parse("2026-05-15 00:00:00", "YYYY-MM-DD HH:mm:ss", "UTC")
assert get_year(year_2026) == 2026

// 6. Check if leap year
func is_leap_year(year: int) -> bool {
    if year % 4 != 0 {
        return false
    }
    if year % 100 != 0 {
        return true
    }
    return year % 400 == 0
}
assert !is_leap_year(2023)
assert is_leap_year(2024)
assert !is_leap_year(1900)
assert is_leap_year(2000)

// 7. Days in month
func days_in_month(year: int, month: int) -> int {
    if month == 1 or month == 3 or month == 5 or month == 7 or month == 8 or month == 10 or month == 12 {
        return 31
    }
    if month == 4 or month == 6 or month == 9 or month == 11 {
        return 30
    }
    if month == 2 {
        if is_leap_year(year) {
            return 29
        }
        return 28
    }
    return 0
}
assert days_in_month(2026, 1) == 31
assert days_in_month(2026, 2) == 28
assert days_in_month(2024, 2) == 29
assert days_in_month(2026, 4) == 30

// 8. First day of month
func first_day_of_month(year: int, month: int) -> int {
    date_str := string(year) + "-" + string(month) + "-01 00:00:00"
    return time.parse(date_str, "YYYY-MM-DD HH:mm:ss", "UTC")
}
first_may := first_day_of_month(2026, 5)
date := time.date(first_may, "UTC")
assert date["day"] == 1
assert date["month"] == 5

// 9. Last day of month
func last_day_of_month(year: int, month: int) -> int {
    days := days_in_month(year, month)
    date_str := string(year) + "-" + string(month) + "-" + string(days) + " 23:59:59"
    return time.parse(date_str, "YYYY-MM-DD HH:mm:ss", "UTC")
}
last_may := last_day_of_month(2026, 5)
date_last := time.date(last_may, "UTC")
assert date_last["day"] == 31
assert date_last["month"] == 5

// 10. First day of year
func first_day_of_year(year: int) -> int {
    date_str := string(year) + "-01-01 00:00:00"
    return time.parse(date_str, "YYYY-MM-DD HH:mm:ss", "UTC")
}
first_2026 := first_day_of_year(2026)
date_first_year := time.date(first_2026, "UTC")
assert date_first_year["month"] == 1
assert date_first_year["day"] == 1

// 11. Last day of year
func last_day_of_year(year: int) -> int {
    date_str := string(year) + "-12-31 23:59:59"
    return time.parse(date_str, "YYYY-MM-DD HH:mm:ss", "UTC")
}
last_2026 := last_day_of_year(2026)
date_last_year := time.date(last_2026, "UTC")
assert date_last_year["month"] == 12
assert date_last_year["day"] == 31

// 12. Week of year (simplified)
func week_of_year(ts: int) -> int {
    // Simplified calculation
    start_of_year := first_day_of_year(get_year(ts))
    days_passed := (ts - start_of_year) / 86400
    return (days_passed / 7) + 1
}
mid_year := time.parse("2026-07-15 00:00:00", "YYYY-MM-DD HH:mm:ss", "UTC")
week_num := week_of_year(mid_year)
assert week_num > 20 and week_num < 35

// 13. Quarter of year
func quarter_of_year(month: int) -> int {
    if month >= 1 and month <= 3 {
        return 1
    }
    if month >= 4 and month <= 6 {
        return 2
    }
    if month >= 7 and month <= 9 {
        return 3
    }
    return 4
}
assert quarter_of_year(1) == 1
assert quarter_of_year(5) == 2
assert quarter_of_year(8) == 3
assert quarter_of_year(11) == 4

// 14. Is same day
func is_same_day(ts1: int, ts2: int) -> bool {
    date1 := time.date(ts1, "UTC")
    date2 := time.date(ts2, "UTC")
    return date1["year"] == date2["year"] and
           date1["month"] == date2["month"] and
           date1["day"] == date2["day"]
}
ts_morning := time.parse("2026-05-15 09:00:00", "YYYY-MM-DD HH:mm:ss", "UTC")
ts_evening := time.parse("2026-05-15 18:00:00", "YYYY-MM-DD HH:mm:ss", "UTC")
assert is_same_day(ts_morning, ts_evening)

ts_next_day := time.parse("2026-05-16 09:00:00", "YYYY-MM-DD HH:mm:ss", "UTC")
assert !is_same_day(ts_morning, ts_next_day)

// 15. Is same month
func is_same_month(ts1: int, ts2: int) -> bool {
    date1 := time.date(ts1, "UTC")
    date2 := time.date(ts2, "UTC")
    return date1["year"] == date2["year"] and
           date1["month"] == date2["month"]
}
assert is_same_month(ts_morning, ts_evening)
assert is_same_month(ts_morning, ts_next_day)

// 16. Is same year
func is_same_year(ts1: int, ts2: int) -> bool {
    date1 := time.date(ts1, "UTC")
    date2 := time.date(ts2, "UTC")
    return date1["year"] == date2["year"]
}
ts_jan := time.parse("2026-01-15 00:00:00", "YYYY-MM-DD HH:mm:ss", "UTC")
ts_dec := time.parse("2026-12-15 00:00:00", "YYYY-MM-DD HH:mm:ss", "UTC")
assert is_same_year(ts_jan, ts_dec)

// 17. Add months (simplified)
func add_months(ts: int, months: int) -> int {
    date := time.date(ts, "UTC")
    year := date["year"]
    month := date["month"] + months
    day := date["day"]

    while month > 12 {
        year = year + 1
        month = month - 12
    }
    while month < 1 {
        year = year - 1
        month = month + 12

    // Adjust day if exceeds month length
    }
    max_day := days_in_month(year, month)
    if day > max_day {
        day = max_day
    }
    date_str := string(year) + "-" + string(month) + "-" + string(day) + " 00:00:00"
    return time.parse(date_str, "YYYY-MM-DD HH:mm:ss", "UTC")
}
jan_ts := time.parse("2026-01-15 00:00:00", "YYYY-MM-DD HH:mm:ss", "UTC")
feb_ts := add_months(jan_ts, 1)
feb_date := time.date(feb_ts, "UTC")
assert feb_date["month"] == 2

// 18. Subtract months
func subtract_months(ts: int, months: int) -> int {
    return add_months(ts, -months)
}
mar_ts := time.parse("2026-03-15 00:00:00", "YYYY-MM-DD HH:mm:ss", "UTC")
jan_back := subtract_months(mar_ts, 2)
jan_date := time.date(jan_back, "UTC")
assert jan_date["month"] == 1

// 19. Add years
func add_years(ts: int, years: int) -> int {
    date := time.date(ts, "UTC")
    year := date["year"] + years
    month := date["month"]
    day := date["day"]

    // Handle Feb 29 on non-leap years
    if month == 2 and day == 29 and !is_leap_year(year) {
        day = 28
    }
    date_str := string(year) + "-" + string(month) + "-" + string(day) + " 00:00:00"
    return time.parse(date_str, "YYYY-MM-DD HH:mm:ss", "UTC")
}
leap_day := time.parse("2024-02-29 00:00:00", "YYYY-MM-DD HH:mm:ss", "UTC")
next_year := add_years(leap_day, 1)
next_date := time.date(next_year, "UTC")
assert next_date["day"] == 28 // Adjusted to Feb 28

// 20. Age calculation
func calculate_age(birth_ts: int, current_ts: int) -> int {
    birth_date := time.date(birth_ts, "UTC")
    current_date := time.date(current_ts, "UTC")

    age := current_date["year"] - birth_date["year"]
    if current_date["month"] < birth_date["month"] or
       (current_date["month"] == birth_date["month"] and current_date["day"] < birth_date["day"]):
        age = age - 1
    return age
}
birth := time.parse("1990-05-15 00:00:00", "YYYY-MM-DD HH:mm:ss", "UTC")
current := time.parse("2026-05-14 00:00:00", "YYYY-MM-DD HH:mm:ss", "UTC")
age := calculate_age(birth, current)
assert age == 35

next_day := time.parse("2026-05-15 00:00:00", "YYYY-MM-DD HH:mm:ss", "UTC")
age_next := calculate_age(birth, next_day)
assert age_next == 36

println("✓ test_29_calendar_operations passed")
