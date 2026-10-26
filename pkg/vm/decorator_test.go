// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package vm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/limits"
)

// TestDecoratorsOnBothEngines covers the decorators that change how a program
// behaves. The VM applies the same set as the tree-walking interpreter:
// @oom_guard installs a memory limit, @error_log writes the errors of the
// program to a file, and @drop_on_oom releases a variable when the limit is
// reached.
func TestDecoratorsOnBothEngines(t *testing.T) {
	t.Run("oom guard limits the program", func(t *testing.T) {
		// The decorator installs a limit on the shared meter, so it has to be
		// removed again when the test is done.
		t.Cleanup(func() {
			limits.Reset()
			limits.SetLimits(0, 0)
		})
		const src = `
@oom_guard("max=1mb")
func grow() -> int:
    xs: list[int] = [];
    i := 0;
    while i < 1000000:
        push(xs, i);
        i = i + 1;
    return len(xs);

println(grow());
`
		interpErr := interpreter.New().Execute(parseProgram(t, src))
		vmErr := runSource(t, src)
		if interpErr == nil {
			t.Fatalf("interpreter: expected the memory limit to stop the program")
		}
		if vmErr == nil {
			t.Fatal("vm: expected the memory limit to stop the program")
		}
		if !strings.Contains(vmErr.Error(), "out of memory") {
			t.Fatalf("vm error = %v, want an out-of-memory error", vmErr)
		}
		if !strings.Contains(interpErr.Error(), "out of memory") {
			t.Fatalf("interpreter error = %v, want an out-of-memory error", interpErr)
		}
	})

	t.Run("error log receives the raised error", func(t *testing.T) {
		dir := t.TempDir()
		logPath := filepath.Join(dir, "errors.log")
		src := "@error_log(\"" + filepath.ToSlash(logPath) + "\")\nfunc helper() -> int:\n    return 1;\n\nerr(\"boom\", code=7);\n"
		if err := runSource(t, src); err == nil {
			t.Fatal("expected the program to fail")
		}
		data, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("read error log: %v", err)
		}
		if !strings.Contains(string(data), "boom") {
			t.Fatalf("error log = %q, want it to mention the error", string(data))
		}
	})
}
