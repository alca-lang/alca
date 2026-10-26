// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alca-lang/alca/pkg/stdlib"
)

func TestCmdBuildDLL(t *testing.T) {
	_, err := stdlib.FindNativeCompiler(false)
	if err != nil {
		t.Skip("no host C compiler available:", err)
	}

	src := `
#ifdef _WIN32
#define EXPORT __declspec(dllexport)
#else
#define EXPORT __attribute__((visibility("default")))
#endif

EXPORT int alca_cli_add(int a, int b) {
    return a + b;
}
`
	tmpDir := t.TempDir()
	cFile := filepath.Join(tmpDir, "cli_test.c")
	if err := os.WriteFile(cFile, []byte(src), 0644); err != nil {
		t.Fatalf("failed to write C source: %v", err)
	}

	outDLL := filepath.Join(tmpDir, "cli_test"+stdlib.NativeLibraryExtension())
	code := cmdBuildDLL([]string{cFile, "-o", outDLL})
	if code != 0 {
		t.Fatalf("cmdBuildDLL exited with code %d", code)
	}

	if _, err := os.Stat(outDLL); err != nil {
		t.Fatalf("expected output DLL at %q, but not found: %v", outDLL, err)
	}
}

func TestCmdBuildPureAlcaDLL(t *testing.T) {
	src := `
pub func add_ten(x: int) -> int {
    return x + 10;
}
`
	tmpDir := t.TempDir()
	alFile := filepath.Join(tmpDir, "pure.al")
	if err := os.WriteFile(alFile, []byte(src), 0644); err != nil {
		t.Fatalf("failed to write Alca source: %v", err)
	}

	outDLL := filepath.Join(tmpDir, "pure.dll")
	code := cmdBuildDLL([]string{alFile, "-o", outDLL})
	if code != 0 {
		t.Fatalf("cmdBuildDLL exited with code %d", code)
	}

	if _, err := os.Stat(outDLL); err != nil {
		t.Fatalf("expected output DLL at %q, but not found: %v", outDLL, err)
	}
}
