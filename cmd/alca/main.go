// Package main implements the Alca language CLI tool and native runner.
package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/alca-lang/alca/pkg/ast"
	"github.com/alca-lang/alca/pkg/diag"
	"github.com/alca-lang/alca/pkg/format"
	"github.com/alca-lang/alca/pkg/interpreter"
	_ "github.com/alca-lang/alca/pkg/jit"
	"github.com/alca-lang/alca/pkg/lexer"
	"github.com/alca-lang/alca/pkg/lsp"
	"github.com/alca-lang/alca/pkg/mod"
	"github.com/alca-lang/alca/pkg/parser"
	"github.com/alca-lang/alca/pkg/sandbox"
	"github.com/alca-lang/alca/pkg/stdlib"
	"github.com/alca-lang/alca/pkg/typecheck"
	"github.com/alca-lang/alca/pkg/version"
)

// Version of the Alca language CLI. The canonical value lives in pkg/version
// so the package manager and registry node report the same toolchain version.
const Version = version.Version

func main() {
	enableUTF8Console()

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
	case "run", "vm":
		os.Exit(cmdRun(os.Args[2:]))

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
	fmt.Println("  run <file.al> [args...]     Run an Alca program (Bytecode VM with JIT acceleration)")
	fmt.Println("  build <file.al> [-o out]    Compile program into a standalone native executable")
	fmt.Println("        --noconsole, --gui    build a GUI application without opening a console window")
	fmt.Println("        --target os/arch      cross-compile (e.g. linux/amd64, windows/arm64, darwin/arm64)")
	fmt.Println("        --runtime <file>      use a specific runtime binary for the target")
	fmt.Println("        --source              embed source instead of bytecode")
	fmt.Println("        --embed <path>        embed a project file or directory (repeatable)")
	fmt.Println("  test <file.al>              Run test blocks and report PASS/FAIL")
	fmt.Println("  check <file.al>             Syntax + strict type check (CI friendly)")
	fmt.Println("  fmt [-w] <file.al>          Format the program (-w rewrites the file in place)")
	fmt.Println("  repl                        Interactive session")
	fmt.Println("  targets                     List build targets and which runtimes are installed")
	fmt.Println("  build-runtimes [--targets a,b] [--out dir] [--tags t]")
	fmt.Println("                              Build runtimes for cross-compilation (needs the Go toolchain)")
	fmt.Println("  lsp                         Language Server Protocol daemon for editors")
	fmt.Println("  version | help")
	fmt.Println()
	fmt.Println("Runtime capabilities (file and network access are denied by default):")
	fmt.Println("  " + sandbox.FlagHint)
	fmt.Println("  Use --allow-read=<path>, --allow-write=<path>, --allow-net=<host[:port]>")
	fmt.Println("  before the file name; --allow-all grants everything.")
	fmt.Println()
	fmt.Print(memoryFlagHint)
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
	stdlib.RegisterStringBuiltins(env)
	stdlib.RegisterSysBuiltins(env)
	stdlib.RegisterCLIBuiltins(env)
	stdlib.RegisterLogBuiltins(env)
	stdlib.RegisterMathBuiltins(env)
	stdlib.RegisterDBBuiltins(env)
	stdlib.RegisterMetricsBuiltins(env)
	stdlib.RegisterSecurityBuiltins(env)
	stdlib.RegisterFFIBuiltins(env)
	stdlib.RegisterMemBuiltins(env)
	stdlib.RegisterMTProtoBuiltins(env)
	stdlib.RegisterAIBuiltins(env)
	stdlib.RegisterCompressBuiltins(env)
	stdlib.RegisterEmbedBuiltins(env)
	stdlib.RegisterHTTPBuiltins(env)
	stdlib.RegisterUrllib2Builtins(env)

	// Build the top-level 'std' namespace object so std.io, std.math, etc. work seamlessly
	stdModule := interpreter.NewMap()
	for _, name := range []string{
		"io", "env", "net", "process", "gui", "time", "path", "crypto",
		"regex", "re", "archive", "zip", "formats", "csv", "ini", "json",
		"sys", "cli", "log", "math", "db", "sql", "sqlite", "postgres", "metrics",
		"ffi", "ai", "security", "compress", "embed", "http", "urllib2", "string", "str",
		"mem", "mtproto",
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

// replCommands are the REPL meta-commands. They are handled by the REPL
// itself and never reach the parser.
var replCommands = map[string]func(*replSession) string{
	"help": func(*replSession) string {
		return "Alca REPL commands:\n" +
			"  help        show this message\n" +
			"  clear       clear the screen\n" +
			"  exit, quit  leave the REPL\n" +
			"\n" +
			"Every statement must end with ';'. A line ending with ':' starts a block;\n" +
			"finish the block with an empty line.\n" +
			"\n" +
			"Examples:\n" +
			"  x: int = 10;          declare a variable\n" +
			"  x = x + 1;            update it\n" +
			"  println(x);           print a value\n" +
			"  for i in 1..3:        start a block, then press Enter on an empty line\n" +
			"      println(i);\n"
	},
	"clear": func(*replSession) string {
		return "\x1b[H\x1b[2J\x1b[3J"
	},
}

// command runs a REPL meta-command and reports whether the input was one. The
// output is the text to print, which is empty for commands with no output.
func (r *replSession) command(input string) (string, bool) {
	cmd, ok := replCommands[strings.TrimSpace(input)]
	if !ok {
		return "", false
	}
	return cmd(r), true
}

// eval parses, statically checks and executes one snippet of REPL input,
// printing values and diagnostics. Declarations made by the snippet persist in
// the interpreter and in the checker.
func (r *replSession) eval(input string) {
	// A snippet typed at the prompt may carry the indentation of the line it
	// was copied from. Indentation is only meaningful inside a block, so strip
	// the common leading whitespace before handing the text to the lexer.
	input = dedentSnippet(input)

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

// dedentSnippet removes the leading whitespace of the first non-blank line from
// every line of a snippet. A snippet typed at the REPL prompt keeps the
// indentation of the line it was copied from, which the lexer would otherwise
// report as an unexpected INDENT token. Only the first line's indentation is
// removed, so the relative structure of a block is preserved.
func dedentSnippet(input string) string {
	lines := strings.Split(input, "\n")
	indent := -1
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent = len(line) - len(strings.TrimLeft(line, " \t"))
		break
	}
	if indent <= 0 {
		return input
	}
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if len(line) < indent {
			lines[i] = strings.TrimLeft(line, " \t")
			continue
		}
		lines[i] = line[indent:]
	}
	return strings.Join(lines, "\n")
}

// cmdTest implements `alca test` with the same capability policy as `alca run`.
func cmdTest(args []string) int {
	memFlags, args, merr := parseMemoryFlags(args)
	if merr != nil {
		fmt.Fprintln(os.Stderr, "error:", merr)
		fmt.Fprintln(os.Stderr, "Usage: alca test ["+sandbox.FlagHint+"] <file.al>")
		return 1
	}
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
	applyMemoryFlags(memFlags)
	return testFile(rest[0])
}

// hasUnclosedTripleQuote reports whether s has an unclosed triple-quoted string (""" or ”').
func hasUnclosedTripleQuote(s string) bool {
	i := 0
	inTriple := byte(0)
	for i < len(s) {
		if inTriple != 0 {
			if s[i] == '\\' && i+1 < len(s) {
				i += 2
				continue
			}
			if s[i] == inTriple && i+2 < len(s) && s[i+1] == inTriple && s[i+2] == inTriple {
				inTriple = 0
				i += 3
				continue
			}
			i++
			continue
		}
		// Skip single-line comments
		if s[i] == '/' && i+1 < len(s) && s[i+1] == '/' {
			i += 2
			for i < len(s) && s[i] != '\n' {
				i++
			}
			continue
		}
		// Skip backtick strings
		if s[i] == '`' {
			i++
			for i < len(s) && s[i] != '`' {
				i++
			}
			if i < len(s) && s[i] == '`' {
				i++
			}
			continue
		}
		// Check for opening triple quote or single quote
		if s[i] == '"' || s[i] == '\'' {
			q := s[i]
			if i+2 < len(s) && s[i+1] == q && s[i+2] == q {
				inTriple = q
				i += 3
				continue
			}
			// Regular single-line string: skip till closing quote or newline
			i++
			for i < len(s) && s[i] != q && s[i] != '\n' {
				if s[i] == '\\' && i+1 < len(s) {
					i += 2
					continue
				}
				i++
			}
			if i < len(s) && s[i] == q {
				i++
			}
			continue
		}
		i++
	}
	return inTriple != 0
}

// startREPL runs the interactive Alca REPL session.
func startREPL(args []string) {
	memFlags, args, merr := parseMemoryFlags(args)
	if merr != nil {
		fmt.Fprintln(os.Stderr, "error:", merr)
		fmt.Fprintln(os.Stderr, "Usage: alca repl ["+sandbox.FlagHint+"]")
		return
	}
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
	applyMemoryFlags(memFlags)
	if wd, err := os.Getwd(); err == nil {
		mod.SetProjectRoot(wd)
	}

	fmt.Printf("Alca v%s REPL\n", Version)
	fmt.Println("Every simple statement ends with ';'. A line ending with ':' starts a block;")
	fmt.Println("finish the block with an empty line. Type 'exit' or 'quit' to leave.")
	if !policy.Read.All && !policy.Write.All && !policy.Net.All && len(policy.Read.Roots) == 0 && len(policy.Write.Roots) == 0 && len(policy.Net.Roots) == 0 {
		fmt.Println("File and network access are disabled; restart with --allow-read/--allow-write/--allow-net or --allow-all.")
	}

	session := newREPLSession()

	for {
		fmt.Print("alca> ")
		line, ok := interpreter.ReadStdinLine()
		if !ok {
			break
		}
		trimmed := strings.TrimSpace(line)

		if trimmed == "exit" || trimmed == "quit" {
			break
		}
		if trimmed == "" {
			continue
		}
		if out, ok := session.command(trimmed); ok {
			fmt.Print(out)
			continue
		}

		// Handle multi-line block input (if line ends with ':') or multiline strings
		var inputBuilder strings.Builder
		inputBuilder.WriteString(line)

		isBlock := strings.HasSuffix(trimmed, ":")
		for isBlock || hasUnclosedTripleQuote(inputBuilder.String()) {
			fmt.Print("...   ")
			subLine, ok := interpreter.ReadStdinLine()
			if !ok {
				break
			}
			// Block input ends on an empty line, unless inside an unclosed multiline string
			if isBlock && !hasUnclosedTripleQuote(inputBuilder.String()) && strings.TrimSpace(subLine) == "" {
				break
			}
			inputBuilder.WriteString("\n")
			inputBuilder.WriteString(subLine)
			if !isBlock && !hasUnclosedTripleQuote(inputBuilder.String()) {
				break
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
