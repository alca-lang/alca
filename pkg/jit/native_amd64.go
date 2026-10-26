// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

//go:build amd64

package jit

import "github.com/alca-lang/alca/pkg/vm"

// HasNativeAMD64 is true on x86-64 hosts.
const HasNativeAMD64 = true

// compileNative attempts native machine code compilation for the function.
func compileNative(fn *vm.CompiledFunction) (NativeFunc, uintptr, int, map[int]uintptr, error) {
	comp := NewCompilerAMD64(fn)
	return comp.Compile()
}
