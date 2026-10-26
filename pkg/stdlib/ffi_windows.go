// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package stdlib

import (
	"fmt"
	"math"
	"strings"
	"syscall"
	"unsafe"

	"github.com/alca-lang/alca/pkg/interpreter"
	"golang.org/x/sys/windows"
)

func allocateNativeFFI(size int) ([]byte, error) {
	address, err := windows.VirtualAlloc(0, uintptr(size), windows.MEM_RESERVE|windows.MEM_COMMIT, windows.PAGE_READWRITE)
	if err != nil {
		return nil, err
	}
	if address == 0 {
		return nil, fmt.Errorf("VirtualAlloc returned a null address")
	}
	return unsafe.Slice((*byte)(foreignPointer(address)), size), nil
}

func freeNativeFFI(buffer []byte) error {
	if len(buffer) == 0 {
		return nil
	}
	address := uintptr(unsafe.Pointer(&buffer[0]))
	return windows.VirtualFree(address, 0, windows.MEM_RELEASE)
}

func LoadDynamicLibrary(path string) (uintptr, error) {
	h, err := syscall.LoadLibrary(path)
	if err != nil {
		return 0, err
	}
	return uintptr(h), nil
}

func UnloadDynamicLibrary(handle uintptr) error {
	if handle == 0 {
		return nil
	}
	return syscall.FreeLibrary(syscall.Handle(handle))
}

func InvokeDynamicProc(handle uintptr, name string, args []interpreter.Value) (uintptr, error) {
	if handle == 0 {
		return 0, fmt.Errorf("invalid library handle")
	}

	proc, err := syscall.GetProcAddress(syscall.Handle(handle), name)
	if err != nil {
		return 0, err
	}

	callArgs := make([]uintptr, len(args))
	isWide := strings.HasSuffix(name, "W")

	for i, arg := range args {
		switch v := arg.(type) {
		case *interpreter.IntegerValue:
			callArgs[i] = uintptr(v.Val)
		case *interpreter.FloatValue:
			callArgs[i] = uintptr(math.Float64bits(v.Val))
		case *interpreter.BoolValue:
			if v.Val {
				callArgs[i] = 1
			} else {
				callArgs[i] = 0
			}
		case *interpreter.StringValue:
			if isWide {
				ptr, err := syscall.UTF16PtrFromString(v.Val)
				if err != nil {
					return 0, err
				}
				callArgs[i] = uintptr(unsafe.Pointer(ptr))
			} else {
				b := append([]byte(v.Val), 0)
				callArgs[i] = uintptr(unsafe.Pointer(&b[0]))
			}
		case *interpreter.BlobValue:
			if len(v.Data) > 0 {
				callArgs[i] = uintptr(unsafe.Pointer(&v.Data[0]))
			} else {
				callArgs[i] = 0
			}
		case *interpreter.NullValue:
			callArgs[i] = 0
		default:
			// Coerce to integer or string pointer
			if iv, ok := arg.(*interpreter.IntegerValue); ok {
				callArgs[i] = uintptr(iv.Val)
			} else {
				s := interpreter.CoerceToString(arg)
				if isWide {
					ptr, _ := syscall.UTF16PtrFromString(s)
					callArgs[i] = uintptr(unsafe.Pointer(ptr))
				} else {
					b := append([]byte(s), 0)
					callArgs[i] = uintptr(unsafe.Pointer(&b[0]))
				}
			}
		}
	}

	r1, _, callErr := syscall.SyscallN(proc, callArgs...)
	// SyscallN returns errno if any, but many Win32 APIs return non-zero errno even on success
	// (e.g. ERROR_SUCCESS or previous thread error). r1 is the true function return value.
	_ = callErr
	return r1, nil
}

func UTF16StringPointer(s string) uintptr {
	u16, err := syscall.UTF16FromString(s)
	if err != nil {
		return 0
	}
	buffer := make([]byte, len(u16)*2)
	for i, value := range u16 {
		buffer[i*2] = byte(value)
		buffer[i*2+1] = byte(value >> 8)
	}
	ptr := uintptr(unsafe.Pointer(&buffer[0]))
	allocatedPointersMu.Lock()
	allocatedPointers[ptr] = buffer
	allocatedPointersMu.Unlock()
	return ptr
}

// foreignPointer converts an address received from native code into an
// unsafe.Pointer. unsafe.Add keeps the conversion explicit and vet-clean.
//
// The address belongs to a foreign allocation, so the race detector's checkptr
// instrumentation cannot validate it against the Go heap. This is the one place
// where that check must be suspended; callers only dereference addresses that
// native code returned or that UTF16StringPointer registered.
//
//go:nocheckptr
func foreignPointer(ptr uintptr) unsafe.Pointer {
	return unsafe.Add(unsafe.Pointer(nil), ptr)
}

func ReadCString(ptr uintptr) (string, error) {
	if ptr == 0 {
		return "", nil
	}
	var b []byte
	for len(b) < maxFFIAllocationSize {
		ch := *(*byte)(foreignPointer(ptr))
		if ch == 0 {
			return string(b), nil
		}
		b = append(b, ch)
		ptr++
	}
	return "", fmt.Errorf("C string exceeds %d bytes or is not NUL-terminated", maxFFIAllocationSize)
}

func ReadUTF16String(ptr uintptr) (string, error) {
	if ptr == 0 {
		return "", nil
	}
	var u16 []uint16
	for len(u16) < maxFFIAllocationSize/2 {
		ch := *(*uint16)(foreignPointer(ptr))
		if ch == 0 {
			return syscall.UTF16ToString(u16), nil
		}
		u16 = append(u16, ch)
		ptr += 2
	}
	return "", fmt.Errorf("UTF-16 string exceeds %d bytes or is not NUL-terminated", maxFFIAllocationSize)
}
