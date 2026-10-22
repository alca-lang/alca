package interpreter_test

import (
	"strings"
	"testing"

	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/lexer"
	"github.com/alca-lang/alca/pkg/limits"
	"github.com/alca-lang/alca/pkg/parser"
)

// runInterpreterSource parses and runs src on a fresh interpreter and returns
// the printed output and the runtime error.
func runInterpreterSource(t *testing.T, src string) (string, error) {
	t.Helper()
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if errs := p.SyntaxErrors(); len(errs) > 0 {
		t.Fatalf("syntax errors: %v", errs)
	}
	var out strings.Builder
	interpreter.SetPrintHook(func(s string) {
		out.WriteString(s)
		out.WriteByte('\n')
	})
	defer interpreter.ResetPrintHook()

	interp := interpreter.New()
	err := interp.Execute(prog)
	return out.String(), err
}

// TestOomGuardDecoratorLimitsMemory verifies that the @oom_guard decorator
// installs a limit on the shared meter and that the program is stopped with a
// regular Alca error when the limit is reached.
func TestOomGuardDecoratorLimitsMemory(t *testing.T) {
	const src = `
@oom_guard("max=1mb")
func grow() -> int:
    xs: list[int] = [];
    i := 0;
    while i < 1000000:
        push(xs, i);
        i = i + 1;
    return len(xs);

println(grow());
`
	limits.Reset()
	limits.SetLimits(0, 0)
	defer func() {
		limits.Reset()
		limits.SetLimits(0, 0)
	}()

	_, err := runInterpreterSource(t, src)
	if err == nil {
		t.Fatal("expected the memory limit to stop the program")
	}
	if !strings.Contains(err.Error(), "out of memory") {
		t.Fatalf("error = %v, want an out-of-memory error", err)
	}
	live, _ := limits.Limits()
	if live != 1<<20 {
		t.Fatalf("live limit = %d, want the limit of the decorator", live)
	}
}

// TestOomGuardDecoratorCannotRaiseHostLimit verifies that a program may
// tighten the limit of its host but not raise it. The operator of a program
// decides how much memory it may use.
func TestOomGuardDecoratorCannotRaiseHostLimit(t *testing.T) {
	const src = `
@oom_guard("max=64mb")
func grow() -> int:
    xs: list[int] = [];
    i := 0;
    while i < 1000000:
        push(xs, i);
        i = i + 1;
    return len(xs);

println(grow());
`
	limits.Reset()
	limits.SetLimits(1<<20, 0)
	defer func() {
		limits.Reset()
		limits.SetLimits(0, 0)
	}()

	_, err := runInterpreterSource(t, src)
	if err == nil {
		t.Fatal("expected the host limit to stop the program")
	}
	if !strings.Contains(err.Error(), "1048576") {
		t.Fatalf("error = %v, want the limit of the host", err)
	}
	live, _ := limits.Limits()
	if live != 1<<20 {
		t.Fatalf("live limit = %d, want the limit of the host", live)
	}
}

// TestOomGuardWithoutDecoratorRunsFreely verifies that a program without the
// decorator is not limited.
func TestOomGuardWithoutDecoratorRunsFreely(t *testing.T) {
	const src = `
func grow() -> int:
    xs: list[int] = [];
    i := 0;
    while i < 1000:
        push(xs, i);
        i = i + 1;
    return len(xs);

println(grow());
`
	limits.Reset()
	limits.SetLimits(0, 0)
	defer func() {
		limits.Reset()
		limits.SetLimits(0, 0)
	}()

	out, err := runInterpreterSource(t, src)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if out != "1000\n" {
		t.Fatalf("output = %q, want %q", out, "1000\n")
	}
}
