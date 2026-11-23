// Copyright 2026 Pawl Programming Language
// Test 32: Thread pools (@threads) - workload distribution and parallel execution.

// 1. Simple thread pool execution
@threads(4) {
    i := 0
    while i < 100 {
        i = i + 1
    }
    println("Thread completed work")
}

// 2. Thread pool with data processing
@threads(8) {
    numbers := [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]
    total := 0
    for n in numbers {
        total = total + n
    }
    assert total == 55
}

// 3. Thread pool with independent calculations
@threads(4) {
    func calculate_square(n: int) -> int {
        return n * n
    }
    result := calculate_square(12)
    assert result == 144
}

// 4. Thread pool with string operations
@threads(2) {
    use std.string
    text := "parallel processing"
    words := string.split(text, " ")
    assert len(words) == 2
}

// 5. Thread pool with collection filtering
@threads(4) {
    numbers := [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]
    evens: list[int] = []
    for n in numbers {
        if n % 2 == 0 {
            evens = evens + [n]
        }
    }
    assert len(evens) == 5
}

// 6. Thread pool with map operations
@threads(4) {
    data := {"a": 1, "b": 2, "c": 3}
    total := 0
    for key in data {
        total = total + data[key]
    }
    assert total == 6
}

// 7. Thread pool with JSON processing
@threads(2) {
    use std.json
    payload := "{\"user\": \"test\", \"id\": 123}"
    parsed := json.parse(payload)
    assert parsed["user"] == "test"
}

// 8. Thread pool with file operations simulation
@threads(2) {
    use std.io
    test_file := "./thread_test.txt"
    io.write_file(test_file, "Thread pool test")
    assert io.file_exists(test_file)
    io.delete_file(test_file)
}

// 9. Thread pool with cryptographic operations
@threads(4) {
    use std.crypto
    hash := crypto.sha256("thread pool test")
    assert len(hash) == 64
}

// 10. Thread pool with math operations
@threads(4) {
    use std.math
    result := math.sqrt(144.0)
    assert result == 12.0
}

// 11. Thread pool with nested loops
@threads(2) {
    total := 0
    i := 0
    while i < 10 {
        j := 0
        while j < 10 {
            total = total + (i * j)
            j = j + 1
        }
        i = i + 1
    }
    assert total == 2025
}

// 12. Thread pool with function composition
func add(x: int, y: int) -> int {
    return x + y
}
func multiply(x: int, y: int) -> int {
    return x * y
}
@threads(4) {
    result := multiply(add(5, 3), 2)
    assert result == 16
}

// 13. Thread pool with conditional execution
@threads(4) {
    value := 25
    if value > 20 {
        result := value / 5
        assert result == 5
    }
}

// 14. Thread pool with array operations
@threads(4) {
    data := [10, 20, 30, 40, 50]
    doubled: list[int] = []
    for val in data {
        doubled = doubled + [val * 2]
    }
    assert doubled[0] == 20
    assert doubled[4] == 100
}

// 15. Thread pool with string manipulation
@threads(2) {
    use std.string
    text := "HELLO WORLD"
    lower := string.lower(text)
    assert lower == "hello world"
}

// 16. Thread pool with range operations
@threads(4) {
    total := 0
    for i in 1..=100 {
        total = total + i
    }
    assert total == 5050
}

// 17. Thread pool with error handling
@threads(2) {
    err_slot: error | null = null
    result := 10 / 2 ?>>> err_slot
    assert err_slot == null
    assert result == 5
}

// 18. Thread pool with struct operations
struct Point {
    x: int
    y: int
}
@threads(4) {
    p := Point(10, 20)
    assert p.x == 10
    assert p.y == 20
}

// 19. Thread pool with time operations
@threads(2) {
    use std.time
    ts := time.now()
    assert ts > 1700000000
}

// 20. Thread pool with boolean logic
@threads(4) {
    a := true
    b := false
    result := a and !b
    assert result
}

println("✓ test_32_thread_pools passed")
