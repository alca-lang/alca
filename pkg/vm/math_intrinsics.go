package vm

import (
	"fmt"
	"math"

	"github.com/alca-lang/alca/pkg/interpreter"
)

// MathBuiltin identifiers are the operands used by OpMathBuiltin.
const (
	MathBuiltinAbs int8 = iota + 1
	MathBuiltinMin
	MathBuiltinMax
	MathBuiltinClamp
	MathBuiltinSqrt
	MathBuiltinRound
	MathBuiltinFloor
	MathBuiltinCeil
)

// MathBuiltinID returns the intrinsic identifier and required arity for a
// supported std.math member.
func MathBuiltinID(name string) (int8, int, bool) {
	switch name {
	case "abs":
		return MathBuiltinAbs, 1, true
	case "min":
		return MathBuiltinMin, 2, true
	case "max":
		return MathBuiltinMax, 2, true
	case "clamp":
		return MathBuiltinClamp, 3, true
	case "sqrt":
		return MathBuiltinSqrt, 1, true
	case "round":
		return MathBuiltinRound, 1, true
	case "floor":
		return MathBuiltinFloor, 1, true
	case "ceil":
		return MathBuiltinCeil, 1, true
	default:
		return 0, 0, false
	}
}

func mathBuiltinArity(id int8) int {
	switch id {
	case MathBuiltinAbs, MathBuiltinSqrt, MathBuiltinRound, MathBuiltinFloor, MathBuiltinCeil:
		return 1
	case MathBuiltinMin, MathBuiltinMax:
		return 2
	case MathBuiltinClamp:
		return 3
	default:
		return 0
	}
}

// MathBuiltinArity returns the number of operands consumed by a std.math
// intrinsic id, or zero for an unknown id.
func MathBuiltinArity(id int8) int {
	return mathBuiltinArity(id)
}

func applyMathBuiltin(id int8, args []interpreter.Value) (interpreter.Value, error) {
	arity := mathBuiltinArity(id)
	if arity == 0 {
		return nil, fmt.Errorf("unknown std.math intrinsic %d", id)
	}
	name := [...]string{"", "abs", "min", "max", "clamp", "sqrt", "round", "floor", "ceil"}[id]
	if len(args) < arity {
		return nil, &RaisedError{Val: interpreter.NewError(400, "math."+name+" received too few arguments")}
	}
	numbers := make([]float64, arity)
	for i := range numbers {
		switch value := args[i].(type) {
		case *interpreter.IntegerValue:
			numbers[i] = float64(value.Val)
		case *interpreter.FloatValue:
			numbers[i] = value.Val
		default:
			return nil, &RaisedError{Val: interpreter.NewError(400, "math."+name+" arguments must be numeric")}
		}
	}

	switch id {
	case MathBuiltinAbs:
		if integer, ok := args[0].(*interpreter.IntegerValue); ok {
			if integer.Val < 0 {
				return &interpreter.IntegerValue{Val: -integer.Val}, nil
			}
			return integer, nil
		}
		return &interpreter.FloatValue{Val: math.Abs(numbers[0])}, nil
	case MathBuiltinMin:
		if numbers[0] < numbers[1] {
			return args[0], nil
		}
		return args[1], nil
	case MathBuiltinMax:
		if numbers[0] > numbers[1] {
			return args[0], nil
		}
		return args[1], nil
	case MathBuiltinClamp:
		value := numbers[0]
		if value < numbers[1] {
			value = numbers[1]
		}
		if value > numbers[2] {
			value = numbers[2]
		}
		if _, ok := args[0].(*interpreter.IntegerValue); ok {
			return &interpreter.IntegerValue{Val: int64(value)}, nil
		}
		return &interpreter.FloatValue{Val: value}, nil
	case MathBuiltinSqrt:
		return &interpreter.FloatValue{Val: math.Sqrt(numbers[0])}, nil
	case MathBuiltinRound:
		return &interpreter.IntegerValue{Val: int64(math.Round(numbers[0]))}, nil
	case MathBuiltinFloor:
		return &interpreter.IntegerValue{Val: int64(math.Floor(numbers[0]))}, nil
	case MathBuiltinCeil:
		return &interpreter.IntegerValue{Val: int64(math.Ceil(numbers[0]))}, nil
	default:
		return nil, fmt.Errorf("unknown std.math intrinsic %d", id)
	}
}

// ApplyMathBuiltin executes a specialized std.math intrinsic for JIT helper
// tiers. It returns an Alca error value instead of a Go error so the bytecode
// VM can raise it through the normal handler chain.
func ApplyMathBuiltin(id int8, args []interpreter.Value) (interpreter.Value, *interpreter.ErrorValue) {
	result, err := applyMathBuiltin(id, args)
	if err == nil {
		return result, nil
	}
	if raised, ok := err.(*RaisedError); ok {
		return nil, raised.Val
	}
	return nil, interpreter.NewError(500, err.Error())
}
