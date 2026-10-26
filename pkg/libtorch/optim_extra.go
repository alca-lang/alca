// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package libtorch

import "math"

// This file adds AdaGrad, AdaDelta and NAdam optimizers, mirroring the
// existing SGD/Adam/AdamW/RMSprop pattern.

// AdaGrad optimizer.
type AdaGrad struct {
	Params      []*Tensor
	LR          float64
	Eps         float64
	WeightDecay float64
	G           map[int][]float64
}

// NewAdaGrad builds an AdaGrad optimizer.
func NewAdaGrad(params []*Tensor, lr, eps, weightDecay float64) *AdaGrad {
	if eps == 0 {
		eps = 1e-10
	}
	return &AdaGrad{Params: params, LR: lr, Eps: eps, WeightDecay: weightDecay, G: map[int][]float64{}}
}

// ZeroGrad clears gradients.
func (o *AdaGrad) ZeroGrad() { zeroAll(o.Params) }

// Step applies one AdaGrad update.
func (o *AdaGrad) Step() {
	for pi, p := range o.Params {
		if p.grad == nil {
			continue
		}
		g := p.grad.data
		gAcc := o.G[pi]
		if gAcc == nil {
			gAcc = make([]float64, len(g))
			o.G[pi] = gAcc
		}
		for i, gi := range g {
			gAcc[i] += gi * gi
			grad := gi
			if o.WeightDecay != 0 {
				grad += o.WeightDecay * p.data[i]
			}
			p.data[i] -= o.LR * grad / (math.Sqrt(gAcc[i]) + o.Eps)
		}
	}
}

// AdaDelta optimizer.
type AdaDelta struct {
	Params      []*Tensor
	Rho         float64
	Eps         float64
	WeightDecay float64
	G           map[int][]float64
	Delta       map[int][]float64
}

// NewAdaDelta builds an AdaDelta optimizer.
func NewAdaDelta(params []*Tensor, rho, eps, weightDecay float64) *AdaDelta {
	if eps == 0 {
		eps = 1e-6
	}
	if rho == 0 {
		rho = 0.9
	}
	return &AdaDelta{Params: params, Rho: rho, Eps: eps, WeightDecay: weightDecay, G: map[int][]float64{}, Delta: map[int][]float64{}}
}

// ZeroGrad clears gradients.
func (o *AdaDelta) ZeroGrad() { zeroAll(o.Params) }

// Step applies one AdaDelta update.
func (o *AdaDelta) Step() {
	for pi, p := range o.Params {
		if p.grad == nil {
			continue
		}
		g := p.grad.data
		gAcc := o.G[pi]
		if gAcc == nil {
			gAcc = make([]float64, len(g))
			o.G[pi] = gAcc
		}
		dAcc := o.Delta[pi]
		if dAcc == nil {
			dAcc = make([]float64, len(g))
			o.Delta[pi] = dAcc
		}
		for i, gi := range g {
			grad := gi
			if o.WeightDecay != 0 {
				grad += o.WeightDecay * p.data[i]
			}
			gAcc[i] = o.Rho*gAcc[i] + (1-o.Rho)*grad*grad
			rmsG := math.Sqrt(gAcc[i] + o.Eps)
			rmsD := math.Sqrt(dAcc[i] + o.Eps)
			dx := -rmsD / rmsG * grad
			dAcc[i] = o.Rho*dAcc[i] + (1-o.Rho)*dx*dx
			p.data[i] += dx
		}
	}
}

// NAdam optimizer (Nesterov-accelerated Adam).
type NAdam struct {
	Params      []*Tensor
	LR          float64
	Beta1       float64
	Beta2       float64
	Eps         float64
	WeightDecay float64
	Decoupled   bool
	T           int
	M           map[int][]float64
	V           map[int][]float64
}

// NewNAdam builds a NAdam optimizer.
func NewNAdam(params []*Tensor, lr float64, decoupled bool) *NAdam {
	return &NAdam{Params: params, LR: lr, Beta1: 0.9, Beta2: 0.999, Eps: 1e-8, Decoupled: decoupled, M: map[int][]float64{}, V: map[int][]float64{}}
}

// SetWeightDecay sets the weight decay.
func (o *NAdam) SetWeightDecay(wd float64) { o.WeightDecay = wd }

// ZeroGrad clears gradients.
func (o *NAdam) ZeroGrad() { zeroAll(o.Params) }

// Step applies one NAdam update.
func (o *NAdam) Step() {
	o.T++
	b1 := o.Beta1
	b2 := o.Beta2
	lr := o.LR * math.Sqrt(1-math.Pow(b2, float64(o.T))) / (1 - math.Pow(b1, float64(o.T)))
	for pi, p := range o.Params {
		if p.grad == nil {
			continue
		}
		g := p.grad.data
		m := o.M[pi]
		if m == nil {
			m = make([]float64, len(g))
			o.M[pi] = m
		}
		v := o.V[pi]
		if v == nil {
			v = make([]float64, len(g))
			o.V[pi] = v
		}
		for i, gi := range g {
			grad := gi
			if o.WeightDecay != 0 && !o.Decoupled {
				grad += o.WeightDecay * p.data[i]
			}
			m[i] = b1*m[i] + (1-b1)*grad
			v[i] = b2*v[i] + (1-b2)*grad*grad
			mhat := m[i]/(1-math.Pow(b1, float64(o.T))) + (1-b1)*grad/(1-math.Pow(b1, float64(o.T)))
			vhat := v[i] / (1 - math.Pow(b2, float64(o.T)))
			p.data[i] -= lr * mhat / (math.Sqrt(vhat) + o.Eps)
			if o.WeightDecay != 0 && o.Decoupled {
				p.data[i] -= o.LR * o.WeightDecay * p.data[i]
			}
		}
	}
}
