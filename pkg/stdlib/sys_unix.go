// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package stdlib

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// DiskUsageStats retrieves disk storage capacity and usage for the specified path on Unix.
func DiskUsageStats(path string) (totalBytes, freeBytes, usedBytes uint64, percentUsed float64, err error) {
	if path == "" {
		path = "/"
	}
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0, 0, 0, err
	}

	blockSize := int64(stat.Bsize)
	blocks := int64(stat.Blocks)
	availableBlocks := int64(stat.Bavail)
	if blockSize <= 0 || blocks < 0 || availableBlocks < 0 {
		return 0, 0, 0, 0, fmt.Errorf("invalid filesystem statistics for %q", path)
	}
	if uint64(blocks) > ^uint64(0)/uint64(blockSize) || uint64(availableBlocks) > ^uint64(0)/uint64(blockSize) {
		return 0, 0, 0, 0, fmt.Errorf("filesystem size counters overflow for %q", path)
	}
	totalBytes = uint64(blocks) * uint64(blockSize)
	freeBytes = uint64(availableBlocks) * uint64(blockSize)
	if totalBytes > freeBytes {
		usedBytes = totalBytes - freeBytes
	}
	if totalBytes > 0 {
		percentUsed = math.Round((float64(usedBytes)/float64(totalBytes))*1000) / 10
	}
	return totalBytes, freeBytes, usedBytes, percentUsed, nil
}

// RamUsageStats retrieves physical RAM status on Linux/Unix.
func RamUsageStats() (totalBytes, freeBytes, usedBytes uint64, percentUsed float64, err error) {
	if runtime.GOOS != "linux" {
		totalBytes, freeBytes, err = unixPhysicalMemory()
		if err != nil {
			return 0, 0, 0, 0, err
		}
		if totalBytes == 0 || freeBytes > totalBytes {
			return 0, 0, 0, 0, fmt.Errorf("invalid system memory counters")
		}
		usedBytes = totalBytes - freeBytes
		percentUsed = math.Round((float64(usedBytes)/float64(totalBytes))*1000) / 10
		return totalBytes, freeBytes, usedBytes, percentUsed, nil
	}
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, 0, 0, err
	}
	defer file.Close()

	var memTotal, memAvailable uint64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			if fields[0] == "MemTotal:" {
				kb, parseErr := strconv.ParseUint(fields[1], 10, 64)
				if parseErr != nil {
					return 0, 0, 0, 0, fmt.Errorf("invalid MemTotal in /proc/meminfo: %w", parseErr)
				}
				memTotal = kb * 1024
			} else if fields[0] == "MemAvailable:" {
				kb, parseErr := strconv.ParseUint(fields[1], 10, 64)
				if parseErr != nil {
					return 0, 0, 0, 0, fmt.Errorf("invalid MemAvailable in /proc/meminfo: %w", parseErr)
				}
				memAvailable = kb * 1024
			}
			if err := scanner.Err(); err != nil {
				return 0, 0, 0, 0, err
			}
			if memTotal == 0 || memAvailable == 0 || memAvailable > memTotal {
				return 0, 0, 0, 0, fmt.Errorf("invalid or incomplete /proc/meminfo")
			}
		}
	}

	totalBytes = memTotal
	freeBytes = memAvailable
	if totalBytes > freeBytes {
		usedBytes = totalBytes - freeBytes
	}
	if totalBytes > 0 {
		percentUsed = math.Round((float64(usedBytes)/float64(totalBytes))*1000) / 10
	}
	return totalBytes, freeBytes, usedBytes, percentUsed, nil
}

// SystemUptime returns system uptime in seconds on Unix.
func SystemUptime() float64 {
	uptime, _ := SystemUptimeStats()
	return uptime
}

func SystemUptimeStats() (float64, error) {
	if runtime.GOOS == "linux" {
		data, err := os.ReadFile("/proc/uptime")
		if err != nil {
			return 0, err
		}
		parts := strings.Fields(string(data))
		if len(parts) < 1 {
			return 0, fmt.Errorf("invalid /proc/uptime")
		}
		seconds, err := strconv.ParseFloat(parts[0], 64)
		if err != nil {
			return 0, fmt.Errorf("parse /proc/uptime: %w", err)
		}
		return seconds, nil
	}
	output, err := exec.Command("sysctl", "-n", "kern.boottime").Output()
	if err != nil {
		return 0, fmt.Errorf("read system boot time: %w", err)
	}
	text := string(output)
	start := strings.Index(text, "sec =")
	if start < 0 {
		return 0, fmt.Errorf("unexpected kern.boottime format")
	}
	value := strings.Fields(strings.TrimSpace(text[start+len("sec ="):]))
	if len(value) == 0 {
		return 0, fmt.Errorf("unexpected kern.boottime format")
	}
	boot, err := strconv.ParseInt(strings.TrimRight(value[0], ",}"), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse kern.boottime: %w", err)
	}
	uptime := float64(time.Now().Unix() - boot)
	if uptime < 0 {
		return 0, fmt.Errorf("system boot time is in the future")
	}
	return uptime, nil
}

func LoadAverageStats() (map[string]float64, error) {
	var data []byte
	var err error
	if runtime.GOOS == "linux" {
		data, err = os.ReadFile("/proc/loadavg")
	} else {
		data, err = exec.Command("sysctl", "-n", "vm.loadavg").Output()
	}
	if err != nil {
		return nil, fmt.Errorf("read system load average: %w", err)
	}
	parts := strings.Fields(strings.Trim(string(data), " {}"))
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid load average response")
	}
	result := make(map[string]float64, 3)
	for i, key := range []string{"1m", "5m", "15m"} {
		value, err := strconv.ParseFloat(parts[i], 64)
		if err != nil {
			return nil, fmt.Errorf("parse %s load average: %w", key, err)
		}
		result[key] = value
	}
	return result, nil
}

// LoadAverage is a compatibility wrapper. Use LoadAverageStats to detect
// unsupported platforms without treating them as valid zero load.
func LoadAverage() map[string]float64 {
	result, err := LoadAverageStats()
	if err != nil {
		return nil
	}
	return result
}

// SystemProcesses enumerates running processes on Unix by scanning /proc.
func SystemProcesses() ([]map[string]interface{}, error) {
	if runtime.GOOS != "linux" {
		output, err := exec.Command("ps", "-axo", "pid=,comm=").Output()
		if err != nil {
			return nil, fmt.Errorf("enumerate processes with ps: %w", err)
		}
		var processes []map[string]interface{}
		for _, line := range strings.Split(string(output), "\n") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			pid, err := strconv.ParseInt(fields[0], 10, 64)
			if err != nil {
				continue
			}
			processes = append(processes, map[string]interface{}{"pid": pid, "name": strings.Join(fields[1:], " ")})
		}
		return processes, nil
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}

	var list []map[string]interface{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.ParseInt(entry.Name(), 10, 64)
		if err != nil {
			continue
		}
		commBytes, _ := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
		name := strings.TrimSpace(string(commBytes))
		if name == "" {
			name = entry.Name()
		}
		list = append(list, map[string]interface{}{
			"pid":  pid,
			"name": name,
		})
	}
	return list, nil
}

// DiskPartitions lists mount points and disk usage on Unix.
func DiskPartitions() ([]map[string]interface{}, error) {
	if runtime.GOOS != "linux" {
		output, err := exec.Command("df", "-Pk").Output()
		if err != nil {
			return nil, fmt.Errorf("enumerate disk partitions with df: %w", err)
		}
		var result []map[string]interface{}
		for i, line := range strings.Split(string(output), "\n") {
			if i == 0 {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) < 6 {
				continue
			}
			totalKB, totalErr := strconv.ParseUint(fields[1], 10, 64)
			usedKB, usedErr := strconv.ParseUint(fields[2], 10, 64)
			freeKB, freeErr := strconv.ParseUint(fields[3], 10, 64)
			if totalErr != nil || usedErr != nil || freeErr != nil || totalKB == 0 {
				continue
			}
			percent := math.Round(float64(usedKB)/float64(totalKB)*1000) / 10
			result = append(result, map[string]interface{}{
				"device": fields[0], "mount": fields[len(fields)-1],
				"total_gb":     math.Round(float64(totalKB)/1024/1024*100) / 100,
				"free_gb":      math.Round(float64(freeKB)/1024/1024*100) / 100,
				"used_gb":      math.Round(float64(usedKB)/1024/1024*100) / 100,
				"percent_used": percent,
			})
		}
		if len(result) == 0 {
			return nil, fmt.Errorf("df returned no usable mounted filesystems")
		}
		return result, nil
	}
	f, err := os.Open("/proc/mounts")
	if err != nil {
		tot, free, used, pct, statErr := DiskUsageStats("/")
		if statErr != nil {
			return nil, fmt.Errorf("read mounted filesystems and root disk usage: %w", statErr)
		}
		toGB := func(b uint64) float64 {
			return math.Round((float64(b)/(1024*1024*1024))*100) / 100
		}
		return []map[string]interface{}{
			{
				"mount":        "/",
				"total_gb":     toGB(tot),
				"free_gb":      toGB(free),
				"used_gb":      toGB(used),
				"percent_used": pct,
			},
		}, nil
	}
	defer f.Close()

	var result []map[string]interface{}
	scanner := bufio.NewScanner(f)
	seen := make(map[string]bool)

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && strings.HasPrefix(fields[0], "/dev/") {
			mount := fields[1]
			if seen[mount] {
				continue
			}
			seen[mount] = true
			tot, free, used, pct, err := DiskUsageStats(mount)
			if err == nil && tot > 0 {
				toGB := func(b uint64) float64 {
					return math.Round((float64(b)/(1024*1024*1024))*100) / 100
				}
				result = append(result, map[string]interface{}{
					"device":       fields[0],
					"mount":        mount,
					"total_gb":     toGB(tot),
					"free_gb":      toGB(free),
					"used_gb":      toGB(used),
					"percent_used": pct,
				})
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func unixPhysicalMemory() (totalBytes, freeBytes uint64, err error) {
	read := func(name string) (uint64, error) {
		out, err := exec.Command("sysctl", "-n", name).Output()
		if err != nil {
			return 0, fmt.Errorf("read sysctl %s: %w", name, err)
		}
		value, err := strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse sysctl %s: %w", name, err)
		}
		return value, nil
	}
	var totalKey, pageSizeKey, freeKey, inactiveKey, speculativeKey string
	switch runtime.GOOS {
	case "darwin":
		totalKey, pageSizeKey = "hw.memsize", "hw.pagesize"
		freeKey, inactiveKey, speculativeKey = "vm.page_free_count", "vm.page_inactive_count", "vm.page_speculative_count"
	case "freebsd":
		totalKey, pageSizeKey = "hw.physmem", "vm.stats.vm.v_page_size"
		freeKey, inactiveKey = "vm.stats.vm.v_free_count", "vm.stats.vm.v_inactive_count"
	case "openbsd":
		totalKey, pageSizeKey = "hw.physmem", "vm.uvmexp.pagesize"
		freeKey, inactiveKey = "vm.uvmexp.free", "vm.uvmexp.inactive"
	case "netbsd":
		totalKey, pageSizeKey = "hw.physmem64", "vm.uvmexp.pagesize"
		freeKey, inactiveKey = "vm.uvmexp.free", "vm.uvmexp.inactive"
	default:
		return 0, 0, fmt.Errorf("system memory statistics are unsupported on %s", runtime.GOOS)
	}
	totalBytes, err = read(totalKey)
	if err != nil {
		return 0, 0, err
	}
	pageSize, err := read(pageSizeKey)
	if err != nil {
		return 0, 0, err
	}
	freePages, err := read(freeKey)
	if err != nil {
		return 0, 0, err
	}
	inactivePages, err := read(inactiveKey)
	if err != nil {
		return 0, 0, err
	}
	availablePages := freePages + inactivePages
	if speculativeKey != "" {
		speculativePages, err := read(speculativeKey)
		if err != nil {
			return 0, 0, err
		}
		availablePages += speculativePages
	}
	if pageSize == 0 || availablePages > ^uint64(0)/pageSize {
		return 0, 0, fmt.Errorf("invalid system memory page counters")
	}
	return totalBytes, availablePages * pageSize, nil
}
