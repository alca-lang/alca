// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

// Package stdlib provides the Power module for battery and power management.
package stdlib

import (
	"strconv"
	"strings"

	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/sandbox"
)

// RegisterPowerBuiltins registers the "std.power" and "power" modules.
func RegisterPowerBuiltins(env *interpreter.Environment) {
	powerModule := interpreter.NewMap()

	register := func(name string, fn func(args ...interpreter.Value) interpreter.Value) {
		powerModule.Set(name, &interpreter.BuiltinFunction{Name: name, Fn: fn})
	}

	// battery_status() -> map {percent, charging, time_remaining}
	register("battery_status", powerBatteryStatus)

	// battery_info() -> map {capacity, cycles, health, technology}
	register("battery_info", powerBatteryInfo)

	// power_plugged() -> bool
	register("power_plugged", powerPowerPlugged)

	// power_source() -> string
	register("power_source", powerPowerSource)

	// suspend() -> bool
	register("suspend", powerSuspend)

	// hibernate() -> bool
	register("hibernate", powerHibernate)

	// reboot() -> bool
	register("reboot", powerReboot)

	// shutdown() -> bool
	register("shutdown", powerShutdown)

	// sensors() -> list[map {name, type, value, unit}]
	register("sensors", powerSensors)

	// cpu_temp() -> list[float]
	register("cpu_temp", powerCpuTemp)

	env.Set("power", powerModule)
	env.Set("std.power", powerModule)
}

// powerBatteryStatus returns battery status
func powerBatteryStatus(args ...interpreter.Value) interpreter.Value {
	percent, charging, timeRemaining, err := getBatteryStatus()
	if err != nil {
		return interpreter.NewError(501, "battery status unavailable: "+err.Error())
	}
	result := interpreter.NewMap()
	result.Set("percent", &interpreter.FloatValue{Val: percent})
	result.Set("charging", &interpreter.BoolValue{Val: charging})
	result.Set("time_remaining", &interpreter.IntegerValue{Val: int64(timeRemaining)})
	return result
}

// powerBatteryInfo returns detailed battery information
func powerBatteryInfo(args ...interpreter.Value) interpreter.Value {
	capacity, cycles, health, technology, err := getBatteryInfo()
	if err != nil {
		return interpreter.NewError(501, "battery information unavailable: "+err.Error())
	}
	result := interpreter.NewMap()
	result.Set("capacity", &interpreter.FloatValue{Val: capacity})
	result.Set("cycles", &interpreter.IntegerValue{Val: int64(cycles)})
	result.Set("health", &interpreter.StringValue{Val: health})
	result.Set("technology", &interpreter.StringValue{Val: technology})
	return result
}

// powerPowerPlugged returns if AC power is connected
func powerPowerPlugged(args ...interpreter.Value) interpreter.Value {
	plugged, err := isPowerPlugged()
	if err != nil {
		return interpreter.NewError(501, "power source unavailable: "+err.Error())
	}
	return &interpreter.BoolValue{Val: plugged}
}

// powerPowerSource returns current power source
func powerPowerSource(args ...interpreter.Value) interpreter.Value {
	source, err := getPowerSource()
	if err != nil {
		return interpreter.NewError(501, "power source unavailable: "+err.Error())
	}
	return &interpreter.StringValue{Val: source}
}

// powerSuspend suspends the system
func powerSuspend(args ...interpreter.Value) interpreter.Value {
	if err := requireSystemControl(); err != nil {
		return interpreter.NewError(403, err.Error())
	}
	err := suspendSystem()
	if err != nil {
		return interpreter.NewError(500, "suspend failed: "+err.Error())
	}
	return &interpreter.BoolValue{Val: true}
}

// powerHibernate hibernates the system
func powerHibernate(args ...interpreter.Value) interpreter.Value {
	if err := requireSystemControl(); err != nil {
		return interpreter.NewError(403, err.Error())
	}
	err := hibernateSystem()
	if err != nil {
		return interpreter.NewError(500, "hibernate failed: "+err.Error())
	}
	return &interpreter.BoolValue{Val: true}
}

// powerReboot reboots the system
func powerReboot(args ...interpreter.Value) interpreter.Value {
	if err := requireSystemControl(); err != nil {
		return interpreter.NewError(403, err.Error())
	}
	err := rebootSystem()
	if err != nil {
		return interpreter.NewError(500, "reboot failed: "+err.Error())
	}
	return &interpreter.BoolValue{Val: true}
}

// powerShutdown shuts down the system
func powerShutdown(args ...interpreter.Value) interpreter.Value {
	if err := requireSystemControl(); err != nil {
		return interpreter.NewError(403, err.Error())
	}
	err := shutdownSystem()
	if err != nil {
		return interpreter.NewError(500, "shutdown failed: "+err.Error())
	}
	return &interpreter.BoolValue{Val: true}
}

// powerSensors returns available sensors
func powerSensors(args ...interpreter.Value) interpreter.Value {
	sensors, err := getSensors()
	if err != nil {
		return interpreter.NewError(501, "sensor data unavailable: "+err.Error())
	}
	result := interpreter.NewList()
	for _, sensor := range sensors {
		sensorMap := interpreter.NewMap()
		sensorMap.Set("name", &interpreter.StringValue{Val: sensor.Name})
		sensorMap.Set("type", &interpreter.StringValue{Val: sensor.Type})
		sensorMap.Set("value", &interpreter.FloatValue{Val: sensor.Value})
		sensorMap.Set("unit", &interpreter.StringValue{Val: sensor.Unit})
		result.Elements = append(result.Elements, sensorMap)
	}
	return result
}

// powerCpuTemp returns CPU temperatures
func powerCpuTemp(args ...interpreter.Value) interpreter.Value {
	temps, err := getCpuTemps()
	if err != nil {
		return interpreter.NewError(501, "CPU temperature unavailable: "+err.Error())
	}
	result := interpreter.NewList()
	for _, temp := range temps {
		result.Elements = append(result.Elements, &interpreter.FloatValue{Val: temp})
	}
	return result
}

// Sensor represents a system sensor
type Sensor struct {
	Name  string
	Type  string
	Value float64
	Unit  string
}

func requireSystemControl() error {
	return sandbox.RequireAllCapabilities("power control")
}

func parsePercentString(s string) float64 {
	value, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return value
}
