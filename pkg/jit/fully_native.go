// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package jit

import "github.com/alca-lang/alca/pkg/vm"

// nativeSafeOpcodes lists the opcodes the AMD64 backend lowers to self-contained
// machine instructions. Opcode operands are still required to be present; the
// scanner below skips them with the instruction table.
//
// Anything outside this set (notably OpCall, OpGetBuiltin, OpStdlibCall,
// OpList, OpMap and stream/use operations) cannot be self-contained. The
// runtime JIT may service some of them through its helper tier, but a pure
// native PE has no VM or JIT engine to resume into, so BuildNativePE must only
// be used when this check returns true.
var nativeSafeOpcodes = map[vm.Opcode]bool{
	vm.OpConstant: true, vm.OpPop: true, vm.OpDup: true,
	vm.OpTrue: true, vm.OpFalse: true, vm.OpNull: true,
	vm.OpGetLocal: true, vm.OpSetLocal: true, vm.OpIncLocal: true,
	vm.OpAddLocal: true, vm.OpSubLocal: true, vm.OpAddLocalLocal: true,
	vm.OpAddLocalConst: true, vm.OpMulLocalConst: true,
	vm.OpGetGlobal: true, vm.OpSetGlobal: true, vm.OpIncGlobal: true,
	vm.OpAddGlobal: true, vm.OpSubGlobal: true, vm.OpAddGlobalGlobal: true,
	vm.OpAddConst: true, vm.OpMulConst: true, vm.OpModConst: true,
	vm.OpModLocalConst:   true,
	vm.OpLoopGlobalLtInc: true, vm.OpLoopGlobalLeInc: true,
	vm.OpLoopLocalLtInc: true, vm.OpLoopLocalLeInc: true,
	vm.OpLoopRangeLocal: true, vm.OpLoopRangeGlobal: true,
	vm.OpNop: true, vm.OpIsInt: true,
	vm.OpAdd: true, vm.OpSub: true, vm.OpMul: true, vm.OpDiv: true, vm.OpMod: true,
	vm.OpNegate: true, vm.OpNot: true,
	vm.OpEqual: true, vm.OpNotEqual: true, vm.OpLessThan: true,
	vm.OpLessEqual: true, vm.OpGreaterThan: true, vm.OpGreaterEqual: true,
	vm.OpJump: true, vm.OpJumpIfFalse: true,
	vm.OpReturnValue: true, vm.OpReturn: true,
}

// FullyNative reports whether every instruction in ins is lowered to native
// machine code without a VM fallback. Runtime type checks inside the emitted
// arithmetic still exist, but for a well-typed program they are not taken.
func FullyNative(ins vm.Instructions) bool {
	ip := 0
	for ip < len(ins) {
		op := vm.Opcode(ins[ip])
		if !nativeSafeOpcodes[op] {
			return false
		}
		ip++
		def, err := vm.Lookup(byte(op))
		if err != nil || def == nil {
			return false
		}
		for _, w := range def.OperandWidths {
			if ip+w > len(ins) {
				return false
			}
			ip += w
		}
	}
	return true
}
