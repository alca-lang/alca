// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package stdlib

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// The prefix under which the native libraries of a program are stored in the
// resources of the bundle.
const embeddedNativePrefix = "native/"

var (
	embeddedLibraryMu    sync.Mutex
	embeddedLibraryPaths = map[string]string{}
)

// NativeLibraryExtension returns the file extension a shared library has on
// the current platform.
func NativeLibraryExtension() string {
	switch runtime.GOOS {
	case "windows":
		return ".dll"
	case "darwin":
		return ".dylib"
	default:
		return ".so"
	}
}

// EmbeddedNativeLibraryName returns the name a native library is stored under
// in the resources of a bundle. A source file is stored under the name of the
// library it builds, with the extension of the platform.
func EmbeddedNativeLibraryName(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "" {
		return ""
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".c", ".s", ".asm":
		name = strings.TrimSuffix(name, filepath.Ext(name)) + NativeLibraryExtension()
	case "":
		name += NativeLibraryExtension()
	}
	return embeddedNativePrefix + name
}

// ExtractEmbeddedLibrary unpacks a native library from the resources of the
// program and returns the path of the file. The library is unpacked once and
// stays in the temporary directory of the process.
func ExtractEmbeddedLibrary(name string) (string, error) {
	resource := EmbeddedNativeLibraryName(name)
	data, err := ReadEmbeddedResource(resource)
	if err != nil {
		return "", fmt.Errorf("the native library %q is not part of the program: %w", name, err)
	}

	embeddedLibraryMu.Lock()
	defer embeddedLibraryMu.Unlock()
	if path, ok := embeddedLibraryPaths[resource]; ok {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	dir, err := os.MkdirTemp("", "alca-native-")
	if err != nil {
		return "", fmt.Errorf("cannot create the directory of the native library: %w", err)
	}
	path := filepath.Join(dir, filepath.Base(resource))
	if err := os.WriteFile(path, data, 0o755); err != nil {
		return "", fmt.Errorf("cannot write the native library: %w", err)
	}
	embeddedLibraryPaths[resource] = path
	return path, nil
}
