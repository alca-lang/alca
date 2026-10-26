// Copyright 2026 Alca Programming Language
// Everyday Test 07: Error handling, stream capture (?>>>), validation, and safe defaults.

func parse_port(port_str: string) -> int:
    val := int(port_str);
    if val < 1 or val > 65535:
        err("port out of range", code=422);
        return 0;
    return val;

func safe_divide(a: int, b: int) -> int:
    if b === 0:
        err("division by zero", code=400);
        return 0;
    return a / b;

// 1. Successful execution
p1 := parse_port("8080");
assert p1 == 8080;

// 2. Capturing an out-of-range error with ?>>>
capture1: error | null = null;
p2 := parse_port("999999") ?>>> capture1;
assert capture1 != null;
if capture1 != null:
    assert capture1.code == 422;
    assert capture1.msg == "port out of range";

// 3. Math Error Capture (?>>>)
div_err: error | null = null;
res := safe_divide(100, 0) ?>>> div_err;
assert div_err != null;
if div_err != null:
    assert div_err.code == 400;

// 4. Safe fallback with default pattern
func get_port_with_default(raw: string, default_port: int) -> int:
    err_slot: error | null = null;
    parsed := parse_port(raw) ?>>> err_slot;
    if err_slot != null:
        return default_port;
    return parsed;

assert get_port_with_default("3000", 80) == 3000;
assert get_port_with_default("70000", 80) == 80;

println("✓ test_07_error_handling_resilience passed");
