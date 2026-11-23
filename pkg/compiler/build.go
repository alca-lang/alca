// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

package compiler

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pawl-lang/pawl/pkg/ast"
	"github.com/pawl-lang/pawl/pkg/lexer"
	"github.com/pawl-lang/pawl/pkg/parser"
	"github.com/pawl-lang/pawl/pkg/typecheck"
)

// Kind represents the compilation artifact output kind.
type Kind int

const (
	Exe Kind = iota
	Shared
	Static
)

func (k Kind) String() string {
	switch k {
	case Exe:
		return "exe"
	case Shared:
		return "shared"
	case Static:
		return "static"
	default:
		return "unknown"
	}
}

// Target is an operating system / architecture pair ("linux/amd64", "windows/amd64").
type Target struct {
	OS   string
	Arch string
}

func (t Target) String() string { return t.OS + "/" + t.Arch }

// HostTarget returns the platform this binary runs on.
func HostTarget() Target { return Target{runtime.GOOS, runtime.GOARCH} }

// SupportedTargets lists the platforms supported for native compilation.
var SupportedTargets = []Target{
	{"linux", "amd64"},
	{"windows", "amd64"},
	{"darwin", "amd64"},
	{"linux", "arm64"},
	{"darwin", "arm64"},
	{"windows", "arm64"},
}

// ParseTarget parses "os/arch" (also accepts "os-arch").
func ParseTarget(s string) (Target, error) {
	s = strings.ReplaceAll(strings.TrimSpace(s), "-", "/")
	parts := strings.Split(s, "/")
	if len(parts) != 2 {
		return Target{}, fmt.Errorf("invalid target %q: expected os/arch, e.g. linux/amd64", s)
	}
	t := Target{strings.ToLower(parts[0]), strings.ToLower(parts[1])}
	for _, ok := range SupportedTargets {
		if ok == t {
			return t, nil
		}
	}
	return Target{}, fmt.Errorf("unsupported target %q (supported: %s)", t, targetList())
}

func targetList() string {
	var s []string
	for _, t := range SupportedTargets {
		s = append(s, t.String())
	}
	return strings.Join(s, ", ")
}

// DefaultOutput returns the default binary filename for a target and kind.
func DefaultOutput(entry string, t Target, k Kind) string {
	base := filepath.Base(entry)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	switch k {
	case Exe:
		if t.OS == "windows" {
			return name + ".exe"
		}
		return name
	case Shared:
		switch t.OS {
		case "windows":
			return name + ".dll"
		case "darwin":
			return "lib" + name + ".dylib"
		default:
			return "lib" + name + ".so"
		}
	case Static:
		switch t.OS {
		case "windows":
			return name + ".lib"
		default:
			return "lib" + name + ".a"
		}
	default:
		return name
	}
}

// Options configures the AOT compilation.
type Options struct {
	Target    Target
	Kind      Kind
	Output    string
	OptLevel  int
	Debug     bool
	NoConsole bool
	Libs      []string
}

// Build compiles a Pawl source file directly to machine code.
// No transpilation, no C compiler invocation, no VM stub.
func Build(entry string, o Options) error {
	if o.Target.OS == "" || o.Target.Arch == "" {
		o.Target = HostTarget()
	}
	if o.Output == "" {
		o.Output = DefaultOutput(entry, o.Target, o.Kind)
	}

	data, err := os.ReadFile(entry)
	if err != nil {
		return fmt.Errorf("cannot read entry file %q: %w", entry, err)
	}
	content := string(data)

	p := parser.New(lexer.New(content))
	prog := p.ParseProgram()
	if len(p.SyntaxErrors()) > 0 {
		return fmt.Errorf("syntax error in %s: %s", entry, p.SyntaxErrors()[0].Message)
	}

	res, errs := typecheck.CheckProgram(prog, entry, content)
	if len(errs) > 0 {
		return fmt.Errorf("type error in %s: %s", entry, errs[0].Message)
	}

	// Supported backend check
	if o.Target.Arch != "amd64" {
		return fmt.Errorf("native backend does not support target architecture %q yet", o.Target.Arch)
	}
	if o.Target.OS != "windows" && o.Target.OS != "linux" {
		return fmt.Errorf("native backend does not support target OS %q yet", o.Target.OS)
	}

	return compileProgramToNative(prog, res, entry, o)
}

// compileProgramToNative coordinates lowering and linking.
// Stubs for unsupported features return structured compile errors.
func compileProgramToNative(prog *ast.Program, typeInfo *typecheck.Result, entry string, o Options) error {
	for _, stmt := range prog.Statements {
		if err := validateSupportedStatement(stmt, entry); err != nil {
			return err
		}
	}

	// In Stage 1, we set up the pipeline contract. Stage 2 will introduce pkg/ir and pkg/codegen.
	// For now, return a clear diagnostic for programs awaiting Stage 2 code generation.
	return fmt.Errorf("native backend code generator is being connected in stage 2 (entry: %s)", entry)
}

func validateSupportedStatement(stmt ast.Statement, file string) error {
	switch s := stmt.(type) {
	case *ast.BaStatement:
		return fmt.Errorf("native backend does not support '@ba' yet (%s:%d)", file, s.Token.Line)
	case *ast.FuncDeclaration:
		if s.IsAsync {
			return fmt.Errorf("native backend does not support 'async' yet (%s:%d)", file, s.Token.Line)
		}
	}
	return nil
}
