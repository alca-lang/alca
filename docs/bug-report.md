# Bug and regression report

This report lists every defect found and fixed while bringing Alca through
phases P0–P6: legacy-syntax migration, tooling, VM engine parity, strict
signatures, cross-compilation, and runtime security. Every entry names the
regression test that locks the fix in.

Method: Alca has two engines (bytecode VM and tree-walking interpreter) that
must agree exactly. Any divergence, silent miscompile, unsound typing, or
missing capability check is treated as a bug. Fixes are always made in the
engine, never by weakening the checker or deleting the test.

Running totals:

- Engine-parity corpus: **91 programs**, each executed on both engines with
  byte-exact stdout comparison (`cmd/alca/engine_test.go:TestEnginesAgree`).
- All fixes below are covered by a deterministic regression test.

## Historical numbering

The P0 fixes were tracked as bugs #1–#12. Two later fixes kept the running
number: bug #20 (stale host runtime) and bug #21 (capability hint dropped).
The remaining items use phase-local identifiers.

## P0 — Engine parity and runtime correctness (bugs #1–#12)

| ID | Symptom | Root cause and fix | Regression test |
| --- | --- | --- | --- |
| P0-1 | `"abc" < "abd"` produced the wrong result on the VM | The VM's ordering comparison did not treat string operands like the interpreter; string ordering is now shared | `engine_test.go` `p26_string_compare` |
| P0-2 | `a + b` on two lists fell through to float arithmetic and errored | The VM compiler had no list+list concatenation case; it now dispatches to the shared concatenation helper | `engine_test.go` `p42_list_concat` |
| P0-3 | Division, modulo, and integer `**` runtime faults reported code 500 instead of 400 | The VM raised generic 500 errors; arithmetic faults now carry the interpreter's code 400 | `engine_test.go` `p18_div_zero_capture`, `q23_pow_negative_int` |
| P0-4 | The `in` membership operator was typed by the checker but implemented by neither engine | Both engines now call one shared `interpreter.InOp` implementation | `engine_test.go` `p35_membership` |
| P0-5 | `for i in 3` iterated a single element on the interpreter | Integer-range iteration only handled explicit ranges; `EvalRange` semantics now apply to single ints too | `engine_test.go` `q14_range_single_int` |
| P0-6 | `for i in 5..1` iterated zero times on the VM | The VM used a forward counter loop; descending ranges now go through `EvalRange` | `engine_test.go` `q12_range_descending`, `q13_range_negative` |
| P0-7 | VM index reads ignored negative indices and silently misbehaved on out-of-bounds or non-container values | Index reads now implement negative indexing and raise code 400 with the interpreter's messages | `engine_test.go` `q01_list_oob`, `q02_list_negative_index`, `q03_str_negative_index`, `q04_str_oob`, `q21_index_json_dynamic` |
| P0-8 | An out-of-bounds index write was silently ignored on the VM | Assignment now validates container and bounds and raises code 400 | `engine_test.go` `q05_set_oob` |
| P0-9 | A standard-library member name shadowed a module name on the interpreter (e.g. `log` used as a value) | Module and module-member names are now flattened without clobbering the module binding | `engine_test.go` "module member does not shadow a module name" |
| P0-10 | Metrics histogram buckets were cumulated twice | `metrics.histogram` cumulated already-cumulative buckets; it now reports each bucket once | `pkg/stdlib/stdlib_test.go:TestMetricsHistogramBucketsAreCumulative` |
| P0-11 | `json_query` returned `null` for an already-decoded map/list value | The query path round-tripped decoded values through `Inspect()` (non-JSON); decoded values are now handled directly | `pkg/stdlib/stdlib_test.go:TestFormatsModule` (JSON Query part 4) |
| P0-12 | VM runtime errors lost the Alca error code in CLI diagnostics (printed code 400 for `err(..., code=7)`) | `RaisedError` values were not unwrapped by the diagnostic formatter; the formatter now keeps the raised code | `pkg/vm/vm_test.go:TestRaisedErrorReportsItsOwnCodeInDiagnostics`, `TestErrExpressionKeepsExplicitCode` |

## P1 — Parser and tooling

| ID | Symptom | Root cause and fix | Regression test |
| --- | --- | --- | --- |
| P1-1 | A block whose body was only a comment swallowed the next dedented statement, so `else`/`elif`/`case` bound to the wrong statement | The parser required an INDENT token; comment-only bodies now parse as empty blocks | `pkg/parser/parser_test.go:TestCommentOnlyBlockBodies`, corpus `q24_comment_only_block_bodies` |
| P1-2 | An unindented statement after a block header was silently attached to the body (two-space indentation was accepted as a body) | Block bodies must start on the header line or an indented line; anything else is `expected an indented block` | `pkg/parser/parser_test.go:TestUnindentedStatementIsNotABlockBody` |
| P1-3 | LSP hover never worked on a module member nested inside a call | The AST walker pruned subtrees whose token position was after the cursor, but a call's token is `(` and a member access's token is `.`; `nodeAt` now checks the whole subtree span | `pkg/lsp/lsp_test.go:TestLSPHoverStdModuleMemberShowsSignature` |

The REPL and formatter were built in P1 as well; their invariants are covered by
`pkg/typecheck/snippet_test.go` (`TestCheckSnippetKeepsDeclarations`,
`TestCheckSnippetRestoreRollsBackFailedSnippet`), `cmd/alca/repl_test.go`, and
`cmd/alca/fmt_test.go`.

## P2 — VM closures and loop semantics

| ID | Symptom | Root cause and fix | Regression test |
| --- | --- | --- | --- |
| P2-1 | Closures mutated stale copies; two independent closures shared one state slot (printed `1,2,3,4`) | The VM captured upvalues by value at closure creation. Closures now capture by reference through serializable upvalue descriptors | `pkg/vm/vm_test.go:TestClosuresCaptureByReferenceOnBothEngines`, corpus `q25`–`q32` |
| P2-2 | The counter-loop fast path left the loop variable at `end+1`, and writing the loop variable inside the body changed the VM's iteration count | Loop variables now follow interpreter semantics: fixed item list, final value is the last iterated item | `pkg/vm/vm_test.go:TestCounterLoopFinalValueMatchesInterpreter`, corpus `q29_loop_variable_capture` |
| P2-3 | No function could ever return a closure: `-> func` rejected function values with `invalid return type: expected func, got FUNCTION` | The runtime return-type check did not recognize function values for the `func` annotation | `pkg/interpreter/interpreter_test.go:TestReturningFunctionValueWithFuncAnnotation`, corpus `q28_lambda_captures_parameter` |
| P2-4 | A `-> void` function ending in an expression statement (e.g. `println(msg);`) failed with `expected void, got STRING` | Falling off the end of a body yields null; the validator treated the last expression as an implicit return | `pkg/interpreter/interpreter_test.go:TestVoidFunctionEndingInExpressionStatement`, corpus `q30_void_function_ends_with_statement` |
| P2-5 | `--vm` rejection of an unsupported feature printed the native-build troubleshooting template ("compilation to native executable failed", disk space, output path) | `FormatUnsupportedFeature` reused the build-error template; it now names the feature and suggests `--interp` | `pkg/diag/diagnostic_test.go:TestFormatUnsupportedFeatureDoesNotBlameBuild` |

## P3 — Type-checker soundness and stdlib signatures

| ID | Symptom | Root cause and fix | Regression test |
| --- | --- | --- | --- |
| P3-1 | `push`/`append` were typed as `void`, so their result (the list) could not be used; `append` was not in the special-call table | Both now return the list type and check element fitting; an empty list adopts the appended element type | `pkg/typecheck/typecheck_test.go:TestValidPrograms` ("collections", "append"), `TestRejectedPrograms` ("append element type") |
| P3-2 | `pop(xs)` was typed as the non-null element type although runtime returns `null` on an empty list | `pop` now returns `T \| null`; unchecked use is rejected | `pkg/typecheck/typecheck_test.go:TestRejectedPrograms` ("pop is nullable") |
| P3-3 | Several stdlib signatures were too loose or wrong (missing arities, non-nullable results such as `re.find`, `path.which`, `env.which`, wrong argument types) | `pkg/typecheck/builtins_table.go` was rewritten against probed runtime behavior under the strict-signature policy; only genuinely dynamic builtins keep `func -> json` | `pkg/typecheck/typecheck_test.go:TestStdlibSignaturesRejectMisuse`, `TestStdlibOptionalArguments` |

## P4 — Cross-compilation

| ID | Symptom | Root cause and fix | Regression test |
| --- | --- | --- | --- |
| P4-1 (bug #20) | A host build could embed a stale `runtimes/` stub that predated payload-format or security-policy changes | `FindRuntime` searched runtime directories before falling back to the running executable; the host target now always prefers the running `alca` binary | `pkg/compiler/compiler_test.go:TestHostBuildUsesTheRunningExecutable` |

Cross-build acceptance was added in the same phase:
`cmd/alca/build_test.go:TestCrossBuildAllTargets` builds all six targets through
`cmdBuild`, verifies each executable header (ELF/PE/Mach-O), decodes every
embedded payload, executes it on the host VM, compares stdout with `alca run`,
and executes the host artifact.

## P5 — Runtime security

| ID | Symptom | Root cause and fix | Regression test |
| --- | --- | --- | --- |
| P5-1 (bug #21) | Capability denials lost the flag hint: the diagnostic collapsed `permission denied ... (pass --allow-read...)` into a generic OS permission message | `diag.CleanErrorMessage` translates OS permission errors; capability denials are now preserved verbatim and get a flag-specific suggestion | `pkg/diag/capability_test.go:TestCleanErrorMessageKeepsCapabilityHint` |
| P5-2 | `go test -race` crashed in the FFI pointer path with `checkptr: pointer arithmetic result points to invalid allocation` | Converting a foreign `uintptr` to `unsafe.Pointer` is not checkptr-valid; the conversion now lives in a single `//go:nocheckptr` helper (`foreignPointer`) | `pkg/stdlib/stdlib_test.go:TestFFIModule` under `go test -race ./...` |
| P5-3 | `metrics.serve` started an HTTP listener with no network grant (and returned before the port was bound) | The server now requires `--allow-net`, binds synchronously, and reports the actual ephemeral port | deny case in `cmd/alca/security_test.go:TestRunDeniesFileAndNetworkAccessWithoutFlags` ("metrics server") |
| P5-4 | SQLite databases were opened without any file grant; a read grant alone allowed a writable database | `db.sqlite`/`db.open("sqlite", ...)` now parse the DSN and require read plus write unless `mode=ro` or `immutable=1`; `file::memory:` stays exempt | `pkg/stdlib/db_security_test.go:TestSQLiteDSNPath`, `TestSQLiteDSNRequiresReadAndWrite`, deny case ("sqlite file") |
| P5-5 | PostgreSQL DSNs were opened without a network grant (including `PGHOST`/`PGPORT` defaults) | URL and keyword DSNs are parsed and checked against `--allow-net` for host and port; Unix-socket paths require write access | `pkg/stdlib/db_security_test.go:TestPostgresDSNRequiresNetworkGrant` |
| P5-6 | `log.set_file` created and wrote log files without a write grant | The path is now checked before the file is opened; the builtin returns a code-403 error | deny case ("log file") in `TestRunDeniesFileAndNetworkAccessWithoutFlags` |
| P5-7 | HTTP builtins and stream writes followed redirects to hosts outside the granted policy | Both HTTP clients re-check every redirect against the policy before following it | `pkg/stdlib/net_security_test.go:TestHTTPRedirectChecksCapabilityPolicy`, `pkg/interpreter/stream_security_test.go:TestStreamRedirectChecksCapabilityPolicy` |
| P5-8 | Built applications inherited the library's permissive default instead of deny-all | CLI (`run`, `test`, `repl`) and the standalone runtime now install an explicit deny-all policy and parse leading `--allow-*` flags; the library default stays permissive for embedding | `cmd/alca/security_test.go:TestRunDeniesFileAndNetworkAccessWithoutFlags`, `TestRunAllowFlagsPermitAccessOnBothEngines`, `TestRunAllowAllPermitsReadAndWrite`, `TestStandaloneApplicationEnforcesCapabilities`, `pkg/sandbox/sandbox_test.go` |

Remaining trusted host access (global `exec`, `process`, `env`, `ffi`, `gui`,
`sys`, and unparsed SQL statements on a granted connection) is intentional and
documented in `docs/stdlib.md` and `docs/architecture.md`; it is not a
capability bug.

## Acceptance regressions (no bug found)

| Requirement | Evidence |
| --- | --- |
| >60-program engine corpus, identical output | 91 entries in `cmd/alca/engine_test.go`, all passing on both engines |
| 5M iterations without crash or memory growth | `pkg/vm/vm_test.go:TestFiveMillionIterationLoopDoesNotGrowMemory` |
| All six targets build with matching output | `cmd/alca/build_test.go:TestCrossBuildAllTargets`; host artifact equals `alca run` |
| File/network denied without flags, allowed with flags | `cmd/alca/security_test.go`, `pkg/sandbox/sandbox_test.go` |
| FFI works under the race detector | `pkg/stdlib/stdlib_test.go:TestFFIModule` in `go test -race ./...` |

## Final verification

```text
gofmt -l .                -> no output
go build ./...            -> ok
go vet ./...              -> no output
go test ./...             -> all packages ok
go test -race -count=1 ./... (CGO_ENABLED=1, CC=gcc) -> all packages ok
```
