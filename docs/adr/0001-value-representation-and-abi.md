# ADR-0001: Value representation and calling convention

- Status: Accepted
- Date: 2026-11-25
- Deciders: Pawl native migration

## Context

The native compiler must support `int` (64-bit), `float` (IEEE-754 binary64),
`bool`, `string`, `list`, `map`, `blob`, `secret`, `json`, `struct`, `class`,
`enum`, `interface`, closures, errors and `null`, on x86-64 and AArch64, with
one backend-agnostic IR. The representation determines codegen, GC, runtime
and stdlib bindings, so it must be fixed before the IR is lowered.

Constraints:
- no libc/CRT in the produced binary;
- no VM, bytecode or interpreter;
- Windows and Linux, x86-64 and ARM64;
- predictable ABI across object images produced by the same compiler.

## Decision

Use a **16-byte tagged value**, two machine words:

```c++
struct Value {          // alignof == 16, sizeof == 16
    std::uint64_t tag;  // kind + flags
    std::uint64_t payload;
};
```

`tag` layout:

```text
bits 63..56  flags   (immutable, heap, weak, has-error, reserved)
bits 55..48  reserved (must be zero)
bits 47..0   kind
```

Initial kinds:

| Kind | Payload |
|---|---|
| `Null` | 0 |
| `Bool` | 0 / 1 |
| `Int` | two's-complement int64 |
| `Float` | IEEE-754 binary64 bit pattern |
| `String` | pointer to `StringObject { u64 len; u64 cap; u8 data[]; }` |
| `List` | pointer to `ListObject` (elements are `Value`) |
| `Map` | pointer to `MapObject` (open-addressing, hash of Value) |
| `Blob` | pointer to `BlobObject { u64 len; u8 data[]; }` |
| `Struct`/`Class` | pointer to object with type id + fields |
| `Closure` | pointer to `ClosureObject` (code ptr + captured env) |
| `Error` | pointer to `ErrorObject` (code + message + cause) |
| `Json` | pointer to a typed JSON tree |

All heap pointers are 16-byte aligned; the low 4 bits are available for future
inline tagging but are not used in v1.

### ABI

- Every Pawl value is passed as two 8-byte words.
- x86-64 SysV: first six integer-class argument slots in `rdi, rsi, rdx, rcx,
  r8, r9`; a 16-byte value consumes two consecutive slots (a value never
  straddles the register/stack boundary). Return in `rax:rdx`; a second return
  channel (error code) uses `r10` or an out-pointer. Stack arguments are
  16-byte aligned.
- AArch64 AAPCS64: `x0..x7` for argument words, return in `x0:x1`, indirect
  result pointer in `x8` when needed. Stack arguments are 16-byte aligned.
- `bool` is still a 16-byte Value at the language boundary; internal IR may
  specialize it to one word.
- `float` is boxed as a `Value` at ABI boundaries. The x64/ARM64 backends may
  unbox arithmetic trees internally (`FloatAdd` etc.) as an optimization that
  is invisible to the ABI.
- Hidden first argument: execution context pointer (`*Runtime`) in `r15`
  (x64) / `x28` (ARM64) as a reserved global register, never a normal argument.
- Closures receive the captured environment pointer as an implicit argument.
- The runtime never returns a C++ object by value across generated code.

### Why not NaN-boxing (64-bit)

NaN-boxing is faster and halves memory, but:
- it steals float NaN space and makes signaling/NaN payload preservation subtle;
- pointer tagging conflicts with future 5-level x86-64 / 52-bit ARM64 VAs;
- representing a string needs an extra length word anyway, and classes need
  type metadata, so the saving is smaller than it looks;
- debugging native code without box metadata is materially harder during a
  multi-stage port.

We choose correctness and a simple ABI first. A future optimization may add an
unboxed fast path per function while keeping the 16-byte ABI.

### Why not 128-bit "struct with union + aux"

The chosen pair is the smallest representation that carries an arbitrary
64-bit immediate or pointer **and** an independent tag. It returns in two
registers on both targets, needs no hidden sret for the common case, and can be
serialized deterministically.

## Consequences

- IR must model `Value` explicitly; no `double`-only or pointer-only nodes.
- The runtime object header is shared by all heap kinds:
  `{ u32 kind; u32 flags; u64 size; u64 type_id; }`.
- Equality/hash/format helpers are runtime functions, not compiler-inlined
  layout assumptions.
- Golden ABI tests will assert register/stack placement on both backends.

## Alternatives considered

1. **64-bit NaN-boxing** — rejected for v1 (see above).
2. **Untagged 128-bit variant** — rejected: every operation would pay a tag
   dispatch anyway, and FFI/runtime boundaries would lose type safety.
3. **Pointer-to-boxed-value everywhere** — rejected: allocation on every
   integer operation.
