// ==========================================
// Production Alca Application
// ==========================================

println("=== Starting Alca Production App ===");

// 1. Static typing and auto-inference
msg: string = "Hello from Alca Native Executable!";
count := 3;
println(msg);
println("counter = " + string(count));

// 2. Structs
struct Server:
    ip: string;
    port: int = 80;
    is_online: bool = true;

srv := Server("192.168.1.100", 8080, true);
println("Server IP: " + srv.ip);
println("Server port: " + string(srv.port));

// 3. Functions
func add(a: int, b: int) -> int:
    return a + b;

total := add(15, 27);
println("Calculation: 15 + 27 = " + string(total));

// 4. Collections and loops
servers: list[string] = ["web1", "web2", "database"];
for s in servers:
    println("Checking node: " + s);

// 5. Streams and file operations
"Audit log entry: Server operational\n" >> "./audit.log";
println("Logged operational event to ./audit.log via stream operator '>>'");

// 6. Memory release test
buf: string | null = "High memory temporary buffer";
println("Buffer allocated: " + string(buf));
buf = null;
if buf == null:
    println("Buffer instantly freed via null assignment: OK");

println("=== Alca Application Finished Successfully ===");
