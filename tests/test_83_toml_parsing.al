// Copyright 2026 Pawl Programming Language
// Test 83: TOML parsing - TOML structure handling.

// 1. Simple key-value parsing
func parse_toml_kv(toml: string) -> map[string, string:
    result: map[string, string] = {}
    lines := string.split(toml, "\n")
    for line in lines {
        if string.contains(line, "=") {
            parts := string.split(line, "=")
            if len(parts) >= 2 {
                key := string.trim(parts[0])
                value := string.trim(string.replace(parts[1], "\"", ""))
                result[key] = value
            }
        }
    }
    return result

toml_kv := "name = \"John\"\nage = 30"
parsed := parse_toml_kv(toml_kv)
assert parsed["name"] == "John"
assert parsed["age"] == "30"

// 2. TOML section parsing
func parse_section(line: string) -> string {
    if string.starts_with(line, "[") and string.ends_with(line, "]") {
        return string.substring(line, 1, len(line) - 1)
    }
    return ""
}
section := parse_section("[database]")
assert section == "database"

// 3. TOML array parsing
func parse_toml_array(toml: string) -> list[string] {
    result: list[string] = []
    start := string.index_of(toml, "[")
    end := string.index_of(toml, "]")
    if start >= 0 and end > start {
        content := string.substring(toml, start + 1, end)
        items := string.split(content, ",")
        for item in items {
            result = result + [string.trim(string.replace(item, "\"", ""))]
        }
    }
    return result
}
toml_arr := "items = [\"a\", \"b\", \"c\"]"
array_parsed := parse_toml_array(toml_arr)
assert len(array_parsed) == 3
assert array_parsed[0] == "a"

// 4. TOML boolean parsing
func parse_toml_bool(value: string) -> bool {
    if value == "true" {
        return true
    }
    return false
}
assert parse_toml_bool("true")
assert !parse_toml_bool("false")

// 5. TOML number parsing
func parse_toml_int(value: string) -> int {
    return int(value)
}
assert parse_toml_int("42") == 42

// 6. TOML date parsing
func parse_toml_date(date_str: string) -> string {
    return date_str // Simplified
}
date := parse_toml_date("2026-05-15")
assert len(date) == 10

// 7. TOML inline table
func parse_inline_table(toml: string) -> map[string, string:
    result: map[string, string] = {}
    content := string.substring(toml, 1, len(toml) - 1)
    pairs := string.split(content, ",")
    for pair in pairs {
        if string.contains(pair, "=") {
            kv := string.split(pair, "=")
            if len(kv) >= 2 {
                key := string.trim(kv[0])
                value := string.trim(string.replace(kv[1], "\"", ""))
                result[key] = value
            }
        }
    }
    return result

inline := "{name = \"test\", value = 42}"
inline_parsed := parse_inline_table(inline)
assert inline_parsed["name"] == "test"

// 8. TOML multiline string
func parse_toml_ml_string(toml: string) -> string {
    if string.starts_with(toml, "\"\"\"") {
        lines := string.split(toml, "\n")
        if len(lines) > 1 {
            return string.join(lines[1..], "\n")
        }
    }
    return toml
}
ml_str := parse_toml_ml_string("\"\"\"\nline1\nline2\n\"\"\"")
assert string.contains(ml_str, "line1")

// 9. TOML validation
func is_valid_toml(toml: string) -> bool {
    has_equals := string.contains(toml, "=")
    has_bracket := string.contains(toml, "[")
    return has_equals or has_bracket
}
assert is_valid_toml("key = value")
assert is_valid_toml("[section]")

// 10. TOML key path
func parse_key_path(key: string) -> list[string] {
    return string.split(key, ".")
}
key_path := parse_key_path("database.host")
assert key_path == ["database", "host"]

println("✓ test_83_toml_parsing passed")
