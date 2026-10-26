// Copyright 2026 Alca Programming Language
// Everyday Test 02: Collections, filtering, aggregation, dictionaries, and list algorithms.

// 1. Filtering list of numbers (finding even numbers)
numbers: list[int] = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10];
evens: list[int] = [];
sum_evens: int = 0;

for n in numbers:
    if n % 2 == 0:
        evens = evens + [n];
        sum_evens = sum_evens + n;

assert len(evens) == 5;
assert sum_evens == 30; // 2 + 4 + 6 + 8 + 10

// 2. Finding minimum and maximum
data: list[int] = [45, 12, 89, 3, 77, 24];
min_val: int = data[0];
max_val: int = data[0];

for val in data:
    if val < min_val:
        min_val = val;
    if val > max_val:
        max_val = val;

assert min_val == 3;
assert max_val == 89;

// 3. Frequency Counter with Map (Dictionary)
tags: list[string] = ["tech", "finance", "tech", "news", "tech", "finance"];
freq: map[string, int] = {};

for tag in tags:
    if tag in freq:
        freq[tag] = freq[tag] + 1;
    else:
        freq[tag] = 1;

assert freq["tech"] == 3;
assert freq["finance"] == 2;
assert freq["news"] == 1;

// 4. Key-Value Lookup & Updating
user_roles: map[string, string] = {"alice": "admin", "bob": "editor", "charlie": "viewer"};

assert user_roles["alice"] == "admin";
user_roles["bob"] = "admin";
assert user_roles["bob"] == "admin";

println("✓ test_02_collections_algorithms passed");
