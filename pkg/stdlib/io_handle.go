// Package stdlib provides advanced stream-based file handles, seeking, locking,
// permissions management, and permission error handling for Alca.
package stdlib

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/sandbox"
)

var (
	pathLocksMu sync.Mutex
	pathLocks   = make(map[string]*os.File)
)

func handleIOError(op, path string, err error) interpreter.Value {
	if err == nil {
		return nil
	}
	if IsPermissionError(err) {
		LogPermissionDenied(op, path, err)
		return interpreter.NewError(403, fmt.Sprintf("access denied: %s on %q: %s", op, path, err.Error()))
	}
	return interpreter.NewError(500, err.Error())
}

// OpenFileHandle creates a streamable FileHandle object supporting read, write, seek, tell, lock, and close.
func OpenFileHandle(path, mode string) (*interpreter.MapValue, error) {
	if mode == "" {
		mode = "r"
	}

	var flag int
	modeLower := strings.ToLower(mode)

	// A handle that can write must have write permission; r+ reads and writes.
	if strings.ContainsAny(modeLower, "wa+") {
		if err := sandbox.RequireWrite(path); err != nil {
			return nil, err
		}
	}
	if modeLower == "r" || modeLower == "rb" || strings.Contains(modeLower, "+") {
		if err := sandbox.RequireRead(path); err != nil {
			return nil, err
		}
	}

	switch modeLower {
	case "r", "rb":
		flag = os.O_RDONLY
	case "w", "wb":
		flag = os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	case "a", "ab":
		flag = os.O_CREATE | os.O_WRONLY | os.O_APPEND
	case "r+", "rb+", "r+b":
		flag = os.O_RDWR
	case "w+", "wb+", "w+b":
		flag = os.O_CREATE | os.O_RDWR | os.O_TRUNC
	case "a+", "ab+", "a+b":
		flag = os.O_CREATE | os.O_RDWR | os.O_APPEND
	default:
		return nil, fmt.Errorf("unsupported open mode: %q (use 'r', 'w', 'a', 'r+', 'w+', 'a+')", mode)
	}

	f, err := os.OpenFile(path, flag, 0644)
	if err != nil {
		return nil, err
	}

	handleMap := interpreter.NewMap()
	handleMap.Set("path", &interpreter.StringValue{Val: path})
	handleMap.Set("mode", &interpreter.StringValue{Val: mode})
	handleMap.Set("is_open", interpreter.TRUE)

	var mu sync.Mutex
	isOpen := true

	// file.close() -> bool
	handleMap.Set("close", &interpreter.BuiltinFunction{
		Name: "close",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			mu.Lock()
			defer mu.Unlock()
			if !isOpen {
				return interpreter.FALSE
			}
			isOpen = false
			handleMap.Set("is_open", interpreter.FALSE)
			err := f.Close()
			if err != nil {
				return handleIOError("close", path, err)
			}
			return interpreter.TRUE
		},
	})

	// file.is_open() -> bool
	handleMap.Set("is_open_fn", &interpreter.BuiltinFunction{
		Name: "is_open",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			mu.Lock()
			defer mu.Unlock()
			if isOpen {
				return interpreter.TRUE
			}
			return interpreter.FALSE
		},
	})

	// file.read(n?: int) -> string
	handleMap.Set("read", &interpreter.BuiltinFunction{
		Name: "read",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			mu.Lock()
			defer mu.Unlock()
			if !isOpen {
				return interpreter.NewError(400, "cannot read from closed file handle")
			}

			n := int64(-1)
			if len(args) >= 1 {
				if iv, ok := args[0].(*interpreter.IntegerValue); ok {
					n = iv.Val
				}
			}

			if n <= 0 {
				data, err := io.ReadAll(f)
				if err != nil {
					return handleIOError("read", path, err)
				}
				return &interpreter.StringValue{Val: string(data)}
			}

			buf := make([]byte, n)
			readBytes, err := f.Read(buf)
			if err != nil && err != io.EOF {
				return handleIOError("read", path, err)
			}
			return &interpreter.StringValue{Val: string(buf[:readBytes])}
		},
	})

	// file.read_line() -> string
	handleMap.Set("read_line", &interpreter.BuiltinFunction{
		Name: "read_line",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			mu.Lock()
			defer mu.Unlock()
			if !isOpen {
				return interpreter.NewError(400, "cannot read from closed file handle")
			}

			var lineBuf bytes.Buffer
			b := make([]byte, 1)
			for {
				nr, err := f.Read(b)
				if nr > 0 {
					if b[0] == '\n' {
						break
					}
					if b[0] != '\r' {
						lineBuf.WriteByte(b[0])
					}
				}
				if err != nil {
					break
				}
			}
			return &interpreter.StringValue{Val: lineBuf.String()}
		},
	})

	// file.read_lines() -> list[string]
	handleMap.Set("read_lines", &interpreter.BuiltinFunction{
		Name: "read_lines",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			mu.Lock()
			defer mu.Unlock()
			if !isOpen {
				return interpreter.NewError(400, "cannot read from closed file handle")
			}

			scanner := bufio.NewScanner(f)
			var lines []interpreter.Value
			for scanner.Scan() {
				lines = append(lines, &interpreter.StringValue{Val: scanner.Text()})
			}
			if err := scanner.Err(); err != nil {
				return handleIOError("read_lines", path, err)
			}
			return interpreter.NewList(lines...)
		},
	})

	// file.write(content: string | blob) -> int (bytes written)
	handleMap.Set("write", &interpreter.BuiltinFunction{
		Name: "write",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			mu.Lock()
			defer mu.Unlock()
			if !isOpen {
				return interpreter.NewError(400, "cannot write to closed file handle")
			}
			if len(args) < 1 {
				return interpreter.NewError(400, "write requires data argument")
			}

			var b []byte
			switch v := args[0].(type) {
			case *interpreter.BlobValue:
				b = v.Data
			default:
				b = []byte(interpreter.CoerceToString(v))
			}

			n, err := f.Write(b)
			if err != nil {
				return handleIOError("write", path, err)
			}
			return &interpreter.IntegerValue{Val: int64(n)}
		},
	})

	// file.seek(offset: int, whence?: int) -> int (new position)
	handleMap.Set("seek", &interpreter.BuiltinFunction{
		Name: "seek",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			mu.Lock()
			defer mu.Unlock()
			if !isOpen {
				return interpreter.NewError(400, "cannot seek on closed file handle")
			}
			if len(args) < 1 {
				return interpreter.NewError(400, "seek requires offset argument")
			}

			offset := int64(0)
			if iv, ok := args[0].(*interpreter.IntegerValue); ok {
				offset = iv.Val
			}

			whence := io.SeekStart
			if len(args) >= 2 {
				if iv, ok := args[1].(*interpreter.IntegerValue); ok {
					switch iv.Val {
					case 0:
						whence = io.SeekStart
					case 1:
						whence = io.SeekCurrent
					case 2:
						whence = io.SeekEnd
					}
				}
			}

			pos, err := f.Seek(offset, whence)
			if err != nil {
				return handleIOError("seek", path, err)
			}
			return &interpreter.IntegerValue{Val: pos}
		},
	})

	// file.tell() -> int
	handleMap.Set("tell", &interpreter.BuiltinFunction{
		Name: "tell",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			mu.Lock()
			defer mu.Unlock()
			if !isOpen {
				return interpreter.NewError(400, "cannot tell on closed file handle")
			}
			pos, err := f.Seek(0, io.SeekCurrent)
			if err != nil {
				return handleIOError("tell", path, err)
			}
			return &interpreter.IntegerValue{Val: pos}
		},
	})

	// file.flush() -> bool
	handleMap.Set("flush", &interpreter.BuiltinFunction{
		Name: "flush",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			mu.Lock()
			defer mu.Unlock()
			if !isOpen {
				return interpreter.NewError(400, "cannot flush closed file handle")
			}
			if err := f.Sync(); err != nil {
				return handleIOError("flush", path, err)
			}
			return interpreter.TRUE
		},
	})

	// file.lock(exclusive?: bool) -> bool
	handleMap.Set("lock", &interpreter.BuiltinFunction{
		Name: "lock",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			mu.Lock()
			defer mu.Unlock()
			if !isOpen {
				return interpreter.NewError(400, "cannot lock closed file handle")
			}
			exclusive := true
			if len(args) >= 1 {
				exclusive = interpreter.IsTruthy(args[0])
			}
			err := LockFileHandle(f, exclusive)
			if err != nil {
				return handleIOError("lock", path, err)
			}
			return interpreter.TRUE
		},
	})

	// file.unlock() -> bool
	handleMap.Set("unlock", &interpreter.BuiltinFunction{
		Name: "unlock",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			mu.Lock()
			defer mu.Unlock()
			if !isOpen {
				return interpreter.NewError(400, "cannot unlock closed file handle")
			}
			err := UnlockFileHandle(f)
			if err != nil {
				return handleIOError("unlock", path, err)
			}
			return interpreter.TRUE
		},
	})

	return handleMap, nil
}

// RegisterAdvancedIOBuiltins registers file stream handles, seek/tell, locking, permissions into std.io.
func RegisterAdvancedIOBuiltins(env *interpreter.Environment, ioModule *interpreter.MapValue, register func(name string, fn func(args ...interpreter.Value) interpreter.Value)) {
	// io.open(path, [mode]) -> FileHandle
	register("open", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "open requires a path argument")
		}
		path := interpreter.CoerceToString(args[0])
		mode := "r"
		if len(args) >= 2 {
			mode = interpreter.CoerceToString(args[1])
		}

		handle, err := OpenFileHandle(path, mode)
		if err != nil {
			return handleIOError("open", path, err)
		}
		return handle
	})

	// io.check_access(path, mode) -> bool
	register("check_access", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "check_access requires path and mode arguments (e.g. 'r', 'w', 'rw')")
		}
		path := interpreter.CoerceToString(args[0])
		mode := interpreter.CoerceToString(args[1])

		ok, _ := CheckFileAccess(path, mode)
		if ok {
			return interpreter.TRUE
		}
		return interpreter.FALSE
	})

	// io.permissions(path) -> map {octal, mode, is_readable, is_writable, is_executable, is_dir}
	register("permissions", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "permissions requires a path argument")
		}
		path := interpreter.CoerceToString(args[0])
		info, err := GetFilePermissions(path)
		if err != nil {
			return handleIOError("permissions", path, err)
		}

		m := interpreter.NewMap()
		m.Set("octal", &interpreter.StringValue{Val: info["octal"].(string)})
		m.Set("mode", &interpreter.StringValue{Val: info["mode"].(string)})
		if info["is_dir"].(bool) {
			m.Set("is_dir", interpreter.TRUE)
		} else {
			m.Set("is_dir", interpreter.FALSE)
		}
		if info["is_readable"].(bool) {
			m.Set("is_readable", interpreter.TRUE)
		} else {
			m.Set("is_readable", interpreter.FALSE)
		}
		if info["is_writable"].(bool) {
			m.Set("is_writable", interpreter.TRUE)
		} else {
			m.Set("is_writable", interpreter.FALSE)
		}
		if info["is_executable"].(bool) {
			m.Set("is_executable", interpreter.TRUE)
		} else {
			m.Set("is_executable", interpreter.FALSE)
		}
		return m
	})

	// io.chmod(path, mode) -> bool
	register("chmod", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "chmod requires path and mode arguments (e.g. 0755 or '0755')")
		}
		path := interpreter.CoerceToString(args[0])
		var modeVal uint64

		switch v := args[1].(type) {
		case *interpreter.IntegerValue:
			modeVal = uint64(v.Val)
		case *interpreter.StringValue:
			parsed, err := strconv.ParseUint(v.Val, 8, 32)
			if err != nil {
				return interpreter.NewError(400, fmt.Sprintf("invalid octal mode string: %q", v.Val))
			}
			modeVal = parsed
		default:
			return interpreter.NewError(400, "mode must be an integer or octal string")
		}

		err := ChangeFilePermissions(path, os.FileMode(modeVal))
		if err != nil {
			return handleIOError("chmod", path, err)
		}
		return interpreter.TRUE
	})

	// io.lock_file(path_or_handle, [exclusive]) -> bool
	register("lock_file", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "lock_file requires path or handle argument")
		}
		exclusive := true
		if len(args) >= 2 {
			exclusive = interpreter.IsTruthy(args[1])
		}

		if m, ok := args[0].(*interpreter.MapValue); ok {
			if lockFn, hasLock := m.Get("lock"); hasLock {
				if bf, okBf := lockFn.(*interpreter.BuiltinFunction); okBf {
					exVal := interpreter.FALSE
					if exclusive {
						exVal = interpreter.TRUE
					}
					return bf.Fn(exVal)
				}
			}
		}

		path := interpreter.CoerceToString(args[0])
		pathLocksMu.Lock()
		defer pathLocksMu.Unlock()

		if _, exists := pathLocks[path]; exists {
			return interpreter.NewError(409, fmt.Sprintf("file %q is already locked by this process", path))
		}

		if err := sandbox.RequireWrite(path); err != nil {
			return handleIOError("lock_file", path, err)
		}
		f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			return handleIOError("lock_file", path, err)
		}

		if err := LockFileHandle(f, exclusive); err != nil {
			f.Close()
			return handleIOError("lock_file", path, err)
		}

		pathLocks[path] = f
		return interpreter.TRUE
	})

	// io.unlock_file(path_or_handle) -> bool
	register("unlock_file", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "unlock_file requires path or handle argument")
		}

		if m, ok := args[0].(*interpreter.MapValue); ok {
			if unlockFn, hasUnlock := m.Get("unlock"); hasUnlock {
				if bf, okBf := unlockFn.(*interpreter.BuiltinFunction); okBf {
					return bf.Fn()
				}
			}
		}

		path := interpreter.CoerceToString(args[0])
		pathLocksMu.Lock()
		defer pathLocksMu.Unlock()

		f, exists := pathLocks[path]
		if !exists {
			return interpreter.FALSE
		}

		_ = UnlockFileHandle(f)
		_ = f.Close()
		delete(pathLocks, path)
		return interpreter.TRUE
	})

	// io.is_permission_error(err_or_code) -> bool
	register("is_permission_error", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.FALSE
		}
		switch v := args[0].(type) {
		case *interpreter.ErrorValue:
			if v.Code == 403 || strings.Contains(strings.ToLower(v.Message), "access denied") || strings.Contains(strings.ToLower(v.Message), "permission denied") {
				return interpreter.TRUE
			}
		case *interpreter.IntegerValue:
			if v.Val == 403 {
				return interpreter.TRUE
			}
		case *interpreter.StringValue:
			s := strings.ToLower(v.Val)
			if strings.Contains(s, "access denied") || strings.Contains(s, "permission denied") {
				return interpreter.TRUE
			}
		}
		return interpreter.FALSE
	})
}
