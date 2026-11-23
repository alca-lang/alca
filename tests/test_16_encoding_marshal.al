// Copyright 2026 Pawl Programming Language
// Test 16: Encoding operations - hex, base64, marshal serialization.

use std.encoding
use std.marshal
use std.crypto

// 1. Hex encoding
original := "Hello Pawl 2026!"
hex_encoded := encoding.to_hex(original)
assert len(hex_encoded) > 0
// Hex should be twice the length of original (each byte = 2 hex chars)
assert len(hex_encoded) == len(original) * 2

// 2. Hex decoding
hex_decoded := encoding.from_hex(hex_encoded)
assert hex_decoded == original

// 3. Base64 encoding
b64_encoded := crypto.base64_encode(original)
assert len(b64_encoded) > 0
// Base64 should be roughly 4/3 of original length
assert len(b64_encoded) >= len(original)

// 4. Base64 decoding
b64_decoded := crypto.base64_decode(b64_encoded)
assert b64_decoded == original

// 5. Binary data encoding
binary_data := "Binary\x00\x01\x02\x03Data"
hex_binary := encoding.to_hex(binary_data)
assert len(hex_binary) == len(binary_data) * 2

decoded_binary := encoding.from_hex(hex_binary)
assert decoded_binary == binary_data

// 6. Supported encodings list
encodings := encoding.supported()
assert len(encodings) > 0
assert "utf-8" in encodings or "UTF-8" in encodings

// 7. UTF-8 encoding
utf8_text := "Привет мир! Здравствуйте! 🌍"
encoded_utf8 := encoding.encode(utf8_text, "utf-8")
assert len(encoded_utf8) > 0

// 8. UTF-8 decoding
decoded_utf8 := encoding.decode(encoded_utf8, "utf-8")
assert decoded_utf8 == utf8_text

// 9. Marshal dump (serialize to file)
test_data: map[string, int] = {"count": 42, "total": 100}
marshal_file := "./test_marshal.dat"
success := marshal.dump(test_data, marshal_file)
assert success

// 10. Marshal load (deserialize from file)
loaded_data := marshal.load(marshal_file)
assert loaded_data != null
if loaded_data != null {
    assert loaded_data["count"] == 42
    assert loaded_data["total"] == 100

// Cleanup
}
use std.io
io.delete_file(marshal_file)

// 11. Marshal dumps (serialize to blob)
complex_data: map[string, any] = {
    "name": "Alice",
    "age": 30,
    "scores": [95, 87, 92]
}
serialized_blob := marshal.dumps(complex_data)
assert len(serialized_blob) > 0

// 12. Marshal loads (deserialize from blob)
deserialized := marshal.loads(serialized_blob)
assert deserialized != null
if deserialized != null {
    assert deserialized["name"] == "Alice"
    assert deserialized["age"] == 30

// 13. Round-trip serialization
}
original_obj: list[int] = [1, 2, 3, 4, 5]
blob := marshal.dumps(original_obj)
restored := marshal.loads(blob)
assert restored == original_obj

// 14. Nested structure serialization
nested := {
    "user": {
        "name": "Bob",
        "metadata": {"role": "admin", "active": true}
    },
    "items": [1, 2, 3]
}

nested_blob := marshal.dumps(nested)
nested_restored := marshal.loads(nested_blob)
assert nested_restored != null

// 15. Unicode in marshal
unicode_obj := {"text": "Привет мир  🌍"}
unicode_blob := marshal.dumps(unicode_obj)
unicode_restored := marshal.loads(unicode_blob)
assert unicode_restored != null
if unicode_restored != null {
    assert unicode_restored["text"] == "Привет мир  🌍"

// 16. Large data serialization
}
large_list: list[int] = []
i := 0
while i < 1000 {
    large_list = large_list + [i]
    i = i + 1
}
large_blob := marshal.dumps(large_list)
assert len(large_blob) > 0

large_restored := marshal.loads(large_blob)
assert len(large_restored) == 1000
assert large_restored[0] == 0
assert large_restored[999] == 999

// 17. Boolean serialization
bool_data := {"flag1": true, "flag2": false}
bool_blob := marshal.dumps(bool_data)
bool_restored := marshal.loads(bool_blob)
assert bool_restored != null
if bool_restored != null {
    assert bool_restored["flag1"] == true
    assert bool_restored["flag2"] == false

// 18. Float serialization
}
float_data := {"pi": 3.14159, "e": 2.71828}
float_blob := marshal.dumps(float_data)
float_restored := marshal.loads(float_blob)
assert float_restored != null
if float_restored != null {
    assert float_restored["pi"] > 3.14 and float_restored["pi"] < 3.15

// 19. Null value serialization
}
null_data := {"value": null}
null_blob := marshal.dumps(null_data)
null_restored := marshal.loads(null_blob)
assert null_restored != null
if null_restored != null {
    assert null_restored["value"] == null

// 20. Empty collections serialization
}
empty_list: list[int] = []
empty_map: map[string, int] = {}

empty_list_blob := marshal.dumps(empty_list)
empty_list_restored := marshal.loads(empty_list_blob)
assert len(empty_list_restored) == 0

empty_map_blob := marshal.dumps(empty_map)
empty_map_restored := marshal.loads(empty_map_blob)
assert len(empty_map_restored) == 0

println("✓ test_16_encoding_marshal passed")
