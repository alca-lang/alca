# Architecture

Alca features a multi-tiered execution architecture fed by a single front end:

1. **Bytecode VM (`pkg/vm`)**: The primary high-performance stack-based virtual machine with lexical upvalues, inline caching, and superinstructions.
2. **Interpreter (`pkg/interpreter`)**: The tree-walking reference semantic engine and fallback evaluator.
3. **JIT Compiler (`pkg/jit`)**: Tiered compilation specializing hot function templates into native code.
4. **Native Standalone Compiler (`cmd/alca/native_build.go`)**: Pure Go standalone Windows PE executable and DLL generator.

## Pipeline

```text
source .al
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

Alca provides two levels of concurrency:

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
- `alca run` automatically falls back to the reference interpreter if the VM encounters an unsupported construct; `alca run --vm` explicitly reports the error.

### 2. JIT Engine
The JIT compiler (`pkg/jit`) compiles closures and function templates once into native machine code. It shares compiled artifacts across instances of the same function template while keeping captured closure values instance-specific. Artifact counts are strictly bounded by `jit.Config.MaxArtifacts`.

### 3. Native AOT & Standalone Bundles
`cmd/alca/native_build.go` and `pkg/compiler` produce standalone binaries:
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
- While Alca's managed runtime is memory-safe (preventing buffer overflows and dangling pointers within Alca code), granting `--allow-ffi` allows loading arbitrary C libraries and manipulating raw native pointers, which operates outside Go's memory-safety guarantees.
- Subprocess execution (`--allow-run`) delegates control to host shell binaries.

## Engine Parity & Testing

Engine parity is the top development priority:
- `cmd/alca/engine_test.go`: Runs an extensive corpus against both the Bytecode VM and Interpreter, asserting identical stdout, evaluation results, and exit codes.
- `cmd/alca/examples_test.go`: Statically checks all shipped examples and verifies determinism across engines.
