// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package libtorch

import "math"

// ClipGradNorm_ clips the total gradient norm of the given parameters to
// maxNorm (in-place). Returns the (pre-clip) total norm.
func ClipGradNorm(params []*Tensor, maxNorm float64) float64 {
	total := 0.0
	for _, p := range params {
		if p.grad == nil {
			continue
		}
		for _, g := range p.grad.data {
			total += g * g
		}
	}
	total = math.Sqrt(total)
	if total > maxNorm && total > 0 {
		scale := maxNorm / total
		for _, p := range params {
			if p.grad == nil {
				continue
			}
			for i := range p.grad.data {
				p.grad.data[i] *= scale
			}
		}
	}
	return total
}

// ClipGradValue_ clamps every gradient element into [-clipValue, clipValue].
func ClipGradValue(params []*Tensor, clipValue float64) {
	for _, p := range params {
		if p.grad == nil {
			continue
		}
		for i, g := range p.grad.data {
			if g > clipValue {
				p.grad.data[i] = clipValue
			} else if g < -clipValue {
				p.grad.data[i] = -clipValue
			}
		}
	}
}

// StepLR multiplies the optimizer's learning rate by gamma every stepSize
// epochs. Call it once per epoch with the current epoch number.
func StepLR(opt Optimizer, epoch, stepSize int, gamma float64) {
	if stepSize <= 0 || epoch%stepSize != 0 || epoch == 0 {
		return
	}
	switch o := opt.(type) {
	case *SGD:
		o.LR *= gamma
	case *Adam:
		o.LR *= gamma
	case *RMSprop:
		o.LR *= gamma
	case *AdaGrad:
		o.LR *= gamma
	case *AdaDelta:
		// AdaDelta has no LR; no-op
	case *NAdam:
		o.LR *= gamma
	}
}

// CosineAnnealingLR sets the optimizer LR following a cosine schedule over
// T_max epochs, with a minimum LR of etaMin.
func CosineAnnealingLR(opt Optimizer, epoch, TMax int, etaMin, baseLR float64) {
	if TMax <= 0 {
		return
	}
	if epoch > TMax {
		epoch = TMax
	}
	lr := etaMin + 0.5*(baseLR-etaMin)*(1+math.Cos(math.Pi*float64(epoch)/float64(TMax)))
	setLR(opt, lr)
}

// ReduceLROnPlateau halves the LR when the monitored metric hasn't improved
// for `patience` epochs. State is tracked by the caller via bestMetric/counter.
func ReduceLROnPlateau(opt Optimizer, metric float64, best *float64, counter *int, patience int, factor float64) bool {
	if *best == 0 || metric < *best {
		*best = metric
		*counter = 0
		return false
	}
	*counter++
	if *counter >= patience {
		mulLR(opt, factor)
		*counter = 0
		return true
	}
	return false
}

func setLR(opt Optimizer, lr float64) {
	switch o := opt.(type) {
	case *SGD:
		o.LR = lr
	case *Adam:
		o.LR = lr
	case *RMSprop:
		o.LR = lr
	case *AdaGrad:
		o.LR = lr
	case *NAdam:
		o.LR = lr
	}
}

func mulLR(opt Optimizer, factor float64) {
	switch o := opt.(type) {
	case *SGD:
		o.LR *= factor
	case *Adam:
		o.LR *= factor
	case *RMSprop:
		o.LR *= factor
	case *AdaGrad:
		o.LR *= factor
	case *NAdam:
		o.LR *= factor
	}
}
