// Copyright 2026 Pawl Programming Language
// Test 90: Decorator pattern - function wrapping and enhancement.

// 1. Simple decorator simulation
func add_logging(f: func(int) -> int) -> func(int) -> int {
    return func(x: int) -> int:
        println("Calling function with " + string(x))
        result := f(x)
        println("Result: " + string(result))
        return result
}
original := func(n: int) -> int:
    return n * 2

decorated := add_logging(original)
result := decorated(5)
assert result == 10

// 2. Decorator with validation
func validate_input(f: func(int) -> int) -> func(int) -> int {
    return func(x: int) -> int:
        if x < 0 {
            return 0
        }
        return f(x)
}
validated := validate_input(original)
result2 := validated(-5)
assert result2 == 0

// 3. Decorator with timing
func time_it(f: func(int) -> int) -> func(int) -> int {
    return func(x: int) -> int:
        use std.time
        start := time.now()
        result := f(x)
        elapsed := time.now() - start
        println("Time: " + string(elapsed))
        return result
}
timed := time_it(original)
result3 := timed(5)
assert result3 == 10

// 4. Decorator with retry
func retry_decorator(f: func(int) -> int, max_retries: int) -> func(int) -> int {
    return func(x: int) -> int:
        attempts := 0
        while attempts < max_retries {
            result := f(x)
            if result > 0 {
                return result
            }
            attempts = attempts + 1
        }
        return 0
}
retried := retry_decorator(original, 3)
result4 := retried(5)
assert result4 == 10

// 5. Decorator composition
func compose_decorators(f: func(int) -> int) -> func(int) -> int {
    return add_logging(validate_input(f))
}
composed := compose_decorators(original)
result5 := composed(5)
assert result5 == 10

// 6. Decorator with caching
cache: map[int, int] = {}

func cache_decorator(f: func(int) -> int) -> func(int) -> int {
    return func(x: int) -> int:
        if x in cache {
            return cache[x]
        }
        result := f(x)
        cache[x] = result
        return result
}
cached := cache_decorator(original)
result6 := cached(5)
assert result6 == 10
assert cached(5) == 10 // Should use cache

// 7. Decorator with pre/post processing
func pre_post(f: func(int) -> int) -> func(int) -> int {
    return func(x: int) -> int:
        println("Pre: " + string(x))
        result := f(x)
        println("Post: " + string(result))
        return result
}
pre_posted := pre_post(original)
result7 := pre_posted(5)
assert result7 == 10

// 8. Decorator with argument modification
func double_arg(f: func(int) -> int) -> func(int) -> int {
    return func(x: int) -> int:
        return f(x * 2)
}
doubled_arg := double_arg(original)
result8 := doubled_arg(5)
assert result8 == 20

// 9. Decorator with result modification
func double_result(f: func(int) -> int) -> func(int) -> int {
    return func(x: int) -> int:
        return f(x) * 2
}
doubled_result := double_result(original)
result9 := doubled_result(5)
assert result9 == 20

// 10. Conditional decorator
func conditional_decorator(condition: bool, f: func(int) -> int) -> func(int) -> int {
    if condition {
        return add_logging(f)
    }
    return f
}
conditional := conditional_decorator(true, original)
result10 := conditional(5)
assert result10 == 10

println("✓ test_90_decorator_pattern passed")
