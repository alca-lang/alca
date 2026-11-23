// Copyright 2026 Pawl Programming Language
// Test 12: Random number generation using crypto module.

use std.crypto

// 1. Random integer using crypto
rand_int := crypto.random_int(1, 100)
assert rand_int >= 1 and rand_int <= 100

// 2. Random string generation
letters := "abcdefghijklmnopqrstuvwxyz"
rand_str := crypto.random_string(10, letters)
assert len(rand_str) == 10
// All characters should be from the allowed set
for ch in rand_str {
    assert ch in letters

// 3. Random UUID generation
}
uuid := crypto.uuid()
assert len(uuid) == 36 // Standard UUID format: 8-4-4-4-12
// UUID should contain hyphens at correct positions
assert uuid[8] == '-'
assert uuid[13] == '-'
assert uuid[18] == '-'
assert uuid[23] == '-'

// 4. Random hex string
hex_chars := "0123456789abcdef"
rand_hex := crypto.random_string(16, hex_chars)
assert len(rand_hex) == 16
for ch in rand_hex {
    assert ch in hex_chars

// 5. Random boolean simulation
}
func random_bool() -> bool {
    return crypto.random_int(0, 1) == 1
}
rand_bool := random_bool()
assert rand_bool == true or rand_bool == false

// 6. Random choice from list simulation
func random_choice(items: list[string]) -> string {
    idx := crypto.random_int(0, len(items) - 1)
    return items[idx]
}
items := ["apple", "banana", "cherry", "date"]
choice := random_choice(items)
assert choice in items

// 7. Random sample simulation
func random_sample(items: list[int], count: int) -> list[int] {
    result: list[int] = []
    indices: list[int] = []
    while len(result) < count and len(result) < len(items) {
        idx := crypto.random_int(0, len(items) - 1)
        if !(idx in indices) {
            indices = indices + [idx]
            result = result + [items[idx]]
        }
    }
    return result
}
numbers := [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]
sample := random_sample(numbers, 3)
assert len(sample) == 3

// 8. Shuffle simulation
func shuffle_list(items: list[int]) -> list[int] {
    result: list[int] = []
    for item in items {
        result = result + [item]
    }
    i := 0
    while i < len(result) {
        j := crypto.random_int(0, len(result) - 1)
        temp := result[i]
        result[i] = result[j]
        result[j] = temp
        i = i + 1
    }
    return result
}
shuffled := shuffle_list([1, 2, 3, 4, 5])
assert len(shuffled) == 5

// 9. Random float simulation
func random_float(min: float, max: float) -> float {
    int_val := crypto.random_int(0, 10000)
    return min + (float(int_val) / 10000.0) * (max - min)
}
rand_float := random_float(0.0, 1.0)
assert rand_float >= 0.0 and rand_float <= 1.0

// 10. Weighted random choice
func weighted_choice(items: list[string], weights: list[int]) -> string {
    total_weight: int = 0
    for w in weights {
        total_weight = total_weight + w
    }
    rand_val := crypto.random_int(0, total_weight - 1)
    cumulative: int = 0
    i := 0
    while i < len(items) {
        cumulative = cumulative + weights[i]
        if rand_val < cumulative {
            return items[i]
        }
        i = i + 1
    }
    return items[len(items) - 1]
}
choices := ["low", "medium", "high"]
weights := [1, 3, 6]
result := weighted_choice(choices, weights)
assert result in choices

println("✓ test_12_random_generation passed")
