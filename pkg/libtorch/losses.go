// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package libtorch

import "math"

// MSELoss returns the mean squared error between pred and target, averaged over
// every element.
func MSELoss(pred, target *Tensor) *Tensor {
	diff := pred.Sub(target)
	sq := diff.Mul(diff)
	n := float64(sq.Numel())
	return sq.SumAll().DivScalar(n)
}

// MSELoss method on Tensor for convenience.
func (t *Tensor) MSELoss(target *Tensor) *Tensor {
	return MSELoss(t, target)
}

// L1Loss returns the mean absolute error between pred and target.
func L1Loss(pred, target *Tensor) *Tensor {
	diff := pred.Sub(target).Abs()
	n := float64(diff.Numel())
	return diff.SumAll().DivScalar(n)
}

// CrossEntropyLoss computes the mean cross-entropy between logits (N, C) and an
// integer class-index target (N). It is numerically stable (log-sum-exp).
func CrossEntropyLoss(logits, targets *Tensor) *Tensor {
	if logits.Dim() != 2 {
		panicf("libtorch: CrossEntropyLoss expects logits of shape (N,C), got %v", logits.shape)
	}
	if targets.Dim() != 1 || targets.shape[0] != logits.shape[0] {
		panicf("libtorch: CrossEntropyLoss targets must be (N,), got %v for logits %v", targets.shape, logits.shape)
	}
	n, c := logits.shape[0], logits.shape[1]
	outer, inner, dim, stride, base := axisLayout(logits.shape, 1)
	_ = outer
	_ = inner
	_ = dim

	// Per-row logsumexp and gather target logit.
	lse := make([]float64, n)
	targetLogit := make([]float64, n)
	targetIdx := make([]int, n)
	for i := 0; i < n; i++ {
		b := base(i, 0)
		maxv := math.Inf(-1)
		for j := 0; j < c; j++ {
			if v := logits.data[b+j*stride]; v > maxv {
				maxv = v
			}
		}
		s := 0.0
		for j := 0; j < c; j++ {
			s += math.Exp(logits.data[b+j*stride] - maxv)
		}
		lse[i] = math.Log(s) + maxv
		ti := int(targets.data[i])
		if ti < 0 || ti >= c {
			panicf("libtorch: CrossEntropyLoss target index %d out of range for C=%d", ti, c)
		}
		targetIdx[i] = ti
		targetLogit[i] = logits.data[b+ti*stride]
	}

	// loss = mean_i (lse_i - targetLogit_i)
	loss := 0.0
	for i := 0; i < n; i++ {
		loss += lse[i] - targetLogit[i]
	}
	loss /= float64(n)
	out := Scalar(loss)

	inShape := logits.shape
	out.attachGrad([]*Tensor{logits}, func() {
		g := out.grad.Item()
		da := make([]float64, logits.Numel())
		scale := g / float64(n)
		for i := 0; i < n; i++ {
			b := base(i, 0)
			// softmax row
			rowMax := math.Inf(-1)
			for j := 0; j < c; j++ {
				if v := logits.data[b+j*stride]; v > rowMax {
					rowMax = v
				}
			}
			rowSum := 0.0
			expRow := make([]float64, c)
			for j := 0; j < c; j++ {
				expRow[j] = math.Exp(logits.data[b+j*stride] - rowMax)
				rowSum += expRow[j]
			}
			for j := 0; j < c; j++ {
				sm := expRow[j] / rowSum
				if j == targetIdx[i] {
					sm -= 1
				}
				da[b+j*stride] = scale * sm
			}
		}
		accumulateGrad(logits, &Tensor{data: da, shape: cloneInts(inShape)})
	})
	return out
}

// NLLLoss computes the mean negative log-likelihood given log-probabilities
// (N, C) and integer class targets (N).
func NLLLoss(logProbs, targets *Tensor) *Tensor {
	if logProbs.Dim() != 2 {
		panicf("libtorch: NLLLoss expects log-probs of shape (N,C), got %v", logProbs.shape)
	}
	n, c := logProbs.shape[0], logProbs.shape[1]
	_, _, _, stride, base := axisLayout(logProbs.shape, 1)
	targetIdx := make([]int, n)
	total := 0.0
	for i := 0; i < n; i++ {
		b := base(i, 0)
		ti := int(targets.data[i])
		if ti < 0 || ti >= c {
			panicf("libtorch: NLLLoss target index %d out of range for C=%d", ti, c)
		}
		targetIdx[i] = ti
		total -= logProbs.data[b+ti*stride]
	}
	out := Scalar(total / float64(n))
	inShape := logProbs.shape
	out.attachGrad([]*Tensor{logProbs}, func() {
		g := out.grad.Item() / float64(n)
		da := make([]float64, logProbs.Numel())
		for i := 0; i < n; i++ {
			b := base(i, 0)
			da[b+targetIdx[i]*stride] += g
		}
		accumulateGrad(logProbs, &Tensor{data: da, shape: cloneInts(inShape)})
	})
	return out
}

// BCEWithLogitsLoss computes binary cross-entropy with logits, averaged over
// elements. target should be 0/1 valued and broadcastable to logits.
func BCEWithLogitsLoss(logits, target *Tensor) *Tensor {
	// Stable: max(x,0) - x*y + log(1+exp(-|x|))
	negAbs := logits.Abs().Neg()
	term := logits.Maximum(fullLike(logits, 0)).Sub(logits.Mul(target)).Add(negAbs.Exp().AddScalar(1).Log())
	n := float64(term.Numel())
	return term.SumAll().DivScalar(n)
}

// SoftmaxCrossEntropy computes softmax followed by cross-entropy in one step
// for a (N, C) logits tensor and (N,) integer targets. Equivalent to
// CrossEntropyLoss but exposed as a combined helper.
func SoftmaxCrossEntropy(logits, targets *Tensor) *Tensor {
	return CrossEntropyLoss(logits, targets)
}
