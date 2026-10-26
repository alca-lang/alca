// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package libtorch

import "math"

// normalizeAxes resolves negative axes, removes duplicates, and sorts ascending.
// With no axes given it returns every axis of a rank-n tensor.
func normalizeAxes(n int, axes []int) []int {
	if len(axes) == 0 {
		all := make([]int, n)
		for i := range all {
			all[i] = i
		}
		return all
	}
	seen := make(map[int]bool)
	out := make([]int, 0, len(axes))
	for _, a := range axes {
		if a < 0 {
			a += n
		}
		if a < 0 || a >= n {
			panicf("libtorch: axis out of range (got %d for rank %d)", a, n)
		}
		if !seen[a] {
			seen[a] = true
			out = append(out, a)
		}
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// reduceShape returns the output shape with reduced dims removed (outShape) and
// with reduced dims kept as size-1 (keepShape).
func reduceShape(shape []int, axes []int, keepDim bool) (outShape, keepShape []int) {
	isReduced := make([]bool, len(shape))
	for _, a := range axes {
		isReduced[a] = true
	}
	keepShape = make([]int, len(shape))
	for i, s := range shape {
		if isReduced[i] {
			keepShape[i] = 1
		} else {
			keepShape[i] = s
		}
	}
	if keepDim {
		return cloneInts(keepShape), keepShape
	}
	outShape = make([]int, 0, len(shape))
	for i, s := range shape {
		if !isReduced[i] {
			outShape = append(outShape, s)
		}
	}
	return outShape, keepShape
}

// outIndexFor maps an input index to the output index for a reduction, honoring
// keepDim.
func outIndexFor(cur []int, isReduced []bool, keepDim bool, outShape []int) []int {
	outIdx := make([]int, len(outShape))
	if keepDim {
		for i := 0; i < len(cur); i++ {
			if isReduced[i] {
				outIdx[i] = 0
			} else {
				outIdx[i] = cur[i]
			}
		}
	} else {
		j := 0
		for i := 0; i < len(cur); i++ {
			if !isReduced[i] {
				outIdx[j] = cur[i]
				j++
			}
		}
	}
	return outIdx
}

// flatOffset computes the row-major flat offset of idx within shape.
func flatOffset(shape, idx []int) int {
	flat := 0
	for i, d := range idx {
		flat = flat*shape[i] + d
	}
	return flat
}

func reducedFlags(n int, axes []int) []bool {
	isReduced := make([]bool, n)
	for _, a := range axes {
		isReduced[a] = true
	}
	return isReduced
}

// sumAxesImpl sums over norm axes, producing outShape (which must match keepDim).
func (t *Tensor) sumAxesImpl(norm []int, keepDim bool, outShape []int) *Tensor {
	out := make([]float64, product(outShape))
	isReduced := reducedFlags(t.Dim(), norm)
	iterate(t.shape, func(flat int, cur []int) {
		oi := flatOffset(outShape, outIndexFor(cur, isReduced, keepDim, outShape))
		out[oi] += t.data[flat]
	})
	return &Tensor{data: out, shape: outShape}
}

// sumAxes is the internal sum used by broadcasting; reduced dims are kept as
// size-1 when keepDims is true.
func (t *Tensor) sumAxes(axes []int, keepDims bool) *Tensor {
	norm := normalizeAxes(t.Dim(), axes)
	outShape, _ := reduceShape(t.shape, norm, keepDims)
	return t.sumAxesImpl(norm, keepDims, outShape)
}

// -----------------------------------------------------------------------------
// Public reductions
// -----------------------------------------------------------------------------

// SumAll returns the sum of every element as a rank-0 tensor.
func (t *Tensor) SumAll() *Tensor {
	acc := 0.0
	for _, v := range t.data {
		acc += v
	}
	out := Scalar(acc)
	inShape := t.shape
	out.attachGrad([]*Tensor{t}, func() {
		accumulateGrad(t, out.grad.broadcastTo(inShape))
	})
	return out
}

// Sum reduces by the given axes (all if empty). When keepDim is true the reduced
// axes are retained as size-1.
func (t *Tensor) Sum(keepDim bool, axes ...int) *Tensor {
	norm := normalizeAxes(t.Dim(), axes)
	outShape, keepShape := reduceShape(t.shape, norm, keepDim)
	inShape := t.shape
	out := t.sumAxesImpl(norm, keepDim, outShape)
	out.attachGrad([]*Tensor{t}, func() {
		g := out.grad
		if !keepDim {
			g = g.reshapeTo(keepShape)
		}
		accumulateGrad(t, g.broadcastTo(inShape))
	})
	return out
}

// Mean reduces by the given axes (all if empty), averaging over the reduced
// elements.
func (t *Tensor) Mean(keepDim bool, axes ...int) *Tensor {
	norm := normalizeAxes(t.Dim(), axes)
	outShape, keepShape := reduceShape(t.shape, norm, keepDim)
	inShape := t.shape
	count := 1
	for _, a := range norm {
		count *= t.shape[a]
	}
	cf := float64(count)
	out := t.sumAxesImpl(norm, keepDim, outShape)
	for i := range out.data {
		out.data[i] /= cf
	}
	out.attachGrad([]*Tensor{t}, func() {
		g := out.grad
		if !keepDim {
			g = g.reshapeTo(keepShape)
		}
		accumulateGrad(t, g.broadcastTo(inShape).DivScalar(cf))
	})
	return out
}

// Max reduces by the given axes, routing the gradient to the argmax (first
// occurrence on ties).
func (t *Tensor) Max(keepDim bool, axes ...int) *Tensor {
	return t.minMax(true, keepDim, axes...)
}

// Min reduces by the given axes, routing the gradient to the argmin.
func (t *Tensor) Min(keepDim bool, axes ...int) *Tensor {
	return t.minMax(false, keepDim, axes...)
}

func (t *Tensor) minMax(isMax bool, keepDim bool, axes ...int) *Tensor {
	norm := normalizeAxes(t.Dim(), axes)
	outShape, keepShape := reduceShape(t.shape, norm, keepDim)
	inShape := t.shape

	out := make([]float64, product(outShape))
	init := math.Inf(-1)
	if !isMax {
		init = math.Inf(1)
	}
	for i := range out {
		out[i] = init
	}
	bestIdx := make([]int, len(out))
	isReduced := reducedFlags(t.Dim(), norm)
	iterate(t.shape, func(flat int, cur []int) {
		oi := flatOffset(outShape, outIndexFor(cur, isReduced, keepDim, outShape))
		v := t.data[flat]
		if (isMax && v > out[oi]) || (!isMax && v < out[oi]) {
			out[oi] = v
			bestIdx[oi] = flat
		}
	})

	res := &Tensor{data: out, shape: outShape}
	inData := t.data
	inStrides := contiguousStrides(inShape)
	res.attachGrad([]*Tensor{t}, func() {
		g := res.grad
		if !keepDim {
			g = g.reshapeTo(keepShape)
		}
		gb := g.broadcastTo(inShape)
		da := make([]float64, len(inData))
		iterate(inShape, func(flat int, cur []int) {
			oi := flatOffset(keepShape, outIndexFor(cur, isReduced, true, keepShape))
			if bestIdx[oi] == flat {
				da[flat] = gb.At(cur...)
			}
		})
		_ = inStrides
		accumulateGrad(t, &Tensor{data: da, shape: cloneInts(inShape)})
	})
	return res
}

// -----------------------------------------------------------------------------
// Elementwise min/max between two tensors
// -----------------------------------------------------------------------------

// Maximum returns the elementwise maximum of t and o with broadcasting.
func (t *Tensor) Maximum(o *Tensor) *Tensor {
	out, ab, bb, _ := binaryForward(t, o, math.Max)
	out.attachGrad([]*Tensor{t, o}, func() {
		tMask := maskFrom(ab, bb, true)
		accumulateGrad(t, out.grad.Mul(tMask))
		accumulateGrad(o, out.grad.Mul(fullLike(out, 1).Sub(tMask)))
	})
	return out
}

// Minimum returns the elementwise minimum of t and o with broadcasting.
func (t *Tensor) Minimum(o *Tensor) *Tensor {
	out, ab, bb, _ := binaryForward(t, o, math.Min)
	out.attachGrad([]*Tensor{t, o}, func() {
		tMask := maskFrom(ab, bb, false)
		accumulateGrad(t, out.grad.Mul(tMask))
		accumulateGrad(o, out.grad.Mul(fullLike(out, 1).Sub(tMask)))
	})
	return out
}

// maskFrom returns 1 where a >= b (isMax) or a <= b (!isMax).
func maskFrom(a, b *Tensor, isMax bool) *Tensor {
	out := Zeros(a.shape...)
	for i := range a.data {
		if (isMax && a.data[i] >= b.data[i]) || (!isMax && a.data[i] <= b.data[i]) {
			out.data[i] = 1
		}
	}
	return out
}
