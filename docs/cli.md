# Command-line interface

Build the CLI with `go build -o alca ./cmd/alca`.

## `alca run`

```text
alca run [--vm|--interp] [capability flags] <file.al> [script args...]
```

- Default engine: bytecode VM. If the VM reports an unsupported construct,
  `run` falls back to the interpreter and prints a note.
- `--vm`: require the VM. Unsupported constructs are reported as
  `UnsupportedFeature` and exit status 1.
- `--interp`: always use the tree-walking interpreter.
- Everything after `<file.al>` is passed to the program unchanged.

The input is parsed and strictly type-checked before any execution. Syntax or
type errors prevent the program from running.

## `alca check`

```text
alca check <file.al>
```

Parses and type-checks a file without executing it. Prints a success line when
there are no diagnostics. Exit status is non-zero for any syntax or type error.

## `alca test`

```text
alca test [capability flags] <file.al>
```

Runs the whole file, including top-level code, then reports the counts of
`test` blocks that passed and failed. Assertions and unhandled runtime errors
produce a non-zero exit status. Test code obeys the same capability policy as
`run`; pass `--allow-read`, etc. when tests need file or network access.

## `alca fmt`

```text
alca fmt [-w|--write] <file.al>
```

Formats using four-space indentation and canonical statement layout. Without
`-w`, writes the formatted program to stdout. With `-w`, rewrites the file and
preserves its permission bits. The formatter re-parses its own output before
returning it, so it cannot silently corrupt a valid program.

## `alca repl`

```text
alca repl [capability flags]
```

An interactive session with a persistent interpreter environment and checker.
Each snippet is type-checked and rejected/rolled back on errors. Blocks end
with an empty line. Use `exit` or `quit` to leave.

File and network access are disabled unless flags are supplied. Expressions
entered alone are evaluated and printed; all other snippets execute like a
script.

## `alca lsp`

```text
alca lsp
```

Starts a Language Server Protocol server over stdin/stdout. It provides
diagnostics, hover, and context-aware completion using the real parser, type
checker, standard-library tables, and AST.

## `alca build`

```text
alca build <file.al> [-o output] [--target os/arch] [--runtime file] [--source]
```

Builds a standalone executable. By default the payload is VM bytecode, so
source text is not embedded. Programs that import other `.al` files, or that
the VM cannot compile, are embedded as source with their module files so they
can run through the interpreter. `--source` forces this mode.

`-o` receives the platform suffix automatically (`.exe` on Windows).
`--runtime` overrides runtime discovery for one build.

## `alca targets`

Lists all supported targets and whether a runtime stub is installed.

## `alca build-runtimes`

```text
alca build-runtimes [--out runtimes] [--targets linux/amd64,windows/arm64] [--tags nodb]
```

Cross-builds `cmd/alca` for each target with the local Go toolchain. This is a
maintainer/packaging step. End users of a built application need no Go
toolchain.

## Capability flags

Execution commands (`run`, `test`, `repl`, standalone applications) use the
following flags:

| Flag | Meaning |
| --- | --- |
| `--allow-read` | Read everything. |
| `--allow-read=<path>[,<path>...]` | Read the given files/subtrees. |
| `--allow-write` | Write everything. |
| `--allow-write=<path>[,<path>...]` | Write the given files/subtrees. |
| `--allow-net` | Network access everywhere. |
| `--allow-net=<host>` | Connect/listen for the host (any port). |
| `--allow-net=<host:port>` | Only the given host and port. |
| `--allow-net=*.example.com` | Subdomains of `example.com`. |
| `--allow-all` | Grant read, write, and network. |
| `--` | Stop parsing flags (useful for script arguments). |

Paths are normalized to absolute paths and resolved through existing symlinks
before checking, so `..` and symlinks cannot escape a granted subtree.
Denied operations return Alca error code `403`, mention the required flag, and
are recorded by the security audit logger.

## Exit status

- `0`: success.
- `1`: syntax/type/runtime error, failed test, invalid command-line usage, or
  missing runtime.
