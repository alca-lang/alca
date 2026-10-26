// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

//go:build (linux || freebsd || openbsd || netbsd) && !darwin

package stdlib

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func batteryPath() (string, error) {
	paths, err := filepath.Glob("/sys/class/power_supply/BAT*")
	if err != nil {
		return "", err
	}
	if len(paths) == 0 {
		return "", fmt.Errorf("no battery is present or supported")
	}
	return paths[0], nil
}

func readPowerFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func getBatteryStatus() (percent float64, charging bool, timeRemaining int64, err error) {
	path, err := batteryPath()
	if err != nil {
		return 0, false, 0, err
	}
	value, err := readPowerFile(filepath.Join(path, "capacity"))
	if err != nil {
		return 0, false, 0, err
	}
	percent, err = strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, false, 0, fmt.Errorf("invalid battery capacity: %w", err)
	}
	status, err := readPowerFile(filepath.Join(path, "status"))
	if err != nil {
		return 0, false, 0, err
	}
	return percent, strings.EqualFold(status, "Charging"), 0, nil
}

func getBatteryInfo() (capacity float64, cycles int64, health string, technology string, err error) {
	path, err := batteryPath()
	if err != nil {
		return 0, 0, "", "", err
	}
	capacityText, err := readPowerFile(filepath.Join(path, "capacity"))
	if err != nil {
		return 0, 0, "", "", err
	}
	capacity, err = strconv.ParseFloat(capacityText, 64)
	if err != nil {
		return 0, 0, "", "", fmt.Errorf("invalid battery capacity: %w", err)
	}
	cyclesText, cycleErr := readPowerFile(filepath.Join(path, "cycle_count"))
	if cycleErr == nil {
		cycles, _ = strconv.ParseInt(cyclesText, 10, 64)
	}
	technology, techErr := readPowerFile(filepath.Join(path, "technology"))
	if techErr != nil {
		technology = "unknown"
	}
	health = "unknown"
	design, designErr := readPowerFile(filepath.Join(path, "charge_full_design"))
	full, fullErr := readPowerFile(filepath.Join(path, "charge_full"))
	if designErr == nil && fullErr == nil {
		designValue, parseDesignErr := strconv.ParseFloat(design, 64)
		fullValue, parseFullErr := strconv.ParseFloat(full, 64)
		if parseDesignErr == nil && parseFullErr == nil && designValue > 0 {
			ratio := fullValue / designValue
			switch {
			case ratio >= 0.8:
				health = "good"
			case ratio >= 0.5:
				health = "degraded"
			default:
				health = "poor"
			}
		}
	}
	return capacity, cycles, health, technology, nil
}

func isPowerPlugged() (bool, error) {
	paths, err := filepath.Glob("/sys/class/power_supply/AC*/online")
	if err != nil {
		return false, err
	}
	if len(paths) > 0 {
		value, err := readPowerFile(paths[0])
		if err != nil {
			return false, err
		}
		return value == "1", nil
	}
	path, err := batteryPath()
	if err != nil {
		return false, err
	}
	status, err := readPowerFile(filepath.Join(path, "status"))
	if err != nil {
		return false, err
	}
	return strings.EqualFold(status, "Charging") || strings.EqualFold(status, "Full"), nil
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

func suspendSystem() error   { return exec.Command("systemctl", "suspend").Run() }
func hibernateSystem() error { return exec.Command("systemctl", "hibernate").Run() }
func rebootSystem() error    { return exec.Command("systemctl", "reboot").Run() }
func shutdownSystem() error  { return exec.Command("systemctl", "poweroff").Run() }

func getSensors() ([]Sensor, error) {
	output, err := exec.Command("sensors").Output()
	if err != nil {
		return nil, fmt.Errorf("sensors command failed: %w", err)
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
		name := strings.ToLower(sensor.Name)
		if strings.Contains(name, "core") || strings.Contains(name, "cpu") {
			temps = append(temps, sensor.Value)
		}
	}
	if len(temps) == 0 {
		return nil, fmt.Errorf("no CPU temperature sensors reported")
	}
	return temps, nil
}
