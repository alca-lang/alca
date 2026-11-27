# ULang++ Programming Language

> **ULang++** is a modern, high-performance systems programming language designed for bare-metal OS development, high-throughput network services, and general systems programming with a native AOT C++ toolchain (**no external linker or libc required**).

---

## Key Features

- **Self-Contained Pipeline**: Custom Lexer, Parser, Typechecker, SSA-like IR, x86-64 machine code generator, and direct PE32+ (Windows) / ELF64 (Linux) image writers. There is **no separate linker**: no object files, symbol tables or relocations yet.
- **Bare-Metal & OS Capabilities (x86-64 only)**: Ring-0 intrinsics (`cpu.cli/sti/hlt/pause`, `in8/16/32`, `out8/16/32`, `read/write_cr0/cr2/cr3`, `invlpg`, `rdmsr/wrmsr`, `cpuid`, `lidt`, `iret`) and raw MMIO (`mem.read_u*/write_u*`). `--target freestanding-elf` emits a Multiboot 1+2 ELF32 image with GDT, identity page tables and a 32→64-bit trampoline; `kernel_main` is used as the entry point. See [the bare-metal notes](docs/os_dev_EN.md) for exactly what works.
- **Runtime**: bump allocation in generated binaries. The mark-sweep GC and green-thread scheduler in `compiler/src/runtime/` are **not linked into generated programs** yet.
- **Standard Library (`std/`)**: ~79 modules. Hosted-side modules (formats, crypto, networking, archives, SQLite3) have mixed coverage; the hardware modules (`idt`, `pic`, `pit`, `vga`, `serial`, `pci`, `keyboard`, `fat32`, `iso9660`, `elf_loader`) are low-level primitives — mostly register/MMIO helpers and header parsers, not complete drivers or filesystems.
- **Extensible Module System**:
  - `use mylib.upp`
  - `use "./../../mylib.upp" as custom_name`
  - Global packages in `ProgramData/ULangLibs/libs` (Windows) / `/var/lib/ULangLibs/libs` (Linux).
- **Dynamic Linking (DLL/SO)**: consuming DLLs is supported on Windows. Generating a DLL with a Win64 export directory is **not implemented** yet.

---

## Quick Start

### Installation

Run the automated installer:

**Windows (PowerShell):**
```powershell
.\install.ps1
```

**Linux / macOS:**
```bash
chmod +x ./install.sh
./install.sh
```

### Building & Running Programs

Create `main.upp`:
```ulang
func main() {
    println("Hello from ULang++!")
    x := 42
    println(x)
}
```

Compile directly to native binary:
```powershell
ulang++ build main.upp
.\main.exe
```

Or compile and run immediately:
```powershell
ulang++ run main.upp
```

### Compiler Commands

| Command | Usage | Description |
| :--- | :--- | :--- |
| `build` | `ulang++ build [-o <out>] [--target <t>] <file>` | Compiles source file to native executable / DLL / ELF |
| `run` | `ulang++ run <file>` | Compiles and executes binary |
| `check` | `ulang++ check <file>` | Runs typechecker and semantic diagnostics |
| `ir` | `ulang++ ir <file>` | Emits ULang++ Intermediate Representation |
| `parse` | `ulang++ parse <file>` | Parses and displays AST |
| `lex` | `ulang++ lex <file>` | Tokenizes source code |
| `version` | `ulang++ version` | Displays toolchain version |

### Building a kernel (verified path)

```ulang
// kernel.upp
use std.cpu
use std.idt

func kernel_main() {
    println("Hello from ring 0")
    idt.write_gate(524288, 0, 4096, 8, 142)
    idt.write_pointer(524288, 256, 524272)
    idt.load(524272)
    loop {
        cpu.hlt()
    }
}
```

```powershell
ulang++ build --target freestanding-elf -o kernel.elf kernel.upp
qemu-system-x86_64 -kernel kernel.elf -display none -serial stdio
```

`println()` in freestanding mode writes to both the VGA text buffer and COM1.
Automated smoke test: `bash scripts/test_boot.sh` (needs `g++` and QEMU).

Mutable globals live in a real data section, and functions can be passed
around (`f := bump; f()`), which is enough for manual dispatch tables.

Generic functions (`func identity[T](x: T) -> T`) and function overloading are
monomorphized at call sites into mangled specializations such as `identity_int`.
`mem.alloc`/`mem.free` use a first-fit free-list allocator (freed blocks are
reused). `defer expr` runs on every exit path, and `func ~Foo(self: Foo)` is an
implicit destructor called for every local `Foo` when its block/function exits.
`throw value` and `try { ... } catch (e) { ... }` propagate exceptions through a
memory-backed error slot and unwind destructors on the way out.

**Still missing:** IDT handler dispatch (ISR stubs/interrupt gates at runtime),
preemptive scheduling, virtual memory management, real filesystems, syscalls,
user mode (ring 3), separate object files/relocations, rethrow/nested typed
handlers, coroutines/atomics, value semantics (`T&`) and
`float`/struct values in `println()`. Operator overloading
(`func +(a: Vec, b: Vec)`) and `const` reassignment checks now work.
Details: [docs/os_dev_EN.md](docs/os_dev_EN.md).

