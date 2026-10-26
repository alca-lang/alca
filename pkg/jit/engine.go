// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package jit

import (
	"encoding/binary"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/limits"
	"github.com/alca-lang/alca/pkg/vm"
)

// Config controls JIT compilation behavior.
type Config struct {
	Enabled   bool // Whether JIT compilation is enabled
	Native    bool // Whether to attempt native machine code generation
	Threshold int  // Call/loop count threshold before triggering JIT (default 5)
	// MaxArtifacts bounds the number of compiled artifacts the engine keeps.
	// Zero means the default bound. A program that keeps creating new
	// functions runs the ones that do not fit in the bytecode VM.
	MaxArtifacts int
}

// DefaultConfig returns the recommended JIT configuration.
func DefaultConfig() Config {
	return Config{
		Enabled:      false,
		Native:       HasNativeAMD64,
		Threshold:    5,
		MaxArtifacts: DefaultMaxArtifacts,
	}
}

// DefaultMaxArtifacts is the default bound on the number of compiled JIT
// artifacts. It keeps the memory of the engine bounded for programs that
// create a large number of distinct functions or closures.
const DefaultMaxArtifacts = 4096

// CompiledJIT holds a JIT-compiled function artifact.
type CompiledJIT struct {
	NativeFn   NativeFunc
	NativeAddr uintptr
	NativeSize int
	IPOffsets  map[int]uintptr
	ThreadedFn *ThreadedFunction
	Fn         *vm.CompiledFunction
}

// Free releases any executable native memory associated with this JIT artifact.
func (c *CompiledJIT) Free() {
	if c.NativeAddr != 0 {
		_ = FreeExec(c.NativeAddr, c.NativeSize)
		c.NativeAddr = 0
		c.NativeFn = nil
	}
}

// memoryCost returns the approximate number of bytes this artifact occupies:
// the native machine code, the threaded instruction table, and the bookkeeping
// of the artifact itself.
func (c *CompiledJIT) memoryCost() int64 {
	cost := int64(c.NativeSize)
	if c.ThreadedFn != nil {
		cost += int64(len(c.ThreadedFn.Insts)) * 64
	}
	return cost + 512
}

// Engine manages JIT profiling, hot-spot compilation, and execution.
type Engine struct {
	mu          sync.RWMutex
	cfg         Config
	cache       map[*vm.CompiledFunction]*CompiledJIT
	counts      map[*vm.CompiledFunction]int
	helperCalls atomic.Int64
}

// Global default JIT engine.
var defaultEngine = NewEngine(DefaultConfig())

// Default returns the process-wide JIT engine.
func Default() *Engine {
	return defaultEngine
}

// NewEngine creates a new JIT Engine with the specified configuration.
func NewEngine(cfg Config) *Engine {
	return &Engine{
		cfg:    cfg,
		cache:  make(map[*vm.CompiledFunction]*CompiledJIT),
		counts: make(map[*vm.CompiledFunction]int),
	}
}

// Config returns the engine configuration.
func (e *Engine) Config() Config {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.cfg
}

// SetEnabled toggles JIT compilation globally on or off.
func (e *Engine) SetEnabled(enabled bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cfg.Enabled = enabled
}

// SetThreshold updates the execution counter threshold before JIT triggers.
func (e *Engine) SetThreshold(threshold int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cfg.Threshold = threshold
}

// Clear frees all JIT-compiled native executable artifacts and resets compilation counters.
func (e *Engine) Clear() {
	e.mu.Lock()
	defer e.mu.Unlock()
	var freed int64
	for _, cjit := range e.cache {
		freed += cjit.memoryCost()
		cjit.Free()
	}
	e.cache = make(map[*vm.CompiledFunction]*CompiledJIT)
	e.counts = make(map[*vm.CompiledFunction]int)
	e.helperCalls.Store(0)
	limits.Refund(freed)
}

// RuntimeHelperCalls returns how many stdlib or collection helpers the JIT
// engine has executed directly since construction or the last Clear.
func (e *Engine) RuntimeHelperCalls() int64 {
	return e.helperCalls.Load()
}

func init() {
	vm.SetJITEngine(defaultEngine)
}

// Compile compiles a VM function using native JIT (if available) or threaded JIT.
func (e *Engine) Compile(fn *vm.CompiledFunction) error {
	_, err := e.CompileArtifact(fn)
	return err
}

// artifactKey returns the function whose compiled artifact is used for fn.
// A closure is a copy of its template that only differs in the values it
// captured, so every instance of a closure shares the code of the template and
// therefore one compiled artifact. Without this, a program that creates many
// closures would compile the same code again and again.
func artifactKey(fn *vm.CompiledFunction) *vm.CompiledFunction {
	if fn == nil {
		return nil
	}
	if fn.Template != nil {
		return fn.Template
	}
	return fn
}

// CompileArtifact compiles and caches a JIT artifact for the function.
func (e *Engine) CompileArtifact(fn *vm.CompiledFunction) (*CompiledJIT, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	key := artifactKey(fn)
	if cjit, ok := e.cache[key]; ok {
		return cjit, nil
	}

	// The artifact cache is bounded. A program that keeps creating new
	// functions must not be able to grow the engine without bound; the
	// bytecode VM runs the functions that do not fit.
	maxArtifacts := e.cfg.MaxArtifacts
	if maxArtifacts <= 0 {
		maxArtifacts = DefaultMaxArtifacts
	}
	if len(e.cache) >= maxArtifacts {
		return nil, fmt.Errorf("JIT artifact limit reached (%d)", maxArtifacts)
	}

	cjit := &CompiledJIT{Fn: key}

	// Attempt native AMD64 compilation if enabled and on supported architecture
	if e.cfg.Native && HasNativeAMD64 {
		nativeFn, addr, size, ipOffsets, err := compileNative(key)
		if err == nil && nativeFn != nil {
			cjit.NativeFn = nativeFn
			cjit.NativeAddr = addr
			cjit.NativeSize = size
			cjit.IPOffsets = ipOffsets
		}
	}

	// Always compile threaded JIT as portable tier / deopt fallback
	cjit.ThreadedFn = CompileThreaded(key)

	e.cache[key] = cjit
	// The compiled code is part of the program's memory footprint, so the
	// memory guard accounts for it.
	limits.Charge(cjit.memoryCost())
	return cjit, nil
}

// InstallAOT registers a precompiled AOT native machine code artifact into the engine cache.
func (e *Engine) InstallAOT(fn *vm.CompiledFunction, addr uintptr, size int, ipOffsets map[int]uintptr) {
	e.mu.Lock()
	defer e.mu.Unlock()
	cjit := &CompiledJIT{
		Fn:         fn,
		NativeFn:   AsNativeFunc(addr),
		NativeAddr: addr,
		NativeSize: size,
		IPOffsets:  ipOffsets,
		ThreadedFn: CompileThreaded(fn),
	}
	e.cache[artifactKey(fn)] = cjit
	// The precompiled code is part of the program's memory footprint as well.
	limits.Charge(cjit.memoryCost())
}

// ShouldCompile checks if a function has become hot enough to warrant JIT compilation.
func (e *Engine) ShouldCompile(fn *vm.CompiledFunction) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.cfg.Enabled {
		return false
	}
	key := artifactKey(fn)
	if _, ok := e.cache[key]; ok {
		return true
	}
	e.counts[key]++
	return e.counts[key] >= e.cfg.Threshold
}

// GetCompiled returns the compiled JIT artifact if available.
func (e *Engine) GetCompiled(fn *vm.CompiledFunction) *CompiledJIT {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.cache[artifactKey(fn)]
}

// CompiledArtifacts returns the number of compiled artifacts the engine keeps.
// Every instance of a closure shares the artifact of its template, so the
// number stays small even when a program creates many closures.
func (e *Engine) CompiledArtifacts() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.cache)
}

// ArtifactBytes returns the total size of the compiled artifacts.
func (e *Engine) ArtifactBytes() int64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var total int64
	for _, cjit := range e.cache {
		total += cjit.memoryCost()
	}
	return total
}

// Execute runs the JIT compiled function.
func (e *Engine) Execute(fn *vm.CompiledFunction, stack []vm.StackSlot, globals []vm.StackSlot, pool *vm.ConstantPool, basePointer int, sp int, callbacks *vm.JITCallbacks) (int, bool, int, *interpreter.ErrorValue) {
	return e.ExecuteAt(fn, stack, globals, pool, basePointer, sp, 0, callbacks)
}

// ExecuteAt runs the JIT compiled function starting execution at entryIP (for OSR).
// A non-nil pending error is an Alca error raised by a JIT runtime helper; the
// caller must deliver it to the normal VM unwinder. It must not re-run the call.
func (e *Engine) ExecuteAt(fn *vm.CompiledFunction, stack []vm.StackSlot, globals []vm.StackSlot, pool *vm.ConstantPool, basePointer int, sp int, entryIP int, callbacks *vm.JITCallbacks) (int, bool, int, *interpreter.ErrorValue) {
	cjit := e.GetCompiled(fn)
	if cjit == nil {
		var err error
		cjit, err = e.CompileArtifact(fn)
		if err != nil || cjit == nil {
			// The function stays in the bytecode VM. The bailout point is the
			// entry of the function, so the VM continues where it stopped.
			return sp, false, entryIP, nil
		}
	}

	newSP, status, bailoutIP, pendingErr := e.executeArtifactAt(cjit, fn, stack, globals, pool, basePointer, sp, entryIP, callbacks)
	return newSP, status == StatusSuccess, bailoutIP, pendingErr
}

// ExecuteArtifact executes a compiled JIT artifact.
func (e *Engine) ExecuteArtifact(cjit *CompiledJIT, stack []vm.StackSlot, globals []vm.StackSlot, pool *vm.ConstantPool, basePointer int, sp int) (int, int64, int, *interpreter.ErrorValue) {
	return e.ExecuteArtifactAt(cjit, stack, globals, pool, basePointer, sp, 0)
}

// ExecuteArtifactAt executes a compiled JIT artifact starting at entryIP.
func (e *Engine) ExecuteArtifactAt(cjit *CompiledJIT, stack []vm.StackSlot, globals []vm.StackSlot, pool *vm.ConstantPool, basePointer int, sp int, entryIP int) (int, int64, int, *interpreter.ErrorValue) {
	return e.executeArtifactAt(cjit, cjit.Fn, stack, globals, pool, basePointer, sp, entryIP, nil)
}

// executeArtifactAt is ExecuteArtifactAt with the VM callbacks that let helpers
// invoke user functions and create closures. fn is the function instance that
// runs: it can differ from the function of the artifact, because every instance
// of a closure shares the compiled code of its template but has its own
// captured values.
func (e *Engine) executeArtifactAt(cjit *CompiledJIT, fn *vm.CompiledFunction, stack []vm.StackSlot, globals []vm.StackSlot, pool *vm.ConstantPool, basePointer int, sp int, entryIP int, callbacks *vm.JITCallbacks) (int, int64, int, *interpreter.ErrorValue) {
	if cjit.NativeFn != nil {
		stackPtr := uintptr(0)
		if len(stack) > 0 {
			stackPtr = uintptr(unsafe.Pointer(&stack[0]))
		}
		globalsPtr := uintptr(0)
		if len(globals) > 0 {
			globalsPtr = uintptr(unsafe.Pointer(&globals[0]))
		}
		poolPtr := uintptr(0)
		if pool != nil && len(pool.Slots) > 0 {
			poolPtr = uintptr(unsafe.Pointer(&pool.Slots[0]))
		}

		var entryOffset uintptr
		if entryIP > 0 && cjit.IPOffsets != nil {
			entryOffset = cjit.IPOffsets[entryIP]
		}

		ctx := NativeContext{
			StackPtr:    stackPtr,
			GlobalsPtr:  globalsPtr,
			PoolPtr:     poolPtr,
			BasePointer: int64(basePointer),
			SP:          int64(sp),
			EntryOffset: entryOffset,
			StackLimit:  int64(len(stack)),
		}

		for {
			status := cjit.NativeFn(&ctx)
			runtime.KeepAlive(stack)
			runtime.KeepAlive(globals)
			runtime.KeepAlive(pool)
			if status != StatusHelper {
				if limits.Exceeded() {
					// Native code does not allocate Alca data itself, so the
					// limit can only be reached through a helper call. Report
					// it as a regular Alca error the program can catch.
					return int(ctx.SP), StatusError, int(ctx.BailoutIP), interpreter.NewError(507, limits.Message())
				}
				return int(ctx.SP), status, int(ctx.BailoutIP), nil
			}

			resumeIP, ok, pendingErr := e.runNativeHelper(fn, stack, globals, &ctx, callbacks)
			if !ok {
				// No side effects have happened; let the VM execute the fused
				// opcode through its normal path.
				return int(ctx.SP), StatusBailout, int(ctx.HelperIP), nil
			}
			if pendingErr != nil {
				return int(ctx.SP), StatusError, resumeIP, pendingErr
			}
			if limits.Exceeded() {
				return int(ctx.SP), StatusError, resumeIP, interpreter.NewError(507, limits.Message())
			}
			if cjit.IPOffsets == nil {
				return int(ctx.SP), StatusError, resumeIP, interpreter.NewError(500, "JIT helper has no resume point")
			}
			offset, found := cjit.IPOffsets[resumeIP]
			if !found && resumeIP != 0 {
				return int(ctx.SP), StatusError, resumeIP, interpreter.NewError(500, "JIT helper resume point is missing")
			}
			ctx.EntryOffset = offset
		}
	}

	if cjit.ThreadedFn != nil {
		ctx := ThreadedContext{
			Stack:       stack,
			Globals:     globals,
			Pool:        pool,
			BasePointer: basePointer,
			SP:          sp,
			Env:         fn.Env,
			SourceFile:  fn.SourceFile,
			Callbacks:   callbacks,
			// The instance that runs, so that captured values are read from
			// the closure that is executing and not from its template.
			Fn: fn,
		}
		status, nextIP := cjit.ThreadedFn.ExecuteAt(&ctx, entryIP)
		e.helperCalls.Add(ctx.HelperCalls)
		if limits.Exceeded() && ctx.Err == nil {
			// The threaded tier allocates Alca data only through helper calls,
			// so an exceeded limit is reported the same way the native tier
			// reports it.
			return ctx.SP, StatusError, nextIP, interpreter.NewError(507, limits.Message())
		}
		return ctx.SP, status, nextIP, ctx.Err
	}

	return sp, StatusBailout, 0, nil
}

// runNativeHelper executes the helper requested at ctx.HelperIP and updates the
// native stack/SP. ok=false means the helper is not safe to run here and no
// state has changed, so the caller can hand the instruction to the VM.
func (e *Engine) runNativeHelper(fn *vm.CompiledFunction, stack []vm.StackSlot, globals []vm.StackSlot, ctx *NativeContext, callbacks *vm.JITCallbacks) (resumeIP int, ok bool, pendingErr *interpreter.ErrorValue) {
	if fn == nil || ctx.HelperIP < 0 || int(ctx.HelperIP) >= len(fn.Instructions) {
		return 0, false, nil
	}
	ip := int(ctx.HelperIP)
	ins := fn.Instructions
	sp := int(ctx.SP)
	resumeIP = int(ctx.ResumeIP)
	if sp < 0 || sp > len(stack) {
		return 0, false, nil
	}

	switch vm.Opcode(ins[ip]) {
	case vm.OpApplyDecorator:
		if fn.Pool == nil {
			return 0, false, nil
		}
		specIdx := int(binary.BigEndian.Uint16(ins[ip+1:]))
		if specIdx < 0 || specIdx >= len(fn.Pool.Vals) {
			return 0, false, nil
		}
		spec, ok := fn.Pool.Vals[specIdx].(*vm.DecoratorSpec)
		if !ok || callbacks == nil || callbacks.ApplyDecorator == nil {
			return 0, false, nil
		}
		callbacks.ApplyDecorator(spec)
		e.helperCalls.Add(1)
		return resumeIP, true, nil

	case vm.OpMakeClass:
		if fn.Pool == nil {
			return 0, false, nil
		}
		templateIdx := int(binary.BigEndian.Uint16(ins[ip+1:]))
		parentIdx := int(binary.BigEndian.Uint16(ins[ip+3:]))
		if templateIdx < 0 || templateIdx >= len(fn.Pool.Vals) {
			return 0, false, nil
		}
		template, ok := fn.Pool.Vals[templateIdx].(*vm.ClassTemplate)
		if !ok {
			return 0, false, nil
		}
		var parent *interpreter.StructDef
		if parentIdx > 0 {
			if parentIdx-1 >= len(globals) {
				return 0, false, nil
			}
			parentValue, isDef := globals[parentIdx-1].ToValue().(*interpreter.StructDef)
			if !isDef || !parentValue.IsClass {
				return 0, false, nil
			}
			parent = parentValue
		}
		for _, method := range template.Methods {
			if method != nil && method.Fn != nil {
				method.Fn.Pool = fn.Pool
			}
		}
		e.helperCalls.Add(1)
		stack[sp] = vm.FromValue(vm.BuildClassDef(template, parent))
		ctx.SP = int64(sp + 1)
		return resumeIP, true, nil

	case vm.OpStdlibCall:
		aliasIdx := int(binary.BigEndian.Uint16(ins[ip+1:]))
		memberIdx := int(binary.BigEndian.Uint16(ins[ip+3:]))
		argc := int(ins[ip+5])
		if argc < 0 || sp < argc || fn.Pool == nil || aliasIdx < 0 || aliasIdx >= len(fn.Pool.Vals) || memberIdx < 0 || memberIdx >= len(fn.Pool.Vals) {
			return 0, false, nil
		}
		alias, ok := fn.Pool.Vals[aliasIdx].(*interpreter.StringValue)
		if !ok {
			return 0, false, nil
		}
		member, ok := fn.Pool.Vals[memberIdx].(*interpreter.StringValue)
		if !ok {
			return 0, false, nil
		}
		res, handled, errVal := invokeStdlibMember(fn.Env, alias.Val, member.Val, stack[sp-argc:sp])
		if !handled {
			return 0, false, nil
		}
		e.helperCalls.Add(1)
		sp -= argc
		ctx.SP = int64(sp)
		if errVal != nil {
			return resumeIP, true, errVal
		}
		stack[sp] = vm.FromValue(res)
		ctx.SP = int64(sp + 1)
		return resumeIP, true, nil

	case vm.OpUse:
		if fn.Env == nil || fn.Pool == nil {
			return 0, false, nil
		}
		useIdx := int(binary.BigEndian.Uint16(ins[ip+1:]))
		if useIdx < 0 || useIdx >= len(fn.Pool.Vals) {
			return 0, false, nil
		}
		useVal, ok := fn.Pool.Vals[useIdx].(*vm.UseStatementValue)
		if !ok {
			return 0, false, nil
		}
		vm.ExecuteUseStatement(fn.Env, useVal, fn.SourceFile)
		e.helperCalls.Add(1)
		return resumeIP, true, nil

	case vm.OpGetBuiltin:
		if fn.Pool == nil || sp >= len(stack) {
			return 0, false, nil
		}
		nameIdx := int(binary.BigEndian.Uint16(ins[ip+1:]))
		if nameIdx < 0 || nameIdx >= len(fn.Pool.Vals) {
			return 0, false, nil
		}
		name, ok := fn.Pool.Vals[nameIdx].(*interpreter.StringValue)
		if !ok {
			return 0, false, nil
		}
		stack[sp] = vm.FromValue(vm.ResolveBuiltinValue(fn.Env, name.Val))
		ctx.SP = int64(sp + 1)
		e.helperCalls.Add(1)
		return resumeIP, true, nil

	case vm.OpGetField:
		if sp < 1 || fn.Pool == nil {
			return 0, false, nil
		}
		fieldIdx := int(binary.BigEndian.Uint16(ins[ip+1:]))
		if fieldIdx < 0 || fieldIdx >= len(fn.Pool.Vals) {
			return 0, false, nil
		}
		field, ok := fn.Pool.Vals[fieldIdx].(*interpreter.StringValue)
		if !ok {
			return 0, false, nil
		}
		res, err := vm.GetFieldValue(stack[sp-1].ToValue(), field.Val)
		if err != nil {
			return 0, false, nil
		}
		stack[sp-1] = vm.FromValue(res)
		e.helperCalls.Add(1)
		return resumeIP, true, nil

	case vm.OpCall:
		argc := int(ins[ip+1])
		if argc < 0 || sp < argc+1 {
			return 0, false, nil
		}
		callee := stack[sp-1-argc].ToValue()
		res, handled, errVal := invokeCallable(callbacks, callee, stack, sp, argc)
		if !handled {
			return 0, false, nil
		}
		e.helperCalls.Add(1)
		sp -= argc + 1
		ctx.SP = int64(sp)
		if errVal != nil {
			return resumeIP, true, errVal
		}
		stack[sp] = vm.FromValue(res)
		ctx.SP = int64(sp + 1)
		return resumeIP, true, nil

	case vm.OpClosure:
		if fn.Pool == nil || sp >= len(stack) {
			return 0, false, nil
		}
		constIdx := int(binary.BigEndian.Uint16(ins[ip+1:]))
		if constIdx < 0 || constIdx >= len(fn.Pool.Vals) {
			return 0, false, nil
		}
		template, ok := fn.Pool.Vals[constIdx].(*vm.CompiledFunction)
		if !ok || callbacks == nil || callbacks.MakeClosure == nil {
			return 0, false, nil
		}
		closure := callbacks.MakeClosure(template)
		if closure == nil {
			return 0, false, nil
		}
		e.helperCalls.Add(1)
		stack[sp] = vm.FromValue(closure)
		ctx.SP = int64(sp + 1)
		return resumeIP, true, nil

	case vm.OpGetFree:
		if sp >= len(stack) {
			return 0, false, nil
		}
		slot, ok := vm.UpvalueValue(fn, int(ins[ip+1]), stack)
		if !ok {
			return 0, false, nil
		}
		e.helperCalls.Add(1)
		stack[sp] = slot
		ctx.SP = int64(sp + 1)
		return resumeIP, true, nil

	case vm.OpSetFree:
		if sp < 1 {
			return 0, false, nil
		}
		if !vm.SetUpvalueValue(fn, int(ins[ip+1]), stack[sp-1], stack) {
			return 0, false, nil
		}
		e.helperCalls.Add(1)
		ctx.SP = int64(sp - 1)
		return resumeIP, true, nil

	case vm.OpList:
		count := int(binary.BigEndian.Uint16(ins[ip+1:]))
		if count < 0 || sp < count {
			return 0, false, nil
		}
		list := vm.BuildListSlots(stack[sp-count : sp])
		e.helperCalls.Add(1)
		sp -= count
		stack[sp] = vm.FromValue(list)
		ctx.SP = int64(sp + 1)
		return resumeIP, true, nil

	case vm.OpMap:
		pairs := int(binary.BigEndian.Uint16(ins[ip+1:]))
		count := pairs * 2
		if pairs < 0 || sp < count {
			return 0, false, nil
		}
		m := vm.BuildMapSlots(stack[sp-count : sp])
		e.helperCalls.Add(1)
		sp -= count
		stack[sp] = vm.FromValue(m)
		ctx.SP = int64(sp + 1)
		return resumeIP, true, nil

	case vm.OpPow:
		if sp < 2 {
			return 0, false, nil
		}
		res, err := vm.PowValue(stack[sp-2].ToValue(), stack[sp-1].ToValue())
		e.helperCalls.Add(1)
		sp -= 2
		ctx.SP = int64(sp)
		if err != nil {
			return resumeIP, true, raisedErrorValue(err)
		}
		stack[sp] = vm.FromValue(res)
		ctx.SP = int64(sp + 1)
		return resumeIP, true, nil

	case vm.OpIn:
		if sp < 2 {
			return 0, false, nil
		}
		res := interpreter.InOp(stack[sp-2].ToValue(), stack[sp-1].ToValue())
		e.helperCalls.Add(1)
		sp -= 2
		ctx.SP = int64(sp)
		if ev, isErr := res.(*interpreter.ErrorValue); isErr {
			return resumeIP, true, ev
		}
		stack[sp] = vm.FromValue(res)
		ctx.SP = int64(sp + 1)
		return resumeIP, true, nil

	case vm.OpIndex:
		if sp < 2 {
			return 0, false, nil
		}
		res, err := vm.GetIndexValue(stack[sp-2].ToValue(), stack[sp-1].ToValue())
		e.helperCalls.Add(1)
		sp -= 2
		ctx.SP = int64(sp)
		if err != nil {
			return resumeIP, true, raisedErrorValue(err)
		}
		stack[sp] = vm.FromValue(res)
		ctx.SP = int64(sp + 1)
		return resumeIP, true, nil

	case vm.OpSetIndex:
		if sp < 3 {
			return 0, false, nil
		}
		err := vm.SetIndexValue(stack[sp-2].ToValue(), stack[sp-1].ToValue(), stack[sp-3].ToValue())
		e.helperCalls.Add(1)
		sp -= 3
		ctx.SP = int64(sp)
		if err != nil {
			return resumeIP, true, raisedErrorValue(err)
		}
		return resumeIP, true, nil

	case vm.OpSetField:
		if sp < 2 || fn.Pool == nil {
			return 0, false, nil
		}
		fieldIdx := int(binary.BigEndian.Uint16(ins[ip+1:]))
		if fieldIdx < 0 || fieldIdx >= len(fn.Pool.Vals) {
			return 0, false, nil
		}
		field, isStr := fn.Pool.Vals[fieldIdx].(*interpreter.StringValue)
		if !isStr {
			return 0, false, nil
		}
		vm.SetFieldValue(stack[sp-1].ToValue(), field.Val, stack[sp-2].ToValue())
		e.helperCalls.Add(1)
		sp -= 2
		ctx.SP = int64(sp)
		return resumeIP, true, nil

	case vm.OpRange:
		if sp < 2 {
			return 0, false, nil
		}
		res := interpreter.EvalRange(stack[sp-2].ToValue(), stack[sp-1].ToValue())
		e.helperCalls.Add(1)
		sp -= 2
		stack[sp] = vm.FromValue(res)
		ctx.SP = int64(sp + 1)
		return resumeIP, true, nil

	case vm.OpIter:
		if sp < 1 {
			return 0, false, nil
		}
		res, err := vm.IterValue(stack[sp-1].ToValue())
		e.helperCalls.Add(1)
		if err != nil {
			return resumeIP, true, raisedErrorValue(err)
		}
		stack[sp-1] = vm.FromValue(res)
		return resumeIP, true, nil

	case vm.OpListLen:
		if sp < 1 {
			return 0, false, nil
		}
		res, err := vm.ListLenValue(stack[sp-1])
		e.helperCalls.Add(1)
		if err != nil {
			return resumeIP, true, raisedErrorValue(err)
		}
		stack[sp-1] = res
		return resumeIP, true, nil

	case vm.OpCallBuiltin1:
		if sp < 1 {
			return 0, false, nil
		}
		id := int8(ins[ip+1])
		slot := stack[sp-1]
		if !vm.ApplyBuiltin1(id, &slot) {
			res, err := vm.CallBuiltin1Fallback(id, slot)
			if err != nil {
				e.helperCalls.Add(1)
				return resumeIP, true, raisedErrorValue(err)
			}
			slot = res
		}
		e.helperCalls.Add(1)
		stack[sp-1] = slot
		return resumeIP, true, nil

	case vm.OpConvLocal:
		if fn.Pool == nil {
			return 0, false, nil
		}
		id := int8(ins[ip+1])
		src := int(ins[ip+2])
		local := ctx.BasePointer + int64(src)
		if local < 0 || int(local) >= len(stack) || sp >= len(stack) {
			return 0, false, nil
		}
		slot := stack[local]
		if !vm.ApplyBuiltin1(id, &slot) {
			res, err := vm.CallBuiltin1Fallback(id, slot)
			if err != nil {
				e.helperCalls.Add(1)
				return resumeIP, true, raisedErrorValue(err)
			}
			slot = res
		}
		e.helperCalls.Add(1)
		stack[sp] = slot
		ctx.SP = int64(sp + 1)
		return resumeIP, true, nil

	case vm.OpConvGlobal:
		if fn.Pool == nil || globals == nil {
			return 0, false, nil
		}
		id := int8(ins[ip+1])
		src := int(binary.BigEndian.Uint16(ins[ip+2:]))
		if src < 0 || src >= len(globals) || sp >= len(stack) {
			return 0, false, nil
		}
		slot := globals[src]
		if !vm.ApplyBuiltin1(id, &slot) {
			res, err := vm.CallBuiltin1Fallback(id, slot)
			if err != nil {
				e.helperCalls.Add(1)
				return resumeIP, true, raisedErrorValue(err)
			}
			slot = res
		}
		e.helperCalls.Add(1)
		stack[sp] = slot
		ctx.SP = int64(sp + 1)
		return resumeIP, true, nil

	case vm.OpEqual, vm.OpNotEqual, vm.OpStrictEqual, vm.OpLessThan, vm.OpLessEqual, vm.OpGreaterThan, vm.OpGreaterEqual:
		if sp < 2 {
			return 0, false, nil
		}
		left := stack[sp-2].ToValue()
		right := stack[sp-1].ToValue()
		var res bool
		switch vm.Opcode(ins[ip]) {
		case vm.OpEqual:
			res = interpreter.LooseEqual(left, right)
		case vm.OpNotEqual:
			res = !interpreter.LooseEqual(left, right)
		case vm.OpStrictEqual:
			res = interpreter.StrictEqual(left, right)
		case vm.OpLessThan:
			res = vm.CompareOrder(vm.OpLessThan, left, right)
		case vm.OpLessEqual:
			res = vm.CompareOrder(vm.OpLessEqual, left, right)
		case vm.OpGreaterThan:
			res = vm.CompareOrder(vm.OpGreaterThan, left, right)
		case vm.OpGreaterEqual:
			res = vm.CompareOrder(vm.OpGreaterEqual, left, right)
		}
		e.helperCalls.Add(1)
		sp -= 2
		if res {
			stack[sp] = vm.FromValue(interpreter.TRUE)
		} else {
			stack[sp] = vm.FromValue(interpreter.FALSE)
		}
		ctx.SP = int64(sp + 1)
		return resumeIP, true, nil

	case vm.OpMakeError:
		if sp < 2 {
			return 0, false, nil
		}
		ev := vm.MakeErrorValue(stack[sp-1], stack[sp-2])
		e.helperCalls.Add(1)
		sp -= 2
		stack[sp] = vm.FromValue(ev)
		ctx.SP = int64(sp + 1)
		return resumeIP, true, nil

	case vm.OpRaise:
		if sp < 1 {
			return 0, false, nil
		}
		ev, isErr := stack[sp-1].ToValue().(*interpreter.ErrorValue)
		if !isErr {
			// The VM reports a plain Go error for a non-error raise; bail so it
			// can reproduce that exactly without changing the stack.
			return 0, false, nil
		}
		e.helperCalls.Add(1)
		sp--
		ctx.SP = int64(sp)
		return resumeIP, true, ev

	case vm.OpAssert:
		if sp < 1 || fn.Pool == nil {
			return 0, false, nil
		}
		msgIdx := int(binary.BigEndian.Uint16(ins[ip+1:]))
		if msgIdx < 0 || msgIdx >= len(fn.Pool.Slots) {
			return 0, false, nil
		}
		slot := stack[sp-1]
		e.helperCalls.Add(1)
		sp--
		ctx.SP = int64(sp)
		if slot.IsBool() && slot.Bool() {
			return resumeIP, true, nil
		}
		msg := interpreter.CoerceToString(fn.Pool.Slots[msgIdx].ToValue())
		return resumeIP, true, interpreter.NewError(500, msg)
	}
	return 0, false, nil
}
