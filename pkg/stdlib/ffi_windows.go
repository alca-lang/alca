//go:build windows

package stdlib

import (
	"fmt"
	"math"
	"strings"
	"syscall"
	"unsafe"

	"github.com/alca-lang/alca/pkg/interpreter"
)

var (
	allocatedUTF16Mu syncMutexHolder
	allocatedUTF16   = make(map[uintptr][]uint16)
)

type syncMutexHolder struct{}

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
	ptr := uintptr(unsafe.Pointer(&u16[0]))
	allocatedPointersMu.Lock()
	allocatedUTF16[ptr] = u16
	allocatedPointersMu.Unlock()
	return ptr
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
	if ptr == 0 {
		return ""
	}
	var u16 []uint16
	for {
		ch := *(*uint16)(unsafe.Pointer(ptr))
		if ch == 0 {
			break
		}
		u16 = append(u16, ch)
		ptr += 2
	}
	return syscall.UTF16ToString(u16)
}
