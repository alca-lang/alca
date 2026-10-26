// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package parser_test

import (
	"testing"

	"github.com/alca-lang/alca/pkg/ast"
	"github.com/alca-lang/alca/pkg/lexer"
	"github.com/alca-lang/alca/pkg/parser"
	"github.com/alca-lang/alca/pkg/token"
)

func TestVoidFunctionDeclaration(t *testing.T) {
	tests := []struct {
		name       string
		src        string
		fnName     string
		isPublic   bool
		retType    string
		numStmts   int
		expectFail bool
	}{
		{
			name:     "bare void function with braces",
			src:      "void greet() { println(\"hello\"); }",
			fnName:   "greet",
			retType:  "void",
			numStmts: 1,
		},
		{
			name:     "bare void function with colon",
			src:      "void greet():\n    println(\"hello\");\n",
			fnName:   "greet",
			retType:  "void",
			numStmts: 1,
		},
		{
			name:     "pub void function",
			src:      "pub void process(item: string) { println(item); }",
			fnName:   "process",
			isPublic: true,
			retType:  "void",
			numStmts: 1,
		},
		{
			name:     "public void function",
			src:      "public void notify(code: int) { println(code); }",
			fnName:   "notify",
			isPublic: true,
			retType:  "void",
			numStmts: 1,
		},
		{
			name:     "void func keyword combination",
			src:      "void func run() { println(\"running\"); }",
			fnName:   "run",
			retType:  "void",
			numStmts: 1,
		},
		{
			name:     "void function with early return",
			src:      "void stop_early() { return; }",
			fnName:   "stop_early",
			retType:  "void",
			numStmts: 1,
		},
		{
			name:     "void function with newline opening brace",
			src:      "void action()\n{\n    return;\n}",
			fnName:   "action",
			retType:  "void",
			numStmts: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l := lexer.New(tc.src)
			p := parser.New(l)
			prog := p.ParseProgram()

			if len(p.Errors()) > 0 {
				if tc.expectFail {
					return
				}
				t.Fatalf("parser errors: %v", p.Errors())
			}

			if len(prog.Statements) != 1 {
				t.Fatalf("expected 1 statement, got %d", len(prog.Statements))
			}

			fn, ok := prog.Statements[0].(*ast.FuncDeclaration)
			if !ok {
				t.Fatalf("expected *ast.FuncDeclaration, got %T", prog.Statements[0])
			}

			if fn.Name != tc.fnName {
				t.Errorf("expected func name %q, got %q", tc.fnName, fn.Name)
			}

			if fn.IsPublic != tc.isPublic {
				t.Errorf("expected isPublic=%v, got %v", tc.isPublic, fn.IsPublic)
			}

			if fn.ReturnType == nil || fn.ReturnType.String() != tc.retType {
				t.Errorf("expected return type %q, got %v", tc.retType, fn.ReturnType)
			}

			if fn.Body == nil || len(fn.Body.Statements) != tc.numStmts {
				t.Errorf("expected %d statements in body, got %d", tc.numStmts, len(fn.Body.Statements))
			}
		})
	}
}

func TestClassWithVoidMethods(t *testing.T) {
	src := `class Worker:
    pub void doWork():
        println("working");

    void finish() {
        println("done");
    }
`
	l := lexer.New(src)
	p := parser.New(l)
	prog := p.ParseProgram()

	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	if len(prog.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Statements))
	}

	cls, ok := prog.Statements[0].(*ast.ClassDeclaration)
	if !ok {
		t.Fatalf("expected *ast.ClassDeclaration, got %T", prog.Statements[0])
	}

	if len(cls.Methods) != 2 {
		t.Fatalf("expected 2 methods, got %d", len(cls.Methods))
	}

	m1 := cls.Methods[0]
	if m1.Func.Name != "doWork" || m1.Func.ReturnType == nil || m1.Func.ReturnType.String() != "void" {
		t.Errorf("m1 mismatch: %v", m1.Func)
	}
	if m1.Visibility != "public" {
		t.Errorf("expected m1 visibility 'public', got %q", m1.Visibility)
	}

	m2 := cls.Methods[1]
	if m2.Func.Name != "finish" || m2.Func.ReturnType == nil || m2.Func.ReturnType.String() != "void" {
		t.Errorf("m2 mismatch: %v", m2.Func)
	}
}

func TestVoidFuncTokenFormatting(t *testing.T) {
	src := "void ping(x: int) {}"
	l := lexer.New(src)
	p := parser.New(l)
	prog := p.ParseProgram()

	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	fn := prog.Statements[0].(*ast.FuncDeclaration)
	if fn.Token.Type != token.TYPE_VOID {
		t.Errorf("expected token.TYPE_VOID, got %v", fn.Token.Type)
	}
	str := fn.String()
	if str != "void ping(x: int) {\n}" {
		t.Errorf("expected 'void ping(x: int) {\\n}', got %q", str)
	}
}
