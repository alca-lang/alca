// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package stdlib

import (
	"math"

	"github.com/alca-lang/alca/pkg/interpreter"
)

// This file holds the scalar std.math helpers, the shared numeric coercion
// helpers and the scalar member registrations. Vector, matrix and quaternion
// members live in math_vector.go, math_matrix.go and math_quat.go.

// coerceToFloat converts an interpreter.Value to float64.
// Returns (value, error) - error is non-nil if the type is invalid.
func coerceToFloat(v interpreter.Value) (float64, interpreter.Value) {
	switch val := v.(type) {
	case *interpreter.FloatValue:
		return val.Val, nil
	case *interpreter.IntegerValue:
		return float64(val.Val), nil
	default:
		// Keep the wording aligned with the VM/JIT math intrinsics so every
		// engine reports the same diagnostic for a non-numeric argument.
		return 0, interpreter.NewError(400, "arguments must be numeric")
	}
}

// coerceToFloatArray converts an interpreter array Value to []float64.
func coerceToFloatArray(v interpreter.Value) []float64 {
	switch val := v.(type) {
	case *interpreter.ListValue:
		result := make([]float64, len(val.Elements))
		for i, elem := range val.Elements {
			f, _ := coerceToFloat(elem)
			result[i] = f
		}
		return result
	default:
		return nil
	}
}

// coerceToFloatSimple converts an interpreter.Value to float64, ignoring errors.
// Used for array elements where we want to default to 0 on error.
func coerceToFloatSimple(v interpreter.Value) float64 {
	f, _ := coerceToFloat(v)
	return f
}

// floatArrayToValue converts []float64 to interpreter.ListValue.
func floatArrayToValue(arr []float64) interpreter.Value {
	elements := make([]interpreter.Value, len(arr))
	for i, v := range arr {
		elements[i] = &interpreter.FloatValue{Val: v}
	}
	return &interpreter.ListValue{Elements: elements}
}

// ---------- Константы ----------

const (
	// Known mathematical constants.
	SQRT3       = 1.7320508075688772
	SQRT1_2     = 0.7071067811865476
	SQRT1_3     = 0.5773502691896258
	TWO_PI      = 6.283185307179586
	PI_2        = 1.5707963267948966
	PI_4        = 0.7853981633974483
	INV_PI      = 0.3183098861837907
	INV_SQRT_PI = 0.5641895835477563
	EULER_GAMMA = 0.5772156649015329
)

// ---------- Базовые функции ----------

// abs возвращает абсолютное значение числа.
func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// sign возвращает знак числа: -1, 0 или 1.
func sign(v float64) int {
	if v > 0 {
		return 1
	} else if v < 0 {
		return -1
	}
	return 0
}

// floor возвращает наибольшее целое число, меньшее или равное v.
func floor(v float64) int64 { return int64(math.Floor(v)) }

// ceil возвращает наименьшее целое число, большее или равное v.
func ceil(v float64) int64 { return int64(math.Ceil(v)) }

// trunc возвращает часть числа v, удалив дробную часть (округление к нулю).
func trunc(v float64) int64 {
	if v >= 0 {
		return int64(math.Trunc(v))
	}
	return int64(math.Ceil(v))
}

// frac возвращает дробную часть числа v.
func frac(v float64) float64 { return v - math.Trunc(v) }

// modf разбивает v на целую и дробную части.
func modf(v float64) (int64, float64) { return int64(math.Trunc(v)), v - math.Trunc(v) }

// ---------- Интерполяция ----------

// lerp возвращает линейную интерполяцию между a и b с весом t: a + t*(b-a).
func lerp(a, b, t float64) float64 { return a + t*(b-a) }

// smoothstep возвращает сглаженное значение от 0 до 1 на основе t:
// f(t) = 3t² - 2t³ для 0 ≤ t ≤ 1.
func smoothstep(a, b, t float64) float64 {
	t = clamp(t, a, b)
	t -= a
	return t * t * (3 - 2*t)
}

// inv_lerp возвращает обратную линейную интерполяцию:
// значение t такое, что lerp(a, b, t) ≈ value. Возвращает NaN если value вне [a,b].
func inv_lerp(a, b, value float64) float64 {
	if a == b {
		return 0
	}
	return (value - a) / (b - a)
}

// ---------- Округление и дробная часть ----------

// round округляет v к ближайшему целому.
func round(v float64) int64 { return int64(math.Round(v)) }

// round_to округляет v до digits знаков после запятой.
func round_to(v float64, digits int) float64 {
	pow10 := math.Pow10(digits)
	return math.Round(v*pow10) / pow10
}

// clamp ограничивает v диапазоном [min, max].
func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// ---------- Числовая теория ----------

// gcd возвращает наибольший общий делитель a и b.
func gcd(a, b int64) int64 {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// lcm возвращает наименьшее общее кратное a и b.
func lcm(a, b int64) int64 { return a / gcd(a, b) * b }

// is_integer проверяет, является ли v целым числом.
func is_integer(v float64) bool { return math.Floor(v) == v && math.Ceil(v) == v }

// is_nan проверяет, является ли v NaN.
func is_nan(v float64) bool { return math.IsNaN(v) }

// is_inf проверяет, является ли v бесконечностью.
func is_inf(v float64) bool { return math.IsInf(v, 0) }

// ---------- Гиперболические функции ----------

// sinh возвращает гиперболический синус от v.
func sinh(v float64) float64 { return math.Sinh(v) }

// cosh возвращает гиперболический косинус от v.
func cosh(v float64) float64 { return math.Cosh(v) }

// tanh возвращает гиперболический тангенс от v.
func tanh(v float64) float64 { return math.Tanh(v) }

// asinh возвращает обратный гиперболический синус от v.
func asinh(v float64) float64 { return math.Asinh(v) }

// acosh возвращает обратный гиперболический косинус от v.
func acosh(v float64) float64 { return math.Acosh(v) }

// atanh возвращает обратный гиперболический тангенс от v.
func atanh(v float64) float64 { return math.Atanh(v) }

// ---------- Тригонometry (расширено) ----------

// csc возвращает cosecant (1/sin) от v.
func csc(v float64) float64 {
	if v == 0 {
		return math.Inf(1)
	}
	return 1 / math.Sin(v)
}

// sec возвращает secant (1/cos) от v.
func sec(v float64) float64 {
	if v == math.Pi/2 {
		return math.Inf(1)
	}
	return 1 / math.Cos(v)
}

// cot возвращает cotangent (1/tan) от v.
func cot(v float64) float64 {
	t := math.Tan(v)
	if t == 0 {
		return math.Inf(1)
	}
	return 1 / t
}

// acsc возвращает arcosecant от v.
func acsc(v float64) float64 {
	if v == 0 {
		return math.Pi / 2
	}
	return math.Asin(1 / v)
}

// asec возвращает arcasecant от v.
func asec(v float64) float64 {
	if v == 1 {
		return 0
	}
	return math.Acos(1 / v)
}

// acot возвращает arccotangent от v.
func acot(v float64) float64 { return math.Atan(1 / v) }

// ---------- Конверсия углов ----------

// grad2rad конвертирует grad в radians (grad = 90° = pi/2).
func grad2rad(v float64) float64 { return v * (math.Pi / 200) }

// rad2grad конвертирует radians в grad.
func rad2grad(v float64) float64 { return v * (200 / math.Pi) }

// deg2grad конвертирует градусы в grad.
func deg2grad(v float64) float64 { return v * (10 / 9) }

// grad2deg конвертирует grad в градусы.
func grad2deg(v float64) float64 { return v * (9 / 10) }

// deg2rad конвертирует градусы в radians.
func deg2rad(v float64) float64 { return v * math.Pi / 180 }

// rad2deg конвертирует radians в градусы.
func rad2deg(v float64) float64 { return v * 180 / math.Pi }

// ---------- Мощность и логарифм ----------

// log1p возвращает ln(1 + v), более точное для малых v.
func log1p(v float64) float64 { return math.Log1p(v) }

// expm1 возвращает e^v - 1, более точное для малых v.
func expm1(v float64) float64 { return math.Expm1(v) }

// hypot возвращает гипотенузу sqrt(a*a + b*b) без переполнения.
func hypot(a, b float64) float64 { return math.Hypot(a, b) }

// ---------- Дополнительные функции ----------

// factorial возвращает факториал n (n!).
func factorial(n int64) int64 {
	if n < 0 {
		return 0
	}
	r := int64(1)
	for i := int64(2); i <= n; i++ {
		r *= i
	}
	return r
}

// doublefactorial возвращает двойной факториал n!!.
func doublefactorial(n int64) int64 {
	if n <= 0 {
		return 1
	}
	r := int64(1)
	for ; n > 0; n -= 2 {
		r *= n
	}
	return r
}

// binomial возвращает биномиальный коэффициент C(n, k).
func binomial(n, k int64) int64 {
	if k < 0 || k > n {
		return 0
	}
	if k > n-k {
		k = n - k
	}
	c := int64(1)
	for i := int64(0); i < k; i++ {
		c = c * (n - i) / (i + 1)
	}
	return c
}

// clamp01 ограничивает v диапазоном [0, 1].
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// min_max возвращает пару (min, max) из a и b.
func min_max(a, b float64) (float64, float64) {
	if a < b {
		return a, b
	}
	return b, a
}

// RegisterMathBuiltins registers the "std.math" and "math" modules into the interpreter environment.
func RegisterMathBuiltins(env *interpreter.Environment) {
	mathModule := interpreter.NewMap()
	registerMathScalarBuiltins(mathModule)
	registerMathMatrixBuiltins(mathModule)
	registerMathQuatBuiltins(mathModule)
	registerMathVectorBuiltins(mathModule)
	env.Set("math", mathModule)
	env.Set("std.math", mathModule)
}

// registerMathScalarBuiltins registers the scalar constants and functions. The
// integer-preserving behaviour of abs/min/max/clamp/round/floor/ceil matches the
// VM and JIT math intrinsics (see vm.MathBuiltin) so every engine agrees.
func registerMathScalarBuiltins(mathModule *interpreter.MapValue) {
	// Constants
	mathModule.Set("PI", &interpreter.FloatValue{Val: math.Pi})
	mathModule.Set("E", &interpreter.FloatValue{Val: math.E})
	mathModule.Set("SQRT2", &interpreter.FloatValue{Val: math.Sqrt2})
	mathModule.Set("SQRT3", &interpreter.FloatValue{Val: SQRT3})
	mathModule.Set("SQRT1_2", &interpreter.FloatValue{Val: SQRT1_2})
	mathModule.Set("SQRT1_3", &interpreter.FloatValue{Val: SQRT1_3})
	mathModule.Set("TWO_PI", &interpreter.FloatValue{Val: TWO_PI})
	mathModule.Set("PI_2", &interpreter.FloatValue{Val: PI_2})
	mathModule.Set("PI_4", &interpreter.FloatValue{Val: PI_4})
	mathModule.Set("INV_PI", &interpreter.FloatValue{Val: INV_PI})
	mathModule.Set("INV_SQRT_PI", &interpreter.FloatValue{Val: INV_SQRT_PI})
	mathModule.Set("EULER_GAMMA", &interpreter.FloatValue{Val: EULER_GAMMA})

	// 1-arg functions - wrapper that converts Value to float64 then calls the function
	register1 := func(name string, fn func(float64) float64) {
		mathModule.Set(name, &interpreter.BuiltinFunction{Name: name, Fn: func(args ...interpreter.Value) interpreter.Value {
			if len(args) < 1 {
				return interpreter.NewError(400, "math."+name+" received too few arguments")
			}
			x, err := coerceToFloat(args[0])
			if err != nil {
				return err
			}
			return &interpreter.FloatValue{Val: fn(x)}
		}})
	}
	register2 := func(name string, fn func(float64, float64) float64) {
		mathModule.Set(name, &interpreter.BuiltinFunction{Name: name, Fn: func(args ...interpreter.Value) interpreter.Value {
			if len(args) < 2 {
				return interpreter.NewError(400, "math."+name+" received too few arguments")
			}
			x, err := coerceToFloat(args[0])
			if err != nil {
				return err
			}
			y, err := coerceToFloat(args[1])
			if err != nil {
				return err
			}
			return &interpreter.FloatValue{Val: fn(x, y)}
		}})
	}
	register3 := func(name string, fn func(float64, float64, float64) float64) {
		mathModule.Set(name, &interpreter.BuiltinFunction{Name: name, Fn: func(args ...interpreter.Value) interpreter.Value {
			if len(args) < 3 {
				return interpreter.NewError(400, "math."+name+" received too few arguments")
			}
			x, err := coerceToFloat(args[0])
			if err != nil {
				return err
			}
			y, err := coerceToFloat(args[1])
			if err != nil {
				return err
			}
			z, err := coerceToFloat(args[2])
			if err != nil {
				return err
			}
			return &interpreter.FloatValue{Val: fn(x, y, z)}
		}})
	}
	registerBool := func(name string, fn func(float64) bool) {
		mathModule.Set(name, &interpreter.BuiltinFunction{Name: name, Fn: func(args ...interpreter.Value) interpreter.Value {
			if len(args) < 1 {
				return interpreter.NewError(400, "math."+name+" received too few arguments")
			}
			x, err := coerceToFloat(args[0])
			if err != nil {
				return err
			}
			return &interpreter.BoolValue{Val: fn(x)}
		}})
	}

	// abs preserves integer-ness: abs(int) -> int, abs(float) -> float.
	mathModule.Set("abs", &interpreter.BuiltinFunction{Name: "abs", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "math.abs received too few arguments")
		}
		if integer, ok := args[0].(*interpreter.IntegerValue); ok {
			if integer.Val < 0 {
				return &interpreter.IntegerValue{Val: -integer.Val}
			}
			return integer
		}
		x, err := coerceToFloat(args[0])
		if err != nil {
			return err
		}
		return &interpreter.FloatValue{Val: abs(x)}
	}})

	// sign, floor, ceil, trunc, round return int64
	mathModule.Set("sign", &interpreter.BuiltinFunction{Name: "sign", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "math.sign received too few arguments")
		}
		x, err := coerceToFloat(args[0])
		if err != nil {
			return err
		}
		return &interpreter.IntegerValue{Val: int64(sign(x))}
	}})
	mathModule.Set("floor", &interpreter.BuiltinFunction{Name: "floor", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "math.floor received too few arguments")
		}
		x, err := coerceToFloat(args[0])
		if err != nil {
			return err
		}
		return &interpreter.IntegerValue{Val: floor(x)}
	}})
	mathModule.Set("ceil", &interpreter.BuiltinFunction{Name: "ceil", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "math.ceil received too few arguments")
		}
		x, err := coerceToFloat(args[0])
		if err != nil {
			return err
		}
		return &interpreter.IntegerValue{Val: ceil(x)}
	}})
	mathModule.Set("trunc", &interpreter.BuiltinFunction{Name: "trunc", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "math.trunc received too few arguments")
		}
		x, err := coerceToFloat(args[0])
		if err != nil {
			return err
		}
		return &interpreter.IntegerValue{Val: trunc(x)}
	}})
	mathModule.Set("round", &interpreter.BuiltinFunction{Name: "round", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "math.round received too few arguments")
		}
		x, err := coerceToFloat(args[0])
		if err != nil {
			return err
		}
		return &interpreter.IntegerValue{Val: round(x)}
	}})
	register1("frac", frac)
	register1("modf", func(v float64) float64 { _, f := modf(v); return f })
	register1("log", math.Log)
	register1("log10", math.Log10)
	register1("log1p", log1p)
	register1("exp", math.Exp)
	register1("exp_m1", expm1)
	register1("sqrt", math.Sqrt)
	register1("cbrt", func(v float64) float64 { return math.Cbrt(v) })
	register1("sin", math.Sin)
	register1("cos", math.Cos)
	register1("tan", math.Tan)
	register1("asin", math.Asin)
	register1("acos", math.Acos)
	register1("atan", math.Atan)
	register1("sinh", sinh)
	register1("cosh", cosh)
	register1("tanh", tanh)
	register1("asinh", asinh)
	register1("acosh", acosh)
	register1("atanh", atanh)
	register1("csc", csc)
	register1("sec", sec)
	register1("cot", cot)
	register1("acsc", acsc)
	register1("asec", asec)
	register1("acot", acot)
	register1("deg2rad", deg2rad)
	register1("rad2deg", rad2deg)
	register1("grad2rad", grad2rad)
	register1("rad2grad", rad2grad)
	register1("grad2deg", grad2deg)
	register1("factorial", func(n float64) float64 { return float64(factorial(int64(n))) })
	register1("doublefactorial", func(n float64) float64 { return float64(doublefactorial(int64(n))) })
	register1("clamp01", clamp01)
	// min_max returns two values; expose first via wrapper
	register1("min_max", func(v float64) float64 { m, _ := min_max(v, 0); return m })

	// 2-arg functions list
	register2("pow", math.Pow)
	register2("hypot", hypot)
	register2("atan2", math.Atan2)
	register2("gcd", func(a, b float64) float64 { return float64(gcd(int64(a), int64(b))) })
	register2("lcm", func(a, b float64) float64 { return float64(lcm(int64(a), int64(b))) })

	// min and max return the chosen operand unchanged, preserving its type:
	// min(2, 3.5) -> 2 (int), max(2, 3.5) -> 3.5 (float).
	mathModule.Set("min", &interpreter.BuiltinFunction{Name: "min", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "math.min received too few arguments")
		}
		x, err := coerceToFloat(args[0])
		if err != nil {
			return err
		}
		y, err := coerceToFloat(args[1])
		if err != nil {
			return err
		}
		if x < y {
			return args[0]
		}
		return args[1]
	}})
	mathModule.Set("max", &interpreter.BuiltinFunction{Name: "max", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "math.max received too few arguments")
		}
		x, err := coerceToFloat(args[0])
		if err != nil {
			return err
		}
		y, err := coerceToFloat(args[1])
		if err != nil {
			return err
		}
		if x > y {
			return args[0]
		}
		return args[1]
	}})

	register2("remainder", math.Remainder)
	register2("dim", math.Dim)
	register2("copysign", math.Copysign)
	// lerp with default t=0.5 for 2-arg form
	register2("lerp", func(a, b float64) float64 { return lerp(a, b, 0.5) })
	// smoothstep with default t=0 for 2-arg form (clamp a to [0,b] first)
	register2("smoothstep", func(a, b float64) float64 { t := clamp(0, a, b); return smoothstep(a, b, t) })
	// inv_lerp default range midpoint
	register2("inv_lerp", func(a, b float64) float64 { return inv_lerp(a, b, (a+b)/2) })

	// 3-arg functions list
	mathModule.Set("clamp", &interpreter.BuiltinFunction{Name: "clamp", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 3 {
			return interpreter.NewError(400, "math.clamp received too few arguments")
		}
		// Try to use integer types if all args are integers
		aInt, aOk := args[0].(*interpreter.IntegerValue)
		minInt, minOk := args[1].(*interpreter.IntegerValue)
		maxInt, maxOk := args[2].(*interpreter.IntegerValue)
		if aOk && minOk && maxOk {
			v := aInt.Val
			min := minInt.Val
			max := maxInt.Val
			if v < min {
				v = min
			}
			if v > max {
				v = max
			}
			return &interpreter.IntegerValue{Val: v}
		}
		// Otherwise use float
		x, err := coerceToFloat(args[0])
		if err != nil {
			return err
		}
		min, err := coerceToFloat(args[1])
		if err != nil {
			return err
		}
		max, err := coerceToFloat(args[2])
		if err != nil {
			return err
		}
		return &interpreter.FloatValue{Val: clamp(x, min, max)}
	}})
	register3("lerp", lerp)
	register3("smoothstep", smoothstep)
	register3("inv_lerp", inv_lerp)
	register3("fma", math.FMA)
	// round_to takes (v float64, digits int) - special case
	mathModule.Set("round_to", &interpreter.BuiltinFunction{Name: "round_to", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "math.round_to received too few arguments")
		}
		v, err := coerceToFloat(args[0])
		if err != nil {
			return err
		}
		digits := 0
		if intVal, ok := args[1].(*interpreter.IntegerValue); ok {
			digits = int(intVal.Val)
		}
		return &interpreter.FloatValue{Val: round_to(v, digits)}
	}})

	// Predicate functions
	registerBool("is_integer", is_integer)
	registerBool("is_nan", is_nan)
	registerBool("is_inf", is_inf)

	// Binomial coefficient
	register2("binomial", func(n, k float64) float64 { return float64(binomial(int64(n), int64(k))) })
}
