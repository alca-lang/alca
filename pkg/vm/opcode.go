// Package vm implements a high-performance register/stack-based bytecode virtual machine
// and optimizing compiler for the Alca programming language.
package vm

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// Instructions represents a sequence of compiled bytecode instructions.
type Instructions []byte

// Opcode represents a single-byte virtual machine instruction identifier.
type Opcode byte

const (
	OpConstant Opcode = iota // Load constant from pool: [OpConstant, index_hi, index_lo]
	OpPop                    // Pop and discard top of stack
	OpDup                    // Duplicate value on top of stack

	// Arithmetic
	OpAdd // Add top two values
	OpSub // Subtract top from second
	OpMul // Multiply top two values
	OpDiv // Divide second by top
	OpMod // Modulo
	OpPow // Power (**)

	// Comparisons
	OpEqual        // ==
	OpNotEqual     // !=
	OpGreaterThan  // >
	OpGreaterEqual // >=
	OpLessThan     // <
	OpLessEqual    // <=
	OpIn           // membership: left in right

	// Boolean & Unary
	OpTrue   // Push true
	OpFalse  // Push false
	OpNull   // Push null
	OpNot    // Logical not
	OpNegate // Arithmetic negation (-)

	// Control Flow
	OpJump        // Unconditional jump to offset (2 bytes)
	OpJumpIfFalse // Jump to offset if top of stack is falsy (2 bytes)

	// Variable Access
	OpGetGlobal  // Load global variable (2 bytes index)
	OpSetGlobal  // Store global variable (2 bytes index)
	OpGetLocal   // Load local variable by frame offset (1 byte index)
	OpSetLocal   // Store local variable by frame offset (1 byte index)
	OpGetBuiltin // Load builtin function or stdlib module (2 bytes index)
	OpClosure    // Create a closure over a compiled function constant (2 bytes const index)
	OpGetFree    // Load captured upvalue (1 byte upvalue index)
	OpSetFree    // Store captured upvalue (1 byte upvalue index)

	// Data Structures
	OpList     // Construct list from N elements on stack (2 bytes count)
	OpMap      // Construct map from N key-value pairs on stack (2 bytes count)
	OpIndex    // Read container element: target[index]
	OpSetIndex // Assign container element: target[index] = value
	OpGetField // Access object/struct field: target.field (2 bytes name constant index)
	OpSetField // Assign object/struct field: target.field = value (2 bytes name constant index)

	// Functions & Calls
	OpCall        // Invoke function with N arguments (1 byte arg count)
	OpReturnValue // Return with value on top of stack
	OpReturn      // Return with null

	// Alca Stream Redirection
	OpStream          // Execute stream operator: source OP target (1 byte stream op code)
	OpUse             // Execute module import use statement (2 bytes constant index)
	OpRange           // Create integer range list from start..end
	OpIncGlobal       // Increment global integer by 1 in-place (2 bytes index)
	OpIncLocal        // Increment local integer by 1 in-place (1 byte index)
	OpAddGlobal       // Add top of stack to global integer in-place (2 bytes index)
	OpAddLocal        // Add top of stack to local integer in-place (1 byte index)
	OpSubGlobal       // Subtract top of stack from global integer in-place (2 bytes index)
	OpSubLocal        // Subtract top of stack from local integer in-place (1 byte index)
	OpLoopGlobalLtInc // Inc global int and loop if < const (2 bytes var, 2 bytes const, 2 bytes target)
	OpLoopGlobalLeInc // Inc global int and loop if <= end global (2 bytes var, 2 bytes end var, 2 bytes target)
	OpModConst        // In-place stack[sp-1] %= constant[idx] (2 bytes const index)
	OpMulConst        // In-place stack[sp-1] *= constant[idx] (2 bytes const index)
	OpAddConst        // In-place stack[sp-1] += constant[idx] (2 bytes const index)

	// Error handling & testing
	OpNop             // Does nothing (used as a jump landing pad)
	OpTryBegin        // Install an error handler: [OpTryBegin, handler_hi, handler_lo]
	OpEndTry          // Remove the innermost error handler (normal completion)
	OpMakeError       // Pop code, pop message, push error value
	OpRaise           // Pop an error value and unwind to the nearest handler
	OpAssert          // Pop condition; raise "assertion failed" (2 bytes message constant) when false
	OpTestPass        // Report a passing test (2 bytes name constant)
	OpTestFail        // Pop error, report failing test (2 bytes name constant)
	OpCallBuiltin1    // Call an always-available 1-argument conversion builtin (1 byte id), no boxing
	OpIsInt           // Replace top of stack with true if it holds an int, else false
	OpLoopRangeLocal  // Range-loop back edge on locals: var+=step, jump to target while in range (1,1,1,2)
	OpLoopRangeGlobal // Range-loop back edge on globals: var+=step, jump to target while in range (2,2,2,2)
	OpLoopListLocal   // List-iteration back edge on locals: idx++, load iter[idx] into var, jump to target while idx < len (1,1,1,1,2)
	OpLoopListGlobal  // List-iteration back edge on globals: idx++, load iter[idx] into var, jump to target while idx < len (2,2,2,2,2)
	OpAddLocalLocal   // local[dst] += local[src] (1 byte dst, 1 byte src)
	OpAddGlobalGlobal // global[dst] += global[src] (2 bytes dst, 2 bytes src)
	OpAddLocalConst   // push local[src] + constant[idx] (1 byte src, 2 bytes const)
	OpMulLocalConst   // push local[src] * constant[idx] (1 byte src, 2 bytes const)
	OpModLocalConst   // push local[src] % constant[idx] (1 byte src, 2 bytes const)
	OpConvLocal       // push builtin1 conversion of local[src] (1 byte conversion id, 1 byte src)
	OpConvGlobal      // push builtin1 conversion of global[src] (1 byte conversion id, 2 bytes src)
	OpIter            // Replace the top of stack with the iterable form of a value
	OpListLen         // Replace the top of stack with the length of a list
	OpWait            // Wait for all background streaming / tasks to complete
	OpLoopLocalLtInc  // Inc local int and loop if < const (1 byte var, 2 bytes const, 2 bytes target)
	OpLoopLocalLeInc  // Inc local int and loop if <= const (1 byte var, 2 bytes const, 2 bytes target)
	OpStrictEqual     // Type-sensitive equality comparison
	OpMathBuiltin     // Apply a specialized std.math numeric builtin (1 byte id)
	OpStdlibCall      // Call a pure stdlib module member (2 bytes alias, 2 bytes member path, 1 byte argc)
	OpMakeClass       // Build a class definition (2 bytes template, 2 bytes base class global + 1)
	OpApplyDecorator  // Apply a decorator to the program (2 bytes decorator spec)
	OpTxBegin         // Start a transaction: snapshot the local slots of the frame
	OpTxEnd           // Commit a transaction: discard the snapshot
)

// Definition describes an opcode's human-readable name and operand byte widths.
type Definition struct {
	Name          string
	OperandWidths []int
}

var definitions = map[Opcode]*Definition{
	OpConstant:        {"OpConstant", []int{2}},
	OpPop:             {"OpPop", []int{}},
	OpDup:             {"OpDup", []int{}},
	OpAdd:             {"OpAdd", []int{}},
	OpSub:             {"OpSub", []int{}},
	OpMul:             {"OpMul", []int{}},
	OpDiv:             {"OpDiv", []int{}},
	OpMod:             {"OpMod", []int{}},
	OpPow:             {"OpPow", []int{}},
	OpEqual:           {"OpEqual", []int{}},
	OpNotEqual:        {"OpNotEqual", []int{}},
	OpGreaterThan:     {"OpGreaterThan", []int{}},
	OpGreaterEqual:    {"OpGreaterEqual", []int{}},
	OpLessThan:        {"OpLessThan", []int{}},
	OpLessEqual:       {"OpLessEqual", []int{}},
	OpIn:              {"OpIn", []int{}},
	OpTrue:            {"OpTrue", []int{}},
	OpFalse:           {"OpFalse", []int{}},
	OpNull:            {"OpNull", []int{}},
	OpNot:             {"OpNot", []int{}},
	OpNegate:          {"OpNegate", []int{}},
	OpJump:            {"OpJump", []int{2}},
	OpJumpIfFalse:     {"OpJumpIfFalse", []int{2}},
	OpGetGlobal:       {"OpGetGlobal", []int{2}},
	OpSetGlobal:       {"OpSetGlobal", []int{2}},
	OpGetLocal:        {"OpGetLocal", []int{1}},
	OpSetLocal:        {"OpSetLocal", []int{1}},
	OpGetBuiltin:      {"OpGetBuiltin", []int{2}},
	OpClosure:         {"OpClosure", []int{2}},
	OpGetFree:         {"OpGetFree", []int{1}},
	OpSetFree:         {"OpSetFree", []int{1}},
	OpList:            {"OpList", []int{2}},
	OpMap:             {"OpMap", []int{2}},
	OpIndex:           {"OpIndex", []int{}},
	OpSetIndex:        {"OpSetIndex", []int{}},
	OpGetField:        {"OpGetField", []int{2}},
	OpSetField:        {"OpSetField", []int{2}},
	OpCall:            {"OpCall", []int{1}},
	OpReturnValue:     {"OpReturnValue", []int{}},
	OpReturn:          {"OpReturn", []int{}},
	OpStream:          {"OpStream", []int{1}},
	OpUse:             {"OpUse", []int{2}},
	OpRange:           {"OpRange", []int{}},
	OpIncGlobal:       {"OpIncGlobal", []int{2}},
	OpIncLocal:        {"OpIncLocal", []int{1}},
	OpAddGlobal:       {"OpAddGlobal", []int{2}},
	OpAddLocal:        {"OpAddLocal", []int{1}},
	OpSubGlobal:       {"OpSubGlobal", []int{2}},
	OpSubLocal:        {"OpSubLocal", []int{1}},
	OpLoopGlobalLtInc: {"OpLoopGlobalLtInc", []int{2, 2, 2}},
	OpLoopGlobalLeInc: {"OpLoopGlobalLeInc", []int{2, 2, 2}},
	OpNop:             {"OpNop", []int{}},
	OpTryBegin:        {"OpTryBegin", []int{2}},
	OpEndTry:          {"OpEndTry", []int{}},
	OpMakeError:       {"OpMakeError", []int{}},
	OpRaise:           {"OpRaise", []int{}},
	OpAssert:          {"OpAssert", []int{2}},
	OpTestPass:        {"OpTestPass", []int{2}},
	OpTestFail:        {"OpTestFail", []int{2}},
	OpModConst:        {"OpModConst", []int{2}},
	OpMulConst:        {"OpMulConst", []int{2}},
	OpAddConst:        {"OpAddConst", []int{2}},
	OpCallBuiltin1:    {"OpCallBuiltin1", []int{1}},
	OpIsInt:           {"OpIsInt", []int{}},
	OpLoopRangeLocal:  {"OpLoopRangeLocal", []int{1, 1, 1, 2}},
	OpLoopRangeGlobal: {"OpLoopRangeGlobal", []int{2, 2, 2, 2}},
	OpLoopListLocal:   {"OpLoopListLocal", []int{1, 1, 1, 1, 2}},
	OpLoopListGlobal:  {"OpLoopListGlobal", []int{2, 2, 2, 2, 2}},
	OpAddLocalLocal:   {"OpAddLocalLocal", []int{1, 1}},
	OpAddGlobalGlobal: {"OpAddGlobalGlobal", []int{2, 2}},
	OpAddLocalConst:   {"OpAddLocalConst", []int{1, 2}},
	OpMulLocalConst:   {"OpMulLocalConst", []int{1, 2}},
	OpModLocalConst:   {"OpModLocalConst", []int{1, 2}},
	OpConvLocal:       {"OpConvLocal", []int{1, 1}},
	OpConvGlobal:      {"OpConvGlobal", []int{1, 2}},
	OpIter:            {"OpIter", []int{}},
	OpListLen:         {"OpListLen", []int{}},
	OpWait:            {"OpWait", []int{}},
	OpLoopLocalLtInc:  {"OpLoopLocalLtInc", []int{1, 2, 2}},
	OpLoopLocalLeInc:  {"OpLoopLocalLeInc", []int{1, 1, 2}},
	OpStrictEqual:     {"OpStrictEqual", []int{}},
	OpMathBuiltin:     {"OpMathBuiltin", []int{1}},
	OpStdlibCall:      {"OpStdlibCall", []int{2, 2, 1}},
	OpMakeClass:       {"OpMakeClass", []int{2, 2}},
	OpApplyDecorator:  {"OpApplyDecorator", []int{2}},
	OpTxBegin:         {"OpTxBegin", []int{}},
	OpTxEnd:           {"OpTxEnd", []int{}},
}

// Lookup returns the Definition for the given Opcode.
func Lookup(op byte) (*Definition, error) {
	def, ok := definitions[Opcode(op)]
	if !ok {
		return nil, fmt.Errorf("opcode %d undefined", op)
	}
	return def, nil
}

// Make encodes an Opcode and its integer operands into raw instruction bytes.
// It panics for invalid operands; compiler code should use MakeChecked instead
// so malformed programs are reported as compilation errors.
func Make(op Opcode, operands ...int) []byte {
	instruction, err := MakeChecked(op, operands...)
	if err != nil {
		panic(err)
	}
	return instruction
}

// MakeChecked encodes an instruction, rejecting operands that do not fit their
// declared widths instead of silently truncating them.
func MakeChecked(op Opcode, operands ...int) ([]byte, error) {
	def, ok := definitions[op]
	if !ok {
		return nil, fmt.Errorf("unknown opcode %d", op)
	}
	if len(operands) != len(def.OperandWidths) {
		return nil, fmt.Errorf("%s expects %d operands, got %d", def.Name, len(def.OperandWidths), len(operands))
	}

	instructionLen := 1
	for _, w := range def.OperandWidths {
		instructionLen += w
	}

	instruction := make([]byte, instructionLen)
	instruction[0] = byte(op)

	offset := 1
	for i, o := range operands {
		width := def.OperandWidths[i]
		switch width {
		case 2:
			if o < 0 || o > 0xFFFF {
				return nil, fmt.Errorf("%s operand %d value %d does not fit in 16 bits", def.Name, i+1, o)
			}
			binary.BigEndian.PutUint16(instruction[offset:], uint16(o))
		case 1:
			if o < 0 || o > 0xFF {
				return nil, fmt.Errorf("%s operand %d value %d does not fit in 8 bits", def.Name, i+1, o)
			}
			instruction[offset] = byte(o)
		default:
			return nil, fmt.Errorf("%s has unsupported operand width %d", def.Name, width)
		}
		offset += width
	}

	return instruction, nil
}

// ReadOperands decodes the operands of an instruction according to its definition.
func ReadOperands(def *Definition, ins Instructions) ([]int, int) {
	operands := make([]int, len(def.OperandWidths))
	offset := 0

	for i, width := range def.OperandWidths {
		switch width {
		case 2:
			operands[i] = int(binary.BigEndian.Uint16(ins[offset:]))
		case 1:
			operands[i] = int(ins[offset])
		}
		offset += width
	}

	return operands, offset
}

// String produces a human-readable bytecode disassembly.
func (ins Instructions) String() string {
	var out strings.Builder

	i := 0
	for i < len(ins) {
		def, err := Lookup(ins[i])
		if err != nil {
			fmt.Fprintf(&out, "ERROR: %s\n", err)
			i++
			continue
		}

		operands, read := ReadOperands(def, ins[i+1:])
		fmt.Fprintf(&out, "%04d %s", i, def.Name)
		for _, o := range operands {
			fmt.Fprintf(&out, " %d", o)
		}
		out.WriteString("\n")

		i += 1 + read
	}

	return out.String()
}
