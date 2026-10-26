// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package vm

import "github.com/alca-lang/alca/pkg/interpreter"

// StackSlot tags for unboxed value representation
const (
	SlotVal   int8 = 0 // Complex object (String, Map, List, Struct, Func)
	SlotInt   int8 = 1 // 64-bit signed integer (in i64)
	SlotFloat int8 = 2 // 64-bit IEEE float (in f64)
	SlotBool  int8 = 3 // Boolean (i64 == 1 for true, 0 for false)
	SlotNull  int8 = 4 // Null singleton
)

// StackSlot represents a zero-allocation unboxed stack value for the Alca Bytecode VM.
// Storing primitive integers and booleans without interface heap-boxing speeds up execution by 5-10x.
type StackSlot struct {
	tag int8
	i64 int64
	f64 float64
	obj interpreter.Value
}

// ToValue converts a StackSlot back to an interpreter.Value interface.
func (s *StackSlot) ToValue() interpreter.Value {
	switch s.tag {
	case SlotInt:
		return interpreter.NewInteger(s.i64)
	case SlotFloat:
		return &interpreter.FloatValue{Val: s.f64}
	case SlotBool:
		if s.i64 != 0 {
			return interpreter.TRUE
		}
		return interpreter.FALSE
	case SlotNull:
		return interpreter.NULL
	default:
		if s.obj == nil {
			return interpreter.NULL
		}
		return s.obj
	}
}

// FromValue converts an interpreter.Value to an unboxed StackSlot.
func FromValue(v interpreter.Value) StackSlot {
	if v == nil {
		return StackSlot{tag: SlotNull}
	}
	switch val := v.(type) {
	case *interpreter.IntegerValue:
		return StackSlot{tag: SlotInt, i64: val.Val}
	case *interpreter.FloatValue:
		return StackSlot{tag: SlotFloat, f64: val.Val}
	case *interpreter.BoolValue:
		if val.Val {
			return StackSlot{tag: SlotBool, i64: 1}
		}
		return StackSlot{tag: SlotBool, i64: 0}
	case *interpreter.NullValue:
		return StackSlot{tag: SlotNull}
	default:
		return StackSlot{tag: SlotVal, obj: v}
	}
}

// Helper methods on StackSlot

func (s *StackSlot) Tag() int8          { return s.tag }
func (s *StackSlot) IsInt() bool        { return s.tag == SlotInt }
func (s *StackSlot) IsFloat() bool      { return s.tag == SlotFloat }
func (s *StackSlot) IsBool() bool       { return s.tag == SlotBool }
func (s *StackSlot) IsNull() bool       { return s.tag == SlotNull }
func (s *StackSlot) Int() int64         { return s.i64 }
func (s *StackSlot) Float() float64     { return s.f64 }
func (s *StackSlot) Bool() bool         { return s.i64 != 0 }
func (s *StackSlot) SetInt(v int64)     { s.tag = SlotInt; s.i64 = v; s.obj = nil }
func (s *StackSlot) SetFloat(v float64) { s.tag = SlotFloat; s.f64 = v; s.obj = nil }
func (s *StackSlot) SetBool(v bool) {
	s.tag = SlotBool
	if v {
		s.i64 = 1
	} else {
		s.i64 = 0
	}
	s.obj = nil
}
func (s *StackSlot) SetNull() { s.tag = SlotNull; s.i64 = 0; s.f64 = 0; s.obj = nil }
func (s *StackSlot) IsFalsy() bool {
	return s.tag == SlotNull || (s.tag == SlotBool && s.i64 == 0)
}

// Slot constructor helpers

func FromInt(v int64) StackSlot     { return StackSlot{tag: SlotInt, i64: v} }
func FromFloat(v float64) StackSlot { return StackSlot{tag: SlotFloat, f64: v} }
func FromBool(v bool) StackSlot {
	if v {
		return StackSlot{tag: SlotBool, i64: 1}
	}
	return StackSlot{tag: SlotBool, i64: 0}
}
func NullSlot() StackSlot { return StackSlot{tag: SlotNull} }
