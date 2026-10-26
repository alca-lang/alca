// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

//go:build darwin

package stdlib

import (
	"os"
	"os/exec"
	"strings"
)

// platformRelease returns macOS version
func platformRelease() string {
	cmd := exec.Command("sw_vers", "-productVersion")
	output, _ := cmd.Output()
	release := strings.TrimSpace(string(output))
	if release == "" {
		return "unknown"
	}
	return release
}

// platformVersion returns human-readable macOS version
func platformVersion() string {
	productCmd := exec.Command("sw_vers", "-productName")
	productOutput, _ := productCmd.Output()
	productName := strings.TrimSpace(string(productOutput))

	versionCmd := exec.Command("sw_vers", "-productVersion")
	versionOutput, _ := versionCmd.Output()
	version := strings.TrimSpace(string(versionOutput))
	if productName == "" || version == "" {
		return "macOS " + platformRelease()
	}

	return productName + " " + version
}

// platformProcessor returns CPU model
func platformProcessor() string {
	cmd := exec.Command("sysctl", "-n", "machdep.cpu.brand_string")
	output, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(output))
	}
	cmd = exec.Command("sysctl", "-n", "hw.model")
	output, err = cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(output))
	}
	return "Unknown"
}

// platformNode returns hostname
func platformNode() string {
	hostname, _ := os.Hostname()
	return hostname
}
