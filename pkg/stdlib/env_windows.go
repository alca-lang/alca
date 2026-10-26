// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package stdlib

import (
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

var (
	advapi32             = syscall.NewLazyDLL("advapi32.dll")
	procRegOpenKeyExW    = advapi32.NewProc("RegOpenKeyExW")
	procRegQueryValueExW = advapi32.NewProc("RegQueryValueExW")
	procRegSetValueExW   = advapi32.NewProc("RegSetValueExW")
	procRegCloseKey      = advapi32.NewProc("RegCloseKey")

	user32DLL               = syscall.NewLazyDLL("user32.dll")
	procSendMessageTimeoutW = user32DLL.NewProc("SendMessageTimeoutW")
)

const (
	hkeyCurrentUser  = uintptr(0x80000001)
	hkeyLocalMachine = uintptr(0x80000002)

	keyRead     = 0x20019
	keyWrite    = 0x20006
	regExpandSz = 2
	regSz       = 1
)

func getRegistryString(root uintptr, subKey string, valueName string) (string, error) {
	subKeyPtr, err := syscall.UTF16PtrFromString(subKey)
	if err != nil {
		return "", err
	}
	var hKey uintptr
	ret, _, _ := procRegOpenKeyExW.Call(
		root,
		uintptr(unsafe.Pointer(subKeyPtr)),
		0,
		uintptr(keyRead),
		uintptr(unsafe.Pointer(&hKey)),
	)
	if ret != 0 {
		return "", fmt.Errorf("RegOpenKeyExW failed with code %d", ret)
	}
	defer procRegCloseKey.Call(hKey)

	valNamePtr, err := syscall.UTF16PtrFromString(valueName)
	if err != nil {
		return "", err
	}

	var valType uint32
	var valSize uint32
	ret, _, _ = procRegQueryValueExW.Call(
		hKey,
		uintptr(unsafe.Pointer(valNamePtr)),
		0,
		uintptr(unsafe.Pointer(&valType)),
		0,
		uintptr(unsafe.Pointer(&valSize)),
	)
	if ret != 0 {
		return "", fmt.Errorf("RegQueryValueExW failed with code %d", ret)
	}

	buf := make([]uint16, (valSize/2)+1)
	ret, _, _ = procRegQueryValueExW.Call(
		hKey,
		uintptr(unsafe.Pointer(valNamePtr)),
		0,
		uintptr(unsafe.Pointer(&valType)),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&valSize)),
	)
	if ret != 0 {
		return "", fmt.Errorf("RegQueryValueExW (data) failed with code %d", ret)
	}

	n := valSize / 2
	if n > 0 && buf[n-1] == 0 {
		n--
	}
	return string(utf16.Decode(buf[:n])), nil
}

func setRegistryString(root uintptr, subKey string, valueName string, value string, isExpand bool) error {
	subKeyPtr, err := syscall.UTF16PtrFromString(subKey)
	if err != nil {
		return err
	}
	var hKey uintptr
	ret, _, _ := procRegOpenKeyExW.Call(
		root,
		uintptr(unsafe.Pointer(subKeyPtr)),
		0,
		uintptr(keyWrite),
		uintptr(unsafe.Pointer(&hKey)),
	)
	if ret != 0 {
		return fmt.Errorf("RegOpenKeyExW failed with code %d", ret)
	}
	defer procRegCloseKey.Call(hKey)

	valNamePtr, err := syscall.UTF16PtrFromString(valueName)
	if err != nil {
		return err
	}

	valUTF16, err := syscall.UTF16FromString(value)
	if err != nil {
		return err
	}

	valType := uint32(regSz)
	if isExpand {
		valType = uint32(regExpandSz)
	}

	cbData := uint32(len(valUTF16) * 2)
	ret, _, _ = procRegSetValueExW.Call(
		hKey,
		uintptr(unsafe.Pointer(valNamePtr)),
		0,
		uintptr(valType),
		uintptr(unsafe.Pointer(&valUTF16[0])),
		uintptr(cbData),
	)
	if ret != 0 {
		return fmt.Errorf("RegSetValueExW failed with code %d", ret)
	}
	return nil
}

// GetUserPath returns the persistent User PATH configured in the Windows Registry (HKCU\Environment\Path).
func GetUserPath() ([]string, error) {
	val, err := getRegistryString(hkeyCurrentUser, "Environment", "Path")
	if err != nil {
		return nil, err
	}
	return filepath.SplitList(val), nil
}

// GetSystemPath returns the persistent Machine/System PATH configured in the Windows Registry.
func GetSystemPath() ([]string, error) {
	val, err := getRegistryString(hkeyLocalMachine, "System\\CurrentControlSet\\Control\\Session Manager\\Environment", "Path")
	if err != nil {
		return nil, err
	}
	return filepath.SplitList(val), nil
}

// SetUserPath sets the persistent User PATH in the Windows Registry and notifies running processes.
func SetUserPath(dirs []string) error {
	val := strings.Join(dirs, ";")
	if err := setRegistryString(hkeyCurrentUser, "Environment", "Path", val, true); err != nil {
		return err
	}
	BroadcastEnvChange()
	return nil
}

// SetSystemPath sets the persistent Machine PATH in the Windows Registry and notifies running processes (requires admin).
func SetSystemPath(dirs []string) error {
	val := strings.Join(dirs, ";")
	if err := setRegistryString(hkeyLocalMachine, "System\\CurrentControlSet\\Control\\Session Manager\\Environment", "Path", val, true); err != nil {
		return err
	}
	BroadcastEnvChange()
	return nil
}

// BroadcastEnvChange sends a WM_SETTINGCHANGE message to all top-level windows to notify that the environment has changed.
func BroadcastEnvChange() {
	if procSendMessageTimeoutW.Find() != nil {
		return
	}
	envPtr, _ := syscall.UTF16PtrFromString("Environment")
	var result uintptr
	_, _, _ = procSendMessageTimeoutW.Call(
		0xFFFF, // HWND_BROADCAST
		0x001A, // WM_SETTINGCHANGE
		0,
		uintptr(unsafe.Pointer(envPtr)),
		0x0002, // SMTO_ABORTIFHUNG
		5000,
		uintptr(unsafe.Pointer(&result)),
	)
}
