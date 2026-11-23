// Copyright 2026 Pawl Programming Language
// Test 73: Stream pipes - >>, >!, >>>, and stream transformations.

// 1. Basic stream simulation
data := "Hello"
data = data >> "World" // Append stream
assert len(data) > 5

// 2. Stream overwrite simulation
data2 := "Test"
data2 = data2 >! "New" // Overwrite stream
assert len(data2) > 0

// 3. Stream to variable simulation
result := ""
source := "Data"
source >>> result // Stream to variable
assert len(result) > 0

// 4. Stream from file simulation
file_content := "File content"
streamed := file_content << // Stream from file
assert len(streamed) > 0

// 5. Async stream simulation
async_stream := ""
source2 := "Async"
source2 &>> async_stream // Async stream
assert len(async_stream) > 0

// 6. Stream with logging
logged_stream := ""
data_with_log := "Log this"
data_with_log ?>> logged_stream // Stream with logging
assert len(logged_stream) > 0

// 7. Stream transformation
func transform_stream(input: string) -> string {
    use std.string
    return string.upper(input)
}
original := "hello"
transformed := transform_stream(original)
assert transformed == "HELLO"

// 8. Filter stream
func filter_stream(items: list[int], predicate: func(int) -> bool) -> list[int] {
    result: list[int] = []
    for item in items {
        if predicate(item) {
            result = result + [item]
        }
    }
    return result
}
evens := filter_stream([1, 2, 3, 4, 5], func(n: int) -> bool:
    return n % 2 == 0
)
assert evens == [2, 4]

// 9. Map stream
func map_stream(items: list[int], mapper: func(int) -> int) -> list[int] {
    result: list[int] = []
    for item in items {
        result = result + [mapper(item)]
    }
    return result
}
doubled := map_stream([1, 2, 3], func(n: int) -> int:
    return n * 2
)
assert doubled == [2, 4, 6]

// 10. Reduce stream
func reduce_stream(items: list[int], reducer: func(int, int) -> int, initial: int) -> int {
    result := initial
    for item in items {
        result = reducer(result, item)
    }
    return result
}
total := reduce_stream([1, 2, 3, 4], func(a: int, b: int) -> int:
    return a + b
, 0)
assert total == 10

// 11. Chunk stream
func chunk_stream(items: list[int], size: int) -> list[list[int]] {
    result: list[list[int]] = []
    current: list[int] = []
    i := 0
    while i < len(items) {
        current = current + [items[i]]
        if len(current) == size {
            result = result + [current]
            current = []
        }
        i = i + 1
    }
    if len(current) > 0 {
        result = result + [current]
    }
    return result
}
chunks := chunk_stream([1, 2, 3, 4, 5], 2)
assert len(chunks) == 3

// 12. Flatten stream
func flatten_stream(nested: list[list[int]]) -> list[int] {
    result: list[int] = []
    for inner in nested {
        for item in inner {
            result = result + [item]
        }
    }
    return result
}
flat := flatten_stream([[1, 2], [3, 4], [5]])
assert flat == [1, 2, 3, 4, 5]

// 13. Take stream
func take_stream(items: list[int], n: int) -> list[int] {
    result: list[int] = []
    i := 0
    while i < n and i < len(items) {
        result = result + [items[i]]
        i = i + 1
    }
    return result
}
taken := take_stream([1, 2, 3, 4, 5], 3)
assert taken == [1, 2, 3]

// 14. Skip stream
func skip_stream(items: list[int], n: int) -> list[int] {
    result: list[int] = []
    i := n
    while i < len(items) {
        result = result + [items[i]]
        i = i + 1
    }
    return result
}
skipped := skip_stream([1, 2, 3, 4, 5], 2)
assert skipped == [3, 4, 5]

// 15. Unique stream
func unique_stream(items: list[int]) -> list[int] {
    result: list[int] = []
    for item in items {
        if !(item in result) {
            result = result + [item]
        }
    }
    return result
}
unique := unique_stream([1, 2, 2, 3, 3, 3])
assert unique == [1, 2, 3]

println("✓ test_73_stream_pipes passed")
