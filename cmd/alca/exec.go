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
	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/jit"
	"github.com/alca-lang/alca/pkg/lexer"
	"github.com/alca-lang/alca/pkg/mod"
	"github.com/alca-lang/alca/pkg/parser"
	"github.com/alca-lang/alca/pkg/sandbox"
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

// cmdRun implements `alca run`.
func cmdRun(args []string) int {
	eng := engineAuto

	// By default, activate JIT acceleration for maximum performance
	if jitEng := jit.Default(); jitEng != nil {
		jitEng.SetEnabled(true)
		jitEng.SetThreshold(1)
	}

	var policyArgs, rest []string
	var memFlags memoryFlags
	for len(args) > 0 {
		a := args[0]
		switch {
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
		case a == "--":
			args = args[1:]
			rest = args
			goto parsed
		default:
			rest = args
			goto parsed
		}
		args = args[1:]
	}
parsed:
	applyMemoryFlags(memFlags)
	policy, leftover, err := sandbox.ParseArgs(policyArgs)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		fmt.Fprintln(os.Stderr, "Usage: alca run [--vm|--interp] ["+sandbox.FlagHint+"] <file.al> [args...]")
		return 1
	}
	if len(leftover) > 0 {
		rest = leftover
	}
	if len(rest) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: alca run [--vm|--interp] ["+sandbox.FlagHint+"] <file.al> [args...]")
		return 1
	}
	filename, scriptArgs := rest[0], rest[1:]
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
	policy, scriptArgs, perr := sandbox.ParseArgs(os.Args[1:])
	if perr != nil {
		fmt.Fprintln(os.Stderr, "error:", perr)
		fmt.Fprintln(os.Stderr, "Usage: <app> ["+sandbox.FlagHint+"] [args...]")
		return 1
	}
	memFlags, _, merr := parseMemoryFlags(os.Args[1:])
	if merr != nil {
		fmt.Fprintln(os.Stderr, "error:", merr)
		return 1
	}
	applyMemoryFlags(memFlags)
	sandbox.SetPolicy(policy)
	stdlib.SetScriptArgs(scriptArgs)
	stdlib.SetEmbeddedResources(bundle.Resources)
	defer stdlib.SetEmbeddedResources(nil)
	defer stdlib.CloseAllDatabases()

	if bundle.Kind == compiler.KindNative || bundle.Kind == compiler.KindBytecode {
		interp := newInterpreter()
		interp.SetFilename(bundle.Name)
		if jitEng := jit.Default(); jitEng != nil {
			jitEng.SetEnabled(true)
			jitEng.SetThreshold(1)
		}
		machine := vm.NewWithEnv(bundle.Code, interp.Env())
		if bundle.Kind == compiler.KindNative && len(bundle.NativeCode) > 0 {
			if jitEng := jit.Default(); jitEng != nil {
				addr, err := jit.AllocExec(len(bundle.NativeCode))
				if err == nil {
					defer jit.FreeExec(addr, len(bundle.NativeCode))
					copy(jit.UnsafeSlice(addr, len(bundle.NativeCode)), bundle.NativeCode)
					ipOffsets := make(map[int]uintptr, len(bundle.LabelOffsets))
					for ip, off := range bundle.LabelOffsets {
						ipOffsets[ip] = addr + uintptr(off)
					}
					jitEng.InstallAOT(machine.MainFunction(), addr, len(bundle.NativeCode), ipOffsets)
				}
			}
		}
		if err := machine.Run(); err != nil {
			fmt.Fprint(os.Stderr, diag.FormatRuntimeError(bundle.Name, "", err, isTerminal(os.Stderr)))
			return 1
		}
		return 0
	}

	// Source bundle: materialise the files so `use pkg` and relative imports resolve.
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
	bundled := make(map[string]string, len(bundle.Modules))
	for imp, rel := range bundle.Modules {
		bundled[imp] = filepath.Join(dir, filepath.FromSlash(rel))
	}
	mod.SetBundled(bundled)
	mod.SetProjectRoot(dir)
	defer mod.SetBundled(nil)
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

// cmdBuild implements `alca build`.
func cmdBuild(args []string) int {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprint(os.Stderr, diag.FormatPanicReport("build", r, isTerminal(os.Stderr)))
		}
	}()
	var input, output, targetStr, runtimePath string
	var embedPaths []string
	forceSource := false
	forceNative := false
	forceBytecode := false
	noConsole := false
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
		case a == "--embed" && i+1 < len(args):
			embedPaths = append(embedPaths, args[i+1])
			i++
		case a == "--source":
			forceSource = true
		case a == "--native" || a == "--aot":
			forceNative = true
		case a == "--bytecode":
			forceBytecode = true
		case a == "--noconsole" || a == "--gui" || a == "-noconsole":
			noConsole = true
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(os.Stderr, "unknown build option %q\n", a)
			return 1
		default:
			input = a
		}
	}
	if input == "" {
		fmt.Fprintln(os.Stderr, "Usage: alca build <file.al> [-o output] [--embed file-or-directory ...] [--noconsole] [--target os/arch] [--runtime file] [--source]")
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

	var stub string
	bundle := &compiler.Bundle{Name: filepath.Base(input)}
	if len(embedPaths) > 0 {
		root, err := filepath.Abs(filepath.Dir(input))
		if err != nil {
			fmt.Fprintln(os.Stderr, "error: cannot resolve project directory:", err)
			return 1
		}
		bundle.Resources, err = collectEmbeddedResources(root, embedPaths)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
	}
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
			useNative := (forceNative || (target.Arch == "amd64" && jit.HasNativeAMD64 && target == compiler.HostTarget() && runtimePath == "")) && !forceBytecode
			if useNative {
				mainFn := &vm.CompiledFunction{
					Instructions: bundle.Code.Instructions,
					Pool:         vm.NewConstantPool(bundle.Code.Constants),
					Globals:      make([]vm.StackSlot, len(c.GlobalNames())+64),
				}
				nativeCode, labelOffsets, aerr := jit.CompileAOT(mainFn)
				if aerr == nil && len(nativeCode) > 0 {
					bundle.Kind = compiler.KindNative
					bundle.NativeCode = nativeCode
					bundle.LabelOffsets = labelOffsets
					mode = "native AMD64 machine code"
				}
			}
			if _, eerr := bundle.Encode(); eerr != nil {
				mode, reason = "source", eerr.Error()
			}
		}
	}
	if mode == "source" {
		bundle.Kind = compiler.KindSource
		bundle.Source = content
		bundle.Files = map[string]string{}
		bundle.Modules = map[string]string{}
		root := filepath.Dir(input)
		if absRoot, err := filepath.Abs(root); err == nil {
			root = absRoot
		}
		if err := collectModules(root, input, prog, bundle.Files, bundle.Modules, map[string]string{}); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
	}

	// Try pure native PE first for Windows AMD64 when native machine code is
	// available. A pure PE has no VM fallback, so it is only safe when every
	// instruction is lowered to native code. Programs with calls (print,
	// println, user functions, stdlib) therefore use the regular runtime stub,
	// which runs the AOT code and falls back to the VM where needed.
	builtNativePE := false
	if len(bundle.Resources) == 0 && !usesRuntimeStdlib(prog) && bundle.Kind == compiler.KindNative &&
		target.OS == "windows" && target.Arch == "amd64" && runtimePath == "" &&
		jit.FullyNative(bundle.Code.Instructions) {
		if peErr := compiler.BuildNativePE(bundle, absOutput, noConsole); peErr == nil {
			builtNativePE = true
			mode = "pure standalone native PE (zero runtime overhead)"
		}
	}

	if !builtNativePE {
		var rerr error
		stub, rerr = compiler.FindRuntime(target, runtimePath)
		if rerr != nil {
			fmt.Fprintln(os.Stderr, "error:", rerr)
			return 1
		}
		if err := compiler.Build(bundle, stub, absOutput); err != nil {
			fmt.Fprint(os.Stderr, diag.FormatCompilerError(input, err, isTerminal(os.Stderr)))
			return 1
		}
		if noConsole && target.OS == "windows" {
			_ = compiler.PatchFileSubsystem(absOutput, compiler.SubsystemWindowsGUI)
		}
	}

	if noConsole {
		mode += ", noconsole / GUI"
	}

	fmt.Printf("Building %s -> %s [%s, %s]\n", input, absOutput, target, mode)
	if reason != "" && mode == "source" {
		fmt.Printf("  note: embedding source because %s\n", reason)
	}
	size := ""
	if fi, err := os.Stat(absOutput); err == nil {
		if fi.Size() >= 1024*1024 {
			size = fmt.Sprintf(" (%.2f MB)", float64(fi.Size())/(1024*1024))
		} else {
			size = fmt.Sprintf(" (%.2f KB)", float64(fi.Size())/1024.0)
		}
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
