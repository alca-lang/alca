// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package stdlib

import (
	"testing"

	"github.com/alca-lang/alca/pkg/interpreter"
)

func TestRandomPRNGSeedRepeatability(t *testing.T) {
	env := interpreter.NewEnvironment()
	RegisterRandomBuiltins(env)

	rndVal, ok := env.Get("std.random")
	if !ok {
		t.Fatal("std.random not registered")
	}
	rnd := rndVal.(*interpreter.MapValue)

	seedFn, _ := rnd.Get("seed")
	randintFn, _ := rnd.Get("randint")
	randomFn, _ := rnd.Get("random")

	// Seed 42
	seedFn.(*interpreter.BuiltinFunction).Fn(&interpreter.IntegerValue{Val: 42})
	seq1Int := randintFn.(*interpreter.BuiltinFunction).Fn(&interpreter.IntegerValue{Val: 1}, &interpreter.IntegerValue{Val: 100}).(*interpreter.IntegerValue).Val
	seq1Float := randomFn.(*interpreter.BuiltinFunction).Fn().(*interpreter.FloatValue).Val

	// Reseed with 42
	seedFn.(*interpreter.BuiltinFunction).Fn(&interpreter.IntegerValue{Val: 42})
	seq2Int := randintFn.(*interpreter.BuiltinFunction).Fn(&interpreter.IntegerValue{Val: 1}, &interpreter.IntegerValue{Val: 100}).(*interpreter.IntegerValue).Val
	seq2Float := randomFn.(*interpreter.BuiltinFunction).Fn().(*interpreter.FloatValue).Val

	if seq1Int != seq2Int {
		t.Errorf("expected same randint after reseed: got %d and %d", seq1Int, seq2Int)
	}
	if seq1Float != seq2Float {
		t.Errorf("expected same random float after reseed: got %f and %f", seq1Float, seq2Float)
	}
}

func TestRandomChoiceShuffleSample(t *testing.T) {
	env := interpreter.NewEnvironment()
	RegisterRandomBuiltins(env)

	rndVal, _ := env.Get("std.random")
	rnd := rndVal.(*interpreter.MapValue)
	cVal, _ := rnd.Get("choice")
	choiceFn := cVal.(*interpreter.BuiltinFunction)
	sVal, _ := rnd.Get("shuffle")
	shuffleFn := sVal.(*interpreter.BuiltinFunction)
	smVal, _ := rnd.Get("sample")
	sampleFn := smVal.(*interpreter.BuiltinFunction)
	bVal, _ := rnd.Get("bool")
	boolFn := bVal.(*interpreter.BuiltinFunction)
	gVal, _ := rnd.Get("gaussian")
	gaussianFn := gVal.(*interpreter.BuiltinFunction)

	items := interpreter.NewList(
		&interpreter.StringValue{Val: "apple"},
		&interpreter.StringValue{Val: "banana"},
		&interpreter.StringValue{Val: "cherry"},
	)

	// Choice
	chosen := choiceFn.Fn(items)
	if chosen.Type() != interpreter.STRING_VAL {
		t.Errorf("expected string from choice, got %s", chosen.Type())
	}

	// Shuffle
	shuffled := shuffleFn.Fn(items).(*interpreter.ListValue)
	if shuffled.Len() != 3 {
		t.Errorf("expected length 3 after shuffle, got %d", shuffled.Len())
	}

	// Sample
	sampled := sampleFn.Fn(items, &interpreter.IntegerValue{Val: 2}).(*interpreter.ListValue)
	if sampled.Len() != 2 {
		t.Errorf("expected sample length 2, got %d", sampled.Len())
	}

	// Bool
	b := boolFn.Fn(&interpreter.FloatValue{Val: 1.0})
	if b != interpreter.TRUE {
		t.Errorf("expected true for chance=1.0, got %v", b)
	}
	b0 := boolFn.Fn(&interpreter.FloatValue{Val: 0.0})
	if b0 != interpreter.FALSE {
		t.Errorf("expected false for chance=0.0, got %v", b0)
	}

	// Gaussian
	g := gaussianFn.Fn(&interpreter.FloatValue{Val: 10.0}, &interpreter.FloatValue{Val: 0.0})
	if g.(*interpreter.FloatValue).Val != 10.0 {
		t.Errorf("expected gaussian with stddev 0 to equal mean 10.0, got %f", g.(*interpreter.FloatValue).Val)
	}
}

func TestCryptoRandomIntStrictValidation(t *testing.T) {
	env := interpreter.NewEnvironment()
	RegisterCryptoBuiltins(env)

	cVal, _ := env.Get("std.crypto")
	crypto := cVal.(*interpreter.MapValue)
	riVal, _ := crypto.Get("random_int")
	randomIntFn := riVal.(*interpreter.BuiltinFunction)

	// Invalid arguments should return NewError (400)
	err1 := randomIntFn.Fn(&interpreter.StringValue{Val: "not_an_int"}, &interpreter.IntegerValue{Val: 100})
	if err1.Type() != interpreter.ERROR_VAL {
		t.Errorf("expected error for non-int argument, got %s", err1.Type())
	}

	err2 := randomIntFn.Fn(&interpreter.IntegerValue{Val: 100}, &interpreter.IntegerValue{Val: 10})
	if err2.Type() != interpreter.ERROR_VAL {
		t.Errorf("expected error when min > max, got %s", err2.Type())
	}

	// Valid arguments
	val := randomIntFn.Fn(&interpreter.IntegerValue{Val: 10}, &interpreter.IntegerValue{Val: 20})
	if val.Type() != interpreter.INTEGER_VAL {
		t.Fatalf("expected integer for valid min/max, got %s", val.Type())
	}
	intVal := val.(*interpreter.IntegerValue).Val
	if intVal < 10 || intVal > 20 {
		t.Errorf("value %d out of range [10, 20]", intVal)
	}
}
