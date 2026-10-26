// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package libtorch

import (
	"math"
	"math/rand"
)

// Module is a composable neural-network component with a forward pass and a set
// of trainable parameters.
type Module interface {
	// Forward runs the module's computation on the input tensor.
	Forward(x *Tensor) *Tensor
	// Parameters returns the trainable parameter tensors.
	Parameters() []*Tensor
}

// randomUniform fills a new tensor of the given shape with U(-bound, bound).
func randomUniform(bound float64, shape ...int) *Tensor {
	t := Zeros(shape...)
	for i := range t.data {
		t.data[i] = (rand.Float64()*2 - 1) * bound
	}
	return t
}

// kaimingBound returns the Kaiming-uniform initialization bound for fanIn.
func kaimingBound(fanIn int) float64 { return 1 / math.Sqrt(float64(fanIn)) }

// -----------------------------------------------------------------------------
// Linear
// -----------------------------------------------------------------------------

// Linear is a dense layer y = x @ W^T + b.
type Linear struct {
	Weight *Tensor // (out, in)
	Bias   *Tensor // (out,) or nil
}

// NewLinear creates a Linear layer with Kaiming-uniform weight init and zero
// bias (when bias is true).
func NewLinear(inFeatures, outFeatures int, bias bool) *Linear {
	w := randomUniform(kaimingBound(inFeatures), outFeatures, inFeatures).WithGrad()
	l := &Linear{Weight: w}
	if bias {
		l.Bias = Zeros(outFeatures).WithGrad()
	}
	return l
}

// Forward computes y = x @ W^T + b.
func (l *Linear) Forward(x *Tensor) *Tensor {
	y := x.MatMul(l.Weight.Transpose())
	if l.Bias != nil {
		y = y.Add(l.Bias)
	}
	return y
}

// Parameters returns the weight (and bias, if present).
func (l *Linear) Parameters() []*Tensor {
	if l.Bias != nil {
		return []*Tensor{l.Weight, l.Bias}
	}
	return []*Tensor{l.Weight}
}

// -----------------------------------------------------------------------------
// Conv2d
// -----------------------------------------------------------------------------

// Conv2dLayer is a 2-D convolution layer over NCHW inputs.
type Conv2dLayer struct {
	Weight           *Tensor // (C_out, C_in, kH, kW)
	Bias             *Tensor // (C_out,) or nil
	StrideH, StrideW int
	PadH, PadW       int
}

// NewConv2d creates a Conv2dLayer with the given kernel size, stride, and
// padding (stride/padding are applied to both spatial axes).
func NewConv2d(inChannels, outChannels, kH, kW, stride, padding int, bias bool) *Conv2dLayer {
	bound := kaimingBound(inChannels * kH * kW)
	w := randomUniform(bound, outChannels, inChannels, kH, kW).WithGrad()
	c := &Conv2dLayer{Weight: w, StrideH: stride, StrideW: stride, PadH: padding, PadW: padding}
	if bias {
		c.Bias = Zeros(outChannels).WithGrad()
	}
	return c
}

// Forward runs the convolution.
func (c *Conv2dLayer) Forward(x *Tensor) *Tensor {
	return Conv2d(x, c.Weight, c.Bias, c.StrideH, c.StrideW, c.PadH, c.PadW)
}

// Parameters returns the weight (and bias, if present).
func (c *Conv2dLayer) Parameters() []*Tensor {
	if c.Bias != nil {
		return []*Tensor{c.Weight, c.Bias}
	}
	return []*Tensor{c.Weight}
}

// -----------------------------------------------------------------------------
// Activations
// -----------------------------------------------------------------------------

// ActKind enumerates the supported stateless activations.
type ActKind int

const (
	// ActReLU is the rectified linear unit.
	ActReLU ActKind = iota
	// ActSigmoid is the logistic sigmoid.
	ActSigmoid
	// ActTanh is the hyperbolic tangent.
	ActTanh
	// ActLeakyReLU is LeakyReLU with a fixed negative slope.
	ActLeakyReLU
	// ActSoftmax is the softmax over the last dimension.
	ActSoftmax
	// ActLogSoftmax is the log-softmax over the last dimension.
	ActLogSoftmax
)

// Activation wraps a stateless activation as a Module.
type Activation struct {
	Kind  ActKind
	Slope float64 // used by ActLeakyReLU
}

// NewReLU creates a ReLU activation module.
func NewReLU() *Activation { return &Activation{Kind: ActReLU} }

// NewSigmoid creates a Sigmoid activation module.
func NewSigmoid() *Activation { return &Activation{Kind: ActSigmoid} }

// NewTanh creates a Tanh activation module.
func NewTanh() *Activation { return &Activation{Kind: ActTanh} }

// NewLeakyReLU creates a LeakyReLU activation module with the given slope.
func NewLeakyReLU(slope float64) *Activation { return &Activation{Kind: ActLeakyReLU, Slope: slope} }

// NewSoftmax creates a Softmax activation module over the last dimension.
func NewSoftmax() *Activation { return &Activation{Kind: ActSoftmax} }

// NewLogSoftmax creates a LogSoftmax activation module over the last dimension.
func NewLogSoftmax() *Activation { return &Activation{Kind: ActLogSoftmax} }

// Forward applies the activation.
func (a *Activation) Forward(x *Tensor) *Tensor {
	switch a.Kind {
	case ActReLU:
		return x.Relu()
	case ActSigmoid:
		return x.Sigmoid()
	case ActTanh:
		return x.Tanh()
	case ActLeakyReLU:
		return x.LeakyRelu(a.Slope)
	case ActSoftmax:
		return x.Softmax()
	case ActLogSoftmax:
		return x.LogSoftmax()
	default:
		panic("libtorch: unknown activation kind")
	}
}

// Parameters returns no trainable parameters.
func (a *Activation) Parameters() []*Tensor { return nil }

// -----------------------------------------------------------------------------
// LayerNorm
// -----------------------------------------------------------------------------

// LayerNorm normalizes over the last dimension with learnable gain and bias.
type LayerNorm struct {
	Gamma *Tensor
	Beta  *Tensor
	Eps   float64
}

// NewLayerNorm creates a LayerNorm over a final dimension of the given size.
func NewLayerNorm(size int, eps float64) *LayerNorm {
	if eps == 0 {
		eps = 1e-5
	}
	return &LayerNorm{Gamma: Ones(size).WithGrad(), Beta: Zeros(size).WithGrad(), Eps: eps}
}

// Forward applies layer normalization.
func (l *LayerNorm) Forward(x *Tensor) *Tensor { return x.LayerNorm(l.Gamma, l.Beta, l.Eps) }

// Parameters returns gamma and beta.
func (l *LayerNorm) Parameters() []*Tensor { return []*Tensor{l.Gamma, l.Beta} }

// -----------------------------------------------------------------------------
// BatchNorm
// -----------------------------------------------------------------------------

// BatchNorm is a batch-normalization layer wrapping shared running statistics.
type BatchNorm struct {
	Gamma       *Tensor
	Beta        *Tensor
	Stats       *BatchNormStats
	ChannelAxis int
	ReduceAxes  []int
	Training    bool
}

// NewBatchNorm2d creates a BatchNorm layer for NCHW inputs (normalizes channel
// dim 1 over N, H, W).
func NewBatchNorm2d(numFeatures int) *BatchNorm {
	return &BatchNorm{
		Gamma: Ones(numFeatures).WithGrad(), Beta: Zeros(numFeatures).WithGrad(),
		Stats:       NewBatchNormStats(numFeatures, 0.1, 1e-5),
		ChannelAxis: 1, ReduceAxes: []int{0, 2, 3}, Training: true,
	}
}

// NewBatchNorm1d creates a BatchNorm layer for (N, C) inputs (normalizes channel
// dim 1 over N).
func NewBatchNorm1d(numFeatures int) *BatchNorm {
	return &BatchNorm{
		Gamma: Ones(numFeatures).WithGrad(), Beta: Zeros(numFeatures).WithGrad(),
		Stats:       NewBatchNormStats(numFeatures, 0.1, 1e-5),
		ChannelAxis: 1, ReduceAxes: []int{0}, Training: true,
	}
}

// SetTraining toggles training (batch statistics) vs inference (running
// statistics) mode.
func (b *BatchNorm) SetTraining(v bool) { b.Training = v }

// Forward applies batch normalization.
func (b *BatchNorm) Forward(x *Tensor) *Tensor {
	return x.BatchNorm(b.Gamma, b.Beta, b.Stats, b.ChannelAxis, b.ReduceAxes, b.Training)
}

// Parameters returns gamma and beta.
func (b *BatchNorm) Parameters() []*Tensor { return []*Tensor{b.Gamma, b.Beta} }

// -----------------------------------------------------------------------------
// Dropout
// -----------------------------------------------------------------------------

// Dropout randomly zeroes elements during training.
type Dropout struct {
	P        float64
	Training bool
}

// NewDropout creates a Dropout layer with drop probability p.
func NewDropout(p float64) *Dropout { return &Dropout{P: p, Training: true} }

// Forward applies dropout.
func (d *Dropout) Forward(x *Tensor) *Tensor { return x.Dropout(d.P, d.Training) }

// Parameters returns no trainable parameters.
func (d *Dropout) Parameters() []*Tensor { return nil }

// -----------------------------------------------------------------------------
// Sequential
// -----------------------------------------------------------------------------

// Sequential chains modules in order.
type Sequential struct {
	Layers []Module
}

// NewSequential creates a Sequential container from the given modules.
func NewSequential(layers ...Module) *Sequential { return &Sequential{Layers: layers} }

// Forward runs the input through each layer in order.
func (s *Sequential) Forward(x *Tensor) *Tensor {
	for _, l := range s.Layers {
		x = l.Forward(x)
	}
	return x
}

// Parameters concatenates the parameters of every child module.
func (s *Sequential) Parameters() []*Tensor {
	var out []*Tensor
	for _, l := range s.Layers {
		out = append(out, l.Parameters()...)
	}
	return out
}

// Add appends a module to the end of the sequence.
func (s *Sequential) Add(m Module) { s.Layers = append(s.Layers, m) }

// CollectParams returns the parameters of an arbitrary Module (including nested
// Sequential containers).
func CollectParams(m Module) []*Tensor { return m.Parameters() }
