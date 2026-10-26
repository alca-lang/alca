// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

// Package stdlib provides the Devices module for device enumeration and management.
package stdlib

import "github.com/alca-lang/alca/pkg/interpreter"

// RegisterDevicesBuiltins registers the "std.devices" and "devices" modules.
func RegisterDevicesBuiltins(env *interpreter.Environment) {
	devicesModule := interpreter.NewMap()

	register := func(name string, fn func(args ...interpreter.Value) interpreter.Value) {
		devicesModule.Set(name, &interpreter.BuiltinFunction{Name: name, Fn: fn})
	}

	// list(type?) -> list[map {id, name, class, driver}]
	register("list", devicesList)

	// get(id) -> map {vendor, product, serial, ...}
	register("get", devicesGet)

	// watch_events(callback) -> watcher
	register("watch_events", devicesWatchEvents)

	env.Set("devices", devicesModule)
	env.Set("std.devices", devicesModule)
}

func devicesList(args ...interpreter.Value) interpreter.Value {
	deviceType := ""
	if len(args) > 0 {
		deviceType = interpreter.CoerceToString(args[0])
	}
	devices, err := listDevices(deviceType)
	if err != nil {
		return interpreter.NewError(500, "failed to enumerate devices: "+err.Error())
	}
	result := interpreter.NewList()
	for _, dev := range devices {
		devMap := interpreter.NewMap()
		devMap.Set("id", &interpreter.StringValue{Val: dev.ID})
		devMap.Set("name", &interpreter.StringValue{Val: dev.Name})
		devMap.Set("class", &interpreter.StringValue{Val: dev.Class})
		devMap.Set("driver", &interpreter.StringValue{Val: dev.Driver})
		result.Elements = append(result.Elements, devMap)
	}
	return result
}

func devicesGet(args ...interpreter.Value) interpreter.Value {
	if len(args) < 1 {
		return interpreter.NewError(400, "devices.get requires device id")
	}
	id := interpreter.CoerceToString(args[0])
	dev, err := getDevice(id)
	if err != nil {
		return interpreter.NewError(500, "failed to query device: "+err.Error())
	}
	if dev == nil {
		return interpreter.NewError(404, "device not found")
	}
	result := interpreter.NewMap()
	result.Set("id", &interpreter.StringValue{Val: dev.ID})
	result.Set("name", &interpreter.StringValue{Val: dev.Name})
	result.Set("vendor", &interpreter.StringValue{Val: dev.Vendor})
	result.Set("product", &interpreter.StringValue{Val: dev.Product})
	result.Set("serial", &interpreter.StringValue{Val: dev.Serial})
	return result
}

func devicesWatchEvents(args ...interpreter.Value) interpreter.Value {
	return interpreter.NewError(501, "device event watching is not supported")
}

type Device struct {
	ID      string
	Name    string
	Class   string
	Driver  string
	Vendor  string
	Product string
	Serial  string
}
