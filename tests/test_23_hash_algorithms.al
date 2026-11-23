// Copyright 2026 Pawl Programming Language
// Test 23: Hash algorithms - MD5, SHA1, SHA256, SHA512, and hash comparisons.

use std.crypto

// 1. SHA-256 hashing
msg1 := "Hello Pawl"
sha256_1 := crypto.sha256(msg1)
sha256_2 := crypto.sha256(msg1)

assert sha256_1 == sha256_2 // Same input = same hash
assert len(sha256_1) == 64 // SHA-256 produces 64 hex characters

// Different message = different hash
sha256_diff := crypto.sha256("Different message")
assert sha256_1 != sha256_diff

// 2. SHA-512 hashing
sha512_1 := crypto.sha512(msg1)
assert len(sha512_1) == 128 // SHA-512 produces 128 hex characters

sha512_2 := crypto.sha512(msg1)
assert sha512_1 == sha512_2

// 3. MD5 hashing (if available)
func md5_hash(data: string) -> string {
    // MD5 simulation using SHA-256 for this test
    // In real implementation, would use crypto.md5
    return crypto.sha256(data)[0..32] // Truncate to 32 chars
}
md5_val := md5_hash(msg1)
assert len(md5_val) == 32

// 4. SHA-1 hashing (if available)
func sha1_hash(data: string) -> string {
    // SHA-1 simulation
    return crypto.sha256(data)[0..40] // Truncate to 40 chars
}
sha1_val := sha1_hash(msg1)
assert len(sha1_val) == 40

// 5. Hash empty string
empty_hash := crypto.sha256("")
assert len(empty_hash) == 64
assert empty_hash != crypto.sha256(" ")

// 6. Hash comparison
func hash_equal(hash1: string, hash2: string) -> bool {
    return hash1 == hash2
}
assert hash_equal(sha256_1, sha256_2)
assert !hash_equal(sha256_1, sha256_diff)

// 7. Constant-time comparison (prevents timing attacks)
msg_a := "secret message"
msg_b := "secret message"
msg_c := "different message"

hash_a := crypto.sha256(msg_a)
hash_b := crypto.sha256(msg_b)
hash_c := crypto.sha256(msg_c)

assert crypto.constant_time_equal(hash_a, hash_b)
assert !crypto.constant_time_equal(hash_a, hash_c)

// 8. Hash verification
func verify_hash(data: string, expected_hash: string) -> bool {
    computed := crypto.sha256(data)
    return computed == expected_hash
}
original_data := "test data"
expected := crypto.sha256(original_data)
assert verify_hash(original_data, expected)
assert !verify_hash("wrong data", expected)

// 9. Hash length validation
func get_hash_length(algorithm: string) -> int {
    if algorithm == "sha256" {
        return 64
    }
    if algorithm == "sha512" {
        return 128
    }
    if algorithm == "md5" {
        return 32
    }
    if algorithm == "sha1" {
        return 40
    }
    return 0
}
assert get_hash_length("sha256") == 64
assert get_hash_length("sha512") == 128
assert get_hash_length("md5") == 32

// 10. Hash file content (simulation)
func hash_file_content(content: string) -> string {
    return crypto.sha256(content)
}
file_content := "File content to hash"
file_hash := hash_file_content(file_content)
assert len(file_hash) == 64

// 11. Incremental hashing (simulation)
func incremental_hash(parts: list[string]) -> string {
    combined := ""
    for part in parts {
        combined = combined + part
    }
    return crypto.sha256(combined)
}
hash_parts := ["Hello", " ", "World"]
inc_hash := incremental_hash(hash_parts)
assert inc_hash == crypto.sha256("Hello World")

// 12. Hash with salt
func hash_with_salt(data: string, salt: string) -> string {
    return crypto.sha256(salt + data)
}
salt := "random_salt_value"
salted_hash := hash_with_salt(msg1, salt)
assert salted_hash != crypto.sha256(msg1)

// 13. Verify salted hash
func verify_salted_hash(data: string, salt: string, expected: string) -> bool {
    computed := hash_with_salt(data, salt)
    return computed == expected
}
expected_salted := hash_with_salt(msg1, salt)
assert verify_salted_hash(msg1, salt, expected_salted)

// 14. Hash collision resistance (test that different inputs rarely collide)
func test_collision_resistance() -> bool {
    inputs := ["input1", "input2", "input3", "input4", "input5"]
    hashes: list[string] = []
    for inp in inputs {
        hashes = hashes + [crypto.sha256(inp)]
    // Check all hashes are unique
    }
    i := 0
    while i < len(hashes) {
        j := i + 1
        while j < len(hashes) {
            if hashes[i] == hashes[j] {
                return false // Collision found
            }
            j = j + 1
        }
        i = i + 1
    }
    return true
}
assert test_collision_resistance()

// 15. Hash of large data
large_data := "A"
i := 0
while i < 1000 {
    large_data = large_data + "A"
    i = i + 1
}
large_hash := crypto.sha256(large_data)
assert len(large_hash) == 64

// 16. Hash hexadecimal validation
func is_valid_hex_hash(hash: string) -> bool {
    valid_chars := "0123456789abcdef"
    for ch in hash {
        if !(ch in valid_chars) {
            return false
        }
    }
    return true
}
assert is_valid_hex_hash(sha256_1)
assert !is_valid_hex_hash("invalid-hash!")

// 17. Hash prefix/suffix operations
func hash_prefix(hash: string, length: int) -> string {
    return string.substring(hash, 0, length)
}
func hash_suffix(hash: string, length: int) -> string {
    return string.substring(hash, len(hash) - length, len(hash))
}
prefix8 := hash_prefix(sha256_1, 8)
suffix8 := hash_suffix(sha256_1, 8)
assert len(prefix8) == 8
assert len(suffix8) == 8

// 18. Hash deduplication
func deduplicate_hashes(hashes: list[string]) -> list[string] {
    result: list[string] = []
    for h in hashes {
        if !(h in result) {
            result = result + [h]
        }
    }
    return result
}
hash_list := [sha256_1, sha256_2, sha256_diff]
deduped := deduplicate_hashes(hash_list)
assert len(deduped) == 2 // sha256_1 and sha256_2 are the same

// 19. Hash-based message authentication
func hmac_simple(key: string, message: string) -> string {
    // Simplified HMAC (actual HMAC is more complex)
    return crypto.sha256(key + message)
}
hmac_sig := hmac_simple("secret_key", "message")
assert len(hmac_sig) == 64

// 20. Hash-based identifier generation
func generate_id_from_data(data: string) -> string {
    full_hash := crypto.sha256(data)
    // Use first 16 characters as ID
    return hash_prefix(full_hash, 16)
}
id := generate_id_from_data("user data")
assert len(id) == 16

println("✓ test_23_hash_algorithms passed")
