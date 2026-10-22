package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alca-lang/alca/pkg/ast"
	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/vm"
)

func sampleBytecode() *vm.Bytecode {
	return &vm.Bytecode{
		Instructions: vm.Instructions{byte(vm.OpConstant), 0, 0, byte(vm.OpPop)},
		Constants: []interpreter.Value{
			&interpreter.IntegerValue{Val: -42},
			&interpreter.FloatValue{Val: 3.25},
			&interpreter.StringValue{Val: "héllo"},
			&vm.CompiledFunction{
				Name: "f", NumLocals: 3, NumParameters: 2,
				NumFree:      2,
				UpvalueDescs: []vm.UpvalueDesc{{IsLocal: true, Index: 1}, {IsLocal: false, Index: 0}},
				Instructions: vm.Instructions{1, 2, 3},
			},
			&interpreter.StructDef{Name: "Server", Fields: []*ast.StructField{
				{Name: "ip", TypeAnnotation: &ast.TypeAnnotation{Types: []string{"string"}}},
				{Name: "port", TypeAnnotation: &ast.TypeAnnotation{Types: []string{"int"}}, DefaultValue: &ast.IntegerLiteral{Value: 80}},
				{Name: "ratio", TypeAnnotation: &ast.TypeAnnotation{Types: []string{"float"}}, DefaultValue: &ast.PrefixExpression{Operator: "-", Right: &ast.FloatLiteral{Value: 1.5}}},
			}},
			&vm.UseStatementValue{Stmt: &ast.UseStatement{ModulePath: "std.math", Alias: "m", Imports: []string{"sqrt"}}},
		},
	}
}

func TestBytecodeRoundTrip(t *testing.T) {
	b := &Bundle{Kind: KindBytecode, Name: "app.al", Code: sampleBytecode()}
	data, err := b.Encode()
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeBundle(data)
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != KindBytecode || got.Name != "app.al" {
		t.Fatalf("bad header: %+v", got)
	}
	if string(got.Code.Instructions) != string(b.Code.Instructions) {
		t.Fatalf("instructions differ")
	}
	if len(got.Code.Constants) != len(b.Code.Constants) {
		t.Fatalf("constant count %d != %d", len(got.Code.Constants), len(b.Code.Constants))
	}
	if v := got.Code.Constants[0].(*interpreter.IntegerValue).Val; v != -42 {
		t.Errorf("int: %d", v)
	}
	if v := got.Code.Constants[2].(*interpreter.StringValue).Val; v != "héllo" {
		t.Errorf("string: %q", v)
	}
	fn := got.Code.Constants[3].(*vm.CompiledFunction)
	if fn.Name != "f" || fn.NumLocals != 3 || fn.NumParameters != 2 {
		t.Errorf("function: %+v", fn)
	}
	if fn.NumFree != 2 || len(fn.UpvalueDescs) != 2 {
		t.Fatalf("free variables not preserved: %+v", fn)
	}
	if d := fn.UpvalueDescs[0]; !d.IsLocal || d.Index != 1 {
		t.Errorf("upvalue 0 = %+v, want local 1", d)
	}
	if d := fn.UpvalueDescs[1]; d.IsLocal || d.Index != 0 {
		t.Errorf("upvalue 1 = %+v, want outer upvalue 0", d)
	}
	st := got.Code.Constants[4].(*interpreter.StructDef)
	if st.Name != "Server" || len(st.Fields) != 3 {
		t.Fatalf("struct: %+v", st)
	}
	if d := st.Fields[1].DefaultValue.(*ast.IntegerLiteral).Value; d != 80 {
		t.Errorf("default: %d", d)
	}
	if d := st.Fields[2].DefaultValue.(*ast.FloatLiteral).Value; d != -1.5 {
		t.Errorf("negative float default: %v", d)
	}
	u := got.Code.Constants[5].(*vm.UseStatementValue).Stmt
	if u.ModulePath != "std.math" || u.Alias != "m" || len(u.Imports) != 1 {
		t.Errorf("use: %+v", u)
	}
}

func TestSourceBundleRoundTrip(t *testing.T) {
	b := &Bundle{
		Kind:      KindSource,
		Name:      "main.al",
		Source:    "println(1);\n",
		Files:     map[string]string{"lib/x.al": "// x"},
		Resources: map[string][]byte{"assets/app.ico": {0, 1, 2, 255}},
	}
	data, _ := b.Encode()
	got, err := DecodeBundle(data)
	if err != nil || got.Source != b.Source || got.Files["lib/x.al"] != "// x" || !bytes.Equal(got.Resources["assets/app.ico"], b.Resources["assets/app.ico"]) {
		t.Fatalf("round trip failed: %v %+v", err, got)
	}
}

func TestSourceIsNotShippedInBytecodeBundles(t *testing.T) {
	b := &Bundle{Kind: KindBytecode, Name: "a.al", Code: &vm.Bytecode{Constants: []interpreter.Value{&interpreter.IntegerValue{Val: 1}}}}
	data, _ := b.Encode()
	if strings.Contains(string(data), "SECRET_SOURCE_TEXT") {
		t.Fatal("unreachable")
	}
}

func TestCorruptionIsDetected(t *testing.T) {
	data, _ := (&Bundle{Kind: KindSource, Name: "a.al", Source: "x"}).Encode()
	data[len(data)-1] ^= 0xFF
	if _, err := DecodeBundle(data); err == nil || !strings.Contains(err.Error(), "integrity") {
		t.Fatalf("want integrity error, got %v", err)
	}
	if _, err := DecodeBundle(data[:10]); err == nil {
		t.Fatal("want error for truncated payload")
	}
}

func TestParseTarget(t *testing.T) {
	good := map[string]Target{"linux/amd64": {"linux", "amd64"}, "windows-arm64": {"windows", "arm64"}, "darwin/arm64": {"darwin", "arm64"}}
	for in, want := range good {
		got, err := ParseTarget(in)
		if err != nil || got != want {
			t.Errorf("%s -> %v, %v", in, got, err)
		}
	}
	for _, bad := range []string{"", "linux", "plan9/amd64", "linux/mips"} {
		if _, err := ParseTarget(bad); err == nil {
			t.Errorf("%q must be rejected", bad)
		}
	}
}

func TestNamingPerTarget(t *testing.T) {
	if RuntimeName(Target{"windows", "amd64"}) != "alca-windows-amd64.exe" {
		t.Error("windows runtime must end in .exe")
	}
	if RuntimeName(Target{"linux", "arm64"}) != "alca-linux-arm64" {
		t.Error("linux runtime name")
	}
	if ExeName("app", Target{"windows", "amd64"}) != "app.exe" || ExeName("app.exe", Target{"windows", "amd64"}) != "app.exe" || ExeName("app", Target{"linux", "amd64"}) != "app" {
		t.Error("ExeName")
	}
}

func TestBuildForEveryTarget(t *testing.T) {
	dir := t.TempDir()
	b := &Bundle{Kind: KindSource, Name: "main.al", Source: "println(1);\n"}
	for _, tg := range SupportedTargets {
		stub := filepath.Join(dir, RuntimeName(tg))
		if err := os.WriteFile(stub, []byte("FAKE-RUNTIME-FOR-"+tg.String()), 0o755); err != nil {
			t.Fatal(err)
		}
		found, err := FindRuntime(tg, stub)
		if err != nil || found != stub {
			t.Fatalf("%s: FindRuntime: %v", tg, err)
		}
		out := filepath.Join(dir, ExeName("out", tg))
		if err := Build(b, stub, out); err != nil {
			t.Fatalf("%s: %v", tg, err)
		}
		raw, _ := os.ReadFile(out)
		if !strings.HasPrefix(string(raw), "FAKE-RUNTIME-FOR-"+tg.String()) {
			t.Fatalf("%s: output must start with that target's runtime", tg)
		}
		if !VerifyMagic(raw[len(raw)-FooterLen:]) {
			t.Fatalf("%s: missing footer", tg)
		}
		got, err := ExtractBundle(out)
		if err != nil || got.Source != b.Source {
			t.Fatalf("%s: extract: %v", tg, err)
		}
	}
}

func TestBuildFromBuiltProgramDoesNotStackPayloads(t *testing.T) {
	dir := t.TempDir()
	stub := filepath.Join(dir, "stub")
	_ = os.WriteFile(stub, []byte("RUNTIME"), 0o755)
	a := filepath.Join(dir, "a")
	if err := Build(&Bundle{Kind: KindSource, Name: "a.al", Source: "1"}, stub, a); err != nil {
		t.Fatal(err)
	}
	b := filepath.Join(dir, "b")
	if err := Build(&Bundle{Kind: KindSource, Name: "b.al", Source: "2"}, a, b); err != nil {
		t.Fatal(err)
	}
	got, err := ExtractBundle(b)
	if err != nil || got.Source != "2" {
		t.Fatalf("want payload 2, got %+v %v", got, err)
	}
	rawA, _ := os.ReadFile(a)
	rawB, _ := os.ReadFile(b)
	if len(rawA) != len(rawB) {
		t.Fatalf("re-building from a built program must replace, not append, the payload (%d vs %d)", len(rawA), len(rawB))
	}
}

// TestHostBuildUsesTheRunningExecutable: a host build must use the running
// alca binary, never a stale runtime stub left in runtimes/. A stale stub can
// predate payload-format or security-policy changes.
func TestHostBuildUsesTheRunningExecutable(t *testing.T) {
	dir := t.TempDir()
	stale := filepath.Join(dir, RuntimeName(HostTarget()))
	if err := os.WriteFile(stale, []byte("STALE"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ALCA_RUNTIMES", dir)
	got, err := FindRuntime(HostTarget(), "")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("host runtime = %q, want the running executable %q", got, want)
	}
}

func TestMissingRuntimeExplainsHowToGetOne(t *testing.T) {
	t.Setenv("ALCA_RUNTIMES", t.TempDir())
	_, err := FindRuntime(Target{"linux", "arm64"}, "")
	if HostTarget() == (Target{"linux", "arm64"}) {
		t.Skip("host is the target")
	}
	if err == nil || !strings.Contains(err.Error(), "build-runtimes") && !strings.Contains(err.Error(), "make runtimes") {
		t.Fatalf("error must tell the user how to obtain a runtime, got: %v", err)
	}
}
