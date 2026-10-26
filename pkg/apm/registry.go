// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package apm

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Environment variables understood by APM.
const (
	// EnvHome overrides the Alca home directory (~/.alca).
	EnvHome = "ALCA_HOME"
	// EnvRegistry sets the default remote registry node URL.
	EnvRegistry = "ALCA_REGISTRY"
	// EnvToken supplies the publish token for `apm publish`.
	EnvToken = "APM_TOKEN"
	// EnvPassword supplies the account password for `apm register`/`apm login`
	// when it is not entered interactively.
	EnvPassword = "APM_PASSWORD"
	// EnvEnablePublish gates the (not yet enabled by default) publish command.
	EnvEnablePublish = "APM_ENABLE_PUBLISH"
)

// IndexFormatVersion identifies the on-disk local registry index layout.
const IndexFormatVersion = 1

// Home returns the Alca home directory: $ALCA_HOME or ~/.alca.
func Home() (string, error) {
	if h := strings.TrimSpace(os.Getenv(EnvHome)); h != "" {
		return filepath.Abs(h)
	}
	userHome, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot locate the user home directory (set %s): %w", EnvHome, err)
	}
	return filepath.Join(userHome, ".alca"), nil
}

// Registry is the local system registry of installed packages.
type Registry struct {
	home string
	dir  string // home/registry
}

// NewRegistry opens (creating if needed) the registry below home.
func NewRegistry(home string) (*Registry, error) {
	if home == "" {
		h, err := Home()
		if err != nil {
			return nil, err
		}
		home = h
	}
	abs, err := filepath.Abs(home)
	if err != nil {
		return nil, err
	}
	r := &Registry{home: abs, dir: filepath.Join(abs, "registry")}
	for _, d := range []string{r.dir, r.PackagesDir(), r.CacheDir(), r.TempDir()} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, fmt.Errorf("cannot create registry directory %s: %w", d, err)
		}
	}
	return r, nil
}

// Home returns the Alca home directory of this registry.
func (r *Registry) Home() string { return r.home }

// Dir returns the registry directory (home/registry).
func (r *Registry) Dir() string { return r.dir }

// PackagesDir returns the directory that holds unpacked packages.
func (r *Registry) PackagesDir() string { return filepath.Join(r.dir, "packages") }

// CacheDir returns the download cache directory.
func (r *Registry) CacheDir() string { return filepath.Join(r.dir, "cache") }

// TempDir returns the staging directory for atomic installs.
func (r *Registry) TempDir() string { return filepath.Join(r.dir, "tmp") }

// IndexFile returns the index.json path.
func (r *Registry) IndexFile() string { return filepath.Join(r.dir, "index.json") }

// PackageDir returns the install directory of a package version.
func (r *Registry) PackageDir(name, version string) string {
	return filepath.Join(r.PackagesDir(), name, version)
}

// ---------------------------------------------------------------------------
// Index
// ---------------------------------------------------------------------------

// Index is the local registry's package index.
type Index struct {
	FormatVersion int                      `json:"format_version"`
	UpdatedAt     string                   `json:"updated_at,omitempty"`
	Packages      map[string]*PackageEntry `json:"packages"`
}

// PackageEntry lists the installed versions of one package.
type PackageEntry struct {
	Name     string                   `json:"name"`
	Versions map[string]*VersionEntry `json:"versions"`
}

// VersionEntry describes one installed package version.
type VersionEntry struct {
	Name         string                    `json:"name,omitempty"`
	Version      string                    `json:"version"`
	Description  string                    `json:"description,omitempty"`
	Entry        string                    `json:"entry"`
	Alca         string                    `json:"alca,omitempty"`
	License      string                    `json:"license,omitempty"`
	Dependencies map[string]string         `json:"dependencies,omitempty"`
	Path         string                    `json:"path"` // relative to the registry directory
	SourceHash   string                    `json:"source_hash,omitempty"`
	InstalledAt  string                    `json:"installed_at,omitempty"`
	Origin       string                    `json:"origin,omitempty"` // "local:<path>" or "remote:<url>"
	Artifacts    map[string]*ArtifactEntry `json:"artifacts,omitempty"`
}

// ArtifactEntry records a compiled artifact shipped with an installed package.
type ArtifactEntry struct {
	File           string `json:"file"`      // relative to the package directory
	MetaFile       string `json:"meta_file"` // relative to the package directory
	SHA256         string `json:"sha256"`
	AlcaVersion    string `json:"alca_version"`
	PayloadVersion uint32 `json:"payload_version"`
	GOOS           string `json:"goos"`
	GOARCH         string `json:"goarch"`
}

// NewIndex returns an empty index.
func NewIndex() *Index {
	return &Index{FormatVersion: IndexFormatVersion, Packages: map[string]*PackageEntry{}}
}

// LoadIndex reads the local index, returning an empty one when missing.
func (r *Registry) LoadIndex() (*Index, error) {
	data, err := os.ReadFile(r.IndexFile())
	if err != nil {
		if os.IsNotExist(err) {
			return NewIndex(), nil
		}
		return nil, fmt.Errorf("cannot read registry index: %w", err)
	}
	idx := NewIndex()
	if err := json.Unmarshal(data, idx); err != nil {
		return nil, fmt.Errorf("registry index %s is corrupt: %w", r.IndexFile(), err)
	}
	if idx.FormatVersion > IndexFormatVersion {
		return nil, fmt.Errorf("registry index format %d is newer than this apm supports (%d); upgrade alca", idx.FormatVersion, IndexFormatVersion)
	}
	if idx.Packages == nil {
		idx.Packages = map[string]*PackageEntry{}
	}
	return idx, nil
}

// SaveIndex writes the index atomically.
func (r *Registry) SaveIndex(idx *Index) error {
	idx.FormatVersion = IndexFormatVersion
	idx.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteFile(r.IndexFile(), data, 0o644)
}

// Installed returns the highest installed version of name.
func (r *Registry) Installed(name string) (*VersionEntry, bool, error) {
	idx, err := r.LoadIndex()
	if err != nil {
		return nil, false, err
	}
	return idx.Installed(name)
}

// Installed returns the highest installed version of name in the index.
func (idx *Index) Installed(name string) (*VersionEntry, bool, error) {
	entry, ok := idx.Packages[name]
	if !ok || len(entry.Versions) == 0 {
		return nil, false, nil
	}
	var best *VersionEntry
	var bestV Version
	for _, ve := range entry.Versions {
		v, err := ParseVersion(ve.Version)
		if err != nil {
			continue
		}
		if best == nil || bestV.Less(v) {
			best, bestV = ve, v
		}
	}
	if best == nil {
		return nil, false, nil
	}
	return best, true, nil
}

// Version returns the specific installed version of name from the index, or false.
func (idx *Index) Version(name, version string) (*VersionEntry, bool) {
	entry, ok := idx.Packages[name]
	if !ok || entry.Versions == nil {
		return nil, false
	}
	ve, ok := entry.Versions[version]
	return ve, ok
}

// SetVersion inserts or replaces a version entry in the index.
func (idx *Index) SetVersion(name string, ve *VersionEntry) {
	entry, ok := idx.Packages[name]
	if !ok {
		entry = &PackageEntry{Name: name, Versions: map[string]*VersionEntry{}}
		idx.Packages[name] = entry
	}
	entry.Versions[ve.Version] = ve
}

// RemoveVersion removes one version from the index. The package entry is dropped
// when no versions remain.
func (idx *Index) RemoveVersion(name, version string) {
	entry, ok := idx.Packages[name]
	if !ok {
		return
	}
	delete(entry.Versions, version)
	if len(entry.Versions) == 0 {
		delete(idx.Packages, name)
	}
}

// Dependents returns installed packages that depend on name.
func (idx *Index) Dependents(name string) []string {
	var out []string
	for pkgName, entry := range idx.Packages {
		if pkgName == name {
			continue
		}
		for _, ve := range entry.Versions {
			if _, ok := ve.Dependencies[name]; ok {
				out = append(out, pkgName)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}

// ---------------------------------------------------------------------------
// Filesystem helpers
// ---------------------------------------------------------------------------

// atomicWriteFile writes data to path via a temp file in the same directory and
// a rename, so readers never observe a partial file.
func atomicWriteFile(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, mode); err != nil && !os.IsNotExist(err) {
		// Best effort on platforms without POSIX permissions.
		_ = err
	}
	return os.Rename(tmpName, path)
}

// sha256Hex returns the hex sha256 of data.
func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// fileExists reports whether path exists and is a regular file.
func fileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Mode().IsRegular()
}

// dirExists reports whether path exists and is a directory.
func dirExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

// cleanRel returns a slash-separated relative path.
func cleanRel(p string) string { return filepath.ToSlash(filepath.Clean(p)) }
