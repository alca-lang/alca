package jit_test

import (
	"strings"
	"testing"

	"github.com/alca-lang/alca/pkg/jit"
	"github.com/alca-lang/alca/pkg/limits"
)

// TestMemoryGuardRaisesCatchableError verifies that the memory guard turns an
// exceeded limit into a regular Alca error. The program below grows a list
// until the limit is reached; the error must be catchable with the ?>>>
// operator, exactly like any other Alca error.
func TestMemoryGuardRaisesCatchableError(t *testing.T) {
	const src = `
func grow() -> int:
    xs: list[int] = [];
    i := 0;
    while i < 1000000:
        push(xs, i);
        i = i + 1;
    return len(xs);

failure: error | null = null;
grow() ?>>> failure;
if failure != null:
    println("caught: " + failure.msg);
else:
    println("no error");
`

	for _, mode := range []jit.StdlibExecutionMode{jit.StdlibVM, jit.StdlibThreadedJIT, jit.StdlibAMD64JIT} {
		mode := mode
		t.Run(mode.String(), func(t *testing.T) {
			limits.Reset()
			limits.SetLimits(8<<20, 0)
			defer func() {
				limits.Reset()
				limits.SetLimits(0, 0)
			}()

			out, err := runStdlibMode(t, src, mode)
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			if !strings.HasPrefix(out, "caught: out of memory") {
				t.Fatalf("output = %q, want a caught out-of-memory error", out)
			}
		})
	}
}

// TestMemoryGuardStopsUncaughtProgram verifies that a program that does not
// catch the limit error fails with the Alca error code 507 instead of taking
// the process down.
func TestMemoryGuardStopsUncaughtProgram(t *testing.T) {
	const src = `
func grow() -> int:
    xs: list[int] = [];
    i := 0;
    while i < 1000000:
        push(xs, i);
        i = i + 1;
    return len(xs);

println(grow());
`

	for _, mode := range []jit.StdlibExecutionMode{jit.StdlibInterpreter, jit.StdlibVM, jit.StdlibThreadedJIT, jit.StdlibAMD64JIT} {
		mode := mode
		t.Run(mode.String(), func(t *testing.T) {
			limits.Reset()
			limits.SetLimits(8<<20, 0)
			defer func() {
				limits.Reset()
				limits.SetLimits(0, 0)
			}()

			_, err := runStdlibMode(t, src, mode)
			if err == nil {
				t.Fatal("expected the memory guard to stop the program")
			}
			if !strings.Contains(err.Error(), "out of memory") {
				t.Fatalf("error = %v, want an out-of-memory error", err)
			}
		})
	}
}

// TestMemoryGuardAllocationBudget verifies that the allocation budget stops a
// string concatenation loop, which allocates quadratically and therefore grows
// much faster than the live memory of the program.
func TestMemoryGuardAllocationBudget(t *testing.T) {
	const src = `
s := "x";
i := 0;
while i < 100000:
    s = s + "0123456789";
    i = i + 1;
println(len(s));
`

	for _, mode := range []jit.StdlibExecutionMode{jit.StdlibVM, jit.StdlibThreadedJIT, jit.StdlibAMD64JIT} {
		mode := mode
		t.Run(mode.String(), func(t *testing.T) {
			limits.Reset()
			limits.SetLimits(0, 1<<20)
			defer func() {
				limits.Reset()
				limits.SetLimits(0, 0)
			}()

			_, err := runStdlibMode(t, src, mode)
			if err == nil {
				t.Fatal("expected the allocation budget to stop the program")
			}
			if !strings.Contains(err.Error(), "out of memory") {
				t.Fatalf("error = %v, want an out-of-memory error", err)
			}
		})
	}
}

// TestMemoryGuardDoesNotAffectNormalPrograms verifies that a program stays
// below the limit and runs to completion.
func TestMemoryGuardDoesNotAffectNormalPrograms(t *testing.T) {
	const src = `
use std.str;
func probe():
    items := [1, 2, 3];
    total := 0;
    for i in 0..2:
        total = total + items[i];
    println(str.upper("ok"));
    println(total);
probe();
`

	for _, mode := range []jit.StdlibExecutionMode{jit.StdlibVM, jit.StdlibThreadedJIT, jit.StdlibAMD64JIT} {
		mode := mode
		t.Run(mode.String(), func(t *testing.T) {
			limits.Reset()
			limits.SetLimits(8<<20, 0)
			defer func() {
				limits.Reset()
				limits.SetLimits(0, 0)
			}()

			out, err := runStdlibMode(t, src, mode)
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			if out != "OK\n6\n" {
				t.Fatalf("output = %q, want %q", out, "OK\n6\n")
			}
		})
	}
}
