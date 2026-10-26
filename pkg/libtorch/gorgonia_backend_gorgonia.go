// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

//go:build gorgonia

package libtorch

import (
	"gorgonia.org/tensor"
)

// toGorgonia converts a libtorch.Tensor to a *tensor.Dense backed by the
// same float64 slice (zero-copy view).
func (t *Tensor) toGorgonia() *tensor.Dense {
	shape := make(tensor.Shape, len(t.shape))
	for i, d := range t.shape {
		shape[i] = d
	}
	return tensor.NewDense(tensor.Float64, shape, tensor.WithBacking(t.data))
}

func fromGorgoniaDense(d *tensor.Dense) *Tensor {
	shape := make([]int, len(d.Shape()))
	for i, s := range d.Shape() {
		shape[i] = s
	}
	data := make([]float64, d.Size())
	copy(data, d.Float64s())
	return &Tensor{data: data, shape: shape}
}

func matMulGorgonia(a, b *Tensor) (*Tensor, bool) {
	if a.Dim() < 2 || b.Dim() < 2 {
		return nil, false
	}
	if a.Dim() == 2 && b.Dim() == 2 {
		ad := a.toGorgonia()
		bd := b.toGorgonia()
		rd, err := ad.MatMul(bd)
		if err != nil {
			return nil, false
		}
		return fromGorgoniaDense(rd), true
	}
	return nil, false
}
