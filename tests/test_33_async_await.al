// Copyright 2026 Pawl Programming Language
// Test 33: Async/await - async functions, await, and promise-like patterns.

// 1. Simple async function
async fn fetch_data() -> int {
    return 42
}
result := await fetch_data()
assert result == 42

// 2. Async function with computation
async fn calculate_sum(a: int, b: int) -> int {
    return a + b
}
sum_result := await calculate_sum(10, 20)
assert sum_result == 30

// 3. Async function with string operations
async fn process_text(text: string) -> string {
    use std.string
    return string.upper(text)
}
text_result := await process_text("hello")
assert text_result == "HELLO"

// 4. Async function with delay simulation
async fn delayed_value() -> int {
    // Simulate async delay
    return 100
}
delayed := await delayed_value()
assert delayed == 100

// 5. Chained async calls
async fn step1() -> int {
    return 10
}
async fn step2(input: int) -> int {
    return input * 2
}
async fn step3(input: int) -> int {
    return input + 5
}
chain_result := await step3(await step2(await step1()))
assert chain_result == 25

// 6. Async function with error handling
async fn might_fail(should_fail: bool) -> int {
    if should_fail {
        err("Operation failed", code=500)
        return 0
    }
    return 42
}
success := await might_fail(false)
assert success == 42

// 7. Async function with collection operations
async fn process_list(items: list[int]) -> int {
    total := 0
    for item in items {
        total = total + item
    }
    return total
}
list_result := await process_list([1, 2, 3, 4, 5])
assert list_result == 15

// 8. Async function with map operations
async fn count_keys(data: map[string, int]) -> int {
    count := 0
    for key in data {
        count = count + 1
    }
    return count
}
map_result := await count_keys({"a": 1, "b": 2, "c": 3})
assert map_result == 3

// 9. Async function with JSON
async fn parse_json(json_str: string) -> map[string, any] {
    use std.json
    return json.parse(json_str)
}
json_result := await parse_json("{\"key\": \"value\"}")
assert json_result["key"] == "value"

// 10. Async function with file operations
async fn read_file_content(path: string) -> string {
    use std.io
    return io.read_file(path)

// Note: This would require actual file
// file_content := await read_file_content("./test.txt");

// 11. Async function with crypto operations
}
async fn hash_data(data: string) -> string {
    use std.crypto
    return crypto.sha256(data)
}
hash_result := await hash_data("test data")
assert len(hash_result) == 64

// 12. Async function with math operations
async fn complex_calc(x: float, y: float) -> float {
    use std.math
    return math.sqrt(x * x + y * y)
}
calc_result := await complex_calc(3.0, 4.0)
assert calc_result == 5.0

// 13. Async function with conditional logic
async fn conditional_return(x: int) -> string {
    if x > 10 {
        return "large"
    }
    return "small"
}
cond_result := await conditional_return(15)
assert cond_result == "large"

// 14. Async function with loop
async fn sum_range(start: int, end: int) -> int {
    total := 0
    for i in start..=end {
        total = total + i
    }
    return total
}
range_result := await sum_range(1, 10)
assert range_result == 55

// 15. Multiple async calls in sequence
async fn task_a() -> int {
    return 1
}
async fn task_b() -> int {
    return 2
}
async fn task_c() -> int {
    return 3
}
seq_result := await task_a() + await task_b() + await task_c()
assert seq_result == 6

// 16. Async function with struct
struct User {
    id: int
    name: string
}
async fn create_user(id: int, name: string) -> User {
    return User(id, name)
}
user := await create_user(123, "Alice")
assert user.id == 123
assert user.name == "Alice"

// 17. Async function with nested operations
async fn nested_calc(x: int) -> int {
    inner := x * 2
    outer := inner + 10
    return outer
}
nested_result := await nested_calc(5)
assert nested_result == 20

// 18. Async function with boolean return
async fn check_condition(x: int) -> bool {
    return x > 0
}
bool_result := await check_condition(5)
assert bool_result

// 19. Async function with list transformation
async fn double_list(items: list[int]) -> list[int] {
    result: list[int] = []
    for item in items {
        result = result + [item * 2]
    }
    return result
}
doubled := await double_list([1, 2, 3])
assert doubled == [2, 4, 6]

// 20. Async function with string formatting
async fn format_name(first: string, last: string) -> string {
    return first + " " + last
}
formatted := await format_name("John", "Doe")
assert formatted == "John Doe"

println("✓ test_33_async_await passed")
