// Copyright 2026 Pawl Programming Language
// Test 76: Stream transformations - map, filter, reduce patterns.

// 1. Map transformation
func map_transform(items: list[int], transform: func(int) -> int) -> list[int] {
    result: list[int] = []
    for item in items {
        result = result + [transform(item)]
    }
    return result
}
mapped := map_transform([1, 2, 3], func(x: int) -> int:
    return x * 2
)
assert mapped == [2, 4, 6]

// 2. Filter transformation
func filter_transform(items: list[int], predicate: func(int) -> bool) -> list[int] {
    result: list[int] = []
    for item in items {
        if predicate(item) {
            result = result + [item]
        }
    }
    return result
}
filtered := filter_transform([1, 2, 3, 4, 5], func(x: int) -> bool:
    return x % 2 == 0
)
assert filtered == [2, 4]

// 3. Reduce transformation
func reduce_transform(items: list[int], reducer: func(int, int) -> int, initial: int) -> int {
    result := initial
    for item in items {
        result = reducer(result, item)
    }
    return result
}
reduced := reduce_transform([1, 2, 3, 4], func(a: int, b: int) -> int:
    return a + b
, 0)
assert reduced == 10

// 4. Map-Filter-Reduce chain
numbers := [1, 2, 3, 4, 5]
step1 := map_transform(numbers, func(x: int) -> int:
    return x * x
)
step2 := filter_transform(step1, func(x: int) -> bool:
    return x > 5
)
step3 := reduce_transform(step2, func(a: int, b: int) -> int:
    return a + b
, 0)
assert step3 == 16 + 25 // 4^2 + 5^2

// 5. Group by transformation
func group_by(items: list[int], key_func: func(int) -> int) -> map[int, list[int]] {
    result: map[int, list[int]] = {}
    for item in items {
        key := key_func(item)
        if key in result {
            result[key] = result[key] + [item]
        }
        else {
            result[key] = [item]
        }
    }
    return result
}
grouped := group_by([1, 2, 3, 4, 5, 6], func(x: int) -> int:
    return x % 2
)
assert len(grouped[0]) == 3 // Even numbers
assert len(grouped[1]) == 3 // Odd numbers

// 6. Flat map transformation
func flat_map(items: list[int], mapper: func(int) -> list[int]) -> list[int] {
    result: list[int] = []
    for item in items {
        mapped := mapper(item)
        for m in mapped {
            result = result + [m]
        }
    }
    return result
}
flat_mapped := flat_map([1, 2], func(x: int) -> list[int]:
    return [x, x * 2]
)
assert flat_mapped == [1, 2, 2, 4]

// 7. Partition transformation
func partition(items: list[int], predicate: func(int) -> bool) -> (list[int], list[int]) {
    true_list: list[int] = []
    false_list: list[int] = []
    for item in items {
        if predicate(item) {
            true_list = true_list + [item]
        }
        else {
            false_list = false_list + [item]
        }
    }
    return (true_list, false_list)
}
(evens, odds) := partition([1, 2, 3, 4, 5], func(x: int) -> bool:
    return x % 2 == 0
)
assert evens == [2, 4]
assert odds == [1, 3, 5]

// 8. Take while transformation
func take_while(items: list[int], predicate: func(int) -> bool) -> list[int] {
    result: list[int] = []
    for item in items {
        if predicate(item) {
            result = result + [item]
        }
        else {
            break
        }
    }
    return result
}
taken := take_while([1, 2, 3, 4, 5], func(x: int) -> bool:
    return x < 4
)
assert taken == [1, 2, 3]

// 9. Drop while transformation
func drop_while(items: list[int], predicate: func(int) -> bool) -> list[int] {
    result: list[int] = []
    dropping := true
    for item in items {
        if dropping and predicate(item) {
            continue
        }
        dropping = false
        result = result + [item]
    }
    return result
}
dropped := drop_while([1, 2, 3, 4, 5], func(x: int) -> bool:
    return x < 3
)
assert dropped == [3, 4, 5]

// 10. Distinct transformation
func distinct(items: list[int]) -> list[int] {
    result: list[int] = []
    for item in items {
        if !(item in result) {
            result = result + [item]
        }
    }
    return result
}
distincted := distinct([1, 2, 2, 3, 3, 3])
assert distincted == [1, 2, 3]

// 11. Zip transformation
func zip(a: list[int], b: list[int]) -> list[list[int]] {
    result: list[list[int]] = []
    min_len := len(a)
    if len(b) < min_len {
        min_len = len(b)
    }
    i := 0
    while i < min_len {
        result = result + [[a[i], b[i]]]
        i = i + 1
    }
    return result
}
zipped := zip([1, 2, 3], [4, 5, 6])
assert zipped == [[1, 4], [2, 5], [3, 6]]

// 12. Unzip transformation
func unzip(pairs: list[list[int]]) -> (list[int], list[int]) {
    first: list[int] = []
    second: list[int] = []
    for pair in pairs {
        first = first + [pair[0]]
        second = second + [pair[1]]
    }
    return (first, second)
}
(first, second) := unzip([[1, 4], [2, 5], [3, 6]])
assert first == [1, 2, 3]
assert second == [4, 5, 6]

// 13. Reverse transformation
func reverse(items: list[int]) -> list[int] {
    result: list[int] = []
    i := len(items) - 1
    while i >= 0 {
        result = result + [items[i]]
        i = i - 1
    }
    return result
}
reversed := reverse([1, 2, 3, 4])
assert reversed == [4, 3, 2, 1]

// 14. Sort transformation
func sort_items(items: list[int]) -> list[int] {
    result: list[int] = []
    for item in items {
        result = result + [item]
    }
    i := 0
    while i < len(result) {
        j := i + 1
        while j < len(result) {
            if result[j] < result[i] {
                temp := result[i]
                result[i] = result[j]
                result[j] = temp
            }
            j = j + 1
        }
        i = i + 1
    }
    return result
}
sorted := sort_items([3, 1, 4, 1, 5])
assert sorted == [1, 1, 3, 4, 5]

// 15. Slice transformation
func slice(items: list[int], start: int, end: int) -> list[int] {
    if start < 0 {
        start = 0
    }
    if end > len(items) {
        end = len(items)
    }
    result: list[int] = []
    i := start
    while i < end {
        result = result + [items[i]]
        i = i + 1
    }
    return result
}
sliced := slice([1, 2, 3, 4, 5], 1, 4)
assert sliced == [2, 3, 4]

println("✓ test_76_stream_transformations passed")
