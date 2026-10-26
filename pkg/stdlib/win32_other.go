// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package stdlib

import "github.com/alca-lang/alca/pkg/interpreter"

// RegisterWin32Builtins registers an unsupported-platform win32 module.
func RegisterWin32Builtins(env *interpreter.Environment) {
	module := interpreter.NewMap()
	for _, name := range []string{
		"reg_open_key", "reg_read_value", "reg_write_value", "reg_enum_keys", "reg_close_key",
		"service_list", "service_start", "service_stop", "service_info",
		"wmi_query", "wmi_get_class", "version_info", "edition", "license_status",
	} {
		module.Set(name, &interpreter.BuiltinFunction{
			Name: name,
			Fn: func(args ...interpreter.Value) interpreter.Value {
				return interpreter.NewError(501, "win32 APIs are only available on Windows")
			},
		})
	}
	env.Set("win32", module)
	env.Set("std.win32", module)
}
