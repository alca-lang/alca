package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runCaptured executes src on the given engine and returns stdout and the exit code.
func runCaptured(t *testing.T, src string, eng engine) (string, int) {
	t.Helper()
	dir := t.TempDir()
	file := filepath.Join(dir, "prog.al")
	if err := os.WriteFile(file, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	prog, content, ok := loadProgram(file)
	if !ok {
		t.Fatalf("program failed to parse/type-check")
	}
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	done := make(chan string)
	go func() { b, _ := io.ReadAll(r); done <- string(b) }()
	code, _ := execute(file, content, prog, eng, true)
	w.Close()
	os.Stdout = old
	return <-done, code
}

// corpus is run on BOTH engines; the output must match the expectation on each.
var corpus = []struct{ name, src, want string }{
	{"integer division and power", `
println(7 / 2);
println(-7 / 2);
println(7 % 3);
println(2 ** 10);
println(7.0 / 2.0);`, "3\n-3\n1\n1024\n3.5\n"},

	{"elif chain", `
func classify(n: int) -> string:
    if n < 0:
        return "neg";
    elif n == 0:
        return "zero";
    elif n < 10:
        return "small";
    else:
        return "big";
println(classify(-5));
println(classify(0));
println(classify(5));
println(classify(50));`, "neg\nzero\nsmall\nbig\n"},

	{"string concat order", `
s: string = "";
for i in 0..2:
    s = "a" + s;
    s = s + "b";
println(s);`, "aaabbb\n"},

	{"float accumulate", `
f: float = 1.5;
f = f + 1;
f = f * 2;
println(f);`, "5.0\n"},

	{"long loop does not leak stack", `
func noop(x: int) -> int:
    return x;
c: int = 0;
for i in 0..300000:
    if i % 2 == 0:
        c = c + 1;
    else:
        noop(i);
println(c);`, "150001\n"},

	{"continue and break in for", `
total: int = 0;
for i in 0..10:
    if i % 2 == 0:
        continue;
    if i > 7:
        break;
    total = total + i;
println(total);`, "16\n"},

	{"continue in while", `
i: int = 0;
odd: int = 0;
while i < 10:
    i = i + 1;
    if i % 2 == 0:
        continue;
    odd = odd + 1;
println(odd);`, "5\n"},

	{"nested loops", `
n: int = 0;
for i in 0..3:
    for j in 0..3:
        n = n + 1;
println(n);`, "16\n"},

	{"loop in function", `
func sum_to(n: int) -> int:
    t: int = 0;
    for i in 1..n:
        t = t + i;
    return t;
println(sum_to(100));`, "5050\n"},

	{"recursion", `
func fib(n: int) -> int:
    if n < 2:
        return n;
    return fib(n - 1) + fib(n - 2);
println(fib(15));`, "610\n"},

	{"default and keyword arguments", `
func greet(name: string, greeting: string = "Hello", punct: string = "!") -> string:
    return greeting + ", " + name + punct;
println(greet("Ann"));
println(greet("Bob", punct="?"));
println(greet("Cy", greeting="Yo", punct="."));`, "Hello, Ann!\nHello, Bob?\nYo, Cy.\n"},

	{"call before definition", `
println(twice(21));
func twice(n: int) -> int:
    return n * 2;`, "42\n"},

	{"lists and for-in", `
xs: list[int] = [1, 2, 3];
push(xs, 4);
sum: int = 0;
for x in xs:
    sum = sum + x;
println(sum);
println(len(xs));
println(pop(xs));`, "10\n4\n4\n"},

	{"maps", `
m: map[string, int] = {"a": 1, "b": 2};
m["c"] = 3;
t: int = 0;
for k in keys(m):
    t = t + m[k];
println(t);
println(len(m));`, "6\n3\n"},

	{"for over string", `
n: int = 0;
for ch in "héllo":
    n = n + 1;
println(n);`, "5\n"},

	{"structs with defaults", `
struct Server:
    ip: string;
    port: int = 80;
    online: bool = true;
a := Server("1.2.3.4");
b := Server("5.6.7.8", 8080);
println(a.port);
println(b.port);
a.port = 9000;
println(a.port);
println(a.online);`, "80\n8080\n9000\ntrue\n"},

	{"error capture", `
func risky(n: int) -> int:
    if n == 0:
        err("division by zero forbidden", code=42);
        return 0;
    return 100 / n;
e: error | null = null;
println(risky(5));
risky(0) ?>>> e;
if e != null:
    println(e.msg);
    println(e.code);`, "20\ndivision by zero forbidden\n42\n"},

	{"runtime error is catchable", `
d: int = 0;
e: error | null = null;
println(10 / d) ?>>> e;
if e != null:
    println("caught");`, "caught\n"},

	{"match", `
func name(n: int) -> string:
    match n:
        case 1:
            return "one";
        case 2:
            return "two";
        else:
            return "many";
println(name(1));
println(name(2));
println(name(9));`, "one\ntwo\nmany\n"},

	{"lambda and pipe", `
inc := |x: int| x + 1;
println(4 |> inc);`, "5\n"},

	{"capture stream", `
captured: string | null = null;
print("hello") >>> captured;
if captured != null:
    println(captured);`, "hello\nhello\n"},

	{"null narrowing", `
func f(s: string | null) -> int:
    if s == null:
        return -1;
    return len(s);
println(f(null));
println(f("abcd"));`, "-1\n4\n"},

	{"globals visible in hoisted functions", `
limit: int = 10;
func over(n: int) -> bool:
    return n > limit;
println(over(11));
println(over(3));`, "true\nfalse\n"},

	{"bool logic", `
a: bool = true;
b: bool = false;
println(a and not b);
println(a or b);
println(not a);`, "true\ntrue\nfalse\n"},

	{"string builtins", `
use std.math;
println(str_upper("abc"));
println(len("héllo"));
println(math.floor(2.7));`, "ABC\n5\n2\n"},

	{"module member does not shadow a module name", `
use std.log;
use std.math;
log.set_level("debug");
println("ok");`, "ok\n"},

	{"p01_casts", `
println(int("42"));
println(int(3.9));
println(float(7));
println(string(3) + "x");
println(bool(0));
`, "42\n3\n7.0\n3x\nfalse\n"},

	{"p02_str_funcs", `
println(str_contains("hello", "ell"));
println(str_starts_with("hello", "he"));
println(str_ends_with("hello", "lo"));
println(str_replace("a-b-c", "-", "+"));
println(len(str_split("a,b,c", ",")));
println(str_trim("  x  "));
println(str_lower("ABC") + str_upper("def"));
`, "true\ntrue\ntrue\na+b+c\n3\nx\nabcDEF\n"},

	{"p03_math", `
use std.math;
println(math.abs(-5));
println(math.floor(2.9));
println(math.ceil(2.1));
println(math.round(2.5));
println(math.sqrt(16.0));
println(math.pow(2.0, 3.0));
println(math.min(3, 7));
println(math.max(3, 7));
println(math.clamp(10, 0, 5));
println(math.hypot(3.0, 4.0));
println(math.PI > 3.14);
`, "5\n2\n3\n3\n4.0\n8.0\n3\n7\n5\n5.0\ntrue\n"},

	{"p04_str_index", `
s: string = "hello";
println(s[0]);
println(s[4]);
`, "h\no\n"},

	{"p06_map_key_order", `
m: map[string, int] = {"b": 2, "a": 1, "c": 3};
for k in keys(m):
    println(k);
`, "b\na\nc\n"},

	{"p09_typeof", `
println(typeof(1));
println(typeof(1.5));
println(typeof("s"));
println(typeof(true));
println(type(null));
`, "INTEGER\nFLOAT\nSTRING\nBOOL\nNULL\n"},

	{"p10_json", `
j := json_decode("{\"a\": [1, 2, 3]}");
println(json_query(j, "a[2]"));
println(json_query("{\"a\": [1, 2, 3]}", "a[1]"));
println(json_encode(j));
`, "3\n2\n{\"a\":[1,2,3]}\n"},

	{"p12_empty_containers", `
xs: list[int] = [];
println(len(xs));
push(xs, 7);
println(xs[0]);
m: map[string, int] = {};
m["k"] = 1;
println(len(m));
`, "0\n7\n1\n"},

	{"p13_nested_struct", `
struct Point:
    x: int;
    y: int;
struct Line:
    a: Point;
    b: Point;
l := Line(Point(1, 2), Point(3, 4));
println(l.a.x + l.b.y);
`, "5\n"},

	{"p14_list_of_structs", `
struct P:
    v: int;
ps: list[P] = [P(1), P(2), P(3)];
t: int = 0;
for p in ps:
    t = t + p.v;
println(t);
`, "6\n"},

	{"p15_struct_nullable_field", `
struct User:
    name: string;
    email: string | null = null;
u := User("Bob");
println(u.email == null);
u.email = "bob@x.io";
println(u.email);
`, "true\nbob@x.io\n"},

	{"p16_assert", `
assert(1 + 1 == 2);
println("ok");
`, "ok\n"},

	{"p17_json_prettify", `
println(json_prettify({"a": [2, 3]}));
`, "{a: [2, 3]}\n"},

	{"p18_div_zero_capture", `
e: error | null = null;
x: int = 0;
println(10 / x) ?>>> e;
if e != null:
    println(e.code);
    println(e.msg);
`, "400\ndivision by zero\n"},

	{"p19_float_compare", `
println(1 == 1.0);
println(2.5 > 2);
println(float(3) <= 3.0);
println(0.1 + 0.2 > 0.3);
`, "true\ntrue\ntrue\ntrue\n"},

	{"p21_crypto", `
use std.crypto;
println(crypto.md5("abc"));
println(crypto.sha1("abc"));
println(crypto.base64_encode("hello"));
println(crypto.base64_decode("aGVsbG8="));
println(crypto.hex_encode("AB"));
println(crypto.hmac_sha256("key", "msg"));
`, "900150983cd24fb0d6963f7d28e17f72\na9993e364706816aba3e25717850c26c9cd0d89d\naGVsbG8=\nhello\n4142\n2d93cbc1be167bcb1637a4a23cbff01a7878f0c50ee833954ea5221bb1b8c628\n"},

	{"p23_path_parts", `
use std.path;
println(path.base("a/b/c.txt"));
println(path.ext("a/b/c.txt"));
println(path.stem("a/b/c.txt"));
`, "c.txt\n.txt\nc\n"},

	{"p24_lambda_capture", `
base: int = 10;
f := |x: int| x + base;
println(4 |> f);
`, "14\n"},

	{"p25_scope_assign", `
x: int = 1;
if true:
    x = 2;
println(x);
`, "2\n"},

	{"p26_string_compare", `
println("abc" < "abd");
println("abc" == "abc");
println("b" > "a");
println("a" != "b");
`, "true\ntrue\ntrue\ntrue\n"},

	{"p27_short_circuit", `
func boom() -> bool:
    err("must not run", code=1);
    return false;
println(false and boom());
println(true or boom());
`, "false\ntrue\n"},

	{"p28_match_strings", `
for s in ["alpha", "beta", "x"]:
    match s:
        case "alpha":
            println(1);
        case "beta":
            println(2);
        else:
            println(0);
`, "1\n2\n0\n"},

	{"p31_mutual_recursion", `
func is_even(n: int) -> bool:
    if n == 0:
        return true;
    return is_odd(n - 1);
func is_odd(n: int) -> bool:
    if n == 0:
        return false;
    return is_even(n - 1);
println(is_even(10));
println(is_odd(7));
`, "true\ntrue\n"},

	{"p32_int_overflow", `
x: int = 9223372036854775807;
println(x + 1);
`, "-9223372036854775808\n"},

	{"p33_nested_func", `
func outer() -> int:
    func inner(x: int) -> int:
        return x * 2;
    return inner(21);
println(outer());
`, "42\n"},

	{"p35_membership", `
xs: list[int] = [1, 2, 3];
println(2 in xs);
println(7 in xs);
`, "true\nfalse\n"},

	{"p36_while_break", `
i: int = 0;
while true:
    i = i + 1;
    if i == 5:
        break;
println(i);
`, "5\n"},

	{"p37_nested_break", `
n: int = 0;
for i in 0..5:
    for j in 0..5:
        if j == 2:
            break;
        n = n + 1;
println(n);
`, "12\n"},

	{"p38_func_as_value", `
func double(x: int) -> int:
    return x * 2;
f := double;
println(f(21));
`, "42\n"},

	{"p39_lambda_two_params", `
add := |a: int, b: int| a + b;
println(add(2, 3));
`, "5\n"},

	{"p40_lambda_pipe_chain", `
inc := |x: int| x + 1;
dbl := |x: int| x * 2;
println(3 |> inc |> dbl);
`, "8\n"},

	{"p41_string_escapes", `
println("a\tb");
println("line1\nline2");
println("quote:\"end\"");
println(len("\\"));
`, "a\tb\nline1\nline2\nquote:\"end\"\n1\n"},

	{"p42_list_concat", `
a: list[int] = [1, 2];
b: list[int] = [3];
println(len(a + b));
`, "3\n"},

	{"p43_map_values", `
m: map[string, int] = {"b": 2, "a": 1};
t: int = 0;
for v in values(m):
    t = t + v;
println(t);
`, "3\n"},

	{"p44_time_deterministic", `
use std.time;
println(time.now() > 0);
println(time.now_ms() > 0);
`, "true\ntrue\n"},

	{"p46_sleep_zero", `
println(sleep(0));
`, "null\n"},

	{"p47_uuid_len", `
use std.crypto;
u := crypto.uuid();
println(len(u));
`, "36\n"},

	{"p48_str_index_loop", `
s: string = "abc";
t: string = "";
for ch in s:
    t = ch + t;
println(t);
`, "cba\n"},

	{"p50_int_float_arg", `
use std.math;
println(math.floor(2.999999));
println(math.round(-2.5));
println(math.abs(-2.5));
`, "2\n-3\n2.5\n"},

	{"q01_list_oob", `
xs: list[int] = [1, 2, 3];
e: error | null = null;
println(xs[5]) ?>>> e;
if e != null:
    println(e.code);
    println(e.msg);
`, "400\nindex 5 out of bounds (len 3)\n"},

	{"q02_list_negative_index", `
xs: list[int] = [1, 2, 3];
println(xs[-1]);
println(xs[-3]);
`, "3\n1\n"},

	{"q03_str_negative_index", `
s: string = "hello";
println(s[-1]);
println(s[-5]);
`, "o\nh\n"},

	{"q04_str_oob", `
s: string = "hi";
e: error | null = null;
println(s[9]) ?>>> e;
if e != null:
    println(e.code);
    println(e.msg);
`, "400\nstring index 9 out of bounds\n"},

	{"q05_set_oob", `
func bad() -> int:
    xs: list[int] = [1, 2, 3];
    xs[5] = 9;
    return 0;
e: error | null = null;
println(bad()) ?>>> e;
if e != null:
    println(e.code);
    println(e.msg);
`, "400\ninvalid list index for assignment\n"},

	{"q06_map_missing_key", `
m: map[string, int] = {"a": 1};
println(m["b"]);
`, "null\n"},

	{"q09_deep_recursion", `
func depth(n: int) -> int:
    if n == 0:
        return 0;
    return 1 + depth(n - 1);
println(depth(500));
`, "500\n"},

	{"q12_range_descending", `
for i in 5..1:
    println(i);
`, "5\n4\n3\n2\n1\n"},

	{"q13_range_negative", `
for i in -3..0:
    println(i);
`, "-3\n-2\n-1\n0\n"},

	{"q14_range_single_int", `
for i in 3:
    println(i);
`, "0\n1\n2\n"},

	{"q16_float_format", `
println(1.0 / 3.0);
println(2.0);
println(-0.0);
println(100000000000000000000.0);
`, "0.3333333333333333\n2.0\n-0.0\n100000000000000000000.0\n"},

	{"q17_int_mod_negative", `
println(-7 % 3);
println(7 % -3);
println(-7 % -3);
`, "-1\n1\n-1\n"},

	{"q18_bool_compare", `
println(true == true);
println(true != false);
`, "true\ntrue\n"},

	{"q19_null_equality", `
x: string | null = null;
println(x == null);
`, "true\n"},

	{"q20_map_values_order", `
m: map[string, int] = {"z": 26, "y": 25, "x": 24};
for v in values(m):
    println(v);
`, "26\n25\n24\n"},

	{"q21_index_json_dynamic", `
j := json_decode("[10, 20, 30]");
println(j[1]);
`, "20\n"},

	{"q23_pow_negative_int", `
e: error | null = null;
println(2 ** -1) ?>>> e;
if e != null:
    println(e.code);
    println(e.msg);
`, "400\nnegative exponent for integer power; use float operands\n"},

	{"q24_comment_only_block_bodies", `
x := 1;
if x == 1:
    // consequence is a comment only
else:
    println("no");
elif_chain := 0;
if x == 2:
    // first arm empty
elif x == 1:
    // second arm empty
else:
    println("no");
match x:
    case 1:
        // case body empty
    else:
        println("no");
println("done");
`, "done\n"},

	{"q25_closure_counter", `
func make_counter() -> func:
    n := 0;
    func next() -> int:
        n = n + 1;
        return n;
    return next;

c := make_counter();
println(c());
println(c());
println(c());
`, "1\n2\n3\n"},

	{"q26_closure_shared_state", `
func make() -> map[string, func]:
    x := 0;
    func inc() -> int:
        x = x + 1;
        return x;
    func get() -> int:
        return x;
    return {"inc": inc, "get": get};

pair := make();
inc := pair["inc"];
get := pair["get"];
println(inc());
println(get());
println(inc());
println(get());
`, "1\n1\n2\n2\n"},

	{"q27_closure_independent_instances", `
func make_counter() -> func:
    n := 0;
    func next() -> int:
        n = n + 1;
        return n;
    return next;

a := make_counter();
b := make_counter();
println(a());
println(a());
println(b());
println(a());
`, "1\n2\n1\n3\n"},

	{"q28_lambda_captures_parameter", `
func make_adder(n: int) -> func:
    return |x: int| x + n;

add5 := make_adder(5);
add9 := make_adder(9);
println(add5(1));
println(add9(1));
`, "6\n10\n"},

	{"q29_loop_variable_capture", `
func probe_loop_capture() -> void:
    a := |x: int| x;
    b := |x: int| x;
    for i in 0..2:
        if i == 0:
            a = |x: int| x + i;
        else:
            b = |x: int| x + i;
    println(a(0));
    println(b(0));

probe_loop_capture();
`, "2\n2\n"},

	{"q30_void_function_ends_with_statement", `
func log(msg: string) -> void:
    println(msg);

func counter() -> void:
    n := 1;

log("hi");
counter();
println("done");
`, "hi\ndone\n"},

	{"q31_three_level_capture", `
func outer() -> func:
    x := 10;
    func mid() -> func:
        func inner() -> int:
            return x;
        return inner;
    return mid;

m := outer();
i := m();
println(i());
`, "10\n"},

	{"q32_closure_calls_closure", `
func make() -> list:
    x := 0;
    func inc() -> int:
        x = x + 1;
        return x;
    func via() -> int:
        return inc() * 10;
    return [inc, via];
pair := make();
i := pair[0];
v := pair[1];
println(i());
println(v());
println(i());
`, "1\n20\n3\n"},
}

func TestEnginesAgree(t *testing.T) {
	for _, tc := range corpus {
		t.Run(tc.name, func(t *testing.T) {
			for _, eng := range []struct {
				name string
				e    engine
			}{{"vm", engineVM}, {"interp", engineInterp}} {
				got, code := runCaptured(t, tc.src, eng.e)
				if code != 0 {
					t.Errorf("[%s] exit code %d, output:\n%s", eng.name, code, got)
					continue
				}
				if got != tc.want {
					t.Errorf("[%s] output mismatch\n got: %q\nwant: %q", eng.name, got, tc.want)
				}
			}
		})
	}
}

func TestVMIsDefaultEngine(t *testing.T) {
	file := filepath.Join(t.TempDir(), "p.al")
	_ = os.WriteFile(file, []byte("println(1);\n"), 0o644)
	prog, content, ok := loadProgram(file)
	if !ok {
		t.Fatal("load failed")
	}
	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w
	_, usedVM := execute(file, content, prog, engineAuto, true)
	w.Close()
	os.Stdout = old
	if !usedVM {
		t.Fatal("auto engine must use the VM when the program is supported")
	}
}

func TestUnsupportedFeatureFallsBackToInterpreter(t *testing.T) {
	src := "threads:\n    println(\"x\");\n"
	file := filepath.Join(t.TempDir(), "p.al")
	_ = os.WriteFile(file, []byte(src), 0o644)
	prog, content, ok := loadProgram(file)
	if !ok {
		t.Skip("threads syntax differs")
	}
	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w
	_, usedVM := execute(file, content, prog, engineAuto, true)
	w.Close()
	os.Stdout = old
	if usedVM {
		t.Fatal("threads blocks are not VM-compilable; the interpreter must be used")
	}
}

// TestExplicitVMReportsUnsupportedFeature: with --vm the user asked for the
// bytecode backend, so an unsupported construct must be reported as such.
// Regression: execute() sent *vm.UnsupportedError through the native-build
// formatter, printing "compilation to native executable failed" and advice
// about the output path even though no executable was being built.
func TestExplicitVMReportsUnsupportedFeature(t *testing.T) {
	src := "threads:\n    println(\"x\");\n"
	file := filepath.Join(t.TempDir(), "p.al")
	_ = os.WriteFile(file, []byte(src), 0o644)
	prog, content, ok := loadProgram(file)
	if !ok {
		t.Skip("threads syntax differs")
	}

	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	done := make(chan string)
	go func() { b, _ := io.ReadAll(r); done <- string(b) }()
	code, usedVM := execute(file, content, prog, engineVM, true)
	w.Close()
	os.Stderr = old
	errOut := <-done

	if code == 0 {
		t.Fatalf("explicit --vm must fail on an unsupported program")
	}
	if usedVM {
		t.Fatalf("the VM must not claim to have run an uncompilable program")
	}
	if !strings.Contains(errOut, "UnsupportedFeature") {
		t.Fatalf("diagnostic must name the unsupported feature, got:\n%s", errOut)
	}
	if strings.Contains(errOut, "BuildError") || strings.Contains(errOut, "native executable") {
		t.Fatalf("diagnostic must not use the native-build template, got:\n%s", errOut)
	}
}

func TestStrictRunRejectsTypeErrors(t *testing.T) {
	file := filepath.Join(t.TempDir(), "bad.al")
	_ = os.WriteFile(file, []byte("a: int = \"x\";\nprintln(a);\n"), 0o644)
	if _, _, ok := loadProgram(file); ok {
		t.Fatal("`alca run` must refuse to execute a program with type errors")
	}
	if !strings.Contains(file, "bad") {
		t.Fatal("unreachable")
	}
}
