// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package libtorch

import "math"

// Optimizer updates model parameters in place using their accumulated gradients.
type Optimizer interface {
	// Step applies one update step.
	Step()
	// ZeroGrad clears the gradient on every parameter.
	ZeroGrad()
}

// zeroAll clears gradients across params.
func zeroAll(params []*Tensor) {
	for _, p := range params {
		p.ZeroGrad()
	}
}

// -----------------------------------------------------------------------------
// SGD
// -----------------------------------------------------------------------------

// SGD is stochastic gradient descent with optional momentum and L2 weight
// decay applied to the gradient.
type SGD struct {
	Params      []*Tensor
	LR          float64
	Momentum    float64
	WeightDecay float64
	velocity    map[*Tensor][]float64
}

// NewSGD builds an SGD optimizer. A zero momentum disables momentum.
func NewSGD(params []*Tensor, lr, momentum, weightDecay float64) *SGD {
	return &SGD{Params: params, LR: lr, Momentum: momentum, WeightDecay: weightDecay, velocity: make(map[*Tensor][]float64)}
}

// ZeroGrad clears gradients on all parameters.
func (o *SGD) ZeroGrad() { zeroAll(o.Params) }

// Step applies one SGD update with momentum.
func (o *SGD) Step() {
	for _, p := range o.Params {
		if p.grad == nil {
			continue
		}
		g := p.grad.data
		if o.WeightDecay != 0 {
			for i := range g {
				g[i] += o.WeightDecay * p.data[i]
			}
		}
		vel, ok := o.velocity[p]
		if !ok {
			vel = make([]float64, len(p.data))
		}
		for i := range p.data {
			vel[i] = o.Momentum*vel[i] + g[i]
			p.data[i] -= o.LR * vel[i]
		}
		o.velocity[p] = vel
	}
}

// -----------------------------------------------------------------------------
// Adam / AdamW
// -----------------------------------------------------------------------------

// Adam is the Adam optimizer with bias correction and optional L2 weight decay.
type Adam struct {
	Params      []*Tensor
	LR          float64
	Beta1       float64
	Beta2       float64
	Eps         float64
	WeightDecay float64
	Decoupled   bool // true => AdamW
	m           map[*Tensor][]float64
	v           map[*Tensor][]float64
	t           int
}

// NewAdam builds an Adam optimizer (decoupled=false) or AdamW (decoupled=true).
func NewAdam(params []*Tensor, lr float64, decoupled bool) *Adam {
	return &Adam{
		Params: params, LR: lr,
		Beta1: 0.9, Beta2: 0.999, Eps: 1e-8,
		Decoupled: decoupled,
		m:         make(map[*Tensor][]float64),
		v:         make(map[*Tensor][]float64),
	}
}

// NewAdamW builds an AdamW (decoupled weight decay) optimizer.
func NewAdamW(params []*Tensor, lr float64) *Adam { return NewAdam(params, lr, true) }

// SetWeightDecay configures L2 (Adam) or decoupled (AdamW) weight decay.
func (o *Adam) SetWeightDecay(wd float64) { o.WeightDecay = wd }

// ZeroGrad clears gradients on all parameters.
func (o *Adam) ZeroGrad() { zeroAll(o.Params) }

// Step applies one Adam/AdamW update.
func (o *Adam) Step() {
	o.t++
	bc1 := 1 - math.Pow(o.Beta1, float64(o.t))
	bc2 := 1 - math.Pow(o.Beta2, float64(o.t))
	for _, p := range o.Params {
		if p.grad == nil {
			continue
		}
		if o.Decoupled && o.WeightDecay != 0 {
			for i := range p.data {
				p.data[i] -= o.LR * o.WeightDecay * p.data[i]
			}
		}
		g := p.grad.data
		if !o.Decoupled && o.WeightDecay != 0 {
			for i := range g {
				g[i] += o.WeightDecay * p.data[i]
			}
		}
		m, okM := o.m[p]
		if !okM {
			m = make([]float64, len(p.data))
		}
		v, okV := o.v[p]
		if !okV {
			v = make([]float64, len(p.data))
		}
		for i := range p.data {
			m[i] = o.Beta1*m[i] + (1-o.Beta1)*g[i]
			v[i] = o.Beta2*v[i] + (1-o.Beta2)*g[i]*g[i]
			mHat := m[i] / bc1
			vHat := v[i] / bc2
			p.data[i] -= o.LR * mHat / (math.Sqrt(vHat) + o.Eps)
		}
		o.m[p] = m
		o.v[p] = v
	}
}

// -----------------------------------------------------------------------------
// RMSprop
// -----------------------------------------------------------------------------

// RMSprop scales the learning rate by an exponential moving average of the
// squared gradient.
type RMSprop struct {
	Params      []*Tensor
	LR          float64
	Rho         float64
	Eps         float64
	Momentum    float64
	WeightDecay float64
	squareAvg   map[*Tensor][]float64
	gradAccum   map[*Tensor][]float64
}

// NewRMSprop builds an RMSprop optimizer.
func NewRMSprop(params []*Tensor, lr, rho, eps, momentum, weightDecay float64) *RMSprop {
	if rho == 0 {
		rho = 0.9
	}
	if eps == 0 {
		eps = 1e-8
	}
	return &RMSprop{
		Params: params, LR: lr, Rho: rho, Eps: eps, Momentum: momentum, WeightDecay: weightDecay,
		squareAvg: make(map[*Tensor][]float64),
		gradAccum: make(map[*Tensor][]float64),
	}
}

// ZeroGrad clears gradients on all parameters.
func (o *RMSprop) ZeroGrad() { zeroAll(o.Params) }

// Step applies one RMSprop update.
func (o *RMSprop) Step() {
	for _, p := range o.Params {
		if p.grad == nil {
			continue
		}
		g := p.grad.data
		if o.WeightDecay != 0 {
			for i := range g {
				g[i] += o.WeightDecay * p.data[i]
			}
		}
		sa, ok := o.squareAvg[p]
		if !ok {
			sa = make([]float64, len(p.data))
		}
		buf, okBuf := o.gradAccum[p]
		if !okBuf {
			buf = make([]float64, len(p.data))
		}
		for i := range p.data {
			sa[i] = o.Rho*sa[i] + (1-o.Rho)*g[i]*g[i]
			update := g[i] / (math.Sqrt(sa[i]) + o.Eps)
			if o.Momentum != 0 {
				buf[i] = o.Momentum*buf[i] + update
				p.data[i] -= o.LR * buf[i]
			} else {
				p.data[i] -= o.LR * update
			}
		}
		o.squareAvg[p] = sa
		o.gradAccum[p] = buf
	}
}
