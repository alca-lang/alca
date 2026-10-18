// Package stdlib provides the Sys standard library module for the Alca programming language.
// It supports hardware monitoring, disk usage, RAM statistics, CPU metrics, and network interfaces.
package stdlib

import (
	"math"
	"net"
	"os"
	"runtime"

	"github.com/alca-lang/alca/pkg/interpreter"
)

// RegisterSysBuiltins registers the "std.sys" and "sys" modules into the interpreter environment.
func RegisterSysBuiltins(env *interpreter.Environment) {
	sysModule := interpreter.NewMap()

	register := func(name string, fn func(args ...interpreter.Value) interpreter.Value) {
		bf := &interpreter.BuiltinFunction{Name: name, Fn: fn}
		sysModule.Set(name, bf)
	}

	// disk_usage(path?) -> map {path, total_bytes, free_bytes, used_bytes, total_gb, free_gb, used_gb, percent_used}
	register("disk_usage", func(args ...interpreter.Value) interpreter.Value {
		targetPath := "."
		if len(args) >= 1 {
			targetPath = interpreter.CoerceToString(args[0])
		}

		totalBytes, freeBytes, usedBytes, percent, err := DiskUsageStats(targetPath)
		if err != nil {
			return interpreter.NewError(500, "failed to read disk usage: "+err.Error())
		}

		m := interpreter.NewMap()
		m.Set("path", &interpreter.StringValue{Val: targetPath})
		m.Set("total_bytes", &interpreter.IntegerValue{Val: int64(totalBytes)})
		m.Set("free_bytes", &interpreter.IntegerValue{Val: int64(freeBytes)})
		m.Set("used_bytes", &interpreter.IntegerValue{Val: int64(usedBytes)})

		toGB := func(b uint64) float64 {
			return math.Round((float64(b)/(1024*1024*1024))*100) / 100
		}
		m.Set("total_gb", &interpreter.FloatValue{Val: toGB(totalBytes)})
		m.Set("free_gb", &interpreter.FloatValue{Val: toGB(freeBytes)})
		m.Set("used_gb", &interpreter.FloatValue{Val: toGB(usedBytes)})
		m.Set("percent_used", &interpreter.FloatValue{Val: percent})

		return m
	})

	// ram_usage() -> map {total_bytes, free_bytes, used_bytes, total_mb, free_mb, used_mb, percent_used}
	register("ram_usage", func(args ...interpreter.Value) interpreter.Value {
		totalBytes, freeBytes, usedBytes, percent, err := RamUsageStats()
		if err != nil {
			// Fallback to Go runtime memory statistics
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			totalBytes = m.Sys
			usedBytes = m.Alloc
			if totalBytes > usedBytes {
				freeBytes = totalBytes - usedBytes
			}
			if totalBytes > 0 {
				percent = math.Round((float64(usedBytes)/float64(totalBytes))*1000) / 10
			}
		}

		m := interpreter.NewMap()
		m.Set("total_bytes", &interpreter.IntegerValue{Val: int64(totalBytes)})
		m.Set("free_bytes", &interpreter.IntegerValue{Val: int64(freeBytes)})
		m.Set("used_bytes", &interpreter.IntegerValue{Val: int64(usedBytes)})

		toMB := func(b uint64) float64 {
			return math.Round((float64(b)/(1024*1024))*10) / 10
		}
		m.Set("total_mb", &interpreter.FloatValue{Val: toMB(totalBytes)})
		m.Set("free_mb", &interpreter.FloatValue{Val: toMB(freeBytes)})
		m.Set("used_mb", &interpreter.FloatValue{Val: toMB(usedBytes)})
		m.Set("percent_used", &interpreter.FloatValue{Val: percent})

		return m
	})

	// cpu_count() -> int
	register("cpu_count", func(args ...interpreter.Value) interpreter.Value {
		return &interpreter.IntegerValue{Val: int64(runtime.NumCPU())}
	})

	// arch() -> string (e.g. "amd64", "arm64")
	register("arch", func(args ...interpreter.Value) interpreter.Value {
		return &interpreter.StringValue{Val: runtime.GOARCH}
	})

	// platform() -> string ("windows", "linux", "darwin")
	register("platform", func(args ...interpreter.Value) interpreter.Value {
		return &interpreter.StringValue{Val: runtime.GOOS}
	})

	// network_interfaces() -> list of maps {name, mac, ip, ips, is_up, is_loopback}
	register("network_interfaces", func(args ...interpreter.Value) interpreter.Value {
		ifaces, err := net.Interfaces()
		if err != nil {
			return interpreter.NewError(500, err.Error())
		}

		var ifaceList []interpreter.Value
		for _, iface := range ifaces {
			m := interpreter.NewMap()
			m.Set("name", &interpreter.StringValue{Val: iface.Name})
			m.Set("mac", &interpreter.StringValue{Val: iface.HardwareAddr.String()})

			if iface.Flags&net.FlagUp != 0 {
				m.Set("is_up", interpreter.TRUE)
			} else {
				m.Set("is_up", interpreter.FALSE)
			}

			if iface.Flags&net.FlagLoopback != 0 {
				m.Set("is_loopback", interpreter.TRUE)
			} else {
				m.Set("is_loopback", interpreter.FALSE)
			}

			var ipList []interpreter.Value
			primaryIP := ""

			addrs, err := iface.Addrs()
			if err == nil {
				for _, addr := range addrs {
					var ip net.IP
					switch v := addr.(type) {
					case *net.IPNet:
						ip = v.IP
					case *net.IPAddr:
						ip = v.IP
					}
					if ip != nil {
						ipStr := ip.String()
						ipList = append(ipList, &interpreter.StringValue{Val: ipStr})
						if primaryIP == "" && ip.To4() != nil && !ip.IsLoopback() {
							primaryIP = ipStr
						}
					}
				}
			}

			m.Set("ips", interpreter.NewList(ipList...))
			m.Set("ip", &interpreter.StringValue{Val: primaryIP})

			ifaceList = append(ifaceList, m)
		}

		return interpreter.NewList(ifaceList...)
	})

	// uptime() -> float (seconds)
	register("uptime", func(args ...interpreter.Value) interpreter.Value {
		return &interpreter.FloatValue{Val: SystemUptime()}
	})

	// whoami() -> map {user, hostname, is_admin}
	register("whoami", func(args ...interpreter.Value) interpreter.Value {
		m := interpreter.NewMap()
		m.Set("user", &interpreter.StringValue{Val: CurrentUserName()})
		host, _ := os.Hostname()
		m.Set("hostname", &interpreter.StringValue{Val: host})
		if IsAdmin() {
			m.Set("is_admin", interpreter.TRUE)
		} else {
			m.Set("is_admin", interpreter.FALSE)
		}
		return m
	})

	// process_list() -> list of maps {pid, ppid, name}
	register("process_list", func(args ...interpreter.Value) interpreter.Value {
		procs, err := SystemProcesses()
		if err != nil {
			if IsPermissionError(err) {
				LogPermissionDenied("process_list", "system", err)
				return interpreter.NewError(403, "access denied enumerating processes: "+err.Error())
			}
			return interpreter.NewError(500, err.Error())
		}
		var list []interpreter.Value
		for _, p := range procs {
			m := interpreter.NewMap()
			if pid, ok := p["pid"].(int64); ok {
				m.Set("pid", &interpreter.IntegerValue{Val: pid})
			}
			if ppid, ok := p["ppid"].(int64); ok {
				m.Set("ppid", &interpreter.IntegerValue{Val: ppid})
			}
			if name, ok := p["name"].(string); ok {
				m.Set("name", &interpreter.StringValue{Val: name})
			}
			list = append(list, m)
		}
		return interpreter.NewList(list...)
	})

	// disk_partitions() -> list of maps {mount, total_gb, free_gb, used_gb, percent_used}
	register("disk_partitions", func(args ...interpreter.Value) interpreter.Value {
		parts, err := DiskPartitions()
		if err != nil {
			return interpreter.NewError(500, err.Error())
		}
		var list []interpreter.Value
		for _, p := range parts {
			m := interpreter.NewMap()
			for k, v := range p {
				switch val := v.(type) {
				case string:
					m.Set(k, &interpreter.StringValue{Val: val})
				case float64:
					m.Set(k, &interpreter.FloatValue{Val: val})
				case int64:
					m.Set(k, &interpreter.IntegerValue{Val: val})
				}
			}
			list = append(list, m)
		}
		return interpreter.NewList(list...)
	})

	// load_avg() -> map {1m, 5m, 15m}
	register("load_avg", func(args ...interpreter.Value) interpreter.Value {
		loads := LoadAverage()
		m := interpreter.NewMap()
		for k, v := range loads {
			m.Set(k, &interpreter.FloatValue{Val: v})
		}
		return m
	})

	env.Set("sys", sysModule)
	env.Set("std.sys", sysModule)
}
