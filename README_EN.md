# Alca Programming Language

[English](README_EN.md) | [Русский](README_RU.md)

**Alca** is a modern, statically-typed programming language designed for systems automation, cross-platform tools, and native application development. It combines the expressive ergonomics of scripting with static type safety, bytecode VM execution with multi-tier JIT acceleration, and zero-toolchain standalone distribution:
- **Windows**: Built-in standalone PE/EXE and native DLL generator (no MSVC or MinGW required).
- **Windows & Linux**: Fast Bytecode VM with JIT acceleration and sandboxed capability-based security.

Programs use the `.al` extension.

```alca
use std.math;

struct Server {
    ip: string;
    port: int = 80;
}

pub fn area(r: float) -> float {
    return math.PI * r * r;
}

srv := Server("127.0.0.1", 8080);
println("listening on " + srv.ip + ":" + string(srv.port));
println(area(2.0));
```

## Quick Start

```bash
go build -o alca ./cmd/alca
./alca run examples/hello.al
```

The Bytecode VM is the primary execution engine. Programs compile directly to standalone native Windows executables (`alca build app.al`) and native DLLs (`alca dll lib.al`).

---

## Documentation & Learning Guides

- 📘 [**Comprehensive Handbook**](docs/handbook_EN.md) ([Русский](docs/handbook_RU.md)) — in-depth handbook covering syntax, types, pipelines, error streams, builtins, sandbox, and the entire `std.*` library.
- 📖 [**Language Specification & Reference**](docs/language_EN.md) ([Русский](docs/language_RU.md)) — formal specification of grammar, types, OOP, pattern matching, and operators.
- 📚 [**Standard Library Catalog**](docs/stdlib_EN.md) ([Русский](docs/stdlib_RU.md)) — detailed signatures and capability requirements for all 40+ stdlib modules.
- 📦 [**Package Manager (APM)**](docs/apm_EN.md) ([Русский](docs/apm_RU.md)) — dependency management, `alca.toml` manifests, lockfiles, registry, and security.
- 💻 [**Editor Tooling & LSP**](docs/editors_EN.md) ([Русский](docs/editors_RU.md)) — VS Code, Cursor, and Zed editor extensions, Language Server Protocol (`alca lsp`).
- 🛠️ [**CLI & Sandboxing Guide**](docs/cli_EN.md) ([Русский](docs/cli_RU.md)) — commands (`run`, `build`, `dll`, `fmt`, `lsp`) and security flags.
- 🏗️ [**Architecture & Execution Tiers**](docs/architecture_EN.md) ([Русский](docs/architecture_RU.md)) — Bytecode VM internals, JIT compiler, PE/DLL assembler, and security model.
- 🌐 [**Cross-Compilation Guide**](docs/cross-compilation_EN.md) ([Русский](docs/cross-compilation_RU.md)) — building native executables for other target OS/architectures.
- 🧪 [**Everyday Test Suite**](tests/) — 10 real-world application test programs (JSON, CSV, HTTP, Crypto, IO, Time, OOP).

---

## Key Language Rules

- **Syntax**: Statements end with `;`. Blocks can be enclosed either in canonical curly braces `{ ... }` or written with `:` and indentation.
- **Functions**: Canonical syntax is `fn name(params) -> ReturnType { ... }` or `pub fn`.
- **Types**: Strict static typing with bidirectional inference (`name: Type = val;` or `name := val;`). No implicit narrowing or coercions.
- **Equality**: `==` and `!=` perform structural value equality between comparable typed values. `===` and `!==` provide strict reference/identity checks.
- **Ranges**: Half-open range `a..b` (`[a, b)`) iterates from `a` up to `b - 1` (e.g. `0..3` visits `0, 1, 2`). Inclusive range `a..=b` (`[a, b]`) visits `a` through `b` (e.g. `0..=3` visits `0, 1, 2, 3`).
- **Abstractions**:
  - `enum`: Tagged enumerations and Algebraic Data Types with pattern matching (`match`).
  - `interface` / `trait`: Polymorphic contracts checked statically at compile time.
  - Concurrency: Background tasks (`@ba`) and thread pools (`@threads`). Full cooperative `async`/`await` parity across both the VM and Interpreter.
- **Classes & Structs**: Field definitions follow `name: Type [= default];` or `pub name: Type [= default];`. Single inheritance with `class Child extends Parent`.
- **Conditions**: Conditions must evaluate strictly to `bool`; truthiness coercion is prohibited.
- **Pipelines & Streams**:
  - Functional pipeline: `x |> f(...)` forwards `x` as the first argument to `f`.
  - Stream pipelines: `>>` (append), `>!` (overwrite), `>>>` (pipe to variable), `<<` (pipe from file/stdin), `&>>` (async background stream), `?>>` / `?>>>` (stream error logging and non-throwing error capture).
- **Dynamic Escape Hatch**: `json` is the sole dynamic type, requiring explicit typed conversion before concrete use.

---

## Capability-Based Sandbox

Alca enforces explicit capability permissions when running scripts. By default, untrusted scripts cannot access the host filesystem, network, subprocesses, environment variables, or foreign function libraries:

- `--allow-read[=<path>]`: Grants read access to files or directories.
- `--allow-write[=<path>]`: Grants write access to files or directories.
- `--allow-net[=<host[:port]>]`: Grants network socket and HTTP access.
- `--allow-run[=<cmd>]`: Grants process execution permissions (`exec.Command` / `std.process`).
- `--allow-env[=<var>]`: Grants access to environment variables (`std.env`).
- `--allow-ffi[=<lib>]`: Grants permission to load native dynamic libraries (.dll, .so).
- `--allow-all`: Unconditionally grants all capabilities.

> **Threat Model Note**: Alca's managed runtime prevents buffer overflows, uninitialized memory reads, and memory corruption. However, when `--allow-ffi` is explicitly granted, native code operates directly in the process address space without Go's memory-safety guarantees.

---

## Command-Line Usage

```text
alca run <file.al> [args...]       # type-check, then run (VM by default)
alca run --jit <file.al> [args...] # opt into multi-tier AMD64 JIT compilation
alca run --vm ...                  # require the VM; reject unsupported constructs
alca run --interp ...              # use the tree-walking interpreter
alca check <file.al>               # syntax + strict type check (CI friendly)
alca test <file.al>                # run test blocks and summarize PASS/FAIL
alca fmt [-w] <file.al>            # format to stdout, or rewrite in place
alca repl                          # interactive session with persistent state
alca lsp                           # Language Server Protocol over stdio
alca build <file.al> [-o out]      # standalone executable for the host (PE/COFF)
alca dll <file.al> [-o out.dll]    # compile into native Windows DLL / Linux SO
alca targets                       # show supported targets and installed runtimes
```

---

## Package Manager (APM)

`apm` installs libraries so programs can `use` them without manual path setup:

```text
go build -o apm ./cmd/apm

apm install -u ./my-library     # from local sources
apm install "json@^1.2.0"       # from a registry (built-in node, $ALCA_REGISTRY or --registry)
apm show                        # list installed libraries
apm remove json
```

Installed libraries live in a local system registry under `$ALCA_HOME` (default `~/.alca`), including a compiled VM build per package that the VM loads automatically.

---

## Development & Testing

```bash
gofmt -l .
go vet ./...
go test ./...
go test -race ./...
```

All 10 everyday test suites under `tests/` can be executed directly:
```bash
alca run --allow-all tests/test_01_strings_text.al
alca run --allow-all tests/test_02_collections_algorithms.al
alca run --allow-all tests/test_03_json_data.al
alca run --allow-all tests/test_04_csv_ini_formats.al
alca run --allow-all tests/test_05_math_financial.al
alca run --allow-all tests/test_06_oop_ecommerce.al
alca run --allow-all tests/test_07_error_handling_resilience.al
alca run --allow-all tests/test_08_file_io_logging.al
alca run --allow-all tests/test_09_crypto_security.al
alca run --allow-all tests/test_10_datetime_bench.al
```

---

## License

Alca is released under the [Apache License, Version 2.0](LICENSE).  
See [NOTICE](NOTICE) for copyright statements and third-party attributions.
