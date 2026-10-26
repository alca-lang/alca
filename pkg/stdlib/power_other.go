// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

//go:build !windows && !darwin && !linux && !freebsd && !openbsd && !netbsd

package stdlib

import "errors"

func getBatteryStatus() (float64, bool, int64, error) {
	return 0, false, 0, errors.New("battery monitoring is not supported on this platform")
}
func getBatteryInfo() (float64, int64, string, string, error) {
	return 0, 0, "", "", errors.New("battery monitoring is not supported on this platform")
}
func isPowerPlugged() (bool, error) {
	return false, errors.New("power source monitoring is not supported on this platform")
}
func getPowerSource() (string, error) {
	return "", errors.New("power source monitoring is not supported on this platform")
}
func suspendSystem() error   { return errors.New("power management is not supported on this platform") }
func hibernateSystem() error { return errors.New("power management is not supported on this platform") }
func rebootSystem() error    { return errors.New("power management is not supported on this platform") }
func shutdownSystem() error  { return errors.New("power management is not supported on this platform") }
func getSensors() ([]Sensor, error) {
	return nil, errors.New("sensor monitoring is not supported on this platform")
}
func getCpuTemps() ([]float64, error) {
	return nil, errors.New("CPU temperature monitoring is not supported on this platform")
}
