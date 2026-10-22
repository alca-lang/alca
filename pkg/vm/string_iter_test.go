package vm

import (
	"testing"

	"github.com/alca-lang/alca/pkg/interpreter"
)

// TestStringIterationMatchesInterpreter covers iterating a string: the VM keeps
// the characters in a rune-backed list and hands out interned single-character
// strings, which must still match the interpreter character for character.
func TestStringIterationMatchesInterpreter(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{
			name: "ascii characters",
			src: `for ch in "abc":
    print(ch + "|");
`,
			want: "a|b|c|",
		},
		{
			name: "multi-byte characters",
			src: `for ch in "héllo":
    print(ch + "|");
`,
			want: "h|é|l|l|o|",
		},
		{
			name: "string length counts characters not bytes",
			src: `s := "héllo";
print(string(len(s)));
`,
			want: "5",
		},
		{
			name: "negative string index",
			src: `s := "abc";
print(s[-1]);
`,
			want: "c",
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

// TestStringIndexErrorsMatchInterpreter checks that an out-of-range string index
// raises an error on both engines.
func TestStringIndexErrorsMatchInterpreter(t *testing.T) {
	src := `s := "abc";
print(s[5]);
`
	interpErr := interpreter.New().Execute(parseProgram(t, src))
	if interpErr == nil {
		t.Fatal("interpreter accepted an out-of-range string index")
	}
	if vmErr := runSource(t, src); vmErr == nil {
		t.Fatalf("vm accepted an out-of-range string index (interpreter: %v)", interpErr)
	}
}
