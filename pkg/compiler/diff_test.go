// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

package compiler_test

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/pawl-lang/pawl/pkg/compiler"
	"github.com/pawl-lang/pawl/pkg/interpreter"
	"github.com/pawl-lang/pawl/pkg/lexer"
	"github.com/pawl-lang/pawl/pkg/parser"
)

func runInterpreter(t *testing.T, src string) string {
	t.Helper()
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if len(p.SyntaxErrors()) > 0 {
		t.Fatalf("parser errors: %v", p.SyntaxErrors())
	}

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	done := make(chan string)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()

	interp := interpreter.New()
	_ = interp.Eval(prog)

	w.Close()
	os.Stdout = oldStdout

	return <-done
}

func runAOT(t *testing.T, src string) string {
	t.Helper()
	if runtime.GOARCH != "amd64" {
		t.Skip("skipping AOT execution on non-amd64")
	}

	dir := t.TempDir()
	srcFile := filepath.Join(dir, "prog.al")
	if err := os.WriteFile(srcFile, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	exeExt := ""
	if runtime.GOOS == "windows" {
		exeExt = ".exe"
	}
	exeFile := filepath.Join(dir, "prog"+exeExt)

	err := compiler.Build(srcFile, compiler.Options{
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
		t.Fatalf("execution failed: %v, out: %s", err, string(out))
	}
	return string(out)
}

func TestDifferentialAOTvsInterpreter(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{
			name: "strings_println",
			src: `
println("differential line 1")
println("differential line 2")
`,
		},
		{
			name: "loop_prints",
			src: `
fn count_to(n: int) {
    for i in 1..=n {
        println("step")
    }
}
count_to(3)
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interpOut := runInterpreter(t, tt.src)
			aotOut := runAOT(t, tt.src)

			// Normalize line endings
			normInterp := strings.ReplaceAll(interpOut, "\r\n", "\n")
			normAOT := strings.ReplaceAll(aotOut, "\r\n", "\n")

			if normInterp != normAOT {
				t.Fatalf("mismatch!\nInterpreter:\n%q\nAOT:\n%q", normInterp, normAOT)
			}
		})
	}
}
