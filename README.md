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
prints a note. Both engines are covered by the same output-parity corpus. The
constructs that currently need the interpreter are listed in
[docs/language.md](docs/language.md#engines).

## Language rules

- Every simple statement ends with `;`. Block headers (`if`, `while`, `for`,
  `func`, `struct`, `class`, `test`, `use`) end with `:` and no semicolon.
- Types are strict and fixed after inference. There are no implicit
  conversions: use `string(x)`, `int(x)`, or `float(x)`.
- Conditions must be `bool`; there is no truthiness in `if`/`while`.
- `int / int` is integer division. `null` is only valid for `T | null` and is
  narrowed by `x != null` checks.
- `json` is the only dynamic type. Values must be converted before typed use.
- Functions return only through `return`; a function body does not implicitly
  return its last expression.
- Closures capture surrounding locals by reference, and `:=` declares a new
  binding while `=` updates an existing one. A name that is already bound is
  never shadowed: declaring it again in a nested scope updates the existing
  binding and is only allowed with the same type. `public`, `private`, and
  `const` declare visibility and immutability.
- Classes support fields, methods and single inheritance (`class Dog extends
  Animal:`), with `self`, `super`, dynamic dispatch and enforced `public` /
  `private` / `const` members. See [docs/language.md](docs/language.md#classes-and-inheritance).
- The type checker runs for `run`, `test`, `check`, `build`, and every REPL
  snippet.

See [docs/language.md](docs/language.md) for the full language summary.

  ## Crypto, HTTP, and optional numerical helpers

  - `crypto.aes_encrypt` and `crypto.aes_decrypt` accept only raw AES keys that
    are exactly 16, 24, or 32 bytes. For passwords, use
    `crypto.aes_encrypt_password(password, plaintext)` and
    `crypto.aes_decrypt_password(password, ciphertext)`; new ciphertexts use
    PBKDF2-HMAC-SHA-256 (600,000 iterations), a fresh random salt, a versioned
    format, and AES-GCM. Existing unversioned ciphertexts from the former
    password-based `aes_encrypt` implementation can still be read by
    `aes_decrypt_password`.
  - `crypto.derive_key(password, salt, iterations, key_len)` derives a raw key
    with PBKDF2-HMAC-SHA-256. `key_len` must be 16, 24, or 32 bytes, and
    `iterations` must be between 1 and 1,000,000. Use a unique random salt and
    an appropriately high iteration count for password-derived keys.
  - `http.request(method, url, headers?, body?, options?)` and
    `http_request(method, url, headers?, body?, options?)` limit response bodies to 16 MiB. The optional fifth
    argument is an options map; `{"timeout": seconds}` overrides the 30-second
    default, up to 300 seconds. Every response header is returned as a list of
    strings. The `get`, `post`, `put`, `delete`, and `patch` helpers accept
    `(url, headers?, body?, options?)`. HTTP servers use finite read, write, and
    idle timeouts and reject request bodies over 16 MiB.
  - `std.ai` is an optional standard-library module of pure-Go numerical helpers,
    not an accelerated runtime subsystem. Its matrix functions convert Alca
    lists to Go `[][]float64` values for calculations and convert results back.

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

## Package manager (APM)

`apm` installs libraries so programs can `use` them without any path setup:

```text
go build -o apm ./cmd/apm

apm install -u ./my-library     # from local sources
apm install "json@^1.2.0"       # from a registry (built-in node, $ALCA_REGISTRY or --registry)
apm show                        # installed libraries
apm remove json

apm register ann ann@example.com  # create an account; prints a one-time token
apm login ann                     # new one-time token if it was lost
apm whoami                        # the signed-in account and its tokens
apm logout                        # revoke the saved token on the node
```

Installed libraries live in a local system registry under `$ALCA_HOME`
(default `~/.alca`), including a compiled VM build per package that the VM
loads automatically (sources remain the primary artifact and the fallback).
The `alca.toml` manifest standard, layout, versioning and resolution rules are
documented in [docs/apm.md](docs/apm.md). Account tokens are saved to
`$ALCA_HOME/credentials.json` (mode `0600`) and reused by `apm publish`, so
publishing after a one-time `apm register`/`apm login` needs no `--token`.

A registry node for hosting libraries ships as `cmd/apm-registry`; deployment,
API and security are covered in
[docs/apm-registry.md](docs/apm-registry.md). With `allow_registration` enabled
the node is self-service: users register, receive a one-time token, and publish.
Publishing stays disabled unless the node enables it (accounts or operator
tokens) and the client sets `APM_ENABLE_PUBLISH=1`.

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
alca build examples/gui.al --embed assets
```

Supported targets: `linux/amd64`, `linux/arm64`, `darwin/amd64`,
`darwin/arm64`, `windows/amd64`, `windows/arm64`. The host target always uses
the running `alca` binary, so a stale stub cannot be picked up accidentally.
`ALCA_RUNTIMES=<dir>`, `<alca-dir>/runtimes`, `~/.alca/runtimes`, and
`./runtimes` are searched for non-host runtimes.

### Embedding application resources

Files are not included automatically. Add only the files or directories your
application needs with repeatable `--embed` options; paths are relative to the
directory containing the entry `.al` file. Directory contents keep their
relative paths in the executable:

```text
alca build app.al --embed assets --embed config/default.json
```

Standalone programs can read embedded resources with `use std.embed;` and
`embed.read("assets/logo.png")`, check them with `embed.exists(path)`, or list
them with `embed.list("assets")`. `gui.webview` can also load an embedded icon
by giving its path as `icon: "assets/app.ico"` (the resource must have been
included with `--embed assets`). WebView2 uses `.ico` files for the native
window icon. Explicit embedding prevents unrelated files such as secrets,
databases, and build outputs from being silently packaged.

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
