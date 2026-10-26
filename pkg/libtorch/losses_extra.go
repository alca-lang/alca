// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package libtorch

// KLDivLoss computes the Kullback-Leibler divergence between input (log-
// probabilities) and target (probabilities), averaged over all elements:
//
//	L = mean(target * (log(target) - input))
//
// This is the standard form applied to a log-softmax output.
func KLDivLoss(input, target *Tensor) *Tensor {
	diff := target.Log().Sub(input)
	return target.Mul(diff).Mean(false)
}

// HuberLoss (smooth L1) is quadratic for small residuals and linear beyond
// delta (default 1).
func HuberLoss(pred, target *Tensor, delta float64) *Tensor {
	if delta == 0 {
		delta = 1
	}
	diff := pred.Sub(target)
	abs := diff.Abs()
	quad := diff.Mul(diff).MulScalar(0.5)
	lin := abs.SubScalar(0.5 * delta).MulScalar(delta)
	mask := abs.LeValue(delta)
	return Where(mask, quad, lin).Mean(false)
}

// SmoothL1Loss is an alias for HuberLoss with delta=1.
func SmoothL1Loss(pred, target *Tensor) *Tensor { return HuberLoss(pred, target, 1) }

// CosineSimilarity computes cosine similarity along the last dimension:
//
//	sim = (x·y) / (||x|| * ||y||)
//
// A small eps guards against zero-norm denominators. The result keeps the last
// axis as size 1.
func CosineSimilarity(x, y *Tensor, eps float64) *Tensor {
	if eps == 0 {
		eps = 1e-8
	}
	last := []int{x.Dim() - 1}
	dot := x.Mul(y).Sum(true, last...)
	nx := x.Mul(x).Sum(true, last...).Sqrt()
	ny := y.Mul(y).Sum(true, last...).Sqrt()
	return dot.Div(nx.Mul(ny).AddScalar(eps))
}

// CosineEmbeddingLoss: target==1 encourages similarity, target==-1 pushes it
// below margin. L = 1-sim (pos) or max(0, sim-margin) (neg).
func CosineEmbeddingLoss(x1, x2, target *Tensor, margin float64) *Tensor {
	sim := CosineSimilarity(x1, x2, 1e-8)
	pos := fullLike(sim, 1).Sub(sim)
	neg := sim.SubScalar(margin).ClampMin(0)
	return Where(target.GtValue(0), pos, neg).Mean(false)
}

// MarginRankingLoss: L = max(0, -target*(x1-x2) + margin).
func MarginRankingLoss(x1, x2, target *Tensor, margin float64) *Tensor {
	term := target.Mul(x1.Sub(x2)).SubScalar(margin).Neg()
	return term.ClampMin(0).Mean(false)
}

// SoftMarginLoss: L = softplus(-target*input), target in {+1,-1}.
func SoftMarginLoss(input, target *Tensor) *Tensor {
	return target.Mul(input).Neg().Softplus().Mean(false)
}

// MultiLabelSoftMarginLoss treats each class as an independent binary problem:
// L = BCEWithLogits(logits, target), averaged.
func MultiLabelSoftMarginLoss(logits, target *Tensor) *Tensor {
	return BCEWithLogitsLoss(logits, target)
}

// PoissonNLLLoss: L = exp(input) - target*input (elementwise, mean).
func PoissonNLLLoss(input, target *Tensor) *Tensor {
	return input.Exp().Sub(target.Mul(input)).Mean(false)
}

// FocalLoss is the multi-class focal loss over (N,C) logits:
//
//	L = -alpha_t * (1-pt)^gamma * log(pt)
//
// where pt is the softmax probability of the target class and targets holds
// integer class indices. alpha weights the target class (1-alpha for class 0).
func FocalLoss(logits, targets *Tensor, gamma, alpha float64) *Tensor {
	if logits.Dim() != 2 {
		panicf("libtorch: FocalLoss expects (N,C) logits, got %v", logits.shape)
	}
	n, c := logits.shape[0], logits.shape[1]
	softmax := logits.Softmax(1)
	logProb := softmax.Log()
	oneHot := Zeros(n, c)
	weights := Zeros(n, 1)
	for i := 0; i < n; i++ {
		cls := int(targets.data[i])
		if cls < 0 || cls >= c {
			panicf("libtorch: FocalLoss class %d out of range [0,%d)", cls, c)
		}
		oneHot.data[i*c+cls] = 1
		if cls == 0 {
			weights.data[i] = 1 - alpha
		} else {
			weights.data[i] = alpha
		}
	}
	// Probability and log-probability of the target class, both (n,1).
	pt := oneHot.Mul(softmax).Sum(true, 1)
	logPt := oneHot.Mul(logProb).Sum(true, 1)
	// (1-pt)^gamma via exp(gamma * log(1-pt)) so it stays differentiable.
	oneMinus := pt.SubScalar(1).Neg()
	factor := oneMinus.Log().MulScalar(gamma).Exp()
	return factor.Mul(logPt).Mul(weights).Neg().Mean(false)
}

// LeValue returns a 0/1 mask that is 1 where t <= v (no gradient).
func (t *Tensor) LeValue(v float64) *Tensor {
	out := Zeros(t.shape...)
	for i, x := range t.data {
		if x <= v {
			out.data[i] = 1
		}
	}
	return out
}

// GtValue returns a 0/1 mask that is 1 where t > v (no gradient).
func (t *Tensor) GtValue(v float64) *Tensor {
	out := Zeros(t.shape...)
	for i, x := range t.data {
		if x > v {
			out.data[i] = 1
		}
	}
	return out
}

// ClampMin clamps every element to be at least minVal.
func (t *Tensor) ClampMin(minVal float64) *Tensor {
	return t.unaryByElement(
		func(x, _ float64) float64 {
			if x < minVal {
				return minVal
			}
			return x
		},
		func(x, _ float64) float64 {
			if x < minVal {
				return 0
			}
			return 1
		},
	)
}

// ClampMax clamps every element to be at most maxVal.
func (t *Tensor) ClampMax(maxVal float64) *Tensor {
	return t.unaryByElement(
		func(x, _ float64) float64 {
			if x > maxVal {
				return maxVal
			}
			return x
		},
		func(x, _ float64) float64 {
			if x > maxVal {
				return 0
			}
			return 1
		},
	)
}
