// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package stdlib

import (
	"encoding/binary"
	"fmt"
	"plugin"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"github.com/alca-lang/alca/pkg/interpreter"
)

var (
	openPluginsMu sync.RWMutex
	openPlugins   = make(map[uintptr]*plugin.Plugin)
)
var nextHandle uintptr = 1

func allocateNativeFFI(size int) ([]byte, error) {
	return syscall.Mmap(-1, 0, size, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_PRIVATE|syscall.MAP_ANON)
}

func freeNativeFFI(buffer []byte) error {
	return syscall.Munmap(buffer)
}

func LoadDynamicLibrary(path string) (uintptr, error) {
	p, err := plugin.Open(path)
	if err != nil {
		return 0, err
	}
	openPluginsMu.Lock()
	defer openPluginsMu.Unlock()
	h := nextHandle
	nextHandle++
	openPlugins[h] = p
	return h, nil
}

func UnloadDynamicLibrary(handle uintptr) error {
	openPluginsMu.RLock()
	_, ok := openPlugins[handle]
	openPluginsMu.RUnlock()
	if !ok {
		return fmt.Errorf("dynamic library not found or closed")
	}
	return fmt.Errorf("Go plugins cannot be unloaded")
}

func InvokeDynamicProc(handle uintptr, name string, args []interpreter.Value) (uintptr, error) {
	openPluginsMu.RLock()
	p, ok := openPlugins[handle]
	openPluginsMu.RUnlock()
	if !ok {
		return 0, fmt.Errorf("dynamic library not found or closed")
	}

	sym, err := p.Lookup(name)
	if err != nil {
		return 0, err
	}

	fn, ok := sym.(func(...interface{}) interface{})
	if !ok {
		return 0, fmt.Errorf("symbol %q is not a supported Go plugin function; Unix FFI supports func(...interface{}) interface{} only", name)
	}
	passed := make([]interface{}, len(args))
	for i, arg := range args {
		passed[i] = arg.Inspect()
	}
	result := fn(passed...)
	switch value := result.(type) {
	case nil:
		return 0, nil
	case int:
		return uintptr(value), nil
	case int32:
		return uintptr(value), nil
	case int64:
		return uintptr(value), nil
	case uint:
		return uintptr(value), nil
	case uint32:
		return uintptr(value), nil
	case uint64:
		return uintptr(value), nil
	case uintptr:
		return value, nil
	case bool:
		if value {
			return 1, nil
		}
		return 0, nil
	case string:
		parsed, err := strconv.ParseUint(strings.TrimSpace(value), 0, strconv.IntSize)
		if err != nil {
			return 0, fmt.Errorf("plugin function %q returned a non-numeric string", name)
		}
		return uintptr(parsed), nil
	default:
		return 0, fmt.Errorf("plugin function %q returned unsupported type %T", name, result)
	}
}

func UTF16StringPointer(s string) uintptr {
	units := utf16.Encode([]rune(s))
	b := make([]byte, (len(units)+1)*2)
	for i, unit := range units {
		binary.LittleEndian.PutUint16(b[i*2:], unit)
	}
	ptr := uintptr(unsafe.Pointer(&b[0]))
	allocatedPointersMu.Lock()
	allocatedPointers[ptr] = b
	allocatedPointersMu.Unlock()
	return ptr
}

func ReadCString(ptr uintptr) (string, error) {
	if ptr == 0 {
		return "", nil
	}
	var b []byte
	for len(b) < maxFFIAllocationSize {
		ch := *(*byte)(unsafe.Pointer(ptr))
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
	units := make([]uint16, 0, 64)
	for len(units) < maxFFIAllocationSize/2 {
		unit := *(*uint16)(unsafe.Pointer(ptr))
		if unit == 0 {
			return string(utf16.Decode(units)), nil
		}
		units = append(units, unit)
		ptr += 2
	}
	return "", fmt.Errorf("UTF-16 string exceeds %d bytes or is not NUL-terminated", maxFFIAllocationSize)
}
