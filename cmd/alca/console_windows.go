//go:build windows

package main

import (
	"os"
	"syscall"
	"unsafe"
)

// enableUTF8Console switches the console code page to UTF-8 and turns on
// virtual terminal processing, so prompts and program output with non-ASCII
// characters (Cyrillic, emoji, box drawing) are rendered correctly and ANSI
// colours work.
func enableUTF8Console() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")

	if setCP := kernel32.NewProc("SetConsoleOutputCP"); setCP.Find() == nil {
		setCP.Call(uintptr(65001))
	}
	if setCP := kernel32.NewProc("SetConsoleCP"); setCP.Find() == nil {
		setCP.Call(uintptr(65001))
	}

	const enableVirtualTerminalProcessing = 0x0004
	getMode := kernel32.NewProc("GetConsoleMode")
	setMode := kernel32.NewProc("SetConsoleMode")
	for _, handle := range []uintptr{os.Stdout.Fd(), os.Stderr.Fd()} {
		var mode uint32
		ret, _, _ := getMode.Call(handle, uintptr(unsafe.Pointer(&mode)))
		if ret == 0 {
			continue
		}
		setMode.Call(handle, uintptr(mode|enableVirtualTerminalProcessing))
	}
}
