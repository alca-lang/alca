// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

// Package stdlib provides the Foreign Function Interface (FFI) module for the Alca programming language.
// It allows dynamic loading of native shared libraries (.dll on Windows, .so on Linux, .dylib on macOS)
// and execution of native functions without CGO or external toolchains.
package stdlib

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"unsafe"

	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/sandbox"
)

// NativeDLLBuilder is initialized by cmd/alca to compile Alca code into pure native PE DLLs.
var NativeDLLBuilder func(sourceCode string, outputPath string) error

var (
	allocatedPointersMu sync.Mutex
	allocatedPointers   = make(map[uintptr][]byte)
	nativePointers      = make(map[uintptr]bool)
)

const maxFFIAllocationSize = 16 << 20

// RegisterFFIBuiltins registers "std.ffi" and "ffi" into the interpreter environment.
func RegisterFFIBuiltins(env *interpreter.Environment) {
	ffiModule := interpreter.NewMap()

	register := func(name string, fn func(args ...interpreter.Value) interpreter.Value) {
		builtin := &interpreter.BuiltinFunction{Name: name, Fn: fn}
		ffiModule.Set(name, builtin)
	}

	// ffi.load(library_path) -> FFILibrary
	register("load", func(args ...interpreter.Value) interpreter.Value {
		if err := requireFFIPermission("ffi.load"); err != nil {
			return interpreter.NewError(403, err.Error())
		}
		if len(args) != 1 {
			return interpreter.NewError(400, "ffi.load requires a library path argument (e.g. 'user32.dll', 'raylib.dll', 'libm.so')")
		}
		path := interpreter.CoerceToString(args[0])

		libHandle, err := LoadDynamicLibrary(path)
		if err != nil {
			if IsPermissionError(err) {
				LogPermissionDenied("ffi_load", path, err)
				return interpreter.NewError(403, fmt.Sprintf("access denied loading library %q: %s", path, err.Error()))
			}
			return interpreter.NewError(500, fmt.Sprintf("failed to load library %q: %s", path, err.Error()))
		}

		libMap := interpreter.NewMap()
		libMap.Set("path", &interpreter.StringValue{Val: path})
		libMap.Set("is_loaded", interpreter.TRUE)

		// lib.call(func_name, ...args) -> int
		libMap.Set("call", &interpreter.BuiltinFunction{
			Name: "call",
			Fn: func(callArgs ...interpreter.Value) interpreter.Value {
				if err := requireFFIPermission("ffi.call"); err != nil {
					return interpreter.NewError(403, err.Error())
				}
				if len(callArgs) < 1 {
					return interpreter.NewError(400, "lib.call requires function name argument")
				}
				funcName := interpreter.CoerceToString(callArgs[0])
				passedArgs := callArgs[1:]

				res, err := InvokeDynamicProc(libHandle, funcName, passedArgs)
				if err != nil {
					if IsPermissionError(err) {
						LogPermissionDenied("ffi_call", fmt.Sprintf("%s::%s", path, funcName), err)
						return interpreter.NewError(403, fmt.Sprintf("access denied executing %q: %s", funcName, err.Error()))
					}
					return interpreter.NewError(500, fmt.Sprintf("call %q failed: %s", funcName, err.Error()))
				}
				return &interpreter.IntegerValue{Val: int64(res)}
			},
		})

		// lib.func(func_name) -> callable BuiltinFunction
		libMap.Set("func", &interpreter.BuiltinFunction{
			Name: "func",
			Fn: func(fArgs ...interpreter.Value) interpreter.Value {
				if err := requireFFIPermission("ffi.func"); err != nil {
					return interpreter.NewError(403, err.Error())
				}
				if len(fArgs) < 1 {
					return interpreter.NewError(400, "lib.func requires function name argument")
				}
				funcName := interpreter.CoerceToString(fArgs[0])
				return &interpreter.BuiltinFunction{
					Name: funcName,
					Fn: func(runArgs ...interpreter.Value) interpreter.Value {
						res, err := InvokeDynamicProc(libHandle, funcName, runArgs)
						if err != nil {
							if IsPermissionError(err) {
								LogPermissionDenied("ffi_call", fmt.Sprintf("%s::%s", path, funcName), err)
								return interpreter.NewError(403, fmt.Sprintf("access denied executing %q: %s", funcName, err.Error()))
							}
							return interpreter.NewError(500, fmt.Sprintf("call %q failed: %s", funcName, err.Error()))
						}
						return &interpreter.IntegerValue{Val: int64(res)}
					},
				}
			},
		})

		// lib.close() -> bool
		libMap.Set("close", &interpreter.BuiltinFunction{
			Name: "close",
			Fn: func(args ...interpreter.Value) interpreter.Value {
				if err := requireFFIPermission("ffi.close"); err != nil {
					return interpreter.NewError(403, err.Error())
				}
				err := UnloadDynamicLibrary(libHandle)
				if err != nil {
					return interpreter.NewError(500, err.Error())
				}
				libMap.Set("is_loaded", interpreter.FALSE)
				return interpreter.TRUE
			},
		})

		return libMap
	})

	// ffi.compile(source_code, [output_path]) -> string
	// Compiles C or C++ source code text into a native shared library (.dll / .so / .dylib)
	// and returns the path to the compiled library.
	register("compile", func(args ...interpreter.Value) interpreter.Value {
		if err := requireFFIPermission("ffi.compile"); err != nil {
			return interpreter.NewError(403, err.Error())
		}
		if len(args) < 1 {
			return interpreter.NewError(400, "ffi.compile requires source code as argument")
		}
		sourceCode := interpreter.CoerceToString(args[0])
		outputPath := ""
		if len(args) >= 2 {
			outputPath = interpreter.CoerceToString(args[1])
		}
		isCpp := strings.Contains(sourceCode, "class ") || strings.Contains(sourceCode, "template<") ||
			strings.Contains(sourceCode, "std::") || strings.Contains(sourceCode, "#include <iostream>")
		dllPath, err := CompileSharedLibraryFromSource(sourceCode, outputPath, isCpp)
		if err != nil {
			return interpreter.NewError(500, err.Error())
		}
		return &interpreter.StringValue{Val: dllPath}
	})

	// ffi.compile_file(source_path, [output_path]) -> string
	// Compiles a .c, .cpp, .cc, or .s file into a native shared library (.dll / .so / .dylib)
	// and returns the path to the compiled library.
	register("compile_file", func(args ...interpreter.Value) interpreter.Value {
		if err := requireFFIPermission("ffi.compile_file"); err != nil {
			return interpreter.NewError(403, err.Error())
		}
		if len(args) < 1 {
			return interpreter.NewError(400, "ffi.compile_file requires source file path as argument")
		}
		sourcePath := interpreter.CoerceToString(args[0])
		outputPath := ""
		if len(args) >= 2 {
			outputPath = interpreter.CoerceToString(args[1])
		}
		dllPath, err := CompileSharedLibrary(sourcePath, outputPath)
		if err != nil {
			return interpreter.NewError(500, err.Error())
		}
		return &interpreter.StringValue{Val: dllPath}
	})

	// ffi.build_native_dll(alca_source, output_path) -> string
	// Compiles Alca source code into a pure native Windows DLL directly with zero external compilers.
	register("build_native_dll", func(args ...interpreter.Value) interpreter.Value {
		if err := requireFFIPermission("ffi.build_native_dll"); err != nil {
			return interpreter.NewError(403, err.Error())
		}
		if len(args) < 2 {
			return interpreter.NewError(400, "ffi.build_native_dll requires alca_source and output_path")
		}
		sourceCode := interpreter.CoerceToString(args[0])
		outputPath := interpreter.CoerceToString(args[1])
		absOutput, err := filepath.Abs(outputPath)
		if err != nil {
			absOutput = outputPath
		}
		if NativeDLLBuilder == nil {
			return interpreter.NewError(500, "native DLL builder hook is not registered")
		}
		if err := NativeDLLBuilder(sourceCode, absOutput); err != nil {
			return interpreter.NewError(500, err.Error())
		}
		return &interpreter.StringValue{Val: absOutput}
	})

	// ffi.load_embedded(name) -> FFILibrary: loads a native library that was
	// built into the program with `alca build --cc` or `--native-lib`. The
	// library is unpacked from the resources of the program and loaded from
	// the temporary directory.
	register("load_embedded", func(args ...interpreter.Value) interpreter.Value {
		if err := requireFFIPermission("ffi.load_embedded"); err != nil {
			return interpreter.NewError(403, err.Error())
		}
		if len(args) != 1 {
			return interpreter.NewError(400, "ffi.load_embedded requires the name of a library")
		}
		name := interpreter.CoerceToString(args[0])
		path, err := ExtractEmbeddedLibrary(name)
		if err != nil {
			return interpreter.NewError(404, err.Error())
		}
		libHandle, err := LoadDynamicLibrary(path)
		if err != nil {
			if IsPermissionError(err) {
				LogPermissionDenied("ffi_load_embedded", name, err)
				return interpreter.NewError(403, fmt.Sprintf("access denied loading library %q: %s", name, err.Error()))
			}
			return interpreter.NewError(500, fmt.Sprintf("failed to load library %q: %s", name, err.Error()))
		}
		libMap := interpreter.NewMap()
		libMap.Set("path", &interpreter.StringValue{Val: name})
		libMap.Set("is_loaded", interpreter.TRUE)
		libMap.Set("call", &interpreter.BuiltinFunction{
			Name: "call",
			Fn: func(callArgs ...interpreter.Value) interpreter.Value {
				if err := requireFFIPermission("ffi.call"); err != nil {
					return interpreter.NewError(403, err.Error())
				}
				if len(callArgs) < 1 {
					return interpreter.NewError(400, "lib.call requires function name argument")
				}
				funcName := interpreter.CoerceToString(callArgs[0])
				res, err := InvokeDynamicProc(libHandle, funcName, callArgs[1:])
				if err != nil {
					return interpreter.NewError(500, fmt.Sprintf("call %q failed: %s", funcName, err.Error()))
				}
				return &interpreter.IntegerValue{Val: int64(res)}
			},
		})
		libMap.Set("func", &interpreter.BuiltinFunction{
			Name: "func",
			Fn: func(fArgs ...interpreter.Value) interpreter.Value {
				if err := requireFFIPermission("ffi.func"); err != nil {
					return interpreter.NewError(403, err.Error())
				}
				if len(fArgs) < 1 {
					return interpreter.NewError(400, "lib.func requires function name argument")
				}
				funcName := interpreter.CoerceToString(fArgs[0])
				return &interpreter.BuiltinFunction{
					Name: funcName,
					Fn: func(runArgs ...interpreter.Value) interpreter.Value {
						res, err := InvokeDynamicProc(libHandle, funcName, runArgs)
						if err != nil {
							return interpreter.NewError(500, fmt.Sprintf("call %q failed: %s", funcName, err.Error()))
						}
						return &interpreter.IntegerValue{Val: int64(res)}
					},
				}
			},
		})
		libMap.Set("close", &interpreter.BuiltinFunction{
			Name: "close",
			Fn: func(args ...interpreter.Value) interpreter.Value {
				if err := requireFFIPermission("ffi.close"); err != nil {
					return interpreter.NewError(403, err.Error())
				}
				err := UnloadDynamicLibrary(libHandle)
				if err != nil {
					return interpreter.NewError(500, err.Error())
				}
				libMap.Set("is_loaded", interpreter.FALSE)
				return interpreter.TRUE
			},
		})
		return libMap
	})

	// ffi.c_str(s) -> pointer (int)
	register("c_str", func(args ...interpreter.Value) interpreter.Value {
		if err := requireFFIPermission("ffi.c_str"); err != nil {
			return interpreter.NewError(403, err.Error())
		}
		if len(args) < 1 {
			return &interpreter.IntegerValue{Val: 0}
		}
		s := interpreter.CoerceToString(args[0])
		b := append([]byte(s), 0)
		ptr := uintptr(unsafe.Pointer(&b[0]))

		allocatedPointersMu.Lock()
		allocatedPointers[ptr] = b
		allocatedPointersMu.Unlock()

		return &interpreter.IntegerValue{Val: int64(ptr)}
	})

	// ffi.utf16_str(s) -> pointer (int)
	register("utf16_str", func(args ...interpreter.Value) interpreter.Value {
		if err := requireFFIPermission("ffi.utf16_str"); err != nil {
			return interpreter.NewError(403, err.Error())
		}
		if len(args) < 1 {
			return &interpreter.IntegerValue{Val: 0}
		}
		s := interpreter.CoerceToString(args[0])
		ptr := UTF16StringPointer(s)
		return &interpreter.IntegerValue{Val: int64(ptr)}
	})

	// ffi.ptr_to_str(ptr) -> string
	register("ptr_to_str", func(args ...interpreter.Value) interpreter.Value {
		if err := requireFFIPermission("ffi.ptr_to_str"); err != nil {
			return interpreter.NewError(403, err.Error())
		}
		if len(args) < 1 {
			return &interpreter.StringValue{Val: ""}
		}
		ptr := int64(0)
		if iv, ok := args[0].(*interpreter.IntegerValue); ok {
			ptr = iv.Val
		}
		if ptr == 0 {
			return &interpreter.StringValue{Val: ""}
		}
		value, err := ReadCString(uintptr(ptr))
		if err != nil {
			return interpreter.NewError(400, err.Error())
		}
		return &interpreter.StringValue{Val: value}
	})

	// ffi.ptr_to_utf16(ptr) -> string
	register("ptr_to_utf16", func(args ...interpreter.Value) interpreter.Value {
		if err := requireFFIPermission("ffi.ptr_to_utf16"); err != nil {
			return interpreter.NewError(403, err.Error())
		}
		if len(args) < 1 {
			return &interpreter.StringValue{Val: ""}
		}
		ptr := int64(0)
		if iv, ok := args[0].(*interpreter.IntegerValue); ok {
			ptr = iv.Val
		}
		if ptr == 0 {
			return &interpreter.StringValue{Val: ""}
		}
		value, err := ReadUTF16String(uintptr(ptr))
		if err != nil {
			return interpreter.NewError(400, err.Error())
		}
		return &interpreter.StringValue{Val: value}
	})

	// ffi.alloc(size_bytes) -> blob
	register("alloc", func(args ...interpreter.Value) interpreter.Value {
		size := int64(64)
		if len(args) >= 1 {
			iv, ok := args[0].(*interpreter.IntegerValue)
			if !ok || iv.Val <= 0 || iv.Val > maxFFIAllocationSize {
				return interpreter.NewError(400, "allocation size must be between 1 and 16777216 bytes")
			}
			size = iv.Val
		}
		if len(args) > 1 {
			return interpreter.NewError(400, "ffi.alloc accepts at most one size")
		}
		buf := make([]byte, int(size))
		return &interpreter.BlobValue{Data: buf}
	})

	// ffi.size_of(type_name) -> int
	register("size_of", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "ffi.size_of requires a type name")
		}
		t := strings.ToLower(interpreter.CoerceToString(args[0]))
		switch t {
		case "byte", "char", "int8", "uint8", "bool":
			return &interpreter.IntegerValue{Val: 1}
		case "short", "int16", "uint16":
			return &interpreter.IntegerValue{Val: 2}
		case "int", "int32", "uint32", "float":
			return &interpreter.IntegerValue{Val: 4}
		case "long":
			size := strconv.IntSize / 8
			if runtime.GOOS == "windows" {
				size = 4
			}
			return &interpreter.IntegerValue{Val: int64(size)}
		case "ptr", "pointer", "size_t", "uintptr":
			return &interpreter.IntegerValue{Val: int64(strconv.IntSize / 8)}
		case "int64", "uint64", "double":
			return &interpreter.IntegerValue{Val: 8}
		default:
			return interpreter.NewError(400, "unsupported FFI type: "+t)
		}
	})

	// ffi.alloc_ptr(size) -> pointer; unlike alloc, this returns an owned native pointer.
	register("alloc_ptr", func(args ...interpreter.Value) interpreter.Value {
		if err := requireFFIPermission("ffi.alloc_ptr"); err != nil {
			return interpreter.NewError(403, err.Error())
		}
		if len(args) != 1 {
			return interpreter.NewError(400, "ffi.alloc_ptr requires a byte size")
		}
		size, ok := args[0].(*interpreter.IntegerValue)
		if !ok || size.Val <= 0 || size.Val > maxFFIAllocationSize {
			return interpreter.NewError(400, "allocation size must be between 1 and 16777216 bytes")
		}
		buffer, err := allocateNativeFFI(int(size.Val))
		if err != nil {
			return interpreter.NewError(500, "native allocation failed: "+err.Error())
		}
		ptr := uintptr(unsafe.Pointer(&buffer[0]))
		allocatedPointersMu.Lock()
		allocatedPointers[ptr] = buffer
		nativePointers[ptr] = true
		allocatedPointersMu.Unlock()
		return &interpreter.IntegerValue{Val: int64(ptr)}
	})

	// ffi.free(pointer) releases memory allocated by alloc_ptr, c_str, or utf16_str.
	register("free", func(args ...interpreter.Value) interpreter.Value {
		if len(args) != 1 {
			return interpreter.NewError(400, "ffi.free requires a pointer")
		}
		ptrValue, ok := args[0].(*interpreter.IntegerValue)
		if !ok || ptrValue.Val <= 0 {
			return interpreter.NewError(400, "ffi.free requires a non-zero integer pointer")
		}
		ptr := uintptr(ptrValue.Val)
		allocatedPointersMu.Lock()
		buffer, exists := allocatedPointers[ptr]
		if exists {
			if nativePointers[ptr] {
				if err := freeNativeFFI(buffer); err != nil {
					allocatedPointersMu.Unlock()
					return interpreter.NewError(500, "native deallocation failed: "+err.Error())
				}
				delete(nativePointers, ptr)
			}
			delete(allocatedPointers, ptr)
		}
		allocatedPointersMu.Unlock()
		if !exists {
			return interpreter.NewError(400, "pointer is not owned by ffi")
		}
		return interpreter.TRUE
	})

	// ffi.copy_to_ptr(pointer, data) copies bytes into an ffi-owned allocation.
	register("copy_to_ptr", func(args ...interpreter.Value) interpreter.Value {
		if len(args) != 2 {
			return interpreter.NewError(400, "ffi.copy_to_ptr requires (pointer, data)")
		}
		ptrValue, ok := args[0].(*interpreter.IntegerValue)
		if !ok || ptrValue.Val <= 0 {
			return interpreter.NewError(400, "pointer must be a non-zero integer")
		}
		var data []byte
		switch value := args[1].(type) {
		case *interpreter.BlobValue:
			data = value.Data
		case *interpreter.StringValue:
			data = []byte(value.Val)
		default:
			return interpreter.NewError(400, "data must be a string or blob")
		}
		allocatedPointersMu.Lock()
		buffer, exists := allocatedPointers[uintptr(ptrValue.Val)]
		if exists && len(data) <= len(buffer) {
			copy(buffer, data)
		}
		allocatedPointersMu.Unlock()
		if !exists {
			return interpreter.NewError(400, "pointer is not owned by ffi")
		}
		if len(data) > len(buffer) {
			return interpreter.NewError(400, "data exceeds the allocated buffer size")
		}
		return interpreter.TRUE
	})

	// ffi.read_from_ptr(pointer, size) reads bytes from an ffi-owned allocation.
	register("read_from_ptr", func(args ...interpreter.Value) interpreter.Value {
		if len(args) != 2 {
			return interpreter.NewError(400, "ffi.read_from_ptr requires (pointer, size)")
		}
		ptrValue, pointerOK := args[0].(*interpreter.IntegerValue)
		sizeValue, sizeOK := args[1].(*interpreter.IntegerValue)
		if !pointerOK || ptrValue.Val <= 0 || !sizeOK || sizeValue.Val < 0 {
			return interpreter.NewError(400, "pointer and size must be non-negative integers")
		}
		allocatedPointersMu.Lock()
		buffer, exists := allocatedPointers[uintptr(ptrValue.Val)]
		if exists && sizeValue.Val <= int64(len(buffer)) {
			result := append([]byte(nil), buffer[:int(sizeValue.Val)]...)
			allocatedPointersMu.Unlock()
			return &interpreter.BlobValue{Data: result}
		}
		allocatedPointersMu.Unlock()
		if !exists {
			return interpreter.NewError(400, "pointer is not owned by ffi")
		}
		return interpreter.NewError(400, "requested size exceeds the allocated buffer")
	})

	env.Set("ffi", ffiModule)
	env.Set("std.ffi", ffiModule)
}

func requireFFIPermission(operation string) error {
	return sandbox.RequireFFI(operation)
}
