// Copyright 2026 Pawl Programming Language
// Test 84: URL operations - encode/decode, parse, and manipulation.

// 1. URL encoding simulation
func url_encode(s: string) -> string {
    result := ""
    for ch in s {
        if ch == " " {
            result = result + "%20"
        }
        else {
            result = result + ch
        }
    }
    return result
}
encoded := url_encode("hello world")
assert string.contains(encoded, "%20")

// 2. URL decoding simulation
func url_decode(s: string) -> string {
    result := string.replace(s, "%20", " ")
    return result
}
decoded := url_decode("hello%20world")
assert decoded == "hello world"

// 3. URL parsing
func parse_url(url: string) -> map[string, string:
    result: map[string, string] = {}
    if string.contains(url, "://") {
        parts := string.split(url, "://")
        result["protocol"] = parts[0]
        rest := parts[1]
        if string.contains(rest, "/") {
            path_parts := string.split(rest, "/")
            result["host"] = path_parts[0]
            if len(path_parts) > 1 {
                result["path"] = "/" + string.join(path_parts[1..], "/")
            }
        }
        else {
            result["host"] = rest
            result["path"] = "/"
        }
    }
    return result

parsed_url := parse_url("https://example.com/path")
assert parsed_url["protocol"] == "https"
assert parsed_url["host"] == "example.com"

// 4. Query string parsing
func parse_query(query: string) -> map[string, string:
    result: map[string, string] = {}
    if string.starts_with(query, "?") {
        query = string.substring(query, 1, len(query))
    }
    pairs := string.split(query, "&")
    for pair in pairs {
        if string.contains(pair, "=") {
            kv := string.split(pair, "=")
            if len(kv) >= 2 {
                result[kv[0]] = kv[1]
            }
        }
    }
    return result

query_str := "?key1=value1&key2=value2"
query_parsed := parse_query(query_str)
assert query_parsed["key1"] == "value1"

// 5. Build query string
func build_query(params: map[string, string) -> string:
    result := "?"
    first := true
    for key in params {
        if !first {
            result = result + "&"
        }
        result = result + key + "=" + params[key]
        first = false
    }
    return result

built := build_query({"a": "1", "b": "2"})
assert string.contains(built, "a=1")

// 6. URL join
func url_join(base: string, path: string) -> string {
    if string.ends_with(base, "/") {
        return base + path
    }
    return base + "/" + path
}
joined := url_join("https://example.com", "path")
assert joined == "https://example.com/path"

// 7. Get domain from URL
func get_domain(url: string) -> string {
    parsed := parse_url(url)
    return parsed["host"]
}
domain := get_domain("https://sub.example.com/path")
assert domain == "sub.example.com"

// 8. Get path from URL
func get_path(url: string) -> string {
    parsed := parse_url(url)
    return parsed["path"]
}
path := get_path("https://example.com/api/users")
assert path == "/api/users"

// 9. Check if URL is HTTPS
func is_https(url: string) -> bool {
    return string.starts_with(url, "https://")
}
assert is_https("https://example.com")
assert !is_https("http://example.com")

// 10. URL validation
func is_valid_url(url: string) -> bool {
    return string.contains(url, "://") and len(url) > 7
}
assert is_valid_url("https://example.com")
assert !is_valid_url("not-a-url")

println("✓ test_84_url_operations passed")
