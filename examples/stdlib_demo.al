// ==========================================
// Alca Extended Standard Library Showcase
// std.time, std.path, std.crypto
// ==========================================

use std.time;
use std.path;
use std.crypto;

println("=== Alca Extended Standard Library ===");

// 1. Time operations
current_ts := time.now();
formatted := time.now_str();
date_info := time.date();

println("Current Timestamp: " + string(current_ts));
println("Formatted Time: " + formatted);
println("Year: " + string(date_info["year"]) + ", Weekday: " + string(date_info["weekday"]));

// 2. Path operations
full_path := path.join("projects", "alca", "main.al");
clean_path := path.clean(full_path);
filename := path.base(clean_path);
directory := path.dir(clean_path);
file_ext := path.ext(clean_path);
file_stem := path.stem(clean_path);

println("\n--- Path Module ---");
println("Joined Path: " + clean_path);
println("Directory: " + directory);
println("Filename: " + filename);
println("Stem (no ext): " + file_stem);
println("Extension: " + file_ext);

// 3. Cryptography & Hashing
hash256 := crypto.sha256("Alca Language Production 2026");
hash_md5 := crypto.md5("Alca Language Production 2026");
b64 := crypto.base64_encode("Admin:SecretToken");
decoded := crypto.base64_decode(b64);
session_id := crypto.uuid();
rand_token := crypto.random_string(32);

println("\n--- Crypto Module ---");
println("SHA-256: " + hash256);
println("MD5: " + hash_md5);
println("Base64 Encoded: " + b64);
println("Base64 Decoded: " + decoded);
println("UUID v4: " + session_id);
println("Secure Token: " + rand_token);
println("Session ID length: " + string(len(session_id)));
println("Token length: " + string(len(rand_token)));

println("\n=== All Standard Modules Verified Successfully! ===");
