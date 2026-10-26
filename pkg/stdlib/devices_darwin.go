// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

//go:build darwin

package stdlib

import (
	"fmt"
	"os/exec"
	"strings"
)

func listDevices(deviceType string) ([]Device, error) {
	if deviceType != "" && deviceType != "usb" {
		return nil, fmt.Errorf("unsupported device type %q", deviceType)
	}
	output, err := exec.Command("system_profiler", "SPUSBDataType").Output()
	if err != nil {
		return nil, fmt.Errorf("system_profiler failed: %w", err)
	}
	devices := make([]Device, 0)
	for index, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasSuffix(line, ":") {
			continue
		}
		name := strings.TrimSuffix(line, ":")
		devices = append(devices, Device{
			ID:    fmt.Sprintf("usb_%d", index),
			Name:  name,
			Class: "usb",
		})
	}
	return devices, nil
}

func getDevice(id string) (*Device, error) {
	devices, err := listDevices("")
	if err != nil {
		return nil, err
	}
	for i := range devices {
		if devices[i].ID == id {
			return &devices[i], nil
		}
	}
	return nil, nil
}
