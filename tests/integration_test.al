// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

use std.libtorch;
func bad():
    println(libtorch.matmul([[1]], [[1, 2]]));
    println(libtorch.matmul([[1, 2]], [[1, 2]]));
bad();
