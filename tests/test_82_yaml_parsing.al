// Copyright 2026 Pawl Programming Language
// Test 82: YAML parsing - YAML structure handling.

// 1. Simple key-value parsing
func parse_yaml_kv(yaml: string) -> map[string, string:
    result: map[string, string] = {}
    lines := string.split(yaml, "\n")
    for line in lines {
        if string.contains(line, ":") {
            parts := string.split(line, ":")
            if len(parts) >= 2 {
                key := string.trim(parts[0])
                value := string.trim(parts[1])
                result[key] = value
            }
        }
    }
    return result

yaml_kv := "name: John\nage: 30"
parsed := parse_yaml_kv(yaml_kv)
assert parsed["name"] == "John"
assert parsed["age"] == "30"

// 2. YAML list parsing
func parse_yaml_list(yaml: string) -> list[string] {
    result: list[string] = []
    lines := string.split(yaml, "\n")
    for line in lines {
        if string.starts_with(string.trim(line), "-") {
            item := string.trim(string.substring(line, 1, len(line)))
            result = result + [item]
        }
    }
    return result
}
yaml_list := "- apple\n- banana\n- cherry"
list_parsed := parse_yaml_list(yaml_list)
assert len(list_parsed) == 3
assert list_parsed[0] == "apple"

// 3. YAML indentation handling
func get_indent(line: string) -> int {
    count := 0
    i := 0
    while i < len(line) {
        if string.substring(line, i, i + 1) == " " {
            count = count + 1
        }
        else {
            break
        }
        i = i + 1
    }
    return count
}
indented := "  key: value"
assert get_indent(indented) == 2

// 4. YAML boolean parsing
func parse_yaml_bool(value: string) -> bool {
    if value == "true" or value == "yes" or value == "on" {
        return true
    }
    return false
}
assert parse_yaml_bool("true")
assert !parse_yaml_bool("false")

// 5. YAML number parsing
func parse_yaml_number(value: string) -> int {
    return int(value)
}
assert parse_yaml_number("42") == 42

// 6. YAML null handling
func is_yaml_null(value: string) -> bool {
    return value == "null" or value == "~" or value == ""
}
assert is_yaml_null("null")
assert !is_yaml_null("value")

// 7. YAML comment removal
func remove_comments(yaml: string) -> string {
    lines := string.split(yaml, "\n")
    result: list[string] = []
    for line in lines {
        if !(string.contains(line, "#")) or string.index_of(line, "#") > string.index_of(line, ":") {
            result = result + [line]
        }
    }
    return string.join(result, "\n")
}
yaml_with_comment := "key: value # comment"
cleaned := remove_comments(yaml_with_comment)
assert !string.contains(cleaned, "#")

// 8. YAML multiline string
func parse_multiline(yaml: string) -> string {
    lines := string.split(yaml, "\n")
    result: list[string] = []
    in_multiline := false
    for line in lines {
        if string.contains(line, "|") {
            in_multiline = true
            continue
        }
        if in_multiline {
            result = result + [string.trim(line)]
        }
    }
    return string.join(result, "\n")
}
yaml_ml := "text: |\n  line1\n  line2"
ml_parsed := parse_multiline(yaml_ml)
assert string.contains(ml_parsed, "line1")

// 9. YAML validation
func is_valid_yaml(yaml: string) -> bool {
    has_colon := string.contains(yaml, ":")
    has_dash := string.contains(yaml, "-")
    return has_colon or has_dash
}
assert is_valid_yaml("key: value")
assert is_valid_yaml("- item")

// 10. YAML nesting level
func nesting_level(line: string) -> int {
    return get_indent(line) / 2
}
nested_line := "    key: value"
assert nesting_level(nested_line) == 2

println("✓ test_82_yaml_parsing passed")
