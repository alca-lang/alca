// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package libtorch

import "fmt"

// broadcastShapes computes the broadcasted shape of two shapes, following
// NumPy/PyTorch trailing-alignment rules. It errors when shapes are
// incompatible.
func broadcastShapes(a, b []int) ([]int, error) {
	ra, rb := len(a), len(b)
	n := ra
	if rb > ra {
		n = rb
	}
	out := make([]int, n)
	for i := 0; i < n; i++ {
		da, db := 1, 1
		if i >= n-ra {
			da = a[i-(n-ra)]
		}
		if i >= n-rb {
			db = b[i-(n-rb)]
		}
		if da == db || db == 1 {
			out[i] = da
		} else if da == 1 {
			out[i] = db
		} else {
			return nil, fmt.Errorf("libtorch: cannot broadcast shapes %v and %v", a, b)
		}
	}
	return out, nil
}

// broadcastTo returns a tensor of shape target that replicates t's values. It
// panics when t's shape is not broadcast-compatible with target.
func (t *Tensor) broadcastTo(target []int) *Tensor {
	if equalInts(t.shape, target) {
		return t
	}
	out := Zeros(target...)
	tn := t.Dim()
	tg := len(target)
	if tn > tg {
		panic(fmt.Sprintf("libtorch: cannot broadcast %v to smaller shape %v", t.shape, target))
	}
	// Map an output index back onto the input by dropping leading padded dims
	// and clamping size-1 input dims to 0.
	inIdx := make([]int, tn)
	off := tg - tn
	iterate(target, func(flat int, idx []int) {
		for i := 0; i < tn; i++ {
			di := idx[off+i]
			if t.shape[i] == 1 {
				di = 0
			}
			inIdx[i] = di
		}
		out.data[flat] = t.At(inIdx...)
	})
	return out
}

// reduceGradToShape sums a gradient down to a target shape by collapsing
// broadcast dimensions. Used in backward passes to undo broadcasting.
func reduceGradToShape(g *Tensor, target []int) *Tensor {
	if equalInts(g.shape, target) {
		return g
	}
	gn := g.Dim()
	tn := len(target)
	if tn > gn {
		panic(fmt.Sprintf("libtorch: cannot reduce grad of shape %v to larger target %v", g.shape, target))
	}
	// Pad target on the left with 1s to the gradient's rank.
	padded := make([]int, gn)
	off := gn - tn
	for i := 0; i < gn; i++ {
		if i < off {
			padded[i] = 1
		} else {
			padded[i] = target[i-off]
		}
	}
	var axes []int
	for i := 0; i < gn; i++ {
		if padded[i] == 1 && g.shape[i] != 1 {
			axes = append(axes, i)
		}
	}
	summed := g.sumAxes(axes, true) // keep dims -> shape == padded
	// Reshape drops the leading padded 1s (and any reduced singletons).
	return summed.reshapeTo(target)
}

// reshapeTo reshapes to the target shape as a pure metadata change; it does not
// attach autograd. The caller guarantees element counts and layout match.
func (t *Tensor) reshapeTo(shape []int) *Tensor {
	if equalInts(t.shape, shape) {
		return t
	}
	return &Tensor{data: t.data, shape: cloneInts(shape)}
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
