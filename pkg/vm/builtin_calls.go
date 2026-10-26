// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package vm

import (
	"fmt"

	"github.com/alca-lang/alca/pkg/interpreter"
)

// Identifiers for the always-available, single-argument builtins the compiler
// lowers to OpCallBuiltin1. Lowering is only applied to a bare-identifier call
// that does not resolve to a user binding, so the runtime name lookup the VM
// would otherwise perform always selects the real builtin.
const (
	builtin1Int int8 = iota + 1
	builtin1Float
	builtin1Bool
	builtin1Len
)

// builtin1Names maps an OpCallBuiltin1 id back to the builtin's registry name,
// used only by the (rare) fallback that runs the real builtin.
var builtin1Names = [...]string{
	builtin1Int:   "int",
	builtin1Float: "float",
	builtin1Bool:  "bool",
	builtin1Len:   "len",
}

// builtin1ID reports the OpCallBuiltin1 id for a builtin name, if specialised.
func builtin1ID(name string) (int, bool) {
	switch name {
	case "int":
		return int(builtin1Int), true
	case "float":
		return int(builtin1Float), true
	case "bool":
		return int(builtin1Bool), true
	case "len":
		return int(builtin1Len), true
	}
	return 0, false
}

// ApplyBuiltin1 evaluates a specialised single-argument builtin directly on an
// unboxed stack slot, avoiding the interpreter-environment lookup (which takes a
// mutex) and the argument boxing/varargs of a generic builtin call. It reports
// whether the slot was fully handled; when it returns false the caller must run
// CallBuiltin1Fallback for exact semantics (e.g. float("3.5")).
//
// It is shared by the VM and the JIT runtime-helper tier.
func ApplyBuiltin1(id int8, slot *StackSlot) bool {
	switch id {
	case builtin1Float:
		switch slot.tag {
		case SlotFloat:
			return true
		case SlotInt:
			slot.f64 = float64(slot.i64)
			slot.tag = SlotFloat
			return true
		case SlotBool:
			if slot.i64 != 0 {
				slot.f64 = 1
			} else {
				slot.f64 = 0
			}
			slot.tag = SlotFloat
			return true
		}
		// null and non-numeric values are conversion errors in the interpreter,
		// so leave them to the fallback for identical errors.

	case builtin1Int:
		switch slot.tag {
		case SlotInt:
			return true
		case SlotFloat:
			slot.i64 = int64(slot.f64)
			slot.tag = SlotInt
			return true
		case SlotBool:
			slot.tag = SlotInt
			return true
		}
		// null and non-numeric values are conversion errors in the interpreter,
		// so leave them to the fallback for identical errors.

	case builtin1Bool:
		switch slot.tag {
		case SlotBool:
			return true
		case SlotInt:
			if slot.i64 != 0 {
				slot.i64 = 1
			} else {
				slot.i64 = 0
			}
			slot.tag = SlotBool
			return true
		case SlotFloat:
			if slot.f64 != 0 {
				slot.i64 = 1
			} else {
				slot.i64 = 0
			}
			slot.tag = SlotBool
			return true
		case SlotNull:
			slot.i64 = 0
			slot.tag = SlotBool
			return true
		default:
			truthy := interpreter.IsTruthy(slot.obj)
			slot.obj = nil
			if truthy {
				slot.i64 = 1
			} else {
				slot.i64 = 0
			}
			slot.tag = SlotBool
			return true
		}

	case builtin1Len:
		var n int64
		if slot.tag == SlotVal {
			switch v := slot.obj.(type) {
			case *interpreter.StringValue:
				n = int64(len(v.RuneSlice()))
			case *interpreter.ListValue:
				n = int64(v.Len())
			case *interpreter.MapValue:
				n = int64(len(v.Pairs))
			case *interpreter.BlobValue:
				n = int64(len(v.Data))
			}
		}
		slot.obj = nil
		slot.i64 = n
		slot.tag = SlotInt
		return true
	}

	return false
}

// CallBuiltin1Fallback runs the real builtin for the cases the slot-level fast
// path does not cover, preserving the interpreter's exact behaviour and errors.
// It is shared by the VM and the JIT runtime-helper tier.
func CallBuiltin1Fallback(id int8, arg StackSlot) (StackSlot, error) {
	fn, ok := interpreter.BuiltinFunctionMap[builtin1Names[id]]
	if !ok {
		return StackSlot{}, fmt.Errorf("unknown builtin %q", builtin1Names[id])
	}
	res := fn.Fn(arg.ToValue())
	if ev, ok := res.(*interpreter.ErrorValue); ok {
		return StackSlot{}, &RaisedError{Val: ev}
	}
	return FromValue(res), nil
}
