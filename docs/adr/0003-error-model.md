# ADR-0003: Error model

- Status: Accepted
- Date: 2026-11-25

## Context

Pawl has first-class errors (`err(...)`, `error` type, `?>>>` propagation) and
the runtime can fail (I/O, allocation, sandbox denial). Generated machine code
cannot unwind C++ exceptions, and the compiler must preserve Pawl-visible
error semantics.

## Decision

Separate three layers:

1. **Compiler internals (C++)** — may use exceptions and `std::expected`-style
   results freely. They never cross into generated code.
2. **Runtime helper ABI** — no C++ exceptions. Every helper returns a status
   word in a dedicated register and writes its result through an out-pointer:

   ```text
   status = runtime_call(ctx, args..., &out)
   status: 0 = OK, nonzero = RuntimeErrorCode
   ```

   On x64 the status is `rax`; on ARM64 `x0`. The out-pointer is a normal
   argument. A null status result is never a valid Pawl value.
3. **Pawl-visible errors** — ordinary `Value` of kind `Error`, carrying
   `{ u32 code; String message; Value cause; u64 trace_id; }`. Functions that
   can fail still return a normal `Value`; `?>>>` checks the `Error` kind and
   returns early, matching the Go semantics.

Runtime helpers that can fail have a sibling `_or_error` form returning a
Pawl `Error` value, used by stdlib bindings; low-level helpers use status codes
to keep the hot path branch-free.

### Panics

- Internal invariants and out-of-memory use `pawl_panic(code, message)`.
- `pawl_panic` writes a diagnostic to stderr, flushes, and terminates the
  process using `ExitProcess` (Windows) or `exit_group` (Linux).
- `panic` never returns; its IR node is `NoReturn`.
- No C++ destructors or unwinding run in the generated program.

### Sandbox denials

Capability denial is a normal `Error` with a stable code (e.g.
`E_CAPABILITY_DENIED`) and a message naming the capability and resource. It is
not a panic, so programs can recover exactly as in the Go implementation.

## Consequences

- IR has `CallStatus` and `CheckError` pseudo-nodes.
- Stack traces are explicit: each frame pushes a `(func_id, pc)` record into a
  runtime trace buffer; `Error.trace_id` indexes it.
- Tests assert status codes and Pawl error messages/positions.

## Alternatives considered

1. **C++ exceptions through generated code** — rejected: requires unwind
   tables/CRT, UB across hand-written asm, and violates "no CRT".
2. **`std::expected` at the runtime ABI** — rejected: it is a C++ type with
   unspecified layout; not an ABI.
3. **Single return value with sentinel tags** — rejected: complicates every
   call site and loses the distinction between Pawl `Error` values and runtime
   faults.
