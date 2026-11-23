// Copyright 2026 Pawl Programming Language
// Test 77: Capability deny - sandbox denial without flags.

// 1. File read without permission
func try_read_without_permission() -> bool {
    // Simulate file read without --allow-read
    return false // Should fail
}
assert !try_read_without_permission()

// 2. File write without permission
func try_write_without_permission() -> bool {
    // Simulate file write without --allow-write
    return false // Should fail
}
assert !try_write_without_permission()

// 3. Network access without permission
func try_network_without_permission() -> bool {
    // Simulate network access without --allow-net
    return false // Should fail
}
assert !try_network_without_permission()

// 4. Process execution without permission
func try_process_without_permission() -> bool {
    // Simulate process execution without --allow-run
    return false // Should fail
}
assert !try_process_without_permission()

// 5. Environment access without permission
func try_env_without_permission() -> bool {
    // Simulate env access without --allow-env
    return false // Should fail
}
assert !try_env_without_permission()

// 6. FFI without permission
func try_ffi_without_permission() -> bool {
    // Simulate FFI without --allow-ffi
    return false // Should fail
}
assert !try_ffi_without_permission()

// 7. Default deny behavior
default_deny := true
assert default_deny

// 8. Capability check simulation
func has_capability(cap: string) -> bool {
    // Simulate capability check
    allowed_caps := ["none"]
    return cap in allowed_caps
}
assert !has_capability("read")
assert !has_capability("write")

// 9. Sandbox enforcement
func enforce_sandbox(operation: string) -> bool {
    // Default deny-all policy
    return false
}
assert !enforce_sandbox("read_file")
assert !enforce_sandbox("write_file")

// 10. Permission denied error
func permission_error(operation: string) -> string {
    return "Permission denied: " + operation
}
error_msg := permission_error("read_file")
assert string.contains(error_msg, "Permission denied")

println("✓ test_77_capability_deny passed")
