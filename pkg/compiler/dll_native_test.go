// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

package compiler_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pawl-lang/pawl/pkg/compiler"
	"github.com/pawl-lang/pawl/pkg/stdlib"
)

func TestBuildNativeDLLFromPawl(t *testing.T) {
	src := `
pub func add(a: int, b: int) -> int {
    return a + b
}

pub func multiply(a: int, b: int) -> int {
    return a * b
}

pub func poly(x: int) -> int {
    x2 := x * x
    return x2 + 3 * x + 5
}
`
	tmpDir := t.TempDir()
	outDLL := filepath.Join(tmpDir, "pure_math.dll")

	err := compiler.BuildNativeDLLFromPawl(src, outDLL)
	if err != nil {
		t.Fatalf("BuildNativeDLLFromPawl failed: %v", err)
	}

	fi, err := os.Stat(outDLL)
	if err != nil {
		t.Fatalf("compiled DLL not found: %v", err)
	}
	if fi.Size() == 0 {
		t.Fatalf("compiled DLL is empty")
	}

	// Test loading via stdlib FFI
	handle, err := stdlib.LoadDynamicLibrary(outDLL)
	if err != nil {
		t.Fatalf("LoadDynamicLibrary failed on generated PE DLL: %v", err)
	}
	defer stdlib.UnloadDynamicLibrary(handle)
}
