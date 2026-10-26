// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

// Negative fixture: passing a wrong argument type.
func add(a: int, b: int) -> int:
    return a + b;

println(add(10, "wrong argument"));
