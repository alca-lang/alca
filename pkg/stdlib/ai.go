// Package stdlib provides optional pure-Go numerical helpers for the Alca programming language.
// Its matrix operations convert interpreter lists to Go slices for calculation and back.
package stdlib

import (
	"fmt"
	"math"
	"math/rand"
	"strings"

	"github.com/alca-lang/alca/pkg/interpreter"
)

// Helper: extract 2D float matrix from ListValue
func toFloatMatrix(val interpreter.Value) ([][]float64, error) {
	list, ok := val.(*interpreter.ListValue)
	if !ok {
		return nil, fmt.Errorf("expected 2D list matrix")
	}
	rows := len(list.ElementsSlice())
	if rows == 0 {
		return [][]float64{}, nil
	}

	matrix := make([][]float64, rows)
	cols := -1

	for i, rowVal := range list.ElementsSlice() {
		rowList, ok := rowVal.(*interpreter.ListValue)
		if !ok {
			return nil, fmt.Errorf("row %d is not a list", i)
		}
		if cols == -1 {
			cols = len(rowList.ElementsSlice())
		} else if len(rowList.ElementsSlice()) != cols {
			return nil, fmt.Errorf("inconsistent matrix column width at row %d", i)
		}

		matrix[i] = make([]float64, cols)
		for j, elem := range rowList.ElementsSlice() {
			switch num := elem.(type) {
			case *interpreter.FloatValue:
				matrix[i][j] = num.Val
			case *interpreter.IntegerValue:
				matrix[i][j] = float64(num.Val)
			default:
				matrix[i][j] = 0.0
			}
		}
	}
	return matrix, nil
}

// Helper: convert 2D float matrix to ListValue
func fromFloatMatrix(matrix [][]float64) *interpreter.ListValue {
	rows := make([]interpreter.Value, len(matrix))
	for i, row := range matrix {
		cols := make([]interpreter.Value, len(row))
		for j, val := range row {
			cols[j] = &interpreter.FloatValue{Val: val}
		}
		rows[i] = interpreter.NewList(cols...)
	}
	return interpreter.NewList(rows...)
}

// Helper: extract float slice from ListValue
func toFloatSlice(val interpreter.Value) ([]float64, error) {
	list, ok := val.(*interpreter.ListValue)
	if !ok {
		return nil, fmt.Errorf("expected list")
	}
	res := make([]float64, len(list.ElementsSlice()))
	for i, elem := range list.ElementsSlice() {
		switch num := elem.(type) {
		case *interpreter.FloatValue:
			res[i] = num.Val
		case *interpreter.IntegerValue:
			res[i] = float64(num.Val)
		default:
			res[i] = 0.0
		}
	}
	return res, nil
}

// Helper: convert float slice to ListValue
func fromFloatSlice(slice []float64) *interpreter.ListValue {
	elems := make([]interpreter.Value, len(slice))
	for i, v := range slice {
		elems[i] = &interpreter.FloatValue{Val: v}
	}
	return interpreter.NewList(elems...)
}

// RegisterAIBuiltins registers "std.ai" and "ai" into the interpreter environment.
func RegisterAIBuiltins(env *interpreter.Environment) {
	aiModule := interpreter.NewMap()

	register := func(name string, fn func(args ...interpreter.Value) interpreter.Value) {
		builtin := &interpreter.BuiltinFunction{Name: name, Fn: fn}
		aiModule.Set(name, builtin)
	}

	// ai.matrix(rows, cols, [init_val or "random" or "he" or "xavier"]) -> 2D list
	register("matrix", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "ai.matrix requires rows and cols arguments")
		}
		rows := int(0)
		cols := int(0)
		if iv, ok := args[0].(*interpreter.IntegerValue); ok {
			rows = int(iv.Val)
		}
		if iv, ok := args[1].(*interpreter.IntegerValue); ok {
			cols = int(iv.Val)
		}
		if rows <= 0 || cols <= 0 {
			return interpreter.NewError(400, "rows and cols must be positive integers")
		}

		initMode := "zeros"
		initNum := 0.0
		if len(args) >= 3 {
			switch v := args[2].(type) {
			case *interpreter.FloatValue:
				initNum = v.Val
				initMode = "constant"
			case *interpreter.IntegerValue:
				initNum = float64(v.Val)
				initMode = "constant"
			case *interpreter.StringValue:
				initMode = strings.ToLower(v.Val)
			}
		}

		m := make([][]float64, rows)
		for i := 0; i < rows; i++ {
			m[i] = make([]float64, cols)
			for j := 0; j < cols; j++ {
				switch initMode {
				case "constant":
					m[i][j] = initNum
				case "random":
					m[i][j] = rand.Float64()*2.0 - 1.0 // [-1.0, 1.0]
				case "xavier", "glorot":
					limit := math.Sqrt(6.0 / float64(rows+cols))
					m[i][j] = (rand.Float64()*2.0 - 1.0) * limit
				case "he":
					std := math.Sqrt(2.0 / float64(rows))
					m[i][j] = rand.NormFloat64() * std
				default:
					m[i][j] = 0.0
				}
			}
		}
		return fromFloatMatrix(m)
	})

	// ai.matmul(A, B) -> Matrix multiplication C = A * B
	register("matmul", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "ai.matmul requires two matrix arguments (A, B)")
		}
		matA, errA := toFloatMatrix(args[0])
		if errA != nil {
			return interpreter.NewError(400, "argument A: "+errA.Error())
		}
		matB, errB := toFloatMatrix(args[1])
		if errB != nil {
			return interpreter.NewError(400, "argument B: "+errB.Error())
		}

		rowsA := len(matA)
		if rowsA == 0 {
			return fromFloatMatrix([][]float64{})
		}
		colsA := len(matA[0])
		rowsB := len(matB)
		if rowsB == 0 {
			return fromFloatMatrix([][]float64{})
		}
		colsB := len(matB[0])

		if colsA != rowsB {
			return interpreter.NewError(400, fmt.Sprintf("incompatible matrix shapes for multiplication: (%dx%d) and (%dx%d)", rowsA, colsA, rowsB, colsB))
		}

		res := make([][]float64, rowsA)
		for i := 0; i < rowsA; i++ {
			res[i] = make([]float64, colsB)
			for j := 0; j < colsB; j++ {
				sum := 0.0
				for k := 0; k < colsA; k++ {
					sum += matA[i][k] * matB[k][j]
				}
				res[i][j] = sum
			}
		}
		return fromFloatMatrix(res)
	})

	// ai.dot(v1, v2) -> float (vector dot product)
	register("dot", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "ai.dot requires two vector arguments")
		}
		v1, err1 := toFloatSlice(args[0])
		if err1 != nil {
			return interpreter.NewError(400, "vector 1: "+err1.Error())
		}
		v2, err2 := toFloatSlice(args[1])
		if err2 != nil {
			return interpreter.NewError(400, "vector 2: "+err2.Error())
		}
		if len(v1) != len(v2) {
			return interpreter.NewError(400, fmt.Sprintf("vector length mismatch: %d != %d", len(v1), len(v2)))
		}
		dot := 0.0
		for i := range v1 {
			dot += v1[i] * v2[i]
		}
		return &interpreter.FloatValue{Val: dot}
	})

	// ai.transpose(matrix) -> transposed matrix
	register("transpose", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "ai.transpose requires a matrix argument")
		}
		mat, err := toFloatMatrix(args[0])
		if err != nil {
			return interpreter.NewError(400, err.Error())
		}
		rows := len(mat)
		if rows == 0 {
			return fromFloatMatrix([][]float64{})
		}
		cols := len(mat[0])

		t := make([][]float64, cols)
		for j := 0; j < cols; j++ {
			t[j] = make([]float64, rows)
			for i := 0; i < rows; i++ {
				t[j][i] = mat[i][j]
			}
		}
		return fromFloatMatrix(t)
	})

	// ai.add(A, B) -> element-wise addition
	register("add", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "ai.add requires two matrix arguments")
		}
		matA, errA := toFloatMatrix(args[0])
		matB, errB := toFloatMatrix(args[1])
		if errA != nil || errB != nil {
			return interpreter.NewError(400, "matrices required")
		}
		if len(matA) != len(matB) || (len(matA) > 0 && len(matA[0]) != len(matB[0])) {
			return interpreter.NewError(400, "matrix dimension mismatch")
		}
		res := make([][]float64, len(matA))
		for i := range matA {
			res[i] = make([]float64, len(matA[i]))
			for j := range matA[i] {
				res[i][j] = matA[i][j] + matB[i][j]
			}
		}
		return fromFloatMatrix(res)
	})

	// ai.relu(x) -> float or list
	register("relu", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return &interpreter.FloatValue{Val: 0}
		}
		switch v := args[0].(type) {
		case *interpreter.FloatValue:
			return &interpreter.FloatValue{Val: math.Max(0, v.Val)}
		case *interpreter.IntegerValue:
			return &interpreter.FloatValue{Val: math.Max(0, float64(v.Val))}
		case *interpreter.ListValue:
			vals, err := toFloatSlice(v)
			if err != nil {
				return interpreter.NewError(400, err.Error())
			}
			out := make([]float64, len(vals))
			for i, x := range vals {
				out[i] = math.Max(0, x)
			}
			return fromFloatSlice(out)
		default:
			return &interpreter.FloatValue{Val: 0}
		}
	})

	// ai.sigmoid(x) -> float or list
	register("sigmoid", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return &interpreter.FloatValue{Val: 0.5}
		}
		sig := func(x float64) float64 {
			if x > 40 {
				return 1.0
			}
			if x < -40 {
				return 0.0
			}
			return 1.0 / (1.0 + math.Exp(-x))
		}
		switch v := args[0].(type) {
		case *interpreter.FloatValue:
			return &interpreter.FloatValue{Val: sig(v.Val)}
		case *interpreter.IntegerValue:
			return &interpreter.FloatValue{Val: sig(float64(v.Val))}
		case *interpreter.ListValue:
			vals, err := toFloatSlice(v)
			if err != nil {
				return interpreter.NewError(400, err.Error())
			}
			out := make([]float64, len(vals))
			for i, x := range vals {
				out[i] = sig(x)
			}
			return fromFloatSlice(out)
		default:
			return &interpreter.FloatValue{Val: 0.5}
		}
	})

	// ai.tanh(x) -> float or list
	register("tanh", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return &interpreter.FloatValue{Val: 0}
		}
		switch v := args[0].(type) {
		case *interpreter.FloatValue:
			return &interpreter.FloatValue{Val: math.Tanh(v.Val)}
		case *interpreter.IntegerValue:
			return &interpreter.FloatValue{Val: math.Tanh(float64(v.Val))}
		case *interpreter.ListValue:
			vals, err := toFloatSlice(v)
			if err != nil {
				return interpreter.NewError(400, err.Error())
			}
			out := make([]float64, len(vals))
			for i, x := range vals {
				out[i] = math.Tanh(x)
			}
			return fromFloatSlice(out)
		default:
			return &interpreter.FloatValue{Val: 0}
		}
	})

	// ai.softmax(vector) -> probability distribution
	register("softmax", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "ai.softmax requires a vector")
		}
		vals, err := toFloatSlice(args[0])
		if err != nil {
			return interpreter.NewError(400, err.Error())
		}
		if len(vals) == 0 {
			return fromFloatSlice([]float64{})
		}

		maxVal := vals[0]
		for _, x := range vals {
			if x > maxVal {
				maxVal = x
			}
		}

		sum := 0.0
		expVals := make([]float64, len(vals))
		for i, x := range vals {
			e := math.Exp(x - maxVal)
			expVals[i] = e
			sum += e
		}

		out := make([]float64, len(vals))
		for i := range expVals {
			out[i] = expVals[i] / sum
		}
		return fromFloatSlice(out)
	})

	// ai.mse(y_true, y_pred) -> float (mean squared error)
	register("mse", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "ai.mse requires y_true and y_pred")
		}
		yTrue, err1 := toFloatSlice(args[0])
		yPred, err2 := toFloatSlice(args[1])
		if err1 != nil || err2 != nil {
			return interpreter.NewError(400, "vectors required")
		}
		if len(yTrue) != len(yPred) || len(yTrue) == 0 {
			return interpreter.NewError(400, "vectors must have equal non-zero length")
		}
		sum := 0.0
		for i := range yTrue {
			diff := yTrue[i] - yPred[i]
			sum += diff * diff
		}
		return &interpreter.FloatValue{Val: sum / float64(len(yTrue))}
	})

	// ai.dense_forward(inputs, weights, bias, [activation]) -> forward pass result
	register("dense_forward", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 3 {
			return interpreter.NewError(400, "ai.dense_forward requires inputs, weights, bias")
		}
		inputs, errIn := toFloatMatrix(args[0])
		weights, errW := toFloatMatrix(args[1])
		if errIn != nil || errW != nil {
			return interpreter.NewError(400, "inputs and weights must be 2D matrices")
		}
		biases, errB := toFloatSlice(args[2])
		if errB != nil {
			return interpreter.NewError(400, "bias must be a list")
		}

		act := "linear"
		if len(args) >= 4 {
			act = strings.ToLower(interpreter.CoerceToString(args[3]))
		}

		// Z = inputs * weights
		rowsIn := len(inputs)
		colsIn := len(inputs[0])
		colsW := len(weights[0])

		if colsIn != len(weights) {
			return interpreter.NewError(400, fmt.Sprintf("shape mismatch: input features %d != weights rows %d", colsIn, len(weights)))
		}

		out := make([][]float64, rowsIn)
		for i := 0; i < rowsIn; i++ {
			out[i] = make([]float64, colsW)
			for j := 0; j < colsW; j++ {
				sum := 0.0
				for k := 0; k < colsIn; k++ {
					sum += inputs[i][k] * weights[k][j]
				}
				if j < len(biases) {
					sum += biases[j]
				}

				// Apply activation
				switch act {
				case "relu":
					sum = math.Max(0, sum)
				case "sigmoid":
					sum = 1.0 / (1.0 + math.Exp(-sum))
				case "tanh":
					sum = math.Tanh(sum)
				}
				out[i][j] = sum
			}
		}
		return fromFloatMatrix(out)
	})

	env.Set("ai", aiModule)
	env.Set("std.ai", aiModule)
}
