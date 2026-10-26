// Copyright 2026 Alca Programming Language
// Everyday Test 01: String manipulation, text sanitization, formatting, and template interpolation.

use std.string;

// 1. Text Sanitization (trimming, casing)
raw_input := "   User_42@Example.Com   ";
cleaned := string.lower(string.trim(raw_input));
assert cleaned == "user_42@example.com";

// 2. String prefixes and suffixes
url := "https://api.alca.dev/v1/users";
assert string.starts_with(url, "https://");
assert string.ends_with(url, "/users");
assert string.contains(url, "api.alca.dev");

// 3. String Splitting and Joining
csv_row := "Apple,Banana,Orange,Mango";
items := string.split(csv_row, ",");
assert len(items) == 4;
assert items[0] == "Apple";
assert items[3] == "Mango";

joined := string.join(items, " | ");
assert joined == "Apple | Banana | Orange | Mango";

// 4. String Replacement & Template Formatting
template := "Hello, {name}! Your balance is {balance} USD.";
output := string.replace(template, "{name}", "Alexander");
output = string.replace(output, "{balance}", "1500");
assert output == "Hello, Alexander! Your balance is 1500 USD.";

// 5. IndexOf and Slicing
path := "/var/log/alca/service.log";
idx := string.index_of(path, "alca");
assert idx > 0;

println("✓ test_01_strings_text passed");
