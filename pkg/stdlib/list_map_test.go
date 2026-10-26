// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package stdlib_test

import (
	"testing"

	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/stdlib"
)

func newTestEnv() *interpreter.Environment {
	env := interpreter.NewEnvironment()
	stdlib.RegisterListBuiltins(env)
	stdlib.RegisterMapBuiltins(env)
	stdlib.RegisterStringBuiltins(env)

	stdModule := interpreter.NewMap()
	for _, name := range []string{"list", "map", "string", "str"} {
		if val, ok := env.Get("std." + name); ok {
			stdModule.Set(name, val)
		} else if val, ok := env.Get(name); ok {
			stdModule.Set(name, val)
		}
	}
	env.Set("std", stdModule)
	return env
}

func TestStdListOperations(t *testing.T) {
	env := newTestEnv()
	listVal, _ := env.Get("std.list")
	listMod := listVal.(*interpreter.MapValue)

	// 1. sort
	sortFn, _ := listMod.Get("sort")
	l := interpreter.NewList(&interpreter.IntegerValue{Val: 5}, &interpreter.IntegerValue{Val: 2}, &interpreter.IntegerValue{Val: 8})
	sorted := sortFn.(*interpreter.BuiltinFunction).Fn(l)
	sList := sorted.(*interpreter.ListValue)
	if sList.Len() != 3 || sList.Elements[0].(*interpreter.IntegerValue).Val != 2 || sList.Elements[2].(*interpreter.IntegerValue).Val != 8 {
		t.Fatalf("list.sort failed: %v", sorted)
	}

	// 2. reverse
	revFn, _ := listMod.Get("reverse")
	rev := revFn.(*interpreter.BuiltinFunction).Fn(sorted)
	rList := rev.(*interpreter.ListValue)
	if rList.Elements[0].(*interpreter.IntegerValue).Val != 8 || rList.Elements[2].(*interpreter.IntegerValue).Val != 2 {
		t.Fatalf("list.reverse failed: %v", rev)
	}

	// 3. filter
	filterFn, _ := listMod.Get("filter")
	predicate := &interpreter.BuiltinFunction{
		Name: "is_even",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			v := args[0].(*interpreter.IntegerValue).Val
			if v%2 == 0 {
				return interpreter.TRUE
			}
			return interpreter.FALSE
		},
	}
	filtered := filterFn.(*interpreter.BuiltinFunction).Fn(sorted, predicate)
	fList := filtered.(*interpreter.ListValue)
	if fList.Len() != 2 || fList.Elements[0].(*interpreter.IntegerValue).Val != 8 || fList.Elements[1].(*interpreter.IntegerValue).Val != 2 {
		t.Fatalf("list.filter failed: %v", filtered)
	}

	// 4. map
	mapFn, _ := listMod.Get("map")
	mapper := &interpreter.BuiltinFunction{
		Name: "double",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			v := args[0].(*interpreter.IntegerValue).Val
			return &interpreter.IntegerValue{Val: v * 2}
		},
	}
	mapped := mapFn.(*interpreter.BuiltinFunction).Fn(fList, mapper)
	mList := mapped.(*interpreter.ListValue)
	if mList.Elements[0].(*interpreter.IntegerValue).Val != 16 || mList.Elements[1].(*interpreter.IntegerValue).Val != 4 {
		t.Fatalf("list.map failed: %v", mapped)
	}

	// 5. reduce
	reduceFn, _ := listMod.Get("reduce")
	reducer := &interpreter.BuiltinFunction{
		Name: "sum",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			acc := args[0].(*interpreter.IntegerValue).Val
			item := args[1].(*interpreter.IntegerValue).Val
			return &interpreter.IntegerValue{Val: acc + item}
		},
	}
	reduced := reduceFn.(*interpreter.BuiltinFunction).Fn(mList, reducer, &interpreter.IntegerValue{Val: 10})
	if reduced.(*interpreter.IntegerValue).Val != 30 {
		t.Fatalf("list.reduce failed: want 30, got %v", reduced)
	}

	// 6. keys and values
	keysFn, _ := listMod.Get("keys")
	valuesFn, _ := listMod.Get("values")
	kRes := keysFn.(*interpreter.BuiltinFunction).Fn(mList).(*interpreter.ListValue)
	vRes := valuesFn.(*interpreter.BuiltinFunction).Fn(mList).(*interpreter.ListValue)
	if kRes.Len() != 2 || kRes.Elements[0].(*interpreter.IntegerValue).Val != 0 || kRes.Elements[1].(*interpreter.IntegerValue).Val != 1 {
		t.Fatalf("list.keys failed: %v", kRes)
	}
	if vRes.Len() != 2 || vRes.Elements[0].(*interpreter.IntegerValue).Val != 16 {
		t.Fatalf("list.values failed: %v", vRes)
	}
}

func TestStdMapOperations(t *testing.T) {
	env := newTestEnv()
	mapVal, _ := env.Get("std.map")
	mapMod := mapVal.(*interpreter.MapValue)

	m := interpreter.NewMap()
	m.Set("b", &interpreter.IntegerValue{Val: 20})
	m.Set("a", &interpreter.IntegerValue{Val: 10})
	m.Set("c", &interpreter.IntegerValue{Val: 30})

	// 1. keys and values
	keysFn, _ := mapMod.Get("keys")
	valuesFn, _ := mapMod.Get("values")
	kRes := keysFn.(*interpreter.BuiltinFunction).Fn(m).(*interpreter.ListValue)
	vRes := valuesFn.(*interpreter.BuiltinFunction).Fn(m).(*interpreter.ListValue)
	if kRes.Len() != 3 || kRes.Elements[0].(*interpreter.StringValue).Val != "b" {
		t.Fatalf("map.keys failed: %v", kRes)
	}
	if vRes.Len() != 3 || vRes.Elements[0].(*interpreter.IntegerValue).Val != 20 {
		t.Fatalf("map.values failed: %v", vRes)
	}

	// 2. sort
	sortFn, _ := mapMod.Get("sort")
	sorted := sortFn.(*interpreter.BuiltinFunction).Fn(m).(*interpreter.MapValue)
	sKeys := sorted.Keys()
	if sKeys[0] != "a" || sKeys[1] != "b" || sKeys[2] != "c" {
		t.Fatalf("map.sort failed: %v", sKeys)
	}

	// 3. reverse
	revFn, _ := mapMod.Get("reverse")
	reversed := revFn.(*interpreter.BuiltinFunction).Fn(sorted).(*interpreter.MapValue)
	rKeys := reversed.Keys()
	if rKeys[0] != "c" || rKeys[1] != "b" || rKeys[2] != "a" {
		t.Fatalf("map.reverse failed: %v", rKeys)
	}

	// 4. filter
	filterFn, _ := mapMod.Get("filter")
	predicate := &interpreter.BuiltinFunction{
		Name: "gt_15",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			val := args[1].(*interpreter.IntegerValue).Val
			if val > 15 {
				return interpreter.TRUE
			}
			return interpreter.FALSE
		},
	}
	filtered := filterFn.(*interpreter.BuiltinFunction).Fn(m, predicate).(*interpreter.MapValue)
	if filtered.Len() != 2 || !filtered.Has("b") || !filtered.Has("c") {
		t.Fatalf("map.filter failed: %v", filtered)
	}

	// 5. map
	mapFn, _ := mapMod.Get("map")
	mapper := &interpreter.BuiltinFunction{
		Name: "add_5",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			val := args[1].(*interpreter.IntegerValue).Val
			return &interpreter.IntegerValue{Val: val + 5}
		},
	}
	mapped := mapFn.(*interpreter.BuiltinFunction).Fn(m, mapper).(*interpreter.MapValue)
	valB, _ := mapped.Get("b")
	if valB.(*interpreter.IntegerValue).Val != 25 {
		t.Fatalf("map.map failed: %v", valB)
	}

	// 6. reduce
	reduceFn, _ := mapMod.Get("reduce")
	reducer := &interpreter.BuiltinFunction{
		Name: "sum_vals",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			acc := args[0].(*interpreter.IntegerValue).Val
			val := args[2].(*interpreter.IntegerValue).Val
			return &interpreter.IntegerValue{Val: acc + val}
		},
	}
	reduced := reduceFn.(*interpreter.BuiltinFunction).Fn(m, reducer, &interpreter.IntegerValue{Val: 0})
	if reduced.(*interpreter.IntegerValue).Val != 60 {
		t.Fatalf("map.reduce failed: want 60, got %v", reduced)
	}
}

func TestStdStringExpanded(t *testing.T) {
	env := newTestEnv()
	strVal, _ := env.Get("std.string")
	strMod := strVal.(*interpreter.MapValue)

	padLeft, _ := strMod.Get("pad_left")
	padded := padLeft.(*interpreter.BuiltinFunction).Fn(
		&interpreter.StringValue{Val: "42"},
		&interpreter.IntegerValue{Val: 5},
		&interpreter.StringValue{Val: "0"},
	)
	if padded.(*interpreter.StringValue).Val != "00042" {
		t.Fatalf("pad_left failed: %v", padded)
	}

	countFn, _ := strMod.Get("count")
	cnt := countFn.(*interpreter.BuiltinFunction).Fn(
		&interpreter.StringValue{Val: "banana"},
		&interpreter.StringValue{Val: "an"},
	)
	if cnt.(*interpreter.IntegerValue).Val != 2 {
		t.Fatalf("count failed: %v", cnt)
	}

	lastIdxFn, _ := strMod.Get("last_index_of")
	lIdx := lastIdxFn.(*interpreter.BuiltinFunction).Fn(
		&interpreter.StringValue{Val: "banana"},
		&interpreter.StringValue{Val: "a"},
	)
	if lIdx.(*interpreter.IntegerValue).Val != 5 {
		t.Fatalf("last_index_of failed: %v", lIdx)
	}

	capFn, _ := strMod.Get("capitalize")
	capitalized := capFn.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: "hELLO"})
	if capitalized.(*interpreter.StringValue).Val != "Hello" {
		t.Fatalf("capitalize failed: %v", capitalized)
	}

	revFn, _ := strMod.Get("reverse")
	reversed := revFn.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: "alca"})
	if reversed.(*interpreter.StringValue).Val != "acla" {
		t.Fatalf("reverse failed: %v", reversed)
	}
}
