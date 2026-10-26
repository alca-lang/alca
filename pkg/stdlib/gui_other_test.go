// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package stdlib_test

import (
	"testing"

	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/stdlib"
)

func TestGUIStubReturnsError(t *testing.T) {
	env := interpreter.NewEnvironment()
	stdlib.RegisterGUIBuiltins(env)
	v, ok := env.Get("gui")
	if !ok {
		t.Fatal("gui module missing")
	}
	m := v.(*interpreter.MapValue)
	fn, _ := m.Get("alert")
	res := fn.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: "x"})
	if _, isErr := res.(*interpreter.ErrorValue); !isErr {
		t.Fatalf("expected ErrorValue, got %T", res)
	}
}
