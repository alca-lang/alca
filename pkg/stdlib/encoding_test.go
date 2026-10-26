// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package stdlib_test

import (
	"testing"

	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/stdlib"
)

func TestEncodingModule(t *testing.T) {
	env := interpreter.NewEnvironment()
	stdlib.RegisterEncodingBuiltins(env)

	val, ok := env.Get("encoding")
	if !ok {
		t.Fatalf("expected 'encoding' module to be registered")
	}
	mod, ok := val.(*interpreter.MapValue)
	if !ok {
		t.Fatalf("expected map, got %T", val)
	}

	encodeFn, _ := mod.Get("encode")
	decodeFn, _ := mod.Get("decode")
	listFn, _ := mod.Get("list_encodings")
	detectFn, _ := mod.Get("detect")
	toU16Fn, _ := mod.Get("to_utf16")
	fromU16Fn, _ := mod.Get("from_utf16")

	// 1. List encodings
	listVal := listFn.(*interpreter.BuiltinFunction).Fn()
	lv, ok := listVal.(*interpreter.ListValue)
	if !ok || lv.Len() < 20 {
		t.Fatalf("expected at least 20 encodings in list, got %v", listVal)
	}

	// 2. Windows-1251 encode & decode
	cyrillicText := "Привет, мир! 123"
	encVal := encodeFn.(*interpreter.BuiltinFunction).Fn(
		&interpreter.StringValue{Val: cyrillicText},
		&interpreter.StringValue{Val: "windows-1251"},
	)
	encStr, ok := encVal.(*interpreter.StringValue)
	if !ok {
		t.Fatalf("encode failed: %v", encVal)
	}
	// "Привет" in CP1251 should not be equal to UTF-8 text
	if encStr.Val == cyrillicText {
		t.Errorf("CP1251 encoded bytes should differ from UTF-8 string")
	}

	decVal := decodeFn.(*interpreter.BuiltinFunction).Fn(
		encStr,
		&interpreter.StringValue{Val: "cp1251"},
	)
	decStr, ok := decVal.(*interpreter.StringValue)
	if !ok || decStr.Val != cyrillicText {
		t.Errorf("expected roundtrip %q, got %q", cyrillicText, decVal.Inspect())
	}

	// 3. CP866 roundtrip
	enc866 := encodeFn.(*interpreter.BuiltinFunction).Fn(
		&interpreter.StringValue{Val: cyrillicText},
		&interpreter.StringValue{Val: "cp866"},
	)
	dec866 := decodeFn.(*interpreter.BuiltinFunction).Fn(
		enc866,
		&interpreter.StringValue{Val: "cp866"},
	)
	if dec866.(*interpreter.StringValue).Val != cyrillicText {
		t.Errorf("CP866 roundtrip failed: got %q", dec866.(*interpreter.StringValue).Val)
	}

	// 4. Shift-JIS
	japaneseText := "こんにちは世界"
	encJIS := encodeFn.(*interpreter.BuiltinFunction).Fn(
		&interpreter.StringValue{Val: japaneseText},
		&interpreter.StringValue{Val: "shift-jis"},
	)
	decJIS := decodeFn.(*interpreter.BuiltinFunction).Fn(
		encJIS,
		&interpreter.StringValue{Val: "shift-jis"},
	)
	if decJIS.(*interpreter.StringValue).Val != japaneseText {
		t.Errorf("Shift-JIS roundtrip failed: got %q", decJIS.(*interpreter.StringValue).Val)
	}

	// 5. UTF-16 conversions
	u16List := toU16Fn.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: "Alca 2026"})
	backStr := fromU16Fn.(*interpreter.BuiltinFunction).Fn(u16List)
	if backStr.(*interpreter.StringValue).Val != "Alca 2026" {
		t.Errorf("UTF-16 roundtrip failed: got %q", backStr.Inspect())
	}

	// 6. Detect encoding
	detVal := detectFn.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: "\xef\xbb\xbfHello"})
	if detVal.(*interpreter.StringValue).Val != "utf-8" {
		t.Errorf("expected utf-8 BOM detection, got %q", detVal.(*interpreter.StringValue).Val)
	}
}
