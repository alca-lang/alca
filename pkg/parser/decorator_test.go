package parser_test

import (
	"strings"
	"testing"

	"github.com/alca-lang/alca/pkg/ast"
	"github.com/alca-lang/alca/pkg/lexer"
	"github.com/alca-lang/alca/pkg/parser"
)

// TestDecoratorWithArgumentsParses verifies that a decorator with an
// argument list is parsed and that the declaration that follows it is still
// parsed. The argument list of a decorator used to leave the parser on the
// closing parenthesis, which made the following declaration invisible.
func TestDecoratorWithArgumentsParses(t *testing.T) {
	cases := []string{
		"@timeout(5s)\nfunc f() -> int:\n    return 1;\n",
		"@oom_guard(\"max=8mb\")\nfunc f() -> int:\n    return 1;\n",
		"@cache(60, \"key\")\nfunc f() -> int:\n    return 1;\n",
	}
	for _, src := range cases {
		p := parser.New(lexer.New(src))
		prog := p.ParseProgram()
		if errs := p.SyntaxErrors(); len(errs) > 0 {
			t.Fatalf("syntax errors for %q: %v", src, errs)
		}
		if len(prog.Statements) != 1 {
			t.Fatalf("program for %q has %d statements, want 1", src, len(prog.Statements))
		}
		decl, ok := prog.Statements[0].(*ast.FuncDeclaration)
		if !ok {
			t.Fatalf("statement for %q is %T, want a function declaration", src, prog.Statements[0])
		}
		if decl.Name != "f" {
			t.Fatalf("function name = %q, want %q", decl.Name, "f")
		}
		if len(decl.Decorators) != 1 {
			t.Fatalf("decorators = %d, want 1", len(decl.Decorators))
		}
		if decl.Decorators[0].Name == "" {
			t.Fatal("the decorator must keep its name")
		}
	}
}

// TestDecoratorWithoutArgumentsStillParses verifies that a bare decorator is
// unaffected by the argument list handling.
func TestDecoratorWithoutArgumentsStillParses(t *testing.T) {
	src := "@memoize\nfunc f() -> int:\n    return 1;\n"
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if errs := p.SyntaxErrors(); len(errs) > 0 {
		t.Fatalf("syntax errors: %v", errs)
	}
	decl, ok := prog.Statements[0].(*ast.FuncDeclaration)
	if !ok {
		t.Fatalf("statement is %T, want a function declaration", prog.Statements[0])
	}
	if len(decl.Decorators) != 1 || decl.Decorators[0].Name != "memoize" {
		t.Fatalf("decorators = %+v, want a single memoize decorator", decl.Decorators)
	}
	if !strings.Contains(decl.String(), "f") {
		t.Fatalf("declaration text = %q, want it to mention the function", decl.String())
	}
}
