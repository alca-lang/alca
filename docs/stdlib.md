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

Aliases: `std.regex` is `std.re`, `std.str` is `std.string`, and `std.zip` is
`std.archive`. The `csv`, `ini`, and `json` modules are also available through
`std.formats`.

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

### `compress`

Gzip and zlib helpers. Compressed output is base64 encoded. Decompression is
capped at 16 MiB by default; pass an explicit non-negative byte limit as the
second argument (maximum 1 GiB) when a different bound is needed.

| Member | Signature |
| --- | --- |
| `compress.gzip_compress` | `(string)->string` |
| `compress.gzip_decompress` | `(string,int?)->string` |
| `compress.zlib_compress` | `(string)->string` |
| `compress.zlib_decompress` | `(string,int?)->string` |

### `crypto`

Hashes, HMAC, encodings, random values, and UUIDs.

| Member | Signature |
| --- | --- |
| `crypto.aes_decrypt` | `(string,string)->string` |
| `crypto.aes_decrypt_password` | `(string,string)->string` |
| `crypto.aes_encrypt` | `(string,string)->string` |
| `crypto.aes_encrypt_password` | `(string,string)->string` |
| `crypto.base64_decode` | `(string)->string` |
| `crypto.base64_encode` | `(string)->string` |
| `crypto.constant_time_equal` | `(string,string)->bool` |
| `crypto.derive_key` | `(string,string,int,int)->string` |
| `crypto.hex_decode` | `(string)->string` |
| `crypto.hex_encode` | `(string)->string` |
| `crypto.hmac_sha256` | `(string,string)->string` |
| `crypto.hmac_sha512` | `(string,string)->string` |
| `crypto.md5` | `(string)->string` |
| `crypto.random_int` | `(int,int)->int` |
| `crypto.random_bytes` | `(int)->blob` |
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

SQLite and PostgreSQL handles. Connections provide `ping`, `exec`, `query`,
`query_row`, `prepare`, connection-pool setters, `begin`, `transaction`, and
`close`. Prepared statements provide `exec`, `query`, `query_row`, and `close`;
transactions provide `exec`, `query`, `prepare`, `commit`, and `rollback`.

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
| `ffi.load_embedded` | `(string)->json` |
| `ffi.ptr_to_str` | `(int)->string` |
| `ffi.ptr_to_utf16` | `(int)->string` |
| `ffi.size_of` | `(string)->int` |
| `ffi.utf16_str` | `(string)->int` |

`ffi.load_embedded` loads a native library that was packed into the program
with `alca build --cc` or `alca build --native-lib`. The library is unpacked
from the resources of the program into the temporary directory of the process
and loaded from there.

### `mem`

Raw memory for a program: it allocates a block and reads and writes that block
by address. The memory a program can reach is only the memory it allocated
itself, so the module cannot be used to read or write the memory of the
process. Trusted code only.

| Member | Signature |
| --- | --- |
| `mem.add` | `(int,int)->int` |
| `mem.alloc` | `(int)->int` |
| `mem.copy` | `(int,int,int)->bool` |
| `mem.fill` | `(int,int,int)->bool` |
| `mem.free` | `(int)->bool` |
| `mem.is_null` | `(int)->bool` |
| `mem.read_bytes` | `(int,int?,int)->blob` |
| `mem.read_f64` | `(int,int?)->float` |
| `mem.read_i16` | `(int,int?)->int` |
| `mem.read_i32` | `(int,int?)->int` |
| `mem.read_i64` | `(int,int?)->int` |
| `mem.read_i8` | `(int,int?)->int` |
| `mem.read_u16` | `(int,int?)->int` |
| `mem.read_u32` | `(int,int?)->int` |
| `mem.read_u64` | `(int,int?)->int` |
| `mem.read_u8` | `(int,int?)->int` |
| `mem.size` | `(int)->int` |
| `mem.size_of` | `(string)->int` |
| `mem.write_bytes` | `(int,blob,int?)->bool` |
| `mem.write_f64` | `(int,float,int?)->bool` |
| `mem.write_i16` | `(int,int,int?)->bool` |
| `mem.write_i32` | `(int,int,int?)->bool` |
| `mem.write_i64` | `(int,int,int?)->bool` |
| `mem.write_i8` | `(int,int,int?)->bool` |
| `mem.write_u16` | `(int,int,int?)->bool` |
| `mem.write_u32` | `(int,int,int?)->bool` |
| `mem.write_u64` | `(int,int,int?)->bool` |
| `mem.write_u8` | `(int,int,int?)->bool` |
| `mem.zero` | `(int,int)->bool` |

The readers take the address and an optional offset, the writers the address,
the value and an optional offset.

```alca
use std.mem;
addr := mem.alloc(8);
mem.write_u32(addr, 305419896);
println(mem.read_u32(addr));
```

### `mtproto`

The protocol of the Telegram network: the AES-IGE cipher, the authorization
key exchange, the message framing, the transport and the serialization layer of
the protocol language. The API of the network is not part of the protocol: a
program passes the identifier of the method and the serialized arguments, so
the module stays independent of the API schema.

| Member | Signature |
| --- | --- |
| `mtproto.call` | `(int,int,blob)->blob` |
| `mtproto.close` | `(int)->bool` |
| `mtproto.connect` | `(int)->json` |
| `mtproto.dc_list` | `()->list[json]` |
| `mtproto.decrypt_message` | `(blob,string,blob)->json` |
| `mtproto.dh_derive` | `(int,blob,blob,blob,blob)->blob` |
| `mtproto.dh_start` | `(string)->json` |
| `mtproto.encrypt_message` | `(blob,int,int,int,int,blob)->blob` |
| `mtproto.fingerprint` | `(blob)->blob` |
| `mtproto.ige_decrypt` | `(blob,blob,blob)->blob` |
| `mtproto.ige_encrypt` | `(blob,blob,blob)->blob` |
| `mtproto.key_hex` | `(blob)->string` |
| `mtproto.key_id` | `(blob)->blob` |
| `mtproto.new_msg_id` | `(int)->int` |
| `mtproto.parse_key` | `(string)->blob` |
| `mtproto.rsa_encrypt` | `(string,blob,blob)->blob` |
| `mtproto.tl_bool` | `(bool)->blob` |
| `mtproto.tl_bytes` | `(blob)->blob` |
| `mtproto.tl_int32` | `(int)->blob` |
| `mtproto.tl_int64` | `(int)->blob` |
| `mtproto.tl_read_bool` | `(blob)->bool` |
| `mtproto.tl_read_bytes` | `(blob)->blob` |
| `mtproto.tl_read_int32` | `(blob)->int` |
| `mtproto.tl_read_int64` | `(blob)->int` |
| `mtproto.tl_read_string` | `(blob)->string` |
| `mtproto.tl_string` | `(string)->blob` |
| `mtproto.use_key` | `(int,blob)->bool` |

The members that open a connection need the network capability of the runtime.

```alca
use std.mtproto;
use std.crypto;

key := crypto.random_bytes(32);
data := crypto.random_bytes(16);
iv := crypto.random_bytes(32);
encrypted := mtproto.ige_encrypt(key, data, iv);
println(len(mtproto.ige_decrypt(key, encrypted, iv)));
```

### `embed`

Read-only access to resources included in standalone builds.

| Member | Signature |
| --- | --- |
| `embed.exists` | `(string)->bool` |
| `embed.list` | `(string?)->list[string]` |
| `embed.read` | `(string)->string` |

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
| `io.read_bytes` | `(string)->blob` |
| `io.read_lines` | `(string)->list[string]` |
| `io.temp_dir` | `(string?)->string` |
| `io.temp_file` | `(string?)->string` |
| `io.unlock_file` | `(any)->bool` |
| `io.write_file` | `(string,string)->bool` |
| `io.write_bytes` | `(string,blob)->bool` |
| `io.walk` | `(string)->list[string]` |

`read_lines` supports lines longer than the default Go scanner limit.
`file_exists` reports access/stat failures as errors instead of treating them
as evidence that a path exists.

### `json`

JSON parse/encode, formatting, validation, and dot-path queries. `parse` always
parses text; `query` accepts either a JSON string or an already-decoded value.

| Member | Signature |
| --- | --- |
| `json.encode` | `(any)->string` |
| `json.parse` | `(string)->json` |
| `json.prettify` | `(any,string?)->string` |
| `json.query` | `(string,string)->json;(json,string)->json` |
| `json.stringify` | `(any)->string` |
| `json.validate` | `(string)->bool` |

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

The bytecode compiler lowers exact-arity positional calls to `math.abs`,
`min`, `max`, `clamp`, `sqrt`, `round`, `floor`, and `ceil` to numeric
intrinsics. The AMD64 JIT executes integer and floating-point arguments
directly; unsupported runtime types deoptimize to the VM for the standard
library's normal error behavior. Other `std.math` functions execute in the JIT
runtime-helper tier; direct unshadowed calls are fused as pure calls. See
[JIT and standalone execution](#jit-and-standalone-execution) for the exact
per-engine behavior.

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
| `net.tcp_connect` | `(string)->map[string, json]` |
| `net.tcp_listen` | `(string)->map[string, json]` |

The simple HTTP helpers limit response bodies to 16 MiB, as does `http`.
`ping` reports a successful TCP connection rather than treating DNS
resolution alone as host reachability.

### `http`

HTTP client requests and a small HTTP server. Client response bodies are
limited to 16 MiB. The optional options map supports a request `timeout` in
seconds (greater than 0, at most 300).

`http.router()` creates a route table. Register handlers with `get`, `post`,
`put`, `patch`, `delete`, `head`, `options`, or `route(method, path, handler,
options?)`; paths support `{name}` parameters and a final `{name:path}`
wildcard. Each handler receives a request map containing `method`, `url`,
`path`, `headers`, `query`, `params`, `body`, and (for JSON requests) `json`.
Return a string for a text response, or use `http.json(value, status?,
headers?)` / `http.response(status, body?, headers?)`.

`router.use(middleware)` adds middleware in registration order. A middleware
receives `(request, next)` and can call `next(request)` to continue or return a
response directly. Route options support a `schema` map with `path`, `query`,
and `body` field maps. Field types are `string`, `int`, `float`, `number`,
`bool`, `object`, `array`, `json`, or `any`; fields are required by default
and can use `{"type": "int", "required": false}` to be optional. Query and
path values are converted to the declared types; JSON bodies are validated
against object fields. Invalid input receives HTTP 422, malformed JSON
receives HTTP 400, missing routes return 404, and method mismatches return 405.

Route options can also include `dependencies`, a map from names to functions.
Each dependency receives the request and its result is available under
`request["dependencies"]`. `summary`, `description`, and `tags` are included
in generated OpenAPI metadata. `router.openapi()` returns a basic OpenAPI 3.0.3
document with route parameters and request-body schemas;
`router.openapi("/openapi.json")` registers a JSON document route. Start
serving with `router.serve(port)`. The returned `HTTPServer` exposes `port` and
`close()`. `http.http_serve(port, handler)` remains available for single-handler
servers.

```alca
use std.http;

router := http.router();
health := |request: map[string, json]| http.json({"ok": true});
options: map[string, json] = {"summary": "Health check"};
router.get("/health", health, options);
server := router.serve(8080);
println(server.port);
server.close();
```

| Member | Signature |
| --- | --- |
| `http.delete` | `(string,map[string, string]?,string?,map[string, json]?)->map[string, json]` |
| `http.get` | `(string,map[string, string]?,string?,map[string, json]?)->map[string, json]` |
| `http.patch` | `(string,map[string, string]?,string?,map[string, json]?)->map[string, json]` |
| `http.post` | `(string,map[string, string]?,string?,map[string, json]?)->map[string, json]` |
| `http.put` | `(string,map[string, string]?,string?,map[string, json]?)->map[string, json]` |
| `http.json` | `(any,int?,map[string, json]?)->map[string, json]` |
| `http.response` | `(int,any?,map[string, json]?)->map[string, json]` |
| `http.request` | `(string,string,map[string, string]?,string?,map[string, json]?)->map[string, json]` |
| `http.router` | `()->HTTPRouter` |
| `http.http_serve` | `(int,any)->HTTPServer` |

`HTTPRouter` provides `get`, `post`, `put`, `patch`, `delete`, `head`,
`options`, `route`, `use`, `serve`, and `openapi` methods. Route registration
returns `bool`; `serve` returns an `HTTPServer`.

| Router method | Signature |
| --- | --- |
| `router.get/post/put/patch/delete/head/options` | `(string,any,map[string, json]?)->bool` |
| `router.route` | `(string,string,any,map[string, json]?)->bool` |
| `router.use` | `(any)->bool` |
| `router.serve` | `(int)->HTTPServer` |
| `router.openapi` | `(string?)->json` |
| `server.close` | `()->bool` |

### `urllib2`

URL parsing/encoding and bounded HTTP request helpers inspired by Python's
`urllib`. `parse` returns a map with `scheme`, `netloc`, `path`, `params`,
`query`, `fragment`, `username`, `password`, `hostname`, and `port`.
`split` returns the classic six URL components as a list. `urlencode` accepts
a map; list values become repeated query parameters. `parse_qs` groups repeated
values into lists, while `parse_qsl` returns ordered key/value pairs.

`request(method, url, data?, headers?, timeout?)` returns a map containing
`status`, `reason`, `body`, `headers`, and the final redirected `url`.
`data` may be a string, blob, or map; maps are form-encoded and use
`application/x-www-form-urlencoded`. `urlopen(url, data?, headers?, timeout?)`
uses GET without data and POST when data is supplied. `get` and `post` are
convenience wrappers. Requests have a 30-second default timeout (up to
300 seconds) and a 16 MiB response cap. Redirects are rechecked against the
network capability policy. `urlretrieve(url, destination)` saves a successful
response under the write capability and returns its filename and headers.

```alca
use std.urllib2;

parts := urllib2.parse("https://example.com/search?q=alca");
query: string = string(parts["query"]);
encoded: string = urllib2.urlencode({"q": "Alca language", "page": "1"});
response := urllib2.get("https://example.com/search?" + encoded);
println(response["status"]);
println(response["body"]);
```

| Member | Signature |
| --- | --- |
| `urllib2.parse` | `(string,string?)->map[string, json]` |
| `urllib2.split` | `(string,string?)->list[string]` |
| `urllib2.join` | `(string,string)->string` |
| `urllib2.quote` | `(string,string?)->string` |
| `urllib2.unquote` | `(string,bool?)->string` |
| `urllib2.urlencode` | `(any)->string` |
| `urllib2.parse_qs` | `(string)->map[string, list[string]]` |
| `urllib2.parse_qsl` | `(string)->list[list[string]]` |
| `urllib2.request` | `(string,string,any?,any?,int?)->map[string, json];(string,string,any?,any?,float?)->map[string, json]` |
| `urllib2.urlopen` | `(string,any?,any?,int?)->map[string, json];(string,any?,any?,float?)->map[string, json]` |
| `urllib2.get` | `(string,any?,int?)->map[string, json];(string,any?,float?)->map[string, json]` |
| `urllib2.post` | `(string,any,any?,int?)->map[string, json];(string,any,any?,float?)->map[string, json]` |
| `urllib2.urlretrieve` | `(string,string)->map[string, json]` |

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
| `time.date` | `(int?,string?)->map[string, json];(float?,string?)->map[string, json]` |
| `time.diff` | `(int,int)->float;(float,float)->float` |
| `time.format` | `(int,string?,string?)->string;(float,string?,string?)->string` |
| `time.measure` | `(json)->map[string, json]` |
| `time.now` | `()->int` |
| `time.now_ms` | `()->int` |
| `time.now_ns` | `()->int` |
| `time.now_str` | `(string?,string?)->string` |
| `time.parse` | `(string,string?,string?)->int` |
| `time.parse_duration` | `(string)->int` |
| `time.since` | `(int)->float;(float)->float` |
| `time.sleep` | `(int)->null;(float)->null;(string)->null` |
| `time.timer` | `(string)->map[string, json]` |
| `time.unix_ms_to_seconds` | `(int)->float` |
| `time.unix_seconds_to_ms` | `(int)->int;(float)->int` |

Timestamps returned by `now` use seconds; `now_ms` and `now_ns` state their
units explicitly. The optional time zone argument accepts an IANA location.
`parse_duration` returns nanoseconds. Avoid inferring timestamp units from
magnitude; `format`/`since`/`date` use seconds, and the conversion helpers
make millisecond conversions explicit. `timer(duration)` returns an object
with blocking `wait()` and `cancel()` methods; `wait()` returns `false` when
cancelled.

### `string`

Unicode-aware text helpers. `length`, `slice`, and `runes` count Unicode code
points (runes), not UTF-8 bytes or grapheme clusters. Slice indices are
zero-based, end-exclusive; negative indices count from the end.

| Member | Signature |
| --- | --- |
| `string.contains` | `(string,string)->bool` |
| `string.ends_with` | `(string,string)->bool` |
| `string.index_of` | `(string,string)->int` |
| `string.join` | `(list[string],string)->string` |
| `string.length` | `(string)->int` |
| `string.lower` | `(string)->string` |
| `string.replace` | `(string,string,string)->string` |
| `string.runes` | `(string)->list[string]` |
| `string.slice` | `(string,int,int)->string` |
| `string.split` | `(string,string)->list[string]` |
| `string.starts_with` | `(string,string)->bool` |
| `string.trim` | `(string)->string` |
| `string.trim_left` | `(string)->string` |
| `string.trim_right` | `(string)->string` |
| `string.upper` | `(string)->string` |

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
- `io`: `append_file`, `check_access`, `chmod`, `copy_file`, `delete_file`, `file_exists`, `file_info`, `file_size`, `glob`, `is_permission_error`, `list_dir`, `lock_file`, `make_dir`, `move_file`, `open`, `permissions`, `read_bytes`, `read_file`, `read_lines`, `temp_dir`, `temp_file`, `unlock_file`, `walk`, `write_bytes`, `write_file`
- `net`: `dns_lookup`, `download_file`, `http_get`, `http_post`, `ping`
- `crypto`: `aes_decrypt`, `aes_decrypt_password`, `aes_encrypt`, `aes_encrypt_password`, `base64_decode`, `base64_encode`, `constant_time_equal`, `derive_key`, `hex_decode`, `hex_encode`, `hmac_sha256`, `hmac_sha512`, `md5`, `random_bytes`, `random_int`, `random_string`, `sha1`, `sha256`, `sha512`, `uuid`
- `compress`: `gzip_compress`, `gzip_decompress`, `zlib_compress`, `zlib_decompress`
- `process`: `cpu_count`, `exec_bg`, `exec_cmd`, `get_pid`, `get_ppid`, `kill_process`, `memory_usage`, `process_exists`, `signal_process`, `uptime`

## JIT and standalone execution

Every registered standard-library member is callable from JIT-compiled
functions. There are three genuinely different execution paths, and none of
them silently pretends that a host-dependent operation was translated to
machine code:

- **Native intrinsic**: the AMD64 backend emits the operation directly. Only
  the eight numeric `std.math` members listed above have intrinsics. They are
  selected only for exact-arity calls through an unshadowed module alias
  (`use std.math;` or `use std.math as m;`) that never escapes and is never
  mutated. If the alias is assigned, passed as a value, used with a subscript,
  or a module member is modified, the call uses the generic JIT helper path (or
  the VM when no JIT is active) so the mutation is observed. If the runtime
  value is not numeric, the intrinsic
  records a deoptimization point and the VM raises the same error the library
  would raise.
- **JIT runtime helper (fused pure call)**: direct calls to pure stdlib members
  are compiled to `OpStdlibCall`. The threaded JIT and the AMD64 JIT engine
  resolve the registered function and execute it in the JIT runtime-helper
  tier, then resume JIT execution immediately after the call. The function body
  still runs as its normal Go implementation; it is not machine code, but the
  call does not enter VM bytecode dispatch. List and map literals used as
  arguments are constructed by the same helper tier, so expressions such as
  `str.join([...], "-")` and `formats.json.encode({...})` stay on this path.
  Whole pure modules are `math`, `string`/`str`, `re`/`regex`, `compress`, and
  `json`. Deterministic value members of mixed modules (`path.join`,
  `crypto.sha256`, `urllib2.parse`, `urllib2.join`, `time.format`, the
  in-memory `csv.parse`/`csv.stringify` and `ini.parse`/`ini.stringify`
  members, the nested `formats.json.*`, `formats.csv.parse`, and
  `formats.ini.parse` paths, and the deterministic `ai` operations) use the
  same fused path; host, clock, entropy, or file-backed members such as
  `path.abs`, `path.expand`, `path.which`, `crypto.uuid`,
  `crypto.aes_encrypt`, `urllib2.request`, `time.now`, `csv.read`,
  `csv.write`, `ini.read`, `ini.write`, and `ai.matrix` use the generic
  host-call helper.
- **JIT runtime helper (host calls)**: for non-pure members, including network,
  database, general file I/O, GUI, process, and the request/download members of
  `urllib2`, the JIT performs the module lookup, member access, and registered
  builtin call in the same runtime-helper tier and then resumes JIT execution.
  The host operation still runs its normal Go implementation and is subject to
  the same capability policy; it is not translated to machine code and does not
  enter VM bytecode dispatch. If the runtime binding is not a callable the
  helper understands (for example a tree-walking `FuncValue` or a namespace
  value), the call site deoptimizes to the VM without side effects.

`use` statements execute in the same JIT runtime-helper tier. After binding the
import, native or threaded execution resumes at the next instruction, so a
top-level pure call is not forced back into the VM merely because the program
had to import a module first.

User-defined compiled functions and closures are called through the same VM
call helper. `OpCall` and `OpClosure` run on the VM that owns the active stack
through `JITCallbacks`, so argument evaluation, recursion depth, captured
upvalues, globals, and handler unwinding match bytecode execution. A closure
created by a JIT frame captures from that same frame and stays valid when it
escapes; `OpGetFree`/`OpSetFree` read and write both open and closed upvalues,
and frame returns close open upvalues exactly as the VM does. An arity
mismatch bails before the call, so the VM reports its exact error without side
effects. Callable values the helper cannot run — currently tree-walking
`FuncValue` values and namespace values — still deoptimize to the VM.

Container access and mutation (`list[i]`, `map[key]`, `obj.field = value`),
including int- and rune-backed list representations, single-argument builtin
conversions (`int`, `float`, `bool`, `len`), `**`, `in`, range construction,
iterable preparation, equality/ordering on strings and collections, `err(...)`
construction and raising, and `assert` also execute in the helper tier instead
of deoptimizing the enclosing function.

Fused lowering is deliberately conservative. It requires a direct member call
through an unshadowed `use` alias with no explicit import list. Assigning the
alias, passing the module as a value, subscripting it, declaring a function
with the same name, or modifying a member disables the fused path for that
alias. The normal call path then observes the mutation exactly as before.
Runtime lookup of a non-builtin binding also falls back to the VM without
having performed any side effects.

| Engine or build | Pure stdlib member (fused) | Host stdlib member | `math.abs` / `min` / `max` / `clamp` / `sqrt` / `round` / `floor` / `ceil` |
| --- | --- | --- | --- |
| Tree-walking interpreter | Direct call, no JIT | Direct call, no JIT | Direct call, no JIT |
| Bytecode VM (JIT disabled) | `OpStdlibCall`, VM executes the member | Direct VM call | `OpMathBuiltin` |
| Threaded JIT | JIT runtime helper; resumes threaded code | JIT runtime helper; resumes threaded code | JIT runtime helper |
| AMD64 native JIT | JIT runtime helper; resumes native code | JIT runtime helper; resumes native code | Inline machine code; invalid types bail out |
| Standalone runtime stub (bytecode or AOT payload) | Bundled runtime executes; AOT code resumes after the helper | Bundled runtime executes; AOT code resumes after the helper | Inline when JIT/AOT code uses an intrinsic |
| Pure native PE/AOT (no VM) | Unavailable | Unavailable | Not selected |

User calls follow the same split: threaded and AMD64 JIT execute compiled
functions and closures through the VM call helper; the bytecode VM executes
`OpCall`/`OpClosure` directly; pure native PE has no VM or callbacks, so those
opcodes are outside its safe set. Pure-native execution that reaches them is
rejected at build time or terminates with status 70 on a runtime bailout.

Programs that import any `std.*` module are never built as a pure native PE.
The build uses the runtime stub, which bundles the VM, so stdlib calls remain
available. Pure native PE selection also requires every opcode to be in the
native-safe set; calls, module imports, collections, container access and
mutation, conversions, streams, strict/deep equality, and math intrinsics are
excluded. The builder additionally rejects boxed constants such as strings,
because their value representation needs the Go runtime.

A pure native PE has no VM to resume after a deoptimization. If native
execution nevertheless reaches a runtime bailout or fault, the executable
prints an explicit diagnostic on stderr and exits with a nonzero status (70);
it never exits 0 as if the program had completed. This is a deliberate hard
failure, not a fallback.

Native code writes stack slots directly instead of going through the VM's
bounds-checked push helper. To keep that safe, the AMD64 backend refuses to
push a slot once the VM stack is full: the frame bails out to the bytecode VM,
which reports the overflow exactly as it does for interpreted code. A pure
native PE applies the same guard and turns the overflow into the explicit
deoptimization diagnostic and exit status 70 instead of writing outside the
stack.

Tooling can query the same classification instead of guessing from module
names:

```go
support := jit.ClassifyStdlibMember("math", "sqrt", jit.StdlibAMD64JIT)
// support.Tier == jit.StdlibNativeTier

support = jit.ClassifyStdlibMember("string", "upper", jit.StdlibAMD64JIT)
// support.Tier == jit.StdlibJITHelperTier

support = jit.ClassifyStdlibMember("urllib2", "request", jit.StdlibAMD64JIT)
// support.Tier == jit.StdlibJITHelperTier (host call, still not machine code)

rows := jit.StdlibSupportTable(jit.StdlibPureNative)
// every row is jit.StdlibUnavailableTier
```

`StdlibDeoptTier` is reserved for call sites that cannot use a runtime helper,
such as a module binding replaced by a tree-walking `FuncValue` or a namespace
value. A module binding replaced by a compiled function is handled by the VM
call helper. The table classifies known members by their normal registered
implementation.

The table covers every member in the canonical signature list and accepts the
documented aliases (`regex`, `str`, `zip`) plus the `std.` prefix.

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

`process`, the global `exec` function, `env`, `ffi`, `mem`, `gui`, and `sys`
expose host resources (command execution, environment mutation, native calls,
raw memory, UI, and machine information).
They are intentionally outside the read/write/network capability model: only
run trusted programs with access to those modules, or omit them from a
restricted build. The `mem` module limits a program to the memory it allocated
itself, so it is not a way around the policy, but it is still low level code.
The members of `mtproto` that open a connection need the network capability;
the rest of the module works without any grant.

Denied operations return Alca error code `403`, unwrap to
`os.ErrPermission` in Go, and are handled by `io.is_permission_error` in Alca.
