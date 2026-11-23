// Copyright 2026 Pawl Programming Language
// Test 42: Basic enums - enum variants, constructors, and usage.

// 1. Simple enum definition
enum Color {
    Red
    Green
    Blue
}
color := Color.Red
assert color == Color.Red

// 2. Enum with values
enum Status {
    Pending
    Active
    Completed
    Failed
}
status := Status.Active
assert status == Status.Active

// 3. Enum comparison
enum Direction {
    North
    South
    East
    West
}
dir1 := Direction.North
dir2 := Direction.North
dir3 := Direction.South

assert dir1 == dir2
assert dir1 != dir3

// 4. Enum in conditional
enum Result {
    Success
    Error
}
result := Result.Success
if result == Result.Success {
    assert true

// 5. Enum with match (if supported)
}
enum Option {
    Some
    None
}
option := Option.Some
match option {
    Option.Some:
        assert true
    Option.None:
        assert false

// 6. Enum as function parameter
}
func get_status_name(s: Status) -> string {
    if s == Status.Pending {
        return "Pending"
    }
    if s == Status.Active {
        return "Active"
    }
    if s == Status.Completed {
        return "Completed"
    }
    return "Failed"
}
assert get_status_name(Status.Active) == "Active"

// 7. Enum in list
colors: list[Color] = [Color.Red, Color.Green, Color.Blue]
assert len(colors) == 3
assert colors[0] == Color.Red

// 8. Enum in map
status_counts: map[Status, int] = {}
status_counts[Status.Pending] = 1
status_counts[Status.Active] = 2
assert status_counts[Status.Pending] == 1

// 9. Enum with associated data (simulation)
enum Event {
    Click
    KeyPress
    Load
}
event := Event.Click
assert event == Event.Click

// 10. Enum as struct field
struct Button {
    label: string
    state: Status = Status.Pending
}
button := Button("Submit", Status.Active)
assert button.state == Status.Active

// 11. Enum conversion to string
func enum_to_string(c: Color) -> string {
    if c == Color.Red {
        return "Red"
    }
    if c == Color.Green {
        return "Green"
    }
    return "Blue"
}
assert enum_to_string(Color.Red) == "Red"

// 12. String to enum conversion
func string_to_color(s: string) -> Color {
    if s == "Red" {
        return Color.Red
    }
    if s == "Green" {
        return Color.Green
    }
    return Color.Blue
}
assert string_to_color("Red") == Color.Red

// 13. Enum default values
enum LogLevel {
    Debug
    Info
    Warn
    Error
}
log_level := LogLevel.Info
assert log_level == LogLevel.Info

// 14. Enum in loop
enum Weekday {
    Monday
    Tuesday
    Wednesday
    Thursday
    Friday
    Saturday
    Sunday
}
days: list[Weekday] = []
days = days + [Weekday.Monday]
days = days + [Weekday.Tuesday]
assert len(days) == 2

// 15. Enum equality check
enum Priority {
    Low
    Medium
    High
}
priority := Priority.High
if priority == Priority.High {
    assert true
}
println("✓ test_42_basic_enums passed")
