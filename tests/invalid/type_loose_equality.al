// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

// Test for loose equality bug
// "1" == 1 should not give true for non-json types

if "1" == 1:
    println("This should not compile");
