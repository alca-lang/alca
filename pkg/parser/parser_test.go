// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

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

func TestClassDeclaration(t *testing.T) {
	src := `class Dog extends Animal:
    public string: name;
    private int: age = 0;

    public func speak() -> string:
        return self.name;

    private func grow() -> int:
        self.age = self.age + 1;
        return self.age;

    public struct Toy:
        public string: label;
`
	prog := mustParse(t, src)
	class, ok := prog.Statements[0].(*ast.ClassDeclaration)
	if !ok {
		t.Fatalf("want ClassDeclaration, got %T", prog.Statements[0])
	}
	if class.Name != "Dog" || class.Parent != "Animal" {
		t.Fatalf("class = %q extends %q, want Dog extends Animal", class.Name, class.Parent)
	}
	if len(class.Fields) != 2 || class.Fields[0].Name != "name" || class.Fields[1].Visibility != "private" {
		t.Fatalf("unexpected fields: %+v", class.Fields)
	}
	if len(class.Methods) != 2 || class.Methods[0].Name() != "speak" || class.Methods[1].Visibility != "private" {
		t.Fatalf("unexpected methods: %+v", class.Methods)
	}
	if len(class.NestedStructs) != 1 || class.NestedStructs[0].Name != "Toy" {
		t.Fatalf("unexpected nested structs: %+v", class.NestedStructs)
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

func TestMultilineStringParsing(t *testing.T) {
	src := "s := \"\"\"line 1\nline 2\"\"\";\nraw := r\"\"\"raw\\nline\"\"\";\n"
	prog := mustParse(t, src)
	if len(prog.Statements) != 2 {
		t.Fatalf("want 2 statements, got %d", len(prog.Statements))
	}
	sDecl := prog.Statements[0].(*ast.VarDeclaration)
	strLit, ok := sDecl.Value.(*ast.StringLiteral)
	if !ok || strLit.Value != "line 1\nline 2" {
		t.Fatalf("unexpected value: %v", sDecl.Value)
	}

	rawDecl := prog.Statements[1].(*ast.VarDeclaration)
	rawLit, ok := rawDecl.Value.(*ast.StringLiteral)
	if !ok || rawLit.Value != "raw\\nline" {
		t.Fatalf("unexpected raw value: %v", rawDecl.Value)
	}
}

func TestFStringParsing(t *testing.T) {
	t.Run("literal only with escaped braces", func(t *testing.T) {
		src := "s := f\"{{hello}} world\";\n"
		prog := mustParse(t, src)
		decl := prog.Statements[0].(*ast.VarDeclaration)
		lit, ok := decl.Value.(*ast.StringLiteral)
		if !ok || lit.Value != "{hello} world" {
			t.Fatalf("expected literal '{hello} world', got %v", decl.Value)
		}
	})

	t.Run("single interpolation", func(t *testing.T) {
		src := "s := f\"hello {name}!\";\n"
		prog := mustParse(t, src)
		decl := prog.Statements[0].(*ast.VarDeclaration)
		// Expect: ("hello " + string(name)) + "!"
		infix, ok := decl.Value.(*ast.InfixExpression)
		if !ok || infix.Operator != "+" {
			t.Fatalf("expected infix +, got %T", decl.Value)
		}
		rightLit, ok := infix.Right.(*ast.StringLiteral)
		if !ok || rightLit.Value != "!" {
			t.Fatalf("expected right literal '!', got %v", infix.Right)
		}
		leftInfix, ok := infix.Left.(*ast.InfixExpression)
		if !ok || leftInfix.Operator != "+" {
			t.Fatalf("expected left infix +, got %v", infix.Left)
		}
		call, ok := leftInfix.Right.(*ast.CallExpression)
		if !ok {
			t.Fatalf("expected CallExpression, got %T", leftInfix.Right)
		}
		fnIdent, ok := call.Function.(*ast.Identifier)
		if !ok || fnIdent.Value != "string" {
			t.Fatalf("expected function 'string', got %v", call.Function)
		}
	})
}

func TestSyntaxErrorDiagnosticsAndSuggestions(t *testing.T) {
	tests := []struct {
		name           string
		src            string
		wantErrSubstr  string
		wantHelpSubstr string
	}{
		{
			name:           "stray closing paren",
			src:            ");\n",
			wantErrSubstr:  "unexpected closing ')'",
			wantHelpSubstr: "unclosed '(' earlier",
		},
		{
			name:           "stray closing bracket",
			src:            "];\n",
			wantErrSubstr:  "unexpected closing ']'",
			wantHelpSubstr: "unclosed '[' earlier",
		},
		{
			name:           "stray closing brace",
			src:            "};\n",
			wantErrSubstr:  "unexpected closing '}'",
			wantHelpSubstr: "unclosed '{' earlier",
		},
		{
			name:           "stray plus operator",
			src:            "+ 1 2;\n",
			wantErrSubstr:  "stray arithmetic operator",
			wantHelpSubstr: "provide an expression to the left",
		},
		{
			name:           "stray asterisk operator",
			src:            "* 10;\n",
			wantErrSubstr:  "stray arithmetic operator",
			wantHelpSubstr: "provide an expression to the left",
		},
		{
			name:           "stray equality operator",
			src:            "== 5;\n",
			wantErrSubstr:  "comparison operator",
			wantHelpSubstr: "comparison operators require a left-hand expression",
		},
		{
			name:           "stray logical and",
			src:            "and true;\n",
			wantErrSubstr:  "logical operator",
			wantHelpSubstr: "provide a boolean expression to the left",
		},
		{
			name:           "stray pipe operator",
			src:            "|> println;\n",
			wantErrSubstr:  "pipeline operator",
			wantHelpSubstr: "passes it to the function",
		},
		{
			name:           "stray colon",
			src:            ": 123;\n",
			wantErrSubstr:  "unexpected ':'",
			wantHelpSubstr: "use ':' only at the end of block headers",
		},
		{
			name:           "stray comma",
			src:            ", 123;\n",
			wantErrSubstr:  "unexpected ','",
			wantHelpSubstr: "remove the extra comma",
		},
		{
			name:           "empty semicolon statement",
			src:            ";\n",
			wantErrSubstr:  "unexpected ';'",
			wantHelpSubstr: "remove the stray semicolon",
		},
		{
			name:           "missing if condition",
			src:            "if:\n    x := 1;\n",
			wantErrSubstr:  "missing condition in 'if'",
			wantHelpSubstr: "provide a boolean condition",
		},
		{
			name:           "missing while condition",
			src:            "while:\n    break;\n",
			wantErrSubstr:  "missing condition in 'while'",
			wantHelpSubstr: "provide a loop condition",
		},
		{
			name:           "missing match expression",
			src:            "match:\n    case 1:\n        x := 1;\n",
			wantErrSubstr:  "missing expression in 'match'",
			wantHelpSubstr: "provide a match expression",
		},
		{
			name:           "missing case pattern",
			src:            "match n:\n    case:\n        x := 1;\n",
			wantErrSubstr:  "missing pattern in 'case'",
			wantHelpSubstr: "provide a pattern to match",
		},
		{
			name:           "missing for variable",
			src:            "for in [1, 2, 3]:\n    println(1);\n",
			wantErrSubstr:  "expected loop variable name",
			wantHelpSubstr: "specify a variable name",
		},
		{
			name:           "missing foreach variable",
			src:            "foreach in [1, 2, 3]:\n    println(1);\n",
			wantErrSubstr:  "expected loop variable name",
			wantHelpSubstr: "specify a variable name",
		},
		{
			name:           "missing function name",
			src:            "func (): \n    return 1;\n",
			wantErrSubstr:  "expected function name",
			wantHelpSubstr: "provide a valid function name",
		},
		{
			name:           "missing struct name",
			src:            "struct :\n    a: int;\n",
			wantErrSubstr:  "expected struct name",
			wantHelpSubstr: "provide a struct name",
		},
		{
			name:           "missing decorator name",
			src:            "@ 123\nfunc f():\n    return 1;\n",
			wantErrSubstr:  "expected identifier after '@'",
			wantHelpSubstr: "provide a decorator name",
		},
		{
			name:           "missing colon in parameter type",
			src:            "func f(x int):\n    return 1;\n",
			wantErrSubstr:  "missing ':' before type annotation for parameter",
			wantHelpSubstr: "write parameter types with a colon",
		},
		{
			name:           "missing identifier after dot",
			src:            "x := obj. ;\n",
			wantErrSubstr:  "expected identifier after '.'",
			wantHelpSubstr: "provide a valid field or method name",
		},
		{
			name:           "unclosed paren",
			src:            "x := (1 + 2;\n",
			wantErrSubstr:  "expected ')' to close parenthesis",
			wantHelpSubstr: "unclosed '(' from line 1, col 6",
		},
		{
			name:           "unclosed bracket index",
			src:            "x := a[10;\n",
			wantErrSubstr:  "expected ']' to close list or index",
			wantHelpSubstr: "unclosed '[' from line 1, col 7",
		},
		{
			name:           "unclosed map literal",
			src:            "x := {\"key\": 1;\n",
			wantErrSubstr:  "expected '}' to close block or map",
			wantHelpSubstr: "unclosed '{' from line 1, col 6",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := parser.New(lexer.New(tc.src))
			p.ParseProgram()
			syntaxErrs := p.SyntaxErrors()
			if len(syntaxErrs) == 0 {
				t.Fatalf("expected syntax error for source:\n%s", tc.src)
			}

			foundErr := false
			foundHelp := false
			var errMsgs []string
			var suggestions []string

			for _, err := range syntaxErrs {
				errMsgs = append(errMsgs, err.Message)
				suggestions = append(suggestions, err.Suggestion)
				if strings.Contains(err.Message, tc.wantErrSubstr) {
					foundErr = true
				}
				if strings.Contains(err.Suggestion, tc.wantHelpSubstr) {
					foundHelp = true
				}
			}

			if !foundErr {
				t.Errorf("missing expected error substring %q; got errors: %v", tc.wantErrSubstr, errMsgs)
			}
			if !foundHelp {
				t.Errorf("missing expected suggestion substring %q; got suggestions: %v", tc.wantHelpSubstr, suggestions)
			}
		})
	}
}

func TestNoCascadingSemicolonOnBadExpression(t *testing.T) {
	// A bad expression should report an error for the invalid token,
	// but NOT a secondary cascading "expected ';' at end of statement".
	src := "* 1 2;\nx := 10;\n"
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	errs := p.SyntaxErrors()

	for _, err := range errs {
		if strings.Contains(err.Message, "expected ';'") {
			t.Fatalf("cascading 'expected ;' error was reported: %s", err.Message)
		}
	}
	if len(prog.Statements) != 1 {
		t.Fatalf("expected parser to recover and parse subsequent statement, got %d statements", len(prog.Statements))
	}
}
