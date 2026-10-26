// Copyright 2026 Alca Programming Language
// Everyday Test 04: Parsing CSV tabular records and INI configuration files.

use std.csv;
use std.ini;

// 1. Parsing CSV data string
csv_data := "name,role,salary\nAlice,Engineer,95000\nBob,Designer,80000\nCharlie,Manager,110000";
records := csv.read(csv_data, true);

assert len(records) == 3;
rec0 := records[0];
assert rec0["name"] == "Alice";
assert rec0["role"] == "Engineer";

rec2 := records[2];
assert rec2["name"] == "Charlie";
assert rec2["salary"] == "110000";

// 2. Parsing INI configuration string
ini_content := "[server]\nhost = 127.0.0.1\nport = 8080\n[database]\nname = production\nmax_connections = 50";
config := ini.parse(ini_content);

assert config != null;
server_section := config["server"];
assert server_section["host"] == "127.0.0.1";
assert server_section["port"] == "8080";

db_section := config["database"];
assert db_section["name"] == "production";

println("✓ test_04_csv_ini_formats passed");
