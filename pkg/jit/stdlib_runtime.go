package jit

import (
	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/vm"
)

// invokeStdlibMember executes a pure standard-library member for the JIT
// runtime-helper tier. It returns ok=false when the binding is not a callable
// builtin, so the caller can fall back to the bytecode VM without changing the
// stack. errVal is non-nil when the builtin returned an Alca error value; that
// error must be raised by the VM rather than converted into a Go error.
func invokeStdlibMember(env *interpreter.Environment, alias, member string, args []vm.StackSlot) (result interpreter.Value, ok bool, errVal *interpreter.ErrorValue) {
	callee, found := vm.LookupStdlibMember(env, alias, member)
	if !found {
		return nil, false, nil
	}
	return invokeBuiltinCallable(callee, args)
}

// invokeCallable executes a builtin or user-defined compiled function with the
// same stack/error conventions as the VM's OpCall. User functions run on the VM
// that owns stack through callbacks, so recursion, upvalues, handlers, and
// globals stay identical to a bytecode call. Unsupported callables return
// ok=false so the VM can execute the instruction itself.
func invokeCallable(callbacks *vm.JITCallbacks, callee interpreter.Value, stack []vm.StackSlot, sp, argc int) (result interpreter.Value, ok bool, errVal *interpreter.ErrorValue) {
	if _, isBuiltin := callee.(*interpreter.BuiltinFunction); isBuiltin {
		return invokeBuiltinCallable(callee, stack[sp-argc:sp])
	}

	fn, isCompiled := callee.(*vm.CompiledFunction)
	if !isCompiled || callbacks == nil || callbacks.Call == nil {
		return nil, false, nil
	}
	// Let the VM produce its exact arity error rather than inventing one that
	// could differ in message or timing. No side effects have happened yet.
	if argc != fn.NumParameters {
		return nil, false, nil
	}
	res, err := callbacks.Call(fn, sp, argc)
	if err != nil {
		if raised, isRaised := err.(*vm.RaisedError); isRaised {
			return nil, true, raised.Val
		}
		return nil, true, interpreter.NewError(500, err.Error())
	}
	if res == nil {
		res = interpreter.NULL
	}
	return res, true, nil
}

func slotValues(args []vm.StackSlot) []interpreter.Value {
	values := make([]interpreter.Value, len(args))
	for i := range args {
		values[i] = args[i].ToValue()
	}
	return values
}

// invokeBuiltinCallable executes a BuiltinFunction with the same stack/error
// conventions as the VM's OpCall builtin case. Non-builtin callables return
// ok=false so the VM can run compiled functions and callable values itself.
func invokeBuiltinCallable(callee interpreter.Value, args []vm.StackSlot) (result interpreter.Value, ok bool, errVal *interpreter.ErrorValue) {
	builtin, isBuiltin := callee.(*interpreter.BuiltinFunction)
	if !isBuiltin {
		return nil, false, nil
	}
	res := builtin.Fn(slotValues(args)...)
	if ev, isError := res.(*interpreter.ErrorValue); isError {
		return nil, true, ev
	}
	if res == nil {
		res = interpreter.NULL
	}
	return res, true, nil
}
