// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

//go:build !windows && !darwin && !linux && !freebsd && !openbsd && !netbsd

package stdlib

import (
	"os"
	"runtime"
)

func platformRelease() string   { return "unknown" }
func platformVersion() string   { return runtime.GOOS }
func platformProcessor() string { return "unknown" }
func platformNode() string {
	name, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return name
}
