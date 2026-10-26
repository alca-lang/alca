// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

use std.math;

struct Server:
    ip: string;
    port: int = 80;

func area(r: float) -> float:
    return math.PI * r * r;

func safe_div(a: int, b: int) -> int:
    if b === 0:
        err("division by zero", code=1);
        return 0;
    return a / b;

srv := Server("127.0.0.1", 8080);
println("listening on " + srv.ip + ":" + string(srv.port));
println(area(2.0));

failure: error | null = null;
safe_div(10, 0) ?>>> failure;
if failure != null:
    println("handled: " + failure.msg);

total: int = 0;
// 1..=10 is inclusive [1, 10], yielding 55. (1..10 is half-open [1, 10))
for i in 1..=10:
    total = total + i;
println(total);
