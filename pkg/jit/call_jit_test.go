// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package jit_test

import (
	"strings"
	"testing"

	"github.com/alca-lang/alca/pkg/jit"
)

// TestUserCallsStayInJIT verifies that calls to compiled user functions run on
// the calling VM through the JIT call helper. Keeping the same frame stack is
// what makes recursion, globals, and captured upvalues behave like bytecode.
func TestUserCallsStayInJIT(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "plain calls and global mutation",
			src: `
g := 0;
func inc() -> int:
    g = g + 1;
    return g;
func probe():
    println(inc());
    println(inc());
    println(inc());
probe();
`,
			want: "1\n2\n3\n",
		},
		{
			name: "recursive calls",
			src: `
func fact(n: int) -> int:
    if n <= 1:
        return 1;
    return n * fact(n - 1);
func probe():
    println(fact(6));
probe();
`,
			want: "720\n",
		},
		{
			name: "nested calls",
			src: `
func add(a: int, b: int) -> int:
    return a + b;
func twice(x: int) -> int:
    return add(x, x);
func probe():
    println(twice(add(2, 3)));
probe();
`,
			want: "10\n",
		},
		{
			name: "captured closure mutation",
			src: `
func make_counter() -> func:
    n := 0;
    func next() -> int:
        n = n + 1;
        return n;
    return next;
func probe():
    c := make_counter();
    println(c());
    println(c());
    println(c());
probe();
`,
			want: "1\n2\n3\n",
		},
		{
			name: "sibling closures share captured state",
			src: `
func make() -> map[string, func]:
    x := 0;
    func inc() -> int:
        x = x + 1;
        return x;
    func get() -> int:
        return x;
    return {"inc": inc, "get": get};
func probe():
    pair := make();
    inc := pair["inc"];
    get := pair["get"];
    println(inc());
    println(get());
    println(inc());
    println(get());
probe();
`,
			want: "1\n1\n2\n2\n",
		},
		{
			name: "error capture around a user call",
			src: `
func fail():
    err("boom", 418);
func probe():
    failure: error | null = null;
    fail() ?>>> failure;
    if failure != null:
        println(failure.msg);
        println(failure.code);
probe();
`,
			want: "boom\n418\n",
		},
		{
			name: "callee catches its own error",
			src: `
func safe() -> int:
    failure: error | null = null;
    err("inner", 500) ?>>> failure;
    if failure != null:
        return 7;
    return 0;
func probe():
    println(safe());
    println("after");
probe();
`,
			want: "7\nafter\n",
		},
		{
			name: "function values and lambdas",
			src: `
func apply(f: func, x: int) -> int:
    return f(x);
func probe():
    sq := |x: int| x * x;
    println(apply(sq, 5));
    println(apply(|x: int| x + 1, 9));
probe();
`,
			want: "25\n10\n",
		},
		{
			name: "top-level closure capture",
			src: `
n := 0;
func next() -> int:
    n = n + 1;
    return n;
println(next());
println(next());
`,
			want: "1\n2\n",
		},
		{
			name: "open upvalue mutation inside factory",
			src: `
func make() -> int:
    n := 0;
    func inc() -> int:
        n = n + 1;
        return n;
    x := inc();
    y := inc();
    return x * 10 + y;
func probe():
    println(make());
probe();
`,
			want: "12\n",
		},
		{
			name: "closure called while factory frame is active",
			src: `
func apply(f: func) -> int:
    return f();
func make() -> int:
    n := 5;
    func get() -> int:
        return n;
    return apply(get);
func probe():
    println(make());
probe();
`,
			want: "5\n",
		},
	}

	for _, tc := range cases {
		tc := tc
		for _, mode := range []jit.StdlibExecutionMode{jit.StdlibThreadedJIT, jit.StdlibAMD64JIT} {
			mode := mode
			t.Run(tc.name+"/"+mode.String(), func(t *testing.T) {
				out, err, helperCalls := runPureStdlibMode(t, tc.src, mode)
				if err != nil {
					t.Fatalf("run: %v", err)
				}
				if out != tc.want {
					t.Fatalf("output = %q, want %q", out, tc.want)
				}
				if helperCalls == 0 {
					t.Fatal("user calls did not use the JIT helper tier")
				}
			})
		}
	}
}

// TestUserCallArityMismatchDeoptimizes checks that an arity error is still
// produced by the VM. The JIT helper refuses the call before touching the
// stack, so the VM reproduces the exact message and timing.
func TestUserCallArityMismatchDeoptimizes(t *testing.T) {
	const src = `
func add(a: int, b: int) -> int:
    return a + b;
func probe():
    println(add(1));
probe();
`
	for _, mode := range []jit.StdlibExecutionMode{jit.StdlibThreadedJIT, jit.StdlibAMD64JIT} {
		mode := mode
		t.Run(mode.String(), func(t *testing.T) {
			_, err, _ := runPureStdlibMode(t, src, mode)
			if err == nil {
				t.Fatal("expected an arity error")
			}
			if !strings.Contains(err.Error(), "wrong number of arguments") {
				t.Fatalf("error = %v, want wrong number of arguments", err)
			}
		})
	}
}
