package jit_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/jit"
	"github.com/alca-lang/alca/pkg/lexer"
	"github.com/alca-lang/alca/pkg/parser"
	"github.com/alca-lang/alca/pkg/stdlib"
	"github.com/alca-lang/alca/pkg/vm"
)

// stdlibTestEnv builds the subset of the standard library used by the
// differential tests, including the aggregate `std` namespace.
func stdlibTestEnv(t *testing.T) *interpreter.Environment {
	t.Helper()
	env := interpreter.NewEnvironment()
	stdlib.RegisterMathBuiltins(env)
	stdlib.RegisterStringBuiltins(env)
	stdlib.RegisterCryptoBuiltins(env)
	stdlib.RegisterPathBuiltins(env)
	stdlib.RegisterRegexBuiltins(env)
	stdlib.RegisterCompressBuiltins(env)
	stdlib.RegisterFormatsBuiltins(env)
	stdlib.RegisterTimeBuiltins(env)
	stdlib.RegisterIOBuiltins(env)
	stdlib.RegisterUrllib2Builtins(env)
	stdlib.RegisterAIBuiltins(env)
	stdlib.RegisterSysBuiltins(env)
	stdlib.RegisterEnvBuiltins(env)

	std := interpreter.NewMap()
	for _, name := range []string{
		"math", "string", "str", "crypto", "path", "regex", "re",
		"compress", "formats", "csv", "ini", "json", "time", "io",
		"urllib2", "ai", "sys", "env",
	} {
		if val, ok := env.Get(name); ok {
			std.Set(name, val)
		}
	}
	env.Set("std", std)
	return env
}

// runStdlibMode parses and runs src under one execution mode, returning the
// printed output and the runtime error (if any).
func runStdlibMode(t *testing.T, src string, mode jit.StdlibExecutionMode) (string, error) {
	t.Helper()

	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if errs := p.SyntaxErrors(); len(errs) > 0 {
		return "", fmt.Errorf("syntax errors: %v", errs)
	}

	var out strings.Builder
	// The differential sources only use println, so record the line ending that
	// the print hook otherwise discards.
	interpreter.SetPrintHook(func(s string) {
		out.WriteString(s)
		out.WriteByte('\n')
	})
	defer interpreter.ResetPrintHook()

	if mode == jit.StdlibInterpreter {
		interp := interpreter.New()
		registerStdlibModules(interp.Env())
		err := interp.Execute(prog)
		return out.String(), err
	}

	var engine *jit.Engine
	switch mode {
	case jit.StdlibVM:
		engine = jit.NewEngine(jit.Config{Enabled: false})
	case jit.StdlibThreadedJIT:
		engine = jit.NewEngine(jit.Config{Enabled: true, Native: false, Threshold: 1})
	case jit.StdlibAMD64JIT:
		if !jit.HasNativeAMD64 {
			return "", fmt.Errorf("native AMD64 backend unavailable")
		}
		engine = jit.NewEngine(jit.Config{Enabled: true, Native: true, Threshold: 1})
	default:
		return "", fmt.Errorf("unsupported mode %s", mode)
	}
	defer engine.Clear()
	vm.SetJITEngine(engine)
	defer vm.SetJITEngine(jit.Default())

	compiler := vm.NewCompiler()
	if err := compiler.Compile(prog); err != nil {
		return "", err
	}
	machine := vm.NewWithEnv(compiler.Bytecode(), stdlibTestEnv(t))
	err := machine.Run()

	if mode != jit.StdlibVM {
		if artifact := engine.GetCompiled(machine.MainFunction()); artifact == nil {
			t.Errorf("%s: main function was not JIT-compiled", mode)
		} else if mode == jit.StdlibAMD64JIT && artifact.NativeFn == nil {
			t.Errorf("%s: main function has no native AMD64 artifact", mode)
		} else if mode == jit.StdlibThreadedJIT && artifact.ThreadedFn == nil {
			t.Errorf("%s: main function has no threaded artifact", mode)
		}
	}
	return out.String(), err
}

func registerStdlibModules(env *interpreter.Environment) {
	stdlib.RegisterMathBuiltins(env)
	stdlib.RegisterStringBuiltins(env)
	stdlib.RegisterCryptoBuiltins(env)
	stdlib.RegisterPathBuiltins(env)
	stdlib.RegisterRegexBuiltins(env)
	stdlib.RegisterCompressBuiltins(env)
	stdlib.RegisterFormatsBuiltins(env)
	stdlib.RegisterTimeBuiltins(env)
	stdlib.RegisterIOBuiltins(env)
	stdlib.RegisterUrllib2Builtins(env)
	stdlib.RegisterAIBuiltins(env)
	stdlib.RegisterSysBuiltins(env)
	stdlib.RegisterEnvBuiltins(env)

	std := interpreter.NewMap()
	for _, name := range []string{
		"math", "string", "str", "crypto", "path", "regex", "re",
		"compress", "formats", "csv", "ini", "json", "time", "io",
		"urllib2", "ai", "sys", "env",
	} {
		if val, ok := env.Get(name); ok {
			std.Set(name, val)
		}
	}
	env.Set("std", std)
}

func stdlibDiffModes() []jit.StdlibExecutionMode {
	modes := []jit.StdlibExecutionMode{
		jit.StdlibInterpreter,
		jit.StdlibVM,
		jit.StdlibThreadedJIT,
	}
	if jit.HasNativeAMD64 {
		modes = append(modes, jit.StdlibAMD64JIT)
	}
	return modes
}

func runStdlibDifferential(t *testing.T, name, src, want string) {
	t.Helper()
	for _, mode := range stdlibDiffModes() {
		mode := mode
		t.Run(name+"/"+mode.String(), func(t *testing.T) {
			out, err := runStdlibMode(t, src, mode)
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			if out != want {
				t.Fatalf("output = %q, want %q", out, want)
			}
		})
	}
}

// TestStdlibAOTDeoptimizesToVM verifies the standalone/AOT payload path: a
// precompiled main function runs natively, executes imports through the helper
// tier, and deoptimizes at the user-function call; the VM finishes with the
// normal registered functions.
func TestStdlibAOTDeoptimizesToVM(t *testing.T) {
	if !jit.HasNativeAMD64 {
		t.Skip("native AMD64 backend is unavailable")
	}

	const src = `
use std.math;
use std.str;
top := str.upper("top");
power := math.pow(2.0, 3.0);
items := [1, 2, 3];
items[1] = 20;
m := {"k": 1};
m["n"] = 2;
m.z = 3;
sum := int(items[1]) + int(m["n"]) + m.z + (2 ** 3);
println(top);
println(power);
println(sum);
func probe():
    println(math.abs(-4));
    println(math.sqrt(81));
    println(str.upper("ok"));
probe();
`
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if errs := p.SyntaxErrors(); len(errs) > 0 {
		t.Fatalf("syntax errors: %v", errs)
	}
	compiler := vm.NewCompiler()
	if err := compiler.Compile(prog); err != nil {
		t.Fatalf("compile: %v", err)
	}
	machine := vm.NewWithEnv(compiler.Bytecode(), stdlibTestEnv(t))
	mainFn := machine.MainFunction()

	native, offsets, err := jit.CompileAOT(mainFn)
	if err != nil {
		t.Fatalf("compile AOT: %v", err)
	}
	if len(native) == 0 {
		t.Fatal("CompileAOT produced no code")
	}
	addr, err := jit.AllocExec(len(native))
	if err != nil {
		t.Fatalf("allocate executable memory: %v", err)
	}
	defer jit.FreeExec(addr, len(native))
	copy(jit.UnsafeSlice(addr, len(native)), native)

	engine := jit.NewEngine(jit.Config{Enabled: true, Native: true, Threshold: 1})
	ipOffsets := make(map[int]uintptr, len(offsets))
	for ip, off := range offsets {
		ipOffsets[ip] = addr + uintptr(off)
	}
	engine.InstallAOT(mainFn, addr, len(native), ipOffsets)
	defer engine.Clear()
	vm.SetJITEngine(engine)
	defer vm.SetJITEngine(jit.Default())

	var out strings.Builder
	interpreter.SetPrintHook(func(s string) {
		out.WriteString(s)
		out.WriteByte('\n')
	})
	defer interpreter.ResetPrintHook()

	if err := machine.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got, want := out.String(), "TOP\n8.0\n33\n4\n9.0\nOK\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestStdlibJITMathIntrinsicsDifferential(t *testing.T) {
	const src = `
use std.math;
func math_probe():
    println(math.abs(-3));
    println(math.sqrt(81));
    println(math.round(2.5));
    println(math.min(2, 3.5));
    println(math.max(2, 3.5));
    println(math.clamp(12, 1, 10));
    println(math.floor(-2.5));
    println(math.ceil(2.1));
math_probe();
println(math.pow(2.0, 10.0));
println(math.sin(0.0));
`
	want := "3\n9.0\n3\n2\n3.5\n10\n-3\n3\n1024.0\n0.0\n"
	runStdlibDifferential(t, "math", src, want)
}

func TestStdlibJITStringAndListRepresentationsDifferential(t *testing.T) {
	const src = `
use std.str;
func string_probe():
    println(str.upper("abc"));
    println(str.length("héllo"));
    println(str.slice("a🙂bc", 1, 2));
    println(str.join(["a", "b"], "-"));
    println(str.join(str.runes("hé"), "-"));
    println(str.replace("banana", "an", "X"));
func list_probe(items: list[string]):
    println(str.join(items, "|"));
func runes_probe(chars: list[string]):
    println(str.join(chars, "+"));
string_probe();
list_probe(["c", "d"]);
runes_probe(str.runes("hé"));
`
	want := "ABC\n5\n🙂\na-b\nh-é\nbXXa\nc|d\nh+é\n"
	runStdlibDifferential(t, "string", src, want)
}

func TestStdlibJITModulesDifferential(t *testing.T) {
	const src = `
use std.crypto;
use std.path;
use std.regex;
use std.compress;
use std.formats;
func module_probe():
    println(crypto.sha256("abc"));
    println(crypto.hex_encode("hi"));
    println(path.join("a", "b", "c"));
    println(path.base("/x/y.txt"));
    println(regex.replace("[0-9]+", "!", "abc123"));
    compressed := compress.gzip_compress("hello");
    println(compress.gzip_decompress(compressed, 1024));
    println(formats.json.encode([1, 2, 3]));
    println(formats.json.encode([1, 2.5, null]));
    println(formats.json.encode({"k": "v"}));
func ints_probe(values: list[int]):
    println(formats.json.encode(values));
module_probe();
ints_probe([4, 5, 6]);
`
	want := "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad\n" +
		"6869\n" +
		filepath.Join("a", "b", "c") + "\n" +
		"y.txt\n" +
		"abc!\n" +
		"hello\n" +
		"[1,2,3]\n" +
		"[1,2.5,null]\n" +
		`{"k":"v"}` + "\n" +
		"[4,5,6]\n"
	runStdlibDifferential(t, "modules", src, want)
}

func TestStdlibJITModuleMutationDifferential(t *testing.T) {
	const src = `
use std.math as m;
m.sqrt = 7;
func mutation_probe():
    println(m.sqrt);
    println(m.abs(-4));
mutation_probe();
`
	runStdlibDifferential(t, "mutation", src, "7\n4\n")
}

func TestStdlibJITAliasesDifferential(t *testing.T) {
	const src = `
use std.re as r;
use std.str as s;
func alias_probe():
    println(r.find("[0-9]+", "abc123"));
    println(s.upper("ok"));
alias_probe();
`
	runStdlibDifferential(t, "aliases", src, "123\nOK\n")
}

func TestStdlibJITComparisonsDifferential(t *testing.T) {
	const src = `
use std.str;
func compare_probe():
    a := str.upper("abc");
    b := str.lower("ABC");
    println(a == b);
    println(a === b);
    println(a != b);
    println(a < b);
    println(a >= b);
    println("apple" < "banana");
compare_probe();
`
	want := "false\nfalse\ntrue\ntrue\nfalse\ntrue\n"
	runStdlibDifferential(t, "comparisons", src, want)
}

// TestStdlibJITUnknownPureMemberErrorDifferential verifies that fusing a pure
// module call does not change the diagnostics for a misspelled member. The
// fused VM path deliberately falls back to the generic field/method resolution
// when a member is absent.
func TestStdlibJITUnknownPureMemberErrorDifferential(t *testing.T) {
	const src = `
use std.str;
func bad():
    println(str.uppper("x"));
bad();
`
	for _, mode := range stdlibDiffModes() {
		mode := mode
		t.Run(mode.String(), func(t *testing.T) {
			out, err := runStdlibMode(t, src, mode)
			if err == nil {
				t.Fatalf("expected an error, got output %q", out)
			}
			if !strings.Contains(err.Error(), `map has no method "uppper"`) {
				t.Fatalf("error = %q, want missing-method diagnostic", err)
			}
		})
	}
}

func TestStdlibJITRuntimeErrorsDifferential(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "math bad type",
			src: `
use std.math;
func bad_type():
    println(math.sqrt("not a number"));
bad_type();
`,
			want: "numeric",
		},
		{
			name: "math missing argument",
			src: `
use std.math;
func bad_arity():
    println(math.clamp(1, 2));
bad_arity();
`,
			want: "too few arguments",
		},
		{
			name: "string bad type",
			src: `
use std.str;
func bad_type():
    println(str.upper(42));
bad_type();
`,
			want: "",
		},
		{
			name: "host builtin error",
			src: `
use std.io;
func bad_read():
    println(io.read_file("__alca_missing_jit_helper_test__"));
bad_read();
`,
			want: "",
		},
		{
			name: "index out of bounds",
			src: `
func bad_index():
    xs := [1, 2, 3];
    println(xs[7]);
bad_index();
`,
			want: "out of bounds",
		},
		{
			name: "negative integer power",
			src: `
func bad_power():
    println(2 ** -1);
bad_power();
`,
			want: "negative exponent",
		},
		{
			name: "invalid list assignment",
			src: `
func bad_set_index():
    xs := [1, 2, 3];
    xs[7] = 1;
bad_set_index();
`,
			want: "invalid list index",
		},
		{
			name: "raised error after stdlib call",
			src: `
use std.str;
func boom():
    println(str.upper("before"));
    err("boom", 418);
boom();
`,
			want: "boom",
		},
		{
			name: "assertion failure after stdlib call",
			src: `
use std.str;
func fail_assert():
    println(str.upper("before"));
    assert 1 == 2;
fail_assert();
`,
			want: "assertion failed",
		},
		{
			name: "mutated math module alias",
			src: `
use std.math as m;
func mutate(module: json):
    module.sqrt = 7;
mutate(m);
println(m.sqrt(9));
`,
			want: "",
		},
	}
	for _, tc := range cases {
		tc := tc
		for _, mode := range stdlibDiffModes() {
			mode := mode
			t.Run(tc.name+"/"+mode.String(), func(t *testing.T) {
				out, err := runStdlibMode(t, tc.src, mode)
				if err == nil {
					t.Fatalf("expected an error, got output %q", out)
				}
				if tc.want != "" && !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tc.want)) {
					t.Fatalf("error %q does not contain %q", err, tc.want)
				}
			})
		}
	}
}
