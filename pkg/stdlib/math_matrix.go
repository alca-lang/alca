// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package stdlib

import (
	"math"

	"github.com/alca-lang/alca/pkg/interpreter"
)

// This file holds the 4x4 matrix helpers and their std.math registrations.

// mat4_identity возвращает 4x4 единичную матрицу.
func mat4_identity() [4][4]float64 {
	var m [4][4]float64
	for i := 0; i < 4; i++ {
		m[i][i] = 1
	}
	return m
}

// mat4_mul умножает матрицы a и b (a * b).
func mat4_mul(a, b [4][4]float64) [4][4]float64 {
	var r [4][4]float64
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			var s float64
			for k := 0; k < 4; k++ {
				s += a[i][k] * b[k][j]
			}
			r[i][j] = s
		}
	}
	return r
}

// mat4_transpose возвращает транспонированную матрицу m.
func mat4_transpose(m [4][4]float64) [4][4]float64 {
	var r [4][4]float64
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			r[i][j] = m[j][i]
		}
	}
	return r
}

// mat4_determinant возвращает определитель матрицы m.
func mat4_determinant(m [4][4]float64) float64 {
	return m[0][3]*m[1][2]*m[2][1]*m[3][0] -
		m[0][2]*m[1][3]*m[2][1]*m[3][0] -
		m[0][3]*m[1][1]*m[2][2]*m[3][0] +
		m[0][1]*m[1][3]*m[2][2]*m[3][0] +
		m[0][2]*m[1][1]*m[2][3]*m[3][0] -
		m[0][1]*m[1][2]*m[2][3]*m[3][0]
}

// mat4_invert возвращает обратную матрицу m.
func mat4_invert(m [4][4]float64) [4][4]float64 {
	det := mat4_determinant(m)
	if det == 0 {
		return [4][4]float64{}
	}
	// Compute adjugate (classical adjoint) then divide by det
	inv00 := m[1][1]*m[2][2]*m[3][3] - m[1][2]*m[2][3]*m[3][1] - m[1][3]*m[2][1]*m[3][2] + m[1][1]*m[2][3]*m[3][2]
	inv01 := -m[0][1]*m[2][2]*m[3][3] + m[0][2]*m[2][1]*m[3][3] + m[0][3]*m[2][1]*m[3][2] - m[0][1]*m[2][3]*m[3][2]
	inv02 := m[0][1]*m[1][2]*m[3][3] - m[0][2]*m[1][1]*m[3][3] - m[0][3]*m[1][1]*m[3][2] + m[0][1]*m[1][3]*m[3][2]
	inv03 := -m[0][1]*m[1][2]*m[2][3] + m[0][2]*m[1][1]*m[2][3] + m[0][3]*m[1][1]*m[2][2] - m[0][1]*m[1][3]*m[2][2]
	inv10 := -m[1][0]*m[2][2]*m[3][3] + m[1][2]*m[2][0]*m[3][3] + m[1][3]*m[2][0]*m[3][2] - m[1][1]*m[2][3]*m[3][0]
	inv11 := m[0][0]*m[2][2]*m[3][3] - m[0][2]*m[2][0]*m[3][3] - m[0][3]*m[2][0]*m[3][2] + m[0][2]*m[2][3]*m[3][0]
	inv12 := -m[0][0]*m[1][2]*m[3][3] + m[0][2]*m[1][0]*m[3][3] + m[0][3]*m[1][0]*m[3][2] - m[0][0]*m[1][3]*m[3][2]
	inv13 := m[0][0]*m[1][2]*m[2][3] - m[0][2]*m[1][0]*m[2][3] - m[0][3]*m[1][0]*m[2][2] + m[0][0]*m[1][3]*m[2][2]
	inv20 := m[1][0]*m[2][1]*m[3][3] - m[1][1]*m[2][0]*m[3][3] - m[1][3]*m[2][0]*m[3][1] + m[1][1]*m[2][3]*m[3][0]
	inv21 := -m[0][0]*m[2][1]*m[3][3] + m[0][1]*m[2][0]*m[3][3] + m[0][3]*m[2][0]*m[3][1] - m[0][1]*m[2][3]*m[3][0]
	inv22 := m[0][0]*m[1][1]*m[3][3] - m[0][1]*m[1][0]*m[3][3] - m[0][3]*m[1][0]*m[3][1] + m[0][1]*m[1][3]*m[3][0]
	inv23 := -m[0][0]*m[1][1]*m[2][3] + m[0][1]*m[1][0]*m[2][3] + m[0][3]*m[1][0]*m[2][1] - m[0][1]*m[1][3]*m[2][1]
	inv30 := -m[1][0]*m[2][1]*m[3][2] + m[1][1]*m[2][0]*m[3][2] + m[1][2]*m[2][1]*m[3][0] - m[1][1]*m[2][2]*m[3][0]
	inv31 := m[0][0]*m[2][1]*m[3][2] - m[0][1]*m[2][0]*m[3][2] - m[0][2]*m[2][1]*m[3][0] + m[0][1]*m[2][2]*m[3][0]
	inv32 := -m[0][0]*m[1][1]*m[3][2] + m[0][1]*m[1][0]*m[3][2] + m[0][2]*m[1][1]*m[3][0] - m[0][1]*m[1][2]*m[3][0]
	inv33 := m[0][0]*m[1][1]*m[2][2] - m[0][1]*m[1][0]*m[2][2] - m[0][2]*m[1][1]*m[2][0] + m[0][1]*m[1][2]*m[2][0]
	var inv [4][4]float64
	inv[0][0], inv[0][1], inv[0][2], inv[0][3] = inv00, inv01, inv02, inv03
	inv[1][0], inv[1][1], inv[1][2], inv[1][3] = inv10, inv11, inv12, inv13
	inv[2][0], inv[2][1], inv[2][2], inv[2][3] = inv20, inv21, inv22, inv23
	inv[3][0], inv[3][1], inv[3][2], inv[3][3] = inv30, inv31, inv32, inv33
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			inv[i][j] /= det
		}
	}
	return inv
}

// mat4_look_at возвращает матрицу вида (view matrix) от eye к center с up-вектором.
func mat4_look_at(eye, center, up [3]float64) [4][4]float64 {
	f := [3]float64{eye[0] - center[0], eye[1] - center[1], eye[2] - center[2]}
	lenf := math.Sqrt(f[0]*f[0] + f[1]*f[1] + f[2]*f[2])
	if lenf > 0 {
		f[0] /= lenf
		f[1] /= lenf
		f[2] /= lenf
	}
	u := cross(up, f)
	lenu := math.Sqrt(u[0]*u[0] + u[1]*u[1] + u[2]*u[2])
	if lenu > 0 {
		u[0] /= lenu
		u[1] /= lenu
		u[2] /= lenu
	}
	v := cross(f, u)
	// Build view matrix with OpenGL-style column-major layout stored in row-major [4][4]
	// r column 0 = u, column 1 = v, column 2 = -f, column 3 = translation
	var r [4][4]float64
	// Column 0 (u)
	r[0][0], r[1][0], r[2][0], r[3][0] = u[0], u[1], u[2], 0
	// Column 1 (v)
	r[0][1], r[1][1], r[2][1], r[3][1] = v[0], v[1], v[2], 0
	// Column 2 (-f)
	r[0][2], r[1][2], r[2][2], r[3][2] = -f[0], -f[1], -f[2], 0
	// Column 3 (translation)
	t30 := -(u[0]*eye[0] + u[1]*eye[1] + u[2]*eye[2])
	t31 := -(v[0]*eye[0] + v[1]*eye[1] + v[2]*eye[2])
	t32 := +f[0]*eye[0] + f[1]*eye[1] + f[2]*eye[2]
	r[0][3], r[1][3], r[2][3], r[3][3] = t30, t31, t32, 1
	return r
}

// mat4_perspective возвращает перспективную проекционную матрицу.
func mat4_perspective(fov, aspect, near, far float64) [4][4]float64 {
	frust := 1 / math.Tan(fov*math.Pi/360)
	var r [4][4]float64
	r[0][0], r[1][1] = frust/aspect, frust
	r[2][2] = -(far + near) / (far - near)
	r[2][3] = -1
	r[3][2] = -(2 * far * near) / (far - near)
	r[3][3] = 0
	return r
}

// mat4_ortho возвращает ортографическую проекционную матрицу.
func mat4_ortho(left, right, bottom, top, near, far float64) [4][4]float64 {
	var r [4][4]float64
	x, y, z := 2/(right-left), 2/(top-bottom), -2/(far-near)
	r[0][0], r[1][1], r[2][2] = x, y, z
	r[3][0], r[3][1], r[3][2] = -(right+left)/(right-left), -(top+bottom)/(top-bottom), -(far+near)/(far-near)
	r[3][3] = 1
	return r
}

// registerMathMatrixBuiltins registers the 4x4 matrix members into the math module.
func registerMathMatrixBuiltins(mathModule *interpreter.MapValue) {
	// Matrix 4x4 functions
	mathModule.Set("mat4_identity", &interpreter.BuiltinFunction{Name: "mat4_identity", Fn: func(args ...interpreter.Value) interpreter.Value {
		return mat4ToValue(mat4_identity())
	}})
	mathModule.Set("mat4_mul", &interpreter.BuiltinFunction{Name: "mat4_mul", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "mat4_mul requires two matrices")
		}
		a := coerceToMat4(args[0])
		b := coerceToMat4(args[1])
		return mat4ToValue(mat4_mul(a, b))
	}})
	mathModule.Set("mat4_transpose", &interpreter.BuiltinFunction{Name: "mat4_transpose", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "mat4_transpose requires a matrix")
		}
		m := coerceToMat4(args[0])
		return mat4ToValue(mat4_transpose(m))
	}})
	mathModule.Set("mat4_determinant", &interpreter.BuiltinFunction{Name: "mat4_determinant", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "mat4_determinant requires a matrix")
		}
		m := coerceToMat4(args[0])
		return &interpreter.FloatValue{Val: mat4_determinant(m)}
	}})
	mathModule.Set("mat4_invert", &interpreter.BuiltinFunction{Name: "mat4_invert", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "mat4_invert requires a matrix")
		}
		m := coerceToMat4(args[0])
		return mat4ToValue(mat4_invert(m))
	}})
	mathModule.Set("mat4_look_at", &interpreter.BuiltinFunction{Name: "mat4_look_at", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 3 {
			return interpreter.NewError(400, "mat4_look_at requires three vectors (eye, center, up)")
		}
		eye := coerceToVec(args[0], 3)
		center := coerceToVec(args[1], 3)
		up := coerceToVec(args[2], 3)
		if eye == nil || center == nil || up == nil {
			return interpreter.NewError(400, "mat4_look_at requires 3D vectors")
		}
		eyeArr := [3]float64{eye[0], eye[1], eye[2]}
		centerArr := [3]float64{center[0], center[1], center[2]}
		upArr := [3]float64{up[0], up[1], up[2]}
		return mat4ToValue(mat4_look_at(eyeArr, centerArr, upArr))
	}})
	mathModule.Set("mat4_perspective", &interpreter.BuiltinFunction{Name: "mat4_perspective", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 4 {
			return interpreter.NewError(400, "mat4_perspective requires four arguments (fov, aspect, near, far)")
		}
		fov, err := coerceToFloat(args[0])
		if err != nil {
			return err
		}
		aspect, err := coerceToFloat(args[1])
		if err != nil {
			return err
		}
		near, err := coerceToFloat(args[2])
		if err != nil {
			return err
		}
		far, err := coerceToFloat(args[3])
		if err != nil {
			return err
		}
		return mat4ToValue(mat4_perspective(fov, aspect, near, far))
	}})
	mathModule.Set("mat4_ortho", &interpreter.BuiltinFunction{Name: "mat4_ortho", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 6 {
			return interpreter.NewError(400, "mat4_ortho requires six arguments (left, right, bottom, top, near, far)")
		}
		left, err := coerceToFloat(args[0])
		if err != nil {
			return err
		}
		right, err := coerceToFloat(args[1])
		if err != nil {
			return err
		}
		bottom, err := coerceToFloat(args[2])
		if err != nil {
			return err
		}
		top, err := coerceToFloat(args[3])
		if err != nil {
			return err
		}
		near, err := coerceToFloat(args[4])
		if err != nil {
			return err
		}
		far, err := coerceToFloat(args[5])
		if err != nil {
			return err
		}
		return mat4ToValue(mat4_ortho(left, right, bottom, top, near, far))
	}})
}
