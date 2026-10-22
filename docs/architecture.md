# Architecture

Alca has two execution engines fed by one front end. The VM is the default; the
interpreter is the reference semantics and the fallback for constructs the VM
does not implement.

## Pipeline

```text
source .al
  -> pkg/lexer        tokens (INDENT/DEDENT, comments, literals)
  -> pkg/parser       AST (pkg/ast), structured syntax diagnostics
  -> pkg/typecheck    strict types, scopes, stdlib signature tables
  -> pkg/vm           bytecode compiler + VM      (default)
     or pkg/interpreter tree-walking evaluator    (reference/fallback)
```

Both engines receive the same builtin environment built by
`cmd/alca.newInterpreter()`:

- `pkg/stdlib` registers modules and top-level builtins.
- `pkg/interpreter` owns values, environments, and core builtins.
- `pkg/vm` uses the interpreter environment for builtin calls, so standard
  library behavior is shared rather than duplicated.

## Parser

`pkg/parser` is a Pratt parser. It records structured diagnostics with source
positions. Indentation is significant: block bodies are parsed at an increased
indent, and an unindented statement after a block header is rejected. A block
may contain only comments and still be a valid empty body.

## Type checker

`pkg/typecheck` is a separate pass with no runtime side effects. It enforces:

- fixed inferred types, no implicit numeric or string conversions;
- nullable narrowing after `!= null` / `== null`;
- declared function and stdlib signatures, including optional, variadic, and
  nullable parameters/returns;
- generic element fitting for `append`/`push`/`pop` and collections;
- no shadowing: a name that is already bound keeps its binding, and a
  redeclaration with another type is an error;
- no `any` in declarations (`json` is the dynamic type).

`pkg/typecheck/builtins_table.go` is the canonical stdlib signature table used
by the checker, LSP, and completion. Signatures are verified against real
runtime behavior; an empty signature means a dynamic `func` returning `json`
only where a precise signature is impossible.

## Interpreter

`pkg/interpreter` walks the AST. It is the semantic reference for:

- closures and mutable captured environments;
- loop iteration and final loop-variable behavior;
- error values and `?>>>` capture;
- stream operators and non-VM constructs (`threads`, `ba`, `tx`, `watch`, ...).

## VM

`pkg/vm` compiles the AST to bytecode and executes it with a stack machine.
Closures use upvalues:

- `OpClosure` captures locals by reference (`FreeScope`, `FreeSymbols`,
  `NumFree`);
- captured descriptors are serialized in bytecode bundles;
- `OpGetFree`/`OpSetFree` read and update the shared binding.

The compiler must explicitly return `*vm.UnsupportedError` for anything it
cannot compile. It must never silently skip a statement or emit code with
different semantics. `alca run` uses that error to fall back to the interpreter;
`--vm` reports it to the user.

Unsupported today includes shell functions, `ba`/`threads`/`watch` blocks,
timeout expressions, capture streams used as values, and keyword arguments in
a call whose callee is only known at run time. The concurrent constructs need
a virtual machine that can run more than one frame stack at a time; the
interpreter forks an environment per task, the virtual machine does not have
that yet.

A closure is a copy of its template that only differs in the values it
captured. The JIT compiles the code of the template once and shares the
artifact between all instances of that closure, so a program that creates many
closures does not compile the same code again and again. The captured values
stay per instance: the runtime helpers read them from the closure that is
executing, not from the shared template. The number of compiled artifacts is
bounded (`jit.Config.MaxArtifacts`, 4096 by default); the functions that do not
fit are executed by the bytecode VM.

## Engine parity

`cmd/alca/engine_test.go` contains a corpus run on both engines. Every test
requires identical stdout and zero exit status on each engine. New language
features and VM fixes must add a corpus entry (or a dedicated two-engine
regression test when setup such as a sandbox policy is required).

`cmd/alca/examples_test.go` additionally type-checks every shipped `.al` file,
rejects the negative fixtures, and runs deterministic examples on both engines.

## Priorities

The order of work is deliberate, and it is not "more features":

1. **Engine parity.** Every construct the interpreter supports should reach the
   bytecode VM and the JIT tiers. The list of exceptions in
   [language.md](language.md#engines) is the backlog: shell functions, the
   concurrent blocks (`ba`, `threads`, `watch`) and timeout expressions, which
   need a virtual machine that can run several frame stacks at once.
2. **One predictable rule per operation.** Equality, coercion, error values
   and the memory guard follow a written table instead of a set of special
   cases. A rule that is documented but not implemented is a bug in the same
   sense as a failing test.
3. **Documentation that matches the implementation.** The two engines are
   compared by the output-parity corpus, and every behaviour that differs
   between them is listed. A claim in `README.md` that the VM does not keep
   is fixed in the README, not left to the reader.
4. **New features last.** A feature that only the interpreter supports adds a
   second implementation of the same semantics, so it is only worth it when
   the rules it depends on are already settled.

## Compiler and bundles

`pkg/compiler` creates standalone files:

```text
[runtime stub for target] [payload] [8-byte size] [ALCA_EXEC_V2 magic]
```

The payload is a versioned, SHA-256-checked gzip envelope. It contains either
serialized VM bytecode (default) or source plus imported module files. Building
from an already-built program strips the old payload first, so payloads never
stack.

The runtime stub detects the footer, extracts the bundle, and runs it. Host
builds use the running CLI executable as the stub; cross builds use matching
stubs from a runtime directory. See [cross-compilation.md](cross-compilation.md).

## Formatting and tooling

- `pkg/format` formats tokens/lines with canonical four-space indentation and
  verifies that the formatted text re-parses to an equivalent AST.
- `pkg/lsp` speaks LSP over stdio and computes diagnostics, hover, and
  completion locally from the parser/checker and stdlib tables.
- `editors/vscode` contains a dependency-free extension that shells out to
  `alca` for diagnostics and formatting.

## Security policy

`pkg/sandbox` holds the process-wide capability policy. It is consulted at
builtin call time, so both engines enforce the same read/write/network grants.
The guarded APIs include standard file operations, streams, archive/format
helpers, logging to a file, network clients and servers, metrics export, and
database connection paths. HTTP redirects are re-checked against the policy.
`cmd/alca` starts execution commands with deny-all and applies leading
`--allow-*` flags. Library/tests default to allow-all for embedding
compatibility; CLI and standalone entry points always install an explicit
policy.

The global `exec` builtin and the `process`, `env`, `ffi`, `gui`, and `sys`
modules provide trusted host access and are intentionally outside the
read/write/network policy. SQL statements are also not inspected; granting a
database connection lets that engine access its own resources. Do not run
untrusted code with those facilities.

## Memory accounting

`pkg/limits` holds the process-wide memory meter. It keeps two numbers: the
memory an Alca program currently holds (list elements, map entries, strings,
environment bindings, the fixed structures of a virtual machine, and compiled
JIT artifacts) and the total it has allocated. The counters are updated where
the values are created, so the estimate follows the data of the program.

The meter does not try to catch a Go out-of-memory condition: that error is
fatal and cannot be recovered. The guard therefore works ahead of the
allocator. The engines check the meter periodically - every few hundred
instructions in the bytecode VM and the tree-walking interpreter, and at every
helper boundary in the JIT, which is where the native tiers allocate - and
raise a regular Alca error (code 507) when a limit was reached. The error can
be handled by the program with `?>>>`.

`cmd/alca` installs the limits from `--max-memory` and `--max-allocation`.
The meter is process-wide, which matches the command line tool and the
standalone runtime: one program per process. Hosts that embed several
interpreters share the budget and should call `limits.Reset()` between
programs.

Denied operations return a `sandbox.DeniedError`, which unwraps to
`os.ErrPermission`, is classified by `stdlib.IsPermissionError`, and is surfaced
as Alca error code `403` with the flag that would grant access.
