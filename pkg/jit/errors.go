// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package jit

import (
	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/vm"
)

// raisedErrorValue converts an error returned by a shared VM helper into the
// Alca error value the JIT delivers to the VM unwinder. Most helper errors are
// *vm.RaisedError; anything else is wrapped as a 500 so it is never dropped.
func raisedErrorValue(err error) *interpreter.ErrorValue {
	if err == nil {
		return nil
	}
	if raised, ok := err.(*vm.RaisedError); ok {
		return raised.Val
	}
	return interpreter.NewError(500, err.Error())
}
