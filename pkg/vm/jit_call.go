package vm

import (
	"fmt"

	"github.com/alca-lang/alca/pkg/interpreter"
)

// initJITCallbacks installs the callbacks that let JIT runtime helpers execute
// user code on this VM. stopFrame is -1 when no helper-driven call is active.
func (vm *VM) initJITCallbacks() {
	vm.jitCallbacks = &JITCallbacks{
		Call:           vm.invokeFromJIT,
		MakeClosure:    vm.makeClosureFromJIT,
		ApplyDecorator: vm.applyDecorator,
	}
	vm.stopFrame = -1
}

// invokeFromJIT executes a compiled function on the VM's existing frame stack.
// The callee and its arguments already occupy stack[..sp-1]; argc is the
// number of arguments. The method returns when that callee returns, leaving its
// return value at the top of the stack. It deliberately does not unwind errors:
// the caller hands the error back to the JIT engine, which delivers it to the
// outer VM loop where the normal unwinder can still see this frame.
func (vm *VM) invokeFromJIT(fn *CompiledFunction, sp, argc int) (interpreter.Value, error) {
	if fn == nil {
		return nil, fmt.Errorf("callback is nil")
	}
	if argc != fn.NumParameters {
		return nil, fmt.Errorf("wrong number of arguments: want=%d, got=%d", fn.NumParameters, argc)
	}
	if vm.framesIndex <= 0 || vm.framesIndex >= MaxFrames {
		return nil, fmt.Errorf("call stack overflow")
	}
	if sp < argc || sp > len(vm.stack) {
		return nil, fmt.Errorf("invalid JIT call stack")
	}
	basePointer := sp - argc
	if basePointer < 0 {
		return nil, fmt.Errorf("invalid JIT call stack")
	}

	// The native/threaded helper's SP is authoritative for this call. The
	// outer run loop may not have synchronized vm.sp with it yet.
	vm.sp = sp

	frame := vm.frames[vm.framesIndex]
	if frame == nil {
		frame = &Frame{}
		vm.frames[vm.framesIndex] = frame
	}
	frame.fn = fn
	frame.ip = 0
	frame.basePointer = basePointer

	stop := vm.framesIndex
	previousStop := vm.stopFrame
	vm.framesIndex++
	vm.sp = basePointer + fn.NumLocals
	if vm.sp < sp {
		vm.sp = sp
	}
	vm.stopFrame = stop

	err := vm.runGuarded()
	vm.stopFrame = previousStop
	if err != nil {
		return nil, err
	}
	if vm.sp > 0 {
		return vm.stack[vm.sp-1].ToValue(), nil
	}
	return interpreter.NULL, nil
}

// makeClosureFromJIT creates a closure for template using the current frame,
// matching OpClosure exactly. It returns nil when the helper cannot safely
// capture from the current frame, which makes the helper bail out to bytecode.
func (vm *VM) makeClosureFromJIT(template *CompiledFunction) *CompiledFunction {
	if template == nil || vm.framesIndex <= 0 {
		return nil
	}
	return vm.newClosure(template, vm.currentFrame())
}

// newClosure instantiates a compiled function template, capturing the upvalues
// described by its descriptors from frame.
func (vm *VM) newClosure(template *CompiledFunction, frame *Frame) *CompiledFunction {
	if template == nil || frame == nil {
		return nil
	}
	closure := &CompiledFunction{
		Instructions:  template.Instructions,
		NumLocals:     template.NumLocals,
		NumParameters: template.NumParameters,
		Name:          template.Name,
		NumFree:       template.NumFree,
		UpvalueDescs:  template.UpvalueDescs,
		Pool:          template.Pool,
		Globals:       template.Globals,
		Env:           vm.env,
		SourceFile:    template.SourceFile,
		// The template is the code the closure was cloned from. The JIT uses it
		// to share one compiled artifact between all instances of the closure.
		Template: template,
	}
	if template.NumFree > 0 {
		closure.Upvalues = make([]*Upvalue, template.NumFree)
		for i, desc := range template.UpvalueDescs {
			if desc.IsLocal {
				closure.Upvalues[i] = vm.captureUpvalue(frame.basePointer + desc.Index)
			} else {
				closure.Upvalues[i] = frame.fn.Upvalues[desc.Index]
			}
		}
	}
	return closure
}

// jitStopReached reports whether returning from the current frame should hand
// control back to the JIT helper that invoked it instead of continuing in the
// caller's bytecode.
func (vm *VM) jitStopReached() bool {
	return vm.stopFrame >= 0 && vm.framesIndex == vm.stopFrame
}

// UpvalueValue reads the current value of fn's captured variable for a JIT
// runtime helper. Open upvalues still point into the live VM stack, which is
// the same slice the helper is operating on.
func UpvalueValue(fn *CompiledFunction, index int, stack []StackSlot) (StackSlot, bool) {
	if fn == nil || index < 0 || index >= len(fn.Upvalues) {
		return StackSlot{}, false
	}
	uv := fn.Upvalues[index]
	if uv == nil {
		return StackSlot{}, false
	}
	if uv.stackIndex >= 0 {
		if uv.stackIndex >= len(stack) {
			return StackSlot{}, false
		}
		return stack[uv.stackIndex], true
	}
	return uv.closed, true
}

// SetUpvalueValue writes slot to fn's captured variable for a JIT runtime
// helper. It updates the caller's live stack slot when the upvalue is open.
func SetUpvalueValue(fn *CompiledFunction, index int, slot StackSlot, stack []StackSlot) bool {
	if fn == nil || index < 0 || index >= len(fn.Upvalues) {
		return false
	}
	uv := fn.Upvalues[index]
	if uv == nil {
		return false
	}
	if uv.stackIndex >= 0 {
		if uv.stackIndex >= len(stack) {
			return false
		}
		stack[uv.stackIndex] = slot
		return true
	}
	uv.closed = slot
	return true
}
