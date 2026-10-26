// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package libtorch

// ConvTranspose2d performs a transposed convolution (deconvolution) over NCHW
// input. Weight is (C_in, C_out, kH, kW) — note the channel ordering differs from
// Conv2d, matching PyTorch. stride, padding, and outputPadding control the
// output geometry:
//
//	H_out = (H-1)*stride - 2*pad + kH + outputPad
//
// Backward is exact.
func ConvTranspose2d(input, weight, bias *Tensor, strideH, strideW, padH, padW, outPadH, outPadW int) *Tensor {
	if input.Dim() != 4 || weight.Dim() != 4 {
		panicf("libtorch: ConvTranspose2d expects rank-4 input and weight, got %v and %v", input.shape, weight.shape)
	}
	n, cin, h, w := input.shape[0], input.shape[1], input.shape[2], input.shape[3]
	cinW, cout, kh, kw := weight.shape[0], weight.shape[1], weight.shape[2], weight.shape[3]
	if cin != cinW {
		panicf("libtorch: ConvTranspose2d channel mismatch: input C=%d weight C_in=%d", cin, cinW)
	}
	hOut := (h-1)*strideH - 2*padH + kh + outPadH
	wOut := (w-1)*strideW - 2*padW + kw + outPadW
	if hOut <= 0 || wOut <= 0 {
		panicf("libtorch: ConvTranspose2d produces empty output for input %v", input.shape)
	}

	inStrides := contiguousStrides(input.shape)
	wtStrides := contiguousStrides(weight.shape)
	outShape := []int{n, cout, hOut, wOut}
	out := Zeros(outShape...)
	outStrides := contiguousStrides(outShape)

	inData := input.data
	wtData := weight.data
	var biasData []float64
	if bias != nil {
		biasData = bias.data
	}

	// Forward: scatter each input through the kernel.
	for ni := 0; ni < n; ni++ {
		for ci := 0; ci < cin; ci++ {
			for ih := 0; ih < h; ih++ {
				for iw := 0; iw < w; iw++ {
					val := inData[ni*inStrides[0]+ci*inStrides[1]+ih*inStrides[2]+iw*inStrides[3]]
					ohBase := ih*strideH - padH
					owBase := iw*strideW - padW
					for co := 0; co < cout; co++ {
						wtRow := ci*wtStrides[0] + co*wtStrides[1]
						for khh := 0; khh < kh; khh++ {
							oh := ohBase + khh
							if oh < 0 || oh >= hOut {
								continue
							}
							for kww := 0; kww < kw; kww++ {
								ow := owBase + kww
								if ow < 0 || ow >= wOut {
									continue
								}
								out.data[ni*outStrides[0]+co*outStrides[1]+oh*outStrides[2]+ow*outStrides[3]] +=
									val * wtData[wtRow+khh*wtStrides[2]+kww*wtStrides[3]]
							}
						}
					}
				}
			}
		}
	}
	// Add bias across each output channel.
	if biasData != nil {
		for ni := 0; ni < n; ni++ {
			for co := 0; co < cout; co++ {
				ohBase := ni*outStrides[0] + co*outStrides[1]
				for p := 0; p < hOut*wOut; p++ {
					out.data[ohBase+p] += biasData[co]
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
			for ci := 0; ci < cin; ci++ {
				for ih := 0; ih < h; ih++ {
					for iw := 0; iw < w; iw++ {
						inIdx := ni*inStrides[0] + ci*inStrides[1] + ih*inStrides[2] + iw*inStrides[3]
						ohBase := ih*strideH - padH
						owBase := iw*strideW - padW
						acc := 0.0
						for co := 0; co < cout; co++ {
							wtRow := ci*wtStrides[0] + co*wtStrides[1]
							for khh := 0; khh < kh; khh++ {
								oh := ohBase + khh
								if oh < 0 || oh >= hOut {
									continue
								}
								for kww := 0; kww < kw; kww++ {
									ow := owBase + kww
									if ow < 0 || ow >= wOut {
										continue
									}
									g := gData[ni*outStrides[0]+co*outStrides[1]+oh*outStrides[2]+ow*outStrides[3]]
									wtIdx := wtRow + khh*wtStrides[2] + kww*wtStrides[3]
									acc += wtData[wtIdx] * g
									dWt[wtIdx] += inData[inIdx] * g
								}
							}
						}
						dIn[inIdx] = acc
					}
				}
			}
		}
		if dBs != nil {
			for ni := 0; ni < n; ni++ {
				for co := 0; co < cout; co++ {
					ohBase := ni*outStrides[0] + co*outStrides[1]
					for p := 0; p < hOut*wOut; p++ {
						dBs[co] += gData[ohBase+p]
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

// UpsampleNearest2d upsamples NCHW input by integer factors using nearest-
// neighbor interpolation. Gradients route to the nearest source pixel.
func (t *Tensor) UpsampleNearest2d(scaleH, scaleW int) *Tensor {
	if t.Dim() != 4 {
		panicf("libtorch: UpsampleNearest2d expects NCHW rank-4, got %v", t.shape)
	}
	if scaleH <= 0 || scaleW <= 0 {
		panic("libtorch: Upsample scale must be positive")
	}
	n, c, h, w := t.shape[0], t.shape[1], t.shape[2], t.shape[3]
	hOut, wOut := h*scaleH, w*scaleW
	outShape := []int{n, c, hOut, wOut}
	out := Zeros(outShape...)
	inStrides := contiguousStrides(t.shape)
	outStrides := contiguousStrides(outShape)
	// Map each output to nearest input.
	srcIdx := make([]int, out.Numel())
	iterate(outShape, func(oflat int, oidx []int) {
		ih := oidx[2] / scaleH
		iw := oidx[3] / scaleW
		iflat := oidx[0]*inStrides[0] + oidx[1]*inStrides[1] + ih*inStrides[2] + iw*inStrides[3]
		out.data[oflat] = t.data[iflat]
		srcIdx[oflat] = iflat
		_ = outStrides
	})
	inShape := t.shape
	out.attachGrad([]*Tensor{t}, func() {
		da := make([]float64, t.Numel())
		for oflat := range out.grad.data {
			da[srcIdx[oflat]] += out.grad.data[oflat]
		}
		accumulateGrad(t, &Tensor{data: da, shape: cloneInts(inShape)})
	})
	return out
}
