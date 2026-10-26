// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package stdlib

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32             = windows.NewLazySystemDLL("kernel32.dll")
	procGetComputerNameW = kernel32.NewProc("GetComputerNameW")
)

// platformRelease returns Windows version
func platformRelease() string {
	v := windows.RtlGetVersion()
	return fmt.Sprintf("%d.%d.%d", v.MajorVersion, v.MinorVersion, v.BuildNumber)
}

// platformVersion returns human-readable Windows version
func platformVersion() string {
	v := windows.RtlGetVersion()
	return fmt.Sprintf("Windows %d.%d (build %d)", v.MajorVersion, v.MinorVersion, v.BuildNumber)
}

// platformProcessor returns CPU model name
func platformProcessor() string {
	if processor := os.Getenv("PROCESSOR_IDENTIFIER"); processor != "" {
		return processor
	}
	return "Unknown"
}

// platformNode returns hostname
func platformNode() string {
	var n uint32 = 256
	buf := make([]uint16, 256)
	procGetComputerNameW.Call(
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&n)),
	)
	return syscall.UTF16ToString(buf[:n])
}
