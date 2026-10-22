// Package stdlib provides the Math standard library module for the Alca programming language.
// It supports mathematical operations, constants (PI, E), rounding, clamping, logarithms, and trigonometry.
package stdlib

import (
	"math"

	"github.com/alca-lang/alca/pkg/interpreter"
)

func coerceToFloat(v interpreter.Value) float64 {
	switch n := v.(type) {
	case *interpreter.FloatValue:
		return n.Val
	case *interpreter.IntegerValue:
		return float64(n.Val)
	default:
		return math.NaN()
	}
}

func validateMathNumbers(args []interpreter.Value, count int, name string) interpreter.Value {
	if len(args) < count {
		return interpreter.NewError(400, name+" received too few arguments")
	}
	for _, arg := range args[:count] {
		switch arg.(type) {
		case *interpreter.FloatValue, *interpreter.IntegerValue:
		default:
			return interpreter.NewError(400, name+" arguments must be numeric")
		}
	}
	return nil
}

// RegisterMathBuiltins registers the "std.math" and "math" modules into the interpreter environment.
func RegisterMathBuiltins(env *interpreter.Environment) {
	mathModule := interpreter.NewMap()

	// Constants
	mathModule.Set("PI", &interpreter.FloatValue{Val: math.Pi})
	mathModule.Set("E", &interpreter.FloatValue{Val: math.E})
	mathModule.Set("SQRT2", &interpreter.FloatValue{Val: math.Sqrt2})
	mathModule.Set("LN2", &interpreter.FloatValue{Val: math.Ln2})
	mathModule.Set("LN10", &interpreter.FloatValue{Val: math.Ln10})

	register1 := func(name string, fn func(float64) float64) {
		mathModule.Set(name, &interpreter.BuiltinFunction{
			Name: name,
			Fn: func(args ...interpreter.Value) interpreter.Value {
				if err := validateMathNumbers(args, 1, name); err != nil {
					return err
				}
				x := coerceToFloat(args[0])
				return &interpreter.FloatValue{Val: fn(x)}
			},
		})
	}

	register2 := func(name string, fn func(float64, float64) float64) {
		mathModule.Set(name, &interpreter.BuiltinFunction{
			Name: name,
			Fn: func(args ...interpreter.Value) interpreter.Value {
				if err := validateMathNumbers(args, 2, name); err != nil {
					return err
				}
				x := coerceToFloat(args[0])
				y := coerceToFloat(args[1])
				return &interpreter.FloatValue{Val: fn(x, y)}
			},
		})
	}

	// abs(x) -> int or float
	mathModule.Set("abs", &interpreter.BuiltinFunction{
		Name: "abs",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			if err := validateMathNumbers(args, 1, "math.abs"); err != nil {
				return err
			}
			if iv, ok := args[0].(*interpreter.IntegerValue); ok {
				if iv.Val < 0 {
					return &interpreter.IntegerValue{Val: -iv.Val}
				}
				return iv
			}
			f := coerceToFloat(args[0])
			return &interpreter.FloatValue{Val: math.Abs(f)}
		},
	})

	// sqrt(x)
	register1("sqrt", math.Sqrt)

	// pow(x, y)
	register2("pow", math.Pow)

	// round(x)
	mathModule.Set("round", &interpreter.BuiltinFunction{
		Name: "round",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			if err := validateMathNumbers(args, 1, "math.round"); err != nil {
				return err
			}
			x := coerceToFloat(args[0])
			return &interpreter.IntegerValue{Val: int64(math.Round(x))}
		},
	})

	// floor(x)
	mathModule.Set("floor", &interpreter.BuiltinFunction{
		Name: "floor",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			if err := validateMathNumbers(args, 1, "math.floor"); err != nil {
				return err
			}
			x := coerceToFloat(args[0])
			return &interpreter.IntegerValue{Val: int64(math.Floor(x))}
		},
	})

	// ceil(x)
	mathModule.Set("ceil", &interpreter.BuiltinFunction{
		Name: "ceil",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			if err := validateMathNumbers(args, 1, "math.ceil"); err != nil {
				return err
			}
			x := coerceToFloat(args[0])
			return &interpreter.IntegerValue{Val: int64(math.Ceil(x))}
		},
	})

	// clamp(x, min, max)
	mathModule.Set("clamp", &interpreter.BuiltinFunction{
		Name: "clamp",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			if err := validateMathNumbers(args, 3, "math.clamp"); err != nil {
				return err
			}
			val := coerceToFloat(args[0])
			minVal := coerceToFloat(args[1])
			maxVal := coerceToFloat(args[2])

			if val < minVal {
				val = minVal
			}
			if val > maxVal {
				val = maxVal
			}

			// Preserve integer type if input was integer
			if _, ok := args[0].(*interpreter.IntegerValue); ok {
				return &interpreter.IntegerValue{Val: int64(val)}
			}
			return &interpreter.FloatValue{Val: val}
		},
	})

	// min(a, b)
	mathModule.Set("min", &interpreter.BuiltinFunction{
		Name: "min",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			if err := validateMathNumbers(args, 2, "math.min"); err != nil {
				return err
			}
			a := coerceToFloat(args[0])
			b := coerceToFloat(args[1])
			if a < b {
				return args[0]
			}
			return args[1]
		},
	})

	// max(a, b)
	mathModule.Set("max", &interpreter.BuiltinFunction{
		Name: "max",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			if err := validateMathNumbers(args, 2, "math.max"); err != nil {
				return err
			}
			a := coerceToFloat(args[0])
			b := coerceToFloat(args[1])
			if a > b {
				return args[0]
			}
			return args[1]
		},
	})

	// Logarithms
	register1("log", math.Log)
	register1("log10", math.Log10)
	register1("log2", math.Log2)

	// Trigonometry
	register1("sin", math.Sin)
	register1("cos", math.Cos)
	register1("tan", math.Tan)
	register1("asin", math.Asin)
	register1("acos", math.Acos)
	register1("atan", math.Atan)
	register2("atan2", math.Atan2)

	// Degree / Radian conversion
	mathModule.Set("deg2rad", &interpreter.BuiltinFunction{
		Name: "deg2rad",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			if err := validateMathNumbers(args, 1, "math.deg2rad"); err != nil {
				return err
			}
			deg := coerceToFloat(args[0])
			return &interpreter.FloatValue{Val: deg * (math.Pi / 180.0)}
		},
	})

	mathModule.Set("rad2deg", &interpreter.BuiltinFunction{
		Name: "rad2deg",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			if err := validateMathNumbers(args, 1, "math.rad2deg"); err != nil {
				return err
			}
			rad := coerceToFloat(args[0])
			return &interpreter.FloatValue{Val: rad * (180.0 / math.Pi)}
		},
	})

	// hypot(x, y)
	register2("hypot", math.Hypot)

	env.Set("math", mathModule)
	env.Set("std.math", mathModule)
}
