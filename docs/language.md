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
Comparison: `== != < <= > >=`. Boolean: `and`, `or`, `not` (and `&&`, `||`,
`!`). Membership/indexing: `in`, `[]`. Ranges use `a..b` and are inclusive.
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
