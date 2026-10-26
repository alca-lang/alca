// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package libtorch

// Embedding is a lookup table mapping integer indices to dense vectors. The
// input is an index tensor (whole-number values); the output appends an
// embedding-dim axis. Gradients scatter back into the referenced rows.
type Embedding struct {
	Weight *Tensor // (num_embeddings, embedding_dim)
}

// NewEmbedding builds an embedding table with a small random init.
func NewEmbedding(numEmbeddings, embeddingDim int) *Embedding {
	return &Embedding{Weight: randomUniform(1.0, numEmbeddings, embeddingDim).WithGrad()}
}

// Forward gathers embedding rows for the given index tensor.
func (e *Embedding) Forward(indices *Tensor) *Tensor {
	numE, dim := e.Weight.shape[0], e.Weight.shape[1]
	outShape := append(cloneInts(indices.shape), dim)
	out := Zeros(outShape...)
	wStrides := contiguousStrides(e.Weight.shape)
	iterate(indices.shape, func(flat int, idx []int) {
		id := int(indices.data[flat])
		if id < 0 || id >= numE {
			panicf("libtorch: Embedding index %d out of range [0,%d)", id, numE)
		}
		// Output flat base for this index position.
		base := flat * dim
		rowBase := id * wStrides[0]
		for d := 0; d < dim; d++ {
			out.data[base+d] = e.Weight.data[rowBase+d]
		}
	})
	wShape := e.Weight.shape
	out.attachGrad([]*Tensor{e.Weight}, func() {
		dw := make([]float64, e.Weight.Numel())
		iterate(indices.shape, func(flat int, idx []int) {
			id := int(indices.data[flat])
			base := flat * dim
			rowBase := id * wStrides[0]
			for d := 0; d < dim; d++ {
				dw[rowBase+d] += out.grad.data[base+d]
			}
		})
		accumulateGrad(e.Weight, &Tensor{data: dw, shape: cloneInts(wShape)})
	})
	return out
}

// Parameters returns the embedding weight.
func (e *Embedding) Parameters() []*Tensor { return []*Tensor{e.Weight} }
