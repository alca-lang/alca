package jit_test

import (
	"testing"

	"github.com/alca-lang/alca/pkg/jit"
)

// TestCompoundAssignmentAcrossEngines verifies that compound assignments
// (x = x + value) behave the same on every engine. The JIT tiers execute the
// int and float cases natively and deoptimize every other type. A
// deoptimization must resume with the original stack pointer, because the VM
// pops the operand and pushes the result for those cases.
func TestCompoundAssignmentAcrossEngines(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "global string",
			src: `
s: string = "x";
s = s + "abc";
println(s);
`,
			want: "xabc\n",
		},
		{
			name: "local string",
			src: `
func grow() -> string:
    s: string = "x";
    s = s + "abc";
    return s;
println(grow());
`,
			want: "xabc\n",
		},
		{
			name: "global float",
			src: `
f: float = 1.5;
f = f + 2.5;
println(f);
`,
			want: "4.0\n",
		},
		{
			name: "local float",
			src: `
func add() -> float:
    f: float = 1.5;
    f = f + 2.5;
    return f;
println(add());
`,
			want: "4.0\n",
		},
		{
			name: "mixed float and int",
			src: `
f: float = 1.5;
f = f + 1;
println(f);
`,
			want: "2.5\n",
		},
		{
			name: "global int",
			src: `
n: int = 1;
n = n + 41;
n = n - 1;
println(n);
`,
			want: "41\n",
		},
		{
			name: "local int",
			src: `
func bump() -> int:
    n: int = 1;
    n = n + 41;
    n = n - 1;
    return n;
println(bump());
`,
			want: "41\n",
		},
		{
			name: "string in a loop",
			src: `
func build() -> string:
    s: string = "";
    for i in 1..3:
        s = s + "x";
    return s;
println(build());
`,
			want: "xxx\n",
		},
	}

	for _, tc := range cases {
		tc := tc
		for _, mode := range []jit.StdlibExecutionMode{jit.StdlibThreadedJIT, jit.StdlibAMD64JIT} {
			mode := mode
			t.Run(tc.name+"/"+mode.String(), func(t *testing.T) {
				out, err, _ := runPureStdlibMode(t, tc.src, mode)
				if err != nil {
					t.Fatalf("run: %v", err)
				}
				if out != tc.want {
					t.Fatalf("output = %q, want %q", out, tc.want)
				}
			})
		}
	}
}
