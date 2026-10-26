// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package libtorch

import "fmt"

// matmul2d computes a(m,k) @ b(k,n) -> (m,n), row-major.
func matmul2d(a, b []float64, m, k, n int) []float64 {
	c := make([]float64, m*n)
	for i := 0; i < m; i++ {
		for p := 0; p < k; p++ {
			av := a[i*k+p]
			if av == 0 {
				continue
			}
			bp := p * n
			ci := i * n
			for j := 0; j < n; j++ {
				c[ci+j] += av * b[bp+j]
			}
		}
	}
	return c
}

// transpose2d returns the transpose of a(m,k) -> (k,m), row-major.
func transpose2d(a []float64, m, k int) []float64 {
	t := make([]float64, m*k)
	for i := 0; i < m; i++ {
		for j := 0; j < k; j++ {
			t[j*m+i] = a[i*k+j]
		}
	}
	return t
}

// MatMul returns t @ o. Both operands must have rank >= 2, shaped
// (batch..., m, k) and (batch..., k, n); leading batch dimensions are broadcast
// per PyTorch rules. For the inner-product of two rank-1 vectors use Dot.
//
// Backward is exact and accumulates through broadcast batch dimensions.
func (t *Tensor) MatMul(o *Tensor) *Tensor {
	if t.Dim() < 2 || o.Dim() < 2 {
		panic("libtorch: MatMul requires rank >= 2 on both operands; use Dot for vectors or Unsqueeze to add a batch axis")
	}
	tm, tk := t.shape[t.Dim()-2], t.shape[t.Dim()-1]
	ok, on := o.shape[o.Dim()-2], o.shape[o.Dim()-1]
	if tk != ok {
		panicf("libtorch: MatMul inner dimensions disagree: %v @ %v", t.shape, o.shape)
	}
	m, k, n := tm, tk, on

	leadT := t.shape[:t.Dim()-2]
	leadO := o.shape[:o.Dim()-2]
	batch, err := broadcastShapes(leadT, leadO)
	if err != nil {
		panicf("libtorch: MatMul batch dimensions do not broadcast: %v vs %v", leadT, leadO)
	}
	coreSizeT := m * k
	coreSizeO := k * n
	coreSizeC := m * n

	mapT := batchIndexMap(leadT, batch)
	mapO := batchIndexMap(leadO, batch)
	batchCount := product(batch)

	outShape := append(cloneInts(batch), m, n)
	out := Zeros(outShape...)
	for b := 0; b < batchCount; b++ {
		aCore := t.data[mapT[b]*coreSizeT : mapT[b]*coreSizeT+coreSizeT]
		bCore := o.data[mapO[b]*coreSizeO : mapO[b]*coreSizeO+coreSizeO]
		// Use gorgonia's optimized kernel when available (2D only).
		if batchCount == 1 && t.Dim() == 2 && o.Dim() == 2 {
			if res, ok := matMulGorgonia(t, o); ok {
				// Replace the output with gorgonia's result (autograd still
				// attached below via the existing attachGrad).
				copy(out.data, res.data)
				// Skip naive path.
				goto gorgoniaDone
			}
		}
		cCore := matmul2d(aCore, bCore, m, k, n)
		copy(out.data[b*coreSizeC:], cCore)
	}
gorgoniaDone:

	out.attachGrad([]*Tensor{t, o}, func() {
		da := Zeros(t.shape...)
		db := Zeros(o.shape...)
		for b := 0; b < batchCount; b++ {
			aOff := mapT[b] * coreSizeT
			bOff := mapO[b] * coreSizeO
			aCore := t.data[aOff : aOff+coreSizeT]
			bCore := o.data[bOff : bOff+coreSizeO]
			dcCore := out.grad.data[b*coreSizeC : b*coreSizeC+coreSizeC]
			// dA = dC @ B^T  -> (m,n) @ (n,k) = (m,k)
			daCore := matmul2d(dcCore, transpose2d(bCore, k, n), m, n, k)
			// dB = A^T @ dC  -> (k,m) @ (m,n) = (k,n)
			dbCore := matmul2d(transpose2d(aCore, m, k), dcCore, k, m, n)
			// Accumulate into the correct leading slice (sums over broadcast).
			addIntoStride(da.data, daCore, mapT[b], coreSizeT)
			addIntoStride(db.data, dbCore, mapO[b], coreSizeO)
		}
		accumulateGrad(t, da)
		accumulateGrad(o, db)
	})
	return out
}

// Dot returns the inner product of two rank-1 vectors of equal length.
func (t *Tensor) Dot(o *Tensor) *Tensor {
	if t.Dim() != 1 || o.Dim() != 1 || t.shape[0] != o.shape[0] {
		panicf("libtorch: Dot requires two rank-1 vectors of equal length, got %v and %v", t.shape, o.shape)
	}
	acc := 0.0
	for i := range t.data {
		acc += t.data[i] * o.data[i]
	}
	out := Scalar(acc)
	out.attachGrad([]*Tensor{t, o}, func() {
		accumulateGrad(t, out.grad.Mul(o))
		accumulateGrad(o, out.grad.Mul(t))
	})
	return out
}

// batchIndexMap maps each linear index of the broadcast batch shape to the
// linear index into a tensor's own leading dimensions, clamping size-1 dims.
func batchIndexMap(lead, batch []int) []int {
	batchCount := product(batch)
	out := make([]int, batchCount)
	if len(lead) == 0 {
		return out // all zeros
	}
	leadStrides := contiguousStrides(lead)
	off := len(batch) - len(lead)
	idx := make([]int, len(batch))
	for flat := 0; flat < batchCount; flat++ {
		leadIdx := 0
		for i := 0; i < len(lead); i++ {
			di := idx[off+i]
			if lead[i] == 1 {
				di = 0
			}
			leadIdx += di * leadStrides[i]
		}
		out[flat] = leadIdx
		// increment batch index
		for d := len(batch) - 1; d >= 0; d-- {
			idx[d]++
			if idx[d] < batch[d] {
				break
			}
			idx[d] = 0
		}
	}
	return out
}

// addIntoStride adds src (len n) into dst at leadIndex*n .. +n.
func addIntoStride(dst, src []float64, leadIndex, n int) {
	off := leadIndex * n
	for i := 0; i < n; i++ {
		dst[off+i] += src[i]
	}
}

func panicf(format string, args ...interface{}) {
	panic(fmt.Sprintf(format, args...))
}
