package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/alca-lang/alca/pkg/ast"
	"github.com/alca-lang/alca/pkg/compiler"
	"github.com/alca-lang/alca/pkg/diag"
	"github.com/alca-lang/alca/pkg/lexer"
	"github.com/alca-lang/alca/pkg/parser"
	"github.com/alca-lang/alca/pkg/stdlib"
	"github.com/alca-lang/alca/pkg/typecheck"
	"github.com/alca-lang/alca/pkg/vm"
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

// cmdRun implements `alca run`.
func cmdRun(args []string) int {
	eng := engineAuto
	var rest []string
	for i, a := range args {
		switch a {
		case "--vm":
			eng = engineVM
		case "--interp":
			eng = engineInterp
		default:
			rest = append(rest, args[i:]...)
			i = len(args)
		}
		if len(rest) > 0 {
			break
		}
	}
	if len(rest) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: alca run [--vm|--interp] <file.al> [args...]")
		return 1
	}
	filename, scriptArgs := rest[0], rest[1:]
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
func execute(filename, content string, prog *ast.Program, eng engine, quiet bool) (int, bool) {
	defer stdlib.CloseAllDatabases()
	if eng != engineInterp {
		c := vm.NewCompiler()
		err := c.Compile(prog)
		if err == nil {
			interp := newInterpreter()
			interp.SetFilename(filename)
			interp.SetSource(content)
			machine := vm.NewWithEnv(c.Bytecode(), interp.Env())
			if rerr := machine.Run(); rerr != nil {
				fmt.Fprint(os.Stderr, diag.FormatRuntimeError(filename, content, rerr, isTerminal(os.Stderr)))
				return 1, true
			}
			if machine.TestFailures > 0 {
				return 1, true
			}
			return 0, true
		}
		if _, isUnsupported := err.(*vm.UnsupportedError); !isUnsupported || eng == engineVM {
			fmt.Fprint(os.Stderr, diag.FormatCompilerError(filename, err, isTerminal(os.Stderr)))
			return 1, false
		}
		if !quiet {
			fmt.Fprintf(os.Stderr, "note: %v; running on the interpreter\n", err)
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
		fmt.Fprint(os.Stderr, diag.FormatCompilerError(filename, err, isTerminal(os.Stderr)))
		return 1
	}
	interp := newInterpreter()
	interp.SetFilename(filename)
	interp.SetSource(content)
	machine := vm.NewWithEnv(c.Bytecode(), interp.Env())
	defer stdlib.CloseAllDatabases()
	if err := machine.Run(); err != nil {
		fmt.Fprint(os.Stderr, diag.FormatRuntimeError(filename, content, err, isTerminal(os.Stderr)))
		return 1
	}
	fmt.Printf("\nTest Summary for %s:\n", filename)
	fmt.Printf("  Total:  %d\n", machine.TestPasses+machine.TestFailures)
	fmt.Printf("  Passed: %d\n", machine.TestPasses)
	fmt.Printf("  Failed: %d\n", machine.TestFailures)
	if machine.TestFailures > 0 {
		return 1
	}
	return 0
}

// ---------------------------------------------------------------------------
// Standalone executables
// ---------------------------------------------------------------------------

func isStandaloneExecutable() bool {
	exePath, err := os.Executable()
	if err != nil {
		return false
	}
	f, err := os.Open(exePath)
	if err != nil {
		return false
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || fi.Size() < int64(compiler.FooterLen) {
		return false
	}
	footer := make([]byte, compiler.FooterLen)
	if _, err := f.ReadAt(footer, fi.Size()-int64(compiler.FooterLen)); err != nil {
		return false
	}
	return compiler.VerifyMagic(footer)
}

// runStandalone executes the program embedded in this executable.
func runStandalone() int {
	exePath, err := os.Executable()
	if err != nil {
		fmt.Fprint(os.Stderr, diag.FormatIOReport("executable", "locate", "alca application binary", err, isTerminal(os.Stderr)))
		return 1
	}
	name := filepath.Base(exePath)
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprint(os.Stderr, diag.FormatPanicReport(name, r, isTerminal(os.Stderr)))
		}
	}()
	bundle, err := compiler.ExtractBundle(exePath)
	if err != nil {
		fmt.Fprint(os.Stderr, diag.FormatCompilerError(name, err, isTerminal(os.Stderr)))
		return 1
	}
	stdlib.SetScriptArgs(os.Args[1:])
	defer stdlib.CloseAllDatabases()

	if bundle.Kind == compiler.KindBytecode {
		interp := newInterpreter()
		interp.SetFilename(bundle.Name)
		machine := vm.NewWithEnv(bundle.Code, interp.Env())
		if err := machine.Run(); err != nil {
			fmt.Fprint(os.Stderr, diag.FormatRuntimeError(bundle.Name, "", err, isTerminal(os.Stderr)))
			return 1
		}
		return 0
	}

	// Source bundle: materialise the files so relative `use "./x.al"` imports resolve.
	dir, err := os.MkdirTemp("", "alca-app-")
	if err != nil {
		fmt.Fprint(os.Stderr, diag.FormatIOReport(name, "create", "temporary directory", err, isTerminal(os.Stderr)))
		return 1
	}
	defer os.RemoveAll(dir)
	for rel, text := range bundle.Files {
		dst := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err == nil {
			_ = os.WriteFile(dst, []byte(text), 0o644)
		}
	}
	entry := filepath.Join(dir, filepath.FromSlash(bundle.Name))
	p := parser.New(lexer.New(bundle.Source))
	prog := p.ParseProgram()
	if len(p.SyntaxErrors()) > 0 {
		fmt.Fprint(os.Stderr, p.DiagnosticReport(bundle.Name, bundle.Source, isTerminal(os.Stderr)))
		return 1
	}
	code, _ := execute(entry, bundle.Source, prog, engineInterp, true)
	return code
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

// collectModules gathers imported .al files (recursively) relative to root.
func collectModules(root, file string, prog *ast.Program, files map[string]string, seen map[string]bool) error {
	for _, imp := range moduleImports(prog) {
		path := imp
		if !strings.HasSuffix(path, ".al") {
			path += ".al"
		}
		abs := path
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(filepath.Dir(file), path)
		}
		abs = filepath.Clean(abs)
		if seen[abs] {
			continue
		}
		seen[abs] = true
		data, err := os.ReadFile(abs)
		if err != nil {
			return fmt.Errorf("cannot read module %q: %w", imp, err)
		}
		rel, err := filepath.Rel(root, abs)
		if err != nil || strings.HasPrefix(rel, "..") {
			return fmt.Errorf("module %q is outside the project directory %q", imp, root)
		}
		files[filepath.ToSlash(rel)] = string(data)
		p := parser.New(lexer.New(string(data)))
		sub := p.ParseProgram()
		if len(p.SyntaxErrors()) > 0 {
			return fmt.Errorf("module %q has syntax errors", imp)
		}
		if err := collectModules(root, abs, sub, files, seen); err != nil {
			return err
		}
	}
	return nil
}

// cmdBuild implements `alca build`.
func cmdBuild(args []string) int {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprint(os.Stderr, diag.FormatPanicReport("build", r, isTerminal(os.Stderr)))
		}
	}()
	var input, output, targetStr, runtimePath string
	forceSource := false
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "-o" && i+1 < len(args):
			output = args[i+1]
			i++
		case (a == "--target" || a == "-t") && i+1 < len(args):
			targetStr = args[i+1]
			i++
		case a == "--runtime" && i+1 < len(args):
			runtimePath = args[i+1]
			i++
		case a == "--source":
			forceSource = true
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(os.Stderr, "unknown build option %q\n", a)
			return 1
		default:
			input = a
		}
	}
	if input == "" {
		fmt.Fprintln(os.Stderr, "Usage: alca build <file.al> [-o output] [--target os/arch] [--runtime file] [--source]")
		return 1
	}

	target := compiler.HostTarget()
	if targetStr != "" {
		t, err := compiler.ParseTarget(targetStr)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		target = t
	}

	prog, content, ok := loadProgram(input)
	if !ok {
		return 1
	}

	if output == "" {
		base := filepath.Base(input)
		output = strings.TrimSuffix(base, filepath.Ext(base))
	}
	output = compiler.ExeName(output, target)
	absOutput, err := filepath.Abs(output)
	if err != nil {
		absOutput = output
	}

	stub, err := compiler.FindRuntime(target, runtimePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	bundle := &compiler.Bundle{Name: filepath.Base(input)}
	mode, reason := "bytecode", ""
	imports := moduleImports(prog)
	switch {
	case forceSource:
		mode, reason = "source", "requested with --source"
	case len(imports) > 0:
		mode, reason = "source", "program imports other .al files"
	default:
		c := vm.NewCompiler()
		if cerr := c.Compile(prog); cerr != nil {
			mode, reason = "source", cerr.Error()
		} else {
			bundle.Kind = compiler.KindBytecode
			bundle.Code = c.Bytecode()
			if _, eerr := bundle.Encode(); eerr != nil {
				mode, reason = "source", eerr.Error()
			}
		}
	}
	if mode == "source" {
		bundle.Kind = compiler.KindSource
		bundle.Source = content
		bundle.Files = map[string]string{}
		root := filepath.Dir(input)
		if err := collectModules(root, input, prog, bundle.Files, map[string]bool{}); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
	}

	fmt.Printf("Building %s -> %s [%s, %s payload]\n", input, absOutput, target, mode)
	if reason != "" && mode == "source" {
		fmt.Printf("  note: embedding source because %s\n", reason)
	}
	if err := compiler.Build(bundle, stub, absOutput); err != nil {
		fmt.Fprint(os.Stderr, diag.FormatCompilerError(input, err, isTerminal(os.Stderr)))
		return 1
	}
	size := ""
	if fi, err := os.Stat(absOutput); err == nil {
		size = fmt.Sprintf(" (%.2f MB)", float64(fi.Size())/(1024*1024))
	}
	fmt.Printf("[OK] %s%s\n", absOutput, size)
	return 0
}

// cmdTargets lists supported targets and runtime availability.
func cmdTargets() int {
	host := compiler.HostTarget()
	fmt.Printf("Host: %s\n\nTarget            Runtime\n", host)
	for _, t := range compiler.SupportedTargets {
		status := "missing  (alca build-runtimes --targets " + t.String() + ")"
		if p, err := compiler.FindRuntime(t, ""); err == nil {
			status = p
		}
		fmt.Printf("%-17s %s\n", t, status)
	}
	return 0
}

// cmdBuildRuntimes cross-builds runtime binaries with the Go toolchain.
func cmdBuildRuntimes(args []string) int {
	out := "runtimes"
	tags := ""
	targets := compiler.SupportedTargets
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--out":
			if i+1 < len(args) {
				out = args[i+1]
				i++
			}
		case "--tags":
			if i+1 < len(args) {
				tags = args[i+1]
				i++
			}
		case "--targets":
			if i+1 < len(args) {
				targets = nil
				for _, ts := range strings.Split(args[i+1], ",") {
					t, err := compiler.ParseTarget(ts)
					if err != nil {
						fmt.Fprintln(os.Stderr, "error:", err)
						return 1
					}
					targets = append(targets, t)
				}
				i++
			}
		}
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: the Go toolchain is required to build runtimes (https://go.dev/dl); it is NOT needed to use them")
		return 1
	}
	if _, err := os.Stat("go.mod"); err != nil {
		fmt.Fprintln(os.Stderr, "error: run this from the root of the Alca source tree (go.mod not found)")
		return 1
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	failed := 0
	for _, t := range targets {
		dst := filepath.Join(out, compiler.RuntimeName(t))
		fmt.Printf("building %-14s -> %s\n", t, dst)
		cmdArgs := []string{"build", "-trimpath", "-ldflags", "-s -w", "-o", dst}
		if tags != "" {
			cmdArgs = append(cmdArgs, "-tags", tags)
		}
		cmdArgs = append(cmdArgs, "./cmd/alca")
		cmd := exec.Command(goBin, cmdArgs...)
		cmd.Env = append(os.Environ(), "GOOS="+t.OS, "GOARCH="+t.Arch, "CGO_ENABLED=0")
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "  failed: %v\n", err)
			failed++
		}
	}
	if failed > 0 {
		return 1
	}
	names := make([]string, 0, len(targets))
	for _, t := range targets {
		names = append(names, t.String())
	}
	sort.Strings(names)
	fmt.Printf("[OK] runtimes for %s in %s/\n", strings.Join(names, ", "), out)
	return 0
}
