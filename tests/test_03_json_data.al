// Copyright 2026 Alca Programming Language
// Everyday Test 03: JSON handling, parsing, serialization, and JSONPath querying.

use std.json;

// 1. JSON Validation
valid_payload := "{\"id\": 101, \"title\": \"Buy groceries\", \"completed\": false}";
invalid_payload := "{id: 101, unquoted}";

assert json.validate(valid_payload);
assert !json.validate(invalid_payload);

// 2. JSON Parsing & Direct Dynamic Indexing
item := json.parse(valid_payload);
assert item != null;

assert item["id"] == 101;
assert item["title"] == "Buy groceries";
assert item["completed"] == false;

// 3. Serializing maps to JSON
payload_map: map[string, string] = {"status": "active", "service": "alca-auth"};
encoded := json.encode(payload_map);
assert json.validate(encoded);

// 4. JSON Prettify
pretty := json.prettify(valid_payload);
assert len(pretty) > len(valid_payload);

println("✓ test_03_json_data passed");
