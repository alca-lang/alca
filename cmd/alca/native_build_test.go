package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestNativeLibraryIsBuiltIntoTheProgram verifies the path of a C insertion: the
// build compiles the C source into a shared library, packs it into the bundle,
// and the program loads it from its own resources with ffi.load_embedded.
func TestNativeLibraryIsBuiltIntoTheProgram(t *testing.T) {
	if _, err := findNativeCompiler(); err != nil {
		t.Skip("no C compiler on this machine")
	}
	dir := t.TempDir()
	source := filepath.Join(dir, "native.c")
	if err := os.WriteFile(source, []byte("long alca_triple(long value) {\n    return value * 3;\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	program := filepath.Join(dir, "prog.al")
	if err := os.WriteFile(program, []byte("use std.ffi;\nlib := ffi.load_embedded(\"native\");\nprintln(lib.call(\"alca_triple\", 7));\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "app.exe")

	cmd := exec.Command("go", "run", "./cmd/alca", "build", "--cc", source, program, "-o", output)
	cmd.Dir = repoRoot(t)
	buildOutput, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %v\n%s", err, buildOutput)
	}
	if !strings.Contains(string(buildOutput), "native/native") {
		t.Fatalf("the build must report the native library:\n%s", buildOutput)
	}

	run := exec.Command(output)
	runOutput, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("the program failed: %v\n%s", err, runOutput)
	}
	if strings.TrimSpace(string(runOutput)) != "21" {
		t.Fatalf("output = %q, want %q", strings.TrimSpace(string(runOutput)), "21")
	}
}
