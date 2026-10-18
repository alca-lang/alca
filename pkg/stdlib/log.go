// Package stdlib provides the Log standard library module for the Alca programming language.
// It supports structured logging with level filtering (DEBUG, INFO, WARN, ERROR, FATAL),
// text and JSON outputs, and automated file rotation.
package stdlib

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/alca-lang/alca/pkg/interpreter"
)

type logLevel int

const (
	levelDebug logLevel = iota
	levelInfo
	levelWarn
	levelError
	levelFatal
)

var levelNames = map[string]logLevel{
	"debug": levelDebug,
	"info":  levelInfo,
	"warn":  levelWarn,
	"error": levelError,
	"fatal": levelFatal,
}

var levelStrings = map[logLevel]string{
	levelDebug: "DEBUG",
	levelInfo:  "INFO",
	levelWarn:  "WARN",
	levelError: "ERROR",
	levelFatal: "FATAL",
}

type loggerState struct {
	mu           sync.Mutex
	minLevel     logLevel
	format       string // "text" or "json"
	filePath     string
	maxSizeBytes int64
	maxBackups   int
	currentFile  *os.File
	currentSize  int64
}

var globalLogger = &loggerState{
	minLevel:     levelInfo,
	format:       "text",
	maxSizeBytes: 10 * 1024 * 1024, // 10 MB default
	maxBackups:   5,
}

func (l *loggerState) rotateIfNeeded(incomingBytes int) error {
	if l.filePath == "" || l.currentFile == nil {
		return nil
	}

	if l.currentSize+int64(incomingBytes) < l.maxSizeBytes {
		return nil
	}

	l.currentFile.Close()
	l.currentFile = nil

	// Shift existing backup files
	ext := filepath.Ext(l.filePath)
	baseWithoutExt := strings.TrimSuffix(l.filePath, ext)

	oldest := fmt.Sprintf("%s.%d%s", baseWithoutExt, l.maxBackups, ext)
	_ = os.Remove(oldest)

	for i := l.maxBackups - 1; i >= 1; i-- {
		oldName := fmt.Sprintf("%s.%d%s", baseWithoutExt, i, ext)
		newName := fmt.Sprintf("%s.%d%s", baseWithoutExt, i+1, ext)
		_ = os.Rename(oldName, newName)
	}

	firstBackup := fmt.Sprintf("%s.1%s", baseWithoutExt, ext)
	_ = os.Rename(l.filePath, firstBackup)

	// Create fresh file
	f, err := os.OpenFile(l.filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	l.currentFile = f
	l.currentSize = 0
	return nil
}

func (l *loggerState) writeLog(lvl logLevel, message string, fields map[string]interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if lvl < l.minLevel {
		return
	}

	now := time.Now()
	lvlStr := levelStrings[lvl]
	var logLine string

	if l.format == "json" {
		record := make(map[string]interface{})
		record["time"] = now.Format(time.RFC3339)
		record["level"] = lvlStr
		record["msg"] = message
		for k, v := range fields {
			record[k] = v
		}
		data, err := json.Marshal(record)
		if err == nil {
			logLine = string(data) + "\n"
		} else {
			logLine = fmt.Sprintf(`{"time":%q,"level":%q,"msg":%q}`+"\n", now.Format(time.RFC3339), lvlStr, message)
		}
	} else {
		// Text format
		var fieldsStr strings.Builder
		for k, v := range fields {
			fieldsStr.WriteString(fmt.Sprintf(" %s=%v", k, v))
		}
		timeStr := now.Format("2006-01-02 15:04:05")
		logLine = fmt.Sprintf("%s [%s] %s%s\n", timeStr, lvlStr, message, fieldsStr.String())
	}

	// Output to stdout/stderr
	if lvl >= levelError {
		fmt.Fprint(os.Stderr, logLine)
	} else {
		fmt.Print(logLine)
	}

	// Output to log file if configured
	if l.filePath != "" {
		if l.currentFile == nil {
			f, err := os.OpenFile(l.filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if err == nil {
				l.currentFile = f
				if fi, err := f.Stat(); err == nil {
					l.currentSize = fi.Size()
				}
			}
		}
		if l.currentFile != nil {
			_ = l.rotateIfNeeded(len(logLine))
			if l.currentFile != nil {
				n, _ := l.currentFile.WriteString(logLine)
				l.currentSize += int64(n)
			}
		}
	}
}

// RegisterLogBuiltins registers the "std.log" and "log" modules into the interpreter environment.
func RegisterLogBuiltins(env *interpreter.Environment) {
	logModule := interpreter.NewMap()

	extractFields := func(args []interpreter.Value, offset int) map[string]interface{} {
		fields := make(map[string]interface{})
		if len(args) > offset {
			if m, ok := args[offset].(*interpreter.MapValue); ok {
				for _, k := range m.Order {
					if v, ok := m.Get(k); ok {
						fields[k] = interpreter.CoerceToString(v)
					}
				}
			}
		}
		return fields
	}

	registerLog := func(name string, lvl logLevel) {
		bf := &interpreter.BuiltinFunction{
			Name: name,
			Fn: func(args ...interpreter.Value) interpreter.Value {
				if len(args) < 1 {
					return interpreter.NULL
				}
				msg := interpreter.CoerceToString(args[0])
				fields := extractFields(args, 1)
				globalLogger.writeLog(lvl, msg, fields)
				return interpreter.NULL
			},
		}
		logModule.Set(name, bf)
	}

	registerLog("debug", levelDebug)
	registerLog("info", levelInfo)
	registerLog("warn", levelWarn)
	registerLog("error", levelError)
	registerLog("fatal", levelFatal)

	// set_level(level_name) -> bool
	logModule.Set("set_level", &interpreter.BuiltinFunction{
		Name: "set_level",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			if len(args) < 1 {
				return interpreter.FALSE
			}
			lvlName := strings.ToLower(interpreter.CoerceToString(args[0]))
			if lvl, ok := levelNames[lvlName]; ok {
				globalLogger.mu.Lock()
				globalLogger.minLevel = lvl
				globalLogger.mu.Unlock()
				return interpreter.TRUE
			}
			return interpreter.FALSE
		},
	})

	// set_format(format_name) -> bool ("text" or "json")
	logModule.Set("set_format", &interpreter.BuiltinFunction{
		Name: "set_format",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			if len(args) < 1 {
				return interpreter.FALSE
			}
			fmtName := strings.ToLower(interpreter.CoerceToString(args[0]))
			if fmtName == "json" || fmtName == "text" {
				globalLogger.mu.Lock()
				globalLogger.format = fmtName
				globalLogger.mu.Unlock()
				return interpreter.TRUE
			}
			return interpreter.FALSE
		},
	})

	// set_file(path, max_size_mb?, max_backups?) -> bool
	logModule.Set("set_file", &interpreter.BuiltinFunction{
		Name: "set_file",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			if len(args) < 1 {
				return interpreter.FALSE
			}
			path := interpreter.CoerceToString(args[0])
			maxMB := int64(10)
			maxBackups := 5

			if len(args) >= 2 {
				if iv, ok := args[1].(*interpreter.IntegerValue); ok && iv.Val > 0 {
					maxMB = iv.Val
				}
			}
			if len(args) >= 3 {
				if iv, ok := args[2].(*interpreter.IntegerValue); ok && iv.Val > 0 {
					maxBackups = int(iv.Val)
				}
			}

			globalLogger.mu.Lock()
			if globalLogger.currentFile != nil {
				globalLogger.currentFile.Close()
				globalLogger.currentFile = nil
			}
			globalLogger.filePath = path
			globalLogger.maxSizeBytes = maxMB * 1024 * 1024
			globalLogger.maxBackups = maxBackups
			globalLogger.mu.Unlock()
			return interpreter.TRUE
		},
	})

	env.Set("log", logModule)
	env.Set("std.log", logModule)
}
