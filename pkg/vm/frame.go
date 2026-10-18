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

// UpvalueDesc is the compile-time capture description of one upvalue of a
// compiled function: where the value comes from when the closure is created.
type UpvalueDesc struct {
	// IsLocal is true when the upvalue is a local slot of the function that
	// creates the closure; false when it is upvalue Index of that function.
	IsLocal bool
	// Index is the local slot or the enclosing function's upvalue index.
	Index int
}

// Upvalue is a reference to a captured variable. While the owning frame is
// alive it points at a stack slot; once that slot's frame returns (or is
// unwound) the value is copied into closed and the upvalue is detached.
type Upvalue struct {
	stackIndex int // absolute index into vm.stack, or -1 once closed
	closed     StackSlot
}

// Value returns the current value of the captured variable.
func (u *Upvalue) Value(stack []StackSlot) StackSlot {
	if u.stackIndex >= 0 {
		return stack[u.stackIndex]
	}
	return u.closed
}

// SetValue stores a new value into the captured variable.
func (u *Upvalue) SetValue(stack []StackSlot, val StackSlot) {
	if u.stackIndex >= 0 {
		stack[u.stackIndex] = val
		return
	}
	u.closed = val
}

// close detaches the upvalue from the stack, preserving the current value.
func (u *Upvalue) close(stack []StackSlot) {
	if u.stackIndex >= 0 {
		u.closed = stack[u.stackIndex]
		u.stackIndex = -1
	}
}

// CompiledFunction represents a compiled bytecode function or a closure over one.
// Closures share the immutable code fields; Upvalues is filled per instance by
// OpClosure and empty (NumFree == 0) for plain functions.
type CompiledFunction struct {
	Instructions  Instructions
	NumLocals     int
	NumParameters int
	Name          string
	NumFree       int
	UpvalueDescs  []UpvalueDesc
	Upvalues      []*Upvalue
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
