// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package stdlib_test

import (
	"math"
	"testing"

	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/lexer"
	"github.com/alca-lang/alca/pkg/parser"
	"github.com/alca-lang/alca/pkg/stdlib"
)

// newLibtorch builds an environment with the libtorch module registered and
// returns a helper that invokes a module member by name, failing the test on any
// returned error value.
func newLibtorch(t *testing.T) func(name string, args ...interpreter.Value) interpreter.Value {
	t.Helper()
	env := interpreter.NewEnvironment()
	stdlib.RegisterLibtorchBuiltins(env)
	modVal, ok := env.Get("libtorch")
	if !ok {
		t.Fatal("libtorch module was not registered")
	}
	mod := modVal.(*interpreter.MapValue)
	return func(name string, args ...interpreter.Value) interpreter.Value {
		t.Helper()
		member, ok := mod.Get(name)
		if !ok {
			t.Fatalf("libtorch has no member %q", name)
		}
		res := member.(*interpreter.BuiltinFunction).Fn(args...)
		if errV, isErr := res.(*interpreter.ErrorValue); isErr {
			t.Fatalf("libtorch.%s returned error: %s", name, errV.Message)
		}
		return res
	}
}

func fnum(v float64) interpreter.Value { return &interpreter.FloatValue{Val: v} }

func flist(vals ...float64) interpreter.Value {
	elems := make([]interpreter.Value, len(vals))
	for i, v := range vals {
		elems[i] = fnum(v)
	}
	return interpreter.NewList(elems...)
}

func ival(v int) interpreter.Value { return &interpreter.IntegerValue{Val: int64(v)} }

// toFloats extracts a flat []float64 from a (possibly nested) list value.
func toFloats(t *testing.T, v interpreter.Value) []float64 {
	t.Helper()
	var out []float64
	var walk func(interpreter.Value)
	walk = func(x interpreter.Value) {
		switch tv := x.(type) {
		case *interpreter.FloatValue:
			out = append(out, tv.Val)
		case *interpreter.IntegerValue:
			out = append(out, float64(tv.Val))
		case *interpreter.ListValue:
			for _, e := range tv.ElementsSlice() {
				walk(e)
			}
		default:
			t.Fatalf("unexpected value in list: %v", x.Inspect())
		}
	}
	walk(v)
	return out
}

func TestLibtorchTensorMath(t *testing.T) {
	call := newLibtorch(t)
	a := call("tensor", flist(1, 2, 3), interpreter.TRUE)
	b := call("tensor", flist(4, 5, 6))
	sum := call("add", a, b)
	got := toFloats(t, call("to_list", sum))
	want := []float64{5, 7, 9}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("add mismatch at %d: got %v want %v", i, got[i], want[i])
		}
	}
	if req := call("requires_grad", a).(*interpreter.BoolValue); !req.Val {
		t.Fatal("expected a.requires_grad == true")
	}
}

func TestLibtorchAutograd(t *testing.T) {
	call := newLibtorch(t)
	// y = x*x, d(sum y)/dx = 2x
	x := call("tensor", flist(1, 2, 3), interpreter.TRUE)
	y := call("mul", x, x)
	call("backward", call("sum_all", y))
	got := toFloats(t, call("to_list", call("grad", x)))
	want := []float64{2, 4, 6}
	for i := range want {
		if math.Abs(got[i]-want[i]) > 1e-9 {
			t.Fatalf("grad mismatch at %d: got %v want %v", i, got[i], want[i])
		}
	}
}

func TestLibtorchTrainsLinearRegression(t *testing.T) {
	call := newLibtorch(t)
	w := call("tensor", flist(0), interpreter.TRUE)
	opt := call("adam", interpreter.NewList(w), fnum(0.1))

	xs := []float64{-2, -1, 0, 1, 2}
	ys := []float64{-6, -3, 0, 3, 6}
	for step := 0; step < 300; step++ {
		call("opt_zero_grad", opt)
		total := call("tensor", flist(0))
		for i, xv := range xs {
			pred := call("mul", w, call("tensor", flist(xv)))
			diff := call("sub", pred, call("tensor", flist(ys[i])))
			total = call("add", total, call("mul", diff, diff))
		}
		call("backward", call("mean", total))
		call("opt_step", opt)
	}
	finalW := call("item", w).(*interpreter.FloatValue).Val
	if math.Abs(finalW-3) > 0.15 {
		t.Fatalf("trained weight = %v, want ~3", finalW)
	}
}

func TestLibtorchConv2dForwardBackward(t *testing.T) {
	call := newLibtorch(t)
	in := call("tensor", interpreter.NewList(
		flist(1, 2, 3, 4),
		flist(5, 6, 7, 8),
		flist(9, 10, 11, 12),
		flist(13, 14, 15, 16),
	), interpreter.TRUE) // (1,4,4) -> will be treated as (4,4); add batch/channel below
	// Reshape to NCHW (1,1,4,4).
	in = call("reshape", in, intlistArg(1, 1, 4, 4))
	// weight (1,1,3,3)
	wt := call("tensor", interpreter.NewList(interpreter.NewList(interpreter.NewList(
		flist(1, 0, 1),
		flist(0, 1, 0),
		flist(1, 0, 1),
	))), interpreter.TRUE)
	conv := call("conv2d", in, wt, ival(1), ival(0)) // stride=1, pad=0, no bias
	shape := toInts(t, call("shape", conv))
	if len(shape) != 4 || shape[2] != 2 || shape[3] != 2 {
		t.Fatalf("conv2d output shape = %v, want [1 1 2 2]", shape)
	}
	call("backward", call("sum_all", conv))
	if call("grad", wt) == interpreter.NULL {
		t.Fatal("expected weight gradient after backward")
	}
}

func intlistArg(vals ...int) interpreter.Value {
	elems := make([]interpreter.Value, len(vals))
	for i, v := range vals {
		elems[i] = ival(v)
	}
	return interpreter.NewList(elems...)
}

func toInts(t *testing.T, v interpreter.Value) []int {
	t.Helper()
	list := v.(*interpreter.ListValue)
	out := make([]int, 0, len(list.ElementsSlice()))
	for _, e := range list.ElementsSlice() {
		switch iv := e.(type) {
		case *interpreter.IntegerValue:
			out = append(out, int(iv.Val))
		case *interpreter.FloatValue:
			out = append(out, int(iv.Val))
		default:
			t.Fatalf("expected int list, got %v", e.Inspect())
		}
	}
	return out
}

func TestLibtorchCrossEntropy(t *testing.T) {
	call := newLibtorch(t)
	logits := call("tensor", interpreter.NewList(flist(2, 1, 0), flist(1, 2, 0)), interpreter.TRUE)
	targets := call("tensor", flist(0, 1))
	loss := call("cross_entropy", logits, targets)
	if lv := call("item", loss).(*interpreter.FloatValue).Val; lv <= 0 {
		t.Fatalf("cross entropy should be positive, got %v", lv)
	}
	call("backward", loss)
	if call("grad", logits) == interpreter.NULL {
		t.Fatal("expected logits gradient")
	}
}

func TestLibtorchSaveLoad(t *testing.T) {
	call := newLibtorch(t)
	path := t.TempDir() + "/t.bin"
	w := call("tensor", flist(1, 2, 3, 4))
	dict := interpreter.NewMap()
	dict.Set("w", w)
	if sv := call("save", &interpreter.StringValue{Val: path}, dict); sv.Type() != interpreter.BOOL_VAL {
		t.Fatalf("save did not return bool: %v", sv.Inspect())
	}
	loaded := call("load", &interpreter.StringValue{Val: path}).(*interpreter.MapValue)
	lw, ok := loaded.Get("w")
	if !ok {
		t.Fatal("load did not return w")
	}
	got := toFloats(t, call("to_list", lw))
	want := []float64{1, 2, 3, 4}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("round trip mismatch at %d: got %v want %v", i, got[i], want[i])
		}
	}
}

func TestLibtorchAlcaProgram(t *testing.T) {
	source := `
use std.libtorch;

x := libtorch.tensor([1.0, 2.0, 3.0], true);
w := libtorch.tensor([0.5, -0.5, 1.0], true);
y := libtorch.mul(x, w);
loss := libtorch.sum_all(y);
libtorch.backward(loss);
g := libtorch.grad(x);
println(libtorch.item(libtorch.sum_all(g)));
`
	parsed := parser.New(lexer.New(source))
	program := parsed.ParseProgram()
	if errs := parsed.SyntaxErrors(); len(errs) > 0 {
		t.Fatalf("syntax error: %s", errs[0].Message)
	}
	runtime := interpreter.New()
	stdlib.RegisterLibtorchBuiltins(runtime.Env())
	if err := runtime.Execute(program); err != nil {
		t.Fatalf("executing libtorch Alca program failed: %v", err)
	}
}

func TestLibtorchCUDAMethods(t *testing.T) {
	source := `
use std.libtorch;

avail := libtorch.cuda_is_available();
count := libtorch.cuda_device_count();
t := libtorch.tensor([1.0, 2.0]);
dev := libtorch.device(t);
is_c := libtorch.is_cuda(t);
t_cpu := libtorch.to(t, "cpu");
mem := libtorch.cuda_memory_allocated();
`
	parsed := parser.New(lexer.New(source))
	program := parsed.ParseProgram()
	if errs := parsed.SyntaxErrors(); len(errs) > 0 {
		t.Fatalf("syntax error: %s", errs[0].Message)
	}
	runtime := interpreter.New()
	stdlib.RegisterLibtorchBuiltins(runtime.Env())
	if err := runtime.Execute(program); err != nil {
		t.Fatalf("executing CUDA Alca program failed: %v", err)
	}
	devVal, _ := runtime.Env().Get("dev")
	if devVal.(*interpreter.StringValue).Val != "cpu" {
		t.Fatalf("expected device 'cpu', got %v", devVal)
	}
	isCVal, _ := runtime.Env().Get("is_c")
	if isCVal.(*interpreter.BoolValue).Val {
		t.Fatal("expected is_cuda to be false")
	}
}
