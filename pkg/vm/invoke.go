// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package vm

import (
	"fmt"

	"github.com/alca-lang/alca/pkg/interpreter"
)

// Call invokes a compiled function from host code such as a GUI callback or an
// HTTP handler. The function shares the globals and environment it was
// compiled with, so it observes and updates the same program state.
func (f *CompiledFunction) Call(args []interpreter.Value) (interpreter.Value, error) {
	return InvokeFunction(f, args, f.Env)
}

// InvokeFunction runs fn to completion on a fresh VM and returns its result.
// It is safe to call while the function's main VM is suspended inside a
// blocking builtin: the callback runs on the same goroutine for native
// controls, or on a helper goroutine for external GUI backends.
func InvokeFunction(fn *CompiledFunction, args []interpreter.Value, env *interpreter.Environment) (interpreter.Value, error) {
	if fn == nil {
		return nil, fmt.Errorf("callback is nil")
	}
	if len(args) < fn.NumParameters && len(fn.Defaults) > 0 {
		minArgs := fn.NumParameters - len(fn.Defaults)
		if len(args) >= minArgs {
			for i := len(args); i < fn.NumParameters; i++ {
				args = append(args, fn.Defaults[i-minArgs])
			}
		}
	}
	if len(args) != fn.NumParameters {
		return nil, fmt.Errorf("wrong number of arguments: want=%d, got=%d", fn.NumParameters, len(args))
	}
	if env == nil {
		env = interpreter.NewEnvironment()
	}

	pool := fn.Pool
	if pool == nil {
		pool = NewConstantPool(nil)
	}
	globals := fn.Globals
	if globals == nil {
		globals = make([]StackSlot, GlobalsSize)
	}

	frames := make([]*Frame, MaxFrames)
	for i := range frames {
		frames[i] = &Frame{}
	}

	machine := &VM{
		pool:         pool,
		globals:      globals,
		stack:        make([]StackSlot, StackSize),
		frames:       frames,
		framesIndex:  1,
		openUpvalues: make(map[int]*Upvalue),
		env:          env,
		lastPopped:   FromValue(interpreter.NULL),
	}

	// The first slot represents the callee, matching executeCall's layout:
	// [fn, arg0, arg1, ..., argN-1, local0, ...].
	frame := frames[0]
	frame.fn = fn
	frame.ip = 0
	frame.basePointer = 1
	machine.stack[0] = FromValue(fn)
	for i, arg := range args {
		machine.stack[1+i] = FromValue(arg)
	}
	machine.sp = 1 + fn.NumLocals
	if machine.sp < 1+len(args) {
		machine.sp = 1 + len(args)
	}
	machine.initJITCallbacks()

	if err := machine.Run(); err != nil {
		return nil, err
	}
	if machine.sp > 0 {
		return machine.stack[machine.sp-1].ToValue(), nil
	}
	return interpreter.NULL, nil
}

// InvokeValue invokes any callable Alca value (CompiledFunction, BuiltinFunction).
func InvokeValue(callee interpreter.Value, args []interpreter.Value, env *interpreter.Environment) (interpreter.Value, error) {
	switch fn := callee.(type) {
	case *CompiledFunction:
		return InvokeFunction(fn, args, env)
	case *interpreter.BuiltinFunction:
		return fn.Fn(args...), nil
	}
	return nil, fmt.Errorf("cannot invoke non-callable %T", callee)
}
