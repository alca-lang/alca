// Package stdlib provides the Foreign Function Interface (FFI) module for the Alca programming language.
// It allows dynamic loading of native shared libraries (.dll on Windows, .so on Linux, .dylib on macOS)
// and execution of native functions without CGO or external toolchains.
package stdlib

import (
	"fmt"
	"strings"
	"sync"
	"unsafe"

	"github.com/alca-lang/alca/pkg/interpreter"
)

var (
	allocatedPointersMu sync.Mutex
	allocatedPointers   = make(map[uintptr][]byte)
)

// RegisterFFIBuiltins registers "std.ffi" and "ffi" into the interpreter environment.
func RegisterFFIBuiltins(env *interpreter.Environment) {
	ffiModule := interpreter.NewMap()

	register := func(name string, fn func(args ...interpreter.Value) interpreter.Value) {
		builtin := &interpreter.BuiltinFunction{Name: name, Fn: fn}
		ffiModule.Set(name, builtin)
	}

	// ffi.load(library_path) -> FFILibrary
	register("load", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
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
				err := UnloadDynamicLibrary(libHandle)
				libMap.Set("is_loaded", interpreter.FALSE)
				if err != nil {
					return interpreter.NewError(500, err.Error())
				}
				return interpreter.TRUE
			},
		})

		return libMap
	})

	// ffi.load_embedded(name) -> FFILibrary: loads a native library that was
	// built into the program with `alca build --cc` or `--native-lib`. The
	// library is unpacked from the resources of the program and loaded from
	// the temporary directory.
	register("load_embedded", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
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
				err := UnloadDynamicLibrary(libHandle)
				libMap.Set("is_loaded", interpreter.FALSE)
				if err != nil {
					return interpreter.NewError(500, err.Error())
				}
				return interpreter.TRUE
			},
		})
		return libMap
	})

	// ffi.c_str(s) -> pointer (int)
	register("c_str", func(args ...interpreter.Value) interpreter.Value {
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
		if len(args) < 1 {
			return &interpreter.IntegerValue{Val: 0}
		}
		s := interpreter.CoerceToString(args[0])
		ptr := UTF16StringPointer(s)
		return &interpreter.IntegerValue{Val: int64(ptr)}
	})

	// ffi.ptr_to_str(ptr) -> string
	register("ptr_to_str", func(args ...interpreter.Value) interpreter.Value {
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
		return &interpreter.StringValue{Val: ReadCString(uintptr(ptr))}
	})

	// ffi.ptr_to_utf16(ptr) -> string
	register("ptr_to_utf16", func(args ...interpreter.Value) interpreter.Value {
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
		return &interpreter.StringValue{Val: ReadUTF16String(uintptr(ptr))}
	})

	// ffi.alloc(size_bytes) -> blob
	register("alloc", func(args ...interpreter.Value) interpreter.Value {
		size := int64(64)
		if len(args) >= 1 {
			if iv, ok := args[0].(*interpreter.IntegerValue); ok && iv.Val > 0 {
				size = iv.Val
			}
		}
		buf := make([]byte, size)
		return &interpreter.BlobValue{Data: buf}
	})

	// ffi.size_of(type_name) -> int
	register("size_of", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return &interpreter.IntegerValue{Val: 8}
		}
		t := strings.ToLower(interpreter.CoerceToString(args[0]))
		switch t {
		case "byte", "char", "int8", "uint8", "bool":
			return &interpreter.IntegerValue{Val: 1}
		case "short", "int16", "uint16":
			return &interpreter.IntegerValue{Val: 2}
		case "int", "int32", "uint32", "float":
			return &interpreter.IntegerValue{Val: 4}
		case "long", "int64", "uint64", "double", "ptr", "pointer", "size_t", "uintptr":
			return &interpreter.IntegerValue{Val: 8}
		default:
			return &interpreter.IntegerValue{Val: 8}
		}
	})

	env.Set("ffi", ffiModule)
	env.Set("std.ffi", ffiModule)
}
