# Bare-metal / OS development — what actually works

This document describes the **current** native x86-64 backend, not the roadmap.
It is intentionally blunt: several capabilities advertised elsewhere are not
implemented yet.

## Verified end-to-end path

```
ULang++ source -> native compiler -> Multiboot ELF32 -> QEMU -kernel -> ring 0
```

`ulang++ build --target freestanding-elf -o kernel.elf kernel.upp` produces an
ELF32 (`EM_386`) image containing:

- Multiboot 1 header (`0x1BADB002`) and Multiboot 2 header with checksum;
- a flat 64-bit GDT and a 32→64-bit trampoline (PAE, EFER.LME, CR0.PG|PE,
  far jump);
- identity page tables for the first 8 MiB (4 × 2 MiB pages);
- a free-list heap allocator (`mem.alloc`/`mem.free`), string helpers and a
  VGA+COM1 `print`/`println`;
- `kernel_main` (or `main`) called from the long-mode stub.

`bash scripts/test_boot.sh` builds the compiler and boots
`tests/boot/kernel_smoke.upp` under QEMU, asserting serial markers.

## Supported intrinsics

| Builtin | Lowered to | Notes |
|---|---|---|
| `cpu.cli/sti/hlt/pause` | `cli`, `sti`, `hlt`, `pause` | real opcodes |
| `cpu.in8/16/32`, `cpu.out8/16/32` | `in`/`out` | DX-addressed port I/O |
| `cpu.read_cr0/cr2/cr3`, `cpu.write_cr0/cr3` | `mov rax, crN` / `mov crN, rax` | `write_cr2` is not declared |
| `cpu.invlpg(addr)` | `invlpg [rax]` | |
| `cpu.rdmsr(ecx)`, `cpu.wrmsr(ecx, value64)` | `rdmsr` / `wrmsr` | value is a 64-bit `int` |
| `cpu.cpuid(leaf, sub)`, `cpuid_eax/ebx/ecx/edx` | `cpuid` | returns one output register each |
| `cpu.lidt(idtrAddr)` | `lidt [rax]` | pointer is the 6-byte IDTR |
| `cpu.iret()` | `iret` | for handler returns |
| `mem.read_u8/16/32/64`, `write_u8/16/32/64` | `RawLoad`/`RawStore` | raw linear addresses |
| `mem.read_i8/i16` | `movzx` | zero-extends (not sign-extends) |

## `std.*` hardware modules

- `idt`: `write_gate`, `write_pointer`, `load`, `return_from_interrupt` work.
  `make_descriptor`/`write_entry` use struct values, which the backend cannot
  return correctly yet — prefer `write_gate`.
- `pic`: `remap`, `send_eoi`, `mask_irq`, `unmask_irq` (real port I/O).
- `pit`: `init(freqHz)` (real PIT programming; no tick handler/scheduler).
- `vga`, `serial`, `pci`, `keyboard`: primitive register/MMIO helpers.
- `fat32`, `iso9660`, `elf_loader`: header decoders only, no data reads.

Module-level `NAME := literal` constants that are never assigned are substituted
at IR lowering. Names that are assigned become **real mutable globals** in an
8-byte-per-slot data section (a separate `PT_LOAD` in ELF64, `.data` in PE32+,
inline data in the Multiboot image), so they can be shared across functions.

Functions are first-class values: `f := bump; f()` and `table[i]()` lower to
`func_addr`/`call_ptr` and execute via `call rax`. This is enough to build
manual dispatch tables (a hand-rolled vtable).

Generic functions (`func identity[T](x: T) -> T`) and overloads are resolved at
call sites: the lowerer infers `T` from the argument types and emits a mangled
monomorphized function (`identity_int`, `identity_float`, ...). Mutable globals
can be used from those specializations normally.

**Memory management:** `mem.alloc(size)` / `mem.free(ptr)` use a first-fit
free-list allocator (16-byte chunk header; freed blocks are reused by the next
matching allocation). `defer expr` schedules cleanup on every return path of the
current function, so `defer mem.free(p)` gives explicit RAII-style ownership.
`func ~Foo(self: Foo)` declares an implicit destructor: every local `Foo` is
registered at construction and destroyed in reverse construction order when its
lexical block ends, or on every `return` out of that block (function body
included).

**Exceptions:** `throw value` writes the value and a separate "has error" flag
into two hidden globals (r14 was callee-saved and could not survive calls).
`try { ... } catch (e) { ... }` checks the flag after every call in the try body
and jumps to the handler on error. Handlers clear the flag; `e` receives the
raised value. A `throw` returns from the throwing function, so all active
destructors run during unwinding.

`catch (e: Type)` filters by a compile-time hash of the thrown expression's
static type; a non-matching exception skips that handler. `finally { ... }`
runs on both the normal and caught paths before leaving the `try` statement.
Rethrowing and multiple sequential typed handlers are not implemented yet.

**Operators and const:** a function named after an operator
(`func +(a: Vec, b: Vec) -> Vec`) is monomorphized and called for infix
expressions whose operands include a custom (pointer-typed) value; builtin
integer/float operators are unchanged. `const x := ...` is now enforced: assigning
to a const name is a typecheck error.

`constexpr NAME := expr` evaluates literals, arithmetic, and shifts at compile
time and inlines a `ConstInt` at each use — no global slot and no runtime
arithmetic. Non-constant initializers are rejected by the typechecker.

The typechecker now rejects wrong argument **counts** for functions with known
signatures (optional/variadic parameters are respected). Argument *types* are
still only partially checked.

## Known limitations

- **Still no separate linker:** one code buffer plus rodata/data and fixups;
  no object files or cross-module symbol resolution beyond source-level
  `use`. Helper names are patched by a hardcoded allow-list. The ADR-0004
  "POB" object format remains a design document only.
- **No interrupts end-to-end:** `lidt` loads a table, but there is no ISR
  dispatch and no `iret`-returning handler generator.
- **No memory management:** flat identity map only; no paging management,
  page-fault handler, user/kernel split or TLB shootdown.
- **No processes/scheduling/syscalls/ring 3.** `sched`, `signal`,
  `multiprocessing`, `mmap` are stubs; `mmap.map_anonymous` returns a fixed
  fake address.
- **`println(int)`** is a no-op: `int_to_str` is still a stub. Print strings
  directly or write bytes with `cpu.out8`/`mem.write_u8`.
- **`--target freestanding-raw`** drops rodata and does not apply call fixups.
- **Hosted side:** ELF64 uses raw Linux syscalls (`brk`/`write`/`exit`); PE32+
  imports KERNEL32 only. No CRT/libc is linked.
- The mark-sweep GC and fibers in `compiler/src/runtime/` are host-side code
  and are not compiled into generated programs.

