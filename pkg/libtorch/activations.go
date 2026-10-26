// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package libtorch

import (
	"math"
)

const sqrtTwo = 1.4142135623730951
const invSqrtTwoPi = 0.3989422804014327

// normPDF is the standard normal probability density function.
func normPDF(x float64) float64 { return invSqrtTwoPi * math.Exp(-0.5*x*x) }

// normCDF is the standard normal cumulative distribution function.
func normCDF(x float64) float64 { return 0.5 * (1 + math.Erf(x/sqrtTwo)) }

// unaryByElement applies fn to produce the output and deriv to produce the local
// gradient multiplier for each input element.
func (t *Tensor) unaryByElement(fn, deriv func(x, y float64) float64) *Tensor {
	out := Zeros(t.shape...)
	for i, v := range t.data {
		out.data[i] = fn(v, 0)
	}
	inShape := t.shape
	out.attachGrad([]*Tensor{t}, func() {
		da := make([]float64, len(t.data))
		for i, v := range t.data {
			da[i] = out.grad.data[i] * deriv(v, out.data[i])
		}
		accumulateGrad(t, &Tensor{data: da, shape: cloneInts(inShape)})
	})
	return out
}

// GELU returns the Gaussian Error Linear Unit. With approx=true it uses the fast
// tanh approximation; otherwise the exact erf form.
func (t *Tensor) GELU(approx bool) *Tensor {
	if approx {
		k := math.Sqrt(2 / math.Pi)
		return t.unaryByElement(
			func(x, _ float64) float64 { return 0.5 * x * (1 + math.Tanh(k*(x+0.044715*x*x*x))) },
			func(x, y float64) float64 {
				// Derivative of the tanh approximation.
				tk := math.Tanh(k * (x + 0.044715*x*x*x))
				sech2 := 1 - tk*tk
				return 0.5*(1+tk) + 0.5*x*sech2*k*(1+3*0.044715*x*x)
			},
		)
	}
	return t.unaryByElement(
		func(x, _ float64) float64 { return x * normCDF(x) },
		func(x, _ float64) float64 { return normCDF(x) + x*normPDF(x) },
	)
}

// ELU returns the exponential linear unit with the given alpha.
func (t *Tensor) ELU(alpha float64) *Tensor {
	if alpha == 0 {
		alpha = 1
	}
	return t.unaryByElement(
		func(x, _ float64) float64 {
			if x > 0 {
				return x
			}
			return alpha * (math.Exp(x) - 1)
		},
		func(x, y float64) float64 {
			if x > 0 {
				return 1
			}
			return y + alpha
		},
	)
}

// SELU returns the scaled exponential linear unit (scaled so it is self-
// normalized), with the standard alpha=1.6732632423543772848170429916717 and
// scale=1.0507009873554804934193349852946.
func (t *Tensor) SELU() *Tensor {
	const alpha = 1.6732632423543772
	const scale = 1.0507009873554805
	return t.unaryByElement(
		func(x, _ float64) float64 {
			if x > 0 {
				return scale * x
			}
			return scale * alpha * (math.Exp(x) - 1)
		},
		func(x, y float64) float64 {
			if x > 0 {
				return scale
			}
			return y + scale*alpha
		},
	)
}

// SiLU returns the Sigmoid Linear Unit (swish): x * sigmoid(x).
func (t *Tensor) SiLU() *Tensor {
	return t.unaryByElement(
		func(x, _ float64) float64 { return x / (1 + math.Exp(-x)) },
		func(x, _ float64) float64 {
			s := 1 / (1 + math.Exp(-x))
			return s * (1 + x*(1-s))
		},
	)
}

// Swish is an alias for SiLU with an optional learnable beta fixed to 1.
func (t *Tensor) Swish() *Tensor { return t.SiLU() }

// Softplus returns log(1 + e^x).
func (t *Tensor) Softplus() *Tensor {
	return t.unaryByElement(
		func(x, _ float64) float64 {
			if x > 30 {
				return x
			}
			return math.Log1p(math.Exp(x))
		},
		func(x, _ float64) float64 { return 1 / (1 + math.Exp(-x)) },
	)
}

// Mish returns x * tanh(softplus(x)).
func (t *Tensor) Mish() *Tensor {
	return t.unaryByElement(
		func(x, _ float64) float64 { return x * math.Tanh(log1pexp(x)) },
		func(x, _ float64) float64 {
			sp := log1pexp(x)
			th := math.Tanh(sp)
			return th + x*(1-th*th)/(1+math.Exp(-x))
		},
	)
}

// Hardtanh clamps values to [minVal, maxVal].
func (t *Tensor) Hardtanh(minVal, maxVal float64) *Tensor {
	return t.unaryByElement(
		func(x, _ float64) float64 { return math.Max(minVal, math.Min(maxVal, x)) },
		func(x, _ float64) float64 {
			if x > minVal && x < maxVal {
				return 1
			}
			return 0
		},
	)
}

// ReLU6 clamps values to [0, 6].
func (t *Tensor) ReLU6() *Tensor {
	return t.Hardtanh(0, 6)
}

// Hardswish returns x * clamp(x+3, 0, 6) / 6.
func (t *Tensor) Hardswish() *Tensor {
	return t.unaryByElement(
		func(x, _ float64) float64 { return x * math.Max(0, math.Min(6, x+3)) / 6 },
		func(x, _ float64) float64 {
			switch {
			case x <= -3:
				return 0
			case x >= 3:
				return 1
			default:
				return (2*x + 3) / 6
			}
		},
	)
}

// HardSigmoid returns clamp(0.2*x + 0.5, 0, 1).
func (t *Tensor) HardSigmoid() *Tensor {
	return t.unaryByElement(
		func(x, _ float64) float64 { return math.Max(0, math.Min(1, 0.2*x+0.5)) },
		func(x, _ float64) float64 {
			if 0.2*x+0.5 > 0 && 0.2*x+0.5 < 1 {
				return 0.2
			}
			return 0
		},
	)
}

// Softsign returns x / (1 + |x|).
func (t *Tensor) Softsign() *Tensor {
	return t.unaryByElement(
		func(x, _ float64) float64 { return x / (1 + math.Abs(x)) },
		func(x, _ float64) float64 {
			d := 1 + math.Abs(x)
			return 1 / (d * d)
		},
	)
}

// log1pexp is a numerically stable softplus.
func log1pexp(x float64) float64 {
	if x > 30 {
		return x
	}
	if x < -30 {
		return math.Exp(x)
	}
	return math.Log1p(math.Exp(x))
}

// Erf returns the elementwise error function.
func (t *Tensor) Erf() *Tensor {
	return t.unaryByElement(
		func(x, _ float64) float64 { return math.Erf(x) },
		func(x, _ float64) float64 { return 2 * invSqrtTwoPi * math.Exp(-x*x) },
	)
}

// Clamp clamps every element into [minVal, maxVal]. A bound of +Inf/-Inf leaves
// that side unclamped.
func (t *Tensor) Clamp(minVal, maxVal float64) *Tensor {
	return t.unaryByElement(
		func(x, _ float64) float64 { return math.Max(minVal, math.Min(maxVal, x)) },
		func(x, _ float64) float64 {
			if x >= minVal && x <= maxVal {
				return 1
			}
			return 0
		},
	)
}

// Clip is an alias for Clamp.
func (t *Tensor) Clip(minVal, maxVal float64) *Tensor { return t.Clamp(minVal, maxVal) }

// Round rounds to the nearest integer (half away from zero). The gradient is
// treated as zero almost everywhere (straight-through is not applied).
func (t *Tensor) Round() *Tensor {
	return t.unaryByElement(
		func(x, _ float64) float64 { return math.Round(x) },
		func(x, _ float64) float64 { return 0 },
	)
}

// Floor returns the largest integer <= x.
func (t *Tensor) Floor() *Tensor {
	return t.unaryByElement(
		func(x, _ float64) float64 { return math.Floor(x) },
		func(x, _ float64) float64 { return 0 },
	)
}

// Ceil returns the smallest integer >= x.
func (t *Tensor) Ceil() *Tensor {
	return t.unaryByElement(
		func(x, _ float64) float64 { return math.Ceil(x) },
		func(x, _ float64) float64 { return 0 },
	)
}

// Sign returns -1, 0, or 1 depending on the sign of x.
func (t *Tensor) Sign() *Tensor {
	return t.unaryByElement(
		func(x, _ float64) float64 { return math.Copysign(1, x) },
		func(x, _ float64) float64 { return 0 },
	)
}

// Frac returns the fractional part of x (x - floor(x)).
func (t *Tensor) Frac() *Tensor {
	return t.unaryByElement(
		func(x, _ float64) float64 { return x - math.Floor(x) },
		func(x, _ float64) float64 { return 1 },
	)
}
