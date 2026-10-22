package jit

import (
	"encoding/binary"
	"fmt"
)

// Register represents an x86-64 64-bit general purpose register.
type Register byte

const (
	RAX Register = 0
	RCX Register = 1
	RDX Register = 2
	RBX Register = 3
	RSP Register = 4
	RBP Register = 5
	RSI Register = 6
	RDI Register = 7
	R8  Register = 8
	R9  Register = 9
	R10 Register = 10
	R11 Register = 11
	R12 Register = 12
	R13 Register = 13
	R14 Register = 14
	R15 Register = 15
)

// Condition represents an x86 conditional jump condition.
type Condition byte

const (
	CondB  Condition = 0x82 // <  (jb / jc / below - float <)
	CondAE Condition = 0x83 // >= (jnb / jnc / above or equal - float >=)
	CondE  Condition = 0x84 // == (je / jz)
	CondNE Condition = 0x85 // != (jne / jnz)
	CondBE Condition = 0x86 // <= (jbe / below or equal - float <=)
	CondA  Condition = 0x87 // >  (ja / above - float >)
	CondL  Condition = 0x8C // <  (jl - signed <)
	CondGE Condition = 0x8D // >= (jge - signed >=)
	CondLE Condition = 0x8E // <= (jle - signed <=)
	CondG  Condition = 0x8F // >  (jg - signed >)
	CondP  Condition = 0x8A // parity set (unordered floating-point comparison)
)

// Label represents a jump target location in the generated machine code.
type Label int

type jumpFixup struct {
	label       Label
	patchOffset int // Offset where 4-byte displacement must be written
	insEnd      int // Offset immediately following the jump instruction
}

// AssemblerAMD64 is a lightweight, high-performance x86-64 machine code generator.
type AssemblerAMD64 struct {
	code   []byte
	labels map[Label]int
	fixups []jumpFixup
	nextID int
}

// NewAssemblerAMD64 creates an empty x86-64 assembler.
func NewAssemblerAMD64() *AssemblerAMD64 {
	return &AssemblerAMD64{
		code:   make([]byte, 0, 1024),
		labels: make(map[Label]int),
		fixups: make([]jumpFixup, 0, 32),
	}
}

// CreateLabel allocates a new unplaced jump label.
func (a *AssemblerAMD64) CreateLabel() Label {
	l := Label(a.nextID)
	a.nextID++
	return l
}

// BindLabel places the label at the current code position.
func (a *AssemblerAMD64) BindLabel(l Label) {
	a.labels[l] = len(a.code)
}

func (a *AssemblerAMD64) emitByte(b byte) {
	a.code = append(a.code, b)
}

func (a *AssemblerAMD64) emitBytes(b ...byte) {
	a.code = append(a.code, b...)
}

func (a *AssemblerAMD64) emitUint32(val uint32) {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], val)
	a.code = append(a.code, buf[:]...)
}

func (a *AssemblerAMD64) emitUint64(val uint64) {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], val)
	a.code = append(a.code, buf[:]...)
}

func modRM(mod byte, reg byte, rm byte) byte {
	return (mod << 6) | ((reg & 7) << 3) | (rm & 7)
}

func sib(scale byte, index byte, base byte) byte {
	return (scale << 6) | ((index & 7) << 3) | (base & 7)
}

// rex emits a 64-bit REX prefix (REX.W = 1).
func (a *AssemblerAMD64) rexW(r, x, b Register) {
	prefix := byte(0x48)
	if r >= 8 {
		prefix |= 0x04
	}
	if x >= 8 {
		prefix |= 0x02
	}
	if b >= 8 {
		prefix |= 0x01
	}
	a.emitByte(prefix)
}

// rex emits a 32-bit REX prefix (REX.W = 0) if extension registers are referenced.
func (a *AssemblerAMD64) rex(r, x, b Register) {
	prefix := byte(0x40)
	if r >= 8 {
		prefix |= 0x04
	}
	if x >= 8 {
		prefix |= 0x02
	}
	if b >= 8 {
		prefix |= 0x01
	}
	if prefix != 0x40 {
		a.emitByte(prefix)
	}
}

// Push pushes a 64-bit register onto the hardware stack.
func (a *AssemblerAMD64) Push(r Register) {
	if r >= 8 {
		a.emitByte(0x41)
	}
	a.emitByte(0x50 + (byte(r) & 7))
}

// Pop pops a 64-bit register from the hardware stack.
func (a *AssemblerAMD64) Pop(r Register) {
	if r >= 8 {
		a.emitByte(0x41)
	}
	a.emitByte(0x58 + (byte(r) & 7))
}

// MovRegImm64: mov reg, imm64
func (a *AssemblerAMD64) MovRegImm64(dst Register, imm uint64) {
	a.rexW(0, 0, dst)
	a.emitByte(0xB8 + (byte(dst) & 7))
	a.emitUint64(imm)
}

// MovRegImm32: mov reg, imm32 (sign-extended to 64-bit)
func (a *AssemblerAMD64) MovRegImm32(dst Register, imm int32) {
	a.rexW(0, 0, dst)
	a.emitByte(0xC7)
	a.emitByte(modRM(0b11, 0, byte(dst)))
	a.emitUint32(uint32(imm))
}

// MovRegReg: mov dst, src (64-bit)
func (a *AssemblerAMD64) MovRegReg(dst, src Register) {
	a.rexW(src, 0, dst)
	a.emitByte(0x89)
	a.emitByte(modRM(0b11, byte(src), byte(dst)))
}

// emitMemOperand encodes [base + disp32] with SIB handling.
func (a *AssemblerAMD64) emitMemOperand(reg Register, base Register, disp int32) {
	// If base is RSP or R12, SIB byte is required by x86 architecture.
	needsSIB := (base == RSP || base == R12)
	a.emitByte(modRM(0b10, byte(reg), byte(base)))
	if needsSIB {
		a.emitByte(sib(0, 4, byte(base))) // index 4 = none
	}
	a.emitUint32(uint32(disp))
}

// MovRegMem: mov dst, [base + disp32] (64-bit load)
func (a *AssemblerAMD64) MovRegMem(dst, base Register, disp int32) {
	a.rexW(dst, 0, base)
	a.emitByte(0x8B)
	a.emitMemOperand(dst, base, disp)
}

// MovMemReg: mov [base + disp32], src (64-bit store)
func (a *AssemblerAMD64) MovMemReg(base Register, disp int32, src Register) {
	a.rexW(src, 0, base)
	a.emitByte(0x89)
	a.emitMemOperand(src, base, disp)
}

// MovMemImm32: mov qword ptr [base + disp32], imm32
func (a *AssemblerAMD64) MovMemImm32(base Register, disp int32, imm int32) {
	a.rexW(0, 0, base)
	a.emitByte(0xC7)
	a.emitMemOperand(0, base, disp)
	a.emitUint32(uint32(imm))
}

// MovMemImm8: mov byte ptr [base + disp32], imm8
func (a *AssemblerAMD64) MovMemImm8(base Register, disp int32, imm byte) {
	a.rex(0, 0, base)
	a.emitByte(0xC6)
	a.emitMemOperand(0, base, disp)
	a.emitByte(imm)
}

// AddRegReg: add dst, src (64-bit)
func (a *AssemblerAMD64) AddRegReg(dst, src Register) {
	a.rexW(src, 0, dst)
	a.emitByte(0x01)
	a.emitByte(modRM(0b11, byte(src), byte(dst)))
}

// AddRegMem: add dst, [base + disp32]
func (a *AssemblerAMD64) AddRegMem(dst, base Register, disp int32) {
	a.rexW(dst, 0, base)
	a.emitByte(0x03)
	a.emitMemOperand(dst, base, disp)
}

// AddMemReg: add [base + disp32], src
func (a *AssemblerAMD64) AddMemReg(base Register, disp int32, src Register) {
	a.rexW(src, 0, base)
	a.emitByte(0x01)
	a.emitMemOperand(src, base, disp)
}

// AddRegImm32: add dst, imm32
func (a *AssemblerAMD64) AddRegImm32(dst Register, imm int32) {
	a.rexW(0, 0, dst)
	a.emitByte(0x81)
	a.emitByte(modRM(0b11, 0, byte(dst)))
	a.emitUint32(uint32(imm))
}

// SubRegReg: sub dst, src (64-bit)
func (a *AssemblerAMD64) SubRegReg(dst, src Register) {
	a.rexW(src, 0, dst)
	a.emitByte(0x29)
	a.emitByte(modRM(0b11, byte(src), byte(dst)))
}

// SubRegMem: sub dst, [base + disp32]
func (a *AssemblerAMD64) SubRegMem(dst, base Register, disp int32) {
	a.rexW(dst, 0, base)
	a.emitByte(0x2B)
	a.emitMemOperand(dst, base, disp)
}

// SubMemReg: sub [base + disp32], src
func (a *AssemblerAMD64) SubMemReg(base Register, disp int32, src Register) {
	a.rexW(src, 0, base)
	a.emitByte(0x29)
	a.emitMemOperand(src, base, disp)
}

// SubRegImm32: sub dst, imm32
func (a *AssemblerAMD64) SubRegImm32(dst Register, imm int32) {
	a.rexW(0, 0, dst)
	a.emitByte(0x81)
	a.emitByte(modRM(0b11, 5, byte(dst)))
	a.emitUint32(uint32(imm))
}

// ImulRegReg: imul dst, src (64-bit signed multiply)
func (a *AssemblerAMD64) ImulRegReg(dst, src Register) {
	a.rexW(dst, 0, src)
	a.emitBytes(0x0F, 0xAF)
	a.emitByte(modRM(0b11, byte(dst), byte(src)))
}

// ImulRegMem: imul dst, [base + disp32]
func (a *AssemblerAMD64) ImulRegMem(dst, base Register, disp int32) {
	a.rexW(dst, 0, base)
	a.emitBytes(0x0F, 0xAF)
	a.emitMemOperand(dst, base, disp)
}

// IncMem: inc qword ptr [base + disp32]
func (a *AssemblerAMD64) IncMem(base Register, disp int32) {
	a.rexW(0, 0, base)
	a.emitByte(0xFF)
	a.emitMemOperand(0, base, disp)
}

// DecMem: dec qword ptr [base + disp32]
func (a *AssemblerAMD64) DecMem(base Register, disp int32) {
	a.rexW(0, 0, base)
	a.emitByte(0xFF)
	a.emitMemOperand(1, base, disp)
}

// IncReg: inc reg (64-bit)
func (a *AssemblerAMD64) IncReg(r Register) {
	a.rexW(0, 0, r)
	a.emitByte(0xFF)
	a.emitByte(modRM(0b11, 0, byte(r)))
}

// DecReg: dec reg (64-bit)
func (a *AssemblerAMD64) DecReg(r Register) {
	a.rexW(0, 0, r)
	a.emitByte(0xFF)
	a.emitByte(modRM(0b11, 1, byte(r)))
}

// NegReg: neg reg (64-bit)
func (a *AssemblerAMD64) NegReg(r Register) {
	a.rexW(0, 0, r)
	a.emitByte(0xF7)
	a.emitByte(modRM(0b11, 3, byte(r)))
}

func (a *AssemblerAMD64) ShlRegImm8(r Register, imm byte) {
	a.rexW(0, 0, r)
	a.emitByte(0xC1)
	a.emitByte(modRM(0b11, 4, byte(r)))
	a.emitByte(imm)
}

func (a *AssemblerAMD64) ShrRegImm8(r Register, imm byte) {
	a.rexW(0, 0, r)
	a.emitByte(0xC1)
	a.emitByte(modRM(0b11, 5, byte(r)))
	a.emitByte(imm)
}

// NotReg: not reg (64-bit bitwise inversion)
func (a *AssemblerAMD64) NotReg(r Register) {
	a.rexW(0, 0, r)
	a.emitByte(0xF7)
	a.emitByte(modRM(0b11, 2, byte(r)))
}

// XorRegReg: xor dst, src (e.g. xor rax, rax to zero register)
func (a *AssemblerAMD64) XorRegReg(dst, src Register) {
	a.rexW(src, 0, dst)
	a.emitByte(0x31)
	a.emitByte(modRM(0b11, byte(src), byte(dst)))
}

// AndRegReg: and dst, src
func (a *AssemblerAMD64) AndRegReg(dst, src Register) {
	a.rexW(src, 0, dst)
	a.emitByte(0x21)
	a.emitByte(modRM(0b11, byte(src), byte(dst)))
}

// OrRegReg: or dst, src
func (a *AssemblerAMD64) OrRegReg(dst, src Register) {
	a.rexW(src, 0, dst)
	a.emitByte(0x09)
	a.emitByte(modRM(0b11, byte(src), byte(dst)))
}

// CmpRegReg: cmp r1, r2
func (a *AssemblerAMD64) CmpRegReg(r1, r2 Register) {
	a.rexW(r2, 0, r1)
	a.emitByte(0x39)
	a.emitByte(modRM(0b11, byte(r2), byte(r1)))
}

// CmpRegMem: cmp r1, [base + disp32]
func (a *AssemblerAMD64) CmpRegMem(r1, base Register, disp int32) {
	a.rexW(r1, 0, base)
	a.emitByte(0x3B)
	a.emitMemOperand(r1, base, disp)
}

// CmpMemReg: cmp [base + disp32], r2
func (a *AssemblerAMD64) CmpMemReg(base Register, disp int32, r2 Register) {
	a.rexW(r2, 0, base)
	a.emitByte(0x39)
	a.emitMemOperand(r2, base, disp)
}

// CmpRegImm32: cmp reg, imm32
func (a *AssemblerAMD64) CmpRegImm32(r Register, imm int32) {
	a.rexW(0, 0, r)
	a.emitByte(0x81)
	a.emitByte(modRM(0b11, 7, byte(r)))
	a.emitUint32(uint32(imm))
}

// CmpMemImm8: cmp byte ptr [base + disp32], imm8
func (a *AssemblerAMD64) CmpMemImm8(base Register, disp int32, imm byte) {
	a.rex(0, 0, base)
	a.emitByte(0x80)
	a.emitMemOperand(7, base, disp)
	a.emitByte(imm)
}

// Cqo: sign-extend RAX into RDX:RAX (64-bit)
func (a *AssemblerAMD64) Cqo() {
	a.emitBytes(0x48, 0x99)
}

// IdivReg: signed divide RDX:RAX by r (64-bit)
func (a *AssemblerAMD64) IdivReg(r Register) {
	a.rexW(0, 0, r)
	a.emitByte(0xF7)
	a.emitByte(modRM(0b11, 7, byte(r)))
}

// TestRegReg: test r1, r2
func (a *AssemblerAMD64) TestRegReg(r1, r2 Register) {
	a.rexW(r2, 0, r1)
	a.emitByte(0x85)
	a.emitByte(modRM(0b11, byte(r2), byte(r1)))
}

// JmpReg: indirect jump to address stored in 64-bit register (jmp reg)
func (a *AssemblerAMD64) JmpReg(r Register) {
	if r >= 8 {
		a.emitByte(0x41)
	}
	a.emitByte(0xFF)
	a.emitByte(modRM(0b11, 4, byte(r)))
}

// Jmp: unconditional jump to label (rel32)
func (a *AssemblerAMD64) Jmp(target Label) {
	a.emitByte(0xE9)
	patch := len(a.code)
	a.emitUint32(0) // placeholder
	a.fixups = append(a.fixups, jumpFixup{
		label:       target,
		patchOffset: patch,
		insEnd:      patch + 4,
	})
}

// Jcc: conditional jump to label (rel32)
func (a *AssemblerAMD64) Jcc(cond Condition, target Label) {
	a.emitBytes(0x0F, byte(cond))
	patch := len(a.code)
	a.emitUint32(0) // placeholder
	a.fixups = append(a.fixups, jumpFixup{
		label:       target,
		patchOffset: patch,
		insEnd:      patch + 4,
	})
}

// ImulRegImm32: imul dst, dst, imm32 (64-bit signed multiply by immediate)
func (a *AssemblerAMD64) ImulRegImm32(dst Register, imm int32) {
	a.rexW(dst, 0, dst)
	a.emitByte(0x69)
	a.emitByte(modRM(0b11, byte(dst), byte(dst)))
	a.emitUint32(uint32(imm))
}

// ImulRegRegImm32: imul dst, src, imm32
func (a *AssemblerAMD64) ImulRegRegImm32(dst, src Register, imm int32) {
	a.rexW(dst, 0, src)
	a.emitByte(0x69)
	a.emitByte(modRM(0b11, byte(dst), byte(src)))
	a.emitUint32(uint32(imm))
}

// Setcc: set byte on condition (e.g. sete, setne, setl, etc.)
func (a *AssemblerAMD64) Setcc(cond Condition, dst Register) {
	a.rex(0, 0, dst)
	a.emitBytes(0x0F, 0x90+(byte(cond)&0x0F))
	a.emitByte(modRM(0b11, 0, byte(dst)))
}

// MovzxByte: movzx r64, r8 (zero-extend 8-bit register to 64-bit)
func (a *AssemblerAMD64) MovzxByte(dst, src Register) {
	a.rexW(dst, 0, src)
	a.emitBytes(0x0F, 0xB6)
	a.emitByte(modRM(0b11, byte(dst), byte(src)))
}

// XMMRegister represents an SSE/AVX 128-bit register (XMM0-XMM7)
type XMMRegister byte

const (
	XMM0 XMMRegister = 0
	XMM1 XMMRegister = 1
	XMM2 XMMRegister = 2
)

// MovsdRegMem: movsd xmm, [base + disp32] (load 64-bit float)
func (a *AssemblerAMD64) MovsdRegMem(dst XMMRegister, base Register, disp int32) {
	a.emitByte(0xF2)
	a.rex(Register(dst), 0, base)
	a.emitBytes(0x0F, 0x10)
	a.emitMemOperand(Register(dst), base, disp)
}

// MovsdMemReg: movsd [base + disp32], xmm (store 64-bit float)
func (a *AssemblerAMD64) MovsdMemReg(base Register, disp int32, src XMMRegister) {
	a.emitByte(0xF2)
	a.rex(Register(src), 0, base)
	a.emitBytes(0x0F, 0x11)
	a.emitMemOperand(Register(src), base, disp)
}

// AddsdRegMem: addsd xmm, [base + disp32]
func (a *AssemblerAMD64) AddsdRegMem(dst XMMRegister, base Register, disp int32) {
	a.emitByte(0xF2)
	a.rex(Register(dst), 0, base)
	a.emitBytes(0x0F, 0x58)
	a.emitMemOperand(Register(dst), base, disp)
}

// SubsdRegMem: subsd xmm, [base + disp32]
func (a *AssemblerAMD64) SubsdRegMem(dst XMMRegister, base Register, disp int32) {
	a.emitByte(0xF2)
	a.rex(Register(dst), 0, base)
	a.emitBytes(0x0F, 0x5C)
	a.emitMemOperand(Register(dst), base, disp)
}

// MulsdRegMem: mulsd xmm, [base + disp32]
func (a *AssemblerAMD64) MulsdRegMem(dst XMMRegister, base Register, disp int32) {
	a.emitByte(0xF2)
	a.rex(Register(dst), 0, base)
	a.emitBytes(0x0F, 0x59)
	a.emitMemOperand(Register(dst), base, disp)
}

// DivsdRegMem: divsd xmm, [base + disp32]
func (a *AssemblerAMD64) DivsdRegMem(dst XMMRegister, base Register, disp int32) {
	a.emitByte(0xF2)
	a.rex(Register(dst), 0, base)
	a.emitBytes(0x0F, 0x5E)
	a.emitMemOperand(Register(dst), base, disp)
}

// UcomisdRegMem: ucomisd xmm, [base + disp32] (compare 64-bit floats)
func (a *AssemblerAMD64) UcomisdRegMem(dst XMMRegister, base Register, disp int32) {
	a.emitByte(0x66)
	a.rex(Register(dst), 0, base)
	a.emitBytes(0x0F, 0x2E)
	a.emitMemOperand(Register(dst), base, disp)
}

func (a *AssemblerAMD64) SqrtsdRegReg(dst, src XMMRegister) {
	a.emitByte(0xF2)
	a.rex(0, Register(dst), Register(src))
	a.emitBytes(0x0F, 0x51)
	a.emitByte(modRM(0b11, byte(dst), byte(src)))
}

func (a *AssemblerAMD64) MovsdRegReg(dst, src XMMRegister) {
	a.emitByte(0xF2)
	a.rex(0, Register(dst), Register(src))
	a.emitBytes(0x0F, 0x10)
	a.emitByte(modRM(0b11, byte(dst), byte(src)))
}

func (a *AssemblerAMD64) AddsdRegReg(dst, src XMMRegister) {
	a.emitByte(0xF2)
	a.rex(0, Register(dst), Register(src))
	a.emitBytes(0x0F, 0x58)
	a.emitByte(modRM(0b11, byte(dst), byte(src)))
}

func (a *AssemblerAMD64) SubsdRegReg(dst, src XMMRegister) {
	a.emitByte(0xF2)
	a.rex(0, Register(dst), Register(src))
	a.emitBytes(0x0F, 0x5C)
	a.emitByte(modRM(0b11, byte(dst), byte(src)))
}

func (a *AssemblerAMD64) UcomisdRegReg(left, right XMMRegister) {
	a.emitByte(0x66)
	a.rex(0, Register(left), Register(right))
	a.emitBytes(0x0F, 0x2E)
	a.emitByte(modRM(0b11, byte(left), byte(right)))
}

func (a *AssemblerAMD64) Cvtsi2sdReg(dst XMMRegister, src Register) {
	a.emitByte(0xF2)
	a.rexW(Register(dst), 0, src)
	a.emitBytes(0x0F, 0x2A)
	a.emitByte(modRM(0b11, byte(dst), byte(src)))
}

func (a *AssemblerAMD64) Cvttsd2siReg(dst Register, src XMMRegister) {
	a.emitByte(0xF2)
	a.rexW(dst, 0, Register(src))
	a.emitBytes(0x0F, 0x2C)
	a.emitByte(modRM(0b11, byte(dst), byte(src)))
}

func (a *AssemblerAMD64) MovqXMMReg(dst XMMRegister, src Register) {
	a.emitByte(0x66)
	a.rexW(Register(dst), 0, src)
	a.emitBytes(0x0F, 0x6E)
	a.emitByte(modRM(0b11, byte(dst), byte(src)))
}

// Ret emits the x86 ret instruction (0xC3).
func (a *AssemblerAMD64) Ret() {
	a.emitByte(0xC3)
}

// Finalize resolves all label addresses and backpatches relative displacements.
func (a *AssemblerAMD64) Finalize() ([]byte, error) {
	for _, f := range a.fixups {
		targetOffset, ok := a.labels[f.label]
		if !ok {
			return nil, fmt.Errorf("unresolved jump label %d", f.label)
		}
		rel32 := int32(targetOffset - f.insEnd)
		binary.LittleEndian.PutUint32(a.code[f.patchOffset:], uint32(rel32))
	}
	return a.code, nil
}
