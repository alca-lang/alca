// Package interpreter implements the tree-walking runtime engine for the Alca programming language.
package interpreter

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

var (
	asyncWg   sync.WaitGroup
	streamClt = &http.Client{Timeout: 15 * time.Second}
)

// WaitAll blocks until all asynchronous streaming, threads, and background workers finish.
func WaitAll() {
	asyncWg.Wait()
}

// AddAsync increments the background async WaitGroup counter.
func AddAsync() {
	asyncWg.Add(1)
}

// DoneAsync decrements the background async WaitGroup counter.
func DoneAsync() {
	asyncWg.Done()
}

// ReadFile reads the full content of a file at the specified path and returns it as a string.
func ReadFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// WriteFile writes string content to a file, appending if appendMode is true or truncating otherwise.
func WriteFile(path string, content string, appendMode bool) error {
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	flags := os.O_CREATE | os.O_WRONLY
	if appendMode {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}

	f, err := os.OpenFile(path, flags, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(content)
	return err
}

// FileExists checks whether a file or directory exists at the given path.
func FileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// writeToNetwork sends content to an HTTP or WebSocket destination URL.
func writeToNetwork(targetURL, content string) error {
	if strings.HasPrefix(targetURL, "ws://") || strings.HasPrefix(targetURL, "wss://") {
		// Fall back to HTTP post for websocket endpoint simulation if no full WS library is attached
		targetURL = strings.Replace(targetURL, "ws://", "http://", 1)
		targetURL = strings.Replace(targetURL, "wss://", "https://", 1)
	}

	req, err := http.NewRequest(http.MethodPost, targetURL, bytes.NewBufferString(content))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "text/plain")

	resp, err := streamClt.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

// isNullTarget checks if a target value denotes null or discarded output.
func isNullTarget(v Value) bool {
	if v == nil || v.Type() == NULL_VAL {
		return true
	}
	if s, ok := v.(*StringValue); ok && (s.Val == "null" || s.Val == "/dev/null" || s.Val == "NUL") {
		return true
	}
	return false
}

// HandleStream executes a stream redirection or pipeline operation between source and target values.
func HandleStream(source Value, operator string, target Value, env *Environment) (Value, error) {
	switch operator {
	case ">>": // STREAM_APPEND
		if isNullTarget(target) {
			return NULL, nil
		}
		targetPath := CoerceToString(target)
		content := CoerceToString(source) + "\n"
		if strings.HasPrefix(targetPath, "http://") || strings.HasPrefix(targetPath, "https://") ||
			strings.HasPrefix(targetPath, "ws://") || strings.HasPrefix(targetPath, "wss://") {
			if err := writeToNetwork(targetPath, content); err != nil {
				return nil, fmt.Errorf("network stream write failed: %w", err)
			}
			return source, nil
		}
		if err := WriteFile(targetPath, content, true); err != nil {
			return nil, fmt.Errorf("stream append error: %w", err)
		}
		return source, nil

	case ">!": // STREAM_OVERWRITE
		if isNullTarget(target) {
			return NULL, nil
		}
		targetPath := CoerceToString(target)
		content := CoerceToString(source)
		if strings.HasPrefix(targetPath, "http://") || strings.HasPrefix(targetPath, "https://") {
			if err := writeToNetwork(targetPath, content); err != nil {
				return nil, fmt.Errorf("network stream write failed: %w", err)
			}
			return source, nil
		}
		if err := WriteFile(targetPath, content, false); err != nil {
			return nil, fmt.Errorf("stream overwrite error: %w", err)
		}
		return source, nil

	case ">>>": // STREAM_VAR (store value in named variable)
		varName := CoerceToString(target)
		if varName == "" {
			return nil, fmt.Errorf("stream variable target cannot be empty")
		}
		if _, ok := env.Update(varName, source); !ok {
			env.Set(varName, source)
		}
		return source, nil

	case "<<": // STREAM_IN (read file or console into value)
		targetStr := ""
		if target != nil {
			targetStr = CoerceToString(target)
		}
		if targetStr == "" || targetStr == "console" || targetStr == "stdin" {
			scanner := bufio.NewScanner(os.Stdin)
			if scanner.Scan() {
				readVal := &StringValue{Val: scanner.Text()}
				if source != nil && source.Type() == STRING_VAL {
					env.Set(source.(*StringValue).Val, readVal)
				}
				return readVal, nil
			}
			return &StringValue{Val: ""}, nil
		}
		content, err := ReadFile(targetStr)
		if err != nil {
			return nil, fmt.Errorf("stream read error: %w", err)
		}
		readVal := &StringValue{Val: content}
		if source != nil && source.Type() == STRING_VAL {
			env.Set(source.(*StringValue).Val, readVal)
		}
		return readVal, nil

	case "?>>": // STREAM_ERR (append to file if source is ErrorValue)
		if errVal, ok := source.(*ErrorValue); ok {
			if isNullTarget(target) {
				return NULL, nil
			}
			targetPath := CoerceToString(target)
			content := fmt.Sprintf("[%s] Error(%d): %s\n", errVal.Time, errVal.Code, errVal.Message)
			if err := WriteFile(targetPath, content, true); err != nil {
				return nil, fmt.Errorf("stream error log failed: %w", err)
			}
		}
		return source, nil

	case "?>>>": // STREAM_ERR_VAR (store in variable if source is ErrorValue)
		if _, ok := source.(*ErrorValue); ok {
			varName := CoerceToString(target)
			if varName != "" {
				if _, updated := env.Update(varName, source); !updated {
					env.Set(varName, source)
				}
			}
		}
		return source, nil

	case "&>>": // STREAM_ASYNC (launch goroutine and write output to target)
		asyncWg.Add(1)
		go func(srcVal Value, tgtVal Value) {
			defer asyncWg.Done()
			if isNullTarget(tgtVal) {
				return
			}
			path := CoerceToString(tgtVal)
			content := CoerceToString(srcVal) + "\n"
			_ = WriteFile(path, content, true)
		}(source, target)
		return source, nil

	case "&&>>": // STREAM_THREAD (dedicated OS thread write)
		asyncWg.Add(1)
		go func(srcVal Value, tgtVal Value) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			defer asyncWg.Done()
			if isNullTarget(tgtVal) {
				return
			}
			path := CoerceToString(tgtVal)
			content := CoerceToString(srcVal) + "\n"
			_ = WriteFile(path, content, true)
		}(source, target)
		return source, nil

	case "|>": // PIPE (pipe output of source into target callable)
		res, err := invokeCallable(target, []Value{source})
		if err != nil {
			return nil, fmt.Errorf("pipeline invocation failed: %w", err)
		}
		return res, nil

	default:
		return nil, fmt.Errorf("unknown stream operator %q", operator)
	}
}
