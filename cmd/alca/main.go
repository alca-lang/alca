// Package main implements the Alca language CLI tool and native runner.
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/alca-lang/alca/pkg/ast"
	"github.com/alca-lang/alca/pkg/diag"
	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/lexer"
	"github.com/alca-lang/alca/pkg/lsp"
	"github.com/alca-lang/alca/pkg/parser"
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
		startREPL()

	case "fmt":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: alca fmt <file.al>")
			os.Exit(1)
		}
		os.Exit(formatFile(os.Args[2]))

	case "test":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: alca test <file.al>")
			os.Exit(1)
		}
		os.Exit(testFile(os.Args[2]))

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
	fmt.Println("  fmt <file.al>               Print the formatted program")
	fmt.Println("  repl                        Interactive session")
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

// startREPL runs the interactive Alca REPL session.
func startREPL() {
	fmt.Printf("Alca v%s REPL\n", Version)
	fmt.Println("Type 'exit' or 'quit' to leave. End a line with ':' for multi-line block.")

	scanner := bufio.NewScanner(os.Stdin)
	interp := newInterpreter()

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

		input := inputBuilder.String()
		l := lexer.New(input)
		p := parser.New(l)
		prog := p.ParseProgram()

		if len(p.SyntaxErrors()) > 0 {
			fmt.Print(p.DiagnosticReport("<repl>", input, isTerminal(os.Stdout)))
			continue
		}

		// If a single expression statement was entered, evaluate and print result
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

				val := interp.Eval(exprStmt.Expression)
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
				continue
			}
		}

		// Execute multi-statement or non-expression script
		interp.SetFilename("<repl>")
		interp.SetSource(input)
		if err := interp.Execute(prog); err != nil {
			fmt.Fprint(os.Stderr, diag.FormatRuntimeError("<repl>", input, err, isTerminal(os.Stderr)))
		}
	}
}

// formatFile parses a .al file and prints its formatted AST.
func formatFile(filename string) int {
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

	fmt.Print(prog.String())
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
