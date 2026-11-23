// Copyright 2026 Pawl Programming Language
// Test 50: Process execution - exec, stdout/stderr capture simulation.

// Note: These tests simulate process execution
// Actual execution requires --allow-run flag

// 1. Command construction
func build_command(program: string, args: list[string]) -> string {
    result := program
    for arg in args {
        result = result + " " + arg
    }
    return result
}
cmd := build_command("ls", ["-la", "/home"])
assert string.contains(cmd, "ls")
assert string.contains(cmd, "-la")

// 2. Command validation
func is_safe_command(cmd: string) -> bool {
    dangerous := ["rm", "del", "format", "shutdown"]
    for d in dangerous {
        if string.contains(cmd, d) {
            return false
        }
    }
    return true
}
assert is_safe_command("ls -la")
assert !is_safe_command("rm -rf /")

// 3. Argument parsing simulation
func parse_args(args: list[string]) -> map[string, string] {
    result: map[string, string] = {}
    i := 0
    while i < len(args) {
        if string.starts_with(args[i], "--") {
            key := string.substring(args[i], 2, len(args[i]))
            if i + 1 < len(args) and !string.starts_with(args[i + 1], "--") {
                result[key] = args[i + 1]
                i = i + 1
            }
        }
        i = i + 1
    }
    return result
}
parsed := parse_args(["--output", "file.txt", "--verbose"])
assert parsed["output"] == "file.txt"
assert parsed["verbose"] == ""

// 4. Environment variable simulation
env_vars: map[string, string] = {"PATH": "/usr/bin", "HOME": "/home/user"}

func get_env(key: string) -> string {
    if key in env_vars {
        return env_vars[key]
    }
    return ""
}
assert get_env("PATH") == "/usr/bin"
assert get_env("NONEXISTENT") == ""

// 5. Process status simulation
enum ProcessStatus {
    Running
    Completed
    Failed
    Killed
}
status := ProcessStatus.Running
assert status == ProcessStatus.Running

// 6. Exit code validation
func is_success_exit(code: int) -> bool {
    return code == 0
}
assert is_success_exit(0)
assert !is_success_exit(1)

// 7. Command timeout simulation
func should_timeout(duration_ms: int, timeout_ms: int) -> bool {
    return duration_ms > timeout_ms
}
assert !should_timeout(1000, 5000)
assert should_timeout(10000, 5000)

// 8. Process ID generation
func generate_pid() -> int {
    return int(time.now()) % 10000
}
pid := generate_pid()
assert pid > 0 and pid < 10000

// 9. Working directory handling
func resolve_path(path: string, base: string) -> string {
    if string.starts_with(path, "/") {
        return path
    }
    return base + "/" + path
}
resolved := resolve_path("subdir", "/home/user")
assert resolved == "/home/user/subdir"

// 10. Output capture simulation
struct ProcessOutput {
    stdout: string = ""
    stderr: string = ""
    exit_code: int = 0
}
output := ProcessOutput("Hello World", "", 0)
assert output.stdout == "Hello World"
assert output.exit_code == 0

println("✓ test_50_process_execution passed")
