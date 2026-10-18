//go:build !windows

package stdlib

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// readPasswordSecure reads a line from the terminal with echo disabled (stty).
// If stdin is not a terminal (pipe/CI) it simply reads the line.
func readPasswordSecure() (string, error) {
	restore := func() {}
	off := exec.Command("stty", "-echo")
	off.Stdin = os.Stdin
	if off.Run() == nil {
		restore = func() {
			on := exec.Command("stty", "echo")
			on.Stdin = os.Stdin
			_ = on.Run()
		}
	}
	defer restore()

	text, err := bufio.NewReader(os.Stdin).ReadString('\n')
	fmt.Println()
	if err != nil {
		return "", err
	}
	return strings.TrimRight(text, "\r\n"), nil
}
