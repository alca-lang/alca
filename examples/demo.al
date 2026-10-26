// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

// ==========================================
// Alca Programming Language — Full Showcase
// ==========================================

// 1. Static typing & dynamic typing
x: string = "Alca Language v2.0";
y := 42;
a: int | null = null;
data: int | string = "Dynamic text";

println(x);
println("Answer: " + string(y));
println("Union value: " + string(data));
println("Null value: " + string(a));

// 2. Structs
struct Server:
    ip: string;
    port: int = 80;
    is_online: bool = true;

srv := Server("127.0.0.1", 8080, true);
println("Server configured at " + srv.ip + ":" + string(srv.port));

// 3. Functions
func multiply(num: int) -> int:
    return num * 2;

println("multiply(21) = " + string(multiply(21)));

// 4. Streams: Append (>>), Overwrite (>!), and capture (>>>)
"Log event: system init\n" >! "./demo.log";
"Log event: node verified\n" >> "./demo.log";

captured: string | null = null;
print("Captured stream text") >>> captured;
if captured != null:
    println("\nCaptured value was: " + captured);

// 5. Loops (for, while, loop)
items: list[string] = ["alpha", "beta", "gamma"];
for it in items:
    println("Item: " + it);

// 6. Instant memory management (null assignment)
buffer: string | null = "Temporary payload";
println("Allocated buffer: " + string(buffer));
buffer = null;
if buffer === null:
    println("Freed buffer instantaneously: buffer is null");

// 7. Error handling
func safe_divide(a: int, b: int) -> int:
    if b === 0:
        err("division by zero forbidden", code=1);
        return 0;
    return a / b;

res := safe_divide(10, 2);
println("10 / 2 = " + string(res));

err_result: error | null = null;
safe_divide(10, 0) ?>>> err_result;
if err_result != null:
    println("Handled error gracefully: " + err_result.msg);

println("Demo complete!");
