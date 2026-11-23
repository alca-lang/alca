// Copyright 2026 Pawl Programming Language
// Test 21: Path manipulation - absolute/relative paths, join, normalize, and operations.

use std.path

// 1. Path joining
joined1 := path.join("home", "user", "documents")
assert joined1 == "home/user/documents" or joined1 == "home\\user\\documents"

joined2 := path.join("/home", "user")
assert string.contains(joined2, "home") and string.contains(joined2, "user")

// 2. Base name extraction
base1 := path.base("/home/user/file.txt")
assert base1 == "file.txt"

base2 := path.base("/home/user/documents/")
assert base2 == "documents" or base2 == ""

// 3. Directory extraction
dir1 := path.dir("/home/user/file.txt")
assert string.contains(dir1, "home") and string.contains(dir1, "user")

dir2 := path.dir("file.txt")
assert dir2 == "." or dir2 == ""

// 4. Extension extraction
ext1 := path.ext("file.txt")
assert ext1 == ".txt"

ext2 := path.ext("archive.tar.gz")
assert ext2 == ".gz"

ext3 := path.ext("noextension")
assert ext3 == ""

// 5. Stem/base name without extension
func stem(filepath: string) -> string {
    base := path.base(filepath)
    ext := path.ext(filepath)
    if len(ext) > 0 {
        return string.substring(base, 0, len(base) - len(ext))
    }
    return base
}
stem1 := stem("file.txt")
assert stem1 == "file"

stem2 := stem("archive.tar.gz")
assert stem2 == "archive.tar"

stem3 := stem("noextension")
assert stem3 == "noextension"

// 6. Absolute path resolution (simulation)
func is_absolute(p: string) -> bool {
    return string.starts_with(p, "/") or
           string.starts_with(p, "C:") or
           string.starts_with(p, "D:") or
           string.starts_with(p, "E:")
}
assert is_absolute("/home/user")
assert is_absolute("C:\\Windows")
assert !is_absolute("relative/path")

// 7. Relative path detection
func is_relative(p: string) -> bool {
    return !is_absolute(p)
}
assert is_relative("folder/file.txt")
assert is_relative("./file.txt")
assert !is_absolute("/absolute/path")

// 8. Path normalization
func normalize(p: string) -> string {
    // Replace backslashes with forward slashes
    result := string.replace(p, "\\", "/")
    // Remove duplicate slashes
    result = string.replace(result, "//", "/")
    // Remove trailing slash (except root)
    if len(result) > 1 and string.ends_with(result, "/") {
        result = string.substring(result, 0, len(result) - 1)
    }
    return result
}
norm1 := normalize("path\\\\to\\\\file")
assert norm1 == "path/to/file"

norm2 := normalize("path//to//file")
assert norm2 == "path/to/file"

norm3 := normalize("/path/to/")
assert norm3 == "/path/to"

// 9. Path simplification (remove . and ..)
func simplify(p: string) -> string {
    parts := string.split(p, "/")
    result: list[string] = []
    for part in parts {
        if part == "." or part == "" {
            continue
        }
        if part == ".." {
            if len(result) > 0 {
                result = result[0..len(result)-1]
            }
        }
        else {
            result = result + [part]
        }
    }
    return string.join(result, "/")
}
simplified1 := simplify("a/b/../c")
assert simplified1 == "a/c"

simplified2 := simplify("a/./b/./c")
assert simplified2 == "a/b/c"

simplified3 := simplify("../a/../b")
assert simplified3 == "b"

// 10. Common prefix detection
func common_prefix(p1: string, p2: string) -> string {
    i := 0
    while i < len(p1) and i < len(p2) {
        if string.substring(p1, i, i + 1) != string.substring(p2, i, i + 1) {
            break
        }
        i = i + 1
    }
    return string.substring(p1, 0, i)
}
prefix := common_prefix("/home/user/docs", "/home/user/pics")
assert prefix == "/home/user/"

// 11. Relative path between two paths
func relative(from: string, to: string) -> string {
    // Simplified relative path calculation
    if is_absolute(from) != is_absolute(to) {
        return to // Can't compute relative
    }
    from_parts := string.split(from, "/")
    to_parts := string.split(to, "/")
    i := 0
    while i < len(from_parts) and i < len(to_parts) {
        if from_parts[i] != to_parts[i] {
            break
        }
        i = i + 1
    }
    ups := len(from_parts) - i
    result: list[string] = []
    j := 0
    while j < ups {
        result = result + [".."]
        j = j + 1
    }
    k := i
    while k < len(to_parts) {
        result = result + [to_parts[k]]
        k = k + 1
    }
    return string.join(result, "/")
}
rel := relative("/home/user/docs", "/home/user/pics")
assert rel == "../pics" or rel == "pics"

// 12. Path validation
func is_valid_path(p: string) -> bool {
    // Basic validation: no invalid characters
    invalid := ["<", ">", ":", "\"", "|", "?", "*"]
    for ch in invalid {
        if string.contains(p, ch) {
            return false
        }
    }
    return true
}
assert is_valid_path("valid/path.txt")
assert !is_valid_path("invalid<path>.txt")

// 13. File name sanitization
func sanitize_filename(name: string) -> string {
    invalid := ["<", ">", ":", "\"", "|", "?", "*", "/", "\\"]
    result := name
    for ch in invalid {
        result = string.replace(result, ch, "_")
    }
    return result
}
sanitized := sanitize_filename("file:name?.txt")
assert string.contains(sanitized, "_")
assert !string.contains(sanitized, ":")

// 14. Path extension change
func change_extension(filepath: string, new_ext: string) -> string {
    base := path.base(filepath)
    old_ext := path.ext(filepath)
    stem := string.substring(base, 0, len(base) - len(old_ext))
    dir := path.dir(filepath)
    if dir == "." or dir == "" {
        return stem + new_ext
    }
    return path.join(dir, stem + new_ext)
}
changed := change_extension("/path/to/file.txt", ".md")
assert string.ends_with(changed, ".md")
assert string.contains(changed, "file.md")

// 15. Path depth calculation
func path_depth(p: string) -> int {
    parts := string.split(p, "/")
    count := 0
    for part in parts {
        if part != "" and part != "." {
            count = count + 1
        }
    }
    return count
}
depth1 := path_depth("a/b/c")
assert depth1 == 3

depth2 := path_depth("/a/b/c")
assert depth2 == 3

// 16. Root path detection
func is_root(p: string) -> bool {
    return p == "/" or p == "\\" or
           string.ends_with(p, ":/") or
           string.ends_with(p, ":\\")
}
assert is_root("/")
assert is_root("C:/")
assert !is_root("C:/Windows")

// 17. Drive letter extraction (Windows)
func get_drive_letter(p: string) -> string {
    if len(p) >= 2 and string.substring(p, 1, 2) == ":" {
        return string.substring(p, 0, 1)
    }
    return ""
}
drive := get_drive_letter("C:/Windows")
assert drive == "C"

drive2 := get_drive_letter("/unix/path")
assert drive2 == ""

// 18. Path case sensitivity check
func is_case_sensitive(p: string) -> bool {
    // Unix is case-sensitive, Windows is not
    return string.starts_with(p, "/") or !string.contains(p, ":")
}
assert is_case_sensitive("/unix/path")
assert !is_case_sensitive("C:\\Windows")

// 19. Path traversal check
func has_traversal(p: string) -> bool {
    return string.contains(p, "..")
}
assert has_traversal("../../../etc/passwd")
assert !has_traversal("normal/path")

// 20. Safe path join (prevents traversal)
func safe_join(base: string, user_path: string) -> string {
    if has_traversal(user_path) {
        return base // Reject traversal attempts
    }
    return path.join(base, user_path)
}
safe := safe_join("/var/www", "../../../etc/passwd")
assert safe == "/var/www"

safe2 := safe_join("/var/www", "images/logo.png")
assert string.contains(safe2, "images")

println("✓ test_21_path_manipulation passed")
