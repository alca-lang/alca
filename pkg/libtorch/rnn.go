// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package libtorch

// This file implements simple (non-cudnn) recurrent cells and layers composed
// from existing differentiable ops, so autograd flows through time automatically.

// RNNCell runs a single Elman RNN step: h' = act(W_ih x + b_ih + W_hh h + b_hh).
// Weight shapes are (H, I) and (H, H). tanhAct selects tanh (vs ReLU).
func RNNCell(x, h *Tensor, wIH, bIH, wHH, bHH *Tensor, tanhAct bool) *Tensor {
	pre := x.MatMul(wIH.Transpose())
	if bIH != nil {
		pre = pre.Add(bIH)
	}
	hh := h.MatMul(wHH.Transpose())
	if bHH != nil {
		hh = hh.Add(bHH)
	}
	s := pre.Add(hh)
	if tanhAct {
		return s.Tanh()
	}
	return s.Relu()
}

// LSTMCell runs a single LSTM step, returning the new (h, c). Weight shapes are
// (4H, I) and (4H, H) with gate order (i, f, g, o).
func LSTMCell(x, h, c *Tensor, wIH, bIH, wHH, bHH *Tensor) (*Tensor, *Tensor) {
	gates := x.MatMul(wIH.Transpose()).Add(h.MatMul(wHH.Transpose()))
	if bIH != nil {
		gates = gates.Add(bIH)
	}
	if bHH != nil {
		gates = gates.Add(bHH)
	}
	hSize := h.shape[len(h.shape)-1]
	i := gates.Narrow(-1, 0, hSize).Sigmoid()
	f := gates.Narrow(-1, hSize, hSize).Sigmoid()
	g := gates.Narrow(-1, 2*hSize, hSize).Tanh()
	o := gates.Narrow(-1, 3*hSize, hSize).Sigmoid()
	cNew := f.Mul(c).Add(i.Mul(g))
	hNew := o.Mul(cNew.Tanh())
	return hNew, cNew
}

// GRUCell runs a single GRU step. Weight shapes are (3H, I) and (3H, H) with
// gate order (r, z, n).
func GRUCell(x, h *Tensor, wIH, bIH, wHH, bHH *Tensor) *Tensor {
	gates := x.MatMul(wIH.Transpose())
	if bIH != nil {
		gates = gates.Add(bIH)
	}
	hSize := h.shape[len(h.shape)-1]
	rPart := h.MatMul(wHH.Transpose())
	if bHH != nil {
		rPart = rPart.Add(bHH)
	}
	rGate := gates.Narrow(-1, 0, hSize).Add(rPart.Narrow(-1, 0, hSize)).Sigmoid()
	zGate := gates.Narrow(-1, hSize, hSize).Add(rPart.Narrow(-1, hSize, hSize)).Sigmoid()
	nGate := gates.Narrow(-1, 2*hSize, hSize).Add(rPart.Narrow(-1, 2*hSize, hSize).Mul(rGate)).Tanh()
	return nGate.MulScalar(1).Mul(fullLike(nGate, 1).Sub(zGate)).Add(h.Mul(zGate))
}

// RNN is a multi-step Elman RNN over a (batch, seq, input) sequence.
type RNN struct {
	InputSize, HiddenSize int
	Tanh                  bool
	WIH, WHH              *Tensor
	BIH, BHH              *Tensor
}

// NewRNN builds a simple RNN layer.
func NewRNN(inputSize, hiddenSize int, tanh bool, bias bool) *RNN {
	r := &RNN{
		InputSize: inputSize, HiddenSize: hiddenSize, Tanh: tanh,
		WIH: randomUniform(kaimingBound(inputSize), hiddenSize, inputSize).WithGrad(),
		WHH: randomUniform(kaimingBound(hiddenSize), hiddenSize, hiddenSize).WithGrad(),
	}
	if bias {
		r.BIH = Zeros(hiddenSize).WithGrad()
		r.BHH = Zeros(hiddenSize).WithGrad()
	}
	return r
}

// Forward runs the sequence, returning (batch, seq, hidden).
func (r *RNN) Forward(x *Tensor) *Tensor {
	b, s := x.shape[0], x.shape[1]
	h := Zeros(b, r.HiddenSize)
	var outs []*Tensor
	for t := 0; t < s; t++ {
		xt := x.Narrow(1, t, 1).Reshape(b, r.InputSize)
		h = RNNCell(xt, h, r.WIH, r.BIH, r.WHH, r.BHH, r.Tanh)
		outs = append(outs, h.Unsqueeze(1))
	}
	return Concat(1, outs...)
}

// Parameters returns the trainable weights and biases.
func (r *RNN) Parameters() []*Tensor {
	params := []*Tensor{r.WIH, r.WHH}
	if r.BIH != nil {
		params = append(params, r.BIH, r.BHH)
	}
	return params
}

// LSTM is a multi-step LSTM over a (batch, seq, input) sequence.
type LSTM struct {
	InputSize, HiddenSize int
	WIH, WHH              *Tensor
	BIH, BHH              *Tensor
}

// NewLSTM builds an LSTM layer.
func NewLSTM(inputSize, hiddenSize int, bias bool) *LSTM {
	l := &LSTM{
		InputSize: inputSize, HiddenSize: hiddenSize,
		WIH: randomUniform(kaimingBound(inputSize), 4*hiddenSize, inputSize).WithGrad(),
		WHH: randomUniform(kaimingBound(hiddenSize), 4*hiddenSize, hiddenSize).WithGrad(),
	}
	if bias {
		// Torch adds a consistent 1 to the forget-gate bias; approximate with zeros.
		l.BIH = Zeros(4 * hiddenSize).WithGrad()
		l.BHH = Zeros(4 * hiddenSize).WithGrad()
	}
	return l
}

// Forward runs the sequence, returning (batch, seq, hidden).
func (l *LSTM) Forward(x *Tensor) *Tensor {
	b, s := x.shape[0], x.shape[1]
	h := Zeros(b, l.HiddenSize)
	c := Zeros(b, l.HiddenSize)
	var outs []*Tensor
	for t := 0; t < s; t++ {
		xt := x.Narrow(1, t, 1).Reshape(b, l.InputSize)
		h, c = LSTMCell(xt, h, c, l.WIH, l.BIH, l.WHH, l.BHH)
		outs = append(outs, h.Unsqueeze(1))
	}
	return Concat(1, outs...)
}

// Parameters returns the trainable weights and biases.
func (l *LSTM) Parameters() []*Tensor {
	params := []*Tensor{l.WIH, l.WHH}
	if l.BIH != nil {
		params = append(params, l.BIH, l.BHH)
	}
	return params
}

// GRU is a multi-step GRU over a (batch, seq, input) sequence.
type GRU struct {
	InputSize, HiddenSize int
	WIH, WHH              *Tensor
	BIH, BHH              *Tensor
}

// NewGRU builds a GRU layer.
func NewGRU(inputSize, hiddenSize int, bias bool) *GRU {
	g := &GRU{
		InputSize: inputSize, HiddenSize: hiddenSize,
		WIH: randomUniform(kaimingBound(inputSize), 3*hiddenSize, inputSize).WithGrad(),
		WHH: randomUniform(kaimingBound(hiddenSize), 3*hiddenSize, hiddenSize).WithGrad(),
	}
	if bias {
		g.BIH = Zeros(3 * hiddenSize).WithGrad()
		g.BHH = Zeros(3 * hiddenSize).WithGrad()
	}
	return g
}

// Forward runs the sequence, returning (batch, seq, hidden).
func (g *GRU) Forward(x *Tensor) *Tensor {
	b, s := x.shape[0], x.shape[1]
	h := Zeros(b, g.HiddenSize)
	var outs []*Tensor
	for t := 0; t < s; t++ {
		xt := x.Narrow(1, t, 1).Reshape(b, g.InputSize)
		h = GRUCell(xt, h, g.WIH, g.BIH, g.WHH, g.BHH)
		outs = append(outs, h.Unsqueeze(1))
	}
	return Concat(1, outs...)
}

// Parameters returns the trainable weights and biases.
func (g *GRU) Parameters() []*Tensor {
	params := []*Tensor{g.WIH, g.WHH}
	if g.BIH != nil {
		params = append(params, g.BIH, g.BHH)
	}
	return params
}
