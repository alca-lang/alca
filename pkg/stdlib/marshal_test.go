// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package stdlib_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/stdlib"
)

func TestMarshalModule(t *testing.T) {
	env := interpreter.NewEnvironment()
	stdlib.RegisterMarshalBuiltins(env)

	val, ok := env.Get("marshal")
	if !ok {
		t.Fatalf("expected 'marshal' module to be registered")
	}
	mod := val.(*interpreter.MapValue)

	dumpsFn, _ := mod.Get("dumps")
	loadsFn, _ := mod.Get("loads")
	dumpFn, _ := mod.Get("dump")
	loadFn, _ := mod.Get("load")

	// 1. Primitive types round-trip
	cases := []interpreter.Value{
		interpreter.NULL,
		interpreter.TRUE,
		interpreter.FALSE,
		&interpreter.IntegerValue{Val: 42},
		&interpreter.IntegerValue{Val: -999999999},
		&interpreter.FloatValue{Val: 3.1415926535},
		&interpreter.StringValue{Val: "Hello, Binary Marshal!"},
		&interpreter.BlobValue{Data: []byte{0x00, 0xFF, 0x42, 0x7F}},
	}

	for _, original := range cases {
		dumped := dumpsFn.(*interpreter.BuiltinFunction).Fn(original)
		if _, ok := dumped.(*interpreter.StringValue); !ok {
			t.Fatalf("expected dumps to return StringValue, got %T: %v", dumped, dumped)
		}
		loaded := loadsFn.(*interpreter.BuiltinFunction).Fn(dumped)
		if original == interpreter.NULL && loaded != interpreter.NULL {
			t.Fatalf("expected NULL, got %v", loaded)
		} else if origInt, ok := original.(*interpreter.IntegerValue); ok {
			if loaded.(*interpreter.IntegerValue).Val != origInt.Val {
				t.Fatalf("int mismatch: %d vs %d", origInt.Val, loaded.(*interpreter.IntegerValue).Val)
			}
		} else if origStr, ok := original.(*interpreter.StringValue); ok {
			if loaded.(*interpreter.StringValue).Val != origStr.Val {
				t.Fatalf("str mismatch: %s vs %s", origStr.Val, loaded.(*interpreter.StringValue).Val)
			}
		}
	}

	// 2. Nested Data Structures (List of Maps)
	itemMap := interpreter.NewMap()
	itemMap.Set("name", &interpreter.StringValue{Val: "Alca Audio"})
	itemMap.Set("version", &interpreter.IntegerValue{Val: 2})
	itemMap.Set("active", interpreter.TRUE)

	nestedList := interpreter.NewList(
		&interpreter.IntegerValue{Val: 100},
		itemMap,
		interpreter.NewList(&interpreter.FloatValue{Val: 1.23}, &interpreter.FloatValue{Val: 4.56}),
	)

	dumpedNested := dumpsFn.(*interpreter.BuiltinFunction).Fn(nestedList)
	loadedNested := loadsFn.(*interpreter.BuiltinFunction).Fn(dumpedNested)

	resList, ok := loadedNested.(*interpreter.ListValue)
	if !ok || resList.Len() != 3 {
		t.Fatalf("expected 3-element list, got %v", loadedNested)
	}
	resMap, ok := resList.ElementsSlice()[1].(*interpreter.MapValue)
	if !ok {
		t.Fatalf("expected element 1 to be Map, got %T", resList.ElementsSlice()[1])
	}
	nameVal, _ := resMap.Get("name")
	if nameVal.(*interpreter.StringValue).Val != "Alca Audio" {
		t.Fatalf("nested map field failed: %v", nameVal)
	}

	// 3. File dump & load
	tmpFile := filepath.Join(t.TempDir(), "test_marshal.bin")
	res := dumpFn.(*interpreter.BuiltinFunction).Fn(itemMap, &interpreter.StringValue{Val: tmpFile})
	if res != interpreter.TRUE {
		t.Fatalf("dump to file failed: %v", res)
	}

	loadedFromFile := loadFn.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: tmpFile})
	loadedFileMap, ok := loadedFromFile.(*interpreter.MapValue)
	if !ok {
		t.Fatalf("expected map from file, got %T", loadedFromFile)
	}
	verVal, _ := loadedFileMap.Get("version")
	if verVal.(*interpreter.IntegerValue).Val != 2 {
		t.Fatalf("expected version 2, got %v", verVal)
	}

	// Clean up check
	if _, err := os.Stat(tmpFile); err != nil {
		t.Fatalf("expected file to exist: %v", err)
	}
}
