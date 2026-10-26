// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

//go:build windows

// Package stdlib provides the win32 module for Windows-specific APIs.
package stdlib

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/sandbox"
	"golang.org/x/sys/windows"
)

var (
	win32Advapi32             = windows.NewLazySystemDLL("advapi32.dll")
	win32ProcRegOpenKeyExW    = win32Advapi32.NewProc("RegOpenKeyExW")
	win32ProcRegCloseKey      = win32Advapi32.NewProc("RegCloseKey")
	win32ProcRegQueryValueExW = win32Advapi32.NewProc("RegQueryValueExW")
	win32ProcRegSetValueExW   = win32Advapi32.NewProc("RegSetValueExW")
	win32ProcRegEnumKeyExW    = win32Advapi32.NewProc("RegEnumKeyExW")
)

const (
	HKEY_LOCAL_MACHINE = 0x80000002
	HKEY_CURRENT_USER  = 0x80000001
	REG_SZ             = 1
	REG_DWORD          = 4
)

// RegisterWin32Builtins registers the "std.win32" and "win32" modules.
func RegisterWin32Builtins(env *interpreter.Environment) {
	win32Module := interpreter.NewMap()

	register := func(name string, fn func(args ...interpreter.Value) interpreter.Value) {
		switch name {
		case "reg_open_key", "reg_read_value", "reg_write_value", "reg_enum_keys", "reg_close_key", "service_start", "service_stop", "wmi_query", "wmi_get_class":
			original := fn
			fn = func(args ...interpreter.Value) interpreter.Value {
				if err := sandbox.RequireAllCapabilities("win32." + name); err != nil {
					return interpreter.NewError(403, err.Error())
				}
				return original(args...)
			}
		}
		win32Module.Set(name, &interpreter.BuiltinFunction{Name: name, Fn: fn})
	}

	// Registry functions
	register("reg_open_key", win32RegOpenKey)
	register("reg_read_value", win32RegReadValue)
	register("reg_write_value", win32RegWriteValue)
	register("reg_enum_keys", win32RegEnumKeys)
	register("reg_close_key", win32RegCloseKey)

	// Services functions
	register("service_list", win32ServiceList)
	register("service_start", win32ServiceStart)
	register("service_stop", win32ServiceStop)
	register("service_info", win32ServiceInfo)

	// WMI functions
	register("wmi_query", win32WmiQuery)
	register("wmi_get_class", win32WmiGetClass)

	// Windows info
	register("version_info", win32VersionInfo)
	register("edition", win32Edition)
	register("license_status", win32LicenseStatus)

	env.Set("win32", win32Module)
	env.Set("std.win32", win32Module)
}

func win32RegOpenKey(args ...interpreter.Value) interpreter.Value {
	if len(args) < 2 || len(args) > 3 {
		return interpreter.NewError(400, "reg_open_key requires (hive, path, access?)")
	}
	hiveStr := interpreter.CoerceToString(args[0])
	path := interpreter.CoerceToString(args[1])
	access := uintptr(0x20019)
	if len(args) == 3 {
		switch strings.ToLower(interpreter.CoerceToString(args[2])) {
		case "read":
		case "write":
			access = 0x2001f
		default:
			return interpreter.NewError(400, "registry access must be read or write")
		}
	}

	var hive uintptr
	switch strings.ToUpper(hiveStr) {
	case "HKEY_LOCAL_MACHINE":
		hive = HKEY_LOCAL_MACHINE
	case "HKEY_CURRENT_USER":
		hive = HKEY_CURRENT_USER
	default:
		return interpreter.NewError(400, "invalid hive")
	}

	var hKey uintptr
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return interpreter.NewError(400, "invalid registry path: "+err.Error())
	}
	ret, _, _ := win32ProcRegOpenKeyExW.Call(hive, uintptr(unsafe.Pointer(pathPtr)), 0, access, uintptr(unsafe.Pointer(&hKey)))
	if ret != 0 {
		return interpreter.NewError(500, fmt.Sprintf("failed to open registry key (error %d)", ret))
	}
	return &interpreter.IntegerValue{Val: int64(hKey)}
}

func win32RegReadValue(args ...interpreter.Value) interpreter.Value {
	hKey, name, ok := win32RegistryArgs(args, 2)
	if !ok {
		return interpreter.NewError(400, "reg_read_value requires (handle, name)")
	}
	namePtr, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return interpreter.NewError(400, err.Error())
	}
	var valueType, dataSize uint32
	ret, _, _ := win32ProcRegQueryValueExW.Call(uintptr(hKey), uintptr(unsafe.Pointer(namePtr)), 0, uintptr(unsafe.Pointer(&valueType)), 0, uintptr(unsafe.Pointer(&dataSize)))
	if ret != 0 {
		return interpreter.NewError(500, fmt.Sprintf("failed to read registry value (error %d)", ret))
	}
	if dataSize > 16<<20 {
		return interpreter.NewError(413, "registry value exceeds 16 MiB")
	}
	data := make([]byte, dataSize)
	if dataSize > 0 {
		ret, _, _ = win32ProcRegQueryValueExW.Call(uintptr(hKey), uintptr(unsafe.Pointer(namePtr)), 0, uintptr(unsafe.Pointer(&valueType)), uintptr(unsafe.Pointer(&data[0])), uintptr(unsafe.Pointer(&dataSize)))
		if ret != 0 {
			return interpreter.NewError(500, fmt.Sprintf("failed to read registry value data (error %d)", ret))
		}
	}
	switch valueType {
	case REG_SZ, 2:
		if len(data)%2 != 0 {
			return interpreter.NewError(500, "registry string has invalid UTF-16 data")
		}
		units := make([]uint16, len(data)/2)
		for i := range units {
			units[i] = binary.LittleEndian.Uint16(data[i*2:])
		}
		return &interpreter.StringValue{Val: syscall.UTF16ToString(units)}
	case REG_DWORD:
		if len(data) < 4 {
			return interpreter.NewError(500, "registry DWORD has invalid data")
		}
		return &interpreter.IntegerValue{Val: int64(binary.LittleEndian.Uint32(data))}
	case 11:
		if len(data) < 8 {
			return interpreter.NewError(500, "registry QWORD has invalid data")
		}
		return &interpreter.IntegerValue{Val: int64(binary.LittleEndian.Uint64(data))}
	default:
		return interpreter.NewError(501, fmt.Sprintf("registry value type %d is not supported", valueType))
	}
}

func win32RegWriteValue(args ...interpreter.Value) interpreter.Value {
	if len(args) < 3 || len(args) > 4 {
		return interpreter.NewError(400, "reg_write_value requires (handle, name, value, type?)")
	}
	if err := sandbox.RequireAllCapabilities("win32 registry write"); err != nil {
		return interpreter.NewError(403, err.Error())
	}
	handle, ok := args[0].(*interpreter.IntegerValue)
	if !ok || handle.Val <= 0 {
		return interpreter.NewError(400, "registry handle must be a positive integer")
	}
	namePtr, err := syscall.UTF16PtrFromString(interpreter.CoerceToString(args[1]))
	if err != nil {
		return interpreter.NewError(400, err.Error())
	}
	var valueType uint32
	var data []byte
	switch value := args[2].(type) {
	case *interpreter.StringValue:
		valueType = REG_SZ
		encoded, err := syscall.UTF16FromString(value.Val)
		if err != nil {
			return interpreter.NewError(400, err.Error())
		}
		data = make([]byte, len(encoded)*2)
		for i, unit := range encoded {
			binary.LittleEndian.PutUint16(data[i*2:], unit)
		}
	case *interpreter.IntegerValue:
		if value.Val < 0 || value.Val > int64(^uint32(0)) {
			return interpreter.NewError(400, "REG_DWORD value must fit in an unsigned 32-bit integer")
		}
		valueType = REG_DWORD
		data = make([]byte, 4)
		binary.LittleEndian.PutUint32(data, uint32(value.Val))
	default:
		return interpreter.NewError(400, "registry value must be a string or integer")
	}
	if len(args) == 4 {
		switch strings.ToUpper(interpreter.CoerceToString(args[3])) {
		case "REG_SZ":
			if _, ok := args[2].(*interpreter.StringValue); !ok {
				return interpreter.NewError(400, "REG_SZ requires a string value")
			}
			valueType = REG_SZ
		case "REG_DWORD":
			if _, ok := args[2].(*interpreter.IntegerValue); !ok {
				return interpreter.NewError(400, "REG_DWORD requires an integer value")
			}
			valueType = REG_DWORD
		default:
			return interpreter.NewError(400, "registry value type must be REG_SZ or REG_DWORD")
		}
	}
	ret, _, _ := win32ProcRegSetValueExW.Call(uintptr(handle.Val), uintptr(unsafe.Pointer(namePtr)), 0, uintptr(valueType), uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)))
	if ret != 0 {
		return interpreter.NewError(500, fmt.Sprintf("failed to write registry value (error %d)", ret))
	}
	return interpreter.TRUE
}

func win32RegEnumKeys(args ...interpreter.Value) interpreter.Value {
	handle, _, ok := win32RegistryArgs(args, 1)
	if !ok {
		return interpreter.NewError(400, "reg_enum_keys requires a handle")
	}
	keys := interpreter.NewList()
	for index := uint32(0); ; index++ {
		buffer := make([]uint16, 256)
		nameLength := uint32(len(buffer))
		ret, _, _ := win32ProcRegEnumKeyExW.Call(uintptr(handle), uintptr(index), uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&nameLength)), 0, 0, 0, 0)
		if ret == 259 {
			break
		}
		if ret == 234 {
			buffer = make([]uint16, 32768)
			nameLength = uint32(len(buffer))
			ret, _, _ = win32ProcRegEnumKeyExW.Call(uintptr(handle), uintptr(index), uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&nameLength)), 0, 0, 0, 0)
		}
		if ret != 0 {
			return interpreter.NewError(500, fmt.Sprintf("failed to enumerate registry keys (error %d)", ret))
		}
		keys.Elements = append(keys.Elements, &interpreter.StringValue{Val: syscall.UTF16ToString(buffer[:nameLength])})
	}
	return keys
}

func win32RegCloseKey(args ...interpreter.Value) interpreter.Value {
	handle, _, ok := win32RegistryArgs(args, 1)
	if !ok {
		return interpreter.NewError(400, "reg_close_key requires handle")
	}
	ret, _, _ := win32ProcRegCloseKey.Call(uintptr(handle))
	if ret != 0 {
		return interpreter.NewError(500, fmt.Sprintf("failed to close registry key (error %d)", ret))
	}
	return &interpreter.BoolValue{Val: true}
}

func win32RegistryArgs(args []interpreter.Value, count int) (uintptr, string, bool) {
	if len(args) != count {
		return 0, "", false
	}
	handle, ok := args[0].(*interpreter.IntegerValue)
	if !ok || handle.Val <= 0 {
		return 0, "", false
	}
	if count == 1 {
		return uintptr(handle.Val), "", true
	}
	return uintptr(handle.Val), interpreter.CoerceToString(args[1]), true
}

func win32ServiceList(args ...interpreter.Value) interpreter.Value {
	cmd := exec.Command("sc", "query", "state=", "all")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return interpreter.NewError(500, fmt.Sprintf("failed to enumerate services: %s: %s", err, strings.TrimSpace(string(output))))
	}
	lines := strings.Split(string(output), "\n")
	services := interpreter.NewList()
	name := ""
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "SERVICE_NAME:") {
			name = strings.TrimSpace(strings.TrimPrefix(line, "SERVICE_NAME:"))
		}
		if name != "" && strings.HasPrefix(line, "STATE") {
			fields := strings.Fields(line)
			if len(fields) >= 4 {
				item := interpreter.NewMap()
				item.Set("name", &interpreter.StringValue{Val: name})
				item.Set("status", &interpreter.StringValue{Val: fields[len(fields)-1]})
				services.Elements = append(services.Elements, item)
				name = ""
			}
		}
	}
	return services
}

func win32ServiceStart(args ...interpreter.Value) interpreter.Value {
	if len(args) < 1 {
		return interpreter.NewError(400, "service_start requires service name")
	}
	if err := sandbox.RequireAllCapabilities("win32 service control"); err != nil {
		return interpreter.NewError(403, err.Error())
	}
	name := interpreter.CoerceToString(args[0])
	cmd := exec.Command("sc", "start", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return interpreter.NewError(500, fmt.Sprintf("failed to start service: %s: %s", err, strings.TrimSpace(string(output))))
	}
	return &interpreter.BoolValue{Val: true}
}

func win32ServiceStop(args ...interpreter.Value) interpreter.Value {
	if len(args) < 1 {
		return interpreter.NewError(400, "service_stop requires service name")
	}
	if err := sandbox.RequireAllCapabilities("win32 service control"); err != nil {
		return interpreter.NewError(403, err.Error())
	}
	name := interpreter.CoerceToString(args[0])
	cmd := exec.Command("sc", "stop", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return interpreter.NewError(500, fmt.Sprintf("failed to stop service: %s: %s", err, strings.TrimSpace(string(output))))
	}
	return &interpreter.BoolValue{Val: true}
}

func win32ServiceInfo(args ...interpreter.Value) interpreter.Value {
	if len(args) < 1 {
		return interpreter.NewError(400, "service_info requires service name")
	}
	name := interpreter.CoerceToString(args[0])
	cmd := exec.Command("sc", "qc", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return interpreter.NewError(500, fmt.Sprintf("failed to query service: %s: %s", err, strings.TrimSpace(string(output))))
	}
	result := interpreter.NewMap()
	result.Set("config", &interpreter.StringValue{Val: string(output)})
	return result
}

func win32WmiQuery(args ...interpreter.Value) interpreter.Value {
	if len(args) < 1 {
		return interpreter.NewError(400, "wmi_query requires WQL query")
	}
	query := interpreter.CoerceToString(args[0])
	return win32RunWmiQuery(query)
}

func win32WmiGetClass(args ...interpreter.Value) interpreter.Value {
	if len(args) < 1 {
		return interpreter.NewError(400, "wmi_get_class requires class name")
	}
	className := interpreter.CoerceToString(args[0])
	if className == "" {
		return interpreter.NewError(400, "WMI class name cannot be empty")
	}
	for _, r := range className {
		if !(r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return interpreter.NewError(400, "invalid WMI class name")
		}
	}
	return win32RunWmiQuery("SELECT * FROM " + className)
}

func win32RunWmiQuery(query string) interpreter.Value {
	encodedQuery := base64.StdEncoding.EncodeToString([]byte(query))
	script := "$q=[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('" + encodedQuery + "')); Get-CimInstance -Query $q | ConvertTo-Json -Compress -Depth 6"
	output, err := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script).CombinedOutput()
	if err != nil {
		return interpreter.NewError(500, fmt.Sprintf("WMI query failed: %s: %s", err, strings.TrimSpace(string(output))))
	}
	trimmed := strings.TrimSpace(string(output))
	results := interpreter.NewList()
	if trimmed == "" {
		return results
	}
	var decoded interface{}
	if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
		return interpreter.NewError(500, "failed to parse WMI response: "+err.Error())
	}
	switch rows := decoded.(type) {
	case []interface{}:
		for _, row := range rows {
			results.Elements = append(results.Elements, win32JSONValue(row))
		}
	case map[string]interface{}:
		results.Elements = append(results.Elements, win32JSONValue(rows))
	default:
		return interpreter.NewError(500, "WMI returned an unexpected response")
	}
	return results
}

func win32JSONValue(value interface{}) interpreter.Value {
	switch item := value.(type) {
	case nil:
		return interpreter.NULL
	case bool:
		return &interpreter.BoolValue{Val: item}
	case float64:
		return &interpreter.FloatValue{Val: item}
	case string:
		return &interpreter.StringValue{Val: item}
	case []interface{}:
		values := make([]interpreter.Value, 0, len(item))
		for _, child := range item {
			values = append(values, win32JSONValue(child))
		}
		return interpreter.NewList(values...)
	case map[string]interface{}:
		result := interpreter.NewMap()
		for key, child := range item {
			result.Set(key, win32JSONValue(child))
		}
		return result
	default:
		return &interpreter.StringValue{Val: fmt.Sprint(item)}
	}
}

func win32VersionInfo(args ...interpreter.Value) interpreter.Value {
	v := windows.RtlGetVersion()
	result := interpreter.NewMap()
	result.Set("major", &interpreter.IntegerValue{Val: int64(v.MajorVersion)})
	result.Set("minor", &interpreter.IntegerValue{Val: int64(v.MinorVersion)})
	result.Set("build", &interpreter.IntegerValue{Val: int64(v.BuildNumber)})
	result.Set("product_type", &interpreter.IntegerValue{Val: int64(v.ProductType)})
	return result
}

func win32Edition(args ...interpreter.Value) interpreter.Value {
	edition, err := getRegistryString(hkeyLocalMachine, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, "ProductName")
	if err != nil {
		return interpreter.NewError(500, "failed to read Windows edition: "+err.Error())
	}
	return &interpreter.StringValue{Val: edition}
}

func win32LicenseStatus(args ...interpreter.Value) interpreter.Value {
	windowsDir := os.Getenv("WINDIR")
	if windowsDir == "" {
		return interpreter.NewError(500, "WINDIR is not set")
	}
	script := filepath.Join(windowsDir, "System32", "slmgr.vbs")
	output, err := exec.Command("cscript.exe", "//Nologo", script, "/xpr").CombinedOutput()
	if err != nil {
		return interpreter.NewError(500, fmt.Sprintf("failed to query license status: %s: %s", err, strings.TrimSpace(string(output))))
	}
	text := strings.TrimSpace(string(output))
	result := interpreter.NewMap()
	result.Set("is_activated", &interpreter.BoolValue{Val: strings.Contains(strings.ToLower(text), "permanently activated")})
	result.Set("product_key", &interpreter.StringValue{Val: ""})
	result.Set("details", &interpreter.StringValue{Val: text})
	return result
}
