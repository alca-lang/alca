// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

//go:build darwin

package stdlib

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func getBatteryStatus() (percent float64, charging bool, timeRemaining int64, err error) {
	output, err := exec.Command("pmset", "-g", "batt").Output()
	if err != nil {
		return 0, false, 0, fmt.Errorf("pmset battery query failed: %w", err)
	}
	found := false
	for _, line := range strings.Split(string(output), "\n") {
		if !strings.Contains(line, "InternalBattery") {
			continue
		}
		found = true
		for _, field := range strings.Fields(line) {
			if strings.HasSuffix(field, "%") {
				percent, err = strconv.ParseFloat(strings.TrimSuffix(field, "%"), 64)
				if err != nil {
					return 0, false, 0, fmt.Errorf("invalid battery percentage: %w", err)
				}
			}
			if field == "charging" {
				charging = true
			}
		}
	}
	if !found {
		return 0, false, 0, fmt.Errorf("no supported battery was found")
	}
	return percent, charging, 0, nil
}

func getBatteryInfo() (capacity float64, cycles int64, health string, technology string, err error) {
	output, err := exec.Command("system_profiler", "SPPowerDataType").Output()
	if err != nil {
		return 0, 0, "", "", fmt.Errorf("system_profiler battery query failed: %w", err)
	}
	health, technology = "unknown", "unknown"
	found := false
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "Cycle Count:"):
			cycles, _ = strconv.ParseInt(strings.TrimSpace(strings.TrimPrefix(line, "Cycle Count:")), 10, 64)
			found = true
		case strings.HasPrefix(line, "Full Charge Capacity (mAh):"):
			capacity, _ = strconv.ParseFloat(strings.TrimSpace(strings.TrimPrefix(line, "Full Charge Capacity (mAh):")), 64)
			found = true
		case strings.HasPrefix(line, "Condition:"):
			health = strings.TrimSpace(strings.TrimPrefix(line, "Condition:"))
			found = true
		case strings.HasPrefix(line, "Manufacturer:"):
			technology = strings.TrimSpace(strings.TrimPrefix(line, "Manufacturer:"))
			found = true
		}
	}
	if !found {
		return 0, 0, "", "", fmt.Errorf("no supported battery information was reported")
	}
	return capacity, cycles, health, technology, nil
}

func isPowerPlugged() (bool, error) {
	output, err := exec.Command("pmset", "-g", "ps").Output()
	if err != nil {
		return false, fmt.Errorf("pmset power query failed: %w", err)
	}
	text := string(output)
	if strings.Contains(text, "AC Power") {
		return true, nil
	}
	if strings.Contains(text, "Battery Power") {
		return false, nil
	}
	return false, fmt.Errorf("power source was not reported")
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

func suspendSystem() error   { return exec.Command("pmset", "sleepnow").Run() }
func hibernateSystem() error { return exec.Command("pmset", "hibernatenow").Run() }
func rebootSystem() error    { return exec.Command("shutdown", "-r", "now").Run() }
func shutdownSystem() error  { return exec.Command("shutdown", "-h", "now").Run() }

func getSensors() ([]Sensor, error) {
	output, err := exec.Command("system_profiler", "SPTemperatureDataType").Output()
	if err != nil {
		return nil, fmt.Errorf("system_profiler temperature query failed: %w", err)
	}
	sensors := make([]Sensor, 0)
	for _, line := range strings.Split(string(output), "\n") {
		if !strings.Contains(line, "°C") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		value := strings.TrimSuffix(fields[1], "°C")
		temperature, err := strconv.ParseFloat(value, 64)
		if err != nil {
			continue
		}
		sensors = append(sensors, Sensor{
			Name:  strings.TrimSuffix(fields[0], ":"),
			Type:  "temperature",
			Value: temperature,
			Unit:  "C",
		})
	}
	if len(sensors) == 0 {
		return nil, fmt.Errorf("no temperature sensors reported")
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
