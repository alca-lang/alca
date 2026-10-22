package jit_test

import (
	"math"
	"strings"
	"testing"
	"unsafe"

	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/jit"
	"github.com/alca-lang/alca/pkg/lexer"
	"github.com/alca-lang/alca/pkg/parser"
	"github.com/alca-lang/alca/pkg/vm"
)

func TestNativeStrictEqualityForPrimitiveSlots(t *testing.T) {
	if !jit.HasNativeAMD64 {
		t.Skip("native AMD64 backend is unavailable")
	}

	fn := &vm.CompiledFunction{
		Instructions: append(
			append(
				append(vm.Make(vm.OpGetLocal, 0), vm.Make(vm.OpGetLocal, 1)...),
				vm.Make(vm.OpStrictEqual)...,
			),
			vm.Make(vm.OpReturnValue)...,
		),
		NumLocals:     2,
		NumParameters: 2,
	}
	engine := jit.NewEngine(jit.Config{Enabled: true, Native: true, Threshold: 1})
	artifact, err := engine.CompileArtifact(fn)
	if err != nil {
		t.Fatalf("compile native function: %v", err)
	}
	if artifact.NativeFn == nil {
		t.Fatal("strict equality function did not compile to native code")
	}
	defer artifact.Free()

	tests := []struct {
		name  string
		left  vm.StackSlot
		right vm.StackSlot
		want  bool
	}{
		{name: "same integers", left: vm.FromInt(7), right: vm.FromInt(7), want: true},
		{name: "different integers", left: vm.FromInt(7), right: vm.FromInt(8), want: false},
		{name: "different primitive types", left: vm.FromInt(7), right: vm.FromFloat(7), want: false},
		{name: "same floats", left: vm.FromFloat(1.5), right: vm.FromFloat(1.5), want: true},
		{name: "NaN floats", left: vm.FromFloat(math.NaN()), right: vm.FromFloat(math.NaN()), want: false},
		{name: "same booleans", left: vm.FromBool(true), right: vm.FromBool(true), want: true},
		{name: "different booleans", left: vm.FromBool(false), right: vm.FromBool(true), want: false},
		{name: "nulls", left: vm.NullSlot(), right: vm.NullSlot(), want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stack := [4]vm.StackSlot{test.left, test.right}
			globals := [1]vm.StackSlot{}
			pool := [1]vm.StackSlot{}
			ctx := &jit.NativeContext{
				StackPtr:    uintptr(unsafe.Pointer(&stack[0])),
				GlobalsPtr:  uintptr(unsafe.Pointer(&globals[0])),
				PoolPtr:     uintptr(unsafe.Pointer(&pool[0])),
				BasePointer: 0,
				SP:          2,
			}
			if status := artifact.NativeFn(ctx); status != jit.StatusSuccess {
				t.Fatalf("native execution status = %d, want success", status)
			}
			result := stack[2]
			if got := result.Bool(); got != test.want || !result.IsBool() {
				t.Fatalf("native equality = %v (tag %d), want %v", got, result.Tag(), test.want)
			}
		})
	}
}

// TestNativeLoopFallsThroughToEpilogue guards the end-of-bytecode label. A loop
// whose exit jump targets the instruction just past the last opcode must reach
// the native epilogue, not bail out on an unbound label.
func TestNativeLoopFallsThroughToEpilogue(t *testing.T) {
	if !jit.HasNativeAMD64 {
		t.Skip("native AMD64 backend is unavailable")
	}

	const src = `
i := 0;
total := 0;
while i < 1000000:
    total = total + (i % 7) * 3;
    i = i + 1;
`
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if len(p.SyntaxErrors()) > 0 {
		t.Fatalf("syntax errors: %v", p.SyntaxErrors())
	}
	c := vm.NewCompiler()
	if err := c.Compile(prog); err != nil {
		t.Fatalf("compile: %v", err)
	}
	code := c.Bytecode()

	native, _, err := jit.CompileAOT(&vm.CompiledFunction{
		Instructions: code.Instructions,
		Pool:         vm.NewConstantPool(code.Constants),
	})
	if err != nil {
		t.Fatalf("compile AOT: %v", err)
	}
	if len(native) == 0 {
		t.Fatal("CompileAOT produced no code")
	}
	addr, err := jit.AllocExec(len(native))
	if err != nil {
		t.Fatalf("allocate executable memory: %v", err)
	}
	defer jit.FreeExec(addr, len(native))
	copy(jit.UnsafeSlice(addr, len(native)), native)

	stack := make([]vm.StackSlot, 16)
	globals := make([]vm.StackSlot, len(c.GlobalNames())+64)
	pool := vm.NewConstantPool(code.Constants)
	ctx := &jit.NativeContext{
		StackPtr:   uintptr(unsafe.Pointer(&stack[0])),
		GlobalsPtr: uintptr(unsafe.Pointer(&globals[0])),
		PoolPtr:    uintptr(unsafe.Pointer(&pool.Slots[0])),
	}
	if status := jit.AsNativeFunc(addr)(ctx); status != jit.StatusSuccess {
		t.Fatalf("native status = %d (bailout IP %d), want success", status, ctx.BailoutIP)
	}

	wantTotal := int64(0)
	for i := int64(0); i < 1000000; i++ {
		wantTotal += (i % 7) * 3
	}
	if got := globals[1].Int(); !globals[1].IsInt() || got != wantTotal {
		t.Fatalf("total = %d (tag %d), want %d", got, globals[1].Tag(), wantTotal)
	}
	if got := globals[0].Int(); got != 1000000 {
		t.Fatalf("i = %d, want 1000000", got)
	}
}

// TestNativeFloatDivisionByNegativeZero ensures the native fast path defers to
// the VM for -0.0, which is numerically zero and must raise division by zero
// rather than producing infinity.
func TestNativeFloatDivisionByNegativeZero(t *testing.T) {
	if !jit.HasNativeAMD64 {
		t.Skip("native AMD64 backend is unavailable")
	}

	const src = `x := 1.0 / -0.0;
`
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if len(p.SyntaxErrors()) > 0 {
		t.Fatalf("syntax errors: %v", p.SyntaxErrors())
	}
	c := vm.NewCompiler()
	if err := c.Compile(prog); err != nil {
		t.Fatalf("compile: %v", err)
	}
	machine := vm.NewWithEnv(c.Bytecode(), interpreter.NewEnvironment())
	engine := jit.NewEngine(jit.Config{Enabled: true, Native: true, Threshold: 1})
	defer engine.Clear()
	vm.SetJITEngine(engine)
	defer vm.SetJITEngine(jit.Default())

	err := machine.Run()
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "division by zero") {
		t.Fatalf("run error = %v, want division by zero", err)
	}
	if artifact := engine.GetCompiled(machine.MainFunction()); artifact == nil || artifact.NativeFn == nil {
		t.Fatal("main function was not executed by the native backend")
	}
}

// TestNativeUnaryTruthinessAndNegation covers primitive semantics that differ
// from a raw i64 test: float truthiness, -0.0, NaN, and float negation.
func TestNativeUnaryTruthinessAndNegation(t *testing.T) {
	if !jit.HasNativeAMD64 {
		t.Skip("native AMD64 backend is unavailable")
	}

	const src = `
a := 1.5;
b := not a;
c := 0.0;
d := not c;
e := 1.5;
f := -e;
g := not true;
h := not false;
n := 0.0 / 1.0;
`
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if len(p.SyntaxErrors()) > 0 {
		t.Fatalf("syntax errors: %v", p.SyntaxErrors())
	}
	c := vm.NewCompiler()
	if err := c.Compile(prog); err != nil {
		t.Fatalf("compile: %v", err)
	}
	code := c.Bytecode()
	pool := vm.NewConstantPool(code.Constants)
	native, _, err := jit.CompileAOT(&vm.CompiledFunction{
		Instructions: code.Instructions,
		Pool:         pool,
	})
	if err != nil {
		t.Fatalf("compile AOT: %v", err)
	}
	addr, err := jit.AllocExec(len(native))
	if err != nil {
		t.Fatalf("allocate executable memory: %v", err)
	}
	defer jit.FreeExec(addr, len(native))
	copy(jit.UnsafeSlice(addr, len(native)), native)

	stack := make([]vm.StackSlot, 16)
	globals := make([]vm.StackSlot, len(c.GlobalNames())+64)
	ctx := &jit.NativeContext{
		StackPtr:   uintptr(unsafe.Pointer(&stack[0])),
		GlobalsPtr: uintptr(unsafe.Pointer(&globals[0])),
		PoolPtr:    uintptr(unsafe.Pointer(&pool.Slots[0])),
	}
	if status := jit.AsNativeFunc(addr)(ctx); status != jit.StatusSuccess {
		t.Fatalf("native status = %d (bailout IP %d), want success", status, ctx.BailoutIP)
	}

	index := make(map[string]int, len(c.GlobalNames()))
	for i, name := range c.GlobalNames() {
		index[name] = i
	}
	for name, want := range map[string]bool{
		"b": false,
		"d": true,
		"g": false,
		"h": true,
	} {
		slot := globals[index[name]]
		if !slot.IsBool() || slot.Bool() != want {
			t.Errorf("%s = %v (tag %d), want bool %v", name, slot.Bool(), slot.Tag(), want)
		}
	}
	if f := globals[index["f"]]; !f.IsFloat() || f.Float() != -1.5 {
		t.Errorf("f = %v (tag %d), want -1.5", f.Float(), f.Tag())
	}
	if n := globals[index["n"]]; !n.IsFloat() || n.Float() != 0 || math.Signbit(n.Float()) {
		t.Errorf("n = %v (tag %d), want +0.0", n.Float(), n.Tag())
	}
}

// TestNativeDivisionAndModuloByNegativeOne verifies the backend does not
// deoptimize valid / -1 and % -1 operations (the hardware IDIV fault only
// concerns MinInt64 / -1, which is defined as the wrapped dividend here).
func TestNativeDivisionAndModuloByNegativeOne(t *testing.T) {
	if !jit.HasNativeAMD64 {
		t.Skip("native AMD64 backend is unavailable")
	}

	const src = `
a := 10 / -1;
b := 10 % -1;
c := 0 - 9223372036854775807 - 1;
d := c / -1;
e := c % -1;
`
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if len(p.SyntaxErrors()) > 0 {
		t.Fatalf("syntax errors: %v", p.SyntaxErrors())
	}
	c := vm.NewCompiler()
	if err := c.Compile(prog); err != nil {
		t.Fatalf("compile: %v", err)
	}
	code := c.Bytecode()
	pool := vm.NewConstantPool(code.Constants)
	native, _, err := jit.CompileAOT(&vm.CompiledFunction{
		Instructions: code.Instructions,
		Pool:         pool,
	})
	if err != nil {
		t.Fatalf("compile AOT: %v", err)
	}
	addr, err := jit.AllocExec(len(native))
	if err != nil {
		t.Fatalf("allocate executable memory: %v", err)
	}
	defer jit.FreeExec(addr, len(native))
	copy(jit.UnsafeSlice(addr, len(native)), native)

	stack := make([]vm.StackSlot, 16)
	globals := make([]vm.StackSlot, len(c.GlobalNames())+64)
	ctx := &jit.NativeContext{
		StackPtr:   uintptr(unsafe.Pointer(&stack[0])),
		GlobalsPtr: uintptr(unsafe.Pointer(&globals[0])),
		PoolPtr:    uintptr(unsafe.Pointer(&pool.Slots[0])),
	}
	if status := jit.AsNativeFunc(addr)(ctx); status != jit.StatusSuccess {
		t.Fatalf("native status = %d (bailout IP %d), want success", status, ctx.BailoutIP)
	}

	index := make(map[string]int, len(c.GlobalNames()))
	for i, name := range c.GlobalNames() {
		index[name] = i
	}
	for name, want := range map[string]int64{
		"a": -10,
		"b": 0,
		"c": math.MinInt64,
		"d": math.MinInt64,
		"e": 0,
	} {
		slot := globals[index[name]]
		if !slot.IsInt() || slot.Int() != want {
			t.Errorf("%s = %d (tag %d), want %d", name, slot.Int(), slot.Tag(), want)
		}
	}
}

func TestNativeMathIntrinsics(t *testing.T) {
	if !jit.HasNativeAMD64 {
		t.Skip("native AMD64 backend is unavailable")
	}

	tests := []struct {
		name string
		id   int8
		args []vm.StackSlot
		want string
	}{
		{name: "abs integer", id: vm.MathBuiltinAbs, args: []vm.StackSlot{vm.FromInt(-8)}, want: "8"},
		{name: "abs float negative zero", id: vm.MathBuiltinAbs, args: []vm.StackSlot{vm.FromFloat(math.Copysign(0, -1))}, want: "0.0"},
		{name: "sqrt integer", id: vm.MathBuiltinSqrt, args: []vm.StackSlot{vm.FromInt(81)}, want: "9.0"},
		{name: "round positive tie", id: vm.MathBuiltinRound, args: []vm.StackSlot{vm.FromFloat(2.5)}, want: "3"},
		{name: "round negative tie", id: vm.MathBuiltinRound, args: []vm.StackSlot{vm.FromFloat(-2.5)}, want: "-3"},
		{name: "floor negative", id: vm.MathBuiltinFloor, args: []vm.StackSlot{vm.FromFloat(-2.1)}, want: "-3"},
		{name: "ceil negative", id: vm.MathBuiltinCeil, args: []vm.StackSlot{vm.FromFloat(-2.1)}, want: "-2"},
		{name: "min mixed", id: vm.MathBuiltinMin, args: []vm.StackSlot{vm.FromInt(2), vm.FromFloat(3)}, want: "2"},
		{name: "max mixed", id: vm.MathBuiltinMax, args: []vm.StackSlot{vm.FromInt(2), vm.FromFloat(3)}, want: "3.0"},
		{name: "clamp int", id: vm.MathBuiltinClamp, args: []vm.StackSlot{vm.FromInt(12), vm.FromInt(1), vm.FromInt(10)}, want: "10"},
		{name: "clamp float", id: vm.MathBuiltinClamp, args: []vm.StackSlot{vm.FromFloat(-2.5), vm.FromInt(-1), vm.FromFloat(1.5)}, want: "-1.0"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			instructions := vm.Instructions{}
			for i := range test.args {
				instructions = append(instructions, vm.Make(vm.OpGetLocal, i)...)
			}
			instructions = append(instructions, vm.Make(vm.OpMathBuiltin, int(test.id))...)
			instructions = append(instructions, vm.Make(vm.OpReturnValue)...)
			fn := &vm.CompiledFunction{
				Instructions:  instructions,
				NumLocals:     len(test.args),
				NumParameters: len(test.args),
			}
			artifact, err := jit.NewEngine(jit.Config{Enabled: true, Native: true, Threshold: 1}).CompileArtifact(fn)
			if err != nil {
				t.Fatalf("compile native function: %v", err)
			}
			if artifact.NativeFn == nil {
				t.Fatal("math intrinsic did not compile to native code")
			}
			defer artifact.Free()

			// The locals live in slots [0, len(args)) and every OpGetLocal
			// pushes a copy onto the operand stack above them, so the frame
			// needs room for the arguments plus the result slot.
			stack := make([]vm.StackSlot, 2*len(test.args)+1)
			copy(stack, test.args)
			globals := [1]vm.StackSlot{}
			pool := [1]vm.StackSlot{}
			ctx := &jit.NativeContext{
				StackPtr:   uintptr(unsafe.Pointer(&stack[0])),
				GlobalsPtr: uintptr(unsafe.Pointer(&globals[0])),
				PoolPtr:    uintptr(unsafe.Pointer(&pool[0])),
				SP:         int64(len(test.args)),
			}
			if status := artifact.NativeFn(ctx); status != jit.StatusSuccess {
				t.Fatalf("native execution status = %d, want success (bailout IP %d)", status, ctx.BailoutIP)
			}
			got := stack[len(test.args)].ToValue().Inspect()
			if got != test.want {
				t.Fatalf("native result = %q, want %q", got, test.want)
			}
		})
	}
}

func TestNativeMathIntrinsicBailsOutForInvalidType(t *testing.T) {
	if !jit.HasNativeAMD64 {
		t.Skip("native AMD64 backend is unavailable")
	}
	fn := &vm.CompiledFunction{
		Instructions: append(
			append(vm.Make(vm.OpGetLocal, 0), vm.Make(vm.OpMathBuiltin, int(vm.MathBuiltinSqrt))...),
			vm.Make(vm.OpReturnValue)...,
		),
		NumLocals:     1,
		NumParameters: 1,
	}
	artifact, err := jit.NewEngine(jit.Config{Enabled: true, Native: true, Threshold: 1}).CompileArtifact(fn)
	if err != nil {
		t.Fatalf("compile native function: %v", err)
	}
	if artifact.NativeFn == nil {
		t.Fatal("math intrinsic did not compile to native code")
	}
	defer artifact.Free()

	stack := [3]vm.StackSlot{vm.FromValue(&interpreter.StringValue{Val: "not a number"})}
	globals := [1]vm.StackSlot{}
	pool := [1]vm.StackSlot{}
	ctx := &jit.NativeContext{
		StackPtr:   uintptr(unsafe.Pointer(&stack[0])),
		GlobalsPtr: uintptr(unsafe.Pointer(&globals[0])),
		PoolPtr:    uintptr(unsafe.Pointer(&pool[0])),
		SP:         1,
	}
	if status := artifact.NativeFn(ctx); status != jit.StatusBailout {
		t.Fatalf("native status = %d, want bailout", status)
	}
	if ctx.BailoutIP != 2 {
		t.Fatalf("bailout IP = %d, want intrinsic instruction at 2", ctx.BailoutIP)
	}
}

func TestThreadedStrictEquality(t *testing.T) {
	for _, test := range []struct {
		name  string
		left  vm.StackSlot
		right vm.StackSlot
		want  bool
	}{
		{name: "equal integers", left: vm.FromInt(5), right: vm.FromInt(5), want: true},
		{name: "different integers", left: vm.FromInt(5), right: vm.FromInt(6), want: false},
		{name: "different types", left: vm.FromInt(5), right: vm.FromFloat(5), want: false},
		{name: "equal booleans", left: vm.FromBool(true), right: vm.FromBool(true), want: true},
		{name: "equal nulls", left: vm.NullSlot(), right: vm.NullSlot(), want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			fn := &vm.CompiledFunction{
				Instructions: append(
					append(
						append(vm.Make(vm.OpGetLocal, 0), vm.Make(vm.OpGetLocal, 1)...),
						vm.Make(vm.OpStrictEqual)...,
					),
					vm.Make(vm.OpReturnValue)...,
				),
				NumLocals:     2,
				NumParameters: 2,
			}
			stack := [4]vm.StackSlot{test.left, test.right}
			ctx := &jit.ThreadedContext{Stack: stack[:], SP: 2}
			status, _ := jit.CompileThreaded(fn).Execute(ctx)
			if status != jit.StatusSuccess {
				t.Fatalf("threaded execution status = %d, want success", status)
			}
			if got := stack[2].Bool(); got != test.want || !stack[2].IsBool() {
				t.Fatalf("threaded equality = %v (tag %d), want %v", got, stack[2].Tag(), test.want)
			}
		})
	}
}

func compileSource(t *testing.T, src string) *vm.Bytecode {
	t.Helper()
	l := lexer.New(src)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.SyntaxErrors()) > 0 {
		t.Fatalf("syntax errors: %v", p.SyntaxErrors())
	}
	c := vm.NewCompiler()
	if err := c.Compile(prog); err != nil {
		t.Fatalf("compile error: %v", err)
	}
	return c.Bytecode()
}

func TestJITArithmeticAndLoops(t *testing.T) {
	src := `
func sum_to(n: int) -> int:
    total: int = 0;
    i: int = 1;
    while i <= n:
        total = total + i;
        i = i + 1;
    return total;

sum_to(100);
`
	bc := compileSource(t, src)
	t.Logf("Bytecode:\n%s", bc.Instructions)
	for i, c := range bc.Constants {
		t.Logf("Const %d: %v (type %T)", i, c, c)
		if fn, ok := c.(*vm.CompiledFunction); ok {
			t.Logf("Fn %s instructions:\n%s", fn.Name, fn.Instructions)
		}
	}
	machine := vm.New(bc)

	// Ensure JIT engine is active with aggressive threshold 1
	engine := jit.NewEngine(jit.Config{
		Enabled:   true,
		Native:    jit.HasNativeAMD64,
		Threshold: 1,
	})
	vm.SetJITEngine(engine)
	defer vm.SetJITEngine(jit.Default())

	if err := machine.Run(); err != nil {
		t.Fatalf("VM run error: %v", err)
	}

	val := machine.LastPopped()
	if val == nil {
		t.Fatalf("expected return value on stack")
	}
	// sum 1..100 = 5050
	if val.Inspect() != "5050" {
		t.Fatalf("expected sum 5050, got %s", val.Inspect())
	}
}

func TestJITThreadedExecution(t *testing.T) {
	src := `
func compute(a: int, b: int) -> int:
    x: int = a * 2;
    y: int = b + 3;
    if x > y:
        return x - y;
    else:
        return y - x;

compute(10, 5);
`
	bc := compileSource(t, src)
	t.Logf("Bytecode:\n%s", bc.Instructions)
	for i, c := range bc.Constants {
		t.Logf("Const %d: %v (type %T)", i, c, c)
		if fn, ok := c.(*vm.CompiledFunction); ok {
			t.Logf("Fn %s instructions:\n%s", fn.Name, fn.Instructions)
		}
	}
	machine := vm.New(bc)

	// Force threaded engine (Native = false)
	engine := jit.NewEngine(jit.Config{
		Enabled:   true,
		Native:    false,
		Threshold: 1,
	})
	vm.SetJITEngine(engine)
	defer vm.SetJITEngine(jit.Default())

	if err := machine.Run(); err != nil {
		t.Fatalf("VM run error: %v", err)
	}

	val := machine.LastPopped()
	// x = 20, y = 8, x - y = 12
	if val.Inspect() != "12" {
		t.Fatalf("expected 12, got %s", val.Inspect())
	}
}

func TestJITBailoutDeoptimization(t *testing.T) {
	// Function uses basic arithmetic then falls back to a complex operation (string concat)
	src := `
func format_sum(a: int, b: int) -> string:
    s: int = a + b;
    return "sum: " + string(s);

format_sum(20, 22);
`
	bc := compileSource(t, src)
	t.Logf("Bytecode:\n%s", bc.Instructions)
	for i, c := range bc.Constants {
		t.Logf("Const %d: %v (type %T)", i, c, c)
		if fn, ok := c.(*vm.CompiledFunction); ok {
			t.Logf("Fn %s instructions:\n%s", fn.Name, fn.Instructions)
		}
	}
	machine := vm.New(bc)

	engine := jit.NewEngine(jit.Config{
		Enabled:   true,
		Native:    jit.HasNativeAMD64,
		Threshold: 1,
	})
	vm.SetJITEngine(engine)
	defer vm.SetJITEngine(jit.Default())

	if err := machine.Run(); err != nil {
		t.Fatalf("VM run error: %v", err)
	}

	val := machine.LastPopped()
	if val.Inspect() != "sum: 42" {
		t.Fatalf("expected \"sum: 42\", got %s", val.Inspect())
	}
}

func TestJITRecursiveFibonacci(t *testing.T) {
	src := `
func fib(n: int) -> int:
    if n <= 1:
        return n;
    return fib(n - 1) + fib(n - 2);

fib(10);
`
	bc := compileSource(t, src)
	t.Logf("Bytecode:\n%s", bc.Instructions)
	for i, c := range bc.Constants {
		t.Logf("Const %d: %v (type %T)", i, c, c)
		if fn, ok := c.(*vm.CompiledFunction); ok {
			t.Logf("Fn %s instructions:\n%s", fn.Name, fn.Instructions)
		}
	}
	machine := vm.New(bc)

	engine := jit.NewEngine(jit.Config{
		Enabled:   true,
		Native:    jit.HasNativeAMD64,
		Threshold: 1,
	})
	vm.SetJITEngine(engine)
	defer vm.SetJITEngine(jit.Default())

	if err := machine.Run(); err != nil {
		t.Fatalf("VM run error: %v", err)
	}

	val := machine.LastPopped()
	// fib(10) = 55
	if val.Inspect() != "55" {
		t.Fatalf("expected fib(10) == 55, got %s", val.Inspect())
	}
}

func TestJITRangeLoop(t *testing.T) {
	src := `
func sum_range() -> int:
    sum: int = 0;
    for i in 1..100:
        sum = sum + i;
    return sum;

sum_range();
`
	bc := compileSource(t, src)
	t.Logf("Bytecode:\n%s", bc.Instructions)
	for i, c := range bc.Constants {
		t.Logf("Const %d: %v (type %T)", i, c, c)
		if fn, ok := c.(*vm.CompiledFunction); ok {
			t.Logf("Fn %s instructions:\n%s", fn.Name, fn.Instructions)
		}
	}
	machine := vm.New(bc)

	engine := jit.NewEngine(jit.Config{
		Enabled:   true,
		Native:    jit.HasNativeAMD64,
		Threshold: 1,
	})
	vm.SetJITEngine(engine)
	defer vm.SetJITEngine(jit.Default())

	if err := machine.Run(); err != nil {
		t.Fatalf("VM run error: %v", err)
	}

	val := machine.LastPopped()
	if val.Inspect() != "5050" {
		t.Fatalf("expected 5050, got %s", val.Inspect())
	}
}

func TestJITGlobalVariables(t *testing.T) {
	src := `
counter: int = 0;
func inc_counter(times: int):
    i: int = 0;
    while i < times:
        counter = counter + 1;
        i = i + 1;

inc_counter(50);
`
	bc := compileSource(t, src)
	t.Logf("Bytecode:\n%s", bc.Instructions)
	for i, c := range bc.Constants {
		t.Logf("Const %d: %v (type %T)", i, c, c)
		if fn, ok := c.(*vm.CompiledFunction); ok {
			t.Logf("Fn %s instructions:\n%s", fn.Name, fn.Instructions)
		}
	}
	machine := vm.New(bc)

	engine := jit.NewEngine(jit.Config{
		Enabled:   true,
		Native:    jit.HasNativeAMD64,
		Threshold: 1,
	})
	vm.SetJITEngine(engine)
	defer vm.SetJITEngine(jit.Default())

	if err := machine.Run(); err != nil {
		t.Fatalf("VM run error: %v", err)
	}

	g := machine.GetGlobal(0)
	if g.Inspect() != "50" {
		t.Fatalf("expected counter == 50, got %s", g.Inspect())
	}
}

func BenchmarkLoopVM(b *testing.B) {
	src := `
func loop_calc(n: int) -> int:
    s: int = 0;
    i: int = 0;
    while i < n:
        s = s + i;
        i = i + 1;
    return s;

loop_calc(100000);
`
	l := lexer.New(src)
	p := parser.New(l)
	prog := p.ParseProgram()
	c := vm.NewCompiler()
	_ = c.Compile(prog)
	bc := c.Bytecode()

	// Disable JIT for baseline
	vm.SetJITEngine(nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		machine := vm.New(bc)
		_ = machine.Run()
	}
}

func BenchmarkLoopJIT(b *testing.B) {
	src := `
func loop_calc(n: int) -> int:
    s: int = 0;
    i: int = 0;
    while i < n:
        s = s + i;
        i = i + 1;
    return s;

loop_calc(100000);
`
	l := lexer.New(src)
	p := parser.New(l)
	prog := p.ParseProgram()
	c := vm.NewCompiler()
	_ = c.Compile(prog)
	bc := c.Bytecode()

	engine := jit.NewEngine(jit.Config{
		Enabled:   true,
		Native:    jit.HasNativeAMD64,
		Threshold: 1,
	})
	vm.SetJITEngine(engine)
	defer vm.SetJITEngine(jit.Default())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		machine := vm.New(bc)
		_ = machine.Run()
	}
}

func TestJITModConstAndMulConst(t *testing.T) {
	src := `
func calc(n: int) -> int:
    sum: int = 0;
    i: int = 0;
    while i < n:
        sum = sum + (i % 7) * 3;
        i = i + 1;
    return sum;

calc(100);
`
	bc := compileSource(t, src)
	t.Logf("Bytecode:\n%s", bc.Instructions)
	for i, c := range bc.Constants {
		t.Logf("Const %d: %v (type %T)", i, c, c)
		if fn, ok := c.(*vm.CompiledFunction); ok {
			t.Logf("Fn %s instructions:\n%s", fn.Name, fn.Instructions)
		}
	}
	machine := vm.New(bc)

	engine := jit.NewEngine(jit.Config{
		Enabled:   true,
		Native:    jit.HasNativeAMD64,
		Threshold: 1,
	})
	vm.SetJITEngine(engine)
	defer vm.SetJITEngine(jit.Default())

	if err := machine.Run(); err != nil {
		t.Fatalf("VM run error: %v", err)
	}

	val := machine.LastPopped()
	if val == nil {
		t.Fatalf("expected return value")
	}
	// 14 full cycles of 63 = 882. For i=98: 0, for i=99: 3. Total = 885.
	if val.Inspect() != "885" {
		t.Fatalf("expected sum 885, got %s", val.Inspect())
	}
}

func TestJITMinInt64DivNoCrash(t *testing.T) {
	src := `
func test_div(a: int, b: int) -> int:
    return a / b;

result := test_div(-9223372036854775808, -1);
`
	bc := compileSource(t, src)
	t.Logf("Bytecode:\n%s", bc.Instructions)
	for i, c := range bc.Constants {
		t.Logf("Const %d: %v (type %T)", i, c, c)
		if fn, ok := c.(*vm.CompiledFunction); ok {
			t.Logf("Fn %s instructions:\n%s", fn.Name, fn.Instructions)
		}
	}
	machine := vm.New(bc)

	engine := jit.NewEngine(jit.Config{
		Enabled:   true,
		Native:    jit.HasNativeAMD64,
		Threshold: 1,
	})
	vm.SetJITEngine(engine)
	defer vm.SetJITEngine(jit.Default())

	if err := machine.Run(); err != nil {
		t.Fatalf("VM run error: %v", err)
	}

	val := machine.GetGlobal(1)
	if val == nil || val.Inspect() != "-9223372036854775808" {
		t.Fatalf("expected -9223372036854775808, got %v", val)
	}
}

func TestJITMinInt64ModByMinusOneNoCrash(t *testing.T) {
	if !jit.HasNativeAMD64 {
		t.Skip("native AMD64 JIT is unavailable")
	}

	for _, tc := range []struct {
		name string
		op   vm.Opcode
		args []int
	}{
		{name: "stack constant", op: vm.OpModConst, args: []int{1}},
		{name: "local constant", op: vm.OpModLocalConst, args: []int{0, 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fnIns := append(vm.Make(vm.OpGetLocal, 0), vm.Make(tc.op, tc.args...)...)
			fnIns = append(fnIns, vm.Make(vm.OpReturnValue)...)
			fn := &vm.CompiledFunction{
				Instructions:  fnIns,
				NumLocals:     1,
				NumParameters: 1,
				Name:          "mod_min_int64",
			}
			mainIns := append(vm.Make(vm.OpConstant, 0), vm.Make(vm.OpConstant, 2)...)
			mainIns = append(mainIns, vm.Make(vm.OpCall, 1)...)
			mainIns = append(mainIns, vm.Make(vm.OpPop)...)
			bc := &vm.Bytecode{
				Instructions: mainIns,
				Constants: []interpreter.Value{
					fn,
					interpreter.NewInteger(-1),
					interpreter.NewInteger(-1 << 63),
				},
			}
			engine := jit.NewEngine(jit.Config{Enabled: true, Native: true, Threshold: 1})
			vm.SetJITEngine(engine)
			defer vm.SetJITEngine(jit.Default())

			machine := vm.New(bc)
			if err := machine.Run(); err != nil {
				t.Fatalf("VM run error: %v", err)
			}
			if got := machine.LastPopped().Inspect(); got != "0" {
				t.Fatalf("modulo result = %s, want 0", got)
			}
			artifact := engine.GetCompiled(fn)
			if artifact == nil || artifact.NativeFn == nil {
				t.Fatal("expected function to be compiled to native code")
			}
		})
	}
}

func TestJITModuloDeoptsForNonIntegerOperands(t *testing.T) {
	if !jit.HasNativeAMD64 {
		t.Skip("native AMD64 JIT is unavailable")
	}

	for _, tc := range []struct {
		name string
		op   vm.Opcode
		args []int
	}{
		{name: "stack constant", op: vm.OpModConst, args: []int{1}},
		{name: "local constant", op: vm.OpModLocalConst, args: []int{0, 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var fnIns []byte
			if tc.op == vm.OpModConst {
				fnIns = append(fnIns, vm.Make(vm.OpGetLocal, 0)...)
			}
			fnIns = append(fnIns, vm.Make(tc.op, tc.args...)...)
			fnIns = append(fnIns, vm.Make(vm.OpReturnValue)...)
			fn := &vm.CompiledFunction{
				Instructions:  fnIns,
				NumLocals:     1,
				NumParameters: 1,
				Name:          "mod_non_integer",
			}
			mainIns := append(vm.Make(vm.OpConstant, 0), vm.Make(vm.OpConstant, 2)...)
			mainIns = append(mainIns, vm.Make(vm.OpCall, 1)...)
			mainIns = append(mainIns, vm.Make(vm.OpPop)...)
			bc := &vm.Bytecode{
				Instructions: mainIns,
				Constants: []interpreter.Value{
					fn,
					interpreter.NewInteger(2),
					&interpreter.StringValue{Val: "not an integer"},
				},
			}
			engine := jit.NewEngine(jit.Config{Enabled: true, Native: true, Threshold: 1})
			vm.SetJITEngine(engine)
			defer vm.SetJITEngine(jit.Default())

			if err := vm.New(bc).Run(); err == nil {
				t.Fatal("expected VM to reject non-integer modulo operands")
			}
			artifact := engine.GetCompiled(fn)
			if artifact == nil || artifact.NativeFn == nil {
				t.Fatal("expected function to be compiled to native code before deoptimization")
			}
		})
	}
}

func TestJITLoopOSR(t *testing.T) {
	src := `
i := 0;
total := 0;
while i < 10000:
    total = total + (i % 7) * 3;
    i = i + 1;
total;
`
	bc := compileSource(t, src)
	t.Logf("Bytecode:\n%s", bc.Instructions)
	for i, c := range bc.Constants {
		t.Logf("Const %d: %v (type %T)", i, c, c)
		if fn, ok := c.(*vm.CompiledFunction); ok {
			t.Logf("Fn %s instructions:\n%s", fn.Name, fn.Instructions)
		}
	}
	machine := vm.New(bc)

	engine := jit.NewEngine(jit.Config{
		Enabled:   true,
		Native:    jit.HasNativeAMD64,
		Threshold: 1,
	})
	vm.SetJITEngine(engine)
	defer vm.SetJITEngine(jit.Default())

	if err := machine.Run(); err != nil {
		t.Fatalf("VM run error: %v", err)
	}

	g := machine.GetGlobal(1)
	if g == nil {
		t.Fatalf("expected total global variable")
	}
	// 1428 cycles of 63 = 89964. For i=9996..9999: 0+3+6+9 = 18. Total = 89982.
	if g.Inspect() != "89982" {
		t.Fatalf("expected total 89982, got %s", g.Inspect())
	}
}

func TestJITFloatOperations(t *testing.T) {
	src := `
func compute_float(x: float, y: float) -> float:
    a: float = (x + y) * 2.5;
    b: float = a / 2.0;
    if b > 10.0:
        return b - 1.5;
    else:
        return b + 1.5;

compute_float(3.5, 2.5);
`
	for _, native := range []bool{true, false} {
		if native && !jit.HasNativeAMD64 {
			continue
		}
		mode := "native"
		if !native {
			mode = "threaded"
		}
		t.Run(mode, func(t *testing.T) {
			bc := compileSource(t, src)
			machine := vm.New(bc)
			engine := jit.NewEngine(jit.Config{
				Enabled:   true,
				Native:    native,
				Threshold: 1,
			})
			vm.SetJITEngine(engine)
			defer vm.SetJITEngine(jit.Default())

			if err := machine.Run(); err != nil {
				t.Fatalf("VM run error: %v", err)
			}
			val := machine.LastPopped()
			if val == nil {
				t.Fatalf("expected result on stack")
			}
			// (3.5 + 2.5) * 2.5 = 6.0 * 2.5 = 15.0; 15.0 / 2.0 = 7.5; 7.5 <= 10.0 -> 7.5 + 1.5 = 9.0
			if val.Inspect() != "9" && val.Inspect() != "9.0" && val.Inspect() != "9.000000" {
				t.Fatalf("expected 9.0, got %s", val.Inspect())
			}
		})
	}
}

func TestJITLocalRangeLoop(t *testing.T) {
	src := `
func loop_local() -> int:
    sum := 0;
    for i in 1..1000:
        sum = sum + i;
    return sum;

loop_local();
`
	for _, native := range []bool{true, false} {
		if native && !jit.HasNativeAMD64 {
			continue
		}
		mode := "native"
		if !native {
			mode = "threaded"
		}
		t.Run(mode, func(t *testing.T) {
			bc := compileSource(t, src)
			machine := vm.New(bc)
			engine := jit.NewEngine(jit.Config{
				Enabled:   true,
				Native:    native,
				Threshold: 1,
			})
			vm.SetJITEngine(engine)
			defer vm.SetJITEngine(jit.Default())

			if err := machine.Run(); err != nil {
				t.Fatalf("VM run error: %v", err)
			}
			val := machine.LastPopped()
			if val == nil {
				t.Fatalf("expected result on stack")
			}
			// sum 1..1000 = 500500
			if val.Inspect() != "500500" {
				t.Fatalf("expected 500500, got %s", val.Inspect())
			}
		})
	}
}
