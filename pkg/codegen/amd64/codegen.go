// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

package amd64

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"

	"github.com/pawl-lang/pawl/pkg/ir"
	"github.com/pawl-lang/pawl/pkg/jit"
)

// ABI specifies the calling convention.
type ABI int

const (
	ABIWin64 ABI = iota
	ABISysV
)

// CallFixup records a call site to be resolved by the linker.
type CallFixup struct {
	FuncName    string
	PatchOffset int
	InsEnd      int
}

// CompiledArtifact holds the machine code, rodata, call fixups and metadata.
type CompiledArtifact struct {
	Code         []byte
	ReadOnlyData []byte
	FunctionRVAs map[string]uint32
	EntryOffset  uint32
	CallFixups   []CallFixup
}

// CodeGenerator translates an IR Program into AMD64 machine code.
type CodeGenerator struct {
	abi          ABI
	asm          *jit.AssemblerAMD64
	funcOffsets  map[string]int
	funcCalls    []funcCallFixup
	funcRVAs     map[string]uint32
	entryOffset  uint32

	rodata       *bytes.Buffer
	stringPool   map[string]int32 // string content -> offset in rodata
	strFixups    []strFixup
}

type strFixup struct {
	rodataOffset int32
	patchOffset  int
	insEnd       int
}

type funcCallFixup struct {
	funcName    string
	patchOffset int
	insEnd      int
}

// NewCodeGenerator initializes an AMD64 codegen for Win64 or SysV.
func NewCodeGenerator(abi ABI) *CodeGenerator {
	return &CodeGenerator{
		abi:         abi,
		asm:         jit.NewAssemblerAMD64(),
		funcOffsets: make(map[string]int),
		funcRVAs:    make(map[string]uint32),
		rodata:      new(bytes.Buffer),
		stringPool:  make(map[string]int32),
	}
}

// internString places (len: int64, data: bytes, null: byte) in rodata.
func (g *CodeGenerator) internString(s string) int32 {
	if off, ok := g.stringPool[s]; ok {
		return off
	}
	// 8-byte align rodata entry
	for g.rodata.Len()%8 != 0 {
		g.rodata.WriteByte(0)
	}
	off := int32(g.rodata.Len())
	g.stringPool[s] = off

	// String object layout:
	// offset 0: len (int64)
	// offset 8: data bytes + null terminator
	_ = binary.Write(g.rodata, binary.LittleEndian, int64(len(s)))
	g.rodata.WriteString(s)
	g.rodata.WriteByte(0) // null-terminated for safety

	return off
}

// Generate translates the whole IR Program.
func (g *CodeGenerator) Generate(prog *ir.Program) (*CompiledArtifact, error) {
	// First compile user functions
	for _, fn := range prog.Functions {
		if fn == prog.EntryFn {
			continue
		}
		if err := g.compileFunction(fn); err != nil {
			return nil, err
		}
	}

	// Then compile entry function (main)
	if prog.EntryFn != nil {
		g.entryOffset = uint32(g.asm.CodeLen())
		if err := g.compileFunction(prog.EntryFn); err != nil {
			return nil, err
		}
	}

	// Resolve direct function call fixups
	code, err := g.asm.Finalize()
	if err != nil {
		return nil, err
	}

	var unpatchedFixups []CallFixup
	for _, fix := range g.funcCalls {
		targetOff, exists := g.funcOffsets[fix.funcName]
		if exists {
			rel32 := int32(targetOff - fix.insEnd)
			binary.LittleEndian.PutUint32(code[fix.patchOffset:], uint32(rel32))
		} else {
			// Leave for linker (runtime helpers: pawl_print, pawl_println, etc.)
			unpatchedFixups = append(unpatchedFixups, CallFixup{
				FuncName:    fix.funcName,
				PatchOffset: fix.patchOffset,
				InsEnd:      fix.insEnd,
			})
		}
	}

	for name, off := range g.funcOffsets {
		g.funcRVAs[name] = uint32(off)
	}

	return &CompiledArtifact{
		Code:         code,
		ReadOnlyData: g.rodata.Bytes(),
		FunctionRVAs: g.funcRVAs,
		EntryOffset:  g.entryOffset,
		CallFixups:   unpatchedFixups,
	}, nil
}

type funcContext struct {
	irFn         *ir.Function
	blockLabels  map[*ir.Block]jit.Label
	slotOffsets  map[int]int32 // VReg.ID -> [rbp - slotOffsets]
	frameSize    int32
}

func (g *CodeGenerator) compileFunction(fn *ir.Function) error {
	for g.asm.CodeLen()%16 != 0 {
		g.asm.Nop()
	}

	g.funcOffsets[fn.Name] = g.asm.CodeLen()

	ctx := &funcContext{
		irFn:        fn,
		blockLabels: make(map[*ir.Block]jit.Label),
		slotOffsets: make(map[int]int32),
	}

	for _, b := range fn.Blocks {
		ctx.blockLabels[b] = g.asm.CreateLabel()
	}

	totalVRegs := fn.NextVRegID
	if totalVRegs < len(fn.Params) {
		totalVRegs = len(fn.Params)
	}

	for id := 0; id < totalVRegs; id++ {
		ctx.slotOffsets[id] = int32((id + 1) * 8)
	}

	localBytes := int32(totalVRegs * 8)
	if g.abi == ABIWin64 {
		localBytes += 32
	}
	frameSize := localBytes
	if (frameSize+8)%16 != 0 {
		frameSize += 16 - ((frameSize + 8) % 16)
	}
	ctx.frameSize = frameSize

	// Prologue
	g.asm.Push(jit.RBP)
	g.asm.MovRegReg(jit.RBP, jit.RSP)
	if ctx.frameSize > 0 {
		g.asm.SubRegImm32(jit.RSP, ctx.frameSize)
	}

	g.storeParams(ctx)

	for _, b := range fn.Blocks {
		g.asm.BindLabel(ctx.blockLabels[b])
		for _, inst := range b.Instructions {
			if err := g.emitInstruction(ctx, inst); err != nil {
				return err
			}
		}
	}

	return nil
}

func (g *CodeGenerator) storeParams(ctx *funcContext) {
	var paramRegs []jit.Register
	if g.abi == ABIWin64 {
		paramRegs = []jit.Register{jit.RCX, jit.RDX, jit.R8, jit.R9}
	} else {
		paramRegs = []jit.Register{jit.RDI, jit.RSI, jit.RDX, jit.RCX, jit.R8, jit.R9}
	}

	for i, p := range ctx.irFn.Params {
		disp := -ctx.slotOffsets[p.ID]
		if i < len(paramRegs) {
			r := paramRegs[i]
			g.asm.MovMemReg(jit.RBP, disp, r)
		} else {
			var callerDisp int32
			if g.abi == ABIWin64 {
				callerDisp = int32(48 + (i-4)*8)
			} else {
				callerDisp = int32(16 + (i-6)*8)
			}
			g.asm.MovRegMem(jit.RAX, jit.RBP, callerDisp)
			g.asm.MovMemReg(jit.RBP, disp, jit.RAX)
		}
	}
}

func (g *CodeGenerator) emitInstruction(ctx *funcContext, inst *ir.Instruction) error {
	switch inst.Op {
	case ir.OpConstInt:
		disp := -ctx.slotOffsets[inst.Dst.ID]
		g.asm.MovRegImm64(jit.RAX, uint64(inst.IntVal))
		g.asm.MovMemReg(jit.RBP, disp, jit.RAX)

	case ir.OpConstBool:
		disp := -ctx.slotOffsets[inst.Dst.ID]
		g.asm.MovRegImm32(jit.RAX, int32(inst.IntVal))
		g.asm.MovMemReg(jit.RBP, disp, jit.RAX)

	case ir.OpConstStr:
		disp := -ctx.slotOffsets[inst.Dst.ID]
		strOff := g.internString(inst.StrVal)
		// We emit lea rax, [rip + strOff] or pass relative offset;
		// For now in single code/rodata space, store the rodata offset
		g.asm.MovRegImm64(jit.RAX, uint64(strOff))
		g.asm.MovMemReg(jit.RBP, disp, jit.RAX)

	case ir.OpPrint:
		return g.emitRuntimeCall(ctx, "pawl_print", inst.Src1)

	case ir.OpPrintln:
		return g.emitRuntimeCall(ctx, "pawl_println", inst.Src1)

	case ir.OpConcatStr:
		return g.emitRuntimeCall2(ctx, "pawl_concat_str", inst.Dst, inst.Src1, inst.Src2)

	case ir.OpIntToStr:
		return g.emitRuntimeCall1Ret(ctx, "pawl_int_to_str", inst.Dst, inst.Src1)

	case ir.OpConstFloat:
		disp := -ctx.slotOffsets[inst.Dst.ID]
		bits := math.Float64bits(inst.FloatVal)
		g.asm.MovRegImm64(jit.RAX, bits)
		g.asm.MovMemReg(jit.RBP, disp, jit.RAX)

	case ir.OpCopy:
		srcDisp := -ctx.slotOffsets[inst.Src1.ID]
		dstDisp := -ctx.slotOffsets[inst.Dst.ID]
		g.asm.MovRegMem(jit.RAX, jit.RBP, srcDisp)
		g.asm.MovMemReg(jit.RBP, dstDisp, jit.RAX)

	case ir.OpAdd:
		g.emitBinaryOp(ctx, inst, func() { g.asm.AddRegReg(jit.RAX, jit.RCX) })
	case ir.OpSub:
		g.emitBinaryOp(ctx, inst, func() { g.asm.SubRegReg(jit.RAX, jit.RCX) })
	case ir.OpMul:
		g.emitBinaryOp(ctx, inst, func() { g.asm.ImulRegReg(jit.RAX, jit.RCX) })
	case ir.OpDiv:
		s1 := -ctx.slotOffsets[inst.Src1.ID]
		s2 := -ctx.slotOffsets[inst.Src2.ID]
		dst := -ctx.slotOffsets[inst.Dst.ID]
		g.asm.MovRegMem(jit.RAX, jit.RBP, s1)
		g.asm.MovRegMem(jit.RCX, jit.RBP, s2)
		g.asm.Cqo()
		g.asm.IdivReg(jit.RCX)
		g.asm.MovMemReg(jit.RBP, dst, jit.RAX)
	case ir.OpMod:
		s1 := -ctx.slotOffsets[inst.Src1.ID]
		s2 := -ctx.slotOffsets[inst.Src2.ID]
		dst := -ctx.slotOffsets[inst.Dst.ID]
		g.asm.MovRegMem(jit.RAX, jit.RBP, s1)
		g.asm.MovRegMem(jit.RCX, jit.RBP, s2)
		g.asm.Cqo()
		g.asm.IdivReg(jit.RCX)
		g.asm.MovMemReg(jit.RBP, dst, jit.RDX)

	case ir.OpNeg:
		s1 := -ctx.slotOffsets[inst.Src1.ID]
		dst := -ctx.slotOffsets[inst.Dst.ID]
		g.asm.MovRegMem(jit.RAX, jit.RBP, s1)
		g.asm.NegReg(jit.RAX)
		g.asm.MovMemReg(jit.RBP, dst, jit.RAX)

	case ir.OpNot:
		s1 := -ctx.slotOffsets[inst.Src1.ID]
		dst := -ctx.slotOffsets[inst.Dst.ID]
		g.asm.MovRegMem(jit.RAX, jit.RBP, s1)
		g.asm.TestRegReg(jit.RAX, jit.RAX)
		g.asm.Setcc(jit.CondE, jit.RAX)
		g.asm.MovzxByte(jit.RAX, jit.RAX)
		g.asm.MovMemReg(jit.RBP, dst, jit.RAX)

	case ir.OpEq:
		g.emitCmp(ctx, inst, jit.CondE)
	case ir.OpNe:
		g.emitCmp(ctx, inst, jit.CondNE)
	case ir.OpLt:
		g.emitCmp(ctx, inst, jit.CondL)
	case ir.OpLe:
		g.emitCmp(ctx, inst, jit.CondLE)
	case ir.OpGt:
		g.emitCmp(ctx, inst, jit.CondG)
	case ir.OpGe:
		g.emitCmp(ctx, inst, jit.CondGE)

	case ir.OpJmp:
		lbl := ctx.blockLabels[inst.Target]
		g.asm.Jmp(lbl)

	case ir.OpBranch:
		condDisp := -ctx.slotOffsets[inst.Src1.ID]
		g.asm.MovRegMem(jit.RAX, jit.RBP, condDisp)
		g.asm.TestRegReg(jit.RAX, jit.RAX)

		thenLbl := ctx.blockLabels[inst.Target]
		elseLbl := ctx.blockLabels[inst.ElseTgt]

		g.asm.Jcc(jit.CondNE, thenLbl)
		g.asm.Jmp(elseLbl)

	case ir.OpCall:
		return g.emitCall(ctx, inst)

	case ir.OpAlloc:
		// Alloc size in bytes is in inst.Src1
		return g.emitRuntimeCall1Ret(ctx, "pawl_alloc", inst.Dst, inst.Src1)

	case ir.OpLoad:
		// Dst = *(Src1 + IntVal)
		baseDisp := -ctx.slotOffsets[inst.Src1.ID]
		dstDisp := -ctx.slotOffsets[inst.Dst.ID]
		g.asm.MovRegMem(jit.RAX, jit.RBP, baseDisp)
		switch inst.Dst.Type {
		case ir.TypeI8:
			g.asm.EmitBytes(0x0F, 0xB6, 0x80) // movzx eax, byte ptr [rax + disp32]
			g.asm.EmitUint32(uint32(inst.IntVal))
		default:
			// 64-bit load: mov rax, [rax + disp32]
			g.asm.EmitBytes(0x48, 0x8B, 0x80)
			g.asm.EmitUint32(uint32(inst.IntVal))
		}
		g.asm.MovMemReg(jit.RBP, dstDisp, jit.RAX)

	case ir.OpStore:
		// *(Src1 + IntVal) = Src2
		baseDisp := -ctx.slotOffsets[inst.Src1.ID]
		valDisp := -ctx.slotOffsets[inst.Src2.ID]
		g.asm.MovRegMem(jit.RAX, jit.RBP, baseDisp)
		g.asm.MovRegMem(jit.RCX, jit.RBP, valDisp)
		switch inst.Src2.Type {
		case ir.TypeI8:
			// mov [rax + disp32], cl
			g.asm.EmitBytes(0x88, 0x88)
			g.asm.EmitUint32(uint32(inst.IntVal))
		default:
			// mov [rax + disp32], rcx
			g.asm.EmitBytes(0x48, 0x89, 0x88)
			g.asm.EmitUint32(uint32(inst.IntVal))
		}

	case ir.OpCallPtr:
		return g.emitCallPtr(ctx, inst)

	case ir.OpSetErr:
		// mov r14, [rbp - disp]
		srcDisp := -ctx.slotOffsets[inst.Src1.ID]
		g.asm.MovRegMem(jit.R14, jit.RBP, srcDisp)

	case ir.OpGetErr:
		// mov [rbp - disp], r14
		dstDisp := -ctx.slotOffsets[inst.Dst.ID]
		g.asm.MovMemReg(jit.RBP, dstDisp, jit.R14)

	case ir.OpRet:
		if inst.Src1.Type != ir.TypeVoid && inst.Src1.ID >= 0 {
			srcDisp := -ctx.slotOffsets[inst.Src1.ID]
			g.asm.MovRegMem(jit.RAX, jit.RBP, srcDisp)
		}
		g.asm.MovRegReg(jit.RSP, jit.RBP)
		g.asm.Pop(jit.RBP)
		g.asm.Ret()

	default:
		return fmt.Errorf("codegen: unsupported opcode %s", inst.Op)
	}
	return nil
}

func (g *CodeGenerator) emitRuntimeCall(ctx *funcContext, runtimeFn string, arg ir.VReg) error {
	argDisp := -ctx.slotOffsets[arg.ID]
	if g.abi == ABIWin64 {
		g.asm.MovRegMem(jit.RCX, jit.RBP, argDisp)
	} else {
		g.asm.MovRegMem(jit.RDI, jit.RBP, argDisp)
	}
	g.asm.EmitByte(0xE8)
	patch := g.asm.CodeLen()
	g.asm.EmitUint32(0)
	g.funcCalls = append(g.funcCalls, funcCallFixup{
		funcName:    runtimeFn,
		patchOffset: patch,
		insEnd:      patch + 4,
	})
	return nil
}

func (g *CodeGenerator) emitRuntimeCall1Ret(ctx *funcContext, runtimeFn string, dst, arg ir.VReg) error {
	argDisp := -ctx.slotOffsets[arg.ID]
	if g.abi == ABIWin64 {
		g.asm.MovRegMem(jit.RCX, jit.RBP, argDisp)
	} else {
		g.asm.MovRegMem(jit.RDI, jit.RBP, argDisp)
	}
	g.asm.EmitByte(0xE8)
	patch := g.asm.CodeLen()
	g.asm.EmitUint32(0)
	g.funcCalls = append(g.funcCalls, funcCallFixup{
		funcName:    runtimeFn,
		patchOffset: patch,
		insEnd:      patch + 4,
	})
	dstDisp := -ctx.slotOffsets[dst.ID]
	g.asm.MovMemReg(jit.RBP, dstDisp, jit.RAX)
	return nil
}

func (g *CodeGenerator) emitRuntimeCall2(ctx *funcContext, runtimeFn string, dst, arg1, arg2 ir.VReg) error {
	s1 := -ctx.slotOffsets[arg1.ID]
	s2 := -ctx.slotOffsets[arg2.ID]
	if g.abi == ABIWin64 {
		g.asm.MovRegMem(jit.RCX, jit.RBP, s1)
		g.asm.MovRegMem(jit.RDX, jit.RBP, s2)
	} else {
		g.asm.MovRegMem(jit.RDI, jit.RBP, s1)
		g.asm.MovRegMem(jit.RSI, jit.RBP, s2)
	}
	g.asm.EmitByte(0xE8)
	patch := g.asm.CodeLen()
	g.asm.EmitUint32(0)
	g.funcCalls = append(g.funcCalls, funcCallFixup{
		funcName:    runtimeFn,
		patchOffset: patch,
		insEnd:      patch + 4,
	})
	dstDisp := -ctx.slotOffsets[dst.ID]
	g.asm.MovMemReg(jit.RBP, dstDisp, jit.RAX)
	return nil
}

func (g *CodeGenerator) emitBinaryOp(ctx *funcContext, inst *ir.Instruction, opFn func()) {
	s1 := -ctx.slotOffsets[inst.Src1.ID]
	s2 := -ctx.slotOffsets[inst.Src2.ID]
	dst := -ctx.slotOffsets[inst.Dst.ID]
	g.asm.MovRegMem(jit.RAX, jit.RBP, s1)
	g.asm.MovRegMem(jit.RCX, jit.RBP, s2)
	opFn()
	g.asm.MovMemReg(jit.RBP, dst, jit.RAX)
}

func (g *CodeGenerator) emitCmp(ctx *funcContext, inst *ir.Instruction, cond jit.Condition) {
	s1 := -ctx.slotOffsets[inst.Src1.ID]
	s2 := -ctx.slotOffsets[inst.Src2.ID]
	dst := -ctx.slotOffsets[inst.Dst.ID]

	g.asm.MovRegMem(jit.RAX, jit.RBP, s1)
	g.asm.MovRegMem(jit.RCX, jit.RBP, s2)
	g.asm.CmpRegReg(jit.RAX, jit.RCX)
	g.asm.Setcc(cond, jit.RAX)
	g.asm.MovzxByte(jit.RAX, jit.RAX)
	g.asm.MovMemReg(jit.RBP, dst, jit.RAX)
}

func (g *CodeGenerator) emitCall(ctx *funcContext, inst *ir.Instruction) error {
	var paramRegs []jit.Register
	if g.abi == ABIWin64 {
		paramRegs = []jit.Register{jit.RCX, jit.RDX, jit.R8, jit.R9}
	} else {
		paramRegs = []jit.Register{jit.RDI, jit.RSI, jit.RDX, jit.RCX, jit.R8, jit.R9}
	}

	for i, arg := range inst.Args {
		argDisp := -ctx.slotOffsets[arg.ID]
		if i < len(paramRegs) {
			reg := paramRegs[i]
			g.asm.MovRegMem(reg, jit.RBP, argDisp)
		} else {
			g.asm.MovRegMem(jit.RAX, jit.RBP, argDisp)
			g.asm.Push(jit.RAX)
		}
	}

	g.asm.EmitByte(0xE8)
	patch := g.asm.CodeLen()
	g.asm.EmitUint32(0)
	g.funcCalls = append(g.funcCalls, funcCallFixup{
		funcName:    inst.CallName,
		patchOffset: patch,
		insEnd:      patch + 4,
	})

	if inst.Dst.Type != ir.TypeVoid && inst.Dst.ID >= 0 {
		dstDisp := -ctx.slotOffsets[inst.Dst.ID]
		g.asm.MovMemReg(jit.RBP, dstDisp, jit.RAX)
	}

	return nil
}

func (g *CodeGenerator) emitCallPtr(ctx *funcContext, inst *ir.Instruction) error {
	var paramRegs []jit.Register
	if g.abi == ABIWin64 {
		paramRegs = []jit.Register{jit.RCX, jit.RDX, jit.R8, jit.R9}
	} else {
		paramRegs = []jit.Register{jit.RDI, jit.RSI, jit.RDX, jit.RCX, jit.R8, jit.R9}
	}

	for i, arg := range inst.Args {
		argDisp := -ctx.slotOffsets[arg.ID]
		if i < len(paramRegs) {
			reg := paramRegs[i]
			g.asm.MovRegMem(reg, jit.RBP, argDisp)
		} else {
			g.asm.MovRegMem(jit.RAX, jit.RBP, argDisp)
			g.asm.Push(jit.RAX)
		}
	}

	// Function pointer is in Src1
	fnPtrDisp := -ctx.slotOffsets[inst.Src1.ID]
	g.asm.MovRegMem(jit.RAX, jit.RBP, fnPtrDisp)
	// call rax: ff d0
	g.asm.EmitBytes(0xFF, 0xD0)

	if inst.Dst.Type != ir.TypeVoid && inst.Dst.ID >= 0 {
		dstDisp := -ctx.slotOffsets[inst.Dst.ID]
		g.asm.MovMemReg(jit.RBP, dstDisp, jit.RAX)
	}

	return nil
}
