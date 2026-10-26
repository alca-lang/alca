// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package stdlib

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/alca-lang/alca/pkg/interpreter"
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

	text, ok := interpreter.ReadStdinLine()
	fmt.Println()
	if !ok {
		return "", fmt.Errorf("could not read password")
	}
	return text, nil
}
