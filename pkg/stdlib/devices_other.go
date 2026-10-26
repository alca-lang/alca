// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

//go:build !windows && !darwin && !linux && !freebsd && !openbsd && !netbsd

package stdlib

import "errors"

func listDevices(string) ([]Device, error) {
	return nil, errors.New("device enumeration is not supported on this platform")
}

func getDevice(string) (*Device, error) {
	return nil, errors.New("device lookup is not supported on this platform")
}
