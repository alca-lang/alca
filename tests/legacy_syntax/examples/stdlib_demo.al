// ==========================================
// Alca Extended Standard Library Showcase
// std.time, std.path, std.crypto
// ==========================================

use std.time
use std.path
use std.crypto

print("=== Alca Extended Standard Library ===")

// 1. Time operations
current_ts = time.now()
formatted = time.now_str("YYYY-MM-DD HH:mm:ss")
date_info = time.date(current_ts)

print("Current Timestamp: " + string(current_ts))
print("Formatted Time: " + formatted)
print("Year: " + string(date_info.year) + ", Weekday: " + date_info.weekday)

// 2. Path operations
full_path = path.join("projects", "alca", "main.al")
clean_path = path.clean(full_path)
filename = path.base(clean_path)
directory = path.dir(clean_path)
file_ext = path.ext(clean_path)
file_stem = path.stem(clean_path)

print("\n--- Path Module ---")
print("Joined Path: " + clean_path)
print("Directory: " + directory)
print("Filename: " + filename)
print("Stem (no ext): " + file_stem)
print("Extension: " + file_ext)

// 3. Cryptography & Hashing
hash256 = crypto.sha256("Alca Language Production 2026")
hash_md5 = crypto.md5("Alca Language Production 2026")
b64 = crypto.base64_encode("Admin:SecretToken")
decoded = crypto.base64_decode(b64)
session_id = crypto.uuid()
rand_token = crypto.random_string(32)

print("\n--- Crypto Module ---")
print("SHA-256: " + hash256)
print("MD5: " + hash_md5)
print("Base64 Encoded: " + b64)
print("Base64 Decoded: " + decoded)
print("UUID v4: " + session_id)
print("Secure Token: " + rand_token)

print("\n=== All Standard Modules Verified Successfully! ===")
