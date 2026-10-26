# Alca Language Specification & Reference

Alca is a statically typed, memory-safe systems automation language. It combines clean scripting syntax with static analysis, fast bytecode VM execution, multi-tier JIT acceleration, and capability-based sandboxing.

---

## 1. Program Structure & Syntax

A program is a sequence of statements.
- **Statement Termination**: Statements terminate with a semicolon (`;`).
- **Block Enclosure**: Blocks may be enclosed either in canonical curly braces (`{ ... }`) or with a colon (`:`) followed by an indented suite.
- **Comments**: Single-line comments start with `//`. Multi-line comments use `/* ... */`.

```alca
use std.math;

// Canonical brace syntax
fn calculate(r: float) -> float {
    if r <= 0.0 {
        return 0.0;
    }
    return math.PI * r * r;
}

// Colon and indentation syntax is also supported
fn greeting(name: string):
    if len(name) == 0:
        println("Hello, Guest!");
    else:
        println("Hello, " + name + "!");
```

### Module Imports
Modules are imported using `use`:
- `use std.math;` — imports a standard library module under its namespace `math`.
- `use "./utils.al";` — imports a relative local file.
- All imported declarations are statically checked against the importing program.

---

## 2. Type System

Alca enforces **strict static typing** with bidirectional inference. Implicit narrowing, widening, or silent coercion is strictly prohibited.

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
```alca
// Explicit typing
count: int = 10;
price: float = 99.95;
name: string = "Alca";
active: bool = true;

// Type inference (inferred once at compile time, never becomes dynamic)
ratio := 1.75;             // float
items := ["apple", "pear"]; // list[string]

// Shadowing is rejected at compile time:
// count: int = 20;        // ERROR: variable 'count' already declared
count = 20;                // OK: variable assignment
```

### Strict Type Arithmetic
Numbers do not coerce silently:
```alca
a: int = 5;
b: float = 2.5;

// c := a + b;             // ERROR: operator '+' cannot be applied to int and float
c := float(a) + b;         // OK: 7.5
quotient := float(a) / 2.0;// Float division: 2.5
int_div := a / 2;          // Integer division truncates towards zero: 2
```

### Null-Safety & Type Narrowing
```alca
maybe_user: string | null = null;

// Property access directly on nullable types is rejected by type checker
if maybe_user != null {
    // Within this block, maybe_user is safely narrowed to string
    println("User is: " + maybe_user);
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
```alca
val := 10 |> double |> add(5); // add(double(10), 5) -> 25
```

### Stream Pipelines (`>>`, `>!`, `>>>`, `<<`, `&>>`, `?>>`, `?>>>`)
Alca provides native stream operators for piping data between variables, files, network sockets, and consoles:
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
```alca
if score >= 90 {
    println("Grade: A");
} elif score >= 75 {
    println("Grade: B");
} else {
    println("Grade: C");
}
```

### Loops: For & While
```alca
// Iterating ranges
for i in 0..5 {
    if i == 2 {
        continue;
    }
    println(i);
}

// Iterating collections
names := ["Alice", "Bob", "Charlie"];
for name in names {
    println("Hello, " + name);
}

// While loop
counter := 3;
while counter > 0 {
    counter = counter - 1;
}
```

---

## 5. Functions, Closures, and Async/Await

### Function Signatures
Functions can be declared using `fn` or `func`:
```alca
pub fn multiply(x: int, y: int = 1) -> int {
    return x * y;
}

// Void functions (no return value)
void log_info(msg: string) {
    println("[INFO] " + msg);
}
```

### First-Class Functions and Closures
```alca
fn make_adder(base: int) -> func {
    fn add(n: int) -> int {
        return base + n;
    }
    return add;
}

add10 := make_adder(10);
assert add10(5) == 15;
```

### Cooperative Async / Await Parity
Both the Bytecode VM and reference Interpreter provide complete parity for `async` function definitions and `await` expressions:
```alca
async fn fetch_status(endpoint: string) -> string {
    return "200 OK from " + endpoint;
}

status := await fetch_status("https://api.alca.org");
assert status == "200 OK from https://api.alca.org";
```

### Background Execution (`@ba`, `@threads`)
- Spawning background worker tasks:
```alca
ba(timeout=5s):
    println("Background calculation running");
```
- Multi-threaded execution pools:
```alca
threads 4:
    println("Running across worker pool");
```

---

## 6. Structs, Classes, and Interfaces

### Structs (Lightweight Value Types)
```alca
struct Point {
    x: int;
    y: int = 0;
}

p := Point(10, 20);
p.x = 15;
assert p.x == 15;
```

### Classes and Single Inheritance
```alca
class Account {
    pub owner: string;
    private balance: float = 0.0;

    pub fn deposit(amount: float) -> void {
        self.balance = self.balance + amount;
    }

    pub fn get_balance() -> float {
        return self.balance;
    }
}

class PremiumAccount extends Account {
    pub bonus_rate: float = 0.05;

    pub fn deposit_with_bonus(amount: float) -> void {
        bonus := amount * self.bonus_rate;
        super.deposit(amount + bonus);
    }
}

acc := PremiumAccount("Alice", 100.0, 0.1);
acc.deposit_with_bonus(100.0);
assert acc.get_balance() == 210.0;
```

### Enums & Pattern Matching
```alca
enum OrderStatus {
    Pending,
    Processing = 1,
    Shipped,
    Delivered,
}

status := OrderStatus.Shipped;
match status {
    case OrderStatus.Pending:
        println("Awaiting payment");
    case OrderStatus.Shipped:
        println("Item is in transit");
    else:
        println("Status updated");
}
```

### Interfaces (Structural Polymorphism)
```alca
interface Notifier {
    fn notify(msg: string) -> bool;
}

class EmailSender {
    pub fn notify(msg: string) -> bool {
        println("Email sent: " + msg);
        return true;
    }
}

sender: Notifier = EmailSender();
sender.notify("System alert");
```

---

## 7. Error Handling & Stream Pipelines

Alca uses explicit error values instead of uncontrolled stack unwinding.

### Error Creation & Capture
- Use `err(message, code=...)` to raise an error value.
- Use `expr ?>>> target` to capture errors into an `error | null` variable without aborting the program:

```alca
use std.io;

failure: error | null = null;
content := io.read_file("nonexistent.log") ?>>> failure;

if failure != null {
    println("Failed with code: " + string(failure.code));
    println("Message: " + failure.msg);
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
- `json_encode(val) -> string`: serializes an Alca value to compact JSON.
- `json_decode(str) -> json`: parses a JSON string into a dynamic `json` value.
- `json_prettify(val) -> string`: formats an Alca value as indented JSON.
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
- `__version__`: Alca compiler/runtime version string.

---

## 9. Capability-Based Sandboxing

Alca scripts run by default in a non-permissive sandbox. Operations requiring elevated host resources must be explicitly enabled at invocation:

- Filesystem: `--allow-read[=<path>]`, `--allow-write[=<path>]`
- Network: `--allow-net[=<host[:port]>]`
- Subprocesses: `--allow-run[=<cmd>]`
- Environment: `--allow-env[=<var>]`
- Native FFI: `--allow-ffi[=<lib>]`
- Full access: `--allow-all`

Attempting an unpermitted operation returns an `error` with code `403` and records an event in the security audit log.
