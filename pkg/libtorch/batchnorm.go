// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package libtorch

import "math"

// BatchNormStats holds the running mean/variance and hyperparameters shared by
// a batch-normalization layer across calls.
type BatchNormStats struct {
	RunningMean []float64
	RunningVar  []float64
	Momentum    float64
	Eps         float64
}

// NewBatchNormStats allocates running statistics for numFeatures channels.
func NewBatchNormStats(numFeatures int, momentum, eps float64) *BatchNormStats {
	if momentum == 0 {
		momentum = 0.1
	}
	if eps == 0 {
		eps = 1e-5
	}
	return &BatchNormStats{
		RunningMean: make([]float64, numFeatures),
		RunningVar:  make([]float64, numFeatures),
		Momentum:    momentum,
		Eps:         eps,
	}
}

// BatchNorm normalizes t over the given reduceAxes independently per channel
// located at channelAxis. During training it uses batch statistics and updates
// stats.RunningMean/Var; during inference it uses the running statistics.
//
// For 2-D inputs use channelAxis=1, reduceAxes={0,2,3} (NCHW). For 1-D (N,C)
// inputs use channelAxis=1, reduceAxes={0}.
func (t *Tensor) BatchNorm(gamma, beta *Tensor, stats *BatchNormStats, channelAxis int, reduceAxes []int, training bool) *Tensor {
	if channelAxis < 0 {
		channelAxis += t.Dim()
	}
	numChannels := t.shape[channelAxis]
	eps := stats.Eps

	// total[i] = number of elements reduced per channel i.
	counts := make([]int, numChannels)
	sum := make([]float64, numChannels)
	sumSq := make([]float64, numChannels)
	chanStride := contiguousStrides(t.shape)[channelAxis]

	// Pass 1: per-channel sums.
	iterate(t.shape, func(flat int, cur []int) {
		c := cur[channelAxis]
		v := t.data[flat]
		sum[c] += v
		sumSq[c] += v * v
		counts[c]++
	})
	mean := make([]float64, numChannels)
	varc := make([]float64, numChannels)
	std := make([]float64, numChannels)
	inv := make([]float64, numChannels)
	for c := 0; c < numChannels; c++ {
		n := float64(counts[c])
		mean[c] = sum[c] / n
		varc[c] = sumSq[c]/n - mean[c]*mean[c]
		if varc[c] < 0 {
			varc[c] = 0
		}
		if training {
			std[c] = math.Sqrt(varc[c] + eps)
			inv[c] = 1 / std[c]
			// Update running statistics (unbiased-ish; matches torch's simple form).
			stats.RunningMean[c] = (1-stats.Momentum)*stats.RunningMean[c] + stats.Momentum*mean[c]
			stats.RunningVar[c] = (1-stats.Momentum)*stats.RunningVar[c] + stats.Momentum*varc[c]
		} else {
			std[c] = math.Sqrt(stats.RunningVar[c] + eps)
			inv[c] = 1 / std[c]
			mean[c] = stats.RunningMean[c]
		}
	}

	out := Zeros(t.shape...)
	xhat := Zeros(t.shape...)
	gammaData := onesIfNil(gamma, numChannels)
	betaData := onesIfNil(beta, numChannels)
	if beta == nil {
		betaData = make([]float64, numChannels)
	}
	iterate(t.shape, func(flat int, cur []int) {
		c := cur[channelAxis]
		xh := (t.data[flat] - mean[c]) * inv[c]
		xhat.data[flat] = xh
		out.data[flat] = gammaData[c]*xh + betaData[c]
	})

	parents := []*Tensor{t}
	if gamma != nil {
		parents = append(parents, gamma)
	}
	if beta != nil {
		parents = append(parents, beta)
	}
	inShape := t.shape
	_ = chanStride
	out.attachGrad(parents, func() {
		da := make([]float64, len(t.data))
		var dgamma []float64
		var dbeta []float64
		if gamma != nil {
			dgamma = make([]float64, numChannels)
		}
		if beta != nil {
			dbeta = make([]float64, numChannels)
		}
		if training {
			// Per-channel partials for the batchnorm backward formula. The
			// gradient w.r.t. xhat is dxhat = g * gamma; the reduction means
			// must be taken over dxhat (not raw g).
			gSum := make([]float64, numChannels)
			gXhatSum := make([]float64, numChannels)
			iterate(t.shape, func(flat int, cur []int) {
				c := cur[channelAxis]
				dxhat := out.grad.data[flat] * gammaData[c]
				gSum[c] += dxhat
				gXhatSum[c] += dxhat * xhat.data[flat]
			})
			iterate(t.shape, func(flat int, cur []int) {
				c := cur[channelAxis]
				n := float64(counts[c])
				dxhat := out.grad.data[flat] * gammaData[c]
				dx := inv[c] * (dxhat - gSum[c]/n - xhat.data[flat]*gXhatSum[c]/n)
				da[flat] = dx
			})
		} else {
			// Inference: treat xhat as constant w.r.t. batch; only route to gamma/beta.
			iterate(t.shape, func(flat int, cur []int) {
				c := cur[channelAxis]
				da[flat] = out.grad.data[flat] * gammaData[c] * inv[c]
			})
		}
		// Gamma/beta grads (both modes).
		if gamma != nil || beta != nil {
			iterate(t.shape, func(flat int, cur []int) {
				c := cur[channelAxis]
				g := out.grad.data[flat]
				if gamma != nil {
					dgamma[c] += g * xhat.data[flat]
				}
				if beta != nil {
					dbeta[c] += g
				}
			})
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
