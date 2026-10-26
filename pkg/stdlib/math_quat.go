// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package stdlib

import (
	"math"

	"github.com/alca-lang/alca/pkg/interpreter"
)

// This file holds the quaternion helpers and their std.math registrations.

// quat_identity возвращает единичный кватернион (0, 0, 0, 1).
func quat_identity() [4]float64 { return [4]float64{0, 0, 0, 1} }

// quat_from_euler возвращает кватернион из Euler-angлов (pitch, yaw, roll) в радианах.
// Порядок вращения: Z -> Y -> X (yaw, pitch, roll).
func quat_from_euler(pitch, yaw, roll float64) [4]float64 {
	cp := math.Cos(pitch / 2)
	sp := math.Sin(pitch / 2)
	cy := math.Cos(yaw / 2)
	sy := math.Sin(yaw / 2)
	cr := math.Cos(roll / 2)
	sr := math.Sin(roll / 2)
	return [4]float64{
		cp*cy*cr + sp*sy*sr, // x
		cp*cy*sr - sp*sy*cr, // y
		cp*sy*sr + sp*cy*cr, // z
		cp*cy*cr - sp*sy*sr, // w
	}
}

// quat_mul возвращает произведение кватернионов a и b (a * b).
func quat_mul(a, b [4]float64) [4]float64 {
	return [4]float64{
		a[3]*b[0] + a[0]*b[3] + a[1]*b[2] - a[2]*b[1],
		a[3]*b[1] - a[0]*b[3] + a[2]*b[0] - a[1]*b[2],
		a[3]*b[2] - a[1]*b[3] + a[0]*b[1] - a[2]*b[0],
		a[3]*b[3] - a[0]*b[0] - a[1]*b[1] - a[2]*b[2],
	}
}

// quat_conj возвращает конъюгированный кватернион (negation of x, y, z).
func quat_conj(q [4]float64) [4]float64 { return [4]float64{-q[0], -q[1], -q[2], q[3]} }

// quat_norm возвращает нормированный кватернион.
func quat_norm(q [4]float64) [4]float64 {
	l := math.Sqrt(q[0]*q[0] + q[1]*q[1] + q[2]*q[2] + q[3]*q[3])
	if l > 0 {
		q[0] /= l
		q[1] /= l
		q[2] /= l
		q[3] /= l
	}
	return q
}

// quat_length возвращает длину кватерниона.
func quat_length(q [4]float64) float64 {
	return math.Sqrt(q[0]*q[0] + q[1]*q[1] + q[2]*q[2] + q[3]*q[3])
}

// quat_inverse возвращает обратный кватернион.
func quat_inverse(q [4]float64) [4]float64 {
	l := quat_length(q)
	if l > 0 {
		q = quat_conj(q)
		for i := 0; i < 4; i++ {
			q[i] /= l
		}
	}
	return q
}

// registerMathQuatBuiltins registers the quaternion members into the math module.
func registerMathQuatBuiltins(mathModule *interpreter.MapValue) {
	// Quaternion functions
	mathModule.Set("quat_identity", &interpreter.BuiltinFunction{Name: "quat_identity", Fn: func(args ...interpreter.Value) interpreter.Value {
		return quatToValue(quat_identity())
	}})
	mathModule.Set("quat_from_euler", &interpreter.BuiltinFunction{Name: "quat_from_euler", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 3 {
			return interpreter.NewError(400, "quat_from_euler requires three arguments (pitch, yaw, roll)")
		}
		pitch, err := coerceToFloat(args[0])
		if err != nil {
			return err
		}
		yaw, err := coerceToFloat(args[1])
		if err != nil {
			return err
		}
		roll, err := coerceToFloat(args[2])
		if err != nil {
			return err
		}
		return quatToValue(quat_from_euler(pitch, yaw, roll))
	}})
	mathModule.Set("quat_mul", &interpreter.BuiltinFunction{Name: "quat_mul", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "quat_mul requires two quaternions")
		}
		a := coerceToQuat(args[0])
		b := coerceToQuat(args[1])
		return quatToValue(quat_mul(a, b))
	}})
	mathModule.Set("quat_conj", &interpreter.BuiltinFunction{Name: "quat_conj", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "quat_conj requires a quaternion")
		}
		q := coerceToQuat(args[0])
		return quatToValue(quat_conj(q))
	}})
	mathModule.Set("quat_norm", &interpreter.BuiltinFunction{Name: "quat_norm", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "quat_norm requires a quaternion")
		}
		q := coerceToQuat(args[0])
		return quatToValue(quat_norm(q))
	}})
	mathModule.Set("quat_length", &interpreter.BuiltinFunction{Name: "quat_length", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "quat_length requires a quaternion")
		}
		q := coerceToQuat(args[0])
		return &interpreter.FloatValue{Val: quat_length(q)}
	}})
	mathModule.Set("quat_inverse", &interpreter.BuiltinFunction{Name: "quat_inverse", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "quat_inverse requires a quaternion")
		}
		q := coerceToQuat(args[0])
		return quatToValue(quat_inverse(q))
	}})
}
