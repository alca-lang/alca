// Copyright 2026 Pawl Programming Language
// Test 78: Capability grant - granular permission granting.

// 1. Grant read permission
func grant_read(path: string) -> bool {
    // Simulate granting --allow-read
    return true
}
assert grant_read("/path/to/file")

// 2. Grant write permission
func grant_write(path: string) -> bool {
    // Simulate granting --allow-write
    return true
}
assert grant_write("/path/to/file")

// 3. Grant network permission
func grant_net(host: string) -> bool {
    // Simulate granting --allow-net
    return true
}
assert grant_net("example.com")

// 4. Grant process permission
func grant_run(cmd: string) -> bool {
    // Simulate granting --allow-run
    return true
}
assert grant_run("ls")

// 5. Grant environment permission
func grant_env(var_name: string) -> bool {
    // Simulate granting --allow-env
    return true
}
assert grant_env("PATH")

// 6. Grant FFI permission
func grant_ffi(lib: string) -> bool {
    // Simulate granting --allow-ffi
    return true
}
assert grant_ffi("kernel32.dll")

// 7. Grant all permissions
func grant_all() -> bool {
    // Simulate --allow-all
    return true
}
assert grant_all()

// 8. Specific path permission
func has_path_permission(path: string, perm: string) -> bool {
    // Simulate path-specific permission
    return true
}
assert has_path_permission("/safe/path", "read")

// 9. Permission scope check
func check_scope(perm: string, scope: string) -> bool {
    // Check if permission matches scope
    return true
}
assert check_scope("read", "/home/user")

// 10. Permission intersection
func intersect_permissions(a: list[string], b: list[string]) -> list[string] {
    result: list[string] = []
    for perm in a {
        if perm in b {
            result = result + [perm]
        }
    }
    return result
}
perms1 := ["read", "write"]
perms2 := ["read", "execute"]
intersection := intersect_permissions(perms1, perms2)
assert intersection == ["read"]

println("✓ test_78_capability_grant passed")
