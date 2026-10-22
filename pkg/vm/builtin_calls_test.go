package vm

import (
	"testing"

	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/lexer"
	"github.com/alca-lang/alca/pkg/parser"
	"github.com/alca-lang/alca/pkg/stdlib"
)

// TestBuiltinConversionsMatchInterpreter covers the OpCallBuiltin1 fast path for
// the always-available single-argument builtins (int/float/bool/len). The VM
// lowers a bare, unbound call to these to an unboxed opcode, so its output must
// stay byte-for-byte identical to the tree-walking interpreter, including the
// fallback for strings and the rune-count semantics of len.
func TestBuiltinConversionsMatchInterpreter(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{
			name: "float of int in a loop",
			src: `x := 0.0;
for i in 1..5:
    x = x + float(i);
println(string(int(x)));
`,
			want: "15",
		},
		{
			name: "int truncates floats toward zero",
			src: `println(string(int(2.9)) + "," + string(int(-2.9)) + "," + string(int(3.0)));
`,
			want: "2,-2,3",
		},
		{
			name: "float of bool",
			src: `println(string(float(true)) + "," + string(float(false)));
`,
			want: "1.0,0.0",
		},
		{
			name: "bool follows truthiness",
			src: `println(string(bool(0)) + "," + string(bool(1)) + "," + string(bool("")) + "," + string(bool("x")) + "," + string(bool(null)));
`,
			want: "false,true,false,true,false",
		},
		{
			name: "len counts runes and elements",
			src: `println(string(len("héllo")) + "," + string(len([10, 20, 30])));
`,
			want: "5,3",
		},
		{
			name: "len of non-container is zero",
			src: `println(string(len(5)));
`,
			want: "0",
		},
		{
			name: "float and int parse strings via fallback",
			src: `println(string(float("3.5")) + "," + string(int("42")));
`,
			want: "3.5,42",
		},
		{
			name: "a user binding named float is not specialised",
			src: `func float(x: int) -> int:
    return x + 100;
println(string(float(1)));
`,
			want: "101",
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

// TestBuiltinConversionErrorsMatchInterpreter makes sure the specialised path
// does not swallow conversions the interpreter rejects: int/float of null and
// of an unparsable string are errors, and the VM must raise them too.
func TestBuiltinConversionErrorsMatchInterpreter(t *testing.T) {
	cases := []struct{ name, src string }{
		{"int of null", "x := int(null);\n"},
		{"float of null", "x := float(null);\n"},
		{"int of unparsable string", "x := int(\"nope\");\n"},
		{"float of unparsable string", "x := float(\"nope\");\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			interpErr := interpreter.New().Execute(parseProgram(t, tc.src))
			if interpErr == nil {
				t.Fatalf("interpreter accepted %q, expected an error", tc.src)
			}
			if vmErr := runSource(t, tc.src); vmErr == nil {
				t.Fatalf("vm accepted %q, expected an error (interpreter: %v)", tc.src, interpErr)
			}
		})
	}
}

func TestMathModuleCallsLowerToVMIntrinsics(t *testing.T) {
	src := `use std.math as m;
result := m.sqrt(81);
`
	program := parseProgram(t, src)
	compiler := NewCompiler()
	if err := compiler.Compile(program); err != nil {
		t.Fatalf("compile: %v", err)
	}
	found := false
	for _, op := range compiler.Bytecode().Instructions {
		if Opcode(op) == OpMathBuiltin {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("math.sqrt call was not lowered to OpMathBuiltin")
	}

	interp := interpreter.New()
	stdlib.RegisterMathBuiltins(interp.Env())
	machine := NewWithEnv(compiler.Bytecode(), interp.Env())
	if err := machine.Run(); err != nil {
		t.Fatalf("run compiled math intrinsic: %v", err)
	}
	if got := machine.GetGlobal(0).Inspect(); got != "9.0" {
		t.Fatalf("math.sqrt result = %q, want 9.0", got)
	}
}

func TestMathIntrinsicLoweringRespectsShadowedAliases(t *testing.T) {
	src := `use std.math as m;
m := {"sqrt": 7};
result := m.sqrt;
`
	p := parser.New(lexer.New(src))
	program := p.ParseProgram()
	if errs := p.SyntaxErrors(); len(errs) > 0 {
		t.Fatalf("syntax errors: %v", errs)
	}
	compiler := NewCompiler()
	if err := compiler.Compile(program); err != nil {
		t.Fatalf("compile: %v", err)
	}
	for _, op := range compiler.Bytecode().Instructions {
		if Opcode(op) == OpMathBuiltin {
			t.Fatal("shadowed math module alias was incorrectly lowered")
		}
	}
}

func TestMathIntrinsicLoweringRespectsModifiedModuleFunctions(t *testing.T) {
	src := `use std.math as m;
m.sqrt = 7;
result := m.sqrt(9);
`
	program := parseProgram(t, src)
	compiler := NewCompiler()
	if err := compiler.Compile(program); err != nil {
		t.Fatalf("compile: %v", err)
	}
	for _, op := range compiler.Bytecode().Instructions {
		if Opcode(op) == OpMathBuiltin {
			t.Fatal("modified math module function was incorrectly lowered")
		}
	}
}

func TestMathIntrinsicLoweringRespectsAliasedModuleMutations(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{
			name: "direct alias escapes to function",
			src: `use std.math as m;
func mutate(mod: json):
    mod.sqrt = 7;
mutate(m);
result := m.sqrt(9);
`,
		},
		{
			name: "aliased module binding",
			src: `use std.math as m;
other := m;
other.sqrt = 7;
result := m.sqrt(9);
`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			program := parseProgram(t, tc.src)
			compiler := NewCompiler()
			if err := compiler.Compile(program); err != nil {
				t.Fatalf("compile: %v", err)
			}
			for _, op := range compiler.Bytecode().Instructions {
				if Opcode(op) == OpMathBuiltin {
					t.Fatal("escaped math module alias was incorrectly lowered")
				}
			}
		})
	}
}

func TestMathIntrinsicPreservesStdlibTypeErrors(t *testing.T) {
	program := parseProgram(t, `use std.math as m;
result := m.sqrt("not a number");
`)
	compiler := NewCompiler()
	if err := compiler.Compile(program); err != nil {
		t.Fatalf("compile: %v", err)
	}
	interp := interpreter.New()
	stdlib.RegisterMathBuiltins(interp.Env())
	machine := NewWithEnv(compiler.Bytecode(), interp.Env())
	err := machine.Run()
	raised, ok := err.(*RaisedError)
	if !ok {
		t.Fatalf("run error = %T (%v), want *RaisedError", err, err)
	}
	if raised.Val.Code != 400 || raised.Val.Message != "math.sqrt arguments must be numeric" {
		t.Fatalf("math error = (%d, %q), want (400, numeric-arguments message)", raised.Val.Code, raised.Val.Message)
	}
}
