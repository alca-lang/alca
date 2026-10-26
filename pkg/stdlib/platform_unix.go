// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

//go:build (linux || freebsd || openbsd || netbsd) && !darwin

package stdlib

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// platformRelease returns Linux distribution version
func platformRelease() string {
	if runtime.GOOS != "linux" {
		output, err := exec.Command("sysctl", "-n", "kern.osrelease").Output()
		if err == nil {
			return strings.TrimSpace(string(output))
		}
		return "unknown"
	}
	if _, err := os.Stat("/etc/os-release"); err == nil {
		data, _ := os.ReadFile("/etc/os-release")
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "VERSION_ID=") {
				return strings.Trim(strings.Split(line, "=")[1], "\"")
			}
		}
	}
	return "unknown"
}

// platformVersion returns human-readable Linux distribution
func platformVersion() string {
	if runtime.GOOS != "linux" {
		return runtime.GOOS + " " + platformRelease()
	}
	if _, err := os.Stat("/etc/os-release"); err == nil {
		data, _ := os.ReadFile("/etc/os-release")
		name := ""
		version := ""
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "NAME=") {
				name = strings.Trim(strings.Split(line, "=")[1], "\"")
			}
			if strings.HasPrefix(line, "VERSION=") {
				version = strings.Trim(strings.Split(line, "=")[1], "\"")
			}
		}
		if name != "" {
			if version != "" {
				return name + " " + version
			}
			return name
		}
	}
	return "Linux"
}

// platformProcessor returns CPU model
func platformProcessor() string {
	if runtime.GOOS == "linux" {
		if data, err := os.ReadFile("/proc/cpuinfo"); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if strings.HasPrefix(line, "model name") {
					parts := strings.Split(line, ":")
					if len(parts) > 1 {
						return strings.TrimSpace(parts[1])
					}
				}
			}
		} else if output, err := exec.Command("sysctl", "-n", "hw.model").Output(); err == nil {
			return strings.TrimSpace(string(output))
		}
	}
	return "Unknown"
}

// platformNode returns hostname
func platformNode() string {
	hostname, _ := os.Hostname()
	return hostname
}
