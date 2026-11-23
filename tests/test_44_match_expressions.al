// Copyright 2026 Pawl Programming Language
// Test 44: Match expressions - pattern matching with guards.

// 1. Simple match on value
value := 5
matched := ""
match value {
    1:
        matched = "one"
    5:
        matched = "five"
    _:
        matched = "other"
}
assert matched == "five"

// 2. Match on string
text := "hello"
result := ""
match text {
    "hello":
        result = "greeting"
    "goodbye":
        result = "farewell"
    _:
        result = "unknown"
}
assert result == "greeting"

// 3. Match with multiple patterns
num := 10
description := ""
match num {
    1, 2, 3:
        description = "small"
    10, 20, 30:
        description = "medium"
    _:
        description = "large"
}
assert description == "medium"

// 4. Match with range
score := 85
grade := ""
match score {
    90..=100:
        grade = "A"
    80..=89:
        grade = "B"
    70..=79:
        grade = "C"
    _:
        grade = "F"
}
assert grade == "B"

// 5. Match with guard
x := 15
category := ""
match x {
    n if n < 10:
        category = "low"
    n if n >= 10 and n < 20:
        category = "medium"
    _:
        category = "high"
}
assert category == "medium"

// 6. Match on enum
enum Color {
    Red
    Green
    Blue
}
color := Color.Red
color_name := ""
match color {
    Color.Red:
        color_name = "red"
    Color.Green:
        color_name = "green"
    Color.Blue:
        color_name = "blue"
}
assert color_name == "red"

// 7. Match on list length
items := [1, 2, 3]
size_desc := ""
match len(items) {
    0:
        size_desc = "empty"
    1:
        size_desc = "single"
    2..=3:
        size_desc = "small"
    _:
        size_desc = "large"
}
assert size_desc == "small"

// 8. Match on boolean
flag := true
bool_str := ""
match flag {
    true:
        bool_str = "yes"
    false:
        bool_str = "no"
}
assert bool_str == "yes"

// 9. Nested match
outer := 5
inner := 10
combined := ""
match outer {
    5:
        match inner {
            10:
                combined = "5-10"
            _:
                combined = "5-other"
        }
    _:
        combined = "other"
}
assert combined == "5-10"

// 10. Match with default case
unknown := 999
default_result := ""
match unknown {
    1, 2, 3:
        default_result = "known"
    _:
        default_result = "default"
}
assert default_result == "default"

println("✓ test_44_match_expressions passed")
