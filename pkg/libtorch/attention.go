// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package libtorch

import "math"

// EqValue returns a 0/1 mask that is 1 where t == v (no gradient).
func (t *Tensor) EqValue(v float64) *Tensor {
	out := Zeros(t.shape...)
	for i, x := range t.data {
		if x == v {
			out.data[i] = 1
		}
	}
	return out
}

// NeValue returns a 0/1 mask that is 1 where t != v (no gradient).
func (t *Tensor) NeValue(v float64) *Tensor {
	out := Zeros(t.shape...)
	for i, x := range t.data {
		if x != v {
			out.data[i] = 1
		}
	}
	return out
}

// CausalMask returns a (seq, seq) float mask with 1 where position i may attend
// to position j (j <= i) and 0 otherwise.
func CausalMask(seq int) *Tensor {
	m := Zeros(seq, seq)
	for i := 0; i < seq; i++ {
		for j := 0; j <= i; j++ {
			m.data[i*seq+j] = 1
		}
	}
	return m
}

// ScaledDotProductAttention computes attention over the last dimension of q, k,
// v. attnMask (optional, broadcastable) marks allowed positions with non-zero;
// disallowed positions are set to -inf before the softmax. dropoutP is applied
// to the attention weights during training. It returns the output and the
// attention weights.
func ScaledDotProductAttention(q, k, v, attnMask *Tensor, dropoutP float64, training bool) (*Tensor, *Tensor) {
	d := float64(q.shape[q.Dim()-1])
	scale := 1 / math.Sqrt(d)
	// scores = q @ k^T / sqrt(d)
	scores := q.MatMul(k.Transpose()).MulScalar(scale)
	if attnMask != nil {
		// Fill -inf where the mask is zero (not allowed).
		fillMask := attnMask.EqValue(0)
		scores = scores.MaskedFill(fillMask, math.Inf(-1))
	}
	attn := scores.Softmax()
	if dropoutP > 0 && training {
		attn = attn.Dropout(dropoutP, training)
	}
	out := attn.MatMul(v)
	return out, attn
}

// splitHeads reshapes (B, S, E) into (B, H, S, HD).
func splitHeads(x *Tensor, numHeads int) *Tensor {
	if x.Dim() != 3 {
		panicf("libtorch: attention expected (batch, seq, embed), got %v", x.shape)
	}
	b, s, e := x.shape[0], x.shape[1], x.shape[2]
	if e%numHeads != 0 {
		panicf("libtorch: embed dim %d not divisible by heads %d", e, numHeads)
	}
	hd := e / numHeads
	return x.Reshape(b, s, numHeads, hd).Permute(0, 2, 1, 3)
}

// mergeHeads reshapes (B, H, S, HD) back into (B, S, E).
func mergeHeads(x *Tensor) *Tensor {
	b, h, s, hd := x.shape[0], x.shape[1], x.shape[2], x.shape[3]
	return x.Permute(0, 2, 1, 3).Reshape(b, s, h*hd)
}

// MultiHeadAttention is a multi-head scaled dot-product attention layer over
// (batch, seq, embed) inputs. Self-attention uses the same input for q, k, v;
// cross-attention supplies separate key/value inputs.
type MultiHeadAttention struct {
	EmbedDim       int
	NumHeads       int
	QW, KW, VW, OW *Tensor // (embed, embed), used as x @ W^T
	QB, KB, VB, OB *Tensor // optional biases (embed,)
	DropoutP       float64
}

// NewMultiHeadAttention builds a MHA layer with learnable projections. Bias
// terms are allocated when bias is true.
func NewMultiHeadAttention(embedDim, numHeads int, bias bool, dropoutP float64) *MultiHeadAttention {
	b := bias
	bound := kaimingBound(embedDim)
	m := &MultiHeadAttention{
		EmbedDim: embedDim,
		NumHeads: numHeads,
		QW:       randomUniform(bound, embedDim, embedDim).WithGrad(),
		KW:       randomUniform(bound, embedDim, embedDim).WithGrad(),
		VW:       randomUniform(bound, embedDim, embedDim).WithGrad(),
		OW:       randomUniform(bound, embedDim, embedDim).WithGrad(),
		DropoutP: dropoutP,
	}
	if b {
		m.QB = Zeros(embedDim).WithGrad()
		m.KB = Zeros(embedDim).WithGrad()
		m.VB = Zeros(embedDim).WithGrad()
		m.OB = Zeros(embedDim).WithGrad()
	}
	return m
}

// proj projects (B,S,E) through W^T (+ bias).
func (m *MultiHeadAttention) proj(x *Tensor, w, bias *Tensor) *Tensor {
	y := x.MatMul(w.Transpose())
	if bias != nil {
		y = y.Add(bias)
	}
	return y
}

// Forward runs attention. kIn and vIn may be nil for self-attention. attnMask is
// optional (non-zero = attend). Returns the (batch, seq, embed) output.
func (m *MultiHeadAttention) Forward(qIn, kIn, vIn, attnMask *Tensor, training bool) *Tensor {
	if kIn == nil {
		kIn = qIn
	}
	if vIn == nil {
		vIn = qIn
	}
	q := splitHeads(m.proj(qIn, m.QW, m.QB), m.NumHeads)
	k := splitHeads(m.proj(kIn, m.KW, m.KB), m.NumHeads)
	v := splitHeads(m.proj(vIn, m.VW, m.VB), m.NumHeads)
	ctx, _ := ScaledDotProductAttention(q, k, v, attnMask, m.DropoutP, training)
	merged := mergeHeads(ctx)
	return m.proj(merged, m.OW, m.OB)
}

// SelfAttention is a convenience wrapper for single-input self-attention.
func (m *MultiHeadAttention) SelfAttention(x *Tensor, attnMask *Tensor, training bool) *Tensor {
	return m.Forward(x, nil, nil, attnMask, training)
}

// Parameters returns all trainable projection weights and biases.
func (m *MultiHeadAttention) Parameters() []*Tensor {
	params := []*Tensor{m.QW, m.KW, m.VW, m.OW}
	if m.QB != nil {
		params = append(params, m.QB, m.KB, m.VB, m.OB)
	}
	return params
}
