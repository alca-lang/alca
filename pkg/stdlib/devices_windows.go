// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package stdlib

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type windowsDevice struct {
	ID    string `json:"InstanceId"`
	Name  string `json:"FriendlyName"`
	Class string `json:"Class"`
}

func listDevices(deviceType string) ([]Device, error) {
	script := "Get-PnpDevice -PresentOnly | Select-Object InstanceId,FriendlyName,Class | ConvertTo-Json -Compress"
	output, err := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("Get-PnpDevice failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	var entries []windowsDevice
	if err := json.Unmarshal(output, &entries); err != nil {
		var single windowsDevice
		if singleErr := json.Unmarshal(output, &single); singleErr != nil {
			return nil, fmt.Errorf("invalid Get-PnpDevice response: %w", err)
		}
		entries = []windowsDevice{single}
	}
	devices := make([]Device, 0, len(entries))
	for _, entry := range entries {
		if deviceType != "" && !strings.EqualFold(deviceType, entry.Class) {
			continue
		}
		devices = append(devices, Device{
			ID:    entry.ID,
			Name:  entry.Name,
			Class: entry.Class,
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
