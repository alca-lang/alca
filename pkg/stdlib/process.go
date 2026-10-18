// Package stdlib provides the standard library modules and builtins for the Alca programming language.
package stdlib

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/alca-lang/alca/pkg/interpreter"
)

var processStartTime = time.Now()

// Exec executes a shell command, waiting for it to complete and returning its output, exit code, and any error.
func Exec(cmd string) (string, int, error) {
	var c *exec.Cmd
	if runtime.GOOS == "windows" {
		c = exec.Command("cmd", "/c", cmd)
	} else {
		c = exec.Command("sh", "-c", cmd)
	}

	out, err := c.CombinedOutput()
	outputStr := string(out)

	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return outputStr, exitErr.ExitCode(), nil
		}
		return outputStr, 1, err
	}

	return outputStr, 0, nil
}

// ExecBackground starts a shell command asynchronously in the background and returns its process PID.
func ExecBackground(cmd string) (int, error) {
	var c *exec.Cmd
	if runtime.GOOS == "windows" {
		c = exec.Command("cmd", "/c", cmd)
	} else {
		c = exec.Command("sh", "-c", cmd)
	}

	if err := c.Start(); err != nil {
		return 0, err
	}

	// Detach and let it run
	go func() {
		_ = c.Wait()
	}()

	return c.Process.Pid, nil
}

// KillProcess terminates a process by its PID.
func KillProcess(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Kill()
}

// ProcessExists checks if a process with the given PID is currently active.
func ProcessExists(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, Signal(0) tests if process exists. On Windows, FindProcess does not verify existence.
	if runtime.GOOS != "windows" {
		err = proc.Signal(syscall.Signal(0))
		return err == nil
	}
	// On Windows, test via tasklist or OpenProcess probe
	cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid))
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), fmt.Sprintf("%d", pid))
}

// GetPID returns the process ID of the caller.
func GetPID() int {
	return os.Getpid()
}

// GetPPID returns the process ID of the caller's parent.
func GetPPID() int {
	return os.Getppid()
}

// Signal sends an OS signal to the specified process PID.
func Signal(pid int, sig string) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}

	sigUpper := strings.ToUpper(strings.TrimSpace(sig))
	switch sigUpper {
	case "SIGKILL", "KILL", "9":
		return proc.Kill()
	case "SIGTERM", "TERM", "15":
		return proc.Signal(syscall.SIGTERM)
	case "SIGINT", "INT", "2":
		return proc.Signal(syscall.SIGINT)
	default:
		return proc.Signal(syscall.SIGTERM)
	}
}

// Uptime returns the approximate system or process uptime in seconds.
func Uptime() int64 {
	return int64(time.Since(processStartTime).Seconds())
}

// MemoryUsage returns statistics on current process memory consumption in bytes.
func MemoryUsage() map[string]int64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return map[string]int64{
		"alloc":       int64(m.Alloc),
		"total_alloc": int64(m.TotalAlloc),
		"sys":         int64(m.Sys),
		"num_gc":      int64(m.NumGC),
	}
}

// CPUCount returns the number of logical CPUs available to the current process.
func CPUCount() int {
	return runtime.NumCPU()
}

// RegisterProcessBuiltins registers process management functions into the interpreter environment.
func RegisterProcessBuiltins(env *interpreter.Environment) {
	processModule := interpreter.NewMap()

	register := func(name string, fn func(args ...interpreter.Value) interpreter.Value) {
		builtin := &interpreter.BuiltinFunction{Name: name, Fn: fn}
		env.Set(name, builtin)
		processModule.Set(name, builtin)
	}

	register("exec_cmd", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "exec_cmd requires a command string")
		}
		cmdStr := interpreter.CoerceToString(args[0])
		out, code, err := Exec(cmdStr)
		if err != nil {
			return interpreter.NewError(500, err.Error())
		}
		res := interpreter.NewMap()
		res.Set("output", &interpreter.StringValue{Val: out})
		res.Set("exit_code", &interpreter.IntegerValue{Val: int64(code)})
		return res
	})

	register("exec_bg", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "exec_bg requires a command string")
		}
		cmdStr := interpreter.CoerceToString(args[0])
		pid, err := ExecBackground(cmdStr)
		if err != nil {
			return interpreter.NewError(500, err.Error())
		}
		return &interpreter.IntegerValue{Val: int64(pid)}
	})

	register("kill_process", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "kill_process requires a pid")
		}
		pid := int(0)
		if iv, ok := args[0].(*interpreter.IntegerValue); ok {
			pid = int(iv.Val)
		}
		if err := KillProcess(pid); err != nil {
			if IsPermissionError(err) {
				LogPermissionDenied("kill_process", fmt.Sprintf("pid=%d", pid), err)
				return interpreter.NewError(403, fmt.Sprintf("access denied killing process %d: %s", pid, err.Error()))
			}
			return interpreter.NewError(500, err.Error())
		}
		return interpreter.TRUE
	})

	register("process_exists", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.FALSE
		}
		pid := int(0)
		if iv, ok := args[0].(*interpreter.IntegerValue); ok {
			pid = int(iv.Val)
		}
		if ProcessExists(pid) {
			return interpreter.TRUE
		}
		return interpreter.FALSE
	})

	register("get_pid", func(args ...interpreter.Value) interpreter.Value {
		return &interpreter.IntegerValue{Val: int64(GetPID())}
	})

	register("get_ppid", func(args ...interpreter.Value) interpreter.Value {
		return &interpreter.IntegerValue{Val: int64(GetPPID())}
	})

	register("signal_process", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "signal_process requires pid and signal name")
		}
		pid := int(0)
		if iv, ok := args[0].(*interpreter.IntegerValue); ok {
			pid = int(iv.Val)
		}
		sig := interpreter.CoerceToString(args[1])
		if err := Signal(pid, sig); err != nil {
			return interpreter.NewError(500, err.Error())
		}
		return interpreter.TRUE
	})

	register("uptime", func(args ...interpreter.Value) interpreter.Value {
		return &interpreter.IntegerValue{Val: Uptime()}
	})

	register("memory_usage", func(args ...interpreter.Value) interpreter.Value {
		mem := MemoryUsage()
		m := interpreter.NewMap()
		for k, v := range mem {
			m.Set(k, &interpreter.IntegerValue{Val: v})
		}
		return m
	})

	register("cpu_count", func(args ...interpreter.Value) interpreter.Value {
		return &interpreter.IntegerValue{Val: int64(CPUCount())}
	})

	env.Set("process", processModule)
	env.Set("std.process", processModule)
}
