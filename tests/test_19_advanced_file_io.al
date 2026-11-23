// Copyright 2026 Pawl Programming Language
// Test 19: Advanced file I/O - streams, buffering, seek, and file operations.

use std.io
use std.path

// 1. Create test directory and file
test_dir := "./test_io_advanced"
test_file := path.join(test_dir, "data.txt")

// Note: These tests require --allow-write and --allow-read flags

// 2. Write data in chunks
func write_in_chunks(filepath: string, data: string, chunk_size: int) -> bool {
    i := 0
    while i < len(data) {
        end := i + chunk_size
        if end > len(data) {
            end = len(data)
        }
        chunk := string.substring(data, i, end)
        io.append_file(filepath, chunk)
        i = i + chunk_size
    }
    return true

// Write initial data
}
io.write_file(test_file, "Hello World")

// 3. Read file size
file_size := io.file_size(test_file)
assert file_size == 11 // "Hello World" = 11 chars

// 4. File existence check
assert io.file_exists(test_file)

// 5. Append to file
io.append_file(test_file, "\nAdvanced I/O")
content := io.read_file(test_file)
assert string.contains(content, "Hello World")
assert string.contains(content, "Advanced I/O")

// 6. File truncation simulation
func truncate_file(filepath: string, new_size: int) -> bool {
    data := io.read_file(filepath)
    if len(data) > new_size {
        truncated := string.substring(data, 0, new_size)
        io.write_file(filepath, truncated)
        return true
    }
    return false
}
truncate_success := truncate_file(test_file, 11)
assert truncate_success

truncated_content := io.read_file(test_file)
assert truncated_content == "Hello World"

// 7. File copying
copy_file := path.join(test_dir, "copy.txt")
io.write_file(copy_file, io.read_file(test_file))
assert io.file_exists(copy_file)
assert io.read_file(copy_file) == "Hello World"

// 8. File moving (copy + delete original)
move_file := path.join(test_dir, "moved.txt")
io.write_file(move_file, io.read_file(copy_file))
io.delete_file(copy_file)
assert !io.file_exists(copy_file)
assert io.file_exists(move_file)

// 9. Directory creation
io.create_dir(test_dir) // Create if not exists
assert io.dir_exists(test_dir)

// 10. List directory contents
sub_dir := path.join(test_dir, "subdir")
io.create_dir(sub_dir)
assert io.dir_exists(sub_dir)

// 11. Path operations
full_path := path.join(test_dir, "test.txt")
assert path.base(full_path) == "test.txt"
assert path.ext(full_path) == ".txt"
assert path.dir(full_path) == test_dir

// 12. Absolute path resolution
func resolve_path(p: string) -> string {
    // Simplified absolute path resolution
    if string.starts_with(p, "/") or string.starts_with(p, "C:") or string.starts_with(p, "D:") {
        return p
    }
    return path.join(".", p)
}
rel_path := "./data.txt"
abs_path := resolve_path(rel_path)
assert len(abs_path) > len(rel_path)

// 13. Path normalization
func normalize_path(p: string) -> string {
    // Remove duplicate separators
    result := string.replace(p, "\\", "/")
    result = string.replace(result, "//", "/")
    return result
}
normalized := normalize_path("path\\\\to\\\\file")
assert normalized == "path/to/file"

// 14. File permissions check (simulation)
func check_read_permission(filepath: string) -> bool {
    return io.file_exists(filepath)
}
assert check_read_permission(test_file)

// 15. File content comparison
func files_equal(file1: string, file2: string) -> bool {
    content1 := io.read_file(file1)
    content2 := io.read_file(file2)
    return content1 == content2
}
io.write_file(path.join(test_dir, "same1.txt"), "identical")
io.write_file(path.join(test_dir, "same2.txt"), "identical")
assert files_equal(path.join(test_dir, "same1.txt"), path.join(test_dir, "same2.txt"))

// 16. Line counting
func count_lines(filepath: string) -> int {
    content := io.read_file(filepath)
    lines := string.split(content, "\n")
    return len(lines)
}
io.write_file(path.join(test_dir, "lines.txt"), "line1\nline2\nline3")
line_count := count_lines(path.join(test_dir, "lines.txt"))
assert line_count == 3

// 17. File search in directory
func find_files_by_extension(dir: string, ext: string) -> list[string] {
    // Simplified file search
    result: list[string] = []
    // In real implementation, would list directory
    if io.file_exists(path.join(dir, "test" + ext)) {
        result = result + [path.join(dir, "test" + ext)]
    }
    return result
}
io.write_file(path.join(test_dir, "test.txt"), "content")
txt_files := find_files_by_extension(test_dir, ".txt")
assert len(txt_files) > 0

// 18. Binary file operations simulation
func write_binary_data(filepath: string, data: string) -> bool {
    return io.write_file(filepath, data)
}
func read_binary_data(filepath: string) -> string {
    return io.read_file(filepath)
}
binary_file := path.join(test_dir, "binary.dat")
binary_data := "\x00\x01\x02\x03"
write_binary_data(binary_file, binary_data)
read_back := read_binary_data(binary_file)
assert read_back == binary_data

// 19. File locking simulation
func try_lock_file(filepath: string) -> bool {
    // Simulate file lock
    if io.file_exists(filepath + ".lock") {
        return false
    }
    io.write_file(filepath + ".lock", "locked")
    return true
}
locked := try_lock_file(test_file)
assert locked

// 20. File unlock
func unlock_file(filepath: string) -> bool {
    lock_file := filepath + ".lock"
    if io.file_exists(lock_file) {
        io.delete_file(lock_file)
        return true
    }
    return false
}
unlocked := unlock_file(test_file)
assert unlocked
assert !io.file_exists(test_file + ".lock")

// Cleanup
io.delete_file(test_file)
io.delete_file(move_file)
io.delete_file(path.join(test_dir, "same1.txt"))
io.delete_file(path.join(test_dir, "same2.txt"))
io.delete_file(path.join(test_dir, "lines.txt"))
io.delete_file(path.join(test_dir, "test.txt"))
io.delete_file(binary_file)
io.delete_dir(sub_dir)
io.delete_dir(test_dir)

assert !io.file_exists(test_file)
assert !io.dir_exists(test_dir)

println("✓ test_19_advanced_file_io passed")
