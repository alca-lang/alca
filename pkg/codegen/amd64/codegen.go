// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

package amd64

import (
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

// CompiledArtifact holds the machine code and metadata.
type CompiledArtifact struct {
	Code         []byte
	FunctionRVAs map[string]uint32
	EntryOffset  uint32
}

// CodeGenerator translates an IR Program into AMD64 machine code.
type CodeGenerator struct {
	abi          ABI
	asm          *jit.AssemblerAMD64
	funcOffsets  map[string]int
	funcCalls    []funcCallFixup
	funcRVAs     map[string]uint32
	entryOffset  uint32
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
	}
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

	for _, fix := range g.funcCalls {
		targetOff, exists := g.funcOffsets[fix.funcName]
		if !exists {
			return nil, fmt.Errorf("undefined function %q called in native codegen", fix.funcName)
		}
		rel32 := int32(targetOff - fix.insEnd)
		binary.LittleEndian.PutUint32(code[fix.patchOffset:], uint32(rel32))
	}

	for name, off := range g.funcOffsets {
		g.funcRVAs[name] = uint32(off)
	}

	return &CompiledArtifact{
		Code:         code,
		FunctionRVAs: g.funcRVAs,
		EntryOffset:  g.entryOffset,
	}, nil
}

type funcContext struct {
	irFn         *ir.Function
	blockLabels  map[*ir.Block]jit.Label
	slotOffsets  map[int]int32 // VReg.ID -> [rbp - slotOffsets]
	frameSize    int32
}

func (g *CodeGenerator) compileFunction(fn *ir.Function) error {
	// 16-byte align function start
	for g.asm.CodeLen()%16 != 0 {
		g.asm.Nop()
	}

	g.funcOffsets[fn.Name] = g.asm.CodeLen()

	ctx := &funcContext{
		irFn:        fn,
		blockLabels: make(map[*ir.Block]jit.Label),
		slotOffsets: make(map[int]int32),
	}

	// Pre-create labels for all blocks
	for _, b := range fn.Blocks {
		ctx.blockLabels[b] = g.asm.CreateLabel()
	}

	// Stack slot layout: assign an 8-byte stack slot for every VReg
	totalVRegs := fn.NextVRegID
	if totalVRegs < len(fn.Params) {
		totalVRegs = len(fn.Params)
	}

	// Slot offset from RBP: [rbp - 8], [rbp - 16], ...
	for id := 0; id < totalVRegs; id++ {
		ctx.slotOffsets[id] = int32((id + 1) * 8)
	}

	// Total local area = totalVRegs * 8, plus optional 32-byte shadow space for Win64 calls
	localBytes := int32(totalVRegs * 8)
	if g.abi == ABIWin64 {
		localBytes += 32 // shadow space for outgoing calls
	}
	// Align stack frame to 16 bytes: (frameSize + 8 (saved rbp)) % 16 == 0 => frameSize % 16 == 8
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

	// Move parameters from ABI registers into their stack slots
	g.storeParams(ctx)

	// Emit blocks
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
	// ABI integer/pointer parameter registers:
	// Win64: RCX, RDX, R8, R9
	// SysV:  RDI, RSI, RDX, RCX, R8, R9
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
			// Stack argument passed by caller:
			// Win64: [rbp + 16 + 32 + (i-4)*8]
			// SysV:  [rbp + 16 + (i-6)*8]
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
		g.asm.MovMemReg(jit.RBP, dst, jit.RDX) // modulo in RDX

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
		g.asm.Setcc(jit.CondE, jit.RAX) // if 0 -> 1, else 0
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

	case ir.OpRet:
		if inst.Src1.Type != ir.TypeVoid && inst.Src1.ID >= 0 {
			srcDisp := -ctx.slotOffsets[inst.Src1.ID]
			g.asm.MovRegMem(jit.RAX, jit.RBP, srcDisp)
		}
		// Epilogue
		g.asm.MovRegReg(jit.RSP, jit.RBP)
		g.asm.Pop(jit.RBP)
		g.asm.Ret()

	default:
		return fmt.Errorf("codegen: unsupported opcode %s", inst.Op)
	}
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

	// Evaluate / pass register arguments
	for i, arg := range inst.Args {
		argDisp := -ctx.slotOffsets[arg.ID]
		if i < len(paramRegs) {
			reg := paramRegs[i]
			g.asm.MovRegMem(reg, jit.RBP, argDisp)
		} else {
			// Push stack argument
			g.asm.MovRegMem(jit.RAX, jit.RBP, argDisp)
			g.asm.Push(jit.RAX)
		}
	}

	// Call rel32
	g.asm.EmitByte(0xE8)
	patch := g.asm.CodeLen()
	g.asm.EmitUint32(0) // placeholder
	g.funcCalls = append(g.funcCalls, funcCallFixup{
		funcName:    inst.CallName,
		patchOffset: patch,
		insEnd:      patch + 4,
	})

	// Store return value if non-void
	if inst.Dst.Type != ir.TypeVoid && inst.Dst.ID >= 0 {
		dstDisp := -ctx.slotOffsets[inst.Dst.ID]
		g.asm.MovMemReg(jit.RBP, dstDisp, jit.RAX)
	}

	return nil
}
