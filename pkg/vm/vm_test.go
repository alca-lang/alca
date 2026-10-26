// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package vm

import (
	"runtime"
	"strings"
	"testing"

	"github.com/alca-lang/alca/pkg/ast"
	"github.com/alca-lang/alca/pkg/diag"
	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/lexer"
	"github.com/alca-lang/alca/pkg/parser"
)

// runSource compiles and runs src on a fresh VM, returning the runtime error.
func runSource(t *testing.T, src string) error {
	t.Helper()
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if errs := p.SyntaxErrors(); len(errs) > 0 {
		t.Fatalf("syntax errors: %v", errs)
	}
	c := NewCompiler()
	if err := c.Compile(prog); err != nil {
		t.Fatalf("compile: %v", err)
	}
	machine := NewWithEnv(c.Bytecode(), interpreter.New().Env())
	return machine.Run()
}

// TestCompilerAcceptsClasses: class programs compile to bytecode. The VM
// implements the same class semantics as the tree-walking interpreter, so no
// interpreter fallback is needed for them.
func TestCompilerAcceptsClasses(t *testing.T) {
	src := "class A:\n    public int: x = 1;\n"
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if errs := p.SyntaxErrors(); len(errs) > 0 {
		t.Fatalf("syntax errors: %v", errs)
	}
	if err := NewCompiler().Compile(prog); err != nil {
		t.Fatalf("class program must compile to bytecode, got %T: %v", err, err)
	}
}

// TestErrExpressionKeepsExplicitCode: err("msg", code=N) must raise an error
// whose code is N on the VM, matching the interpreter.
func TestErrExpressionKeepsExplicitCode(t *testing.T) {
	err := runSource(t, "err(\"boom\", code=7);\n")
	re, ok := err.(*RaisedError)
	if !ok {
		t.Fatalf("expected *RaisedError, got %T: %v", err, err)
	}
	if re.Val.Code != 7 {
		t.Fatalf("raised error code = %d, want 7 (message %q)", re.Val.Code, re.Val.Message)
	}
}

// TestRaisedErrorReportsItsOwnCodeInDiagnostics: formatting a VM runtime error
// must print the Alca error code the program used, not a fallback code.
// A regression here made `err("boom", code=7)` display "(code 400)" on the VM
// while the interpreter printed "(code 7)".
func TestRaisedErrorReportsItsOwnCodeInDiagnostics(t *testing.T) {
	err := runSource(t, "err(\"boom\", code=7);\n")
	if err == nil {
		t.Fatalf("expected a raised error")
	}
	report := diag.FormatRuntimeError("t.al", "err(\"boom\", code=7);\n", err, false)
	if !strings.Contains(report, "(code 7)") {
		t.Fatalf("diagnostic must report code 7, got:\n%s", report)
	}
	if strings.Contains(report, "(code 400)") {
		t.Fatalf("diagnostic must not use the fallback code 400, got:\n%s", report)
	}
}

func TestLooseAndStrictEqualityOnBothEngines(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{name: "JSON int and float loose", src: `dynamic: json = 1.0; println(dynamic == 1);`, want: "true"},
		{name: "int and float strict", src: `println(1 === 1.0);`, want: "false"},
		{name: "JSON string and int loose", src: `dynamic: json = "1"; println(dynamic == 1);`, want: "true"},
		{name: "string and int strict", src: `println("1" === 1);`, want: "false"},
		{name: "nested strict equality", src: `println([1] === [1.0]);`, want: "false"},
		{name: "same type strict equality", src: `println("alca" === "alca");`, want: "true"},
		{name: "strict inequality", src: `println(1 != 2);`, want: "true"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			interpreterOutput, vmOutput := runOnBothEngines(t, test.src)
			if interpreterOutput != test.want {
				t.Errorf("interpreter output = %q, want %q", interpreterOutput, test.want)
			}
			if vmOutput != test.want {
				t.Errorf("VM output = %q, want %q", vmOutput, test.want)
			}
		})
	}
}

// runOnBothEngines executes src on the tree-walking interpreter and on the VM
// and returns the text each printed. Both engines write through the
// interpreter's print hook, which (like the real println) appends no newline,
// so the comparison covers every printed value in order.
func runOnBothEngines(t *testing.T, src string) (interpOut, vmOut string) {
	t.Helper()

	var interp strings.Builder
	interpreter.SetPrintHook(func(s string) { interp.WriteString(s) })
	interpErr := interpreter.New().Execute(parseProgram(t, src))
	interpreter.ResetPrintHook()
	if interpErr != nil {
		t.Fatalf("interpreter: %v", interpErr)
	}

	var machine strings.Builder
	interpreter.SetPrintHook(func(s string) { machine.WriteString(s) })
	vmErr := runSource(t, src)
	interpreter.ResetPrintHook()
	if vmErr != nil {
		t.Fatalf("vm: %v", vmErr)
	}
	return interp.String(), machine.String()
}

// TestClosuresCaptureByReferenceOnBothEngines is the regression test for the
// silent closure miscompile: the VM used to copy captured values at closure
// creation, so a closure that mutated a variable saw a stale copy. Two
// independent counters even printed "1,2,3,4" because they shared one slot.
// Every program must print the same thing on both engines.
func TestClosuresCaptureByReferenceOnBothEngines(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{
			name: "counter mutates captured binding",
			src: `func make_counter() -> func:
    n := 0;
    func next() -> int:
        n = n + 1;
        return n;
    return next;

c := make_counter();
println(c());
println(c());
println(c());
`,
			want: "123",
		},
		{
			name: "independent instances have independent state",
			src: `func make_counter() -> func:
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
`,
			want: "1213",
		},
		{
			name: "sibling closures share state",
			src: `func make() -> map[string, func]:
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
`,
			want: "1122",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			interpOut, vmOut := runOnBothEngines(t, tc.src)
			if interpOut != tc.want {
				t.Errorf("interpreter output = %q, want %q", interpOut, tc.want)
			}
			if vmOut != tc.want {
				t.Errorf("vm output = %q, want %q", vmOut, tc.want)
			}
			if interpOut != vmOut {
				t.Errorf("engines disagree: interpreter %q, vm %q", interpOut, vmOut)
			}
		})
	}
}

// TestCounterLoopFinalValueMatchesInterpreter is the regression test for the
// counter-loop fast path: the VM incremented the loop variable before the exit
// test (leaving end+1) while the interpreter leaves the last iterated item, and
// a body write to the loop variable changed the VM's iteration count but not
// the interpreter's fixed item list.
func TestCounterLoopFinalValueMatchesInterpreter(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{
			name: "loop variable captured by closure keeps last item",
			src: `func probe() -> void:
    a := |x: int| x;
    for i in 0..=4:
        if i == 0:
            a = |x: int| x + i;
    println(a(0));

probe();
`,
			want: "4",
		},
		{
			name: "writing the loop variable does not change iteration count",
			src: `n := 0;
for i in 0..=4:
    n = n + 1;
    i = i + 1;
println(n);
`,
			want: "5",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			interpOut, vmOut := runOnBothEngines(t, tc.src)
			if interpOut != tc.want {
				t.Errorf("interpreter output = %q, want %q", interpOut, tc.want)
			}
			if vmOut != tc.want {
				t.Errorf("vm output = %q, want %q", vmOut, tc.want)
			}
			if interpOut != vmOut {
				t.Errorf("engines disagree: interpreter %q, vm %q", interpOut, vmOut)
			}
		})
	}
}

// TestFiveMillionIterationLoopDoesNotGrowMemory is the acceptance regression
// for long-running loops: five million iterations must complete without a
// panic, stack overflow, or unbounded heap growth. The loop deliberately keeps
// no collection alive, so a correctly reusing VM must return to roughly its
// starting heap size.
func TestFiveMillionIterationLoopDoesNotGrowMemory(t *testing.T) {
	if testing.Short() {
		t.Skip("5M-iteration stress test skipped in -short mode")
	}
	const src = `total := 0;
for i in 0..5000000:
    total = total + 1;
`
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	if err := runSource(t, src); err != nil {
		t.Fatalf("5M-iteration loop failed: %v", err)
	}

	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	const limit = 64 << 20
	if after.HeapAlloc > before.HeapAlloc {
		growth := after.HeapAlloc - before.HeapAlloc
		if growth > limit {
			t.Fatalf("heap grew by %.1f MiB after 5M iterations (limit %d MiB)", float64(growth)/(1<<20), limit>>20)
		}
	}
}

// parseProgram parses src for the interpreter side of a comparison.
func parseProgram(t *testing.T, src string) *ast.Program {
	t.Helper()
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if errs := p.SyntaxErrors(); len(errs) > 0 {
		t.Fatalf("syntax errors: %v", errs)
	}
	return prog
}

func TestMultilineAndFStringsExecution(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "multiline triple double quote",
			src: `s := """hello
world""";
println(s);
`,
			want: "hello\nworld",
		},
		{
			name: "multiline raw string verbatim backslashes",
			src: `s := r"""line1\nnot newline""";
println(s);
`,
			want: `line1\nnot newline`,
		},
		{
			name: "fstring simple interpolation",
			src: `name := "Alice";
println(f"Hello, {name}!");
`,
			want: "Hello, Alice!",
		},
		{
			name: "fstring arithmetic interpolation",
			src: `x := 15;
y := 27;
println(f"{x} + {y} = {x + y}");
`,
			want: "15 + 27 = 42",
		},
		{
			name: "fstring escaped braces",
			src: `val := "ok";
println(f"{{start}} {val} {{end}}");
`,
			want: "{start} ok {end}",
		},
		{
			name: "multiline fstring",
			src: `item := "widget";
count := 3;
println(f"""Order summary:
- {item} x {count}""");
`,
			want: "Order summary:\n- widget x 3",
		},
		{
			name: "raw fstring rf",
			src: `score := 100;
println(rf"Points: {score}\n[raw]");
`,
			want: "Points: 100\\n[raw]",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			interpOut, vmOut := runOnBothEngines(t, tc.src)
			if interpOut != tc.want {
				t.Errorf("interpreter output = %q, want %q", interpOut, tc.want)
			}
			if vmOut != tc.want {
				t.Errorf("vm output = %q, want %q", vmOut, tc.want)
			}
			if interpOut != vmOut {
				t.Errorf("engines disagree: interpreter %q, vm %q", interpOut, vmOut)
			}
		})
	}
}

func TestWaitStatementOnBothEngines(t *testing.T) {
	src := `x := 10;
wait;
println(x);
`
	interpOut, vmOut := runOnBothEngines(t, src)
	if interpOut != "10" || vmOut != "10" {
		t.Fatalf("expected 10 on both engines, got interp=%q, vm=%q", interpOut, vmOut)
	}
}
