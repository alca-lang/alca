//go:build windows

package stdlib

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"
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

	text, err := bufio.NewReader(os.Stdin).ReadString('\n')
	fmt.Println()
	if err != nil {
		return "", err
	}
	return strings.TrimRight(text, "\r\n"), nil
}
