# Command-Line Interface (CLI)

Build the CLI binary with:
```bash
go build -o alca ./cmd/alca
```

## Commands Overview

```text
Alca Programming Language CLI v1.7.9

Usage:
  alca <command> [arguments]

Commands:
  run <file.al> [args...]     Run an Alca program (Bytecode VM with JIT acceleration)
  build <file.al> [-o out]    Compile program into a standalone native executable
        --protect, -p         protect binary: AES-256-GCM encryption + symbol obfuscation
        --noconsole, --gui    build a GUI application without opening a console window
        --target os/arch      cross-compile (e.g. linux/amd64, windows/arm64, darwin/arm64)
        --runtime <file>      use a specific runtime binary for the target
        --source              embed source instead of bytecode
        --embed <path>        embed a project file or directory (repeatable)
  init [name]                 Initialize a new Alca project
  dll <src.c|cpp|al> [-o out] Compile native C/C++ or Alca source into a shared library (.dll / .so)
  test <file.al>              Run test blocks and report PASS/FAIL
  check <file.al>             Syntax + strict type check (CI friendly)
  fmt [-w] <file.al>          Format the program (-w rewrites the file in place)
  repl                        Interactive session
  targets                     List build targets and which runtimes are installed
  build-runtimes [--targets a,b] [--out dir] [--tags t]
                              Build runtimes for cross-compilation (needs the Go toolchain)
  lsp                         Language Server Protocol daemon for editors
  version | help
```

---

## 1. `alca run`

```bash
alca run [--vm|--interp|--jit] [capability flags] <file.al> [script args...]
```

- **Default engine**: Bytecode VM. High-speed register-based VM. If the code uses interpreter-only dynamic features (e.g. dynamic AST streaming), it cleanly falls back with a diagnostic note.
- `--vm`: Forces the virtual machine. Rejects non-VM constructs with an error code.
- `--interp`: Forces the reference AST tree-walking interpreter.
- `--jit`: Enables multi-tier AMD64 JIT compilation for hot loops and functions.
- Memory limits:
  - `--max-memory=<size>`: Maximum resident memory (e.g. `128mb`, `1gb`).
  - `--max-allocation=<size>`: Maximum cumulative memory allocations.
- Script arguments after `<file.al>` are accessible inside the program via `cli.args()` from `std.cli`.

---

## 2. `alca check`

```bash
alca check <file.al>
```

Parses and type-checks a file without executing it. Returns exit code `0` on clean check; prints compiler error diagnostics with line/column pointers and returns exit code `1` on failure. Ideal for CI pipelines and pre-commit hooks.

---

## 3. `alca test`

```bash
alca test [capability flags] <file.al>
```

Discovers and executes top-level code and all `test "name" { ... }` blocks. Reports test count, passes, and failures. Exits with non-zero status if any assertion (`assert`) or test block fails.

---

## 4. `alca build`

```bash
alca build <file.al> [-o output] [--protect] [--gui] [--target os/arch] [--source] [--embed <path>]
```

Compiles an Alca program into a standalone, single-file native executable without external compiler dependencies:
- **Windows**: Built-in native PE/COFF builder.
- `--protect`, `-p`: Encrypts payload with AES-256-GCM and obfuscates symbols.
- `--noconsole`, `--gui`: Sets the PE subsystem to GUI mode (avoids showing a console window on launch).
- `--source`: Embeds original source code and local imports instead of compiled VM bytecode.
- `--embed <path>`: Packages assets, templates, or databases directly into the executable (accessible at runtime via `use std.embed; embed.read(...)`). Can be repeated for multiple files/directories.

---

## 5. `alca dll`

```bash
alca dll <source> [-o output.dll]
```

Compiles native C/C++ code or Alca exported symbols into a native dynamic library (`.dll` on Windows, `.so` on Linux) with zero third-party toolchains.

---

## 6. `alca fmt`

```bash
alca fmt [-w|--write] <file.al>
```

Formats Alca code using canonical rules (4-space indentation, consistent statement layout, comment preservation). With `-w`, rewrites the file in place. Round-trips through the parser AST before writing to guarantee safety.

---

## 7. `alca repl`

```bash
alca repl [capability flags]
```

Interactive REPL shell with incremental static type checking, multi-line blocks, persistent environment, and immediate expression evaluation. Type `exit` or `quit` to leave.

---

## 8. `alca lsp`

```bash
alca lsp
```

Starts an LSP (Language Server Protocol) server over standard input/output. Compatible with VS Code, Neovim, Helix, and Sublime Text. Provides completions, diagnostics, and hover information directly from `pkg/typecheck` and AST tables.

---

## 9. Capability Sandbox Flags

By default, Alca programs execute in a restricted sandbox. The following flags explicitly grant permissions:

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

Attempted access without the required capability raises an Alca runtime error (`code 403`) and logs an audit record via the security subsystem.
