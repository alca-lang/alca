package vm

import (
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
    for i in 0..4:
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
for i in 0..4:
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
