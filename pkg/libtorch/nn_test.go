// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package libtorch

import (
	"math"
	"testing"
)

func TestConv2dGrad(t *testing.T) {
	in := Arange(0, 16, 1).Reshape(1, 1, 4, 4).WithGrad()
	wt := Arange(0, 9, 1).Reshape(1, 1, 3, 3).WithGrad()
	bs := FromFlatMust([]float64{0.25}, 1).WithGrad()
	checkGradients(t, "conv2d", []*Tensor{in, wt, bs}, func() *Tensor {
		return Conv2d(in, wt, bs, 1, 1, 0, 0).SumAll()
	})
}

func TestConv2dStridePadGrad(t *testing.T) {
	in := Arange(0, 32, 1).Reshape(1, 1, 4, 8).WithGrad()
	wt := Arange(0, 9, 1).Reshape(1, 1, 3, 3).WithGrad()
	checkGradients(t, "conv2d-stridepad", []*Tensor{in, wt}, func() *Tensor {
		return Conv2d(in, wt, nil, 2, 2, 1, 1).SumAll()
	})
}

func TestMaxPool2dGrad(t *testing.T) {
	in := Arange(0, 16, 1).Reshape(1, 1, 4, 4).WithGrad()
	checkGradients(t, "maxpool2d", []*Tensor{in}, func() *Tensor {
		return in.MaxPool2d(2, 2, 2, 2).SumAll()
	})
}

func TestSoftmaxRowsSumToOne(t *testing.T) {
	x := FromFlatMust([]float64{1, 2, 3, 4, 5, 6}, 2, 3)
	s := x.Softmax()
	for r := 0; r < 2; r++ {
		sum := s.data[r*3] + s.data[r*3+1] + s.data[r*3+2]
		if math.Abs(sum-1) > 1e-9 {
			t.Fatalf("softmax row %d sums to %v, want 1", r, sum)
		}
	}
}

func TestSoftmaxGrad(t *testing.T) {
	x := FromFlatMust([]float64{0.5, -1, 2, 3, -0.5, 1}, 2, 3).WithGrad()
	checkGradients(t, "softmax", []*Tensor{x}, func() *Tensor {
		return x.Softmax().Mul(x).SumAll()
	})
	checkGradients(t, "logsoftmax", []*Tensor{x}, func() *Tensor {
		return x.LogSoftmax().Neg().SumAll()
	})
}

func TestCrossEntropyGrad(t *testing.T) {
	logits := FromFlatMust([]float64{1.2, -0.5, 0.3, 2.1, 0.7, -1.4}, 3, 2).WithGrad()
	targets := FromFlatMust([]float64{0, 1, 0}, 3)
	checkGradients(t, "cross-entropy", []*Tensor{logits}, func() *Tensor {
		return CrossEntropyLoss(logits, targets)
	})
}

func TestLayerNormGrad(t *testing.T) {
	x := Arange(0, 8, 1).Reshape(2, 4).WithGrad()
	gamma := FromFlatMust([]float64{1, 1, 1, 1}, 4).WithGrad()
	beta := FromFlatMust([]float64{0, 0, 0, 0}, 4).WithGrad()
	checkGradients(t, "layernorm", []*Tensor{x, gamma, beta}, func() *Tensor {
		return x.LayerNorm(gamma, beta, 1e-5).SumAll()
	})
}

func TestBatchNorm2dGrad(t *testing.T) {
	x := Arange(0, 32, 1).Reshape(2, 2, 2, 4).WithGrad()
	gamma := FromFlatMust([]float64{1.5, 0.5}, 2).WithGrad()
	beta := FromFlatMust([]float64{0.1, -0.2}, 2).WithGrad()
	stats := NewBatchNormStats(2, 0.1, 1e-5)
	checkGradients(t, "batchnorm2d", []*Tensor{x, gamma, beta}, func() *Tensor {
		return x.BatchNorm(gamma, beta, stats, 1, []int{0, 2, 3}, true).SumAll()
	})
}

func TestMSEAndBCEGrad(t *testing.T) {
	p := FromFlatMust([]float64{0.3, 0.8, 0.4, 0.6}, 2, 2).WithGrad()
	tg := FromFlatMust([]float64{0, 1, 0, 1}, 2, 2)
	checkGradients(t, "mse", []*Tensor{p}, func() *Tensor {
		return MSELoss(p, tg)
	})
	checkGradients(t, "bce-logits", []*Tensor{p}, func() *Tensor {
		return BCEWithLogitsLoss(p, tg)
	})
}

func TestLinearForward(t *testing.T) {
	l := NewLinear(2, 3, true)
	// Force known weights.
	copy(l.Weight.data, []float64{1, 2, 3, 4, 5, 6}) // (3,2)
	copy(l.Bias.data, []float64{0.1, 0.2, 0.3})
	x := FromFlatMust([]float64{1, 2}, 1, 2)
	y := l.Forward(x) // (1,3)
	want := []float64{1*1 + 2*2 + 0.1, 1*3 + 2*4 + 0.2, 1*5 + 2*6 + 0.3}
	for i, wv := range want {
		if math.Abs(y.data[i]-wv) > 1e-9 {
			t.Fatalf("linear forward mismatch at %d: got %v want %v", i, y.data[i], wv)
		}
	}
}

func TestOptimizerTrainsLinear(t *testing.T) {
	l := NewLinear(1, 1, false)
	copy(l.Weight.data, []float64{0.0})
	// Target: y = 3x.
	xs := []*Tensor{FromFlatMust([]float64{-2}, 1, 1), FromFlatMust([]float64{-1}, 1, 1), FromFlatMust([]float64{0}, 1, 1),
		FromFlatMust([]float64{1}, 1, 1), FromFlatMust([]float64{2}, 1, 1)}
	ys := []float64{-6, -3, 0, 3, 6}
	opt := NewAdam(l.Parameters(), 0.1, false)
	var firstLoss float64
	for step := 0; step < 400; step++ {
		opt.ZeroGrad()
		total := Zeros(1, 1)
		for i, x := range xs {
			pred := l.Forward(x)
			total = total.Add(pred.Sub(FromFlatMust([]float64{ys[i]}, 1, 1)).PowScalar(2))
		}
		loss := total.DivScalar(float64(len(xs)))
		if step == 0 {
			firstLoss = loss.Item()
		}
		loss.Backward()
		opt.Step()
	}
	finalW := l.Weight.data[0]
	if math.Abs(finalW-3) > 0.1 {
		t.Fatalf("trained weight = %v, want ~3 (first loss %v)", finalW, firstLoss)
	}
}

func TestSequentialForwardBackward(t *testing.T) {
	model := NewSequential(
		NewLinear(4, 8, true),
		NewReLU(),
		NewLinear(8, 2, true),
	)
	x := FromFlatMust([]float64{0.5, -0.5, 1.0, -1.0}, 1, 4).WithGrad()
	logits := model.Forward(x)
	if logits.Shape()[1] != 2 {
		t.Fatalf("sequential output shape = %v, want (1,2)", logits.Shape())
	}
	target := FromFlatMust([]float64{1}, 1)
	loss := CrossEntropyLoss(logits, target)
	loss.Backward()
	if x.Grad() == nil {
		t.Fatal("expected gradient flowing to input")
	}
	for _, p := range model.Parameters() {
		if p.Grad() == nil {
			t.Fatalf("expected gradient for parameter %v", p.Shape())
		}
	}
}

func TestSerializationRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/model.bin"
	model := map[string]*Tensor{
		"w": Arange(0, 6, 1).Reshape(2, 3),
		"b": FromFlatMust([]float64{1, 2}, 2),
		"s": Scalar(3.5),
	}
	if err := Save(path, model); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded) != len(model) {
		t.Fatalf("loaded %d tensors, want %d", len(loaded), len(model))
	}
	for name, orig := range model {
		ld := loaded[name]
		if ld == nil {
			t.Fatalf("missing tensor %q", name)
		}
		if !equalInts(ld.Shape(), orig.Shape()) {
			t.Fatalf("shape mismatch for %q: %v vs %v", name, ld.Shape(), orig.Shape())
		}
		for i := range orig.data {
			if ld.data[i] != orig.data[i] {
				t.Fatalf("value mismatch for %q at %d: %v vs %v", name, i, ld.data[i], orig.data[i])
			}
		}
	}
}

func TestMaxMinReductionValues(t *testing.T) {
	x := FromFlatMust([]float64{1, 5, 3, 9, 2, 7}, 2, 3)
	mx := x.Max(false, 1)
	want := []float64{5, 9}
	for i, wv := range want {
		if mx.data[i] != wv {
			t.Fatalf("max mismatch at %d: got %v want %v", i, mx.data[i], wv)
		}
	}
	mn := x.Min(false, 0)
	wantMin := []float64{1, 2, 3}
	for i, wv := range wantMin {
		if mn.data[i] != wv {
			t.Fatalf("min mismatch at %d: got %v want %v", i, mn.data[i], wv)
		}
	}
}

func TestConcatStack(t *testing.T) {
	a := FromFlatMust([]float64{1, 2}, 1, 2)
	b := FromFlatMust([]float64{3, 4}, 1, 2)
	c := Concat(0, a, b)
	if !equalInts(c.Shape(), []int{2, 2}) {
		t.Fatalf("concat shape = %v want [2 2]", c.Shape())
	}
	s := Stack(0, a, b)
	if !equalInts(s.Shape(), []int{2, 1, 2}) {
		t.Fatalf("stack shape = %v want [2 1 2]", s.Shape())
	}
}
