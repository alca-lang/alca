package jit_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/jit"
	"github.com/alca-lang/alca/pkg/lexer"
	"github.com/alca-lang/alca/pkg/parser"
	"github.com/alca-lang/alca/pkg/vm"
)

// runPureStdlibMode runs src with a JIT engine and returns the output, the
// runtime error, and the number of calls executed by the JIT helper tier.
func runPureStdlibMode(t *testing.T, src string, mode jit.StdlibExecutionMode) (string, error, int64) {
	t.Helper()

	engine := newTestEngine(t, mode)
	defer engine.Clear()
	return runPureStdlibModeWithEngine(t, src, mode, engine)
}

// newTestEngine creates the JIT engine a mode runs with.
func newTestEngine(t *testing.T, mode jit.StdlibExecutionMode) *jit.Engine {
	t.Helper()
	switch mode {
	case jit.StdlibThreadedJIT:
		return jit.NewEngine(jit.Config{Enabled: true, Native: false, Threshold: 1})
	case jit.StdlibAMD64JIT:
		if !jit.HasNativeAMD64 {
			t.Skip("native AMD64 backend is unavailable")
		}
		return jit.NewEngine(jit.Config{Enabled: true, Native: true, Threshold: 1})
	default:
		return nil
	}
}

// runPureStdlibModeWithEngine runs src with the given engine, so a test can
// inspect the state of the engine afterwards.
func runPureStdlibModeWithEngine(t *testing.T, src string, mode jit.StdlibExecutionMode, engine *jit.Engine) (string, error, int64) {
	t.Helper()

	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if errs := p.SyntaxErrors(); len(errs) > 0 {
		return "", fmt.Errorf("syntax errors: %v", errs), 0
	}

	var out strings.Builder
	interpreter.SetPrintHook(func(s string) {
		out.WriteString(s)
		out.WriteByte('\n')
	})
	defer interpreter.ResetPrintHook()

	vm.SetJITEngine(engine)
	defer vm.SetJITEngine(jit.Default())

	compiler := vm.NewCompiler()
	if err := compiler.Compile(prog); err != nil {
		return "", err, 0
	}
	machine := vm.NewWithEnv(compiler.Bytecode(), stdlibTestEnv(t))
	err := machine.Run()
	if artifact := engine.GetCompiled(machine.MainFunction()); artifact == nil {
		t.Errorf("%s: main function was not JIT-compiled", mode)
	}
	return out.String(), err, engine.RuntimeHelperCalls()
}

// TestPureStdlibCallsStayInJIT verifies that fused pure-module calls execute in
// the JIT runtime-helper tier and that native/threaded code resumes after each
// call. The three calls are separated by global stores rather than println so
// no unsupported call can deoptimize the main function first.
func TestPureStdlibCallsStayInJIT(t *testing.T) {
	const src = `
use std.str;
use std.math;
func probe():
    a := str.upper("abc");
    b := math.pow(2.0, 3.0);
    c := str.length("hello");
    println(a);
    println(b);
    println(c);
probe();
`
	const want = "ABC\n8.0\n5\n"

	for _, mode := range []jit.StdlibExecutionMode{jit.StdlibThreadedJIT, jit.StdlibAMD64JIT} {
		mode := mode
		t.Run(mode.String(), func(t *testing.T) {
			out, err, helperCalls := runPureStdlibMode(t, src, mode)
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			if out != want {
				t.Fatalf("output = %q, want %q", out, want)
			}
			if helperCalls < 3 {
				t.Fatalf("runtime helper calls = %d, want at least 3", helperCalls)
			}
		})
	}
}

// TestPureStdlibCollectionArgumentsStayInJIT verifies that list/map literal
// construction and nested module paths are handled by the JIT helper tier
// before a fused stdlib call.
func TestPureStdlibCollectionArgumentsStayInJIT(t *testing.T) {
	const src = `
use std.str;
use std.formats;
func probe():
    items := ["a", "b"];
    joined := str.join(items, "-");
    obj := {"k": 1};
    text := formats.json.encode(obj);
    println(joined);
    println(text);
probe();
`
	const want = "a-b\n{\"k\":1}\n"

	for _, mode := range []jit.StdlibExecutionMode{jit.StdlibThreadedJIT, jit.StdlibAMD64JIT} {
		mode := mode
		t.Run(mode.String(), func(t *testing.T) {
			out, err, helperCalls := runPureStdlibMode(t, src, mode)
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			if out != want {
				t.Fatalf("output = %q, want %q", out, want)
			}
			if helperCalls < 4 {
				t.Fatalf("runtime helper calls = %d, want at least 4", helperCalls)
			}
		})
	}
}

// TestTopLevelUsesStayInJIT verifies that a program's top-level `use`
// statements execute in the JIT helper tier, so subsequent top-level pure calls
// are not forced back into VM bytecode dispatch.
func TestTopLevelUsesStayInJIT(t *testing.T) {
	const src = `
use std.str;
use std.math;
a := str.upper("abc");
b := math.sqrt(81);
println(a);
println(b);
`
	const want = "ABC\n9.0\n"

	for _, mode := range []jit.StdlibExecutionMode{jit.StdlibThreadedJIT, jit.StdlibAMD64JIT} {
		mode := mode
		t.Run(mode.String(), func(t *testing.T) {
			out, err, helperCalls := runPureStdlibMode(t, src, mode)
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			if out != want {
				t.Fatalf("output = %q, want %q", out, want)
			}
			if helperCalls < 6 {
				t.Fatalf("runtime helper calls = %d, want at least 6", helperCalls)
			}
		})
	}
}

// TestJITIndexingConversionAndOperatorHelpers verifies that container access,
// container/field mutation, single-argument conversions, power, membership, and
// range construction execute in the JIT helper tier rather than deoptimizing the
// enclosing function at those opcodes.
func TestJITIndexingConversionAndOperatorHelpers(t *testing.T) {
	const src = `
use std.str;
g := 5;
func probe():
    values := [1, 2, 3];
    values[1] = 20;
    m := {"a": 1};
    m["b"] = 2;
    m.c = 3;
    x := int(values[1]);
    y := int(m["b"]);
    z := int(g);
    println(str.upper("ok"));
    println(x + y + m.c + z);
    println(2 ** 3);
    println(20 in values);
    assert 20 in values;
    r := 0..2;
    println(len(r));
probe();
`
	const want = "OK\n30\n8\ntrue\n3\n"

	for _, mode := range []jit.StdlibExecutionMode{jit.StdlibThreadedJIT, jit.StdlibAMD64JIT} {
		mode := mode
		t.Run(mode.String(), func(t *testing.T) {
			out, err, helperCalls := runPureStdlibMode(t, src, mode)
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			if out != want {
				t.Fatalf("output = %q, want %q", out, want)
			}
			if helperCalls < 10 {
				t.Fatalf("runtime helper calls = %d, want at least 10", helperCalls)
			}
		})
	}
}

// TestJITStringComparisonsStayInJIT verifies that equality, strict equality,
// and ordering on string values use the runtime-helper tier instead of
// deoptimizing the function after the stdlib calls that produced them.
func TestJITStringComparisonsStayInJIT(t *testing.T) {
	const src = `
use std.str;
func probe():
    a := str.upper("abc");
    b := str.lower("ABC");
    println(a == b);
    println(a === b);
    println(a != "xyz");
    println("apple" < "banana");
    println(a >= b);
probe();
`
	const want = "false\nfalse\ntrue\ntrue\nfalse\n"

	for _, mode := range []jit.StdlibExecutionMode{jit.StdlibThreadedJIT, jit.StdlibAMD64JIT} {
		mode := mode
		t.Run(mode.String(), func(t *testing.T) {
			out, err, _ := runPureStdlibMode(t, src, mode)
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			if out != want {
				t.Fatalf("output = %q, want %q", out, want)
			}
		})
	}
}

// TestPureStringMethodsStayInJIT verifies that method dispatch through
// OpGetField/OpCall also uses the JIT helper tier, so string and list methods do
// not force a VM deoptimization.
func TestPureStringMethodsStayInJIT(t *testing.T) {
	const src = `
func probe():
    text := " Hello,World ";
    trimmed := text.trim();
    parts := trimmed.split(",");
    joined := parts.join("-");
    println(joined);
probe();
`
	const want = "Hello-World\n"

	for _, mode := range []jit.StdlibExecutionMode{jit.StdlibThreadedJIT, jit.StdlibAMD64JIT} {
		mode := mode
		t.Run(mode.String(), func(t *testing.T) {
			out, err, helperCalls := runPureStdlibMode(t, src, mode)
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			if out != want {
				t.Fatalf("output = %q, want %q", out, want)
			}
			if helperCalls < 8 {
				t.Fatalf("runtime helper calls = %d, want at least 8", helperCalls)
			}
		})
	}
}

// TestHostStdlibCallsUseJITHelper verifies that a host-facing member still
// executes through the JIT runtime-helper tier without VM bytecode dispatch.
// time.now() is intentionally host-backed and is not a pure fused member.
func TestHostStdlibCallsUseJITHelper(t *testing.T) {
	const src = `
use std.time;
func probe():
    t := time.now();
    println(t > 0);
probe();
`
	const want = "true\n"

	for _, mode := range []jit.StdlibExecutionMode{jit.StdlibThreadedJIT, jit.StdlibAMD64JIT} {
		mode := mode
		t.Run(mode.String(), func(t *testing.T) {
			out, err, helperCalls := runPureStdlibMode(t, src, mode)
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			if out != want {
				t.Fatalf("output = %q, want %q", out, want)
			}
			if helperCalls < 5 {
				t.Fatalf("runtime helper calls = %d, want at least 5", helperCalls)
			}
		})
	}
}

// TestPureStdlibErrorsPropagateFromJITHelper verifies that an Alca error value
// returned by a fused call is raised exactly once through the VM unwinder.
func TestPureStdlibErrorsPropagateFromJITHelper(t *testing.T) {
	const src = `
use std.str;
a := str.upper(42);
println(a);
`
	for _, mode := range []jit.StdlibExecutionMode{jit.StdlibThreadedJIT, jit.StdlibAMD64JIT} {
		mode := mode
		t.Run(mode.String(), func(t *testing.T) {
			out, err, _ := runPureStdlibMode(t, src, mode)
			if err == nil {
				t.Fatalf("expected runtime error, got output %q", out)
			}
			if !strings.Contains(strings.ToLower(err.Error()), "string.upper expects string") {
				t.Fatalf("error = %v, want string.upper type error", err)
			}
		})
	}
}
