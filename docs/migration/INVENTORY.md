# Migration inventory: Go → C++

Status: Stage 0/1 snapshot. Source of truth is the code, not these notes; update
this file whenever a package moves.

## 1. Repository scale (Go, excluding tests)

| Package | Files | LOC (incl. tests) | Role | C++ target | Port order | Status |
|---|---:|---:|---|---|---|---|
| `pkg/token` | 2 | 521 | token kinds, keywords, positions | `compiler/src/token` | 1 | **ported + differential** |
| `pkg/lexer` | 2 | 1,406 | byte lexer, NEWLINE/INDENT/DEDENT, literals | `compiler/src/lexer` | 1 | **ported + differential** |
| `pkg/ast` | 2 | 2,073 | AST nodes | `compiler/src/ast` | 1b | **ported + tested** |
| `pkg/parser` | 6 | 3,678 | recursive-descent parser | `compiler/src/parser` | 1b | **ported + tested** |
| `pkg/diag` | 3 | 576 | diagnostics, capability errors | `compiler/src/diag` | 2 | **ported + tested** |
| `pkg/typecheck` | 11 | 5,715 | types, modules, generics | `compiler/src/typecheck` | 2 | **ported + tested** |
| `pkg/mod` | 2 | 344 | local/APM module graph | `compiler/src/mod` | 2 | **ported + tested** |
| `pkg/dunder` | 1 | 38 | `__` builtin names | `compiler/src/dunder` | 2 | **ported + tested** |
| `pkg/ir` | 3 | 1,706 | typed IR + lowering | `compiler/src/ir` | 3 | pending |
| `pkg/codegen/amd64` | 1 | 500 | narrow AOT encoder | `compiler/src/codegen/x64` | 4 | pending |
| `pkg/link` | 4 | 1,065 | PE/ELF writers | `compiler/src/link/{pe,elf}` | 4 | pending |
| `pkg/compiler` | 7 | 975 | AOT build orchestration | `compiler/src/driver` | 4 | pending |
| `pkg/vm` | 38 | 9,408 | bytecode VM (to delete) | — | n/a | **do not port** |
| `pkg/interpreter` | 13 | 6,291 | tree interpreter (to delete) | — | n/a | **do not port** |
| `pkg/jit` | 27 | 8,478 | JIT (to delete) | — | n/a | **do not port** |
| `pkg/format` | 2 | 598 | formatter | `compiler/src/format` | 8 | pending |
| `pkg/lsp` | 4 | 1,221 | LSP | `compiler/src/lsp` | 8 | pending |
| `pkg/sandbox` | 2 | 574 | capability model | `compiler/src/runtime/sandbox` | 8 | pending |
| `pkg/limits` | 2 | 377 | resource limits | `compiler/src/runtime/limits` | 8 | pending |
| `pkg/stdlib` | 107 | 28,398 | ~40 modules | `compiler/src/stdlib` | 7 | pending |
| `pkg/libtorch` | 31 | 5,592 | tensor/AI backend | `compiler/src/stdlib/torch` | 7d | pending |
| `pkg/mtproto` | 7 | 1,452 | Telegram MTProto | `compiler/src/stdlib/mtproto` | 7c | pending |
| `cmd/pawl` | 18 | 3,408 | CLI | `compiler/src/cli` | 5 | partial (`lex`, `version`) |
| `pkg/version` | 1 | 7 | version string | `compiler/src/version` | 5 | pending |

Totals from the working tree: ~65–70k Go lines, 192 Go files. The VM,
interpreter and JIT are **not** ported: they are replaced by native code
generation and never leave a migration oracle contract.

## 2. Dependency graph (Go packages, non-test imports)

```text
token        <- lexer, ast, parser, typecheck, format, lsp, vm
lexer        <- parser, typecheck, compiler, format, lsp
ast          <- parser, typecheck, ir, compiler, format, lsp
diag         <- parser, typecheck, lsp, interpreter
mod          <- typecheck, interpreter
typecheck    <- ir, compiler, jit, lsp
ir           <- compiler
codegen/x64  <- compiler, link
link         <- compiler (uses jit memory helpers today)
compiler     <- cmd/pawl
stdlib       <- interpreter, libtorch, mtproto, sandbox
```

Important frontend property: `token → lexer → ast → parser → typecheck → ir`
has no dependency on the VM, interpreter or JIT. Those can be deleted once the
native pipeline passes the corpus.

## 3. External dependencies (go.mod) and their native replacements

| Go dependency | Used by | Native plan |
|---|---|---|
| `modernc.org/sqlite` | stdlib.db | vendor SQLite amalgamation, build at `pawl` build time |
| `lib/pq` | stdlib.db | implement PostgreSQL wire protocol |
| `golang.org/x/image` | stdlib.image | own BMP/PNG/JPEG/WebP or permissive vendored decoders |
| `golang.org/x/text` | encoding | own UTF-8/16/32 + charset tables |
| `golang.org/x/sys` | Windows syscalls | direct `kernel32`/`ntdll` declarations |
| `go-mp3` | stdlib.audio | vendored permissive MP3 decoder (minimp3) |
| `go-webview2` | stdlib.gui | direct WebView2 COM calls |
| `gorgonia.org/tensor` | libtorch | own tensor core + optional CUDA driver API |
| `github.com/...` indirects | — | eliminated with the Go tree |

## 4. Blockers and risks

1. **ABI/value representation** — fixed by ADR-0001 before codegen.
2. **GC/allocator** — fixed by ADR-0002; must not call libc malloc in output.
3. **Runtime embedding** — fixed by ADR-0004 (own `.pob` relocatable images).
4. **Stdlib breadth** — largest risk; module-by-module with a parity matrix.
5. **Sandbox** — must live in native runtime, not only in the typechecker.
6. **ARM64** — a real second backend, not x86 emulation.
7. **Go oracle lifetime** — keep only until each package's differential tests
   pass, then delete the package and its oracle coverage.

## 5. What can be ported immediately

- `token`, `lexer` (done).
- `ast` node definitions and `parser` (pure data + recursive descent).
- `diag` formatting.
- `mod` resolution logic (filesystem only).
- `typecheck` core once AST is stable.

Blocked on ADRs: `ir`, both codegen backends, linker, runtime, stdlib
bindings that need the value ABI.

## 6. Migration-only Go artifacts

These are intentionally kept and must be deleted in Stage 9:

- `pkg/**`, `cmd/pawl`, `go.mod`, `go.sum`, Go tests and Go CI.
- `tools/oracle/lexdump` — produces golden token dumps for differential tests.
- Frozen Go oracle is **read-only**: no new language features.

Everything under `compiler/` and `tools/oracle/` is excluded from the final
shipping compiler except `compiler/`.
