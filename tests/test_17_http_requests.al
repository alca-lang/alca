// Copyright 2026 Pawl Programming Language
// Test 17: HTTP requests - GET, POST, headers, status codes, and response handling.

use std.http
use std.net

// Note: These tests are designed to work with local/mock servers.
// In production, use actual test endpoints.

// 1. HTTP GET request (mock simulation)
func http_get_simulation(url: string) -> map[string, any] {
    // Simulate HTTP GET response
    if string.contains(url, "example.com") {
        return {
            "status": 200,
            "body": "{\"message\": \"success\"}",
            "headers": {"content-type": "application/json"}
        }
    }
    else {
        return {
            "status": 404,
            "body": "Not Found",
            "headers": {"content-type": "text/plain"}
        }
    }
}
response := http_get_simulation("https://example.com/api")
assert response["status"] == 200
assert string.contains(response["body"], "success")

// 2. HTTP POST request simulation
func http_post_simulation(url: string, data: string) -> map[string, any] {
    if string.contains(url, "api/create") {
        return {
            "status": 201,
            "body": "{\"id\": 123, \"created\": true}",
            "headers": {"content-type": "application/json"}
        }
    }
    else {
        return {
            "status": 400,
            "body": "Bad Request",
            "headers": {"content-type": "text/plain"}
        }
    }
}
post_response := http_post_simulation("https://api.example.com/create", "{\"name\": \"test\"}")
assert post_response["status"] == 201

// 3. Header parsing simulation
func parse_headers(headers: map[string, string]) -> map[string, string] {
    result: map[string, string] = {}
    for key in headers {
        result[string.lower(key)] = headers[key]
    }
    return result
}
raw_headers := {"Content-Type": "application/json", "Authorization": "Bearer token"}
parsed := parse_headers(raw_headers)
assert parsed["content-type"] == "application/json"
assert parsed["authorization"] == "Bearer token"

// 4. Status code validation
func is_success(status: int) -> bool {
    return status >= 200 and status < 300
}
func is_redirect(status: int) -> bool {
    return status >= 300 and status < 400
}
func is_client_error(status: int) -> bool {
    return status >= 400 and status < 500
}
func is_server_error(status: int) -> bool {
    return status >= 500 and status < 600
}
assert is_success(200)
assert is_success(204)
assert is_redirect(301)
assert is_client_error(404)
assert is_server_error(500)

// 5. Query string construction
func build_query(params: map[string, string]) -> string {
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
}
query_params := {"search": "pawl", "page": "1", "limit": "10"}
query_string := build_query(query_params)
assert string.contains(query_string, "search=pawl")
assert string.contains(query_string, "page=1")

// 6. URL parsing simulation
func parse_url(url: string) -> map[string, string] {
    // Simplified URL parsing
    result: map[string, string] = {}
    if string.contains(url, "://") {
        parts := string.split(url, "://")
        result["protocol"] = parts[0]
        rest := parts[1]
        if string.contains(rest, "/") {
            path_parts := string.split(rest, "/")
            result["host"] = path_parts[0]
            result["path"] = "/" + string.join(path_parts[1..], "/")
        }
        else {
            result["host"] = rest
            result["path"] = "/"
        }
    }
    return result
}
parsed_url := parse_url("https://example.com/api/users")
assert parsed_url["protocol"] == "https"
assert parsed_url["host"] == "example.com"
assert parsed_url["path"] == "/api/users"

// 7. Cookie parsing simulation
func parse_cookies(cookie_header: string) -> map[string, string] {
    cookies: map[string, string] = {}
    pairs := string.split(cookie_header, "; ")
    for pair in pairs {
        if string.contains(pair, "=") {
            kv := string.split(pair, "=")
            cookies[kv[0]] = kv[1]
        }
    }
    return cookies
}
cookie_str := "session=abc123; user=john; theme=dark"
cookies := parse_cookies(cookie_str)
assert cookies["session"] == "abc123"
assert cookies["user"] == "john"

// 8. JSON response parsing
use std.json
json_response := "{\"status\": \"ok\", \"data\": {\"count\": 42}}"
parsed_json := json.parse(json_response)
assert parsed_json["status"] == "ok"
assert parsed_json["data"]["count"] == 42

// 9. Request timeout simulation
func request_with_timeout(url: string, timeout_ms: int) -> map[string, any] {
    // Simulate timeout
    if string.contains(url, "slow") {
        return {"status": 0, "error": "timeout"}
    }
    return {"status": 200, "body": "OK"}
}
timeout_response := request_with_timeout("https://slow.example.com", 5000)
assert timeout_response["status"] == 0

// 10. Retry logic simulation
func http_request_with_retry(url: string, max_retries: int) -> map[string, any] {
    attempts := 0
    while attempts < max_retries {
        response := http_get_simulation(url)
        if response["status"] == 200 {
            return response
        }
        attempts = attempts + 1
    }
    return {"status": 0, "error": "max retries exceeded"}
}
retry_response := http_request_with_retry("https://example.com/api", 3)
assert retry_response["status"] == 200

// 11. User-Agent header construction
func build_user_agent(app_name: string, version: string) -> string {
    return app_name + "/" + version + " (Pawl/1.0)"
}
user_agent := build_user_agent("MyApp", "1.0.0")
assert user_agent == "MyApp/1.0.0 (Pawl/1.0)"

// 12. Basic authentication header
func build_auth_header(username: string, password: string) -> string {
    use std.crypto
    credentials := username + ":" + password
    encoded := crypto.base64_encode(credentials)
    return "Basic " + encoded
}
auth_header := build_auth_header("admin", "secret")
assert string.contains(auth_header, "Basic ")
assert string.contains(auth_header, "YWRtaW46c2VjcmV0") // base64 of "admin:secret"

// 13. Content-Type detection
func get_content_type(extension: string) -> string {
    types := {
        ".json": "application/json",
        ".xml": "application/xml",
        ".html": "text/html",
        ".txt": "text/plain",
        ".pdf": "application/pdf"
    }
    if extension in types {
        return types[extension]
    }
    return "application/octet-stream"
}
assert get_content_type(".json") == "application/json"
assert get_content_type(".html") == "text/html"
assert get_content_type(".unknown") == "application/octet-stream"

// 14. Response body size calculation
func get_response_size(response: map[string, any]) -> int {
    body := response["body"]
    if typeof(body) == "string" {
        return len(body)
    }
    return 0
}
sample_response := {"status": 200, "body": "Hello World"}
size := get_response_size(sample_response)
assert size == 11

// 15. HTTP method validation
func is_valid_method(method: string) -> bool {
    valid_methods := ["GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"]
    return method in valid_methods
}
assert is_valid_method("GET")
assert is_valid_method("POST")
assert !is_valid_method("INVALID")

println("✓ test_17_http_requests passed")
