package jit_test

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/jit"
	"github.com/alca-lang/alca/pkg/lexer"
	"github.com/alca-lang/alca/pkg/parser"
	"github.com/alca-lang/alca/pkg/vm"
)

// aesDecryptFixture builds a deterministic AES-GCM ciphertext so the pure
// crypto.aes_decrypt member can be compared across engines.
func aesDecryptFixture(t *testing.T) (key string, ciphertext string) {
	t.Helper()
	keyBytes := []byte("0123456789abcdef")
	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		t.Fatalf("aes.NewCipher: %v", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("cipher.NewGCM: %v", err)
	}
	nonce := []byte("012345678901")
	sealed := gcm.Seal(nil, nonce, []byte("hello"), nil)
	raw := append(append([]byte{}, nonce...), sealed...)
	return string(keyBytes), base64.StdEncoding.EncodeToString(raw)
}

// TestStdlibParityAcrossEngines runs the same program on the tree-walking
// interpreter, the bytecode VM, the threaded JIT, and the AMD64 JIT and
// requires every engine to produce the same output. The programs exercise the
// deterministic members of the pure and mixed standard-library modules, which
// are the members the JIT engines execute through the runtime-helper tier.
func TestStdlibParityAcrossEngines(t *testing.T) {
	ciphertextKey, ciphertextValue := aesDecryptFixture(t)

	cases := []struct {
		name string
		src  string
		want []string
	}{
		{
			name: "path pure members",
			src: `
use std.path;
func path_probe():
    println(path.join("a", "b"));
    println(path.base("/x/y.txt"));
    println(path.ext("/x/y.txt"));
    println(path.stem("/x/y.txt"));
    println(path.clean("a/./b/../c"));
    println(path.is_abs("relative.txt"));
    println(path.rel("/a/b", "/a/b/c/d.txt"));
    parts := path.split("/x/y.txt");
    println(parts[1]);
    println(path.to_slash("a/b"));
    println(path.from_slash("a/b"));
    println(path.matches("*.txt", "a.txt"));
path_probe();
`,
			want: []string{"y.txt", ".txt", "true"},
		},
		{
			name: "crypto pure members",
			src: `
use std.crypto;
func crypto_probe():
    println(crypto.sha256("abc"));
    println(crypto.sha1("abc"));
    println(crypto.md5("abc"));
    println(crypto.base64_encode("hi"));
    println(crypto.base64_decode("aGk="));
    println(crypto.hex_encode("hi"));
    println(crypto.hex_decode("6869"));
    println(crypto.hmac_sha256("key", "data"));
    println(crypto.constant_time_equal("abc", "abc"));
    println(crypto.constant_time_equal("abc", "abd"));
    println(crypto.aes_decrypt("` + ciphertextKey + `", "` + ciphertextValue + `"));
crypto_probe();
`,
			want: []string{
				"ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
				"aGk=",
				"6869",
				"hello",
			},
		},
		{
			name: "urllib2 pure members",
			src: `
use std.urllib2;
func url_probe():
    parsed := urllib2.parse("https://example.com/a/b?x=1&y=2#frag");
    println(parsed["scheme"]);
    println(parsed["netloc"]);
    println(parsed["path"]);
    println(parsed["query"]);
    println(parsed["fragment"]);
    parts := urllib2.split("https://example.com/a");
    println(parts[0]);
    println(urllib2.join("https://example.com/a/", "b"));
    println(urllib2.quote("a b/c"));
    println(urllib2.unquote("a%20b"));
    println(urllib2.urlencode({"a": "1", "b": ["2", "3"]}));
    query := urllib2.parse_qs("a=1&b=2");
    println(query["b"][0]);
    pairs := urllib2.parse_qsl("a=1&b=2");
    println(pairs[1][0]);
url_probe();
`,
			want: []string{"https", "example.com", "a=1&b=2", "a=1&b=2"},
		},
		{
			name: "time pure members",
			src: `
use std.time;
func time_probe():
    println(time.format(0, "date", "UTC"));
    println(time.parse("2020-01-02T03:04:05Z"));
    println(time.parse_duration("1h30m"));
    println(time.diff(10, 4));
    println(time.unix_ms_to_seconds(1500));
    println(time.unix_seconds_to_ms(2.5));
time_probe();
`,
			want: []string{"1970-01-01", "1577934245", "5400000000000"},
		},
		{
			name: "formats pure members",
			src: `
use std.formats;
func format_probe():
    println(formats.json.encode({"a": 1}));
    println(formats.json.parse("{\"a\":1}")["a"]);
    println(formats.json.validate("{\"a\":1}"));
    rows := formats.csv.parse("a,b\n1,2");
    println(rows[0]["a"]);
    section := formats.ini.parse("[s]\nk=v");
    println(section["s"]["k"]);
format_probe();
`,
			want: []string{`{"a":1}`, "1", "true", "1", "v"},
		},
		{
			name: "ai pure members",
			src: `
use std.ai;
func ai_probe():
    product := ai.matmul([[1, 2], [3, 4]], [[5, 6], [7, 8]]);
    println(product[0][0]);
    println(product[1][1]);
    println(ai.dot([1, 2, 3], [4, 5, 6]));
    transposed := ai.transpose([[1, 2, 3], [4, 5, 6]]);
    println(transposed[2][0]);
    sum := ai.add([[1, 2]], [[3, 4]]);
    println(sum[0][1]);
    activated := ai.relu([-1, 2]);
    println(activated[0]);
    println(activated[1]);
    println(ai.mse([1, 2], [1, 2]));
ai_probe();
`,
			want: []string{"19", "50", "32", "3", "6", "0", "2"},
		},
		{
			name: "string regex and compress members",
			src: `
use std.str;
use std.regex;
use std.compress;
func text_probe():
    println(str.trim_left("  x  "));
    println(str.trim_right("  x  "));
    println(str.contains("hello", "ell"));
    println(str.index_of("hello", "l"));
    println(str.starts_with("hello", "he"));
    println(str.ends_with("hello", "lo"));
    println(str.slice("abcdef", 1, 3));
    println(str.runes("hé"));
    println(regex.match("[0-9]+", "a1"));
    println(regex.find_all("[0-9]+", "a1b22"));
    println(regex.split(",", "a,b,c"));
    println(regex.quote("a.b"));
    zipped := compress.zlib_compress("hello");
    println(compress.zlib_decompress(zipped));
text_probe();
`,
			want: []string{"x", "2", "true", "bc", `["1", "22"]`},
		},
		{
			name: "host machine and environment members",
			src: `
use std.sys;
use std.env;
func host_probe():
    println(sys.platform);
    println(sys.arch);
    println(sys.cpu_count() > 0);
    println(env.get_env("ALCA_PARITY_TEST_MISSING") == null);
host_probe();
`,
			want: []string{"true"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var baseline string
			for i, mode := range stdlibDiffModes() {
				out, err := runStdlibMode(t, tc.src, mode)
				if err != nil {
					t.Fatalf("%s: run: %v", mode, err)
				}
				if i == 0 {
					baseline = out
					continue
				}
				if out != baseline {
					t.Fatalf("%s output = %q, want %q", mode, out, baseline)
				}
			}
			if baseline == "" {
				t.Fatal("program produced no output")
			}
			for _, want := range tc.want {
				if !strings.Contains(baseline, want) {
					t.Fatalf("output %q does not contain %q", baseline, want)
				}
			}
		})
	}
}

// runStdlibAOTMode runs src through the standalone/AOT path: the main function
// is compiled to native machine code, installed into the engine, and executed
// with the VM available for deoptimization.
func runStdlibAOTMode(t *testing.T, src string) (string, error) {
	t.Helper()
	if !jit.HasNativeAMD64 {
		t.Skip("native AMD64 backend is unavailable")
	}

	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if errs := p.SyntaxErrors(); len(errs) > 0 {
		return "", fmt.Errorf("syntax errors: %v", errs)
	}

	var out strings.Builder
	interpreter.SetPrintHook(func(s string) {
		out.WriteString(s)
		out.WriteByte('\n')
	})
	defer interpreter.ResetPrintHook()

	compiler := vm.NewCompiler()
	if err := compiler.Compile(prog); err != nil {
		return "", err
	}
	machine := vm.NewWithEnv(compiler.Bytecode(), stdlibTestEnv(t))
	mainFn := machine.MainFunction()

	native, offsets, err := jit.CompileAOT(mainFn)
	if err != nil {
		return "", err
	}
	if len(native) == 0 {
		return "", fmt.Errorf("CompileAOT produced no code")
	}
	addr, err := jit.AllocExec(len(native))
	if err != nil {
		return "", err
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

	err = machine.Run()
	return out.String(), err
}

// TestStdlibAOTParityWithVM checks that the standalone/AOT execution path
// produces the same output as the bytecode VM for a program that mixes pure
// standard-library calls, container operations, and user function calls.
func TestStdlibAOTParityWithVM(t *testing.T) {
	const src = `
use std.str;
use std.math;
use std.formats;
func double(x: int) -> int:
    return x * 2;
func probe():
    items := [1, 2, 3];
    items[1] = 20;
    mapping := {"k": 1};
    mapping["n"] = 2;
    total := int(items[1]) + int(mapping["n"]) + double(3);
    println(str.upper("aot"));
    println(math.abs(-4));
    println(formats.json.encode(mapping));
    println(total);
probe();
`
	vmOut, err := runStdlibMode(t, src, jit.StdlibVM)
	if err != nil {
		t.Fatalf("vm run: %v", err)
	}
	aotOut, err := runStdlibAOTMode(t, src)
	if err != nil {
		t.Fatalf("aot run: %v", err)
	}
	if aotOut != vmOut {
		t.Fatalf("aot output = %q, want %q", aotOut, vmOut)
	}
	want := "AOT\n4\n{\"k\":1,\"n\":2}\n28\n"
	if vmOut != want {
		t.Fatalf("vm output = %q, want %q", vmOut, want)
	}
}

// TestStdlibErrorParityAcrossJITModes checks that a failing program reports
// the same error text on the bytecode VM, the threaded JIT, and the AMD64 JIT.
// The tree-walking interpreter is excluded because it annotates errors with
// source positions.
func TestStdlibErrorParityAcrossJITModes(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{
			name: "string type error",
			src: `
use std.str;
func bad():
    println(str.upper(42));
bad();
`,
		},
		{
			name: "math type error",
			src: `
use std.math;
func bad():
    println(math.sqrt("not a number"));
bad();
`,
		},
		{
			name: "math arity error",
			src: `
use std.math;
func bad():
    println(math.clamp(1, 2));
bad();
`,
		},
		{
			name: "missing pure member",
			src: `
use std.str;
func bad():
    println(str.uppper("x"));
bad();
`,
		},
		{
			name: "host file error",
			src: `
use std.io;
func bad():
    println(io.read_file("__alca_missing_parity_test__"));
bad();
`,
		},
		{
			name: "index out of bounds",
			src: `
func bad():
    xs := [1, 2, 3];
    println(xs[7]);
bad();
`,
		},
		{
			name: "raised error after stdlib call",
			src: `
use std.str;
func bad():
    println(str.upper("before"));
    err("boom", 418);
bad();
`,
		},
		{
			name: "assertion failure after stdlib call",
			src: `
use std.str;
func bad():
    println(str.upper("before"));
    assert 1 == 2;
bad();
`,
		},
		{
			name: "url parse error",
			src: `
use std.urllib2;
func bad():
    println(urllib2.parse("://bad"));
bad();
`,
		},
		{
			name: "base64 decode error",
			src: `
use std.crypto;
func bad():
    println(crypto.base64_decode("!!!"));
bad();
`,
		},
		{
			name: "matrix shape error",
			src: `
use std.ai;
func bad():
    println(ai.matmul([[1, 2]], [[1, 2]]));
bad();
`,
		},
	}

	modes := []jit.StdlibExecutionMode{jit.StdlibVM, jit.StdlibThreadedJIT}
	if jit.HasNativeAMD64 {
		modes = append(modes, jit.StdlibAMD64JIT)
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var baseline string
			for i, mode := range modes {
				out, err := runStdlibMode(t, tc.src, mode)
				if err == nil {
					t.Fatalf("%s: expected an error, got output %q", mode, out)
				}
				if i == 0 {
					baseline = err.Error()
					continue
				}
				if err.Error() != baseline {
					t.Fatalf("%s error = %q, want %q", mode, err.Error(), baseline)
				}
			}
		})
	}
}

// TestStdlibParityHostFileMembers checks that the host-backed members that are
// not fused still behave the same on every engine. The file-backed members run
// in the generic JIT runtime-helper tier.
func TestStdlibParityHostFileMembers(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "data.csv")
	if err := os.WriteFile(csvPath, []byte("a,b\n1,2\n"), 0o644); err != nil {
		t.Fatalf("write csv fixture: %v", err)
	}
	textPath := filepath.Join(dir, "data.txt")
	if err := os.WriteFile(textPath, []byte("hello file\n"), 0o644); err != nil {
		t.Fatalf("write text fixture: %v", err)
	}

	src := `
use std.io;
use std.csv;
use std.path;
func host_probe():
    println(io.read_file("` + textPath + `"));
    rows := csv.read("` + csvPath + `");
    println(rows[0]["a"]);
    println(path.base("` + csvPath + `"));
host_probe();
`
	for _, mode := range stdlibDiffModes() {
		mode := mode
		t.Run(mode.String(), func(t *testing.T) {
			out, err := runStdlibMode(t, src, mode)
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			want := "hello file\n\n1\ndata.csv\n"
			if out != want {
				t.Fatalf("output = %q, want %q", out, want)
			}
		})
	}
}

// TestStdlibParityAliasedAndNestedMembers checks that aliased imports and
// nested module paths keep working the same way on every engine.
func TestStdlibParityAliasedAndNestedMembers(t *testing.T) {
	const src = `
use std.str as s;
use std.formats as f;
use std.math as m;
func alias_probe():
    println(s.upper("ok"));
    println(f.json.encode([1, 2]));
    println(m.abs(-2));
alias_probe();
`
	for _, mode := range stdlibDiffModes() {
		mode := mode
		t.Run(mode.String(), func(t *testing.T) {
			out, err := runStdlibMode(t, src, mode)
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			want := "OK\n[1,2]\n2\n"
			if out != want {
				t.Fatalf("output = %q, want %q", out, want)
			}
		})
	}
}

// TestStdlibParityInsideLoopAndHelperCalls checks that pure stdlib calls
// behave the same when they run inside loops, inside nested user functions,
// and when the enclosing function is re-entered by the JIT helper tier.
func TestStdlibParityInsideLoopAndHelperCalls(t *testing.T) {
	const src = `
use std.str;
func twice(text: string) -> string:
    return str.upper(text);
func probe():
    total := 0;
    for i in 0..3:
        total = total + len(twice("abc"));
    println(total);
probe();
`
	for _, mode := range stdlibDiffModes() {
		mode := mode
		t.Run(mode.String(), func(t *testing.T) {
			out, err := runStdlibMode(t, src, mode)
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			if out != "12\n" {
				t.Fatalf("output = %q, want %q", out, "12\n")
			}
		})
	}
}
