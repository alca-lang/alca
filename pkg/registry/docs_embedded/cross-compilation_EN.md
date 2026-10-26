# Cross-compilation

Alca builds standalone executables without requiring a target toolchain at
build time. The build machine only needs the prebuilt Alca runtime stubs.

## How the output is assembled

```text
[target runtime stub] + [Alca payload] + [uint64 payload length] + [ALCA_EXEC_V2 magic]
```

The runtime stub is `cmd/alca` compiled for the target OS/architecture. It
contains the VM, interpreter, and standard library. The payload is:

- VM bytecode by default (source text is not shipped); or
- source plus imported `.al` modules when requested explicitly via `--source`,
  when the program imports local files, or when it uses a construct the VM cannot compile.

Additionally, external resources (files or directories) can be packed directly into the standalone binary using one or more `--embed <path>` flags. At runtime, embedded assets are accessible via `use std.embed; embed.read("path")` or `embed.list()`.

The payload envelope is versioned and checksummed with SHA-256. The runtime
verifies the checksum before decoding. A corrupted or truncated payload is
rejected with a diagnostic instead of executing partially.

## Building runtime stubs

From the repository root, with the Go toolchain installed:

```text
go run ./cmd/alca build-runtimes --out runtimes
```

This builds:

| Target | Runtime file |
| --- | --- |
| `linux/amd64` | `runtimes/alca-linux-amd64` |
| `linux/arm64` | `runtimes/alca-linux-arm64` |
| `darwin/amd64` | `runtimes/alca-darwin-amd64` |
| `darwin/arm64` | `runtimes/alca-darwin-arm64` |
| `windows/amd64` | `runtimes/alca-windows-amd64.exe` |
| `windows/arm64` | `runtimes/alca-windows-arm64.exe` |

The stubs are generated artifacts and are excluded by `.gitignore`. Rebuild
them after changing payload format, standard library, or runtime security
policy.

Useful options:

```text
--out <dir>          output directory (default: runtimes)
--targets a,b        subset, e.g. linux/amd64,windows/arm64
--tags nodb          build runtimes without database drivers
```

## Building an application

```text
alca build app.al --target linux/amd64 -o app
alca build app.al --target windows/arm64 -o app
```

If `runtimes/` is present, discovery finds it automatically. Explicit lookup
order for non-host targets:

1. `--runtime <file>`
2. `$ALCA_RUNTIMES`
3. `<directory of alca>/runtimes`
4. `<directory of alca>`
5. `~/.alca/runtimes`
6. `./runtimes`

The host target always uses the currently running `alca` executable, even if a
different stub exists in a runtime directory. This guarantees that host builds
match the CLI's payload format and security policy.

`alca targets` shows which target runtimes are available.

## Verification

The integration test `cmd/alca/build_test.go`:

- builds a deterministic program for all six targets through `cmdBuild`;
- checks the target executable header (ELF, PE, or Mach-O);
- extracts and decodes each embedded payload and verifies the program string;
- runs every decoded bytecode payload on the host VM and compares its stdout
  with `alca run` on both engines;
- executes the host artifact and compares its stdout with both engines.

A separate test builds a host application and verifies that it denies reads and
network access without capability flags and accepts `--allow-read` /
`--allow-net`. On a machine without Linux/macOS execution support, non-host
artifacts are verified structurally and by running their platform-independent
VM payload; direct execution still requires a matching OS, container, or
emulator.

## Security of built applications

Standalone applications enforce the same capability policy as `alca run`:
file and network access are denied unless flags are passed to the application:

```text
app --allow-read=./data --allow-net=api.example.com
```

Flags must come before script arguments. `--` ends flag parsing if a script
argument begins with a dash.
