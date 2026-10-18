// ==========================================
// Alca Programming Language — Full Showcase
// ==========================================

// 1. Static typing & dynamic typing
x: string = "Alca Language v1.0"
y = 42
a = null
data: int | string = "Dynamic text"

print(x)
print("Answer: " + y)

// 2. Structs
struct Server:
    ip: string
    port: int = 80
    is_online: bool = true

srv = Server("127.0.0.1", 8080, true)
print("Server configured at " + srv.ip + ":" + srv.port)

// 3. Functions
func multiply(num: int) -> int:
    return num * 2

print("multiply(21) = " + multiply(21))

// 4. Streams: Append (>>), Overwrite (>!), and capture (>>>)
"Log event: system init\n" >! "./demo.log"
"Log event: node verified\n" >> "./demo.log"

captured = null
print("Captured stream text") >>> captured
print("Captured value was: " + captured)

// 5. Loops (for, while, loop)
items = ["alpha", "beta", "gamma"]
for it in items:
    print("Item: " + it)

// 6. Instant memory management (null assignment)
buffer: string = "Temporary payload"
print("Allocated buffer: " + buffer)
buffer = null
print("Freed buffer instantaneously: buffer is null")

// 7. Error handling
func safe_divide(a: int, b: int):
    if b == 0:
        err("division by zero forbidden", code=1)
        return null
    return a / b

res = safe_divide(10, 2)
print("10 / 2 = " + res)

err_result = null
safe_divide(10, 0) ?>>> err_result
if err_result != null:
    print("Handled error gracefully: " + err_result.msg)

print("Demo complete!")
