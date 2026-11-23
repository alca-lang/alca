// Copyright 2026 Pawl Programming Language
// Test 75: Functional pipes - |> chaining and functional composition.

// 1. Basic pipe chaining
func add_one(x: int) -> int {
    return x + 1
}
func multiply_two(x: int) -> int {
    return x * 2
}
result := 5 |> add_one |> multiply_two
assert result == 12

// 2. Pipe with string operations
func to_upper(s: string) -> string {
    use std.string
    return string.upper(s)
}
func append_exclamation(s: string) -> string {
    return s + "!"
}
text_result := "hello" |> to_upper |> append_exclamation
assert text_result == "HELLO!"

// 3. Pipe with list operations
func double_list(items: list[int]) -> list[int] {
    result: list[int] = []
    for item in items {
        result = result + [item * 2]
    }
    return result
}
func sum_list(items: list[int]) -> int {
    total := 0
    for item in items {
        total = total + item
    }
    return total
}
list_result := [1, 2, 3] |> double_list |> sum_list
assert list_result == 12

// 4. Function composition
func compose(f: func(int) -> int, g: func(int) -> int) -> func(int) -> int {
    return func(x: int) -> int:
        return f(g(x))
}
composed := compose(multiply_two, add_one)
assert composed(5) == 12

// 5. Multiple composition
func add_five(x: int) -> int {
    return x + 5
}
func square(x: int) -> int {
    return x * x
}
multi_composed := compose(square, compose(add_five, add_one))
assert multi_composed(2) == 64 // ((2+1)+5)^2 = 64

// 6. Pipe with filter
func filter_even(items: list[int]) -> list[int] {
    result: list[int] = []
    for item in items {
        if item % 2 == 0 {
            result = result + [item]
        }
    }
    return result
}
filtered := [1, 2, 3, 4, 5] |> filter_even
assert filtered == [2, 4]

// 7. Pipe with map
func map_squared(items: list[int]) -> list[int] {
    result: list[int] = []
    for item in items {
        result = result + [item * item]
    }
    return result
}
mapped := [1, 2, 3] |> map_squared
assert mapped == [1, 4, 9]

// 8. Pipe chain with multiple operations
func take_first(items: list[int]) -> int {
    if len(items) > 0 {
        return items[0]
    }
    return 0
}
first_squared := [5, 10, 15] |> take_first |> square
assert first_squared == 25

// 9. Pipe with conditional
func if_positive(x: int) -> int {
    if x > 0 {
        return x
    }
    return 0
}
conditional := -5 |> if_positive
assert conditional == 0

positive_cond := 5 |> if_positive
assert positive_cond == 5

// 10. Pipe with validation
func validate_positive(x: int) -> int {
    if x < 0 {
        err("Value must be positive", code=400)
        return 0
    }
    return x
}
validated := 10 |> validate_positive
assert validated == 10

println("✓ test_75_functional_pipes passed")
