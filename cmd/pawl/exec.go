// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pawl-lang/pawl/pkg/ast"
	"github.com/pawl-lang/pawl/pkg/compiler"
	"github.com/pawl-lang/pawl/pkg/diag"
	"github.com/pawl-lang/pawl/pkg/interpreter"
	"github.com/pawl-lang/pawl/pkg/jit"
	"github.com/pawl-lang/pawl/pkg/lexer"
	"github.com/pawl-lang/pawl/pkg/mod"
	"github.com/pawl-lang/pawl/pkg/parser"
	"github.com/pawl-lang/pawl/pkg/sandbox"
	"github.com/pawl-lang/pawl/pkg/stdlib"
	"github.com/pawl-lang/pawl/pkg/typecheck"
	"github.com/pawl-lang/pawl/pkg/vm"
)

type engine int

const (
	engineAuto engine = iota // VM, falling back to the interpreter for unsupported features
	engineVM
	engineInterp
)

// loadProgram reads, parses and strictly type-checks a source file. Any diagnostic aborts.
func loadProgram(filename string) (*ast.Program, string, bool) {
	data, err := os.ReadFile(filename)
	if err != nil {
		fmt.Fprint(os.Stderr, diag.FormatIOReport(filename, "read", filename, err, isTerminal(os.Stderr)))
		return nil, "", false
	}
	// Project-local imports may read freely; anything outside the program's
	// directory needs a read capability. This is set before checking so the
	// checker sees the same resolution the engines will use.
	if abs, err := filepath.Abs(filename); err == nil {
		mod.SetProjectRoot(filepath.Dir(abs))
	}
	content := string(data)
	p := parser.New(lexer.New(content))
	prog := p.ParseProgram()
	if len(p.SyntaxErrors()) > 0 {
		fmt.Fprint(os.Stderr, p.DiagnosticReport(filename, content, isTerminal(os.Stderr)))
		return nil, content, false
	}
	checker := typecheck.NewChecker(filename, content)
	if errs := checker.Check(prog); len(errs) > 0 {
		fmt.Fprint(os.Stderr, diag.FormatReport(filename, content, errs, isTerminal(os.Stderr)))
		return nil, content, false
	}
	return prog, content, true
}

// cmdRun implements `pawl run`.
func cmdRun(args []string) int {
	eng := engineAuto
	if jitEng := jit.Default(); jitEng != nil {
		jitEng.SetEnabled(false)
	}

	var policyArgs, scriptArgs []string
	var filename string
	var memFlags memoryFlags
	foundFilename := false
	stopFlagParsing := false

	for len(args) > 0 {
		a := args[0]
		if stopFlagParsing {
			scriptArgs = append(scriptArgs, a)
			args = args[1:]
			continue
		}

		switch {
		case a == "--":
			stopFlagParsing = true
		case a == "--vm":
			eng = engineVM
		case a == "--jit":
			eng = engineVM
			if jitEng := jit.Default(); jitEng != nil {
				jitEng.SetEnabled(true)
				jitEng.SetThreshold(1)
			}
		case a == "--no-jit":
			if jitEng := jit.Default(); jitEng != nil {
				jitEng.SetEnabled(false)
			}
		case a == "--interp":
			eng = engineInterp
		case strings.HasPrefix(a, "--max-memory=") || strings.HasPrefix(a, "--max-allocation="):
			parsed, remaining, err := parseMemoryFlags(args)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				return 1
			}
			memFlags = parsed
			args = remaining
			continue
		case a == "--allow-all" || strings.HasPrefix(a, "--allow-"):
			policyArgs = append(policyArgs, a)
		default:
			if !foundFilename {
				filename = a
				foundFilename = true
			} else {
				scriptArgs = append(scriptArgs, a)
			}
		}
		args = args[1:]
	}

	applyMemoryFlags(memFlags)
	policy, leftover, err := sandbox.ParseArgs(policyArgs)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		fmt.Fprintln(os.Stderr, "Usage: pawl run [--vm|--interp] ["+sandbox.FlagHint+"] <file.al> [args...]")
		return 1
	}
	if len(leftover) > 0 {
		scriptArgs = append(leftover, scriptArgs...)
	}
	if filename == "" {
		fmt.Fprintln(os.Stderr, "Usage: pawl run [--vm|--interp] ["+sandbox.FlagHint+"] <file.al> [args...]")
		return 1
	}
	previousPolicy := sandbox.CurrentPolicy()
	sandbox.SetPolicy(policy)
	defer sandbox.SetPolicy(previousPolicy)
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprint(os.Stderr, diag.FormatPanicReport(filename, r, isTerminal(os.Stderr)))
		}
	}()
	stdlib.SetScriptArgs(scriptArgs)

	prog, content, ok := loadProgram(filename)
	if !ok {
		return 1
	}
	code, _ := execute(filename, content, prog, eng, false)
	return code
}

// execute runs a checked program. It returns the exit code and whether the VM was used.
func execute(filename, content string, prog *ast.Program, eng engine, quiet bool) (exitCode int, usedVM bool) {
	defer stdlib.CloseAllDatabases()
	defer func() {
		if err := interpreter.WaitAll(); err != nil {
			fmt.Fprintf(os.Stderr, "asynchronous operation failed: %v\n", err)
			exitCode = 1
		}
	}()
	if eng != engineInterp {
		c := vm.NewCompiler()
		err := c.Compile(prog)
		if err == nil {
			interp := newInterpreter()
			interp.SetFilename(filename)
			interp.SetSource(content)
			machine := vm.NewWithEnv(c.Bytecode(), interp.Env())
			machine.SetSourceFile(filename)
			if rerr := machine.Run(); rerr != nil {
				fmt.Fprint(os.Stderr, diag.FormatRuntimeError(filename, content, rerr, isTerminal(os.Stderr)))
				return 1, true
			}
			if machine.TestFailures > 0 {
				return 1, true
			}
			return 0, true
		}
		if uerr, isUnsupported := err.(*vm.UnsupportedError); isUnsupported {
			if eng == engineVM {
				fmt.Fprint(os.Stderr, diag.FormatUnsupportedFeature(filename, uerr.Feature, uerr.Line, isTerminal(os.Stderr)))
				return 1, false
			}
			if !quiet {
				fmt.Fprintf(os.Stderr, "note: %v; running on the interpreter\n", err)
			}
		} else {
			fmt.Fprint(os.Stderr, diag.FormatCompilerError(filename, err, isTerminal(os.Stderr)))
			return 1, false
		}
	}
	interp := newInterpreter()
	interp.SetFilename(filename)
	interp.SetSource(content)
	if err := interp.Execute(prog); err != nil {
		fmt.Fprint(os.Stderr, diag.FormatRuntimeError(filename, content, err, isTerminal(os.Stderr)))
		return 1, false
	}
	return 0, false
}

// testFile runs the whole program (so tests can use the functions defined next to them)
// and summarises the PASS/FAIL results of its test blocks.
func testFile(filename string) int {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprint(os.Stderr, diag.FormatPanicReport(filename, r, isTerminal(os.Stderr)))
		}
	}()
	prog, content, ok := loadProgram(filename)
	if !ok {
		return 1
	}
	hasTests := false
	for _, st := range prog.Statements {
		if _, ok := st.(*ast.TestStatement); ok {
			hasTests = true
		}
	}
	if !hasTests {
		fmt.Printf("No test blocks found in %s\n", filename)
		return 0
	}
	c := vm.NewCompiler()
	if err := c.Compile(prog); err != nil {
		if _, ok := err.(*vm.UnsupportedError); !ok {
			fmt.Fprint(os.Stderr, diag.FormatCompilerError(filename, err, isTerminal(os.Stderr)))
			return 1
		}
		// The VM cannot run this program (for example it declares classes).
		// Tests still work on the reference interpreter, which knows how to
		// report and continue after a failed test block.
		fmt.Fprintf(os.Stderr, "note: %v; running tests on the interpreter\n", err)
		interp := newInterpreter()
		interp.SetFilename(filename)
		interp.SetSource(content)
		interp.SetTestMode(true)
		defer stdlib.CloseAllDatabases()
		if rerr := interp.Execute(prog); rerr != nil {
			fmt.Fprint(os.Stderr, diag.FormatRuntimeError(filename, content, rerr, isTerminal(os.Stderr)))
			return 1
		}
		return printTestSummary(filename, interp.TestPasses, interp.TestFailures)
	}
	interp := newInterpreter()
	interp.SetFilename(filename)
	interp.SetSource(content)
	machine := vm.NewWithEnv(c.Bytecode(), interp.Env())
	machine.SetSourceFile(filename)
	defer stdlib.CloseAllDatabases()
	if err := machine.Run(); err != nil {
		fmt.Fprint(os.Stderr, diag.FormatRuntimeError(filename, content, err, isTerminal(os.Stderr)))
		return 1
	}
	return printTestSummary(filename, machine.TestPasses, machine.TestFailures)
}

func printTestSummary(filename string, passes, failures int) int {
	fmt.Printf("\nTest Summary for %s:\n", filename)
	fmt.Printf("  Total:  %d\n", passes+failures)
	fmt.Printf("  Passed: %d\n", passes)
	fmt.Printf("  Failed: %d\n", failures)
	if failures > 0 {
		return 1
	}
	return 0
}

// ---------------------------------------------------------------------------
// Standalone executables (deprecated, retained as no-op stubs if referenced)
// ---------------------------------------------------------------------------

func isStandaloneExecutable() bool {
	return false
}

func runStandalone() int {
	return 0
}

// ---------------------------------------------------------------------------
// build / cross-compilation
// ---------------------------------------------------------------------------

// moduleImports returns non-std `use` paths of a program.
func moduleImports(prog *ast.Program) []string {
	var out []string
	var walk func(stmts []ast.Statement)
	walk = func(stmts []ast.Statement) {
		for _, st := range stmts {
			switch s := st.(type) {
			case *ast.UseStatement:
				if !strings.HasPrefix(s.ModulePath, "std.") && s.ModulePath != "std" {
					out = append(out, s.ModulePath)
				}
			case *ast.BlockStatement:
				walk(s.Statements)
			}
		}
	}
	walk(prog.Statements)
	return out
}

// collectModules gathers imported .al files (recursively), following local
// paths and installed APM packages. It records the module-path -> relative
// file map so the standalone runtime can resolve `use pkg` without the
// machine's registry.
func collectModules(root, file string, prog *ast.Program, files, modules map[string]string, seen map[string]string) error {
	for _, imp := range moduleImports(prog) {
		if _, ok := modules[imp]; ok {
			continue
		}
		resolved, found, err := mod.Resolve(imp, file)
		if err != nil {
			return fmt.Errorf("cannot resolve module %q: %w", imp, err)
		}
		if !found || resolved.SourcePath == "" {
			return fmt.Errorf("cannot load module %q: not found locally or in the APM registry", imp)
		}
		abs := resolved.SourcePath
		if rel, ok := seen[abs]; ok {
			modules[imp] = rel
			continue
		}
		var rel string
		if resolved.Package != "" {
			within := ""
			if resolved.Root != "" {
				if r, err := filepath.Rel(resolved.Root, abs); err == nil {
					within = filepath.ToSlash(r)
				}
			}
			if within == "" || strings.HasPrefix(within, "..") {
				return fmt.Errorf("package module %q has an unexpected location", imp)
			}
			rel = "__apm/" + resolved.Package + "/" + resolved.Version + "/" + within
		} else {
			r, err := filepath.Rel(root, abs)
			if err != nil || strings.HasPrefix(r, "..") {
				return fmt.Errorf("module %q is outside the project directory %q", imp, root)
			}
			rel = filepath.ToSlash(r)
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			return fmt.Errorf("cannot read module %q: %w", imp, err)
		}
		files[rel] = string(data)
		modules[imp] = rel
		seen[abs] = rel
		p := parser.New(lexer.New(string(data)))
		sub := p.ParseProgram()
		if len(p.SyntaxErrors()) > 0 {
			return fmt.Errorf("module %q has syntax errors", imp)
		}
		if err := collectModules(root, abs, sub, files, modules, seen); err != nil {
			return err
		}
	}
	return nil
}

// usesRuntimeStdlib reports whether a program imports standard library modules
// that require the full runtime engine (GUI, HTTP/Net, DB, etc.).
func usesRuntimeStdlib(prog *ast.Program) bool {
	var walk func(stmts []ast.Statement) bool
	walk = func(stmts []ast.Statement) bool {
		for _, st := range stmts {
			switch s := st.(type) {
			case *ast.UseStatement:
				if strings.HasPrefix(s.ModulePath, "std.") || s.ModulePath == "std" {
					return true
				}
			case *ast.BlockStatement:
				if walk(s.Statements) {
					return true
				}
			}
		}
		return false
	}
	return walk(prog.Statements)
}

func collectEmbeddedResources(root string, paths []string) (map[string][]byte, error) {
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve project directory %q: %w", root, err)
	}
	resources := make(map[string][]byte)
	for _, path := range paths {
		source := path
		if !filepath.IsAbs(source) {
			source = filepath.Join(root, source)
		}
		source, err := filepath.Abs(source)
		if err != nil {
			return nil, fmt.Errorf("cannot resolve embedded path %q: %w", path, err)
		}
		rel, err := filepath.Rel(root, source)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			return nil, fmt.Errorf("embedded path %q must be inside project directory %q", path, root)
		}
		info, err := os.Lstat(source)
		if err != nil {
			return nil, fmt.Errorf("cannot access embedded path %q: %w", path, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("embedded path %q is a symbolic link; embed its target explicitly from within the project", path)
		}
		resolvedSource, err := filepath.EvalSymlinks(source)
		if err != nil {
			return nil, fmt.Errorf("cannot resolve embedded path %q: %w", path, err)
		}
		resolvedRel, err := filepath.Rel(root, resolvedSource)
		if err != nil || resolvedRel == ".." || strings.HasPrefix(resolvedRel, ".."+string(filepath.Separator)) || filepath.IsAbs(resolvedRel) {
			return nil, fmt.Errorf("embedded path %q resolves outside project directory %q", path, root)
		}
		addFile := func(file string) error {
			fileInfo, err := os.Lstat(file)
			if err != nil {
				return err
			}
			if fileInfo.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("embedded resource %q is a symbolic link", file)
			}
			if !fileInfo.Mode().IsRegular() {
				return fmt.Errorf("embedded resource %q is not a regular file", file)
			}
			fileRel, err := filepath.Rel(root, file)
			if err != nil || fileRel == ".." || strings.HasPrefix(fileRel, ".."+string(filepath.Separator)) || filepath.IsAbs(fileRel) {
				return fmt.Errorf("embedded resource %q is outside project directory %q", file, root)
			}
			data, err := os.ReadFile(file)
			if err != nil {
				return fmt.Errorf("cannot read embedded resource %q: %w", file, err)
			}
			resources[filepath.ToSlash(fileRel)] = data
			return nil
		}
		if info.IsDir() {
			err = filepath.WalkDir(source, func(file string, entry os.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if entry.Type()&os.ModeSymlink != 0 {
					return fmt.Errorf("embedded resource %q is a symbolic link", file)
				}
				if entry.IsDir() {
					return nil
				}
				return addFile(file)
			})
		} else {
			err = addFile(source)
		}
		if err != nil {
			return nil, fmt.Errorf("cannot embed %q: %w", path, err)
		}
	}
	return resources, nil
}

// cmdBuild implements `pawl build`.
func cmdBuild(args []string) int {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprint(os.Stderr, diag.FormatPanicReport("build", r, isTerminal(os.Stderr)))
		}
	}()

	var opts compiler.Options
	var input string

	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-o" && i+1 < len(args):
			opts.Output = args[i+1]
			i++
		case (a == "--target" || a == "-t") && i+1 < len(args):
			t, err := compiler.ParseTarget(args[i+1])
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				return 1
			}
			opts.Target = t
			i++
		case a == "--shared" || a == "--dll":
			opts.Kind = compiler.Shared
		case a == "--static":
			opts.Kind = compiler.Static
		case a == "-O0":
			opts.OptLevel = 0
		case a == "-O1":
			opts.OptLevel = 1
		case a == "-O2":
			opts.OptLevel = 2
		case a == "-O3":
			opts.OptLevel = 3
		case a == "-g":
			opts.Debug = true
		case a == "--no-console" || a == "--noconsole" || a == "--gui" || a == "-noconsole":
			opts.NoConsole = true
		case (a == "-l" || a == "--lib") && i+1 < len(args):
			opts.Libs = append(opts.Libs, args[i+1])
			i++
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(os.Stderr, "unknown build option %q\n", a)
			return 1
		default:
			input = a
		}
	}

	if input == "" {
		fmt.Fprintln(os.Stderr, "Usage: pawl build <file.al> [-o output] [--target os/arch] [--shared|--static] [-O2] [-g] [--no-console]")
		return 1
	}

	if opts.Target.OS == "" {
		opts.Target = compiler.HostTarget()
	}

	if err := compiler.Build(input, opts); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	out := opts.Output
	if out == "" {
		out = compiler.DefaultOutput(input, opts.Target, opts.Kind)
	}
	fmt.Printf("[OK] %s (%s, %s)\n", out, opts.Target, opts.Kind)
	return 0
}

// cmdTargets lists supported targets.
func cmdTargets() int {
	host := compiler.HostTarget()
	fmt.Printf("Host: %s\n\nSupported Targets:\n", host)
	for _, t := range compiler.SupportedTargets {
		fmt.Printf("  %-17s\n", t)
	}
	return 0
}
