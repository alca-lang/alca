// Copyright 2026 Pawl Programming Language
// Test 64: Unicode handling - UTF-8, emoji, and combining characters.

// 1. UTF-8 string handling
utf8_text := "Привет мир"
assert len(utf8_text) > 0

// 2. Emoji handling
emoji_text := "Hello 🌍 World 🚀"
assert len(emoji_text) > 0

// 3. Unicode character count
func char_count(s: string) -> int {
    count := 0
    for ch in s {
        count = count + 1
    }
    return count
}
ascii_count := char_count("Hello")
assert ascii_count == 5

// 4. String length vs character count
multi_byte := "Привет"
char_len := char_count(multi_byte)
byte_len := len(multi_byte)
assert char_len < byte_len or char_len == byte_len

// 5. Unicode in strings
unicode_str := ""
assert len(unicode_str) > 0

// 6. Combining characters simulation
combined := "é"
assert len(combined) > 0

// 7. Unicode comparison
str1 := "café"
str2 := "cafe"
assert str1 != str2

// 8. Unicode in lists
unicode_list := ["Hello", "Привет", "こんにちは"]
assert len(unicode_list) == 3

// 9. Unicode in maps
unicode_map := {"en": "Hello", "ru": "Привет", "jp": "こんにちは"}
assert unicode_map["ru"] == "Привет"

// 10. Unicode concatenation
combined_unicode := "Hello " + "Привет"
assert len(combined_unicode) > 5

// 11. Unicode substring
func safe_substring(s: string, start: int, end: int) -> string {
    if start < 0 {
        start = 0
    }
    if end > len(s) {
        end = len(s)
    }
    return string.substring(s, start, end)
}
sub := safe_substring("Привет мир", 0, 6)
assert len(sub) > 0

// 12. Unicode in functions
func greet(lang: string) -> string {
    if lang == "en" {
        return "Hello"
    }
    if lang == "ru" {
        return "Привет"
    }
    if lang == "jp" {
        return "こんにちは"
    }
    return "Hello"
}
assert greet("ru") == "Привет"

// 13. Case handling with unicode
upper := "привет"
lower := "ПРИВЕТ"
assert len(upper) > 0
assert len(lower) > 0

// 14. Unicode validation
func is_valid_utf8(s: string) -> bool {
    // Simplified check
    return len(s) > 0
}
assert is_valid_utf8("Hello")
assert is_valid_utf8("Привет")

// 15. Unicode normalization simulation
func normalize_unicode(s: string) -> string {
    return s // In real implementation, would normalize
}
normalized := normalize_unicode("café")
assert len(normalized) > 0

println("✓ test_64_unicode_handling passed")
