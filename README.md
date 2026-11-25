# ULang++ Programming Language

> **ULang++** is a modern, high-performance systems programming language designed for bare-metal OS development, high-throughput network services, and general systems programming with a native AOT C++ toolchain (**no external linker or libc required**).

---

## Key Features

- **Self-Contained Pipeline**: Custom Lexer, Parser, Typechecker, SSA-like IR, x86-64 machine code generator, and direct PE32+ (Windows) / ELF64 (Linux & Multiboot) object writers.
- **Bare-Metal & OS Capabilities**: Direct CPU intrinsics (`cpu.cli()`, `cpu.sti()`, `cpu.hlt()`, `cpu.in8()`, `cpu.out8()`), CR0/CR2/CR3 register access, MMIO physical memory drivers (`mem.write_u16`, `mem.read_u32`), Multiboot 1/2 ELF headers and raw flat binaries.
- **Memory Safety & Runtime**: Capability-based sandbox, built-in Mark-Sweep Garbage Collector, M:N cooperative fibers / green-threads async runtime.
- **Rich Standard Library (`std/`)**: Over 79 fully native modules covering crypto, async/event loop, files, POSIX/Win32 OS APIs, networking (Ethernet, ARP, ICMP, TCP/UDP sockets), data formats (JSON, XML, HTML, CSV, INI), archives (ZIP, TAR, GZ, ZSTD, LZMA, BZ2), databases (SQLite3), and hardware drivers (VGA, PIC, PIT, IDT, PCI, Serial UART).
- **Extensible Module System**:
  - `use mylib.al`
  - `use "./../../mylib.al" as custom_name`
  - Global packages in `ProgramData/ULangLibs/libs` (Windows) / `/var/lib/ULangLibs/libs` (Linux).
- **Dynamic Linking (DLL/SO)**: Create and consume shared libraries with Win64 ABI export tables (`ulang++ build --target dll`).

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

Create `main.al`:
```ulang
func main() {
    println("Hello from ULang++!")
    x := 42
    println(x)
}
```

Compile directly to native binary:
```powershell
ulang++ build main.al
.\main.exe
```

Or compile and run immediately:
```powershell
ulang++ run main.al
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
