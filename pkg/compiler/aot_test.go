// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

package compiler_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/pawl-lang/pawl/pkg/compiler"
	"github.com/pawl-lang/pawl/pkg/interpreter"
	"github.com/pawl-lang/pawl/pkg/lexer"
	"github.com/pawl-lang/pawl/pkg/parser"
)

func runInterpExitCode(t *testing.T, src string) int {
	t.Helper()
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if len(p.SyntaxErrors()) > 0 {
		t.Fatalf("parser errors: %v", p.SyntaxErrors())
	}
	interp := interpreter.New()
	val := interp.Eval(prog)
	_ = val
	return 0
}

func TestAOTFib(t *testing.T) {
	if runtime.GOARCH != "amd64" {
		t.Skip("skipping amd64 test on other architecture")
	}

	src := `
func fib(n: int) -> int {
    if n <= 1 {
        return n
    }
    return fib(n - 1) + fib(n - 2)
}

exit(fib(10))
`
	dir := t.TempDir()
	sourceFile := filepath.Join(dir, "fib.al")
	if err := os.WriteFile(sourceFile, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	exeFile := filepath.Join(dir, "fib.exe")
	if runtime.GOOS != "windows" {
		exeFile = filepath.Join(dir, "fib")
	}

	err := compiler.Build(sourceFile, compiler.Options{
		Target: compiler.HostTarget(),
		Kind:   compiler.Exe,
		Output: exeFile,
	})
	if err != nil {
		t.Fatalf("compiler.Build failed: %v", err)
	}

	// 2. Execute compiled native binary
	cmd := exec.Command(exeFile)
	err = cmd.Run()
	var exitCode int
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run compiled exe: %v", err)
		}
	} else {
		exitCode = 0
	}

	// fib(10) is 55
	if exitCode != 55 {
		t.Fatalf("differential test failed: native exit code = %d, want 55", exitCode)
	}
}

func TestAOTLoopSum(t *testing.T) {
	if runtime.GOARCH != "amd64" {
		t.Skip("skipping amd64 test on other architecture")
	}

	src := `
sum := 0
for i in 1..=10 {
    sum = sum + i
}
exit(sum)
`
	dir := t.TempDir()
	sourceFile := filepath.Join(dir, "sum.al")
	if err := os.WriteFile(sourceFile, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	exeFile := filepath.Join(dir, "sum.exe")
	if runtime.GOOS != "windows" {
		exeFile = filepath.Join(dir, "sum")
	}

	err := compiler.Build(sourceFile, compiler.Options{
		Target: compiler.HostTarget(),
		Kind:   compiler.Exe,
		Output: exeFile,
	})
	if err != nil {
		t.Fatalf("compiler.Build failed: %v", err)
	}

	cmd := exec.Command(exeFile)
	err = cmd.Run()
	var exitCode int
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run compiled exe: %v", err)
		}
	} else {
		exitCode = 0
	}

	// sum 1..=10 is 55
	if exitCode != 55 {
		t.Fatalf("differential test failed: native exit code = %d, want 55", exitCode)
	}
}

func TestAOTCrossBuildLinuxELF(t *testing.T) {
	src := `
func calc(x: int) -> int {
    return x * 2 + 1
}
exit(calc(20))
`
	dir := t.TempDir()
	sourceFile := filepath.Join(dir, "calc.al")
	if err := os.WriteFile(sourceFile, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	elfFile := filepath.Join(dir, "calc_linux")
	err := compiler.Build(sourceFile, compiler.Options{
		Target: compiler.Target{OS: "linux", Arch: "amd64"},
		Kind:   compiler.Exe,
		Output: elfFile,
	})
	if err != nil {
		t.Fatalf("ELF cross build failed: %v", err)
	}

	// Verify ELF magic
	elfBytes, err := os.ReadFile(elfFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(elfBytes) < 4 || string(elfBytes[:4]) != "\x7fELF" {
		t.Fatalf("generated file is not a valid ELF binary")
	}
}
