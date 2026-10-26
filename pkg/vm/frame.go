// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

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

// ConstantPool holds a program's constants in both unboxed (Slots) and boxed
// (Vals) form. Every function compiled as part of one program shares one pool,
// and each function carries a pointer to it so a compiled module loaded from an
// artifact can be called from a different VM than the one that ran its
// top-level code.
type ConstantPool struct {
	Slots []StackSlot
	Vals  []interpreter.Value
}

// NewConstantPool builds a pool from boxed constant values.
func NewConstantPool(vals []interpreter.Value) *ConstantPool {
	slots := make([]StackSlot, len(vals))
	for i, c := range vals {
		slots[i] = FromValue(c)
	}
	return &ConstantPool{Slots: slots, Vals: vals}
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
	// Pool is the constant pool this function's instructions index into.
	Pool *ConstantPool
	// Globals is the module globals array this function reads and writes. A
	// nil value means the executing VM's own globals, which is the case for
	// the program being run directly; functions loaded from an artifact keep
	// the globals array of the module instance they were compiled for.
	Globals []StackSlot
	// Env is the environment the function belongs to. Host code (GUI callbacks,
	// HTTP handlers) uses it when invoking a closure outside its main VM.
	Env *interpreter.Environment
	// Template is the function this one was cloned from. A closure shares its
	// code with the template, so the JIT can reuse one compiled artifact for
	// every instance instead of compiling the same code again.
	Template *CompiledFunction
	// SourceFile is the program file this function was loaded from. The VM and
	// JIT runtime helper use it to resolve `use` statements relative to the
	// importing file.
	SourceFile string
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
