// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

// Test for shadow variable assignment bug
// This should be a compile error: nested declaration should not silently mutate outer variable

count: int = 2;
if true:
    count := 3;  // This should be an error: 'count' is already declared
