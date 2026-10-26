// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package stdlib

import (
	"github.com/alca-lang/alca/pkg/interpreter"
)

// This file holds the conversions between interpreter values and the fixed
// size vector, matrix and quaternion types used by the linear algebra helpers,
// plus the small 3D primitives shared by the matrix and vector operations.

// coerceToMat4 converts an interpreter Value to [4][4]float64 matrix.
func coerceToMat4(v interpreter.Value) [4][4]float64 {
	var m [4][4]float64
	switch val := v.(type) {
	case *interpreter.ListValue:
		if len(val.Elements) == 4 {
			for i, row := range val.Elements {
				if rowArr, ok := row.(*interpreter.ListValue); ok && len(rowArr.Elements) == 4 {
					for j, elem := range rowArr.Elements {
						m[i][j] = coerceToFloatSimple(elem)
					}
				}
			}
		}
	}
	return m
}

// mat4ToValue converts [4][4]float64 to interpreter Value.
func mat4ToValue(m [4][4]float64) interpreter.Value {
	rows := make([]interpreter.Value, 4)
	for i := 0; i < 4; i++ {
		cols := make([]interpreter.Value, 4)
		for j := 0; j < 4; j++ {
			cols[j] = &interpreter.FloatValue{Val: m[i][j]}
		}
		rows[i] = &interpreter.ListValue{Elements: cols}
	}
	return &interpreter.ListValue{Elements: rows}
}

// coerceToVec converts an interpreter Value to a vector array.
func coerceToVec(v interpreter.Value, size int) []float64 {
	switch val := v.(type) {
	case *interpreter.ListValue:
		if len(val.Elements) == size {
			result := make([]float64, size)
			for i, elem := range val.Elements {
				result[i] = coerceToFloatSimple(elem)
			}
			return result
		}
	}
	return nil
}

// vecToValue converts []float64 to interpreter ListValue.
func vecToValue(v []float64) interpreter.Value {
	elements := make([]interpreter.Value, len(v))
	for i, val := range v {
		elements[i] = &interpreter.FloatValue{Val: val}
	}
	return &interpreter.ListValue{Elements: elements}
}

// coerceToQuat converts an interpreter Value to [4]float64 quaternion.
func coerceToQuat(v interpreter.Value) [4]float64 {
	q := [4]float64{0, 0, 0, 1}
	if arr := coerceToVec(v, 4); arr != nil {
		q[0], q[1], q[2], q[3] = arr[0], arr[1], arr[2], arr[3]
	}
	return q
}

// quatToValue converts [4]float64 to interpreter Value.
func quatToValue(q [4]float64) interpreter.Value {
	return vecToValue([]float64{q[0], q[1], q[2], q[3]})
}

// ---------- 3D Math Helpers ----------

// cross returns the cross product of vectors a and b (3D).
func cross(a, b [3]float64) [3]float64 {
	return [3]float64{
		a[1]*b[2] - a[2]*b[1],
		a[2]*b[0] - a[0]*b[2],
		a[0]*b[1] - a[1]*b[0],
	}
}

// dot returns the dot product of vectors a and b (3D).
func dot(a, b [3]float64) float64 { return a[0]*b[0] + a[1]*b[1] + a[2]*b[2] }
