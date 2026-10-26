// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

// Negative fixture: a function returning the wrong type.
func add(a: int, b: int) -> int:
    return "invalid return type";

println(add(1, 2));
