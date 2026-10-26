// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package vm

import (
	"testing"

	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/lexer"
	"github.com/alca-lang/alca/pkg/parser"
)

// TestInvokeCompiledCallback verifies that host code (GUI callbacks, HTTP
// handlers) can invoke a VM closure through interpreter.Invoke and that it
// shares globals and captured upvalues with the suspended program.
func TestInvokeCompiledCallback(t *testing.T) {
	src := `
count := 0;
func bump() -> int:
    count = count + 1;
    return count;
`
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if errs := p.SyntaxErrors(); len(errs) > 0 {
		t.Fatalf("syntax errors: %v", errs)
	}
	c := NewCompiler()
	if err := c.Compile(prog); err != nil {
		t.Fatalf("compile: %v", err)
	}
	machine := NewWithEnv(c.Bytecode(), interpreter.New().Env())
	if err := machine.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}

	index := -1
	for i, name := range c.GlobalNames() {
		if name == "bump" {
			index = i
			break
		}
	}
	if index < 0 {
		t.Fatal("bump was not compiled to a global")
	}
	fn, ok := machine.GlobalAt(index).(*CompiledFunction)
	if !ok {
		t.Fatalf("expected *CompiledFunction, got %T", machine.GlobalAt(index))
	}

	for want := int64(1); want <= 3; want++ {
		res, err := interpreter.Invoke(fn, nil)
		if err != nil {
			t.Fatalf("invoke: %v", err)
		}
		iv, ok := res.(*interpreter.IntegerValue)
		if !ok || iv.Val != want {
			t.Fatalf("call %d returned %v, want %d", want, res, want)
		}
	}
}
