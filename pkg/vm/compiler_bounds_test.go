// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package vm

import (
	"fmt"
	"strings"
	"testing"

	"github.com/alca-lang/alca/pkg/ast"
	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/lexer"
	"github.com/alca-lang/alca/pkg/parser"
)

func TestMakeCheckedRejectsOutOfRangeOperands(t *testing.T) {
	if _, err := MakeChecked(OpGetLocal, 256); err == nil {
		t.Fatal("expected 8-bit operand overflow to be rejected")
	}
	if _, err := MakeChecked(OpConstant, 65536); err == nil {
		t.Fatal("expected 16-bit operand overflow to be rejected")
	}
}

func TestCompilerRejectsOutOfRangeLocalOperand(t *testing.T) {
	c := NewCompiler()
	c.emit(OpGetLocal, 256)
	if err := c.Compile(&ast.Program{}); err == nil {
		t.Fatal("expected compiler to report an out-of-range local index")
	}
}

func TestCompilerRejectsTooManyFunctionParameters(t *testing.T) {
	var src strings.Builder
	src.WriteString("func too_many(")
	for i := 0; i < 256; i++ {
		if i > 0 {
			src.WriteString(", ")
		}
		fmt.Fprintf(&src, "p%d: int", i)
	}
	src.WriteString(") -> int:\n    return 0;\n")

	p := parser.New(lexer.New(src.String()))
	program := p.ParseProgram()
	if errs := p.SyntaxErrors(); len(errs) > 0 {
		t.Fatalf("unexpected syntax errors: %v", errs)
	}
	if err := NewCompiler().Compile(program); err == nil {
		t.Fatal("expected compiler to reject more than 255 parameters")
	}
}

func TestCompilerRejectsTooManyConstants(t *testing.T) {
	c := NewCompiler()
	c.constants = make([]interpreter.Value, 0x10000)
	c.addConstant(interpreter.NewInteger(1))
	if err := c.Compile(&ast.Program{}); err == nil {
		t.Fatal("expected compiler to reject a constant index outside the 16-bit range")
	}
}

func TestCompilerRejectsOutOfRangeJumpTarget(t *testing.T) {
	c := NewCompiler()
	jumpPos := c.emit(OpJump, 0)
	c.scopes[c.scopeIndex].instructions = append(c.scopes[c.scopeIndex].instructions, make([]byte, 0x10000)...)
	c.patch(jumpPos)
	if err := c.Compile(&ast.Program{}); err == nil {
		t.Fatal("expected compiler to reject an out-of-range jump target")
	}
}
