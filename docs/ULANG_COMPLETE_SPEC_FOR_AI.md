# ULang++ 2.4 Complete Language & Architecture Specification (LLM Reference)

> **Target Audience: Large Language Models (LLMs), AI code assistants, compiler engineers.**
> **Language Name:** ULang++ (version 2.4)
> **Primary Executable:** `u++` (alias `ulang++`)
> **Source File Extension:** `.upp`
> **Dynamic Library Extension:** `.dll` (PE x64 on Windows) / `.so` (ELF on Linux)
> **Default Custom Libs Location:** `ProgramData/ULangLibs/libs/` (Windows) or `/var/lib/ULangLibs/libs/` (Linux)

---

## 1. Quick Syntax & Grammar Cheat Sheet for Code Generation

### 1.1 Program Entry & Statements
- Files have `.upp` extension.
- Functions are declared using `func name(params) -> return_type { ... }` or `pub func ...` for module exports.
- Top-level executable scripts can have loose statements or a `func main()`.
- Variables are declared with `:=` (type-inferred assignment) or assigned with `=`.
- Dynamic and loose equality (`==` for structural/value comparison, `===` for strict reference/identity equality).

### 1.2 Primitive Types
- `int` (64-bit signed integer)
- `bool` (`true`, `false`)
- `string` (UTF-8 immutable byte slice with length)
- `list[T]` (resizable dynamic array)
- `map[K, V]` (hash map)
- `void` (unit / no value)

### 1.3 Control Flow
- `if condition { ... } else if condition { ... } else { ... }`
- `while condition { ... }`
- `loop { ... break ... }`
- `for item in collection { ... }`
- Pattern matching: `match val { pattern => { ... }, _ => { ... } }`

### 1.4 Module System & Imports
- `use std.io` - Automatically searches adjacent compiler `result/` and system `ULangLibs` directories.
- `use mymodule` - Imports `mymodule.upp` in the local directory or custom library path.
- `use "./relative/path/mod.upp"` - Explicit relative path.

### 1.5 Low-Level Bare-Metal & OS Development Features
- `std.cpu`: `cpu.in8(port)`, `cpu.out8(port, val)`, `cpu.cli()`, `cpu.sti()`, `cpu.hlt()`.
- `std.mem`: `mem.read_u8(addr, offset)`, `mem.write_u8(addr, offset, val)`, `mem.write_u16(...)`.
- VGA buffer base: `0xB8000` (753664). COM1 base port: `0x3F8` (1016).

---

## 2. CLI Toolchain Reference (`u++` / `ulang++`)
```bash
u++ build [-o <out>] [--target <exe|dll>] <file.upp>  # Compile directly to native binary
u++ run <file.upp>                                   # Compile and immediately execute
u++ check <file.upp>                                 # Typecheck and validate AST
u++ parse <file.upp>                                 # Dump AST structure
u++ lex <file.upp>                                   # Tokenize input
u++ ir <file.upp>                                    # Emit low-level intermediate representation
u++ version                                          # Output "ULang++ 2.4"
```

---



## --- Documentation Section: language_EN.md ---


# ulang++ Language Specification & Reference

ulang++ is a statically typed, memory-safe systems automation language. It combines clean scripting syntax with static analysis, fast bytecode VM execution, multi-tier JIT acceleration, and capability-based sandboxing.

---

## 1. Program Structure & Syntax

A program is a sequence of statements.
- **Statement Termination**: Statements terminate with a semicolon (`;`).
- **Block Enclosure**: Blocks may be enclosed either in canonical curly braces (`{ ... }`) or with a colon (`:`) followed by an indented suite.
- **Comments**: Single-line comments start with `//`. Multi-line comments use `/* ... */`.

```ulang++
use std.math

// Canonical brace syntax
fn calculate(r: float) -> float {
    if r <= 0.0 {
        return 0.0
    }
    return math.PI * r * r
}

// Colon and indentation syntax is also supported
fn greeting(name: string) {
    if len(name) == 0 {
        println("Hello, Guest!")
    }
    else {
        println("Hello, " + name + "!")
    }
}
```

### Module Imports
Modules are imported using `use`:
- `use std.math;` — imports a standard library module under its namespace `math`.
- `use "./utils.upp";` — imports a relative local file.
- All imported declarations are statically checked against the importing program.

---

## 2. Type System

ulang++ enforces **strict static typing** with bidirectional inference. Implicit narrowing, widening, or silent coercion is strictly prohibited.

### Built-in Types
- **Primitive types**: `int`, `float`, `string`, `bool`
- **Unit type**: `void`
- **Dynamic escape hatch**: `json` (the only dynamically typed value; requires conversion before use in typed contexts)
- **Nullability**: `T | null` (values of type `T` cannot be `null` unless explicitly typed as nullable)
- **Collections**: `list[T]`, `map[K, V]`
- **Functions**: `func`
- **Errors**: `error`
- **User-defined**: `struct`, `class`, `enum`, `interface`

### Variable Declarations & Immutability
```ulang++
// Explicit typing
count: int = 10
price: float = 99.95
name: string = "ulang++"
active: bool = true

// Type inference (inferred once at compile time, never becomes dynamic)
ratio := 1.75             // float
items := ["apple", "pear"] // list[string]

// Shadowing is rejected at compile time:
// count: int = 20;        // ERROR: variable 'count' already declared
count = 20                // OK: variable assignment
```

### Strict Type Arithmetic
Numbers do not coerce silently:
```ulang++
a: int = 5
b: float = 2.5

// c := a + b;             // ERROR: operator '+' cannot be applied to int and float
c := float(a) + b         // OK: 7.5
quotient := float(a) / 2.0// Float division: 2.5
int_div := a / 2          // Integer division truncates towards zero: 2
```

### Null-Safety & Type Narrowing
```ulang++
maybe_user: string | null = null

// Property access directly on nullable types is rejected by type checker
if maybe_user != null {
    // Within this block, maybe_user is safely narrowed to string
    println("User is: " + maybe_user)
}
```

---

## 3. Operators

### Arithmetic & Strings
- `+`, `-`, `*`, `/`, `%`, `**` (exponentiation).
- `+` on strings performs concatenation.

### Comparisons & Identity
- Value equality: `==` and `!=` perform structural value comparison. Comparing incompatible types (e.g. `"1" == 1`) is rejected at compile time.
- Identity: `===` and `!==` perform strict reference/pointer identity checks.
- Ordering: `<`, `<=`, `>`, `>=`.

### Boolean Logic
- Logical AND: `and` or `&&`
- Logical OR: `or` or `||`
- Logical NOT: `not` or `!`

### Ranges
- Half-open range: `a..b` (`[a, b)`): iterates from `a` up to `b - 1` (e.g., `0..3` produces `0, 1, 2`).
- Inclusive range: `a..=b` (`[a, b]`): iterates from `a` up to and including `b` (e.g., `0..=3` produces `0, 1, 2, 3`).

### Functional Pipelines (`|>`)
The forward pipe operator `x |> f(...)` evaluates `x` and passes it as the first argument to `f(...)`:
```ulang++
val := 10 |> double |> add(5) // add(double(10), 5) -> 25
```

### Stream Pipelines (`>>`, `>!`, `>>>`, `<<`, `&>>`, `?>>`, `?>>>`)
ulang++ provides native stream operators for piping data between variables, files, network sockets, and consoles:
- `data >> "file.log"`: Append content to file or POST payload to URL (`STREAM_APPEND`).
- `data >! "file.txt"`: Overwrite file with content (`STREAM_OVERWRITE`).
- `expr >>> var_name`: Stream result directly into an environment variable (`STREAM_VAR`).
- `var_name << "file.txt"` or `var_name << console`: Stream read from file or stdin (`STREAM_IN`).
- `data &>> "file.log"`: Non-blocking asynchronous background stream append (`STREAM_ASYNC`).
- `data &&>> "https://..."`: Asynchronous background stream to network socket (`STREAM_THREAD`).
- `expr ?>> "errors.log"`: Stream error details to log file if `expr` produces an error.
- `expr ?>>> err_var`: Capture runtime error into variable of type `error | null` without throwing.
- Discarding output: piping to `null`, `"/dev/null"`, or `"NUL"` drops stream output silently.

### Membership
- `item in collection` evaluates whether `item` is an element of a list, map key, or substring.

---

## 4. Control Flow

### If / Elif / Else
Conditions must strictly evaluate to `bool` (no "truthy/falsy" values):
```ulang++
if score >= 90 {
    println("Grade: A")
} elif score >= 75 {
    println("Grade: B")
} else {
    println("Grade: C")
}
```

### Loops: For & While
```ulang++
// Iterating ranges
for i in 0..5 {
    if i == 2 {
        continue
    }
    println(i)
}

// Iterating collections
names := ["Alice", "Bob", "Charlie"]
for name in names {
    println("Hello, " + name)
}

// While loop
counter := 3
while counter > 0 {
    counter = counter - 1
}
```

---

## 5. Functions, Closures, and Async/Await

### Function Signatures
Functions can be declared using `fn` or `func`:
```ulang++
pub fn multiply(x: int, y: int = 1) -> int {
    return x * y
}

// Void functions (no return value)
void log_info(msg: string) {
    println("[INFO] " + msg)
}
```

### First-Class Functions and Closures
```ulang++
fn make_adder(base: int) -> func {
    fn add(n: int) -> int {
        return base + n
    }
    return add
}

add10 := make_adder(10)
assert add10(5) == 15
```

### Cooperative Async / Await Parity
Both the Bytecode VM and reference Interpreter provide complete parity for `async` function definitions and `await` expressions:
```ulang++
async fn fetch_status(endpoint: string) -> string {
    return "200 OK from " + endpoint
}

status := await fetch_status("https://api.ulang++.org")
assert status == "200 OK from https://api.ulang++.org"
```

### Background Execution (`@ba`, `@threads`)
- Spawning background worker tasks:
```ulang++
ba(timeout=5s) {
    println("Background calculation running")
}
```
- Multi-threaded execution pools:
```ulang++
threads 4 {
    println("Running across worker pool")
}
```

---

## 6. Structs, Classes, and Interfaces

### Structs (Lightweight Value Types)
```ulang++
struct Point {
    x: int
    y: int = 0
}

p := Point(10, 20)
p.x = 15
assert p.x == 15
```

### Classes and Single Inheritance
```ulang++
class Account {
    pub owner: string
    private balance: float = 0.0

    pub fn deposit(amount: float) -> void {
        self.balance = self.balance + amount
    }

    pub fn get_balance() -> float {
        return self.balance
    }
}

class PremiumAccount extends Account {
    pub bonus_rate: float = 0.05

    pub fn deposit_with_bonus(amount: float) -> void {
        bonus := amount * self.bonus_rate
        super.deposit(amount + bonus)
    }
}

acc := PremiumAccount("Alice", 100.0, 0.1)
acc.deposit_with_bonus(100.0)
assert acc.get_balance() == 210.0
```

### Enums & Pattern Matching
```ulang++
enum OrderStatus {
    Pending,
    Processing = 1,
    Shipped,
    Delivered,
}

status := OrderStatus.Shipped
match status {
    case OrderStatus.Pending {
        println("Awaiting payment")
    }
    case OrderStatus.Shipped {
        println("Item is in transit")
    }
    else {
        println("Status updated")
    }
}
```

### Interfaces (Structural Polymorphism)
```ulang++
interface Notifier {
    fn notify(msg: string) -> bool
}

class EmailSender {
    pub fn notify(msg: string) -> bool {
        println("Email sent: " + msg)
        return true
    }
}

sender: Notifier = EmailSender()
sender.notify("System alert")
```

---

## 7. Error Handling & Stream Pipelines

ulang++ uses explicit error values instead of uncontrolled stack unwinding.

### Error Creation & Capture
- Use `err(message, code=...)` to raise an error value.
- Use `expr ?>>> target` to capture errors into an `error | null` variable without aborting the program:

```ulang++
use std.io

failure: error | null = null
content := io.read_file("nonexistent.log") ?>>> failure

if failure != null {
    println("Failed with code: " + string(failure.code))
    println("Message: " + failure.msg)
}
```

### Error Stream Redirection
- `expr ?>> "errors.log"` redirects runtime error output directly to a file or stream.
- Built-in error inspection functions: `is_permission_error(e)`, `io.is_permission_error(e)`.

---

## 8. Global Built-in Functions & Variables

Global built-ins are always available without importing any module:

### I/O & Inspection
- `print(...args) -> string`: prints arguments separated by spaces.
- `println(...args) -> string`: prints arguments with trailing newline.
- `input(prompt?: string) -> string`: reads line from standard input.
- `len(x) -> int`: rune length of strings, elements count in lists/maps, byte count in blobs.
- `type(x) -> string` / `typeof(x) -> string`: returns type name string (`"int"`, `"float"`, `"string"`, `"bool"`, `"list"`, `"map"`, `"func"`, `"null"`).

### Explicit Type Constructors
- `int(x) -> int`, `float(x) -> float`, `string(x) -> string`, `bool(x) -> bool`, `list(x) -> list`.

### Collection Operations
- `append(list, elem) -> list`: returns new list with appended element.
- `push(list, elem) -> list`: in-place element push.
- `pop(list) -> any`: pops and returns last element (or `null` if empty). Also available under `use std.list; list.pop(list)`.
- `keys(map) -> list[string]`: list of dictionary keys.
- `values(map) -> list[any]`: list of dictionary values.
- `range(start, end, step? = 1) -> list[int]`: list of integers from `start` to `end`.

### Math & Strings
- `abs(x)`, `min(a, b)`, `max(a, b)`.
- `str_trim(s)`, `str_upper(s)`, `str_lower(s)`, `str_contains(s, sub)`, `str_starts_with(s, prefix)`, `str_ends_with(s, suffix)`, `str_split(s, sep)`, `str_replace(s, old, new)`.

### JSON Helpers (Zero-import)
- `json_encode(val) -> string`: serializes a ulang++ value to compact JSON.
- `json_decode(str) -> json`: parses a JSON string into a dynamic `json` value.
- `json_prettify(val) -> string`: formats a ulang++ value as indented JSON.
- `json_query(json, query) -> json`: queries a JSON document using dot/index path notation.
*(For modular access, `use std.json;` provides `json.parse`, `json.encode` / `json.stringify`, `json.prettify`, `json.query`, and `json.validate`.)*

### Process Execution
- `sleep(ms: int | float)`: pause current thread.
- `exit(code?: int)`: exit program with status code.
- `time_now() -> string`: returns current UTC timestamp in RFC3339 format.
- `exec(cmd_str) -> string`: executes host shell command (requires `--allow-run`).

> [!NOTE]
> Environment access is not global; import `std.env` and call `env.get_env(name)` and `env.set_env(name, val)`.
> Former global aliases (such as `read_file`, `write_file`, `file_exists`, `uuid`, `sha256`, `cpu_count`, etc.) have been removed to preserve strict module boundaries and security capabilities; import them explicitly via `use std.io;`, `use std.crypto;`, `use std.process;`, and `use std.env;`. Calling removed aliases produces compile-time deprecation diagnostics.

### Special Runtime Variables (Dunders)
- `__file__`: absolute path to current source file.
- `__dir__` / `__dirname__`: directory of current file.
- `__line__`: current source line number.
- `__name__`: module name or `"__main__"`.
- `__version__`: ulang++ compiler/runtime version string.

---

## 9. Capability-Based Sandboxing

ulang++ scripts run by default in a non-permissive sandbox. Operations requiring elevated host resources must be explicitly enabled at invocation:

- Filesystem: `--allow-read[=<path>]`, `--allow-write[=<path>]`
- Network: `--allow-net[=<host[:port]>]`
- Subprocesses: `--allow-run[=<cmd>]`
- Environment: `--allow-env[=<var>]`
- Native FFI: `--allow-ffi[=<lib>]`
- Full access: `--allow-all`

Attempting an unpermitted operation returns an `error` with code `403` and records an event in the security audit log.



## --- Documentation Section: stdlib_EN.md ---


# Standard Library Reference (Stdlib)

The ulang++ Standard Library provides built-in modules designed for high-performance system automation, networking, cryptography, formats, I/O, audio/GUI, and machine learning.

> 📘 **Looking for comprehensive guides and usage examples?**  
> See the [**English Handbook**](handbook_EN.md) or [**Русская методичка**](handbook_RU.md) for conceptual explanations, architectural patterns, and real-world examples.

## Table of Contents
1. [Module Import & Sandboxing](#module-import--sandboxing)
2. [Standard Library Modules](#modules)
3. [Global Builtins & Deprecations](#global-builtins)
4. [JIT and Standalone Execution](#jit-and-standalone-execution)
5. [Capability Policy](#capability-policy)

## Module Import & Sandboxing

Import a module with `use std.<name>;`, then call its members through the module namespace:

```ulang++
use std.io

text := io.read_file("input.txt")
println(text)
```

### Sandbox Capabilities Matrix

By default, untrusted scripts run in a restricted environment. Standard library modules verify capabilities via `pkg/sandbox`:

| Module | Required Capability Flag | Behavior when Denied |
| --- | --- | --- |
| `std.io`, `std.archive` (read) | `--allow-read[=<path>]` | Returns `error` with code `403` |
| `std.io`, `std.archive` (write) | `--allow-write[=<path>]` | Returns `error` with code `403` |
| `std.net`, `std.http`, `std.urllib2` | `--allow-net[=<host[:port]>]` | Returns `error` with code `403` |
| `std.process` | `--allow-run[=<cmd>]` | Returns `error` with code `403` |
| `std.env` | `--allow-env[=<var>]` | Returns `error` with code `403` |
| `std.ffi` | `--allow-ffi[=<lib>]` | Returns `error` with code `403` |
| Any module | `--allow-all` | Unconditionally grants all operations |

`docs/language.md` covers the type system. The canonical signature table is
[`pkg/typecheck/builtins_table.go`](../pkg/typecheck/builtins_table.go); this
page mirrors it. Signature notation:

- `(args) -> result` describes the accepted overloads, separated by `;`.
- `?` marks an optional argument, and `...` marks a variadic argument.
- An empty signature is a dynamic member whose arguments are not checked
  and whose result is `json`.

Aliases: `std.regex` is `std.re`, `std.str` is `std.string`, and `std.zip` is
`std.archive`. The `csv`, `ini`, and `json` modules are also available through
`std.formats`.

## Modules

### `audio`

Low-level audio processing, DSP synthesis, waveform manipulation, analysis, and WAV encoding/decoding.

Audio buffers support method-based inspection and sample manipulation (`channels()`, `sample_rate()`, `num_samples()`, `duration()`, `sample(ch, idx)`, `set_sample(ch, idx, val)`, `channel_data(ch)`).

| Member | Signature |
| --- | --- |
| `audio.create_buffer` | `(int?,int?,int?)->AudioBuffer` |
| `audio.read` | `(string\|blob)->AudioBuffer` |
| `audio.read_wav` | `(string)->AudioBuffer` |
| `audio.write_wav` | `(string,AudioBuffer,int?)->bool` |
| `audio.encode_wav` | `(AudioBuffer,int?)->string` |
| `audio.decode` | `(string\|blob)->AudioBuffer` |
| `audio.decode_wav` | `(string)->AudioBuffer` |
| `audio.decode_mp3` | `(string\|blob)->AudioBuffer` |
| `audio.encode_base64` | `(AudioBuffer,int?)->string` |
| `audio.decode_base64` | `(string)->AudioBuffer` |
| `audio.sine` | `(float,float,int?,float?)->AudioBuffer` |
| `audio.square` | `(float,float,int?,float?,float?)->AudioBuffer` |
| `audio.sawtooth` | `(float,float,int?,float?)->AudioBuffer` |
| `audio.triangle` | `(float,float,int?,float?)->AudioBuffer` |
| `audio.noise` | `(float,string?,int?,float?)->AudioBuffer` |
| `audio.silence` | `(float,int?,int?)->AudioBuffer` |
| `audio.chirp` | `(float,float,float,int?,float?)->AudioBuffer` |
| `audio.gain` | `(AudioBuffer,float)->AudioBuffer` |
| `audio.normalize` | `(AudioBuffer,float?)->AudioBuffer` |
| `audio.mix` | `(AudioBuffer,AudioBuffer,float?,float?)->AudioBuffer` |
| `audio.pan` | `(AudioBuffer,float)->AudioBuffer` |
| `audio.delay` | `(AudioBuffer,float?,float?,float?)->AudioBuffer` |
| `audio.lowpass` | `(AudioBuffer,float,float?)->AudioBuffer` |
| `audio.highpass` | `(AudioBuffer,float,float?)->AudioBuffer` |
| `audio.adsr` | `(AudioBuffer,float,float,float,float)->AudioBuffer` |
| `audio.fft` | `(AudioBuffer,int?,int?)->list[float]` |
| `audio.dominant_frequency` | `(AudioBuffer,int?)->float` |
| `audio.rms` | `(AudioBuffer,int?)->float` |
| `audio.peak` | `(AudioBuffer,int?)->float` |
| `audio.zero_crossings` | `(AudioBuffer,int?)->int` |
| `audio.play` | `(AudioBuffer\|string,bool?)->bool` |
| `audio.play_base64` | `(string,bool?)->bool` |
| `audio.pause` | `()->null` |
| `audio.resume` | `()->null` |
| `audio.volume` | `(int)->null` |
| `audio.stop` | `()->null` |
| `audio.beep` | `(int?,int?)->null` |

### `encoding`

Comprehensive text encoding/decoding module supporting all major character sets (UTF-8, UTF-16, Windows-1250..1258, CP866, CP437, ISO-8859-1..16, KOI8-R, KOI8-U, Shift-JIS, GBK, GB18030, Big5, EUC-JP, EUC-KR).

| Member | Signature |
| --- | --- |
| `encoding.decode` | `(string,string)->string` |
| `encoding.detect` | `(string)->string` |
| `encoding.encode` | `(string,string)->string` |
| `encoding.from_utf16` | `(list[int])->string` |
| `encoding.list_encodings` | `()->list[string]` |
| `encoding.to_utf16` | `(string)->list[int]` |

### `marshal`

Binary object serialization for fast, compact data interchange and caching of arbitrary ulang++ values (null, bool, int, float, string, blob, list, map, error).

| Member | Signature |
| --- | --- |
| `marshal.dump` | `(json,string)->bool` |
| `marshal.dumps` | `(json)->string` |
| `marshal.load` | `(string)->json` |
| `marshal.loads` | `(string)->json` |

### `image`

Universal graphics processing supporting PNG, JPEG, GIF, BMP, and WebP from files, URLs, base64 strings, or raw bytes.

Image objects support method-based manipulation:
* `.width()`, `.height()`, `.bounds()`
* `.pixel(x, y)` -> `[r, g, b, a]`
* `.set_pixel(x, y, r, g, b, a?)`
* `.resize(w, h)`, `.crop(x, y, w, h)`, `.rotate(deg)`, `.flip_h()`, `.flip_v()`
* `.grayscale()`, `.invert()`, `.brightness(delta)`, `.contrast(factor)`
* `.to_base64(format?)`

| Member | Signature |
| --- | --- |
| `image.create` | `(int,int,list[int]?)->Image` |
| `image.read` | `(string\|blob)->Image` |
| `image.decode` | `(string\|blob)->Image` |
| `image.write` | `(Image,string,int?)->bool` |
| `image.encode` | `(Image,string?,int?)->string` |
| `image.to_base64` | `(Image,string?)->string` |
| `image.screenshot` | `(int?,int?,int?,int?)->Image` |

### `devices`

Device enumeration is platform-specific. `devices.list` returns available
devices; `devices.get` looks up a device by its platform identifier.
`devices.watch_events` is not implemented and returns an unsupported-operation
error.

| Member | Signature |
| --- | --- |
| `devices.get` | `(string)->map[string, json]` |
| `devices.list` | `(string?)->list[map[string, json]]` |
| `devices.watch_events` | `(...)->error` |

### `platform` [Cross-platform]

Operating-system identity, architecture, and detected platform capabilities across Windows, Linux, and macOS.
The capability values reflect current probes where possible; `requires_root`
is true when the current process is not elevated and some privileged operations
may be unavailable.

| Member | Signature |
| --- | --- |
| `platform.api_version` | `()->string` |
| `platform.architecture` | `()->map[string, json]` |
| `platform.capabilities` | `()->map[string, json]` |
| `platform.has_feature` | `(string)->bool` |
| `platform.info` | `()->map[string, json]` |
| `platform.linux_distro` | `()->map[string, json]` |
| `platform.mac_ver` | `()->map[string, json]` |
| `platform.machine` | `()->string` |
| `platform.node` | `()->string` |
| `platform.processor` | `()->string` |
| `platform.release` | `()->string` |
| `platform.system` | `()->string` |
| `platform.uname` | `()->list[string]` |
| `platform.version` | `()->string` |
| `platform.win32_ver` | `()->list[json]` |

### `power` [Cross-platform]

Battery and sensor information and operating-system power controls on Windows, Linux, and macOS. The
`suspend`, `hibernate`, `reboot`, and `shutdown` operations require
`--allow-all`; availability and returned sensor data depend on the host OS and
hardware (e.g. ACPI/WMI on Windows, `/sys/class/power_supply` on Linux, IOKit on macOS).

| Member | Signature |
| --- | --- |
| `power.battery_info` | `()->map[string, json]` |
| `power.battery_status` | `()->map[string, json]` |
| `power.cpu_temp` | `()->list[float]` |
| `power.hibernate` | `()->bool` |
| `power.power_plugged` | `()->bool` |
| `power.power_source` | `()->string` |
| `power.reboot` | `()->bool` |
| `power.sensors` | `()->list[map[string, json]]` |
| `power.shutdown` | `()->bool` |
| `power.suspend` | `()->bool` |

### `win32` [Windows only]

Windows-specific registry, service, and WMI helpers. Calls on non-Windows
systems return an unsupported-platform error. Operations that change registry
values or service state require `--allow-all`. Registry access and WMI queries
also require `--allow-all`; `reg_open_key(hive, path, access?)` defaults to
read access and accepts `read` or `write`.

| Member | Signature |
| --- | --- |
| `win32.edition` | `()->string` |
| `win32.license_status` | `()->map[string, json]` |
| `win32.reg_close_key` | `(int)->bool` |
| `win32.reg_enum_keys` | `(int)->list[string]` |
| `win32.reg_open_key` | `(string,string,string?)->int` |
| `win32.reg_read_value` | `(int,string)->json` |
| `win32.reg_write_value` | `(int,string,string \| int,string?)->bool` |
| `win32.service_info` | `(string)->map[string, json]` |
| `win32.service_list` | `()->list[map[string, json]]` |
| `win32.service_start` | `(string)->bool` |
| `win32.service_stop` | `(string)->bool` |
| `win32.version_info` | `()->map[string, json]` |
| `win32.wmi_get_class` | `(string)->list[map[string, json]]` |
| `win32.wmi_query` | `(string)->list[map[string, json]]` |

### `libtorch`

Unified high-performance machine learning and multidimensional tensor framework. Powered by an internal computational graph engine with dynamic automatic differentiation (Autograd), CUDA GPU acceleration, neural network layers, loss functions, and optimizers.

#### Module Functions

| Member | Signature | Description |
| --- | --- | --- |
| `libtorch.tensor` | `(any,list[int]?)->Tensor` | Creates tensor from nested lists/numbers with shape |
| `libtorch.zeros` | `(list[int])->Tensor` | Tensor filled with zeros |
| `libtorch.ones` | `(list[int])->Tensor` | Tensor filled with ones |
| `libtorch.rand` | `(list[int])->Tensor` | Uniform random values in [0, 1) |
| `libtorch.randn` | `(list[int])->Tensor` | Standard normal distribution (mean 0, variance 1) |
| `libtorch.full` | `(list[int],float)->Tensor` | Tensor filled with scalar value |
| `libtorch.eye` | `(int,int?)->Tensor` | 2D identity matrix |
| `libtorch.arange` | `(float,float,float?)->Tensor` | Values with step from start to end |
| `libtorch.matrix` | `(list[list[float]])->Tensor` | 2D matrix tensor constructor |
| `libtorch.add` | `(Tensor,Tensor)->Tensor` | Elementwise addition |
| `libtorch.sub` | `(Tensor,Tensor)->Tensor` | Elementwise subtraction |
| `libtorch.mul` | `(Tensor,Tensor)->Tensor` | Elementwise multiplication |
| `libtorch.div` | `(Tensor,Tensor)->Tensor` | Elementwise division |
| `libtorch.matmul` | `(Tensor,Tensor)->Tensor` | Matrix multiplication (2D / batched) |
| `libtorch.dot` | `(Tensor,Tensor)->float` | Vector dot product |
| `libtorch.pow` | `(Tensor,float)->Tensor` | Elementwise power exponentiation |
| `libtorch.abs` | `(Tensor)->Tensor` | Absolute value |
| `libtorch.neg` | `(Tensor)->Tensor` | Elementwise negation |
| `libtorch.exp` | `(Tensor)->Tensor` | Exponential function |
| `libtorch.log` | `(Tensor)->Tensor` | Natural logarithm |
| `libtorch.sqrt` | `(Tensor)->Tensor` | Square root |
| `libtorch.sum` | `(Tensor,int?)->Tensor` | Sum reduction over dimension |
| `libtorch.mean` | `(Tensor,int?)->Tensor` | Arithmetic mean reduction |
| `libtorch.max` | `(Tensor,int?)->Tensor` | Maximum value reduction |
| `libtorch.min` | `(Tensor,int?)->Tensor` | Minimum value reduction |
| `libtorch.relu` | `(Tensor)->Tensor` | Rectified Linear Unit activation |
| `libtorch.leaky_relu` | `(Tensor,float?)->Tensor` | Leaky ReLU with slope |
| `libtorch.sigmoid` | `(Tensor)->Tensor` | Logistic sigmoid activation |
| `libtorch.tanh` | `(Tensor)->Tensor` | Hyperbolic tangent activation |
| `libtorch.softmax` | `(Tensor,int?)->Tensor` | Softmax probability distribution |
| `libtorch.log_softmax` | `(Tensor,int?)->Tensor` | Log-Softmax normalized probabilities |
| `libtorch.linear` | `(Tensor,Tensor,Tensor?)->Tensor` | Affine transformation: $xW^T + b$ |
| `libtorch.dense_forward`| `(Tensor,Tensor,Tensor,string?)->Tensor` | Forward pass with fused activation |
| `libtorch.conv2d` | `(Tensor,Tensor,Tensor?,int?,int?)->Tensor` | 2D spatial convolution |
| `libtorch.max_pool2d` | `(Tensor,int,int?)->Tensor` | 2D max pooling |
| `libtorch.batch_norm_2d`| `(Tensor,Tensor,Tensor,Tensor,Tensor,bool?,float?,float?)->Tensor` | 2D batch normalization |
| `libtorch.layer_norm` | `(Tensor,list[int],Tensor?,Tensor?,float?)->Tensor` | Layer normalization |
| `libtorch.dropout` | `(Tensor,float?,bool?)->Tensor` | Random dropout regularization |
| `libtorch.mse_loss` | `(Tensor,Tensor)->Tensor` | Mean Squared Error loss |
| `libtorch.cross_entropy`| `(Tensor,Tensor)->Tensor` | Categorical cross entropy loss |
| `libtorch.bce_with_logits`| `(Tensor,Tensor)->Tensor` | Binary cross entropy with logits |
| `libtorch.l1_loss` | `(Tensor,Tensor)->Tensor` | Mean absolute error (L1) loss |
| `libtorch.backward` | `(Tensor)->bool` | Executes backpropagation autograd pass |
| `libtorch.grad` | `(Tensor)->Tensor` | Retrieves accumulated gradient tensor |
| `libtorch.requires_grad`| `(Tensor,bool)->Tensor` | Sets gradient tracking flag |
| `libtorch.detach` | `(Tensor)->Tensor` | Detaches tensor from computation graph |
| `libtorch.zero_grad` | `(Tensor)->bool` | Clears accumulated gradient |
| `libtorch.adam` | `(list[Tensor],float?,float?,float?)->Optimizer` | Adam adaptive optimizer |
| `libtorch.adamw` | `(list[Tensor],float?,float?,float?,float?)->Optimizer` | AdamW with decoupled weight decay |
| `libtorch.sgd` | `(list[Tensor],float?,float?,float?)->Optimizer` | Stochastic Gradient Descent |
| `libtorch.rmsprop` | `(list[Tensor],float?,float?)->Optimizer` | RMSprop optimizer |
| `libtorch.opt_step` | `(Optimizer)->bool` | Updates parameters using gradients |
| `libtorch.opt_zero_grad`| `(Optimizer)->bool` | Resets gradients of all parameters |
| `libtorch.reshape` | `(Tensor,list[int])->Tensor` | Changes tensor shape views |
| `libtorch.transpose` | `(Tensor,int,int)->Tensor` | Swaps two dimensions |
| `libtorch.permute` | `(Tensor,list[int])->Tensor` | Permutes multidimensional axes |
| `libtorch.flatten` | `(Tensor,int?,int?)->Tensor` | Flattens contiguous dimensions |
| `libtorch.squeeze` | `(Tensor,int?)->Tensor` | Removes singleton dimensions |
| `libtorch.unsqueeze` | `(Tensor,int)->Tensor` | Inserts singleton dimension |
| `libtorch.stack` | `(list[Tensor],int?)->Tensor` | Concatenates along a new dimension |
| `libtorch.concat` | `(list[Tensor],int?)->Tensor` | Concatenates along an existing axis |
| `libtorch.cuda_is_available`| `()->bool` | Checks if NVIDIA CUDA GPU is present |
| `libtorch.cuda_device_count`| `()->int` | Number of available CUDA GPUs |
| `libtorch.cuda_device_name` | `(int?)->string` | GPU model name |
| `libtorch.cuda_memory_allocated`| `(int?)->int` | VRAM allocated by tensors (bytes) |
| `libtorch.cuda_synchronize` | `(int?)->bool` | Waits for GPU compute completion |
| `libtorch.cuda` | `(Tensor,int?)->Tensor` | Moves tensor to GPU VRAM |
| `libtorch.cpu` | `(Tensor)->Tensor` | Moves tensor to host RAM |
| `libtorch.to` | `(Tensor,string)->Tensor` | Dispatches tensor to device (`"cuda"`, `"cpu"`) |
| `libtorch.is_cuda` | `(Tensor)->bool` | Returns true if tensor resides on GPU |
| `libtorch.save` | `(Tensor,string)->bool` | Serializes tensor to binary file |
| `libtorch.load` | `(string)->Tensor` | Deserializes tensor from file |
| `libtorch.to_list` | `(Tensor)->list[any]` | Converts tensor to nested ulang++ list |
| `libtorch.item` | `(Tensor)->float` | Extracts scalar value from 0-dim tensor |
| `libtorch.shape` | `(Tensor)->list[int]` | Returns tensor shape dimensions |
| `libtorch.dim` | `(Tensor)->int` | Returns number of tensor dimensions (rank) |
| `libtorch.numel` | `(Tensor)->int` | Total count of elements in tensor |
| `libtorch.clone` | `(Tensor)->Tensor` | Deep copy of tensor data |

#### Tensor Method Syntax
Tensors in ulang++ support chained OOP method call syntax:
```ulang++
use std.libtorch

// Model training step with OOP tensor methods
x := libtorch.randn([32, 128])
w := libtorch.randn([128, 10])
w.requires_grad(true)

out := x.matmul(w).relu()
loss := out.sum()
loss.backward()

println("Gradient shape:", w.grad().shape())
```

### `archive`

ZIP and tar.gz creation, extraction, and listing.

| Member | Signature |
| --- | --- |
| `archive.list` | `(string)->list[map[string, json]]` |
| `archive.tar_create` | `(string,string)->bool` |
| `archive.tar_extract` | `(string,string)->bool` |
| `archive.zip_create` | `(string,string)->bool` |
| `archive.zip_extract` | `(string,string)->bool` |

### `cli`

Command-line arguments, terminal prompts, colors, and progress bars.

| Member | Signature |
| --- | --- |
| `cli.args` | `()->list[string]` |
| `cli.color` | `(string,string)->string` |
| `cli.confirm` | `(string,bool?)->bool` |
| `cli.parse` | `(json)->map[string, json]` |
| `cli.progress_bar` | `(int,int,int?,string?)->string` |
| `cli.prompt` | `(string,string?)->string` |
| `cli.prompt_password` | `(string)->string` |

### `compress`

Gzip and zlib helpers. Compressed output is base64 encoded. Decompression is
capped at 16 MiB by default; pass an explicit non-negative byte limit as the
second argument (maximum 1 GiB) when a different bound is needed.

| Member | Signature |
| --- | --- |
| `compress.gzip_compress` | `(string)->string` |
| `compress.gzip_decompress` | `(string,int?)->string` |
| `compress.zlib_compress` | `(string)->string` |
| `compress.zlib_decompress` | `(string,int?)->string` |

### `crypto`

Hashes, HMAC, symmetric encryption, encodings, and cryptographically secure random values.

> [!WARNING]
> `crypto.md5` and `crypto.sha1` are cryptographically broken and vulnerable to practical collision attacks. They are provided solely for legacy checksums and file verification. Never use them for passwords, digital signatures, or security tokens. Use `crypto.sha256` or `crypto.sha512` instead.

> [!NOTE]
> `crypto.random_int(min, max)` uses the OS CSPRNG for cryptographic safety and strictly validates that `min <= max` with integer arguments. For fast, seedable, reproducible pseudo-random numbers (e.g. for games, graphics, and simulations), use `use std.random;` instead.

| Member | Signature |
| --- | --- |
| `crypto.aes_decrypt` | `(string,string)->string` |
| `crypto.aes_decrypt_password` | `(string,string)->string` |
| `crypto.aes_encrypt` | `(string,string)->string` |
| `crypto.aes_encrypt_password` | `(string,string)->string` |
| `crypto.base64_decode` | `(string)->string` |
| `crypto.base64_encode` | `(string)->string` |
| `crypto.constant_time_equal` | `(string,string)->bool` |
| `crypto.derive_key` | `(string,string,int,int)->string` |
| `crypto.hex_decode` | `(string)->string` |
| `crypto.hex_encode` | `(string)->string` |
| `crypto.hmac_sha256` | `(string,string)->string` |
| `crypto.hmac_sha512` | `(string,string)->string` |
| `crypto.md5` | `(string)->string` |
| `crypto.random_int` | `(int,int)->int` |
| `crypto.random_bytes` | `(int)->blob` |
| `crypto.random_string` | `(int,string?)->string` |
| `crypto.sha1` | `(string)->string` |
| `crypto.sha256` | `(string)->string` |
| `crypto.sha512` | `(string)->string` |
| `crypto.uuid` | `()->string` |

### `csv`

CSV parsing and writing (aliases also appear under `formats`).

| Member | Signature |
| --- | --- |
| `csv.parse` | `(string,bool?)->list[json]` |
| `csv.read` | `(string,bool?)->list[json]` |
| `csv.stringify` | `(json,list[string]?)->string` |
| `csv.write` | `(string,json,list[string]?)->bool` |

### `db`

SQLite and PostgreSQL handles. Connections provide `ping`, `exec`, `query`,
`query_row`, `prepare`, connection-pool setters, `begin`, `transaction`, and
`close`. Prepared statements provide `exec`, `query`, `query_row`, and `close`;
transactions provide `exec`, `query`, `prepare`, `commit`, and `rollback`.

| Member | Signature |
| --- | --- |
| `db.open` | `(string,string)->json` |
| `db.postgres` | `(string)->json` |
| `db.sqlite` | `(string?)->json` |

### `env`

Environment variables, OS identity, and PATH management.

| Member | Signature |
| --- | --- |
| `env.add_path` | `(string)->bool` |
| `env.add_system_path` | `(string)->bool` |
| `env.add_user_path` | `(string)->bool` |
| `env.all_env` | `()->map[string, json]` |
| `env.broadcast_env_change` | `()->bool` |
| `env.expand_env` | `(string)->string` |
| `env.get_env` | `(string)->string` |
| `env.get_path` | `()->list[string]` |
| `env.get_system_path` | `()->list[string]` |
| `env.get_user_path` | `()->list[string]` |
| `env.has_path` | `(string)->bool` |
| `env.home_dir` | `()->string` |
| `env.hostname` | `()->string` |
| `env.is_linux` | `()->bool` |
| `env.is_windows` | `()->bool` |
| `env.os_name` | `()->string` |
| `env.path_separator` | `()->string` |
| `env.persist_user_path` | `(string)->bool` |
| `env.prepend_path` | `(string)->bool` |
| `env.remove_path` | `(string)->bool` |
| `env.remove_user_path` | `(string)->bool` |
| `env.set_env` | `(string,string)->bool` |
| `env.set_path` | `(list[string])->bool;(string)->bool` |
| `env.unset_env` | `(string)->bool` |
| `env.username` | `()->string` |
| `env.which` | `(string)->string | null` |
| `env.working_dir` | `()->string` |

### `ffi`

Dynamic native shared library loading, C/C++ runtime compilation, and pointer memory helpers (requires `--allow-ffi` or `--allow-all`).
- Windows dynamically loads `.dll` libraries (Win32 API, Raylib, C/C++ DLLs).
- Unix/Linux dynamically loads `.so` / `.dylib` shared libraries and plugins.
- `ffi.compile` and `ffi.compile_file` compile C/C++ code into native shared libraries on the fly.
- `ffi.build_native_dll` compiles ulang++ source code into pure native PE DLLs.
- Pointer operations (`c_str`, `utf16_str`, `alloc_ptr`, `copy_to_ptr`, `read_from_ptr`, `free`) allow zero-overhead buffer interchange with native libraries.

| Member | Signature |
| --- | --- |
| `ffi.alloc` | `(int?)->blob` |
| `ffi.alloc_ptr` | `(int)->int` |
| `ffi.build_native_dll` | `(string,string)->string` |
| `ffi.c_str` | `(string)->int` |
| `ffi.compile` | `(string,string?)->string` |
| `ffi.compile_file` | `(string,string?)->string` |
| `ffi.copy_to_ptr` | `(int,blob \| string)->bool` |
| `ffi.free` | `(int)->bool` |
| `ffi.load` | `(string)->json` |
| `ffi.load_embedded` | `(string)->json` |
| `ffi.ptr_to_str` | `(int)->string` |
| `ffi.ptr_to_utf16` | `(int)->string` |
| `ffi.read_from_ptr` | `(int,int)->blob` |
| `ffi.size_of` | `(string)->int` |
| `ffi.utf16_str` | `(string)->int` |

`ffi.compile` compiles C/C++ source code dynamically at runtime into a native shared library (.dll / .so) and returns the library path.
`ffi.compile_file` compiles a `.c` or `.cpp` file directly into a native shared library.
`ffi.load_embedded` loads a native library that was packed into the program with `ulang++ build --embed` or native resource flags. The library is extracted to a temporary directory and loaded dynamically.

### `mem`

Raw memory for a program: it allocates a block and reads and writes that block
by address. The memory a program can reach is only the memory it allocated
itself, so the module cannot be used to read or write the memory of the
process. Trusted code only.

| Member | Signature |
| --- | --- |
| `mem.add` | `(int,int)->int` |
| `mem.alloc` | `(int)->int` |
| `mem.copy` | `(int,int,int)->bool` |
| `mem.fill` | `(int,int,int)->bool` |
| `mem.free` | `(int)->bool` |
| `mem.is_null` | `(int)->bool` |
| `mem.read_bytes` | `(int,int?,int)->blob` |
| `mem.read_f64` | `(int,int?)->float` |
| `mem.read_i16` | `(int,int?)->int` |
| `mem.read_i32` | `(int,int?)->int` |
| `mem.read_i64` | `(int,int?)->int` |
| `mem.read_i8` | `(int,int?)->int` |
| `mem.read_u16` | `(int,int?)->int` |
| `mem.read_u32` | `(int,int?)->int` |
| `mem.read_u64` | `(int,int?)->int` |
| `mem.read_u8` | `(int,int?)->int` |
| `mem.size` | `(int)->int` |
| `mem.size_of` | `(string)->int` |
| `mem.write_bytes` | `(int,blob,int?)->bool` |
| `mem.write_f64` | `(int,float,int?)->bool` |
| `mem.write_i16` | `(int,int,int?)->bool` |
| `mem.write_i32` | `(int,int,int?)->bool` |
| `mem.write_i64` | `(int,int,int?)->bool` |
| `mem.write_i8` | `(int,int,int?)->bool` |
| `mem.write_u16` | `(int,int,int?)->bool` |
| `mem.write_u32` | `(int,int,int?)->bool` |
| `mem.write_u64` | `(int,int,int?)->bool` |
| `mem.write_u8` | `(int,int,int?)->bool` |
| `mem.zero` | `(int,int)->bool` |

The readers take the address and an optional offset, the writers the address,
the value and an optional offset.

```ulang++
use std.mem
addr := mem.alloc(8)
mem.write_u32(addr, 305419896)
println(mem.read_u32(addr))
```

### `mtproto`

The protocol of the Telegram network: the AES-IGE cipher, the authorization
key exchange, the message framing, the transport and the serialization layer of
the protocol language. The API of the network is not part of the protocol: a
program passes the identifier of the method and the serialized arguments, so
the module stays independent of the API schema.

| Member | Signature |
| --- | --- |
| `mtproto.call` | `(int,int,blob)->blob` |
| `mtproto.close` | `(int)->bool` |
| `mtproto.connect` | `(int)->json` |
| `mtproto.dc_list` | `()->list[json]` |
| `mtproto.decrypt_message` | `(blob,string,blob)->json` |
| `mtproto.dh_derive` | `(int,blob,blob,blob,blob)->blob` |
| `mtproto.dh_start` | `(string)->json` |
| `mtproto.encrypt_message` | `(blob,int,int,int,int,blob)->blob` |
| `mtproto.fingerprint` | `(blob)->blob` |
| `mtproto.ige_decrypt` | `(blob,blob,blob)->blob` |
| `mtproto.ige_encrypt` | `(blob,blob,blob)->blob` |
| `mtproto.key_hex` | `(blob)->string` |
| `mtproto.key_id` | `(blob)->blob` |
| `mtproto.new_msg_id` | `(int)->int` |
| `mtproto.parse_key` | `(string)->blob` |
| `mtproto.rsa_encrypt` | `(string,blob,blob)->blob` |
| `mtproto.tl_bool` | `(bool)->blob` |
| `mtproto.tl_bytes` | `(blob)->blob` |
| `mtproto.tl_int32` | `(int)->blob` |
| `mtproto.tl_int64` | `(int)->blob` |
| `mtproto.tl_read_bool` | `(blob)->bool` |
| `mtproto.tl_read_bytes` | `(blob)->blob` |
| `mtproto.tl_read_int32` | `(blob)->int` |
| `mtproto.tl_read_int64` | `(blob)->int` |
| `mtproto.tl_read_string` | `(blob)->string` |
| `mtproto.tl_string` | `(string)->blob` |
| `mtproto.use_key` | `(int,blob)->bool` |

The members that open a connection need the network capability of the runtime.

```ulang++
use std.mtproto
use std.crypto

key := crypto.random_bytes(32)
data := crypto.random_bytes(16)
iv := crypto.random_bytes(32)
encrypted := mtproto.ige_encrypt(key, data, iv)
println(len(mtproto.ige_decrypt(key, encrypted, iv)))
```

### `embed`

Read-only access to resources included in standalone builds.

| Member | Signature |
| --- | --- |
| `embed.exists` | `(string)->bool` |
| `embed.list` | `(string?)->list[string]` |
| `embed.read` | `(string)->string` |

### `formats`

Dynamic namespace containing the `csv`, `ini`, and `json` helpers.

| Member | Signature |
| --- | --- |
| `formats.csv` | dynamic (`json`) |
| `formats.ini` | dynamic (`json`) |
| `formats.json` | dynamic (`json`) |

### `gui` [Cross-platform / Multi-backend]

Desktop user interface automation, native dialogs, windows, and embedded WebView helpers. Requires trusted execution (`--allow-all`).
- **Windows**: Uses native Win32 controls, TaskDialogIndirect, and Microsoft Edge WebView2.
- **macOS**: Uses Cocoa / osascript dialogs and WebKit WKWebView.
- **Linux**: Uses Tk, Qt, Zenity / kdialog, or WebKitGTK.
- **Backend Selection**: Call `gui.set_backend("win32" | "tk" | "qt" | "auto")` to configure preferred engine.

| Member | Signature |
| --- | --- |
| `gui.alert` | `(string,string?)->bool` |
| `gui.confirm` | `(string,string?)->bool` |
| `gui.prompt` | `(string,string?,string?)->string` |
| `gui.notify` | `(string,string?)->bool` |
| `gui.msgbox` | `(string,string?,int?)->int` |
| `gui.open_file` | `(string?,string?)->string` |
| `gui.save_file` | `(string?,string?)->string` |
| `gui.screen_size` | `()->map[string, int]` |
| `gui.width` | `()->int` |
| `gui.height` | `()->int` |
| `gui.screenshot` | `(string?)->blob` |
| `gui.window` | `(string,int,int)->map[string, json]` |
| `gui.tk_window` | `(string,int,int)->map[string, json]` |
| `gui.qt_window` | `(string,int,int)->map[string, json]` |
| `gui.webview` | `(string,string,int,int)->map[string, json]` |
| `gui.set_backend` | `(string)->bool` |
| `gui.backends` | `()->map[string, bool]` |
| `gui.task_manager` | `()->bool` |
| `gui.native_task_manager` | `()->bool` |

### `ini`

INI parsing and writing.

| Member | Signature |
| --- | --- |
| `ini.parse` | `(string)->json` |
| `ini.read` | `(string)->json` |
| `ini.stringify` | `(json)->string` |
| `ini.write` | `(string,json)->bool` |

### `io`

File and directory operations, streams, locks, and permission checks.

| Member | Signature |
| --- | --- |
| `io.append_file` | `(string,string)->bool` |
| `io.check_access` | `(string,string)->bool` |
| `io.chmod` | `(string,int)->bool;(string,string)->bool` |
| `io.copy_file` | `(string,string)->bool` |
| `io.delete_file` | `(string)->bool` |
| `io.file_exists` | `(string)->bool` |
| `io.file_info` | `(string)->map[string, json]` |
| `io.file_size` | `(string)->int` |
| `io.glob` | `(string)->list[string]` |
| `io.is_permission_error` | `(any)->bool` |
| `io.list_dir` | `(string?)->list[string]` |
| `io.lock_file` | `(any,bool?)->bool` |
| `io.make_dir` | `(string)->bool` |
| `io.move_file` | `(string,string)->bool` |
| `io.open` | `(string,string?)->json` |
| `io.permissions` | `(string)->map[string, json]` |
| `io.read_file` | `(string)->string` |
| `io.read_bytes` | `(string)->blob` |
| `io.read_lines` | `(string)->list[string]` |
| `io.temp_dir` | `(string?)->string` |
| `io.temp_file` | `(string?)->string` |
| `io.unlock_file` | `(any)->bool` |
| `io.write_file` | `(string,string)->bool` |
| `io.write_bytes` | `(string,blob)->bool` |
| `io.walk` | `(string)->list[string]` |

`read_lines` supports lines longer than the default Go scanner limit.
`file_exists` reports access/stat failures as errors instead of treating them
as evidence that a path exists.

### `json`

JSON parse/encode, formatting, validation, and dot-path queries. `parse` always
parses text; `query` accepts either a JSON string or an already-decoded value.

| Member | Signature |
| --- | --- |
| `json.encode` | `(any)->string` |
| `json.parse` | `(string)->json` |
| `json.prettify` | `(any,string?)->string` |
| `json.query` | `(string,string)->json;(json,string)->json` |
| `json.stringify` | `(any)->string` |
| `json.validate` | `(string)->bool` |

### `log`

Structured logging and the security audit trail.

| Member | Signature |
| --- | --- |
| `log.access_denied` | `(string,string,string?)->bool` |
| `log.audit` | `(string,string,string,string,json?)->bool` |
| `log.debug` | `(string,json?)->null` |
| `log.error` | `(string,json?)->null` |
| `log.fatal` | `(string,json?)->null` |
| `log.get_access_errors` | `()->list[map[string, json]]` |
| `log.info` | `(string,json?)->null` |
| `log.set_file` | `(string,int?,int?)->bool` |
| `log.set_format` | `(string)->bool` |
| `log.set_level` | `(string)->bool` |
| `log.warn` | `(string,json?)->null` |

### `math`

Constants and trigonometric, rounding, and aggregate helpers.

| Member | Signature |
| --- | --- |
| `math.E` | `float` |
| `math.LN10` | `float` |
| `math.LN2` | `float` |
| `math.PI` | `float` |
| `math.SQRT2` | `float` |
| `math.abs` | `(int)->int;(float)->float` |
| `math.acos` | `(float)->float` |
| `math.asin` | `(float)->float` |
| `math.atan` | `(float)->float` |
| `math.atan2` | `(float,float)->float` |
| `math.ceil` | `(float)->int` |
| `math.clamp` | `(int,int,int)->int;(float,float,float)->float` |
| `math.cos` | `(float)->float` |
| `math.deg2rad` | `(float)->float` |
| `math.floor` | `(float)->int` |
| `math.hypot` | `(float,float)->float` |
| `math.log` | `(float)->float` |
| `math.log10` | `(float)->float` |
| `math.log2` | `(float)->float` |
| `math.max` | `(int,int)->int;(float,float)->float` |
| `math.min` | `(int,int)->int;(float,float)->float` |
| `math.pow` | `(float,float)->float;(int,int)->float` |
| `math.rad2deg` | `(float)->float` |
| `math.round` | `(float)->int` |
| `math.sin` | `(float)->float` |
| `math.sqrt` | `(float)->float;(int)->float` |
| `math.tan` | `(float)->float` |

The bytecode compiler lowers exact-arity positional calls to `math.abs`,
`min`, `max`, `clamp`, `sqrt`, `round`, `floor`, and `ceil` to numeric
intrinsics. The AMD64 JIT executes integer and floating-point arguments
directly; unsupported runtime types deoptimize to the VM for the standard
library's normal error behavior. Other `std.math` functions execute in the JIT
runtime-helper tier; direct unshadowed calls are fused as pure calls. See
[JIT and standalone execution](#jit-and-standalone-execution) for the exact
per-engine behavior.

### `metrics`

Prometheus counters, gauges, histograms, and an HTTP exporter.

| Member | Signature |
| --- | --- |
| `metrics.counter` | `(string,string?)->json` |
| `metrics.export` | `()->string` |
| `metrics.gauge` | `(string,string?)->json` |
| `metrics.histogram` | `(string,string?,json?)->json` |
| `metrics.serve` | `(int?,string?)->string` |

### `net`

HTTP requests, DNS, TCP, ping, and downloads.

| Member | Signature |
| --- | --- |
| `net.dns_lookup` | `(string)->list[string]` |
| `net.download_file` | `(string,string)->bool` |
| `net.http_get` | `(string)->map[string, json]` |
| `net.http_post` | `(string,string,string)->map[string, json]` |
| `net.ping` | `(string)->bool` |
| `net.tcp_connect` | `(string)->map[string, json]` |
| `net.tcp_listen` | `(string)->map[string, json]` |

The simple HTTP helpers limit response bodies to 16 MiB, as does `http`.
`ping` reports a successful TCP connection rather than treating DNS
resolution alone as host reachability.

### `http`

HTTP client requests and a small HTTP server. Client response bodies are
limited to 16 MiB. The optional options map supports a request `timeout` in
seconds (greater than 0, at most 300).

`http.router()` creates a route table. Register handlers with `get`, `post`,
`put`, `patch`, `delete`, `head`, `options`, or `route(method, path, handler,
options?)`; paths support `{name}` parameters and a final `{name:path}`
wildcard. Each handler receives a request map containing `method`, `url`,
`path`, `headers`, `query`, `params`, `body`, and (for JSON requests) `json`.
Return a string for a text response, or use `http.json(value, status?,
headers?)` / `http.response(status, body?, headers?)`.

`router.use(middleware)` adds middleware in registration order. A middleware
receives `(request, next)` and can call `next(request)` to continue or return a
response directly. Route options support a `schema` map with `path`, `query`,
and `body` field maps. Field types are `string`, `int`, `float`, `number`,
`bool`, `object`, `array`, `json`, or `any`; fields are required by default
and can use `{"type": "int", "required": false}` to be optional. Query and
path values are converted to the declared types; JSON bodies are validated
against object fields. Invalid input receives HTTP 422, malformed JSON
receives HTTP 400, missing routes return 404, and method mismatches return 405.

Route options can also include `dependencies`, a map from names to functions.
Each dependency receives the request and its result is available under
`request["dependencies"]`. `summary`, `description`, and `tags` are included
in generated OpenAPI metadata. `router.openapi()` returns a basic OpenAPI 3.0.3
document with route parameters and request-body schemas;
`router.openapi("/openapi.json")` registers a JSON document route. Start
serving with `router.serve(port)`. The returned `HTTPServer` exposes `port` and
`close()`. `http.http_serve(port, handler)` remains available for single-handler
servers.

```ulang++
use std.http

router := http.router()
health := |request: map[string, json]| http.json({"ok": true})
options: map[string, json] = {"summary": "Health check"}
router.get("/health", health, options)
server := router.serve(8080)
println(server.port)
server.close()
```

| Member | Signature |
| --- | --- |
| `http.delete` | `(string,map[string, string]?,string?,map[string, json]?)->map[string, json]` |
| `http.get` | `(string,map[string, string]?,string?,map[string, json]?)->map[string, json]` |
| `http.patch` | `(string,map[string, string]?,string?,map[string, json]?)->map[string, json]` |
| `http.post` | `(string,map[string, string]?,string?,map[string, json]?)->map[string, json]` |
| `http.put` | `(string,map[string, string]?,string?,map[string, json]?)->map[string, json]` |
| `http.json` | `(any,int?,map[string, json]?)->map[string, json]` |
| `http.response` | `(int,any?,map[string, json]?)->map[string, json]` |
| `http.request` | `(string,string,map[string, string]?,string?,map[string, json]?)->map[string, json]` |
| `http.router` | `()->HTTPRouter` |
| `http.http_serve` | `(int,any)->HTTPServer` |

`HTTPRouter` provides `get`, `post`, `put`, `patch`, `delete`, `head`,
`options`, `route`, `use`, `serve`, and `openapi` methods. Route registration
returns `bool`; `serve` returns an `HTTPServer`.

| Router method | Signature |
| --- | --- |
| `router.get/post/put/patch/delete/head/options` | `(string,any,map[string, json]?)->bool` |
| `router.route` | `(string,string,any,map[string, json]?)->bool` |
| `router.use` | `(any)->bool` |
| `router.serve` | `(int)->HTTPServer` |
| `router.openapi` | `(string?)->json` |
| `server.close` | `()->bool` |

### `urllib2`

URL parsing/encoding and bounded HTTP request helpers inspired by Python's
`urllib`. `parse` returns a map with `scheme`, `netloc`, `path`, `params`,
`query`, `fragment`, `username`, `password`, `hostname`, and `port`.
`split` returns the classic six URL components as a list. `urlencode` accepts
a map; list values become repeated query parameters. `parse_qs` groups repeated
values into lists, while `parse_qsl` returns ordered key/value pairs.

`request(method, url, data?, headers?, timeout?)` returns a map containing
`status`, `reason`, `body`, `headers`, and the final redirected `url`.
`data` may be a string, blob, or map; maps are form-encoded and use
`application/x-www-form-urlencoded`. `urlopen(url, data?, headers?, timeout?)`
uses GET without data and POST when data is supplied. `get` and `post` are
convenience wrappers. Requests have a 30-second default timeout (up to
300 seconds) and a 16 MiB response cap. Redirects are rechecked against the
network capability policy. `urlretrieve(url, destination)` saves a successful
response under the write capability and returns its filename and headers.
`tcp_request` is an alias for the same policy-aware HTTP request path.
WebSocket calls use a real `ws`/`wss` transport with masked client frames,
fragment handling, ping/pong, UTF-8 validation, and a 16 MiB message limit.
The deprecated `websocket_handshake` alias opens and returns a connection just
like `websocket_connect`; callers should migrate to the newer name.
The legacy `websocket_frame_read` and `websocket_frame_write` helpers remain
unsupported; use `websocket_receive` and `websocket_send` with a connection ID.

```ulang++
use std.urllib2

parts := urllib2.parse("https://example.com/search?q=ulang++")
query: string = string(parts["query"])
encoded: string = urllib2.urlencode({"q": "ulang++ language", "page": "1"})
response := urllib2.get("https://example.com/search?" + encoded)
println(response["status"])
println(response["body"])
```

| Member | Signature |
| --- | --- |
| `urllib2.parse` | `(string,string?)->map[string, json]` |
| `urllib2.split` | `(string,string?)->list[string]` |
| `urllib2.join` | `(string,string)->string` |
| `urllib2.quote` | `(string,string?)->string` |
| `urllib2.unquote` | `(string,bool?)->string` |
| `urllib2.urlencode` | `(any)->string` |
| `urllib2.parse_qs` | `(string)->map[string, list[string]]` |
| `urllib2.parse_qsl` | `(string)->list[list[string]]` |
| `urllib2.request` | `(string,string,any?,any?,int?)->map[string, json];(string,string,any?,any?,float?)->map[string, json]` |
| `urllib2.urlopen` | `(string,any?,any?,int?)->map[string, json];(string,any?,any?,float?)->map[string, json]` |
| `urllib2.get` | `(string,any?,int?)->map[string, json];(string,any?,float?)->map[string, json]` |
| `urllib2.post` | `(string,any,any?,int?)->map[string, json];(string,any,any?,float?)->map[string, json]` |
| `urllib2.urlretrieve` | `(string,string)->map[string, json]` |
| `urllib2.tcp_request` | `(string,string,any?,any?,int?)->map[string, json];(string,string,any?,any?,float?)->map[string, json]` |
| `urllib2.websocket_connect` | `(string,map[string,string]?,list[string]?,int?)->map[string, json];(string,map[string,string]?,list[string]?,float?)->map[string, json]` |
| `urllib2.websocket_send` | `(string,string \| blob,int?)->bool;(string,string \| blob,string?)->bool` |
| `urllib2.websocket_receive` | `(string,int?)->string \| blob;(string,float?)->string \| blob` |
| `urllib2.websocket_ping` | `(string,any?)->bool` |
| `urllib2.websocket_pong` | `(string,any?)->bool` |
| `urllib2.websocket_close` | `(string,int?,string?)->bool` |
| `urllib2.websocket_handshake` | `(string,map[string,string]?)->map[string, json]` |

### `path`

Portable path manipulation.

| Member | Signature |
| --- | --- |
| `path.abs` | `(string)->string` |
| `path.base` | `(string)->string` |
| `path.clean` | `(string)->string` |
| `path.dir` | `(string)->string` |
| `path.expand` | `(string)->string` |
| `path.ext` | `(string)->string` |
| `path.from_slash` | `(string)->string` |
| `path.is_abs` | `(string)->bool` |
| `path.join` | `(string,...string)->string` |
| `path.join_list` | `(list[string])->string;(...string)->string` |
| `path.list_sep` | `()->string` |
| `path.matches` | `(string,string)->bool` |
| `path.rel` | `(string,string)->string` |
| `path.sep` | `()->string` |
| `path.split` | `(string)->list[string]` |
| `path.split_list` | `(string)->list[string]` |
| `path.stem` | `(string)->string` |
| `path.to_slash` | `(string)->string` |
| `path.which` | `(string)->string | null` |

### `process`

Process inspection, signaling, and command execution; trusted code only.

| Member | Signature |
| --- | --- |
| `process.cpu_count` | `()->int` |
| `process.exec_bg` | `(string)->int` |
| `process.exec_cmd` | `(string)->map[string, json]` |
| `process.get_pid` | `()->int` |
| `process.get_ppid` | `()->int` |
| `process.kill_process` | `(int)->bool` |
| `process.memory_usage` | `()->map[string, json]` |
| `process.process_exists` | `(int)->bool` |
| `process.signal_process` | `(int,string)->bool` |
| `process.uptime` | `()->int` |

### `re`

Regular expressions (alias: `regex`).

| Member | Signature |
| --- | --- |
| `re.find` | `(string,string)->string | null` |
| `re.find_all` | `(string,string,int?)->list[string]` |
| `re.find_all_groups` | `(string,string,int?)->list[list[string]]` |
| `re.find_groups` | `(string,string)->list[string] | null` |
| `re.match` | `(string,string)->bool` |
| `re.quote` | `(string)->string` |
| `re.replace` | `(string,string,string)->string` |
| `re.split` | `(string,string,int?)->list[string]` |

### `sys`

System resource information, hardware metrics, and process enumeration. Process details are
limited to fields exposed by the host implementation. RAM, uptime, process,
and load metrics use host-specific sources; unavailable metrics return an
error rather than Go runtime memory or fabricated zero values. Windows does
not expose the Unix-style load average and `sys.load_avg` returns an unsupported status.

| Member | Signature |
| --- | --- |
| `sys.arch` | `()->string` |
| `sys.boot_time` | `()->float` |
| `sys.cpu_count` | `()->int` |
| `sys.disk_partitions` | `()->list[map[string, json]]` |
| `sys.disk_usage` | `(string?)->map[string, json]` |
| `sys.load_avg` | `()->map[string, json]` |
| `sys.network_interfaces` | `()->list[map[string, json]]` |
| `sys.pids` | `()->list[int]` |
| `sys.platform` | `()->string` |
| `sys.process_info` | `(int)->map[string, json]` |
| `sys.process_iter` | `()->list[map[string, json]]` |
| `sys.process_list` | `()->list[map[string, json]]` |
| `sys.ram_usage` | `()->map[string, json]` |
| `sys.uptime` | `()->float` |
| `sys.virtual_memory` | `()->map[string, json]` |
| `sys.whoami` | `()->map[string, json]` |

### `time`

Clocks, formatting, parsing, and sleeps.

| Member | Signature |
| --- | --- |
| `time.date` | `(int?,string?)->map[string, json];(float?,string?)->map[string, json]` |
| `time.diff` | `(int,int)->float;(float,float)->float` |
| `time.format` | `(int,string?,string?)->string;(float,string?,string?)->string` |
| `time.measure` | `(json)->map[string, json]` |
| `time.now` | `()->int` |
| `time.now_ms` | `()->int` |
| `time.now_ns` | `()->int` |
| `time.now_str` | `(string?,string?)->string` |
| `time.parse` | `(string,string?,string?)->int` |
| `time.parse_duration` | `(string)->int` |
| `time.since` | `(int)->float;(float)->float` |
| `time.sleep` | `(int)->null;(float)->null;(string)->null` |
| `time.timer` | `(string)->map[string, json]` |
| `time.unix_ms_to_seconds` | `(int)->float` |
| `time.unix_seconds_to_ms` | `(int)->int;(float)->int` |

Timestamps returned by `now` use seconds; `now_ms` and `now_ns` state their
units explicitly. The optional time zone argument accepts an IANA location.
`parse_duration` returns nanoseconds. Avoid inferring timestamp units from
magnitude; `format`/`since`/`date` use seconds, and the conversion helpers
make millisecond conversions explicit. `timer(duration)` returns an object
with blocking `wait()` and `cancel()` methods; `wait()` returns `false` when
cancelled.

### `string`

Unicode-aware text helpers. `length`, `slice`, and `runes` count Unicode code
points (runes), not UTF-8 bytes or grapheme clusters. Slice indices are
zero-based, end-exclusive; negative indices count from the end.

| Member | Signature |
| --- | --- |
| `string.capitalize` | `(string)->string` |
| `string.center` | `(string,int,string?)->string` |
| `string.char_at` | `(string,int)->string` |
| `string.code_point_at` | `(string,int)->int` |
| `string.contains` | `(string,string)->bool` |
| `string.count` | `(string,string)->int` |
| `string.ends_with` | `(string,string)->bool` |
| `string.format` | `(string,any...)->string` |
| `string.from_code_point` | `(int)->string` |
| `string.index_of` | `(string,string)->int` |
| `string.is_alpha` | `(string)->bool` |
| `string.is_alphanumeric` | `(string)->bool` |
| `string.is_blank` | `(string)->bool` |
| `string.is_digit` | `(string)->bool` |
| `string.is_empty` | `(string)->bool` |
| `string.is_lower` | `(string)->bool` |
| `string.is_numeric` | `(string)->bool` |
| `string.is_upper` | `(string)->bool` |
| `string.join` | `(list[string],string)->string` |
| `string.last_index_of` | `(string,string)->int` |
| `string.length` | `(string)->int` |
| `string.lines` | `(string)->list[string]` |
| `string.lower` | `(string)->string` |
| `string.pad` | `(string,int,string?,string?)->string` |
| `string.pad_left` | `(string,int,string?)->string` |
| `string.pad_right` | `(string,int,string?)->string` |
| `string.repeat` | `(string,int)->string` |
| `string.replace` | `(string,string,string)->string` |
| `string.replace_n` | `(string,string,string,int)->string` |
| `string.reverse` | `(string)->string` |
| `string.runes` | `(string)->list[string]` |
| `string.slice` | `(string,int,int)->string` |
| `string.split` | `(string,string)->list[string]` |
| `string.starts_with` | `(string,string)->bool` |
| `string.title` | `(string)->string` |
| `string.trim` | `(string)->string` |
| `string.trim_left` | `(string)->string` |
| `string.trim_prefix` | `(string,string)->string` |
| `string.trim_right` | `(string)->string` |
| `string.trim_suffix` | `(string,string)->string` |
| `string.upper` | `(string)->string` |
| `string.words` | `(string)->list[string]` |

### `list`

Functional sequence utilities and list manipulation helpers.

| Member | Signature |
| --- | --- |
| `list.all` | `(list[any],any?)->bool` |
| `list.any` | `(list[any],any?)->bool` |
| `list.concat` | `(list[any],list[any])->list[any]` |
| `list.contains` | `(list[any],any)->bool` |
| `list.filter` | `(list[any],any)->list[any]` |
| `list.find` | `(list[any],any)->any` |
| `list.find_index` | `(list[any],any)->int` |
| `list.flatten` | `(list[any])->list[any]` |
| `list.index_of` | `(list[any],any)->int` |
| `list.join` | `(list[any],string?)->string` |
| `list.keys` | `(list[any])->list[int]` |
| `list.length` | `(list[any])->int` |
| `list.map` | `(list[any],any)->list[any]` |
| `list.pop` | `(list[any])->any` |
| `list.push` | `(list[any],any)->list[any]` |
| `list.reduce` | `(list[any],any,any?)->any` |
| `list.reverse` | `(list[any])->list[any]` |
| `list.slice` | `(list[any],int,int?)->list[any]` |
| `list.sort` | `(list[any],any?)->list[any]` |
| `list.unique` | `(list[any])->list[any]` |
| `list.values` | `(list[any])->list[any]` |

### `map`

Dictionary and key-value mapping helpers with preserved insertion order.

| Member | Signature |
| --- | --- |
| `map.clone` | `(map[any, any])->map[any, any]` |
| `map.delete` | `(map[any, any],any)->any` |
| `map.entries` | `(map[any, any])->list[list[any]]` |
| `map.filter` | `(map[any, any],any)->map[any, any]` |
| `map.from_entries` | `(list[any])->map[string, any]` |
| `map.get` | `(map[any, any],any,any?)->any` |
| `map.has` | `(map[any, any],any)->bool` |
| `map.keys` | `(map[any, any])->list[string]` |
| `map.length` | `(map[any, any])->int` |
| `map.map` | `(map[any, any],any)->map[any, any]` |
| `map.merge` | `(map[any, any],map[any, any])->map[any, any]` |
| `map.reduce` | `(map[any, any],any,any?)->any` |
| `map.reverse` | `(map[any, any])->map[any, any]` |
| `map.set` | `(map[any, any],any,any)->map[any, any]` |
| `map.sort` | `(map[any, any],any?)->map[any, any]` |
| `map.values` | `(map[any, any])->list[any]` |

### `random`

Fast seedable pseudo-random number generator for games, graphics, procedural generation, and Monte-Carlo simulations. It uses an internal PRNG distinct from `crypto.random_*` which allows repeatable sequences using `random.seed(n)`.

| Member | Signature |
| --- | --- |
| `random.seed` | `(int)->null` |
| `random.random` | `()->float` |
| `random.randint` | `(int,int)->int` |
| `random.uniform` | `(float,float)->float` |
| `random.choice` | `(list[any])->any` |
| `random.shuffle` | `(list[any])->list[any]` |
| `random.sample` | `(list[any],int)->list[any]` |
| `random.gaussian` | `(float?,float?)->float` |
| `random.bool` | `(float?)->bool` |

### Error Handling & Inspection

Errors in ulang++ are first-class inspectable objects (`error` type) providing typed categories, standard error codes, and UTC timestamps:

| Category | Code | Description |
| --- | --- | --- |
| `invalid_argument` | 400 | Bad argument type, out-of-range value, or validation failure |
| `permission_denied` | 403 | Sandbox capability denial or filesystem access violation |
| `not_found` | 404 | File, directory, device, or key not found |
| `already_exists` | 409 | Resource or file already exists |
| `io_error` | 500 | Disk I/O read/write failure |
| `network_error` | 502 / 504 | Socket connection timeout, connection reset, or DNS failure |
| `internal_error` | 500 | General host exception |

Error values provide inspection methods:
- `err.code()`: returns the integer error code (e.g. `404`).
- `err.category()`: returns the category string (e.g. `"not_found"`).
- `err.message()`: returns the detailed error message.
- `err.time()`: returns the RFC3339 UTC creation timestamp string.

## Global builtins

These functions are available without a `use` statement.

### Special runtime identifiers (dunder variables)

The following immutable built-in variables are always available globally:

| Identifier | Type | Description |
| --- | --- | --- |
| `__file__` | `string` | File path of the currently executing source file (or `"<stdin>"` if inline/REPL). |
| `__dir__` | `string` | Directory containing `__file__` (or `"."` if none). |
| `__dirname__` | `string` | Alias for `__dir__`. |
| `__name__` | `string` | `"__main__"` when executing the top-level script, or the module path when imported. |
| `__line__` | `int` | Current 1-based source code line number where the identifier appears. |
| `__version__` | `string` | ulang++ runtime and toolchain version (e.g. `"1.7.9"`). |
| `__column__` | `int` | Current 1-based source column where the identifier appears. |
| `__os__` | `string` | Operating system (`runtime.GOOS`). |
| `__arch__` | `string` | Target architecture (`runtime.GOARCH`). |
| `__exe__` | `string` | Absolute path of the running executable. |
| `__argv__` | `list[string]` | Command-line arguments passed to the script. |
| `__compiled__` | `bool` | `true` when running from a compiled bundle. |

All `__*__`-style names are reserved: user code cannot declare, shadow or assign to them in either execution engine.

### Global functions

| Function | Signature |
| --- | --- |
| `abs` | `(int)->int;(float)->float` |
| `bool` | `(any)->bool` |
| `exec` | `(string)->string` |
| `exit` | `(int?)->void` |
| `float` | `(any)->float` |
| `input` | `(string?)->string` |
| `int` | `(any)->int` |
| `json_decode` | `(string)->json` |
| `json_encode` | `(any)->string` |
| `json_prettify` | `(any)->string` |
| `json_query` | `(json,string)->json` |
| `max` | `(int,int)->int;(float,float)->float` |
| `min` | `(int,int)->int;(float,float)->float` |
| `print` | `(any...)->string` |
| `println` | `(any...)->string` |
| `sleep` | `(int)->null;(float)->null` |
| `str_contains` | `(string,string)->bool` |
| `str_ends_with` | `(string,string)->bool` |
| `str_lower` | `(string)->string` |
| `str_replace` | `(string,string,string)->string` |
| `str_split` | `(string,string)->list[string]` |
| `str_starts_with` | `(string,string)->bool` |
| `str_trim` | `(string)->string` |
| `str_upper` | `(string)->string` |
| `string` | `(any)->string` |
| `time_now` | `()->string` |
| `type` | `(any)->string` |
| `typeof` | `(any)->string` |

> [!NOTE]
> Global convenience aliases for standard library functions (such as `read_file`, `write_file`, `get_env`, `sha256`, `cpu_count`, etc.) have been removed and deprecated to keep the global namespace clean and maintain strict module boundaries. Standard library functions must be imported explicitly through their module namespace (e.g. `use std.io;` then `io.read_file(...)`, `use std.env;` then `env.get_env(...)`, `use std.crypto;` then `crypto.sha256(...)`). Calling removed global aliases triggers compile-time deprecation diagnostics guiding you to the appropriate `std.*` module.

Former global aliases:

- `env`: `add_path`, `add_system_path`, `add_user_path`, `all_env`, `broadcast_env_change`, `expand_env`, `get_env`, `get_path`, `get_system_path`, `get_user_path`, `has_path`, `home_dir`, `hostname`, `is_linux`, `is_windows`, `os_name`, `path_separator`, `persist_user_path`, `prepend_path`, `remove_path`, `remove_user_path`, `set_env`, `set_path`, `unset_env`, `username`, `which`, `working_dir`
- `io`: `append_file`, `check_access`, `chmod`, `copy_file`, `delete_file`, `file_exists`, `file_info`, `file_size`, `glob`, `is_permission_error`, `list_dir`, `lock_file`, `make_dir`, `move_file`, `open`, `permissions`, `read_bytes`, `read_file`, `read_lines`, `temp_dir`, `temp_file`, `unlock_file`, `walk`, `write_bytes`, `write_file`
- `net`: `dns_lookup`, `download_file`, `http_get`, `http_post`, `ping`
- `crypto`: `aes_decrypt`, `aes_decrypt_password`, `aes_encrypt`, `aes_encrypt_password`, `base64_decode`, `base64_encode`, `constant_time_equal`, `derive_key`, `hex_decode`, `hex_encode`, `hmac_sha256`, `hmac_sha512`, `md5`, `random_bytes`, `random_int`, `random_string`, `sha1`, `sha256`, `sha512`, `uuid`
- `compress`: `gzip_compress`, `gzip_decompress`, `zlib_compress`, `zlib_decompress`
- `process`: `cpu_count`, `exec_bg`, `exec_cmd`, `get_pid`, `get_ppid`, `kill_process`, `memory_usage`, `process_exists`, `signal_process`, `uptime`

<a id="jit-and-standalone-execution"></a>
## JIT and standalone execution

Every registered standard-library member is callable from JIT-compiled
functions. There are three genuinely different execution paths, and none of
them silently pretends that a host-dependent operation was translated to
machine code:

- **Native intrinsic**: the AMD64 backend emits the operation directly. Only
  the eight numeric `std.math` members listed above have intrinsics. They are
  selected only for exact-arity calls through an unshadowed module alias
  (`use std.math;` or `use std.math as m;`) that never escapes and is never
  mutated. If the alias is assigned, passed as a value, used with a subscript,
  or a module member is modified, the call uses the generic JIT helper path (or
  the VM when no JIT is active) so the mutation is observed. If the runtime
  value is not numeric, the intrinsic
  records a deoptimization point and the VM raises the same error the library
  would raise.
- **JIT runtime helper (fused pure call)**: direct calls to pure stdlib members
  are compiled to `OpStdlibCall`. The threaded JIT and the AMD64 JIT engine
  resolve the registered function and execute it in the JIT runtime-helper
  tier, then resume JIT execution immediately after the call. The function body
  still runs as its normal Go implementation; it is not machine code, but the
  call does not enter VM bytecode dispatch. List and map literals used as
  arguments are constructed by the same helper tier, so expressions such as
  `str.join([...], "-")` and `formats.json.encode({...})` stay on this path.
  Whole pure modules are `math`, `string`/`str`, `re`/`regex`, `compress`, and
  `json`. Deterministic value members of mixed modules (`path.join`,
  `crypto.sha256`, `urllib2.parse`, `urllib2.join`, `time.format`, the
  in-memory `csv.parse`/`csv.stringify` and `ini.parse`/`ini.stringify`
  members, the nested `formats.json.*`, `formats.csv.parse`, and
  `formats.ini.parse` paths, and the deterministic `ai` operations) use the
  same fused path; host, clock, entropy, or file-backed members such as
  `path.abs`, `path.expand`, `path.which`, `crypto.uuid`,
  `crypto.aes_encrypt`, `urllib2.request`, `time.now`, `csv.read`,
  `csv.write`, `ini.read`, `ini.write`, and `libtorch.matrix` use the generic
  host-call helper.
- **JIT runtime helper (host calls)**: for non-pure members, including network,
  database, general file I/O, GUI, process, and the request/download members of
  `urllib2`, the JIT performs the module lookup, member access, and registered
  builtin call in the same runtime-helper tier and then resumes JIT execution.
  The host operation still runs its normal Go implementation and is subject to
  the same capability policy; it is not translated to machine code and does not
  enter VM bytecode dispatch. If the runtime binding is not a callable the
  helper understands (for example a tree-walking `FuncValue` or a namespace
  value), the call site deoptimizes to the VM without side effects.

`use` statements execute in the same JIT runtime-helper tier. After binding the
import, native or threaded execution resumes at the next instruction, so a
top-level pure call is not forced back into the VM merely because the program
had to import a module first.

User-defined compiled functions and closures are called through the same VM
call helper. `OpCall` and `OpClosure` run on the VM that owns the active stack
through `JITCallbacks`, so argument evaluation, recursion depth, captured
upvalues, globals, and handler unwinding match bytecode execution. A closure
created by a JIT frame captures from that same frame and stays valid when it
escapes; `OpGetFree`/`OpSetFree` read and write both open and closed upvalues,
and frame returns close open upvalues exactly as the VM does. An arity
mismatch bails before the call, so the VM reports its exact error without side
effects. Callable values the helper cannot run — currently tree-walking
`FuncValue` values and namespace values — still deoptimize to the VM.

Container access and mutation (`list[i]`, `map[key]`, `obj.field = value`),
including int- and rune-backed list representations, single-argument builtin
conversions (`int`, `float`, `bool`, `len`), `**`, `in`, range construction,
iterable preparation, equality/ordering on strings and collections, `err(...)`
construction and raising, and `assert` also execute in the helper tier instead
of deoptimizing the enclosing function.

Fused lowering is deliberately conservative. It requires a direct member call
through an unshadowed `use` alias with no explicit import list. Assigning the
alias, passing the module as a value, subscripting it, declaring a function
with the same name, or modifying a member disables the fused path for that
alias. The normal call path then observes the mutation exactly as before.
Runtime lookup of a non-builtin binding also falls back to the VM without
having performed any side effects.

| Engine or build | Pure stdlib member (fused) | Host stdlib member | `math.abs` / `min` / `max` / `clamp` / `sqrt` / `round` / `floor` / `ceil` |
| --- | --- | --- | --- |
| Tree-walking interpreter | Direct call, no JIT | Direct call, no JIT | Direct call, no JIT |
| Bytecode VM (JIT disabled) | `OpStdlibCall`, VM executes the member | Direct VM call | `OpMathBuiltin` |
| Threaded JIT | JIT runtime helper; resumes threaded code | JIT runtime helper; resumes threaded code | JIT runtime helper |
| AMD64 native JIT | JIT runtime helper; resumes native code | JIT runtime helper; resumes native code | Inline machine code; invalid types bail out |
| Standalone runtime stub (bytecode or AOT payload) | Bundled runtime executes; AOT code resumes after the helper | Bundled runtime executes; AOT code resumes after the helper | Inline when JIT/AOT code uses an intrinsic |
| Pure native PE/AOT (no VM) | Unavailable | Unavailable | Not selected |

User calls follow the same split: threaded and AMD64 JIT execute compiled
functions and closures through the VM call helper; the bytecode VM executes
`OpCall`/`OpClosure` directly; pure native PE has no VM or callbacks, so those
opcodes are outside its safe set. Pure-native execution that reaches them is
rejected at build time or terminates with status 70 on a runtime bailout.

Programs that import any `std.*` module are never built as a pure native PE.
The build uses the runtime stub, which bundles the VM, so stdlib calls remain
available. Pure native PE selection also requires every opcode to be in the
native-safe set; calls, module imports, collections, container access and
mutation, conversions, streams, strict/deep equality, and math intrinsics are
excluded. The builder additionally rejects boxed constants such as strings,
because their value representation needs the Go runtime.

A pure native PE has no VM to resume after a deoptimization. If native
execution nevertheless reaches a runtime bailout or fault, the executable
prints an explicit diagnostic on stderr and exits with a nonzero status (70);
it never exits 0 as if the program had completed. This is a deliberate hard
failure, not a fallback.

Native code writes stack slots directly instead of going through the VM's
bounds-checked push helper. To keep that safe, the AMD64 backend refuses to
push a slot once the VM stack is full: the frame bails out to the bytecode VM,
which reports the overflow exactly as it does for interpreted code. A pure
native PE applies the same guard and turns the overflow into the explicit
deoptimization diagnostic and exit status 70 instead of writing outside the
stack.

Tooling can query the same classification instead of guessing from module
names:

```go
support := jit.ClassifyStdlibMember("math", "sqrt", jit.StdlibAMD64JIT)
// support.Tier == jit.StdlibNativeTier

support = jit.ClassifyStdlibMember("string", "upper", jit.StdlibAMD64JIT)
// support.Tier == jit.StdlibJITHelperTier

support = jit.ClassifyStdlibMember("urllib2", "request", jit.StdlibAMD64JIT)
// support.Tier == jit.StdlibJITHelperTier (host call, still not machine code)

rows := jit.StdlibSupportTable(jit.StdlibPureNative)
// every row is jit.StdlibUnavailableTier
```

`StdlibDeoptTier` is reserved for call sites that cannot use a runtime helper,
such as a module binding replaced by a tree-walking `FuncValue` or a namespace
value. A module binding replaced by a compiled function is handled by the VM
call helper. The table classifies known members by their normal registered
implementation.

The table covers every member in the canonical signature list and accepts the
documented aliases (`regex`, `str`, `zip`) plus the `std.` prefix.

## Capability policy

The CLI and standalone applications install a deny-all policy before running
a program. The following operations are checked at call time on both engines:

- `io`, `csv`, `ini`, `archive`, `log.set_file`, and format helpers require
  `--allow-read` or `--allow-write` for the relevant path. This includes
  `io.open`, streams, globbing, directory operations, and archive extraction.
- `net` requests, DNS lookups, TCP connections/listeners, downloads, and
  `metrics.serve` require `--allow-net`. HTTP redirects are checked against
  the policy before being followed.
- `db.sqlite`, `db.open("sqlite", ...)`, and `db.open("postgres", ...)` enforce
  the same file and network rules. A writable SQLite database requires both
  read and write grants; a `mode=ro` or `immutable=1` DSN needs only read.
  PostgreSQL URL and keyword DSNs are checked against `--allow-net` for their
  host and port (defaults: `localhost:5432`). SQL statements themselves are not
  parsed: a granted SQLite connection can use SQL features such as `ATTACH`, so
  only grant database access to trusted programs.

`process`, the global `exec` function, `env`, `ffi`, `mem`, `gui`, and `sys`
expose host resources (command execution, environment mutation, native calls,
raw memory, UI, and machine information).
They are intentionally outside the read/write/network capability model: only
run trusted programs with access to those modules, or omit them from a
restricted build. The `mem` module limits a program to the memory it allocated
itself, so it is not a way around the policy, but it is still low level code.
The members of `mtproto` that open a connection need the network capability;
the rest of the module works without any grant.

Denied operations return ulang++ error code `403`, unwrap to
`os.ErrPermission` in Go, and are handled by `io.is_permission_error` in ulang++.



## --- Documentation Section: cli_EN.md ---


# Command-Line Interface (CLI)

Build the CLI binary with:
```bash
go build -o ulang++ ./cmd/ulang++
```

## Commands Overview

```text
ulang++ Programming Language CLI v1.7.9

Usage:
  ulang++ <command> [arguments]

Commands:
  run <file.upp> [args...]     Run a ulang++ program (Bytecode VM with JIT acceleration)
  build <file.upp> [-o out]    Compile program into a standalone native executable
        --protect, -p         protect binary: AES-256-GCM encryption + symbol obfuscation
        --noconsole, --gui    build a GUI application without opening a console window
        --target os/arch      cross-compile (e.g. linux/amd64, windows/arm64, darwin/arm64)
        --runtime <file>      use a specific runtime binary for the target
        --source              embed source instead of bytecode
        --embed <path>        embed a project file or directory (repeatable)
  init [name]                 Initialize a new ulang++ project
  dll <src.c|cpp|al> [-o out] Compile native C/C++ or ulang++ source into a shared library (.dll / .so)
  test <file.upp>              Run test blocks and report PASS/FAIL
  check <file.upp>             Syntax + strict type check (CI friendly)
  fmt [-w] <file.upp>          Format the program (-w rewrites the file in place)
  repl                        Interactive session
  targets                     List build targets and which runtimes are installed
  build-runtimes [--targets a,b] [--out dir] [--tags t]
                              Build runtimes for cross-compilation (needs the Go toolchain)
  lsp                         Language Server Protocol daemon for editors
  version | help
```

---

## 1. `ulang++ run`

```bash
ulang++ run [--vm|--interp|--jit] [capability flags] <file.upp> [script args...]
```

- **Default engine**: Bytecode VM. High-speed register-based VM. If the code uses interpreter-only dynamic features (e.g. dynamic AST streaming), it cleanly falls back with a diagnostic note.
- `--vm`: Forces the virtual machine. Rejects non-VM constructs with an error code.
- `--interp`: Forces the reference AST tree-walking interpreter.
- `--jit`: Enables multi-tier AMD64 JIT compilation for hot loops and functions.
- Memory limits:
  - `--max-memory=<size>`: Maximum resident memory (e.g. `128mb`, `1gb`).
  - `--max-allocation=<size>`: Maximum cumulative memory allocations.
- Script arguments after `<file.upp>` are accessible inside the program via `cli.args()` from `std.cli`.

---

## 2. `ulang++ check`

```bash
ulang++ check <file.upp>
```

Parses and type-checks a file without executing it. Returns exit code `0` on clean check; prints compiler error diagnostics with line/column pointers and returns exit code `1` on failure. Ideal for CI pipelines and pre-commit hooks.

---

## 3. `ulang++ test`

```bash
ulang++ test [capability flags] <file.upp>
```

Discovers and executes top-level code and all `test "name" { ... }` blocks. Reports test count, passes, and failures. Exits with non-zero status if any assertion (`assert`) or test block fails.

---

## 4. `ulang++ build`

```bash
ulang++ build <file.upp> [-o output] [--protect] [--gui] [--target os/arch] [--source] [--embed <path>]
```

Compiles a ulang++ program into a standalone, single-file native executable without external compiler dependencies:
- **Windows**: Built-in native PE/COFF builder.
- `--protect`, `-p`: Encrypts payload with AES-256-GCM and obfuscates symbols.
- `--noconsole`, `--gui`: Sets the PE subsystem to GUI mode (avoids showing a console window on launch).
- `--source`: Embeds original source code and local imports instead of compiled VM bytecode.
- `--embed <path>`: Packages assets, templates, or databases directly into the executable (accessible at runtime via `use std.embed; embed.read(...)`). Can be repeated for multiple files/directories.

---

## 5. `ulang++ dll`

```bash
ulang++ dll <source> [-o output.dll]
```

Compiles native C/C++ code or ulang++ exported symbols into a native dynamic library (`.dll` on Windows, `.so` on Linux) with zero third-party toolchains.

---

## 6. `ulang++ fmt`

```bash
ulang++ fmt [-w|--write] <file.upp>
```

Formats ulang++ code using canonical rules (4-space indentation, consistent statement layout, comment preservation). With `-w`, rewrites the file in place. Round-trips through the parser AST before writing to guarantee safety.

---

## 7. `ulang++ repl`

```bash
ulang++ repl [capability flags]
```

Interactive REPL shell with incremental static type checking, multi-line blocks, persistent environment, and immediate expression evaluation. Type `exit` or `quit` to leave.

---

## 8. `ulang++ lsp`

```bash
ulang++ lsp
```

Starts an LSP (Language Server Protocol) server over standard input/output. Compatible with VS Code, Neovim, Helix, and Sublime Text. Provides completions, diagnostics, and hover information directly from `pkg/typecheck` and AST tables.

---

## 9. Capability Sandbox Flags

By default, ulang++ programs execute in a restricted sandbox. The following flags explicitly grant permissions:

| Flag | Description |
| --- | --- |
| `--allow-read` | Allow reading any file on the filesystem. |
| `--allow-read=<path>` | Allow reading only the specified file or directory subtree. |
| `--allow-write` | Allow writing/creating/deleting any file. |
| `--allow-write=<path>` | Allow writing only within the specified path or subtree. |
| `--allow-net` | Allow outbound and inbound network connections anywhere. |
| `--allow-net=<host[:port]>` | Allow network access only to a specific host, port, or wildcard domain (`*.example.com`). |
| `--allow-run` | Allow spawning subprocesses via `exec.Command` / `std.process`. |
| `--allow-run=<cmd>` | Allow executing only the specified binary or executable name. |
| `--allow-env` | Allow reading and writing all environment variables. |
| `--allow-env=<var>` | Allow access only to the specified environment variable. |
| `--allow-ffi` | Allow loading native dynamic libraries via FFI. |
| `--allow-ffi=<lib>` | Allow loading only the specified library path. |
| `--allow-all` | Unconditionally grant all capabilities (read, write, net, run, env, ffi). |
| `--` | Terminates CLI flag processing; following tokens are passed to script arguments. |

Attempted access without the required capability raises a ulang++ runtime error (`code 403`) and logs an audit record via the security subsystem.



## --- Documentation Section: architecture_EN.md ---


# Architecture

ulang++ features a multi-tiered execution architecture fed by a single front end:

1. **Bytecode VM (`pkg/vm`)**: The primary high-performance stack-based virtual machine with lexical upvalues, inline caching, and superinstructions.
2. **Interpreter (`pkg/interpreter`)**: The tree-walking reference semantic engine and fallback evaluator.
3. **JIT Compiler (`pkg/jit`)**: Tiered compilation specializing hot function templates into native code.
4. **Native Standalone Compiler (`cmd/ulang++/native_build.go`)**: Pure Go standalone Windows PE executable and DLL generator.

## Pipeline

```text
source .upp
  -> pkg/lexer        tokens (braces, comments, literals, operators)
  -> pkg/parser       Pratt parser, AST (pkg/ast), structured diagnostics
  -> pkg/typecheck    strict bidirectional types, scopes, stdlib signature tables
  -> Execution Tier:
       ├── pkg/vm           bytecode compiler + VM (default)
       ├── pkg/interpreter  tree-walking evaluator (reference semantics)
       ├── pkg/jit          tiered template-specialized JIT
       └── Native AOT       embedded standalone PE binary or native DLL
```

All engines share runtime builtins and types:

- `pkg/stdlib` registers modules and top-level builtins across 20+ domains.
- `pkg/interpreter` currently houses runtime values, environments, and core builtins.
- `pkg/vm` calls into the shared builtin environment, ensuring strict semantic parity.

## Parser & Syntax

`pkg/parser` is a Pratt parser recording structured diagnostics with source positions.
- **Blocks**: Canonical syntax uses curly braces `{ ... }`. Python-style `:` and indentation are supported for backward compatibility.
- **Declarations**: `fn name(params) -> ReturnType { ... }` or `pub fn`. Variables use `name: Type = val;` or `name := val;`.
- **Constructs**: Strict type checking ensures that every AST node has unambiguous semantics across all execution tiers.

## Type Checker

`pkg/typecheck` is a dedicated static analysis pass with no runtime side effects. It enforces:

- Fixed inferred types with zero implicit numeric or string conversions.
- Nullable type narrowing after `!= null` and `== null` guards.
- Declared function and stdlib signatures, verifying optional, variadic, and nullable parameters.
- No variable shadowing: redeclaring an existing symbol in the same scope is an error.
- Strict condition typing: conditional expressions must evaluate strictly to `bool`.
- Dynamic typing containment: `json` is the sole dynamic type; any dynamic field access requires explicit type conversion.

`pkg/typecheck/builtins_table.go` is the canonical standard library signature table shared between the checker, LSP, and completion tooling.

## Concurrency & Async/Await

ulang++ provides two levels of concurrency:

1. **Production Concurrency (`@ba` and `@threads`)**:
   - `@ba { ... }` executes isolated background asynchronous tasks with optional timeout and memory limit guards.
   - `@threads(N) { ... }` executes tasks across a managed worker pool.
2. **`async` / `await` Syntax**:
   - The keywords `async` and `await` are parsed, checked by the static typechecker, and executed cooperatively by both the Interpreter and Bytecode VM.
   - A full non-blocking Promise/Future event-loop runtime is scheduled for v2.1. In v2.0, use `@ba` and `@threads` for production background execution.

## Execution Tiers

### 1. Bytecode VM
`pkg/vm` compiles the AST to bytecode instructions and runs a fast stack machine:
- Closures use upvalues (`OpClosure`, `OpGetFree`, `OpSetFree`) to capture lexical environments by reference.
- Any construct not yet supported by the bytecode compiler explicitly returns `*vm.UnsupportedError`. It never silently skips statements or alters semantics.
- `ulang++ run` automatically falls back to the reference interpreter if the VM encounters an unsupported construct; `ulang++ run --vm` explicitly reports the error.

### 2. JIT Engine
The JIT compiler (`pkg/jit`) compiles closures and function templates once into native machine code. It shares compiled artifacts across instances of the same function template while keeping captured closure values instance-specific. Artifact counts are strictly bounded by `jit.Config.MaxArtifacts`.

### 3. Native AOT & Standalone Bundles
`cmd/ulang++/native_build.go` and `pkg/compiler` produce standalone binaries:
- Generates valid Windows Portable Executable (PE) 32/64-bit binaries and dynamic-link libraries (.dll) directly from Go without external C compilers.
- Embedded runtime payloads are compressed with gzip and verified with SHA-256.

## Security & Sandbox Policy

`pkg/sandbox` implements a fine-grained, capability-based security model. By default, untrusted scripts run in a restricted environment:

- `--allow-read[=<path>]`: Grants read access to the filesystem (or specific subtrees).
- `--allow-write[=<path>]`: Grants write access to the filesystem (or specific subtrees).
- `--allow-net[=<host[:port]>]`: Grants network socket and HTTP client/server access.
- `--allow-run[=<command>]`: Grants permission to spawn subprocesses (`exec.Command`).
- `--allow-env[=<variable>]`: Grants permission to read/write environment variables (`os.Getenv`).
- `--allow-ffi[=<library>]`: Grants permission to load native dynamic libraries (.dll, .so).
- `--allow-all`: Unconditionally grants all capabilities.

### Threat Model & Limitations
- The capability sandbox strictly gates calls into standard library facilities (filesystem, network, subprocesses, environment variables, FFI).
- While ulang++'s managed runtime is memory-safe (preventing buffer overflows and dangling pointers within ulang++ code), granting `--allow-ffi` allows loading arbitrary C libraries and manipulating raw native pointers, which operates outside Go's memory-safety guarantees.
- Subprocess execution (`--allow-run`) delegates control to host shell binaries.

## Engine Parity & Testing

Engine parity is the top development priority:
- `cmd/ulang++/engine_test.go`: Runs an extensive corpus against both the Bytecode VM and Interpreter, asserting identical stdout, evaluation results, and exit codes.
- `cmd/ulang++/examples_test.go`: Statically checks all shipped examples and verifies determinism across engines.



## --- Documentation Section: handbook_EN.md ---


# Comprehensive ulang++ Programming Language Handbook

[English](handbook_EN.md) | [Русский](handbook_RU.md)

> **Language Version:** ulang++ v1.7.9  
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

**ulang++** is a modern statically-typed general-purpose programming language engineered for standalone systems automation, high-performance tooling, and cross-platform native applications.

### Core Architectural Pillars
- **Zero-Toolchain Distribution:** Builds self-contained native Windows PE/EXE executables (`ulang++ build`) and shared DLL libraries (`ulang++ dll`) without requiring MSVC, MinGW, or Clang.
- **Four-Tier Execution Hierarchy:**
  1. *Bytecode Virtual Machine (VM)*: High-throughput virtual CPU with register/stack frames.
  2. *Multi-Tier AMD64 JIT*: Native machine code compilation for hot loops and functions.
  3. *Reference Tree-Walking Interpreter*: Full dynamic introspection and meta-programming support.
  4. *Native PE / DLL AOT Generator*: Direct byte-level PE/COFF assembler.
- **Capability-Based Sandbox:** Strict default-deny access model governing the filesystem, network sockets, subprocess execution, environment variables, and native dynamic libraries.

---

## 2. Syntax, Lexical Structure, and Program Layout

ulang++ source files use the `.upp` extension.

### 2.1 Statement Termination and Blocks
- Simple statements terminate with a semicolon (`;`).
- Code blocks are enclosed either in **canonical curly braces `{ ... }`** or a colon (`:`) followed by a 4-space indented suite:

```ulang++
// Canonical curly braces (recommended)
fn max(a: int, b: int) -> int {
    if a > b {
        return a
    }
    return b
}

// Colon and indentation syntax
fn min(a: int, b: int) -> int {
    if a < b {
        return a
    }
    return b
}
```

### 2.2 Comments
```ulang++
// Single-line comment

/*
  Multi-line
  comment block
*/
```

### 2.3 Module Imports
The `use` keyword imports standard library modules or relative local scripts:
```ulang++
use std.math
use std.io
use "./helpers.upp"
```

---

## 3. Type System and Safety

ulang++ enforces **strict static typing** with bidirectional inference. Implicit widening, narrowing, or silent coercions are strictly prohibited.

### 3.1 Primitive Types
- `int`: 64-bit signed integer.
- `float`: 64-bit IEEE 754 double-precision floating-point number.
- `string`: Immutable UTF-8 character sequence.
- `bool`: Boolean value (`true`, `false`).
- `void`: Unit return type for functions producing no value.

### 3.2 Declarations and Type Inference
```ulang++
// Explicit type annotations
counter: int = 0
pi: float = 3.14159265
title: string = "ulang++ Language"
is_ready: bool = true

// Type inference via `:=` (type is fixed permanently at compile time)
ratio := 2.5                // float
names := ["Alice", "Bob"]   // list[string]

// Shadowing within the same or nested scope is rejected:
// counter: int = 5;         // COMPILE ERROR: variable 'counter' already declared
counter = 5                 // OK: mutation / assignment
```

### 3.3 Strict Arithmetic
Numbers do not coerce silently:
```ulang++
x: int = 10
y: float = 2.5

// total := x + y;            // COMPILE ERROR: operator '+' cannot be applied to int and float
total := float(x) + y        // OK: 12.5

int_res := 7 / 2             // Integer division truncates towards zero: 3
float_res := float(7) / 2.0  // Floating-point division: 3.5
```

### 3.4 Null-Safety and Type Narrowing
Values of type `T` cannot be `null`. Nullable values must be explicitly declared as `T | null`:
```ulang++
user_name: string | null = null

// Direct member access on nullable types is rejected by typecheck:
// len(user_name);            // COMPILE ERROR: value may be null

// Safe narrowing within conditional checks:
if user_name != null {
    // Within this block, user_name is safely narrowed to string
    println("Length: " + string(len(user_name)))
}
```

### 3.5 Dynamic Escape Hatch: `json`
`json` is the sole dynamic type in ulang++, designed for unstructured external data (e.g. REST APIs). Values must be explicitly converted before use in typed contexts.

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
```ulang++
users := ["Alex", "Maria", "Ivan"]
for u in users {
    if u == "Maria" {
        continue
    }
    println("User: " + u)
}

n := 5
while n > 0 {
    n = n - 1
}
```

### 4.5 Functional Pipelines (Pipeline Operator `|>`)
The `|>` operator forwards the left expression as the **first argument** to the function on the right, eliminating nested function calls:

```ulang++
fn double(x: int) -> int { return x * 2
 }
fn add(x: int, y: int) -> int { return x + y
 }
fn to_str(x: int) -> string { return "Result: " + string(x)
 }

// Single argument chaining:
val := 10 |> double // 20

// Multi-argument function pipeline (first argument passed automatically):
res := 5 |> double |> add(3) |> to_str
// Equivalent to: to_str(add(double(5), 3)) -> "Result: 13"
```

### 4.6 Stream Pipelines
ulang++ provides native I/O stream operators compiled directly into virtual machine opcodes (`OpStream`):

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
```ulang++
// 1. Stream file write and append:
"System initializing..." >! "app.log"        // Overwrite (>!)
"[INFO] Database connected" >> "app.log"     // Append (>>)

// 2. Stream read (<<):
config_str << "settings.json" // Read file into config_str
user_name << console          // Read line from stdin

// 3. Stream save into variable (>>>):
(math.sqrt(144.0) * 2.0) >>> computed_res
assert computed_res == 24.0

// 4. Network stream POST:
"{\"status\": \"healthy\"}" >> "https://monitoring.internal/api/beat"

// 5. Non-blocking async stream append (&>>):
heavy_audit_dump &>> "audit.log"

// 6. Discard stream output:
debug_dump >> "/dev/null" // also accepts "null" and "NUL"

// 7. Error capture and redirection:
risky_task() ?>> "errors.log"

failure: error | null = null
read_file("missing.txt") ?>>> failure
if failure != null {
    println("Error caught: " + failure.msg)
}
```

---

## 5. Functions, Closures, and Concurrency

### 5.1 Function Declarations
```ulang++
pub fn calculate_discount(price: float, rate: float = 0.1) -> float {
    return price * (1.0 - rate)
}

void log_event(event: string) {
    println("[LOG] " + event)
}
```

### 5.2 Closures and Higher-Order Functions
```ulang++
fn make_prefixer(prefix: string) -> func {
    fn format(msg: string) -> string {
        return prefix + ": " + msg
    }
    return format
}

logger := make_prefixer("[SYSTEM]")
println(logger("Startup OK")) // "[SYSTEM]: Startup OK"
```

### 5.3 Asynchronous Execution (`async` / `await`)
Both the Bytecode VM and Interpreter provide full parity for cooperative asynchronous execution:
```ulang++
async fn load_remote_config(url: string) -> string {
    return "{\"status\": \"ok\"}"
}

cfg := await load_remote_config("https://config.server.local")
```

### 5.4 Concurrency: Background Workers (`@ba`) and Thread Pools (`@threads`)
- `@ba` spawns an isolated background worker:
  ```ulang++
  ba(timeout=10s) {
      println("Background synchronization completed")
  }
  ```
- `@threads` distributes computation across system threads:
  ```ulang++
  threads 8 {
      println("Parallel task processing")
  }
  ```

---

## 6. Object-Oriented and Contract Programming

### 6.1 Structs
```ulang++
struct Point {
    x: int
    y: int = 0
}

pt := Point(10, 20)
pt.x = 25
```

### 6.2 Classes and Single Inheritance
```ulang++
class Vehicle {
    pub brand: string
    private mileage: int = 0

    pub fn drive(km: int) -> void {
        self.mileage = self.mileage + km
    }

    pub fn get_mileage() -> int {
        return self.mileage
    }
}

class ElectricCar extends Vehicle {
    pub battery_level: int = 100

    pub fn drive(km: int) -> void {
        super.drive(km)
        self.battery_level = self.battery_level - (km / 10)
    }
}

tesla := ElectricCar("Tesla", 0, 100)
tesla.drive(50)
assert tesla.get_mileage() == 50
assert tesla.battery_level == 95
```

### 6.3 Enums and Pattern Matching
```ulang++
enum ConnectionState {
    Disconnected,
    Connecting,
    Connected = 10,
}

state := ConnectionState.Connected
match state {
    case ConnectionState.Disconnected {
        println("No connection")
    }
    case ConnectionState.Connecting {
        println("Connecting...")
    }
    case ConnectionState.Connected {
        println("Connected!")
    }
    else {
        println("Unknown state")
    }
}
```

### 6.4 Interfaces
```ulang++
interface StorageDriver {
    fn write_chunk(data: string) -> bool
    fn read_chunk() -> string
}

class LocalDiskStorage {
    pub fn write_chunk(data: string) -> bool {
        return true
    }
    pub fn read_chunk() -> string {
        return "data"
    }
}

driver: StorageDriver = LocalDiskStorage()
driver.write_chunk("payload")
```

---

## 7. Error Handling and Resilience

Instead of unhandled exception unwinding, ulang++ uses explicit error values and stream operators.

### 7.1 Error Capture with `?>>>`
```ulang++
use std.io

failure: error | null = null
data := io.read_file("nonexistent.txt") ?>>> failure

if failure != null {
    println("Code: " + string(failure.code))
    println("Message: " + failure.msg)
} else {
    println("Content: " + data)
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
```ulang++
print("Processing:", 100)
println("Done!", 200, true)
name := input("Enter name: ")
```

### 8.2 Type and Size Inspection
```ulang++
assert len("Hello") == 5
assert len([1, 2, 3]) == 3
assert type(42) == "int"
assert type("text") == "string"
assert type(null) == "null"
```

### 8.3 Type Casts
```ulang++
assert int(4.99) == 4
assert int("123") == 123
assert float(10) == 10.0
assert string(100) == "100"
assert bool(1) == true
```

### 8.4 Collection Operations (Zero-Import)
```ulang++
items := [10, 20]
new_list := append(items, 30) // [10, 20, 30]
push(items, 30)                // In-place mutation
last := pop(items)             // 30
k := keys({"a": 1, "b": 2})    // ["a", "b"]
v := values({"a": 1, "b": 2})  // [1, 2]
r := range(1, 5)               // [1, 2, 3, 4, 5]
```

### 8.5 Math & String Helpers
```ulang++
assert abs(-50) == 50
assert max(10, 25) == 25
assert min(10, 25) == 10

assert str_trim("  hello  ") == "hello"
assert str_upper("ulang++") == "ulang++"
assert str_lower("ulang++") == "ulang++"
assert str_contains("ulang++ lang", "lang") == true
assert str_starts_with("app.upp", "app") == true
assert str_ends_with("app.upp", ".upp") == true
assert len(str_split("a,b,c", ",")) == 3
assert str_replace("foo bar foo", "foo", "qux") == "qux bar qux"
```

### 8.6 Zero-Import JSON and `std.json`
ulang++ provides both zero-import global JSON helpers and a full-featured standard module `std.json`:

```ulang++
// Zero-import globals:
data := {"status": 200, "ok": true}
json_str := json_encode(data)
parsed := json_decode(json_str)
assert parsed["status"] == 200
pretty := json_prettify(data)
q_res := json_query(parsed, "status")
assert q_res == 200

// Modular standard library (use std.json):
use std.json
doc := json.parse("{\"score\": 98}")
assert json.validate("{\"score\": 98}") == true
compact := json.encode(doc)
formatted := json.prettify(doc, "  ")
```

### 8.7 Process and System Helpers
```ulang++
sleep(10) // Sleep for 10ms
ts := time_now() // Current UTC timestamp in RFC3339 format

// Subprocess execution (requires --allow-run):
// out := exec("hostname");

// Environment variables are accessed via std.env:
use std.env
h := env.get_env("PATH")
env.set_env("PORT", "8080")
```

### 8.8 Dunder Variables
| Variable | Type | Description |
| --- | --- | --- |
| `__file__` | `string` | Absolute path to current source script. |
| `__dir__` / `__dirname__` | `string` | Directory of current source file. |
| `__line__` | `int` | Current source line number. |
| `__name__` | `string` | Module name or `"__main__"`. |
| `__version__` | `string` | ulang++ runtime version string. |

### 8.9 Standard Library Namespaces (Former Global Aliases Deprecation)
To maintain strict namespace hygiene, deterministic static analysis, and capability-based security boundaries, convenience aliases are not present in global scope. Calling former global aliases triggers compile-time deprecation diagnostics pointing to their respective modules:

- **Filesystem (`use std.io;`)**: `io.read_file`, `io.write_file`, `io.append_file`, `io.file_exists`, `io.file_size`, `io.delete_file`, `io.list_dir`, `io.make_dir`.
- **Cryptography (`use std.crypto;`)**: `crypto.uuid`, `crypto.random_string`, `crypto.random_int`, `crypto.hex_encode`, `crypto.hex_decode`, `crypto.sha256`, `crypto.sha512`.
- **Process & Metrics (`use std.process;`)**: `process.cpu_count`, `process.memory_usage`, `process.uptime`, `process.exec_cmd`, `process.exec_bg`.
- **Environment (`use std.env;`)**: `env.home_dir`, `env.hostname`, `env.username`, `env.is_windows`, `env.is_linux`, `env.get_env`, `env.set_env`.

---

## 9. Capability-Based Sandbox & Security

ulang++ enforces explicit capability permissions when running scripts (deny-by-default):

| Flag | Meaning | Example |
| --- | --- | --- |
| `--allow-read` | Read any file on filesystem | `ulang++ run --allow-read app.upp` |
| `--allow-read=<path>` | Read within specific path subtree | `ulang++ run --allow-read=./data app.upp` |
| `--allow-write` | Write any file on filesystem | `ulang++ run --allow-write app.upp` |
| `--allow-write=<path>` | Write within specific path subtree | `ulang++ run --allow-write=./logs app.upp` |
| `--allow-net` | Connect/listen to any network socket | `ulang++ run --allow-net app.upp` |
| `--allow-net=<host>` | Access specific host/port | `ulang++ run --allow-net=api.site.com:443 app.upp` |
| `--allow-run` | Execute subprocesses (`exec`) | `ulang++ run --allow-run app.upp` |
| `--allow-run=<cmd>` | Allow specific binary name | `ulang++ run --allow-run=git app.upp` |
| `--allow-env` | Read and write environment variables | `ulang++ run --allow-env app.upp` |
| `--allow-env=<var>` | Access specific environment variable | `ulang++ run --allow-env=PORT app.upp` |
| `--allow-ffi` | Load native dynamic libraries | `ulang++ run --allow-ffi app.upp` |
| `--allow-all` | Grant all capabilities | `ulang++ run --allow-all app.upp` |

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
ulang++ run --allow-all app.upp
ulang++ check app.upp # Strict syntax and type checker (CI)
```

<a id="112-unit-tests"></a>
### 11.2 Unit Tests
```bash
ulang++ test tests/test_01_strings_text.upp
```

Writing test blocks:
```ulang++
test "Core business logic" {
    assert 2 + 2 === 4
    assert "ulang++" + "_lang" == "pawl_lang"
}
```

<a id="113-standalone-executables-windows-peexe"></a>
### 11.3 Standalone Executables (Windows PE/EXE)
```bash
ulang++ build main.upp -o myapp.exe --protect --gui
```

<a id="114-native-dynamic-libraries-dll"></a>
### 11.4 Native Dynamic Libraries (DLL)
```bash
ulang++ dll lib.upp -o mylib.dll
```

<a id="115-package-management-apm"></a>
### 11.5 Package Management (APM)
ulang++ includes the official package manager **APM** (`apm`) to manage dependencies, lockfiles, and registry publishing:
```bash
apm init                       # Initialize ulang++.toml manifest
apm install pkg_name@^1.2.0    # Install package with SemVer resolution
apm publish                    # Publish archive to registry
```
See the full guide in [apm_EN.md](apm_EN.md).

<a id="116-ide--editor-integration-lsp"></a>
### 11.6 IDE & Editor Integration (LSP)
ulang++ features a built-in Language Server Protocol daemon and editor extensions:
```bash
ulang++ lsp                       # Run LSP server on stdio
```
First-party editor packages are provided in `editors/vscode` (VS Code & Cursor) and `editors/zed` (Zed Editor). See the full setup instructions in [editors_EN.md](editors_EN.md).



## --- Documentation Section: cross-compilation_EN.md ---


# Cross-compilation

ulang++ builds standalone executables without requiring a target toolchain at
build time. The build machine only needs the prebuilt ulang++ runtime stubs.

## How the output is assembled

```text
[target runtime stub] + [ulang++ payload] + [uint64 payload length] + [PAWL_EXEC_V2 magic]
```

The runtime stub is `cmd/ulang++` compiled for the target OS/architecture. It
contains the VM, interpreter, and standard library. The payload is:

- VM bytecode by default (source text is not shipped); or
- source plus imported `.upp` modules when requested explicitly via `--source`,
  when the program imports local files, or when it uses a construct the VM cannot compile.

Additionally, external resources (files or directories) can be packed directly into the standalone binary using one or more `--embed <path>` flags. At runtime, embedded assets are accessible via `use std.embed; embed.read("path")` or `embed.list()`.

The payload envelope is versioned and checksummed with SHA-256. The runtime
verifies the checksum before decoding. A corrupted or truncated payload is
rejected with a diagnostic instead of executing partially.

## Building runtime stubs

From the repository root, with the Go toolchain installed:

```text
go run ./cmd/ulang++ build-runtimes --out runtimes
```

This builds:

| Target | Runtime file |
| --- | --- |
| `linux/amd64` | `runtimes/ulang++-linux-amd64` |
| `linux/arm64` | `runtimes/ulang++-linux-arm64` |
| `darwin/amd64` | `runtimes/ulang++-darwin-amd64` |
| `darwin/arm64` | `runtimes/ulang++-darwin-arm64` |
| `windows/amd64` | `runtimes/ulang++-windows-amd64.exe` |
| `windows/arm64` | `runtimes/ulang++-windows-arm64.exe` |

The stubs are generated artifacts and are excluded by `.gitignore`. Rebuild
them after changing payload format, standard library, or runtime security
policy.

Useful options:

```text
--out <dir>          output directory (default: runtimes)
--targets a,b        subset, e.g. linux/amd64,windows/arm64
--tags nodb          build runtimes without database drivers
```

## Building an application

```text
ulang++ build app.upp --target linux/amd64 -o app
ulang++ build app.upp --target windows/arm64 -o app
```

If `runtimes/` is present, discovery finds it automatically. Explicit lookup
order for non-host targets:

1. `--runtime <file>`
2. `$PAWL_RUNTIMES`
3. `<directory of ulang++>/runtimes`
4. `<directory of ulang++>`
5. `~/.ulang++/runtimes`
6. `./runtimes`

The host target always uses the currently running `ulang++` executable, even if a
different stub exists in a runtime directory. This guarantees that host builds
match the CLI's payload format and security policy.

`ulang++ targets` shows which target runtimes are available.

## Verification

The integration test `cmd/ulang++/build_test.go`:

- builds a deterministic program for all six targets through `cmdBuild`;
- checks the target executable header (ELF, PE, or Mach-O);
- extracts and decodes each embedded payload and verifies the program string;
- runs every decoded bytecode payload on the host VM and compares its stdout
  with `ulang++ run` on both engines;
- executes the host artifact and compares its stdout with both engines.

A separate test builds a host application and verifies that it denies reads and
network access without capability flags and accepts `--allow-read` /
`--allow-net`. On a machine without Linux/macOS execution support, non-host
artifacts are verified structurally and by running their platform-independent
VM payload; direct execution still requires a matching OS, container, or
emulator.

## Security of built applications

Standalone applications enforce the same capability policy as `ulang++ run`:
file and network access are denied unless flags are passed to the application:

```text
app --allow-read=./data --allow-net=api.example.com
```

Flags must come before script arguments. `--` ends flag parsing if a script
argument begins with a dash.



## --- Architecture Decision Records (ADRs) ---



### ADR: 0001-value-representation-and-abi.md


# ADR-0001: Value representation and calling convention

- Status: Accepted
- Date: 2026-11-25
- Deciders: ulang++ native migration

## Context

The native compiler must support `int` (64-bit), `float` (IEEE-754 binary64),
`bool`, `string`, `list`, `map`, `blob`, `secret`, `json`, `struct`, `class`,
`enum`, `interface`, closures, errors and `null`, on x86-64 and AArch64, with
one backend-agnostic IR. The representation determines codegen, GC, runtime
and stdlib bindings, so it must be fixed before the IR is lowered.

Constraints:
- no libc/CRT in the produced binary;
- no VM, bytecode or interpreter;
- Windows and Linux, x86-64 and ARM64;
- predictable ABI across object images produced by the same compiler.

## Decision

Use a **16-byte tagged value**, two machine words:

```c++
struct Value {          // alignof == 16, sizeof == 16
    std::uint64_t tag;  // kind + flags
    std::uint64_t payload;
};
```

`tag` layout:

```text
bits 63..56  flags   (immutable, heap, weak, has-error, reserved)
bits 55..48  reserved (must be zero)
bits 47..0   kind
```

Initial kinds:

| Kind | Payload |
|---|---|
| `Null` | 0 |
| `Bool` | 0 / 1 |
| `Int` | two's-complement int64 |
| `Float` | IEEE-754 binary64 bit pattern |
| `String` | pointer to `StringObject { u64 len; u64 cap; u8 data[]; }` |
| `List` | pointer to `ListObject` (elements are `Value`) |
| `Map` | pointer to `MapObject` (open-addressing, hash of Value) |
| `Blob` | pointer to `BlobObject { u64 len; u8 data[]; }` |
| `Struct`/`Class` | pointer to object with type id + fields |
| `Closure` | pointer to `ClosureObject` (code ptr + captured env) |
| `Error` | pointer to `ErrorObject` (code + message + cause) |
| `Json` | pointer to a typed JSON tree |

All heap pointers are 16-byte aligned; the low 4 bits are available for future
inline tagging but are not used in v1.

### ABI

- Every ulang++ value is passed as two 8-byte words.
- x86-64 SysV: first six integer-class argument slots in `rdi, rsi, rdx, rcx,
  r8, r9`; a 16-byte value consumes two consecutive slots (a value never
  straddles the register/stack boundary). Return in `rax:rdx`; a second return
  channel (error code) uses `r10` or an out-pointer. Stack arguments are
  16-byte aligned.
- AArch64 AAPCS64: `x0..x7` for argument words, return in `x0:x1`, indirect
  result pointer in `x8` when needed. Stack arguments are 16-byte aligned.
- `bool` is still a 16-byte Value at the language boundary; internal IR may
  specialize it to one word.
- `float` is boxed as a `Value` at ABI boundaries. The x64/ARM64 backends may
  unbox arithmetic trees internally (`FloatAdd` etc.) as an optimization that
  is invisible to the ABI.
- Hidden first argument: execution context pointer (`*Runtime`) in `r15`
  (x64) / `x28` (ARM64) as a reserved global register, never a normal argument.
- Closures receive the captured environment pointer as an implicit argument.
- The runtime never returns a C++ object by value across generated code.

### Why not NaN-boxing (64-bit)

NaN-boxing is faster and halves memory, but:
- it steals float NaN space and makes signaling/NaN payload preservation subtle;
- pointer tagging conflicts with future 5-level x86-64 / 52-bit ARM64 VAs;
- representing a string needs an extra length word anyway, and classes need
  type metadata, so the saving is smaller than it looks;
- debugging native code without box metadata is materially harder during a
  multi-stage port.

We choose correctness and a simple ABI first. A future optimization may add an
unboxed fast path per function while keeping the 16-byte ABI.

### Why not 128-bit "struct with union + aux"

The chosen pair is the smallest representation that carries an arbitrary
64-bit immediate or pointer **and** an independent tag. It returns in two
registers on both targets, needs no hidden sret for the common case, and can be
serialized deterministically.

## Consequences

- IR must model `Value` explicitly; no `double`-only or pointer-only nodes.
- The runtime object header is shared by all heap kinds:
  `{ u32 kind; u32 flags; u64 size; u64 type_id; }`.
- Equality/hash/format helpers are runtime functions, not compiler-inlined
  layout assumptions.
- Golden ABI tests will assert register/stack placement on both backends.

## Alternatives considered

1. **64-bit NaN-boxing** — rejected for v1 (see above).
2. **Untagged 128-bit variant** — rejected: every operation would pay a tag
   dispatch anyway, and FFI/runtime boundaries would lose type safety.
3. **Pointer-to-boxed-value everywhere** — rejected: allocation on every
   integer operation.


### ADR: 0002-memory-model.md


# ADR-0002: Memory model and allocator

- Status: Accepted
- Date: 2026-11-25

## Context

The output binary must not link libc/CRT and must not depend on a Go runtime.
ulang++ programs allocate strings, lists, maps, objects, closures and errors.
The compiler itself can use the host C++ runtime, but generated code cannot.

## Decision

Use a **non-moving, precise mark-sweep collector** over a **size-class slab
allocator**, backed directly by OS virtual memory:

- Windows: `VirtualAlloc` / `VirtualFree` (`MEM_COMMIT | MEM_RESERVE`).
- Linux: `mmap` / `munmap` with `MAP_PRIVATE | MAP_ANONYMOUS`.

No `malloc`, `free`, `new`, `delete`, `mmap` wrapper from libc, no CRT startup.

### Object model

- Every heap object starts with a 32-byte header:
  `{ u32 kind; u32 flags; u64 size; u64 type_id; u64 mark; u64 reserved; }`.
- Allocation size classes: 32, 48, 64, 96, 128, 192, 256, 384, 512, 768, 1024,
  then whole pages for larger objects. Each class has a free list and a span
  list.
- Large objects (> 16 KiB) are page-aligned and tracked separately.
- Strings and blobs are immutable after construction; this permits sharing and
  makes hashing stable.
- Maps use open addressing with tombstones; list growth is amortized doubling.

### GC

- Stop-the-world mark-sweep. No moving, so raw pointers held by native handles
  and FFI remain valid.
- Roots: shadow stack of `Value` slots plus a per-frame stack map emitted by
  the codegen backend (precise). Registers are spilled at safepoints.
- Global roots: module globals, constants, runtime tables, active closures.
- Mark phase uses an explicit worklist (no recursion) with a mark bit in the
  object header.
- Sweep returns whole free spans to the OS when a span becomes empty.
- Collection is triggered by allocation bytes since last GC and a configurable
  heap limit; `ulang++` programs can call `gc.collect()`.
- No finalizers in v1; resources (files, sockets, handles) have explicit
  `close` and a deterministic `defer`/scope-exit mechanism.

### Arenas

- The compiler (C++) uses arenas and `std::unique_ptr` for AST/IR; this is
  host-only and unrelated to the generated runtime.
- Generated code may use a per-function bump arena for temporary strings/lists
  that provably do not escape; this is an IR optimization and is disabled in
  debug builds to keep GC stress tests meaningful.

### Threads and async

- Each OS thread has its own allocation buffer and shadow stack.
- A global stop-the-world handshake parks threads at safepoints.
- `async`/`ba` tasks are scheduled by a native runtime scheduler; task stacks
  are GC roots.

## Consequences

- Codegen must emit stack maps and safepoints. Until that exists, Stage 4 uses
  a conservative scan of the shadow stack only (all slots are `Value`-shaped),
  which is sound because the runtime knows every shadow slot is a `Value`.
- No destructors/finalizers means stdlib APIs need explicit `close`.
- ASan/UBSan apply to the compiler process, not to generated code; generated
  code gets its own redzone/canary debug mode.

## Alternatives considered

1. **Reference counting** — rejected: cycles through closures/classes are
   common in ulang++ programs; weak/cycle collection adds complexity.
2. **Copying/compacting GC** — rejected: moves objects, invalidates FFI
   pointers and native handles.
3. **Arena-only / never free** — rejected for long-running servers and GUI.
4. **libc malloc** — forbidden by the product requirements.


### ADR: 0003-error-model.md


# ADR-0003: Error model

- Status: Accepted
- Date: 2026-11-25

## Context

ulang++ has first-class errors (`err(...)`, `error` type, `?>>>` propagation) and
the runtime can fail (I/O, allocation, sandbox denial). Generated machine code
cannot unwind C++ exceptions, and the compiler must preserve ulang++-visible
error semantics.

## Decision

Separate three layers:

1. **Compiler internals (C++)** — may use exceptions and `std::expected`-style
   results freely. They never cross into generated code.
2. **Runtime helper ABI** — no C++ exceptions. Every helper returns a status
   word in a dedicated register and writes its result through an out-pointer:

   ```text
   status = runtime_call(ctx, args..., &out)
   status: 0 = OK, nonzero = RuntimeErrorCode
   ```

   On x64 the status is `rax`; on ARM64 `x0`. The out-pointer is a normal
   argument. A null status result is never a valid ulang++ value.
3. **ulang++-visible errors** — ordinary `Value` of kind `Error`, carrying
   `{ u32 code; String message; Value cause; u64 trace_id; }`. Functions that
   can fail still return a normal `Value`; `?>>>` checks the `Error` kind and
   returns early, matching the Go semantics.

Runtime helpers that can fail have a sibling `_or_error` form returning a
ulang++ `Error` value, used by stdlib bindings; low-level helpers use status codes
to keep the hot path branch-free.

### Panics

- Internal invariants and out-of-memory use `pawl_panic(code, message)`.
- `pawl_panic` writes a diagnostic to stderr, flushes, and terminates the
  process using `ExitProcess` (Windows) or `exit_group` (Linux).
- `panic` never returns; its IR node is `NoReturn`.
- No C++ destructors or unwinding run in the generated program.

### Sandbox denials

Capability denial is a normal `Error` with a stable code (e.g.
`E_CAPABILITY_DENIED`) and a message naming the capability and resource. It is
not a panic, so programs can recover exactly as in the Go implementation.

## Consequences

- IR has `CallStatus` and `CheckError` pseudo-nodes.
- Stack traces are explicit: each frame pushes a `(func_id, pc)` record into a
  runtime trace buffer; `Error.trace_id` indexes it.
- Tests assert status codes and ulang++ error messages/positions.

## Alternatives considered

1. **C++ exceptions through generated code** — rejected: requires unwind
   tables/CRT, UB across hand-written asm, and violates "no CRT".
2. **`std::expected` at the runtime ABI** — rejected: it is a C++ type with
   unspecified layout; not an ABI.
3. **Single return value with sentinel tags** — rejected: complicates every
   call site and loses the distinction between ulang++ `Error` values and runtime
   faults.


### ADR: 0004-embedded-runtime-object-format.md


# ADR-0004: Embedded runtime and object-image format

- Status: Accepted
- Date: 2026-11-25

## Context

`ulang++ build` may not invoke `gcc`, `clang`, `ld`, `as`, `nasm`, `llvm-*`, or
`go`. The runtime and stdlib must therefore be compiled **before** the ulang++
project is built, embedded into the `ulang++` executable, and merged with user
code by the compiler's own linker.

We also need PE32+ and ELF64 output for x86-64 and ARM64. A full ELF/PE object
format is unnecessary internally; a small, explicit relocatable format is
easier to verify and keeps the same code path for both targets.

## Decision

Define **POB (ulang++ Object Binary)**, a little-endian relocatable image:

```text
magic  "POBS" (4 bytes)
version u32
arch    u16   (1 = x86_64, 2 = aarch64)
os      u16   (0 = any, 1 = windows, 2 = linux)
flags   u32
sections:
  .text    u8[]   executable
  .rodata  u8[]   read-only
  .data    u8[]   writable, zero-initialized tail allowed
  .bss     u64 size
symbols:
  name[] (NUL-separated string table, u32 offsets)
  { name_off u32; section u16; value u64; size u64; binding u8; kind u8 }
relocations:
  { section u16; offset u64; symbol u32; type u8; addend i64 }
imports:
  { name_off u32; library_off u32 }   // Windows: DLL + symbol; Linux: vDSO/syscall id
```

Relocation types: `ABS64`, `REL32`, `PLT32`, `PCREL32`, `ADDR32NB` (PE),
`TLS` (reserved). The writer is explicit little-endian; no C structs are
written to disk.

### Build pipeline

1. When building the **compiler** (host step, outside ulang++ project builds):
   - compile runtime/stdlib C/C++ sources with the host C++ toolchain;
   - convert their objects into `.pob` with `tools/pobgen` (part of the
     compiler build, run once);
   - embed the `.pob` images as `constexpr unsigned char[]` arrays in a
     generated C++ translation unit (`runtime_images.cpp`), selected by
     arch/OS.
   - Alternatively, once the ulang++ backend is self-hosting enough, runtime
     sources in ulang++ can be emitted by the same backend. The embedding step is
     unchanged.

2. When building a **ulang++ project**:
   - compile every module to `.pob` (text/rodata/data + symbols);
   - link user `.pob` files plus the embedded runtime/stdlib `.pob` for the
     selected target with the compiler's own resolver;
   - apply relocations, lay out sections, resolve imports to OS DLLs or raw
     syscalls;
   - emit PE32+ or ELF64 directly. No system linker, no external process.

### Symbol policy

- Runtime symbols are prefixed `pawl_`; stdlib symbols `pawl_std_`.
- Name mangling is deterministic and documented (`pawl_m_<module>_<name>` for
  module globals; `<module>.<Type>.<method>` names are hashed only when
  necessary, never relying on C++ ABI).
- User code cannot accidentally override runtime symbols; the linker resolves
  runtime first and reports a collision.
- Each image records a content hash; `--verbose-build` prints the hash of every
  embedded runtime image.

### PE/ELF specifics

- PE: one `.text`, `.rdata`, `.data`, `.bss`; import directory built from
  `imports`; base relocations only if the image is not fixed-base (we prefer
  fixed base / `IMAGE_FILE_RELOCS_STRIPPED` for executables). ARM64 uses the
  same writer with machine = `IMAGE_FILE_MACHINE_ARM64`.
- ELF: `ET_EXEC`, two or more `PT_LOAD`, no `PT_INTERP`, no `.dynamic`, no
  libc. ARM64 uses `EM_AARCH64`; x86-64 uses `EM_X86_64`.
- Both writers share a `Layout` abstraction (sections, symbols, relocations)
  so the backend and linker stay target-agnostic.

### Security/protection

The documented `--protect` flag is implemented as:
- strip symbol/name tables from output unless `--debug`;
- mark `.text` RX, `.rodata` R, `.data` RW (PE section characteristics,
  ELF `p_flags`);
- optional W^X enforcement check before writing;
- no fake flags: anything not implemented is rejected with a diagnostic.

## Consequences

- `.pob` must be versioned and fuzzed; malformed input produces diagnostics,
  not crashes.
- Embedded images increase the `ulang++` binary size; a build option can embed
  only the needed modules per target (LTO-like pruning at module granularity).
- Reproduction is byte-for-byte deterministic for identical inputs and
  compiler version.

## Alternatives considered

1. **Emit ELF/PE objects and shell out to a linker** — forbidden.
2. **Trampoline through libc/CRT** — forbidden.
3. **One monolithic precompiled blob per OS/arch** — rejected: prevents
   stdlib pruning and makes symbol resolution opaque.
4. **LLVM MC/object** — forbidden (external toolchain/LLVM dependency).


### ADR: 0005-run-execution-mode.md


# ADR-0005: `ulang++ run` execution mode

- Status: Accepted
- Date: 2026-11-25

## Context

The legacy `ulang++ run` uses the Go VM/interpreter/JIT. The native compiler must
run programs without any interpreter, bytecode or JIT. Two options were
allowed by the migration brief:

1. build a temporary executable and spawn it;
2. generate code in memory and load/execute it.

## Decision

Use **build-to-temporary-file + spawn** for `ulang++ run` in v1:

1. compile the project exactly like `ulang++ build` into a temporary file inside
   the OS temp directory (or `--out` if given);
2. spawn it with inherited stdio, passing program arguments and environment;
3. propagate the exit code; delete the temp file on exit unless `--keep`;
4. no compiler process is launched during the build itself; the only spawned
   process is the user's freshly built program.

Rationale: correctness, debuggability, identical semantics to `ulang++ build`,
and no executable-memory loader to secure. The Windows loader and Linux
`execve` path are the same code the OS uses for the shipped binary.

An **optional** `--in-memory` mode may be added later: allocate RW memory,
apply relocations, `mprotect`/`VirtualProtect` to RX, and call the entry point.
It must use the same `.pob` linker and runtime, never an interpreter. It is
off by default and not required for parity.

## Consequences

- `run` needs a writable temp directory and the ability to execute files there.
  On failure, a clear diagnostic is emitted and no fallback interpreter exists.
- Tests assert that `run` never loads a VM/bytecode module and that the child
  process is the compiled binary.
- Sandbox capabilities are enforced in the runtime of the child, not by the
  parent compiler.

## Alternatives considered

1. **In-process interpreter** — forbidden by the product requirements.
2. **JIT in memory** — rejected for v1: more attack surface, harder debugging,
   and unnecessary for correctness; remains an optional future mode.
3. **Pipe source to a hidden VM** — forbidden.

