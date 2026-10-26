// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package vm

import (
	"sync"

	"github.com/alca-lang/alca/pkg/interpreter"
)

// JITCallbacks gives JIT runtime helpers access to the VM that owns the active
// stack. A helper that calls a user-defined function must run it on that same
// VM so recursion depth, open upvalues, handlers, and error unwinding behave
// exactly like a bytecode call. Callbacks are only valid on the goroutine that
// is currently executing the VM.
type JITCallbacks struct {
	// Call executes fn with argc arguments already pushed ending at sp. The
	// callee slot is immediately below the arguments. It returns the function
	// result or the error that terminated it.
	Call func(fn *CompiledFunction, sp int, argc int) (interpreter.Value, error)
	// MakeClosure creates a closure instance for template in the current
	// frame, capturing its declared upvalues.
	MakeClosure func(template *CompiledFunction) *CompiledFunction
	// ApplyDecorator applies a decorator to the running program. It is used by
	// the JIT tiers for the decorators that change how the program behaves,
	// such as @oom_guard and @error_log.
	ApplyDecorator func(spec *DecoratorSpec)
}

// JITEngine represents an optional JIT compilation and execution engine for the VM.
type JITEngine interface {
	ShouldCompile(fn *CompiledFunction) bool
	Compile(fn *CompiledFunction) error
	Execute(fn *CompiledFunction, stack []StackSlot, globals []StackSlot, pool *ConstantPool, basePointer int, sp int, callbacks *JITCallbacks) (newSP int, completed bool, bailoutIP int, pendingErr *interpreter.ErrorValue)
	ExecuteAt(fn *CompiledFunction, stack []StackSlot, globals []StackSlot, pool *ConstantPool, basePointer int, sp int, entryIP int, callbacks *JITCallbacks) (newSP int, completed bool, bailoutIP int, pendingErr *interpreter.ErrorValue)
}

var (
	jitMu     sync.RWMutex
	globalJIT JITEngine
)

// SetJITEngine installs the process-wide JIT engine.
func SetJITEngine(engine JITEngine) {
	jitMu.Lock()
	defer jitMu.Unlock()
	globalJIT = engine
}

// CurrentJITEngine returns the installed JIT engine, or nil if JIT is disabled.
func CurrentJITEngine() JITEngine {
	jitMu.RLock()
	defer jitMu.RUnlock()
	return globalJIT
}
