# Alca language summary

This is a practical reference for the syntax and semantics enforced by the
parser and type checker. The implementation is the source of truth; when in
doubt use `alca check`.

## Program structure

A program is a sequence of statements. Simple statements end in `;`. Blocks
start with `:` and their body is indented. Indentation is significant for block
membership; a statement at the outer indentation after a block header is a
syntax error rather than being silently included.

```alca
use std.math;

x: int = 40;
if x > 0:
    println("positive");
else:
    println("non-positive");
```

`use` imports a standard module (`use std.math;`) or a relative `.al` file
(`use "./util.al";`). Imported modules are type-checked with the importing
program.

## Types

Built-in type names include `int`, `float`, `string`, `bool`, `list`,
`map`, `func`, `json`, `void`, `error`, `null`, and user `struct` names.

- Declarations use `name: Type = value;` or inference with `name := value;`.
  Inference fixes the type; it does not become dynamic.
- `any` is intentionally not accepted in declarations. `json` is the dynamic
  escape hatch.
- A name that is already bound is never shadowed. Declaring it again in a
  nested scope updates the existing binding, and only the same type is allowed:

  ```alca
  count: int = 1;
  if count > 0:
      count: int = 2;   // updates the outer binding, not a new one
      println(count);   // 2
  println(count);       // 2
  ```

  A redeclaration with another type is a type error
  (`shadowing 'count' changes its type from int to string`).
- Lists are written `list[int]`, `list[string]`, etc. Empty list literals need
  a contextual type.
- Maps are written `map[string, int]`, etc.
- Optional/nullable values use `T | null`. `null` is not assignable to `T`.
- Numbers do not mix implicitly. `1 + 2.0` is rejected; convert explicitly.
- `int / int` truncates toward zero. Use `float(a) / float(b)` for real
  division.

```alca
count: int = 3;
ratio := 0.5;              // float
items: list[string] = ["a", "b"];
scores: map[string, int] = {"a": 1};
maybe: string | null = null;
if maybe != null:
    println(maybe);        // narrowed to string
```

## Operators

Arithmetic: `+ - * / % **`. String `+` concatenates two strings only.
Comparison: `==` performs loose equality with limited numeric coercion
(`1 == 1.0` and `"1" == 1` are true); `===` requires matching runtime types,
including recursively within lists and maps. `!=` is the negation of `==`.
Ordering comparisons are `< <= > >=`. Boolean: `and`, `or`, `not` (and `&&`,
`||`, `!`). Membership/indexing: `in`, `[]`. Ranges use `a..b` and are inclusive.

`==` follows one table, so there are no surprises:

| Left | Right | Result |
| --- | --- | --- |
| same type | same type | value equality; lists and maps compare recursively |
| `int` | `float` | numeric comparison (`1 == 1.0` is true) |
| `string` | number | true only when the string is a number with that value (`"1" == 1`, `"x" == 1` is false) |
| `bool` | number | `true` compares as `1`, `false` as `0` |
| anything else | | false |

The coercion exists because `json` is a dynamic type: a value read from JSON
has no static type, so `config["port"] == 8080` has to work. Use `===` when the
difference matters, for example to tell the string `"1"` from the number `1`.
Pipeline `|>` calls a function with the left value as its argument.

## Control flow

```alca
if cond:
    ...
elif other:
    ...
else:
    ...

for i in 0..3:          // i = 0,1,2,3
    if i == 1:
        continue;
    if i == 3:
        break;

while cond:
    ...

for item in items:
    println(item);
```

`for i in a..b` iterates inclusively from `a` to `b`. Assigning the loop
variable inside the body does not change the iteration sequence. `break` and
`continue` are supported on both engines.

## Functions, lambdas, and closures

```alca
func add(a: int, b: int = 1) -> int:
    return a + b;

func make_counter() -> func:
    n := 0;
    func next() -> int:
        n = n + 1;
        return n;
    return next;

c := make_counter();
println(c());  // 1
println(c());  // 2
```

- Parameters may have default values. Calls may use named arguments.
- A function with no return value is declared `-> void`.
- Function bodies do not implicitly return their final expression.
- Nested functions and lambdas (`|x: int| x + 1`) capture enclosing locals by
  reference. Separate calls to a factory create independent captured state.
- A function value can be stored in a variable, list, or map and called later.

## Structs

```alca
struct Point:
    x: int;
    y: int = 0;

p := Point(10, 20);
println(p.x);
p.x = 11;
```

Fields are typed, may have literal defaults, and struct values are mutable.
Struct definitions are values and can be passed around like other values.

## Classes and inheritance

A class adds methods, visibility and single inheritance to a struct:

```alca
class Animal:
    public string: name;
    private int: age = 0;

    public func speak() -> string:
        return self.name + " makes a sound";

class Dog extends Animal:
    public string: breed = "mutt";

    public func speak() -> string:
        return self.name + " says woof";

    public func parent_speak() -> string:
        return super.speak();

d := Dog("Rex", 3, "lab");
println(d.speak());          // Rex says woof
println(d.parent_speak());   // Rex makes a sound
```

- Fields keep the struct rules: a type, an optional literal default, and
  `public` or `private` visibility.
- The constructor takes the fields of the base class first, in declaration
  order, then the fields of the derived class.
- Inside a method, `self` is the receiver and `super` reaches the parent
  implementation. Assigning through `super` is rejected; assign to
  `self.<field>` instead.
- Method calls dispatch dynamically, so an `Animal` that holds a `Dog` calls
  the method of the `Dog`. A method is also a value: `speak := d.speak;`
  keeps working when it is called later.
- A class can declare nested types (`public struct Toy:` inside a class).

Classes run on both engines. `alca run --vm` compiles them to bytecode like any
other declaration; see [Engines](#engines).

## Errors

Use `err(message, code=...)` to raise a runtime error. Error-producing calls
return an `error` value, which can be captured with `?>>>` or checked with
`is_permission_error` / `io.is_permission_error`:

```alca
failure: error | null = null;
read_file("missing.txt") ?>>> failure;
if failure != null:
    println("failed: " + failure.msg);
```

Permission failures use code `403`; other runtime failures use the code supplied
by the standard library or the program.

The two stream operators differ in what they do with a failure:

- `expr ?>>> target` captures the error value in `target` and continues. The
  target keeps its type, so it has to be `error | null` (or `error`);
- `expr ?>> target` writes the error value to a path or URL instead, so the
  target has to be a string.

An error value carries `msg`, `code` and `time`. A program that only checks
whether something failed can use `is_permission_error`; a program that has to
react to a specific failure compares `failure.code`.

Values that come from JSON have the dynamic type `json`. They are not errors
by themselves: a failed call returns an `error` value, while a successful call
that returns JSON data returns a `json` value. Convert a `json` value before
using it in a typed position, and compare it with `===` when the exact type
matters.

## Memory limits

The runtime can limit the memory a program may use. `alca run --max-memory=256mb`
bounds the memory the program holds at once, and `--max-allocation=1gb` bounds
the total amount it allocates. A size of `0` disables a limit; without the flags
a program is not limited.

When a limit is reached the program stops with a regular Alca error (code
`507`), so it can be handled like any other error:

```alca
failure: error | null = null;
grow() ?>>> failure;
if failure != null:
    println("stopped: " + failure.msg);
```

The limit covers the data of the program: list elements, map entries, strings,
environment bindings, the fixed structures of a virtual machine, and compiled
JIT code. It is an estimate, not an exact byte count of the Go heap, and it
does not cover the memory the Go runtime itself needs. A program that is
stopped keeps its limit: the error is reported again on the next check.

The tree-walking interpreter also understands the `@oom_guard` decorator, which
installs the same limit for the program:

```alca
@oom_guard("max=64mb")
func work() -> int:
    ...
```

Function decorators are only supported by the tree-walking interpreter; the
bytecode VM rejects them, so a program that uses one has to run with
`--interp`.

## Engines

Alca has two engines: the bytecode virtual machine (with the JIT tiers on top
of it) and the tree-walking interpreter. They implement the same language, with
one documented exception: a few constructs exist only in the interpreter.

Interpreter-only today:

- shell functions (`shfunc`) and their streaming;
- `ba` background blocks and `ba for` loops, `threads` blocks and `watch`
  blocks, because they run code concurrently and the virtual machine executes
  one frame stack;
- timeout expressions (`timeout(5s) expr`), for the same reason;
- capture streams used as values;
- keyword arguments in a call whose callee is only known at run time.

Everything else runs on both engines, including classes, decorators,
transactions, mocks, closures, error handling and the memory guard. When a
program uses an interpreter-only construct, the behaviour depends on how it is
started:

- `alca run file.al` runs it on the interpreter and prints a note on stderr
  (`running on the interpreter`);
- `alca run --vm file.al` reports the unsupported construct as an error;
- `alca build file.al` embeds the source and runs it on the interpreter of the
  standalone binary.

## Tests

```alca
test "addition":
    assert 1 + 1 == 2;

test "strings":
    assert "a" + "b" == "ab";
```

`alca test file.al` runs the whole program (including top-level setup) and
reports the number of passed and failed test blocks. A failed assertion or
unhandled runtime error exits non-zero.

## Modules and standard library

The standard library is available as modules (`io`, `net`, `math`, `path`,
`time`, `crypto`, `re`, `archive`, `csv`, `ini`, `log`, `metrics`, `db`, ...)
after `use std.<name>;`. Some functions are also registered as top-level
builtins for compatibility. See [stdlib.md](stdlib.md) and
`pkg/typecheck/builtins_table.go` for exact signatures.

## Formatting

`alca fmt file.al` uses four-space indentation, one statement per line, and
preserves comments. `-w` rewrites in place. Programs that cannot be re-parsed
after formatting are rejected by the formatter's AST round-trip check.
