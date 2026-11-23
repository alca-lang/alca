// Copyright 2026 Pawl Programming Language
// Test 46: Custom errors - creating and handling custom error types.

// 1. Basic error creation
func validate_age(age: int) -> int {
    if age < 0 {
        err("Age cannot be negative", code=400)
        return 0
    }
    if age > 150 {
        err("Age is unrealistic", code=400)
        return 0
    }
    return age
}
valid_age := validate_age(25)
assert valid_age == 25

// 2. Error capture with ?>>>
err_slot: error | null = null
invalid_age := validate_age(-5) ?>>> err_slot
assert err_slot != null
if err_slot != null {
    assert err_slot.code == 400

// 3. Custom error with context
}
func divide_with_check(a: int, b: int) -> int {
    if b === 0 {
        err("Division by zero: " + string(a) + " / 0", code=500)
        return 0
    }
    return a / b
}
result := divide_with_check(10, 2)
assert result == 5

// 4. Error wrapping simulation
func safe_parse_int(s: string) -> int {
    try {
        return int(s)
    }
    catch {
        err("Invalid integer: " + s, code=422)
        return 0
    }
}
parsed := safe_parse_int("42")
assert parsed == 42

// 5. Error with custom message
func check_permission(user: string, resource: string) -> bool {
    if user == "guest" {
        err("User '" + user + "' cannot access '" + resource + "'", code=403)
        return false
    }
    return true
}
allowed := check_permission("admin", "data")
assert allowed

// 6. Multiple error types
enum ErrorType {
    ValidationError
    AuthError
    SystemError
}
func classify_error(code: int) -> ErrorType {
    if code == 400 {
        return ErrorType.ValidationError
    }
    if code == 403 {
        return ErrorType.AuthError
    }
    return ErrorType.SystemError
}
err_type := classify_error(403)
assert err_type == ErrorType.AuthError

// 7. Error recovery
func safe_operation() -> int {
    err_slot: error | null = null
    result := validate_age(-1) ?>>> err_slot
    if err_slot != null {
        return 0 // Default value on error
    }
    return result
}
recovered := safe_operation()
assert recovered == 0

// 8. Error aggregation
errors: list[error] = []

func collect_errors() {
    err_slot: error | null = null
    validate_age(-1) ?>>> err_slot
    if err_slot != null {
        errors = errors + [err_slot]
    }
}
collect_errors()
assert len(errors) > 0

// 9. Error message extraction
func get_error_message(e: error) -> string {
    return e.msg
}
test_err_slot: error | null = null
validate_age(-5) ?>>> test_err_slot
if test_err_slot != null {
    msg := get_error_message(test_err_slot)
    assert len(msg) > 0

// 10. Error code extraction
}
func get_error_code(e: error) -> int {
    return e.code
}
test_err_slot2: error | null = null
validate_age(-5) ?>>> test_err_slot2
if test_err_slot2 != null {
    code := get_error_code(test_err_slot2)
    assert code == 400
}
println("✓ test_46_custom_errors passed")
