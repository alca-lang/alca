// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package apm

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/alca-lang/alca/pkg/lexer"
	"github.com/alca-lang/alca/pkg/parser"
	"github.com/alca-lang/alca/pkg/typecheck"
)

// ManifestName is the manifest file of an Alca library.
const ManifestName = "alca.toml"

// Archive limits protect installers and registry nodes from hostile packages.
const (
	// MaxPackageBytes is the maximum size of an unpacked package.
	MaxPackageBytes = 128 << 20
	// MaxFileBytes is the maximum size of one file inside a package.
	MaxFileBytes = 32 << 20
	// MaxPackageFiles is the maximum number of entries inside a package.
	MaxPackageFiles = 2000
)

// ReadManifest reads and validates the alca.toml of a package directory.
func ReadManifest(dir string) (*Manifest, error) {
	data, err := os.ReadFile(filepath.Join(dir, ManifestName))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no %s found in %s: not an Alca library", ManifestName, dir)
		}
		return nil, fmt.Errorf("cannot read %s: %w", ManifestName, err)
	}
	m, err := ParseManifest(data)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// ValidatePackageDir checks the standard library structure: a valid manifest,
// an existing entry file, parseable .al sources and a type-checkable entry.
func ValidatePackageDir(dir string) (*Manifest, error) {
	// Type-checking resolves `use` statements, so packages installed in the
	// local registry must be visible here even when the caller (a test, the CLI)
	// has not wired the engine hooks yet. Enable is idempotent.
	Enable()
	m, err := ReadManifest(dir)
	if err != nil {
		return nil, err
	}
	entryRel := cleanRel(m.Package.Entry)
	entry := filepath.Join(dir, filepath.FromSlash(entryRel))
	if !fileExists(entry) {
		return nil, fmt.Errorf("entry %q (from alca.toml) does not exist in %s", m.Package.Entry, dir)
	}
	if err := parseAllSources(dir); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(entry)
	if err != nil {
		return nil, err
	}
	src := string(data)
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	// parseAllSources already checked syntax, but keep a defensive guard.
	if len(p.SyntaxErrors()) > 0 {
		return nil, fmt.Errorf("entry %s has syntax errors: %s", entryRel, p.SyntaxErrors()[0].Message)
	}
	checker := typecheck.NewChecker(entry, src)
	if errs := checker.Check(prog); len(errs) > 0 {
		return nil, fmt.Errorf("entry %s has %d type error(s): %s", entryRel, len(errs), errs[0].Message)
	}
	if ok, err := AlcaCompatible(m.Package.Alca); err != nil {
		return nil, err
	} else if !ok {
		return nil, fmt.Errorf("package %s %s requires alca %q, running alca %s",
			m.Package.Name, m.Package.Version, m.Package.Alca, currentVersion())
	}
	return m, nil
}

// parseAllSources parses every .al file of the package, reporting the first
// syntax error with its file name.
func parseAllSources(dir string) error {
	return walkPackageFiles(dir, func(rel, abs string) error {
		if !strings.HasSuffix(rel, ".al") {
			return nil
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			return err
		}
		p := parser.New(lexer.New(string(data)))
		p.ParseProgram()
		if errs := p.SyntaxErrors(); len(errs) > 0 {
			return fmt.Errorf("%s has syntax errors: %s (%s:%d)", rel, errs[0].Message, rel, errs[0].Line)
		}
		return nil
	})
}

// walkPackageFiles visits every file of a package in deterministic order,
// skipping build/cache directories and version control metadata.
func walkPackageFiles(dir string, fn func(rel, abs string) error) error {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if path != dir && (name == ".alca" || name == ".git" || name == "node_modules" || strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if name == ".DS_Store" || name == "Thumbs.db" || name == ManifestName+".tmp" {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, abs := range files {
		rel, err := filepath.Rel(dir, abs)
		if err != nil {
			continue
		}
		if err := fn(filepath.ToSlash(rel), abs); err != nil {
			return err
		}
	}
	return nil
}

// CopyTree copies the sources of a package directory into dst, skipping
// generated artifacts and version control metadata.
func CopyTree(src, dst string) error {
	return walkPackageFiles(src, func(rel, abs string) error {
		target := filepath.Join(dst, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

// PackDir builds the canonical package archive (gzipped tar, flat layout) of a
// validated package directory: alca.toml, sources, docs and tests. Generated
// artifacts (.alca), VCS data and caches are excluded; compiled builds travel
// through the registry's artifact endpoints instead.
func PackDir(dir string) ([]byte, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	count := 0
	var total int64
	err := walkPackageFiles(dir, func(rel, abs string) error {
		count++
		if count > MaxPackageFiles {
			return fmt.Errorf("package contains more than %d files", MaxPackageFiles)
		}
		fi, err := os.Stat(abs)
		if err != nil {
			return err
		}
		if fi.Size() > MaxFileBytes {
			return fmt.Errorf("file %s is larger than %d bytes", rel, MaxFileBytes)
		}
		total += fi.Size()
		if total > MaxPackageBytes {
			return fmt.Errorf("package is larger than %d bytes", MaxPackageBytes)
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			return err
		}
		hdr := &tar.Header{
			Name:    rel,
			Mode:    0o644,
			Size:    int64(len(data)),
			ModTime: fi.ModTime(),
			Format:  tar.FormatPAX,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		_, err = tw.Write(data)
		return err
	})
	if err != nil {
		tw.Close()
		zw.Close()
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// UnpackArchive extracts a package tarball into dir. It refuses absolute paths,
// parent escapes, links and oversized members, so a hostile archive cannot
// write outside dir.
func UnpackArchive(data []byte, dir string) error {
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("package archive is not valid gzip: %w", err)
	}
	defer zr.Close()
	tr := tar.NewReader(zr)
	count := 0
	var total int64
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("package archive is corrupt: %w", err)
		}
		count++
		if count > MaxPackageFiles {
			return fmt.Errorf("package archive contains more than %d files", MaxPackageFiles)
		}
		name := filepath.ToSlash(filepath.Clean(hdr.Name))
		if name == "." || strings.HasPrefix(name, "/") || strings.Contains(name, "..") || strings.Contains(name, ":") {
			return fmt.Errorf("package archive contains an unsafe path %q", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			dirTarget := filepath.Join(dir, filepath.FromSlash(name))
			rel, err := filepath.Rel(dir, dirTarget)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
				return fmt.Errorf("package archive directory escapes target directory: %q", hdr.Name)
			}
			if err := os.MkdirAll(dirTarget, 0o755); err != nil {
				return err
			}
			continue
		case tar.TypeReg:
		default:
			return fmt.Errorf("package archive contains unsupported entry %q (only regular files and directories are allowed)", hdr.Name)
		}
		if hdr.Size < 0 || hdr.Size > MaxFileBytes {
			return fmt.Errorf("package archive member %q is too large", hdr.Name)
		}
		total += hdr.Size
		if total > MaxPackageBytes {
			return fmt.Errorf("package archive expands to more than %d bytes", MaxPackageBytes)
		}
		target := filepath.Join(dir, filepath.FromSlash(name))
		rel, err := filepath.Rel(dir, target)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			return fmt.Errorf("package archive member escapes target directory: %q", hdr.Name)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			return err
		}
		_, cerr := io.CopyN(out, tr, hdr.Size)
		out.Close()
		if cerr != nil {
			return fmt.Errorf("package archive member %q is truncated: %w", hdr.Name, cerr)
		}
	}
	return nil
}
