// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

package compiler_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/pawl-lang/pawl/pkg/compiler"
)

func TestAOTPrintln(t *testing.T) {
	if runtime.GOARCH != "amd64" {
		t.Skip("skipping amd64 test on other architecture")
	}

	src := `
println("Hello from Pawl AOT compiler!")
println("Line 2 of output")
`
	dir := t.TempDir()
	sourceFile := filepath.Join(dir, "hello.al")
	if err := os.WriteFile(sourceFile, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	exeFile := filepath.Join(dir, "hello.exe")

	err := compiler.Build(sourceFile, compiler.Options{
		Target: compiler.HostTarget(),
		Kind:   compiler.Exe,
		Output: exeFile,
	})
	if err != nil {
		t.Fatalf("compiler.Build failed: %v", err)
	}

	cmd := exec.Command(exeFile)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("cmd: %s", exeFile)
		t.Fatalf("execution failed: %v, output: %s", err, string(out))
	}

	expected := "Hello from Pawl AOT compiler!\r\nLine 2 of output\r\n"
	if runtime.GOOS != "windows" {
		expected = "Hello from Pawl AOT compiler!\nLine 2 of output\n"
	}

	outStr := strings.ReplaceAll(string(out), "\r\n", "\n")
	expStr := strings.ReplaceAll(expected, "\r\n", "\n")

	if outStr != expStr {
		t.Fatalf("unexpected stdout: got %q, want %q", outStr, expStr)
	}
}

func TestAOTCrossBuildLinuxELFWithOutput(t *testing.T) {
	src := `
println("cross-compiled linux string")
`
	dir := t.TempDir()
	sourceFile := filepath.Join(dir, "linux_test.al")
	if err := os.WriteFile(sourceFile, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	elfFile := filepath.Join(dir, "linux_app")
	err := compiler.Build(sourceFile, compiler.Options{
		Target: compiler.Target{OS: "linux", Arch: "amd64"},
		Kind:   compiler.Exe,
		Output: elfFile,
	})
	if err != nil {
		t.Fatalf("ELF build failed: %v", err)
	}

	elfBytes, err := os.ReadFile(elfFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(elfBytes) < 4 || string(elfBytes[:4]) != "\x7fELF" {
		t.Fatalf("not an ELF binary")
	}
}
