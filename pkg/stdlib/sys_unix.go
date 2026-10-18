//go:build !windows

package stdlib

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"syscall"
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

	totalBytes = stat.Blocks * uint64(stat.Bsize)
	freeBytes = stat.Bavail * uint64(stat.Bsize)
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
				kb, _ := strconv.ParseUint(fields[1], 10, 64)
				memTotal = kb * 1024
			} else if fields[0] == "MemAvailable:" {
				kb, _ := strconv.ParseUint(fields[1], 10, 64)
				memAvailable = kb * 1024
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
	data, err := os.ReadFile("/proc/uptime")
	if err == nil {
		parts := strings.Fields(string(data))
		if len(parts) >= 1 {
			if sec, err := strconv.ParseFloat(parts[0], 64); err == nil {
				return sec
			}
		}
	}
	return 0
}

// SystemProcesses enumerates running processes on Unix by scanning /proc.
func SystemProcesses() ([]map[string]interface{}, error) {
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
	f, err := os.Open("/proc/mounts")
	if err != nil {
		tot, free, used, pct, _ := DiskUsageStats("/")
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
	return result, nil
}

// LoadAverage returns 1m, 5m, 15m load averages on Unix.
func LoadAverage() map[string]float64 {
	res := map[string]float64{"1m": 0.0, "5m": 0.0, "15m": 0.0}
	data, err := os.ReadFile("/proc/loadavg")
	if err == nil {
		parts := strings.Fields(string(data))
		if len(parts) >= 3 {
			if v1, err := strconv.ParseFloat(parts[0], 64); err == nil {
				res["1m"] = v1
			}
			if v5, err := strconv.ParseFloat(parts[1], 64); err == nil {
				res["5m"] = v5
			}
			if v15, err := strconv.ParseFloat(parts[2], 64); err == nil {
				res["15m"] = v15
			}
		}
	}
	return res
}
