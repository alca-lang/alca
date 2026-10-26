// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package libtorch

// gradEnabled controls whether operations record autograd graphs. It is a
// package-level flag toggled by SetGradEnabled / NoGrad for inference code.
var gradEnabled = true

// SetGradEnabled turns gradient recording on or off and returns a restore
// function that reverts to the previous setting.
func SetGradEnabled(v bool) func() {
	prev := gradEnabled
	gradEnabled = v
	return func() { gradEnabled = prev }
}

// NoGrad runs f with gradient recording disabled, then restores the prior mode.
func NoGrad(f func()) {
	restore := SetGradEnabled(false)
	defer restore()
	f()
}

// attachGrad wires autograd for a produced tensor given its parents and a
// backward closure. The closure is invoked during Backward with out.grad already
// populated; it must accumulate gradients into the parents. attachGrad is a no-op
// when no parent tracks gradients or recording is disabled.
func (out *Tensor) attachGrad(parents []*Tensor, backward func()) {
	if !gradEnabled {
		return
	}
	anyGrad := false
	for _, p := range parents {
		if p.requiresGrad || p.gradFn != nil {
			anyGrad = true
			break
		}
	}
	if !anyGrad {
		return
	}
	out.requiresGrad = true
	out.parents = parents
	out.gradFn = backward
}

// track reports whether a tensor participates in autograd.
func (t *Tensor) track() bool { return t.requiresGrad || t.gradFn != nil }

// accumulateGrad adds g into t.grad, creating it if needed. The gradient is
// reduced to t's shape first (undoing any broadcasting) and cloned so it never
// aliases another tensor's buffer.
func accumulateGrad(t *Tensor, g *Tensor) {
	if g == nil || !t.requiresGrad {
		return
	}
	g = reduceGradToShape(g, t.shape)
	if t.grad == nil {
		t.grad = g.Clone()
	} else {
		t.grad = t.grad.Add(g)
	}
}

// Backward runs reverse-mode autodiff from a scalar output, seeding the output
// gradient with ones. It panics if the tensor is not a single element.
func (t *Tensor) Backward() {
	if len(t.data) != 1 {
		panic("libtorch: Backward() requires a scalar output; use BackwardWith to supply a gradient")
	}
	t.BackwardWith(Ones(t.shape...))
}

// BackwardWith runs reverse-mode autodiff from an output with a supplied
// gradient of the same shape.
func (t *Tensor) BackwardWith(seed *Tensor) {
	// Topological order: parents appear before their dependents.
	visited := make(map[*Tensor]bool)
	order := make([]*Tensor, 0)
	var visit func(*Tensor)
	visit = func(x *Tensor) {
		if visited[x] {
			return
		}
		visited[x] = true
		for _, p := range x.parents {
			visit(p)
		}
		order = append(order, x)
	}
	visit(t)

	t.grad = seed.Clone()
	for i := len(order) - 1; i >= 0; i-- {
		x := order[i]
		if x.gradFn != nil && x.grad != nil {
			x.gradFn()
		}
	}
}

// attachReshape records the reshape edge so backward reshapes the gradient back
// to the input's shape.
func (out *Tensor) attachReshape(t *Tensor) {
	out.attachGrad([]*Tensor{t}, func() {
		accumulateGrad(t, out.grad.reshapeTo(t.shape))
	})
}

// attachPermute records a permute edge; backward applies the inverse
// permutation to the gradient.
func (out *Tensor) attachPermute(t *Tensor, dims []int) {
	inverse := make([]int, len(dims))
	for i, ax := range dims {
		inverse[ax] = i
	}
	out.attachGrad([]*Tensor{t}, func() {
		accumulateGrad(t, out.grad.Permute(inverse...))
	})
}
