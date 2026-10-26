// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

// Package libtorch provides a pure-Go N-dimensional tensor with reverse-mode
// automatic differentiation (autograd), broadcasting, and a set of neural
// network primitives (ops, layers, optimizers, losses, serialization).
//
// It is deliberately a foundational subset of PyTorch: contiguous float64
// tensors, an eager autograd graph, and CPU-only naive implementations. There
// is no GPU, no pretrained-model hub, and no C++/libtorch FFI. The design
// favors clarity and correctness over peak performance.
package libtorch

import (
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"
)

// Tensor is a contiguous, row-major float64 array with an optional autograd
// relationship to the tensors it was derived from.
//
// A Tensor is safe to read concurrently. It is NOT safe to mutate concurrently
// with reads or other mutations.
type Tensor struct {
	data  []float64
	shape []int

	// Device placement.
	device Device
	gpuPtr uintptr

	// Autograd bookkeeping.
	requiresGrad bool
	grad         *Tensor
	parents      []*Tensor
	gradFn       func()
	// nonLeaf marks tensors produced by operations that participate in autograd
	// but are not themselves leaves; their gradFn accumulates into parents.
	// (Presently unused externally, retained for clarity in graph traversal.)
}

// -----------------------------------------------------------------------------
// Constructors
// -----------------------------------------------------------------------------

// NewTensor builds a tensor from flat row-major data and a shape. The data slice
// is adopted (not copied); callers that need to retain ownership should Clone.
func NewTensor(shape []int, data []float64) (*Tensor, error) {
	n := product(shape)
	if n != len(data) {
		return nil, fmt.Errorf("libtorch: shape %v has %d elements but data has %d", shape, n, len(data))
	}
	sh := make([]int, len(shape))
	copy(sh, shape)
	return &Tensor{data: data, shape: sh}, nil
}

// Zeros returns a tensor of the given shape filled with 0.
func Zeros(shape ...int) *Tensor {
	return &Tensor{data: make([]float64, product(shape)), shape: cloneInts(shape)}
}

// Ones returns a tensor of the given shape filled with 1.
func Ones(shape ...int) *Tensor {
	t := Zeros(shape...)
	for i := range t.data {
		t.data[i] = 1
	}
	return t
}

// Full returns a tensor of the given shape filled with v.
func Full(v float64, shape ...int) *Tensor {
	t := Zeros(shape...)
	for i := range t.data {
		t.data[i] = v
	}
	return t
}

// Rand returns a tensor of the given shape filled with random values from U[0, 1).
func Rand(shape ...int) *Tensor {
	n := product(shape)
	data := make([]float64, n)
	for i := range data {
		data[i] = rand.Float64()
	}
	return &Tensor{data: data, shape: cloneInts(shape)}
}

// Randn returns a tensor of the given shape filled with random values from N(0, 1).
func Randn(shape ...int) *Tensor {
	n := product(shape)
	data := make([]float64, n)
	for i := range data {
		data[i] = rand.NormFloat64()
	}
	return &Tensor{data: data, shape: cloneInts(shape)}
}

// Scalar returns a rank-0 (0-dimensional) tensor holding v.
func Scalar(v float64) *Tensor {
	return &Tensor{data: []float64{v}, shape: []int{}}
}

// FromFlat returns a tensor adopting the given row-major data with the given
// shape. It errors when the element counts disagree.
func FromFlat(data []float64, shape ...int) (*Tensor, error) {
	return NewTensor(shape, data)
}

// Arange returns a 1-D tensor of values in [start, stop) with the given step.
func Arange(start, stop, step float64) *Tensor {
	if step == 0 {
		panic("libtorch: Arange step must be non-zero")
	}
	n := int(math.Ceil((stop - start) / step))
	if n < 0 {
		n = 0
	}
	data := make([]float64, n)
	for i := 0; i < n; i++ {
		data[i] = start + float64(i)*step
	}
	return &Tensor{data: data, shape: []int{n}}
}

// LinSpace returns a 1-D tensor of num points between start and end inclusive.
func LinSpace(start, end float64, num int) *Tensor {
	if num <= 0 {
		return Zeros(0)
	}
	if num == 1 {
		return &Tensor{data: []float64{start}, shape: []int{1}}
	}
	step := (end - start) / float64(num-1)
	data := make([]float64, num)
	for i := 0; i < num; i++ {
		data[i] = start + float64(i)*step
	}
	return &Tensor{data: data, shape: []int{num}}
}

// Eye returns an n x n identity matrix.
func Eye(n int) *Tensor {
	t := Zeros(n, n)
	for i := 0; i < n; i++ {
		t.data[i*n+i] = 1
	}
	return t
}

// -----------------------------------------------------------------------------
// Accessors
// -----------------------------------------------------------------------------

// Shape returns a copy of the tensor's shape.
func (t *Tensor) Shape() []int { return cloneInts(t.shape) }

// Dim returns the number of dimensions (rank).
func (t *Tensor) Dim() int { return len(t.shape) }

// Numel returns the total number of elements.
func (t *Tensor) Numel() int { return len(t.data) }

// Data returns a copy of the flat row-major element slice.
func (t *Tensor) Data() []float64 { return slicesClone(t.data) }

// Item returns the single element of a rank-0 or single-element tensor. It
// panics if the tensor does not hold exactly one element.
func (t *Tensor) Item() float64 {
	if len(t.data) != 1 {
		panic(fmt.Sprintf("libtorch: Item() requires exactly one element, tensor has %d", len(t.data)))
	}
	return t.data[0]
}

// ScalarValue returns the value of a rank-0 tensor, aliasing Item.
func (t *Tensor) ScalarValue() float64 { return t.Item() }

// At returns the element at the given multi-dimensional index.
func (t *Tensor) At(idx ...int) float64 {
	return t.data[t.flatIndex(idx)]
}

// SetAt writes v at the given multi-dimensional index.
func (t *Tensor) SetAt(v float64, idx ...int) {
	t.data[t.flatIndex(idx)] = v
}

func (t *Tensor) flatIndex(idx []int) int {
	if len(idx) != len(t.shape) {
		panic(fmt.Sprintf("libtorch: index %v does not match shape %v", idx, t.shape))
	}
	flat := 0
	for i, d := range idx {
		if d < 0 || d >= t.shape[i] {
			panic(fmt.Sprintf("libtorch: index %v out of range for shape %v", idx, t.shape))
		}
		flat = flat*t.shape[i] + d
	}
	return flat
}

// Clone returns a deep copy of the tensor data and shape, without autograd state.
func (t *Tensor) Clone() *Tensor {
	return &Tensor{data: slicesClone(t.data), shape: cloneInts(t.shape), device: t.device}
}

// Detach returns a tensor sharing the same data but severed from the autograd
// graph (requiresGrad=false, no grad, no parents).
func (t *Tensor) Detach() *Tensor {
	return &Tensor{data: t.data, shape: cloneInts(t.shape), device: t.device}
}

// RequiresGrad reports whether this tensor tracks gradients.
func (t *Tensor) RequiresGrad() bool { return t.requiresGrad }

// Grad returns the accumulated gradient tensor, or nil if none exists.
func (t *Tensor) Grad() *Tensor { return t.grad }

// SetRequiresGrad enables or disables gradient tracking for this leaf tensor.
// It panics on non-leaf tensors (those produced by operations).
func (t *Tensor) SetRequiresGrad(v bool) *Tensor {
	if len(t.parents) > 0 || t.gradFn != nil {
		panic("libtorch: SetRequiresGrad may only be called on leaf tensors")
	}
	t.requiresGrad = v
	return t
}

// WithGrad is a fluent alias for SetRequiresGrad that returns the tensor.
func (t *Tensor) WithGrad() *Tensor { return t.SetRequiresGrad(true) }

// WithGradIf enables gradient tracking only when v is true, returning the tensor.
func (t *Tensor) WithGradIf(v bool) *Tensor {
	if v {
		return t.SetRequiresGrad(true)
	}
	return t
}

// ZeroGrad clears the accumulated gradient on this tensor.
func (t *Tensor) ZeroGrad() { t.grad = nil }

// -----------------------------------------------------------------------------
// Shape manipulation
// -----------------------------------------------------------------------------

// Reshape returns a tensor with the same flat data and a new shape. The result
// shares the backing array. newShape may contain at most one -1, which is
// inferred from the remaining dimensions and the total element count.
func (t *Tensor) Reshape(newShape ...int) *Tensor {
	shape := inferShape(newShape, len(t.data))
	if product(shape) != len(t.data) {
		panic(fmt.Sprintf("libtorch: cannot reshape %v (n=%d) into %v (n=%d)", t.shape, len(t.data), shape, product(shape)))
	}
	out := &Tensor{data: t.data, shape: shape}
	if t.requiresGrad || t.gradFn != nil {
		out.attachReshape(t)
	}
	return out
}

// Flatten collapses all dimensions into a single 1-D dimension.
func (t *Tensor) Flatten() *Tensor { return t.Reshape(len(t.data)) }

// Transpose returns a view with the last two dimensions swapped. For rank-2
// tensors this is the standard matrix transpose; for higher rank it swaps the
// final two axes. It requires the tensor to be at least rank 2.
func (t *Tensor) Transpose() *Tensor {
	d := t.Dim()
	if d < 2 {
		panic("libtorch: Transpose requires a tensor of rank >= 2")
	}
	return t.Permute(swapLastTwo(permIdentity(d))...)
}

// T is an alias for Transpose.
func (t *Tensor) T() *Tensor { return t.Transpose() }

// Permute returns a copy with its dimensions rearranged according to dims, a
// permutation of 0..Dim()-1.
func (t *Tensor) Permute(dims ...int) *Tensor {
	d := t.Dim()
	if len(dims) != d {
		panic(fmt.Sprintf("libtorch: Permute expects %d dims, got %d", d, len(dims)))
	}
	seen := make([]bool, d)
	outShape := make([]int, d)
	for _, ax := range dims {
		if ax < 0 || ax >= d || seen[ax] {
			panic(fmt.Sprintf("libtorch: Permute dims %v is not a permutation of 0..%d", dims, d-1))
		}
		seen[ax] = true
		outShape[ax] = 0 // placeholder
	}
	// outShape[i] = t.shape[dims[i]]
	for i := range dims {
		outShape[i] = t.shape[dims[i]]
	}
	out := Zeros(outShape...)
	// Build inverse permutation to map output index back to input index.
	inverse := make([]int, d)
	for i, ax := range dims {
		inverse[ax] = i
	}
	idx := make([]int, d)
	outIdx := make([]int, d)
	iterate(outShape, func(flat int, cur []int) {
		// cur is index in output space; map to input space via inverse.
		for i := 0; i < d; i++ {
			outIdx[i] = cur[inverse[i]]
		}
		_ = idx
		out.data[flat] = t.At(outIdx...)
	})
	if t.requiresGrad || t.gradFn != nil {
		out.attachPermute(t, dims)
	}
	return out
}

// Squeeze removes all size-1 dimensions, or a specific axis if given.
func (t *Tensor) Squeeze(axis ...int) *Tensor {
	if len(axis) == 0 {
		var ns []int
		for _, s := range t.shape {
			if s != 1 {
				ns = append(ns, s)
			}
		}
		return t.Reshape(ns...)
	}
	ax := axis[0]
	if ax < 0 {
		ax += t.Dim()
	}
	if ax < 0 || ax >= t.Dim() || t.shape[ax] != 1 {
		panic(fmt.Sprintf("libtorch: Squeeze axis %d is not a size-1 dimension of %v", axis[0], t.shape))
	}
	ns := make([]int, 0, t.Dim()-1)
	for i, s := range t.shape {
		if i != ax {
			ns = append(ns, s)
		}
	}
	return t.Reshape(ns...)
}

// Unsqueeze inserts a size-1 dimension at the given axis.
func (t *Tensor) Unsqueeze(axis int) *Tensor {
	if axis < 0 {
		axis += t.Dim() + 1
	}
	if axis < 0 || axis > t.Dim() {
		panic(fmt.Sprintf("libtorch: Unsqueeze axis %d out of range for shape %v", axis, t.shape))
	}
	ns := make([]int, 0, t.Dim()+1)
	ns = append(ns, t.shape[:axis]...)
	ns = append(ns, 1)
	ns = append(ns, t.shape[axis:]...)
	return t.Reshape(ns...)
}

// Contiguous returns the tensor itself; all libtorch tensors are contiguous.
func (t *Tensor) Contiguous() *Tensor { return t }

// -----------------------------------------------------------------------------
// String / inspection
// -----------------------------------------------------------------------------

// String renders the tensor in a compact nested-list form.
func (t *Tensor) String() string {
	if t.Dim() == 0 {
		return "tensor(" + formatFloat(t.data[0]) + ")"
	}
	var b strings.Builder
	b.WriteString("tensor(")
	writeNested(&b, t.data, t.shape, 0, 0)
	b.WriteString(")")
	return b.String()
}

// Inspect mirrors String for debugging.
func (t *Tensor) Inspect() string { return t.String() }

func writeNested(b *strings.Builder, data []float64, shape []int, dim, off int) {
	if dim == len(shape) {
		b.WriteString(formatFloat(data[off]))
		return
	}
	b.WriteString("[")
	stride := product(shape[dim+1:])
	for i := 0; i < shape[dim]; i++ {
		if i > 0 {
			b.WriteString(", ")
		}
		writeNested(b, data, shape, dim+1, off+i*stride)
	}
	b.WriteString("]")
}

func formatFloat(v float64) string {
	if v == math.Trunc(v) && math.Abs(v) < 1e15 {
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	return strconv.FormatFloat(v, 'g', -1, 64)
}

// ToNested converts the tensor into nested Go slices (float64 leaves), suitable
// for handing to the interpreter as lists.
func (t *Tensor) ToNested() interface{} {
	return nestedSlice(t.data, t.shape, 0, 0)
}

func nestedSlice(data []float64, shape []int, dim, off int) interface{} {
	if dim == len(shape) {
		return data[off]
	}
	out := make([]interface{}, shape[dim])
	stride := product(shape[dim+1:])
	for i := 0; i < shape[dim]; i++ {
		out[i] = nestedSlice(data, shape, dim+1, off+i*stride)
	}
	return out
}

// -----------------------------------------------------------------------------
// Internal helpers
// -----------------------------------------------------------------------------

func product(shape []int) int {
	n := 1
	for _, s := range shape {
		n *= s
	}
	return n
}

func cloneInts(s []int) []int {
	out := make([]int, len(s))
	copy(out, s)
	return out
}

func slicesClone(s []float64) []float64 {
	out := make([]float64, len(s))
	copy(out, s)
	return out
}

// contiguousStrides returns row-major strides for a shape.
func contiguousStrides(shape []int) []int {
	strides := make([]int, len(shape))
	acc := 1
	for i := len(shape) - 1; i >= 0; i-- {
		strides[i] = acc
		acc *= shape[i]
	}
	return strides
}

// iterate walks every index of shape in row-major order, calling fn with the
// flat offset and the current multi-dimensional index (reused buffer).
func iterate(shape []int, fn func(flat int, idx []int)) {
	if product(shape) == 0 {
		return
	}
	idx := make([]int, len(shape))
	total := product(shape)
	for flat := 0; flat < total; flat++ {
		fn(flat, idx)
		// increment idx in row-major order
		for d := len(shape) - 1; d >= 0; d-- {
			idx[d]++
			if idx[d] < shape[d] {
				break
			}
			idx[d] = 0
		}
	}
}

func inferShape(shape []int, n int) []int {
	out := make([]int, len(shape))
	copy(out, shape)
	infer := -1
	total := 1
	for i, s := range shape {
		if s == -1 {
			if infer >= 0 {
				panic("libtorch: only one -1 allowed in reshape")
			}
			infer = i
			continue
		}
		if s < 0 {
			panic("libtorch: negative dimension other than -1 is not allowed")
		}
		total *= s
	}
	if infer >= 0 {
		if total == 0 || n%total != 0 {
			panic(fmt.Sprintf("libtorch: cannot infer -1 for reshape of %d elements into %v", n, shape))
		}
		out[infer] = n / total
	}
	return out
}

func permIdentity(d int) []int {
	p := make([]int, d)
	for i := range p {
		p[i] = i
	}
	return p
}

func swapLastTwo(p []int) []int {
	n := len(p)
	p[n-1], p[n-2] = p[n-2], p[n-1]
	return p
}
