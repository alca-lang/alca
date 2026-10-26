// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package stdlib

import (
	"fmt"
	"math"
	"path/filepath"
	"syscall"
	"unsafe"
)

var (
	kernel32DLL              = syscall.NewLazyDLL("kernel32.dll")
	procGetDiskFreeSpaceExW  = kernel32DLL.NewProc("GetDiskFreeSpaceExW")
	procGlobalMemoryStatusEx = kernel32DLL.NewProc("GlobalMemoryStatusEx")
)

type memoryStatusEx struct {
	cbSize                  uint32
	dwMemoryLoad            uint32
	ullTotalPhys            uint64
	ullAvailPhys            uint64
	ullTotalPageFile        uint64
	ullAvailPageFile        uint64
	ullTotalVirtual         uint64
	ullAvailVirtual         uint64
	ullAvailExtendedVirtual uint64
}

// DiskUsageStats retrieves disk storage capacity and usage for the specified path on Windows.
func DiskUsageStats(path string) (totalBytes, freeBytes, usedBytes uint64, percentUsed float64, err error) {
	if path == "" {
		path = "."
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}
	volume := filepath.VolumeName(absPath)
	if volume != "" {
		volume += "\\"
	} else {
		volume = absPath
	}

	volPtr, err := syscall.UTF16PtrFromString(volume)
	if err != nil {
		return 0, 0, 0, 0, err
	}

	var freeBytesAvailable, totalNumberOfBytes, totalNumberOfFreeBytes uint64
	ret, _, callErr := procGetDiskFreeSpaceExW.Call(
		uintptr(unsafe.Pointer(volPtr)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalNumberOfBytes)),
		uintptr(unsafe.Pointer(&totalNumberOfFreeBytes)),
	)
	if ret == 0 {
		return 0, 0, 0, 0, callErr
	}

	totalBytes = totalNumberOfBytes
	freeBytes = freeBytesAvailable
	if totalBytes > freeBytes {
		usedBytes = totalBytes - freeBytes
	}
	if totalBytes > 0 {
		percentUsed = math.Round((float64(usedBytes)/float64(totalBytes))*1000) / 10
	}

	return totalBytes, freeBytes, usedBytes, percentUsed, nil
}

// RamUsageStats retrieves physical RAM status on Windows using GlobalMemoryStatusEx.
func RamUsageStats() (totalBytes, freeBytes, usedBytes uint64, percentUsed float64, err error) {
	var stat memoryStatusEx
	stat.cbSize = uint32(unsafe.Sizeof(stat))

	ret, _, callErr := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&stat)))
	if ret == 0 {
		return 0, 0, 0, 0, callErr
	}

	totalBytes = stat.ullTotalPhys
	freeBytes = stat.ullAvailPhys
	if totalBytes > freeBytes {
		usedBytes = totalBytes - freeBytes
	}
	percentUsed = float64(stat.dwMemoryLoad)

	return totalBytes, freeBytes, usedBytes, percentUsed, nil
}

var (
	procGetTickCount64           = kernel32DLL.NewProc("GetTickCount64")
	procCreateToolhelp32Snapshot = kernel32DLL.NewProc("CreateToolhelp32Snapshot")
	procProcess32FirstW          = kernel32DLL.NewProc("Process32FirstW")
	procProcess32NextW           = kernel32DLL.NewProc("Process32NextW")
	procCloseHandleSnap          = kernel32DLL.NewProc("CloseHandle")
	procGetLogicalDriveStringsW  = kernel32DLL.NewProc("GetLogicalDriveStringsW")
)

type processEntry32W struct {
	dwSize              uint32
	cntUsage            uint32
	th32ProcessID       uint32
	th32DefaultHeapID   uintptr
	th32ModuleID        uint32
	cntThreads          uint32
	th32ParentProcessID uint32
	pcPriClassBase      int32
	dwFlags             uint32
	szExeFile           [260]uint16
}

// SystemUptime returns system uptime in seconds on Windows.
func SystemUptime() float64 {
	uptime, _ := SystemUptimeStats()
	return uptime
}

func SystemUptimeStats() (float64, error) {
	ret, _, err := procGetTickCount64.Call()
	if ret == 0 && err != syscall.Errno(0) {
		return 0, err
	}
	return float64(ret) / 1000.0, nil
}

// SystemProcesses enumerates running processes on Windows.
func SystemProcesses() ([]map[string]interface{}, error) {
	if procCreateToolhelp32Snapshot == nil {
		return nil, nil
	}
	// TH32CS_SNAPPROCESS = 0x00000002
	hSnap, _, _ := procCreateToolhelp32Snapshot.Call(0x00000002, 0)
	if hSnap == 0 || hSnap == ^uintptr(0) {
		return nil, syscall.GetLastError()
	}
	defer procCloseHandleSnap.Call(hSnap)

	var entry processEntry32W
	entry.dwSize = uint32(unsafe.Sizeof(entry))

	var list []map[string]interface{}
	r1, _, _ := procProcess32FirstW.Call(hSnap, uintptr(unsafe.Pointer(&entry)))
	for r1 != 0 {
		name := syscall.UTF16ToString(entry.szExeFile[:])
		list = append(list, map[string]interface{}{
			"pid":  int64(entry.th32ProcessID),
			"ppid": int64(entry.th32ParentProcessID),
			"name": name,
		})
		r1, _, _ = procProcess32NextW.Call(hSnap, uintptr(unsafe.Pointer(&entry)))
	}
	return list, nil
}

// DiskPartitions lists all logical disk partitions and their usage on Windows.
func DiskPartitions() ([]map[string]interface{}, error) {
	buf := make([]uint16, 256)
	r1, _, err := procGetLogicalDriveStringsW.Call(uintptr(len(buf)), uintptr(unsafe.Pointer(&buf[0])))
	if r1 == 0 {
		return nil, err
	}

	var drives []string
	var current []uint16
	for _, c := range buf[:r1] {
		if c == 0 {
			if len(current) > 0 {
				drives = append(drives, syscall.UTF16ToString(current))
				current = nil
			}
		} else {
			current = append(current, c)
		}
	}

	var result []map[string]interface{}
	for _, drive := range drives {
		tot, free, used, pct, err := DiskUsageStats(drive)
		if err == nil {
			toGB := func(b uint64) float64 {
				return math.Round((float64(b)/(1024*1024*1024))*100) / 100
			}
			result = append(result, map[string]interface{}{
				"mount":        drive,
				"total_gb":     toGB(tot),
				"free_gb":      toGB(free),
				"used_gb":      toGB(used),
				"percent_used": pct,
			})
		}
	}
	return result, nil
}

func LoadAverageStats() (map[string]float64, error) {
	return nil, fmt.Errorf("load averages are not provided by Windows")
}

// LoadAverage is a compatibility wrapper. Use LoadAverageStats to detect
// unsupported platforms without treating them as valid zero load.
func LoadAverage() map[string]float64 {
	return nil
}
