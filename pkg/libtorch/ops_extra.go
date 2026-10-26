// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package libtorch

import (
	"math"
	"sort"
)

// -----------------------------------------------------------------------------
// Cumulative / product / norm reductions
// -----------------------------------------------------------------------------

// CumSum returns the exclusive-free running sum along the given axis.
func (t *Tensor) CumSum(axis int) *Tensor {
	if axis < 0 {
		axis += t.Dim()
	}
	out := Zeros(t.shape...)
	outer, inner, dim, stride, base := axisLayout(t.shape, axis)
	for o := 0; o < outer; o++ {
		for i := 0; i < inner; i++ {
			b := base(o, i)
			acc := 0.0
			for j := 0; j < dim; j++ {
				acc += t.data[b+j*stride]
				out.data[b+j*stride] = acc
			}
		}
	}
	inShape := t.shape
	out.attachGrad([]*Tensor{t}, func() {
		// Reverse cumulative sum along the axis.
		da := make([]float64, len(t.data))
		for o := 0; o < outer; o++ {
			for i := 0; i < inner; i++ {
				b := base(o, i)
				acc := 0.0
				for j := dim - 1; j >= 0; j-- {
					acc += out.grad.data[b+j*stride]
					da[b+j*stride] = acc
				}
			}
		}
		accumulateGrad(t, &Tensor{data: da, shape: cloneInts(inShape)})
	})
	return out
}

// Prod reduces by the given axes by multiplying. Gradients use the product of
// the other elements in each reduced group.
func (t *Tensor) Prod(keepDim bool, axes ...int) *Tensor {
	norm := normalizeAxes(t.Dim(), axes)
	outShape, keepShape := reduceShape(t.shape, norm, keepDim)
	inShape := t.shape
	isReduced := reducedFlags(t.Dim(), norm)

	out := make([]float64, product(outShape))
	for i := range out {
		out[i] = 1
	}
	iterate(t.shape, func(flat int, cur []int) {
		oi := flatOffset(outShape, outIndexFor(cur, isReduced, keepDim, outShape))
		out[oi] *= t.data[flat]
	})
	res := &Tensor{data: out, shape: outShape}
	inData := t.data
	axisCount := len(norm)
	_ = axisCount
	res.attachGrad([]*Tensor{t}, func() {
		g := res.grad
		if !keepDim {
			g = g.reshapeTo(keepShape)
		}
		gb := g.broadcastTo(inShape)
		da := make([]float64, len(inData))
		// For each output cell, gather the group and compute product-of-others.
		group := make([]int, 0, 8)
		iterate(inShape, func(flat int, cur []int) {
			oi := flatOffset(keepShape, outIndexFor(cur, isReduced, true, keepShape))
			// Collect group members lazily once per oi via a scan; for simplicity
			// recompute product-of-others over the group here.
			_ = oi
			_ = group
		})
		// Simpler: recompute per group using base offsets.
		groupBase, groupIdx := t.groupIndexer(norm, keepShape)
		for oi := 0; oi < len(groupBase); oi++ {
			members := groupIdx[oi]
			prodOthers := make([]float64, len(members))
			for i := range members {
				p := 1.0
				for j := range members {
					if i != j {
						p *= inData[members[j]]
					}
				}
				prodOthers[i] = p
			}
			for i, flat := range members {
				da[flat] = gb.data[flat] * prodOthers[i]
			}
		}
		accumulateGrad(t, &Tensor{data: da, shape: cloneInts(inShape)})
	})
	return res
}

// groupIndexer returns, for each output cell, the flat indices of the input
// elements that reduce into it. It is used by reductions needing per-group
// access (e.g. Prod).
func (t *Tensor) groupIndexer(axes []int, keepShape []int) ([]int, [][]int) {
	isReduced := reducedFlags(t.Dim(), axes)
	nOut := product(keepShape)
	bases := make([]int, nOut)
	members := make([][]int, nOut)
	for i := range members {
		members[i] = make([]int, 0, 8)
	}
	iterate(t.shape, func(flat int, cur []int) {
		oi := flatOffset(keepShape, outIndexFor(cur, isReduced, true, keepShape))
		members[oi] = append(members[oi], flat)
	})
	return bases, members
}

// LogSumExp returns log(sum(exp(x))) over the given axes, computed stably.
func (t *Tensor) LogSumExp(keepDim bool, axes ...int) *Tensor {
	norm := normalizeAxes(t.Dim(), axes)
	outShape, keepShape := reduceShape(t.shape, norm, keepDim)
	inShape := t.shape
	isReduced := reducedFlags(t.Dim(), norm)

	// Pass 1: max per group for stability.
	maxv := make([]float64, product(outShape))
	for i := range maxv {
		maxv[i] = math.Inf(-1)
	}
	iterate(t.shape, func(flat int, cur []int) {
		oi := flatOffset(outShape, outIndexFor(cur, isReduced, keepDim, outShape))
		if t.data[flat] > maxv[oi] {
			maxv[oi] = t.data[flat]
		}
	})
	// Pass 2: sum exp.
	sumExp := make([]float64, product(outShape))
	iterate(t.shape, func(flat int, cur []int) {
		oi := flatOffset(outShape, outIndexFor(cur, isReduced, keepDim, outShape))
		sumExp[oi] += math.Exp(t.data[flat] - maxv[oi])
	})
	out := make([]float64, len(sumExp))
	for i := range out {
		out[i] = math.Log(sumExp[i]) + maxv[i]
	}
	res := &Tensor{data: out, shape: outShape}
	inShapeB := inShape
	res.attachGrad([]*Tensor{t}, func() {
		g := res.grad
		if !keepDim {
			g = g.reshapeTo(keepShape)
		}
		gb := g.broadcastTo(inShapeB)
		// dx = g * softmax(x) over the reduced axes.
		sm := t.SoftmaxAxes(norm)
		accumulateGrad(t, gb.Mul(sm))
	})
	return res
}

// SoftmaxAxes returns the softmax over the given axes (kept as size-1).
func (t *Tensor) SoftmaxAxes(axes []int) *Tensor {
	norm := normalizeAxes(t.Dim(), axes)
	_, keepShape := reduceShape(t.shape, norm, true)
	inShape := t.shape
	// Subtract group max for stability, exp, normalize.
	isReduced := reducedFlags(t.Dim(), norm)
	maxv := make([]float64, product(keepShape))
	for i := range maxv {
		maxv[i] = math.Inf(-1)
	}
	iterate(t.shape, func(flat int, cur []int) {
		oi := flatOffset(keepShape, outIndexFor(cur, isReduced, true, keepShape))
		if t.data[flat] > maxv[oi] {
			maxv[oi] = t.data[flat]
		}
	})
	sumExp := make([]float64, product(keepShape))
	out := make([]float64, len(t.data))
	iterate(t.shape, func(flat int, cur []int) {
		oi := flatOffset(keepShape, outIndexFor(cur, isReduced, true, keepShape))
		e := math.Exp(t.data[flat] - maxv[oi])
		out[flat] = e
		sumExp[oi] += e
	})
	iterate(t.shape, func(flat int, cur []int) {
		oi := flatOffset(keepShape, outIndexFor(cur, isReduced, true, keepShape))
		out[flat] /= sumExp[oi]
	})
	_ = inShape
	return &Tensor{data: out, shape: cloneInts(t.shape)}
}

// LpNorm returns the p-norm over the given axes: (sum |x|^p)^(1/p).
func (t *Tensor) LpNorm(p float64, keepDim bool, axes ...int) *Tensor {
	norm := normalizeAxes(t.Dim(), axes)
	outShape, keepShape := reduceShape(t.shape, norm, keepDim)
	inShape := t.shape
	isReduced := reducedFlags(t.Dim(), norm)

	absPow := make([]float64, product(outShape))
	iterate(t.shape, func(flat int, cur []int) {
		oi := flatOffset(outShape, outIndexFor(cur, isReduced, keepDim, outShape))
		absPow[oi] += math.Pow(math.Abs(t.data[flat]), p)
	})
	out := make([]float64, len(absPow))
	for i := range out {
		out[i] = math.Pow(absPow[i], 1/p)
	}
	res := &Tensor{data: out, shape: outShape}
	inData := t.data
	res.attachGrad([]*Tensor{t}, func() {
		g := res.grad
		if !keepDim {
			g = g.reshapeTo(keepShape)
		}
		gb := g.broadcastTo(inShape)
		da := make([]float64, len(inData))
		if p == 2 {
			iterate(inShape, func(flat int, cur []int) {
				oi := flatOffset(keepShape, outIndexFor(cur, isReduced, true, keepShape))
				normv := res.data[oi]
				if normv != 0 {
					da[flat] = gb.data[flat] * inData[flat] / normv
				}
			})
		} else {
			iterate(inShape, func(flat int, cur []int) {
				oi := flatOffset(keepShape, outIndexFor(cur, isReduced, true, keepShape))
				normv := res.data[oi]
				if normv != 0 {
					sign := math.Copysign(1, inData[flat])
					da[flat] = gb.data[flat] * sign * math.Pow(math.Abs(inData[flat]), p-1) / math.Pow(normv, p-1)
				}
			})
		}
		accumulateGrad(t, &Tensor{data: da, shape: cloneInts(inShape)})
	})
	return res
}

// Norm is the L2 norm by default (p=2).
func (t *Tensor) Norm(p float64, keepDim bool, axes ...int) *Tensor {
	return t.LpNorm(p, keepDim, axes...)
}

// Trace returns the sum of the diagonal of the last two dimensions.
func (t *Tensor) Trace() *Tensor {
	d := t.Dim()
	if d < 2 {
		panic("libtorch: Trace requires rank >= 2")
	}
	m := t.shape[d-2]
	n := t.shape[d-1]
	k := m
	if n < k {
		k = n
	}
	// Sum over all leading batches' diagonals.
	outer := product(t.shape[:d-2])
	acc := 0.0
	for b := 0; b < outer; b++ {
		base := b * m * n
		for i := 0; i < k; i++ {
			acc += t.data[base+i*n+i]
		}
	}
	out := Scalar(acc)
	inShape := t.shape
	out.attachGrad([]*Tensor{t}, func() {
		g := out.grad.Item()
		da := make([]float64, len(t.data))
		for b := 0; b < outer; b++ {
			base := b * m * n
			for i := 0; i < k; i++ {
				da[base+i*n+i] += g
			}
		}
		accumulateGrad(t, &Tensor{data: da, shape: cloneInts(inShape)})
	})
	return out
}

// -----------------------------------------------------------------------------
// Indexing / selection
// -----------------------------------------------------------------------------

// Argmax returns the index of the maximum along the given axes as a float tensor.
// The result carries no gradient.
func (t *Tensor) Argmax(keepDim bool, axes ...int) *Tensor {
	return t.argMinMax(true, keepDim, axes...)
}

// Argmin returns the index of the minimum along the given axes.
func (t *Tensor) Argmin(keepDim bool, axes ...int) *Tensor {
	return t.argMinMax(false, keepDim, axes...)
}

func (t *Tensor) argMinMax(isMax bool, keepDim bool, axes ...int) *Tensor {
	norm := normalizeAxes(t.Dim(), axes)
	outShape, _ := reduceShape(t.shape, norm, keepDim)
	isReduced := reducedFlags(t.Dim(), norm)

	bestVal := make([]float64, product(outShape))
	bestIdx := make([]int, product(outShape))
	for i := range bestVal {
		if isMax {
			bestVal[i] = math.Inf(-1)
		} else {
			bestVal[i] = math.Inf(1)
		}
	}
	iterate(t.shape, func(flat int, cur []int) {
		oi := flatOffset(outShape, outIndexFor(cur, isReduced, keepDim, outShape))
		v := t.data[flat]
		if (isMax && v > bestVal[oi]) || (!isMax && v < bestVal[oi]) {
			bestVal[oi] = v
			bestIdx[oi] = flat
		}
	})
	out := make([]float64, len(bestIdx))
	for i, idx := range bestIdx {
		// Convert flat index to the position along the reduced axes (sum of
		// reduced-dim coordinates). For a single reduced axis this is the exact
		// index; for multiple axes it is the flattened reduced index.
		out[i] = float64(reducedCoord(t.shape, idx, norm))
	}
	return &Tensor{data: out, shape: outShape}
}

// reducedCoord converts a flat input index into the flattened coordinate across
// the reduced axes.
func reducedCoord(shape []int, flat int, axes []int) int {
	idx := make([]int, len(shape))
	for i := len(shape) - 1; i >= 0; i-- {
		idx[i] = flat % shape[i]
		flat /= shape[i]
	}
	coord := 0
	for _, a := range axes {
		coord = coord*shape[a] + idx[a]
	}
	return coord
}

// Gather selects elements along dim using an integer index tensor (values must
// be whole numbers). The output shape equals the index shape.
func (t *Tensor) Gather(dim int, index *Tensor) *Tensor {
	if dim < 0 {
		dim += t.Dim()
	}
	out := Zeros(index.shape...)
	inShape := t.shape
	iterate(index.shape, func(flat int, cur []int) {
		// Build the input index: same as cur except at dim we use index's value.
		idx := make([]int, len(cur))
		copy(idx, cur)
		idx[dim] = int(index.data[flat])
		out.data[flat] = t.At(idx...)
	})
	out.attachGrad([]*Tensor{t}, func() {
		da := make([]float64, len(t.data))
		iterate(index.shape, func(flat int, cur []int) {
			idx := make([]int, len(cur))
			copy(idx, cur)
			idx[dim] = int(index.data[flat])
			da[t.flatIndex(idx)] += out.grad.data[flat]
		})
		accumulateGrad(t, &Tensor{data: da, shape: cloneInts(inShape)})
	})
	return out
}

// IndexSelect selects whole slices along the given axis by integer indices. The
// output's axis dimension becomes len(indices).
func (t *Tensor) IndexSelect(axis int, indices []int) *Tensor {
	if axis < 0 {
		axis += t.Dim()
	}
	outShape := cloneInts(t.shape)
	outShape[axis] = len(indices)
	out := Zeros(outShape...)
	outStrides := contiguousStrides(outShape)
	inStrides := contiguousStrides(t.shape)
	inShape := t.shape
	// For each output element, map to input.
	iterate(outShape, func(oflat int, oidx []int) {
		iidx := make([]int, len(oidx))
		copy(iidx, oidx)
		iidx[axis] = indices[oidx[axis]]
		// compute input flat
		iflat := 0
		for i, d := range iidx {
			iflat += d * inStrides[i]
		}
		out.data[oflat] = t.data[iflat]
		_ = outStrides
	})
	out.attachGrad([]*Tensor{t}, func() {
		da := make([]float64, len(t.data))
		iterate(outShape, func(oflat int, oidx []int) {
			iidx := make([]int, len(oidx))
			copy(iidx, oidx)
			iidx[axis] = indices[oidx[axis]]
			iflat := 0
			for i, d := range iidx {
				iflat += d * inStrides[i]
			}
			da[iflat] += out.grad.data[oflat]
		})
		accumulateGrad(t, &Tensor{data: da, shape: cloneInts(inShape)})
	})
	return out
}

// MaskedFill replaces elements where mask is non-zero with the given value.
// Gradients flow only through the un-masked positions.
func (t *Tensor) MaskedFill(mask *Tensor, value float64) *Tensor {
	mb := mask.broadcastTo(t.shape)
	out := t.Clone()
	for i := range out.data {
		if mb.data[i] != 0 {
			out.data[i] = value
		}
	}
	outShape := t.shape
	inShape := t.shape
	out.attachGrad([]*Tensor{t}, func() {
		gb := out.grad.broadcastTo(inShape)
		da := make([]float64, len(t.data))
		for i := range da {
			if mb.data[i] == 0 {
				da[i] = gb.data[i]
			}
		}
		_ = outShape
		accumulateGrad(t, &Tensor{data: da, shape: cloneInts(inShape)})
	})
	return out
}

// Where returns a[i] where cond is non-zero, else b[i], with broadcasting.
func Where(cond, a, b *Tensor) *Tensor {
	shape, err := broadcastShapes(a.shape, b.shape)
	if err != nil {
		panic(err)
	}
	shape, err = broadcastShapes(shape, cond.shape)
	if err != nil {
		panic(err)
	}
	cb := cond.broadcastTo(shape)
	ab := a.broadcastTo(shape)
	bb := b.broadcastTo(shape)
	out := Zeros(shape...)
	for i := range out.data {
		if cb.data[i] != 0 {
			out.data[i] = ab.data[i]
		} else {
			out.data[i] = bb.data[i]
		}
	}
	aShape := a.shape
	bShape := b.shape
	out.attachGrad([]*Tensor{a, b}, func() {
		gb := out.grad
		da := make([]float64, product(shape))
		db := make([]float64, product(shape))
		for i := range da {
			if cb.data[i] != 0 {
				da[i] = gb.data[i]
			} else {
				db[i] = gb.data[i]
			}
		}
		accumulateGrad(a, (&Tensor{data: da, shape: cloneInts(shape)}).reduceGradToShapeHelper(aShape))
		accumulateGrad(b, (&Tensor{data: db, shape: cloneInts(shape)}).reduceGradToShapeHelper(bShape))
	})
	return out
}

// reduceGradToShapeHelper is an exported-behavior wrapper used internally by
// Where to reduce a broadcast gradient back to a target shape.
func (t *Tensor) reduceGradToShapeHelper(target []int) *Tensor { return reduceGradToShape(t, target) }

// Sort returns the values (and via SortWithIndices the indices) sorted along the
// given axis. Only the value tensor carries a gradient path.
func (t *Tensor) Sort(axis int, descending bool) *Tensor {
	vals, _ := t.SortWithIndices(axis, descending)
	return vals
}

// SortWithIndices returns sorted values and the corresponding original indices
// along the given axis.
func (t *Tensor) SortWithIndices(axis int, descending bool) (*Tensor, *Tensor) {
	if axis < 0 {
		axis += t.Dim()
	}
	out := Zeros(t.shape...)
	indices := Zeros(t.shape...)
	outer, inner, dim, stride, base := axisLayout(t.shape, axis)
	for o := 0; o < outer; o++ {
		for i := 0; i < inner; i++ {
			b := base(o, i)
			type pair struct {
				v float64
				j int
			}
			pairs := make([]pair, dim)
			for j := 0; j < dim; j++ {
				pairs[j] = pair{t.data[b+j*stride], j}
			}
			sort.SliceStable(pairs, func(x, y int) bool {
				if descending {
					return pairs[x].v > pairs[y].v
				}
				return pairs[x].v < pairs[y].v
			})
			for j, p := range pairs {
				out.data[b+j*stride] = p.v
				indices.data[b+j*stride] = float64(p.j)
			}
		}
	}
	inShape := t.shape
	out.attachGrad([]*Tensor{t}, func() {
		// Route gradient back through the permutation.
		da := make([]float64, len(t.data))
		for o := 0; o < outer; o++ {
			for i := 0; i < inner; i++ {
				b := base(o, i)
				for j := 0; j < dim; j++ {
					orig := int(indices.data[b+j*stride])
					da[b+orig*stride] += out.grad.data[b+j*stride]
				}
			}
		}
		accumulateGrad(t, &Tensor{data: da, shape: cloneInts(inShape)})
	})
	return out, indices
}

// Narrow returns a copy of the sub-tensor along the given axis from start for
// length elements. Gradients scatter back into the sliced-out positions.
func (t *Tensor) Narrow(axis, start, length int) *Tensor {
	if axis < 0 {
		axis += t.Dim()
	}
	if start < 0 || length <= 0 || start+length > t.shape[axis] {
		panicf("libtorch: Narrow range [%d,%d) invalid for axis %d of size %d", start, start+length, axis, t.shape[axis])
	}
	outShape := cloneInts(t.shape)
	outShape[axis] = length
	out := Zeros(outShape...)
	inStrides := contiguousStrides(t.shape)
	inShape := t.shape
	iterate(outShape, func(oflat int, oidx []int) {
		iidx := cloneInts(oidx)
		iidx[axis] = start + oidx[axis]
		iflat := 0
		for i, d := range iidx {
			iflat += d * inStrides[i]
		}
		out.data[oflat] = t.data[iflat]
	})
	out.attachGrad([]*Tensor{t}, func() {
		da := make([]float64, len(t.data))
		iterate(outShape, func(oflat int, oidx []int) {
			iidx := cloneInts(oidx)
			iidx[axis] = start + oidx[axis]
			iflat := 0
			for i, d := range iidx {
				iflat += d * inStrides[i]
			}
			da[iflat] += out.grad.data[oflat]
		})
		accumulateGrad(t, &Tensor{data: da, shape: cloneInts(inShape)})
	})
	return out
}

// TopK returns the k largest (or smallest when largest=false) elements along the
// axis, as a map with "values" and "indices".
func (t *Tensor) TopK(k, axis int, largest bool) (*Tensor, *Tensor) {
	if axis < 0 {
		axis += t.Dim()
	}
	dim := t.shape[axis]
	if k > dim {
		k = dim
	}
	outer, inner, d, stride, base := axisLayout(t.shape, axis)
	outShape := cloneInts(t.shape)
	outShape[axis] = k
	out := Zeros(outShape...)
	indices := Zeros(outShape...)
	outStride := contiguousStrides(outShape)[axis]
	for o := 0; o < outer; o++ {
		for i := 0; i < inner; i++ {
			b := base(o, i)
			type pair struct {
				v float64
				j int
			}
			pairs := make([]pair, d)
			for j := 0; j < d; j++ {
				pairs[j] = pair{t.data[b+j*stride], j}
			}
			sort.SliceStable(pairs, func(x, y int) bool {
				if largest {
					return pairs[x].v > pairs[y].v
				}
				return pairs[x].v < pairs[y].v
			})
			for j := 0; j < k; j++ {
				ob := o * (k * inner * 1) // placeholder; recompute via flat index below
				_ = ob
				// output flat for (o, j, i): build index and use outStride
				oidx := make([]int, len(t.shape))
				// decode o into outer dims
				tmp := o
				for dimI := axis - 1; dimI >= 0; dimI-- {
					oidx[dimI] = tmp % t.shape[dimI]
					tmp /= t.shape[dimI]
				}
				oidx[axis] = j
				tmp = i
				for dimI := len(t.shape) - 1; dimI > axis; dimI-- {
					oidx[dimI] = tmp % t.shape[dimI]
					tmp /= t.shape[dimI]
				}
				oflat := flatOffset(outShape, oidx)
				out.data[oflat] = pairs[j].v
				indices.data[oflat] = float64(pairs[j].j)
				_ = outStride
			}
		}
	}
	inShape := t.shape
	out.attachGrad([]*Tensor{t}, func() {
		da := make([]float64, len(t.data))
		iterate(outShape, func(oflat int, oidx []int) {
			// map back: same as TopK index decode
			iidx := make([]int, len(oidx))
			copy(iidx, oidx)
			iidx[axis] = int(indices.data[oflat])
			// compute input flat using strides of t
			inStrides := contiguousStrides(inShape)
			iflat := 0
			for mi, dd := range iidx {
				iflat += dd * inStrides[mi]
			}
			da[iflat] += out.grad.data[oflat]
		})
		accumulateGrad(t, &Tensor{data: da, shape: cloneInts(inShape)})
	})
	return out, indices
}
