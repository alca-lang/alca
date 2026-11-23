// Copyright 2026 Pawl Programming Language
// Test 88: Memoization - caching results for performance.

// 1. Simple memoization
memo_cache: map[string, int] = {}

func memoized_fib(n: int) -> int {
    key := string(n)
    if key in memo_cache {
        return memo_cache[key]
    }
    if n <= 1 {
        return n
    }
    result := memoized_fib(n - 1) + memoized_fib(n - 2)
    memo_cache[key] = result
    return result
}
assert memoized_fib(10) == 55
assert memoized_fib(10) == 55 // Should use cache

// 2. Memoize expensive function
expensive_cache: map[int, int] = {}

func expensive_calculation(x: int) -> int {
    if x in expensive_cache {
        return expensive_cache[x]
    }
    result := x * x * x
    expensive_cache[x] = result
    return result
}
assert expensive_calculation(5) == 125
assert expensive_calculation(5) == 125

// 3. Clear memoization cache
func clear_memo_cache() {
    for key in memo_cache {
        memo_cache[key] = 0

// 4. Memoization with multiple arguments
    }
}
multi_cache: map[string, int] = {}

func memoized_add(a: int, b: int) -> int {
    key := string(a) + ":" + string(b)
    if key in multi_cache {
        return multi_cache[key]
    }
    result := a + b
    multi_cache[key] = result
    return result
}
assert memoized_add(5, 10) == 15
assert memoized_add(5, 10) == 15

// 5. Memoization cache size
func cache_size() -> int {
    count := 0
    for key in memo_cache {
        count = count + 1
    }
    return count
}
size := cache_size()
assert size > 0

// 6. Memoization hit rate
hits := 0
misses := 0

func memoized_with_stats(n: int) -> int {
    key := string(n)
    if key in memo_cache {
        hits = hits + 1
        return memo_cache[key]
    }
    misses = misses + 1
    if n <= 1 {
        return n
    }
    result := memoized_with_stats(n - 1) + memoized_with_stats(n - 2)
    memo_cache[key] = result
    return result
}
memoized_with_stats(5)
assert misses > 0

// 7. Memoization with expiration (simulation)
struct CacheEntry {
    value: int
    timestamp: int = 0
}
timed_cache: map[string, CacheEntry] = {}

func memoized_timed(n: int) -> int {
    key := string(n)
    use std.time
    current := int(time.now())
    if key in timed_cache {
        entry := timed_cache[key]
        if current - entry.timestamp < 1000 { // 1 second expiry
            return entry.value
        }
    }
    result := n * n
    timed_cache[key] = CacheEntry(result, current)
    return result
}
assert memoized_timed(10) == 100

// 8. Memoization capacity limit
func limited_memo(n: int, limit: int) -> int {
    if cache_size() >= limit {
        clear_memo_cache()
    }
    return memoized_fib(n)
}
limited := limited_memo(8, 5)
assert limited == 21

// 9. Memoization key collision check
func unique_key(a: int, b: int) -> string {
    return string(a) + "_" + string(b)
}
key1 := unique_key(1, 2)
key2 := unique_key(12, 3)
assert key1 != key2

// 10. Memoization with string keys
str_cache: map[string, string] = {}

func memoized_upper(s: string) -> string {
    use std.string
    if s in str_cache {
        return str_cache[s]
    }
    result := string.upper(s)
    str_cache[s] = result
    return result
}
assert memoized_upper("hello") == "HELLO"
assert memoized_upper("hello") == "HELLO"

println("✓ test_88_memoization passed")
