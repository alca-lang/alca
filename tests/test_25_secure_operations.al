// Copyright 2026 Pawl Programming Language
// Test 25: Secure operations - constant-time compare, secure erase, memory safety.

use std.crypto

// 1. Constant-time comparison (prevents timing attacks)
data1 := "secret_data_123"
data2 := "secret_data_123"
data3 := "different_data"

hash1 := crypto.sha256(data1)
hash2 := crypto.sha256(data2)
hash3 := crypto.sha256(data3)

assert crypto.constant_time_equal(hash1, hash2)
assert !crypto.constant_time_equal(hash1, hash3)

// 2. Compare different length strings
short := "abc"
long := "abcdef"
assert !crypto.constant_time_equal(short, long)

// 3. Compare empty strings
empty1 := ""
empty2 := ""
assert crypto.constant_time_equal(empty1, empty2)

// 4. Secure erase simulation (overwrite sensitive data)
func secure_erase(data: string) -> string {
    // In real implementation, would zero out memory
    // For simulation, return empty string
    return ""
}
sensitive := "password123"
erased := secure_erase(sensitive)
assert erased == ""

// 5. Secure string handling
struct SecureString {
    data: string
    is_active: bool = true
}
func create_secure_string(value: string) -> SecureString {
    return SecureString(value, true)
}
func erase_secure_string(s: SecureString) -> SecureString {
    s.data = ""
    s.is_active = false
    return s
}
secure_pass := create_secure_string("my_password")
assert secure_pass.is_active

erased_pass := erase_secure_string(secure_pass)
assert !erased_pass.is_active
assert erased_pass.data == ""

// 6. Secure random generation for cryptographic purposes
func secure_random_bytes(length: int) -> string {
    // Use crypto.random_string for cryptographically secure random
    chars := "0123456789abcdef"
    return crypto.random_string(length, chars)
}
secure_bytes := secure_random_bytes(32)
assert len(secure_bytes) == 32

// 7. Secure key generation
func generate_secure_key(length: int) -> string {
    return secure_random_bytes(length)
}
secure_key := generate_secure_key(32)
assert len(secure_key) == 32

// 8. Secure password hashing (bcrypt simulation)
func hash_password(password: string, salt: string) -> string {
    // Simulate bcrypt with multiple rounds
    hash := password + salt
    i := 0
    while i < 10000 { // Multiple rounds for security
        hash = crypto.sha256(hash)
        i = i + 1
    }
    return hash
}
func verify_password(password: string, salt: string, hash: string) -> bool {
    computed := hash_password(password, salt)
    return crypto.constant_time_equal(computed, hash)
}
password_salt := crypto.random_string(16, "0123456789abcdef")
password_hash := hash_password("user_password", password_salt)
assert verify_password("user_password", password_salt, password_hash)
assert !verify_password("wrong_password", password_salt, password_hash)

// 9. Salt generation
func generate_salt(length: int) -> string {
    return crypto.random_string(length, "0123456789abcdef")
}
salt1 := generate_salt(16)
salt2 := generate_salt(16)
assert salt1 != salt2 // Salts should be unique

// 10. Pepper (server-side secret) usage
pepper := "server_secret_pepper"
peppered_hash := hash_password("password", password_salt + pepper)
assert len(peppered_hash) == 64

// 11. Memory zeroing simulation
func zero_memory(size: int) -> string {
    result := ""
    i := 0
    while i < size {
        result = result + "\x00"
        i = i + 1
    }
    return result
}
zeroed := zero_memory(10)
assert len(zeroed) == 10

// 12. Secure buffer handling
struct SecureBuffer {
    data: string
    capacity: int
}
func create_secure_buffer(capacity: int) -> SecureBuffer {
    return SecureBuffer("", capacity)
}
func write_secure_buffer(buf: SecureBuffer, data: string) -> SecureBuffer {
    if len(data) <= buf.capacity {
        buf.data = data
    }
    return buf
}
func clear_secure_buffer(buf: SecureBuffer) -> SecureBuffer {
    buf.data = ""
    return buf
}
secure_buf := create_secure_buffer(100)
secure_buf = write_secure_buffer(secure_buf, "sensitive data")
assert secure_buf.data == "sensitive data"

secure_buf = clear_secure_buffer(secure_buf)
assert secure_buf.data == ""

// 13. Safe string comparison (prevent timing attacks on passwords)
func safe_compare_password(input: string, stored: string) -> bool {
    return crypto.constant_time_equal(crypto.sha256(input), stored)
}
stored_hash := crypto.sha256("correct_password")
assert safe_compare_password("correct_password", stored_hash)
assert !safe_compare_password("wrong_password", stored_hash)

// 14. Secure token generation
func generate_secure_token() -> string {
    return crypto.random_string(32, "0123456789abcdef")
}
token1 := generate_secure_token()
token2 := generate_secure_token()
assert len(token1) == 32
assert token1 != token2

// 15. Token validation
func validate_token(token: string, expected: string) -> bool {
    return crypto.constant_time_equal(token, expected)
}
valid_token := generate_secure_token()
assert validate_token(valid_token, valid_token)
assert !validate_token(valid_token, "wrong_token")

// 16. Secure session ID generation
func generate_session_id() -> string {
    return crypto.uuid()
}
session_id := generate_session_id()
assert len(session_id) == 36

// 17. CSRF token generation
func generate_csrf_token() -> string {
    random_part := crypto.random_string(16, "0123456789abcdef")
    timestamp := string(int(time.now()))
    return crypto.sha256(random_part + timestamp)
}
csrf_token := generate_csrf_token()
assert len(csrf_token) == 64

// 18. Nonce generation (for replay attack prevention)
func generate_nonce() -> string {
    return crypto.random_string(16, "0123456789abcdef")
}
nonce1 := generate_nonce()
nonce2 := generate_nonce()
assert nonce1 != nonce2

// 19. Timestamp validation
func is_timestamp_valid(timestamp: int, max_age_seconds: int) -> bool {
    current := int(time.now())
    return (current - timestamp) <= max_age_seconds
}
recent_ts := int(time.now())
old_ts := recent_ts - 1000

assert is_timestamp_valid(recent_ts, 60)
assert !is_timestamp_valid(old_ts, 60)

// 20. Rate limiting token bucket
struct TokenBucket {
    tokens: int
    max_tokens: int
    last_refill: int
}
func create_token_bucket(max: int) -> TokenBucket {
    return TokenBucket(max, max, int(time.now()))
}
func consume_token(bucket: TokenBucket, tokens: int) -> TokenBucket {
    current := int(time.now())
    elapsed := current - bucket.last_refill
    // Refill 1 token per second
    refill := elapsed
    bucket.tokens = bucket.tokens + refill
    if bucket.tokens > bucket.max_tokens {
        bucket.tokens = bucket.max_tokens
    }
    bucket.last_refill = current
    if bucket.tokens >= tokens {
        bucket.tokens = bucket.tokens - tokens
    }
    return bucket
}
bucket := create_token_bucket(10)
bucket = consume_token(bucket, 5)
assert bucket.tokens == 5

bucket = consume_token(bucket, 3)
assert bucket.tokens == 2

println("✓ test_25_secure_operations passed")
