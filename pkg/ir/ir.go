// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

package ir

import (
	"fmt"
	"strings"
)

// Type represents an IR primitive type.
type Type int

const (
	TypeVoid Type = iota
	TypeI8
	TypeI64
	TypeF64
	TypePtr
)

func (t Type) String() string {
	switch t {
	case TypeVoid:
		return "void"
	case TypeI8:
		return "i8"
	case TypeI64:
		return "i64"
	case TypeF64:
		return "f64"
	case TypePtr:
		return "ptr"
	default:
		return "unknown"
	}
}

// VReg represents a typed virtual register.
type VReg struct {
	ID   int
	Type Type
}

func (v VReg) String() string {
	return fmt.Sprintf("v%d:%s", v.ID, v.Type)
}

// Op represents the instruction operation.
type Op int

const (
	OpNop Op = iota
	OpConstInt
	OpConstFloat
	OpConstBool
	OpCopy
	// Arithmetic
	OpAdd
	OpSub
	OpMul
	OpDiv
	OpMod
	// Negation
	OpNeg
	// Bitwise / Logical
	OpNot
	// Comparisons
	OpEq
	OpNe
	OpLt
	OpLe
	OpGt
	OpGe
	// Control flow
	OpJmp
	OpBranch
	OpCall
	OpRet
)

func (o Op) String() string {
	switch o {
	case OpNop:
		return "nop"
	case OpConstInt:
		return "const_int"
	case OpConstFloat:
		return "const_float"
	case OpConstBool:
		return "const_bool"
	case OpCopy:
		return "copy"
	case OpAdd:
		return "add"
	case OpSub:
		return "sub"
	case OpMul:
		return "mul"
	case OpDiv:
		return "div"
	case OpMod:
		return "mod"
	case OpNeg:
		return "neg"
	case OpNot:
		return "not"
	case OpEq:
		return "eq"
	case OpNe:
		return "ne"
	case OpLt:
		return "lt"
	case OpLe:
		return "le"
	case OpGt:
		return "gt"
	case OpGe:
		return "ge"
	case OpJmp:
		return "jmp"
	case OpBranch:
		return "branch"
	case OpCall:
		return "call"
	case OpRet:
		return "ret"
	default:
		return "unknown_op"
	}
}

// Instruction is a 3-address IR statement.
type Instruction struct {
	Op       Op
	Dst      VReg
	Src1     VReg
	Src2     VReg
	IntVal   int64
	FloatVal float64
	Target   *Block
	ElseTgt  *Block
	CallName string
	Args     []VReg
}

func (inst *Instruction) String() string {
	switch inst.Op {
	case OpConstInt:
		return fmt.Sprintf("  %s = const_int %d", inst.Dst, inst.IntVal)
	case OpConstFloat:
		return fmt.Sprintf("  %s = const_float %f", inst.Dst, inst.FloatVal)
	case OpConstBool:
		b := 0
		if inst.IntVal != 0 {
			b = 1
		}
		return fmt.Sprintf("  %s = const_bool %d", inst.Dst, b)
	case OpCopy:
		return fmt.Sprintf("  %s = copy %s", inst.Dst, inst.Src1)
	case OpAdd, OpSub, OpMul, OpDiv, OpMod, OpEq, OpNe, OpLt, OpLe, OpGt, OpGe:
		return fmt.Sprintf("  %s = %s %s, %s", inst.Dst, inst.Op, inst.Src1, inst.Src2)
	case OpNeg, OpNot:
		return fmt.Sprintf("  %s = %s %s", inst.Dst, inst.Op, inst.Src1)
	case OpJmp:
		return fmt.Sprintf("  jmp %s", inst.Target.Name)
	case OpBranch:
		return fmt.Sprintf("  branch %s, then %s, else %s", inst.Src1, inst.Target.Name, inst.ElseTgt.Name)
	case OpCall:
		var args []string
		for _, a := range inst.Args {
			args = append(args, a.String())
		}
		if inst.Dst.Type != TypeVoid {
			return fmt.Sprintf("  %s = call @%s(%s)", inst.Dst, inst.CallName, strings.Join(args, ", "))
		}
		return fmt.Sprintf("  call @%s(%s)", inst.CallName, strings.Join(args, ", "))
	case OpRet:
		if inst.Src1.Type != TypeVoid {
			return fmt.Sprintf("  ret %s", inst.Src1)
		}
		return "  ret"
	default:
		return "  nop"
	}
}

// Block represents a basic block with sequential instructions ending in a terminator.
type Block struct {
	Name         string
	Instructions []*Instruction
}

// Function represents an IR function.
type Function struct {
	Name       string
	Params     []VReg
	ReturnType Type
	Blocks     []*Block
	NextVRegID int
}

func (f *Function) NewVReg(t Type) VReg {
	v := VReg{ID: f.NextVRegID, Type: t}
	f.NextVRegID++
	return v
}

func (f *Function) NewBlock(name string) *Block {
	b := &Block{Name: fmt.Sprintf("%s_%d", name, len(f.Blocks))}
	f.Blocks = append(f.Blocks, b)
	return b
}

// Program is a whole IR translation unit.
type Program struct {
	Functions []*Function
	EntryFn   *Function
}
