// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package parser_test

import (
	"testing"

	"github.com/alca-lang/alca/pkg/ast"
	"github.com/alca-lang/alca/pkg/lexer"
	"github.com/alca-lang/alca/pkg/parser"
)

func TestFunctionDeclarationWithBraces(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		fnName   string
		numStmts int
	}{
		{
			name:     "single line with return",
			src:      "func add(a: int, b: int) -> int { return a + b; }",
			fnName:   "add",
			numStmts: 1,
		},
		{
			name:     "single line void",
			src:      "func greet(name: string) { println(name); }",
			fnName:   "greet",
			numStmts: 1,
		},
		{
			name:     "empty function",
			src:      "func noop() {}",
			fnName:   "noop",
			numStmts: 0,
		},
		{
			name: "multiline function",
			src: `func calculate(x: int, y: int) -> int {
    z := x * y;
    return z + 1;
}`,
			fnName:   "calculate",
			numStmts: 2,
		},
		{
			name: "function with if control flow inside braces",
			src: `func abs(n: int) -> int {
    if n < 0:
        return -n;
    return n;
}`,
			fnName:   "abs",
			numStmts: 2,
		},
		{
			name: "function with opening brace on next line",
			src: `func hello()
{
    println("hello world");
}`,
			fnName:   "hello",
			numStmts: 1,
		},
		{
			name: "legacy colon syntax still supported",
			src: `func legacy(x: int) -> int:
    return x * 2;`,
			fnName:   "legacy",
			numStmts: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New(tt.src)
			p := parser.New(l)
			prog := p.ParseProgram()

			if len(p.SyntaxErrors()) > 0 {
				t.Fatalf("unexpected syntax errors for %s: %s", tt.name, p.DiagnosticReport("<test>", tt.src, false))
			}
			if len(prog.Statements) != 1 {
				t.Fatalf("expected 1 statement, got %d", len(prog.Statements))
			}
			fn, ok := prog.Statements[0].(*ast.FuncDeclaration)
			if !ok {
				t.Fatalf("expected *ast.FuncDeclaration, got %T", prog.Statements[0])
			}
			if fn.Name != tt.fnName {
				t.Errorf("expected function name %q, got %q", tt.fnName, fn.Name)
			}
			if len(fn.Body.Statements) != tt.numStmts {
				t.Errorf("expected %d body statements, got %d", tt.numStmts, len(fn.Body.Statements))
			}
		})
	}
}

func TestClassAndStructMethodsWithBraces(t *testing.T) {
	src := `class MathService:
    func double(x: int) -> int {
        return x * 2;
    }
`
	l := lexer.New(src)
	p := parser.New(l)
	prog := p.ParseProgram()

	if len(p.SyntaxErrors()) > 0 {
		t.Fatalf("unexpected syntax error: %s", p.DiagnosticReport("<test>", src, false))
	}
	classDecl, ok := prog.Statements[0].(*ast.ClassDeclaration)
	if !ok {
		t.Fatalf("expected ClassDeclaration, got %T", prog.Statements[0])
	}
	if len(classDecl.Methods) != 1 {
		t.Fatalf("expected 1 method, got %d", len(classDecl.Methods))
	}
	m := classDecl.Methods[0]
	if m.Func.Name != "double" {
		t.Errorf("expected method name 'double', got %q", m.Func.Name)
	}
	if len(m.Func.Body.Statements) != 1 {
		t.Errorf("expected 1 statement in method, got %d", len(m.Func.Body.Statements))
	}
}
