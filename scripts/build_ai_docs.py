import os
import re
from pathlib import Path

ROOT_DIR = Path(__file__).resolve().parent.parent

OUTPUT_FILE = ROOT_DIR / "docs" / "ULANG_COMPLETE_SPEC_FOR_AI.md"

LLM_PREAMBLE = """# ULang++ 2.1 Complete Language & Architecture Specification (LLM Reference)

> **Target Audience: Large Language Models (LLMs), AI code assistants, compiler engineers.**
> **Language Name:** ULang++ (version 2.1)
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
u++ version                                          # Output "ULang++ 2.1"
```

---
"""

def clean_content(text: str) -> str:
    # Replace any leftover mentions of Pawl with ULang++
    text = re.sub(r'\bpawl\b', 'ulang++', text, flags=re.IGNORECASE)
    text = re.sub(r'\.al\b', '.upp', text)
    return text

def build_single_doc():
    docs_dir = ROOT_DIR / "docs"
    out_lines = [LLM_PREAMBLE]

    # Selected core documentation files to merge in logical reading order for AI
    core_files = [
        "language_EN.md",
        "stdlib_EN.md",
        "cli_EN.md",
        "architecture_EN.md",
        "handbook_EN.md",
        "cross-compilation_EN.md"
    ]

    for filename in core_files:
        filepath = docs_dir / filename
        if filepath.exists():
            out_lines.append(f"\n\n## --- Documentation Section: {filename} ---\n\n")
            with open(filepath, "r", encoding="utf-8", errors="replace") as f:
                content = clean_content(f.read())
                out_lines.append(content)

    # Append ADRs (Architecture Decision Records)
    adr_dir = docs_dir / "adr"
    if adr_dir.exists():
        out_lines.append("\n\n## --- Architecture Decision Records (ADRs) ---\n\n")
        for adr in sorted(adr_dir.glob("*.md")):
            out_lines.append(f"\n### ADR: {adr.name}\n\n")
            with open(adr, "r", encoding="utf-8", errors="replace") as f:
                out_lines.append(clean_content(f.read()))

    OUTPUT_FILE.parent.mkdir(parents=True, exist_ok=True)
    with open(OUTPUT_FILE, "w", encoding="utf-8") as f:
        f.write("\n".join(out_lines))

    print(f"Successfully generated single-file AI documentation: {OUTPUT_FILE} ({os.path.getsize(OUTPUT_FILE)} bytes)")

if __name__ == "__main__":
    build_single_doc()
