// Package apm implements the Alca package manager.
package apm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// LockedPackage represents a locked dependency in apm.lock.
type LockedPackage struct {
	Version      string            `json:"version"`
	SHA256       string            `json:"sha256,omitempty"`
	Dependencies map[string]string `json:"dependencies,omitempty"`
}

// Lockfile represents the contents of apm.lock.
type Lockfile struct {
	Version  int                      `json:"version"`
	Packages map[string]LockedPackage `json:"packages"`
}

// NewLockfile creates a new empty Lockfile.
func NewLockfile() *Lockfile {
	return &Lockfile{
		Version:  1,
		Packages: make(map[string]LockedPackage),
	}
}

// FindLockfilePath searches for an existing apm.lock or alca.toml in startDir or its parent directories.
func FindLockfilePath(startDir string) (lockPath string, inProject bool) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", false
	}
	for {
		candidate := filepath.Join(dir, "apm.lock")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, true
		}
		tomlCandidate := filepath.Join(dir, "alca.toml")
		if _, err := os.Stat(tomlCandidate); err == nil {
			return filepath.Join(dir, "apm.lock"), true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", false
}

// ReadLockfile loads and unmarshals an apm.lock file.
func ReadLockfile(path string) (*Lockfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var lock Lockfile
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, fmt.Errorf("corrupt apm.lock: %w", err)
	}
	if lock.Packages == nil {
		lock.Packages = make(map[string]LockedPackage)
	}
	return &lock, nil
}

// Write writes the lockfile to path formatted as indented JSON.
func (l *Lockfile) Write(path string) error {
	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

// UpdateFromInstalled merges installed packages into the lockfile.
func (l *Lockfile) UpdateFromInstalled(pkgs []InstalledPackage) {
	if l.Packages == nil {
		l.Packages = make(map[string]LockedPackage)
	}
	for _, p := range pkgs {
		lp := LockedPackage{
			Version:      p.Version,
			Dependencies: p.Dependencies,
		}
		if existing, ok := l.Packages[p.Name]; ok && existing.SHA256 != "" {
			lp.SHA256 = existing.SHA256
		}
		l.Packages[p.Name] = lp
	}
}
