# Alca

Alca is a statically typed, scripting-syntax language with a bytecode VM, a
tree-walking interpreter, a formatter, an LSP server, and standalone
cross-compilation. Programs use the `.al` extension.

```alca
use std.math;

struct Server:
    ip: string;
    port: int = 80;

func area(r: float) -> float:
    return math.PI * r * r;

srv := Server("127.0.0.1", 8080);
println("listening on " + srv.ip + ":" + string(srv.port));
println(area(2.0));
```

## Quick start

```text
go build -o alca ./cmd/alca
./alca run examples/hello.al
```

The VM is the default engine. Constructs the VM cannot compile are reported
explicitly with `--vm`; without it, `alca run` falls back to the interpreter and
prints a note. Both engines are covered by the same output-parity corpus.

## Language rules

- Every simple statement ends with `;`. Block headers (`if`, `while`, `for`,
  `func`, `struct`, `test`, `use`) end with `:` and no semicolon.
- Types are strict and fixed after inference. There are no implicit
  conversions: use `string(x)`, `int(x)`, or `float(x)`.
- Conditions must be `bool`; there is no truthiness in `if`/`while`.
- `int / int` is integer division. `null` is only valid for `T | null` and is
  narrowed by `x != null` checks.
- `json` is the only dynamic type. Values must be converted before typed use.
- Functions return only through `return`; a function body does not implicitly
  return its last expression.
- Closures capture surrounding locals by reference, and `:=` declares a new
  binding while `=` updates an existing one.
- The type checker runs for `run`, `test`, `check`, `build`, and every REPL
  snippet.

See [docs/language.md](docs/language.md) for the full language summary.

## Commands

```text
alca run <file.al> [args...]       # type-check, then run (VM by default)
alca run --vm ...                  # require the VM; reject unsupported constructs
alca run --interp ...              # use the tree-walking interpreter
alca check <file.al>               # syntax + strict type check (CI friendly)
alca test <file.al>                # run test blocks and summarize PASS/FAIL
alca fmt [-w] <file.al>            # format to stdout, or rewrite in place
alca repl                          # interactive session with persistent state
alca lsp                           # Language Server Protocol over stdio
alca build <file.al> [-o out]      # standalone executable for the host
alca build ... --target os/arch    # cross-compile (see below)
alca targets                       # show targets and installed runtimes
alca build-runtimes [--out dir]    # build runtime stubs (needs Go, once)
```

## Runtime security

`run`, `test`, `repl`, and standalone executables deny file-system and network
access by default. Grant only what a program needs:

```text
alca run --allow-read=./data app.al
alca run --allow-read=./data --allow-write=./out app.al
alca run --allow-net=api.example.com app.al
alca run --allow-all app.al
```

- `--allow-read[=<path>]` and `--allow-write[=<path>]` accept a path or a
  comma-separated list of paths; a path grants its whole subtree.
- `--allow-net[=<host[:port]>]` accepts a host, a `host:port` pair, a
  `*.example.com` wildcard, or a comma-separated list.
- Omitting `=<value>` grants the whole capability (`--allow-read` is broader
  than `--allow-read=./data`).
- Denials are reported as Alca error code `403` with the flag that would grant
  access. Use `io.is_permission_error(err)` to handle them in a program.

The policy covers standard file and network APIs (including SQLite/PostgreSQL
access). The global `exec` builtin plus `process`, `env`, `ffi`, `gui`, and
`sys` provide trusted host access and are not sandboxed; do not run untrusted
programs with those facilities.

Standalone applications accept the same flags before script arguments:

```text
reader.exe --allow-read=./data input.txt
```

## Cross-compilation

A built executable is `runtime stub for the target + payload + footer`. The
payload is serialized VM bytecode by default; source is embedded only when the
program imports other `.al` files or the VM cannot compile it. Cross-target
builds need the runtime stubs built once:

```text
go run ./cmd/alca build-runtimes --out runtimes
alca targets
alca build examples/hello.al --target linux/amd64 -o hello
alca build examples/hello.al --target windows/arm64 -o hello
```

Supported targets: `linux/amd64`, `linux/arm64`, `darwin/amd64`,
`darwin/arm64`, `windows/amd64`, `windows/arm64`. The host target always uses
the running `alca` binary, so a stale stub cannot be picked up accidentally.
`ALCA_RUNTIMES=<dir>`, `<alca-dir>/runtimes`, `~/.alca/runtimes`, and
`./runtimes` are searched for non-host runtimes.

## Development

```text
gofmt -l .
go vet ./...
go test ./...
go test -race ./...
```

The `nodb` build omits SQLite/Postgres drivers:

```text
go build -tags nodb -o alca ./cmd/alca
```

Tests under `cmd/alca` include an engine-parity corpus that runs every program
on both the VM and the interpreter and compares stdout exactly. Optional
cross-compilation tests run when `runtimes/` is populated.

More documentation:

- [docs/language.md](docs/language.md)
- [docs/cli.md](docs/cli.md)
- [docs/architecture.md](docs/architecture.md)
- [docs/cross-compilation.md](docs/cross-compilation.md)
- [docs/stdlib.md](docs/stdlib.md)
- [docs/bug-report.md](docs/bug-report.md)
