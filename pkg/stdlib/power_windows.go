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

type windowsBattery struct {
	EstimatedChargeRemaining *float64 `json:"EstimatedChargeRemaining"`
	BatteryStatus            *uint32  `json:"BatteryStatus"`
	DesignCapacity           *float64 `json:"DesignCapacity"`
	FullChargeCapacity       *float64 `json:"FullChargeCapacity"`
	Chemistry                *uint32  `json:"Chemistry"`
}

func queryWindowsBattery(properties string) (windowsBattery, error) {
	script := "Get-CimInstance -ClassName Win32_Battery | Select-Object -First 1 " + properties + " | ConvertTo-Json -Compress"
	output, err := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script).CombinedOutput()
	if err != nil {
		return windowsBattery{}, fmt.Errorf("battery query failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	var battery windowsBattery
	if err := json.Unmarshal(output, &battery); err != nil {
		return windowsBattery{}, fmt.Errorf("no supported battery was found")
	}
	if battery.BatteryStatus == nil && battery.EstimatedChargeRemaining == nil {
		return windowsBattery{}, fmt.Errorf("no supported battery was found")
	}
	return battery, nil
}

func getBatteryStatus() (percent float64, charging bool, timeRemaining int64, err error) {
	battery, err := queryWindowsBattery("EstimatedChargeRemaining,BatteryStatus")
	if err != nil {
		return 0, false, 0, err
	}
	if battery.EstimatedChargeRemaining != nil {
		percent = *battery.EstimatedChargeRemaining
	}
	if battery.BatteryStatus != nil {
		charging = *battery.BatteryStatus == 2 || *battery.BatteryStatus == 6 || *battery.BatteryStatus == 7 || *battery.BatteryStatus == 8 || *battery.BatteryStatus == 9
	}
	return percent, charging, 0, nil
}

func getBatteryInfo() (capacity float64, cycles int64, health string, technology string, err error) {
	battery, err := queryWindowsBattery("DesignCapacity,FullChargeCapacity,Chemistry")
	if err != nil {
		return 0, 0, "", "", err
	}
	health, technology = "unknown", "unknown"
	if battery.FullChargeCapacity != nil {
		capacity = *battery.FullChargeCapacity
	}
	if battery.DesignCapacity != nil && battery.FullChargeCapacity != nil && *battery.DesignCapacity > 0 {
		ratio := *battery.FullChargeCapacity / *battery.DesignCapacity
		switch {
		case ratio >= 0.8:
			health = "good"
		case ratio >= 0.5:
			health = "degraded"
		default:
			health = "poor"
		}
	}
	if battery.Chemistry != nil {
		technology = fmt.Sprintf("chemistry-%d", *battery.Chemistry)
	}
	return capacity, 0, health, technology, nil
}

func isPowerPlugged() (bool, error) {
	battery, err := queryWindowsBattery("BatteryStatus")
	if err != nil {
		return false, err
	}
	return battery.BatteryStatus != nil && *battery.BatteryStatus == 2, nil
}

func getPowerSource() (string, error) {
	plugged, err := isPowerPlugged()
	if err != nil {
		return "", err
	}
	if plugged {
		return "ac", nil
	}
	return "battery", nil
}

func suspendSystem() error {
	return exec.Command("rundll32.exe", "powrprof.dll,SetSuspendState", "0,1,0").Run()
}
func hibernateSystem() error { return exec.Command("shutdown", "/h").Run() }
func rebootSystem() error    { return exec.Command("shutdown", "/r", "/t", "0").Run() }
func shutdownSystem() error  { return exec.Command("shutdown", "/s", "/t", "0").Run() }

func getSensors() ([]Sensor, error) {
	script := "Get-CimInstance -ClassName Win32_TemperatureProbe | Select-Object Name,CurrentReading | ConvertTo-Json -Compress"
	output, err := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("temperature query failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	var probes []struct {
		Name           string   `json:"Name"`
		CurrentReading *float64 `json:"CurrentReading"`
	}
	if err := json.Unmarshal(output, &probes); err != nil {
		var single struct {
			Name           string   `json:"Name"`
			CurrentReading *float64 `json:"CurrentReading"`
		}
		if singleErr := json.Unmarshal(output, &single); singleErr != nil {
			return nil, fmt.Errorf("no temperature sensors were reported")
		}
		probes = []struct {
			Name           string   `json:"Name"`
			CurrentReading *float64 `json:"CurrentReading"`
		}{single}
	}
	sensors := make([]Sensor, 0, len(probes))
	for _, probe := range probes {
		if probe.CurrentReading == nil || *probe.CurrentReading == 0 {
			continue
		}
		sensors = append(sensors, Sensor{
			Name:  probe.Name,
			Type:  "temperature",
			Value: *probe.CurrentReading/10 - 273.15,
			Unit:  "C",
		})
	}
	if len(sensors) == 0 {
		return nil, fmt.Errorf("no temperature sensors were reported")
	}
	return sensors, nil
}

func getCpuTemps() ([]float64, error) {
	sensors, err := getSensors()
	if err != nil {
		return nil, err
	}
	temps := make([]float64, 0)
	for _, sensor := range sensors {
		if strings.Contains(strings.ToLower(sensor.Name), "cpu") {
			temps = append(temps, sensor.Value)
		}
	}
	if len(temps) == 0 {
		return nil, fmt.Errorf("no CPU temperature sensors reported")
	}
	return temps, nil
}
