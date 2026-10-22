package jit

import (
	"encoding/binary"

	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/vm"
)

// ThreadedContext holds the execution context for threaded JIT execution.
type ThreadedContext struct {
	Stack       []vm.StackSlot
	Globals     []vm.StackSlot
	Pool        *vm.ConstantPool
	BasePointer int
	SP          int
	BailoutIP   int
	Status      int64
	Err         *interpreter.ErrorValue
	Env         *interpreter.Environment
	SourceFile  string
	HelperCalls int64
	Callbacks   *vm.JITCallbacks
	// Fn is the function instance that is running. The compiled code is
	// shared between all instances of a closure, so the captured values have
	// to be read from the instance and not from the compiled template.
	Fn *vm.CompiledFunction
}

// upvalueOwner returns the function whose captured values belong to the
// running instance.
func (c *ThreadedContext) upvalueOwner(fallback *vm.CompiledFunction) *vm.CompiledFunction {
	if c.Fn != nil {
		return c.Fn
	}
	return fallback
}

// ThreadedInst is a pre-decoded, optimized instruction representation.
type ThreadedInst struct {
	Op     vm.Opcode
	Arg0   int64
	Arg1   int64
	Arg2   int64
	Target int
	Exec   func(ctx *ThreadedContext, inst *ThreadedInst) int // returns next IP
}

// ThreadedFunction is a compiled threaded JIT function.
type ThreadedFunction struct {
	Insts []ThreadedInst
}

// CompileThreaded compiles a bytecode function into a pre-decoded threaded JIT function.
func CompileThreaded(fn *vm.CompiledFunction) *ThreadedFunction {
	ins := fn.Instructions
	insts := make([]ThreadedInst, len(ins))

	ip := 0
	for ip < len(ins) {
		currentIP := ip
		op := vm.Opcode(ins[ip])
		ip++

		inst := ThreadedInst{Op: op}

		switch op {
		case vm.OpConstant:
			inst.Arg0 = int64(binary.BigEndian.Uint16(ins[ip:]))
			ip += 2
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				if ctx.Pool == nil || i.Arg0 < 0 || int(i.Arg0) >= len(ctx.Pool.Slots) ||
					ctx.SP < 0 || ctx.SP >= len(ctx.Stack) {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				ctx.Stack[ctx.SP] = ctx.Pool.Slots[i.Arg0]
				ctx.SP++
				return currentIP + 3
			}

		case vm.OpPop:
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				if ctx.SP > 0 {
					ctx.SP--
				}
				return currentIP + 1
			}

		case vm.OpDup:
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				if ctx.SP > 0 {
					ctx.Stack[ctx.SP] = ctx.Stack[ctx.SP-1]
					ctx.SP++
				}
				return currentIP + 1
			}

		case vm.OpTrue:
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				ctx.Stack[ctx.SP] = vm.FromBool(true)
				ctx.SP++
				return currentIP + 1
			}

		case vm.OpFalse:
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				ctx.Stack[ctx.SP] = vm.FromBool(false)
				ctx.SP++
				return currentIP + 1
			}

		case vm.OpNull:
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				ctx.Stack[ctx.SP] = vm.NullSlot()
				ctx.SP++
				return currentIP + 1
			}

		case vm.OpGetLocal:
			inst.Arg0 = int64(ins[ip])
			ip++
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				ctx.Stack[ctx.SP] = ctx.Stack[ctx.BasePointer+int(i.Arg0)]
				ctx.SP++
				return currentIP + 2
			}

		case vm.OpSetLocal:
			inst.Arg0 = int64(ins[ip])
			ip++
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				ctx.SP--
				ctx.Stack[ctx.BasePointer+int(i.Arg0)] = ctx.Stack[ctx.SP]
				return currentIP + 2
			}

		case vm.OpIncLocal:
			inst.Arg0 = int64(ins[ip])
			ip++
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				slot := &ctx.Stack[ctx.BasePointer+int(i.Arg0)]
				if !slot.IsInt() {
					// The VM adds one to every other type through the slow
					// arithmetic path, so those cases deoptimize.
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				slot.SetInt(slot.Int() + 1)
				return currentIP + 2
			}

		case vm.OpAddLocal:
			inst.Arg0 = int64(ins[ip])
			ip++
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				if ctx.SP < 1 {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				top := ctx.Stack[ctx.SP-1]
				slot := &ctx.Stack[ctx.BasePointer+int(i.Arg0)]
				if slot.IsInt() && top.IsInt() {
					ctx.SP--
					slot.SetInt(slot.Int() + top.Int())
					return currentIP + 2
				}
				if slot.IsFloat() && top.IsFloat() {
					ctx.SP--
					slot.SetFloat(slot.Float() + top.Float())
					return currentIP + 2
				}
				// The VM pops the operand and pushes the result for every
				// other type, so the stack pointer must stay untouched when
				// the instruction deoptimizes.
				ctx.Status = StatusBailout
				ctx.BailoutIP = currentIP
				return -1
			}

		case vm.OpSubLocal:
			inst.Arg0 = int64(ins[ip])
			ip++
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				if ctx.SP < 1 {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				top := ctx.Stack[ctx.SP-1]
				slot := &ctx.Stack[ctx.BasePointer+int(i.Arg0)]
				if slot.IsInt() && top.IsInt() {
					ctx.SP--
					slot.SetInt(slot.Int() - top.Int())
					return currentIP + 2
				}
				if slot.IsFloat() && top.IsFloat() {
					ctx.SP--
					slot.SetFloat(slot.Float() - top.Float())
					return currentIP + 2
				}
				ctx.Status = StatusBailout
				ctx.BailoutIP = currentIP
				return -1
			}

		case vm.OpAddLocalLocal:
			inst.Arg0 = int64(ins[ip])
			inst.Arg1 = int64(ins[ip+1])
			ip += 2
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				src := ctx.Stack[ctx.BasePointer+int(i.Arg1)].Int()
				dst := &ctx.Stack[ctx.BasePointer+int(i.Arg0)]
				dst.SetInt(dst.Int() + src)
				return currentIP + 3
			}

		case vm.OpAddLocalConst:
			inst.Arg0 = int64(ins[ip])
			inst.Arg1 = int64(binary.BigEndian.Uint16(ins[ip+1:]))
			ip += 3
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				src := ctx.Stack[ctx.BasePointer+int(i.Arg0)].Int()
				c := ctx.Pool.Slots[i.Arg1].Int()
				ctx.Stack[ctx.SP] = vm.FromInt(src + c)
				ctx.SP++
				return currentIP + 4
			}

		case vm.OpMulLocalConst:
			inst.Arg0 = int64(ins[ip])
			inst.Arg1 = int64(binary.BigEndian.Uint16(ins[ip+1:]))
			ip += 3
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				src := ctx.Stack[ctx.BasePointer+int(i.Arg0)].Int()
				c := ctx.Pool.Slots[i.Arg1].Int()
				ctx.Stack[ctx.SP] = vm.FromInt(src * c)
				ctx.SP++
				return currentIP + 4
			}

		case vm.OpGetGlobal:
			inst.Arg0 = int64(binary.BigEndian.Uint16(ins[ip:]))
			ip += 2
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				if i.Arg0 < 0 || int(i.Arg0) >= len(ctx.Globals) ||
					ctx.SP < 0 || ctx.SP >= len(ctx.Stack) {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				ctx.Stack[ctx.SP] = ctx.Globals[i.Arg0]
				ctx.SP++
				return currentIP + 3
			}

		case vm.OpSetGlobal:
			inst.Arg0 = int64(binary.BigEndian.Uint16(ins[ip:]))
			ip += 2
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				if i.Arg0 < 0 || int(i.Arg0) >= len(ctx.Globals) || ctx.SP < 1 {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				ctx.SP--
				ctx.Globals[i.Arg0] = ctx.Stack[ctx.SP]
				return currentIP + 3
			}

		case vm.OpIncGlobal:
			inst.Arg0 = int64(binary.BigEndian.Uint16(ins[ip:]))
			ip += 2
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				if i.Arg0 < 0 || int(i.Arg0) >= len(ctx.Globals) {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				slot := &ctx.Globals[i.Arg0]
				if !slot.IsInt() {
					// The VM adds one to every other type through the slow
					// arithmetic path, so those cases deoptimize.
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				slot.SetInt(slot.Int() + 1)
				return currentIP + 3
			}

		case vm.OpAddGlobal:
			inst.Arg0 = int64(binary.BigEndian.Uint16(ins[ip:]))
			ip += 2
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				if ctx.SP < 1 {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				val := ctx.Stack[ctx.SP-1]
				slot := &ctx.Globals[i.Arg0]
				if slot.IsInt() && val.IsInt() {
					ctx.SP--
					slot.SetInt(slot.Int() + val.Int())
					return currentIP + 3
				}
				if slot.IsFloat() && val.IsFloat() {
					ctx.SP--
					slot.SetFloat(slot.Float() + val.Float())
					return currentIP + 3
				}
				// The VM pops the operand and pushes the result for every
				// other type, so the stack pointer must stay untouched when
				// the instruction deoptimizes.
				ctx.Status = StatusBailout
				ctx.BailoutIP = currentIP
				return -1
			}

		case vm.OpSubGlobal:
			inst.Arg0 = int64(binary.BigEndian.Uint16(ins[ip:]))
			ip += 2
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				if ctx.SP < 1 {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				val := ctx.Stack[ctx.SP-1]
				slot := &ctx.Globals[i.Arg0]
				if slot.IsInt() && val.IsInt() {
					ctx.SP--
					slot.SetInt(slot.Int() - val.Int())
					return currentIP + 3
				}
				if slot.IsFloat() && val.IsFloat() {
					ctx.SP--
					slot.SetFloat(slot.Float() - val.Float())
					return currentIP + 3
				}
				ctx.Status = StatusBailout
				ctx.BailoutIP = currentIP
				return -1
			}

		case vm.OpAddGlobalGlobal:
			inst.Arg0 = int64(binary.BigEndian.Uint16(ins[ip:]))
			inst.Arg1 = int64(binary.BigEndian.Uint16(ins[ip+2:]))
			ip += 4
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				src := ctx.Globals[i.Arg1]
				dst := &ctx.Globals[i.Arg0]
				if dst.IsInt() && src.IsInt() {
					dst.SetInt(dst.Int() + src.Int())
					return currentIP + 5
				}
				if dst.IsFloat() && src.IsFloat() {
					dst.SetFloat(dst.Float() + src.Float())
					return currentIP + 5
				}
				ctx.Status = StatusBailout
				ctx.BailoutIP = currentIP
				return -1
			}

		case vm.OpAddConst:
			inst.Arg0 = int64(binary.BigEndian.Uint16(ins[ip:]))
			ip += 2
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				top := &ctx.Stack[ctx.SP-1]
				k := ctx.Pool.Slots[i.Arg0]
				if top.IsInt() && k.IsInt() {
					top.SetInt(top.Int() + k.Int())
					return currentIP + 3
				}
				if top.IsFloat() && k.IsFloat() {
					top.SetFloat(top.Float() + k.Float())
					return currentIP + 3
				}
				ctx.Status = StatusBailout
				ctx.BailoutIP = currentIP
				return -1
			}

		case vm.OpMulConst:
			inst.Arg0 = int64(binary.BigEndian.Uint16(ins[ip:]))
			ip += 2
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				top := &ctx.Stack[ctx.SP-1]
				k := ctx.Pool.Slots[i.Arg0]
				if top.IsInt() && k.IsInt() {
					top.SetInt(top.Int() * k.Int())
					return currentIP + 3
				}
				if top.IsFloat() && k.IsFloat() {
					top.SetFloat(top.Float() * k.Float())
					return currentIP + 3
				}
				ctx.Status = StatusBailout
				ctx.BailoutIP = currentIP
				return -1
			}

		case vm.OpModConst:
			inst.Arg0 = int64(binary.BigEndian.Uint16(ins[ip:]))
			ip += 2
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				top := &ctx.Stack[ctx.SP-1]
				k := ctx.Pool.Slots[i.Arg0]
				if top.IsInt() && k.IsInt() && k.Int() != 0 {
					top.SetInt(top.Int() % k.Int())
					return currentIP + 3
				}
				ctx.Status = StatusBailout
				ctx.BailoutIP = currentIP
				return -1
			}

		case vm.OpModLocalConst:
			inst.Arg0 = int64(ins[ip])
			inst.Arg1 = int64(binary.BigEndian.Uint16(ins[ip+1:]))
			ip += 3
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				src := ctx.Stack[ctx.BasePointer+int(i.Arg0)]
				c := ctx.Pool.Slots[i.Arg1]
				if src.IsInt() && c.IsInt() && c.Int() != 0 {
					ctx.Stack[ctx.SP] = vm.FromInt(src.Int() % c.Int())
					ctx.SP++
					return currentIP + 4
				}
				ctx.Status = StatusBailout
				ctx.BailoutIP = currentIP
				return -1
			}

		case vm.OpAdd:
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				r := &ctx.Stack[ctx.SP-1]
				l := &ctx.Stack[ctx.SP-2]
				if l.IsInt() && r.IsInt() {
					l.SetInt(l.Int() + r.Int())
					ctx.SP--
					return currentIP + 1
				}
				if l.IsFloat() && r.IsFloat() {
					l.SetFloat(l.Float() + r.Float())
					ctx.SP--
					return currentIP + 1
				}
				// Deoptimize
				ctx.Status = StatusBailout
				ctx.BailoutIP = currentIP
				return -1
			}

		case vm.OpSub:
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				r := &ctx.Stack[ctx.SP-1]
				l := &ctx.Stack[ctx.SP-2]
				if l.IsInt() && r.IsInt() {
					l.SetInt(l.Int() - r.Int())
					ctx.SP--
					return currentIP + 1
				}
				if l.IsFloat() && r.IsFloat() {
					l.SetFloat(l.Float() - r.Float())
					ctx.SP--
					return currentIP + 1
				}
				ctx.Status = StatusBailout
				ctx.BailoutIP = currentIP
				return -1
			}

		case vm.OpMul:
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				r := &ctx.Stack[ctx.SP-1]
				l := &ctx.Stack[ctx.SP-2]
				if l.IsInt() && r.IsInt() {
					l.SetInt(l.Int() * r.Int())
					ctx.SP--
					return currentIP + 1
				}
				if l.IsFloat() && r.IsFloat() {
					l.SetFloat(l.Float() * r.Float())
					ctx.SP--
					return currentIP + 1
				}
				ctx.Status = StatusBailout
				ctx.BailoutIP = currentIP
				return -1
			}

		case vm.OpDiv:
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				r := &ctx.Stack[ctx.SP-1]
				l := &ctx.Stack[ctx.SP-2]
				if l.IsInt() && r.IsInt() {
					if r.Int() == 0 {
						ctx.Status = StatusBailout
						ctx.BailoutIP = currentIP
						return -1
					}
					l.SetInt(l.Int() / r.Int())
					ctx.SP--
					return currentIP + 1
				}
				if l.IsFloat() && r.IsFloat() {
					if r.Float() == 0 {
						ctx.Status = StatusBailout
						ctx.BailoutIP = currentIP
						return -1
					}
					l.SetFloat(l.Float() / r.Float())
					ctx.SP--
					return currentIP + 1
				}
				ctx.Status = StatusBailout
				ctx.BailoutIP = currentIP
				return -1
			}

		case vm.OpMod:
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				r := &ctx.Stack[ctx.SP-1]
				l := &ctx.Stack[ctx.SP-2]
				if l.IsInt() && r.IsInt() {
					if r.Int() == 0 {
						ctx.Status = StatusBailout
						ctx.BailoutIP = currentIP
						return -1
					}
					l.SetInt(l.Int() % r.Int())
					ctx.SP--
					return currentIP + 1
				}
				ctx.Status = StatusBailout
				ctx.BailoutIP = currentIP
				return -1
			}

		case vm.OpNegate:
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				slot := &ctx.Stack[ctx.SP-1]
				if slot.IsInt() {
					slot.SetInt(-slot.Int())
					return currentIP + 1
				}
				if slot.IsFloat() {
					slot.SetFloat(-slot.Float())
					return currentIP + 1
				}
				ctx.Status = StatusBailout
				ctx.BailoutIP = currentIP
				return -1
			}

		case vm.OpNot:
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				slot := &ctx.Stack[ctx.SP-1]
				slot.SetBool(slot.IsFalsy())
				return currentIP + 1
			}

		case vm.OpEqual:
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				r := &ctx.Stack[ctx.SP-1]
				l := &ctx.Stack[ctx.SP-2]
				eq := false
				if l.IsInt() && r.IsInt() {
					eq = (l.Int() == r.Int())
				} else if l.IsFloat() && r.IsFloat() {
					eq = (l.Float() == r.Float())
				} else if l.IsBool() && r.IsBool() {
					eq = (l.Bool() == r.Bool())
				} else if l.IsNull() && r.IsNull() {
					eq = true
				} else {
					eq = interpreter.LooseEqual(l.ToValue(), r.ToValue())
				}
				l.SetBool(eq)
				ctx.SP--
				return currentIP + 1
			}

		case vm.OpStrictEqual:
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				r := &ctx.Stack[ctx.SP-1]
				l := &ctx.Stack[ctx.SP-2]
				eq := false
				if l.Tag() != r.Tag() {
					eq = false
				} else if l.IsInt() {
					eq = l.Int() == r.Int()
				} else if l.IsFloat() {
					eq = l.Float() == r.Float()
				} else if l.IsBool() {
					eq = l.Bool() == r.Bool()
				} else if l.IsNull() {
					eq = true
				} else {
					eq = interpreter.StrictEqual(l.ToValue(), r.ToValue())
				}
				l.SetBool(eq)
				ctx.SP--
				return currentIP + 1
			}

		case vm.OpNotEqual:
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				r := &ctx.Stack[ctx.SP-1]
				l := &ctx.Stack[ctx.SP-2]
				ne := true
				if l.IsInt() && r.IsInt() {
					ne = (l.Int() != r.Int())
				} else if l.IsFloat() && r.IsFloat() {
					ne = (l.Float() != r.Float())
				} else if l.IsBool() && r.IsBool() {
					ne = (l.Bool() != r.Bool())
				} else if l.IsNull() && r.IsNull() {
					ne = false
				} else {
					ne = !interpreter.LooseEqual(l.ToValue(), r.ToValue())
				}
				l.SetBool(ne)
				ctx.SP--
				return currentIP + 1
			}

		case vm.OpLessThan:
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				r := &ctx.Stack[ctx.SP-1]
				l := &ctx.Stack[ctx.SP-2]
				lt := false
				if l.IsInt() && r.IsInt() {
					lt = (l.Int() < r.Int())
				} else if l.IsFloat() && r.IsFloat() {
					lt = (l.Float() < r.Float())
				} else {
					lt = vm.CompareOrder(vm.OpLessThan, l.ToValue(), r.ToValue())
				}
				l.SetBool(lt)
				ctx.SP--
				return currentIP + 1
			}

		case vm.OpLessEqual:
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				r := &ctx.Stack[ctx.SP-1]
				l := &ctx.Stack[ctx.SP-2]
				le := false
				if l.IsInt() && r.IsInt() {
					le = (l.Int() <= r.Int())
				} else if l.IsFloat() && r.IsFloat() {
					le = (l.Float() <= r.Float())
				} else {
					le = vm.CompareOrder(vm.OpLessEqual, l.ToValue(), r.ToValue())
				}
				l.SetBool(le)
				ctx.SP--
				return currentIP + 1
			}

		case vm.OpGreaterThan:
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				r := &ctx.Stack[ctx.SP-1]
				l := &ctx.Stack[ctx.SP-2]
				gt := false
				if l.IsInt() && r.IsInt() {
					gt = (l.Int() > r.Int())
				} else if l.IsFloat() && r.IsFloat() {
					gt = (l.Float() > r.Float())
				} else {
					gt = vm.CompareOrder(vm.OpGreaterThan, l.ToValue(), r.ToValue())
				}
				l.SetBool(gt)
				ctx.SP--
				return currentIP + 1
			}

		case vm.OpGreaterEqual:
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				r := &ctx.Stack[ctx.SP-1]
				l := &ctx.Stack[ctx.SP-2]
				ge := false
				if l.IsInt() && r.IsInt() {
					ge = (l.Int() >= r.Int())
				} else if l.IsFloat() && r.IsFloat() {
					ge = (l.Float() >= r.Float())
				} else {
					ge = vm.CompareOrder(vm.OpGreaterEqual, l.ToValue(), r.ToValue())
				}
				l.SetBool(ge)
				ctx.SP--
				return currentIP + 1
			}

		case vm.OpJump:
			inst.Target = int(binary.BigEndian.Uint16(ins[ip:]))
			ip += 2
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				return i.Target
			}

		case vm.OpJumpIfFalse:
			inst.Target = int(binary.BigEndian.Uint16(ins[ip:]))
			ip += 2
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				ctx.SP--
				slot := ctx.Stack[ctx.SP]
				if slot.IsFalsy() {
					return i.Target
				}
				return currentIP + 3
			}

		case vm.OpLoopRangeLocal:
			inst.Arg0 = int64(ins[ip])   // varIdx
			inst.Arg1 = int64(ins[ip+1]) // endIdx
			inst.Arg2 = int64(ins[ip+2]) // stepIdx
			inst.Target = int(binary.BigEndian.Uint16(ins[ip+3:]))
			ip += 5
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				vSlot := &ctx.Stack[ctx.BasePointer+int(i.Arg0)]
				eSlot := &ctx.Stack[ctx.BasePointer+int(i.Arg1)]
				sSlot := &ctx.Stack[ctx.BasePointer+int(i.Arg2)]
				step := sSlot.Int()
				vSlot.SetInt(vSlot.Int() + step)
				curVal := vSlot.Int()
				endVal := eSlot.Int()
				if step >= 0 {
					if curVal <= endVal {
						return i.Target
					}
				} else {
					if curVal >= endVal {
						return i.Target
					}
				}
				return currentIP + 6
			}

		case vm.OpLoopRangeGlobal:
			inst.Arg0 = int64(binary.BigEndian.Uint16(ins[ip:]))
			inst.Arg1 = int64(binary.BigEndian.Uint16(ins[ip+2:]))
			inst.Arg2 = int64(binary.BigEndian.Uint16(ins[ip+4:]))
			inst.Target = int(binary.BigEndian.Uint16(ins[ip+6:]))
			ip += 8
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				vSlot := &ctx.Globals[i.Arg0]
				eSlot := &ctx.Globals[i.Arg1]
				sSlot := &ctx.Globals[i.Arg2]
				step := sSlot.Int()
				vSlot.SetInt(vSlot.Int() + step)
				curVal := vSlot.Int()
				endVal := eSlot.Int()
				if step >= 0 {
					if curVal <= endVal {
						return i.Target
					}
				} else {
					if curVal >= endVal {
						return i.Target
					}
				}
				return currentIP + 9
			}

		case vm.OpLoopGlobalLtInc:
			inst.Arg0 = int64(binary.BigEndian.Uint16(ins[ip:]))
			inst.Arg1 = int64(binary.BigEndian.Uint16(ins[ip+2:]))
			inst.Target = int(binary.BigEndian.Uint16(ins[ip+4:]))
			ip += 6
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				slot := &ctx.Globals[i.Arg0]
				slot.SetInt(slot.Int() + 1)
				if slot.Int() < ctx.Pool.Slots[i.Arg1].Int() {
					return i.Target
				}
				return currentIP + 7
			}

		case vm.OpLoopGlobalLeInc:
			inst.Arg0 = int64(binary.BigEndian.Uint16(ins[ip:]))
			inst.Arg1 = int64(binary.BigEndian.Uint16(ins[ip+2:]))
			inst.Target = int(binary.BigEndian.Uint16(ins[ip+4:]))
			ip += 6
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				slot := &ctx.Globals[i.Arg0]
				slot.SetInt(slot.Int() + 1)
				if slot.Int() <= ctx.Globals[i.Arg1].Int() {
					return i.Target
				}
				return currentIP + 7
			}

		case vm.OpLoopLocalLtInc:
			inst.Arg0 = int64(ins[ip])
			inst.Arg1 = int64(binary.BigEndian.Uint16(ins[ip+1:]))
			inst.Target = int(binary.BigEndian.Uint16(ins[ip+3:]))
			ip += 5
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				vSlot := &ctx.Stack[ctx.BasePointer+int(i.Arg0)]
				cSlot := ctx.Pool.Slots[i.Arg1]
				if vSlot.IsInt() && cSlot.IsInt() {
					vSlot.SetInt(vSlot.Int() + 1)
					if vSlot.Int() < cSlot.Int() {
						return i.Target
					}
					return currentIP + 6
				}
				ctx.Status = StatusBailout
				ctx.BailoutIP = currentIP
				return -1
			}

		case vm.OpLoopLocalLeInc:
			inst.Arg0 = int64(ins[ip])
			inst.Arg1 = int64(ins[ip+1])
			inst.Target = int(binary.BigEndian.Uint16(ins[ip+2:]))
			ip += 4
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				vSlot := &ctx.Stack[ctx.BasePointer+int(i.Arg0)]
				eSlot := &ctx.Stack[ctx.BasePointer+int(i.Arg1)]
				if vSlot.IsInt() && eSlot.IsInt() {
					vSlot.SetInt(vSlot.Int() + 1)
					if vSlot.Int() <= eSlot.Int() {
						return i.Target
					}
					return currentIP + 5
				}
				ctx.Status = StatusBailout
				ctx.BailoutIP = currentIP
				return -1
			}

		case vm.OpIsInt:
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				top := &ctx.Stack[ctx.SP-1]
				top.SetBool(top.IsInt())
				return currentIP + 1
			}

		case vm.OpUse:
			inst.Arg0 = int64(binary.BigEndian.Uint16(ins[ip:]))
			ip += 2
			nextIP := ip
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				if ctx.Env == nil || ctx.Pool == nil || i.Arg0 < 0 || int(i.Arg0) >= len(ctx.Pool.Vals) {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				useVal, ok := ctx.Pool.Vals[i.Arg0].(*vm.UseStatementValue)
				if !ok {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				vm.ExecuteUseStatement(ctx.Env, useVal, ctx.SourceFile)
				ctx.HelperCalls++
				return nextIP
			}

		case vm.OpGetBuiltin:
			inst.Arg0 = int64(binary.BigEndian.Uint16(ins[ip:]))
			ip += 2
			nextIP := ip
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				if ctx.Pool == nil || i.Arg0 < 0 || int(i.Arg0) >= len(ctx.Pool.Vals) || ctx.SP >= len(ctx.Stack) {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				name, ok := ctx.Pool.Vals[i.Arg0].(*interpreter.StringValue)
				if !ok {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				ctx.Stack[ctx.SP] = vm.FromValue(vm.ResolveBuiltinValue(ctx.Env, name.Val))
				ctx.SP++
				ctx.HelperCalls++
				return nextIP
			}

		case vm.OpGetField:
			inst.Arg0 = int64(binary.BigEndian.Uint16(ins[ip:]))
			ip += 2
			nextIP := ip
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				if ctx.SP < 1 || ctx.Pool == nil || i.Arg0 < 0 || int(i.Arg0) >= len(ctx.Pool.Vals) {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				field, ok := ctx.Pool.Vals[i.Arg0].(*interpreter.StringValue)
				if !ok {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				res, err := vm.GetFieldValue(ctx.Stack[ctx.SP-1].ToValue(), field.Val)
				if err != nil {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				ctx.Stack[ctx.SP-1] = vm.FromValue(res)
				ctx.HelperCalls++
				return nextIP
			}

		case vm.OpCall:
			inst.Arg0 = int64(ins[ip])
			ip++
			nextIP := ip
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				argc := int(i.Arg0)
				if argc < 0 || ctx.SP < argc+1 {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				callee := ctx.Stack[ctx.SP-1-argc].ToValue()
				res, handled, errVal := invokeCallable(ctx.Callbacks, callee, ctx.Stack, ctx.SP, argc)
				if !handled {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				ctx.HelperCalls++
				ctx.SP -= argc + 1
				if errVal != nil {
					ctx.Status = StatusError
					ctx.Err = errVal
					ctx.BailoutIP = nextIP
					return -1
				}
				ctx.Stack[ctx.SP] = vm.FromValue(res)
				ctx.SP++
				return nextIP
			}

		case vm.OpClosure:
			inst.Arg0 = int64(binary.BigEndian.Uint16(ins[ip:]))
			ip += 2
			nextIP := ip
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				if ctx.SP < 0 || ctx.SP >= len(ctx.Stack) || ctx.Pool == nil || i.Arg0 < 0 || int(i.Arg0) >= len(ctx.Pool.Vals) ||
					ctx.Callbacks == nil || ctx.Callbacks.MakeClosure == nil {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				template, ok := ctx.Pool.Vals[i.Arg0].(*vm.CompiledFunction)
				if !ok {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				closure := ctx.Callbacks.MakeClosure(template)
				if closure == nil {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				ctx.HelperCalls++
				ctx.Stack[ctx.SP] = vm.FromValue(closure)
				ctx.SP++
				return nextIP
			}

		case vm.OpGetFree:
			inst.Arg0 = int64(ins[ip])
			ip++
			nextIP := ip
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				if ctx.SP < 0 || ctx.SP >= len(ctx.Stack) {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				slot, ok := vm.UpvalueValue(ctx.upvalueOwner(fn), int(i.Arg0), ctx.Stack)
				if !ok {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				ctx.HelperCalls++
				ctx.Stack[ctx.SP] = slot
				ctx.SP++
				return nextIP
			}

		case vm.OpSetFree:
			inst.Arg0 = int64(ins[ip])
			ip++
			nextIP := ip
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				if ctx.SP < 1 {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				if !vm.SetUpvalueValue(ctx.upvalueOwner(fn), int(i.Arg0), ctx.Stack[ctx.SP-1], ctx.Stack) {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				ctx.HelperCalls++
				ctx.SP--
				return nextIP
			}

		case vm.OpMathBuiltin:
			id := int8(ins[ip])
			ip++
			inst.Arg0 = int64(id)
			nextIP := ip
			arity := vm.MathBuiltinArity(id)
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				if arity == 0 || ctx.SP < arity {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				args := make([]interpreter.Value, arity)
				for j := range args {
					args[j] = ctx.Stack[ctx.SP-arity+j].ToValue()
				}
				result, errVal := vm.ApplyMathBuiltin(int8(i.Arg0), args)
				ctx.SP -= arity
				if errVal != nil {
					ctx.Status = StatusError
					ctx.Err = errVal
					ctx.BailoutIP = nextIP
					return -1
				}
				ctx.HelperCalls++
				ctx.Stack[ctx.SP] = vm.FromValue(result)
				ctx.SP++
				return nextIP
			}

		case vm.OpStdlibCall:
			inst.Arg0 = int64(binary.BigEndian.Uint16(ins[ip:]))
			inst.Arg1 = int64(binary.BigEndian.Uint16(ins[ip+2:]))
			inst.Arg2 = int64(ins[ip+4])
			ip += 5
			nextIP := ip
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				argc := int(i.Arg2)
				if ctx.Pool == nil || i.Arg0 < 0 || int(i.Arg0) >= len(ctx.Pool.Vals) ||
					i.Arg1 < 0 || int(i.Arg1) >= len(ctx.Pool.Vals) ||
					argc < 0 || ctx.SP < argc {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				aliasVal, aliasOK := ctx.Pool.Vals[i.Arg0].(*interpreter.StringValue)
				memberVal, memberOK := ctx.Pool.Vals[i.Arg1].(*interpreter.StringValue)
				if !aliasOK || !memberOK {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				alias := aliasVal.Val
				member := memberVal.Val
				res, ok, errVal := invokeStdlibMember(ctx.Env, alias, member, ctx.Stack[ctx.SP-argc:ctx.SP])
				if !ok {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				ctx.HelperCalls++
				ctx.SP -= argc
				if errVal != nil {
					ctx.Status = StatusError
					ctx.Err = errVal
					ctx.BailoutIP = nextIP
					return -1
				}
				ctx.Stack[ctx.SP] = vm.FromValue(res)
				ctx.SP++
				return nextIP
			}

		case vm.OpApplyDecorator:
			inst.Arg0 = int64(binary.BigEndian.Uint16(ins[ip:]))
			ip += 2
			nextIP := ip
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				if ctx.Pool == nil || i.Arg0 < 0 || int(i.Arg0) >= len(ctx.Pool.Vals) {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				spec, ok := ctx.Pool.Vals[i.Arg0].(*vm.DecoratorSpec)
				if !ok || ctx.Callbacks == nil || ctx.Callbacks.ApplyDecorator == nil {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				ctx.Callbacks.ApplyDecorator(spec)
				ctx.HelperCalls++
				return nextIP
			}

		case vm.OpMakeClass:
			inst.Arg0 = int64(binary.BigEndian.Uint16(ins[ip:]))
			inst.Arg1 = int64(binary.BigEndian.Uint16(ins[ip+2:]))
			ip += 4
			nextIP := ip
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				if ctx.Pool == nil || i.Arg0 < 0 || int(i.Arg0) >= len(ctx.Pool.Vals) {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				template, ok := ctx.Pool.Vals[i.Arg0].(*vm.ClassTemplate)
				if !ok {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				var parent *interpreter.StructDef
				if i.Arg1 > 0 {
					parentValue, isDef := ctx.Globals[i.Arg1-1].ToValue().(*interpreter.StructDef)
					if !isDef || !parentValue.IsClass {
						ctx.Status = StatusBailout
						ctx.BailoutIP = currentIP
						return -1
					}
					parent = parentValue
				}
				for _, method := range template.Methods {
					if method != nil && method.Fn != nil {
						method.Fn.Pool = ctx.Pool
					}
				}
				ctx.HelperCalls++
				ctx.Stack[ctx.SP] = vm.FromValue(vm.BuildClassDef(template, parent))
				ctx.SP++
				return nextIP
			}

		case vm.OpList:
			inst.Arg0 = int64(binary.BigEndian.Uint16(ins[ip:]))
			ip += 2
			nextIP := ip
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				count := int(i.Arg0)
				if count < 0 || ctx.SP < count || ctx.SP >= len(ctx.Stack) {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				slots := ctx.Stack[ctx.SP-count : ctx.SP]
				list := vm.BuildListSlots(slots)
				ctx.SP -= count
				ctx.Stack[ctx.SP] = vm.FromValue(list)
				ctx.SP++
				ctx.HelperCalls++
				return nextIP
			}

		case vm.OpMap:
			inst.Arg0 = int64(binary.BigEndian.Uint16(ins[ip:]))
			ip += 2
			nextIP := ip
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				count := int(i.Arg0) * 2
				if i.Arg0 < 0 || count < 0 || ctx.SP < count || ctx.SP >= len(ctx.Stack) {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				slots := ctx.Stack[ctx.SP-count : ctx.SP]
				m := vm.BuildMapSlots(slots)
				ctx.SP -= count
				ctx.Stack[ctx.SP] = vm.FromValue(m)
				ctx.SP++
				ctx.HelperCalls++
				return nextIP
			}

		case vm.OpPow:
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				if ctx.SP < 2 {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				res, err := vm.PowValue(ctx.Stack[ctx.SP-2].ToValue(), ctx.Stack[ctx.SP-1].ToValue())
				ctx.SP -= 2
				ctx.HelperCalls++
				if err != nil {
					ctx.Status = StatusError
					ctx.Err = raisedErrorValue(err)
					ctx.BailoutIP = currentIP + 1
					return -1
				}
				ctx.Stack[ctx.SP] = vm.FromValue(res)
				ctx.SP++
				return currentIP + 1
			}

		case vm.OpIn:
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				if ctx.SP < 2 {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				res := interpreter.InOp(ctx.Stack[ctx.SP-2].ToValue(), ctx.Stack[ctx.SP-1].ToValue())
				ctx.SP -= 2
				ctx.HelperCalls++
				if ev, isErr := res.(*interpreter.ErrorValue); isErr {
					ctx.Status = StatusError
					ctx.Err = ev
					ctx.BailoutIP = currentIP + 1
					return -1
				}
				ctx.Stack[ctx.SP] = vm.FromValue(res)
				ctx.SP++
				return currentIP + 1
			}

		case vm.OpIndex:
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				if ctx.SP < 2 {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				res, err := vm.GetIndexValue(ctx.Stack[ctx.SP-2].ToValue(), ctx.Stack[ctx.SP-1].ToValue())
				ctx.SP -= 2
				ctx.HelperCalls++
				if err != nil {
					ctx.Status = StatusError
					ctx.Err = raisedErrorValue(err)
					ctx.BailoutIP = currentIP + 1
					return -1
				}
				ctx.Stack[ctx.SP] = vm.FromValue(res)
				ctx.SP++
				return currentIP + 1
			}

		case vm.OpSetIndex:
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				if ctx.SP < 3 {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				err := vm.SetIndexValue(ctx.Stack[ctx.SP-2].ToValue(), ctx.Stack[ctx.SP-1].ToValue(), ctx.Stack[ctx.SP-3].ToValue())
				ctx.SP -= 3
				ctx.HelperCalls++
				if err != nil {
					ctx.Status = StatusError
					ctx.Err = raisedErrorValue(err)
					ctx.BailoutIP = currentIP + 1
					return -1
				}
				return currentIP + 1
			}

		case vm.OpSetField:
			inst.Arg0 = int64(binary.BigEndian.Uint16(ins[ip:]))
			ip += 2
			nextIP := ip
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				if ctx.SP < 2 || ctx.Pool == nil || i.Arg0 < 0 || int(i.Arg0) >= len(ctx.Pool.Vals) {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				field, ok := ctx.Pool.Vals[i.Arg0].(*interpreter.StringValue)
				if !ok {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				vm.SetFieldValue(ctx.Stack[ctx.SP-1].ToValue(), field.Val, ctx.Stack[ctx.SP-2].ToValue())
				ctx.SP -= 2
				ctx.HelperCalls++
				return nextIP
			}

		case vm.OpRange:
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				if ctx.SP < 2 {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				res := interpreter.EvalRange(ctx.Stack[ctx.SP-2].ToValue(), ctx.Stack[ctx.SP-1].ToValue())
				ctx.SP -= 2
				ctx.Stack[ctx.SP] = vm.FromValue(res)
				ctx.SP++
				ctx.HelperCalls++
				return currentIP + 1
			}

		case vm.OpIter:
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				if ctx.SP < 1 {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				res, err := vm.IterValue(ctx.Stack[ctx.SP-1].ToValue())
				ctx.HelperCalls++
				if err != nil {
					ctx.Status = StatusError
					ctx.Err = raisedErrorValue(err)
					ctx.BailoutIP = currentIP + 1
					return -1
				}
				ctx.Stack[ctx.SP-1] = vm.FromValue(res)
				return currentIP + 1
			}

		case vm.OpListLen:
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				if ctx.SP < 1 {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				res, err := vm.ListLenValue(ctx.Stack[ctx.SP-1])
				ctx.HelperCalls++
				if err != nil {
					ctx.Status = StatusError
					ctx.Err = raisedErrorValue(err)
					ctx.BailoutIP = currentIP + 1
					return -1
				}
				ctx.Stack[ctx.SP-1] = res
				return currentIP + 1
			}

		case vm.OpCallBuiltin1:
			inst.Arg0 = int64(int8(ins[ip]))
			ip++
			nextIP := ip
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				if ctx.SP < 1 {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				id := int8(i.Arg0)
				slot := ctx.Stack[ctx.SP-1]
				if !vm.ApplyBuiltin1(id, &slot) {
					res, err := vm.CallBuiltin1Fallback(id, slot)
					if err != nil {
						ctx.HelperCalls++
						ctx.Status = StatusError
						ctx.Err = raisedErrorValue(err)
						ctx.BailoutIP = nextIP
						return -1
					}
					slot = res
				}
				ctx.HelperCalls++
				ctx.Stack[ctx.SP-1] = slot
				return nextIP
			}

		case vm.OpConvLocal:
			inst.Arg0 = int64(int8(ins[ip]))
			inst.Arg1 = int64(int8(ins[ip+1]))
			ip += 2
			nextIP := ip
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				local := ctx.BasePointer + int(i.Arg1)
				if local < 0 || local >= len(ctx.Stack) || ctx.SP >= len(ctx.Stack) {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				id := int8(i.Arg0)
				slot := ctx.Stack[local]
				if !vm.ApplyBuiltin1(id, &slot) {
					res, err := vm.CallBuiltin1Fallback(id, slot)
					if err != nil {
						ctx.HelperCalls++
						ctx.Status = StatusError
						ctx.Err = raisedErrorValue(err)
						ctx.BailoutIP = nextIP
						return -1
					}
					slot = res
				}
				ctx.HelperCalls++
				ctx.Stack[ctx.SP] = slot
				ctx.SP++
				return nextIP
			}

		case vm.OpConvGlobal:
			inst.Arg0 = int64(int8(ins[ip]))
			inst.Arg1 = int64(binary.BigEndian.Uint16(ins[ip+1:]))
			ip += 3
			nextIP := ip
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				if ctx.Globals == nil || i.Arg1 < 0 || int(i.Arg1) >= len(ctx.Globals) || ctx.SP >= len(ctx.Stack) {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				id := int8(i.Arg0)
				slot := ctx.Globals[i.Arg1]
				if !vm.ApplyBuiltin1(id, &slot) {
					res, err := vm.CallBuiltin1Fallback(id, slot)
					if err != nil {
						ctx.HelperCalls++
						ctx.Status = StatusError
						ctx.Err = raisedErrorValue(err)
						ctx.BailoutIP = nextIP
						return -1
					}
					slot = res
				}
				ctx.HelperCalls++
				ctx.Stack[ctx.SP] = slot
				ctx.SP++
				return nextIP
			}

		case vm.OpMakeError:
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				if ctx.SP < 2 {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				ev := vm.MakeErrorValue(ctx.Stack[ctx.SP-1], ctx.Stack[ctx.SP-2])
				ctx.SP -= 2
				ctx.Stack[ctx.SP] = vm.FromValue(ev)
				ctx.SP++
				ctx.HelperCalls++
				return currentIP + 1
			}

		case vm.OpRaise:
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				if ctx.SP < 1 {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				ev, isErr := ctx.Stack[ctx.SP-1].ToValue().(*interpreter.ErrorValue)
				if !isErr {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				ctx.SP--
				ctx.HelperCalls++
				ctx.Status = StatusError
				ctx.Err = ev
				ctx.BailoutIP = currentIP + 1
				return -1
			}

		case vm.OpAssert:
			inst.Arg0 = int64(binary.BigEndian.Uint16(ins[ip:]))
			ip += 2
			nextIP := ip
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				if ctx.SP < 1 || ctx.Pool == nil || i.Arg0 < 0 || int(i.Arg0) >= len(ctx.Pool.Slots) {
					ctx.Status = StatusBailout
					ctx.BailoutIP = currentIP
					return -1
				}
				slot := ctx.Stack[ctx.SP-1]
				ctx.SP--
				ctx.HelperCalls++
				if slot.IsBool() && slot.Bool() {
					return nextIP
				}
				msg := interpreter.CoerceToString(ctx.Pool.Slots[i.Arg0].ToValue())
				ctx.Status = StatusError
				ctx.Err = interpreter.NewError(500, msg)
				ctx.BailoutIP = nextIP
				return -1
			}

		case vm.OpNop:
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				return currentIP + 1
			}

		case vm.OpReturnValue:
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				ctx.Status = StatusSuccess
				return -1
			}

		case vm.OpReturn:
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				ctx.Stack[ctx.SP] = vm.NullSlot()
				ctx.SP++
				ctx.Status = StatusSuccess
				return -1
			}

		default:
			// Fallback/bailout to VM
			def, _ := vm.Lookup(byte(op))
			if def != nil {
				for _, w := range def.OperandWidths {
					ip += w
				}
			}
			inst.Exec = func(ctx *ThreadedContext, i *ThreadedInst) int {
				ctx.Status = StatusBailout
				ctx.BailoutIP = currentIP
				return -1
			}
		}

		insts[currentIP] = inst
	}

	return &ThreadedFunction{Insts: insts}
}

// Execute runs the threaded JIT function until completion or bailout.
func (tf *ThreadedFunction) Execute(ctx *ThreadedContext) (status int64, nextIP int) {
	return tf.ExecuteAt(ctx, 0)
}

// ExecuteAt runs the threaded JIT function starting at entryIP until completion or bailout.
func (tf *ThreadedFunction) ExecuteAt(ctx *ThreadedContext, entryIP int) (status int64, nextIP int) {
	ip := entryIP
	for ip >= 0 && ip < len(tf.Insts) {
		inst := &tf.Insts[ip]
		if inst.Exec == nil {
			ctx.Status = StatusBailout
			ctx.BailoutIP = ip
			return StatusBailout, ip
		}
		next := inst.Exec(ctx, inst)
		if next < 0 {
			return ctx.Status, ctx.BailoutIP
		}
		ip = next
	}
	ctx.Status = StatusSuccess
	return StatusSuccess, ip
}
