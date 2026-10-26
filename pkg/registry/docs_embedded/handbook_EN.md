# Comprehensive Alca Programming Language Handbook

[English](handbook_EN.md) | [Русский](handbook_RU.md)

> **Language Version:** Alca v2.0.0  
> **Document Status:** Canonical Handbook  
> **Target Audience:** Systems automation developers, cross-platform tool creators, and backend service developers.

---

## Table of Contents

1. [Introduction and Platform Architecture](#1-introduction-and-platform-architecture)
2. [Syntax, Lexical Structure, and Program Layout](#2-syntax-lexical-structure-and-program-layout)
3. [Type System and Safety](#3-type-system-and-safety)
4. [Control Flow, Operators, and Pipelines](#4-control-flow-operators-and-pipelines)
   - [4.5 Functional Pipelines (`|>`)](#45-functional-pipelines-pipeline-operator-)
   - [4.6 Stream Pipelines (`>>`, `>!`, `>>>`, `<<`, `&>>`, `?>>>`)](#46-stream-pipelines)
5. [Functions, Closures, and Concurrency](#5-functions-closures-and-concurrency)
6. [Object-Oriented and Contract Programming](#6-object-oriented-and-contract-programming)
7. [Error Handling and Resilience](#7-error-handling-and-resilience)
8. [Global Built-in Functions and System Variables](#8-global-built-in-functions-and-system-variables)
9. [Capability-Based Sandbox & Security](#9-capability-based-sandbox--security)
10. [Standard Library Reference (Stdlib)](#10-standard-library-reference-stdlib)
    - [10.1 Collections and Data Structures](#101-collections-and-data-structures)
    - [10.2 I/O and Filesystem](#102-io-and-filesystem)
    - [10.3 Networking and Web](#103-networking-and-web)
    - [10.4 Cryptography and Security](#104-cryptography-and-security)
    - [10.5 Data Formats and Configuration](#105-data-formats-and-configuration)
    - [10.6 Date, Time, and Benchmarking](#106-date-time-and-benchmarking)
    - [10.7 Mathematics and Computation](#107-mathematics-and-computation)
    - [10.8 System Processes and Environment](#108-system-processes-and-environment)
    - [10.9 Native Integration and OS](#109-native-integration-and-os)
    - [10.10 Multimedia and Graphical UI](#1010-multimedia-and-graphical-ui)
    - [10.11 Databases, Logging, and Telemetry](#1011-databases-logging-and-telemetry)
11. [Build, Distribution, and Testing](#11-build-distribution-and-testing)
    - [11.1 Execution & Type Check](#111-execution--type-check)
    - [11.2 Unit Tests](#112-unit-tests)
    - [11.3 Standalone Executables (Windows PE/EXE)](#113-standalone-executables-windows-peexe)
    - [11.4 Native Dynamic Libraries (DLL)](#114-native-dynamic-libraries-dll)
    - [11.5 Package Management (APM)](#115-package-management-apm)
    - [11.6 IDE & Editor Integration (LSP)](#116-ide--editor-integration-lsp)

---

## 1. Introduction and Platform Architecture

**Alca** is a modern statically-typed general-purpose programming language engineered for standalone systems automation, high-performance tooling, and cross-platform native applications.

### Core Architectural Pillars
- **Zero-Toolchain Distribution:** Builds self-contained native Windows PE/EXE executables (`alca build`) and shared DLL libraries (`alca dll`) without requiring MSVC, MinGW, or Clang.
- **Four-Tier Execution Hierarchy:**
  1. *Bytecode Virtual Machine (VM)*: High-throughput virtual CPU with register/stack frames.
  2. *Multi-Tier AMD64 JIT*: Native machine code compilation for hot loops and functions.
  3. *Reference Tree-Walking Interpreter*: Full dynamic introspection and meta-programming support.
  4. *Native PE / DLL AOT Generator*: Direct byte-level PE/COFF assembler.
- **Capability-Based Sandbox:** Strict default-deny access model governing the filesystem, network sockets, subprocess execution, environment variables, and native dynamic libraries.

---

## 2. Syntax, Lexical Structure, and Program Layout

Alca source files use the `.al` extension.

### 2.1 Statement Termination and Blocks
- Simple statements terminate with a semicolon (`;`).
- Code blocks are enclosed either in **canonical curly braces `{ ... }`** or a colon (`:`) followed by a 4-space indented suite:

```alca
// Canonical curly braces (recommended)
fn max(a: int, b: int) -> int {
    if a > b {
        return a;
    }
    return b;
}

// Colon and indentation syntax
fn min(a: int, b: int) -> int:
    if a < b:
        return a;
    return b;
```

### 2.2 Comments
```alca
// Single-line comment

/*
  Multi-line
  comment block
*/
```

### 2.3 Module Imports
The `use` keyword imports standard library modules or relative local scripts:
```alca
use std.math;
use std.io;
use "./helpers.al";
```

---

## 3. Type System and Safety

Alca enforces **strict static typing** with bidirectional inference. Implicit widening, narrowing, or silent coercions are strictly prohibited.

### 3.1 Primitive Types
- `int`: 64-bit signed integer.
- `float`: 64-bit IEEE 754 double-precision floating-point number.
- `string`: Immutable UTF-8 character sequence.
- `bool`: Boolean value (`true`, `false`).
- `void`: Unit return type for functions producing no value.

### 3.2 Declarations and Type Inference
```alca
// Explicit type annotations
counter: int = 0;
pi: float = 3.14159265;
title: string = "Alca Language";
is_ready: bool = true;

// Type inference via `:=` (type is fixed permanently at compile time)
ratio := 2.5;                // float
names := ["Alice", "Bob"];   // list[string]

// Shadowing within the same or nested scope is rejected:
// counter: int = 5;         // COMPILE ERROR: variable 'counter' already declared
counter = 5;                 // OK: mutation / assignment
```

### 3.3 Strict Arithmetic
Numbers do not coerce silently:
```alca
x: int = 10;
y: float = 2.5;

// total := x + y;            // COMPILE ERROR: operator '+' cannot be applied to int and float
total := float(x) + y;        // OK: 12.5

int_res := 7 / 2;             // Integer division truncates towards zero: 3
float_res := float(7) / 2.0;  // Floating-point division: 3.5
```

### 3.4 Null-Safety and Type Narrowing
Values of type `T` cannot be `null`. Nullable values must be explicitly declared as `T | null`:
```alca
user_name: string | null = null;

// Direct member access on nullable types is rejected by typecheck:
// len(user_name);            // COMPILE ERROR: value may be null

// Safe narrowing within conditional checks:
if user_name != null {
    // Within this block, user_name is safely narrowed to string
    println("Length: " + string(len(user_name)));
}
```

### 3.5 Dynamic Escape Hatch: `json`
`json` is the sole dynamic type in Alca, designed for unstructured external data (e.g. REST APIs). Values must be explicitly converted before use in typed contexts.

---

## 4. Control Flow, Operators, and Pipelines

### 4.1 Equality and Comparisons
- `==` and `!=`: Structural value equality. Incompatible types (e.g. `"1" == 1`) trigger compile errors.
- `===` and `!==`: Strict memory pointer/identity comparison.
- `<`, `<=`, `>`, `>=`: Ordering comparisons.

### 4.2 Boolean Logic
- Logical AND: `and` (or `&&`).
- Logical OR: `or` (or `||`).
- Logical NOT: `not` (or `!`).
- Conditions in `if`, `elif`, `while` must evaluate strictly to `bool` (truthy/falsy coercion does not exist).

### 4.3 Ranges
- `a..b`: Half-open range $[a, b)$. Number $b$ is excluded (e.g., `0..3` produces `0, 1, 2`).
- `a..=b`: Inclusive range $[a, b]$. Number $b$ is included (e.g., `0..=3` produces `0, 1, 2, 3`).

### 4.4 Loops
```alca
users := ["Alex", "Maria", "Ivan"];
for u in users {
    if u == "Maria" {
        continue;
    }
    println("User: " + u);
}

n := 5;
while n > 0 {
    n = n - 1;
}
```

### 4.5 Functional Pipelines (Pipeline Operator `|>`)
The `|>` operator forwards the left expression as the **first argument** to the function on the right, eliminating nested function calls:

```alca
fn double(x: int) -> int { return x * 2; }
fn add(x: int, y: int) -> int { return x + y; }
fn to_str(x: int) -> string { return "Result: " + string(x); }

// Single argument chaining:
val := 10 |> double; // 20

// Multi-argument function pipeline (first argument passed automatically):
res := 5 |> double |> add(3) |> to_str;
// Equivalent to: to_str(add(double(5), 3)) -> "Result: 13"
```

### 4.6 Stream Pipelines
Alca provides native I/O stream operators compiled directly into virtual machine opcodes (`OpStream`):

| Operator | Token | Behavior |
| :--- | :--- | :--- |
| `>>` | `STREAM_APPEND` | **Append** stream to file or HTTP/WS POST endpoint with trailing newline. |
| `>!` | `STREAM_OVERWRITE` | **Overwrite** file contents (or network POST). |
| `>>>` | `STREAM_VAR` | **Stream assign** expression result directly into an environment variable. |
| `<<` | `STREAM_IN` | **Stream read** from file or console input (`stdin`). |
| `&>>` | `STREAM_ASYNC` | **Async background file stream** executed in a goroutine without blocking. |
| `&&>>` | `STREAM_THREAD` | **Async background network stream**. |
| `?>>` | `STREAM_ERR` | **Stream error** output to log file: `[TIME] Error(CODE): MSG`. |
| `?>>>` | `STREAM_ERR_VAR` | **Capture error** object into variable of type `error \| null` without throwing. |

#### Stream Pipeline Examples:
```alca
// 1. Stream file write and append:
"System initializing..." >! "app.log";        // Overwrite (>!)
"[INFO] Database connected" >> "app.log";     // Append (>>)

// 2. Stream read (<<):
config_str << "settings.json"; // Read file into config_str
user_name << console;          // Read line from stdin

// 3. Stream save into variable (>>>):
(math.sqrt(144.0) * 2.0) >>> computed_res;
assert computed_res == 24.0;

// 4. Network stream POST:
"{\"status\": \"healthy\"}" >> "https://monitoring.internal/api/beat";

// 5. Non-blocking async stream append (&>>):
heavy_audit_dump &>> "audit.log";

// 6. Discard stream output:
debug_dump >> "/dev/null"; // also accepts "null" and "NUL"

// 7. Error capture and redirection:
risky_task() ?>> "errors.log";

failure: error | null = null;
read_file("missing.txt") ?>>> failure;
if failure != null {
    println("Error caught: " + failure.msg);
}
```

---

## 5. Functions, Closures, and Concurrency

### 5.1 Function Declarations
```alca
pub fn calculate_discount(price: float, rate: float = 0.1) -> float {
    return price * (1.0 - rate);
}

void log_event(event: string) {
    println("[LOG] " + event);
}
```

### 5.2 Closures and Higher-Order Functions
```alca
fn make_prefixer(prefix: string) -> func {
    fn format(msg: string) -> string {
        return prefix + ": " + msg;
    }
    return format;
}

logger := make_prefixer("[SYSTEM]");
println(logger("Startup OK")); // "[SYSTEM]: Startup OK"
```

### 5.3 Asynchronous Execution (`async` / `await`)
Both the Bytecode VM and Interpreter provide full parity for cooperative asynchronous execution:
```alca
async fn load_remote_config(url: string) -> string {
    return "{\"status\": \"ok\"}";
}

cfg := await load_remote_config("https://config.server.local");
```

### 5.4 Concurrency: Background Workers (`@ba`) and Thread Pools (`@threads`)
- `@ba` spawns an isolated background worker:
  ```alca
  ba(timeout=10s):
      println("Background synchronization completed");
  ```
- `@threads` distributes computation across system threads:
  ```alca
  threads 8:
      println("Parallel task processing");
  ```

---

## 6. Object-Oriented and Contract Programming

### 6.1 Structs
```alca
struct Point {
    x: int;
    y: int = 0;
}

pt := Point(10, 20);
pt.x = 25;
```

### 6.2 Classes and Single Inheritance
```alca
class Vehicle {
    pub brand: string;
    private mileage: int = 0;

    pub fn drive(km: int) -> void {
        self.mileage = self.mileage + km;
    }

    pub fn get_mileage() -> int {
        return self.mileage;
    }
}

class ElectricCar extends Vehicle {
    pub battery_level: int = 100;

    pub fn drive(km: int) -> void {
        super.drive(km);
        self.battery_level = self.battery_level - (km / 10);
    }
}

tesla := ElectricCar("Tesla", 0, 100);
tesla.drive(50);
assert tesla.get_mileage() == 50;
assert tesla.battery_level == 95;
```

### 6.3 Enums and Pattern Matching
```alca
enum ConnectionState {
    Disconnected,
    Connecting,
    Connected = 10,
}

state := ConnectionState.Connected;
match state {
    case ConnectionState.Disconnected:
        println("No connection");
    case ConnectionState.Connecting:
        println("Connecting...");
    case ConnectionState.Connected:
        println("Connected!");
    else:
        println("Unknown state");
}
```

### 6.4 Interfaces
```alca
interface StorageDriver {
    fn write_chunk(data: string) -> bool;
    fn read_chunk() -> string;
}

class LocalDiskStorage {
    pub fn write_chunk(data: string) -> bool {
        return true;
    }
    pub fn read_chunk() -> string {
        return "data";
    }
}

driver: StorageDriver = LocalDiskStorage();
driver.write_chunk("payload");
```

---

## 7. Error Handling and Resilience

Instead of unhandled exception unwinding, Alca uses explicit error values and stream operators.

### 7.1 Error Capture with `?>>>`
```alca
use std.io;

failure: error | null = null;
data := io.read_file("nonexistent.txt") ?>>> failure;

if failure != null {
    println("Code: " + string(failure.code));
    println("Message: " + failure.msg);
} else {
    println("Content: " + data);
}
```

### 7.2 Error Codes
- `400`: Invalid parameters / bad request.
- `403`: Sandbox access violation (Permission Denied).
- `500`: Runtime execution error.
- `507`: Memory allocation limit exceeded (OOM Guard).
- Helper: `is_permission_error(err)` checks whether an error is a sandbox denial.

---

## 8. Global Built-in Functions and System Variables

Global built-ins are accessible across the program **without importing any module**:

### 8.1 Terminal I/O
```alca
print("Processing:", 100);
println("Done!", 200, true);
name := input("Enter name: ");
```

### 8.2 Type and Size Inspection
```alca
assert len("Hello") == 5;
assert len([1, 2, 3]) == 3;
assert type(42) == "int";
assert type("text") == "string";
assert type(null) == "null";
```

### 8.3 Type Casts
```alca
assert int(4.99) == 4;
assert int("123") == 123;
assert float(10) == 10.0;
assert string(100) == "100";
assert bool(1) == true;
```

### 8.4 Collection Operations (Zero-Import)
```alca
items := [10, 20];
new_list := append(items, 30); // [10, 20, 30]
push(items, 30);                // In-place mutation
last := pop(items);             // 30
k := keys({"a": 1, "b": 2});    // ["a", "b"]
v := values({"a": 1, "b": 2});  // [1, 2]
r := range(1, 5);               // [1, 2, 3, 4, 5]
```

### 8.5 Math & String Helpers
```alca
assert abs(-50) == 50;
assert max(10, 25) == 25;
assert min(10, 25) == 10;

assert str_trim("  hello  ") == "hello";
assert str_upper("alca") == "ALCA";
assert str_lower("ALCA") == "alca";
assert str_contains("alca lang", "lang") == true;
assert str_starts_with("app.al", "app") == true;
assert str_ends_with("app.al", ".al") == true;
assert len(str_split("a,b,c", ",")) == 3;
assert str_replace("foo bar foo", "foo", "qux") == "qux bar qux";
```

### 8.6 Zero-Import JSON and `std.json`
Alca provides both zero-import global JSON helpers and a full-featured standard module `std.json`:

```alca
// Zero-import globals:
data := {"status": 200, "ok": true};
json_str := json_encode(data);
parsed := json_decode(json_str);
assert parsed["status"] == 200;
pretty := json_prettify(data);
q_res := json_query(parsed, "status");
assert q_res == 200;

// Modular standard library (use std.json):
use std.json;
doc := json.parse("{\"score\": 98}");
assert json.validate("{\"score\": 98}") == true;
compact := json.encode(doc);
formatted := json.prettify(doc, "  ");
```

### 8.7 Process and System Helpers
```alca
sleep(10); // Sleep for 10ms
ts := time_now(); // Current UTC timestamp in RFC3339 format

// Subprocess execution (requires --allow-run):
// out := exec("hostname");

// Environment variables are accessed via std.env:
use std.env;
h := env.get_env("PATH");
env.set_env("PORT", "8080");
```

### 8.8 Dunder Variables
| Variable | Type | Description |
| --- | --- | --- |
| `__file__` | `string` | Absolute path to current source script. |
| `__dir__` / `__dirname__` | `string` | Directory of current source file. |
| `__line__` | `int` | Current source line number. |
| `__name__` | `string` | Module name or `"__main__"`. |
| `__version__` | `string` | Alca runtime version string. |

### 8.9 Standard Library Namespaces (Former Global Aliases Deprecation)
To maintain strict namespace hygiene, deterministic static analysis, and capability-based security boundaries, convenience aliases are not present in global scope. Calling former global aliases triggers compile-time deprecation diagnostics pointing to their respective modules:

- **Filesystem (`use std.io;`)**: `io.read_file`, `io.write_file`, `io.append_file`, `io.file_exists`, `io.file_size`, `io.delete_file`, `io.list_dir`, `io.make_dir`.
- **Cryptography (`use std.crypto;`)**: `crypto.uuid`, `crypto.random_string`, `crypto.random_int`, `crypto.hex_encode`, `crypto.hex_decode`, `crypto.sha256`, `crypto.sha512`.
- **Process & Metrics (`use std.process;`)**: `process.cpu_count`, `process.memory_usage`, `process.uptime`, `process.exec_cmd`, `process.exec_bg`.
- **Environment (`use std.env;`)**: `env.home_dir`, `env.hostname`, `env.username`, `env.is_windows`, `env.is_linux`, `env.get_env`, `env.set_env`.

---

## 9. Capability-Based Sandbox & Security

Alca enforces explicit capability permissions when running scripts (deny-by-default):

| Flag | Meaning | Example |
| --- | --- | --- |
| `--allow-read` | Read any file on filesystem | `alca run --allow-read app.al` |
| `--allow-read=<path>` | Read within specific path subtree | `alca run --allow-read=./data app.al` |
| `--allow-write` | Write any file on filesystem | `alca run --allow-write app.al` |
| `--allow-write=<path>` | Write within specific path subtree | `alca run --allow-write=./logs app.al` |
| `--allow-net` | Connect/listen to any network socket | `alca run --allow-net app.al` |
| `--allow-net=<host>` | Access specific host/port | `alca run --allow-net=api.site.com:443 app.al` |
| `--allow-run` | Execute subprocesses (`exec`) | `alca run --allow-run app.al` |
| `--allow-run=<cmd>` | Allow specific binary name | `alca run --allow-run=git app.al` |
| `--allow-env` | Read and write environment variables | `alca run --allow-env app.al` |
| `--allow-env=<var>` | Access specific environment variable | `alca run --allow-env=PORT app.al` |
| `--allow-ffi` | Load native dynamic libraries | `alca run --allow-ffi app.al` |
| `--allow-all` | Grant all capabilities | `alca run --allow-all app.al` |

---

## 10. Standard Library Reference (Stdlib)

Import modules using `use std.<module_name>;`.

<a id="101-collections-and-data-structures"></a>
### 10.1 Collections and Data Structures (`list`, `map`, `string`)
- `std.list`: `list.len`, `list.append`, `list.sort`, `list.reverse`, `list.any`, `list.all`.
- `std.map`: `map.has_key`, `map.delete_key`, `map.keys`, `map.values`.
- `std.string`: `str.contains`, `str.starts_with`, `str.ends_with`, `str.lower`, `str.upper`, `str.trim`, `str.split`, `str.join`, `str.replace`.

<a id="102-io-and-filesystem"></a>
### 10.2 I/O and Filesystem (`io`, `path`, `archive`, `compress`)
- `std.io`: `io.read_file`, `io.write_file`, `io.append_file`, `io.file_exists`, `io.file_size`, `io.delete_file`, `io.make_dir`, `io.list_dir`.
- `std.path`: `path.join`, `path.base`, `path.dir`, `path.ext`, `path.abs`.
- `std.archive` / `std.compress`: `archive.zip`, `archive.unzip` (with Zip-Slip path sanitization), `compress.gzip`, `compress.gunzip`.

<a id="103-networking-and-web"></a>
### 10.3 Networking and Web (`net`, `http`, `urllib2`, `mtproto`)
- `std.http`: HTTP server, routing (`router.get`, `router.post`), middleware, JSON responses.
- `std.urllib2`: Client HTTP requests (`urllib2.get`, `urllib2.post`), WebSocket support.
- `std.mtproto`: Telegram MTProto v2.0 protocol layer, Diffie-Hellman key exchange, AES-IGE encryption, TL serialization. See [stdlib_EN.md](stdlib_EN.md#mtproto).

<a id="104-cryptography-and-security"></a>
### 10.4 Cryptography and Security (`crypto`)
- `crypto.sha256`, `crypto.sha512`, `crypto.hmac_sha256`, `crypto.constant_time_equal`, `crypto.base64_encode`, `crypto.base64_decode`, `crypto.hex_encode`, `crypto.hex_decode`, `crypto.uuid`, `crypto.random_string`, `crypto.random_int`.

<a id="105-data-formats-and-configuration"></a>
### 10.5 Data Formats and Configuration (`json`, `csv`, `ini`, `marshal`)
- `std.json`: `json.read(str)`, `json.format(obj)`.
- `std.csv`: `csv.read(path_or_str)`, `csv.write(table)`.
- `std.ini`: `ini.read(path_or_str)`.

<a id="106-date-time-and-benchmarking"></a>
### 10.6 Date, Time, and Benchmarking (`time`)
- `time.now()`, `time.now_ms()`, `time.format(ts, layout, tz)`, `time.parse(str, layout, tz)`, `time.date(ts, tz)`, `time.diff(t1, t2)`, `time.sleep(ms)`.

<a id="107-mathematics-and-computation"></a>
### 10.7 Mathematics, Deep Learning, and Computation (`math`, `libtorch`)
- `std.math`: `math.PI`, `math.E`, `math.sin`, `math.cos`, `math.tan`, `math.sqrt`, `math.pow`, `math.round`, `math.floor`, `math.ceil`, `math.abs`. Vectors, matrices, and quaternions (`math_vector`, `math_matrix`, `math_quat`).
- `std.libtorch`: 85 tensor and neural network functions, N-dimensional tensors with OOP method chaining (`x.matmul(w).relu()`), reverse-mode Autograd, backpropagation, and optional CUDA hardware acceleration. See [stdlib_EN.md](stdlib_EN.md#libtorch).

<a id="108-system-processes-and-environment"></a>
### 10.8 System Processes and Environment (`sys`, `process`, `env`, `cli`)
- `sys.os()`, `sys.arch()`, `env.get()`, `env.set()`, `process.run()`, `cli.args()`.

<a id="109-native-integration-and-os"></a>
### 10.9 Native Integration (FFI) and OS (`ffi`, `win32`, `devices`, `power`)
- `ffi.open(lib_path)`, `ffi.call(symbol, ...args)`, `std.win32`, `std.devices`, `std.power`.

<a id="1010-multimedia-and-graphical-ui"></a>
### 10.10 Multimedia and Graphical UI (`gui`, `audio`, `image`)
- `std.gui`: Native windowing, buttons, canvas, WebView container.
- `std.audio`: Waveform synthesis (sine, square, sawtooth, noise), ADSR envelopes, FFT analysis, WAV playback.
- `std.image`: Shape drawing, geometric rendering, image loading/saving.

<a id="1011-databases-logging-and-telemetry"></a>
### 10.11 Databases, Logging, and Telemetry (`db`, `log`, `metrics`)
- `db.open("sqlite", "file.db")`: SQLite transactions and queries.
- `log.info`, `log.warn`, `log.error`: Structured logging.
- `metrics.counter`, `metrics.gauge`: Application performance telemetry.

---

## 11. Build, Distribution, and Testing

<a id="111-execution--type-check"></a>
### 11.1 Execution & Type Check
```bash
alca run --allow-all app.al
alca check app.al # Strict syntax and type checker (CI)
```

<a id="112-unit-tests"></a>
### 11.2 Unit Tests
```bash
alca test tests/test_01_strings_text.al
```

Writing test blocks:
```alca
test "Core business logic":
    assert 2 + 2 === 4;
    assert "alca" + "_lang" == "alca_lang";
```

<a id="113-standalone-executables-windows-peexe"></a>
### 11.3 Standalone Executables (Windows PE/EXE)
```bash
alca build main.al -o myapp.exe --protect --gui
```

<a id="114-native-dynamic-libraries-dll"></a>
### 11.4 Native Dynamic Libraries (DLL)
```bash
alca dll lib.al -o mylib.dll
```

<a id="115-package-management-apm"></a>
### 11.5 Package Management (APM)
Alca includes the official package manager **APM** (`apm`) to manage dependencies, lockfiles, and registry publishing:
```bash
apm init                       # Initialize alca.toml manifest
apm install pkg_name@^1.2.0    # Install package with SemVer resolution
apm publish                    # Publish archive to registry
```
See the full guide in [apm_EN.md](apm_EN.md).

<a id="116-ide--editor-integration-lsp"></a>
### 11.6 IDE & Editor Integration (LSP)
Alca features a built-in Language Server Protocol daemon and editor extensions:
```bash
alca lsp                       # Run LSP server on stdio
```
First-party editor packages are provided in `editors/vscode` (VS Code & Cursor) and `editors/zed` (Zed Editor). See the full setup instructions in [editors_EN.md](editors_EN.md).
