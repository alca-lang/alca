// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package jit

import (
	"unsafe"
)

// Status codes returned by native JIT execution.
const (
	StatusSuccess int64 = 0 // Function completed normally with return value
	StatusBailout int64 = 1 // Deoptimized: continue in bytecode VM at BailoutIP
	StatusError   int64 = 2 // Runtime fault or error
	StatusHelper  int64 = 3 // Native code requests a JIT runtime helper at HelperIP
)

// NativeContext is the C-layout context passed to native JIT code.
// Every field is 8-byte aligned and accessed by fixed offsets in machine code.
type NativeContext struct {
	StackPtr    uintptr // Offset  0: pointer to vm.stack[0]
	GlobalsPtr  uintptr // Offset  8: pointer to globals[0]
	PoolPtr     uintptr // Offset 16: pointer to constant pool slots[0]
	BasePointer int64   // Offset 24: frame base pointer
	SP          int64   // Offset 32: current stack pointer (in and out)
	Status      int64   // Offset 40: exit status (StatusSuccess, StatusBailout, StatusError, StatusHelper)
	BailoutIP   int64   // Offset 48: bytecode instruction pointer to resume VM execution
	EntryOffset uintptr // Offset 56: native code entry address for OSR (0 for default start)
	HelperIP    int64   // Offset 64: bytecode IP of a request for a JIT runtime helper
	ResumeIP    int64   // Offset 72: bytecode IP at which to resume native execution
	HelperArgs  int64   // Offset 80: argument count for a helper call
	StackLimit  int64   // Offset 88: number of usable stack slots (0 = unknown, no limit)
}

// FuncVal represents the memory layout of a Go function pointer.
type FuncVal struct {
	Fn uintptr
}

// NativeFunc is the Go signature for calling into JIT-compiled machine code.
type NativeFunc func(ctx *NativeContext) int64

// AsNativeFunc wraps an executable code pointer as a callable Go function.
func AsNativeFunc(addr uintptr) NativeFunc {
	fv := &FuncVal{Fn: addr}
	return *(*NativeFunc)(unsafe.Pointer(&fv))
}

// UnsafeSlice creates a Go byte slice spanning length bytes at addr.
//
// addr points into JIT-allocated executable memory, which is not a Go heap
// allocation, so it must be expressed as uintptr. unsafe.Add keeps the
// conversion explicit without a bare uintptr-to-Pointer cast, which go vet
// would flag as a possible unsafe.Pointer misuse.
func UnsafeSlice(addr uintptr, length int) []byte {
	return unsafe.Slice((*byte)(unsafe.Add(unsafe.Pointer(nil), addr)), length)
}

// unsafeSlice creates a Go byte slice spanning length bytes at addr.
func unsafeSlice(addr uintptr, length int) []byte {
	return UnsafeSlice(addr, length)
}
