# Pawl Programming Language

[English](README_EN.md) | [Русский](README_RU.md)

> **Legacy document (Pawl-era branding).** For the current ULang++ toolchain see [README.md](../README.md) and [docs/os_dev_EN.md](docs/os_dev_EN.md). The feature list below overstates what the native backend currently supports.

**Pawl** is a high-performance, statically-typed systems programming language designed for bare-metal operating system development, low-level tooling, and standalone native applications.

### Key Capabilities:
- 🚀 **Native AOT Compiler**: Direct compilation to x86-64 machine code (`pawl build`), generating standalone **PE** (`.exe`), **ELF**, and **Raw Binary** without external linkers.
- 💻 **Operating System & Bare-Metal Ready**:
  - CPU Ring 0 intrinsics: `cpu.cli()`, `cpu.sti()`, `cpu.hlt()`, `cpu.pause()`, `cpu.in8()`, `cpu.out8()`, `cpu.read_cr3()`, `cpu.write_cr3()`, `cpu.invlpg()`.
  - Direct physical memory access (MMIO): `mem.read_u8/16/32/64` and `mem.write_u8/16/32/64` (VGA buffer at `0xB8000`, APIC registers, PIC interrupt controller).
- 💿 **Multiboot 1 / GRUB / QEMU Support**: Native `--target freestanding-elf` with automated Multiboot header generation at `0x100000` for direct hypervisor execution (`qemu-system-x86_64 -kernel kernel.elf`).
- ⚡ **Advanced Memory Management**: Mark-Sweep Garbage Collector alongside fast Bump Allocation.
- 🧵 **Async & Green Threads**: M:N cooperative fiber scheduler.

Programs use the `.upp` extension (legacy `.al` files are still accepted — this document is from the Pawl era; prefer [README.md](README.md) and the current `.upp` examples).

```pawl
use std.cpu
use std.mem

// Minimal OS Kernel: write to VGA text buffer and halt CPU
func kernel_main() {
    cpu.cli()
    vga := 753664 // 0xB8000
    attr := 3840  // White on black

    // Write "PAWL" directly to VGA buffer
    mem.write_u16(vga + 0, attr + 80) // 'P'
    mem.write_u16(vga + 2, attr + 65) // 'A'
    mem.write_u16(vga + 4, attr + 87) // 'W'
    mem.write_u16(vga + 6, attr + 76) // 'L'

    loop {
        cpu.hlt()
    }
}
```

## Quick Start & Building

```powershell
# Build the native compiler:
.\compiler\build.ps1

# Build a standalone native executable:
.\compiler\build_bin\pawl.exe build -o my_app.exe my_app.al

# Build an OS kernel for QEMU / GRUB:
.\compiler\build_bin\pawl.exe build --target freestanding-elf -o kernel.elf kernel.al
```

---

## Documentation & Learning Guides

- 📘 [**Comprehensive Handbook**](docs/handbook_EN.md) ([Русский](docs/handbook_RU.md)) — in-depth handbook covering syntax, types, pipelines, error streams, builtins, sandbox, and the entire `std.*` library.
- 📖 [**Language Specification & Reference**](docs/language_EN.md) ([Русский](docs/language_RU.md)) — formal specification of grammar, types, OOP, pattern matching, and operators.
- 📚 [**Standard Library Catalog**](docs/stdlib_EN.md) ([Русский](docs/stdlib_RU.md)) — detailed signatures and capability requirements for all 40+ stdlib modules.
- 📦 [**Package Manager (APM)**](https://github.com/pawl-lang/apm) — standalone package manager for Pawl (`pawl.toml`, `pawl.lock`, registry, CLI).
- 💻 [**Editor Tooling & LSP**](docs/editors_EN.md) ([Русский](docs/editors_RU.md)) — VS Code, Cursor, and Zed editor extensions, Language Server Protocol (`pawl lsp`).
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

Pawl enforces explicit capability permissions when running scripts. By default, untrusted scripts cannot access the host filesystem, network, subprocesses, environment variables, or foreign function libraries:

- `--allow-read[=<path>]`: Grants read access to files or directories.
- `--allow-write[=<path>]`: Grants write access to files or directories.
- `--allow-net[=<host[:port]>]`: Grants network socket and HTTP access.
- `--allow-run[=<cmd>]`: Grants process execution permissions (`exec.Command` / `std.process`).
- `--allow-env[=<var>]`: Grants access to environment variables (`std.env`).
- `--allow-ffi[=<lib>]`: Grants permission to load native dynamic libraries (.dll, .so).
- `--allow-all`: Unconditionally grants all capabilities.

> **Threat Model Note**: Pawl's managed runtime prevents buffer overflows, uninitialized memory reads, and memory corruption. However, when `--allow-ffi` is explicitly granted, native code operates directly in the process address space without Go's memory-safety guarantees.

---

## Command-Line Usage

```text
pawl run <file.al> [args...]       # type-check, then run (VM by default)
pawl run --jit <file.al> [args...] # opt into multi-tier AMD64 JIT compilation
pawl run --vm ...                  # require the VM; reject unsupported constructs
pawl run --interp ...              # use the tree-walking interpreter
pawl check <file.al>               # syntax + strict type check (CI friendly)
pawl test <file.al>                # run test blocks and summarize PASS/FAIL
pawl fmt [-w] <file.al>            # format to stdout, or rewrite in place
pawl repl                          # interactive session with persistent state
pawl lsp                           # Language Server Protocol over stdio
pawl build <file.al> [-o out]      # standalone executable for the host (PE/COFF)
pawl build <file.al> --shared [-o lib.dll] # compile into native shared library (.dll / .so / .dylib)
pawl targets                       # show supported targets and installed runtimes
```

---

## Package Manager (APM)

APM is hosted in its own repository: [**github.com/pawl-lang/apm**](https://github.com/pawl-lang/apm).

`apm` installs libraries so programs can `use` them without manual path setup:

```bash
# Clone & install APM
git clone https://github.com/pawl-lang/apm.git
cd apm && go build -o apm ./cmd/apm

apm install "json@^1.2.0"       # from package registry
apm show                        # list installed libraries
```

Installed libraries live in a local system registry under `$PAWL_HOME` (default `~/.pawl`), including a compiled VM build per package that the VM loads automatically.

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
pawl run --allow-all tests/test_01_strings_text.al
pawl run --allow-all tests/test_02_collections_algorithms.al
pawl run --allow-all tests/test_03_json_data.al
pawl run --allow-all tests/test_04_csv_ini_formats.al
pawl run --allow-all tests/test_05_math_financial.al
pawl run --allow-all tests/test_06_oop_ecommerce.al
pawl run --allow-all tests/test_07_error_handling_resilience.al
pawl run --allow-all tests/test_08_file_io_logging.al
pawl run --allow-all tests/test_09_crypto_security.al
pawl run --allow-all tests/test_10_datetime_bench.al
```

---

## License

Pawl is released under the [Apache License, Version 2.0](LICENSE).  
See [NOTICE](NOTICE) for copyright statements and third-party attributions.
