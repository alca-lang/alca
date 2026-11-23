// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCmdBuildHelpAndFlags(t *testing.T) {
	// Running pawl build without arguments prints usage and returns 1
	code := cmdBuild([]string{})
	if code != 1 {
		t.Fatalf("cmdBuild with no args returned %d, want 1", code)
	}

	// Unknown option returns 1
	code = cmdBuild([]string{"--bad-flag", "test.al"})
	if code != 1 {
		t.Fatalf("cmdBuild with bad flag returned %d, want 1", code)
	}
}

func TestCmdBuildInvocation(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "sample.al")
	if err := os.WriteFile(src, []byte("println(42)\n"), 0644); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(dir, "sample_out.exe")
	// cmdBuild invokes compiler.Build
	code := cmdBuild([]string{src, "-o", out, "--target", "windows/amd64"})
	// In stage 1, compiler.Build reaches stage 2 generator hook and reports error
	if code == 0 {
		t.Fatal("expected non-zero exit code while codegen is being connected in stage 2")
	}
}
