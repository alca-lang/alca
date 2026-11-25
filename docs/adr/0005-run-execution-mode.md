# ADR-0005: `pawl run` execution mode

- Status: Accepted
- Date: 2026-11-25

## Context

The legacy `pawl run` uses the Go VM/interpreter/JIT. The native compiler must
run programs without any interpreter, bytecode or JIT. Two options were
allowed by the migration brief:

1. build a temporary executable and spawn it;
2. generate code in memory and load/execute it.

## Decision

Use **build-to-temporary-file + spawn** for `pawl run` in v1:

1. compile the project exactly like `pawl build` into a temporary file inside
   the OS temp directory (or `--out` if given);
2. spawn it with inherited stdio, passing program arguments and environment;
3. propagate the exit code; delete the temp file on exit unless `--keep`;
4. no compiler process is launched during the build itself; the only spawned
   process is the user's freshly built program.

Rationale: correctness, debuggability, identical semantics to `pawl build`,
and no executable-memory loader to secure. The Windows loader and Linux
`execve` path are the same code the OS uses for the shipped binary.

An **optional** `--in-memory` mode may be added later: allocate RW memory,
apply relocations, `mprotect`/`VirtualProtect` to RX, and call the entry point.
It must use the same `.pob` linker and runtime, never an interpreter. It is
off by default and not required for parity.

## Consequences

- `run` needs a writable temp directory and the ability to execute files there.
  On failure, a clear diagnostic is emitted and no fallback interpreter exists.
- Tests assert that `run` never loads a VM/bytecode module and that the child
  process is the compiled binary.
- Sandbox capabilities are enforced in the runtime of the child, not by the
  parent compiler.

## Alternatives considered

1. **In-process interpreter** — forbidden by the product requirements.
2. **JIT in memory** — rejected for v1: more attack surface, harder debugging,
   and unnecessary for correctness; remains an optional future mode.
3. **Pipe source to a hidden VM** — forbidden.
