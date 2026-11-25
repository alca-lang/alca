# ADR-0004: Embedded runtime and object-image format

- Status: Accepted
- Date: 2026-11-25

## Context

`pawl build` may not invoke `gcc`, `clang`, `ld`, `as`, `nasm`, `llvm-*`, or
`go`. The runtime and stdlib must therefore be compiled **before** the Pawl
project is built, embedded into the `pawl` executable, and merged with user
code by the compiler's own linker.

We also need PE32+ and ELF64 output for x86-64 and ARM64. A full ELF/PE object
format is unnecessary internally; a small, explicit relocatable format is
easier to verify and keeps the same code path for both targets.

## Decision

Define **POB (Pawl Object Binary)**, a little-endian relocatable image:

```text
magic  "POBS" (4 bytes)
version u32
arch    u16   (1 = x86_64, 2 = aarch64)
os      u16   (0 = any, 1 = windows, 2 = linux)
flags   u32
sections:
  .text    u8[]   executable
  .rodata  u8[]   read-only
  .data    u8[]   writable, zero-initialized tail allowed
  .bss     u64 size
symbols:
  name[] (NUL-separated string table, u32 offsets)
  { name_off u32; section u16; value u64; size u64; binding u8; kind u8 }
relocations:
  { section u16; offset u64; symbol u32; type u8; addend i64 }
imports:
  { name_off u32; library_off u32 }   // Windows: DLL + symbol; Linux: vDSO/syscall id
```

Relocation types: `ABS64`, `REL32`, `PLT32`, `PCREL32`, `ADDR32NB` (PE),
`TLS` (reserved). The writer is explicit little-endian; no C structs are
written to disk.

### Build pipeline

1. When building the **compiler** (host step, outside Pawl project builds):
   - compile runtime/stdlib C/C++ sources with the host C++ toolchain;
   - convert their objects into `.pob` with `tools/pobgen` (part of the
     compiler build, run once);
   - embed the `.pob` images as `constexpr unsigned char[]` arrays in a
     generated C++ translation unit (`runtime_images.cpp`), selected by
     arch/OS.
   - Alternatively, once the Pawl backend is self-hosting enough, runtime
     sources in Pawl can be emitted by the same backend. The embedding step is
     unchanged.

2. When building a **Pawl project**:
   - compile every module to `.pob` (text/rodata/data + symbols);
   - link user `.pob` files plus the embedded runtime/stdlib `.pob` for the
     selected target with the compiler's own resolver;
   - apply relocations, lay out sections, resolve imports to OS DLLs or raw
     syscalls;
   - emit PE32+ or ELF64 directly. No system linker, no external process.

### Symbol policy

- Runtime symbols are prefixed `pawl_`; stdlib symbols `pawl_std_`.
- Name mangling is deterministic and documented (`pawl_m_<module>_<name>` for
  module globals; `<module>.<Type>.<method>` names are hashed only when
  necessary, never relying on C++ ABI).
- User code cannot accidentally override runtime symbols; the linker resolves
  runtime first and reports a collision.
- Each image records a content hash; `--verbose-build` prints the hash of every
  embedded runtime image.

### PE/ELF specifics

- PE: one `.text`, `.rdata`, `.data`, `.bss`; import directory built from
  `imports`; base relocations only if the image is not fixed-base (we prefer
  fixed base / `IMAGE_FILE_RELOCS_STRIPPED` for executables). ARM64 uses the
  same writer with machine = `IMAGE_FILE_MACHINE_ARM64`.
- ELF: `ET_EXEC`, two or more `PT_LOAD`, no `PT_INTERP`, no `.dynamic`, no
  libc. ARM64 uses `EM_AARCH64`; x86-64 uses `EM_X86_64`.
- Both writers share a `Layout` abstraction (sections, symbols, relocations)
  so the backend and linker stay target-agnostic.

### Security/protection

The documented `--protect` flag is implemented as:
- strip symbol/name tables from output unless `--debug`;
- mark `.text` RX, `.rodata` R, `.data` RW (PE section characteristics,
  ELF `p_flags`);
- optional W^X enforcement check before writing;
- no fake flags: anything not implemented is rejected with a diagnostic.

## Consequences

- `.pob` must be versioned and fuzzed; malformed input produces diagnostics,
  not crashes.
- Embedded images increase the `pawl` binary size; a build option can embed
  only the needed modules per target (LTO-like pruning at module granularity).
- Reproduction is byte-for-byte deterministic for identical inputs and
  compiler version.

## Alternatives considered

1. **Emit ELF/PE objects and shell out to a linker** — forbidden.
2. **Trampoline through libc/CRT** — forbidden.
3. **One monolithic precompiled blob per OS/arch** — rejected: prevents
   stdlib pruning and makes symbol resolution opaque.
4. **LLVM MC/object** — forbidden (external toolchain/LLVM dependency).
