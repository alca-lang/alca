// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

// Package stdlib provides the Platform module for system identification.
package stdlib

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/alca-lang/alca/pkg/interpreter"
)

// RegisterPlatformBuiltins registers the "std.platform" and "platform" modules.
func RegisterPlatformBuiltins(env *interpreter.Environment) {
	platformModule := interpreter.NewMap()

	register := func(name string, fn func(args ...interpreter.Value) interpreter.Value) {
		platformModule.Set(name, &interpreter.BuiltinFunction{Name: name, Fn: fn})
	}

	// system() -> string (windows, linux, darwin)
	register("system", func(args ...interpreter.Value) interpreter.Value {
		return &interpreter.StringValue{Val: runtime.GOOS}
	})

	// release() -> string (version number)
	register("release", func(args ...interpreter.Value) interpreter.Value {
		return &interpreter.StringValue{Val: platformRelease()}
	})

	// version() -> string (human-readable version)
	register("version", func(args ...interpreter.Value) interpreter.Value {
		return &interpreter.StringValue{Val: platformVersion()}
	})

	// machine() -> string (x86_64, arm64, etc.)
	register("machine", func(args ...interpreter.Value) interpreter.Value {
		return &interpreter.StringValue{Val: runtime.GOARCH}
	})

	// processor() -> string (CPU model name)
	register("processor", func(args ...interpreter.Value) interpreter.Value {
		return &interpreter.StringValue{Val: platformProcessor()}
	})

	// node() -> string (hostname)
	register("node", func(args ...interpreter.Value) interpreter.Value {
		return &interpreter.StringValue{Val: platformNode()}
	})

	// architecture() -> map with architecture details
	register("architecture", func(args ...interpreter.Value) interpreter.Value {
		result := interpreter.NewMap()
		result.Set("bits", &interpreter.IntegerValue{Val: platformArchBits()})
		result.Set("linkage", &interpreter.StringValue{Val: platformArchLinkage()})
		result.Set("executable", &interpreter.StringValue{Val: platformArchExecutable()})
		return result
	})

	// info() -> map with all platform info
	register("info", func(args ...interpreter.Value) interpreter.Value {
		result := interpreter.NewMap()
		result.Set("system", &interpreter.StringValue{Val: runtime.GOOS})
		result.Set("release", &interpreter.StringValue{Val: platformRelease()})
		result.Set("version", &interpreter.StringValue{Val: platformVersion()})
		result.Set("machine", &interpreter.StringValue{Val: runtime.GOARCH})
		result.Set("processor", &interpreter.StringValue{Val: platformProcessor()})
		result.Set("node", &interpreter.StringValue{Val: platformNode()})
		return result
	})

	register("uname", func(args ...interpreter.Value) interpreter.Value {
		return interpreter.NewList(
			&interpreter.StringValue{Val: runtime.GOOS},
			&interpreter.StringValue{Val: platformRelease()},
			&interpreter.StringValue{Val: platformVersion()},
			&interpreter.StringValue{Val: runtime.GOARCH},
			&interpreter.StringValue{Val: platformNode()},
		)
	})

	register("win32_ver", func(args ...interpreter.Value) interpreter.Value {
		if runtime.GOOS != "windows" {
			return interpreter.NewError(501, "win32_ver is only available on Windows")
		}
		version := strings.Split(platformRelease(), ".")
		values := make([]interpreter.Value, 3)
		for i := range values {
			if i < len(version) {
				number, _ := strconv.ParseInt(version[i], 10, 64)
				values[i] = &interpreter.IntegerValue{Val: number}
			} else {
				values[i] = &interpreter.IntegerValue{Val: 0}
			}
		}
		return interpreter.NewList(
			values[0], values[1], values[2],
			&interpreter.IntegerValue{Val: 0},
			&interpreter.StringValue{Val: platformVersion()},
		)
	})

	register("linux_distro", func(args ...interpreter.Value) interpreter.Value {
		if runtime.GOOS != "linux" {
			return interpreter.NewError(501, "linux_distro is only available on Linux")
		}
		data, err := os.ReadFile("/etc/os-release")
		if err != nil {
			return interpreter.NewError(500, "failed to read Linux distribution information: "+err.Error())
		}
		result := interpreter.NewMap()
		for _, line := range strings.Split(string(data), "\n") {
			key, value, found := strings.Cut(line, "=")
			if !found || key == "" {
				continue
			}
			value = strings.TrimSpace(value)
			if unquoted, err := strconv.Unquote(value); err == nil {
				value = unquoted
			}
			result.Set(key, &interpreter.StringValue{Val: value})
		}
		return result
	})

	register("mac_ver", func(args ...interpreter.Value) interpreter.Value {
		if runtime.GOOS != "darwin" {
			return interpreter.NewError(501, "mac_ver is only available on macOS")
		}
		product, err := exec.Command("sw_vers", "-productName").Output()
		if err != nil {
			return interpreter.NewError(500, "failed to read macOS product name: "+err.Error())
		}
		version, err := exec.Command("sw_vers", "-productVersion").Output()
		if err != nil {
			return interpreter.NewError(500, "failed to read macOS version: "+err.Error())
		}
		build, err := exec.Command("sw_vers", "-buildVersion").Output()
		if err != nil {
			return interpreter.NewError(500, "failed to read macOS build: "+err.Error())
		}
		result := interpreter.NewMap()
		result.Set("product", &interpreter.StringValue{Val: strings.TrimSpace(string(product))})
		result.Set("version", &interpreter.StringValue{Val: strings.TrimSpace(string(version))})
		result.Set("build", &interpreter.StringValue{Val: strings.TrimSpace(string(build))})
		return result
	})

	// capabilities() -> map of supported features
	register("capabilities", func(args ...interpreter.Value) interpreter.Value {
		result := interpreter.NewMap()
		result.Set("supports_ffi", &interpreter.BoolValue{Val: true})
		result.Set("supports_gui", &interpreter.BoolValue{Val: platformSupportsGUI()})
		result.Set("supports_wmi", &interpreter.BoolValue{Val: platformSupportsWMI()})
		result.Set("supports_acpi", &interpreter.BoolValue{Val: platformSupportsACPI()})
		result.Set("supports_sensors", &interpreter.BoolValue{Val: platformSupportsSensors()})
		result.Set("supports_drivers", &interpreter.BoolValue{Val: platformSupportsDrivers()})
		result.Set("requires_root", &interpreter.BoolValue{Val: platformRequiresRoot()})
		return result
	})

	// has_feature(feature) -> bool
	register("has_feature", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "has_feature requires a feature name")
		}
		feature := interpreter.CoerceToString(args[0])
		caps := platformCapabilities()
		switch feature {
		case "ffi":
			return &interpreter.BoolValue{Val: caps.SupportsFFI}
		case "gui":
			return &interpreter.BoolValue{Val: caps.SupportsGUI}
		case "wmi":
			return &interpreter.BoolValue{Val: caps.SupportsWMI}
		case "acpi":
			return &interpreter.BoolValue{Val: caps.SupportsACPI}
		case "sensors":
			return &interpreter.BoolValue{Val: caps.SupportsSensors}
		case "drivers":
			return &interpreter.BoolValue{Val: caps.SupportsDrivers}
		default:
			return &interpreter.BoolValue{Val: false}
		}
	})

	// api_version() -> string
	register("api_version", func(args ...interpreter.Value) interpreter.Value {
		return &interpreter.StringValue{Val: "1.0.0"}
	})

	env.Set("platform", platformModule)
	env.Set("std.platform", platformModule)
}

// PlatformCapabilities describes available system features
type PlatformCapabilities struct {
	SupportsFFI     bool
	SupportsGUI     bool
	SupportsWMI     bool
	SupportsACPI    bool
	SupportsSensors bool
	SupportsDrivers bool
	RequiresRoot    bool
}

// platformArchBits returns architecture bitness
func platformArchBits() int64 {
	return strconv.IntSize
}

// platformArchLinkage returns linkage type
func platformArchLinkage() string {
	switch runtime.GOOS {
	case "windows":
		return "PE"
	case "darwin":
		return "Mach-O"
	case "linux", "freebsd", "openbsd", "netbsd":
		return "ELF"
	default:
		return "unknown"
	}
}

// platformArchExecutable returns executable format
func platformArchExecutable() string {
	switch runtime.GOOS {
	case "windows":
		return "Windows-PE"
	case "darwin":
		return "Darwin-Mach-O"
	case "linux":
		return "Linux-ELF"
	case "freebsd", "openbsd", "netbsd":
		return strings.ToUpper(runtime.GOOS[:1]) + runtime.GOOS[1:] + "-ELF"
	default:
		return runtime.GOOS + "-unknown"
	}
}

// platformSupportsGUI returns if GUI is supported
func platformSupportsGUI() bool {
	switch runtime.GOOS {
	case "windows":
		session := strings.ToLower(os.Getenv("SESSIONNAME"))
		return session != "" && session != "services"
	case "darwin":
		if os.Getenv("SSH_TTY") != "" {
			return false
		}
		uid := strconv.Itoa(os.Getuid())
		return exec.Command("launchctl", "print", "gui/"+uid).Run() == nil
	case "linux":
		return os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != ""
	default:
		return false
	}
}

// platformSupportsWMI returns if WMI is available (Windows only)
func platformSupportsWMI() bool {
	if runtime.GOOS != "windows" {
		return false
	}
	return platformProbeWindowsPowerShell("$ErrorActionPreference='Stop'; Get-CimInstance Win32_OperatingSystem -ErrorAction Stop | Out-Null")
}

// platformSupportsACPI returns if ACPI is available
func platformSupportsACPI() bool {
	switch runtime.GOOS {
	case "linux":
		_, err := os.Stat("/sys/firmware/acpi")
		return err == nil
	case "windows":
		return platformProbeWindowsPowerShell("if (Test-Path 'HKLM:\\HARDWARE\\ACPI') { exit 0 } else { exit 1 }")
	default:
		return false
	}
}

// platformSupportsSensors returns if sensor APIs are available
func platformSupportsSensors() bool {
	switch runtime.GOOS {
	case "linux":
		for _, path := range []string{"/sys/class/hwmon", "/sys/class/thermal"} {
			if entries, err := os.ReadDir(path); err == nil && len(entries) > 0 {
				return true
			}
		}
		return false
	case "darwin":
		_, err := exec.LookPath("powermetrics")
		return err == nil
	case "windows":
		return platformProbeWindowsPowerShell("$ErrorActionPreference='Stop'; $s=Get-CimInstance -Namespace root/wmi -ClassName MSAcpi_ThermalZoneTemperature -ErrorAction SilentlyContinue; if ($s) { exit 0 } else { exit 1 }")
	default:
		return false
	}
}

// platformSupportsDrivers returns if driver enumeration is available
func platformSupportsDrivers() bool {
	switch runtime.GOOS {
	case "linux":
		_, err := os.Stat("/proc/modules")
		return err == nil
	case "windows":
		_, err := exec.LookPath("driverquery.exe")
		return err == nil
	case "darwin":
		_, err := exec.LookPath("kmutil")
		return err == nil
	default:
		return false
	}
}

// platformRequiresRoot reports whether this process lacks elevated privileges.
func platformRequiresRoot() bool {
	return !IsAdmin()
}

// platformCapabilities returns the full capabilities struct
func platformCapabilities() PlatformCapabilities {
	return PlatformCapabilities{
		SupportsFFI:     runtime.GOOS == "windows" || runtime.GOOS == "linux" || runtime.GOOS == "darwin" || runtime.GOOS == "freebsd",
		SupportsGUI:     platformSupportsGUI(),
		SupportsWMI:     platformSupportsWMI(),
		SupportsACPI:    platformSupportsACPI(),
		SupportsSensors: platformSupportsSensors(),
		SupportsDrivers: platformSupportsDrivers(),
		RequiresRoot:    platformRequiresRoot(),
	}
}

func platformProbeWindowsPowerShell(script string) bool {
	if _, err := exec.LookPath("powershell.exe"); err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script).Run() == nil
}
