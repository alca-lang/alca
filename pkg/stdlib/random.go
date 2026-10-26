// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package stdlib

import (
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/alca-lang/alca/pkg/interpreter"
)

type safePRNG struct {
	mu  sync.Mutex
	rng *rand.Rand
}

func newSafePRNG(seed int64) *safePRNG {
	return &safePRNG{
		rng: rand.New(rand.NewSource(seed)),
	}
}

func (p *safePRNG) Seed(seed int64) {
	p.mu.Lock()
	p.rng.Seed(seed)
	p.mu.Unlock()
}

func (p *safePRNG) Float64() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.rng.Float64()
}

func (p *safePRNG) Int63n(n int64) int64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.rng.Int63n(n)
}

func (p *safePRNG) NormFloat64() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.rng.NormFloat64()
}

var globalPRNG = newSafePRNG(time.Now().UnixNano())

// RegisterRandomBuiltins registers the non-cryptographic, fast seedable PRNG module "std.random".
func RegisterRandomBuiltins(env *interpreter.Environment) {
	mod := interpreter.NewMap()

	register := func(name string, fn func(args ...interpreter.Value) interpreter.Value) {
		mod.Set(name, &interpreter.BuiltinFunction{Name: name, Fn: fn})
	}

	// random.seed(int) -> null
	register("seed", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "random.seed requires integer seed")
		}
		iv, ok := args[0].(*interpreter.IntegerValue)
		if !ok {
			return interpreter.NewError(400, "random.seed expects integer argument")
		}
		globalPRNG.Seed(iv.Val)
		return interpreter.NULL
	})

	// random.random() -> float in [0.0, 1.0)
	register("random", func(args ...interpreter.Value) interpreter.Value {
		return &interpreter.FloatValue{Val: globalPRNG.Float64()}
	})

	// random.randint(min: int, max: int) -> int in [min, max]
	register("randint", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "random.randint requires (min: int, max: int)")
		}
		minVal, ok1 := args[0].(*interpreter.IntegerValue)
		maxVal, ok2 := args[1].(*interpreter.IntegerValue)
		if !ok1 || !ok2 {
			return interpreter.NewError(400, "random.randint expects integer arguments")
		}
		min := minVal.Val
		max := maxVal.Val
		if max < min {
			return interpreter.NewError(400, "random.randint: max must be >= min")
		}
		diff := max - min + 1
		n := globalPRNG.Int63n(diff)
		return &interpreter.IntegerValue{Val: min + n}
	})

	// random.uniform(a: float, b: float) -> float
	register("uniform", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "random.uniform requires (a, b)")
		}
		a := ltFloatArg(args, 0, 0.0)
		b := ltFloatArg(args, 1, 1.0)
		r := globalPRNG.Float64()
		return &interpreter.FloatValue{Val: a + (b-a)*r}
	})

	// random.choice(list) -> element
	register("choice", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "random.choice requires list")
		}
		lv, ok := args[0].(*interpreter.ListValue)
		if !ok || lv.Len() == 0 {
			return interpreter.NULL
		}
		elems := lv.ElementsSlice()
		idx := globalPRNG.Int63n(int64(len(elems)))
		return elems[idx]
	})

	// random.shuffle(list) -> new shuffled list
	register("shuffle", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewList()
		}
		lv, ok := args[0].(*interpreter.ListValue)
		if !ok {
			return interpreter.NewList()
		}
		elems := lv.ElementsSlice()
		cp := make([]interpreter.Value, len(elems))
		copy(cp, elems)
		for i := len(cp) - 1; i > 0; i-- {
			j := globalPRNG.Int63n(int64(i + 1))
			cp[i], cp[j] = cp[j], cp[i]
		}
		return interpreter.NewList(cp...)
	})

	// random.sample(list, k: int) -> list
	register("sample", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "random.sample requires (list, k: int)")
		}
		lv, ok := args[0].(*interpreter.ListValue)
		if !ok {
			return interpreter.NewList()
		}
		k := int(ltIntArg(args, 1, 0))
		elems := lv.ElementsSlice()
		if k <= 0 {
			return interpreter.NewList()
		}
		if k > len(elems) {
			k = len(elems)
		}
		// Fisher-Yates partial shuffle
		indices := make([]int, len(elems))
		for i := range indices {
			indices[i] = i
		}
		for i := 0; i < k; i++ {
			j := i + int(globalPRNG.Int63n(int64(len(elems)-i)))
			indices[i], indices[j] = indices[j], indices[i]
		}
		res := make([]interpreter.Value, k)
		for i := 0; i < k; i++ {
			res[i] = elems[indices[i]]
		}
		return interpreter.NewList(res...)
	})

	// random.gaussian(mean: float, stddev: float) -> float
	register("gaussian", func(args ...interpreter.Value) interpreter.Value {
		mean := ltFloatArg(args, 0, 0.0)
		stddev := ltFloatArg(args, 1, 1.0)
		val := globalPRNG.NormFloat64()*stddev + mean
		return &interpreter.FloatValue{Val: val}
	})

	// random.bool(chance?: float) -> bool
	register("bool", func(args ...interpreter.Value) interpreter.Value {
		chance := 0.5
		if len(args) >= 1 {
			chance = math.Max(0.0, math.Min(1.0, ltFloatArg(args, 0, 0.5)))
		}
		if globalPRNG.Float64() < chance {
			return interpreter.TRUE
		}
		return interpreter.FALSE
	})

	env.Set("random", mod)
	env.Set("std.random", mod)
}
