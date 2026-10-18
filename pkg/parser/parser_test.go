package parser_test

import (
	"strings"
	"testing"

	"github.com/alca-lang/alca/pkg/ast"
	"github.com/alca-lang/alca/pkg/lexer"
	"github.com/alca-lang/alca/pkg/parser"
)

func parse(src string) (*ast.Program, []string) {
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	var msgs []string
	for _, e := range p.SyntaxErrors() {
		msgs = append(msgs, e.Message)
	}
	return prog, msgs
}

func mustParse(t *testing.T, src string) *ast.Program {
	t.Helper()
	prog, errs := parse(src)
	if len(errs) > 0 {
		t.Fatalf("unexpected syntax errors: %v", errs)
	}
	return prog
}

func TestAnnotatedAndInferredDeclarations(t *testing.T) {
	prog := mustParse(t, "y: int = 10;\nname: string | null = \"alice\";\nz := 3.5;\n")
	if len(prog.Statements) != 3 {
		t.Fatalf("want 3 statements, got %d", len(prog.Statements))
	}
	y := prog.Statements[0].(*ast.VarDeclaration)
	if y.TypeAnnotation == nil || y.TypeAnnotation.Types[0] != "int" {
		t.Errorf("y: bad annotation %v", y.TypeAnnotation)
	}
	n := prog.Statements[1].(*ast.VarDeclaration)
	if len(n.TypeAnnotation.Types) != 2 {
		t.Errorf("name: want union of 2 types, got %v", n.TypeAnnotation.Types)
	}
	z := prog.Statements[2].(*ast.VarDeclaration)
	if z.TypeAnnotation != nil || z.Value == nil {
		t.Errorf("z := 3.5 must be an inferred declaration with a value")
	}
}

func TestPlainAssignmentIsNotADeclaration(t *testing.T) {
	prog := mustParse(t, "x = 5;")
	es, ok := prog.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("want ExpressionStatement, got %T", prog.Statements[0])
	}
	if _, ok := es.Expression.(*ast.AssignExpression); !ok {
		t.Fatalf("want AssignExpression, got %T", es.Expression)
	}
}

func TestSemicolonIsMandatory(t *testing.T) {
	cases := map[string]string{
		"declaration":  "x: int = 5\n",
		"expression":   "println(1)\n",
		"return":       "func f() -> int:\n    return 1\n",
		"use":          "use std.math\n",
		"struct field": "struct S:\n    a: int\n",
		"break":        "while true:\n    break\n",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			_, errs := parse(src)
			if len(errs) == 0 {
				t.Fatalf("expected a missing-';' error")
			}
			if !strings.Contains(strings.Join(errs, " "), "expected ';'") {
				t.Fatalf("wrong diagnostic: %v", errs)
			}
		})
	}
}

func TestSemicolonRecovery(t *testing.T) {
	prog, errs := parse("a: int = 1\nb: int = 2;\nc: int = 3;\n")
	if len(errs) != 1 {
		t.Fatalf("want exactly 1 error (no cascade), got %v", errs)
	}
	if len(prog.Statements) != 3 {
		t.Fatalf("parser must recover and keep all 3 statements, got %d", len(prog.Statements))
	}
}

func TestBlocksDoNotTakeSemicolons(t *testing.T) {
	src := "func f(a: int) -> int:\n    if a > 0:\n        return a;\n    elif a == 0:\n        return 0;\n    else:\n        return -a;\n\nwhile false:\n    break;\nfor i in 0..3:\n    println(i);\n"
	mustParse(t, src)
}

func TestCStyleDeclarationRejected(t *testing.T) {
	_, errs := parse("int x = 5;\n")
	if len(errs) == 0 || !strings.Contains(errs[0], "C-style") {
		t.Fatalf("want C-style declaration error, got %v", errs)
	}
}

func TestDeclarationNeedsInitializer(t *testing.T) {
	_, errs := parse("x: int;\n")
	if len(errs) == 0 || !strings.Contains(errs[0], "initial value") {
		t.Fatalf("want initializer error, got %v", errs)
	}
}

func TestErrCodeKeywordIsNotAnAssignment(t *testing.T) {
	prog := mustParse(t, "func f():\n    err(\"x\", code=7);\n")
	fn := prog.Statements[0].(*ast.FuncDeclaration)
	es := fn.Body.Statements[0].(*ast.ExpressionStatement)
	e := es.Expression.(*ast.ErrExpression)
	if _, isAssign := e.Code.(*ast.AssignExpression); isAssign {
		t.Fatalf("code=7 must not parse as an assignment to a variable named code")
	}
	if lit, ok := e.Code.(*ast.IntegerLiteral); !ok || lit.Value != 7 {
		t.Fatalf("want integer literal 7, got %T", e.Code)
	}
}

func TestStructAndMatch(t *testing.T) {
	src := "struct Server:\n    ip: string;\n    port: int = 80;\n\nn: int = 1;\nmatch n:\n    case 1:\n        println(\"one\");\n    else:\n        println(\"other\");\n"
	prog := mustParse(t, src)
	if _, ok := prog.Statements[0].(*ast.StructDeclaration); !ok {
		t.Fatalf("want StructDeclaration, got %T", prog.Statements[0])
	}
}

// A block whose body is only comment lines produces no INDENT token (comments
// are invisible to the lexer). It must parse as an empty block so that the
// following else/elif/case still binds to the statement.
func TestCommentOnlyBlockBodies(t *testing.T) {
	t.Run("if-else", func(t *testing.T) {
		src := "x := 1;\nif x == 1:\n    // only a comment\nelse:\n    println(\"no\");\n"
		prog := mustParse(t, src)
		stmt, ok := prog.Statements[1].(*ast.IfStatement)
		if !ok {
			t.Fatalf("statement 1: want IfStatement, got %T", prog.Statements[1])
		}
		if len(stmt.Consequence.Statements) != 0 {
			t.Fatalf("consequence: want empty block, got %d statements", len(stmt.Consequence.Statements))
		}
		if stmt.ElseBlock == nil || len(stmt.ElseBlock.Statements) != 1 {
			t.Fatalf("else block must hold the println, got %+v", stmt.ElseBlock)
		}
	})

	t.Run("elif-chain", func(t *testing.T) {
		src := "x := 1;\nif x == 1:\n    // c1\nelif x == 2:\n    // c2\nelse:\n    // c3\n"
		prog := mustParse(t, src)
		stmt := prog.Statements[1].(*ast.IfStatement)
		if len(stmt.Alternatives) != 1 {
			t.Fatalf("want 1 elif clause, got %d", len(stmt.Alternatives))
		}
		if stmt.ElseBlock == nil {
			t.Fatalf("else block missing after comment-only elif body")
		}
	})

	t.Run("match-case-default", func(t *testing.T) {
		src := "value := 1;\nmatch value:\n    case 1:\n        // first case\n    else:\n        println(\"other\");\n"
		prog := mustParse(t, src)
		stmt := prog.Statements[1].(*ast.MatchStatement)
		if len(stmt.Cases) != 1 {
			t.Fatalf("want 1 case, got %d", len(stmt.Cases))
		}
		if stmt.Default == nil || len(stmt.Default.Statements) != 1 {
			t.Fatalf("default block must hold the println, got %+v", stmt.Default)
		}
	})
}

// A statement written on the line after a block header without indentation is
// not a valid body and must be rejected, never silently swallowed.
func TestUnindentedStatementIsNotABlockBody(t *testing.T) {
	src := "x := 1;\nif x == 1:\ny := 2;\n"
	prog, errs := parse(src)
	if len(errs) == 0 || !strings.Contains(errs[0], "expected an indented block") {
		t.Fatalf("want 'expected an indented block', got %v", errs)
	}
	stmt, ok := prog.Statements[1].(*ast.IfStatement)
	if !ok {
		t.Fatalf("statement 1: want IfStatement, got %T", prog.Statements[1])
	}
	if len(stmt.Consequence.Statements) != 0 {
		t.Fatalf("unindented statement was swallowed into the if body")
	}
	if len(prog.Statements) != 3 {
		t.Fatalf("want 3 top-level statements, got %d", len(prog.Statements))
	}
	if _, ok := prog.Statements[2].(*ast.VarDeclaration); !ok {
		t.Fatalf("statement 2: want VarDeclaration, got %T", prog.Statements[2])
	}
}
