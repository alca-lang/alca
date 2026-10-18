package vm

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/alca-lang/alca/pkg/ast"
	"github.com/alca-lang/alca/pkg/interpreter"
)

const (
	StackSize   = 65536
	GlobalsSize = 65536
	MaxFrames   = 2048
)

// VM executes compiled Alca bytecode instructions on a high-speed unboxed virtual stack machine.
type VM struct {
	constants []StackSlot
	constVals []interpreter.Value
	globals   []StackSlot

	stack []StackSlot
	sp    int // Stack pointer points to next free slot

	frames      []*Frame
	framesIndex int

	lastPopped StackSlot
	env        *interpreter.Environment

	handlers     []handler
	TestFailures int
	TestPasses   int
}

// handler is an installed error handler (try region).
type handler struct {
	frameIdx int // index into frames of the owning frame
	ip       int // instruction to resume at (the catch block)
	sp       int // stack height to restore
}

// RaisedError is the Go error carrying an Alca error value (err(...) or a failing builtin).
type RaisedError struct{ Val *interpreter.ErrorValue }

func (e *RaisedError) Error() string { return e.Val.Message }

// AlcaErrorCode reports the Alca error code carried by the raised error value,
// so diagnostics can print the code the program actually used.
func (e *RaisedError) AlcaErrorCode() int64 { return e.Val.Code }

// New creates a new VM initialized with compiled bytecode.
func New(bytecode *Bytecode) *VM {
	return NewWithEnv(bytecode, interpreter.NewEnvironment())
}

// NewWithEnv creates a new VM with an attached environment for builtins and standard libraries.
func NewWithEnv(bytecode *Bytecode, env *interpreter.Environment) *VM {
	registerIterBuiltin(env)
	mainFn := &CompiledFunction{Instructions: bytecode.Instructions}
	mainFrame := NewFrame(mainFn, 0)

	frames := make([]*Frame, MaxFrames)
	frames[0] = mainFrame

	slotConstants := make([]StackSlot, len(bytecode.Constants))
	for i, c := range bytecode.Constants {
		slotConstants[i] = FromValue(c)
	}

	return &VM{
		constants:   slotConstants,
		constVals:   bytecode.Constants,
		globals:     make([]StackSlot, GlobalsSize),
		stack:       make([]StackSlot, StackSize),
		sp:          0,
		frames:      frames,
		framesIndex: 1,
		env:         env,
		lastPopped:  FromValue(interpreter.NULL),
	}
}

// StackTop returns the value at the top of the stack without popping it.
func (vm *VM) StackTop() interpreter.Value {
	if vm.sp == 0 {
		return nil
	}
	return vm.stack[vm.sp-1].ToValue()
}

// LastPoppedStackElem returns the most recently popped value.
func (vm *VM) LastPoppedStackElem() interpreter.Value {
	return vm.lastPopped.ToValue()
}

func (vm *VM) currentFrame() *Frame {
	return vm.frames[vm.framesIndex-1]
}

func (vm *VM) pushFrame(f *Frame) {
	vm.frames[vm.framesIndex] = f
	vm.framesIndex++
}

func (vm *VM) popFrame() *Frame {
	vm.framesIndex--
	return vm.frames[vm.framesIndex]
}

func (vm *VM) pushSlot(s StackSlot) error {
	if vm.sp >= StackSize {
		return fmt.Errorf("stack overflow")
	}
	vm.stack[vm.sp] = s
	vm.sp++
	return nil
}

func (vm *VM) push(v interpreter.Value) error {
	return vm.pushSlot(FromValue(v))
}

func (vm *VM) pop() interpreter.Value {
	if vm.sp == 0 {
		return interpreter.NULL
	}
	vm.sp--
	vm.lastPopped = vm.stack[vm.sp]
	return vm.lastPopped.ToValue()
}

// Run executes the bytecode until completion. Errors raised by err(...), failing
// built-ins or runtime faults (division by zero, ...) are delivered to the nearest
// installed handler (see OpTryBegin) and execution resumes there; without a handler
// the error terminates the program.
func (vm *VM) Run() error {
	for {
		err := vm.runGuarded()
		if err == nil {
			return nil
		}
		if !vm.unwind(err) {
			return err
		}
	}
}

// runGuarded converts Go runtime panics (stack exhaustion, bad operands) into errors.
func (vm *VM) runGuarded() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("runtime fault: %v", r)
		}
	}()
	return vm.run()
}

// unwind transfers control to the innermost handler. It reports false if none exists.
func (vm *VM) unwind(err error) bool {
	if len(vm.handlers) == 0 {
		return false
	}
	h := vm.handlers[len(vm.handlers)-1]
	vm.handlers = vm.handlers[:len(vm.handlers)-1]

	var ev *interpreter.ErrorValue
	if re, ok := err.(*RaisedError); ok {
		ev = re.Val
	} else {
		ev = interpreter.NewError(500, err.Error())
	}
	vm.framesIndex = h.frameIdx + 1
	vm.frames[h.frameIdx].ip = h.ip
	vm.sp = h.sp
	vm.stack[vm.sp] = FromValue(ev)
	vm.sp++
	return true
}

// run is the interpreter loop proper.
func (vm *VM) run() error {
	frame := vm.currentFrame()
	ins := frame.Instructions()
	ip := frame.ip
	sp := vm.sp
	stack := vm.stack
	globals := vm.globals
	constants := vm.constants

	for ip < len(ins) {
		op := Opcode(ins[ip])
		ip++

		switch op {
		case OpConstant:
			constIndex := binary.BigEndian.Uint16(ins[ip:])
			ip += 2
			stack[sp] = constants[constIndex]
			sp++

		case OpPop:
			if sp > 0 {
				sp--
				vm.lastPopped = stack[sp]
			}

		case OpDup:
			if sp > 0 {
				stack[sp] = stack[sp-1]
				sp++
			}

		case OpAdd:
			r := &stack[sp-1]
			l := &stack[sp-2]
			if l.tag == SlotInt && r.tag == SlotInt {
				l.i64 += r.i64
				sp--
				continue
			}
			if l.tag == SlotFloat && r.tag == SlotFloat {
				l.f64 += r.f64
				sp--
				continue
			}
			vm.sp = sp
			right := vm.pop()
			left := vm.pop()
			if err := vm.executeBinaryOp(OpAdd, left, right); err != nil {
				frame.ip = ip
				vm.sp = sp
				return err
			}
			sp = vm.sp

		case OpSub:
			r := &stack[sp-1]
			l := &stack[sp-2]
			if l.tag == SlotInt && r.tag == SlotInt {
				l.i64 -= r.i64
				sp--
				continue
			}
			if l.tag == SlotFloat && r.tag == SlotFloat {
				l.f64 -= r.f64
				sp--
				continue
			}
			vm.sp = sp
			right := vm.pop()
			left := vm.pop()
			if err := vm.executeBinaryOp(OpSub, left, right); err != nil {
				frame.ip = ip
				vm.sp = sp
				return err
			}
			sp = vm.sp

		case OpMul:
			r := &stack[sp-1]
			l := &stack[sp-2]
			if l.tag == SlotInt && r.tag == SlotInt {
				l.i64 *= r.i64
				sp--
				continue
			}
			if l.tag == SlotFloat && r.tag == SlotFloat {
				l.f64 *= r.f64
				sp--
				continue
			}
			vm.sp = sp
			right := vm.pop()
			left := vm.pop()
			if err := vm.executeBinaryOp(OpMul, left, right); err != nil {
				frame.ip = ip
				vm.sp = sp
				return err
			}
			sp = vm.sp

		case OpDiv:
			r := &stack[sp-1]
			l := &stack[sp-2]
			if l.tag == SlotInt && r.tag == SlotInt {
				if r.i64 == 0 {
					frame.ip = ip
					vm.sp = sp
					return &RaisedError{Val: interpreter.NewError(400, "division by zero")}
				}
				l.i64 /= r.i64
				sp--
				continue
			}
			vm.sp = sp
			right := vm.pop()
			left := vm.pop()
			if err := vm.executeBinaryOp(OpDiv, left, right); err != nil {
				frame.ip = ip
				vm.sp = sp
				return err
			}
			sp = vm.sp

		case OpMod:
			r := &stack[sp-1]
			l := &stack[sp-2]
			if l.tag == SlotInt && r.tag == SlotInt {
				if r.i64 == 0 {
					frame.ip = ip
					vm.sp = sp
					return &RaisedError{Val: interpreter.NewError(400, "modulo by zero")}
				}
				l.i64 %= r.i64
				sp--
				continue
			}
			vm.sp = sp
			right := vm.pop()
			left := vm.pop()
			if err := vm.executeBinaryOp(OpMod, left, right); err != nil {
				frame.ip = ip
				vm.sp = sp
				return err
			}
			sp = vm.sp

		case OpPow:
			vm.sp = sp
			right := vm.pop()
			left := vm.pop()
			if err := vm.executeBinaryOp(OpPow, left, right); err != nil {
				frame.ip = ip
				vm.sp = sp
				return err
			}
			sp = vm.sp

		case OpLessThan:
			r := &stack[sp-1]
			l := &stack[sp-2]
			if l.tag == SlotInt && r.tag == SlotInt {
				l.tag = SlotBool
				if l.i64 < r.i64 {
					l.i64 = 1
				} else {
					l.i64 = 0
				}
				sp--
				continue
			}
			vm.sp = sp
			right := vm.pop()
			left := vm.pop()
			if err := vm.executeComparison(OpLessThan, left, right); err != nil {
				frame.ip = ip
				vm.sp = sp
				return err
			}
			sp = vm.sp

		case OpLessEqual:
			r := &stack[sp-1]
			l := &stack[sp-2]
			if l.tag == SlotInt && r.tag == SlotInt {
				l.tag = SlotBool
				if l.i64 <= r.i64 {
					l.i64 = 1
				} else {
					l.i64 = 0
				}
				sp--
				continue
			}
			vm.sp = sp
			right := vm.pop()
			left := vm.pop()
			if err := vm.executeComparison(OpLessEqual, left, right); err != nil {
				frame.ip = ip
				vm.sp = sp
				return err
			}
			sp = vm.sp

		case OpGreaterThan:
			r := &stack[sp-1]
			l := &stack[sp-2]
			if l.tag == SlotInt && r.tag == SlotInt {
				l.tag = SlotBool
				if l.i64 > r.i64 {
					l.i64 = 1
				} else {
					l.i64 = 0
				}
				sp--
				continue
			}
			vm.sp = sp
			right := vm.pop()
			left := vm.pop()
			if err := vm.executeComparison(OpGreaterThan, left, right); err != nil {
				frame.ip = ip
				vm.sp = sp
				return err
			}
			sp = vm.sp

		case OpGreaterEqual:
			r := &stack[sp-1]
			l := &stack[sp-2]
			if l.tag == SlotInt && r.tag == SlotInt {
				l.tag = SlotBool
				if l.i64 >= r.i64 {
					l.i64 = 1
				} else {
					l.i64 = 0
				}
				sp--
				continue
			}
			vm.sp = sp
			right := vm.pop()
			left := vm.pop()
			if err := vm.executeComparison(OpGreaterEqual, left, right); err != nil {
				frame.ip = ip
				vm.sp = sp
				return err
			}
			sp = vm.sp

		case OpEqual:
			r := &stack[sp-1]
			l := &stack[sp-2]
			if l.tag == SlotInt && r.tag == SlotInt {
				l.tag = SlotBool
				if l.i64 == r.i64 {
					l.i64 = 1
				} else {
					l.i64 = 0
				}
				sp--
				continue
			}
			vm.sp = sp
			right := vm.pop()
			left := vm.pop()
			if err := vm.executeComparison(OpEqual, left, right); err != nil {
				frame.ip = ip
				vm.sp = sp
				return err
			}
			sp = vm.sp

		case OpNotEqual:
			r := &stack[sp-1]
			l := &stack[sp-2]
			if l.tag == SlotInt && r.tag == SlotInt {
				l.tag = SlotBool
				if l.i64 != r.i64 {
					l.i64 = 1
				} else {
					l.i64 = 0
				}
				sp--
				continue
			}
			vm.sp = sp
			right := vm.pop()
			left := vm.pop()
			if err := vm.executeComparison(OpNotEqual, left, right); err != nil {
				frame.ip = ip
				vm.sp = sp
				return err
			}
			sp = vm.sp

		case OpIn:
			vm.sp = sp
			right := vm.pop()
			left := vm.pop()
			res := interpreter.InOp(left, right)
			if ev, ok := res.(*interpreter.ErrorValue); ok {
				frame.ip = ip
				vm.sp = sp
				return &RaisedError{Val: ev}
			}
			if err := vm.push(res); err != nil {
				return err
			}
			sp = vm.sp

		case OpTrue:
			stack[sp] = StackSlot{tag: SlotBool, i64: 1}
			sp++

		case OpFalse:
			stack[sp] = StackSlot{tag: SlotBool, i64: 0}
			sp++

		case OpNull:
			stack[sp] = StackSlot{tag: SlotNull}
			sp++

		case OpNot:
			slot := &stack[sp-1]
			if slot.tag == SlotBool {
				if slot.i64 != 0 {
					slot.i64 = 0
				} else {
					slot.i64 = 1
				}
			} else {
				if interpreter.IsTruthy(slot.ToValue()) {
					*slot = StackSlot{tag: SlotBool, i64: 0}
				} else {
					*slot = StackSlot{tag: SlotBool, i64: 1}
				}
			}

		case OpNegate:
			slot := &stack[sp-1]
			if slot.tag == SlotInt {
				slot.i64 = -slot.i64
			} else if slot.tag == SlotFloat {
				slot.f64 = -slot.f64
			}

		case OpJump:
			pos := int(binary.BigEndian.Uint16(ins[ip:]))
			ip = pos

		case OpJumpIfFalse:
			pos := int(binary.BigEndian.Uint16(ins[ip:]))
			ip += 2
			sp--
			slot := &stack[sp]
			var isFalse bool
			if slot.tag == SlotBool {
				isFalse = (slot.i64 == 0)
			} else if slot.tag == SlotInt {
				isFalse = (slot.i64 == 0)
			} else if slot.tag == SlotNull {
				isFalse = true
			} else {
				isFalse = !interpreter.IsTruthy(slot.ToValue())
			}
			if isFalse {
				ip = pos
			}

		case OpSetGlobal:
			globalIndex := binary.BigEndian.Uint16(ins[ip:])
			ip += 2
			sp--
			globals[globalIndex] = stack[sp]

		case OpGetGlobal:
			globalIndex := binary.BigEndian.Uint16(ins[ip:])
			ip += 2
			stack[sp] = globals[globalIndex]
			sp++

		case OpGetBuiltin:
			constIndex := binary.BigEndian.Uint16(ins[ip:])
			ip += 2
			nameVal := vm.constVals[constIndex].(*interpreter.StringValue).Val
			val, ok := vm.env.Get(nameVal)
			if !ok {
				if fn, exists := interpreter.BuiltinFunctionMap[nameVal]; exists {
					val = fn
				} else {
					val = interpreter.NULL
				}
			}
			stack[sp] = FromValue(val)
			sp++

		case OpSetLocal:
			localIndex := int(ins[ip])
			ip++
			sp--
			stack[frame.basePointer+localIndex] = stack[sp]

		case OpGetLocal:
			localIndex := int(ins[ip])
			ip++
			stack[sp] = stack[frame.basePointer+localIndex]
			sp++

		case OpIncGlobal:
			globalIndex := binary.BigEndian.Uint16(ins[ip:])
			ip += 2
			if globals[globalIndex].tag == SlotInt {
				globals[globalIndex].i64++
				continue
			}
			vm.sp = sp
			res, err := vm.slowArith(OpAdd, globals[globalIndex], StackSlot{tag: SlotInt, i64: 1})
			if err != nil {
				frame.ip = ip
				return err
			}
			sp = vm.sp
			globals[globalIndex] = res
		case OpIncLocal:
			localIndex := int(ins[ip])
			ip++
			if stack[frame.basePointer+localIndex].tag == SlotInt {
				stack[frame.basePointer+localIndex].i64++
				continue
			}
			vm.sp = sp
			res, err := vm.slowArith(OpAdd, stack[frame.basePointer+localIndex], StackSlot{tag: SlotInt, i64: 1})
			if err != nil {
				frame.ip = ip
				return err
			}
			sp = vm.sp
			stack[frame.basePointer+localIndex] = res
		case OpAddGlobal:
			globalIndex := binary.BigEndian.Uint16(ins[ip:])
			ip += 2
			sp--
			if globals[globalIndex].tag == SlotInt && stack[sp].tag == SlotInt {
				globals[globalIndex].i64 += stack[sp].i64
				continue
			}
			vm.sp = sp
			res, err := vm.slowArith(OpAdd, globals[globalIndex], stack[sp])
			if err != nil {
				frame.ip = ip
				return err
			}
			sp = vm.sp
			globals[globalIndex] = res
		case OpAddLocal:
			localIndex := int(ins[ip])
			ip++
			sp--
			if stack[frame.basePointer+localIndex].tag == SlotInt && stack[sp].tag == SlotInt {
				stack[frame.basePointer+localIndex].i64 += stack[sp].i64
				continue
			}
			vm.sp = sp
			res, err := vm.slowArith(OpAdd, stack[frame.basePointer+localIndex], stack[sp])
			if err != nil {
				frame.ip = ip
				return err
			}
			sp = vm.sp
			stack[frame.basePointer+localIndex] = res
		case OpSubGlobal:
			globalIndex := binary.BigEndian.Uint16(ins[ip:])
			ip += 2
			sp--
			if globals[globalIndex].tag == SlotInt && stack[sp].tag == SlotInt {
				globals[globalIndex].i64 -= stack[sp].i64
				continue
			}
			vm.sp = sp
			res, err := vm.slowArith(OpSub, globals[globalIndex], stack[sp])
			if err != nil {
				frame.ip = ip
				return err
			}
			sp = vm.sp
			globals[globalIndex] = res
		case OpSubLocal:
			localIndex := int(ins[ip])
			ip++
			sp--
			if stack[frame.basePointer+localIndex].tag == SlotInt && stack[sp].tag == SlotInt {
				stack[frame.basePointer+localIndex].i64 -= stack[sp].i64
				continue
			}
			vm.sp = sp
			res, err := vm.slowArith(OpSub, stack[frame.basePointer+localIndex], stack[sp])
			if err != nil {
				frame.ip = ip
				return err
			}
			sp = vm.sp
			stack[frame.basePointer+localIndex] = res
		case OpLoopGlobalLtInc:
			varIdx := binary.BigEndian.Uint16(ins[ip:])
			constIdx := binary.BigEndian.Uint16(ins[ip+2:])
			target := int(binary.BigEndian.Uint16(ins[ip+4:]))
			ip += 6
			if globals[varIdx].tag != SlotInt || constants[constIdx].tag != SlotInt {
				frame.ip = ip
				vm.sp = sp
				return fmt.Errorf("range loop requires integer bounds")
			}
			globals[varIdx].i64++
			if globals[varIdx].i64 < constants[constIdx].i64 {
				ip = target
			}
		case OpLoopGlobalLeInc:
			varIdx := binary.BigEndian.Uint16(ins[ip:])
			endIdx := binary.BigEndian.Uint16(ins[ip+2:])
			target := int(binary.BigEndian.Uint16(ins[ip+4:]))
			ip += 6
			if globals[varIdx].tag != SlotInt || globals[endIdx].tag != SlotInt {
				frame.ip = ip
				vm.sp = sp
				return fmt.Errorf("range loop requires integer bounds")
			}
			globals[varIdx].i64++
			if globals[varIdx].i64 <= globals[endIdx].i64 {
				ip = target
			}
		case OpModConst:
			constIdx := binary.BigEndian.Uint16(ins[ip:])
			ip += 2
			if stack[sp-1].tag == SlotInt && constants[constIdx].tag == SlotInt && constants[constIdx].i64 != 0 {
				stack[sp-1].i64 %= constants[constIdx].i64
				continue
			}
			vm.sp = sp
			res, err := vm.slowArith(OpMod, stack[sp-1], constants[constIdx])
			if err != nil {
				frame.ip = ip
				return err
			}
			sp = vm.sp
			stack[sp-1] = res
		case OpMulConst:
			constIdx := binary.BigEndian.Uint16(ins[ip:])
			ip += 2
			if stack[sp-1].tag == SlotInt && constants[constIdx].tag == SlotInt {
				stack[sp-1].i64 *= constants[constIdx].i64
				continue
			}
			vm.sp = sp
			res, err := vm.slowArith(OpMul, stack[sp-1], constants[constIdx])
			if err != nil {
				frame.ip = ip
				return err
			}
			sp = vm.sp
			stack[sp-1] = res
		case OpAddConst:
			constIdx := binary.BigEndian.Uint16(ins[ip:])
			ip += 2
			if stack[sp-1].tag == SlotInt && constants[constIdx].tag == SlotInt {
				stack[sp-1].i64 += constants[constIdx].i64
				continue
			}
			vm.sp = sp
			res, err := vm.slowArith(OpAdd, stack[sp-1], constants[constIdx])
			if err != nil {
				frame.ip = ip
				return err
			}
			sp = vm.sp
			stack[sp-1] = res
		case OpList:
			numElements := int(binary.BigEndian.Uint16(ins[ip:]))
			ip += 2
			vm.sp = sp
			list := vm.buildList(sp-numElements, sp)
			sp -= numElements
			stack[sp] = FromValue(list)
			sp++

		case OpMap:
			numPairs := int(binary.BigEndian.Uint16(ins[ip:]))
			ip += 2
			vm.sp = sp
			m := vm.buildMap(sp-(numPairs*2), sp)
			sp -= (numPairs * 2)
			stack[sp] = FromValue(m)
			sp++

		case OpRange:
			end := stack[sp-1].ToValue()
			start := stack[sp-2].ToValue()
			sp -= 2
			stack[sp] = FromValue(interpreter.EvalRange(start, end))
			sp++

		case OpIndex:
			idx := stack[sp-1].ToValue()
			target := stack[sp-2].ToValue()
			sp -= 2
			res, err := vm.executeIndex(target, idx)
			if err != nil {
				frame.ip = ip
				vm.sp = sp
				return err
			}
			stack[sp] = FromValue(res)
			sp++

		case OpSetIndex:
			idx := stack[sp-1].ToValue()
			target := stack[sp-2].ToValue()
			val := stack[sp-3].ToValue()
			sp -= 3
			if err := vm.executeSetIndex(target, idx, val); err != nil {
				frame.ip = ip
				vm.sp = sp
				return err
			}

		case OpGetField:
			nameIndex := binary.BigEndian.Uint16(ins[ip:])
			ip += 2
			fieldName := vm.constVals[nameIndex].(*interpreter.StringValue).Val
			sp--
			target := stack[sp].ToValue()
			res, err := vm.executeGetField(target, fieldName)
			if err != nil {
				frame.ip = ip
				vm.sp = sp
				return err
			}
			stack[sp] = FromValue(res)
			sp++

		case OpSetField:
			nameIndex := binary.BigEndian.Uint16(ins[ip:])
			ip += 2
			fieldName := vm.constVals[nameIndex].(*interpreter.StringValue).Val
			target := stack[sp-1].ToValue()
			val := stack[sp-2].ToValue()
			sp -= 2
			if m, ok := target.(*interpreter.MapValue); ok {
				m.Set(fieldName, val)
			} else if si, ok := target.(*interpreter.StructInstance); ok {
				si.Fields[fieldName] = val
			}

		case OpUse:
			constIndex := binary.BigEndian.Uint16(ins[ip:])
			ip += 2
			useVal := vm.constVals[constIndex].(*UseStatementValue)
			interp := interpreter.New()
			interp.ExecuteInEnv(&ast.Program{Statements: []ast.Statement{useVal.Stmt}}, vm.env)

		case OpCall:
			numArgs := int(ins[ip])
			ip++
			fnIndex := sp - 1 - numArgs
			fn := stack[fnIndex].ToValue()
			frame.ip = ip
			vm.sp = sp
			if err := vm.executeCall(fn, numArgs); err != nil {
				return err
			}
			frame = vm.currentFrame()
			ins = frame.Instructions()
			ip = frame.ip
			sp = vm.sp
			stack = vm.stack

		case OpReturnValue:
			sp--
			returnVal := stack[sp]
			frame = vm.popFrame()
			sp = frame.basePointer - 1
			stack[sp] = returnVal
			sp++
			if vm.framesIndex == 0 {
				vm.sp = sp
				vm.lastPopped = returnVal
				return nil
			}
			frame = vm.currentFrame()
			ins = frame.Instructions()
			ip = frame.ip

		case OpReturn:
			frame = vm.popFrame()
			sp = frame.basePointer - 1
			stack[sp] = StackSlot{tag: SlotNull}
			sp++
			if vm.framesIndex == 0 {
				vm.sp = sp
				vm.lastPopped = StackSlot{tag: SlotNull}
				return nil
			}
			frame = vm.currentFrame()
			ins = frame.Instructions()
			ip = frame.ip

		case OpNop:

		case OpTryBegin:
			target := int(binary.BigEndian.Uint16(ins[ip:]))
			ip += 2
			vm.handlers = append(vm.handlers, handler{frameIdx: vm.framesIndex - 1, ip: target, sp: sp})

		case OpEndTry:
			if n := len(vm.handlers); n > 0 {
				vm.handlers = vm.handlers[:n-1]
			}

		case OpMakeError:
			codeSlot := stack[sp-1]
			msgSlot := stack[sp-2]
			sp -= 2
			code := int64(500)
			if codeSlot.tag == SlotInt {
				code = codeSlot.i64
			}
			ev := interpreter.NewError(code, interpreter.CoerceToString(msgSlot.ToValue()))
			stack[sp] = FromValue(ev)
			sp++

		case OpRaise:
			sp--
			frame.ip = ip
			vm.sp = sp
			if ev, ok := stack[sp].ToValue().(*interpreter.ErrorValue); ok {
				return &RaisedError{Val: ev}
			}
			return fmt.Errorf("raise requires an error value")

		case OpAssert:
			msgIdx := binary.BigEndian.Uint16(ins[ip:])
			ip += 2
			sp--
			if !(stack[sp].tag == SlotBool && stack[sp].i64 != 0) {
				frame.ip = ip
				vm.sp = sp
				return &RaisedError{Val: interpreter.NewError(500, interpreter.CoerceToString(constants[msgIdx].ToValue()))}
			}

		case OpTestPass:
			nameIdx := binary.BigEndian.Uint16(ins[ip:])
			ip += 2
			vm.TestPasses++
			fmt.Printf("[PASS] Test: %s\n", interpreter.CoerceToString(constants[nameIdx].ToValue()))

		case OpTestFail:
			nameIdx := binary.BigEndian.Uint16(ins[ip:])
			ip += 2
			sp--
			msg := interpreter.CoerceToString(stack[sp].ToValue())
			if ev, ok := stack[sp].ToValue().(*interpreter.ErrorValue); ok {
				msg = ev.Message
			}
			vm.TestFailures++
			fmt.Printf("[FAIL] Test: %s - %s\n", interpreter.CoerceToString(constants[nameIdx].ToValue()), msg)

		case OpStream:
			opByte := ins[ip]
			ip++
			target := stack[sp-1].ToValue()
			source := stack[sp-2].ToValue()
			sp -= 2
			res, err := vm.executeStream(source, target, opByte)
			if err != nil {
				frame.ip = ip
				vm.sp = sp
				return err
			}
			stack[sp] = FromValue(res)
			sp++
		}
	}

	frame.ip = ip
	vm.sp = sp
	return nil
}

func (vm *VM) executeBinaryOp(op Opcode, left, right interpreter.Value) error {
	// String concatenation
	if left.Type() == interpreter.STRING_VAL || right.Type() == interpreter.STRING_VAL {
		if op == OpAdd {
			s1 := interpreter.CoerceToString(left)
			s2 := interpreter.CoerceToString(right)
			return vm.push(&interpreter.StringValue{Val: s1 + s2})
		}
	}

	// List concatenation (mirrors the interpreter's '+' on two lists)
	if op == OpAdd && left.Type() == interpreter.LIST_VAL && right.Type() == interpreter.LIST_VAL {
		l1 := left.(*interpreter.ListValue)
		l2 := right.(*interpreter.ListValue)
		combined := make([]interpreter.Value, 0, len(l1.Elements)+len(l2.Elements))
		combined = append(combined, l1.Elements...)
		combined = append(combined, l2.Elements...)
		return vm.push(interpreter.NewList(combined...))
	}

	// Integer arithmetic
	if left.Type() == interpreter.INTEGER_VAL && right.Type() == interpreter.INTEGER_VAL {
		l := left.(*interpreter.IntegerValue).Val
		r := right.(*interpreter.IntegerValue).Val
		switch op {
		case OpAdd:
			return vm.push(&interpreter.IntegerValue{Val: l + r})
		case OpSub:
			return vm.push(&interpreter.IntegerValue{Val: l - r})
		case OpMul:
			return vm.push(&interpreter.IntegerValue{Val: l * r})
		case OpDiv:
			if r == 0 {
				return &RaisedError{Val: interpreter.NewError(400, "division by zero")}
			}
			return vm.push(&interpreter.IntegerValue{Val: l / r})
		case OpMod:
			if r == 0 {
				return &RaisedError{Val: interpreter.NewError(400, "modulo by zero")}
			}
			return vm.push(&interpreter.IntegerValue{Val: l % r})
		case OpPow:
			if r < 0 {
				return &RaisedError{Val: interpreter.NewError(400, "negative exponent for integer power; use float operands")}
			}
			return vm.push(&interpreter.IntegerValue{Val: interpreter.IntPow(l, r)})
		}
	}

	if op == OpMod {
		// '%' is only defined for int/int, matching the interpreter.
		return &RaisedError{Val: interpreter.NewError(400, "modulo requires integer operands")}
	}

	// Float arithmetic
	lf := toFloat(left)
	rf := toFloat(right)
	switch op {
	case OpAdd:
		return vm.push(&interpreter.FloatValue{Val: lf + rf})
	case OpSub:
		return vm.push(&interpreter.FloatValue{Val: lf - rf})
	case OpMul:
		return vm.push(&interpreter.FloatValue{Val: lf * rf})
	case OpDiv:
		if rf == 0 {
			return &RaisedError{Val: interpreter.NewError(400, "division by zero")}
		}
		return vm.push(&interpreter.FloatValue{Val: lf / rf})
	case OpPow:
		return vm.push(&interpreter.FloatValue{Val: math.Pow(lf, rf)})
	}

	return fmt.Errorf("unsupported binary operation")
}

// toFloat mirrors the interpreter's getNumericFloat: only int/float operands
// have a numeric value, everything else is treated as zero.
func toFloat(v interpreter.Value) float64 {
	switch n := v.(type) {
	case *interpreter.FloatValue:
		return n.Val
	case *interpreter.IntegerValue:
		return float64(n.Val)
	default:
		return 0
	}
}

func (vm *VM) executeComparison(op Opcode, left, right interpreter.Value) error {
	switch op {
	case OpEqual:
		return vm.push(nativeBool(interpreter.ValuesEqual(left, right)))
	case OpNotEqual:
		return vm.push(nativeBool(!interpreter.ValuesEqual(left, right)))
	case OpGreaterThan, OpGreaterEqual, OpLessThan, OpLessEqual:
		// Strings order lexicographically, numbers numerically.
		if ls, ok := left.(*interpreter.StringValue); ok {
			if rs, ok := right.(*interpreter.StringValue); ok {
				switch op {
				case OpGreaterThan:
					return vm.push(nativeBool(ls.Val > rs.Val))
				case OpGreaterEqual:
					return vm.push(nativeBool(ls.Val >= rs.Val))
				case OpLessThan:
					return vm.push(nativeBool(ls.Val < rs.Val))
				case OpLessEqual:
					return vm.push(nativeBool(ls.Val <= rs.Val))
				}
			}
		}
		lf, rf := toFloat(left), toFloat(right)
		switch op {
		case OpGreaterThan:
			return vm.push(nativeBool(lf > rf))
		case OpGreaterEqual:
			return vm.push(nativeBool(lf >= rf))
		case OpLessThan:
			return vm.push(nativeBool(lf < rf))
		case OpLessEqual:
			return vm.push(nativeBool(lf <= rf))
		}
	}
	return fmt.Errorf("unknown comparison operator")
}

func nativeBool(b bool) interpreter.Value {
	if b {
		return interpreter.TRUE
	}
	return interpreter.FALSE
}

func (vm *VM) buildList(startIdx, endIdx int) interpreter.Value {
	elements := make([]interpreter.Value, endIdx-startIdx)
	for i := startIdx; i < endIdx; i++ {
		elements[i-startIdx] = vm.stack[i].ToValue()
	}
	return interpreter.NewList(elements...)
}

func (vm *VM) buildMap(startIdx, endIdx int) interpreter.Value {
	m := interpreter.NewMap()
	for i := startIdx; i < endIdx; i += 2 {
		key := interpreter.CoerceToString(vm.stack[i].ToValue())
		val := vm.stack[i+1].ToValue()
		m.Set(key, val)
	}
	return m
}

func (vm *VM) executeIndex(target, idx interpreter.Value) (interpreter.Value, error) {
	switch t := target.(type) {
	case *interpreter.ListValue:
		iv, ok := idx.(*interpreter.IntegerValue)
		if !ok {
			return nil, &RaisedError{Val: interpreter.NewError(400, "list index must be an integer")}
		}
		iVal := iv.Val
		n := int64(t.Len())
		if iVal < 0 {
			iVal += n
		}
		if iVal < 0 || iVal >= n {
			return nil, &RaisedError{Val: interpreter.NewError(400, fmt.Sprintf("index %d out of bounds (len %d)", iv.Val, n))}
		}
		return t.Elements[iVal], nil

	case *interpreter.MapValue:
		key := interpreter.CoerceToString(idx)
		if val, ok := t.Get(key); ok {
			return val, nil
		}
		return interpreter.NULL, nil

	case *interpreter.StringValue:
		iv, ok := idx.(*interpreter.IntegerValue)
		if !ok {
			return nil, &RaisedError{Val: interpreter.NewError(400, "string index must be an integer")}
		}
		runes := []rune(t.Val) // index by character, not by byte
		iVal := iv.Val
		n := int64(len(runes))
		if iVal < 0 {
			iVal += n
		}
		if iVal < 0 || iVal >= n {
			return nil, &RaisedError{Val: interpreter.NewError(400, fmt.Sprintf("string index %d out of bounds", iv.Val))}
		}
		return &interpreter.StringValue{Val: string(runes[iVal])}, nil
	}

	return nil, &RaisedError{Val: interpreter.NewError(400, fmt.Sprintf("cannot index value of type %s", target.Type()))}
}

func (vm *VM) executeSetIndex(target, idx, val interpreter.Value) error {
	switch t := target.(type) {
	case *interpreter.ListValue:
		if iv, ok := idx.(*interpreter.IntegerValue); ok {
			iVal := iv.Val
			n := int64(t.Len())
			if iVal < 0 {
				iVal += n
			}
			if iVal >= 0 && iVal < n {
				t.Elements[iVal] = val
				return nil
			}
		}
		return &RaisedError{Val: interpreter.NewError(400, "invalid list index for assignment")}
	case *interpreter.MapValue:
		t.Set(interpreter.CoerceToString(idx), val)
		return nil
	}
	return &RaisedError{Val: interpreter.NewError(400, "cannot assign to indexed target")}
}

func (vm *VM) executeGetField(target interpreter.Value, field string) (interpreter.Value, error) {
	if m, ok := target.(*interpreter.MapValue); ok {
		if val, exists := m.Get(field); exists {
			return val, nil
		}
	}
	if si, ok := target.(*interpreter.StructInstance); ok {
		if val, exists := si.Fields[field]; exists {
			return val, nil
		}
	}
	if ev, ok := target.(*interpreter.ErrorValue); ok {
		switch field {
		case "msg", "message":
			return &interpreter.StringValue{Val: ev.Message}, nil
		case "code":
			return &interpreter.IntegerValue{Val: ev.Code}, nil
		case "time":
			return &interpreter.StringValue{Val: ev.Time}, nil
		}
		return interpreter.NULL, nil
	}
	// Support method call dispatch via interpreter.CallMethod
	methodWrapper := &interpreter.BuiltinFunction{
		Name: field,
		Fn: func(args ...interpreter.Value) interpreter.Value {
			res, err := interpreter.CallMethod(target, field, args)
			if err != nil {
				return interpreter.NewError(500, err.Error())
			}
			return res
		},
	}
	return methodWrapper, nil
}

func (vm *VM) executeCall(fn interpreter.Value, numArgs int) error {
	switch callee := fn.(type) {
	case *CompiledFunction:
		if numArgs != callee.NumParameters {
			return fmt.Errorf("wrong number of arguments: want=%d, got=%d", callee.NumParameters, numArgs)
		}
		basePointer := vm.sp - numArgs
		frame := NewFrame(callee, basePointer)
		vm.pushFrame(frame)
		vm.sp = basePointer + callee.NumLocals

	case *interpreter.BuiltinFunction:
		args := make([]interpreter.Value, numArgs)
		for idx, s := range vm.stack[vm.sp-numArgs : vm.sp] {
			args[idx] = s.ToValue()
		}
		res := callee.Fn(args...)
		vm.sp = vm.sp - 1 - numArgs // Clean up function and arguments from stack
		if ev, ok := res.(*interpreter.ErrorValue); ok {
			return &RaisedError{Val: ev}
		}
		if res != nil {
			vm.push(res)
		} else {
			vm.push(interpreter.NULL)
		}

	case *interpreter.FuncValue:
		// Fallback to tree-walking closure invocation
		args := make([]interpreter.Value, numArgs)
		for idx, s := range vm.stack[vm.sp-numArgs : vm.sp] {
			args[idx] = s.ToValue()
		}
		interp := interpreter.New()
		res, err := interp.InvokeFunction(callee, args)
		vm.sp = vm.sp - 1 - numArgs
		if err != nil {
			return err
		}
		if res != nil {
			vm.push(res)
		} else {
			vm.push(interpreter.NULL)
		}

	case *interpreter.StructDef:
		// Struct instantiation via VM
		args := make([]interpreter.Value, numArgs)
		for idx, s := range vm.stack[vm.sp-numArgs : vm.sp] {
			args[idx] = s.ToValue()
		}
		interp := interpreter.New()
		res, err := interp.InvokeFunction(callee, args)
		vm.sp = vm.sp - 1 - numArgs
		if err != nil {
			return err
		}
		if res != nil {
			vm.push(res)
		} else {
			vm.push(interpreter.NULL)
		}

	default:
		return fmt.Errorf("calling non-function: %T", fn)
	}
	return nil
}

func (vm *VM) executeStream(source, target interpreter.Value, opByte byte) (interpreter.Value, error) {
	opMap := map[byte]string{
		1: ">>",
		2: ">!",
		3: ">>>",
		4: "<<",
		5: "?>>",
		6: "?>>>",
		7: "&>>",
		8: "&&>>",
		9: "|>",
	}
	opStr := opMap[opByte]
	return interpreter.HandleStream(source, opStr, target, vm.env)
}

// slowArith evaluates "l op r" through the generic (type-dispatching) path and
// returns the result slot. Used by the int-specialised fast opcodes whenever
// their operands are not both plain integers (floats, strings, lists, ...).
func (vm *VM) slowArith(op Opcode, l, r StackSlot) (StackSlot, error) {
	if err := vm.executeBinaryOp(op, l.ToValue(), r.ToValue()); err != nil {
		return StackSlot{}, err
	}
	res := vm.stack[vm.sp-1]
	vm.sp--
	return res, nil
}

// registerIterBuiltin installs the hidden "@iter" helper used by for-in loops: lists iterate
// as-is, maps iterate their keys and strings iterate their characters.
func registerIterBuiltin(env *interpreter.Environment) {
	env.Set("@iter", &interpreter.BuiltinFunction{Name: "@iter", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) != 1 {
			return interpreter.NewError(400, "iteration needs exactly one value")
		}
		switch x := args[0].(type) {
		case *interpreter.ListValue:
			return x
		case *interpreter.MapValue:
			keys := x.Keys()
			out := make([]interpreter.Value, len(keys))
			for i, k := range keys {
				out[i] = &interpreter.StringValue{Val: k}
			}
			return &interpreter.ListValue{Elements: out}
		case *interpreter.StringValue:
			var out []interpreter.Value
			for _, r := range x.Val {
				out = append(out, &interpreter.StringValue{Val: string(r)})
			}
			return &interpreter.ListValue{Elements: out}
		case *interpreter.IntegerValue:
			out := make([]interpreter.Value, 0)
			for i := int64(0); i < x.Val; i++ {
				out = append(out, &interpreter.IntegerValue{Val: i})
			}
			return &interpreter.ListValue{Elements: out}
		}
		return interpreter.NewError(400, "value is not iterable: "+string(args[0].Type()))
	}})
}
