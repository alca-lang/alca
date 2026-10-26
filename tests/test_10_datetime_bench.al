// Copyright 2026 Alca Programming Language
// Everyday Test 10: Date/time operations, parsing, formatting, duration diffs, and benchmark timing.

use std.time;

// 1. Timestamps and current time
ts := time.now();
assert ts > 1700000000; // modern epoch timestamp

ms := time.now_ms();
assert ms > ts * 1000;

// 2. Parsing and Formatting with UTC location for deterministic round-trip
target_date_str := "2026-05-15 12:00:00";
parsed_ts := time.parse(target_date_str, "YYYY-MM-DD HH:mm:ss", "UTC");
assert parsed_ts > 1700000000;

formatted_back := time.format(parsed_ts, "YYYY-MM-DD HH:mm:ss", "UTC");
assert formatted_back == target_date_str;

// 3. Date breakdown (calendar components in UTC)
dt := time.date(parsed_ts, "UTC");
assert dt["year"] == 2026;
assert dt["month"] == 5;
assert dt["day"] == 15;
assert dt["hour"] == 12;

// 4. Time difference and duration
t1 := time.parse("2026-01-01 00:00:00", "YYYY-MM-DD HH:mm:ss", "UTC");
t2 := time.parse("2026-01-01 01:30:00", "YYYY-MM-DD HH:mm:ss", "UTC");
difference_sec := time.diff(t2, t1);
assert difference_sec == 5400.0; // 90 minutes = 5400 seconds

// 5. Benchmark timing simulation
start_time := time.now_ms();

// Perform everyday workload loop (fibonacci calculation)
func fib(n: int) -> int:
    if n <= 1:
        return n;
    return fib(n - 1) + fib(n - 2);

res := fib(15);
assert res == 610;

end_time := time.now_ms();
elapsed := end_time - start_time;
assert elapsed >= 0;

// 6. Non-blocking/small sleep
time.sleep(1);

println("✓ test_10_datetime_bench passed");
