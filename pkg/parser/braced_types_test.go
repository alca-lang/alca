// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"testing"

	"github.com/alca-lang/alca/pkg/ast"
	"github.com/alca-lang/alca/pkg/lexer"
)

func TestBracedStructAndClassAndMatch(t *testing.T) {
	src := `
struct Point {
    x: int;
    y: int;
}

class User {
    pub name: string;
    pub age: int = 18;

    pub fn get_name() -> string {
        return self.name;
    }
}

match 1 {
    case 1:
        println("one");
    else:
        println("other");
}
`
	l := lexer.New(src)
	p := New(l)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	if len(prog.Statements) != 3 {
		t.Fatalf("expected 3 statements, got %d", len(prog.Statements))
	}

	s, ok := prog.Statements[0].(*ast.StructDeclaration)
	if !ok || s.Name != "Point" || len(s.Fields) != 2 {
		t.Fatalf("expected Point struct with 2 fields, got %#v", prog.Statements[0])
	}

	c, ok := prog.Statements[1].(*ast.ClassDeclaration)
	if !ok || c.Name != "User" || len(c.Fields) != 2 || len(c.Methods) != 1 {
		t.Fatalf("expected User class with 2 fields and 1 method, got %#v", prog.Statements[1])
	}

	m, ok := prog.Statements[2].(*ast.MatchStatement)
	if !ok || len(m.Cases) != 1 || m.Default == nil {
		t.Fatalf("expected Match statement, got %#v", prog.Statements[2])
	}
}

func TestEnumInterfaceAsyncAwait(t *testing.T) {
	src := `
enum Status {
    Pending,
    Active = 10,
    Done,
}

interface Greeter {
    fn greet(name: string) -> string;
}

async fn fetch_data() -> string {
    return "ok";
}

val := await fetch_data();
`
	l := lexer.New(src)
	p := New(l)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	if len(prog.Statements) != 4 {
		t.Fatalf("expected 4 statements, got %d", len(prog.Statements))
	}

	en, ok := prog.Statements[0].(*ast.EnumDeclaration)
	if !ok || en.Name != "Status" || len(en.Variants) != 3 {
		t.Fatalf("expected Status enum with 3 variants, got %#v", prog.Statements[0])
	}

	iface, ok := prog.Statements[1].(*ast.InterfaceDeclaration)
	if !ok || iface.Name != "Greeter" || len(iface.Methods) != 1 {
		t.Fatalf("expected Greeter interface with 1 method, got %#v", prog.Statements[1])
	}

	fn, ok := prog.Statements[2].(*ast.FuncDeclaration)
	if !ok || fn.Name != "fetch_data" || !fn.IsAsync {
		t.Fatalf("expected async function fetch_data, got %#v", prog.Statements[2])
	}

	vdec, ok := prog.Statements[3].(*ast.VarDeclaration)
	if !ok {
		t.Fatalf("expected VarDeclaration, got %#v", prog.Statements[3])
	}
	_, isAwait := vdec.Value.(*ast.AwaitExpression)
	if !isAwait {
		t.Fatalf("expected AwaitExpression, got %#v", vdec.Value)
	}
}
