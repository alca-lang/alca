// Copyright 2026 Pawl Programming Language
// Test 15: String operations, regex patterns, search, and replacement.

use std.string
use std.regex

// 1. Basic regex matching
email_pattern := "[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}"
valid_email := "user@example.com"
invalid_email := "not-an-email"

assert regex.match(valid_email, email_pattern)
assert !regex.match(invalid_email, email_pattern)

// 2. Phone number pattern (simplified)
phone_pattern := "\\+?[0-9]{10,15}"
valid_phone := "+1234567890"
invalid_phone := "123"

assert regex.match(valid_phone, phone_pattern)
assert !regex.match(invalid_phone, phone_pattern)

// 3. URL pattern detection
url_pattern := "https?://[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}(/[^\\s]*)?"
valid_url := "https://example.com/path"
invalid_url := "not-a-url"

assert regex.match(valid_url, url_pattern)
assert !regex.match(invalid_url, url_pattern)

// 4. Regex find all occurrences
text := "The prices are $10, $20, and $30"
price_pattern := "\\$[0-9]+"
matches := regex.find_all(text, price_pattern)
assert len(matches) == 3
assert matches[0] == "$10"
assert matches[2] == "$30"

// 5. Regex replace
html := "<b>bold</b> and <i>italic</i>"
stripped := regex.replace(html, "<[^>]+>", "")
assert stripped == "bold and italic"

// 6. Case insensitive matching
case_text := "Hello WORLD"
lower_pattern := "[a-z]+"
assert regex.match(case_text, lower_pattern) // Should match "Hello"

// 7. Date pattern validation (YYYY-MM-DD)
date_pattern := "[0-9]{4}-[0-9]{2}-[0-9]{2}"
valid_date := "2026-05-15"
invalid_date := "15-05-2026"

assert regex.match(valid_date, date_pattern)
assert !regex.match(invalid_date, date_pattern)

// 8. Hex color code pattern
hex_pattern := "#[0-9A-Fa-f]{6}"
valid_hex := "#FF5733"
invalid_hex := "FF5733"

assert regex.match(valid_hex, hex_pattern)
assert !regex.match(invalid_hex, hex_pattern)

// 9. Username pattern (alphanumeric + underscore)
username_pattern := "^[a-zA-Z0-9_]{3,20}$"
valid_username := "user_123"
invalid_username := "user@123"

assert regex.match(valid_username, username_pattern)
assert !regex.match(invalid_username, username_pattern)

// 10. IPv4 address pattern
ipv4_pattern := "^[0-9]{1,3}\\.[0-9]{1,3}\\.[0-9]{1,3}\\.[0-9]{1,3}$"
valid_ip := "192.168.1.1"
invalid_ip := "256.1.1.1"

assert regex.match(valid_ip, ipv4_pattern)
assert !regex.match(invalid_ip, ipv4_pattern)

// 11. Advanced string operations with regex
func extract_domain(email: string) -> string {
    domain_pattern := "@([a-zA-Z0-9.-]+\\.[a-zA-Z]{2,})"
    matches := regex.find_all(email, domain_pattern)
    if len(matches) > 0 {
        // Remove the @ prefix
        return string.substring(matches[0], 1, len(matches[0]))
    }
    return ""
}
domain := extract_domain("user@example.com")
assert domain == "example.com"

// 12. Validate password strength
func is_strong_password(password: string) -> bool {
    // At least 8 chars, 1 uppercase, 1 lowercase, 1 digit
    has_upper := regex.match(password, "[A-Z]")
    has_lower := regex.match(password, "[a-z]")
    has_digit := regex.match(password, "[0-9]")
    has_length := len(password) >= 8
    return has_upper and has_lower and has_digit and has_length
}
weak_pass := "weak"
strong_pass := "StrongPass123"

assert !is_strong_password(weak_pass)
assert is_strong_password(strong_pass)

// 13. Split by regex delimiter
csv_data := "apple,banana;cherry|date"
delim_pattern := "[,;|]"
parts := regex.split(csv_data, delim_pattern)
assert len(parts) == 4
assert parts[0] == "apple"
assert parts[3] == "date"

// 14. Remove extra whitespace
messy_text := "Hello    World   Test"
cleaned := regex.replace(messy_text, "\\s+", " ")
assert cleaned == "Hello World Test"

// 15. Extract numbers from text
text_with_nums := "Item 1: $10, Item 2: $20, Item 3: $30"
num_pattern := "[0-9]+"
numbers := regex.find_all(text_with_nums, num_pattern)
assert len(numbers) == 6 // 1, 10, 2, 20, 3, 30

// 16. Validate credit card (Luhn check simulation)
func luhn_check(card: string) -> bool {
    // Simplified Luhn algorithm
    total: int = 0
    is_even := false
    i := len(card) - 1
    while i >= 0 {
        digit := int(string.substring(card, i, i + 1))
        if is_even {
            digit = digit * 2
            if digit > 9 {
                digit = digit - 9
            }
        }
        total = total + digit
        is_even = !is_even
        i = i - 1
    }
    return total % 10 == 0
}
valid_card := "4532015112830366" // Test card number
assert luhn_check(valid_card)

// 17. String escape/unescape simulation
func escape_special(text: string) -> string {
    result := string.replace(text, "\n", "\\n")
    result = string.replace(result, "\t", "\\t")
    result = string.replace(result, "\"", "\\\"")
    return result
}
escaped := escape_special("Hello\nWorld\t\"Test\"")
assert string.contains(escaped, "\\n")
assert string.contains(escaped, "\\t")

// 18. Validate JSON structure (basic)
func is_valid_json_structure(json_str: string) -> bool {
    // Check for basic JSON structure
    has_braces := string.contains(json_str, "{") and string.contains(json_str, "}")
    has_quotes := string.contains(json_str, "\"")
    return has_braces and has_quotes
}
valid_json := "{\"key\": \"value\"}"
invalid_json := "not json"

assert is_valid_json_structure(valid_json)
assert !is_valid_json_structure(invalid_json)

println("✓ test_15_string_regex passed")
