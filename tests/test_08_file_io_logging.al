// Copyright 2026 Alca Programming Language
// Everyday Test 08: File I/O, appending log entries, reading content, and path manipulation.

use std.io;
use std.path;

// 1. Path operations
dir_name := "temp_logs";
file_name := "app_test.log";
full_path := path.join(dir_name, file_name);

assert path.base(full_path) == file_name;
assert path.ext(full_path) == ".log";

// 2. Writing & Appending to a file
test_file := "./daily_io_test.txt";

// Write initial header
io.write_file(test_file, "[INFO] Application started\n");
assert io.file_exists(test_file);

// Append logs
io.append_file(test_file, "[INFO] Connecting to database...\n");
io.append_file(test_file, "[SUCCESS] Connected successfully\n");

// 3. Reading and verifying content
content := io.read_file(test_file);
assert len(content) > 0;

// Verify file size is positive
size := io.file_size(test_file);
assert size > 20;

// 4. Cleanup
io.delete_file(test_file);
assert !io.file_exists(test_file);

println("✓ test_08_file_io_logging passed");
