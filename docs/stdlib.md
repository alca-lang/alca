# Standard library

Import a module with `use std.<name>;`, then call its members through the
module namespace:

```alca
use std.io;

text := io.read_file("input.txt");
println(text);
```

`docs/language.md` covers the type system. The canonical signature table is
[`pkg/typecheck/builtins_table.go`](../pkg/typecheck/builtins_table.go); this
page mirrors it. Signature notation:

- `(args) -> result` describes the accepted overloads, separated by `;`.
- `?` marks an optional argument, and `...` marks a variadic argument.
- An empty signature is a dynamic member whose arguments are not checked
  and whose result is `json`.

Aliases: `std.regex` is `std.re` and `std.zip` is `std.archive`. `std.csv`,
`std.ini`, and `std.json` are also available through `std.formats`.

## Modules

### `ai`

Small dense-matrix and neural-network primitives; values are plain nested lists.

| Member | Signature |
| --- | --- |
| `ai.add` | `(list[list[float]],list[list[float]])->list[list[float]];(list[list[int]],list[list[int]])->list[list[float]]` |
| `ai.dense_forward` | `(list[list[float]],list[list[float]],list[float],string?)->list[list[float]];(list[list[int]],list[list[int]],list[int],string?)->list[list[float]]` |
| `ai.dot` | `(list[float],list[float])->float;(list[int],list[int])->float` |
| `ai.matmul` | `(list[list[float]],list[list[float]])->list[list[float]];(list[list[int]],list[list[int]])->list[list[float]]` |
| `ai.matrix` | `(int,int,json?)->list[list[float]]` |
| `ai.mse` | `(list[float],list[float])->float;(list[int],list[int])->float` |
| `ai.relu` | `(float)->float;(int)->float;(list[float])->list[float];(list[int])->list[float]` |
| `ai.sigmoid` | `(float)->float;(int)->float;(list[float])->list[float];(list[int])->list[float]` |
| `ai.softmax` | `(list[float])->list[float];(list[int])->list[float]` |
| `ai.tanh` | `(float)->float;(int)->float;(list[float])->list[float];(list[int])->list[float]` |
| `ai.transpose` | `(list[list[float]])->list[list[float]];(list[list[int]])->list[list[float]]` |

### `archive`

ZIP and tar.gz creation, extraction, and listing.

| Member | Signature |
| --- | --- |
| `archive.list` | `(string)->list[map[string, json]]` |
| `archive.tar_create` | `(string,string)->bool` |
| `archive.tar_extract` | `(string,string)->bool` |
| `archive.zip_create` | `(string,string)->bool` |
| `archive.zip_extract` | `(string,string)->bool` |

### `cli`

Command-line arguments, terminal prompts, colors, and progress bars.

| Member | Signature |
| --- | --- |
| `cli.args` | `()->list[string]` |
| `cli.color` | `(string,string)->string` |
| `cli.confirm` | `(string,bool?)->bool` |
| `cli.parse` | `(json)->map[string, json]` |
| `cli.progress_bar` | `(int,int,int?,string?)->string` |
| `cli.prompt` | `(string,string?)->string` |
| `cli.prompt_password` | `(string)->string` |

### `crypto`

Hashes, HMAC, encodings, random values, and UUIDs.

| Member | Signature |
| --- | --- |
| `crypto.base64_decode` | `(string)->string` |
| `crypto.base64_encode` | `(string)->string` |
| `crypto.hex_decode` | `(string)->string` |
| `crypto.hex_encode` | `(string)->string` |
| `crypto.hmac_sha256` | `(string,string)->string` |
| `crypto.md5` | `(string)->string` |
| `crypto.random_int` | `(int,int)->int` |
| `crypto.random_string` | `(int,string?)->string` |
| `crypto.sha1` | `(string)->string` |
| `crypto.sha256` | `(string)->string` |
| `crypto.sha512` | `(string)->string` |
| `crypto.uuid` | `()->string` |

### `csv`

CSV parsing and writing (aliases also appear under `formats`).

| Member | Signature |
| --- | --- |
| `csv.parse` | `(string,bool?)->list[json]` |
| `csv.read` | `(string,bool?)->list[json]` |
| `csv.stringify` | `(json,list[string]?)->string` |
| `csv.write` | `(string,json,list[string]?)->bool` |

### `db`

SQLite and PostgreSQL handles.

| Member | Signature |
| --- | --- |
| `db.open` | `(string,string)->json` |
| `db.postgres` | `(string)->json` |
| `db.sqlite` | `(string?)->json` |

### `env`

Environment variables, OS identity, and PATH management.

| Member | Signature |
| --- | --- |
| `env.add_path` | `(string)->bool` |
| `env.add_system_path` | `(string)->bool` |
| `env.add_user_path` | `(string)->bool` |
| `env.all_env` | `()->map[string, json]` |
| `env.broadcast_env_change` | `()->bool` |
| `env.expand_env` | `(string)->string` |
| `env.get_env` | `(string)->string` |
| `env.get_path` | `()->list[string]` |
| `env.get_system_path` | `()->list[string]` |
| `env.get_user_path` | `()->list[string]` |
| `env.has_path` | `(string)->bool` |
| `env.home_dir` | `()->string` |
| `env.hostname` | `()->string` |
| `env.is_linux` | `()->bool` |
| `env.is_windows` | `()->bool` |
| `env.os_name` | `()->string` |
| `env.path_separator` | `()->string` |
| `env.persist_user_path` | `(string)->bool` |
| `env.prepend_path` | `(string)->bool` |
| `env.remove_path` | `(string)->bool` |
| `env.remove_user_path` | `(string)->bool` |
| `env.set_env` | `(string,string)->bool` |
| `env.set_path` | `(list[string])->bool;(string)->bool` |
| `env.unset_env` | `(string)->bool` |
| `env.username` | `()->string` |
| `env.which` | `(string)->string | null` |
| `env.working_dir` | `()->string` |

### `ffi`

Raw foreign-function calls and pointer helpers; trusted code only.

| Member | Signature |
| --- | --- |
| `ffi.alloc` | `(int?)->blob` |
| `ffi.c_str` | `(string)->int` |
| `ffi.load` | `(string)->json` |
| `ffi.ptr_to_str` | `(int)->string` |
| `ffi.ptr_to_utf16` | `(int)->string` |
| `ffi.size_of` | `(string)->int` |
| `ffi.utf16_str` | `(string)->int` |

### `formats`

Dynamic namespace containing the `csv`, `ini`, and `json` helpers.

| Member | Signature |
| --- | --- |
| `formats.csv` | dynamic (`json`) |
| `formats.ini` | dynamic (`json`) |
| `formats.json` | dynamic (`json`) |

### `gui`

Native dialogs, windows, and WebView helpers (platform-specific); trusted code only.

| Member | Signature |
| --- | --- |
| `gui.alert` | dynamic (`json`) |
| `gui.confirm` | dynamic (`json`) |
| `gui.height` | dynamic (`json`) |
| `gui.msgbox` | dynamic (`json`) |
| `gui.notify` | dynamic (`json`) |
| `gui.open_file` | dynamic (`json`) |
| `gui.prompt` | dynamic (`json`) |
| `gui.save_file` | dynamic (`json`) |
| `gui.screen_size` | dynamic (`json`) |
| `gui.screenshot` | dynamic (`json`) |
| `gui.webview` | dynamic (`json`) |
| `gui.width` | dynamic (`json`) |
| `gui.window` | dynamic (`json`) |

### `ini`

INI parsing and writing.

| Member | Signature |
| --- | --- |
| `ini.parse` | `(string)->json` |
| `ini.read` | `(string)->json` |
| `ini.stringify` | `(json)->string` |
| `ini.write` | `(string,json)->bool` |

### `io`

File and directory operations, streams, locks, and permission checks.

| Member | Signature |
| --- | --- |
| `io.append_file` | `(string,string)->bool` |
| `io.check_access` | `(string,string)->bool` |
| `io.chmod` | `(string,int)->bool;(string,string)->bool` |
| `io.copy_file` | `(string,string)->bool` |
| `io.delete_file` | `(string)->bool` |
| `io.file_exists` | `(string)->bool` |
| `io.file_info` | `(string)->map[string, json]` |
| `io.file_size` | `(string)->int` |
| `io.glob` | `(string)->list[string]` |
| `io.is_permission_error` | `(any)->bool` |
| `io.list_dir` | `(string?)->list[string]` |
| `io.lock_file` | `(any,bool?)->bool` |
| `io.make_dir` | `(string)->bool` |
| `io.move_file` | `(string,string)->bool` |
| `io.open` | `(string,string?)->json` |
| `io.permissions` | `(string)->map[string, json]` |
| `io.read_file` | `(string)->string` |
| `io.read_lines` | `(string)->list[string]` |
| `io.temp_dir` | `(string?)->string` |
| `io.temp_file` | `(string?)->string` |
| `io.unlock_file` | `(any)->bool` |
| `io.write_file` | `(string,string)->bool` |

### `log`

Structured logging and the security audit trail.

| Member | Signature |
| --- | --- |
| `log.access_denied` | `(string,string,string?)->bool` |
| `log.audit` | `(string,string,string,string,json?)->bool` |
| `log.debug` | `(string,json?)->null` |
| `log.error` | `(string,json?)->null` |
| `log.fatal` | `(string,json?)->null` |
| `log.get_access_errors` | `()->list[map[string, json]]` |
| `log.info` | `(string,json?)->null` |
| `log.set_file` | `(string,int?,int?)->bool` |
| `log.set_format` | `(string)->bool` |
| `log.set_level` | `(string)->bool` |
| `log.warn` | `(string,json?)->null` |

### `math`

Constants and trigonometric, rounding, and aggregate helpers.

| Member | Signature |
| --- | --- |
| `math.E` | `float` |
| `math.LN10` | `float` |
| `math.LN2` | `float` |
| `math.PI` | `float` |
| `math.SQRT2` | `float` |
| `math.abs` | `(int)->int;(float)->float` |
| `math.acos` | `(float)->float` |
| `math.asin` | `(float)->float` |
| `math.atan` | `(float)->float` |
| `math.atan2` | `(float,float)->float` |
| `math.ceil` | `(float)->int` |
| `math.clamp` | `(int,int,int)->int;(float,float,float)->float` |
| `math.cos` | `(float)->float` |
| `math.deg2rad` | `(float)->float` |
| `math.floor` | `(float)->int` |
| `math.hypot` | `(float,float)->float` |
| `math.log` | `(float)->float` |
| `math.log10` | `(float)->float` |
| `math.log2` | `(float)->float` |
| `math.max` | `(int,int)->int;(float,float)->float` |
| `math.min` | `(int,int)->int;(float,float)->float` |
| `math.pow` | `(float,float)->float;(int,int)->float` |
| `math.rad2deg` | `(float)->float` |
| `math.round` | `(float)->int` |
| `math.sin` | `(float)->float` |
| `math.sqrt` | `(float)->float;(int)->float` |
| `math.tan` | `(float)->float` |

### `metrics`

Prometheus counters, gauges, histograms, and an HTTP exporter.

| Member | Signature |
| --- | --- |
| `metrics.counter` | `(string,string?)->json` |
| `metrics.export` | `()->string` |
| `metrics.gauge` | `(string,string?)->json` |
| `metrics.histogram` | `(string,string?,json?)->json` |
| `metrics.serve` | `(int?,string?)->string` |

### `net`

HTTP requests, DNS, TCP, ping, and downloads.

| Member | Signature |
| --- | --- |
| `net.dns_lookup` | `(string)->list[string]` |
| `net.download_file` | `(string,string)->bool` |
| `net.http_get` | `(string)->map[string, json]` |
| `net.http_post` | `(string,string,string)->map[string, json]` |
| `net.ping` | `(string)->bool` |

### `path`

Portable path manipulation.

| Member | Signature |
| --- | --- |
| `path.abs` | `(string)->string` |
| `path.base` | `(string)->string` |
| `path.clean` | `(string)->string` |
| `path.dir` | `(string)->string` |
| `path.expand` | `(string)->string` |
| `path.ext` | `(string)->string` |
| `path.from_slash` | `(string)->string` |
| `path.is_abs` | `(string)->bool` |
| `path.join` | `(string,...string)->string` |
| `path.join_list` | `(list[string])->string;(...string)->string` |
| `path.list_sep` | `()->string` |
| `path.matches` | `(string,string)->bool` |
| `path.rel` | `(string,string)->string` |
| `path.sep` | `()->string` |
| `path.split` | `(string)->list[string]` |
| `path.split_list` | `(string)->list[string]` |
| `path.stem` | `(string)->string` |
| `path.to_slash` | `(string)->string` |
| `path.which` | `(string)->string | null` |

### `process`

Process inspection, signaling, and command execution; trusted code only.

| Member | Signature |
| --- | --- |
| `process.cpu_count` | `()->int` |
| `process.exec_bg` | `(string)->int` |
| `process.exec_cmd` | `(string)->map[string, json]` |
| `process.get_pid` | `()->int` |
| `process.get_ppid` | `()->int` |
| `process.kill_process` | `(int)->bool` |
| `process.memory_usage` | `()->map[string, json]` |
| `process.process_exists` | `(int)->bool` |
| `process.signal_process` | `(int,string)->bool` |
| `process.uptime` | `()->int` |

### `re`

Regular expressions (alias: `regex`).

| Member | Signature |
| --- | --- |
| `re.find` | `(string,string)->string | null` |
| `re.find_all` | `(string,string,int?)->list[string]` |
| `re.find_all_groups` | `(string,string,int?)->list[list[string]]` |
| `re.find_groups` | `(string,string)->list[string] | null` |
| `re.match` | `(string,string)->bool` |
| `re.quote` | `(string)->string` |
| `re.replace` | `(string,string,string)->string` |
| `re.split` | `(string,string,int?)->list[string]` |

### `sys`

Operating-system and hardware information; trusted code only.

| Member | Signature |
| --- | --- |
| `sys.arch` | `()->string` |
| `sys.cpu_count` | `()->int` |
| `sys.disk_partitions` | `()->list[map[string, json]]` |
| `sys.disk_usage` | `(string?)->map[string, json]` |
| `sys.load_avg` | `()->map[string, json]` |
| `sys.network_interfaces` | `()->list[map[string, json]]` |
| `sys.platform` | `()->string` |
| `sys.process_list` | `()->list[map[string, json]]` |
| `sys.ram_usage` | `()->map[string, json]` |
| `sys.uptime` | `()->float` |
| `sys.whoami` | `()->map[string, json]` |

### `time`

Clocks, formatting, parsing, and sleeps.

| Member | Signature |
| --- | --- |
| `time.date` | `(int?)->map[string, json];(float?)->map[string, json]` |
| `time.diff` | `(int,int)->float;(float,float)->float` |
| `time.format` | `(int,string?)->string;(float,string?)->string` |
| `time.measure` | `(json)->map[string, json]` |
| `time.now` | `()->int` |
| `time.now_ms` | `()->int` |
| `time.now_ns` | `()->int` |
| `time.now_str` | `(string?)->string` |
| `time.parse` | `(string,string?)->int` |
| `time.since` | `(int)->float;(float)->float` |
| `time.sleep` | `(int)->null;(float)->null;(string)->null` |

## Global builtins

These functions are available without a `use` statement.

| Function | Signature |
| --- | --- |
| `abs` | `(int)->int;(float)->float` |
| `bool` | `(any)->bool` |
| `exec` | `(string)->string` |
| `exit` | `(int?)->void` |
| `float` | `(any)->float` |
| `input` | `(string?)->string` |
| `int` | `(any)->int` |
| `json_decode` | `(string)->json` |
| `json_encode` | `(any)->string` |
| `json_prettify` | `(any)->string` |
| `json_query` | `(json,string)->json` |
| `max` | `(int,int)->int;(float,float)->float` |
| `min` | `(int,int)->int;(float,float)->float` |
| `print` | `(any...)->string` |
| `println` | `(any...)->string` |
| `sleep` | `(int)->null;(float)->null` |
| `str_contains` | `(string,string)->bool` |
| `str_ends_with` | `(string,string)->bool` |
| `str_lower` | `(string)->string` |
| `str_replace` | `(string,string,string)->string` |
| `str_split` | `(string,string)->list[string]` |
| `str_starts_with` | `(string,string)->bool` |
| `str_trim` | `(string)->string` |
| `str_upper` | `(string)->string` |
| `string` | `(any)->string` |
| `time_now` | `()->string` |
| `type` | `(any)->string` |
| `typeof` | `(any)->string` |

Many standard-library functions are also registered as global aliases:

- `env`: `add_path`, `add_system_path`, `add_user_path`, `all_env`, `broadcast_env_change`, `expand_env`, `get_env`, `get_path`, `get_system_path`, `get_user_path`, `has_path`, `home_dir`, `hostname`, `is_linux`, `is_windows`, `os_name`, `path_separator`, `persist_user_path`, `prepend_path`, `remove_path`, `remove_user_path`, `set_env`, `set_path`, `unset_env`, `username`, `which`, `working_dir`
- `io`: `append_file`, `check_access`, `chmod`, `copy_file`, `delete_file`, `file_exists`, `file_info`, `file_size`, `glob`, `is_permission_error`, `list_dir`, `lock_file`, `make_dir`, `move_file`, `open`, `permissions`, `read_file`, `read_lines`, `temp_dir`, `temp_file`, `unlock_file`, `write_file`
- `net`: `dns_lookup`, `download_file`, `http_get`, `http_post`, `ping`
- `process`: `cpu_count`, `exec_bg`, `exec_cmd`, `get_pid`, `get_ppid`, `kill_process`, `memory_usage`, `process_exists`, `signal_process`, `uptime`

## Capability policy

The CLI and standalone applications install a deny-all policy before running
a program. The following operations are checked at call time on both engines:

- `io`, `csv`, `ini`, `archive`, `log.set_file`, and format helpers require
  `--allow-read` or `--allow-write` for the relevant path. This includes
  `io.open`, streams, globbing, directory operations, and archive extraction.
- `net` requests, DNS lookups, TCP connections/listeners, downloads, and
  `metrics.serve` require `--allow-net`. HTTP redirects are checked against
  the policy before being followed.
- `db.sqlite`, `db.open("sqlite", ...)`, and `db.open("postgres", ...)` enforce
  the same file and network rules. A writable SQLite database requires both
  read and write grants; a `mode=ro` or `immutable=1` DSN needs only read.
  PostgreSQL URL and keyword DSNs are checked against `--allow-net` for their
  host and port (defaults: `localhost:5432`). SQL statements themselves are not
  parsed: a granted SQLite connection can use SQL features such as `ATTACH`, so
  only grant database access to trusted programs.

`process`, the global `exec` function, `env`, `ffi`, `gui`, and `sys` expose
host resources (command execution, environment mutation, native calls, UI, and
machine information).
They are intentionally outside the read/write/network capability model: only
run trusted programs with access to those modules, or omit them from a
restricted build.

Denied operations return Alca error code `403`, unwrap to
`os.ErrPermission` in Go, and are handled by `io.is_permission_error` in Alca.
