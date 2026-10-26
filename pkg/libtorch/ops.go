// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package libtorch

import "math"

// binaryForward broadcasts a and b to a common shape and applies fn elementwise,
// returning the output plus the shared operands used.
func binaryForward(a, b *Tensor, fn func(float64, float64) float64) (*Tensor, *Tensor, *Tensor, []int) {
	shape, err := broadcastShapes(a.shape, b.shape)
	if err != nil {
		panic(err)
	}
	ab := a.broadcastTo(shape)
	bb := b.broadcastTo(shape)
	out := Zeros(shape...)
	for i := range out.data {
		out.data[i] = fn(ab.data[i], bb.data[i])
	}
	return out, ab, bb, shape
}

// Add returns a + b with broadcasting and autograd.
func (t *Tensor) Add(o *Tensor) *Tensor {
	out, _, _, _ := binaryForward(t, o, func(a, b float64) float64 { return a + b })
	out.attachGrad([]*Tensor{t, o}, func() {
		accumulateGrad(t, out.grad)
		accumulateGrad(o, out.grad)
	})
	return out
}

// Sub returns a - b with broadcasting and autograd.
func (t *Tensor) Sub(o *Tensor) *Tensor {
	out, _, _, _ := binaryForward(t, o, func(a, b float64) float64 { return a - b })
	out.attachGrad([]*Tensor{t, o}, func() {
		accumulateGrad(t, out.grad)
		accumulateGrad(o, out.grad.Neg())
	})
	return out
}

// Mul returns a * b with broadcasting and autograd.
func (t *Tensor) Mul(o *Tensor) *Tensor {
	out, ab, bb, _ := binaryForward(t, o, func(a, b float64) float64 { return a * b })
	out.attachGrad([]*Tensor{t, o}, func() {
		accumulateGrad(t, out.grad.Mul(bb))
		accumulateGrad(o, out.grad.Mul(ab))
	})
	return out
}

// Div returns a / b with broadcasting and autograd.
func (t *Tensor) Div(o *Tensor) *Tensor {
	out, ab, bb, _ := binaryForward(t, o, func(a, b float64) float64 { return a / b })
	out.attachGrad([]*Tensor{t, o}, func() {
		accumulateGrad(t, out.grad.Div(bb))
		// d/db (a/b) = -a / b^2
		accumulateGrad(o, out.grad.Mul(ab).Neg().Div(bb.Mul(bb)))
	})
	return out
}

// Pow returns a ** b (elementwise) with broadcasting and autograd.
func (t *Tensor) Pow(o *Tensor) *Tensor {
	out, ab, bb, _ := binaryForward(t, o, math.Pow)
	out.attachGrad([]*Tensor{t, o}, func() {
		// d/da (a^b) = b * a^(b-1)
		da := out.grad.Mul(bb).Mul(ab.Pow(bb.SubScalar(1)))
		accumulateGrad(t, da)
		// d/db (a^b) = a^b * ln(a)
		db := out.grad.Mul(out).Mul(ab.Log())
		accumulateGrad(o, db)
	})
	return out
}

// -----------------------------------------------------------------------------
// Unary ops
// -----------------------------------------------------------------------------

// unaryForward applies fn elementwise, returning the output and the input view.
func unaryForward(t *Tensor, fn func(float64) float64) (*Tensor, *Tensor) {
	out := Zeros(t.shape...)
	for i, v := range t.data {
		out.data[i] = fn(v)
	}
	return out, t
}

// Neg returns -t.
func (t *Tensor) Neg() *Tensor {
	out, _ := unaryForward(t, func(v float64) float64 { return -v })
	out.attachGrad([]*Tensor{t}, func() {
		accumulateGrad(t, out.grad.Neg())
	})
	return out
}

// Exp returns e^t.
func (t *Tensor) Exp() *Tensor {
	out, _ := unaryForward(t, math.Exp)
	out.attachGrad([]*Tensor{t}, func() {
		accumulateGrad(t, out.grad.Mul(out))
	})
	return out
}

// Log returns the natural logarithm of t.
func (t *Tensor) Log() *Tensor {
	out, _ := unaryForward(t, math.Log)
	out.attachGrad([]*Tensor{t}, func() {
		accumulateGrad(t, out.grad.Div(t))
	})
	return out
}

// Sqrt returns the elementwise square root.
func (t *Tensor) Sqrt() *Tensor {
	out, _ := unaryForward(t, math.Sqrt)
	out.attachGrad([]*Tensor{t}, func() {
		// d/dx sqrt(x) = 1 / (2 sqrt(x))
		accumulateGrad(t, out.grad.Div(out.MulScalar(2)))
	})
	return out
}

// Relu returns max(0, t).
func (t *Tensor) Relu() *Tensor {
	out, _ := unaryForward(t, func(v float64) float64 {
		if v > 0 {
			return v
		}
		return 0
	})
	out.attachGrad([]*Tensor{t}, func() {
		accumulateGrad(t, out.grad.Mul(t.gtZeroMask()))
	})
	return out
}

// Sigmoid returns 1 / (1 + e^-t).
func (t *Tensor) Sigmoid() *Tensor {
	out, _ := unaryForward(t, func(v float64) float64 { return 1 / (1 + math.Exp(-v)) })
	out.attachGrad([]*Tensor{t}, func() {
		// s * (1 - s)
		oneMinus := fullLike(out, 1).Sub(out)
		accumulateGrad(t, out.grad.Mul(out).Mul(oneMinus))
	})
	return out
}

// Tanh returns the hyperbolic tangent of t.
func (t *Tensor) Tanh() *Tensor {
	out, _ := unaryForward(t, math.Tanh)
	out.attachGrad([]*Tensor{t}, func() {
		oneMinusSq := fullLike(out, 1).Sub(out.Mul(out))
		accumulateGrad(t, out.grad.Mul(oneMinusSq))
	})
	return out
}

// Abs returns the elementwise absolute value.
func (t *Tensor) Abs() *Tensor {
	out, _ := unaryForward(t, math.Abs)
	out.attachGrad([]*Tensor{t}, func() {
		accumulateGrad(t, out.grad.Mul(t.signMask()))
	})
	return out
}

// LeakyRelu returns x if x > 0 else slope*x.
func (t *Tensor) LeakyRelu(slope float64) *Tensor {
	out, _ := unaryForward(t, func(v float64) float64 {
		if v > 0 {
			return v
		}
		return slope * v
	})
	out.attachGrad([]*Tensor{t}, func() {
		accumulateGrad(t, out.grad.Mul(t.leakyReluMask(slope)))
	})
	return out
}

// -----------------------------------------------------------------------------
// Scalar conveniences
// -----------------------------------------------------------------------------

// AddScalar returns t + v.
func (t *Tensor) AddScalar(v float64) *Tensor { return t.Add(Full(v, t.shape...)) }

// SubScalar returns t - v.
func (t *Tensor) SubScalar(v float64) *Tensor { return t.Sub(Full(v, t.shape...)) }

// MulScalar returns t * v.
func (t *Tensor) MulScalar(v float64) *Tensor { return t.Mul(Full(v, t.shape...)) }

// DivScalar returns t / v.
func (t *Tensor) DivScalar(v float64) *Tensor { return t.Div(Full(v, t.shape...)) }

// PowScalar returns t ** v.
func (t *Tensor) PowScalar(v float64) *Tensor { return t.Pow(Full(v, t.shape...)) }

// PowTensor is an alias for Pow used internally for clarity.
func (t *Tensor) PowTensor(o *Tensor) *Tensor { return t.Pow(o) }

// -----------------------------------------------------------------------------
// Mask helpers used by backward passes (raw, no autograd)
// -----------------------------------------------------------------------------

func (t *Tensor) gtZeroMask() *Tensor {
	out := Zeros(t.shape...)
	for i, v := range t.data {
		if v > 0 {
			out.data[i] = 1
		}
	}
	return out
}

func (t *Tensor) signMask() *Tensor {
	out := Zeros(t.shape...)
	for i, v := range t.data {
		switch {
		case v > 0:
			out.data[i] = 1
		case v < 0:
			out.data[i] = -1
		}
	}
	return out
}

func (t *Tensor) leakyReluMask(slope float64) *Tensor {
	out := Zeros(t.shape...)
	for i, v := range t.data {
		if v > 0 {
			out.data[i] = 1
		} else {
			out.data[i] = slope
		}
	}
	return out
}

// fullLike returns a tensor of the given value with t's shape (raw, no autograd).
func fullLike(t *Tensor, v float64) *Tensor { return Full(v, t.shape...) }
