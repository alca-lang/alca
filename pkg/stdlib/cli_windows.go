// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package stdlib

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"github.com/alca-lang/alca/pkg/interpreter"
)

// readPasswordSecure reads a line from the console without echoing characters.
func readPasswordSecure() (string, error) {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getConsoleMode := kernel32.NewProc("GetConsoleMode")
	setConsoleMode := kernel32.NewProc("SetConsoleMode")

	if getConsoleMode.Find() == nil && setConsoleMode.Find() == nil {
		var oldMode uint32
		handle := os.Stdin.Fd()
		ret, _, _ := getConsoleMode.Call(handle, uintptr(unsafe.Pointer(&oldMode)))
		if ret != 0 {
			const enableEchoInput = 0x0004
			setConsoleMode.Call(handle, uintptr(oldMode&^enableEchoInput))
			defer setConsoleMode.Call(handle, uintptr(oldMode))
		}
	}

	text, ok := interpreter.ReadStdinLine()
	fmt.Println()
	if !ok {
		return "", fmt.Errorf("could not read password")
	}
	return text, nil
}
