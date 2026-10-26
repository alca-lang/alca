// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package jit_test

import (
	"testing"
	"unsafe"

	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/jit"
	"github.com/alca-lang/alca/pkg/vm"
)

// TestNativeStackLimitBailsOut verifies that the AMD64 backend refuses to
// push a slot past the end of the VM stack. The native code writes stack slots
// directly, so without this guard a frame that runs out of stack space would
// write outside the VM's stack array and corrupt unrelated memory. Instead the
// function bails out to the bytecode VM, which reports the overflow itself.
func TestNativeStackLimitBailsOut(t *testing.T) {
	if !jit.HasNativeAMD64 {
		t.Skip("native AMD64 backend is unavailable")
	}

	instructions := vm.Instructions{}
	for i := 0; i < 16; i++ {
		instructions = append(instructions, vm.Make(vm.OpConstant, 0)...)
	}
	instructions = append(instructions, vm.Make(vm.OpReturnValue)...)

	pool := vm.NewConstantPool([]interpreter.Value{&interpreter.IntegerValue{Val: 7}})
	fn := &vm.CompiledFunction{
		Instructions: instructions,
		NumLocals:    1,
		Pool:         pool,
	}
	engine := jit.NewEngine(jit.Config{Enabled: true, Native: true, Threshold: 1})
	artifact, err := engine.CompileArtifact(fn)
	if err != nil {
		t.Fatalf("compile native function: %v", err)
	}
	if artifact.NativeFn == nil {
		t.Fatal("function did not compile to native code")
	}
	defer artifact.Free()

	// The stack holds four slots and the limit is reported to the native code,
	// so the fifth push must not happen.
	stack := make([]vm.StackSlot, 4)
	ctx := &jit.NativeContext{
		StackPtr:   uintptr(unsafe.Pointer(&stack[0])),
		GlobalsPtr: 0,
		PoolPtr:    uintptr(unsafe.Pointer(&pool.Slots[0])),
		SP:         0,
		StackLimit: 4,
	}
	status := artifact.NativeFn(ctx)
	if status != jit.StatusBailout {
		t.Fatalf("native status = %d, want bailout when the stack limit is reached", status)
	}
	if ctx.SP > 4 {
		t.Fatalf("stack pointer = %d, want at most 4", ctx.SP)
	}
}

// TestNativeStackLimitUnsetAllowsGrowth verifies that a native context without
// a stack limit keeps the previous behaviour: the code runs to completion.
func TestNativeStackLimitUnsetAllowsGrowth(t *testing.T) {
	if !jit.HasNativeAMD64 {
		t.Skip("native AMD64 backend is unavailable")
	}

	instructions := vm.Instructions{}
	for i := 0; i < 4; i++ {
		instructions = append(instructions, vm.Make(vm.OpConstant, 0)...)
	}
	instructions = append(instructions, vm.Make(vm.OpReturnValue)...)

	pool := vm.NewConstantPool([]interpreter.Value{&interpreter.IntegerValue{Val: 7}})
	fn := &vm.CompiledFunction{
		Instructions: instructions,
		NumLocals:    1,
		Pool:         pool,
	}
	engine := jit.NewEngine(jit.Config{Enabled: true, Native: true, Threshold: 1})
	artifact, err := engine.CompileArtifact(fn)
	if err != nil {
		t.Fatalf("compile native function: %v", err)
	}
	if artifact.NativeFn == nil {
		t.Fatal("function did not compile to native code")
	}
	defer artifact.Free()

	stack := make([]vm.StackSlot, 8)
	ctx := &jit.NativeContext{
		StackPtr: uintptr(unsafe.Pointer(&stack[0])),
		PoolPtr:  uintptr(unsafe.Pointer(&pool.Slots[0])),
		SP:       0,
	}
	if status := artifact.NativeFn(ctx); status != jit.StatusSuccess {
		t.Fatalf("native status = %d, want success", status)
	}
}
