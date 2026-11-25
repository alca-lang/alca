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
		{
			name: "list_creation_indexing_iteration",
			src: `
xs := [10, 20, 30]
println(len(xs))
println(xs[0])
println(xs[1])
println(xs[2])
for item in xs {
    println(item)
}
`,
		},
		{
			name: "struct_and_field_mutation",
			src: `
struct Point {
    x: int
    y: int
}
pt := Point(5, 12)
println(pt.x)
println(pt.y)
pt.x = 42
println(pt.x)
`,
		},
		{
			name: "class_and_methods",
			src: `
class Counter {
    count: int

    func bump() -> int {
        self.count = self.count + 1
        return self.count
    }
}
c := Counter(10)
println(c.count)
println(c.bump())
println(c.bump())
println(c.count)
`,
		},
		{
			name: "enums_and_match",
			src: `
enum Color {
    Red,
    Green,
    Blue,
}
c := Color.Green
match c {
    case Color.Red {
        println(1)
    }
    case Color.Green {
        println(2)
    }
    else {
        println(3)
    }
}
`,
		},
		{
			name: "null_and_nullable",
			src: `
struct Box {
    val: int
}
b: Box | null = null
if b == null {
    println("is null")
}
b = Box(77)
if b != null {
    println(b.val)
}
`,
		},
		{
			name: "error_instantiation_and_stream_capture",
			src: `
func validate(val: int) -> int {
    if val < 0 {
        err("invalid number", code=422)
        return 0
    }
    return val * 2
}
e: error | null = null
res := validate(-10) ?>>> e
if e != null {
    println(e.code)
    println(e.msg)
}
`,
		},
		{
			name: "map_operations",
			src: `
m: map[string, int] = {"alpha": 10, "beta": 20}
println(len(m))
println(m["alpha"])
println(m["beta"])
m["gamma"] = 30
println(len(m))
println(m["gamma"])
`,
		},
		{
			name: "math_stdlib",
			src: `
use std.math

println(math.min(10, 25))
println(math.max(10, 25))
println(math.abs(-42))
println(math.abs(99))
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
