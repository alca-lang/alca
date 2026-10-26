// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package stdlib_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alca-lang/alca/pkg/stdlib"
)

func TestCompileSharedLibraryFromSource(t *testing.T) {
	_, err := stdlib.FindNativeCompiler(false)
	if err != nil {
		t.Skip("no host C compiler available:", err)
	}

	cCode := `
#ifdef _WIN32
#define EXPORT __declspec(dllexport)
#else
#define EXPORT __attribute__((visibility("default")))
#endif

EXPORT int add_test(int a, int b) {
    return a + b;
}
`
	tmpDir := t.TempDir()
	outDLL := filepath.Join(tmpDir, "test_add"+stdlib.NativeLibraryExtension())

	builtPath, err := stdlib.CompileSharedLibraryFromSource(cCode, outDLL, false)
	if err != nil {
		t.Fatalf("CompileSharedLibraryFromSource failed: %v", err)
	}

	if _, err := os.Stat(builtPath); err != nil {
		t.Fatalf("compiled DLL not found at %q: %v", builtPath, err)
	}
}

func TestCompileSharedLibraryFile(t *testing.T) {
	_, err := stdlib.FindNativeCompiler(false)
	if err != nil {
		t.Skip("no host C compiler available:", err)
	}

	cCode := `
#ifdef _WIN32
#define EXPORT __declspec(dllexport)
#else
#define EXPORT __attribute__((visibility("default")))
#endif

EXPORT int mul_test(int a, int b) {
    return a * b;
}
`
	tmpDir := t.TempDir()
	cFile := filepath.Join(tmpDir, "mul.c")
	if err := os.WriteFile(cFile, []byte(cCode), 0644); err != nil {
		t.Fatalf("failed to write C file: %v", err)
	}

	outDLL := filepath.Join(tmpDir, "mul"+stdlib.NativeLibraryExtension())
	builtPath, err := stdlib.CompileSharedLibrary(cFile, outDLL)
	if err != nil {
		t.Fatalf("CompileSharedLibrary failed: %v", err)
	}

	if _, err := os.Stat(builtPath); err != nil {
		t.Fatalf("compiled DLL not found at %q: %v", builtPath, err)
	}
}
