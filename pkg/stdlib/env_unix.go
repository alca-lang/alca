// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package stdlib

import (
	"os"
	"path/filepath"
	"strings"
)

// GetUserPath returns persistent user PATH entries found in ~/.profile.
func GetUserPath() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return []string{}, nil
	}
	profilePath := filepath.Join(home, ".profile")
	data, err := os.ReadFile(profilePath)
	if err != nil {
		return []string{}, nil
	}
	lines := strings.Split(string(data), "\n")
	var paths []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "export PATH=") {
			val := strings.TrimPrefix(l, "export PATH=")
			val = strings.Trim(val, `"'`)
			parts := strings.Split(val, ":")
			for _, p := range parts {
				if p != "$PATH" && p != "${PATH}" && p != "" {
					paths = append(paths, p)
				}
			}
		}
	}
	return paths, nil
}

// GetSystemPath returns standard Unix system PATH directories.
func GetSystemPath() ([]string, error) {
	return []string{"/usr/local/sbin", "/usr/local/bin", "/usr/sbin", "/usr/bin", "/sbin", "/bin"}, nil
}

// SetUserPath updates persistent user PATH in ~/.profile.
func SetUserPath(dirs []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	profilePath := filepath.Join(home, ".profile")
	content := ""
	if data, err := os.ReadFile(profilePath); err == nil {
		lines := strings.Split(string(data), "\n")
		var kept []string
		for _, l := range lines {
			if !strings.HasPrefix(strings.TrimSpace(l), "export PATH=") {
				kept = append(kept, l)
			}
		}
		content = strings.Join(kept, "\n")
	}
	if len(dirs) > 0 {
		exportLine := `export PATH="` + strings.Join(dirs, ":") + `:$PATH"`
		if content != "" && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += exportLine + "\n"
	}
	return os.WriteFile(profilePath, []byte(content), 0644)
}

// SetSystemPath is a stub on Unix.
func SetSystemPath(dirs []string) error {
	return nil
}

// BroadcastEnvChange is a no-op on non-Windows platforms.
func BroadcastEnvChange() {
	// Not applicable on Unix
}
