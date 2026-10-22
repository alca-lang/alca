//go:build !windows

package jit

import (
	"fmt"
	"syscall"
	"unsafe"
)

// AllocExec allocates size bytes of executable, read-write memory using mmap.
func AllocExec(size int) (uintptr, error) {
	if size <= 0 {
		return 0, fmt.Errorf("invalid memory allocation size: %d", size)
	}
	prot := syscall.PROT_READ | syscall.PROT_WRITE | syscall.PROT_EXEC
	flags := syscall.MAP_ANON | syscall.MAP_PRIVATE
	data, err := syscall.Mmap(-1, 0, size, prot, flags)
	if err != nil {
		return 0, fmt.Errorf("mmap failed: %v", err)
	}
	if len(data) == 0 {
		return 0, fmt.Errorf("mmap returned empty slice")
	}
	return uintptr(unsafe.Pointer(&data[0])), nil
}

// FreeExec releases an executable memory region allocated by AllocExec.
func FreeExec(addr uintptr, size int) error {
	if addr == 0 || size <= 0 {
		return nil
	}
	var b []byte
	hdr := (*struct {
		Data uintptr
		Len  int
		Cap  int
	})(unsafe.Pointer(&b))
	hdr.Data = addr
	hdr.Len = size
	hdr.Cap = size
	return syscall.Munmap(b)
}
