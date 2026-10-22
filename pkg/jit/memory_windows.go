//go:build windows

package jit

import (
	"fmt"
	"syscall"
)

var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	procVirtualAlloc = kernel32.NewProc("VirtualAlloc")
	procVirtualFree  = kernel32.NewProc("VirtualFree")
)

const (
	memCommit            = 0x1000
	memReserve           = 0x2000
	memRelease           = 0x8000
	pageExecuteReadWrite = 0x40
)

// AllocExec allocates size bytes of executable, read-write memory.
func AllocExec(size int) (uintptr, error) {
	if size <= 0 {
		return 0, fmt.Errorf("invalid memory allocation size: %d", size)
	}
	addr, _, err := procVirtualAlloc.Call(
		0,
		uintptr(size),
		memCommit|memReserve,
		pageExecuteReadWrite,
	)
	if addr == 0 {
		return 0, fmt.Errorf("VirtualAlloc failed: %v", err)
	}
	return addr, nil
}

// FreeExec releases an executable memory region.
func FreeExec(addr uintptr, size int) error {
	if addr == 0 {
		return nil
	}
	r, _, err := procVirtualFree.Call(addr, 0, memRelease)
	if r == 0 {
		return fmt.Errorf("VirtualFree failed: %v", err)
	}
	return nil
}
