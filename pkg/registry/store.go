// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

// Package registry implements the server side of an Alca package registry
// node: durable on-disk storage, metadata indexing, and the helpers the HTTP
// API builds on. Storage is plain files, so a node is easy to back up, inspect
// and serve from any filesystem.
package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/alca-lang/alca/pkg/apm"
)

// Storage errors shared with the HTTP layer.
var (
	// ErrNotFound means a package, version or artifact does not exist.
	ErrNotFound = errors.New("not found")
	// ErrVersionExists means the version is already published with different content.
	ErrVersionExists = errors.New("version already published")
	// ErrArtifactExists means the artifact is already published with different content.
	ErrArtifactExists = errors.New("artifact already published")
)

// Store is a filesystem-backed package store.
type Store struct {
	root string
	mu   sync.RWMutex
	idx  map[string][]apm.VersionInfo // package name -> versions, cached
}

// NewStore opens (creating if needed) a store rooted at dir.
func NewStore(dir string) (*Store, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	s := &Store{root: abs}
	for _, d := range []string{s.packagesDir(), s.tmpDir()} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, fmt.Errorf("cannot create registry storage %s: %w", d, err)
		}
	}
	if err := s.reload(); err != nil {
		return nil, err
	}
	return s, nil
}

// Root returns the storage root directory.
func (s *Store) Root() string { return s.root }

func (s *Store) packagesDir() string { return filepath.Join(s.root, "packages") }
func (s *Store) tmpDir() string      { return filepath.Join(s.root, "tmp") }

func (s *Store) versionDir(name, version string) string {
	return filepath.Join(s.packagesDir(), name, version)
}

func (s *Store) archivePath(name, version string) string {
	return filepath.Join(s.versionDir(name, version), "package.tar.gz")
}

func (s *Store) metaPath(name, version string) string {
	return filepath.Join(s.versionDir(name, version), "meta.json")
}

func (s *Store) artifactPath(name, version, goos, goarch string, bin bool) string {
	ext := ".bin"
	if !bin {
		ext = ".json"
	}
	return filepath.Join(s.versionDir(name, version), "artifacts", goos+"-"+goarch+ext)
}

// reload rebuilds the in-memory index by scanning meta.json files.
func (s *Store) reload() error {
	idx := map[string][]apm.VersionInfo{}
	pkgsDir := s.packagesDir()
	entries, err := os.ReadDir(pkgsDir)
	if err != nil {
		if os.IsNotExist(err) {
			s.mu.Lock()
			s.idx = idx
			s.mu.Unlock()
			return nil
		}
		return err
	}
	for _, pkgEntry := range entries {
		if !pkgEntry.IsDir() || !apm.ValidName(pkgEntry.Name()) {
			continue
		}
		verEntries, err := os.ReadDir(filepath.Join(pkgsDir, pkgEntry.Name()))
		if err != nil {
			continue
		}
		for _, verEntry := range verEntries {
			if !verEntry.IsDir() {
				continue
			}
			data, err := os.ReadFile(s.metaPath(pkgEntry.Name(), verEntry.Name()))
			if err != nil {
				continue
			}
			var info apm.VersionInfo
			if err := json.Unmarshal(data, &info); err != nil {
				continue
			}
			if info.Name == "" {
				info.Name = pkgEntry.Name()
			}
			if info.Version == "" {
				info.Version = verEntry.Name()
			}
			idx[info.Name] = append(idx[info.Name], info)
		}
	}
	for name := range idx {
		sortVersions(idx[name])
	}
	s.mu.Lock()
	s.idx = idx
	s.mu.Unlock()
	return nil
}

func sortVersions(vs []apm.VersionInfo) {
	sort.Slice(vs, func(i, j int) bool {
		vi, erri := apm.ParseVersion(vs[i].Version)
		vj, errj := apm.ParseVersion(vs[j].Version)
		if erri != nil || errj != nil {
			return vs[i].Version < vs[j].Version
		}
		return vi.Less(vj)
	})
}

// List returns a summary of every package, optionally filtered by a substring
// query over name and description.
func (s *Store) List(query string, limit, offset int) ([]apm.PackageSummary, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	q := strings.ToLower(strings.TrimSpace(query))
	var out []apm.PackageSummary
	for name, versions := range s.idx {
		if len(versions) == 0 {
			continue
		}
		latest := versions[len(versions)-1]
		if q != "" && !strings.Contains(strings.ToLower(name), q) && !strings.Contains(strings.ToLower(latest.Description), q) {
			continue
		}
		out = append(out, apm.PackageSummary{
			Name:        name,
			Description: latest.Description,
			Latest:      latest.Version,
			Versions:    len(versions),
			Publisher:   latest.Publisher,
			Yanked:      latest.Yanked,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	total := len(out)
	if offset > 0 {
		if offset >= len(out) {
			out = nil
		} else {
			out = out[offset:]
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, total, nil
}

// Versions returns every published version of a package, oldest first.
func (s *Store) Versions(name string) ([]apm.VersionInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	versions, ok := s.idx[name]
	if !ok || len(versions) == 0 {
		return nil, ErrNotFound
	}
	return append([]apm.VersionInfo(nil), versions...), nil
}

// Version returns one published version.
func (s *Store) Version(name, version string) (*apm.VersionInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := range s.idx[name] {
		if s.idx[name][i].Version == version {
			out := s.idx[name][i]
			return &out, nil
		}
	}
	return nil, ErrNotFound
}

// ArtifactUpload holds binary and metadata for one artifact during atomic publish.
type ArtifactUpload struct {
	GOOS   string           `json:"goos"`
	GOARCH string           `json:"goarch"`
	Meta   apm.ArtifactMeta `json:"meta"`
	Bin    []byte           `json:"-"`
}

// SavePackage stores a validated package archive and updates the index atomically.
func (s *Store) SavePackage(info apm.VersionInfo, archive []byte) (alreadyStored bool, err error) {
	return s.SavePackageWithArtifacts(info, archive, nil)
}

// SavePackageWithArtifacts stores a package archive AND all its compiled build artifacts
// in a single atomic transaction.
func (s *Store) SavePackageWithArtifacts(info apm.VersionInfo, archive []byte, artifacts []ArtifactUpload) (alreadyStored bool, err error) {
	if !apm.ValidName(info.Name) {
		return false, fmt.Errorf("invalid package name %q", info.Name)
	}
	if _, err := apm.ParseVersion(info.Version); err != nil {
		return false, fmt.Errorf("invalid package version %q", info.Version)
	}
	sum := sha256HexBytes(archive)
	if existing, err := s.Version(info.Name, info.Version); err == nil {
		if existing.SHA256 == sum {
			return true, nil
		}
		return false, fmt.Errorf("%w: %s %s", ErrVersionExists, info.Name, info.Version)
	}
	info.SHA256 = sum
	info.Size = int64(len(archive))
	info.Artifacts = nil

	dir := s.versionDir(info.Name, info.Version)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, err
	}
	if err := atomicWrite(s.archivePath(info.Name, info.Version), archive, 0o644); err != nil {
		return false, err
	}

	for _, a := range artifacts {
		binSum := sha256HexBytes(a.Bin)
		artDir := filepath.Dir(s.artifactPath(info.Name, info.Version, a.GOOS, a.GOARCH, true))
		if err := os.MkdirAll(artDir, 0o755); err != nil {
			return false, err
		}
		if err := atomicWrite(s.artifactPath(info.Name, info.Version, a.GOOS, a.GOARCH, true), a.Bin, 0o644); err != nil {
			return false, err
		}
		metaData, err := json.MarshalIndent(a.Meta, "", "  ")
		if err != nil {
			return false, err
		}
		if err := atomicWrite(s.artifactPath(info.Name, info.Version, a.GOOS, a.GOARCH, false), metaData, 0o644); err != nil {
			return false, err
		}
		info.Artifacts = append(info.Artifacts, apm.ArtifactInfo{
			GOOS:           a.GOOS,
			GOARCH:         a.GOARCH,
			AlcaVersion:    a.Meta.AlcaVersion,
			PayloadVersion: a.Meta.PayloadVersion,
			SHA256:         binSum,
			Size:           int64(len(a.Bin)),
			Entry:          a.Meta.Entry,
			SourceHash:     a.Meta.SourceHash,
			Exports:        a.Meta.Exports,
			CreatedAt:      a.Meta.CreatedAt,
		})
	}

	meta, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return false, err
	}
	if err := atomicWrite(s.metaPath(info.Name, info.Version), meta, 0o644); err != nil {
		return false, err
	}
	// Update the cached index last so a concurrent reader never sees a version
	// without its archive.
	s.mu.Lock()
	versions := s.idx[info.Name]
	replaced := false
	for i := range versions {
		if versions[i].Version == info.Version {
			versions[i] = info
			replaced = true
			break
		}
	}
	if !replaced {
		versions = append(versions, info)
	}
	sortVersions(versions)
	s.idx[info.Name] = versions
	s.mu.Unlock()
	return false, nil
}

// YankVersion toggles the Yanked status of a specific package version.
func (s *Store) YankVersion(name, version string, yanked bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	versions, ok := s.idx[name]
	if !ok {
		return ErrNotFound
	}
	found := false
	for i := range versions {
		if versions[i].Version == version {
			versions[i].Yanked = yanked
			updated := versions[i]
			metaData, err := json.MarshalIndent(updated, "", "  ")
			if err != nil {
				return err
			}
			if err := atomicWrite(s.metaPath(name, version), metaData, 0o644); err != nil {
				return err
			}
			found = true
			break
		}
	}
	if !found {
		return ErrNotFound
	}
	return nil
}

// DeleteVersion deletes a specific version from storage and from the index.
func (s *Store) DeleteVersion(name, version string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	versions, ok := s.idx[name]
	if !ok {
		return ErrNotFound
	}
	var remaining []apm.VersionInfo
	found := false
	for _, v := range versions {
		if v.Version == version {
			found = true
			_ = os.RemoveAll(s.versionDir(name, version))
		} else {
			remaining = append(remaining, v)
		}
	}
	if !found {
		return ErrNotFound
	}
	if len(remaining) == 0 {
		delete(s.idx, name)
		_ = os.RemoveAll(filepath.Join(s.packagesDir(), name))
	} else {
		s.idx[name] = remaining
	}
	return nil
}

// DeletePackage completely deletes a package and all its versions from storage.
func (s *Store) DeletePackage(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.idx[name]; !ok {
		return ErrNotFound
	}
	_ = os.RemoveAll(filepath.Join(s.packagesDir(), name))
	delete(s.idx, name)
	return nil
}

// SaveArtifact stores a compiled build for a published version. Uploading the
// same build twice is idempotent.
func (s *Store) SaveArtifact(name, version, goos, goarch string, bin []byte, meta apm.ArtifactMeta) (alreadyStored bool, err error) {
	info, err := s.Version(name, version)
	if err != nil {
		return false, ErrNotFound
	}
	sum := sha256HexBytes(bin)
	for _, a := range info.Artifacts {
		if a.GOOS == goos && a.GOARCH == goarch {
			if a.SHA256 == sum {
				return true, nil
			}
			return false, fmt.Errorf("%w: %s %s %s/%s", ErrArtifactExists, name, version, goos, goarch)
		}
	}
	if err := os.MkdirAll(filepath.Dir(s.artifactPath(name, version, goos, goarch, true)), 0o755); err != nil {
		return false, err
	}
	if err := atomicWrite(s.artifactPath(name, version, goos, goarch, true), bin, 0o644); err != nil {
		return false, err
	}
	metaData, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return false, err
	}
	if err := atomicWrite(s.artifactPath(name, version, goos, goarch, false), metaData, 0o644); err != nil {
		return false, err
	}

	ref := apm.ArtifactInfo{
		GOOS:           goos,
		GOARCH:         goarch,
		AlcaVersion:    meta.AlcaVersion,
		PayloadVersion: meta.PayloadVersion,
		SHA256:         sum,
		Size:           int64(len(bin)),
		Entry:          meta.Entry,
		SourceHash:     meta.SourceHash,
		Exports:        meta.Exports,
		CreatedAt:      meta.CreatedAt,
	}
	s.mu.Lock()
	versions := s.idx[name]
	for i := range versions {
		if versions[i].Version != version {
			continue
		}
		versions[i].Artifacts = append(versions[i].Artifacts, ref)
		updated := versions[i]
		s.idx[name] = versions
		metaData, err := json.MarshalIndent(updated, "", "  ")
		if err == nil {
			_ = atomicWrite(s.metaPath(name, version), metaData, 0o644)
		}
		break
	}
	s.mu.Unlock()
	return false, nil
}

// PackageArchive returns the stored archive and metadata of a version.
func (s *Store) PackageArchive(name, version string) ([]byte, *apm.VersionInfo, error) {
	info, err := s.Version(name, version)
	if err != nil {
		return nil, nil, err
	}
	data, err := os.ReadFile(s.archivePath(name, version))
	if err != nil {
		return nil, nil, fmt.Errorf("archive for %s %s is missing: %w", name, version, err)
	}
	return data, info, nil
}

// ArtifactBin returns a stored compiled build and its metadata.
func (s *Store) ArtifactBin(name, version, goos, goarch string) ([]byte, *apm.ArtifactMeta, error) {
	if _, err := s.Version(name, version); err != nil {
		return nil, nil, err
	}
	data, err := os.ReadFile(s.artifactPath(name, version, goos, goarch, true))
	if err != nil {
		return nil, nil, ErrNotFound
	}
	metaData, err := os.ReadFile(s.artifactPath(name, version, goos, goarch, false))
	if err != nil {
		return nil, nil, ErrNotFound
	}
	var meta apm.ArtifactMeta
	if err := json.Unmarshal(metaData, &meta); err != nil {
		return nil, nil, fmt.Errorf("artifact metadata is corrupt: %w", err)
	}
	return data, &meta, nil
}

// TempDir creates a temporary directory inside the store for request staging.
func (s *Store) TempDir(pattern string) (string, error) {
	return os.MkdirTemp(s.tmpDir(), pattern)
}
