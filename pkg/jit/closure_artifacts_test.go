package jit_test

import (
	"testing"

	"github.com/alca-lang/alca/pkg/jit"
)

// TestClosuresShareOneArtifact verifies that a program that creates many
// closures does not make the JIT engine compile the same code again and again.
// Every instance of a closure shares the compiled code of its template, while
// the captured values stay per instance.
func TestClosuresShareOneArtifact(t *testing.T) {
	const src = `
func make() -> func:
    n := 0;
    func next() -> int:
        n = n + 1;
        return n;
    return next;

func probe():
    i := 0;
    while i < 500:
        f := make();
        f();
        f();
        i = i + 1;
probe();
`

	for _, mode := range []jit.StdlibExecutionMode{jit.StdlibThreadedJIT, jit.StdlibAMD64JIT} {
		mode := mode
		t.Run(mode.String(), func(t *testing.T) {
			engine := jit.NewEngine(jit.Config{Enabled: true, Native: mode == jit.StdlibAMD64JIT, Threshold: 1})
			defer engine.Clear()
			out, err, _ := runPureStdlibModeWithEngine(t, src, mode, engine)
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			if out != "" {
				t.Fatalf("output = %q, want no output", out)
			}
			artifacts := engine.CompiledArtifacts()
			// The program has a handful of functions: the main function, make,
			// next and the probe helper. The 500 closures must not add 500
			// artifacts of their own.
			if artifacts > 8 {
				t.Fatalf("compiled artifacts = %d, want the closures to share one artifact", artifacts)
			}
		})
	}
}

// TestArtifactLimitKeepsProgramCorrect verifies that the bound on the number
// of compiled artifacts only costs speed: the functions that do not fit run in
// the bytecode VM and the program produces the same result.
func TestArtifactLimitKeepsProgramCorrect(t *testing.T) {
	const src = `
func one() -> int:
    return 1;
func two() -> int:
    return 2;
func three() -> int:
    return 3;
func probe():
    println(one() + two() + three());
probe();
`
	const want = "6\n"

	for _, mode := range []jit.StdlibExecutionMode{jit.StdlibThreadedJIT, jit.StdlibAMD64JIT} {
		mode := mode
		t.Run(mode.String(), func(t *testing.T) {
			engine := jit.NewEngine(jit.Config{
				Enabled:      true,
				Native:       mode == jit.StdlibAMD64JIT,
				Threshold:    1,
				MaxArtifacts: 1,
			})
			defer engine.Clear()
			out, err, _ := runPureStdlibModeWithEngine(t, src, mode, engine)
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			if out != want {
				t.Fatalf("output = %q, want %q", out, want)
			}
			if artifacts := engine.CompiledArtifacts(); artifacts > 1 {
				t.Fatalf("compiled artifacts = %d, want the limit to be respected", artifacts)
			}
		})
	}
}

// TestClosureInstancesKeepTheirOwnCaptures verifies that sharing the compiled
// code between the instances of a closure does not mix up their captured
// values.
func TestClosureInstancesKeepTheirOwnCaptures(t *testing.T) {
	const src = `
func make(start: int) -> func:
    n := start;
    func next() -> int:
        n = n + 1;
        return n;
    return next;

func probe():
    a := make(10);
    b := make(100);
    println(a());
    println(a());
    println(b());
    println(b());
probe();
`
	const want = "11\n12\n101\n102\n"

	for _, mode := range []jit.StdlibExecutionMode{jit.StdlibThreadedJIT, jit.StdlibAMD64JIT} {
		mode := mode
		t.Run(mode.String(), func(t *testing.T) {
			engine := jit.NewEngine(jit.Config{Enabled: true, Native: mode == jit.StdlibAMD64JIT, Threshold: 1})
			defer engine.Clear()
			out, err, _ := runPureStdlibModeWithEngine(t, src, mode, engine)
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			if out != want {
				t.Fatalf("output = %q, want %q", out, want)
			}
		})
	}
}
