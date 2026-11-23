// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

package compiler_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pawl-lang/pawl/pkg/compiler"
)

func TestParseTarget(t *testing.T) {
	cases := []struct {
		input   string
		wantOS  string
		wantArch string
		wantErr bool
	}{
		{"linux/amd64", "linux", "amd64", false},
		{"windows-amd64", "windows", "amd64", false},
		{"darwin/arm64", "darwin", "arm64", false},
		{"unknown/os", "", "", true},
	}

	for _, c := range cases {
		tgt, err := compiler.ParseTarget(c.input)
		if c.wantErr && err == nil {
			t.Errorf("ParseTarget(%q) expected error, got nil", c.input)
		}
		if !c.wantErr && err != nil {
			t.Errorf("ParseTarget(%q) unexpected error: %v", c.input, err)
		}
		if !c.wantErr {
			if tgt.OS != c.wantOS || tgt.Arch != c.wantArch {
				t.Errorf("ParseTarget(%q) = %v, want %s/%s", c.input, tgt, c.wantOS, c.wantArch)
			}
		}
	}
}

func TestBuildValidation(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "async_sample.al")
	if err := os.WriteFile(src, []byte("async func foo() {\n  println(1)\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	err := compiler.Build(src, compiler.Options{
		Target: compiler.Target{OS: "windows", Arch: "amd64"},
		Kind:   compiler.Exe,
	})
	if err == nil {
		t.Fatal("expected error on unsupported 'async' feature, got nil")
	}
	if !strings.Contains(err.Error(), "native backend does not support 'async' yet") {
		t.Fatalf("unexpected error message: %v", err)
	}
}
