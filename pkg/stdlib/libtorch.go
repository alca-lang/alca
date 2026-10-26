// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package stdlib

import (
	"fmt"
	"math"
	"math/rand"
	"strings"

	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/libtorch"
)

// ltTensor wraps a libtorch.Tensor as an opaque interpreter value. It preserves
// object identity across assignments (Clone returns the same handle) so that the
// autograd graph remains intact, mirroring Python tensor semantics.
type ltTensor struct{ t *libtorch.Tensor }

func (v *ltTensor) Type() interpreter.ValueType { return "LIBTORCH_TENSOR" }
func (v *ltTensor) Inspect() string             { return v.t.String() }
func (v *ltTensor) Clone() interpreter.Value    { return v } // share identity

func (v *ltTensor) Index(index interpreter.Value) interpreter.Value {
	iv, ok := index.(*interpreter.IntegerValue)
	if !ok {
		return interpreter.NewError(400, "tensor index must be an integer")
	}
	t := v.t
	dim := t.Dim()
	if dim == 0 {
		return interpreter.NewError(400, "cannot index a rank-0 tensor")
	}
	n := int64(t.Shape()[0])
	idx := iv.Val
	if idx < 0 {
		idx += n
	}
	if idx < 0 || idx >= n {
		return interpreter.NewError(400, fmt.Sprintf("index %d out of bounds (len %d)", iv.Val, n))
	}
	if dim == 1 {
		return &interpreter.FloatValue{Val: t.Data()[idx]}
	}
	subShape := t.Shape()[1:]
	subStride := 1
	for _, d := range subShape {
		subStride *= d
	}
	start := int(idx) * subStride
	subData := t.Data()[start : start+subStride]
	subT, err := libtorch.NewTensor(subShape, subData)
	if err != nil {
		return interpreter.NewError(400, err.Error())
	}
	return tensorVal(subT)
}

// ltOpt wraps a libtorch.Optimizer as an opaque interpreter value.
type ltOpt struct{ o libtorch.Optimizer }

func (v *ltOpt) Type() interpreter.ValueType { return "LIBTORCH_OPTIMIZER" }
func (v *ltOpt) Inspect() string             { return "<libtorch optimizer>" }
func (v *ltOpt) Clone() interpreter.Value    { return v }

// tensorVal wraps a tensor as an interpreter value.
func tensorVal(t *libtorch.Tensor) interpreter.Value { return &ltTensor{t: t} }

// CallMethod implements interpreter.MethodProvider, enabling Python-like method
// syntax on tensors: t.relu(), t.matmul(w), t.reshape([2,3]), etc.
func (v *ltTensor) CallMethod(name string, args []interpreter.Value) (interpreter.Value, error) {
	t := v.t
	switch name {
	// Unary
	case "relu":
		return tensorVal(t.Relu()), nil
	case "sigmoid":
		return tensorVal(t.Sigmoid()), nil
	case "tanh":
		return tensorVal(t.Tanh()), nil
	case "exp":
		return tensorVal(t.Exp()), nil
	case "log":
		return tensorVal(t.Log()), nil
	case "sqrt":
		return tensorVal(t.Sqrt()), nil
	case "abs":
		return tensorVal(t.Abs()), nil
	case "neg":
		return tensorVal(t.Neg()), nil
	case "softmax":
		if len(args) >= 1 {
			return tensorVal(t.Softmax(ltIntArg(args, 0, t.Dim()-1))), nil
		}
		return tensorVal(t.Softmax()), nil
	case "log_softmax":
		if len(args) >= 1 {
			return tensorVal(t.LogSoftmax(ltIntArg(args, 0, t.Dim()-1))), nil
		}
		return tensorVal(t.LogSoftmax()), nil
	case "gelu":
		return tensorVal(t.GELU(false)), nil
	case "silu":
		return tensorVal(t.SiLU()), nil
	case "mish":
		return tensorVal(t.Mish()), nil
	case "softplus":
		return tensorVal(t.Softplus()), nil
	case "flatten":
		return tensorVal(t.Flatten()), nil
	case "transpose":
		return tensorVal(t.Transpose()), nil
	case "detach":
		return tensorVal(t.Detach()), nil
	case "clone":
		return tensorVal(t.Clone()), nil
	// Binary
	case "add":
		if len(args) < 1 {
			return nil, fmt.Errorf("add requires 1 arg")
		}
		return tensorVal(t.Add(ltTensorArg(args, 0))), nil
	case "sub":
		if len(args) < 1 {
			return nil, fmt.Errorf("sub requires 1 arg")
		}
		return tensorVal(t.Sub(ltTensorArg(args, 0))), nil
	case "mul":
		if len(args) < 1 {
			return nil, fmt.Errorf("mul requires 1 arg")
		}
		return tensorVal(t.Mul(ltTensorArg(args, 0))), nil
	case "div":
		if len(args) < 1 {
			return nil, fmt.Errorf("div requires 1 arg")
		}
		return tensorVal(t.Div(ltTensorArg(args, 0))), nil
	case "matmul":
		if len(args) < 1 {
			return nil, fmt.Errorf("matmul requires 1 arg")
		}
		b := ltTensorArg(args, 0)
		if t.Dim() == 1 && b.Dim() == 1 {
			return tensorVal(t.Dot(b)), nil
		}
		if t.Dim() == 1 && b.Dim() >= 2 {
			res := t.Unsqueeze(0).MatMul(b)
			return tensorVal(res.Squeeze(0)), nil
		}
		if t.Dim() >= 2 && b.Dim() == 1 {
			res := t.MatMul(b.Unsqueeze(1))
			return tensorVal(res.Squeeze(res.Dim() - 1)), nil
		}
		return tensorVal(t.MatMul(b)), nil
	case "dot":
		if len(args) < 1 {
			return nil, fmt.Errorf("dot requires 1 arg")
		}
		return tensorVal(t.Dot(ltTensorArg(args, 0))), nil
	case "mse":
		if len(args) < 1 {
			return nil, fmt.Errorf("mse requires 1 arg")
		}
		return tensorVal(libtorch.MSELoss(t, ltTensorArg(args, 0))), nil
	case "pow":
		if len(args) < 1 {
			return nil, fmt.Errorf("pow requires 1 arg")
		}
		return tensorVal(t.Pow(ltTensorArg(args, 0))), nil
	// Shape
	case "reshape":
		if len(args) < 1 {
			return nil, fmt.Errorf("reshape requires shape")
		}
		return tensorVal(t.Reshape(ltIntListArg(args, 0)...)), nil
	case "unsqueeze":
		return tensorVal(t.Unsqueeze(ltIntArg(args, 0, 0))), nil
	case "squeeze":
		if len(args) >= 1 {
			return tensorVal(t.Squeeze(ltIntArg(args, 0, 0))), nil
		}
		return tensorVal(t.Squeeze()), nil
	case "permute":
		if len(args) < 1 {
			return nil, fmt.Errorf("permute requires dims")
		}
		return tensorVal(t.Permute(ltIntListArg(args, 0)...)), nil
	// Reductions
	case "sum":
		keepdim := false
		var axes []int
		if len(args) >= 1 {
			keepdim = ltBoolArg(args, 0, false)
		}
		if len(args) >= 2 {
			axes = ltIntListArg(args, 1)
		}
		return tensorVal(t.Sum(keepdim, axes...)), nil
	case "mean":
		keepdim := false
		var axes []int
		if len(args) >= 1 {
			keepdim = ltBoolArg(args, 0, false)
		}
		if len(args) >= 2 {
			axes = ltIntListArg(args, 1)
		}
		return tensorVal(t.Mean(keepdim, axes...)), nil
	case "max":
		keepdim := false
		var axes []int
		if len(args) >= 1 {
			keepdim = ltBoolArg(args, 0, false)
		}
		if len(args) >= 2 {
			axes = ltIntListArg(args, 1)
		}
		return tensorVal(t.Max(keepdim, axes...)), nil
	case "min":
		keepdim := false
		var axes []int
		if len(args) >= 1 {
			keepdim = ltBoolArg(args, 0, false)
		}
		if len(args) >= 2 {
			axes = ltIntListArg(args, 1)
		}
		return tensorVal(t.Min(keepdim, axes...)), nil
	// Properties
	case "shape":
		shape := t.Shape()
		elems := make([]interpreter.Value, len(shape))
		for i, d := range shape {
			elems[i] = &interpreter.IntegerValue{Val: int64(d)}
		}
		return interpreter.NewList(elems...), nil
	case "item":
		return &interpreter.FloatValue{Val: t.Item()}, nil
	case "dim":
		return &interpreter.IntegerValue{Val: int64(t.Dim())}, nil
	case "numel":
		return &interpreter.IntegerValue{Val: int64(t.Numel())}, nil
	case "requires_grad":
		if t.RequiresGrad() {
			return interpreter.TRUE, nil
		}
		return interpreter.FALSE, nil
	case "requires_grad_":
		enabled := ltBoolArg(args, 0, true)
		t.SetRequiresGrad(enabled)
		return tensorVal(t), nil
	case "to_list":
		return tensorToList(t), nil
	case "device":
		return &interpreter.StringValue{Val: string(t.Device())}, nil
	case "is_cuda":
		if t.IsCUDA() {
			return interpreter.TRUE, nil
		}
		return interpreter.FALSE, nil
	case "to":
		if len(args) < 1 {
			return nil, fmt.Errorf("to requires a device name")
		}
		targetDev := libtorch.Device(interpreter.CoerceToString(args[0]))
		res, err := t.To(targetDev)
		if err != nil {
			return nil, err
		}
		return tensorVal(res), nil
	case "cuda":
		res, err := t.CUDA()
		if err != nil {
			return nil, err
		}
		return tensorVal(res), nil
	case "cpu":
		return tensorVal(t.CPU()), nil
	default:
		return nil, fmt.Errorf("tensor has no method %q", name)
	}
}

// asTensor extracts the tensor from an opaque value.
func asTensor(v interpreter.Value) (*libtorch.Tensor, bool) {
	if tv, ok := v.(*ltTensor); ok {
		return tv.t, true
	}
	if lv, ok := v.(*interpreter.ListValue); ok {
		return ltBuildTensor(lv), true
	}
	if _, ok := v.(*interpreter.IntegerValue); ok {
		return ltBuildTensor(v), true
	}
	if _, ok := v.(*interpreter.FloatValue); ok {
		return ltBuildTensor(v), true
	}
	return nil, false
}

// ltTensorArg returns the tensor at args[i], panicking with a clear message on
// mismatch (the panic is converted to an error value by the registration shim).
func ltTensorArg(args []interpreter.Value, i int) *libtorch.Tensor {
	if i >= len(args) {
		panic(fmt.Sprintf("libtorch: missing tensor argument at position %d", i+1))
	}
	t, ok := asTensor(args[i])
	if !ok {
		panic(fmt.Sprintf("libtorch: argument %d is not a tensor", i+1))
	}
	return t
}

func ltIntArg(args []interpreter.Value, i int, def int) int {
	if i >= len(args) {
		return def
	}
	switch v := args[i].(type) {
	case *interpreter.IntegerValue:
		return int(v.Val)
	case *interpreter.FloatValue:
		return int(v.Val)
	case *interpreter.BoolValue:
		if v.Val {
			return 1
		}
		return 0
	default:
		return def
	}
}

func ltFloatArg(args []interpreter.Value, i int, def float64) float64 {
	if i >= len(args) {
		return def
	}
	switch v := args[i].(type) {
	case *interpreter.FloatValue:
		return v.Val
	case *interpreter.IntegerValue:
		return float64(v.Val)
	default:
		return def
	}
}

func ltBoolArg(args []interpreter.Value, i int, def bool) bool {
	if i >= len(args) {
		return def
	}
	if v, ok := args[i].(*interpreter.BoolValue); ok {
		return v.Val
	}
	return def
}

func ltIntListArg(args []interpreter.Value, i int) []int {
	if i >= len(args) {
		panic(fmt.Sprintf("libtorch: missing integer list argument at position %d", i+1))
	}
	if list, ok := args[i].(*interpreter.ListValue); ok {
		elems := list.ElementsSlice()
		out := make([]int, len(elems))
		for j, e := range elems {
			switch v := e.(type) {
			case *interpreter.IntegerValue:
				out[j] = int(v.Val)
			case *interpreter.FloatValue:
				out[j] = int(v.Val)
			default:
				panic(fmt.Sprintf("libtorch: list element %d is not a number", j))
			}
		}
		return out
	}
	if iv, ok := args[i].(*interpreter.IntegerValue); ok {
		out := []int{int(iv.Val)}
		for j := i + 1; j < len(args); j++ {
			if nextIv, ok := args[j].(*interpreter.IntegerValue); ok {
				out = append(out, int(nextIv.Val))
			} else {
				break
			}
		}
		return out
	}
	panic(fmt.Sprintf("libtorch: argument %d is not a list or integer", i+1))
}

func ltTensorListArg(args []interpreter.Value, i int) []*libtorch.Tensor {
	if i >= len(args) {
		panic(fmt.Sprintf("libtorch: missing tensor list argument at position %d", i+1))
	}
	list, ok := args[i].(*interpreter.ListValue)
	if !ok {
		panic(fmt.Sprintf("libtorch: argument %d is not a list", i+1))
	}
	elems := list.ElementsSlice()
	out := make([]*libtorch.Tensor, 0, len(elems))
	for _, e := range elems {
		t, ok := asTensor(e)
		if !ok {
			panic("libtorch: list contains a non-tensor element")
		}
		out = append(out, t)
	}
	return out
}

// ltBuildTensor converts an interpreter value (nested lists or a scalar) into a
// tensor.
func ltBuildTensor(v interpreter.Value) *libtorch.Tensor {
	if tv, ok := v.(*ltTensor); ok {
		return tv.t
	}
	list, ok := v.(*interpreter.ListValue)
	if !ok {
		return libtorch.Scalar(ltScalarFloat(v))
	}
	elems := list.ElementsSlice()
	if len(elems) == 0 {
		return libtorch.Zeros(0)
	}
	subShape := ltBuildTensor(elems[0]).Shape()
	var data []float64
	for i, e := range elems {
		st := ltBuildTensor(e)
		if !ltEqualShape(st.Shape(), subShape) {
			panic(fmt.Sprintf("libtorch: ragged nested list at element %d: %v vs %v", i, st.Shape(), subShape))
		}
		data = append(data, st.Data()...)
	}
	shape := append([]int{len(elems)}, subShape...)
	t, err := libtorch.NewTensor(shape, data)
	if err != nil {
		panic(err.Error())
	}
	return t
}

func ltScalarFloat(v interpreter.Value) float64 {
	switch n := v.(type) {
	case *interpreter.FloatValue:
		return n.Val
	case *interpreter.IntegerValue:
		return float64(n.Val)
	case *interpreter.BoolValue:
		if n.Val {
			return 1
		}
		return 0
	case *ltTensor:
		return n.t.Item()
	default:
		panic("libtorch: expected a number")
	}
}

func ltEqualShape(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ltNestedToValue converts a Go nested slice (float64 leaves) into interpreter
// lists.
func ltNestedToValue(n interface{}) interpreter.Value {
	switch v := n.(type) {
	case float64:
		return &interpreter.FloatValue{Val: v}
	case []interface{}:
		elems := make([]interpreter.Value, len(v))
		for i, e := range v {
			elems[i] = ltNestedToValue(e)
		}
		return interpreter.NewList(elems...)
	default:
		return interpreter.NULL
	}
}

// tensorToList converts a tensor into nested interpreter lists.
func tensorToList(t *libtorch.Tensor) interpreter.Value {
	return ltNestedToValue(t.ToNested())
}

// RegisterLibtorchBuiltins registers "std.libtorch" and "libtorch" into the
// interpreter environment.
func RegisterLibtorchBuiltins(env *interpreter.Environment) {
	mod := interpreter.NewMap()

	// reg installs a builtin that converts any panic raised by libtorch (which
	// signals shape/type errors by panicking) into a 400 error value.
	reg := func(name string, fn func(args ...interpreter.Value) interpreter.Value) {
		mod.Set(name, &interpreter.BuiltinFunction{Name: name, Fn: func(args ...interpreter.Value) (res interpreter.Value) {
			defer func() {
				if r := recover(); r != nil {
					res = interpreter.NewError(400, fmt.Sprint(r))
				}
			}()
			return fn(args...)
		}})
	}

	// ---------------------------------------------------------------------
	// Creation
	// ---------------------------------------------------------------------

	// libtorch.zeros(shape: list[int], requires_grad?: bool) -> tensor
	reg("zeros", func(args ...interpreter.Value) interpreter.Value {
		shape := ltIntListArg(args, 0)
		return tensorVal(libtorch.Zeros(shape...).WithGradIf(ltBoolArg(args, 1, false)))
	})

	// libtorch.ones(shape: list[int], requires_grad?: bool) -> tensor
	reg("ones", func(args ...interpreter.Value) interpreter.Value {
		shape := ltIntListArg(args, 0)
		return tensorVal(libtorch.Ones(shape...).WithGradIf(ltBoolArg(args, 1, false)))
	})

	// libtorch.full(shape: list[int], value: float, requires_grad?: bool) -> tensor
	reg("full", func(args ...interpreter.Value) interpreter.Value {
		shape := ltIntListArg(args, 0)
		v := ltFloatArg(args, 1, 0)
		return tensorVal(libtorch.Full(v, shape...).WithGradIf(ltBoolArg(args, 2, false)))
	})

	// libtorch.tensor(nested: list|float, requires_grad?: bool) -> tensor
	reg("tensor", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			panic("libtorch: tensor requires data")
		}
		t := ltBuildTensor(args[0])
		return tensorVal(t.WithGradIf(ltBoolArg(args, 1, false)))
	})

	// libtorch.arange(start, stop, step?, requires_grad?) -> tensor
	reg("arange", func(args ...interpreter.Value) interpreter.Value {
		start := ltFloatArg(args, 0, 0)
		stop := ltFloatArg(args, 1, 0)
		step := ltFloatArg(args, 2, 1)
		if step == 0 {
			step = 1
		}
		return tensorVal(libtorch.Arange(start, stop, step).WithGradIf(ltBoolArg(args, 3, false)))
	})

	// libtorch.eye(n: int, requires_grad?: bool) -> tensor
	reg("eye", func(args ...interpreter.Value) interpreter.Value {
		n := ltIntArg(args, 0, 0)
		return tensorVal(libtorch.Eye(n).WithGradIf(ltBoolArg(args, 1, false)))
	})

	// libtorch.rand(shape: list[int], requires_grad?: bool) -> tensor (U[0,1))
	reg("rand", func(args ...interpreter.Value) interpreter.Value {
		shape := ltIntListArg(args, 0)
		n := 1
		for _, d := range shape {
			n *= d
		}
		data := make([]float64, n)
		for i := range data {
			data[i] = rand.Float64()
		}
		t, _ := libtorch.NewTensor(shape, data)
		return tensorVal(t.WithGradIf(ltBoolArg(args, 1, false)))
	})

	// ---------------------------------------------------------------------
	// Inspection / conversion
	// ---------------------------------------------------------------------

	// libtorch.to_list(t) -> nested list
	reg("to_list", func(args ...interpreter.Value) interpreter.Value {
		return tensorToList(ltTensorArg(args, 0))
	})

	// libtorch.item(t) -> float (single-element tensors only)
	reg("item", func(args ...interpreter.Value) interpreter.Value {
		return &interpreter.FloatValue{Val: ltTensorArg(args, 0).Item()}
	})

	// libtorch.shape(t) -> list[int]
	reg("shape", func(args ...interpreter.Value) interpreter.Value {
		shape := ltTensorArg(args, 0).Shape()
		elems := make([]interpreter.Value, len(shape))
		for i, d := range shape {
			elems[i] = &interpreter.IntegerValue{Val: int64(d)}
		}
		return interpreter.NewList(elems...)
	})

	// libtorch.dim(t) -> int
	reg("dim", func(args ...interpreter.Value) interpreter.Value {
		return &interpreter.IntegerValue{Val: int64(ltTensorArg(args, 0).Dim())}
	})

	// libtorch.numel(t) -> int
	reg("numel", func(args ...interpreter.Value) interpreter.Value {
		return &interpreter.IntegerValue{Val: int64(ltTensorArg(args, 0).Numel())}
	})

	// libtorch.requires_grad(t) -> bool
	reg("requires_grad", func(args ...interpreter.Value) interpreter.Value {
		if ltTensorArg(args, 0).RequiresGrad() {
			return interpreter.TRUE
		}
		return interpreter.FALSE
	})

	// libtorch.requires_grad_(t, enabled: bool) -> tensor (in-place flag)
	reg("requires_grad_", func(args ...interpreter.Value) interpreter.Value {
		t := ltTensorArg(args, 0)
		t.SetRequiresGrad(ltBoolArg(args, 1, true))
		return tensorVal(t)
	})

	// libtorch.detach(t) -> tensor
	reg("detach", func(args ...interpreter.Value) interpreter.Value {
		return tensorVal(ltTensorArg(args, 0).Detach())
	})

	// libtorch.clone(t) -> tensor (deep copy, no autograd)
	reg("clone", func(args ...interpreter.Value) interpreter.Value {
		return tensorVal(ltTensorArg(args, 0).Clone())
	})

	// ---------------------------------------------------------------------
	// Binary ops (broadcasting + autograd)
	// ---------------------------------------------------------------------
	binop := func(name string, op func(a, b *libtorch.Tensor) *libtorch.Tensor) {
		reg(name, func(args ...interpreter.Value) interpreter.Value {
			return tensorVal(op(ltTensorArg(args, 0), ltTensorArg(args, 1)))
		})
	}
	binop("add", func(a, b *libtorch.Tensor) *libtorch.Tensor { return a.Add(b) })
	binop("sub", func(a, b *libtorch.Tensor) *libtorch.Tensor { return a.Sub(b) })
	binop("mul", func(a, b *libtorch.Tensor) *libtorch.Tensor { return a.Mul(b) })
	binop("div", func(a, b *libtorch.Tensor) *libtorch.Tensor { return a.Div(b) })
	binop("pow", func(a, b *libtorch.Tensor) *libtorch.Tensor { return a.Pow(b) })
	binop("maximum", func(a, b *libtorch.Tensor) *libtorch.Tensor { return a.Maximum(b) })
	binop("minimum", func(a, b *libtorch.Tensor) *libtorch.Tensor { return a.Minimum(b) })

	// libtorch.matmul(a, b) -> tensor (batched, autograd)
	reg("matmul", func(args ...interpreter.Value) interpreter.Value {
		a := ltTensorArg(args, 0)
		b := ltTensorArg(args, 1)
		if a.Dim() == 1 && b.Dim() == 1 {
			return tensorVal(a.Dot(b))
		}
		if a.Dim() == 1 && b.Dim() >= 2 {
			res := a.Unsqueeze(0).MatMul(b)
			return tensorVal(res.Squeeze(0))
		}
		if a.Dim() >= 2 && b.Dim() == 1 {
			res := a.MatMul(b.Unsqueeze(1))
			return tensorVal(res.Squeeze(res.Dim() - 1))
		}
		return tensorVal(a.MatMul(b))
	})

	// libtorch.dot(a, b) -> tensor (inner product of rank-1 vectors)
	reg("dot", func(args ...interpreter.Value) interpreter.Value {
		return tensorVal(ltTensorArg(args, 0).Dot(ltTensorArg(args, 1)))
	})

	// ---------------------------------------------------------------------
	// Unary ops
	// ---------------------------------------------------------------------
	unop := func(name string, op func(a *libtorch.Tensor) *libtorch.Tensor) {
		reg(name, func(args ...interpreter.Value) interpreter.Value {
			return tensorVal(op(ltTensorArg(args, 0)))
		})
	}
	unop("neg", func(a *libtorch.Tensor) *libtorch.Tensor { return a.Neg() })
	unop("exp", func(a *libtorch.Tensor) *libtorch.Tensor { return a.Exp() })
	unop("log", func(a *libtorch.Tensor) *libtorch.Tensor { return a.Log() })
	unop("sqrt", func(a *libtorch.Tensor) *libtorch.Tensor { return a.Sqrt() })
	unop("abs", func(a *libtorch.Tensor) *libtorch.Tensor { return a.Abs() })
	unop("relu", func(a *libtorch.Tensor) *libtorch.Tensor { return a.Relu() })
	unop("sigmoid", func(a *libtorch.Tensor) *libtorch.Tensor { return a.Sigmoid() })
	unop("tanh", func(a *libtorch.Tensor) *libtorch.Tensor { return a.Tanh() })

	// libtorch.leaky_relu(t, slope: float) -> tensor
	reg("leaky_relu", func(args ...interpreter.Value) interpreter.Value {
		return tensorVal(ltTensorArg(args, 0).LeakyRelu(ltFloatArg(args, 1, 0.01)))
	})

	// libtorch.softmax(t, axis?: int) -> tensor
	reg("softmax", func(args ...interpreter.Value) interpreter.Value {
		t := ltTensorArg(args, 0)
		if len(args) >= 2 {
			return tensorVal(t.Softmax(ltIntArg(args, 1, t.Dim()-1)))
		}
		return tensorVal(t.Softmax())
	})

	// libtorch.log_softmax(t, axis?: int) -> tensor
	reg("log_softmax", func(args ...interpreter.Value) interpreter.Value {
		t := ltTensorArg(args, 0)
		if len(args) >= 2 {
			return tensorVal(t.LogSoftmax(ltIntArg(args, 1, t.Dim()-1)))
		}
		return tensorVal(t.LogSoftmax())
	})

	// ---------------------------------------------------------------------
	// Reductions
	// ---------------------------------------------------------------------

	// libtorch.sum_all(t) -> tensor (rank-0)
	reg("sum_all", func(args ...interpreter.Value) interpreter.Value {
		return tensorVal(ltTensorArg(args, 0).SumAll())
	})

	// libtorch.sum(t, keepdim: bool, axes: list[int]) -> tensor
	reg("sum", func(args ...interpreter.Value) interpreter.Value {
		t := ltTensorArg(args, 0)
		keepdim := ltBoolArg(args, 1, false)
		if len(args) >= 3 {
			return tensorVal(t.Sum(keepdim, ltIntListArg(args, 2)...))
		}
		return tensorVal(t.Sum(keepdim))
	})

	// libtorch.mean(t, keepdim: bool, axes: list[int]) -> tensor
	reg("mean", func(args ...interpreter.Value) interpreter.Value {
		t := ltTensorArg(args, 0)
		keepdim := ltBoolArg(args, 1, false)
		if len(args) >= 3 {
			return tensorVal(t.Mean(keepdim, ltIntListArg(args, 2)...))
		}
		return tensorVal(t.Mean(keepdim))
	})

	// libtorch.max(t, keepdim: bool, axes: list[int]) -> tensor
	reg("max", func(args ...interpreter.Value) interpreter.Value {
		t := ltTensorArg(args, 0)
		keepdim := ltBoolArg(args, 1, false)
		if len(args) >= 3 {
			return tensorVal(t.Max(keepdim, ltIntListArg(args, 2)...))
		}
		return tensorVal(t.Max(keepdim))
	})

	// libtorch.min(t, keepdim: bool, axes: list[int]) -> tensor
	reg("min", func(args ...interpreter.Value) interpreter.Value {
		t := ltTensorArg(args, 0)
		keepdim := ltBoolArg(args, 1, false)
		if len(args) >= 3 {
			return tensorVal(t.Min(keepdim, ltIntListArg(args, 2)...))
		}
		return tensorVal(t.Min(keepdim))
	})

	// ---------------------------------------------------------------------
	// Shape ops
	// ---------------------------------------------------------------------

	// libtorch.reshape(t, shape: list[int]) -> tensor
	reg("reshape", func(args ...interpreter.Value) interpreter.Value {
		return tensorVal(ltTensorArg(args, 0).Reshape(ltIntListArg(args, 1)...))
	})

	// libtorch.flatten(t) -> tensor
	reg("flatten", func(args ...interpreter.Value) interpreter.Value {
		return tensorVal(ltTensorArg(args, 0).Flatten())
	})

	// libtorch.transpose(t) -> tensor
	reg("transpose", func(args ...interpreter.Value) interpreter.Value {
		return tensorVal(ltTensorArg(args, 0).Transpose())
	})

	// libtorch.permute(t, dims: list[int]) -> tensor
	reg("permute", func(args ...interpreter.Value) interpreter.Value {
		return tensorVal(ltTensorArg(args, 0).Permute(ltIntListArg(args, 1)...))
	})

	// libtorch.unsqueeze(t, axis: int) -> tensor
	reg("unsqueeze", func(args ...interpreter.Value) interpreter.Value {
		return tensorVal(ltTensorArg(args, 0).Unsqueeze(ltIntArg(args, 1, 0)))
	})

	// libtorch.squeeze(t, axis?: int) -> tensor
	reg("squeeze", func(args ...interpreter.Value) interpreter.Value {
		t := ltTensorArg(args, 0)
		if len(args) >= 2 {
			return tensorVal(t.Squeeze(ltIntArg(args, 1, 0)))
		}
		return tensorVal(t.Squeeze())
	})

	// libtorch.concat(axis: int, tensors: list[tensor]) -> tensor
	reg("concat", func(args ...interpreter.Value) interpreter.Value {
		return tensorVal(libtorch.Concat(ltIntArg(args, 0, 0), ltTensorListArg(args, 1)...))
	})

	// libtorch.stack(axis: int, tensors: list[tensor]) -> tensor
	reg("stack", func(args ...interpreter.Value) interpreter.Value {
		return tensorVal(libtorch.Stack(ltIntArg(args, 0, 0), ltTensorListArg(args, 1)...))
	})

	// ---------------------------------------------------------------------
	// Autograd
	// ---------------------------------------------------------------------

	// libtorch.backward(loss: tensor) -> null (runs reverse-mode autodiff)
	reg("backward", func(args ...interpreter.Value) interpreter.Value {
		ltTensorArg(args, 0).Backward()
		return interpreter.NULL
	})

	// libtorch.grad(t) -> tensor | null
	reg("grad", func(args ...interpreter.Value) interpreter.Value {
		g := ltTensorArg(args, 0).Grad()
		if g == nil {
			return interpreter.NULL
		}
		return tensorVal(g)
	})

	// libtorch.zero_grad(t) -> null
	reg("zero_grad", func(args ...interpreter.Value) interpreter.Value {
		ltTensorArg(args, 0).ZeroGrad()
		return interpreter.NULL
	})

	// ---------------------------------------------------------------------
	// Neural-network functional ops
	// ---------------------------------------------------------------------

	// libtorch.linear(x, weight, bias?) -> tensor (x @ weight^T + bias)
	reg("linear", func(args ...interpreter.Value) interpreter.Value {
		x := ltTensorArg(args, 0)
		w := ltTensorArg(args, 1)
		y := x.MatMul(w.Transpose())
		if len(args) >= 3 {
			if b, ok := asTensor(args[2]); ok {
				y = y.Add(b)
			}
		}
		return tensorVal(y)
	})

	// libtorch.conv2d(x, weight, bias?, stride: int, padding: int) -> tensor
	reg("conv2d", func(args ...interpreter.Value) interpreter.Value {
		x := ltTensorArg(args, 0)
		w := ltTensorArg(args, 1)
		var bias *libtorch.Tensor
		biasIdx := 2
		if len(args) >= 3 {
			if _, isInt := args[2].(*interpreter.IntegerValue); !isInt {
				if b, ok := asTensor(args[2]); ok {
					bias = b
					biasIdx = 3
				}
			}
		}
		stride := ltIntArg(args, biasIdx, 1)
		pad := ltIntArg(args, biasIdx+1, 0)
		return tensorVal(libtorch.Conv2d(x, w, bias, stride, stride, pad, pad))
	})

	// libtorch.max_pool2d(t, kH, kW, strideH, strideW) -> tensor
	reg("max_pool2d", func(args ...interpreter.Value) interpreter.Value {
		t := ltTensorArg(args, 0)
		kH := ltIntArg(args, 1, 2)
		kW := ltIntArg(args, 2, kH)
		sH := ltIntArg(args, 3, kH)
		sW := ltIntArg(args, 4, kW)
		return tensorVal(t.MaxPool2d(kH, kW, sH, sW))
	})

	// libtorch.layer_norm(x, gamma, beta, eps: float) -> tensor
	reg("layer_norm", func(args ...interpreter.Value) interpreter.Value {
		x := ltTensorArg(args, 0)
		gamma, _ := asTensor(args[1])
		beta, _ := asTensor(args[2])
		eps := ltFloatArg(args, 3, 1e-5)
		return tensorVal(x.LayerNorm(gamma, beta, eps))
	})

	// libtorch.batch_norm_2d(x, gamma, beta, training: bool) -> tensor
	reg("batch_norm_2d", func(args ...interpreter.Value) interpreter.Value {
		x := ltTensorArg(args, 0)
		gamma, _ := asTensor(args[1])
		beta, _ := asTensor(args[2])
		training := ltBoolArg(args, 3, true)
		stats := libtorch.NewBatchNormStats(x.Shape()[1], 0.1, 1e-5)
		return tensorVal(x.BatchNorm(gamma, beta, stats, 1, []int{0, 2, 3}, training))
	})

	// libtorch.dropout(t, p: float, training: bool) -> tensor
	reg("dropout", func(args ...interpreter.Value) interpreter.Value {
		return tensorVal(ltTensorArg(args, 0).Dropout(ltFloatArg(args, 1, 0.5), ltBoolArg(args, 2, true)))
	})

	// ---------------------------------------------------------------------
	// Losses
	// ---------------------------------------------------------------------

	// libtorch.mse_loss(pred, target) -> tensor (scalar)
	reg("mse_loss", func(args ...interpreter.Value) interpreter.Value {
		return tensorVal(libtorch.MSELoss(ltTensorArg(args, 0), ltTensorArg(args, 1)))
	})

	// libtorch.l1_loss(pred, target) -> tensor (scalar)
	reg("l1_loss", func(args ...interpreter.Value) interpreter.Value {
		return tensorVal(libtorch.L1Loss(ltTensorArg(args, 0), ltTensorArg(args, 1)))
	})

	// libtorch.cross_entropy(logits, targets) -> tensor (scalar)
	reg("cross_entropy", func(args ...interpreter.Value) interpreter.Value {
		return tensorVal(libtorch.CrossEntropyLoss(ltTensorArg(args, 0), ltTensorArg(args, 1)))
	})

	// libtorch.bce_with_logits(logits, target) -> tensor (scalar)
	reg("bce_with_logits", func(args ...interpreter.Value) interpreter.Value {
		return tensorVal(libtorch.BCEWithLogitsLoss(ltTensorArg(args, 0), ltTensorArg(args, 1)))
	})

	// ---------------------------------------------------------------------
	// Optimizers
	// ---------------------------------------------------------------------

	// libtorch.sgd(params: list[tensor], lr, momentum?, weight_decay?) -> optimizer
	reg("sgd", func(args ...interpreter.Value) interpreter.Value {
		params := ltTensorListArg(args, 0)
		lr := ltFloatArg(args, 1, 0.01)
		return &ltOpt{o: libtorch.NewSGD(params, lr, ltFloatArg(args, 2, 0), ltFloatArg(args, 3, 0))}
	})

	// libtorch.adam(params: list[tensor], lr, weight_decay?) -> optimizer
	reg("adam", func(args ...interpreter.Value) interpreter.Value {
		params := ltTensorListArg(args, 0)
		lr := ltFloatArg(args, 1, 0.001)
		opt := libtorch.NewAdam(params, lr, false)
		opt.SetWeightDecay(ltFloatArg(args, 2, 0))
		return &ltOpt{o: opt}
	})

	// libtorch.adamw(params: list[tensor], lr, weight_decay?) -> optimizer
	reg("adamw", func(args ...interpreter.Value) interpreter.Value {
		params := ltTensorListArg(args, 0)
		lr := ltFloatArg(args, 1, 0.001)
		opt := libtorch.NewAdam(params, lr, true)
		opt.SetWeightDecay(ltFloatArg(args, 2, 0))
		return &ltOpt{o: opt}
	})

	// libtorch.rmsprop(params: list[tensor], lr, rho?, eps?, momentum?, weight_decay?) -> optimizer
	reg("rmsprop", func(args ...interpreter.Value) interpreter.Value {
		params := ltTensorListArg(args, 0)
		lr := ltFloatArg(args, 1, 0.01)
		return &ltOpt{o: libtorch.NewRMSprop(params, lr, ltFloatArg(args, 2, 0.9), ltFloatArg(args, 3, 1e-8), ltFloatArg(args, 4, 0), ltFloatArg(args, 5, 0))}
	})

	// libtorch.opt_step(opt) -> null
	reg("opt_step", func(args ...interpreter.Value) interpreter.Value {
		optArg(args, 0).Step()
		return interpreter.NULL
	})

	// libtorch.opt_zero_grad(opt) -> null
	reg("opt_zero_grad", func(args ...interpreter.Value) interpreter.Value {
		optArg(args, 0).ZeroGrad()
		return interpreter.NULL
	})

	// ---------------------------------------------------------------------
	// Serialization
	// ---------------------------------------------------------------------

	// libtorch.save(path: string, tensors: map[string, tensor]) -> bool
	reg("save", func(args ...interpreter.Value) interpreter.Value {
		path := interpreter.CoerceToString(args[0])
		m := args[1].(*interpreter.MapValue)
		dict := make(map[string]*libtorch.Tensor)
		for _, key := range m.Keys() {
			val, _ := m.Get(key)
			if t, ok := asTensor(val); ok {
				dict[key] = t
			}
		}
		if err := libtorch.Save(path, dict); err != nil {
			return interpreter.NewError(500, err.Error())
		}
		return interpreter.TRUE
	})

	// libtorch.load(path: string) -> map[string, tensor]
	reg("load", func(args ...interpreter.Value) interpreter.Value {
		path := interpreter.CoerceToString(args[0])
		dict, err := libtorch.Load(path)
		if err != nil {
			return interpreter.NewError(500, err.Error())
		}
		out := interpreter.NewMap()
		for name, t := range dict {
			out.Set(name, tensorVal(t))
		}
		return out
	})

	// ---------------------------------------------------------------------
	// Extended activations (new)
	// ---------------------------------------------------------------------
	unop("gelu", func(a *libtorch.Tensor) *libtorch.Tensor { return a.GELU(false) })
	unop("gelu_approx", func(a *libtorch.Tensor) *libtorch.Tensor { return a.GELU(true) })
	unop("elu", func(a *libtorch.Tensor) *libtorch.Tensor { return a.ELU(1) })
	unop("selu", func(a *libtorch.Tensor) *libtorch.Tensor { return a.SELU() })
	unop("silu", func(a *libtorch.Tensor) *libtorch.Tensor { return a.SiLU() })
	unop("swish", func(a *libtorch.Tensor) *libtorch.Tensor { return a.Swish() })
	unop("softplus", func(a *libtorch.Tensor) *libtorch.Tensor { return a.Softplus() })
	unop("mish", func(a *libtorch.Tensor) *libtorch.Tensor { return a.Mish() })
	unop("hardtanh", func(a *libtorch.Tensor) *libtorch.Tensor { return a.Hardtanh(-1, 1) })
	unop("relu6", func(a *libtorch.Tensor) *libtorch.Tensor { return a.ReLU6() })
	unop("hardswish", func(a *libtorch.Tensor) *libtorch.Tensor { return a.Hardswish() })
	unop("hardsigmoid", func(a *libtorch.Tensor) *libtorch.Tensor { return a.HardSigmoid() })
	unop("softsign", func(a *libtorch.Tensor) *libtorch.Tensor { return a.Softsign() })
	unop("erf", func(a *libtorch.Tensor) *libtorch.Tensor { return a.Erf() })
	unop("frac", func(a *libtorch.Tensor) *libtorch.Tensor { return a.Frac() })
	unop("round", func(a *libtorch.Tensor) *libtorch.Tensor { return a.Round() })
	unop("floor", func(a *libtorch.Tensor) *libtorch.Tensor { return a.Floor() })
	unop("ceil", func(a *libtorch.Tensor) *libtorch.Tensor { return a.Ceil() })
	unop("sign", func(a *libtorch.Tensor) *libtorch.Tensor { return a.Sign() })
	unop("abs", func(a *libtorch.Tensor) *libtorch.Tensor { return a.Abs() })

	// ---------------------------------------------------------------------
	// Extended utility ops
	// ---------------------------------------------------------------------
	reg("clamp", func(args ...interpreter.Value) interpreter.Value {
		t := ltTensorArg(args, 0)
		mn := ltFloatArg(args, 1, math.Inf(-1))
		mx := ltFloatArg(args, 2, math.Inf(1))
		return tensorVal(t.Clamp(mn, mx))
	})
	reg("cumsum", func(args ...interpreter.Value) interpreter.Value {
		t := ltTensorArg(args, 0)
		axis := ltIntArg(args, 1, t.Dim()-1)
		return tensorVal(t.CumSum(axis))
	})
	reg("argmax", func(args ...interpreter.Value) interpreter.Value {
		t := ltTensorArg(args, 0)
		keepdim := ltBoolArg(args, 1, false)
		var axes []int
		if len(args) >= 3 {
			axes = ltIntListArg(args, 2)
		}
		return tensorVal(t.Argmax(keepdim, axes...))
	})
	reg("argmin", func(args ...interpreter.Value) interpreter.Value {
		t := ltTensorArg(args, 0)
		keepdim := ltBoolArg(args, 1, false)
		var axes []int
		if len(args) >= 3 {
			axes = ltIntListArg(args, 2)
		}
		return tensorVal(t.Argmin(keepdim, axes...))
	})
	reg("masked_fill", func(args ...interpreter.Value) interpreter.Value {
		t := ltTensorArg(args, 0)
		m := ltTensorArg(args, 1)
		v := ltFloatArg(args, 2, 0)
		return tensorVal(t.MaskedFill(m, v))
	})
	reg("where", func(args ...interpreter.Value) interpreter.Value {
		return tensorVal(libtorch.Where(ltTensorArg(args, 0), ltTensorArg(args, 1), ltTensorArg(args, 2)))
	})
	reg("gather", func(args ...interpreter.Value) interpreter.Value {
		t := ltTensorArg(args, 0)
		dim := ltIntArg(args, 1, 0)
		idx := ltTensorArg(args, 2)
		return tensorVal(t.Gather(dim, idx))
	})
	reg("index_select", func(args ...interpreter.Value) interpreter.Value {
		t := ltTensorArg(args, 0)
		axis := ltIntArg(args, 1, 0)
		idx := ltIntListArg(args, 2)
		return tensorVal(t.IndexSelect(axis, idx))
	})
	reg("trace", func(args ...interpreter.Value) interpreter.Value {
		return tensorVal(ltTensorArg(args, 0).Trace())
	})
	reg("logsumexp", func(args ...interpreter.Value) interpreter.Value {
		t := ltTensorArg(args, 0)
		keepdim := ltBoolArg(args, 1, false)
		var axes []int
		if len(args) >= 3 {
			axes = ltIntListArg(args, 2)
		}
		return tensorVal(t.LogSumExp(keepdim, axes...))
	})
	reg("lp_norm", func(args ...interpreter.Value) interpreter.Value {
		t := ltTensorArg(args, 0)
		p := ltFloatArg(args, 1, 2)
		keepdim := ltBoolArg(args, 2, false)
		var axes []int
		if len(args) >= 4 {
			axes = ltIntListArg(args, 3)
		}
		return tensorVal(t.LpNorm(p, keepdim, axes...))
	})
	reg("prod", func(args ...interpreter.Value) interpreter.Value {
		t := ltTensorArg(args, 0)
		keepdim := ltBoolArg(args, 1, false)
		var axes []int
		if len(args) >= 3 {
			axes = ltIntListArg(args, 2)
		}
		return tensorVal(t.Prod(keepdim, axes...))
	})

	// ---------------------------------------------------------------------
	// Extended losses
	// ---------------------------------------------------------------------
	reg("smooth_l1_loss", func(args ...interpreter.Value) interpreter.Value {
		return tensorVal(libtorch.SmoothL1Loss(ltTensorArg(args, 0), ltTensorArg(args, 1)))
	})
	reg("kl_div_loss", func(args ...interpreter.Value) interpreter.Value {
		return tensorVal(libtorch.KLDivLoss(ltTensorArg(args, 0), ltTensorArg(args, 1)))
	})
	reg("poisson_nll_loss", func(args ...interpreter.Value) interpreter.Value {
		return tensorVal(libtorch.PoissonNLLLoss(ltTensorArg(args, 0), ltTensorArg(args, 1)))
	})
	reg("soft_margin_loss", func(args ...interpreter.Value) interpreter.Value {
		return tensorVal(libtorch.SoftMarginLoss(ltTensorArg(args, 0), ltTensorArg(args, 1)))
	})

	// ---------------------------------------------------------------------
	// Extended nn / attention
	// ---------------------------------------------------------------------
	reg("conv_transpose2d", func(args ...interpreter.Value) interpreter.Value {
		x := ltTensorArg(args, 0)
		w := ltTensorArg(args, 1)
		var bias *libtorch.Tensor
		biasIdx := 2
		if len(args) >= 3 {
			if _, isInt := args[2].(*interpreter.IntegerValue); !isInt {
				if b, ok := asTensor(args[2]); ok {
					bias = b
					biasIdx = 3
				}
			}
		}
		stride := ltIntArg(args, biasIdx, 1)
		pad := ltIntArg(args, biasIdx+1, 0)
		outPad := ltIntArg(args, biasIdx+2, 0)
		return tensorVal(libtorch.ConvTranspose2d(x, w, bias, stride, stride, pad, pad, outPad, outPad))
	})
	reg("upsample_nearest", func(args ...interpreter.Value) interpreter.Value {
		t := ltTensorArg(args, 0)
		scale := ltIntArg(args, 1, 2)
		return tensorVal(t.UpsampleNearest2d(scale, scale))
	})
	reg("group_norm", func(args ...interpreter.Value) interpreter.Value {
		x := ltTensorArg(args, 0)
		gamma, _ := asTensor(args[1])
		beta, _ := asTensor(args[2])
		numGroups := ltIntArg(args, 3, 1)
		eps := ltFloatArg(args, 4, 1e-5)
		return tensorVal(x.GroupNorm(gamma, beta, numGroups, eps))
	})
	reg("instance_norm_2d", func(args ...interpreter.Value) interpreter.Value {
		x := ltTensorArg(args, 0)
		gamma, _ := asTensor(args[1])
		beta, _ := asTensor(args[2])
		eps := ltFloatArg(args, 3, 1e-5)
		return tensorVal(x.InstanceNorm2d(gamma, beta, eps))
	})
	reg("causal_mask", func(args ...interpreter.Value) interpreter.Value {
		n := ltIntArg(args, 0, 1)
		return tensorVal(libtorch.CausalMask(n))
	})
	reg("scaled_dot_product_attention", func(args ...interpreter.Value) interpreter.Value {
		q := ltTensorArg(args, 0)
		k := ltTensorArg(args, 1)
		v := ltTensorArg(args, 2)
		var mask *libtorch.Tensor
		if len(args) >= 4 {
			if m, ok := asTensor(args[3]); ok {
				mask = m
			}
		}
		dropout := ltFloatArg(args, 4, 0)
		training := ltBoolArg(args, 5, false)
		out, _ := libtorch.ScaledDotProductAttention(q, k, v, mask, dropout, training)
		return tensorVal(out)
	})

	// ---------------------------------------------------------------------
	// Extended optimizers
	// ---------------------------------------------------------------------
	reg("adagrad", func(args ...interpreter.Value) interpreter.Value {
		params := ltTensorListArg(args, 0)
		lr := ltFloatArg(args, 1, 0.01)
		return &ltOpt{o: libtorch.NewAdaGrad(params, lr, ltFloatArg(args, 2, 1e-10), ltFloatArg(args, 3, 0))}
	})
	reg("adadelta", func(args ...interpreter.Value) interpreter.Value {
		params := ltTensorListArg(args, 0)
		return &ltOpt{o: libtorch.NewAdaDelta(params, ltFloatArg(args, 1, 0.9), ltFloatArg(args, 2, 1e-6), ltFloatArg(args, 3, 0))}
	})
	reg("nadam", func(args ...interpreter.Value) interpreter.Value {
		params := ltTensorListArg(args, 0)
		lr := ltFloatArg(args, 1, 0.002)
		opt := libtorch.NewNAdam(params, lr, ltBoolArg(args, 3, false))
		opt.SetWeightDecay(ltFloatArg(args, 2, 0))
		return &ltOpt{o: opt}
	})

	// ---------------------------------------------------------------------
	// Training utilities
	// ---------------------------------------------------------------------
	reg("clip_grad_norm", func(args ...interpreter.Value) interpreter.Value {
		params := ltTensorListArg(args, 0)
		maxNorm := ltFloatArg(args, 1, 1)
		n := libtorch.ClipGradNorm(params, maxNorm)
		return &interpreter.FloatValue{Val: n}
	})
	reg("clip_grad_value", func(args ...interpreter.Value) interpreter.Value {
		params := ltTensorListArg(args, 0)
		v := ltFloatArg(args, 1, 1)
		libtorch.ClipGradValue(params, v)
		return interpreter.NULL
	})

	// libtorch.step_lr(opt, epoch, step_size, gamma) -> null
	reg("step_lr", func(args ...interpreter.Value) interpreter.Value {
		opt := optArg(args, 0)
		epoch := ltIntArg(args, 1, 0)
		stepSize := ltIntArg(args, 2, 1)
		gamma := ltFloatArg(args, 3, 0.1)
		libtorch.StepLR(opt, epoch, stepSize, gamma)
		return interpreter.NULL
	})

	// libtorch.cosine_annealing_lr(opt, epoch, t_max, eta_min, base_lr) -> null
	reg("cosine_annealing_lr", func(args ...interpreter.Value) interpreter.Value {
		opt := optArg(args, 0)
		epoch := ltIntArg(args, 1, 0)
		tMax := ltIntArg(args, 2, 10)
		etaMin := ltFloatArg(args, 3, 0)
		baseLR := ltFloatArg(args, 4, 0.01)
		libtorch.CosineAnnealingLR(opt, epoch, tMax, etaMin, baseLR)
		return interpreter.NULL
	})

	// libtorch.reduce_lr_on_plateau(opt, metric, best, counter, patience, factor) -> bool
	reg("reduce_lr_on_plateau", func(args ...interpreter.Value) interpreter.Value {
		opt := optArg(args, 0)
		metric := ltFloatArg(args, 1, 0)
		// best and counter are passed as single-element lists (mutable via reference)
		best := ltFloatArg(args, 2, 0)
		counter := ltIntArg(args, 3, 0)
		patience := ltIntArg(args, 4, 5)
		factor := ltFloatArg(args, 5, 0.5)
		bestPtr := &best
		counterPtr := &counter
		reduced := libtorch.ReduceLROnPlateau(opt, metric, bestPtr, counterPtr, patience, factor)
		if reduced {
			return interpreter.TRUE
		}
		return interpreter.FALSE
	})

	// libtorch.matrix(rows: int, cols: int, init?: float|string) -> tensor
	reg("matrix", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			panic("libtorch: matrix requires rows and cols")
		}
		rows := ltIntArg(args, 0, 0)
		cols := ltIntArg(args, 1, 0)
		if rows <= 0 || cols <= 0 {
			panic("libtorch: rows and cols must be positive integers")
		}
		if len(args) >= 3 {
			switch v := args[2].(type) {
			case *interpreter.FloatValue:
				return tensorVal(libtorch.Full(v.Val, rows, cols))
			case *interpreter.IntegerValue:
				return tensorVal(libtorch.Full(float64(v.Val), rows, cols))
			case *interpreter.StringValue:
				switch strings.ToLower(v.Val) {
				case "ones":
					return tensorVal(libtorch.Ones(rows, cols))
				case "random", "rand":
					data := make([]float64, rows*cols)
					for i := range data {
						data[i] = rand.Float64()*2.0 - 1.0 // [-1.0, 1.0]
					}
					t, _ := libtorch.NewTensor([]int{rows, cols}, data)
					return tensorVal(t)
				case "he":
					std := math.Sqrt(2.0 / float64(rows))
					data := make([]float64, rows*cols)
					for i := range data {
						data[i] = rand.NormFloat64() * std
					}
					t, _ := libtorch.NewTensor([]int{rows, cols}, data)
					return tensorVal(t)
				case "xavier", "glorot":
					limit := math.Sqrt(6.0 / float64(rows+cols))
					data := make([]float64, rows*cols)
					for i := range data {
						data[i] = (rand.Float64()*2.0 - 1.0) * limit
					}
					t, _ := libtorch.NewTensor([]int{rows, cols}, data)
					return tensorVal(t)
				case "zeros":
					return tensorVal(libtorch.Zeros(rows, cols))
				}
			}
		}
		return tensorVal(libtorch.Zeros(rows, cols))
	})

	// libtorch.dense_forward(inputs, weights, biases, act?: string) -> tensor
	reg("dense_forward", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 3 {
			panic("libtorch: dense_forward requires inputs, weights, and biases")
		}
		inputs := ltTensorArg(args, 0)
		weights := ltTensorArg(args, 1)
		biases := ltTensorArg(args, 2)
		act := "linear"
		if len(args) >= 4 {
			act = strings.ToLower(interpreter.CoerceToString(args[3]))
		}
		squeezeBatch := false
		if inputs.Dim() == 1 {
			inputs = inputs.Unsqueeze(0)
			squeezeBatch = true
		}
		if weights.Dim() == 1 {
			weights = weights.Unsqueeze(1)
		}
		out := inputs.MatMul(weights).Add(biases)
		switch act {
		case "relu":
			out = out.Relu()
		case "sigmoid":
			out = out.Sigmoid()
		case "tanh":
			out = out.Tanh()
		case "gelu":
			out = out.GELU(false)
		case "silu":
			out = out.SiLU()
		}
		if squeezeBatch {
			out = out.Squeeze(0)
		}
		return tensorVal(out)
	})

	// libtorch.mse(pred, target) -> tensor
	reg("mse", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			panic("libtorch: mse requires pred and target")
		}
		return tensorVal(libtorch.MSELoss(ltTensorArg(args, 0), ltTensorArg(args, 1)))
	})

	// ---------------------------------------------------------------------
	// CUDA / Device management
	// ---------------------------------------------------------------------
	reg("cuda_is_available", func(args ...interpreter.Value) interpreter.Value {
		if libtorch.CUDAAvailable() {
			return interpreter.TRUE
		}
		return interpreter.FALSE
	})
	reg("cuda_device_count", func(args ...interpreter.Value) interpreter.Value {
		return &interpreter.IntegerValue{Val: int64(libtorch.CUDADeviceCount())}
	})
	reg("cuda_device_name", func(args ...interpreter.Value) interpreter.Value {
		idx := ltIntArg(args, 0, 0)
		name, err := libtorch.CUDADeviceName(idx)
		if err != nil {
			panic(err.Error())
		}
		return &interpreter.StringValue{Val: name}
	})
	reg("cuda_memory_allocated", func(args ...interpreter.Value) interpreter.Value {
		return &interpreter.IntegerValue{Val: libtorch.CUDAMemoryAllocated()}
	})
	reg("cuda_synchronize", func(args ...interpreter.Value) interpreter.Value {
		if err := libtorch.CUDASynchronize(); err != nil {
			panic(err.Error())
		}
		return interpreter.NULL
	})
	reg("device", func(args ...interpreter.Value) interpreter.Value {
		return &interpreter.StringValue{Val: string(ltTensorArg(args, 0).Device())}
	})
	reg("is_cuda", func(args ...interpreter.Value) interpreter.Value {
		if ltTensorArg(args, 0).IsCUDA() {
			return interpreter.TRUE
		}
		return interpreter.FALSE
	})
	reg("to", func(args ...interpreter.Value) interpreter.Value {
		t := ltTensorArg(args, 0)
		if len(args) < 2 {
			panic("libtorch: to requires a device target")
		}
		targetDev := libtorch.Device(interpreter.CoerceToString(args[1]))
		res, err := t.To(targetDev)
		if err != nil {
			panic(err.Error())
		}
		return tensorVal(res)
	})
	reg("cuda", func(args ...interpreter.Value) interpreter.Value {
		t := ltTensorArg(args, 0)
		res, err := t.CUDA()
		if err != nil {
			panic(err.Error())
		}
		return tensorVal(res)
	})
	reg("cpu", func(args ...interpreter.Value) interpreter.Value {
		return tensorVal(ltTensorArg(args, 0).CPU())
	})

	env.Set("libtorch", mod)
	env.Set("std.libtorch", mod)
}

// optArg extracts an optimizer opaque value at args[i].
func optArg(args []interpreter.Value, i int) libtorch.Optimizer {
	if i >= len(args) {
		panic("libtorch: missing optimizer argument")
	}
	ov, ok := args[i].(*ltOpt)
	if !ok {
		panic("libtorch: argument is not an optimizer")
	}
	return ov.o
}
