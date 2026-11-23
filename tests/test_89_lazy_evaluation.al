// Copyright 2026 Pawl Programming Language
// Test 89: Lazy evaluation - deferred computation.

// 1. Lazy value simulation
struct LazyValue {
    computed: bool = false
    value: int = 0
}
func get_lazy(lv: LazyValue) -> int {
    if !lv.computed {
        lv.value = 42
        lv.computed = true
    }
    return lv.value
}
lazy := LazyValue(false, 0)
result := get_lazy(lazy)
assert result == 42
assert lazy.computed

// 2. Lazy computation
func compute_if_needed(needed: bool) -> int {
    if needed {
        return 100
    }
    return 0
}
computed := compute_if_needed(true)
assert computed == 100

// 3. Lazy list generation
func generate_lazy_list(n: int) -> list[int] {
    result: list[int] = []
    i := 0
    while i < n {
        result = result + [i * i]
        i = i + 1
    }
    return result
}
lazy_list := generate_lazy_list(5)
assert lazy_list == [0, 1, 4, 9, 16]

// 4. Lazy function application
func lazy_apply(f: func(int) -> int, x: int) -> func() -> int {
    return func() -> int:
        return f(x)
}
lazy_func := lazy_apply(func(n: int) -> int:
    return n * 2
, 5)
assert lazy_func() == 10

// 5. Lazy map
func lazy_map(items: list[int], mapper: func(int) -> int) -> list[int] {
    result: list[int] = []
    for item in items {
        result = result + [mapper(item)]
    }
    return result
}
lazy_mapped := lazy_map([1, 2, 3], func(n: int) -> int:
    return n * n
)
assert lazy_mapped == [1, 4, 9]

// 6. Lazy filter
func lazy_filter(items: list[int], predicate: func(int) -> bool) -> list[int] {
    result: list[int] = []
    for item in items {
        if predicate(item) {
            result = result + [item]
        }
    }
    return result
}
lazy_filtered := lazy_filter([1, 2, 3, 4], func(n: int) -> bool:
    return n % 2 == 0
)
assert lazy_filtered == [2, 4]

// 7. Lazy take
func lazy_take(items: list[int], n: int) -> list[int] {
    result: list[int] = []
    i := 0
    while i < n and i < len(items) {
        result = result + [items[i]]
        i = i + 1
    }
    return result
}
lazy_taken := lazy_take([1, 2, 3, 4, 5], 3)
assert lazy_taken == [1, 2, 3]

// 8. Lazy drop
func lazy_drop(items: list[int], n: int) -> list[int] {
    result: list[int] = []
    i := n
    while i < len(items) {
        result = result + [items[i]]
        i = i + 1
    }
    return result
}
lazy_dropped := lazy_drop([1, 2, 3, 4, 5], 2)
assert lazy_dropped == [3, 4, 5]

// 9. Lazy range
func lazy_range(start: int, end: int) -> list[int] {
    result: list[int] = []
    i := start
    while i < end {
        result = result + [i]
        i = i + 1
    }
    return result
}
lazy_ranged := lazy_range(0, 5)
assert lazy_ranged == [0, 1, 2, 3, 4]

// 10. Lazy repeat
func lazy_repeat(value: int, n: int) -> list[int] {
    result: list[int] = []
    i := 0
    while i < n {
        result = result + [value]
        i = i + 1
    }
    return result
}
lazy_repeated := lazy_repeat(5, 3)
assert lazy_repeated == [5, 5, 5]

println("✓ test_89_lazy_evaluation passed")
