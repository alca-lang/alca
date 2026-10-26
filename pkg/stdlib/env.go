// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

// Package stdlib provides the standard library modules and builtins for the Alca programming language.
package stdlib

import (
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/sandbox"
)

// GetEnv retrieves the value of the environment variable named by key.
func GetEnv(name string) string {
	if err := sandbox.RequireEnv(name); err != nil {
		return ""
	}
	return os.Getenv(name)
}

// SetEnv sets the value of the environment variable named by key.
func SetEnv(name, value string) error {
	if err := sandbox.RequireEnv(name); err != nil {
		return err
	}
	return os.Setenv(name, value)
}

// UnsetEnv unsets a single environment variable.
func UnsetEnv(name string) error {
	if err := sandbox.RequireEnv(name); err != nil {
		return err
	}
	return os.Unsetenv(name)
}

// AllEnv returns a map of all current environment variables.
func AllEnv() map[string]string {
	if err := sandbox.RequireEnv("*"); err != nil {
		return make(map[string]string)
	}
	envVars := make(map[string]string)
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			envVars[parts[0]] = parts[1]
		}
	}
	return envVars
}

// ExpandEnv replaces ${var} or $var in the string according to current environment variables.
func ExpandEnv(s string) string {
	return os.ExpandEnv(s)
}

// HomeDir returns the current user's home directory path.
func HomeDir() string {
	dir, err := os.UserHomeDir()
	if err != nil {
		return os.Getenv("HOME")
	}
	return dir
}

// WorkingDir returns the current working directory path.
func WorkingDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}

// Hostname returns the system host name reported by the kernel.
func Hostname() string {
	name, err := os.Hostname()
	if err != nil {
		return "localhost"
	}
	return name
}

// Username returns the current system username.
func Username() string {
	u, err := user.Current()
	if err == nil && u.Username != "" {
		// Strip domain on Windows (e.g., DOMAIN\username -> username)
		parts := strings.Split(u.Username, "\\")
		return parts[len(parts)-1]
	}
	if name := os.Getenv("USER"); name != "" {
		return name
	}
	return os.Getenv("USERNAME")
}

// OSName returns the running operating system identifier ("windows", "linux", "darwin").
func OSName() string {
	return runtime.GOOS
}

// IsWindows reports whether the application is running on Windows.
func IsWindows() bool {
	return runtime.GOOS == "windows"
}

// IsLinux reports whether the application is running on Linux.
func IsLinux() bool {
	return runtime.GOOS == "linux"
}

// PathSeparator returns the operating-system-specific path list separator (":" or ";").
func PathSeparator() string {
	return string(os.PathListSeparator)
}

// GetPath returns the current process PATH directories as a slice of strings.
func GetPath() []string {
	pathVal := os.Getenv("PATH")
	if pathVal == "" {
		return []string{}
	}
	return filepath.SplitList(pathVal)
}

// SetPath sets the current process PATH environment variable from a slice of directory paths.
func SetPath(dirs []string) error {
	joined := strings.Join(dirs, string(os.PathListSeparator))
	return os.Setenv("PATH", joined)
}

// HasPath checks if the given directory path is present in the process PATH.
func HasPath(dir string) bool {
	cleanDir := filepath.Clean(dir)
	for _, p := range GetPath() {
		cleanP := filepath.Clean(p)
		if runtime.GOOS == "windows" {
			if strings.EqualFold(cleanP, cleanDir) {
				return true
			}
		} else {
			if cleanP == cleanDir {
				return true
			}
		}
	}
	return false
}

// AddPath appends a directory to the process PATH if it is not already included.
func AddPath(dir string) error {
	if HasPath(dir) {
		return nil
	}
	current := GetPath()
	current = append(current, dir)
	return SetPath(current)
}

// PrependPath inserts a directory at the beginning of the process PATH, giving it top priority.
func PrependPath(dir string) error {
	cleanDir := filepath.Clean(dir)
	newDirs := []string{dir}
	for _, p := range GetPath() {
		cleanP := filepath.Clean(p)
		match := false
		if runtime.GOOS == "windows" {
			match = strings.EqualFold(cleanP, cleanDir)
		} else {
			match = (cleanP == cleanDir)
		}
		if !match {
			newDirs = append(newDirs, p)
		}
	}
	return SetPath(newDirs)
}

// RemovePath removes a directory from the process PATH.
func RemovePath(dir string) error {
	cleanDir := filepath.Clean(dir)
	var newDirs []string
	for _, p := range GetPath() {
		cleanP := filepath.Clean(p)
		match := false
		if runtime.GOOS == "windows" {
			match = strings.EqualFold(cleanP, cleanDir)
		} else {
			match = (cleanP == cleanDir)
		}
		if !match {
			newDirs = append(newDirs, p)
		}
	}
	return SetPath(newDirs)
}

// Which resolves the absolute path of an executable in the current PATH.
func Which(name string) (string, error) {
	return exec.LookPath(name)
}

// AddUserPath permanently adds a directory to the persistent user PATH (in Windows Registry or ~/.profile),
// broadcasts the change to the desktop environment, and also updates the current process PATH.
func AddUserPath(dir string) error {
	dirs, err := GetUserPath()
	if err != nil {
		dirs = []string{}
	}
	cleanDir := filepath.Clean(dir)
	found := false
	for _, d := range dirs {
		match := false
		if runtime.GOOS == "windows" {
			match = strings.EqualFold(filepath.Clean(d), cleanDir)
		} else {
			match = (filepath.Clean(d) == cleanDir)
		}
		if match {
			found = true
			break
		}
	}
	if !found {
		dirs = append(dirs, dir)
		if err := SetUserPath(dirs); err != nil {
			return err
		}
	}
	_ = AddPath(dir)
	return nil
}

// RemoveUserPath permanently removes a directory from the persistent user PATH.
func RemoveUserPath(dir string) error {
	dirs, err := GetUserPath()
	if err != nil {
		return err
	}
	cleanDir := filepath.Clean(dir)
	var newDirs []string
	for _, d := range dirs {
		match := false
		if runtime.GOOS == "windows" {
			match = strings.EqualFold(filepath.Clean(d), cleanDir)
		} else {
			match = (filepath.Clean(d) == cleanDir)
		}
		if !match {
			newDirs = append(newDirs, d)
		}
	}
	if err := SetUserPath(newDirs); err != nil {
		return err
	}
	_ = RemovePath(dir)
	return nil
}

// AddSystemPath permanently adds a directory to the machine/system PATH (requires administrator privileges).
func AddSystemPath(dir string) error {
	dirs, err := GetSystemPath()
	if err != nil {
		return err
	}
	cleanDir := filepath.Clean(dir)
	for _, d := range dirs {
		if strings.EqualFold(filepath.Clean(d), cleanDir) {
			return nil
		}
	}
	dirs = append(dirs, dir)
	return SetSystemPath(dirs)
}

// RegisterEnvBuiltins registers environment variables and system info into the interpreter.
func RegisterEnvBuiltins(env *interpreter.Environment) {
	envModule := interpreter.NewMap()

	register := func(name string, fn func(args ...interpreter.Value) interpreter.Value) {
		builtin := &interpreter.BuiltinFunction{Name: name, Fn: fn}
		envModule.Set(name, builtin)
	}

	register("get_env", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return &interpreter.StringValue{Val: ""}
		}
		name := interpreter.CoerceToString(args[0])
		return &interpreter.StringValue{Val: GetEnv(name)}
	})

	register("set_env", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "set_env requires name and value arguments")
		}
		name := interpreter.CoerceToString(args[0])
		val := interpreter.CoerceToString(args[1])
		if err := SetEnv(name, val); err != nil {
			return interpreter.NewError(500, err.Error())
		}
		return interpreter.TRUE
	})

	register("unset_env", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "unset_env requires name argument")
		}
		name := interpreter.CoerceToString(args[0])
		if err := UnsetEnv(name); err != nil {
			return interpreter.NewError(500, err.Error())
		}
		return interpreter.TRUE
	})

	register("all_env", func(args ...interpreter.Value) interpreter.Value {
		all := AllEnv()
		m := interpreter.NewMap()
		for k, v := range all {
			m.Set(k, &interpreter.StringValue{Val: v})
		}
		return m
	})

	register("expand_env", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return &interpreter.StringValue{Val: ""}
		}
		str := interpreter.CoerceToString(args[0])
		return &interpreter.StringValue{Val: ExpandEnv(str)}
	})

	register("home_dir", func(args ...interpreter.Value) interpreter.Value {
		return &interpreter.StringValue{Val: HomeDir()}
	})

	register("working_dir", func(args ...interpreter.Value) interpreter.Value {
		return &interpreter.StringValue{Val: WorkingDir()}
	})

	register("hostname", func(args ...interpreter.Value) interpreter.Value {
		return &interpreter.StringValue{Val: Hostname()}
	})

	register("username", func(args ...interpreter.Value) interpreter.Value {
		return &interpreter.StringValue{Val: Username()}
	})

	register("os_name", func(args ...interpreter.Value) interpreter.Value {
		return &interpreter.StringValue{Val: OSName()}
	})

	register("is_windows", func(args ...interpreter.Value) interpreter.Value {
		if IsWindows() {
			return interpreter.TRUE
		}
		return interpreter.FALSE
	})

	register("is_linux", func(args ...interpreter.Value) interpreter.Value {
		if IsLinux() {
			return interpreter.TRUE
		}
		return interpreter.FALSE
	})

	register("path_separator", func(args ...interpreter.Value) interpreter.Value {
		return &interpreter.StringValue{Val: PathSeparator()}
	})

	// Process PATH functions

	// get_path() -> list of string directory paths
	register("get_path", func(args ...interpreter.Value) interpreter.Value {
		paths := GetPath()
		vals := make([]interpreter.Value, len(paths))
		for i, p := range paths {
			vals[i] = &interpreter.StringValue{Val: p}
		}
		return interpreter.NewList(vals...)
	})

	// set_path(dirs) -> bool
	register("set_path", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "set_path requires a list or string argument")
		}
		var dirs []string
		if lv, ok := args[0].(*interpreter.ListValue); ok {
			for _, elem := range lv.ElementsSlice() {
				dirs = append(dirs, interpreter.CoerceToString(elem))
			}
		} else {
			str := interpreter.CoerceToString(args[0])
			dirs = filepath.SplitList(str)
		}
		if err := SetPath(dirs); err != nil {
			return interpreter.NewError(500, err.Error())
		}
		return interpreter.TRUE
	})

	// has_path(dir) -> bool
	register("has_path", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.FALSE
		}
		dir := interpreter.CoerceToString(args[0])
		if HasPath(dir) {
			return interpreter.TRUE
		}
		return interpreter.FALSE
	})

	// add_path(dir) -> bool
	register("add_path", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "add_path requires a directory path argument")
		}
		dir := interpreter.CoerceToString(args[0])
		if err := AddPath(dir); err != nil {
			return interpreter.NewError(500, err.Error())
		}
		return interpreter.TRUE
	})

	// prepend_path(dir) -> bool
	register("prepend_path", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "prepend_path requires a directory path argument")
		}
		dir := interpreter.CoerceToString(args[0])
		if err := PrependPath(dir); err != nil {
			return interpreter.NewError(500, err.Error())
		}
		return interpreter.TRUE
	})

	// remove_path(dir) -> bool
	register("remove_path", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "remove_path requires a directory path argument")
		}
		dir := interpreter.CoerceToString(args[0])
		if err := RemovePath(dir); err != nil {
			return interpreter.NewError(500, err.Error())
		}
		return interpreter.TRUE
	})

	// which(binary_name) -> string or null
	register("which", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NULL
		}
		bin := interpreter.CoerceToString(args[0])
		path, err := Which(bin)
		if err != nil {
			return interpreter.NULL
		}
		return &interpreter.StringValue{Val: path}
	})

	// Persistent PATH functions

	// get_user_path() -> list of strings
	register("get_user_path", func(args ...interpreter.Value) interpreter.Value {
		paths, err := GetUserPath()
		if err != nil {
			return interpreter.NewError(500, err.Error())
		}
		vals := make([]interpreter.Value, len(paths))
		for i, p := range paths {
			vals[i] = &interpreter.StringValue{Val: p}
		}
		return interpreter.NewList(vals...)
	})

	// get_system_path() -> list of strings
	register("get_system_path", func(args ...interpreter.Value) interpreter.Value {
		paths, err := GetSystemPath()
		if err != nil {
			return interpreter.NewError(500, err.Error())
		}
		vals := make([]interpreter.Value, len(paths))
		for i, p := range paths {
			vals[i] = &interpreter.StringValue{Val: p}
		}
		return interpreter.NewList(vals...)
	})

	// add_user_path(dir) / persist_user_path(dir) -> bool
	persistUserPathFn := func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "add_user_path requires a directory path argument")
		}
		dir := interpreter.CoerceToString(args[0])
		if err := AddUserPath(dir); err != nil {
			return interpreter.NewError(500, err.Error())
		}
		return interpreter.TRUE
	}
	register("add_user_path", persistUserPathFn)
	register("persist_user_path", persistUserPathFn)

	// remove_user_path(dir) -> bool
	register("remove_user_path", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "remove_user_path requires a directory path argument")
		}
		dir := interpreter.CoerceToString(args[0])
		if err := RemoveUserPath(dir); err != nil {
			return interpreter.NewError(500, err.Error())
		}
		return interpreter.TRUE
	})

	// add_system_path(dir) -> bool
	register("add_system_path", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "add_system_path requires a directory path argument")
		}
		dir := interpreter.CoerceToString(args[0])
		if err := AddSystemPath(dir); err != nil {
			return interpreter.NewError(500, err.Error())
		}
		return interpreter.TRUE
	})

	// broadcast_env_change() -> bool
	register("broadcast_env_change", func(args ...interpreter.Value) interpreter.Value {
		BroadcastEnvChange()
		return interpreter.TRUE
	})

	env.Set("env", envModule)
	env.Set("std.env", envModule)
}
