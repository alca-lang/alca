# Changelog

This file describes the work contained in the three commits following
`35dd058` (Release ULang++ 2.1): `cf3074f`, `b00ce4f`, `51e59a9`.

## Summary

The native C++ backend grew from a one-file AOT generator into a usable
bare-metal toolchain with a C++-like language surface. A separate UEFI
terminal OS was added under `os/`. Everything is covered by unit tests and a
QEMU boot smoke test.

## 1. Native compiler / backend (`cf3074f`)

### Low-level path

- Genuine Ring-0 intrinsics: `cli/sti/hlt/pause`, `in8/16/32`, `out8/16/32`,
  `read/write_cr0/cr2/cr3`, `invlpg`, `rdmsr/wrmsr`, `cpuid`, `lidt`, `iret`.
- Real Multiboot 1+2 ELF32 image with GDT, identity page tables and a
  32→64-bit trampoline; `kernel_main` is selected as entry point.
- Freestanding `print`/`println` writes to VGA text buffer and COM1 instead of
  being a `xor eax,eax; ret` stub.
- Fixed a latent UB in the freestanding heap pointer initialization
  (`uint32_t` shifted by 32–56 bits).
- `std/idt.upp` gained `write_gate`, `write_pointer`, `load`,
  `return_from_interrupt`; `std/pic.upp` gained `unmask_irq`; both were fixed
  so they actually lower to IR.

### Memory and runtime model

- Mutable top-level variables now live in a real data section:
  `Op::LoadGlobal` / `Op::StoreGlobal`, `GlobalVar`, `DataFixup`, with a
  separate `PT_LOAD` in ELF64, `.data` in PE32+, inline data in Multiboot.
- Function addresses are first-class: `Op::FuncAddr`, `Op::CallPtr`;
  `f := bump; f()` and `table[i]()` work, enabling manual vtables.
- First-fit free-list heap (`mem.alloc` / `mem.free`) with a 16-byte chunk
  header; freed blocks are reused. Hidden heap/error slots are emitted by the
  linker.
- RIP-relative addressing for all data access; Microsoft x64 shadow space
  reserved at call sites (`51e59a9`).

## 2. Language features (`cf3074f`, `51e59a9`)

- Generics (`func identity[T](x: T) -> T`) and function overloading,
  monomorphized at call sites into mangled functions.
- Operator overloading, e.g. `func +(a: Vec, b: Vec) -> Vec`.
- `const` enforcement: assigning to a const name is a typecheck error.
- `constexpr NAME := expr`: literals, arithmetic and shifts evaluated at
  compile time and inlined as `ConstInt`; no global slot, no runtime math.
- `defer expr`: cleanup on every exit path of a function.
- Implicit destructors `func ~Foo(self: Foo)` with **block-scoped** lifetime:
  objects are destroyed at block exit or on `return`, in reverse order.
- Exceptions: `throw value`, `try { ... } catch (e) { ... }`, typed
  `catch (e: Type)`, and `finally { ... }`. The error slot moved from
  callee-saved `r14` into hidden globals so exceptions survive calls;
  destructors run during unwinding.
- Typechecker now checks call arity; identical named types (generic params)
  are accepted; function-address arithmetic yields an integer.

## 3. UEFI application support and OS (`b00ce4f`, `51e59a9`)

- New `--target efi-app` / `--target uefi` and `linkEFIApp`:
  PE32+ `EFI_APPLICATION` (`Subsystem=10`), no imports, no CRT,
  `.text`/`.data`/`.bss`, 1 MiB heap area, RIP-relative globals, entry stub
  forwarding `ImageHandle` (RCX) and `SystemTable` (RDX) to `efi_main`.
- `os/` contains a minimal UEFI terminal written in ULang++:
  output via `EFI_SIMPLE_TEXT_OUTPUT_PROTOCOL -> OutputString`,
  input via `EFI_SIMPLE_TEXT_INPUT_PROTOCOL -> ReadKeyStroke`,
  UTF-16 conversion by hand, banner, character echo, Enter/Backspace handling.
- `os/build.sh` builds the compiler and `os/esp/EFI/BOOT/BOOTX64.EFI`;
  `os/run.sh` boots it in QEMU with combined OVMF or split Code/Vars firmware.

## 4. Tooling, tests and docs

- CI: native C++ unit tests and a boot smoke test (`scripts/test_boot.sh`).
- 69 unit tests passing, including globals, indirect calls, generics,
  overloads, heap/free, defer, destructors, exceptions, typed catch/finally,
  operators and constexpr.
- Golden token files renamed `.al.tokens` → `.upp.tokens` after the language
  rebrand; `compiler/tests/corpus.txt` updated.
- Docs: `README.md`, `README_EN.md`, `README_RU.md`, `docs/os_dev_EN.md`,
  `docs/stdlib_EN.md`, `os/README.md`, this `CHANGELOG.md`.
- Installers `install.ps1`, `install.sh`, `ulang.ico` restored so README
  installation instructions remain valid.

## Verification

```
69 passed, 0 failed                       (native unit tests)
bash scripts/test_boot.sh            → PASS (Multiboot/QEMU)
QEMU + edk2-x86_64-code.fd           → "ULANG UEFI TYPE", no exception
result/ulang++.exe                   → ULang++ 2.1 (rebuilt, *.exe ignored)
```

## Known limitations

- UEFI build is an application using firmware console protocols, not a kernel
  owning the machine: no own scheduler, paging, interrupt handling or FS.
- Rethrow and multiple sequential typed handlers are not implemented.
- Value semantics (`T&`), coroutines/atomics and a real cross-module object
  linker with relocations remain future work.
