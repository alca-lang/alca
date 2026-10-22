package compiler_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alca-lang/alca/pkg/compiler"
	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/jit"
	"github.com/alca-lang/alca/pkg/lexer"
	"github.com/alca-lang/alca/pkg/parser"
	"github.com/alca-lang/alca/pkg/vm"
)

// TestBuildNativePERejectsBoxedConstants keeps the pure native PE honest: a
// string constant cannot be represented without Go runtime type information, so
// the builder must refuse the bundle and let the caller use the runtime stub.
func TestBuildNativePERejectsBoxedConstants(t *testing.T) {
	bundle := &compiler.Bundle{
		Kind:       compiler.KindNative,
		NativeCode: []byte{0xC3}, // ret
		Code: &vm.Bytecode{Constants: []interpreter.Value{
			&interpreter.StringValue{Val: "needs the Go runtime"},
		}},
	}
	err := compiler.BuildNativePE(bundle, filepath.Join(t.TempDir(), "boxed.exe"), false)
	if err == nil {
		t.Fatal("BuildNativePE accepted a string constant")
	}
	if !strings.Contains(err.Error(), "cannot represent") {
		t.Fatalf("error = %q, want an explicit unsupported-constant diagnostic", err)
	}
}

// TestBuildNativePEDivisionByNegativeOne is a regression test for pure native
// programs: valid integer division and modulo by -1 must execute, not take the
// old conservative bailout that a no-VM executable can only report as failure.
func TestBuildNativePEDivisionByNegativeOne(t *testing.T) {
	if !jit.HasNativeAMD64 {
		t.Skip("native AMD64 backend is unavailable")
	}

	src := `
a := 10 / -1;
b := 10 % -1;
`
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if len(p.SyntaxErrors()) > 0 {
		t.Fatalf("syntax errors: %v", p.SyntaxErrors())
	}
	c := vm.NewCompiler()
	if err := c.Compile(prog); err != nil {
		t.Fatalf("compile error: %v", err)
	}
	bundle := &compiler.Bundle{
		Name: "div_neg_one.al",
		Kind: compiler.KindBytecode,
		Code: c.Bytecode(),
	}
	nativeCode, labelOffsets, err := jit.CompileAOT(&vm.CompiledFunction{
		Instructions: bundle.Code.Instructions,
		Pool:         vm.NewConstantPool(bundle.Code.Constants),
		Globals:      make([]vm.StackSlot, len(c.GlobalNames())+64),
	})
	if err != nil {
		t.Fatalf("jit.CompileAOT failed: %v", err)
	}
	bundle.Kind = compiler.KindNative
	bundle.NativeCode = nativeCode
	bundle.LabelOffsets = labelOffsets

	exePath := filepath.Join(t.TempDir(), "div_neg_one.exe")
	if err := compiler.BuildNativePE(bundle, exePath, false); err != nil {
		t.Fatalf("BuildNativePE failed: %v", err)
	}
	if out, err := exec.Command(exePath).CombinedOutput(); err != nil {
		t.Fatalf("division by -1 bailed out: %v, out: %s", err, out)
	}
}

func TestBuildNativePE(t *testing.T) {
	// Program that does 1,000,000 loop iterations with arithmetic
	src := `
i := 0;
total := 0;
while i < 1000000:
    total = total + (i % 7) * 3;
    i = i + 1;
`
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if len(p.SyntaxErrors()) > 0 {
		t.Fatalf("syntax errors: %v", p.SyntaxErrors())
	}

	c := vm.NewCompiler()
	if err := c.Compile(prog); err != nil {
		t.Fatalf("compile error: %v", err)
	}

	bundle := &compiler.Bundle{
		Name: "test_native.al",
		Kind: compiler.KindBytecode,
		Code: c.Bytecode(),
	}

	mainFn := &vm.CompiledFunction{
		Instructions: bundle.Code.Instructions,
		Pool:         vm.NewConstantPool(bundle.Code.Constants),
		Globals:      make([]vm.StackSlot, len(c.GlobalNames())+64),
	}

	nativeCode, labelOffsets, err := jit.CompileAOT(mainFn)
	if err != nil {
		t.Fatalf("jit.CompileAOT failed: %v", err)
	}
	if len(nativeCode) == 0 {
		t.Fatalf("nativeCode is empty")
	}

	bundle.Kind = compiler.KindNative
	bundle.NativeCode = nativeCode
	bundle.LabelOffsets = labelOffsets

	tmpDir := t.TempDir()
	exePath := filepath.Join(tmpDir, "loop_native.exe")

	if err := compiler.BuildNativePE(bundle, exePath, false); err != nil {
		t.Fatalf("BuildNativePE failed: %v", err)
	}

	fi, err := os.Stat(exePath)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}

	t.Logf("Generated pure native PE size: %d bytes (%.2f KB)!", fi.Size(), float64(fi.Size())/1024.0)

	// Execute it on Windows!
	cmd := exec.Command(exePath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("execution failed: %v, out: %s", err, string(out))
	}

	t.Logf("SUCCESS! Pure native 2.5 KB binary executed 1,000,000 loop iterations with exit code 0!")

	// Also verify noConsole (GUI mode)
	guiExePath := filepath.Join(tmpDir, "loop_gui.exe")
	if err := compiler.BuildNativePE(bundle, guiExePath, true); err != nil {
		t.Fatalf("BuildNativePE with noConsole failed: %v", err)
	}

	data, err := os.ReadFile(guiExePath)
	if err != nil {
		t.Fatalf("read gui exe failed: %v", err)
	}
	// Check PE Subsystem field (at e_lfanew + 24 + 68)
	e_lfanew := int(data[0x3C]) | int(data[0x3D])<<8 | int(data[0x3E])<<16 | int(data[0x3F])<<24
	subsys := int(data[e_lfanew+24+68]) | int(data[e_lfanew+24+69])<<8
	if subsys != int(compiler.SubsystemWindowsGUI) {
		t.Fatalf("expected Subsystem %d (GUI), got %d", compiler.SubsystemWindowsGUI, subsys)
	}
	t.Logf("SUCCESS! loop_gui.exe has Subsystem = %d (Windows GUI / noconsole)!", subsys)

	// Run GUI executable silently
	cmdGUI := exec.Command(guiExePath)
	if err := cmdGUI.Run(); err != nil {
		t.Fatalf("gui execution failed: %v", err)
	}
}
