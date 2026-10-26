// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package stdlib

import "github.com/alca-lang/alca/pkg/interpreter"

// guiStubMethods lists the std.gui API surface that is implemented natively
// on Windows only. Every method still exists so programs type-check and link
// everywhere, but it returns a clear runtime error instead of crashing.
var guiStubMethods = []string{
	"alert", "msgbox", "confirm", "prompt", "open_file", "save_file", "notify",
	"screen_size", "width", "height", "screenshot", "webview", "task_manager", "native_task_manager",
}

// RegisterGUIBuiltins registers std.gui. The widget window API works on every
// platform through the Tcl/Tk and Qt backends; native dialogs are Windows-only.
func RegisterGUIBuiltins(env *interpreter.Environment) {
	guiModule := interpreter.NewMap()

	register := func(name string, fn func(args ...interpreter.Value) interpreter.Value) *interpreter.BuiltinFunction {
		builtin := &interpreter.BuiltinFunction{Name: name, Fn: fn}
		guiModule.Set(name, builtin)
		env.Set("gui_"+name, builtin)
		return builtin
	}

	for _, name := range guiStubMethods {
		n := name
		register(n, func(args ...interpreter.Value) interpreter.Value {
			return interpreter.NewError(501, "std.gui."+n+" is only supported on Windows")
		})
	}

	makeWindow := func(forced string, args ...interpreter.Value) interpreter.Value {
		title, width, height, backend := parseWindowArgs(args)
		if forced != "" {
			backend = forced
		}
		return windowValue(title, width, height, backend)
	}
	register("window", func(args ...interpreter.Value) interpreter.Value {
		return makeWindow("", args...)
	})
	register("tk_window", func(args ...interpreter.Value) interpreter.Value {
		return makeWindow("tk", args...)
	})
	register("qt_window", func(args ...interpreter.Value) interpreter.Value {
		return makeWindow("qt", args...)
	})

	register("set_backend", func(args ...interpreter.Value) interpreter.Value {
		if len(args) > 0 {
			SetDefaultGUIBackend(interpreter.CoerceToString(args[0]))
		}
		return interpreter.TRUE
	})

	register("backends", func(args ...interpreter.Value) interpreter.Value {
		result := interpreter.NewMap()
		for name, available := range BackendInfo() {
			switch v := available.(type) {
			case bool:
				if v {
					result.Set(name, interpreter.TRUE)
				} else {
					result.Set(name, interpreter.FALSE)
				}
			case string:
				result.Set(name, &interpreter.StringValue{Val: v})
			}
		}
		return result
	})

	env.Set("gui", guiModule)
	env.Set("std.gui", guiModule)
}
