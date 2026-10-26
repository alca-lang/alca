// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package libtorch

import (
	"math"
	"testing"
)

const gradEps = 1e-5
const gradTol = 1e-3

// checkGradients compares analytic gradients of build()'s scalar output against
// central finite differences for each provided leaf.
func checkGradients(t *testing.T, name string, leaves []*Tensor, build func() *Tensor) {
	t.Helper()
	out := build()
	if out.Numel() != 1 {
		t.Fatalf("%s: build must produce a scalar, got shape %v", name, out.Shape())
	}
	out.Backward()
	for _, leaf := range leaves {
		if leaf.grad == nil {
			if leaf.RequiresGrad() {
				t.Errorf("%s: leaf %v has no gradient but requires one", name, leaf.Shape())
			}
			continue
		}
		if leaf.grad.Numel() != leaf.Numel() {
			t.Errorf("%s: grad shape %v != leaf shape %v", name, leaf.grad.Shape(), leaf.Shape())
			continue
		}
		for i := range leaf.data {
			got := leaf.grad.data[i]
			want := numericGrad(build, leaf, i)
			if math.Abs(got-want) > gradTol {
				t.Errorf("%s: grad mismatch at flat %d: analytic=%.6f numeric=%.6f", name, i, got, want)
			}
		}
		leaf.ZeroGrad()
	}
}

func numericGrad(build func() *Tensor, leaf *Tensor, i int) float64 {
	orig := leaf.data[i]
	leaf.data[i] = orig + gradEps
	fPlus := build().Item()
	leaf.data[i] = orig - gradEps
	fMinus := build().Item()
	leaf.data[i] = orig
	return (fPlus - fMinus) / (2 * gradEps)
}

func TestAddMulChainGrad(t *testing.T) {
	a := FromFlatMust([]float64{1, 2, 3, 4}, 2, 2).WithGrad()
	b := FromFlatMust([]float64{5, 6, 7, 8}, 2, 2).WithGrad()
	checkGradients(t, "add-mul", []*Tensor{a, b}, func() *Tensor {
		return a.Add(b).Mul(a).SumAll()
	})
}

func TestBroadcastAddGrad(t *testing.T) {
	a := FromFlatMust([]float64{1, 2, 3}, 3, 1).WithGrad()
	b := FromFlatMust([]float64{10, 20, 30, 40}, 1, 4).WithGrad()
	checkGradients(t, "broadcast-add", []*Tensor{a, b}, func() *Tensor {
		return a.Add(b).SumAll()
	})
}

func TestMatMulGrad(t *testing.T) {
	a := FromFlatMust([]float64{1, 2, 3, 4, 5, 6}, 2, 3).WithGrad()
	b := FromFlatMust([]float64{0.5, 1, 1.5, 2, 2.5, 3}, 3, 2).WithGrad()
	checkGradients(t, "matmul", []*Tensor{a, b}, func() *Tensor {
		return a.MatMul(b).SumAll()
	})
}

func TestBatchedMatMulGrad(t *testing.T) {
	a := Arange(0, 24, 1).Reshape(2, 3, 4).WithGrad()
	b := Arange(0, 12, 1).Reshape(4, 3).WithGrad()
	checkGradients(t, "batched-matmul", []*Tensor{a, b}, func() *Tensor {
		// (2,3,4) @ (4,3) -> (2,3,3)
		return a.MatMul(b).SumAll()
	})
}

func TestActivationsGrad(t *testing.T) {
	a := FromFlatMust([]float64{-1.5, -0.5, 0.5, 1.5}, 4).WithGrad()
	checkGradients(t, "relu", []*Tensor{a}, func() *Tensor {
		return a.Relu().SumAll()
	})
	checkGradients(t, "sigmoid", []*Tensor{a}, func() *Tensor {
		return a.Sigmoid().SumAll()
	})
	checkGradients(t, "tanh", []*Tensor{a}, func() *Tensor {
		return a.Tanh().SumAll()
	})
	checkGradients(t, "exp-log", []*Tensor{a}, func() *Tensor {
		return a.Abs().AddScalar(1).Log().SumAll()
	})
	checkGradients(t, "sqrt", []*Tensor{a}, func() *Tensor {
		return a.Mul(a).AddScalar(1).Sqrt().SumAll()
	})
}

func TestReductionsGrad(t *testing.T) {
	a := Arange(0, 12, 1).Reshape(3, 4).WithGrad()
	checkGradients(t, "sum-axis0", []*Tensor{a}, func() *Tensor {
		return a.Sum(false, 0).SumAll()
	})
	checkGradients(t, "mean-axis1", []*Tensor{a}, func() *Tensor {
		return a.Mean(true, 1).SumAll()
	})
}

func TestTransposeGrad(t *testing.T) {
	a := Arange(0, 6, 1).Reshape(2, 3).WithGrad()
	checkGradients(t, "transpose", []*Tensor{a}, func() *Tensor {
		w := Ones(2, 5)
		return a.Transpose().MatMul(w).SumAll()
	})
}

func TestDotGrad(t *testing.T) {
	a := FromFlatMust([]float64{1, 2, 3}, 3).WithGrad()
	b := FromFlatMust([]float64{4, 5, 6}, 3).WithGrad()
	checkGradients(t, "dot", []*Tensor{a, b}, func() *Tensor {
		return a.Dot(b)
	})
}

func TestForwardValues(t *testing.T) {
	a := FromFlatMust([]float64{1, 2, 3, 4}, 2, 2)
	b := FromFlatMust([]float64{5, 6, 7, 8}, 2, 2)
	sum := a.Add(b)
	want := []float64{6, 8, 10, 12}
	for i, v := range want {
		if sum.data[i] != v {
			t.Fatalf("add value mismatch at %d: got %v want %v", i, sum.data[i], v)
		}
	}
	prod := a.MatMul(b) // [[1,2],[3,4]] @ [[5,6],[7,8]] = [[19,22],[43,50]]
	wantP := []float64{19, 22, 43, 50}
	for i, v := range wantP {
		if prod.data[i] != v {
			t.Fatalf("matmul value mismatch at %d: got %v want %v", i, prod.data[i], v)
		}
	}
}

func TestTransposeValues(t *testing.T) {
	a := FromFlatMust([]float64{1, 2, 3, 4, 5, 6}, 2, 3)
	tr := a.Transpose()
	if !equalInts(tr.Shape(), []int{3, 2}) {
		t.Fatalf("transpose shape = %v want [3 2]", tr.Shape())
	}
	want := []float64{1, 4, 2, 5, 3, 6}
	for i, v := range want {
		if tr.data[i] != v {
			t.Fatalf("transpose value mismatch at %d: got %v want %v", i, tr.data[i], v)
		}
	}
}

// FromFlatMust is a test helper that panics on error.
func FromFlatMust(data []float64, shape ...int) *Tensor {
	t, err := FromFlat(data, shape...)
	if err != nil {
		panic(err)
	}
	return t
}
