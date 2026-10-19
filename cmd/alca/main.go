// Package main implements the Alca language CLI tool and native runner.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/alca-lang/alca/pkg/ast"
	"github.com/alca-lang/alca/pkg/diag"
	"github.com/alca-lang/alca/pkg/format"
	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/lexer"
	"github.com/alca-lang/alca/pkg/lsp"
	"github.com/alca-lang/alca/pkg/parser"
	"github.com/alca-lang/alca/pkg/sandbox"
	"github.com/alca-lang/alca/pkg/stdlib"
	"github.com/alca-lang/alca/pkg/typecheck"
)

// Version of the Alca language CLI.
const Version = "2.0.0"

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprint(os.Stderr, diag.FormatPanicReport("alca", r, isTerminal(os.Stderr)))
			os.Exit(1)
		}
	}()

	// If this binary has an embedded Alca application payload, run it natively as a standalone app!
	if isStandaloneExecutable() {
		os.Exit(runStandalone())
	}

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	command := os.Args[1]

	switch command {
	case "run":
		os.Exit(cmdRun(os.Args[2:]))

	case "vm":
		os.Exit(cmdRun(append([]string{"--vm"}, os.Args[2:]...)))

	case "build":
		os.Exit(cmdBuild(os.Args[2:]))

	case "targets":
		os.Exit(cmdTargets())

	case "build-runtimes":
		os.Exit(cmdBuildRuntimes(os.Args[2:]))

	case "repl":
		startREPL(os.Args[2:])

	case "fmt":
		os.Exit(cmdFmt(os.Args[2:]))

	case "test":
		os.Exit(cmdTest(os.Args[2:]))

	case "check":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: alca check <file.al>")
			os.Exit(1)
		}
		os.Exit(checkFile(os.Args[2]))

	case "lsp":
		if err := lsp.RunStdio(); err != nil {
			fmt.Fprintf(os.Stderr, "LSP server error: %s\n", err)
			os.Exit(1)
		}
		os.Exit(0)

	case "version", "-v", "--version":
		fmt.Printf("alca version %s\n", Version)

	case "help", "-h", "--help":
		printUsage()

	default:
		// If argument ends in .al, treat as 'alca run <arg>'
		if strings.HasSuffix(command, ".al") {
			os.Exit(cmdRun(os.Args[1:]))
		}
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Printf("Alca Programming Language CLI v%s\n\n", Version)
	fmt.Println("Usage:")
	fmt.Println("  alca <command> [arguments]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  run <file.al> [args...]     Type-check, then run on the bytecode VM (default engine)")
	fmt.Println("        --interp              run on the tree-walking interpreter instead")
	fmt.Println("        --vm                  require the VM (error if the program needs the interpreter)")
	fmt.Println("  build <file.al> [-o out]    Build a standalone executable")
	fmt.Println("        --target os/arch      cross-compile, e.g. linux/amd64, windows/arm64, darwin/arm64")
	fmt.Println("        --runtime <file>      use a specific runtime binary for the target")
	fmt.Println("        --source              embed source instead of bytecode")
	fmt.Println("  targets                     List build targets and which runtimes are installed")
	fmt.Println("  build-runtimes [--targets a,b] [--out dir] [--tags t]")
	fmt.Println("                              Build runtimes for cross-compilation (needs the Go toolchain)")
	fmt.Println("  check <file.al>             Syntax + strict type check (CI friendly)")
	fmt.Println("  test <file.al>              Run test blocks and report PASS/FAIL")
	fmt.Println("  fmt [-w] <file.al>          Format the program (-w rewrites the file in place)")
	fmt.Println("  repl                        Interactive session")
	fmt.Println()
	fmt.Println("Runtime capabilities (file and network access are denied by default):")
	fmt.Println("  " + sandbox.FlagHint)
	fmt.Println("  Use --allow-read=<path>, --allow-write=<path>, --allow-net=<host[:port]>")
	fmt.Println("  before the file name; --allow-all grants everything.")
	fmt.Println("  lsp                         Language Server Protocol daemon for editors")
	fmt.Println("  version | help")
}

// checkFile performs static analysis, syntax validation, and compile-time type verification.
func checkFile(filename string) int {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprint(os.Stderr, diag.FormatPanicReport(filename, r, isTerminal(os.Stderr)))
		}
	}()

	data, err := os.ReadFile(filename)
	if err != nil {
		fmt.Fprint(os.Stderr, diag.FormatIOReport(filename, "read", filename, err, isTerminal(os.Stderr)))
		return 1
	}

	content := string(data)
	l := lexer.New(content)
	p := parser.New(l)
	prog := p.ParseProgram()

	if len(p.SyntaxErrors()) > 0 {
		fmt.Fprint(os.Stderr, p.DiagnosticReport(filename, content, isTerminal(os.Stderr)))
		return 1
	}

	checker := typecheck.NewChecker(filename, content)
	typeErrors := checker.Check(prog)
	if len(typeErrors) > 0 {
		fmt.Fprint(os.Stderr, diag.FormatReport(filename, content, typeErrors, isTerminal(os.Stderr)))
		return 1
	}

	fmt.Printf("[OK] %s: No syntax or static type errors found (passed all static checks).\n", filename)
	return 0
}

// newInterpreter creates an interpreter instance with all standard libraries registered.
func newInterpreter() *interpreter.Interpreter {
	interp := interpreter.New()

	// Register module parser to support 'use' statements for .al files
	interpreter.SetModuleParser(func(content string) (*ast.Program, error) {
		l := lexer.New(content)
		p := parser.New(l)
		prog := p.ParseProgram()
		if len(p.SyntaxErrors()) > 0 {
			return nil, fmt.Errorf("%s", p.DiagnosticReport("<module>", content, false))
		}
		return prog, nil
	})

	// Register standard library builtins into environment
	env := interp.Env()
	stdlib.RegisterIOBuiltins(env)
	stdlib.RegisterEnvBuiltins(env)
	stdlib.RegisterNetBuiltins(env)
	stdlib.RegisterProcessBuiltins(env)
	stdlib.RegisterGUIBuiltins(env)
	stdlib.RegisterTimeBuiltins(env)
	stdlib.RegisterPathBuiltins(env)
	stdlib.RegisterCryptoBuiltins(env)
	stdlib.RegisterRegexBuiltins(env)
	stdlib.RegisterArchiveBuiltins(env)
	stdlib.RegisterFormatsBuiltins(env)
	stdlib.RegisterSysBuiltins(env)
	stdlib.RegisterCLIBuiltins(env)
	stdlib.RegisterLogBuiltins(env)
	stdlib.RegisterMathBuiltins(env)
	stdlib.RegisterDBBuiltins(env)
	stdlib.RegisterMetricsBuiltins(env)
	stdlib.RegisterSecurityBuiltins(env)
	stdlib.RegisterFFIBuiltins(env)
	stdlib.RegisterAIBuiltins(env)

	// Build the top-level 'std' namespace object so std.io, std.math, etc. work seamlessly
	stdModule := interpreter.NewMap()
	for _, name := range []string{
		"io", "env", "net", "process", "gui", "time", "path", "crypto",
		"regex", "re", "archive", "zip", "formats", "csv", "ini", "json",
		"sys", "cli", "log", "math", "db", "sql", "sqlite", "postgres", "metrics",
		"ffi", "ai", "security",
	} {
		if val, ok := env.Get(name); ok {
			stdModule.Set(name, val)
		}
	}
	env.Set("std", stdModule)

	return interp
}

// replSession carries the persistent state of one interactive REPL session:
// the interpreter environment and the static checker with the symbols declared
// so far.
type replSession struct {
	interp  *interpreter.Interpreter
	checker *typecheck.Checker
}

// newREPLSession creates a session with all standard libraries registered.
func newREPLSession() *replSession {
	return &replSession{
		interp:  newInterpreter(),
		checker: typecheck.NewChecker("<repl>", ""),
	}
}

// eval parses, statically checks and executes one snippet of REPL input,
// printing values and diagnostics. Declarations made by the snippet persist in
// the interpreter and in the checker.
func (r *replSession) eval(input string) {
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()

	if len(p.SyntaxErrors()) > 0 {
		fmt.Print(p.DiagnosticReport("<repl>", input, isTerminal(os.Stdout)))
		return
	}

	snap := r.checker.Snapshot()
	typeErrs := r.checker.CheckSnippet(prog)
	if len(typeErrs) > 0 {
		r.checker.Restore(snap)
		fmt.Fprint(os.Stderr, diag.FormatReport("<repl>", input, typeErrs, isTerminal(os.Stderr)))
		return
	}

	// If a single expression statement was entered, evaluate and print result.
	if len(prog.Statements) == 1 {
		if exprStmt, ok := prog.Statements[0].(*ast.ExpressionStatement); ok {
			isPrintCall := false
			if call, ok := exprStmt.Expression.(*ast.CallExpression); ok {
				if ident, ok := call.Function.(*ast.Identifier); ok {
					if ident.Value == "print" || ident.Value == "println" {
						isPrintCall = true
					}
				}
			}

			val := r.interp.Eval(exprStmt.Expression)
			if val != nil && val.Type() != interpreter.NULL_VAL && !isPrintCall {
				if errVal, ok := val.(*interpreter.ErrorValue); ok {
					fmt.Fprint(os.Stderr, diag.FormatRuntimeError("<repl>", input, &diag.RuntimeError{
						Filename: "<repl>",
						Line:     1,
						Column:   1,
						Message:  errVal.Message,
						Code:     errVal.Code,
					}, isTerminal(os.Stderr)))
				} else {
					fmt.Println(val.Inspect())
				}
			}
			return
		}
	}

	// Execute multi-statement or non-expression script.
	r.interp.SetFilename("<repl>")
	r.interp.SetSource(input)
	if err := r.interp.Execute(prog); err != nil {
		fmt.Fprint(os.Stderr, diag.FormatRuntimeError("<repl>", input, err, isTerminal(os.Stderr)))
	}
}

// cmdTest implements `alca test` with the same capability policy as `alca run`.
func cmdTest(args []string) int {
	policy, rest, err := sandbox.ParseArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		fmt.Fprintln(os.Stderr, "Usage: alca test ["+sandbox.FlagHint+"] <file.al>")
		return 1
	}
	if len(rest) != 1 {
		fmt.Fprintln(os.Stderr, "Usage: alca test ["+sandbox.FlagHint+"] <file.al>")
		return 1
	}
	previous := sandbox.CurrentPolicy()
	sandbox.SetPolicy(policy)
	defer sandbox.SetPolicy(previous)
	return testFile(rest[0])
}

// startREPL runs the interactive Alca REPL session.
func startREPL(args []string) {
	policy, rest, err := sandbox.ParseArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		fmt.Fprintln(os.Stderr, "Usage: alca repl ["+sandbox.FlagHint+"]")
		return
	}
	if len(rest) > 0 {
		fmt.Fprintln(os.Stderr, "Usage: alca repl ["+sandbox.FlagHint+"]")
		return
	}
	sandbox.SetPolicy(policy)
	defer sandbox.ResetPolicy()

	fmt.Printf("Alca v%s REPL\n", Version)
	fmt.Println("Every simple statement ends with ';'. A line ending with ':' starts a block;")
	fmt.Println("finish the block with an empty line. Type 'exit' or 'quit' to leave.")
	if !policy.Read.All && !policy.Write.All && !policy.Net.All && len(policy.Read.Roots) == 0 && len(policy.Write.Roots) == 0 && len(policy.Net.Roots) == 0 {
		fmt.Println("File and network access are disabled; restart with --allow-read/--allow-write/--allow-net or --allow-all.")
	}

	scanner := bufio.NewScanner(os.Stdin)
	session := newREPLSession()

	for {
		fmt.Print("alca> ")
		if !scanner.Scan() {
			break
		}
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "exit" || trimmed == "quit" {
			break
		}
		if trimmed == "" {
			continue
		}

		// Handle multi-line block input if line ends with ':'
		var inputBuilder strings.Builder
		inputBuilder.WriteString(line)

		if strings.HasSuffix(trimmed, ":") {
			for {
				fmt.Print("...   ")
				if !scanner.Scan() {
					break
				}
				subLine := scanner.Text()
				if strings.TrimSpace(subLine) == "" {
					break
				}
				inputBuilder.WriteString("\n")
				inputBuilder.WriteString(subLine)
			}
		}

		session.eval(inputBuilder.String())
	}
	fmt.Println()
}

// cmdFmt implements the 'alca fmt' command: it formats a source file, printing
// the result to stdout, or rewriting the file in place when -w is given.
func cmdFmt(args []string) int {
	write := false
	var files []string
	for _, arg := range args {
		switch arg {
		case "-w", "--write":
			write = true
		default:
			files = append(files, arg)
		}
	}
	if len(files) != 1 {
		fmt.Fprintln(os.Stderr, "Usage: alca fmt [-w] <file.al>")
		return 1
	}
	return formatFile(files[0], write)
}

// formatFile reformats a .al file with the canonical formatter.
func formatFile(filename string, write bool) int {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprint(os.Stderr, diag.FormatPanicReport(filename, r, isTerminal(os.Stderr)))
		}
	}()

	data, err := os.ReadFile(filename)
	if err != nil {
		fmt.Fprint(os.Stderr, diag.FormatIOReport(filename, "read", filename, err, isTerminal(os.Stderr)))
		return 1
	}

	formatted, err := format.FormatSource(filename, string(data))
	if err != nil {
		var syntaxErr *format.SyntaxError
		if errors.As(err, &syntaxErr) {
			fmt.Fprint(os.Stderr, syntaxErr.Report)
		} else {
			fmt.Fprintf(os.Stderr, "error: %s\n", err)
		}
		return 1
	}

	if !write {
		fmt.Print(formatted)
		return 0
	}

	mode := os.FileMode(0o644)
	if info, statErr := os.Stat(filename); statErr == nil {
		mode = info.Mode().Perm()
	}
	if err := os.WriteFile(filename, []byte(formatted), mode); err != nil {
		fmt.Fprint(os.Stderr, diag.FormatIOReport(filename, "write", filename, err, isTerminal(os.Stderr)))
		return 1
	}
	return 0
}

// isTerminal checks whether the file descriptor refers to a terminal console.
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
