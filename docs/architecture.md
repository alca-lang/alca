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
- no shadowing and no `any` in declarations.

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

Unsupported today includes decorators, shell functions, `ba`/`threads`/`tx`/
`watch`/`mock`/`wait` blocks, timeout expressions, capture streams used as
values, and some keyword-argument edge cases.

## Engine parity

`cmd/alca/engine_test.go` contains a corpus run on both engines. Every test
requires identical stdout and zero exit status on each engine. New language
features and VM fixes must add a corpus entry (or a dedicated two-engine
regression test when setup such as a sandbox policy is required).

`cmd/alca/examples_test.go` additionally type-checks every shipped `.al` file,
rejects the negative fixtures, and runs deterministic examples on both engines.

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

Denied operations return a `sandbox.DeniedError`, which unwraps to
`os.ErrPermission`, is classified by `stdlib.IsPermissionError`, and is surfaced
as Alca error code `403` with the flag that would grant access.
