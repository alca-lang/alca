// Copyright 2026 Pawl Programming Language
// Test 43: Enum methods - methods defined on enum types.

// 1. Enum with method simulation
enum Color {
    Red
    Green
    Blue
}
func color_to_hex(c: Color) -> string {
    if c == Color.Red {
        return "#FF0000"
    }
    if c == Color.Green {
        return "#00FF00"
    }
    return "#0000FF"
}
assert color_to_hex(Color.Red) == "#FF0000"

// 2. Enum with boolean methods
enum Status {
    Pending
    Active
    Completed
    Failed
}
func is_terminal(s: Status) -> bool {
    return s == Status.Completed or s == Status.Failed
}
assert !is_terminal(Status.Active)
assert is_terminal(Status.Completed)

// 3. Enum with comparison methods
enum Priority {
    Low
    Medium
    High
}
func is_higher(p1: Priority, p2: Priority) -> bool {
    if p1 == Priority.High and p2 != Priority.High {
        return true
    }
    if p1 == Priority.Medium and p2 == Priority.Low {
        return true
    }
    return false
}
assert is_higher(Priority.High, Priority.Low)
assert !is_higher(Priority.Low, Priority.High)

// 4. Enum with transformation
enum Temperature {
    Celsius
    Fahrenheit
}
func convert_temp(value: float, from: Temperature, to: Temperature) -> float {
    if from == to {
        return value
    }
    if from == Temperature.Celsius and to == Temperature.Fahrenheit {
        return value * 9.0 / 5.0 + 32.0
    }
    if from == Temperature.Fahrenheit and to == Temperature.Celsius {
        return (value - 32.0) * 5.0 / 9.0
    }
    return value
}
celsius := 0.0
fahrenheit := convert_temp(celsius, Temperature.Celsius, Temperature.Fahrenheit)
assert fahrenheit == 32.0

// 5. Enum with validation
enum Permission {
    Read
    Write
    Execute
    Admin
}
func has_permission(user_perm: Permission, required: Permission) -> bool {
    if user_perm == Permission.Admin {
        return true
    }
    return user_perm == required
}
assert has_permission(Permission.Admin, Permission.Write)
assert !has_permission(Permission.Read, Permission.Write)

// 6. Enum with aggregation
enum Day {
    Monday
    Tuesday
    Wednesday
    Thursday
    Friday
    Saturday
    Sunday
}
func is_weekend(d: Day) -> bool {
    return d == Day.Saturday or d == Day.Sunday
}
assert is_weekend(Day.Saturday)
assert !is_weekend(Day.Monday)

// 7. Enum with cycling
func next_day(d: Day) -> Day {
    if d == Day.Monday {
        return Day.Tuesday
    }
    if d == Day.Tuesday {
        return Day.Wednesday
    }
    if d == Day.Wednesday {
        return Day.Thursday
    }
    if d == Day.Thursday {
        return Day.Friday
    }
    if d == Day.Friday {
        return Day.Saturday
    }
    if d == Day.Saturday {
        return Day.Sunday
    }
    return Day.Monday
}
assert next_day(Day.Friday) == Day.Saturday
assert next_day(Day.Sunday) == Day.Monday

// 8. Enum with list operations
func all_colors() -> list[Color] {
    return [Color.Red, Color.Green, Color.Blue]
}
colors := all_colors()
assert len(colors) == 3

// 9. Enum with filtering
func active_statuses() -> list[Status] {
    return [Status.Active, Status.Pending]
}
active := active_statuses()
assert len(active) == 2

// 10. Enum with counting
func count_colors() -> int {
    return 3
}
assert count_colors() == 3

println("✓ test_43_enum_methods passed")
