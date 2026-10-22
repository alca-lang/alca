//go:build !amd64

package jit

import (
	"errors"

	"github.com/alca-lang/alca/pkg/vm"
)

// HasNativeAMD64 is false on non-x86-64 hosts.
const HasNativeAMD64 = false

// compileNative reports unsupported on non-x86-64 hosts.
func compileNative(fn *vm.CompiledFunction) (NativeFunc, uintptr, int, map[int]uintptr, error) {
	return nil, 0, 0, nil, errors.New("native machine code JIT is only available on x86-64 hosts")
}

// CompileAOT reports unsupported on non-x86-64 hosts.
func CompileAOT(fn *vm.CompiledFunction) ([]byte, map[int]int, error) {
	return nil, nil, errors.New("native machine code AOT is only available on x86-64 hosts")
}
