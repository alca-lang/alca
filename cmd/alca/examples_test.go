// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alca-lang/alca/pkg/diag"
	"github.com/alca-lang/alca/pkg/lexer"
	"github.com/alca-lang/alca/pkg/parser"
	"github.com/alca-lang/alca/pkg/typecheck"
)

// repoRoot locates the repository root by walking up until go.mod is found.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repository root (go.mod) not found above the test working directory")
		}
		dir = parent
	}
}

// collectAlFiles returns every .al file under root, sorted.
func collectAlFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".al") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return files
}

// diagnosticsOf parses and type-checks a file, returning every diagnostic.
func diagnosticsOf(path string) ([]*diag.SyntaxError, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	content := string(data)
	p := parser.New(lexer.New(content))
	prog := p.ParseProgram()
	if errs := p.SyntaxErrors(); len(errs) > 0 {
		return errs, nil
	}
	checker := typecheck.NewChecker(path, content)
	return checker.Check(prog), nil
}

// TestAllShippedAlFilesCheckCleanly locks in the P0 migration: every example,
// benchmark and shipped test program must pass strict parsing and type-checking.
// The only exceptions are the deliberately-invalid fixtures in tests/invalid.
func TestAllShippedAlFilesCheckCleanly(t *testing.T) {
	root := repoRoot(t)
	var files []string
	for _, dir := range []string{"examples", "benchmarks"} {
		files = append(files, collectAlFiles(t, filepath.Join(root, dir))...)
	}
	// Top-level shipped test programs (skipping tests/invalid on purpose).
	entries, err := os.ReadDir(filepath.Join(root, "tests"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".al") {
			files = append(files, filepath.Join(root, "tests", e.Name()))
		}
	}
	if len(files) < 10 {
		t.Fatalf("expected the shipped .al corpus to be migrated, found only %d files", len(files))
	}
	for _, file := range files {
		rel, _ := filepath.Rel(root, file)
		t.Run(rel, func(t *testing.T) {
			errs, err := diagnosticsOf(file)
			if err != nil {
				t.Fatal(err)
			}
			if len(errs) > 0 {
				for _, e := range errs {
					t.Errorf("%s:%d:%d: %s", rel, e.Line, e.Column, e.Message)
				}
			}
		})
	}
}

// TestInvalidFixturesAreRejected checks that the negative fixtures still make
// the parser/type-checker fail with the intended class of diagnostic.
func TestInvalidFixturesAreRejected(t *testing.T) {
	root := repoRoot(t)
	files, err := filepath.Glob(filepath.Join(root, "tests", "invalid", "*.al"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no fixtures found under tests/invalid (err=%v)", err)
	}
	wantSubstring := map[string]string{
		"parse_missing_semicolon.al": "expected ';'",
		"parse_missing_colon.al":     "expected ':'",
		"type_bad_assignment.al":     "cannot assign",
		"type_bad_return.al":         "return",
		"type_bad_argument.al":       "argument",
		"type_loose_equality.al":     "cannot compare",
		"type_shadow_assignment.al":  "already declared",
	}
	for _, file := range files {
		name := filepath.Base(file)
		t.Run(name, func(t *testing.T) {
			errs, err := diagnosticsOf(file)
			if err != nil {
				t.Fatal(err)
			}
			if len(errs) == 0 {
				t.Fatalf("%s must not pass the checker", name)
			}
			want, ok := wantSubstring[name]
			if !ok {
				t.Fatalf("fixture %s has no expected diagnostic registered", name)
			}
			for _, e := range errs {
				if strings.Contains(e.Message, want) {
					return
				}
			}
			t.Errorf("no diagnostic containing %q; got %v", want, errs)
		})
	}
}

// runWithTempCwd executes fn with the process working directory moved to a
// fresh temporary directory, so example programs cannot write into the repo.
func runWithTempCwd(t *testing.T, fn func()) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatal(err)
		}
	}()
	fn()
}

// runExample executes a checked program on one engine and captures its stdout.
func runExample(t *testing.T, path string, eng engine) (string, int) {
	t.Helper()
	prog, content, ok := loadProgram(path)
	if !ok {
		t.Fatalf("%s failed to parse/type-check", path)
	}
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	done := make(chan string)
	go func() { b, _ := io.ReadAll(r); done <- string(b) }()
	code, _ := execute(path, content, prog, eng, true)
	w.Close()
	os.Stdout = old
	return <-done, code
}

// examplesWithDeterministicOutput is the subset of examples whose stdout is
// byte-identical when run twice in the same process; the VM and the interpreter
// must agree exactly on all of them.
var examplesWithDeterministicOutput = []string{
	"examples/hello.al",
	"examples/app.al",
	"examples/demo.al",
	"examples/gui_demo.al",
}

// goldenOutputs holds exact byte-for-byte expected outputs for examples,
// guaranteeing that semantic changes in ranges, operators or builtins do not pass silently.
var goldenOutputs = map[string]string{
	"examples/hello.al":    "listening on 127.0.0.1:8080\n12.566370614359172\nhandled: division by zero\n55\n",
	"examples/app.al":      "=== Starting Alca Production App ===\nHello from Alca Native Executable!\ncounter = 3\nServer IP: 192.168.1.100\nServer port: 8080\nCalculation: 15 + 27 = 42\nChecking node: web1\nChecking node: web2\nChecking node: database\nLogged operational event to ./audit.log via stream operator '>>'\nBuffer allocated: High memory temporary buffer\nBuffer instantly freed via null assignment: OK\n=== Alca Application Finished Successfully ===\n",
	"examples/demo.al":     "Alca Language v2.0\nAnswer: 42\nUnion value: Dynamic text\nNull value: null\nServer configured at 127.0.0.1:8080\nmultiply(21) = 42\nCaptured stream text\n\nCaptured value was: Captured stream text\nItem: alpha\nItem: beta\nItem: gamma\nAllocated buffer: Temporary payload\nFreed buffer instantaneously: buffer is null\n10 / 2 = 5\nHandled error gracefully: division by zero forbidden\nDemo complete!\n",
	"examples/gui_demo.al": "=== Alca GUI Module Loaded ===\n1. Native dialogs API ready: gui.alert, gui.confirm, gui.prompt, gui.open_file, gui.save_file, gui.notify\n2. Desktop WebView API ready: gui.webview(title, htmlOrURL, width, height)\n3. Native Windows & Widgets API ready: win = gui.window(title, w, h)\n   - win.add_label(text)\n   - win.add_button(text, callback)\n   - win.add_input(placeholder)\n   - win.add_checkbox(text, checked)\n   - win.add_progress(percent)\n   - win.show() / win.run()\n=== GUI Verification Complete ===\n",
}

// TestExamplesEnginesAgree runs deterministic examples on both engines and
// requires identical output, matching golden references, and a zero exit status from each.
func TestExamplesEnginesAgree(t *testing.T) {
	root := repoRoot(t)
	for _, rel := range examplesWithDeterministicOutput {
		rel := rel
		t.Run(rel, func(t *testing.T) {
			path := filepath.Join(root, filepath.FromSlash(rel))
			var vmOut, interpOut string
			var vmCode, interpCode int
			runWithTempCwd(t, func() {
				vmOut, vmCode = runExample(t, path, engineVM)
				interpOut, interpCode = runExample(t, path, engineInterp)
			})
			if vmCode != 0 {
				t.Errorf("[vm] exit code %d, output:\n%s", vmCode, vmOut)
			}
			if interpCode != 0 {
				t.Errorf("[interp] exit code %d, output:\n%s", interpCode, interpOut)
			}
			if vmOut != interpOut {
				t.Errorf("engine output mismatch\n vm: %q\nint: %q", vmOut, interpOut)
			}
			if expected, ok := goldenOutputs[rel]; ok {
				if vmOut != expected {
					t.Errorf("output does not match golden reference for %s:\n got: %q\nwant: %q", rel, vmOut, expected)
				}
			}
		})
	}
}

// TestExamplesRunWithoutCrash executes the remaining examples that touch the
// clock, randomness, the file system or the network: stdout may differ between
// runs, but both engines must complete successfully. Database demos are only
// attempted when the binary was built with drivers.
func TestExamplesRunWithoutCrash(t *testing.T) {
	root := repoRoot(t)
	examples := []string{
		"examples/stdlib_demo.al",
		"examples/stdlib_super_demo.al",
		"examples/full_capabilities.al",
		"examples/path_demo.al",
	}
	if dbDriversAvailable {
		examples = append(examples, "examples/db_sqlite_demo.al")
	}
	for _, rel := range examples {
		rel := rel
		t.Run(rel, func(t *testing.T) {
			path := filepath.Join(root, filepath.FromSlash(rel))
			for _, eng := range []struct {
				name string
				e    engine
			}{{"vm", engineVM}, {"interp", engineInterp}} {
				var code int
				runWithTempCwd(t, func() {
					_, code = runExample(t, path, eng.e)
				})
				if code != 0 {
					t.Errorf("[%s] exit code %d", eng.name, code)
				}
			}
		})
	}
}

// TestIntegrationTestFilePasses runs tests/integration_test.al end to end.
func TestIntegrationTestFilePasses(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "tests", "integration_test.al")
	code := testFile(path)
	if code != 0 {
		t.Fatalf("alca test %s exited with code %d", path, code)
	}
}
