// Copyright 2026 Alca Programming Language
// Everyday Test 09: Cryptographic hashing, token generation, HMAC signatures, and Base64 encoding.

use std.crypto;

// 1. SHA-256 Hashing
msg := "hello alca security";
hash1 := crypto.sha256(msg);
hash2 := crypto.sha256(msg);

assert hash1 == hash2;
assert len(hash1) == 64; // 64 hex characters for SHA-256
assert hash1 != crypto.sha256("different message");

// 2. HMAC-SHA256 Signatures (API Authentication simulation)
secret_key := "super_secret_jwt_api_signing_key";
payload := "{\"user_id\": 42, \"role\": \"admin\"}";
signature := crypto.hmac_sha256(secret_key, payload);
assert len(signature) == 64;

// Signature verification with constant time comparison
expected_sig := crypto.hmac_sha256(secret_key, payload);
assert crypto.constant_time_equal(signature, expected_sig);

tampered_sig := crypto.hmac_sha256("wrong_key", payload);
assert !crypto.constant_time_equal(signature, tampered_sig);

// 3. Base64 Encoding & Decoding
original_data := "Alca Secure Payload 2026!";
b64 := crypto.base64_encode(original_data);
assert len(b64) > 0;
decoded := crypto.base64_decode(b64);
assert decoded == original_data;

// Hex encoding & decoding
hex_str := crypto.hex_encode("API_TOKEN");
assert len(hex_str) == 18;
assert crypto.hex_decode(hex_str) == "API_TOKEN";

// 4. Secure Tokens & UUID generation
token := crypto.random_string(32, "abcdef0123456789");
assert len(token) == 32;

id := crypto.uuid();
assert len(id) == 36; // standard UUID length: 8-4-4-4-12

rand_num := crypto.random_int(1, 100);
assert rand_num >= 1 and rand_num <= 100;

println("✓ test_09_crypto_security passed");
