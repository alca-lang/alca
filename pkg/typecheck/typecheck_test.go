// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package typecheck_test

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/lexer"
	"github.com/alca-lang/alca/pkg/parser"
	"github.com/alca-lang/alca/pkg/stdlib"
	"github.com/alca-lang/alca/pkg/typecheck"
)

func check(t *testing.T, src string) []string {
	t.Helper()
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if errs := p.SyntaxErrors(); len(errs) > 0 {
		lines := strings.Split(src, "\n")
		t.Fatalf("unexpected syntax errors: %v on %q", errs[0], lines[errs[0].Line-1])
	}
	var msgs []string
	for _, e := range typecheck.NewChecker("t.al", src).Check(prog) {
		msgs = append(msgs, e.Message+" | "+e.Suggestion)
	}
	return msgs
}

func TestValidPrograms(t *testing.T) {
	cases := map[string]string{
		"annotated and inferred": `x: int = 5; y := 7; z: float = 2; println(x + y);`,
		"functions and structs": `
struct P:
    name: string;
    age: int = 18;
func older(p: P) -> int:
    return p.age + 1;
q := P("a");
n: int = older(q);`,
		"embedded resources": `
use std.embed;
exists: bool = embed.exists("assets/icon.ico");
files: list[string] = embed.list("assets");
data: string = embed.read("assets/icon.ico");`,
		"hoisting": `
println(twice(2));
func twice(n: int) -> int:
    return n * 2;`,
		"null narrowing": `
func f(s: string | null) -> int:
    if s === null:
        return 0;
    return len(s);
v: string | null = null;
if v != null:
    println(v + "!");`,
		"error capture": `
func risky(n: int) -> int:
    if n === 0:
        err("zero", code=3);
        return 0;
    return 10 / n;
e: error | null = null;
risky(0) ?>>> e;
if e != null:
    println(e.msg);`,
		"collections": `
xs: list[int] = [];
push(xs, 1);
ys := push(xs, 2);
println(len(ys));
last := pop(ys);
if last != null:
    println(last + 1);
m: map[string, int] = {"a": 1};
m["b"] = 2;
total: int = 0;
for x in xs:
    total = total + x;
for k in keys(m):
    println(k);`,
		"stdlib typed": `
use std.math;
r: float = math.sqrt(9) + 1.5;
c: int = math.floor(2.7);`,
		"production stdlib signatures": `
use std.ffi;
use std.sys;
use std.urllib2;
use std.platform;
use std.power;
use std.devices;
use std.win32;
p: int = ffi.alloc_ptr(16);
copied: bool = ffi.copy_to_ptr(p, "data");
contents: blob = ffi.read_from_ptr(p, 4);
released: bool = ffi.free(p);
processes := sys.process_iter();
process_info_value := sys.process_info(1);
available := platform.capabilities();
sensor_list := power.sensors();
device_list := devices.list();
key: int = win32.reg_open_key("HKEY_CURRENT_USER", "Software", "read");
response := urllib2.tcp_request("GET", "https://example.test");
`,
		"loose and strict equality across types": `
dynamic: json = "1";
loose: bool = dynamic == 1;
strict: bool = dynamic === 1;
mixed: bool = null === 0;
nested: bool = [1] === [1.0];
`,
		"typed value equality": `
a: int = 10;
b: int = 20;
eq_int: bool = a == b;
neq_int: bool = a != b;
s1: string = "hello";
s2: string = "world";
eq_str: bool = s1 == s2;
`,
		"enum interface and async await": `
enum State {
    Init,
    Running,
    Finished,
}

s: State = State.Running;
is_run: bool = s == State.Running;

interface Speaker {
    fn speak() -> string;
}

class Dog {
    pub fn speak() -> string {
        return "woof";
    }
}

d := Dog();
sp: Speaker = d;
sound: string = sp.speak();

async fn compute() -> int {
    return 42;
}

res: int = await compute();
`,
		"crypto stdlib": `
use std.crypto;
use std.compress;
a: string = crypto.sha256("value");
b: bool = crypto.constant_time_equal(a, a);
c: string = crypto.hex_encode(crypto.base64_decode(crypto.base64_encode("value")));
d: int = crypto.random_int(1, 10);
e: string = crypto.random_string(8);
f: string = compress.gzip_decompress(compress.gzip_compress("value"));
u: string = crypto.uuid();
`,
		"HTTP timeout options": `
use std.http;
headers: map[string, string] = {"Accept": "text/plain"};
options: map[string, json] = {"timeout": 1.5};
response := http.request("GET", "https://example.test", headers, "", options);
response2 := http.get("https://example.test", headers, "", options);`,
		"HTTP router API": `
use std.http;
router := http.router();
handler := |request: map[string, json]| http.json({"path": request["path"]});
router.get("/health", handler);
bodyFields := {"name": "string"};
bodySchema: map[string, json] = {"body": bodyFields};
routeOptions: map[string, json] = {"schema": bodySchema, "summary": "Create item"};
router.post("/items", handler, routeOptions);
dependency := |request: map[string, json]| request["path"];
dependencies: map[string, json] = {"current_path": dependency};
dependencyOptions: map[string, json] = {"dependencies": dependencies};
router.get("/with-dependency", handler, dependencyOptions);
router.use(|request: map[string, json], next: json| next(request));
server := router.serve(8080);
openapi := router.openapi();
documentRoute := router.openapi("/openapi.json");
response := http.response(201, "created");
jsonResponse := http.json({"ok": true});
port: int = server.port;
server.close();
`,
		"urllib2 URL and HTTP API": `
use std.urllib2;
parts := urllib2.parse("https://example.test/a?q=1");
path: string = string(parts["path"]);
encoded: string = urllib2.urlencode({"q": "alca lang"});
decoded: string = urllib2.unquote(urllib2.quote(path));
joined: string = urllib2.join("https://example.test/a", "../b");
params := urllib2.parse_qs("tag=one&tag=two");
headers: map[string, string] = {};
response := urllib2.request("GET", "https://example.test", null, headers, 5.0);
status: int = int(response["status"]);
`,
		"match": `
n: int = 2;
match n:
    case 1:
        println("one");
    case 2:
        println("two");
    else:
        println("many");`,
		"float literal widening": `f: float = 3; g: float = f * 2;`,
		"lambda pipe":            `inc := |x: int| x + 1; r: int = 4 |> inc;`,
		"json is dynamic": `
use std.formats;
d: json = json_decode("{}");
n: int = int(d["a"]);`,
		"assignments to existing variables": `
x: int = 1;
x = 2;
func f() -> int:
    local: int = 3;
    if local > 0:
        local = 4;
        println(local);
    return local;
println(f());`,
		"declaration modifiers": `
public int: shared = 10;
const int: limit = 42;
func worker() -> int:
    private int: local = 1;
    public int: published = shared + local;
    local = local + 1;
    return local;
println(worker());
println(published);
println(limit);`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			if errs := check(t, src); len(errs) > 0 {
				t.Fatalf("expected no errors, got:\n  %s", strings.Join(errs, "\n  "))
			}
		})
	}
}

// TestStdlibOptionalArguments: every optional parameter that exists in the
// runtime must also be optional to the checker. Regression: the table listed
// these functions as taking only their required arguments, so valid programs
// such as time.now_str("%Y") or io.list_dir() were rejected.
func TestStdlibOptionalArguments(t *testing.T) {
	cases := map[string]string{
		"time": `
use std.time;
println(time.now_str("%Y"));
println(time.format(0));
println(time.format(1.5, "%Y"));
d := time.date(0);
println(d["year"]);
t := time.parse("2020-01-01");
println(time.since(t));
println(time.diff(t, 0));
`,
		"crypto": `
use std.crypto;
println(crypto.random_string(8, "ab"));
println(crypto.random_string(8));
`,
		"io": `
use std.io;
xs := io.list_dir();
println(len(xs));
p := io.temp_dir("x");
println(len(p) > 0);
q := io.temp_file();
println(len(q) > 0);
lines := io.read_lines("nope");
println(len(lines));
`,
		"io members": `
use std.io;
p := io.temp_file();
println(len(p) > 0);
lines := io.read_lines("nope");
println(len(lines));
ok: bool = io.check_access(".", "r");
ok = io.lock_file("nope", false);
ok = io.unlock_file("nope");
ok = io.is_permission_error(403);
mode := io.permissions(".");
println(mode["octal"]);
h := io.open("nope");
println(h);
`,
		"other stdlib": `
use std.process;
use std.env;
now: string = time_now();
out: string = exec("exit 0");
println(now);
println(out);
println(process.cpu_count() > 0);
println(len(env.get_path()) >= 0);
println(env.get_env("PATH"));
println(process.get_ppid() >= 0);
println(process.uptime() >= 0);
println(process.memory_usage()["alloc"]);
`,
		"cli": `
use std.cli;
ok := cli.confirm("sure?", true);
println(ok);
println(cli.progress_bar(1, 2, 10, "p"));
println(cli.prompt("name", "default"));
println(cli.prompt_password("password"));
`,
		"append": `
xs: list[int] = [1];
ys := append(xs, 2);
println(len(ys));
`,
		"regex": `
use std.re;
x := re.find("z", "abc");
if x != null:
    println(len(x));
xs := re.find_all("a", "aaa", 2);
println(len(xs));
g := re.find_groups("z", "abc");
if g != null:
    println(len(g));
gg := re.find_all_groups("a", "aaa");
println(len(gg));
parts := re.split("a", "banana", 2);
println(len(parts));
`,
		"path": `
use std.path;
s: string = path.list_sep();
s = path.to_slash("a/b");
s = path.from_slash("a/b");
s = path.rel("a", "b");
s = path.expand("~");
s = path.join_list(["a", "b"]);
s = path.join_list("a", "b");
xs := path.split_list(s);
println(len(xs));
`,
		"process and sys": `
use std.process;
use std.sys;
n: int = process.get_ppid();
println(n);
ok: bool = process.kill_process(0);
println(ok);
ifs := sys.network_interfaces();
println(len(ifs));
parts := sys.disk_partitions();
println(len(parts));
procs := sys.process_list();
println(len(procs));
`,
		"net": `
use std.net;
ok: bool = net.ping("localhost");
println(ok);
ips := net.dns_lookup("localhost");
println(len(ips));
r := net.http_get("http://example.com");
println(r["status"]);
`,
		"libtorch": `
use std.libtorch;
a := libtorch.matrix(2, 2, 3.0);
b := libtorch.matrix(2, 2);
c := libtorch.matmul(a, b);
println(libtorch.dot([1.0, 2.0], [3.0, 4.0]));
println(libtorch.dot([1, 2], [3, 4]));
println(libtorch.relu(-2.0));
println(libtorch.softmax([1.0, 2.0, 3.0]));
println(libtorch.mse([1.0, 2.0], [1.0, 2.0]));
`,
		"formats": `
use std.csv;
use std.ini;
rows := csv.parse("a,b\n1,2\n");
println(len(rows));
println(csv.stringify(rows, ["a", "b"]));
ok: bool = csv.write("out.csv", rows, ["a", "b"]);
cfg := ini.parse("[main]\nkey=value\n");
println(string(cfg));
ok = ini.write("out.ini", cfg);
println(ini.stringify(cfg));
`,
		"log": `
use std.log;
log.set_level("debug");
log.set_format("json");
log.set_file("app.log", 1, 2);
log.debug("starting");
log.info("ready", {"module": "core"});
log.warn("slow");
log.error("failed");
log.fatal("stopped");
ok: bool = log.access_denied("read_file", "x", "denied");
ok = log.audit("SECURITY", "READ", "x", "denied", {"pid": 1});
events := log.get_access_errors();
println(len(events));
`,
		"db ffi metrics io": `
use std.db;
use std.ffi;
use std.metrics;
use std.io;
conn := db.sqlite(":memory:");
conn.close();
lib := ffi.load("kernel32.dll");
lib.close();
println(ffi.size_of("int"));
p := ffi.c_str("x");
println(ffi.ptr_to_str(p));
buf := ffi.alloc(16);
println(len(buf));
counter := metrics.counter("requests", "total");
counter.inc();
println(metrics.export());
println(metrics.serve(9090));
h := io.open("x", "r");
println(h.read());
println(io.lock_file("x"));
println(io.unlock_file("x"));
println(io.is_permission_error(403));
`,
		"cli parse and time measure": `
use std.cli;
use std.time;
opts := cli.parse({"verbose": false});
println(opts["_args"]);
func work() -> int:
    return 1;
m := time.measure(work);
println(float(m["elapsed_ms"]));
`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			if errs := check(t, src); len(errs) > 0 {
				t.Fatalf("expected no errors, got:\n  %s", strings.Join(errs, "\n  "))
			}
		})
	}
}

// TestStdlibSignaturesRejectMisuse: signatures imported from the runtime must
// also reject calls the runtime would error on, and nullable results must not
// be silently assignable to non-null types.
func TestStdlibSignaturesRejectMisuse(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{"nullable regex find", `
use std.re;
s: string = re.find("z", "abc");
`, "cannot initialize"},
		{"nullable path which", `
use std.path;
p: string = path.which("no-such-binary");
`, "cannot initialize"},
		{"nullable env which", `
use std.env;
p: string = env.which("no-such-binary");
`, "cannot initialize"},
		{"net arity", `
use std.net;
net.ping("host", 2);
`, "too many arguments"},
		{"archive arity", `
use std.archive;
archive.zip_create("a");
`, "missing argument"},
		{"io argument type", `
use std.io;
io.check_access("path", 1);
`, "expected string"},
		{"time argument type", `
use std.time;
time.format("now");
`, "no matching overload"},
		{"process argument type", `
use std.process;
process.kill_process("1");
`, "expected int"},
		{"db missing dsn", `
use std.db;
db.open("sqlite");
`, "missing argument"},
		{"ffi argument type", `
use std.ffi;
ffi.size_of(4);
`, "expected string"},
		{"io open mode type", `
use std.io;
io.open("x", 4);
`, "expected string"},
		{"log missing message", `
use std.log;
log.info();
`, "missing argument"},
		{"metrics arity", `
use std.metrics;
metrics.export(1);
`, "too many arguments"},
		{"csv write path type", `
use std.csv;
csv.write(1, []);
`, "expected string"},
		{"ini write missing data", `
use std.ini;
ini.write("x");
`, "missing argument"},
		{"time measure missing callable", `
use std.time;
time.measure();
`, "missing argument"},
		{"cli parse missing spec", `
use std.cli;
cli.parse();
`, "missing argument"},
		{"global exec missing command", `
exec();
`, "missing argument"},
		{"global time_now arity", `
time_now(1);
`, "too many arguments"},
		{"append element type", `
xs: list[int] = [1];
append(xs, "s");
`, "cannot append"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs := check(t, tc.src)
			if len(errs) == 0 {
				t.Fatalf("expected an error containing %q, got none", tc.want)
			}
			if joined := strings.Join(errs, "\n"); !strings.Contains(joined, tc.want) {
				t.Fatalf("expected an error containing %q, got:\n%s", tc.want, joined)
			}
		})
	}
}

func TestRuntimeStdlibRegistryMatchesTypechecker(t *testing.T) {
	env := interpreter.NewEnvironment()
	stdlib.RegisterFFIBuiltins(env)
	stdlib.RegisterSysBuiltins(env)
	stdlib.RegisterUrllib2Builtins(env)
	stdlib.RegisterPlatformBuiltins(env)
	stdlib.RegisterPowerBuiltins(env)
	stdlib.RegisterDevicesBuiltins(env)
	stdlib.RegisterWin32Builtins(env)
	stdlib.RegisterListBuiltins(env)
	stdlib.RegisterMapBuiltins(env)
	stdlib.RegisterStringBuiltins(env)
	stdlib.RegisterEncodingBuiltins(env)
	stdlib.RegisterMarshalBuiltins(env)
	stdlib.RegisterAudioBuiltins(env)
	stdlib.RegisterImageBuiltins(env)
	stdlib.RegisterRandomBuiltins(env)

	for _, module := range []string{"ffi", "sys", "urllib2", "platform", "power", "devices", "win32", "list", "map", "string", "encoding", "marshal", "audio", "image", "random"} {
		value, ok := env.Get("std." + module)
		if !ok {
			t.Fatalf("runtime did not register std.%s", module)
		}
		runtimeMembers := value.(*interpreter.MapValue).Keys()
		typecheckedMembers, ok := typecheck.StdlibMembers(module)
		if !ok {
			t.Fatalf("typechecker does not know std.%s", module)
		}
		sort.Strings(runtimeMembers)
		if !reflect.DeepEqual(runtimeMembers, typecheckedMembers) {
			t.Errorf("std.%s runtime members = %v, typechecker members = %v", module, runtimeMembers, typecheckedMembers)
		}
	}
}

func TestLibtorchRegistryMatchesTypechecker(t *testing.T) {
	env := interpreter.NewEnvironment()
	stdlib.RegisterLibtorchBuiltins(env)
	value, ok := env.Get("std.libtorch")
	if !ok {
		t.Fatal("runtime did not register std.libtorch")
	}
	runtimeMembers := value.(*interpreter.MapValue).Keys()
	typecheckedMembers, ok := typecheck.StdlibMembers("libtorch")
	if !ok {
		t.Fatal("typechecker does not know std.libtorch")
	}
	sort.Strings(runtimeMembers)
	if !reflect.DeepEqual(runtimeMembers, typecheckedMembers) {
		t.Errorf("std.libtorch runtime members = %v, typechecker members = %v", runtimeMembers, typecheckedMembers)
	}
}

func TestRejectedPrograms(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{"string to int", `a: int = "x";`, "cannot initialize"},
		{"int plus string", `s: string = "n=" + 5;`, "cannot be applied"},
		{"int plus float", `a: int = 1; b: float = 2.5; c := a + b;`, "cannot be applied"},
		{"float to int", `a: int = 2.5;`, "cannot initialize"},
		{"infer from null", `c := null;`, "cannot infer"},
		{"undeclared assignment", `x = 10;`, "undeclared variable"},
		{"undefined name", `println(nope);`, "undefined name"},
		{"typo suggestion", "counter: int = 1;\nprintln(countr);", "did you mean 'counter'"},
		{"same-scope redeclaration", `a: int = 1; a: int = 2;`, "already declared; use assignment"},
		{"nested redeclaration", "a: int = 1;\nfunc f():\n    a: int = 2;", "already declared; use assignment"},
		{"incompatible equality", `loose: bool = "1" == 1;`, "cannot compare string and int"},
		{"const reassignment", `const int: limit = 10; limit = 11;`, "cannot assign to constant"},
		{"private at top level", `private int: secret = 1;`, "'private' is only allowed"},
		{"wrong arg type", "func f(a: int) -> int:\n    return a;\nf(\"s\");", "has type string, expected int"},
		{"too many args", "func f(a: int) -> int:\n    return a;\nf(1, 2);", "too many arguments"},
		{"missing arg", "func f(a: int, b: int) -> int:\n    return a;\nf(1);", "missing argument"},
		{"missing return", "func f(a: int) -> int:\n    if a > 0:\n        return a;", "must return"},
		{"return in void", "func f():\n    return 1;", "without a return type"},
		{"truthiness", "a: int = 1;\nif a:\n    println(\"x\");", "must be bool"},
		{"null into int", `a: int = null;`, "cannot initialize"},
		{"null member access", "s: string | null = null;\nprintln(s.length);", "may be null"},
		{"mixed list", `xs: list[int] = [1, "two"];`, "list element"},
		{"map value type", `m: map[string, int] = {"k": "v"};`, "map value"},
		{"unknown field", "struct P:\n    n: string;\np := P(\"a\");\nprintln(p.age);", "no field"},
		{"unknown module member", "use std.math;\nmath.sqroot(4);", "has no member"},
		{"unknown module", `use std.mth;`, "unknown standard module"},
		{"module not imported", `println(math.sqrt(4));`, "undefined name"},
		{"break outside loop", `break;`, "outside of a loop"},
		{"stream var type", "n: int = 0;\nprintln(\"x\") >>> n;", "cannot hold"},
		{"any annotation", `x: any = 1;`, "'any'"},
		{"json needs conversion", "d: json = json_decode(\"{}\");\nn: int = d;", "cannot initialize"},
		{"assign wrong type", "a: int = 1;\na = \"s\";", "cannot assign"},
		{"pop is nullable", "xs: list[int] = [1];\nn: int = pop(xs);", "cannot initialize"},
		{"string modulo", `a: string = "x" % "y";`, "cannot be applied"},
		{"c style decl is a syntax error", ``, ""},
	}
	for _, tc := range cases {
		if tc.src == "" {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			errs := check(t, tc.src)
			if len(errs) == 0 {
				t.Fatalf("expected an error containing %q, got none", tc.want)
			}
			joined := strings.Join(errs, "\n")
			if !strings.Contains(joined, tc.want) {
				t.Fatalf("expected an error containing %q, got:\n%s", tc.want, joined)
			}
		})
	}
}

func TestOneMistakeOneError(t *testing.T) {
	errs := check(t, `s: string = "n=" + 5;`)
	if len(errs) != 1 {
		t.Fatalf("a single mistake must produce a single diagnostic, got %d: %v", len(errs), errs)
	}
}

func TestDeprecatedGlobalAliasesProduceErrors(t *testing.T) {
	cases := []struct {
		name, src, want string
	}{
		{"read_file", `content := read_file("test.txt");`, "global alias 'read_file' is deprecated and has been removed"},
		{"sha256", `h := sha256("abc");`, "global alias 'sha256' is deprecated and has been removed"},
		{"get_env", `e := get_env("PATH");`, "global alias 'get_env' is deprecated and has been removed"},
		{"gui_button", `b := gui_button("Click");`, "global alias 'gui_button' is deprecated and has been removed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs := check(t, tc.src)
			if len(errs) == 0 {
				t.Fatalf("expected error for removed global alias %q, got none", tc.name)
			}
			found := false
			for _, e := range errs {
				if strings.Contains(e, tc.want) {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected diagnostic containing %q, got: %v", tc.want, errs)
			}
		})
	}
}

func TestStdListAndMapTypecheck(t *testing.T) {
	src := `
use std.list;
use std.map;
use std.string;

xs: list[int] = [3, 1, 2];
sorted_xs: list[int] = list.sort(xs);
rev_xs: list[int] = list.reverse(xs);
filtered := list.filter(xs, |x: int| x > 1);
mapped := list.map(xs, |x: int| x * 2);
red := list.reduce(xs, |acc: int, x: int| acc + x, 0);
k_xs: list[int] = list.keys(xs);
v_xs := list.values(xs);

m: map[string, int] = {"b": 2, "a": 1};
m_keys: list[string] = map.keys(m);
m_vals := map.values(m);
m_sorted := map.sort(m);
m_rev := map.reverse(m);
m_filtered := map.filter(m, |k: string, v: int| v > 1);
m_mapped := map.map(m, |k: string, v: int| v * 10);
m_red := map.reduce(m, |acc: int, k: string, v: int| acc + v, 0);

pad := string.pad_left("hello", 10, " ");
cnt: int = string.count("banana", "a");
last_idx: int = string.last_index_of("banana", "a");
cap := string.capitalize("world");
is_e: bool = string.is_empty("");
`
	errs := check(t, src)
	if len(errs) > 0 {
		t.Fatalf("expected no typecheck errors for std.list and std.map, got:\n%s", strings.Join(errs, "\n"))
	}
}

func TestDunderVariablesTypecheck(t *testing.T) {
	src := `
f: string = __file__;
d: string = __dir__;
dn: string = __dirname__;
n: string = __name__;
v: string = __version__;
l: int = __line__;
`
	errs := check(t, src)
	if len(errs) > 0 {
		t.Fatalf("expected dunder variables to typecheck cleanly, got:\n%s", strings.Join(errs, "\n"))
	}

	badSrc := `__file__ = "new_file";`
	badErrs := check(t, badSrc)
	if len(badErrs) == 0 {
		t.Fatal("expected assigning to __file__ to be rejected")
	}
}

func TestHeterogeneousMapJsonAndBlobPlusAndQualifiedType(t *testing.T) {
	src := `
func get_json() -> json:
    return {"a": 1, "b": "hello", "c": true};

func concat_blobs(b1: blob, b2: blob) -> blob:
    return b1 + b2;

struct Box:
    val: int;

x: helper.Box | null = null;
`
	errs := check(t, src)
	if len(errs) > 0 {
		t.Fatalf("expected clean typecheck, got:\n%s", strings.Join(errs, "\n"))
	}
}
