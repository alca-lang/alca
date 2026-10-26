// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package stdlib

import (
	"fmt"
	"sort"

	"github.com/alca-lang/alca/pkg/interpreter"
)

// RegisterListBuiltins registers helpers in std.list.
func RegisterListBuiltins(env *interpreter.Environment) {
	module := interpreter.NewMap()
	register := func(name string, fn func(...interpreter.Value) interpreter.Value) {
		module.Set(name, &interpreter.BuiltinFunction{Name: name, Fn: fn})
	}

	getList := func(args []interpreter.Value, index int, name string) (*interpreter.ListValue, interpreter.Value) {
		if len(args) <= index {
			return nil, interpreter.NewError(400, fmt.Sprintf("list.%s requires a list argument", name))
		}
		val, ok := args[index].(*interpreter.ListValue)
		if !ok {
			return nil, interpreter.NewError(400, fmt.Sprintf("list.%s expects a list, got %s", name, args[index].Type()))
		}
		return val, nil
	}

	// length(list) -> int
	register("length", func(args ...interpreter.Value) interpreter.Value {
		l, errVal := getList(args, 0, "length")
		if errVal != nil {
			return errVal
		}
		return &interpreter.IntegerValue{Val: int64(l.Len())}
	})

	// push(list, elem) -> list
	register("push", func(args ...interpreter.Value) interpreter.Value {
		l, errVal := getList(args, 0, "push")
		if errVal != nil {
			return errVal
		}
		if len(args) < 2 {
			return interpreter.NewError(400, "list.push requires an element to push")
		}
		l.Append(args[1])
		return l
	})

	// pop(list) -> any
	register("pop", func(args ...interpreter.Value) interpreter.Value {
		l, errVal := getList(args, 0, "pop")
		if errVal != nil {
			return errVal
		}
		l.Materialize()
		if len(l.Elements) == 0 {
			return interpreter.NULL
		}
		last := l.Elements[len(l.Elements)-1]
		l.Elements = l.Elements[:len(l.Elements)-1]
		return last
	})

	// reverse(list) -> list
	register("reverse", func(args ...interpreter.Value) interpreter.Value {
		l, errVal := getList(args, 0, "reverse")
		if errVal != nil {
			return errVal
		}
		l.Materialize()
		n := len(l.Elements)
		for i := 0; i < n/2; i++ {
			l.Elements[i], l.Elements[n-1-i] = l.Elements[n-1-i], l.Elements[i]
		}
		return l
	})

	// sort(list, fn?) -> list
	register("sort", func(args ...interpreter.Value) interpreter.Value {
		l, errVal := getList(args, 0, "sort")
		if errVal != nil {
			return errVal
		}
		l.Materialize()
		var sortErr error
		if len(args) > 1 && args[1] != nil && args[1].Type() != interpreter.NULL_VAL {
			fn := args[1]
			sort.SliceStable(l.Elements, func(i, j int) bool {
				if sortErr != nil {
					return false
				}
				a, b := l.Elements[i], l.Elements[j]
				res, err := interpreter.Invoke(fn, []interpreter.Value{a, b})
				if err != nil {
					sortErr = err
					return false
				}
				if bVal, ok := res.(*interpreter.BoolValue); ok {
					return bVal.Val
				}
				if iVal, ok := res.(*interpreter.IntegerValue); ok {
					return iVal.Val < 0
				}
				if fVal, ok := res.(*interpreter.FloatValue); ok {
					return fVal.Val < 0
				}
				return interpreter.IsTruthy(res)
			})
		} else {
			sort.SliceStable(l.Elements, func(i, j int) bool {
				a, b := l.Elements[i], l.Elements[j]
				if a.Type() == interpreter.INTEGER_VAL && b.Type() == interpreter.INTEGER_VAL {
					return a.(*interpreter.IntegerValue).Val < b.(*interpreter.IntegerValue).Val
				}
				if a.Type() == interpreter.FLOAT_VAL && b.Type() == interpreter.FLOAT_VAL {
					return a.(*interpreter.FloatValue).Val < b.(*interpreter.FloatValue).Val
				}
				if a.Type() == interpreter.INTEGER_VAL && b.Type() == interpreter.FLOAT_VAL {
					return float64(a.(*interpreter.IntegerValue).Val) < b.(*interpreter.FloatValue).Val
				}
				if a.Type() == interpreter.FLOAT_VAL && b.Type() == interpreter.INTEGER_VAL {
					return a.(*interpreter.FloatValue).Val < float64(b.(*interpreter.IntegerValue).Val)
				}
				if a.Type() == interpreter.STRING_VAL && b.Type() == interpreter.STRING_VAL {
					return a.(*interpreter.StringValue).Val < b.(*interpreter.StringValue).Val
				}
				return interpreter.CoerceToString(a) < interpreter.CoerceToString(b)
			})
		}
		if sortErr != nil {
			return interpreter.NewError(500, fmt.Sprintf("list.sort comparator error: %s", sortErr.Error()))
		}
		return l
	})

	// filter(list, fn) -> list
	register("filter", func(args ...interpreter.Value) interpreter.Value {
		l, errVal := getList(args, 0, "filter")
		if errVal != nil {
			return errVal
		}
		if len(args) < 2 {
			return interpreter.NewError(400, "list.filter requires a predicate function")
		}
		fn := args[1]
		elems := l.ElementsSlice()
		res := make([]interpreter.Value, 0)
		for idx, el := range elems {
			ret, err := interpreter.Invoke(fn, []interpreter.Value{el, &interpreter.IntegerValue{Val: int64(idx)}})
			if err != nil {
				// Retry with single argument if arity mismatched
				ret, err = interpreter.Invoke(fn, []interpreter.Value{el})
			}
			if err != nil {
				return interpreter.NewError(500, fmt.Sprintf("list.filter callback error: %s", err.Error()))
			}
			if interpreter.IsTruthy(ret) {
				res = append(res, el)
			}
		}
		return interpreter.NewList(res...)
	})

	// map(list, fn) -> list
	register("map", func(args ...interpreter.Value) interpreter.Value {
		l, errVal := getList(args, 0, "map")
		if errVal != nil {
			return errVal
		}
		if len(args) < 2 {
			return interpreter.NewError(400, "list.map requires a function argument")
		}
		fn := args[1]
		elems := l.ElementsSlice()
		res := make([]interpreter.Value, len(elems))
		for idx, el := range elems {
			ret, err := interpreter.Invoke(fn, []interpreter.Value{el, &interpreter.IntegerValue{Val: int64(idx)}})
			if err != nil {
				ret, err = interpreter.Invoke(fn, []interpreter.Value{el})
			}
			if err != nil {
				return interpreter.NewError(500, fmt.Sprintf("list.map callback error: %s", err.Error()))
			}
			res[idx] = ret
		}
		return interpreter.NewList(res...)
	})

	// reduce(list, fn, initial?) -> any
	register("reduce", func(args ...interpreter.Value) interpreter.Value {
		l, errVal := getList(args, 0, "reduce")
		if errVal != nil {
			return errVal
		}
		if len(args) < 2 {
			return interpreter.NewError(400, "list.reduce requires a reducer function")
		}
		fn := args[1]
		elems := l.ElementsSlice()
		var acc interpreter.Value
		startIndex := 0
		if len(args) >= 3 {
			acc = args[2]
		} else {
			if len(elems) == 0 {
				return interpreter.NewError(400, "list.reduce on empty list with no initial value")
			}
			acc = elems[0]
			startIndex = 1
		}
		for i := startIndex; i < len(elems); i++ {
			ret, err := interpreter.Invoke(fn, []interpreter.Value{acc, elems[i], &interpreter.IntegerValue{Val: int64(i)}})
			if err != nil {
				ret, err = interpreter.Invoke(fn, []interpreter.Value{acc, elems[i]})
			}
			if err != nil {
				return interpreter.NewError(500, fmt.Sprintf("list.reduce callback error: %s", err.Error()))
			}
			acc = ret
		}
		return acc
	})

	// keys(list) -> list[int]
	register("keys", func(args ...interpreter.Value) interpreter.Value {
		l, errVal := getList(args, 0, "keys")
		if errVal != nil {
			return errVal
		}
		n := l.Len()
		res := make([]interpreter.Value, n)
		for i := 0; i < n; i++ {
			res[i] = &interpreter.IntegerValue{Val: int64(i)}
		}
		return interpreter.NewList(res...)
	})

	// values(list) -> list
	register("values", func(args ...interpreter.Value) interpreter.Value {
		l, errVal := getList(args, 0, "values")
		if errVal != nil {
			return errVal
		}
		return l.Clone()
	})

	// contains(list, elem) -> bool
	register("contains", func(args ...interpreter.Value) interpreter.Value {
		l, errVal := getList(args, 0, "contains")
		if errVal != nil {
			return errVal
		}
		if len(args) < 2 {
			return interpreter.FALSE
		}
		for _, el := range l.ElementsSlice() {
			if interpreter.ValuesEqual(el, args[1]) {
				return interpreter.TRUE
			}
		}
		return interpreter.FALSE
	})

	// index_of(list, elem) -> int
	register("index_of", func(args ...interpreter.Value) interpreter.Value {
		l, errVal := getList(args, 0, "index_of")
		if errVal != nil {
			return errVal
		}
		if len(args) < 2 {
			return &interpreter.IntegerValue{Val: -1}
		}
		for idx, el := range l.ElementsSlice() {
			if interpreter.ValuesEqual(el, args[1]) {
				return &interpreter.IntegerValue{Val: int64(idx)}
			}
		}
		return &interpreter.IntegerValue{Val: -1}
	})

	// slice(list, start, end?) -> list
	register("slice", func(args ...interpreter.Value) interpreter.Value {
		l, errVal := getList(args, 0, "slice")
		if errVal != nil {
			return errVal
		}
		if len(args) < 2 {
			return interpreter.NewError(400, "list.slice requires start index")
		}
		startVal, ok1 := args[1].(*interpreter.IntegerValue)
		if !ok1 {
			return interpreter.NewError(400, "list.slice start index must be integer")
		}
		elems := l.ElementsSlice()
		n := int64(len(elems))
		start := startVal.Val
		if start < 0 {
			start += n
			if start < 0 {
				start = 0
			}
		}
		end := n
		if len(args) >= 3 && args[2] != nil && args[2].Type() != interpreter.NULL_VAL {
			if endVal, ok2 := args[2].(*interpreter.IntegerValue); ok2 {
				end = endVal.Val
				if end < 0 {
					end += n
				}
				if end > n {
					end = n
				}
			}
		}
		if start > end {
			start = end
		}
		if start > n {
			start = n
		}
		return interpreter.NewList(elems[start:end]...)
	})

	// join(list, sep?) -> string
	register("join", func(args ...interpreter.Value) interpreter.Value {
		l, errVal := getList(args, 0, "join")
		if errVal != nil {
			return errVal
		}
		sep := ""
		if len(args) >= 2 {
			sep = interpreter.CoerceToString(args[1])
		}
		elems := l.ElementsSlice()
		var b []byte
		for i, el := range elems {
			if i > 0 {
				b = append(b, sep...)
			}
			b = append(b, interpreter.CoerceToString(el)...)
		}
		return &interpreter.StringValue{Val: string(b)}
	})

	// concat(list1, list2) -> list
	register("concat", func(args ...interpreter.Value) interpreter.Value {
		l1, errVal := getList(args, 0, "concat")
		if errVal != nil {
			return errVal
		}
		l2, errVal := getList(args, 1, "concat")
		if errVal != nil {
			return errVal
		}
		e1 := l1.ElementsSlice()
		e2 := l2.ElementsSlice()
		res := make([]interpreter.Value, len(e1)+len(e2))
		copy(res, e1)
		copy(res[len(e1):], e2)
		return interpreter.NewList(res...)
	})

	// find(list, fn) -> any
	register("find", func(args ...interpreter.Value) interpreter.Value {
		l, errVal := getList(args, 0, "find")
		if errVal != nil {
			return errVal
		}
		if len(args) < 2 {
			return interpreter.NewError(400, "list.find requires a predicate function")
		}
		fn := args[1]
		for idx, el := range l.ElementsSlice() {
			ret, err := interpreter.Invoke(fn, []interpreter.Value{el, &interpreter.IntegerValue{Val: int64(idx)}})
			if err != nil {
				ret, err = interpreter.Invoke(fn, []interpreter.Value{el})
			}
			if err != nil {
				return interpreter.NewError(500, fmt.Sprintf("list.find callback error: %s", err.Error()))
			}
			if interpreter.IsTruthy(ret) {
				return el
			}
		}
		return interpreter.NULL
	})

	// find_index(list, fn) -> int
	register("find_index", func(args ...interpreter.Value) interpreter.Value {
		l, errVal := getList(args, 0, "find_index")
		if errVal != nil {
			return errVal
		}
		if len(args) < 2 {
			return interpreter.NewError(400, "list.find_index requires a predicate function")
		}
		fn := args[1]
		for idx, el := range l.ElementsSlice() {
			ret, err := interpreter.Invoke(fn, []interpreter.Value{el, &interpreter.IntegerValue{Val: int64(idx)}})
			if err != nil {
				ret, err = interpreter.Invoke(fn, []interpreter.Value{el})
			}
			if err != nil {
				return interpreter.NewError(500, fmt.Sprintf("list.find_index callback error: %s", err.Error()))
			}
			if interpreter.IsTruthy(ret) {
				return &interpreter.IntegerValue{Val: int64(idx)}
			}
		}
		return &interpreter.IntegerValue{Val: -1}
	})

	// any(list, fn?) -> bool
	register("any", func(args ...interpreter.Value) interpreter.Value {
		l, errVal := getList(args, 0, "any")
		if errVal != nil {
			return errVal
		}
		elems := l.ElementsSlice()
		if len(args) >= 2 && args[1] != nil && args[1].Type() != interpreter.NULL_VAL {
			fn := args[1]
			for idx, el := range elems {
				ret, err := interpreter.Invoke(fn, []interpreter.Value{el, &interpreter.IntegerValue{Val: int64(idx)}})
				if err != nil {
					ret, err = interpreter.Invoke(fn, []interpreter.Value{el})
				}
				if err != nil {
					return interpreter.NewError(500, err.Error())
				}
				if interpreter.IsTruthy(ret) {
					return interpreter.TRUE
				}
			}
		} else {
			for _, el := range elems {
				if interpreter.IsTruthy(el) {
					return interpreter.TRUE
				}
			}
		}
		return interpreter.FALSE
	})

	// all(list, fn?) -> bool
	register("all", func(args ...interpreter.Value) interpreter.Value {
		l, errVal := getList(args, 0, "all")
		if errVal != nil {
			return errVal
		}
		elems := l.ElementsSlice()
		if len(args) >= 2 && args[1] != nil && args[1].Type() != interpreter.NULL_VAL {
			fn := args[1]
			for idx, el := range elems {
				ret, err := interpreter.Invoke(fn, []interpreter.Value{el, &interpreter.IntegerValue{Val: int64(idx)}})
				if err != nil {
					ret, err = interpreter.Invoke(fn, []interpreter.Value{el})
				}
				if err != nil {
					return interpreter.NewError(500, err.Error())
				}
				if !interpreter.IsTruthy(ret) {
					return interpreter.FALSE
				}
			}
		} else {
			for _, el := range elems {
				if !interpreter.IsTruthy(el) {
					return interpreter.FALSE
				}
			}
		}
		return interpreter.TRUE
	})

	// flatten(list) -> list
	register("flatten", func(args ...interpreter.Value) interpreter.Value {
		l, errVal := getList(args, 0, "flatten")
		if errVal != nil {
			return errVal
		}
		res := make([]interpreter.Value, 0)
		for _, el := range l.ElementsSlice() {
			if subList, ok := el.(*interpreter.ListValue); ok {
				res = append(res, subList.ElementsSlice()...)
			} else {
				res = append(res, el)
			}
		}
		return interpreter.NewList(res...)
	})

	// unique(list) -> list
	register("unique", func(args ...interpreter.Value) interpreter.Value {
		l, errVal := getList(args, 0, "unique")
		if errVal != nil {
			return errVal
		}
		res := make([]interpreter.Value, 0)
		seen := make(map[string]bool)
		for _, el := range l.ElementsSlice() {
			k := el.Inspect()
			if !seen[k] {
				seen[k] = true
				res = append(res, el)
			}
		}
		return interpreter.NewList(res...)
	})

	env.Set("std.list", module)
}
