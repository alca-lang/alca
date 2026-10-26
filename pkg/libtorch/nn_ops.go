// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package libtorch

import (
	"math"
	"math/rand"
)

// axisLayout describes how to walk 1-D rows along a chosen axis. It returns the
// outer count, inner count, row length, the stride between row elements, and a
// function producing the flat base offset for row (o, i).
func axisLayout(shape []int, axis int) (outer, inner, dim, axisStride int, base func(o, i int) int) {
	strides := contiguousStrides(shape)
	outerDims := shape[:axis]
	innerDims := shape[axis+1:]
	outerStrides := strides[:axis]
	innerStrides := strides[axis+1:]
	outer = product(outerDims)
	inner = product(innerDims)
	dim = shape[axis]
	axisStride = strides[axis]
	base = func(o, i int) int {
		return decodeOffset(o, outerDims, outerStrides) + decodeOffset(i, innerDims, innerStrides)
	}
	return outer, inner, dim, axisStride, base
}

// decodeOffset converts a linear index k over dims into a flat offset using the
// provided strides.
func decodeOffset(k int, dims, strides []int) int {
	off := 0
	for i := len(dims) - 1; i >= 0; i-- {
		d := k % dims[i]
		k /= dims[i]
		off += d * strides[i]
	}
	return off
}

// -----------------------------------------------------------------------------
// Softmax / LogSoftmax
// -----------------------------------------------------------------------------

// Softmax returns the softmax along the given axis (last axis by default).
func (t *Tensor) Softmax(axis ...int) *Tensor {
	ax := t.Dim() - 1
	if len(axis) > 0 {
		ax = axis[0]
		if ax < 0 {
			ax += t.Dim()
		}
	}
	outer, inner, dim, stride, base := axisLayout(t.shape, ax)
	out := Zeros(t.shape...)
	for o := 0; o < outer; o++ {
		for i := 0; i < inner; i++ {
			b := base(o, i)
			// max for stability
			maxv := math.Inf(-1)
			for j := 0; j < dim; j++ {
				if v := t.data[b+j*stride]; v > maxv {
					maxv = v
				}
			}
			sum := 0.0
			for j := 0; j < dim; j++ {
				e := math.Exp(t.data[b+j*stride] - maxv)
				out.data[b+j*stride] = e
				sum += e
			}
			for j := 0; j < dim; j++ {
				out.data[b+j*stride] /= sum
			}
		}
	}
	inShape := t.shape
	out.attachGrad([]*Tensor{t}, func() {
		// dx_j = s_j * (g_j - sum_k g_k s_k)
		da := make([]float64, len(t.data))
		for o := 0; o < outer; o++ {
			for i := 0; i < inner; i++ {
				b := base(o, i)
				dot := 0.0
				for j := 0; j < dim; j++ {
					dot += out.grad.data[b+j*stride] * out.data[b+j*stride]
				}
				for j := 0; j < dim; j++ {
					s := out.data[b+j*stride]
					da[b+j*stride] = s * (out.grad.data[b+j*stride] - dot)
				}
			}
		}
		accumulateGrad(t, &Tensor{data: da, shape: cloneInts(inShape)})
	})
	return out
}

// LogSoftmax returns the log-softmax along the given axis (last axis by default).
func (t *Tensor) LogSoftmax(axis ...int) *Tensor {
	ax := t.Dim() - 1
	if len(axis) > 0 {
		ax = axis[0]
		if ax < 0 {
			ax += t.Dim()
		}
	}
	outer, inner, dim, stride, base := axisLayout(t.shape, ax)
	out := Zeros(t.shape...)
	for o := 0; o < outer; o++ {
		for i := 0; i < inner; i++ {
			b := base(o, i)
			maxv := math.Inf(-1)
			for j := 0; j < dim; j++ {
				if v := t.data[b+j*stride]; v > maxv {
					maxv = v
				}
			}
			lse := 0.0
			for j := 0; j < dim; j++ {
				lse += math.Exp(t.data[b+j*stride] - maxv)
			}
			lse = math.Log(lse) + maxv
			for j := 0; j < dim; j++ {
				out.data[b+j*stride] = t.data[b+j*stride] - lse
			}
		}
	}
	inShape := t.shape
	out.attachGrad([]*Tensor{t}, func() {
		// softmax from log-softmax
		sm := out.Exp()
		da := make([]float64, len(t.data))
		for o := 0; o < outer; o++ {
			for i := 0; i < inner; i++ {
				b := base(o, i)
				gsum := 0.0
				for j := 0; j < dim; j++ {
					gsum += out.grad.data[b+j*stride]
				}
				for j := 0; j < dim; j++ {
					da[b+j*stride] = out.grad.data[b+j*stride] - sm.data[b+j*stride]*gsum
				}
			}
		}
		accumulateGrad(t, &Tensor{data: da, shape: cloneInts(inShape)})
	})
	return out
}

// -----------------------------------------------------------------------------
// Concat / Stack
// -----------------------------------------------------------------------------

// Concat joins tensors along the given axis. All non-axis dimensions must match.
func Concat(axis int, tensors ...*Tensor) *Tensor {
	if len(tensors) == 0 {
		panic("libtorch: Concat requires at least one tensor")
	}
	if len(tensors) == 1 {
		return tensors[0]
	}
	ref := tensors[0]
	if axis < 0 {
		axis += ref.Dim()
	}
	shape := cloneInts(ref.shape)
	total := 0
	for _, t := range tensors {
		if t.Dim() != ref.Dim() {
			panicf("libtorch: Concat rank mismatch: %v vs %v", t.shape, ref.shape)
		}
		for d := 0; d < ref.Dim(); d++ {
			if d != axis && t.shape[d] != ref.shape[d] {
				panicf("libtorch: Concat non-axis dims differ: %v vs %v", t.shape, ref.shape)
			}
		}
		total += t.shape[axis]
	}
	shape[axis] = total
	out := Zeros(shape...)

	// Copy each tensor's slab into position along axis.
	axisStride := contiguousStrides(shape)[axis]
	sliceSize := axisStride // elements per "row" block between axis slices
	// Number of elements before the axis dimension (block of one slice).
	dstOff := 0
	for _, t := range tensors {
		n := t.shape[axis]
		copySlab(out.data, dstOff*axisStride, t.data, n*axisStride)
		dstOff += n
	}
	_ = sliceSize

	// Backward: split grad along axis and accumulate.
	out.attachGrad(tensors, func() {
		srcOff := 0
		for _, t := range tensors {
			n := t.shape[axis]
			da := make([]float64, t.Numel())
			copySlab(da, 0, out.grad.data[srcOff*axisStride:], n*axisStride)
			accumulateGrad(t, &Tensor{data: da, shape: cloneInts(t.shape)})
			srcOff += n
		}
	})
	return out
}

// Stack joins tensors along a newly created leading axis.
func Stack(axis int, tensors ...*Tensor) *Tensor {
	unsqueezed := make([]*Tensor, len(tensors))
	for i, t := range tensors {
		unsqueezed[i] = t.Unsqueeze(axis)
	}
	return Concat(axis, unsqueezed...)
}

// copySlab copies n elements from src into dst at dstOff.
func copySlab(dst []float64, dstOff int, src []float64, n int) {
	copy(dst[dstOff:dstOff+n], src[:n])
}

// -----------------------------------------------------------------------------
// Dropout
// -----------------------------------------------------------------------------

// Dropout zeroes elements with probability p during training (scaling the rest
// by 1/(1-p)); in inference it is the identity. The mask is captured for backward.
func (t *Tensor) Dropout(p float64, training bool) *Tensor {
	if !training || p <= 0 {
		return t
	}
	if p >= 1 {
		panic("libtorch: Dropout p must be < 1")
	}
	scale := 1 / (1 - p)
	mask := Zeros(t.shape...)
	out := Zeros(t.shape...)
	for i, v := range t.data {
		if rand.Float64() >= p {
			mask.data[i] = scale
			out.data[i] = v * scale
		}
	}
	out.attachGrad([]*Tensor{t}, func() {
		accumulateGrad(t, out.grad.Mul(mask))
	})
	return out
}

// -----------------------------------------------------------------------------
// LayerNorm (normalizes over the last dimension)
// -----------------------------------------------------------------------------

// LayerNorm normalizes over the last dimension using per-element gain and bias,
// with a small epsilon for numerical stability.
func (t *Tensor) LayerNorm(gamma, beta *Tensor, eps float64) *Tensor {
	if eps == 0 {
		eps = 1e-5
	}
	last := t.Dim() - 1
	outer, inner, dim, stride, base := axisLayout(t.shape, last)
	_ = inner
	out := Zeros(t.shape...)
	xhat := Zeros(t.shape...)
	for o := 0; o < outer; o++ {
		b := base(o, 0)
		row := gatherRow(t.data, b, dim, stride)
		mean := meanOf(row)
		varc := varianceOf(row, mean)
		inv := 1 / math.Sqrt(varc+eps)
		for j := 0; j < dim; j++ {
			xh := (row[j] - mean) * inv
			xhat.data[b+j*stride] = xh
			g := 1.0
			if gamma != nil {
				g = gamma.data[j]
			}
			be := 0.0
			if beta != nil {
				be = beta.data[j]
			}
			out.data[b+j*stride] = g*xh + be
		}
	}
	parents := []*Tensor{t}
	if gamma != nil {
		parents = append(parents, gamma)
	}
	if beta != nil {
		parents = append(parents, beta)
	}
	inShape := t.shape
	out.attachGrad(parents, func() {
		da := make([]float64, len(t.data))
		var dgamma []float64
		var dbeta []float64
		if gamma != nil {
			dgamma = make([]float64, gamma.Numel())
		}
		if beta != nil {
			dbeta = make([]float64, beta.Numel())
		}
		for o := 0; o < outer; o++ {
			b := base(o, 0)
			// Gather per-element dOut, xhat.
			dxhat := make([]float64, dim)
			for j := 0; j < dim; j++ {
				dxhat[j] = out.grad.data[b+j*stride]
			}
			// gamma/beta grads
			if gamma != nil {
				for j := 0; j < dim; j++ {
					dgamma[j] += dxhat[j] * xhat.data[b+j*stride]
				}
			}
			if beta != nil {
				for j := 0; j < dim; j++ {
					dbeta[j] += dxhat[j]
				}
			}
			// dx = (1/std) * (dxhat - mean(dxhat) - xhat * mean(dxhat*xhat))
			gammaRow := onesIfNil(gamma, dim)
			dxhatTimesGamma := make([]float64, dim)
			for j := 0; j < dim; j++ {
				dxhatTimesGamma[j] = dxhat[j] * gammaRow[j]
			}
			m1 := meanOf(dxhatTimesGamma)
			xhDot := 0.0
			for j := 0; j < dim; j++ {
				xhDot += dxhatTimesGamma[j] * xhat.data[b+j*stride]
			}
			m2 := xhDot / float64(dim)
			row := gatherRow(t.data, b, dim, stride)
			mean := meanOf(row)
			varc := varianceOf(row, mean)
			inv := 1 / math.Sqrt(varc+eps)
			for j := 0; j < dim; j++ {
				da[b+j*stride] = inv * (dxhatTimesGamma[j] - m1 - xhat.data[b+j*stride]*m2)
			}
		}
		accumulateGrad(t, &Tensor{data: da, shape: cloneInts(inShape)})
		if gamma != nil {
			accumulateGrad(gamma, &Tensor{data: dgamma, shape: cloneInts(gamma.shape)})
		}
		if beta != nil {
			accumulateGrad(beta, &Tensor{data: dbeta, shape: cloneInts(beta.shape)})
		}
	})
	return out
}

func onesIfNil(t *Tensor, n int) []float64 {
	if t == nil {
		out := make([]float64, n)
		for i := range out {
			out[i] = 1
		}
		return out
	}
	return t.data
}

func gatherRow(data []float64, base, dim, stride int) []float64 {
	row := make([]float64, dim)
	for j := 0; j < dim; j++ {
		row[j] = data[base+j*stride]
	}
	return row
}

func meanOf(v []float64) float64 {
	s := 0.0
	for _, x := range v {
		s += x
	}
	return s / float64(len(v))
}

func varianceOf(v []float64, mean float64) float64 {
	s := 0.0
	for _, x := range v {
		d := x - mean
		s += d * d
	}
	return s / float64(len(v))
}
