package vm

import (
	"testing"

	"github.com/alca-lang/alca/pkg/interpreter"
)

// TestSuperinstructionsMatchInterpreter covers the fused superinstructions
// (local-to-local add, local-times-constant, fused conversions). They must
// produce byte-identical output to the interpreter, including the error cases
// where a conversion or a modulo is not defined.
func TestSuperinstructionsMatchInterpreter(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{
			name: "local to local add",
			src: `func f(a: int, b: int) -> int:
    x := a;
    x = x + b;
    return x;
print(string(f(3, 4)));
`,
			want: "7",
		},
		{
			name: "global to global add",
			src: `a := 10;
b := 5;
a = a + b;
print(string(a));
`,
			want: "15",
		},
		{
			name: "local times constant",
			src: `func f(a: int) -> int:
    return a * 3;
print(string(f(7)));
`,
			want: "21",
		},
		{
			name: "local modulo constant",
			src: `func f(a: int) -> int:
    return a % 5;
print(string(f(17)));
`,
			want: "2",
		},
		{
			name: "fused float conversion of a local",
			src: `func f(a: int) -> float:
    return float(a) * 0.5;
print(string(f(5)));
`,
			want: "2.5",
		},
		{
			name: "fused int conversion of a local",
			src: `func f(a: float) -> int:
    return int(a);
print(string(f(2.9)));
`,
			want: "2",
		},
		{
			name: "fused bool conversion of a local",
			src: `func f(a: int) -> bool:
    return bool(a);
print(string(f(0)) + "," + string(f(3)));
`,
			want: "false,true",
		},
		{
			name: "fused len of a local list",
			src: `func f(xs: list) -> int:
    return len(xs);
print(string(f([1, 2, 3])));
`,
			want: "3",
		},
		{
			name: "float conversion of a string local",
			src: `func f(a: string) -> float:
    return float(a);
print(string(f("3.5")));
`,
			want: "3.5",
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
		})
	}
}

// TestSuperinstructionErrorsMatchInterpreter checks that the fused opcodes raise
// the same runtime errors as the interpreter instead of silently succeeding.
func TestSuperinstructionErrorsMatchInterpreter(t *testing.T) {
	cases := []struct{ name, src string }{
		{
			name: "modulo by zero",
			src: `func f(a: int) -> int:
    return a % 0;
print(string(f(5)));
`,
		},
		{
			name: "float conversion of a non-numeric string",
			src: `func f(a: string) -> float:
    return float(a);
print(string(f("abc")));
`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			interpErr := interpreter.New().Execute(parseProgram(t, tc.src))
			if interpErr == nil {
				t.Fatalf("interpreter accepted %q, expected an error", tc.src)
			}
			vmErr := runSource(t, tc.src)
			if vmErr == nil {
				t.Fatalf("vm accepted %q, expected an error (interpreter: %v)", tc.src, interpErr)
			}
		})
	}
}
