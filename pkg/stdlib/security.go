// Package stdlib provides security auditing, access control validation,
// and permission error tracking for the Alca programming language.
package stdlib

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/sandbox"
)

// AccessDenialEvent captures a security or permission access failure.
type AccessDenialEvent struct {
	Timestamp string `json:"time"`
	Operation string `json:"operation"`
	Resource  string `json:"resource"`
	User      string `json:"user"`
	IsAdmin   bool   `json:"is_admin"`
	Error     string `json:"error"`
}

var (
	recentAccessErrorsMu sync.RWMutex
	recentAccessErrors   []AccessDenialEvent
	maxRecentErrors      = 50
)

// IsPermissionError checks whether an error is caused by OS access denial or privilege violation.
func IsPermissionError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, os.ErrPermission) || errors.Is(err, fs.ErrPermission) || os.IsPermission(err) {
		return true
	}

	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		if errors.Is(pathErr.Err, os.ErrPermission) || os.IsPermission(pathErr.Err) {
			return true
		}
		errStr := strings.ToLower(pathErr.Err.Error())
		if strings.Contains(errStr, "access is denied") ||
			strings.Contains(errStr, "permission denied") ||
			strings.Contains(errStr, "operation not permitted") ||
			strings.Contains(errStr, "privilege not held") {
			return true
		}
	}

	var linkErr *os.LinkError
	if errors.As(err, &linkErr) {
		if os.IsPermission(linkErr.Err) {
			return true
		}
	}

	errStr := strings.ToLower(err.Error())
	if strings.Contains(errStr, "access is denied") ||
		strings.Contains(errStr, "permission denied") ||
		strings.Contains(errStr, "operation not permitted") ||
		strings.Contains(errStr, "privilege not held") ||
		strings.Contains(errStr, "access denied") {
		return true
	}

	return isPlatformPermissionError(err)
}

// LogPermissionDenied logs a structured security audit record for an access failure.
func LogPermissionDenied(operation, resource string, err error) {
	username := CurrentUserName()
	isAdmin := IsAdmin()
	errText := ""
	if err != nil {
		errText = err.Error()
	}

	event := AccessDenialEvent{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Operation: operation,
		Resource:  resource,
		User:      username,
		IsAdmin:   isAdmin,
		Error:     errText,
	}

	recentAccessErrorsMu.Lock()
	recentAccessErrors = append(recentAccessErrors, event)
	if len(recentAccessErrors) > maxRecentErrors {
		recentAccessErrors = recentAccessErrors[len(recentAccessErrors)-maxRecentErrors:]
	}
	recentAccessErrorsMu.Unlock()

	fields := map[string]interface{}{
		"category": "ACCESS_DENIED",
		"op":       operation,
		"resource": resource,
		"user":     username,
		"is_admin": isAdmin,
	}
	if errText != "" {
		fields["err"] = errText
	}

	msg := fmt.Sprintf("[SECURITY_AUDIT] Access denied: %s on %q (user=%s, admin=%v)", operation, resource, username, isAdmin)
	globalLogger.writeLog(levelError, msg, fields)
}

// GetRecentAccessErrors returns all recently recorded access violation events.
func GetRecentAccessErrors() []AccessDenialEvent {
	recentAccessErrorsMu.RLock()
	defer recentAccessErrorsMu.RUnlock()
	res := make([]AccessDenialEvent, len(recentAccessErrors))
	copy(res, recentAccessErrors)
	return res
}

// CheckFileAccess tests whether the current process can access path with the requested mode.
// Mode can contain: "r" (read), "w" (write), "x" (execute).
func CheckFileAccess(path string, mode string) (bool, error) {
	cleanPath := filepath.Clean(path)
	modeLower := strings.ToLower(mode)
	if strings.Contains(modeLower, "r") {
		if err := sandbox.RequireRead(cleanPath); err != nil {
			return false, err
		}
	}
	if strings.Contains(modeLower, "w") {
		if err := sandbox.RequireWrite(cleanPath); err != nil {
			return false, err
		}
	}
	info, err := os.Stat(cleanPath)
	if err != nil {
		if IsPermissionError(err) {
			LogPermissionDenied("stat", cleanPath, err)
		}
		return false, err
	}

	// Check read access
	if strings.Contains(modeLower, "r") {
		if info.IsDir() {
			f, err := os.Open(cleanPath)
			if err != nil {
				if IsPermissionError(err) {
					LogPermissionDenied("read_dir", cleanPath, err)
				}
				return false, err
			}
			f.Close()
		} else {
			f, err := os.OpenFile(cleanPath, os.O_RDONLY, 0)
			if err != nil {
				if IsPermissionError(err) {
					LogPermissionDenied("read_file", cleanPath, err)
				}
				return false, err
			}
			f.Close()
		}
	}

	// Check write access
	if strings.Contains(modeLower, "w") {
		if info.IsDir() {
			testFile := filepath.Join(cleanPath, fmt.Sprintf(".alca_perm_test_%d", time.Now().UnixNano()))
			f, err := os.Create(testFile)
			if err != nil {
				if IsPermissionError(err) {
					LogPermissionDenied("write_dir", cleanPath, err)
				}
				return false, err
			}
			f.Close()
			_ = os.Remove(testFile)
		} else {
			f, err := os.OpenFile(cleanPath, os.O_WRONLY|os.O_APPEND, 0)
			if err != nil {
				if IsPermissionError(err) {
					LogPermissionDenied("write_file", cleanPath, err)
				}
				return false, err
			}
			f.Close()
		}
	}

	// Check execute access
	if strings.Contains(modeLower, "x") {
		return checkExecutableAccess(cleanPath, info)
	}

	return true, nil
}

// GetFilePermissions returns file permission details including octal and symbol representation.
func GetFilePermissions(path string) (map[string]interface{}, error) {
	cleanPath := filepath.Clean(path)
	if err := sandbox.RequireRead(cleanPath); err != nil {
		return nil, err
	}
	info, err := os.Stat(cleanPath)
	if err != nil {
		if IsPermissionError(err) {
			LogPermissionDenied("stat", cleanPath, err)
		}
		return nil, err
	}

	perm := info.Mode().Perm()
	octal := fmt.Sprintf("%04o", perm)
	modeStr := info.Mode().String()

	readable, _ := CheckFileAccess(cleanPath, "r")
	writable, _ := CheckFileAccess(cleanPath, "w")
	executable, _ := CheckFileAccess(cleanPath, "x")

	return map[string]interface{}{
		"octal":         octal,
		"mode":          modeStr,
		"is_dir":        info.IsDir(),
		"is_readable":   readable,
		"is_writable":   writable,
		"is_executable": executable,
	}, nil
}

// ChangeFilePermissions updates file mode permissions.
func ChangeFilePermissions(path string, mode os.FileMode) error {
	cleanPath := filepath.Clean(path)
	if err := sandbox.RequireWrite(cleanPath); err != nil {
		return err
	}
	err := os.Chmod(cleanPath, mode)
	if err != nil {
		if IsPermissionError(err) {
			LogPermissionDenied("chmod", cleanPath, err)
		}
		return err
	}
	return nil
}

// RegisterSecurityBuiltins registers audit and security builtins into std.log and environment.
func RegisterSecurityBuiltins(env *interpreter.Environment) {
	logModuleVal, hasLog := env.Get("std.log")
	var logMap *interpreter.MapValue
	if hasLog {
		if m, ok := logModuleVal.(*interpreter.MapValue); ok {
			logMap = m
		}
	}
	if logMap == nil {
		logMap = interpreter.NewMap()
	}

	// log.access_denied(operation, resource, [reason], [fields])
	accessDeniedFn := &interpreter.BuiltinFunction{
		Name: "access_denied",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			if len(args) < 2 {
				return interpreter.NewError(400, "access_denied requires operation and resource arguments")
			}
			op := interpreter.CoerceToString(args[0])
			resource := interpreter.CoerceToString(args[1])
			reason := "permission denied"
			if len(args) >= 3 {
				reason = interpreter.CoerceToString(args[2])
			}
			LogPermissionDenied(op, resource, errors.New(reason))
			return interpreter.TRUE
		},
	}
	logMap.Set("access_denied", accessDeniedFn)

	// log.audit(category, action, resource, status, [details])
	auditFn := &interpreter.BuiltinFunction{
		Name: "audit",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			if len(args) < 4 {
				return interpreter.NewError(400, "audit requires category, action, resource, and status")
			}
			cat := interpreter.CoerceToString(args[0])
			act := interpreter.CoerceToString(args[1])
			res := interpreter.CoerceToString(args[2])
			status := interpreter.CoerceToString(args[3])
			fields := map[string]interface{}{
				"category": cat,
				"action":   act,
				"resource": res,
				"status":   status,
				"user":     CurrentUserName(),
				"admin":    IsAdmin(),
			}
			if len(args) >= 5 {
				if m, ok := args[4].(*interpreter.MapValue); ok {
					for _, k := range m.Order {
						if v, ok := m.Get(k); ok {
							fields[k] = interpreter.CoerceToString(v)
						}
					}
				} else {
					fields["details"] = interpreter.CoerceToString(args[4])
				}
			}
			msg := fmt.Sprintf("[AUDIT] %s:%s on %q -> %s", cat, act, res, status)
			lvl := levelInfo
			if strings.EqualFold(status, "denied") || strings.EqualFold(status, "failed") || strings.EqualFold(status, "forbidden") {
				lvl = levelError
			}
			globalLogger.writeLog(lvl, msg, fields)
			return interpreter.TRUE
		},
	}
	logMap.Set("audit", auditFn)

	// log.get_access_errors() -> list of maps
	getAccessErrorsFn := &interpreter.BuiltinFunction{
		Name: "get_access_errors",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			events := GetRecentAccessErrors()
			resList := make([]interpreter.Value, len(events))
			for i, ev := range events {
				m := interpreter.NewMap()
				m.Set("time", &interpreter.StringValue{Val: ev.Timestamp})
				m.Set("operation", &interpreter.StringValue{Val: ev.Operation})
				m.Set("resource", &interpreter.StringValue{Val: ev.Resource})
				m.Set("user", &interpreter.StringValue{Val: ev.User})
				if ev.IsAdmin {
					m.Set("is_admin", interpreter.TRUE)
				} else {
					m.Set("is_admin", interpreter.FALSE)
				}
				m.Set("error", &interpreter.StringValue{Val: ev.Error})
				resList[i] = m
			}
			return interpreter.NewList(resList...)
		},
	}
	logMap.Set("get_access_errors", getAccessErrorsFn)

	env.Set("log", logMap)
	env.Set("std.log", logMap)
}
