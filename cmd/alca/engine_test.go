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
