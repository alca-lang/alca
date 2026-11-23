// Copyright 2026 Pawl Programming Language
// Test 31: Background tasks (@ba) - timeout, result handling, and async execution.

// 1. Simple background task
@ba {
    result := 42
    println("Background task completed: " + string(result))
}

// 2. Background task with timeout
@ba(5000) {
    // Task that completes within 5 seconds
    total := 0
    i := 0
    while i < 100 {
        total = total + i
        i = i + 1
    }
    println("Task result: " + string(total))
}

// 3. Background task with return value simulation
func background_calculation() -> int {
    total := 0
    i := 0
    while i < 1000 {
        total = total + i
        i = i + 1
    }
    return total
}
@ba {
    result := background_calculation()
    assert result == 499500 // Sum of 0..999
}

// 4. Multiple background tasks
@ba {
    println("Task 1 started")
}

@ba {
    println("Task 2 started")
}

@ba {
    println("Task 3 started")
}

// 5. Background task with error handling
@ba {
    err_slot: error | null = null
    result := 10 / 0 ?>>> err_slot
    if err_slot != null {
        println("Error caught in background: " + err_slot.msg)
    }
}

// 6. Background task with file I/O simulation
@ba {
    use std.io
    test_file := "./bg_test.txt"
    io.write_file(test_file, "Background task wrote this")
    assert io.file_exists(test_file)
    io.delete_file(test_file)
}

// 7. Background task with network simulation
@ba {
    // Simulate network request
    use std.crypto
    token := crypto.random_string(16, "0123456789abcdef")
    assert len(token) == 16
}

// 8. Background task with computation
@ba {
    // Fibonacci calculation
    func fib(n: int) -> int {
        if n <= 1 {
            return n
        }
        return fib(n - 1) + fib(n - 2)
    }
    result := fib(20)
    assert result == 6765
}

// 9. Background task with data processing
@ba {
    data := [1, 2, 3, 4, 5]
    total := 0
    for val in data {
        total = total + val
    }
    assert total == 15
}

// 10. Background task with timeout that should succeed
@ba(10000) {
    // Long-running task that completes within timeout
    i := 0
    while i < 100000 {
        i = i + 1
    }
    println("Long task completed")
}

// 11. Background task chaining simulation
func task1() -> int {
    return 10
}
func task2(input: int) -> int {
    return input * 2
}
@ba {
    result1 := task1()
    result2 := task2(result1)
    assert result2 == 20
}

// 12. Background task with string operations
@ba {
    use std.string
    text := "Hello World"
    upper := string.upper(text)
    assert upper == "HELLO WORLD"
}

// 13. Background task with JSON processing
@ba {
    use std.json
    data := {"key": "value", "number": 42}
    encoded := json.encode(data)
    assert json.validate(encoded)
}

// 14. Background task with collection operations
@ba {
    numbers := [5, 2, 8, 1, 9]
    // Simple sort
    i := 0
    while i < len(numbers) {
        j := i + 1
        while j < len(numbers) {
            if numbers[j] < numbers[i] {
                temp := numbers[i]
                numbers[i] = numbers[j]
                numbers[j] = temp
            }
            j = j + 1
        }
        i = i + 1
    }
    assert numbers[0] == 1
    assert numbers[4] == 9
}

// 15. Background task with map operations
@ba {
    scores := {"alice": 95, "bob": 87, "charlie": 92}
    total := 0
    for key in scores {
        total = total + scores[key]
    }
    assert total == 274
}

// 16. Background task with conditional logic
@ba {
    value := 15
    if value > 10 {
        result := value * 2
        assert result == 30
    }
}

// 17. Background task with loop
@ba {
    count := 0
    i := 0
    while i < 100 {
        count = count + 1
        i = i + 1
    }
    assert count == 100
}

// 18. Background task with nested structures
@ba {
    struct Nested {
        value: int
        nested: Nested = null
    }
    outer := Nested(10, null)
    assert outer.value == 10
}

// 19. Background task with function calls
func helper(x: int) -> int {
    return x * x
}
@ba {
    result := helper(5)
    assert result == 25
}

// 20. Background task completion check
@ba {
    completed := true
    assert completed
}

println("✓ test_31_background_tasks passed")
