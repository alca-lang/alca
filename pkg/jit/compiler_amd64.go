// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package jit

import (
	"encoding/binary"

	"github.com/alca-lang/alca/pkg/vm"
)

// CompilerAMD64 compiles Alca VM bytecode into native x86-64 machine instructions.
type CompilerAMD64 struct {
	asm         *AssemblerAMD64
	fn          *vm.CompiledFunction
	labels      map[int]Label // Bytecode IP -> Assembler Label
	exitLabel   Label
	deoptLabel  Label
	helperLabel Label
}

// NewCompilerAMD64 creates an AMD64 native JIT compiler for the given function.
func NewCompilerAMD64(fn *vm.CompiledFunction) *CompilerAMD64 {
	return &CompilerAMD64{
		asm:    NewAssemblerAMD64(),
		fn:     fn,
		labels: make(map[int]Label),
	}
}

// Assemble translates the function into raw AMD64 machine code bytes without allocating memory.
func (c *CompilerAMD64) Assemble() ([]byte, map[int]int, error) {
	ins := c.fn.Instructions
	asm := c.asm

	c.exitLabel = asm.CreateLabel()
	c.deoptLabel = asm.CreateLabel()
	c.helperLabel = asm.CreateLabel()

	// Pre-create labels for all instruction offsets. The offset just past the
	// last instruction also gets a label: loops compile their exit jump to that
	// offset, which means "fall through to the function epilogue". Without the
	// label the native backend would bail out on every completed loop.
	ip := 0
	for ip < len(ins) {
		c.labels[ip] = asm.CreateLabel()
		op := vm.Opcode(ins[ip])
		ip++
		def, _ := vm.Lookup(byte(op))
		if def != nil {
			for _, w := range def.OperandWidths {
				ip += w
			}
		}
	}
	c.labels[len(ins)] = asm.CreateLabel()

	// -------------------------------------------------------------------------
	// Function Prologue
	// -------------------------------------------------------------------------
	// Preserve callee-saved registers (ABI compliance)
	asm.Push(RBX)
	asm.Push(R12)
	asm.Push(R13)
	asm.Push(R14)
	asm.Push(R15)
	asm.Push(RBP)

	// Load pointers from NativeContext (passed in RAX)
	asm.MovRegReg(R12, RAX)     // R12 = ctx pointer
	asm.MovRegMem(R13, R12, 0)  // R13 = ctx.StackPtr
	asm.MovRegMem(R14, R12, 8)  // R14 = ctx.GlobalsPtr
	asm.MovRegMem(R15, R12, 16) // R15 = ctx.PoolPtr
	asm.MovRegMem(RBP, R12, 24) // RBP = ctx.BasePointer (int64)
	asm.MovRegMem(RBX, R12, 32) // RBX = ctx.SP (current stack pointer index)

	// OSR dispatch: if ctx.EntryOffset != 0, jump directly to target native code
	asm.MovRegMem(RAX, R12, 56)
	asm.TestRegReg(RAX, RAX)
	lStart := asm.CreateLabel()
	asm.Jcc(CondE, lStart)
	asm.JmpReg(RAX)
	asm.BindLabel(lStart)

	// -------------------------------------------------------------------------
	// Bytecode Instruction Emission Loop
	// -------------------------------------------------------------------------
	ip = 0
	for ip < len(ins) {
		currentIP := ip
		asm.BindLabel(c.labels[currentIP])

		op := vm.Opcode(ins[ip])
		ip++

		switch op {
		case vm.OpConstant:
			constIdx := int32(binary.BigEndian.Uint16(ins[ip:]))
			ip += 2
			c.emitStackHeadroomCheck(currentIP)
			// srcSlot = r15 + constIdx * 40
			c.emitSlotAddrImm(RSI, R15, int64(constIdx))
			// dstSlot = r13 + rbx * 40
			c.emitSlotAddrReg(RDI, R13, RBX)
			c.emitSlotCopy(RDI, RSI)
			asm.IncReg(RBX) // sp++

		case vm.OpPop:
			// sp--
			asm.DecReg(RBX)

		case vm.OpDup:
			c.emitStackHeadroomCheck(currentIP)
			// dst = stack[sp], src = stack[sp-1]
			c.emitSlotAddrReg(RDI, R13, RBX)
			asm.MovRegReg(RAX, RBX)
			asm.DecReg(RAX)
			c.emitSlotAddrReg(RSI, R13, RAX)
			c.emitSlotCopy(RDI, RSI)
			asm.IncReg(RBX)

		case vm.OpTrue:
			c.emitPushImm(1, vm.SlotBool, currentIP)

		case vm.OpFalse:
			c.emitPushImm(0, vm.SlotBool, currentIP)

		case vm.OpNull:
			c.emitPushImm(0, vm.SlotNull, currentIP)

		case vm.OpGetLocal:
			localIdx := int64(ins[ip])
			ip++
			c.emitStackHeadroomCheck(currentIP)
			// src = stack[rbp + localIdx]
			asm.MovRegReg(RAX, RBP)
			asm.AddRegImm32(RAX, int32(localIdx))
			c.emitSlotAddrReg(RSI, R13, RAX)
			// dst = stack[rbx]
			c.emitSlotAddrReg(RDI, R13, RBX)
			c.emitSlotCopy(RDI, RSI)
			asm.IncReg(RBX)

		case vm.OpSetLocal:
			localIdx := int64(ins[ip])
			ip++
			asm.DecReg(RBX) // val = pop()
			c.emitSlotAddrReg(RSI, R13, RBX)
			// dst = stack[rbp + localIdx]
			asm.MovRegReg(RAX, RBP)
			asm.AddRegImm32(RAX, int32(localIdx))
			c.emitSlotAddrReg(RDI, R13, RAX)
			c.emitSlotCopy(RDI, RSI)

		case vm.OpIncLocal:
			localIdx := int64(ins[ip])
			ip++
			asm.MovRegReg(RAX, RBP)
			asm.AddRegImm32(RAX, int32(localIdx))
			c.emitSlotAddrReg(RSI, R13, RAX)
			c.emitIncSlot(RSI, currentIP)

		case vm.OpAddLocal, vm.OpSubLocal:
			localIdx := int64(ins[ip])
			ip++
			// operand = stack[sp-1]; the stack pointer is only changed on the
			// native paths, because the VM pops the operand and pushes the
			// result when it deoptimizes.
			asm.MovRegReg(RAX, RBX)
			asm.DecReg(RAX)
			c.emitSlotAddrReg(RSI, R13, RAX)
			// dst = stack[rbp + localIdx]
			asm.MovRegReg(RAX, RBP)
			asm.AddRegImm32(RAX, int32(localIdx))
			c.emitSlotAddrReg(RDI, R13, RAX)
			c.emitAddSubSlot(op, RDI, RSI, currentIP)

		case vm.OpAddLocalLocal:
			dstIdx := int64(ins[ip])
			srcIdx := int64(ins[ip+1])
			ip += 2
			// rdx = local[src].i64
			asm.MovRegReg(RAX, RBP)
			asm.AddRegImm32(RAX, int32(srcIdx))
			c.emitSlotAddrReg(RSI, R13, RAX)
			asm.MovRegMem(RDX, RSI, 8)
			// local[dst].i64 += rdx
			asm.MovRegReg(RAX, RBP)
			asm.AddRegImm32(RAX, int32(dstIdx))
			c.emitSlotAddrReg(RDI, R13, RAX)
			asm.AddMemReg(RDI, 8, RDX)

		case vm.OpAddLocalConst:
			srcIdx := int64(ins[ip])
			constIdx := int32(binary.BigEndian.Uint16(ins[ip+1:]))
			ip += 3
			// rdx = local[src].i64
			asm.MovRegReg(RAX, RBP)
			asm.AddRegImm32(RAX, int32(srcIdx))
			c.emitSlotAddrReg(RSI, R13, RAX)
			asm.MovRegMem(RDX, RSI, 8)
			// rdx += const[constIdx].i64
			c.emitSlotAddrImm(RAX, R15, int64(constIdx))
			asm.AddRegMem(RDX, RAX, 8)
			// push rdx as SlotInt
			c.emitPushReg(RDX, vm.SlotInt)

		case vm.OpMulLocalConst:
			srcIdx := int64(ins[ip])
			constIdx := int32(binary.BigEndian.Uint16(ins[ip+1:]))
			ip += 3
			asm.MovRegReg(RAX, RBP)
			asm.AddRegImm32(RAX, int32(srcIdx))
			c.emitSlotAddrReg(RSI, R13, RAX)
			asm.MovRegMem(RDX, RSI, 8)
			c.emitSlotAddrImm(RAX, R15, int64(constIdx))
			asm.ImulRegMem(RDX, RAX, 8)
			c.emitPushReg(RDX, vm.SlotInt)

		case vm.OpGetGlobal:
			globalIdx := int32(binary.BigEndian.Uint16(ins[ip:]))
			ip += 2
			c.emitSlotAddrImm(RSI, R14, int64(globalIdx))
			c.emitSlotAddrReg(RDI, R13, RBX)
			c.emitSlotCopy(RDI, RSI)
			asm.IncReg(RBX)

		case vm.OpSetGlobal:
			globalIdx := int32(binary.BigEndian.Uint16(ins[ip:]))
			ip += 2
			asm.DecReg(RBX)
			c.emitSlotAddrReg(RSI, R13, RBX)
			c.emitSlotAddrImm(RDI, R14, int64(globalIdx))
			c.emitSlotCopy(RDI, RSI)

		case vm.OpIncGlobal:
			globalIdx := int32(binary.BigEndian.Uint16(ins[ip:]))
			ip += 2
			c.emitSlotAddrImm(RDI, R14, int64(globalIdx))
			c.emitIncSlot(RDI, currentIP)

		case vm.OpAddGlobal, vm.OpSubGlobal:
			globalIdx := int32(binary.BigEndian.Uint16(ins[ip:]))
			ip += 2
			// operand = stack[sp-1]; the stack pointer is only changed on the
			// native paths, because the VM pops the operand and pushes the
			// result when it deoptimizes.
			asm.MovRegReg(RAX, RBX)
			asm.DecReg(RAX)
			c.emitSlotAddrReg(RSI, R13, RAX)
			c.emitSlotAddrImm(RDI, R14, int64(globalIdx))
			c.emitAddSubSlot(op, RDI, RSI, currentIP)
		case vm.OpAddGlobalGlobal:
			dstIdx := int32(binary.BigEndian.Uint16(ins[ip:]))
			srcIdx := int32(binary.BigEndian.Uint16(ins[ip+2:]))
			ip += 4
			c.emitSlotAddrImm(RSI, R14, int64(srcIdx))
			c.emitSlotAddrImm(RDI, R14, int64(dstIdx))
			lDeoptAGG := asm.CreateLabel()
			lContAGG := asm.CreateLabel()
			asm.CmpMemImm8(RSI, 0, byte(vm.SlotInt))
			asm.Jcc(CondNE, lDeoptAGG)
			asm.CmpMemImm8(RDI, 0, byte(vm.SlotInt))
			asm.Jcc(CondE, lContAGG)
			asm.BindLabel(lDeoptAGG)
			c.emitBailout(currentIP)
			asm.BindLabel(lContAGG)
			asm.MovRegMem(RDX, RSI, 8)
			asm.AddMemReg(RDI, 8, RDX)

		case vm.OpAddConst:
			constIdx := int32(binary.BigEndian.Uint16(ins[ip:]))
			ip += 2
			asm.MovRegReg(RAX, RBX)
			asm.DecReg(RAX)
			c.emitSlotAddrReg(RDI, R13, RAX)
			c.emitSlotAddrImm(RSI, R15, int64(constIdx))

			lFloatAddConst := asm.CreateLabel()
			lDeoptAddConst := asm.CreateLabel()
			lContIntAddConst := asm.CreateLabel()
			lContFloatAddConst := asm.CreateLabel()
			lDoneAddConst := asm.CreateLabel()

			asm.CmpMemImm8(RDI, 0, byte(vm.SlotInt))
			asm.Jcc(CondNE, lFloatAddConst)
			asm.CmpMemImm8(RSI, 0, byte(vm.SlotInt))
			asm.Jcc(CondE, lContIntAddConst)
			asm.Jmp(lDeoptAddConst)

			asm.BindLabel(lFloatAddConst)
			asm.CmpMemImm8(RDI, 0, byte(vm.SlotFloat))
			asm.Jcc(CondNE, lDeoptAddConst)
			asm.CmpMemImm8(RSI, 0, byte(vm.SlotFloat))
			asm.Jcc(CondE, lContFloatAddConst)

			asm.BindLabel(lDeoptAddConst)
			c.emitBailout(currentIP)

			asm.BindLabel(lContIntAddConst)
			asm.MovRegMem(RDX, RSI, 8)
			asm.AddMemReg(RDI, 8, RDX)
			asm.Jmp(lDoneAddConst)

			asm.BindLabel(lContFloatAddConst)
			asm.MovsdRegMem(XMM0, RDI, 16)
			asm.AddsdRegMem(XMM0, RSI, 16)
			asm.MovsdMemReg(RDI, 16, XMM0)

			asm.BindLabel(lDoneAddConst)

		case vm.OpMulConst:
			constIdx := int32(binary.BigEndian.Uint16(ins[ip:]))
			ip += 2
			asm.MovRegReg(RAX, RBX)
			asm.DecReg(RAX)
			c.emitSlotAddrReg(RDI, R13, RAX)
			c.emitSlotAddrImm(RSI, R15, int64(constIdx))

			lFloatMulConst := asm.CreateLabel()
			lDeoptMulConst := asm.CreateLabel()
			lContIntMulConst := asm.CreateLabel()
			lContFloatMulConst := asm.CreateLabel()
			lDoneMulConst := asm.CreateLabel()

			asm.CmpMemImm8(RDI, 0, byte(vm.SlotInt))
			asm.Jcc(CondNE, lFloatMulConst)
			asm.CmpMemImm8(RSI, 0, byte(vm.SlotInt))
			asm.Jcc(CondE, lContIntMulConst)
			asm.Jmp(lDeoptMulConst)

			asm.BindLabel(lFloatMulConst)
			asm.CmpMemImm8(RDI, 0, byte(vm.SlotFloat))
			asm.Jcc(CondNE, lDeoptMulConst)
			asm.CmpMemImm8(RSI, 0, byte(vm.SlotFloat))
			asm.Jcc(CondE, lContFloatMulConst)

			asm.BindLabel(lDeoptMulConst)
			c.emitBailout(currentIP)

			asm.BindLabel(lContIntMulConst)
			asm.MovRegMem(RDX, RSI, 8)
			asm.ImulRegMem(RDX, RDI, 8)
			asm.MovMemReg(RDI, 8, RDX)
			asm.Jmp(lDoneMulConst)

			asm.BindLabel(lContFloatMulConst)
			asm.MovsdRegMem(XMM0, RDI, 16)
			asm.MulsdRegMem(XMM0, RSI, 16)
			asm.MovsdMemReg(RDI, 16, XMM0)

			asm.BindLabel(lDoneMulConst)

		case vm.OpModConst:
			constIdx := int32(binary.BigEndian.Uint16(ins[ip:]))
			ip += 2
			asm.MovRegReg(RAX, RBX)
			asm.DecReg(RAX)
			c.emitSlotAddrReg(RDI, R13, RAX)
			c.emitSlotAddrImm(RSI, R15, int64(constIdx))
			asm.MovRegMem(RCX, RSI, 8) // divisor
			lDeoptModConst := asm.CreateLabel()
			lMinusOneModConst := asm.CreateLabel()
			lContModConst := asm.CreateLabel()
			lDoneModConst := asm.CreateLabel()
			asm.CmpMemImm8(RDI, 0, byte(vm.SlotInt))
			asm.Jcc(CondNE, lDeoptModConst)
			asm.CmpMemImm8(RSI, 0, byte(vm.SlotInt))
			asm.Jcc(CondNE, lDeoptModConst)
			asm.TestRegReg(RCX, RCX)
			asm.Jcc(CondE, lDeoptModConst)
			asm.CmpRegImm32(RCX, -1)
			asm.Jcc(CondE, lMinusOneModConst)
			asm.Jmp(lContModConst)
			asm.BindLabel(lMinusOneModConst)
			asm.MovMemImm32(RDI, 8, 0)
			asm.Jmp(lDoneModConst)
			asm.BindLabel(lDeoptModConst)
			c.emitBailout(currentIP)
			asm.BindLabel(lContModConst)
			asm.MovRegMem(RAX, RDI, 8)
			asm.Cqo()
			asm.IdivReg(RCX)
			asm.MovMemReg(RDI, 8, RDX)
			asm.BindLabel(lDoneModConst)

		case vm.OpModLocalConst:
			srcIdx := int64(ins[ip])
			constIdx := int32(binary.BigEndian.Uint16(ins[ip+1:]))
			ip += 3
			asm.MovRegReg(RAX, RBP)
			asm.AddRegImm32(RAX, int32(srcIdx))
			c.emitSlotAddrReg(RSI, R13, RAX)
			asm.MovRegMem(RAX, RSI, 8) // local[src].i64
			asm.CmpMemImm8(RSI, 0, byte(vm.SlotInt))
			lDeoptModLocalConst := asm.CreateLabel()
			lMinusOneModLocalConst := asm.CreateLabel()
			lContModLocalConst := asm.CreateLabel()
			lDoneModLocalConst := asm.CreateLabel()
			asm.Jcc(CondNE, lDeoptModLocalConst)
			c.emitSlotAddrImm(RDX, R15, int64(constIdx))
			asm.CmpMemImm8(RDX, 0, byte(vm.SlotInt))
			asm.Jcc(CondNE, lDeoptModLocalConst)
			asm.MovRegMem(RCX, RDX, 8) // const.i64
			asm.TestRegReg(RCX, RCX)
			asm.Jcc(CondE, lDeoptModLocalConst)
			asm.CmpRegImm32(RCX, -1)
			asm.Jcc(CondE, lMinusOneModLocalConst)
			asm.Jmp(lContModLocalConst)
			asm.BindLabel(lMinusOneModLocalConst)
			c.emitPushImm(0, vm.SlotInt, currentIP)
			asm.Jmp(lDoneModLocalConst)
			asm.BindLabel(lDeoptModLocalConst)
			c.emitBailout(currentIP)
			asm.BindLabel(lContModLocalConst)
			asm.Cqo()
			asm.IdivReg(RCX)
			c.emitPushReg(RDX, vm.SlotInt)
			asm.BindLabel(lDoneModLocalConst)

		case vm.OpLoopGlobalLtInc:
			varIdx := int32(binary.BigEndian.Uint16(ins[ip:]))
			constIdx := int32(binary.BigEndian.Uint16(ins[ip+2:]))
			targetOffset := int(binary.BigEndian.Uint16(ins[ip+4:]))
			ip += 6

			c.emitSlotAddrImm(RSI, R14, int64(varIdx))
			asm.IncMem(RSI, 8)
			asm.MovRegMem(RAX, RSI, 8)

			c.emitSlotAddrImm(RDI, R15, int64(constIdx))
			asm.CmpRegMem(RAX, RDI, 8)

			targetLabel, ok := c.labels[targetOffset]
			if !ok {
				c.emitBailout(currentIP)
			} else {
				asm.Jcc(CondL, targetLabel)
			}

		case vm.OpLoopGlobalLeInc:
			varIdx := int32(binary.BigEndian.Uint16(ins[ip:]))
			endIdx := int32(binary.BigEndian.Uint16(ins[ip+2:]))
			targetOffset := int(binary.BigEndian.Uint16(ins[ip+4:]))
			ip += 6

			c.emitSlotAddrImm(RSI, R14, int64(varIdx))
			asm.IncMem(RSI, 8)
			asm.MovRegMem(RAX, RSI, 8)

			c.emitSlotAddrImm(RDI, R14, int64(endIdx))
			asm.CmpRegMem(RAX, RDI, 8)

			targetLabel, ok := c.labels[targetOffset]
			if !ok {
				c.emitBailout(currentIP)
			} else {
				asm.Jcc(CondLE, targetLabel)
			}

		case vm.OpLoopLocalLtInc:
			localIdx := int64(ins[ip])
			constIdx := int32(binary.BigEndian.Uint16(ins[ip+1:]))
			targetOffset := int(binary.BigEndian.Uint16(ins[ip+3:]))
			ip += 5

			asm.MovRegReg(RAX, RBP)
			asm.AddRegImm32(RAX, int32(localIdx))
			c.emitSlotAddrReg(RSI, R13, RAX)
			asm.IncMem(RSI, 8)
			asm.MovRegMem(RAX, RSI, 8)

			c.emitSlotAddrImm(RDI, R15, int64(constIdx))
			asm.CmpRegMem(RAX, RDI, 8)

			targetLabel, ok := c.labels[targetOffset]
			if !ok {
				c.emitBailout(currentIP)
			} else {
				asm.Jcc(CondL, targetLabel)
			}

		case vm.OpLoopLocalLeInc:
			varIdx := int64(ins[ip])
			endIdx := int64(ins[ip+1])
			targetOffset := int(binary.BigEndian.Uint16(ins[ip+2:]))
			ip += 4

			asm.MovRegReg(RAX, RBP)
			asm.AddRegImm32(RAX, int32(varIdx))
			c.emitSlotAddrReg(RSI, R13, RAX)
			asm.IncMem(RSI, 8)
			asm.MovRegMem(RAX, RSI, 8)

			asm.MovRegReg(RDX, RBP)
			asm.AddRegImm32(RDX, int32(endIdx))
			c.emitSlotAddrReg(RDI, R13, RDX)
			asm.CmpRegMem(RAX, RDI, 8)

			targetLabel, ok := c.labels[targetOffset]
			if !ok {
				c.emitBailout(currentIP)
			} else {
				asm.Jcc(CondLE, targetLabel)
			}

		case vm.OpNop:
			// no-op

		case vm.OpIsInt:
			asm.MovRegReg(RAX, RBX)
			asm.DecReg(RAX)
			c.emitSlotAddrReg(RDI, R13, RAX)
			asm.CmpMemImm8(RDI, 0, byte(vm.SlotInt))
			lIsIntTrue := asm.CreateLabel()
			lIsIntDone := asm.CreateLabel()
			asm.Jcc(CondE, lIsIntTrue)
			asm.MovMemImm8(RDI, 0, byte(vm.SlotBool))
			asm.MovMemImm32(RDI, 8, 0)
			asm.Jmp(lIsIntDone)
			asm.BindLabel(lIsIntTrue)
			asm.MovMemImm8(RDI, 0, byte(vm.SlotBool))
			asm.MovMemImm32(RDI, 8, 1)
			asm.BindLabel(lIsIntDone)

		case vm.OpAdd:
			asm.MovRegReg(RAX, RBX)
			asm.DecReg(RAX)
			c.emitSlotAddrReg(RSI, R13, RAX)
			asm.DecReg(RAX)
			c.emitSlotAddrReg(RDI, R13, RAX)

			lFloatAdd := asm.CreateLabel()
			lDeoptAdd := asm.CreateLabel()
			lContIntAdd := asm.CreateLabel()
			lContFloatAdd := asm.CreateLabel()
			lDoneAdd := asm.CreateLabel()

			asm.CmpMemImm8(RSI, 0, byte(vm.SlotInt))
			asm.Jcc(CondNE, lFloatAdd)
			asm.CmpMemImm8(RDI, 0, byte(vm.SlotInt))
			asm.Jcc(CondE, lContIntAdd)
			asm.Jmp(lDeoptAdd)

			asm.BindLabel(lFloatAdd)
			asm.CmpMemImm8(RSI, 0, byte(vm.SlotFloat))
			asm.Jcc(CondNE, lDeoptAdd)
			asm.CmpMemImm8(RDI, 0, byte(vm.SlotFloat))
			asm.Jcc(CondE, lContFloatAdd)

			asm.BindLabel(lDeoptAdd)
			c.emitBailout(currentIP)

			asm.BindLabel(lContIntAdd)
			asm.DecReg(RBX)
			asm.MovRegMem(RDX, RSI, 8)
			asm.AddMemReg(RDI, 8, RDX)
			asm.Jmp(lDoneAdd)

			asm.BindLabel(lContFloatAdd)
			asm.DecReg(RBX)
			asm.MovsdRegMem(XMM0, RDI, 16)
			asm.AddsdRegMem(XMM0, RSI, 16)
			asm.MovsdMemReg(RDI, 16, XMM0)

			asm.BindLabel(lDoneAdd)

		case vm.OpSub:
			asm.MovRegReg(RAX, RBX)
			asm.DecReg(RAX)
			c.emitSlotAddrReg(RSI, R13, RAX)
			asm.DecReg(RAX)
			c.emitSlotAddrReg(RDI, R13, RAX)

			lFloatSub := asm.CreateLabel()
			lDeoptSub := asm.CreateLabel()
			lContIntSub := asm.CreateLabel()
			lContFloatSub := asm.CreateLabel()
			lDoneSub := asm.CreateLabel()

			asm.CmpMemImm8(RSI, 0, byte(vm.SlotInt))
			asm.Jcc(CondNE, lFloatSub)
			asm.CmpMemImm8(RDI, 0, byte(vm.SlotInt))
			asm.Jcc(CondE, lContIntSub)
			asm.Jmp(lDeoptSub)

			asm.BindLabel(lFloatSub)
			asm.CmpMemImm8(RSI, 0, byte(vm.SlotFloat))
			asm.Jcc(CondNE, lDeoptSub)
			asm.CmpMemImm8(RDI, 0, byte(vm.SlotFloat))
			asm.Jcc(CondE, lContFloatSub)

			asm.BindLabel(lDeoptSub)
			c.emitBailout(currentIP)

			asm.BindLabel(lContIntSub)
			asm.DecReg(RBX)
			asm.MovRegMem(RDX, RSI, 8)
			asm.SubMemReg(RDI, 8, RDX)
			asm.Jmp(lDoneSub)

			asm.BindLabel(lContFloatSub)
			asm.DecReg(RBX)
			asm.MovsdRegMem(XMM0, RDI, 16)
			asm.SubsdRegMem(XMM0, RSI, 16)
			asm.MovsdMemReg(RDI, 16, XMM0)

			asm.BindLabel(lDoneSub)

		case vm.OpMul:
			asm.MovRegReg(RAX, RBX)
			asm.DecReg(RAX)
			c.emitSlotAddrReg(RSI, R13, RAX)
			asm.DecReg(RAX)
			c.emitSlotAddrReg(RDI, R13, RAX)

			lFloatMul := asm.CreateLabel()
			lDeoptMul := asm.CreateLabel()
			lContIntMul := asm.CreateLabel()
			lContFloatMul := asm.CreateLabel()
			lDoneMul := asm.CreateLabel()

			asm.CmpMemImm8(RSI, 0, byte(vm.SlotInt))
			asm.Jcc(CondNE, lFloatMul)
			asm.CmpMemImm8(RDI, 0, byte(vm.SlotInt))
			asm.Jcc(CondE, lContIntMul)
			asm.Jmp(lDeoptMul)

			asm.BindLabel(lFloatMul)
			asm.CmpMemImm8(RSI, 0, byte(vm.SlotFloat))
			asm.Jcc(CondNE, lDeoptMul)
			asm.CmpMemImm8(RDI, 0, byte(vm.SlotFloat))
			asm.Jcc(CondE, lContFloatMul)

			asm.BindLabel(lDeoptMul)
			c.emitBailout(currentIP)

			asm.BindLabel(lContIntMul)
			asm.DecReg(RBX)
			asm.MovRegMem(RDX, RSI, 8)
			asm.ImulRegMem(RDX, RDI, 8)
			asm.MovMemReg(RDI, 8, RDX)
			asm.Jmp(lDoneMul)

			asm.BindLabel(lContFloatMul)
			asm.DecReg(RBX)
			asm.MovsdRegMem(XMM0, RDI, 16)
			asm.MulsdRegMem(XMM0, RSI, 16)
			asm.MovsdMemReg(RDI, 16, XMM0)

			asm.BindLabel(lDoneMul)

		case vm.OpDiv:
			asm.MovRegReg(RAX, RBX)
			asm.DecReg(RAX)
			c.emitSlotAddrReg(RSI, R13, RAX)
			asm.DecReg(RAX)
			c.emitSlotAddrReg(RDI, R13, RAX)

			lFloatDiv := asm.CreateLabel()
			lDeoptDiv := asm.CreateLabel()
			lNegIntDiv := asm.CreateLabel()
			lContIntDiv := asm.CreateLabel()
			lContFloatDiv := asm.CreateLabel()
			lDoneDiv := asm.CreateLabel()

			asm.CmpMemImm8(RSI, 0, byte(vm.SlotInt))
			asm.Jcc(CondNE, lFloatDiv)
			asm.CmpMemImm8(RDI, 0, byte(vm.SlotInt))
			asm.Jcc(CondNE, lDeoptDiv)
			// Division by zero deoptimizes so the VM can raise the normal
			// error. Division by -1 is valid and would fault in IDIV for
			// MinInt64, so negate explicitly (two's-complement wrap matches
			// the VM's MinInt64 / -1 rule).
			asm.MovRegMem(RCX, RSI, 8)
			asm.TestRegReg(RCX, RCX)
			asm.Jcc(CondE, lDeoptDiv)
			asm.CmpRegImm32(RCX, -1)
			asm.Jcc(CondE, lNegIntDiv)
			asm.Jmp(lContIntDiv)

			asm.BindLabel(lFloatDiv)
			asm.CmpMemImm8(RSI, 0, byte(vm.SlotFloat))
			asm.Jcc(CondNE, lDeoptDiv)
			asm.CmpMemImm8(RDI, 0, byte(vm.SlotFloat))
			asm.Jcc(CondNE, lDeoptDiv)
			// Check float divisor != 0.0. Shifting out the sign bit treats both
			// +0.0 and -0.0 as zero, matching the VM's numeric comparison and
			// letting it raise the normal division-by-zero error.
			asm.MovRegMem(RCX, RSI, 16)
			asm.ShlRegImm8(RCX, 1)
			asm.TestRegReg(RCX, RCX)
			asm.Jcc(CondE, lDeoptDiv)
			asm.Jmp(lContFloatDiv)

			asm.BindLabel(lNegIntDiv)
			asm.DecReg(RBX)
			asm.MovRegMem(RAX, RDI, 8)
			asm.NegReg(RAX)
			asm.MovMemReg(RDI, 8, RAX)
			asm.Jmp(lDoneDiv)

			asm.BindLabel(lDeoptDiv)
			c.emitBailout(currentIP)

			asm.BindLabel(lContIntDiv)
			asm.DecReg(RBX)
			asm.MovRegMem(RAX, RDI, 8)
			asm.Cqo()
			asm.IdivReg(RCX)
			asm.MovMemReg(RDI, 8, RAX)
			asm.Jmp(lDoneDiv)

			asm.BindLabel(lContFloatDiv)
			asm.DecReg(RBX)
			asm.MovsdRegMem(XMM0, RDI, 16)
			asm.DivsdRegMem(XMM0, RSI, 16)
			asm.MovsdMemReg(RDI, 16, XMM0)

			asm.BindLabel(lDoneDiv)

		case vm.OpMod:
			asm.MovRegReg(RAX, RBX)
			asm.DecReg(RAX)
			c.emitSlotAddrReg(RSI, R13, RAX)
			asm.DecReg(RAX)
			c.emitSlotAddrReg(RDI, R13, RAX)

			lDeoptMod := asm.CreateLabel()
			lMinusOneMod := asm.CreateLabel()
			lContMod := asm.CreateLabel()
			lDoneMod := asm.CreateLabel()
			asm.CmpMemImm8(RSI, 0, byte(vm.SlotInt))
			asm.Jcc(CondNE, lDeoptMod)
			asm.CmpMemImm8(RDI, 0, byte(vm.SlotInt))
			asm.Jcc(CondNE, lDeoptMod)
			// Modulo by zero deoptimizes. A divisor of -1 always produces zero
			// (including MinInt64 % -1), so avoid IDIV's hardware fault.
			asm.MovRegMem(RCX, RSI, 8)
			asm.TestRegReg(RCX, RCX)
			asm.Jcc(CondE, lDeoptMod)
			asm.CmpRegImm32(RCX, -1)
			asm.Jcc(CondE, lMinusOneMod)
			asm.Jmp(lContMod)

			asm.BindLabel(lMinusOneMod)
			asm.DecReg(RBX)
			asm.MovMemImm32(RDI, 8, 0)
			asm.Jmp(lDoneMod)

			asm.BindLabel(lDeoptMod)
			c.emitBailout(currentIP)

			asm.BindLabel(lContMod)
			asm.DecReg(RBX)
			asm.MovRegMem(RAX, RDI, 8)
			asm.Cqo()
			asm.IdivReg(RCX)
			asm.MovMemReg(RDI, 8, RDX)

			asm.BindLabel(lDoneMod)

		case vm.OpNegate:
			asm.MovRegReg(RAX, RBX)
			asm.DecReg(RAX)
			c.emitSlotAddrReg(RDI, R13, RAX)
			lIntNeg := asm.CreateLabel()
			lFloatNeg := asm.CreateLabel()
			lDoneNeg := asm.CreateLabel()
			asm.CmpMemImm8(RDI, 0, byte(vm.SlotInt))
			asm.Jcc(CondE, lIntNeg)
			asm.CmpMemImm8(RDI, 0, byte(vm.SlotFloat))
			asm.Jcc(CondE, lFloatNeg)
			// The VM leaves non-numeric values unchanged for unary minus.
			asm.Jmp(lDoneNeg)

			asm.BindLabel(lIntNeg)
			asm.MovRegMem(RDX, RDI, 8)
			asm.NegReg(RDX)
			asm.MovMemReg(RDI, 8, RDX)
			asm.Jmp(lDoneNeg)

			asm.BindLabel(lFloatNeg)
			asm.MovRegMem(RDX, RDI, 16)
			asm.MovRegImm64(RAX, 0x8000000000000000)
			asm.XorRegReg(RDX, RAX)
			asm.MovMemReg(RDI, 16, RDX)

			asm.BindLabel(lDoneNeg)

		case vm.OpStrictEqual:
			c.emitStrictComparison(currentIP)

		case vm.OpMathBuiltin:
			id := int8(ins[ip])
			ip++
			c.emitMathBuiltin(id, currentIP)

		case vm.OpApplyDecorator:
			ip += 2
			c.emitHelperCall(currentIP, ip, 0)

		case vm.OpMakeClass:
			ip += 4
			c.emitHelperCall(currentIP, ip, 0)

		case vm.OpStdlibCall:
			argc := int(ins[ip+4])
			ip += 5
			c.emitHelperCall(currentIP, ip, argc)

		case vm.OpGetBuiltin, vm.OpGetField, vm.OpUse, vm.OpClosure:
			ip += 2
			c.emitHelperCall(currentIP, ip, 0)

		case vm.OpGetFree, vm.OpSetFree:
			ip++
			c.emitHelperCall(currentIP, ip, 0)

		case vm.OpCall:
			argc := int(ins[ip])
			ip++
			c.emitHelperCall(currentIP, ip, argc)

		case vm.OpList, vm.OpMap:
			count := int(binary.BigEndian.Uint16(ins[ip:]))
			ip += 2
			c.emitHelperCall(currentIP, ip, count)

		case vm.OpPow, vm.OpIn, vm.OpIndex, vm.OpSetIndex, vm.OpRange, vm.OpIter, vm.OpListLen, vm.OpMakeError, vm.OpRaise:
			c.emitHelperCall(currentIP, ip, 0)

		case vm.OpSetField:
			ip += 2
			c.emitHelperCall(currentIP, ip, 0)

		case vm.OpAssert:
			ip += 2
			c.emitHelperCall(currentIP, ip, 0)

		case vm.OpCallBuiltin1:
			ip++
			c.emitHelperCall(currentIP, ip, 0)

		case vm.OpConvLocal:
			ip += 2
			c.emitHelperCall(currentIP, ip, 0)

		case vm.OpConvGlobal:
			ip += 3
			c.emitHelperCall(currentIP, ip, 0)

		case vm.OpEqual, vm.OpNotEqual, vm.OpLessThan, vm.OpLessEqual, vm.OpGreaterThan, vm.OpGreaterEqual:
			c.emitComparison(op, currentIP)

		case vm.OpNot:
			c.emitNot(currentIP)

		case vm.OpJump:
			targetOffset := int(binary.BigEndian.Uint16(ins[ip:]))
			ip += 2
			targetLabel, ok := c.labels[targetOffset]
			if !ok {
				c.emitBailout(currentIP)
			} else {
				asm.Jmp(targetLabel)
			}

		case vm.OpJumpIfFalse:
			targetOffset := int(binary.BigEndian.Uint16(ins[ip:]))
			ip += 2
			// pop condition: slot = stack[--sp]
			asm.DecReg(RBX)
			c.emitSlotAddrReg(RDI, R13, RBX)
			targetLabel, ok := c.labels[targetOffset]
			if !ok {
				c.emitBailout(currentIP)
			} else {
				// Falsy in Alca: tag == SlotNull (4) OR (tag == SlotBool (3) && i64 == 0)
				asm.CmpMemImm8(RDI, 0, byte(vm.SlotNull))
				asm.Jcc(CondE, targetLabel)
				lNotBool := asm.CreateLabel()
				asm.CmpMemImm8(RDI, 0, byte(vm.SlotBool))
				asm.Jcc(CondNE, lNotBool)
				asm.MovRegMem(RDX, RDI, 8) // i64
				asm.TestRegReg(RDX, RDX)
				asm.Jcc(CondE, targetLabel)
				asm.BindLabel(lNotBool)
			}

		case vm.OpLoopRangeLocal:
			varIdx := int64(ins[ip])
			endIdx := int64(ins[ip+1])
			stepIdx := int64(ins[ip+2])
			targetOffset := int(binary.BigEndian.Uint16(ins[ip+3:]))
			ip += 5

			// rdx = step: stack[rbp + stepIdx].i64
			asm.MovRegReg(RAX, RBP)
			asm.AddRegImm32(RAX, int32(stepIdx))
			c.emitSlotAddrReg(RDI, R13, RAX)
			asm.MovRegMem(RDX, RDI, 8) // rdx = step

			// varSlot = stack[rbp + varIdx]
			asm.MovRegReg(RAX, RBP)
			asm.AddRegImm32(RAX, int32(varIdx))
			c.emitSlotAddrReg(RSI, R13, RAX)
			asm.AddMemReg(RSI, 8, RDX) // var += step
			asm.MovRegMem(RCX, RSI, 8) // rcx = current var value

			// endSlot = stack[rbp + endIdx]
			asm.MovRegReg(RAX, RBP)
			asm.AddRegImm32(RAX, int32(endIdx))
			c.emitSlotAddrReg(RDI, R13, RAX)

			targetLabel, ok := c.labels[targetOffset]
			if !ok {
				c.emitBailout(currentIP)
			} else {
				lNegative := asm.CreateLabel()
				lDone := asm.CreateLabel()
				asm.TestRegReg(RDX, RDX)
				asm.Jcc(CondL, lNegative)
				// Step >= 0: loop while var <= end
				asm.CmpRegMem(RCX, RDI, 8)
				asm.Jcc(CondLE, targetLabel)
				asm.Jmp(lDone)
				// Step < 0: loop while var >= end
				asm.BindLabel(lNegative)
				asm.CmpRegMem(RCX, RDI, 8)
				asm.Jcc(CondGE, targetLabel)
				asm.BindLabel(lDone)
			}

		case vm.OpLoopRangeGlobal:
			varIdx := int32(binary.BigEndian.Uint16(ins[ip:]))
			endIdx := int32(binary.BigEndian.Uint16(ins[ip+2:]))
			stepIdx := int32(binary.BigEndian.Uint16(ins[ip+4:]))
			targetOffset := int(binary.BigEndian.Uint16(ins[ip+6:]))
			ip += 8

			// rdx = step: globals[stepIdx].i64
			c.emitSlotAddrImm(RDI, R14, int64(stepIdx))
			asm.MovRegMem(RDX, RDI, 8)

			// globals[varIdx].i64 += step
			c.emitSlotAddrImm(RSI, R14, int64(varIdx))
			asm.AddMemReg(RSI, 8, RDX)
			asm.MovRegMem(RCX, RSI, 8)

			// end = globals[endIdx].i64
			c.emitSlotAddrImm(RDI, R14, int64(endIdx))

			targetLabel, ok := c.labels[targetOffset]
			if !ok {
				c.emitBailout(currentIP)
			} else {
				lNegative := asm.CreateLabel()
				lDone := asm.CreateLabel()
				asm.TestRegReg(RDX, RDX)
				asm.Jcc(CondL, lNegative)
				// Step >= 0: loop while var <= end
				asm.CmpRegMem(RCX, RDI, 8)
				asm.Jcc(CondLE, targetLabel)
				asm.Jmp(lDone)
				// Step < 0: loop while var >= end
				asm.BindLabel(lNegative)
				asm.CmpRegMem(RCX, RDI, 8)
				asm.Jcc(CondGE, targetLabel)
				asm.BindLabel(lDone)
			}

		case vm.OpReturnValue:
			// Normal function return with value on top of stack
			c.emitEpilogue(StatusSuccess)

		case vm.OpReturn:
			// Return with null
			c.emitPushImm(0, vm.SlotNull, currentIP)
			c.emitEpilogue(StatusSuccess)

		default:
			// For any opcode not directly emitted as native x86 machine instructions
			// (e.g. OpCall, OpStream, OpUse, OpMap, OpList), advance operands and bailout cleanly to the VM!
			def, _ := vm.Lookup(byte(op))
			if def != nil {
				for _, w := range def.OperandWidths {
					ip += w
				}
			}
			c.emitBailout(currentIP)
		}
	}

	// -------------------------------------------------------------------------
	// Default Epilogue and Bailout Handlers
	// -------------------------------------------------------------------------
	asm.BindLabel(c.labels[len(ins)]) // fall-through end of bytecode
	asm.BindLabel(c.exitLabel)
	// Epilogue: restore sp, return 0
	asm.MovMemReg(R12, 32, RBX) // ctx.SP = rbx
	asm.MovMemImm32(R12, 40, int32(StatusSuccess))
	asm.XorRegReg(RAX, RAX) // return StatusSuccess (0)
	asm.Pop(RBP)
	asm.Pop(R15)
	asm.Pop(R14)
	asm.Pop(R13)
	asm.Pop(R12)
	asm.Pop(RBX)
	asm.Ret()

	asm.BindLabel(c.deoptLabel)
	// Bailout: restore sp, set status = 1, return 1
	asm.MovMemReg(R12, 32, RBX)
	asm.MovMemImm32(R12, 40, int32(StatusBailout))
	asm.MovRegImm32(RAX, int32(StatusBailout))
	asm.Pop(RBP)
	asm.Pop(R15)
	asm.Pop(R14)
	asm.Pop(R13)
	asm.Pop(R12)
	asm.Pop(RBX)
	asm.Ret()

	asm.BindLabel(c.helperLabel)
	// Helper request: the JIT engine executes one standard-library or
	// collection-construction helper and resumes native code at ResumeIP.
	asm.MovMemReg(R12, 32, RBX)
	asm.MovMemImm32(R12, 40, int32(StatusHelper))
	asm.MovRegImm32(RAX, int32(StatusHelper))
	asm.Pop(RBP)
	asm.Pop(R15)
	asm.Pop(R14)
	asm.Pop(R13)
	asm.Pop(R12)
	asm.Pop(RBX)
	asm.Ret()

	machineCode, err := asm.Finalize()
	if err != nil {
		return nil, nil, err
	}

	ipOffsets := make(map[int]int, len(c.labels))
	for bytecodeIP, label := range c.labels {
		if offset, ok := asm.labels[label]; ok {
			ipOffsets[bytecodeIP] = offset
		}
	}

	return machineCode, ipOffsets, nil
}

// Compile generates executable machine code for the function in executable memory.
func (c *CompilerAMD64) Compile() (NativeFunc, uintptr, int, map[int]uintptr, error) {
	machineCode, labelOffsets, err := c.Assemble()
	if err != nil {
		return nil, 0, 0, nil, err
	}

	addr, err := AllocExec(len(machineCode))
	if err != nil {
		return nil, 0, 0, nil, err
	}

	dst := unsafeSlice(addr, len(machineCode))
	copy(dst, machineCode)

	ipOffsets := make(map[int]uintptr, len(labelOffsets))
	for bytecodeIP, offset := range labelOffsets {
		ipOffsets[bytecodeIP] = addr + uintptr(offset)
	}

	nativeFn := AsNativeFunc(addr)
	return nativeFn, addr, len(machineCode), ipOffsets, nil
}

// CompileAOT compiles fn Ahead-of-Time into raw AMD64 machine code bytes without allocating memory.
func CompileAOT(fn *vm.CompiledFunction) ([]byte, map[int]int, error) {
	c := NewCompilerAMD64(fn)
	return c.Assemble()
}

// emitSlotAddrReg computes dstReg = baseReg + indexReg * 40
func (c *CompilerAMD64) emitSlotAddrReg(dstReg, baseReg, indexReg Register) {
	asm := c.asm
	asm.MovRegReg(dstReg, indexReg)
	// imul dstReg, dstReg, 40
	asm.rexW(dstReg, 0, dstReg)
	asm.emitByte(0x6B)
	asm.emitByte(modRM(0b11, byte(dstReg), byte(dstReg)))
	asm.emitByte(40)
	// add dstReg, baseReg
	asm.AddRegReg(dstReg, baseReg)
}

// emitSlotAddrImm computes dstReg = baseReg + immIndex * 40
func (c *CompilerAMD64) emitSlotAddrImm(dstReg, baseReg Register, immIndex int64) {
	disp := immIndex * 40
	asm := c.asm
	asm.MovRegImm64(dstReg, uint64(disp))
	asm.AddRegReg(dstReg, baseReg)
}

// emitSlotCopy copies 40 bytes from srcPtr to dstPtr using RAX/RCX
func (c *CompilerAMD64) emitSlotCopy(dstPtr, srcPtr Register) {
	asm := c.asm
	for offset := int32(0); offset < 40; offset += 8 {
		asm.MovRegMem(RCX, srcPtr, offset)
		asm.MovMemReg(dstPtr, offset, RCX)
	}
}

// nativeStackLimitOffset is the NativeContext offset of the stack slot limit.
const nativeStackLimitOffset = 88

// emitIncSlot increments an integer slot in place. Only integers are handled
// natively: the VM adds one to every other type, which needs the slow
// arithmetic path, so those cases deoptimize instead.
func (c *CompilerAMD64) emitIncSlot(slot Register, currentIP int) {
	asm := c.asm
	lNotInt := asm.CreateLabel()
	asm.CmpMemImm8(slot, 0, byte(vm.SlotInt))
	asm.Jcc(CondNE, lNotInt)
	asm.IncMem(slot, 8)
	lDone := asm.CreateLabel()
	asm.Jmp(lDone)
	asm.BindLabel(lNotInt)
	c.emitBailout(currentIP)
	asm.BindLabel(lDone)
}

// emitAddSubSlot emits the arithmetic of the compound assignment opcodes that
// combine the value on top of the stack with a local or a global slot.
//
// Only an int with an int and a float with a float are handled natively.
// Every other combination deoptimizes to the VM, which pops the operand and
// pushes the result. The stack pointer is therefore only changed on the
// native paths: a deoptimization has to resume with the original pointer.
func (c *CompilerAMD64) emitAddSubSlot(op vm.Opcode, dst, operand Register, currentIP int) {
	asm := c.asm
	add := op == vm.OpAddLocal || op == vm.OpAddGlobal

	lFloat := asm.CreateLabel()
	lDeopt := asm.CreateLabel()
	lContInt := asm.CreateLabel()
	lContFloat := asm.CreateLabel()
	lDone := asm.CreateLabel()

	asm.CmpMemImm8(dst, 0, byte(vm.SlotInt))
	asm.Jcc(CondNE, lFloat)
	asm.CmpMemImm8(operand, 0, byte(vm.SlotInt))
	asm.Jcc(CondE, lContInt)
	asm.Jmp(lDeopt)

	asm.BindLabel(lFloat)
	asm.CmpMemImm8(dst, 0, byte(vm.SlotFloat))
	asm.Jcc(CondNE, lDeopt)
	asm.CmpMemImm8(operand, 0, byte(vm.SlotFloat))
	asm.Jcc(CondE, lContFloat)

	asm.BindLabel(lDeopt)
	c.emitBailout(currentIP)

	asm.BindLabel(lContInt)
	asm.DecReg(RBX) // the operand is consumed
	asm.MovRegMem(RDX, operand, 8)
	if add {
		asm.AddMemReg(dst, 8, RDX)
	} else {
		asm.SubMemReg(dst, 8, RDX)
	}
	asm.Jmp(lDone)

	asm.BindLabel(lContFloat)
	asm.DecReg(RBX)
	asm.MovsdRegMem(XMM0, dst, 16)
	if add {
		asm.AddsdRegMem(XMM0, operand, 16)
	} else {
		asm.SubsdRegMem(XMM0, operand, 16)
	}
	asm.MovsdMemReg(dst, 16, XMM0)

	asm.BindLabel(lDone)
}

// emitStackHeadroomCheck refuses to push a slot when the VM stack is already
// full. The native code writes stack slots directly, so a frame that runs past
// the end of the stack would corrupt memory outside the VM's stack. When the
// limit is reached the function bails out to the bytecode VM, which reports the
// overflow the same way it does for interpreted code.
func (c *CompilerAMD64) emitStackHeadroomCheck(currentIP int) {
	asm := c.asm
	asm.MovRegMem(RAX, R12, nativeStackLimitOffset)
	asm.TestRegReg(RAX, RAX)
	lProceed := asm.CreateLabel()
	asm.Jcc(CondE, lProceed) // no limit configured
	asm.CmpRegReg(RBX, RAX)
	asm.Jcc(CondB, lProceed) // sp < limit
	c.emitBailout(currentIP)
	asm.BindLabel(lProceed)
}

// emitPushImm pushes an immediate 64-bit value with a given tag
func (c *CompilerAMD64) emitPushImm(val int64, tag int8, currentIP int) {
	asm := c.asm
	c.emitStackHeadroomCheck(currentIP)
	c.emitSlotAddrReg(RDI, R13, RBX)
	// tag at +0
	asm.MovMemImm8(RDI, 0, byte(tag))
	// i64 at +8
	if val >= -2147483648 && val <= 2147483647 {
		asm.MovMemImm32(RDI, 8, int32(val))
	} else {
		asm.MovRegImm64(RAX, uint64(val))
		asm.MovMemReg(RDI, 8, RAX)
	}
	// clear obj at +24, +32
	asm.MovMemImm32(RDI, 24, 0)
	asm.MovMemImm32(RDI, 32, 0)
	asm.IncReg(RBX) // sp++
}

// emitPushReg pushes register RDX as SlotInt
func (c *CompilerAMD64) emitPushReg(valReg Register, tag int8) {
	asm := c.asm
	c.emitSlotAddrReg(RDI, R13, RBX)
	asm.MovMemImm8(RDI, 0, byte(tag))
	asm.MovMemReg(RDI, 8, valReg)
	asm.MovMemImm32(RDI, 24, 0)
	asm.MovMemImm32(RDI, 32, 0)
	asm.IncReg(RBX)
}

// emitComparison emits comparison between stack[sp-2] and stack[sp-1] and pushes bool
func (c *CompilerAMD64) emitComparison(op vm.Opcode, currentIP int) {
	asm := c.asm
	asm.MovRegReg(RAX, RBX)
	asm.DecReg(RAX)
	c.emitSlotAddrReg(RSI, R13, RAX)
	asm.DecReg(RAX)
	c.emitSlotAddrReg(RDI, R13, RAX)

	lFloatComp := asm.CreateLabel()
	lDeopt := asm.CreateLabel()
	lContInt := asm.CreateLabel()
	lContFloat := asm.CreateLabel()

	asm.CmpMemImm8(RSI, 0, byte(vm.SlotInt))
	asm.Jcc(CondNE, lFloatComp)
	asm.CmpMemImm8(RDI, 0, byte(vm.SlotInt))
	asm.Jcc(CondE, lContInt)
	asm.Jmp(lDeopt)

	asm.BindLabel(lFloatComp)
	asm.CmpMemImm8(RSI, 0, byte(vm.SlotFloat))
	asm.Jcc(CondNE, lDeopt)
	asm.CmpMemImm8(RDI, 0, byte(vm.SlotFloat))
	asm.Jcc(CondE, lContFloat)

	asm.BindLabel(lDeopt)
	// Non-numeric operands (notably strings and collections) need the shared
	// value-level comparison semantics. Jump to the runtime-helper tier instead
	// of deoptimizing the whole function.
	c.emitHelperCall(currentIP, currentIP+1, 0)

	// Int path
	asm.BindLabel(lContInt)
	asm.DecReg(RBX)            // sp--
	asm.MovRegMem(RDX, RSI, 8) // rdx = right.i64
	asm.MovRegMem(RCX, RDI, 8) // rcx = left.i64
	asm.CmpRegReg(RCX, RDX)    // cmp left, right: computes (left - right)

	var intCond Condition
	switch op {
	case vm.OpEqual:
		intCond = CondE
	case vm.OpNotEqual:
		intCond = CondNE
	case vm.OpLessThan:
		intCond = CondL
	case vm.OpLessEqual:
		intCond = CondLE
	case vm.OpGreaterThan:
		intCond = CondG
	case vm.OpGreaterEqual:
		intCond = CondGE
	}

	lTrue := asm.CreateLabel()
	lDone := asm.CreateLabel()

	asm.Jcc(intCond, lTrue)
	// False
	asm.MovMemImm8(RDI, 0, byte(vm.SlotBool))
	asm.MovMemImm32(RDI, 8, 0)
	asm.MovMemImm32(RDI, 24, 0)
	asm.MovMemImm32(RDI, 32, 0)
	asm.Jmp(lDone)

	// Float path
	asm.BindLabel(lContFloat)
	asm.DecReg(RBX)
	asm.MovsdRegMem(XMM0, RDI, 16)
	asm.UcomisdRegMem(XMM0, RSI, 16)

	var floatCond Condition
	switch op {
	case vm.OpEqual:
		floatCond = CondE
	case vm.OpNotEqual:
		floatCond = CondNE
	case vm.OpLessThan:
		floatCond = CondB
	case vm.OpLessEqual:
		floatCond = CondBE
	case vm.OpGreaterThan:
		floatCond = CondA
	case vm.OpGreaterEqual:
		floatCond = CondAE
	}

	asm.Jcc(floatCond, lTrue)
	// False
	asm.MovMemImm8(RDI, 0, byte(vm.SlotBool))
	asm.MovMemImm32(RDI, 8, 0)
	asm.MovMemImm32(RDI, 24, 0)
	asm.MovMemImm32(RDI, 32, 0)
	asm.Jmp(lDone)

	// True
	asm.BindLabel(lTrue)
	asm.MovMemImm8(RDI, 0, byte(vm.SlotBool))
	asm.MovMemImm32(RDI, 8, 1)
	asm.MovMemImm32(RDI, 24, 0)
	asm.MovMemImm32(RDI, 32, 0)

	asm.BindLabel(lDone)
}

func (c *CompilerAMD64) emitStrictComparison(currentIP int) {
	asm := c.asm
	asm.MovRegReg(RAX, RBX)
	asm.DecReg(RAX)
	c.emitSlotAddrReg(RSI, R13, RAX)
	asm.DecReg(RAX)
	c.emitSlotAddrReg(RDI, R13, RAX)

	lFalse := asm.CreateLabel()
	lTrue := asm.CreateLabel()
	lInt := asm.CreateLabel()
	lFloat := asm.CreateLabel()
	lBool := asm.CreateLabel()
	lNull := asm.CreateLabel()
	lObject := asm.CreateLabel()
	lDone := asm.CreateLabel()

	// Strict equality first requires identical runtime tags.
	asm.MovRegMem(RAX, RSI, 0)
	asm.CmpRegMem(RAX, RDI, 0)
	asm.Jcc(CondNE, lFalse)

	asm.CmpRegImm32(RAX, int32(vm.SlotInt))
	asm.Jcc(CondE, lInt)
	asm.CmpRegImm32(RAX, int32(vm.SlotFloat))
	asm.Jcc(CondE, lFloat)
	asm.CmpRegImm32(RAX, int32(vm.SlotBool))
	asm.Jcc(CondE, lBool)
	asm.CmpRegImm32(RAX, int32(vm.SlotNull))
	asm.Jcc(CondE, lNull)
	asm.Jmp(lObject)

	asm.BindLabel(lInt)
	asm.MovRegMem(RAX, RSI, 8)
	asm.CmpRegMem(RAX, RDI, 8)
	asm.Jcc(CondE, lTrue)
	asm.Jmp(lFalse)

	asm.BindLabel(lFloat)
	asm.MovsdRegMem(XMM0, RDI, 16)
	asm.UcomisdRegMem(XMM0, RSI, 16)
	// UCOMISD reports unordered comparisons with both ZF and CF set.
	asm.Jcc(CondNE, lFalse)
	asm.Jcc(CondB, lFalse)
	asm.Jmp(lTrue)

	asm.BindLabel(lBool)
	asm.MovRegMem(RAX, RSI, 8)
	asm.CmpRegMem(RAX, RDI, 8)
	asm.Jcc(CondE, lTrue)
	asm.Jmp(lFalse)

	asm.BindLabel(lNull)
	asm.Jmp(lTrue)

	asm.BindLabel(lObject)
	// Strings and collections need the VM's value-level deep equality.
	c.emitHelperCall(currentIP, currentIP+1, 0)

	asm.BindLabel(lFalse)
	asm.MovMemImm8(RDI, 0, byte(vm.SlotBool))
	asm.MovMemImm32(RDI, 8, 0)
	asm.MovMemImm32(RDI, 24, 0)
	asm.MovMemImm32(RDI, 32, 0)
	asm.DecReg(RBX)
	asm.Jmp(lDone)

	asm.BindLabel(lTrue)
	asm.MovMemImm8(RDI, 0, byte(vm.SlotBool))
	asm.MovMemImm32(RDI, 8, 1)
	asm.MovMemImm32(RDI, 24, 0)
	asm.MovMemImm32(RDI, 32, 0)
	asm.DecReg(RBX)

	asm.BindLabel(lDone)
}

func (c *CompilerAMD64) emitStoreBoolResult(slot Register, value int32) {
	asm := c.asm
	asm.MovMemImm8(slot, 0, byte(vm.SlotBool))
	asm.MovMemImm32(slot, 8, value)
	asm.MovMemImm32(slot, 24, 0)
	asm.MovMemImm32(slot, 32, 0)
}

// emitNot mirrors the VM's truthiness rule for primitive slots. Boxed values
// (SlotVal) need the VM's value-level check and deoptimize.
func (c *CompilerAMD64) emitNot(currentIP int) {
	asm := c.asm
	asm.MovRegReg(RAX, RBX)
	asm.DecReg(RAX)
	c.emitSlotAddrReg(RDI, R13, RAX)

	lInt := asm.CreateLabel()
	lFloat := asm.CreateLabel()
	lTrue := asm.CreateLabel()
	lFalse := asm.CreateLabel()
	lDone := asm.CreateLabel()

	asm.CmpMemImm8(RDI, 0, byte(vm.SlotBool))
	asm.Jcc(CondE, lInt)
	asm.CmpMemImm8(RDI, 0, byte(vm.SlotInt))
	asm.Jcc(CondE, lInt)
	asm.CmpMemImm8(RDI, 0, byte(vm.SlotFloat))
	asm.Jcc(CondE, lFloat)
	asm.CmpMemImm8(RDI, 0, byte(vm.SlotNull))
	asm.Jcc(CondE, lTrue)
	c.emitBailout(currentIP)

	asm.BindLabel(lInt)
	asm.MovRegMem(RDX, RDI, 8)
	asm.TestRegReg(RDX, RDX)
	asm.Jcc(CondE, lTrue)
	asm.Jmp(lFalse)

	asm.BindLabel(lFloat)
	asm.MovRegMem(RDX, RDI, 16)
	asm.MovRegImm64(RAX, 0x7FFFFFFFFFFFFFFF)
	asm.AndRegReg(RDX, RAX)
	asm.TestRegReg(RDX, RDX)
	asm.Jcc(CondE, lTrue)
	asm.Jmp(lFalse)

	asm.BindLabel(lTrue)
	c.emitStoreBoolResult(RDI, 1)
	asm.Jmp(lDone)

	asm.BindLabel(lFalse)
	c.emitStoreBoolResult(RDI, 0)

	asm.BindLabel(lDone)
}

func (c *CompilerAMD64) emitMathBuiltin(id int8, currentIP int) {
	arity := 1
	switch id {
	case vm.MathBuiltinAbs, vm.MathBuiltinSqrt, vm.MathBuiltinRound, vm.MathBuiltinFloor, vm.MathBuiltinCeil:
		arity = 1
	case vm.MathBuiltinMin, vm.MathBuiltinMax:
		arity = 2
	case vm.MathBuiltinClamp:
		arity = 3
	default:
		c.emitBailout(currentIP)
		return
	}

	asm := c.asm
	asm.MovRegReg(RAX, RBX)
	asm.SubRegImm32(RAX, int32(arity))
	c.emitSlotAddrReg(RDI, R13, RAX)
	if arity > 1 {
		asm.IncReg(RAX)
		c.emitSlotAddrReg(RSI, R13, RAX)
	}
	if arity > 2 {
		asm.IncReg(RAX)
		c.emitSlotAddrReg(RDX, R13, RAX)
	}

	switch id {
	case vm.MathBuiltinAbs:
		c.emitMathAbs(currentIP)
	case vm.MathBuiltinSqrt:
		deopt := asm.CreateLabel()
		done := asm.CreateLabel()
		c.emitSlotToXMM(RDI, XMM0, deopt)
		asm.SqrtsdRegReg(XMM0, XMM0)
		c.emitStoreFloatResult(RDI, XMM0)
		asm.Jmp(done)
		asm.BindLabel(deopt)
		c.emitBailout(currentIP)
		asm.BindLabel(done)
	case vm.MathBuiltinRound, vm.MathBuiltinFloor, vm.MathBuiltinCeil:
		c.emitMathRounding(id, currentIP)
	case vm.MathBuiltinMin, vm.MathBuiltinMax:
		c.emitMathMinMax(id, currentIP)
	case vm.MathBuiltinClamp:
		c.emitMathClamp(currentIP)
	}

	for i := 1; i < arity; i++ {
		asm.DecReg(RBX)
	}
}

func (c *CompilerAMD64) emitMathAbs(currentIP int) {
	asm := c.asm
	deopt := asm.CreateLabel()
	isFloat := asm.CreateLabel()
	done := asm.CreateLabel()
	asm.CmpMemImm8(RDI, 0, byte(vm.SlotInt))
	asm.Jcc(CondNE, isFloat)
	asm.MovRegMem(RAX, RDI, 8)
	asm.TestRegReg(RAX, RAX)
	asm.Jcc(CondGE, done)
	asm.NegReg(RAX)
	asm.MovMemReg(RDI, 8, RAX)
	asm.Jmp(done)

	asm.BindLabel(isFloat)
	asm.CmpMemImm8(RDI, 0, byte(vm.SlotFloat))
	asm.Jcc(CondNE, deopt)
	asm.MovRegMem(RAX, RDI, 16)
	asm.ShlRegImm8(RAX, 1)
	asm.ShrRegImm8(RAX, 1)
	asm.MovMemReg(RDI, 16, RAX)
	asm.Jmp(done)
	asm.BindLabel(deopt)
	c.emitBailout(currentIP)
	asm.BindLabel(done)
}

func (c *CompilerAMD64) emitSlotToXMM(slot Register, dst XMMRegister, deopt Label) {
	asm := c.asm
	isFloat := asm.CreateLabel()
	done := asm.CreateLabel()
	asm.CmpMemImm8(slot, 0, byte(vm.SlotFloat))
	asm.Jcc(CondE, isFloat)
	asm.CmpMemImm8(slot, 0, byte(vm.SlotInt))
	asm.Jcc(CondNE, deopt)
	asm.MovRegMem(RAX, slot, 8)
	asm.Cvtsi2sdReg(dst, RAX)
	asm.Jmp(done)
	asm.BindLabel(isFloat)
	asm.MovsdRegMem(dst, slot, 16)
	asm.BindLabel(done)
}

func (c *CompilerAMD64) emitStoreFloatResult(slot Register, value XMMRegister) {
	asm := c.asm
	asm.MovMemImm8(slot, 0, byte(vm.SlotFloat))
	asm.MovMemImm32(slot, 8, 0)
	asm.MovsdMemReg(slot, 16, value)
	asm.MovMemImm32(slot, 24, 0)
	asm.MovMemImm32(slot, 32, 0)
}

func (c *CompilerAMD64) emitStoreIntResult(slot Register, value Register) {
	asm := c.asm
	asm.MovMemImm8(slot, 0, byte(vm.SlotInt))
	asm.MovMemReg(slot, 8, value)
	asm.MovMemImm32(slot, 16, 0)
	asm.MovMemImm32(slot, 24, 0)
	asm.MovMemImm32(slot, 32, 0)
}

func (c *CompilerAMD64) emitMathRounding(id int8, currentIP int) {
	asm := c.asm
	deopt := asm.CreateLabel()
	done := asm.CreateLabel()
	c.emitSlotToXMM(RDI, XMM0, deopt)
	if id == vm.MathBuiltinRound {
		negative := asm.CreateLabel()
		convert := asm.CreateLabel()
		asm.XorRegReg(RAX, RAX)
		asm.MovqXMMReg(XMM1, RAX)
		asm.UcomisdRegReg(XMM0, XMM1)
		asm.Jcc(CondP, convert)
		asm.Jcc(CondB, negative)
		asm.MovRegImm64(RAX, 0x3FE0000000000000)
		asm.MovqXMMReg(XMM1, RAX)
		asm.AddsdRegReg(XMM0, XMM1)
		asm.Jmp(convert)
		asm.BindLabel(negative)
		asm.MovRegImm64(RAX, 0x3FE0000000000000)
		asm.MovqXMMReg(XMM1, RAX)
		asm.SubsdRegReg(XMM0, XMM1)
		asm.BindLabel(convert)
		asm.Cvttsd2siReg(RAX, XMM0)
		c.emitStoreIntResult(RDI, RAX)
	} else {
		asm.Cvttsd2siReg(RAX, XMM0)
		store := asm.CreateLabel()
		asm.MovRegImm64(RCX, 0x8000000000000000)
		asm.CmpRegReg(RAX, RCX)
		asm.Jcc(CondE, store)
		asm.Cvtsi2sdReg(XMM1, RAX)
		asm.UcomisdRegReg(XMM0, XMM1)
		adjust := asm.CreateLabel()
		asm.Jcc(CondP, store)
		if id == vm.MathBuiltinFloor {
			asm.Jcc(CondB, adjust)
		} else {
			asm.Jcc(CondA, adjust)
		}
		asm.Jmp(store)
		asm.BindLabel(adjust)
		if id == vm.MathBuiltinFloor {
			asm.DecReg(RAX)
		} else {
			asm.IncReg(RAX)
		}
		asm.BindLabel(store)
		c.emitStoreIntResult(RDI, RAX)
	}
	asm.Jmp(done)
	asm.BindLabel(deopt)
	c.emitBailout(currentIP)
	asm.BindLabel(done)
}

func (c *CompilerAMD64) emitMathMinMax(id int8, currentIP int) {
	asm := c.asm
	deopt := asm.CreateLabel()
	chooseSecond := asm.CreateLabel()
	done := asm.CreateLabel()
	c.emitSlotToXMM(RDI, XMM0, deopt)
	c.emitSlotToXMM(RSI, XMM1, deopt)
	asm.UcomisdRegReg(XMM0, XMM1)
	asm.Jcc(CondP, chooseSecond)
	if id == vm.MathBuiltinMin {
		asm.Jcc(CondB, done)
	} else {
		asm.Jcc(CondA, done)
	}
	asm.BindLabel(chooseSecond)
	c.emitSlotCopy(RDI, RSI)
	asm.BindLabel(done)
	afterDeopt := asm.CreateLabel()
	asm.Jmp(afterDeopt)
	asm.BindLabel(deopt)
	c.emitBailout(currentIP)
	asm.BindLabel(afterDeopt)
}

func (c *CompilerAMD64) emitMathClamp(currentIP int) {
	asm := c.asm
	deopt := asm.CreateLabel()
	skipMin := asm.CreateLabel()
	skipMax := asm.CreateLabel()
	c.emitSlotToXMM(RDI, XMM0, deopt)
	c.emitSlotToXMM(RSI, XMM1, deopt)
	c.emitSlotToXMM(RDX, XMM2, deopt)
	asm.UcomisdRegReg(XMM0, XMM1)
	asm.Jcc(CondP, skipMin)
	asm.Jcc(CondAE, skipMin)
	asm.MovsdRegReg(XMM0, XMM1)
	asm.BindLabel(skipMin)
	asm.UcomisdRegReg(XMM0, XMM2)
	asm.Jcc(CondP, skipMax)
	asm.Jcc(CondBE, skipMax)
	asm.MovsdRegReg(XMM0, XMM2)
	asm.BindLabel(skipMax)
	storeFloat := asm.CreateLabel()
	finish := asm.CreateLabel()
	asm.CmpMemImm8(RDI, 0, byte(vm.SlotInt))
	asm.Jcc(CondNE, storeFloat)
	asm.Cvttsd2siReg(RAX, XMM0)
	c.emitStoreIntResult(RDI, RAX)
	asm.Jmp(finish)
	asm.BindLabel(storeFloat)
	asm.CmpMemImm8(RDI, 0, byte(vm.SlotFloat))
	asm.Jcc(CondNE, deopt)
	c.emitStoreFloatResult(RDI, XMM0)
	asm.Jmp(finish)
	asm.BindLabel(deopt)
	c.emitBailout(currentIP)
	asm.BindLabel(finish)
}

// emitEpilogue finishes function execution
func (c *CompilerAMD64) emitEpilogue(status int64) {
	c.asm.Jmp(c.exitLabel)
}

// emitBailout saves state and deoptimizes to bytecode VM at given IP
func (c *CompilerAMD64) emitBailout(currentIP int) {
	asm := c.asm
	asm.MovMemImm32(R12, 48, int32(currentIP)) // ctx.BailoutIP = currentIP
	asm.Jmp(c.deoptLabel)
}

// emitHelperCall records a runtime-helper request and returns to the Go JIT
// engine. The native stack/SP are left untouched, so the engine can inspect the
// helper's operands and arguments, execute it, and re-enter at resumeIP.
func (c *CompilerAMD64) emitHelperCall(currentIP, resumeIP, numArgs int) {
	asm := c.asm
	asm.MovMemImm32(R12, 64, int32(currentIP))
	asm.MovMemImm32(R12, 72, int32(resumeIP))
	asm.MovMemImm32(R12, 80, int32(numArgs))
	asm.Jmp(c.helperLabel)
}
