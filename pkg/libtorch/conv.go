// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package libtorch

// Conv2d performs a 2-D convolution over NCHW input with a (C_out, C_in, kH, kW)
// kernel and optional per-output-channel bias. stride and padding are per-axis.
//
// Output shape is (N, C_out, H_out, W_out) where
//
//	H_out = (H + 2*padH - kH) / strideH + 1  (and likewise for W).
//
// Backward is exact and accumulates into dInput, dWeight, and dBias.
func Conv2d(input, weight, bias *Tensor, strideH, strideW, padH, padW int) *Tensor {
	if input.Dim() != 4 {
		panicf("libtorch: Conv2d input must be NCHW rank-4, got %v", input.shape)
	}
	if weight.Dim() != 4 {
		panicf("libtorch: Conv2d weight must be rank-4 (C_out,C_in,kH,kW), got %v", weight.shape)
	}
	n, cin, h, w := input.shape[0], input.shape[1], input.shape[2], input.shape[3]
	cout, cinW, kh, kw := weight.shape[0], weight.shape[1], weight.shape[2], weight.shape[3]
	if cin != cinW {
		panicf("libtorch: Conv2d channel mismatch: input C=%d weight C_in=%d", cin, cinW)
	}
	hOut := (h+2*padH-kh)/strideH + 1
	wOut := (w+2*padW-kw)/strideW + 1
	if hOut <= 0 || wOut <= 0 {
		panicf("libtorch: Conv2d produces empty output for input %v kernel %v", input.shape, weight.shape[2:])
	}

	inStrides := contiguousStrides(input.shape)
	wtStrides := contiguousStrides(weight.shape)
	outShape := []int{n, cout, hOut, wOut}
	out := Zeros(outShape...)
	outStrides := contiguousStrides(outShape)

	var biasData []float64
	if bias != nil {
		biasData = bias.data
	}

	inData := input.data
	wtData := weight.data

	// Forward.
	for ni := 0; ni < n; ni++ {
		for co := 0; co < cout; co++ {
			b := 0.0
			if biasData != nil {
				b = biasData[co]
			}
			for oh := 0; oh < hOut; oh++ {
				for ow := 0; ow < wOut; ow++ {
					acc := b
					ihBase := oh*strideH - padH
					iwBase := ow*strideW - padW
					for ci := 0; ci < cin; ci++ {
						for khh := 0; khh < kh; khh++ {
							ih := ihBase + khh
							if ih < 0 || ih >= h {
								continue
							}
							inRow := ni*inStrides[0] + ci*inStrides[1] + ih*inStrides[2]
							wtRow := co*wtStrides[0] + ci*wtStrides[1] + khh*wtStrides[2]
							for kww := 0; kww < kw; kww++ {
								iw := iwBase + kww
								if iw < 0 || iw >= w {
									continue
								}
								acc += inData[inRow+iw*inStrides[3]] * wtData[wtRow+kww*wtStrides[3]]
							}
						}
					}
					out.data[ni*outStrides[0]+co*outStrides[1]+oh*outStrides[2]+ow*outStrides[3]] = acc
				}
			}
		}
	}

	parents := []*Tensor{input, weight}
	if bias != nil {
		parents = append(parents, bias)
	}
	out.attachGrad(parents, func() {
		gData := out.grad.data
		dIn := make([]float64, input.Numel())
		dWt := make([]float64, weight.Numel())
		var dBs []float64
		if bias != nil {
			dBs = make([]float64, bias.Numel())
		}
		for ni := 0; ni < n; ni++ {
			for co := 0; co < cout; co++ {
				for oh := 0; oh < hOut; oh++ {
					for ow := 0; ow < wOut; ow++ {
						g := gData[ni*outStrides[0]+co*outStrides[1]+oh*outStrides[2]+ow*outStrides[3]]
						if dBs != nil {
							dBs[co] += g
						}
						ihBase := oh*strideH - padH
						iwBase := ow*strideW - padW
						for ci := 0; ci < cin; ci++ {
							for khh := 0; khh < kh; khh++ {
								ih := ihBase + khh
								if ih < 0 || ih >= h {
									continue
								}
								inRow := ni*inStrides[0] + ci*inStrides[1] + ih*inStrides[2]
								wtRow := co*wtStrides[0] + ci*wtStrides[1] + khh*wtStrides[2]
								for kww := 0; kww < kw; kww++ {
									iw := iwBase + kww
									if iw < 0 || iw >= w {
										continue
									}
									inIdx := inRow + iw*inStrides[3]
									wtIdx := wtRow + kww*wtStrides[3]
									dIn[inIdx] += wtData[wtIdx] * g
									dWt[wtIdx] += inData[inIdx] * g
								}
							}
						}
					}
				}
			}
		}
		accumulateGrad(input, &Tensor{data: dIn, shape: cloneInts(input.shape)})
		accumulateGrad(weight, &Tensor{data: dWt, shape: cloneInts(weight.shape)})
		if bias != nil {
			accumulateGrad(bias, &Tensor{data: dBs, shape: cloneInts(bias.shape)})
		}
	})
	return out
}

// MaxPool2d performs max pooling over NCHW input with the given kernel and
// stride. Backward routes gradients to the argmax within each window.
func (t *Tensor) MaxPool2d(kH, kW, strideH, strideW int) *Tensor {
	if t.Dim() != 4 {
		panicf("libtorch: MaxPool2d input must be NCHW rank-4, got %v", t.shape)
	}
	nc, c, h, w := t.shape[0], t.shape[1], t.shape[2], t.shape[3]
	hOut := (h-kH)/strideH + 1
	wOut := (w-kW)/strideW + 1
	if hOut <= 0 || wOut <= 0 {
		panicf("libtorch: MaxPool2d produces empty output for input %v kernel %v", t.shape[2:], []int{kH, kW})
	}
	inStrides := contiguousStrides(t.shape)
	outShape := []int{nc, c, hOut, wOut}
	out := Zeros(outShape...)
	outStrides := contiguousStrides(outShape)

	bestIdx := make([]int, out.Numel())
	iterate(outShape, func(oflat int, oidx []int) {
		ni, coi, oh, ow := oidx[0], oidx[1], oidx[2], oidx[3]
		maxv := t.data[ni*inStrides[0]+coi*inStrides[1]+oh*strideH*inStrides[2]+ow*strideW*inStrides[3]]
		best := ni*inStrides[0] + coi*inStrides[1] + oh*strideH*inStrides[2] + ow*strideW*inStrides[3]
		for khh := 0; khh < kH; khh++ {
			ih := oh*strideH + khh
			if ih >= h {
				break
			}
			for kww := 0; kww < kW; kww++ {
				iw := ow*strideW + kww
				if iw >= w {
					break
				}
				idx := ni*inStrides[0] + coi*inStrides[1] + ih*inStrides[2] + iw*inStrides[3]
				if v := t.data[idx]; v > maxv {
					maxv = v
					best = idx
				}
			}
		}
		out.data[oflat] = maxv
		bestIdx[oflat] = best
		_ = outStrides
	})

	inShape := t.shape
	out.attachGrad([]*Tensor{t}, func() {
		da := make([]float64, t.Numel())
		for oflat := range out.grad.data {
			da[bestIdx[oflat]] += out.grad.data[oflat]
		}
		accumulateGrad(t, &Tensor{data: da, shape: cloneInts(inShape)})
	})
	return out
}
