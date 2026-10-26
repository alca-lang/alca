// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package stdlib

import (
	"fmt"
	"sort"

	"github.com/alca-lang/alca/pkg/interpreter"
)

// RegisterMapBuiltins registers helpers in std.map.
func RegisterMapBuiltins(env *interpreter.Environment) {
	module := interpreter.NewMap()
	register := func(name string, fn func(...interpreter.Value) interpreter.Value) {
		module.Set(name, &interpreter.BuiltinFunction{Name: name, Fn: fn})
	}

	getMap := func(args []interpreter.Value, index int, name string) (*interpreter.MapValue, interpreter.Value) {
		if len(args) <= index {
			return nil, interpreter.NewError(400, fmt.Sprintf("map.%s requires a map argument", name))
		}
		val, ok := args[index].(*interpreter.MapValue)
		if !ok {
			return nil, interpreter.NewError(400, fmt.Sprintf("map.%s expects a map, got %s", name, args[index].Type()))
		}
		return val, nil
	}

	// length(map) -> int
	register("length", func(args ...interpreter.Value) interpreter.Value {
		m, errVal := getMap(args, 0, "length")
		if errVal != nil {
			return errVal
		}
		return &interpreter.IntegerValue{Val: int64(m.Len())}
	})

	// keys(map) -> list[string]
	register("keys", func(args ...interpreter.Value) interpreter.Value {
		m, errVal := getMap(args, 0, "keys")
		if errVal != nil {
			return errVal
		}
		keys := m.Keys()
		elems := make([]interpreter.Value, len(keys))
		for i, k := range keys {
			elems[i] = &interpreter.StringValue{Val: k}
		}
		return interpreter.NewList(elems...)
	})

	// values(map) -> list[any]
	register("values", func(args ...interpreter.Value) interpreter.Value {
		m, errVal := getMap(args, 0, "values")
		if errVal != nil {
			return errVal
		}
		return interpreter.NewList(m.Values()...)
	})

	// has(map, key) -> bool
	register("has", func(args ...interpreter.Value) interpreter.Value {
		m, errVal := getMap(args, 0, "has")
		if errVal != nil {
			return errVal
		}
		if len(args) < 2 {
			return interpreter.FALSE
		}
		key := interpreter.CoerceToString(args[1])
		if m.Has(key) {
			return interpreter.TRUE
		}
		return interpreter.FALSE
	})

	// get(map, key, default?) -> any
	register("get", func(args ...interpreter.Value) interpreter.Value {
		m, errVal := getMap(args, 0, "get")
		if errVal != nil {
			return errVal
		}
		if len(args) < 2 {
			return interpreter.NULL
		}
		key := interpreter.CoerceToString(args[1])
		if val, ok := m.Get(key); ok {
			return val
		}
		if len(args) >= 3 {
			return args[2]
		}
		return interpreter.NULL
	})

	// set(map, key, val) -> map
	register("set", func(args ...interpreter.Value) interpreter.Value {
		m, errVal := getMap(args, 0, "set")
		if errVal != nil {
			return errVal
		}
		if len(args) < 3 {
			return interpreter.NewError(400, "map.set requires key and value arguments")
		}
		key := interpreter.CoerceToString(args[1])
		m.Set(key, args[2])
		return m
	})

	// delete(map, key) -> any
	register("delete", func(args ...interpreter.Value) interpreter.Value {
		m, errVal := getMap(args, 0, "delete")
		if errVal != nil {
			return errVal
		}
		if len(args) < 2 {
			return interpreter.NULL
		}
		key := interpreter.CoerceToString(args[1])
		val, _ := m.Get(key)
		m.Delete(key)
		if val != nil {
			return val
		}
		return interpreter.NULL
	})

	// sort(map, fn?) -> map
	register("sort", func(args ...interpreter.Value) interpreter.Value {
		m, errVal := getMap(args, 0, "sort")
		if errVal != nil {
			return errVal
		}
		keys := m.Keys()
		var sortErr error
		if len(args) > 1 && args[1] != nil && args[1].Type() != interpreter.NULL_VAL {
			fn := args[1]
			sort.SliceStable(keys, func(i, j int) bool {
				if sortErr != nil {
					return false
				}
				k1, k2 := keys[i], keys[j]
				v1, _ := m.Get(k1)
				v2, _ := m.Get(k2)
				res, err := interpreter.Invoke(fn, []interpreter.Value{
					&interpreter.StringValue{Val: k1},
					v1,
					&interpreter.StringValue{Val: k2},
					v2,
				})
				if err != nil {
					res, err = interpreter.Invoke(fn, []interpreter.Value{
						&interpreter.StringValue{Val: k1},
						&interpreter.StringValue{Val: k2},
					})
				}
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
				return interpreter.IsTruthy(res)
			})
		} else {
			sort.Strings(keys)
		}
		if sortErr != nil {
			return interpreter.NewError(500, fmt.Sprintf("map.sort comparator error: %s", sortErr.Error()))
		}
		res := interpreter.NewMap()
		for _, k := range keys {
			val, _ := m.Get(k)
			res.Set(k, val)
		}
		return res
	})

	// reverse(map) -> map
	register("reverse", func(args ...interpreter.Value) interpreter.Value {
		m, errVal := getMap(args, 0, "reverse")
		if errVal != nil {
			return errVal
		}
		keys := m.Keys()
		res := interpreter.NewMap()
		for i := len(keys) - 1; i >= 0; i-- {
			k := keys[i]
			val, _ := m.Get(k)
			res.Set(k, val)
		}
		return res
	})

	// filter(map, fn) -> map
	register("filter", func(args ...interpreter.Value) interpreter.Value {
		m, errVal := getMap(args, 0, "filter")
		if errVal != nil {
			return errVal
		}
		if len(args) < 2 {
			return interpreter.NewError(400, "map.filter requires a predicate function")
		}
		fn := args[1]
		res := interpreter.NewMap()
		for _, k := range m.Keys() {
			val, _ := m.Get(k)
			ret, err := interpreter.Invoke(fn, []interpreter.Value{
				&interpreter.StringValue{Val: k},
				val,
			})
			if err != nil {
				ret, err = interpreter.Invoke(fn, []interpreter.Value{val})
			}
			if err != nil {
				return interpreter.NewError(500, fmt.Sprintf("map.filter callback error: %s", err.Error()))
			}
			if interpreter.IsTruthy(ret) {
				res.Set(k, val)
			}
		}
		return res
	})

	// map(map, fn) -> map
	register("map", func(args ...interpreter.Value) interpreter.Value {
		m, errVal := getMap(args, 0, "map")
		if errVal != nil {
			return errVal
		}
		if len(args) < 2 {
			return interpreter.NewError(400, "map.map requires a function argument")
		}
		fn := args[1]
		res := interpreter.NewMap()
		for _, k := range m.Keys() {
			val, _ := m.Get(k)
			ret, err := interpreter.Invoke(fn, []interpreter.Value{
				&interpreter.StringValue{Val: k},
				val,
			})
			if err != nil {
				ret, err = interpreter.Invoke(fn, []interpreter.Value{val})
			}
			if err != nil {
				return interpreter.NewError(500, fmt.Sprintf("map.map callback error: %s", err.Error()))
			}
			// If returned a [newKey, newVal] list:
			if lVal, ok := ret.(*interpreter.ListValue); ok && lVal.Len() == 2 {
				slice := lVal.ElementsSlice()
				newK := interpreter.CoerceToString(slice[0])
				res.Set(newK, slice[1])
			} else {
				res.Set(k, ret)
			}
		}
		return res
	})

	// reduce(map, fn, initial?) -> any
	register("reduce", func(args ...interpreter.Value) interpreter.Value {
		m, errVal := getMap(args, 0, "reduce")
		if errVal != nil {
			return errVal
		}
		if len(args) < 2 {
			return interpreter.NewError(400, "map.reduce requires a reducer function")
		}
		fn := args[1]
		keys := m.Keys()
		var acc interpreter.Value
		startIndex := 0
		if len(args) >= 3 {
			acc = args[2]
		} else {
			if len(keys) == 0 {
				return interpreter.NewError(400, "map.reduce on empty map with no initial value")
			}
			firstVal, _ := m.Get(keys[0])
			acc = firstVal
			startIndex = 1
		}
		for i := startIndex; i < len(keys); i++ {
			k := keys[i]
			val, _ := m.Get(k)
			ret, err := interpreter.Invoke(fn, []interpreter.Value{
				acc,
				&interpreter.StringValue{Val: k},
				val,
			})
			if err != nil {
				ret, err = interpreter.Invoke(fn, []interpreter.Value{acc, val})
			}
			if err != nil {
				return interpreter.NewError(500, fmt.Sprintf("map.reduce callback error: %s", err.Error()))
			}
			acc = ret
		}
		return acc
	})

	// merge(map1, map2) -> map
	register("merge", func(args ...interpreter.Value) interpreter.Value {
		m1, errVal := getMap(args, 0, "merge")
		if errVal != nil {
			return errVal
		}
		m2, errVal := getMap(args, 1, "merge")
		if errVal != nil {
			return errVal
		}
		res := interpreter.NewMap()
		for _, k := range m1.Keys() {
			v, _ := m1.Get(k)
			res.Set(k, v)
		}
		for _, k := range m2.Keys() {
			v, _ := m2.Get(k)
			res.Set(k, v)
		}
		return res
	})

	// entries(map) -> list[list[any]]
	register("entries", func(args ...interpreter.Value) interpreter.Value {
		m, errVal := getMap(args, 0, "entries")
		if errVal != nil {
			return errVal
		}
		keys := m.Keys()
		res := make([]interpreter.Value, len(keys))
		for i, k := range keys {
			v, _ := m.Get(k)
			res[i] = interpreter.NewList(&interpreter.StringValue{Val: k}, v)
		}
		return interpreter.NewList(res...)
	})

	// from_entries(list) -> map
	register("from_entries", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "map.from_entries requires a list of entries")
		}
		listVal, ok := args[0].(*interpreter.ListValue)
		if !ok {
			return interpreter.NewError(400, "map.from_entries argument must be a list")
		}
		res := interpreter.NewMap()
		for _, el := range listVal.ElementsSlice() {
			if pair, ok := el.(*interpreter.ListValue); ok && pair.Len() >= 2 {
				s := pair.ElementsSlice()
				k := interpreter.CoerceToString(s[0])
				res.Set(k, s[1])
			}
		}
		return res
	})

	// clone(map) -> map
	register("clone", func(args ...interpreter.Value) interpreter.Value {
		m, errVal := getMap(args, 0, "clone")
		if errVal != nil {
			return errVal
		}
		return m.Clone()
	})

	env.Set("std.map", module)
}
