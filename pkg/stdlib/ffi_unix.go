//go:build !windows

package stdlib

import (
	"fmt"
	"plugin"
	"unsafe"

	"github.com/alca-lang/alca/pkg/interpreter"
)

var openPlugins = make(map[uintptr]*plugin.Plugin)
var nextHandle uintptr = 1

func LoadDynamicLibrary(path string) (uintptr, error) {
	p, err := plugin.Open(path)
	if err != nil {
		return 0, err
	}
	h := nextHandle
	nextHandle++
	openPlugins[h] = p
	return h, nil
}

func UnloadDynamicLibrary(handle uintptr) error {
	delete(openPlugins, handle)
	return nil
}

func InvokeDynamicProc(handle uintptr, name string, args []interpreter.Value) (uintptr, error) {
	p, ok := openPlugins[handle]
	if !ok {
		return 0, fmt.Errorf("dynamic library not found or closed")
	}

	sym, err := p.Lookup(name)
	if err != nil {
		return 0, err
	}

	// If symbol is a function func(...interface{}) interface{}
	if fn, ok := sym.(func(...interface{}) interface{}); ok {
		var passed []interface{}
		for _, a := range args {
			passed = append(passed, a.Inspect())
		}
		_ = fn(passed...)
		return 0, nil
	}

	return 0, nil
}

func UTF16StringPointer(s string) uintptr {
	b := append([]byte(s), 0)
	return uintptr(unsafe.Pointer(&b[0]))
}

func ReadCString(ptr uintptr) string {
	if ptr == 0 {
		return ""
	}
	var b []byte
	for {
		ch := *(*byte)(unsafe.Pointer(ptr))
		if ch == 0 {
			break
		}
		b = append(b, ch)
		ptr++
	}
	return string(b)
}

func ReadUTF16String(ptr uintptr) string {
	return ReadCString(ptr)
}
