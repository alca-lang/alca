# ADR-0002: Memory model and allocator

- Status: Accepted
- Date: 2026-11-25

## Context

The output binary must not link libc/CRT and must not depend on a Go runtime.
Pawl programs allocate strings, lists, maps, objects, closures and errors.
The compiler itself can use the host C++ runtime, but generated code cannot.

## Decision

Use a **non-moving, precise mark-sweep collector** over a **size-class slab
allocator**, backed directly by OS virtual memory:

- Windows: `VirtualAlloc` / `VirtualFree` (`MEM_COMMIT | MEM_RESERVE`).
- Linux: `mmap` / `munmap` with `MAP_PRIVATE | MAP_ANONYMOUS`.

No `malloc`, `free`, `new`, `delete`, `mmap` wrapper from libc, no CRT startup.

### Object model

- Every heap object starts with a 32-byte header:
  `{ u32 kind; u32 flags; u64 size; u64 type_id; u64 mark; u64 reserved; }`.
- Allocation size classes: 32, 48, 64, 96, 128, 192, 256, 384, 512, 768, 1024,
  then whole pages for larger objects. Each class has a free list and a span
  list.
- Large objects (> 16 KiB) are page-aligned and tracked separately.
- Strings and blobs are immutable after construction; this permits sharing and
  makes hashing stable.
- Maps use open addressing with tombstones; list growth is amortized doubling.

### GC

- Stop-the-world mark-sweep. No moving, so raw pointers held by native handles
  and FFI remain valid.
- Roots: shadow stack of `Value` slots plus a per-frame stack map emitted by
  the codegen backend (precise). Registers are spilled at safepoints.
- Global roots: module globals, constants, runtime tables, active closures.
- Mark phase uses an explicit worklist (no recursion) with a mark bit in the
  object header.
- Sweep returns whole free spans to the OS when a span becomes empty.
- Collection is triggered by allocation bytes since last GC and a configurable
  heap limit; `pawl` programs can call `gc.collect()`.
- No finalizers in v1; resources (files, sockets, handles) have explicit
  `close` and a deterministic `defer`/scope-exit mechanism.

### Arenas

- The compiler (C++) uses arenas and `std::unique_ptr` for AST/IR; this is
  host-only and unrelated to the generated runtime.
- Generated code may use a per-function bump arena for temporary strings/lists
  that provably do not escape; this is an IR optimization and is disabled in
  debug builds to keep GC stress tests meaningful.

### Threads and async

- Each OS thread has its own allocation buffer and shadow stack.
- A global stop-the-world handshake parks threads at safepoints.
- `async`/`ba` tasks are scheduled by a native runtime scheduler; task stacks
  are GC roots.

## Consequences

- Codegen must emit stack maps and safepoints. Until that exists, Stage 4 uses
  a conservative scan of the shadow stack only (all slots are `Value`-shaped),
  which is sound because the runtime knows every shadow slot is a `Value`.
- No destructors/finalizers means stdlib APIs need explicit `close`.
- ASan/UBSan apply to the compiler process, not to generated code; generated
  code gets its own redzone/canary debug mode.

## Alternatives considered

1. **Reference counting** — rejected: cycles through closures/classes are
   common in Pawl programs; weak/cycle collection adds complexity.
2. **Copying/compacting GC** — rejected: moves objects, invalidates FFI
   pointers and native handles.
3. **Arena-only / never free** — rejected for long-running servers and GUI.
4. **libc malloc** — forbidden by the product requirements.
