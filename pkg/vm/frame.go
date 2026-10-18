package vm

import (
	"fmt"

	"github.com/alca-lang/alca/pkg/ast"
	"github.com/alca-lang/alca/pkg/interpreter"
)

// UseStatementValue packages an ast.UseStatement as an interpreter.Value for OpUse.
type UseStatementValue struct {
	Stmt *ast.UseStatement
}

func (u *UseStatementValue) Type() interpreter.ValueType { return "USE_STATEMENT" }
func (u *UseStatementValue) Inspect() string             { return u.Stmt.String() }
func (u *UseStatementValue) Clone() interpreter.Value    { return &UseStatementValue{Stmt: u.Stmt} }

// CompiledFunction represents a compiled bytecode function closure.
type CompiledFunction struct {
	Instructions  Instructions
	NumLocals     int
	NumParameters int
	Name          string
}

// Type returns interpreter.FUNC_VAL.
func (f *CompiledFunction) Type() interpreter.ValueType {
	return interpreter.FUNC_VAL
}

// Inspect returns a human-readable representation of the compiled function.
func (f *CompiledFunction) Inspect() string {
	if f.Name != "" {
		return fmt.Sprintf("<compiled function %s>", f.Name)
	}
	return fmt.Sprintf("<compiled function at %p>", f)
}

// Clone returns the compiled function instance.
func (f *CompiledFunction) Clone() interpreter.Value {
	return f
}

// Frame represents an active call frame in the Bytecode Virtual Machine call stack.
type Frame struct {
	fn          *CompiledFunction
	ip          int
	basePointer int
}

// NewFrame creates a new execution Frame for a compiled function.
func NewFrame(fn *CompiledFunction, basePointer int) *Frame {
	return &Frame{
		fn:          fn,
		ip:          0,
		basePointer: basePointer,
	}
}

// Instructions returns the bytecode instruction sequence for the frame.
func (f *Frame) Instructions() Instructions {
	return f.fn.Instructions
}
