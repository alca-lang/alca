// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package stdlib

import (
	"math"

	"github.com/alca-lang/alca/pkg/interpreter"
)

// This file holds the fixed-size vector helpers and their std.math
// registrations.

// vec2 создает 2D-вектор.
func vec2(x, y float64) [2]float64 { return [2]float64{x, y} }

// vec3 создает 3D-вектор.
func vec3(x, y, z float64) [3]float64 { return [3]float64{x, y, z} }

// vec4 создает 4D-вектор.
func vec4(x, y, z, w float64) [4]float64 { return [4]float64{x, y, z, w} }

// vec2_len возвращает длину вектора v.
func vec2_len(v [2]float64) float64 { return math.Sqrt(v[0]*v[0] + v[1]*v[1]) }

// vec3_len возвращает длину вектора v.
func vec3_len(v [3]float64) float64 { return math.Sqrt(v[0]*v[0] + v[1]*v[1] + v[2]*v[2]) }

// vec4_len возвращает длину вектора v.
func vec4_len(v [4]float64) float64 { return math.Sqrt(v[0]*v[0] + v[1]*v[1] + v[2]*v[2] + v[3]*v[3]) }

// vec2_dot возвращает скалярное произведение векторов a и b.
func vec2_dot(a, b [2]float64) float64 { return a[0]*b[0] + a[1]*b[1] }

// vec3_dot возвращает скалярное произведение векторов a и b.
func vec3_dot(a, b [3]float64) float64 { return a[0]*b[0] + a[1]*b[1] + a[2]*b[2] }

// vec4_dot возвращает скалярное произведение векторов a и b.
func vec4_dot(a, b [4]float64) float64 { return a[0]*b[0] + a[1]*b[1] + a[2]*b[2] + a[3]*b[3] }

// vec2_cross возвращает псевдосклярное произведение 2D-векторов a и b (z-компонента).
func vec2_cross(a, b [2]float64) float64 { return a[0]*b[1] - a[1]*b[0] }

// vec3_cross возвращает векторное произведение векторов a и b.
func vec3_cross(a, b [3]float64) [3]float64 { return cross(a, b) }

// vec2_normalize возвращает нормированный 2D-вектор.
func vec2_normalize(v [2]float64) [2]float64 {
	l := vec2_len(v)
	if l > 0 {
		return [2]float64{v[0] / l, v[1] / l}
	}
	return v
}

// vec3_normalize возвращает нормированный 3D-вектор.
func vec3_normalize(v [3]float64) [3]float64 {
	l := vec3_len(v)
	if l > 0 {
		return [3]float64{v[0] / l, v[1] / l, v[2] / l}
	}
	return v
}

// vec4_normalize возвращает нормированный 4D-вектор.
func vec4_normalize(v [4]float64) [4]float64 {
	l := vec4_len(v)
	if l > 0 {
		return [4]float64{v[0] / l, v[1] / l, v[2] / l, v[3] / l}
	}
	return v
}

// vec2_distance возвращает расстояние между векторами a и b.
func vec2_distance(a, b [2]float64) float64 { return vec2_len(vec2_sub(a, b)) }

// vec3_distance возвращает расстояние между векторами a и b.
func vec3_distance(a, b [3]float64) float64 { return vec3_len(vec3_sub(a, b)) }

// vec4_distance возвращает расстояние между векторами a и b.
func vec4_distance(a, b [4]float64) float64 { return vec4_len(vec4_sub(a, b)) }

// vec2_sub возвращает разность векторов a - b.
func vec2_sub(a, b [2]float64) [2]float64 { return [2]float64{a[0] - b[0], a[1] - b[1]} }

// vec3_sub возвращает разность векторов a - b.
func vec3_sub(a, b [3]float64) [3]float64 { return [3]float64{a[0] - b[0], a[1] - b[1], a[2] - b[2]} }

// vec4_sub возвращает разность векторов a - b.
func vec4_sub(a, b [4]float64) [4]float64 {
	return [4]float64{a[0] - b[0], a[1] - b[1], a[2] - b[2], a[3] - b[3]}
}

// vec2_lerp возвращает линейную интерполяцию между a и b с весом t.
func vec2_lerp(a, b [2]float64, t float64) [2]float64 {
	return [2]float64{lerp(a[0], b[0], t), lerp(a[1], b[1], t)}
}

// vec3_lerp возвращает линейную интерполяцию между a и b с весом t.
func vec3_lerp(a, b [3]float64, t float64) [3]float64 {
	return [3]float64{lerp(a[0], b[0], t), lerp(a[1], b[1], t), lerp(a[2], b[2], t)}
}

// vec4_lerp возвращает линейную интерполяцию между a и b с весом t.
func vec4_lerp(a, b [4]float64, t float64) [4]float64 {
	return [4]float64{lerp(a[0], b[0], t), lerp(a[1], b[1], t), lerp(a[2], b[2], t), lerp(a[3], b[3], t)}
}

// registerMathVectorBuiltins registers the vector members into the math module.
func registerMathVectorBuiltins(mathModule *interpreter.MapValue) {
	// Vector constructors
	mathModule.Set("vec2", &interpreter.BuiltinFunction{Name: "vec2", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "vec2 requires two arguments (x, y)")
		}
		x, err := coerceToFloat(args[0])
		if err != nil {
			return err
		}
		y, err := coerceToFloat(args[1])
		if err != nil {
			return err
		}
		return vecToValue([]float64{x, y})
	}})
	mathModule.Set("vec3", &interpreter.BuiltinFunction{Name: "vec3", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 3 {
			return interpreter.NewError(400, "vec3 requires three arguments (x, y, z)")
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
		return vecToValue([]float64{x, y, z})
	}})
	mathModule.Set("vec4", &interpreter.BuiltinFunction{Name: "vec4", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 4 {
			return interpreter.NewError(400, "vec4 requires four arguments (x, y, z, w)")
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
		w, err := coerceToFloat(args[3])
		if err != nil {
			return err
		}
		return vecToValue([]float64{x, y, z, w})
	}})

	// Vector operations - length
	mathModule.Set("vec2_len", &interpreter.BuiltinFunction{Name: "vec2_len", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "vec2_len requires a 2D vector")
		}
		v := coerceToVec(args[0], 2)
		if v == nil {
			return interpreter.NewError(400, "vec2_len requires a 2D vector")
		}
		return &interpreter.FloatValue{Val: vec2_len([2]float64{v[0], v[1]})}
	}})
	mathModule.Set("vec3_len", &interpreter.BuiltinFunction{Name: "vec3_len", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "vec3_len requires a 3D vector")
		}
		v := coerceToVec(args[0], 3)
		if v == nil {
			return interpreter.NewError(400, "vec3_len requires a 3D vector")
		}
		return &interpreter.FloatValue{Val: vec3_len([3]float64{v[0], v[1], v[2]})}
	}})
	mathModule.Set("vec4_len", &interpreter.BuiltinFunction{Name: "vec4_len", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "vec4_len requires a 4D vector")
		}
		v := coerceToVec(args[0], 4)
		if v == nil {
			return interpreter.NewError(400, "vec4_len requires a 4D vector")
		}
		return &interpreter.FloatValue{Val: vec4_len([4]float64{v[0], v[1], v[2], v[3]})}
	}})

	// Vector operations - dot product
	mathModule.Set("vec2_dot", &interpreter.BuiltinFunction{Name: "vec2_dot", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "vec2_dot requires two 2D vectors")
		}
		a := coerceToVec(args[0], 2)
		b := coerceToVec(args[1], 2)
		if a == nil || b == nil {
			return interpreter.NewError(400, "vec2_dot requires two 2D vectors")
		}
		return &interpreter.FloatValue{Val: vec2_dot([2]float64{a[0], a[1]}, [2]float64{b[0], b[1]})}
	}})
	mathModule.Set("vec3_dot", &interpreter.BuiltinFunction{Name: "vec3_dot", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "vec3_dot requires two 3D vectors")
		}
		a := coerceToVec(args[0], 3)
		b := coerceToVec(args[1], 3)
		if a == nil || b == nil {
			return interpreter.NewError(400, "vec3_dot requires two 3D vectors")
		}
		return &interpreter.FloatValue{Val: vec3_dot([3]float64{a[0], a[1], a[2]}, [3]float64{b[0], b[1], b[2]})}
	}})
	mathModule.Set("vec4_dot", &interpreter.BuiltinFunction{Name: "vec4_dot", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "vec4_dot requires two 4D vectors")
		}
		a := coerceToVec(args[0], 4)
		b := coerceToVec(args[1], 4)
		if a == nil || b == nil {
			return interpreter.NewError(400, "vec4_dot requires two 4D vectors")
		}
		return &interpreter.FloatValue{Val: vec4_dot([4]float64{a[0], a[1], a[2], a[3]}, [4]float64{b[0], b[1], b[2], b[3]})}
	}})

	// Vector operations - cross product
	mathModule.Set("vec2_cross", &interpreter.BuiltinFunction{Name: "vec2_cross", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "vec2_cross requires two 2D vectors")
		}
		a := coerceToVec(args[0], 2)
		b := coerceToVec(args[1], 2)
		if a == nil || b == nil {
			return interpreter.NewError(400, "vec2_cross requires two 2D vectors")
		}
		return &interpreter.FloatValue{Val: vec2_cross([2]float64{a[0], a[1]}, [2]float64{b[0], b[1]})}
	}})
	mathModule.Set("vec3_cross", &interpreter.BuiltinFunction{Name: "vec3_cross", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "vec3_cross requires two 3D vectors")
		}
		a := coerceToVec(args[0], 3)
		b := coerceToVec(args[1], 3)
		if a == nil || b == nil {
			return interpreter.NewError(400, "vec3_cross requires two 3D vectors")
		}
		result := vec3_cross([3]float64{a[0], a[1], a[2]}, [3]float64{b[0], b[1], b[2]})
		return vecToValue([]float64{result[0], result[1], result[2]})
	}})

	// Vector operations - normalize
	mathModule.Set("vec2_normalize", &interpreter.BuiltinFunction{Name: "vec2_normalize", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "vec2_normalize requires a 2D vector")
		}
		v := coerceToVec(args[0], 2)
		if v == nil {
			return interpreter.NewError(400, "vec2_normalize requires a 2D vector")
		}
		result := vec2_normalize([2]float64{v[0], v[1]})
		return vecToValue([]float64{result[0], result[1]})
	}})
	mathModule.Set("vec3_normalize", &interpreter.BuiltinFunction{Name: "vec3_normalize", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "vec3_normalize requires a 3D vector")
		}
		v := coerceToVec(args[0], 3)
		if v == nil {
			return interpreter.NewError(400, "vec3_normalize requires a 3D vector")
		}
		result := vec3_normalize([3]float64{v[0], v[1], v[2]})
		return vecToValue([]float64{result[0], result[1], result[2]})
	}})
	mathModule.Set("vec4_normalize", &interpreter.BuiltinFunction{Name: "vec4_normalize", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "vec4_normalize requires a 4D vector")
		}
		v := coerceToVec(args[0], 4)
		if v == nil {
			return interpreter.NewError(400, "vec4_normalize requires a 4D vector")
		}
		result := vec4_normalize([4]float64{v[0], v[1], v[2], v[3]})
		return vecToValue([]float64{result[0], result[1], result[2], result[3]})
	}})

	// Vector operations - distance
	mathModule.Set("vec2_distance", &interpreter.BuiltinFunction{Name: "vec2_distance", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "vec2_distance requires two 2D vectors")
		}
		a := coerceToVec(args[0], 2)
		b := coerceToVec(args[1], 2)
		if a == nil || b == nil {
			return interpreter.NewError(400, "vec2_distance requires two 2D vectors")
		}
		return &interpreter.FloatValue{Val: vec2_distance([2]float64{a[0], a[1]}, [2]float64{b[0], b[1]})}
	}})
	mathModule.Set("vec3_distance", &interpreter.BuiltinFunction{Name: "vec3_distance", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "vec3_distance requires two 3D vectors")
		}
		a := coerceToVec(args[0], 3)
		b := coerceToVec(args[1], 3)
		if a == nil || b == nil {
			return interpreter.NewError(400, "vec3_distance requires two 3D vectors")
		}
		return &interpreter.FloatValue{Val: vec3_distance([3]float64{a[0], a[1], a[2]}, [3]float64{b[0], b[1], b[2]})}
	}})
	mathModule.Set("vec4_distance", &interpreter.BuiltinFunction{Name: "vec4_distance", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "vec4_distance requires two 4D vectors")
		}
		a := coerceToVec(args[0], 4)
		b := coerceToVec(args[1], 4)
		if a == nil || b == nil {
			return interpreter.NewError(400, "vec4_distance requires two 4D vectors")
		}
		return &interpreter.FloatValue{Val: vec4_distance([4]float64{a[0], a[1], a[2], a[3]}, [4]float64{b[0], b[1], b[2], b[3]})}
	}})

	// Vector operations - subtraction
	mathModule.Set("vec2_sub", &interpreter.BuiltinFunction{Name: "vec2_sub", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "vec2_sub requires two 2D vectors")
		}
		a := coerceToVec(args[0], 2)
		b := coerceToVec(args[1], 2)
		if a == nil || b == nil {
			return interpreter.NewError(400, "vec2_sub requires two 2D vectors")
		}
		result := vec2_sub([2]float64{a[0], a[1]}, [2]float64{b[0], b[1]})
		return vecToValue([]float64{result[0], result[1]})
	}})
	mathModule.Set("vec3_sub", &interpreter.BuiltinFunction{Name: "vec3_sub", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "vec3_sub requires two 3D vectors")
		}
		a := coerceToVec(args[0], 3)
		b := coerceToVec(args[1], 3)
		if a == nil || b == nil {
			return interpreter.NewError(400, "vec3_sub requires two 3D vectors")
		}
		result := vec3_sub([3]float64{a[0], a[1], a[2]}, [3]float64{b[0], b[1], b[2]})
		return vecToValue([]float64{result[0], result[1], result[2]})
	}})
	mathModule.Set("vec4_sub", &interpreter.BuiltinFunction{Name: "vec4_sub", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "vec4_sub requires two 4D vectors")
		}
		a := coerceToVec(args[0], 4)
		b := coerceToVec(args[1], 4)
		if a == nil || b == nil {
			return interpreter.NewError(400, "vec4_sub requires two 4D vectors")
		}
		result := vec4_sub([4]float64{a[0], a[1], a[2], a[3]}, [4]float64{b[0], b[1], b[2], b[3]})
		return vecToValue([]float64{result[0], result[1], result[2], result[3]})
	}})

	// Vector operations - lerp
	mathModule.Set("vec2_lerp", &interpreter.BuiltinFunction{Name: "vec2_lerp", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 3 {
			return interpreter.NewError(400, "vec2_lerp requires two 2D vectors and a t value")
		}
		a := coerceToVec(args[0], 2)
		b := coerceToVec(args[1], 2)
		t, err := coerceToFloat(args[2])
		if err != nil {
			return err
		}
		if a == nil || b == nil {
			return interpreter.NewError(400, "vec2_lerp requires two 2D vectors")
		}
		result := vec2_lerp([2]float64{a[0], a[1]}, [2]float64{b[0], b[1]}, t)
		return vecToValue([]float64{result[0], result[1]})
	}})
	mathModule.Set("vec3_lerp", &interpreter.BuiltinFunction{Name: "vec3_lerp", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 3 {
			return interpreter.NewError(400, "vec3_lerp requires two 3D vectors and a t value")
		}
		a := coerceToVec(args[0], 3)
		b := coerceToVec(args[1], 3)
		t, err := coerceToFloat(args[2])
		if err != nil {
			return err
		}
		if a == nil || b == nil {
			return interpreter.NewError(400, "vec3_lerp requires two 3D vectors")
		}
		result := vec3_lerp([3]float64{a[0], a[1], a[2]}, [3]float64{b[0], b[1], b[2]}, t)
		return vecToValue([]float64{result[0], result[1], result[2]})
	}})
	mathModule.Set("vec4_lerp", &interpreter.BuiltinFunction{Name: "vec4_lerp", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 3 {
			return interpreter.NewError(400, "vec4_lerp requires two 4D vectors and a t value")
		}
		a := coerceToVec(args[0], 4)
		b := coerceToVec(args[1], 4)
		t, err := coerceToFloat(args[2])
		if err != nil {
			return err
		}
		if a == nil || b == nil {
			return interpreter.NewError(400, "vec4_lerp requires two 4D vectors")
		}
		result := vec4_lerp([4]float64{a[0], a[1], a[2], a[3]}, [4]float64{b[0], b[1], b[2], b[3]}, t)
		return vecToValue([]float64{result[0], result[1], result[2], result[3]})
	}})
}
