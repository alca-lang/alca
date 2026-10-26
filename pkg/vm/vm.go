// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package vm

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/alca-lang/alca/pkg/ast"
	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/limits"
	"github.com/alca-lang/alca/pkg/version"
)

const (
	StackSize   = 65536
	GlobalsSize = 65536
	MaxFrames   = 2048
)

// limitCheckInterval is the number of instructions the dispatch loop executes
// between two memory-guard checks. Checking on every instruction would cost
// more than the guard saves, and the guard only needs to react before a
// program can grow without bound.
const limitCheckInterval = limits.CheckInterval

// vmFixedMemory is the approximate number of bytes a virtual machine spends on
// its fixed structures: the unboxed stack, the globals array, and the frame
// table. The memory guard charges it when a machine is created.
const vmFixedMemory = int64(StackSize+GlobalsSize)*40 + int64(MaxFrames)*64

// VM executes compiled Alca bytecode instructions on a high-speed unboxed virtual stack machine.
type VM struct {
	pool    *ConstantPool
	globals []StackSlot

	stack []StackSlot
	sp    int // Stack pointer points to next free slot

	frames      []*Frame
	framesIndex int

	// openUpvalues interns the open upvalues of the live stack by absolute stack
	// index, so sibling closures share one reference to the same variable.
	openUpvalues map[int]*Upvalue

	lastPopped StackSlot
	env        *interpreter.Environment

	// sourceFile is the file whose program this VM is executing. `use`
	// statements resolve relative to it before falling back to the working
	// directory and the package registry.
	sourceFile string

	handlers      []handler
	pendingJITErr *interpreter.ErrorValue
	TestFailures  int
	TestPasses    int

	// jitCallbacks lets JIT runtime helpers invoke user functions and create
	// closures on this VM. stopFrame marks the caller frame of the innermost
	// helper-driven call; run stops as soon as that frame is active again.
	jitCallbacks *JITCallbacks
	stopFrame    int
	// errorLog is the file the errors of the program are written to, if the
	// @error_log decorator set one.
	errorLog string
	// loggedError is the error that was written to the error log last, so that
	// an error is never logged twice.
	loggedError *interpreter.ErrorValue
	// dropOnOOM holds the global slots of the variables that were marked with
	// @drop_on_oom. They are released when the memory limit is reached.
	dropOnOOM map[int]bool
	// txStack holds the snapshots of the transactions that are open.
	txStack []txSnapshot
	// stopReturn is set when a JIT helper's OSR completion returns control to
	// the helper's caller frame. run checks it at the top of the dispatch loop
	// and stops instead of resuming the caller's bytecode.
	stopReturn bool
}

// ModuleLoader is an optional hook that handles a `use` statement by running a
// compiled artifact (for example an installed APM package) instead of reading
// source. Returning true means the statement was fully handled, including
// binding its exports into env.
type ModuleLoader func(stmt *ast.UseStatement, fromFile string, env *interpreter.Environment) bool

var (
	moduleLoaderMu sync.RWMutex
	moduleLoader   ModuleLoader
)

// SetModuleLoader installs the process-wide compiled-module loader. The APM
// integration points it at the registry so `use pkg` runs a compatible compiled
// artifact when one exists and falls back to source otherwise.
func SetModuleLoader(l ModuleLoader) {
	moduleLoaderMu.Lock()
	defer moduleLoaderMu.Unlock()
	moduleLoader = l
}

func currentModuleLoader() ModuleLoader {
	moduleLoaderMu.RLock()
	defer moduleLoaderMu.RUnlock()
	return moduleLoader
}

// handler is an installed error handler (try region).
type handler struct {
	frameIdx int // index into frames of the owning frame
	ip       int // instruction to resume at (the catch block)
	sp       int // stack height to restore
}

// RaisedError is the Go error carrying an Alca error value (err(...) or a failing builtin).
type RaisedError struct{ Val *interpreter.ErrorValue }

func (e *RaisedError) Error() string { return e.Val.Message }

// AlcaErrorCode reports the Alca error code carried by the raised error value,
// so diagnostics can print the code the program actually used.
func (e *RaisedError) AlcaErrorCode() int64 { return e.Val.Code }

// New creates a new VM initialized with compiled bytecode.
func New(bytecode *Bytecode) *VM {
	return NewWithEnv(bytecode, interpreter.NewEnvironment())
}

// NewWithEnv creates a new VM with an attached environment for builtins and standard libraries.
func NewWithEnv(bytecode *Bytecode, env *interpreter.Environment) *VM {
	if env != nil {
		if _, ok := env.Get("__file__"); !ok {
			env.Set("__file__", &interpreter.StringValue{Val: "<stdin>"})
		}
		if _, ok := env.Get("__dir__"); !ok {
			env.Set("__dir__", &interpreter.StringValue{Val: "."})
		}
		if _, ok := env.Get("__dirname__"); !ok {
			env.Set("__dirname__", &interpreter.StringValue{Val: "."})
		}
		if _, ok := env.Get("__name__"); !ok {
			env.Set("__name__", &interpreter.StringValue{Val: "__main__"})
		}
		if _, ok := env.Get("__version__"); !ok {
			env.Set("__version__", &interpreter.StringValue{Val: version.Version})
		}
	}
	registerIterBuiltin(env)
	registerAsyncBuiltins(env)
	pool := NewConstantPool(bytecode.Constants)
	mainFn := &CompiledFunction{Instructions: bytecode.Instructions, Pool: pool, Env: env}

	frames := make([]*Frame, MaxFrames)
	for i := 0; i < MaxFrames; i++ {
		frames[i] = &Frame{}
	}
	frames[0].fn = mainFn
	frames[0].ip = 0
	frames[0].basePointer = 0

	vm := &VM{
		pool:         pool,
		globals:      make([]StackSlot, GlobalsSize),
		stack:        make([]StackSlot, StackSize),
		sp:           0,
		frames:       frames,
		framesIndex:  1,
		openUpvalues: make(map[int]*Upvalue),
		env:          env,
		lastPopped:   FromValue(interpreter.NULL),
	}
	// Every function template compiled into this program indexes the same pool
	// and reads this instance's globals.
	attachProgram(mainFn, pool, vm.globals, env)
	vm.initJITCallbacks()
	// The fixed structures of the machine are part of the program's memory
	// footprint, so the memory guard accounts for them.
	limits.Charge(vmFixedMemory)
	return vm
}

// MainFunction returns the root top-level compiled function of the VM.
func (vm *VM) MainFunction() *CompiledFunction {
	if len(vm.frames) > 0 && vm.frames[0] != nil {
		return vm.frames[0].fn
	}
	return nil
}

// attachProgram records the constant pool and globals array that a program's
// function templates execute against. It is called when a program's bytecode is
// loaded, so functions loaded from compiled artifacts keep the module state of
// the instance they were created for.
func attachProgram(main *CompiledFunction, pool *ConstantPool, globals []StackSlot, env *interpreter.Environment) {
	main.Pool = pool
	main.Env = env
	if globals != nil {
		main.Globals = globals
	}
	for _, c := range pool.Vals {
		fn, ok := c.(*CompiledFunction)
		if !ok {
			continue
		}
		fn.Pool = pool
		fn.Env = env
		if globals != nil {
			fn.Globals = globals
		}
	}
}

// framePool returns the constant pool the current frame's instructions index
// into, falling back to the VM's main pool for functions created without one.
func (vm *VM) framePool(frame *Frame) *ConstantPool {
	if frame != nil && frame.fn != nil && frame.fn.Pool != nil {
		return frame.fn.Pool
	}
	return vm.pool
}

// frameGlobals returns the globals array the current frame reads and writes:
// the module instance of a loaded artifact, or the VM's own globals.
func (vm *VM) frameGlobals(frame *Frame) []StackSlot {
	if frame != nil && frame.fn != nil && frame.fn.Globals != nil {
		return frame.fn.Globals
	}
	return vm.globals
}

// StackTop returns the value at the top of the stack without popping it.
func (vm *VM) StackTop() interpreter.Value {
	if vm.sp == 0 {
		return nil
	}
	return vm.stack[vm.sp-1].ToValue()
}

// SetSourceFile records the program file this VM is executing so `use`
// statements resolve imports relative to it (then the working directory, then
// the package registry).
func (vm *VM) SetSourceFile(path string) {
	vm.sourceFile = path
	if main := vm.MainFunction(); main != nil {
		setFunctionSourceFile(main, path)
	}
	if vm.env != nil && path != "" {
		vm.env.Set("__file__", &interpreter.StringValue{Val: path})
		dir := filepath.Dir(path)
		if dir == "" {
			dir = "."
		}
		vm.env.Set("__dir__", &interpreter.StringValue{Val: dir})
		vm.env.Set("__dirname__", &interpreter.StringValue{Val: dir})
	}
}

// setFunctionSourceFile records path on fn and every function template in its
// constant pool. JIT-compiled functions need this to execute `use` statements
// through the runtime-helper tier after leaving the VM dispatch loop.
func setFunctionSourceFile(fn *CompiledFunction, path string) {
	if fn == nil {
		return
	}
	fn.SourceFile = path
	if fn.Pool == nil {
		return
	}
	for _, value := range fn.Pool.Vals {
		if nested, ok := value.(*CompiledFunction); ok {
			nested.SourceFile = path
		}
	}
}

// SourceFile returns the file recorded by SetSourceFile.
func (vm *VM) SourceFile() string { return vm.sourceFile }

// GlobalAt returns the value stored in global slot index, or NULL when the
// index is out of range. Compiled module loaders use it to collect a module's
// exports after running its artifact bytecode.
func (vm *VM) GlobalAt(index int) interpreter.Value {
	if index < 0 || index >= len(vm.globals) {
		return interpreter.NULL
	}
	return vm.globals[index].ToValue()
}

// GetGlobal returns the value of the global at index.
func (vm *VM) GetGlobal(index int) interpreter.Value {
	return vm.GlobalAt(index)
}

// LastPoppedStackElem returns the most recently popped value.
func (vm *VM) LastPoppedStackElem() interpreter.Value {
	return vm.lastPopped.ToValue()
}

// LastPopped returns the most recently popped value (alias for LastPoppedStackElem).
func (vm *VM) LastPopped() interpreter.Value {
	return vm.lastPopped.ToValue()
}

func (vm *VM) currentFrame() *Frame {
	return vm.frames[vm.framesIndex-1]
}

func (vm *VM) pushFrame(f *Frame) {
	vm.frames[vm.framesIndex] = f
	vm.framesIndex++
}

func (vm *VM) popFrame() *Frame {
	vm.framesIndex--
	return vm.frames[vm.framesIndex]
}

// captureUpvalue returns the open upvalue for an absolute stack slot, creating it
// on first capture. Reusing the same Upvalue makes sibling closures share the
// captured variable exactly like the interpreter's shared environment bindings.
func (vm *VM) captureUpvalue(stackIndex int) *Upvalue {
	if uv, ok := vm.openUpvalues[stackIndex]; ok {
		return uv
	}
	uv := &Upvalue{stackIndex: stackIndex}
	vm.openUpvalues[stackIndex] = uv
	return uv
}

// closeUpvalues detaches every open upvalue at or above stack index from the
// stack, preserving its current value. Called when a frame returns or is
// unwound, because the slots it owned are about to be reused.
func (vm *VM) closeUpvalues(from int) {
	if len(vm.openUpvalues) == 0 {
		return
	}
	for idx, uv := range vm.openUpvalues {
		if idx >= from {
			uv.close(vm.stack)
			delete(vm.openUpvalues, idx)
		}
	}
}

func (vm *VM) pushSlot(s StackSlot) error {
	if vm.sp >= StackSize {
		return fmt.Errorf("stack overflow")
	}
	vm.stack[vm.sp] = s
	vm.sp++
	return nil
}

func (vm *VM) push(v interpreter.Value) error {
	return vm.pushSlot(FromValue(v))
}

func (vm *VM) pop() interpreter.Value {
	if vm.sp == 0 {
		return interpreter.NULL
	}
	vm.sp--
	vm.lastPopped = vm.stack[vm.sp]
	return vm.lastPopped.ToValue()
}

// Run executes the bytecode until completion. Errors raised by err(...), failing
// built-ins or runtime faults (division by zero, ...) are delivered to the nearest
// installed handler (see OpTryBegin) and execution resumes there; without a handler
// the error terminates the program.
func (vm *VM) Run() error {
	for {
		err := vm.runGuarded()
		if err == nil {
			return nil
		}
		if raised, ok := err.(*RaisedError); ok {
			vm.logError(raised.Val)
		}
		if !vm.unwind(err) {
			return err
		}
	}
}

// runGuarded converts Go runtime panics (stack exhaustion, bad operands) into errors.
func (vm *VM) runGuarded() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("runtime fault: %v", r)
		}
	}()
	return vm.run()
}

// unwind transfers control to the innermost handler. It reports false if none exists.
func (vm *VM) unwind(err error) bool {
	if len(vm.handlers) == 0 {
		return false
	}
	h := vm.handlers[len(vm.handlers)-1]
	vm.handlers = vm.handlers[:len(vm.handlers)-1]

	// The transactions that are abandoned by the error are rolled back.
	vm.rollbackTransactions(h.frameIdx)

	var ev *interpreter.ErrorValue
	if re, ok := err.(*RaisedError); ok {
		ev = re.Val
	} else {
		ev = interpreter.NewError(500, err.Error())
	}
	// Frames between the raised frame and the handler die here; their slots are
	// about to be reused, so close any upvalues pointing into them.
	vm.closeUpvalues(h.sp)
	vm.framesIndex = h.frameIdx + 1
	vm.frames[h.frameIdx].ip = h.ip
	vm.sp = h.sp
	vm.stack[vm.sp] = FromValue(ev)
	vm.sp++
	return true
}

// run is the interpreter loop proper.
func (vm *VM) run() error {
	frame := vm.currentFrame()
	ins := frame.Instructions()
	ip := frame.ip
	sp := vm.sp
	stack := vm.stack
	globals := vm.frameGlobals(frame)
	pool := vm.framePool(frame)
	bp := frame.basePointer

	if jitEng := CurrentJITEngine(); jitEng != nil && frame.ip == 0 && frame.fn != nil {
		if jitEng.ShouldCompile(frame.fn) {
			newSP, completed, bailoutIP, pendingErr := jitEng.Execute(frame.fn, stack, globals, pool, frame.basePointer, sp, vm.jitCallbacks)
			if pendingErr != nil {
				vm.sp = newSP
				frame.ip = bailoutIP
				return &RaisedError{Val: pendingErr}
			}
			if completed {
				vm.sp = newSP
				if newSP > 0 {
					vm.lastPopped = vm.stack[newSP-1]
				}
				vm.popFrame()
				vm.closeUpvalues(frame.basePointer)
				return nil
			}
			sp = newSP
			ip = bailoutIP
			frame.ip = bailoutIP
		}
	}

	osrCheck := func(target int) bool {
		if jitEng := CurrentJITEngine(); jitEng != nil && frame.fn != nil {
			if jitEng.ShouldCompile(frame.fn) {
				newSP, completed, bailoutIP, pendingErr := jitEng.ExecuteAt(frame.fn, stack, globals, pool, bp, sp, target, vm.jitCallbacks)
				if pendingErr != nil {
					sp = newSP
					ip = bailoutIP
					frame.ip = bailoutIP
					vm.sp = sp
					vm.pendingJITErr = pendingErr
					return true
				}
				if completed {
					sp = newSP
					if vm.framesIndex == 1 {
						vm.sp = sp
						if newSP > 0 {
							vm.lastPopped = vm.stack[newSP-1]
						}
						vm.popFrame()
						vm.closeUpvalues(frame.basePointer)
						return true
					}
					var returnVal StackSlot
					if newSP > bp {
						returnVal = vm.stack[newSP-1]
					} else {
						returnVal = StackSlot{tag: SlotNull}
					}
					frame = vm.popFrame()
					vm.closeUpvalues(frame.basePointer)
					sp = frame.basePointer - 1
					stack[sp] = returnVal
					sp++
					if vm.framesIndex == 0 {
						vm.sp = sp
						vm.lastPopped = returnVal
						return true
					}
					if vm.jitStopReached() {
						vm.stopFrame = -1
						vm.stopReturn = true
						vm.sp = sp
						vm.lastPopped = returnVal
						return true
					}
					frame = vm.currentFrame()
					ins = frame.Instructions()
					ip = frame.ip
					bp = frame.basePointer
					globals = vm.frameGlobals(frame)
					pool = vm.framePool(frame)
					return true
				}
				sp = newSP
				ip = bailoutIP
				frame.ip = bailoutIP
				return true
			}
		}
		return false
	}

	for ip < len(ins) {
		if vm.stopReturn {
			vm.stopReturn = false
			return nil
		}
		if vm.pendingJITErr != nil {
			errVal := vm.pendingJITErr
			vm.pendingJITErr = nil
			return &RaisedError{Val: errVal}
		}
		// The memory guard is checked periodically instead of on every
		// instruction: the check is a mutex-protected counter read, and
		// exceeding a limit raises a regular Alca error that the program can
		// catch.
		if limits.Tick() && limits.Exceeded() {
			// The variables marked with @drop_on_oom are released before the
			// limit is reported.
			vm.dropOOMVariables()
			frame.ip = ip
			vm.sp = sp
			return &RaisedError{Val: interpreter.NewError(507, limits.Message())}
		}
		op := Opcode(ins[ip])
		ip++

		switch op {
		case OpConstant:
			constIndex := binary.BigEndian.Uint16(ins[ip:])
			ip += 2
			stack[sp] = pool.Slots[constIndex]
			sp++

		case OpPop:
			if sp > 0 {
				sp--
				vm.lastPopped = stack[sp]
			}

		case OpDup:
			if sp > 0 {
				stack[sp] = stack[sp-1]
				sp++
			}

		case OpAdd:
			r := &stack[sp-1]
			l := &stack[sp-2]
			if l.tag == SlotInt && r.tag == SlotInt {
				l.i64 += r.i64
				sp--
				continue
			}
			if l.tag == SlotFloat && r.tag == SlotFloat {
				l.f64 += r.f64
				sp--
				continue
			}
			vm.sp = sp
			right := vm.pop()
			left := vm.pop()
			if err := vm.executeBinaryOp(OpAdd, left, right); err != nil {
				frame.ip = ip
				vm.sp = sp
				return err
			}
			sp = vm.sp

		case OpSub:
			r := &stack[sp-1]
			l := &stack[sp-2]
			if l.tag == SlotInt && r.tag == SlotInt {
				l.i64 -= r.i64
				sp--
				continue
			}
			if l.tag == SlotFloat && r.tag == SlotFloat {
				l.f64 -= r.f64
				sp--
				continue
			}
			vm.sp = sp
			right := vm.pop()
			left := vm.pop()
			if err := vm.executeBinaryOp(OpSub, left, right); err != nil {
				frame.ip = ip
				vm.sp = sp
				return err
			}
			sp = vm.sp

		case OpMul:
			r := &stack[sp-1]
			l := &stack[sp-2]
			if l.tag == SlotInt && r.tag == SlotInt {
				l.i64 *= r.i64
				sp--
				continue
			}
			if l.tag == SlotFloat && r.tag == SlotFloat {
				l.f64 *= r.f64
				sp--
				continue
			}
			vm.sp = sp
			right := vm.pop()
			left := vm.pop()
			if err := vm.executeBinaryOp(OpMul, left, right); err != nil {
				frame.ip = ip
				vm.sp = sp
				return err
			}
			sp = vm.sp

		case OpDiv:
			r := &stack[sp-1]
			l := &stack[sp-2]
			if l.tag == SlotInt && r.tag == SlotInt {
				if r.i64 == 0 {
					frame.ip = ip
					vm.sp = sp
					return &RaisedError{Val: interpreter.NewError(400, "division by zero")}
				}
				if l.i64 == -9223372036854775808 && r.i64 == -1 {
					l.i64 = -9223372036854775808
				} else {
					l.i64 /= r.i64
				}
				sp--
				continue
			}
			if l.tag == SlotFloat && r.tag == SlotFloat {
				if r.f64 == 0 {
					frame.ip = ip
					vm.sp = sp
					return &RaisedError{Val: interpreter.NewError(400, "division by zero")}
				}
				l.f64 /= r.f64
				sp--
				continue
			}
			vm.sp = sp
			right := vm.pop()
			left := vm.pop()
			if err := vm.executeBinaryOp(OpDiv, left, right); err != nil {
				frame.ip = ip
				vm.sp = sp
				return err
			}
			sp = vm.sp

		case OpMod:
			r := &stack[sp-1]
			l := &stack[sp-2]
			if l.tag == SlotInt && r.tag == SlotInt {
				if r.i64 == 0 {
					frame.ip = ip
					vm.sp = sp
					return &RaisedError{Val: interpreter.NewError(400, "modulo by zero")}
				}
				if l.i64 == -9223372036854775808 && r.i64 == -1 {
					l.i64 = 0
				} else {
					l.i64 %= r.i64
				}
				sp--
				continue
			}
			vm.sp = sp
			right := vm.pop()
			left := vm.pop()
			if err := vm.executeBinaryOp(OpMod, left, right); err != nil {
				frame.ip = ip
				vm.sp = sp
				return err
			}
			sp = vm.sp

		case OpPow:
			vm.sp = sp
			right := vm.pop()
			left := vm.pop()
			if err := vm.executeBinaryOp(OpPow, left, right); err != nil {
				frame.ip = ip
				vm.sp = sp
				return err
			}
			sp = vm.sp

		case OpLessThan:
			r := &stack[sp-1]
			l := &stack[sp-2]
			if l.tag == SlotInt && r.tag == SlotInt {
				l.tag = SlotBool
				if l.i64 < r.i64 {
					l.i64 = 1
				} else {
					l.i64 = 0
				}
				sp--
				continue
			}
			vm.sp = sp
			right := vm.pop()
			left := vm.pop()
			if err := vm.executeComparison(OpLessThan, left, right); err != nil {
				frame.ip = ip
				vm.sp = sp
				return err
			}
			sp = vm.sp

		case OpLessEqual:
			r := &stack[sp-1]
			l := &stack[sp-2]
			if l.tag == SlotInt && r.tag == SlotInt {
				l.tag = SlotBool
				if l.i64 <= r.i64 {
					l.i64 = 1
				} else {
					l.i64 = 0
				}
				sp--
				continue
			}
			vm.sp = sp
			right := vm.pop()
			left := vm.pop()
			if err := vm.executeComparison(OpLessEqual, left, right); err != nil {
				frame.ip = ip
				vm.sp = sp
				return err
			}
			sp = vm.sp

		case OpGreaterThan:
			r := &stack[sp-1]
			l := &stack[sp-2]
			if l.tag == SlotInt && r.tag == SlotInt {
				l.tag = SlotBool
				if l.i64 > r.i64 {
					l.i64 = 1
				} else {
					l.i64 = 0
				}
				sp--
				continue
			}
			vm.sp = sp
			right := vm.pop()
			left := vm.pop()
			if err := vm.executeComparison(OpGreaterThan, left, right); err != nil {
				frame.ip = ip
				vm.sp = sp
				return err
			}
			sp = vm.sp

		case OpGreaterEqual:
			r := &stack[sp-1]
			l := &stack[sp-2]
			if l.tag == SlotInt && r.tag == SlotInt {
				l.tag = SlotBool
				if l.i64 >= r.i64 {
					l.i64 = 1
				} else {
					l.i64 = 0
				}
				sp--
				continue
			}
			vm.sp = sp
			right := vm.pop()
			left := vm.pop()
			if err := vm.executeComparison(OpGreaterEqual, left, right); err != nil {
				frame.ip = ip
				vm.sp = sp
				return err
			}
			sp = vm.sp

		case OpEqual:
			r := &stack[sp-1]
			l := &stack[sp-2]
			if l.tag == SlotInt && r.tag == SlotInt {
				l.tag = SlotBool
				if l.i64 == r.i64 {
					l.i64 = 1
				} else {
					l.i64 = 0
				}
				sp--
				continue
			}
			vm.sp = sp
			right := vm.pop()
			left := vm.pop()
			if err := vm.executeComparison(OpEqual, left, right); err != nil {
				frame.ip = ip
				vm.sp = sp
				return err
			}
			sp = vm.sp

		case OpNotEqual:
			r := &stack[sp-1]
			l := &stack[sp-2]
			if l.tag == SlotInt && r.tag == SlotInt {
				l.tag = SlotBool
				if l.i64 != r.i64 {
					l.i64 = 1
				} else {
					l.i64 = 0
				}
				sp--
				continue
			}
			vm.sp = sp
			right := vm.pop()
			left := vm.pop()
			if err := vm.executeComparison(OpNotEqual, left, right); err != nil {
				frame.ip = ip
				vm.sp = sp
				return err
			}
			sp = vm.sp

		case OpStrictEqual:
			vm.sp = sp
			right := vm.pop()
			left := vm.pop()
			if err := vm.executeComparison(OpStrictEqual, left, right); err != nil {
				frame.ip = ip
				vm.sp = sp
				return err
			}
			sp = vm.sp

		case OpMathBuiltin:
			id := int8(ins[ip])
			ip++
			arity := mathBuiltinArity(id)
			if arity == 0 {
				frame.ip = ip
				vm.sp = sp
				return fmt.Errorf("unknown std.math intrinsic %d", id)
			}
			if sp < arity {
				frame.ip = ip
				vm.sp = sp
				return fmt.Errorf("stack underflow in std.math intrinsic")
			}
			base := sp - arity
			args := make([]interpreter.Value, arity)
			for i := range args {
				args[i] = stack[base+i].ToValue()
			}
			result, err := applyMathBuiltin(id, args)
			if err != nil {
				frame.ip = ip
				vm.sp = sp
				return err
			}
			stack[base] = FromValue(result)
			sp = base + 1

		case OpIn:
			vm.sp = sp
			right := vm.pop()
			left := vm.pop()
			res := interpreter.InOp(left, right)
			if ev, ok := res.(*interpreter.ErrorValue); ok {
				frame.ip = ip
				vm.sp = sp
				return &RaisedError{Val: ev}
			}
			if err := vm.push(res); err != nil {
				return err
			}
			sp = vm.sp

		case OpTrue:
			stack[sp] = StackSlot{tag: SlotBool, i64: 1}
			sp++

		case OpFalse:
			stack[sp] = StackSlot{tag: SlotBool, i64: 0}
			sp++

		case OpNull:
			stack[sp] = StackSlot{tag: SlotNull}
			sp++

		case OpIsInt:
			slot := &stack[sp-1]
			if slot.tag == SlotInt {
				slot.i64 = 1
			} else {
				slot.i64 = 0
			}
			slot.obj = nil
			slot.tag = SlotBool

		case OpNot:
			slot := &stack[sp-1]
			if slot.tag == SlotBool {
				if slot.i64 != 0 {
					slot.i64 = 0
				} else {
					slot.i64 = 1
				}
			} else {
				if interpreter.IsTruthy(slot.ToValue()) {
					*slot = StackSlot{tag: SlotBool, i64: 0}
				} else {
					*slot = StackSlot{tag: SlotBool, i64: 1}
				}
			}

		case OpNegate:
			slot := &stack[sp-1]
			if slot.tag == SlotInt {
				slot.i64 = -slot.i64
			} else if slot.tag == SlotFloat {
				slot.f64 = -slot.f64
			}

		case OpJump:
			pos := int(binary.BigEndian.Uint16(ins[ip:]))
			if pos < ip {
				if osrCheck(pos) {
					if vm.framesIndex == 0 {
						return nil
					}
					continue
				}
			}
			ip = pos

		case OpJumpIfFalse:
			pos := int(binary.BigEndian.Uint16(ins[ip:]))
			ip += 2
			sp--
			slot := &stack[sp]
			var isFalse bool
			if slot.tag == SlotBool {
				isFalse = (slot.i64 == 0)
			} else if slot.tag == SlotInt {
				isFalse = (slot.i64 == 0)
			} else if slot.tag == SlotNull {
				isFalse = true
			} else {
				isFalse = !interpreter.IsTruthy(slot.ToValue())
			}
			if isFalse {
				ip = pos
			}

		case OpSetGlobal:
			globalIndex := binary.BigEndian.Uint16(ins[ip:])
			ip += 2
			sp--
			globals[globalIndex] = stack[sp]

		case OpGetGlobal:
			globalIndex := binary.BigEndian.Uint16(ins[ip:])
			ip += 2
			stack[sp] = globals[globalIndex]
			sp++

		case OpGetBuiltin:
			constIndex := binary.BigEndian.Uint16(ins[ip:])
			ip += 2
			nameVal := pool.Vals[constIndex].(*interpreter.StringValue).Val
			stack[sp] = FromValue(ResolveBuiltinValue(vm.env, nameVal))
			sp++

		case OpClosure:
			constIndex := binary.BigEndian.Uint16(ins[ip:])
			ip += 2
			template := pool.Vals[constIndex].(*CompiledFunction)
			closure := vm.newClosure(template, frame)
			stack[sp] = FromValue(closure)
			sp++

		case OpGetFree:
			freeIndex := int(ins[ip])
			ip++
			uv := frame.fn.Upvalues[freeIndex]
			if uv.stackIndex >= 0 {
				stack[sp] = stack[uv.stackIndex]
			} else {
				stack[sp] = uv.closed
			}
			sp++

		case OpSetFree:
			freeIndex := int(ins[ip])
			ip++
			sp--
			uv := frame.fn.Upvalues[freeIndex]
			if uv.stackIndex >= 0 {
				stack[uv.stackIndex] = stack[sp]
			} else {
				uv.closed = stack[sp]
			}

		case OpSetLocal:
			localIndex := int(ins[ip])
			ip++
			sp--
			stack[frame.basePointer+localIndex] = stack[sp]

		case OpGetLocal:
			localIndex := int(ins[ip])
			ip++
			stack[sp] = stack[frame.basePointer+localIndex]
			sp++

		case OpIncGlobal:
			globalIndex := binary.BigEndian.Uint16(ins[ip:])
			ip += 2
			if globals[globalIndex].tag == SlotInt {
				globals[globalIndex].i64++
				continue
			}
			vm.sp = sp
			res, err := vm.slowArith(OpAdd, globals[globalIndex], StackSlot{tag: SlotInt, i64: 1})
			if err != nil {
				frame.ip = ip
				return err
			}
			sp = vm.sp
			globals[globalIndex] = res
		case OpIncLocal:
			localIndex := int(ins[ip])
			ip++
			if stack[frame.basePointer+localIndex].tag == SlotInt {
				stack[frame.basePointer+localIndex].i64++
				continue
			}
			vm.sp = sp
			res, err := vm.slowArith(OpAdd, stack[frame.basePointer+localIndex], StackSlot{tag: SlotInt, i64: 1})
			if err != nil {
				frame.ip = ip
				return err
			}
			sp = vm.sp
			stack[frame.basePointer+localIndex] = res
		case OpAddGlobal:
			globalIndex := binary.BigEndian.Uint16(ins[ip:])
			ip += 2
			sp--
			if globals[globalIndex].tag == SlotInt && stack[sp].tag == SlotInt {
				globals[globalIndex].i64 += stack[sp].i64
				continue
			}
			if globals[globalIndex].tag == SlotFloat && stack[sp].tag == SlotFloat {
				globals[globalIndex].f64 += stack[sp].f64
				continue
			}
			vm.sp = sp
			res, err := vm.slowArith(OpAdd, globals[globalIndex], stack[sp])
			if err != nil {
				frame.ip = ip
				return err
			}
			sp = vm.sp
			globals[globalIndex] = res
		case OpAddLocal:
			localIndex := int(ins[ip])
			ip++
			sp--
			if stack[frame.basePointer+localIndex].tag == SlotInt && stack[sp].tag == SlotInt {
				stack[frame.basePointer+localIndex].i64 += stack[sp].i64
				continue
			}
			if stack[frame.basePointer+localIndex].tag == SlotFloat && stack[sp].tag == SlotFloat {
				stack[frame.basePointer+localIndex].f64 += stack[sp].f64
				continue
			}
			vm.sp = sp
			res, err := vm.slowArith(OpAdd, stack[frame.basePointer+localIndex], stack[sp])
			if err != nil {
				frame.ip = ip
				return err
			}
			sp = vm.sp
			stack[frame.basePointer+localIndex] = res
		case OpSubGlobal:
			globalIndex := binary.BigEndian.Uint16(ins[ip:])
			ip += 2
			sp--
			if globals[globalIndex].tag == SlotInt && stack[sp].tag == SlotInt {
				globals[globalIndex].i64 -= stack[sp].i64
				continue
			}
			if globals[globalIndex].tag == SlotFloat && stack[sp].tag == SlotFloat {
				globals[globalIndex].f64 -= stack[sp].f64
				continue
			}
			vm.sp = sp
			res, err := vm.slowArith(OpSub, globals[globalIndex], stack[sp])
			if err != nil {
				frame.ip = ip
				return err
			}
			sp = vm.sp
			globals[globalIndex] = res
		case OpSubLocal:
			localIndex := int(ins[ip])
			ip++
			sp--
			if stack[frame.basePointer+localIndex].tag == SlotInt && stack[sp].tag == SlotInt {
				stack[frame.basePointer+localIndex].i64 -= stack[sp].i64
				continue
			}
			if stack[frame.basePointer+localIndex].tag == SlotFloat && stack[sp].tag == SlotFloat {
				stack[frame.basePointer+localIndex].f64 -= stack[sp].f64
				continue
			}
			vm.sp = sp
			res, err := vm.slowArith(OpSub, stack[frame.basePointer+localIndex], stack[sp])
			if err != nil {
				frame.ip = ip
				return err
			}
			sp = vm.sp
			stack[frame.basePointer+localIndex] = res
		case OpLoopGlobalLtInc:
			varIdx := binary.BigEndian.Uint16(ins[ip:])
			constIdx := binary.BigEndian.Uint16(ins[ip+2:])
			target := int(binary.BigEndian.Uint16(ins[ip+4:]))
			ip += 6
			if globals[varIdx].tag != SlotInt || pool.Slots[constIdx].tag != SlotInt {
				frame.ip = ip
				vm.sp = sp
				return fmt.Errorf("range loop requires integer bounds")
			}
			globals[varIdx].i64++
			if globals[varIdx].i64 < pool.Slots[constIdx].i64 {
				if osrCheck(target) {
					if vm.framesIndex == 0 {
						return nil
					}
					continue
				}
				ip = target
			}
		case OpLoopGlobalLeInc:
			varIdx := binary.BigEndian.Uint16(ins[ip:])
			endIdx := binary.BigEndian.Uint16(ins[ip+2:])
			target := int(binary.BigEndian.Uint16(ins[ip+4:]))
			ip += 6
			if globals[varIdx].tag != SlotInt || globals[endIdx].tag != SlotInt {
				frame.ip = ip
				vm.sp = sp
				return fmt.Errorf("range loop requires integer bounds")
			}
			globals[varIdx].i64++
			if globals[varIdx].i64 <= globals[endIdx].i64 {
				if osrCheck(target) {
					if vm.framesIndex == 0 {
						return nil
					}
					continue
				}
				ip = target
			}
		case OpLoopLocalLtInc:
			varIdx := int(ins[ip])
			constIdx := binary.BigEndian.Uint16(ins[ip+1:])
			target := int(binary.BigEndian.Uint16(ins[ip+3:]))
			ip += 5
			slot := &stack[bp+varIdx]
			if slot.tag != SlotInt || pool.Slots[constIdx].tag != SlotInt {
				frame.ip = ip
				vm.sp = sp
				return fmt.Errorf("range loop requires integer bounds")
			}
			slot.i64++
			if slot.i64 < pool.Slots[constIdx].i64 {
				if osrCheck(target) {
					if vm.framesIndex == 0 {
						return nil
					}
					continue
				}
				ip = target
			}
		case OpLoopLocalLeInc:
			varIdx := int(ins[ip])
			endIdx := int(ins[ip+1])
			target := int(binary.BigEndian.Uint16(ins[ip+2:]))
			ip += 4
			v := &stack[bp+varIdx]
			e := &stack[bp+endIdx]
			if v.tag != SlotInt || e.tag != SlotInt {
				frame.ip = ip
				vm.sp = sp
				return fmt.Errorf("range loop requires integer bounds")
			}
			v.i64++
			if v.i64 <= e.i64 {
				if osrCheck(target) {
					if vm.framesIndex == 0 {
						return nil
					}
					continue
				}
				ip = target
			}
		case OpLoopRangeLocal:
			varIdx := int(ins[ip])
			endIdx := int(ins[ip+1])
			stepIdx := int(ins[ip+2])
			target := int(binary.BigEndian.Uint16(ins[ip+3:]))
			ip += 5
			v := &stack[bp+varIdx]
			e := &stack[bp+endIdx]
			st := &stack[bp+stepIdx]
			if v.tag != SlotInt || e.tag != SlotInt || st.tag != SlotInt {
				frame.ip = ip
				vm.sp = sp
				return fmt.Errorf("range loop requires integer bounds")
			}
			v.i64 += st.i64
			if st.i64 > 0 {
				if v.i64 <= e.i64 {
					if osrCheck(target) {
						if vm.framesIndex == 0 {
							return nil
						}
						continue
					}
					ip = target
				}
			} else if v.i64 >= e.i64 {
				if osrCheck(target) {
					if vm.framesIndex == 0 {
						return nil
					}
					continue
				}
				ip = target
			}
		case OpLoopRangeGlobal:
			varIdx := int(binary.BigEndian.Uint16(ins[ip:]))
			endIdx := int(binary.BigEndian.Uint16(ins[ip+2:]))
			stepIdx := int(binary.BigEndian.Uint16(ins[ip+4:]))
			target := int(binary.BigEndian.Uint16(ins[ip+6:]))
			ip += 8
			v := &globals[varIdx]
			e := &globals[endIdx]
			st := &globals[stepIdx]
			if v.tag != SlotInt || e.tag != SlotInt || st.tag != SlotInt {
				frame.ip = ip
				vm.sp = sp
				return fmt.Errorf("range loop requires integer bounds")
			}
			v.i64 += st.i64
			if st.i64 > 0 {
				if v.i64 <= e.i64 {
					if osrCheck(target) {
						if vm.framesIndex == 0 {
							return nil
						}
						continue
					}
					ip = target
				}
			} else if v.i64 >= e.i64 {
				if osrCheck(target) {
					if vm.framesIndex == 0 {
						return nil
					}
					continue
				}
				ip = target
			}
		case OpLoopListLocal:
			idxIdx := int(ins[ip])
			lenIdx := int(ins[ip+1])
			iterIdx := int(ins[ip+2])
			varIdx := int(ins[ip+3])
			target := int(binary.BigEndian.Uint16(ins[ip+4:]))
			ip += 6
			bp := frame.basePointer
			idx := &stack[bp+idxIdx]
			ln := &stack[bp+lenIdx]
			it := &stack[bp+iterIdx]
			v := &stack[bp+varIdx]
			if idx.tag != SlotInt || ln.tag != SlotInt {
				frame.ip = ip
				vm.sp = sp
				return fmt.Errorf("list loop requires integer index")
			}
			idx.i64++
			if idx.i64 < ln.i64 {
				if it.tag == SlotVal {
					if lst, ok := it.obj.(*interpreter.ListValue); ok {
						i := idx.i64
						if lst.Ints != nil {
							n := int64(len(lst.Ints))
							if i >= 0 && i < n {
								*v = StackSlot{tag: SlotInt, i64: lst.Ints[i]}
								ip = target
							}
						} else if lst.Runes != nil {
							n := int64(len(lst.Runes))
							if i >= 0 && i < n {
								*v = FromValue(interpreter.SingleRuneString(lst.Runes[i]))
								ip = target
							}
						} else {
							n := int64(len(lst.Elements))
							if i >= 0 && i < n {
								*v = FromValue(lst.Elements[i])
								ip = target
							}
						}
					}
				}
			}
		case OpLoopListGlobal:
			idxIdx := int(binary.BigEndian.Uint16(ins[ip:]))
			lenIdx := int(binary.BigEndian.Uint16(ins[ip+2:]))
			iterIdx := int(binary.BigEndian.Uint16(ins[ip+4:]))
			varIdx := int(binary.BigEndian.Uint16(ins[ip+6:]))
			target := int(binary.BigEndian.Uint16(ins[ip+8:]))
			ip += 10
			idx := &globals[idxIdx]
			ln := &globals[lenIdx]
			it := &globals[iterIdx]
			v := &globals[varIdx]
			if idx.tag != SlotInt || ln.tag != SlotInt {
				frame.ip = ip
				vm.sp = sp
				return fmt.Errorf("list loop requires integer index")
			}
			idx.i64++
			if idx.i64 < ln.i64 {
				if it.tag == SlotVal {
					if lst, ok := it.obj.(*interpreter.ListValue); ok {
						i := idx.i64
						if lst.Ints != nil {
							n := int64(len(lst.Ints))
							if i >= 0 && i < n {
								*v = StackSlot{tag: SlotInt, i64: lst.Ints[i]}
								ip = target
							}
						} else if lst.Runes != nil {
							n := int64(len(lst.Runes))
							if i >= 0 && i < n {
								*v = FromValue(interpreter.SingleRuneString(lst.Runes[i]))
								ip = target
							}
						} else {
							n := int64(len(lst.Elements))
							if i >= 0 && i < n {
								*v = FromValue(lst.Elements[i])
								ip = target
							}
						}
					}
				}
			}
		case OpAddLocalLocal:
			dstIdx := int(ins[ip])
			srcIdx := int(ins[ip+1])
			ip += 2
			bp := frame.basePointer
			d := &stack[bp+dstIdx]
			s := &stack[bp+srcIdx]
			if d.tag == SlotInt && s.tag == SlotInt {
				d.i64 += s.i64
				continue
			}
			if d.tag == SlotFloat && s.tag == SlotFloat {
				d.f64 += s.f64
				continue
			}
			vm.sp = sp
			res, err := vm.slowArith(OpAdd, *d, *s)
			if err != nil {
				frame.ip = ip
				return err
			}
			sp = vm.sp
			stack[bp+dstIdx] = res
		case OpAddGlobalGlobal:
			dstIdx := int(binary.BigEndian.Uint16(ins[ip:]))
			srcIdx := int(binary.BigEndian.Uint16(ins[ip+2:]))
			ip += 4
			d := &globals[dstIdx]
			s := &globals[srcIdx]
			if d.tag == SlotInt && s.tag == SlotInt {
				d.i64 += s.i64
				continue
			}
			if d.tag == SlotFloat && s.tag == SlotFloat {
				d.f64 += s.f64
				continue
			}
			vm.sp = sp
			res, err := vm.slowArith(OpAdd, *d, *s)
			if err != nil {
				frame.ip = ip
				return err
			}
			sp = vm.sp
			globals[dstIdx] = res
		case OpAddLocalConst:
			srcIdx := int(ins[ip])
			constIdx := int(binary.BigEndian.Uint16(ins[ip+1:]))
			ip += 3
			l := stack[frame.basePointer+srcIdx]
			k := pool.Slots[constIdx]
			if l.tag == SlotInt && k.tag == SlotInt {
				stack[sp] = StackSlot{tag: SlotInt, i64: l.i64 + k.i64}
				sp++
				continue
			}
			if l.tag == SlotFloat && k.tag == SlotFloat {
				stack[sp] = StackSlot{tag: SlotFloat, f64: l.f64 + k.f64}
				sp++
				continue
			}
			vm.sp = sp
			res, err := vm.slowArith(OpAdd, l, k)
			if err != nil {
				frame.ip = ip
				return err
			}
			sp = vm.sp
			stack[sp] = res
			sp++
		case OpMulLocalConst:
			srcIdx := int(ins[ip])
			constIdx := int(binary.BigEndian.Uint16(ins[ip+1:]))
			ip += 3
			l := stack[frame.basePointer+srcIdx]
			k := pool.Slots[constIdx]
			if l.tag == SlotInt && k.tag == SlotInt {
				stack[sp] = StackSlot{tag: SlotInt, i64: l.i64 * k.i64}
				sp++
				continue
			}
			if l.tag == SlotFloat && k.tag == SlotFloat {
				stack[sp] = StackSlot{tag: SlotFloat, f64: l.f64 * k.f64}
				sp++
				continue
			}
			vm.sp = sp
			res, err := vm.slowArith(OpMul, l, k)
			if err != nil {
				frame.ip = ip
				return err
			}
			sp = vm.sp
			stack[sp] = res
			sp++
		case OpModLocalConst:
			srcIdx := int(ins[ip])
			constIdx := int(binary.BigEndian.Uint16(ins[ip+1:]))
			ip += 3
			l := stack[frame.basePointer+srcIdx]
			k := pool.Slots[constIdx]
			if l.tag == SlotInt && k.tag == SlotInt && k.i64 != 0 {
				stack[sp] = StackSlot{tag: SlotInt, i64: l.i64 % k.i64}
				sp++
				continue
			}
			vm.sp = sp
			res, err := vm.slowArith(OpMod, l, k)
			if err != nil {
				frame.ip = ip
				return err
			}
			sp = vm.sp
			stack[sp] = res
			sp++
		case OpConvLocal:
			convID := int8(ins[ip])
			srcIdx := int(ins[ip+1])
			ip += 2
			slot := stack[frame.basePointer+srcIdx]
			if !ApplyBuiltin1(convID, &slot) {
				frame.ip = ip
				vm.sp = sp
				res, err := CallBuiltin1Fallback(convID, slot)
				if err != nil {
					return err
				}
				slot = res
			}
			stack[sp] = slot
			sp++
		case OpConvGlobal:
			convID := int8(ins[ip])
			srcIdx := int(binary.BigEndian.Uint16(ins[ip+1:]))
			ip += 3
			slot := globals[srcIdx]
			if !ApplyBuiltin1(convID, &slot) {
				frame.ip = ip
				vm.sp = sp
				res, err := CallBuiltin1Fallback(convID, slot)
				if err != nil {
					return err
				}
				slot = res
			}
			stack[sp] = slot
			sp++
		case OpModConst:
			constIdx := binary.BigEndian.Uint16(ins[ip:])
			ip += 2
			if stack[sp-1].tag == SlotInt && pool.Slots[constIdx].tag == SlotInt && pool.Slots[constIdx].i64 != 0 {
				stack[sp-1].i64 %= pool.Slots[constIdx].i64
				continue
			}
			vm.sp = sp
			res, err := vm.slowArith(OpMod, stack[sp-1], pool.Slots[constIdx])
			if err != nil {
				frame.ip = ip
				return err
			}
			sp = vm.sp
			stack[sp-1] = res
		case OpMulConst:
			constIdx := binary.BigEndian.Uint16(ins[ip:])
			ip += 2
			if stack[sp-1].tag == SlotInt && pool.Slots[constIdx].tag == SlotInt {
				stack[sp-1].i64 *= pool.Slots[constIdx].i64
				continue
			}
			if stack[sp-1].tag == SlotFloat && pool.Slots[constIdx].tag == SlotFloat {
				stack[sp-1].f64 *= pool.Slots[constIdx].f64
				continue
			}
			vm.sp = sp
			res, err := vm.slowArith(OpMul, stack[sp-1], pool.Slots[constIdx])
			if err != nil {
				frame.ip = ip
				return err
			}
			sp = vm.sp
			stack[sp-1] = res
		case OpAddConst:
			constIdx := binary.BigEndian.Uint16(ins[ip:])
			ip += 2
			if stack[sp-1].tag == SlotInt && pool.Slots[constIdx].tag == SlotInt {
				stack[sp-1].i64 += pool.Slots[constIdx].i64
				continue
			}
			if stack[sp-1].tag == SlotFloat && pool.Slots[constIdx].tag == SlotFloat {
				stack[sp-1].f64 += pool.Slots[constIdx].f64
				continue
			}
			vm.sp = sp
			res, err := vm.slowArith(OpAdd, stack[sp-1], pool.Slots[constIdx])
			if err != nil {
				frame.ip = ip
				return err
			}
			sp = vm.sp
			stack[sp-1] = res
		case OpIter:
			// Convert the top of stack into the form the generic for-loop
			// iterates over, without an environment lookup and boxing.
			iterVal, iterErr := IterValue(stack[sp-1].ToValue())
			if iterErr != nil {
				frame.ip = ip
				vm.sp = sp
				return iterErr
			}
			stack[sp-1] = FromValue(iterVal)

		case OpListLen:
			// Length of the list on top of stack, without a builtin call.
			lenSlot, lenErr := ListLenValue(stack[sp-1])
			if lenErr != nil {
				frame.ip = ip
				vm.sp = sp
				return lenErr
			}
			stack[sp-1] = lenSlot

		case OpList:
			numElements := int(binary.BigEndian.Uint16(ins[ip:]))
			ip += 2
			vm.sp = sp
			list := vm.buildList(sp-numElements, sp)
			sp -= numElements
			stack[sp] = FromValue(list)
			sp++

		case OpMap:
			numPairs := int(binary.BigEndian.Uint16(ins[ip:]))
			ip += 2
			vm.sp = sp
			m := vm.buildMap(sp-(numPairs*2), sp)
			sp -= (numPairs * 2)
			stack[sp] = FromValue(m)
			sp++

		case OpRange:
			end := stack[sp-1].ToValue()
			start := stack[sp-2].ToValue()
			sp -= 2
			stack[sp] = FromValue(interpreter.EvalRange(start, end))
			sp++

		case OpRangeInclusive:
			end := stack[sp-1].ToValue()
			start := stack[sp-2].ToValue()
			sp -= 2
			stack[sp] = FromValue(interpreter.EvalRangeInclusive(start, end))
			sp++

		case OpIndex:
			// Fast path: list[non-negative int] avoids boxing the index and the
			// type switch in executeIndex for the common indexing case.
			if stack[sp-1].tag == SlotInt && stack[sp-2].tag == SlotVal {
				if lst, ok := stack[sp-2].obj.(*interpreter.ListValue); ok {
					i := stack[sp-1].i64
					if lst.Ints != nil {
						n := int64(len(lst.Ints))
						if i < 0 {
							i += n
						}
						if i >= 0 && i < n {
							sp -= 2
							stack[sp] = StackSlot{tag: SlotInt, i64: lst.Ints[i]}
							sp++
							continue
						}
					} else if lst.Runes != nil {
						n := int64(len(lst.Runes))
						if i < 0 {
							i += n
						}
						if i >= 0 && i < n {
							sp -= 2
							stack[sp] = FromValue(interpreter.SingleRuneString(lst.Runes[i]))
							sp++
							continue
						}
					} else {
						n := int64(len(lst.Elements))
						if i < 0 {
							i += n
						}
						if i >= 0 && i < n {
							sp -= 2
							stack[sp] = FromValue(lst.Elements[i])
							sp++
							continue
						}
					}
				}
			}
			idx := stack[sp-1].ToValue()
			target := stack[sp-2].ToValue()
			sp -= 2
			res, err := vm.executeIndex(target, idx)
			if err != nil {
				frame.ip = ip
				vm.sp = sp
				return err
			}
			stack[sp] = FromValue(res)
			sp++

		case OpSetIndex:
			idx := stack[sp-1].ToValue()
			target := stack[sp-2].ToValue()
			val := stack[sp-3].ToValue()
			sp -= 3
			if err := vm.executeSetIndex(target, idx, val); err != nil {
				frame.ip = ip
				vm.sp = sp
				return err
			}

		case OpGetField:
			nameIndex := binary.BigEndian.Uint16(ins[ip:])
			ip += 2
			fieldName := pool.Vals[nameIndex].(*interpreter.StringValue).Val
			sp--
			target := stack[sp].ToValue()
			res, err := vm.executeGetField(target, fieldName)
			if err != nil {
				frame.ip = ip
				vm.sp = sp
				return err
			}
			stack[sp] = FromValue(res)
			sp++

		case OpSetField:
			nameIndex := binary.BigEndian.Uint16(ins[ip:])
			ip += 2
			fieldName := pool.Vals[nameIndex].(*interpreter.StringValue).Val
			target := stack[sp-1].ToValue()
			val := stack[sp-2].ToValue()
			sp -= 2
			if m, ok := target.(*interpreter.MapValue); ok {
				m.Set(fieldName, val)
			} else if si, ok := target.(*interpreter.StructInstance); ok {
				si.Fields[fieldName] = val
			}

		case OpUse:
			constIndex := binary.BigEndian.Uint16(ins[ip:])
			ip += 2
			useVal := pool.Vals[constIndex].(*UseStatementValue)
			ExecuteUseStatement(vm.env, useVal, vm.sourceFile)

		case OpCall:
			numArgs := int(ins[ip])
			ip++
			fnIndex := sp - 1 - numArgs
			fn := stack[fnIndex].ToValue()
			frame.ip = ip
			vm.sp = sp
			if err := vm.executeCall(fn, numArgs); err != nil {
				return err
			}
			frame = vm.currentFrame()
			ins = frame.Instructions()
			ip = frame.ip
			sp = vm.sp
			stack = vm.stack
			bp = frame.basePointer
			globals = vm.frameGlobals(frame)
			pool = vm.framePool(frame)

		case OpStdlibCall:
			aliasIdx := binary.BigEndian.Uint16(ins[ip:])
			memberIdx := binary.BigEndian.Uint16(ins[ip+2:])
			numArgs := int(ins[ip+4])
			ip += 5
			alias := pool.Vals[aliasIdx].(*interpreter.StringValue).Val
			member := pool.Vals[memberIdx].(*interpreter.StringValue).Val
			frame.ip = ip
			vm.sp = sp
			if err := vm.executeStdlibCall(alias, member, numArgs); err != nil {
				return err
			}
			frame = vm.currentFrame()
			ins = frame.Instructions()
			ip = frame.ip
			sp = vm.sp
			stack = vm.stack
			bp = frame.basePointer
			globals = vm.frameGlobals(frame)
			pool = vm.framePool(frame)

		case OpCallBuiltin1:
			id := int8(ins[ip])
			ip++
			// The argument is the top of stack; the result replaces it in
			// place, so unlike OpCall there is no callee slot to consume.
			slot := &stack[sp-1]
			if !ApplyBuiltin1(id, slot) {
				frame.ip = ip
				vm.sp = sp
				res, err := CallBuiltin1Fallback(id, *slot)
				if err != nil {
					return err
				}
				stack[sp-1] = res
			}

		case OpTxBegin:
			// A transaction snapshots the local slots of the frame, so that the
			// changes of the block are discarded when it fails.
			vm.beginTransaction(frame, bp)
		case OpTxEnd:
			vm.commitTransaction()

		case OpApplyDecorator:
			specIdx := binary.BigEndian.Uint16(ins[ip:])
			ip += 2
			spec, ok := pool.Vals[specIdx].(*DecoratorSpec)
			if !ok {
				frame.ip = ip
				vm.sp = sp
				return fmt.Errorf("invalid decorator")
			}
			vm.applyDecorator(spec)

		case OpMakeClass:
			templateIdx := binary.BigEndian.Uint16(ins[ip:])
			parentIdx := binary.BigEndian.Uint16(ins[ip+2:])
			ip += 4
			template, ok := pool.Vals[templateIdx].(*ClassTemplate)
			if !ok {
				frame.ip = ip
				vm.sp = sp
				return fmt.Errorf("invalid class template")
			}
			var parent *interpreter.StructDef
			if parentIdx > 0 {
				parentValue, isDef := globals[parentIdx-1].ToValue().(*interpreter.StructDef)
				if !isDef || !parentValue.IsClass {
					frame.ip = ip
					vm.sp = sp
					return fmt.Errorf("base class of %q is not a class", template.Name)
				}
				parent = parentValue
			}
			// The method bodies index the constant pool of the program they
			// belong to.
			for _, method := range template.Methods {
				if method != nil && method.Fn != nil {
					method.Fn.Pool = pool
				}
			}
			stack[sp] = FromValue(buildClassDef(template, parent))
			sp++

		case OpReturnValue:
			sp--
			returnVal := stack[sp]
			frame = vm.popFrame()
			vm.closeUpvalues(frame.basePointer)
			sp = frame.basePointer - 1
			stack[sp] = returnVal
			sp++
			if vm.framesIndex == 0 {
				vm.sp = sp
				vm.lastPopped = returnVal
				return nil
			}
			if vm.jitStopReached() {
				vm.stopFrame = -1
				vm.sp = sp
				vm.lastPopped = returnVal
				return nil
			}
			frame = vm.currentFrame()
			ins = frame.Instructions()
			ip = frame.ip
			bp = frame.basePointer
			globals = vm.frameGlobals(frame)
			pool = vm.framePool(frame)

		case OpReturn:
			frame = vm.popFrame()
			vm.closeUpvalues(frame.basePointer)
			sp = frame.basePointer - 1
			stack[sp] = StackSlot{tag: SlotNull}
			sp++
			if vm.framesIndex == 0 {
				vm.sp = sp
				vm.lastPopped = StackSlot{tag: SlotNull}
				return nil
			}
			if vm.jitStopReached() {
				vm.stopFrame = -1
				vm.sp = sp
				vm.lastPopped = StackSlot{tag: SlotNull}
				return nil
			}
			frame = vm.currentFrame()
			ins = frame.Instructions()
			ip = frame.ip
			bp = frame.basePointer
			globals = vm.frameGlobals(frame)
			pool = vm.framePool(frame)

		case OpNop:

		case OpWait:
			if err := interpreter.WaitAll(); err != nil {
				return fmt.Errorf("asynchronous operation failed: %w", err)
			}

		case OpTryBegin:
			target := int(binary.BigEndian.Uint16(ins[ip:]))
			ip += 2
			vm.handlers = append(vm.handlers, handler{frameIdx: vm.framesIndex - 1, ip: target, sp: sp})

		case OpEndTry:
			if n := len(vm.handlers); n > 0 {
				vm.handlers = vm.handlers[:n-1]
			}

		case OpMakeError:
			codeSlot := stack[sp-1]
			msgSlot := stack[sp-2]
			sp -= 2
			ev := MakeErrorValue(codeSlot, msgSlot)
			stack[sp] = FromValue(ev)
			sp++

		case OpRaise:
			sp--
			frame.ip = ip
			vm.sp = sp
			if ev, ok := stack[sp].ToValue().(*interpreter.ErrorValue); ok {
				vm.logError(ev)
				return &RaisedError{Val: ev}
			}
			return fmt.Errorf("raise requires an error value")

		case OpAssert:
			msgIdx := binary.BigEndian.Uint16(ins[ip:])
			ip += 2
			sp--
			if !(stack[sp].tag == SlotBool && stack[sp].i64 != 0) {
				frame.ip = ip
				vm.sp = sp
				ev := interpreter.NewError(500, interpreter.CoerceToString(pool.Slots[msgIdx].ToValue()))
				vm.logError(ev)
				return &RaisedError{Val: ev}
			}

		case OpTestPass:
			nameIdx := binary.BigEndian.Uint16(ins[ip:])
			ip += 2
			vm.TestPasses++
			fmt.Printf("[PASS] Test: %s\n", interpreter.CoerceToString(pool.Slots[nameIdx].ToValue()))

		case OpTestFail:
			nameIdx := binary.BigEndian.Uint16(ins[ip:])
			ip += 2
			sp--
			msg := interpreter.CoerceToString(stack[sp].ToValue())
			if ev, ok := stack[sp].ToValue().(*interpreter.ErrorValue); ok {
				msg = ev.Message
			}
			vm.TestFailures++
			fmt.Printf("[FAIL] Test: %s - %s\n", interpreter.CoerceToString(pool.Slots[nameIdx].ToValue()), msg)

		case OpStream:
			opByte := ins[ip]
			ip++
			target := stack[sp-1].ToValue()
			source := stack[sp-2].ToValue()
			sp -= 2
			res, err := vm.executeStream(source, target, opByte)
			if err != nil {
				frame.ip = ip
				vm.sp = sp
				return err
			}
			stack[sp] = FromValue(res)
			sp++
		}
	}

	frame.ip = ip
	vm.sp = sp
	return nil
}

func (vm *VM) executeBinaryOp(op Opcode, left, right interpreter.Value) error {
	// Power is the one operator whose interpreter semantics are not shared with
	// the other arithmetic cases; route it through PowValue so the VM and the
	// JIT runtime-helper tier stay identical.
	if op == OpPow {
		res, err := PowValue(left, right)
		if err != nil {
			return err
		}
		return vm.push(res)
	}

	// String concatenation
	if left.Type() == interpreter.STRING_VAL || right.Type() == interpreter.STRING_VAL {
		if op == OpAdd {
			s1 := interpreter.CoerceToString(left)
			s2 := interpreter.CoerceToString(right)
			return vm.push(&interpreter.StringValue{Val: interpreter.Concat(s1, s2)})
		}
	}

	// List concatenation (mirrors the interpreter's '+' on two lists)
	if op == OpAdd && left.Type() == interpreter.LIST_VAL && right.Type() == interpreter.LIST_VAL {
		l1 := left.(*interpreter.ListValue)
		l2 := right.(*interpreter.ListValue)
		l1Elems := l1.ElementsSlice()
		l2Elems := l2.ElementsSlice()
		combined := make([]interpreter.Value, 0, len(l1Elems)+len(l2Elems))
		combined = append(combined, l1Elems...)
		combined = append(combined, l2Elems...)
		return vm.push(interpreter.NewList(combined...))
	}

	// Integer arithmetic
	if left.Type() == interpreter.INTEGER_VAL && right.Type() == interpreter.INTEGER_VAL {
		l := left.(*interpreter.IntegerValue).Val
		r := right.(*interpreter.IntegerValue).Val
		switch op {
		case OpAdd:
			return vm.push(&interpreter.IntegerValue{Val: l + r})
		case OpSub:
			return vm.push(&interpreter.IntegerValue{Val: l - r})
		case OpMul:
			return vm.push(&interpreter.IntegerValue{Val: l * r})
		case OpDiv:
			if r == 0 {
				return &RaisedError{Val: interpreter.NewError(400, "division by zero")}
			}
			if l == -9223372036854775808 && r == -1 {
				return vm.push(&interpreter.IntegerValue{Val: -9223372036854775808})
			}
			return vm.push(&interpreter.IntegerValue{Val: l / r})
		case OpMod:
			if r == 0 {
				return &RaisedError{Val: interpreter.NewError(400, "modulo by zero")}
			}
			if l == -9223372036854775808 && r == -1 {
				return vm.push(&interpreter.IntegerValue{Val: 0})
			}
			return vm.push(&interpreter.IntegerValue{Val: l % r})
		}
	}

	if op == OpMod {
		// '%' is only defined for int/int, matching the interpreter.
		return &RaisedError{Val: interpreter.NewError(400, "modulo requires integer operands")}
	}

	// Float arithmetic
	lf := toFloat(left)
	rf := toFloat(right)
	switch op {
	case OpAdd:
		return vm.push(&interpreter.FloatValue{Val: lf + rf})
	case OpSub:
		return vm.push(&interpreter.FloatValue{Val: lf - rf})
	case OpMul:
		return vm.push(&interpreter.FloatValue{Val: lf * rf})
	case OpDiv:
		if rf == 0 {
			return &RaisedError{Val: interpreter.NewError(400, "division by zero")}
		}
		return vm.push(&interpreter.FloatValue{Val: lf / rf})
	}

	return fmt.Errorf("unsupported binary operation")
}

// PowValue applies `**` with the VM's exact semantics: integer operands stay
// integer (and reject negative exponents), everything else is coerced to a
// float like the interpreter does.
func PowValue(left, right interpreter.Value) (interpreter.Value, error) {
	if left.Type() == interpreter.INTEGER_VAL && right.Type() == interpreter.INTEGER_VAL {
		l := left.(*interpreter.IntegerValue).Val
		r := right.(*interpreter.IntegerValue).Val
		if r < 0 {
			return nil, &RaisedError{Val: interpreter.NewError(400, "negative exponent for integer power; use float operands")}
		}
		return &interpreter.IntegerValue{Val: interpreter.IntPow(l, r)}, nil
	}
	return &interpreter.FloatValue{Val: math.Pow(toFloat(left), toFloat(right))}, nil
}

// toFloat mirrors the interpreter's getNumericFloat: only int/float operands
// have a numeric value, everything else is treated as zero.
func toFloat(v interpreter.Value) float64 {
	switch n := v.(type) {
	case *interpreter.FloatValue:
		return n.Val
	case *interpreter.IntegerValue:
		return float64(n.Val)
	default:
		return 0
	}
}

func (vm *VM) executeComparison(op Opcode, left, right interpreter.Value) error {
	switch op {
	case OpEqual:
		return vm.push(nativeBool(interpreter.LooseEqual(left, right)))
	case OpNotEqual:
		return vm.push(nativeBool(!interpreter.LooseEqual(left, right)))
	case OpStrictEqual:
		return vm.push(nativeBool(interpreter.StrictEqual(left, right)))
	case OpGreaterThan, OpGreaterEqual, OpLessThan, OpLessEqual:
		return vm.push(nativeBool(CompareOrder(op, left, right)))
	}
	return fmt.Errorf("unknown comparison operator")
}

// CompareOrder applies <, <=, >, and >= with the VM's exact semantics: two
// strings compare lexicographically, and everything else is coerced to a float.
// It is shared by the VM and the JIT runtime-helper tier.
func CompareOrder(op Opcode, left, right interpreter.Value) bool {
	if ls, ok := left.(*interpreter.StringValue); ok {
		if rs, ok := right.(*interpreter.StringValue); ok {
			switch op {
			case OpGreaterThan:
				return ls.Val > rs.Val
			case OpGreaterEqual:
				return ls.Val >= rs.Val
			case OpLessThan:
				return ls.Val < rs.Val
			case OpLessEqual:
				return ls.Val <= rs.Val
			}
		}
	}
	lf, rf := toFloat(left), toFloat(right)
	switch op {
	case OpGreaterThan:
		return lf > rf
	case OpGreaterEqual:
		return lf >= rf
	case OpLessThan:
		return lf < rf
	case OpLessEqual:
		return lf <= rf
	}
	return false
}

func nativeBool(b bool) interpreter.Value {
	if b {
		return interpreter.TRUE
	}
	return interpreter.FALSE
}

// buildList materialises the list elements pushed on the stack. When every
// element is an int the list is stored unboxed, which avoids one allocation per
// element; the list materialises itself before any mutating operation.
func (vm *VM) buildList(startIdx, endIdx int) interpreter.Value {
	return BuildListSlots(vm.stack[startIdx:endIdx])
}

func (vm *VM) buildMap(startIdx, endIdx int) interpreter.Value {
	return BuildMapSlots(vm.stack[startIdx:endIdx])
}

func (vm *VM) executeIndex(target, idx interpreter.Value) (interpreter.Value, error) {
	return GetIndexValue(target, idx)
}

// GetIndexValue resolves one container index. It is shared by the VM and the
// JIT runtime-helper tier so list, string, map, boundary, and type errors are
// identical in both paths.
func GetIndexValue(target, idx interpreter.Value) (interpreter.Value, error) {
	if ip, ok := target.(interpreter.IndexProvider); ok {
		res := ip.Index(idx)
		if errVal, isErr := res.(*interpreter.ErrorValue); isErr {
			return nil, &RaisedError{Val: errVal}
		}
		return res, nil
	}

	switch t := target.(type) {
	case *interpreter.ListValue:
		iv, ok := idx.(*interpreter.IntegerValue)
		if !ok {
			return nil, &RaisedError{Val: interpreter.NewError(400, "list index must be an integer")}
		}
		iVal := iv.Val
		n := int64(t.Len())
		if iVal < 0 {
			iVal += n
		}
		if iVal < 0 || iVal >= n {
			return nil, &RaisedError{Val: interpreter.NewError(400, fmt.Sprintf("index %d out of bounds (len %d)", iv.Val, n))}
		}
		if ints := t.IntSlice(); ints != nil {
			return &interpreter.IntegerValue{Val: ints[iVal]}, nil
		}
		if t.Elements == nil {
			if runes := t.RuneSlice(); runes != nil {
				return interpreter.SingleRuneString(runes[iVal]), nil
			}
		}
		return t.Elements[iVal], nil

	case *interpreter.MapValue:
		key := interpreter.CoerceToString(idx)
		if val, ok := t.Get(key); ok {
			return val, nil
		}
		return interpreter.NULL, nil

	case *interpreter.StringValue:
		iv, ok := idx.(*interpreter.IntegerValue)
		if !ok {
			return nil, &RaisedError{Val: interpreter.NewError(400, "string index must be an integer")}
		}
		runes := t.RuneSlice() // index by character, not by byte
		iVal := iv.Val
		n := int64(len(runes))
		if iVal < 0 {
			iVal += n
		}
		if iVal < 0 || iVal >= n {
			return nil, &RaisedError{Val: interpreter.NewError(400, fmt.Sprintf("string index %d out of bounds", iv.Val))}
		}
		return &interpreter.StringValue{Val: string(runes[iVal])}, nil
	}

	return nil, &RaisedError{Val: interpreter.NewError(400, fmt.Sprintf("cannot index value of type %s", target.Type()))}
}

func (vm *VM) executeSetIndex(target, idx, val interpreter.Value) error {
	return SetIndexValue(target, idx, val)
}

// SetIndexValue assigns one container index. It is shared by the VM and the JIT
// runtime-helper tier so mutation semantics and errors match exactly.
func SetIndexValue(target, idx, val interpreter.Value) error {
	switch t := target.(type) {
	case *interpreter.ListValue:
		if iv, ok := idx.(*interpreter.IntegerValue); ok {
			t.Materialize()
			iVal := iv.Val
			n := int64(t.Len())
			if iVal < 0 {
				iVal += n
			}
			if iVal >= 0 && iVal < n {
				t.Elements[iVal] = val
				return nil
			}
		}
		return &RaisedError{Val: interpreter.NewError(400, "invalid list index for assignment")}
	case *interpreter.MapValue:
		t.Set(interpreter.CoerceToString(idx), val)
		return nil
	}
	return &RaisedError{Val: interpreter.NewError(400, "cannot assign to indexed target")}
}

// SetFieldValue assigns a named field. It mirrors the VM's OpSetField: maps and
// struct instances are mutated, and other targets are a no-op rather than an
// error.
func SetFieldValue(target interpreter.Value, field string, val interpreter.Value) {
	if m, ok := target.(*interpreter.MapValue); ok {
		m.Set(field, val)
	} else if si, ok := target.(*interpreter.StructInstance); ok {
		si.Fields[field] = val
	}
}

// IterValue converts a value into the list form the generic for-loop iterates
// over. It is shared by the VM and the JIT runtime-helper tier so list, map key
// order, string runes, integer ranges, and non-iterable errors are identical.
func IterValue(val interpreter.Value) (interpreter.Value, error) {
	switch x := val.(type) {
	case *interpreter.ListValue:
		return val, nil
	case *interpreter.MapValue:
		keys := x.Keys()
		out := make([]interpreter.Value, len(keys))
		for i, k := range keys {
			out[i] = &interpreter.StringValue{Val: k}
		}
		return interpreter.NewList(out...), nil
	case *interpreter.StringValue:
		return interpreter.NewRuneList(x.RuneSlice()), nil
	case *interpreter.IntegerValue:
		out := make([]interpreter.Value, 0)
		for i := int64(0); i < x.Val; i++ {
			out = append(out, &interpreter.IntegerValue{Val: i})
		}
		return interpreter.NewList(out...), nil
	}
	return nil, &RaisedError{Val: interpreter.NewError(400, "value is not iterable: "+string(val.Type()))}
}

// ListLenValue implements the OpListLen fast path: a list contributes its length
// directly, and any other value falls back to the real `len` builtin so rune
// counts, maps, blobs, and error messages match the interpreter.
func ListLenValue(slot StackSlot) (StackSlot, error) {
	if lst, ok := slot.obj.(*interpreter.ListValue); ok {
		return StackSlot{tag: SlotInt, i64: int64(lst.Len())}, nil
	}
	return CallBuiltin1Fallback(builtin1Len, slot)
}

// MakeErrorValue builds the error value OpMakeError pushes. It is shared by the
// VM and the JIT runtime-helper tier so the default 500 code and the message
// coercion match exactly.
func MakeErrorValue(codeSlot, msgSlot StackSlot) interpreter.Value {
	code := int64(500)
	if codeSlot.tag == SlotInt {
		code = codeSlot.i64
	}
	return interpreter.NewError(code, interpreter.CoerceToString(msgSlot.ToValue()))
}

func (vm *VM) executeGetField(target interpreter.Value, field string) (interpreter.Value, error) {
	return GetFieldValue(target, field)
}

// GetFieldValue resolves one field or method access. It is shared by the VM and
// the JIT runtime-helper tier so both produce the same method wrappers and
// errors.
func GetFieldValue(target interpreter.Value, field string) (interpreter.Value, error) {
	if ed, ok := target.(*interpreter.EnumDef); ok {
		if val, exists := ed.Variants[field]; exists {
			return val, nil
		}
		return interpreter.NULL, fmt.Errorf("enum %s has no variant %s", ed.Name, field)
	}
	if ev, ok := target.(*interpreter.EnumValue); ok {
		switch field {
		case "tag":
			return &interpreter.IntegerValue{Val: ev.Tag}, nil
		case "name":
			return &interpreter.StringValue{Val: ev.Name}, nil
		default:
			return interpreter.NULL, nil
		}
	}
	if m, ok := target.(*interpreter.MapValue); ok {
		if val, exists := m.Get(field); exists {
			return val, nil
		}
	}
	if si, ok := target.(*interpreter.StructInstance); ok {
		if val, exists := si.Fields[field]; exists {
			return val, nil
		}
		// A method of a class is bound to the receiver when it is read, which
		// keeps the compiled code shared between all receivers.
		if method := si.Def.LookupMethod(field); method != nil && method.Compiled != nil {
			return method.Compiled.Bind(si), nil
		}
	}
	if super, ok := target.(*interpreter.SuperValue); ok {
		if val, exists := super.Instance.Fields[field]; exists {
			return val, nil
		}
		if method := super.Start.LookupMethod(field); method != nil && method.Compiled != nil {
			return method.Compiled.Bind(super.Instance), nil
		}
	}
	if ev, ok := target.(*interpreter.ErrorValue); ok {
		switch field {
		case "msg", "message":
			return &interpreter.StringValue{Val: ev.Message}, nil
		case "code":
			return &interpreter.IntegerValue{Val: ev.Code}, nil
		case "time":
			return &interpreter.StringValue{Val: ev.Time}, nil
		}
		return interpreter.NULL, nil
	}
	// Support method call dispatch via interpreter.CallMethod
	methodWrapper := &interpreter.BuiltinFunction{
		Name: field,
		Fn: func(args ...interpreter.Value) interpreter.Value {
			if field == "stream" {
				return interpreter.NewError(500, "fn.stream() is not supported by the VM backend; run the program with the interpreter")
			}
			res, err := interpreter.CallMethod(target, field, args)
			if err != nil {
				return interpreter.NewError(500, err.Error())
			}
			return res
		},
	}
	return methodWrapper, nil
}

func (vm *VM) executeCall(fn interpreter.Value, numArgs int) error {
	switch callee := fn.(type) {
	case *CompiledFunction:
		if numArgs != callee.NumParameters {
			return fmt.Errorf("wrong number of arguments: want=%d, got=%d", callee.NumParameters, numArgs)
		}
		basePointer := vm.sp - numArgs
		if vm.framesIndex >= MaxFrames {
			return fmt.Errorf("call stack overflow")
		}
		frame := vm.frames[vm.framesIndex]
		if frame == nil {
			frame = &Frame{}
			vm.frames[vm.framesIndex] = frame
		}
		frame.fn = callee
		frame.ip = 0
		frame.basePointer = basePointer
		vm.framesIndex++
		vm.sp = basePointer + callee.NumLocals

		if jitEng := CurrentJITEngine(); jitEng != nil {
			if jitEng.ShouldCompile(callee) {
				pool := callee.Pool
				if pool == nil {
					pool = vm.pool
				}
				globals := callee.Globals
				if globals == nil {
					globals = vm.globals
				}
				newSP, completed, bailoutIP, pendingErr := jitEng.Execute(callee, vm.stack, globals, pool, basePointer, vm.sp, vm.jitCallbacks)
				if pendingErr != nil {
					vm.sp = newSP
					frame.ip = bailoutIP
					return &RaisedError{Val: pendingErr}
				}
				if completed {
					var returnVal StackSlot
					if newSP > basePointer {
						returnVal = vm.stack[newSP-1]
					} else {
						returnVal = StackSlot{tag: SlotNull}
					}
					vm.popFrame()
					vm.closeUpvalues(basePointer)
					vm.sp = basePointer - 1
					vm.stack[vm.sp] = returnVal
					vm.sp++
					return nil
				}
				vm.sp = newSP
				frame.ip = bailoutIP
			}
		}

	case *interpreter.BuiltinFunction:
		args := make([]interpreter.Value, numArgs)
		for idx, s := range vm.stack[vm.sp-numArgs : vm.sp] {
			args[idx] = s.ToValue()
		}
		res := callee.Fn(args...)
		vm.sp = vm.sp - 1 - numArgs // Clean up function and arguments from stack
		if ev, ok := res.(*interpreter.ErrorValue); ok {
			return &RaisedError{Val: ev}
		}
		if res != nil {
			vm.push(res)
		} else {
			vm.push(interpreter.NULL)
		}

	case *interpreter.FuncValue:
		// Fallback to tree-walking closure invocation
		args := make([]interpreter.Value, numArgs)
		for idx, s := range vm.stack[vm.sp-numArgs : vm.sp] {
			args[idx] = s.ToValue()
		}
		interp := interpreter.New()
		res, err := interp.InvokeFunction(callee, args)
		vm.sp = vm.sp - 1 - numArgs
		if err != nil {
			return err
		}
		if res != nil {
			vm.push(res)
		} else {
			vm.push(interpreter.NULL)
		}

	case *interpreter.StructDef:
		// Struct instantiation via VM
		args := make([]interpreter.Value, numArgs)
		for idx, s := range vm.stack[vm.sp-numArgs : vm.sp] {
			args[idx] = s.ToValue()
		}
		interp := interpreter.New()
		res, err := interp.InvokeFunction(callee, args)
		vm.sp = vm.sp - 1 - numArgs
		if err != nil {
			return err
		}
		if res != nil {
			vm.push(res)
		} else {
			vm.push(interpreter.NULL)
		}

	default:
		return fmt.Errorf("calling non-function: %T", fn)
	}
	return nil
}

func (vm *VM) executeStream(source, target interpreter.Value, opByte byte) (interpreter.Value, error) {
	opMap := map[byte]string{
		1: ">>",
		2: ">!",
		3: ">>>",
		4: "<<",
		5: "?>>",
		6: "?>>>",
		7: "&>>",
		8: "&&>>",
		9: "|>",
	}
	opStr := opMap[opByte]
	return interpreter.HandleStream(source, opStr, target, vm.env)
}

// slowArith evaluates "l op r" through the generic (type-dispatching) path and
// returns the result slot. Used by the int-specialised fast opcodes whenever
// their operands are not both plain integers (floats, strings, lists, ...).
func (vm *VM) slowArith(op Opcode, l, r StackSlot) (StackSlot, error) {
	if err := vm.executeBinaryOp(op, l.ToValue(), r.ToValue()); err != nil {
		return StackSlot{}, err
	}
	res := vm.stack[vm.sp-1]
	vm.sp--
	return res, nil
}

// registerIterBuiltin installs the hidden "@iter" helper used by for-in loops: lists iterate
// as-is, maps iterate their keys and strings iterate their characters.
func registerIterBuiltin(env *interpreter.Environment) {
	env.Set("@iter", &interpreter.BuiltinFunction{Name: "@iter", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) != 1 {
			return interpreter.NewError(400, "iteration needs exactly one value")
		}
		switch x := args[0].(type) {
		case *interpreter.ListValue:
			return x
		case *interpreter.MapValue:
			keys := x.Keys()
			out := make([]interpreter.Value, len(keys))
			for i, k := range keys {
				out[i] = &interpreter.StringValue{Val: k}
			}
			return &interpreter.ListValue{Elements: out}
		case *interpreter.StringValue:
			// A rune-backed list avoids allocating a StringValue per character.
			return interpreter.NewRuneList(x.RuneSlice())
		case *interpreter.IntegerValue:
			out := make([]interpreter.Value, 0)
			for i := int64(0); i < x.Val; i++ {
				out = append(out, &interpreter.IntegerValue{Val: i})
			}
			return &interpreter.ListValue{Elements: out}
		}
		return interpreter.NewError(400, "value is not iterable: "+string(args[0].Type()))
	}})
}

func registerAsyncBuiltins(env *interpreter.Environment) {
	env.Set("@ba", &interpreter.BuiltinFunction{
		Name: "@ba",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			if len(args) == 0 {
				return interpreter.NULL
			}
			fn := args[0]
			interpreter.AddAsync()
			go func() {
				defer interpreter.DoneAsync()
				_, _ = InvokeValue(fn, nil, env)
			}()
			return interpreter.NULL
		},
	})
	env.Set("@threads", &interpreter.BuiltinFunction{
		Name: "@threads",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			var wg sync.WaitGroup
			for _, arg := range args {
				fn := arg
				wg.Add(1)
				go func() {
					runtime.LockOSThread()
					defer runtime.UnlockOSThread()
					defer wg.Done()
					_, _ = InvokeValue(fn, nil, env)
				}()
			}
			wg.Wait()
			return interpreter.NULL
		},
	})
	env.Set("@timeout", &interpreter.BuiltinFunction{
		Name: "@timeout",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			if len(args) < 2 {
				return interpreter.NULL
			}
			fn := args[0]
			durVal := args[1]
			var dur time.Duration
			if iv, ok := durVal.(*interpreter.IntegerValue); ok {
				dur = time.Duration(iv.Val) * time.Millisecond
			} else if sv, ok := durVal.(*interpreter.StringValue); ok {
				d, err := time.ParseDuration(sv.Val)
				if err == nil {
					dur = d
				}
			}
			if dur <= 0 {
				res, _ := InvokeValue(fn, nil, env)
				return res
			}
			ctx, cancel := context.WithTimeout(context.Background(), dur)
			defer cancel()
			resChan := make(chan interpreter.Value, 1)
			go func() {
				res, _ := InvokeValue(fn, nil, env)
				resChan <- res
			}()
			select {
			case res := <-resChan:
				return res
			case <-ctx.Done():
				return interpreter.NewError(408, fmt.Sprintf("execution timed out after %s", dur.String()))
			}
		},
	})
}
