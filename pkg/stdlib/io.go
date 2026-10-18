// Package stdlib provides the standard library modules and builtins for the Alca programming language.
package stdlib

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/alca-lang/alca/pkg/interpreter"
)

// ReadFile reads the entire contents of a file as a string.
func ReadFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// WriteFile writes string content to a file, replacing any existing contents.
func WriteFile(path, content string) error {
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, []byte(content), 0644)
}

// AppendFile appends string content to a file, creating it if it does not exist.
func AppendFile(path, content string) error {
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(content)
	return err
}

// FileExists checks whether a file or directory exists at the given path.
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil || !os.IsNotExist(err)
}

// DeleteFile removes the file or directory at the given path.
func DeleteFile(path string) error {
	return os.RemoveAll(path)
}

// ListDir lists all entry names within a directory.
func ListDir(path string) ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	return names, nil
}

// MakeDir creates a new directory and any necessary parent directories.
func MakeDir(path string) error {
	return os.MkdirAll(path, 0755)
}

// CopyFile copies a file from src to dst.
func CopyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destDir := filepath.Dir(dst)
	if destDir != "" && destDir != "." {
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return err
		}
	}

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

// MoveFile moves or renames a file from src to dst.
func MoveFile(src, dst string) error {
	destDir := filepath.Dir(dst)
	if destDir != "" && destDir != "." {
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return err
		}
	}
	err := os.Rename(src, dst)
	if err != nil {
		// Attempt copy and delete fallback across filesystems
		if copyErr := CopyFile(src, dst); copyErr == nil {
			return os.Remove(src)
		}
		return err
	}
	return nil
}

// FileSize returns the size of a file in bytes.
func FileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// FileInfo returns metadata about a file as a map.
func FileInfo(path string) map[string]interface{} {
	info, err := os.Stat(path)
	if err != nil {
		return map[string]interface{}{
			"exists": false,
			"error":  err.Error(),
		}
	}
	return map[string]interface{}{
		"exists":   true,
		"name":     info.Name(),
		"size":     info.Size(),
		"is_dir":   info.IsDir(),
		"mode":     info.Mode().String(),
		"mod_time": info.ModTime().UTC().Format(time.RFC3339),
	}
}

// ReadLines reads a text file and returns a slice of its lines.
func ReadLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

// Glob returns the names of all files matching pattern.
func Glob(pattern string) ([]string, error) {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	return matches, nil
}

// TempFile creates a temporary file with the given prefix and returns its path.
func TempFile(prefix string) (string, error) {
	f, err := os.CreateTemp("", prefix+"-*")
	if err != nil {
		return "", err
	}
	path := f.Name()
	f.Close()
	return path, nil
}

// TempDir creates a temporary directory with the given prefix and returns its path.
func TempDir(prefix string) (string, error) {
	dir, err := os.MkdirTemp("", prefix+"-*")
	if err != nil {
		return "", err
	}
	return dir, nil
}

// RegisterIOBuiltins registers all File I/O functions into the interpreter environment.
func RegisterIOBuiltins(env *interpreter.Environment) {
	ioModule := interpreter.NewMap()

	register := func(name string, fn func(args ...interpreter.Value) interpreter.Value) {
		builtin := &interpreter.BuiltinFunction{Name: name, Fn: fn}
		env.Set(name, builtin)
		ioModule.Set(name, builtin)
	}

	register("read_file", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "read_file requires a path argument")
		}
		path := interpreter.CoerceToString(args[0])
		content, err := ReadFile(path)
		if err != nil {
			return handleIOError("read_file", path, err)
		}
		return &interpreter.StringValue{Val: content}
	})

	register("write_file", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "write_file requires path and content arguments")
		}
		path := interpreter.CoerceToString(args[0])
		content := interpreter.CoerceToString(args[1])
		if err := WriteFile(path, content); err != nil {
			return handleIOError("write_file", path, err)
		}
		return interpreter.TRUE
	})

	register("append_file", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "append_file requires path and content arguments")
		}
		path := interpreter.CoerceToString(args[0])
		content := interpreter.CoerceToString(args[1])
		if err := AppendFile(path, content); err != nil {
			return handleIOError("append_file", path, err)
		}
		return interpreter.TRUE
	})

	register("file_exists", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.FALSE
		}
		path := interpreter.CoerceToString(args[0])
		if FileExists(path) {
			return interpreter.TRUE
		}
		return interpreter.FALSE
	})

	register("delete_file", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "delete_file requires a path argument")
		}
		path := interpreter.CoerceToString(args[0])
		if err := DeleteFile(path); err != nil {
			return handleIOError("delete_file", path, err)
		}
		return interpreter.TRUE
	})

	register("list_dir", func(args ...interpreter.Value) interpreter.Value {
		path := "."
		if len(args) >= 1 {
			path = interpreter.CoerceToString(args[0])
		}
		files, err := ListDir(path)
		if err != nil {
			return handleIOError("list_dir", path, err)
		}
		elems := make([]interpreter.Value, len(files))
		for i, f := range files {
			elems[i] = &interpreter.StringValue{Val: f}
		}
		return interpreter.NewList(elems...)
	})

	register("make_dir", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "make_dir requires a path argument")
		}
		path := interpreter.CoerceToString(args[0])
		if err := MakeDir(path); err != nil {
			return handleIOError("make_dir", path, err)
		}
		return interpreter.TRUE
	})

	register("copy_file", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "copy_file requires src and dst arguments")
		}
		src := interpreter.CoerceToString(args[0])
		dst := interpreter.CoerceToString(args[1])
		if err := CopyFile(src, dst); err != nil {
			return handleIOError("copy_file", src+" -> "+dst, err)
		}
		return interpreter.TRUE
	})

	register("move_file", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "move_file requires src and dst arguments")
		}
		src := interpreter.CoerceToString(args[0])
		dst := interpreter.CoerceToString(args[1])
		if err := MoveFile(src, dst); err != nil {
			return handleIOError("move_file", src+" -> "+dst, err)
		}
		return interpreter.TRUE
	})

	register("file_size", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "file_size requires a path argument")
		}
		path := interpreter.CoerceToString(args[0])
		size, err := FileSize(path)
		if err != nil {
			return handleIOError("file_size", path, err)
		}
		return &interpreter.IntegerValue{Val: size}
	})

	register("file_info", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "file_info requires a path argument")
		}
		path := interpreter.CoerceToString(args[0])
		info := FileInfo(path)
		m := interpreter.NewMap()
		for k, v := range info {
			switch val := v.(type) {
			case string:
				m.Set(k, &interpreter.StringValue{Val: val})
			case int64:
				m.Set(k, &interpreter.IntegerValue{Val: val})
			case bool:
				if val {
					m.Set(k, interpreter.TRUE)
				} else {
					m.Set(k, interpreter.FALSE)
				}
			default:
				m.Set(k, &interpreter.StringValue{Val: fmt.Sprintf("%v", val)})
			}
		}
		return m
	})

	register("read_lines", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "read_lines requires a path argument")
		}
		path := interpreter.CoerceToString(args[0])
		lines, err := ReadLines(path)
		if err != nil {
			return handleIOError("read_lines", path, err)
		}
		elems := make([]interpreter.Value, len(lines))
		for i, l := range lines {
			elems[i] = &interpreter.StringValue{Val: l}
		}
		return interpreter.NewList(elems...)
	})

	register("glob", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "glob requires a pattern argument")
		}
		pattern := interpreter.CoerceToString(args[0])
		matches, err := Glob(pattern)
		if err != nil {
			return interpreter.NewError(500, err.Error())
		}
		elems := make([]interpreter.Value, len(matches))
		for i, m := range matches {
			elems[i] = &interpreter.StringValue{Val: m}
		}
		return interpreter.NewList(elems...)
	})

	register("temp_file", func(args ...interpreter.Value) interpreter.Value {
		prefix := "alca"
		if len(args) >= 1 {
			prefix = interpreter.CoerceToString(args[0])
		}
		path, err := TempFile(prefix)
		if err != nil {
			return interpreter.NewError(500, err.Error())
		}
		return &interpreter.StringValue{Val: path}
	})

	register("temp_dir", func(args ...interpreter.Value) interpreter.Value {
		prefix := "alca"
		if len(args) >= 1 {
			prefix = interpreter.CoerceToString(args[0])
		}
		dir, err := TempDir(prefix)
		if err != nil {
			return interpreter.NewError(500, err.Error())
		}
		return &interpreter.StringValue{Val: dir}
	})

	// Register stream handles, seeking, locking, permissions, and security
	RegisterAdvancedIOBuiltins(env, ioModule, register)

	// Also bind standard module namespaces
	env.Set("io", ioModule)
	env.Set("std.io", ioModule)
}
