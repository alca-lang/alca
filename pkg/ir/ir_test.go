// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

package ir_test

import (
	"testing"

	"github.com/pawl-lang/pawl/pkg/ir"
	"github.com/pawl-lang/pawl/pkg/lexer"
	"github.com/pawl-lang/pawl/pkg/parser"
	"github.com/pawl-lang/pawl/pkg/typecheck"
)

func lowerSnippet(t *testing.T, src string) *ir.Program {
	t.Helper()
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if len(p.SyntaxErrors()) > 0 {
		t.Fatalf("syntax error: %v", p.SyntaxErrors()[0])
	}
	res, errs := typecheck.CheckProgram(prog, "test.al", src)
	if len(errs) > 0 {
		t.Fatalf("type error: %v", errs[0])
	}
	irProg, err := ir.Lower(prog, res)
	if err != nil {
		t.Fatalf("ir lowering error: %v", err)
	}
	return irProg
}

func TestLowerFib(t *testing.T) {
	src := `
func fib(n: int) -> int {
    if n <= 1 {
        return n
    }
    return fib(n - 1) + fib(n - 2)
}
`
	prog := lowerSnippet(t, src)
	if len(prog.Functions) != 2 { // fib and main
		t.Fatalf("expected 2 functions, got %d", len(prog.Functions))
	}
	fibFn := prog.Functions[0]
	if fibFn.Name != "fib" {
		t.Fatalf("first fn name = %s, want fib", fibFn.Name)
	}
	if len(fibFn.Params) != 1 || fibFn.Params[0].Type != ir.TypeI64 {
		t.Fatalf("fib param = %+v", fibFn.Params)
	}
	if fibFn.ReturnType != ir.TypeI64 {
		t.Fatalf("fib return = %s", fibFn.ReturnType)
	}
}

func TestLowerForLoop(t *testing.T) {
	src := `
sum := 0
for i in 1..=10 {
    sum = sum + i
}
`
	prog := lowerSnippet(t, src)
	mainFn := prog.EntryFn
	if mainFn == nil {
		t.Fatal("entry fn is nil")
	}

	foundBranch := false
	for _, b := range mainFn.Blocks {
		for _, inst := range b.Instructions {
			if inst.Op == ir.OpBranch {
				foundBranch = true
			}
		}
	}
	if !foundBranch {
		t.Fatal("expected OpBranch in for loop lowering")
	}
}
