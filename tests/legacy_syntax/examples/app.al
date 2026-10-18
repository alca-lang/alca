// ==========================================
// Production Alca Application
// ==========================================

print("=== Starting Alca Production App ===")

// 1. Static typing and auto-inference
msg: string = "Hello from Alca Native Executable!"
count = 3
print(msg)

// 2. Structs
struct Server:
    ip: string
    port: int = 80
    is_online: bool = true

srv = Server("192.168.1.100", 8080, true)
print("Server IP: " + srv.ip)

// 3. Functions
func add(a: int, b: int) -> int:
    return a + b

total = add(15, 27)
print("Calculation: 15 + 27 = " + total)

// 4. Collections and loops
servers = ["web1", "web2", "database"]
for s in servers:
    print("Checking node: " + s)

// 5. Streams and file operations
"Audit log entry: Server operational\n" >> "./audit.log"
print("Logged operational event to ./audit.log via stream operator '>>'")

// 6. Memory release test
buf: string = "High memory temporary buffer"
print("Buffer allocated: " + buf)
buf = null
print("Buffer instantly freed via null assignment: OK")

print("=== Alca Application Finished Successfully ===")
