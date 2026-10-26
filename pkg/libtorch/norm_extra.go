// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package libtorch

import "math"

// GroupNorm normalizes over groups of channels for an (N, C, ...) input. Each
// sample is normalized independently: channels are split into numGroups groups,
// and each group (its channels plus all spatial positions) is normalized to zero
// mean and unit variance, then rescaled by per-channel gamma/beta.
func (t *Tensor) GroupNorm(gamma, beta *Tensor, numGroups int, eps float64) *Tensor {
	if eps == 0 {
		eps = 1e-5
	}
	if t.Dim() < 2 {
		panic("libtorch: GroupNorm expects rank >= 2 (N, C, ...)")
	}
	n, c := t.shape[0], t.shape[1]
	if numGroups <= 0 || c%numGroups != 0 {
		panicf("libtorch: GroupNorm channels %d not divisible by numGroups %d", c, numGroups)
	}
	cPerGroup := c / numGroups
	spatial := product(t.shape[2:])
	blockSize := cPerGroup * spatial
	C := c

	mean := make([]float64, n*numGroups)
	inv := make([]float64, n*numGroups)
	for gi := 0; gi < n*numGroups; gi++ {
		start := gi * blockSize
		m := 0.0
		for j := 0; j < blockSize; j++ {
			m += t.data[start+j]
		}
		m /= float64(blockSize)
		varc := 0.0
		for j := 0; j < blockSize; j++ {
			d := t.data[start+j] - m
			varc += d * d
		}
		varc /= float64(blockSize)
		mean[gi] = m
		inv[gi] = 1 / math.Sqrt(varc+eps)
	}

	out := Zeros(t.shape...)
	xhat := Zeros(t.shape...)
	gammaData := onesIfNil(gamma, C)
	betaData := onesIfNil(beta, C)
	if beta == nil {
		betaData = make([]float64, C)
	}
	for ni := 0; ni < n; ni++ {
		for g := 0; g < numGroups; g++ {
			gi := ni*numGroups + g
			start := ni*C*spatial + g*cPerGroup*spatial
			for j := 0; j < blockSize; j++ {
				cIdx := g*cPerGroup + j/spatial
				xh := (t.data[start+j] - mean[gi]) * inv[gi]
				xhat.data[start+j] = xh
				out.data[start+j] = gammaData[cIdx]*xh + betaData[cIdx]
			}
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
		da := make([]float64, t.Numel())
		var dgamma, dbeta []float64
		if gamma != nil {
			dgamma = make([]float64, C)
		}
		if beta != nil {
			dbeta = make([]float64, C)
		}
		for ni := 0; ni < n; ni++ {
			for g := 0; g < numGroups; g++ {
				gi := ni*numGroups + g
				start := ni*C*spatial + g*cPerGroup*spatial
				// Accumulate dxhat sums over the group.
				sum1, sum2 := 0.0, 0.0
				for j := 0; j < blockSize; j++ {
					cIdx := g*cPerGroup + j/spatial
					dxhat := out.grad.data[start+j] * gammaData[cIdx]
					sum1 += dxhat
					sum2 += dxhat * xhat.data[start+j]
					if gamma != nil {
						dgamma[cIdx] += out.grad.data[start+j] * xhat.data[start+j]
					}
					if beta != nil {
						dbeta[cIdx] += out.grad.data[start+j]
					}
				}
				m1 := sum1 / float64(blockSize)
				m2 := sum2 / float64(blockSize)
				for j := 0; j < blockSize; j++ {
					cIdx := g*cPerGroup + j/spatial
					dxhat := out.grad.data[start+j] * gammaData[cIdx]
					da[start+j] = inv[gi] * (dxhat - m1 - xhat.data[start+j]*m2)
				}
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

// InstanceNorm2d normalizes each channel of each sample independently over the
// spatial dimensions (equivalent to GroupNorm with numGroups == C).
func (t *Tensor) InstanceNorm2d(gamma, beta *Tensor, eps float64) *Tensor {
	if t.Dim() != 4 {
		panicf("libtorch: InstanceNorm2d expects NCHW rank-4, got %v", t.shape)
	}
	return t.GroupNorm(gamma, beta, t.shape[1], eps)
}

// InstanceNorm1d normalizes each channel of each sample over the last dimension.
func (t *Tensor) InstanceNorm1d(gamma, beta *Tensor, eps float64) *Tensor {
	if t.Dim() != 3 {
		panicf("libtorch: InstanceNorm1d expects (N, C, L), got %v", t.shape)
	}
	return t.GroupNorm(gamma, beta, t.shape[1], eps)
}
