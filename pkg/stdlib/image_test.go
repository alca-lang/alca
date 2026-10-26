// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package stdlib_test

import (
	"path/filepath"
	"testing"

	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/stdlib"
)

func TestImageModule(t *testing.T) {
	env := interpreter.NewEnvironment()
	stdlib.RegisterImageBuiltins(env)

	val, ok := env.Get("image")
	if !ok {
		t.Fatalf("expected 'image' module to be registered")
	}
	mod := val.(*interpreter.MapValue)

	createFn, _ := mod.Get("create")
	encodeFn, _ := mod.Get("encode")
	decodeFn, _ := mod.Get("decode")
	writeFn, _ := mod.Get("write")
	readFn, _ := mod.Get("read")
	toB64Fn, _ := mod.Get("to_base64")

	// 1. Create a 100x50 image with red color [255, 0, 0, 255]
	redColor := interpreter.NewList(
		&interpreter.IntegerValue{Val: 255},
		&interpreter.IntegerValue{Val: 0},
		&interpreter.IntegerValue{Val: 0},
		&interpreter.IntegerValue{Val: 255},
	)
	imgVal := createFn.(*interpreter.BuiltinFunction).Fn(
		&interpreter.IntegerValue{Val: 100},
		&interpreter.IntegerValue{Val: 50},
		redColor,
	)
	if imgVal.Type() != "IMAGE" {
		t.Fatalf("expected IMAGE value, got %s", imgVal.Type())
	}
	mp, ok := imgVal.(interpreter.MethodProvider)
	if !ok {
		t.Fatalf("expected MethodProvider, got %T", imgVal)
	}

	wVal, _ := mp.CallMethod("width", nil)
	hVal, _ := mp.CallMethod("height", nil)
	if wVal.(*interpreter.IntegerValue).Val != 100 || hVal.(*interpreter.IntegerValue).Val != 50 {
		t.Fatalf("size mismatch: %v x %v", wVal, hVal)
	}

	// 2. Set pixel at (10, 10) to blue
	mp.CallMethod("set_pixel", []interpreter.Value{
		&interpreter.IntegerValue{Val: 10},
		&interpreter.IntegerValue{Val: 10},
		&interpreter.IntegerValue{Val: 0},
		&interpreter.IntegerValue{Val: 0},
		&interpreter.IntegerValue{Val: 255},
		&interpreter.IntegerValue{Val: 255},
	})

	pxVal, _ := mp.CallMethod("pixel", []interpreter.Value{
		&interpreter.IntegerValue{Val: 10},
		&interpreter.IntegerValue{Val: 10},
	})
	pxList := pxVal.(*interpreter.ListValue).ElementsSlice()
	if pxList[2].(*interpreter.IntegerValue).Val != 255 {
		t.Fatalf("expected blue pixel at (10,10), got %v", pxVal)
	}

	// 3. Transformations: resize, grayscale, rotate
	resized, _ := mp.CallMethod("resize", []interpreter.Value{
		&interpreter.IntegerValue{Val: 200},
		&interpreter.IntegerValue{Val: 100},
	})
	rmp := resized.(interpreter.MethodProvider)
	rw, _ := rmp.CallMethod("width", nil)
	if rw.(*interpreter.IntegerValue).Val != 200 {
		t.Fatalf("expected resized width 200, got %v", rw)
	}

	gray, _ := mp.CallMethod("grayscale", nil)
	if gray.Type() != "IMAGE" {
		t.Fatalf("expected IMAGE from grayscale")
	}

	rot, _ := mp.CallMethod("rotate", []interpreter.Value{&interpreter.IntegerValue{Val: 90}})
	rotMP := rot.(interpreter.MethodProvider)
	rotW, _ := rotMP.CallMethod("width", nil)
	rotH, _ := rotMP.CallMethod("height", nil)
	if rotW.(*interpreter.IntegerValue).Val != 50 || rotH.(*interpreter.IntegerValue).Val != 100 {
		t.Fatalf("expected rotated 50x100, got %vx%v", rotW, rotH)
	}

	// 4. Encode & Decode PNG
	encBytesVal := encodeFn.(*interpreter.BuiltinFunction).Fn(imgVal, &interpreter.StringValue{Val: "png"})
	encBytes, ok := encBytesVal.(*interpreter.StringValue)
	if !ok || len(encBytes.Val) == 0 {
		t.Fatalf("encode failed: %v", encBytesVal)
	}

	decVal := decodeFn.(*interpreter.BuiltinFunction).Fn(encBytes)
	if decVal.Type() != "IMAGE" {
		t.Fatalf("decode failed: %v", decVal)
	}

	// 5. Base64
	b64Val := toB64Fn.(*interpreter.BuiltinFunction).Fn(imgVal, &interpreter.StringValue{Val: "png"})
	b64Str := b64Val.(*interpreter.StringValue).Val
	if len(b64Str) == 0 {
		t.Fatalf("to_base64 produced empty string")
	}

	decB64 := decodeFn.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: "data:image/png;base64," + b64Str})
	if decB64.Type() != "IMAGE" {
		t.Fatalf("decode from Data URI base64 failed")
	}

	// 6. File Save & Read
	tmpFile := filepath.Join(t.TempDir(), "test_img.png")
	wrRes := writeFn.(*interpreter.BuiltinFunction).Fn(imgVal, &interpreter.StringValue{Val: tmpFile})
	if wrRes != interpreter.TRUE {
		t.Fatalf("image.write failed: %v", wrRes)
	}

	readImg := readFn.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: tmpFile})
	if readImg.Type() != "IMAGE" {
		t.Fatalf("image.read failed: %v", readImg)
	}
}
