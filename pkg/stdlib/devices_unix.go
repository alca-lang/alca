// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

//go:build (linux || freebsd || openbsd || netbsd) && !darwin

package stdlib

import (
	"fmt"
	"os/exec"
	"strings"
)

func listDevices(deviceType string) ([]Device, error) {
	if deviceType != "" && deviceType != "pci" && deviceType != "usb" {
		return nil, fmt.Errorf("unsupported device type %q", deviceType)
	}
	devices := make([]Device, 0)
	var failures []string
	if deviceType == "" || deviceType == "pci" {
		output, err := exec.Command("lspci").Output()
		if err != nil {
			failures = append(failures, "lspci: "+err.Error())
		} else {
			for _, line := range strings.Split(string(output), "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				fields := strings.Fields(line)
				id := fields[0]
				devices = append(devices, Device{ID: "pci_" + id, Name: line, Class: "pci"})
			}
		}
	}
	if deviceType == "" || deviceType == "usb" {
		output, err := exec.Command("lsusb").Output()
		if err != nil {
			failures = append(failures, "lsusb: "+err.Error())
		} else {
			for index, line := range strings.Split(string(output), "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				devices = append(devices, Device{ID: fmt.Sprintf("usb_%d", index), Name: line, Class: "usb"})
			}
		}
	}
	if len(devices) == 0 && len(failures) > 0 {
		return nil, fmt.Errorf("device listing tools unavailable (%s)", strings.Join(failures, "; "))
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
