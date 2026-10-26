// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

// Package mod resolves Alca `use` module paths to loadable files.
//
// The package is deliberately small and dependency-light: both the tree-walking
// interpreter and the static checker call it, and a package registry (APM) can
// install a resolver to extend it with installed libraries.
//
// Resolution order for a non-std module path:
//
//  1. overrides installed for a standalone source bundle;
//  2. a local .al file next to the importing file, then next to the current
//     working directory (the historical behaviour);
//  3. the registered registry resolver, which looks up installed packages.
//
// Local files that escape the project root require an explicit read capability,
// so a program cannot use `use` to probe arbitrary files when the CLI runs it
// with the default deny-all policy. Module loading itself is a language
// feature, not program I/O, so files inside the project and all registry
// packages stay readable under deny-all.
package mod

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/alca-lang/alca/pkg/sandbox"
)

// Resolved describes where a `use` module was found.
type Resolved struct {
	ModulePath string // the path written in the source
	SourcePath string // absolute .al source file, empty when only an artifact exists
	// ArtifactPath is an absolute compiled artifact (serialized VM bytecode plus
	// a sidecar metadata file). Callers must verify compatibility before use and
	// fall back to SourcePath when it does not match.
	ArtifactPath string
	Package      string // registry package name, when the module came from APM
	Version      string // registry package version
	Root         string // package root directory or project directory
}

// Resolver looks a module path up in an external source, such as the APM
// registry. It returns found=false when the module is unknown, and an error
// only for real failures (corrupt registry, permission denial, ...).
type Resolver func(modulePath, fromFile string) (Resolved, bool, error)

var (
	mu          sync.RWMutex
	resolver    Resolver
	bundled     map[string]string
	projectRoot string
)

// SetResolver installs the registry resolver. Passing nil removes it.
func SetResolver(r Resolver) {
	mu.Lock()
	defer mu.Unlock()
	resolver = r
}

// SetBundled installs source overrides for a standalone program: module import
// paths mapped to absolute .al file paths inside the materialised bundle.
func SetBundled(files map[string]string) {
	mu.Lock()
	defer mu.Unlock()
	if len(files) == 0 {
		bundled = nil
		return
	}
	bundled = make(map[string]string, len(files))
	for k, v := range files {
		bundled[k] = v
	}
}

// SetProjectRoot sets the directory that project-local imports may read under
// the default deny-all capability policy. Standalone programs set it to the
// directory where the embedded sources were materialised.
func SetProjectRoot(dir string) {
	mu.Lock()
	defer mu.Unlock()
	projectRoot = dir
}

// ProjectRoot returns the configured project root, or "" when unset.
func ProjectRoot() string {
	mu.RLock()
	defer mu.RUnlock()
	return projectRoot
}

// BindingName is the name a `use` statement binds when no alias is given:
// the last dotted segment, or the file base name for path-style imports.
func BindingName(modulePath string) string {
	p := strings.TrimSpace(modulePath)
	if p == "" {
		return ""
	}
	if strings.HasSuffix(p, ".al") || strings.ContainsAny(p, `/\`) {
		base := filepath.Base(filepath.FromSlash(p))
		return strings.TrimSuffix(base, filepath.Ext(base))
	}
	if i := strings.LastIndexByte(p, '.'); i >= 0 && i+1 < len(p) {
		return p[i+1:]
	}
	return p
}

// Resolve maps a module path written in source to a loadable file. found=false
// means no local file and no registered resolver knew the module.
func Resolve(modulePath, fromFile string) (Resolved, bool, error) {
	modulePath = strings.TrimSpace(modulePath)
	if modulePath == "" {
		return Resolved{}, false, nil
	}

	mu.RLock()
	b := bundled
	r := resolver
	mu.RUnlock()

	if p, ok := b[modulePath]; ok {
		return Resolved{ModulePath: modulePath, SourcePath: p}, true, nil
	}
	if res, ok, err := resolveLocal(modulePath, fromFile); ok || err != nil {
		return res, ok, err
	}
	if r != nil {
		return r(modulePath, fromFile)
	}
	return Resolved{}, false, nil
}

// resolveLocal looks for an .al file next to the importing file and then in the
// working directory, preserving the historical relative-import behaviour while
// making it independent of where the CLI happens to be launched from.
func resolveLocal(modulePath, fromFile string) (Resolved, bool, error) {
	rel := localRelativePath(modulePath)
	if rel == "" {
		return Resolved{}, false, nil
	}

	var dirs []string
	if fromFile != "" && fromFile != "<repl>" && !strings.HasPrefix(fromFile, "<") {
		dirs = append(dirs, filepath.Dir(fromFile))
	}
	if wd, err := os.Getwd(); err == nil {
		dirs = append(dirs, wd)
	}

	seen := map[string]bool{}
	for _, dir := range dirs {
		abs, err := filepath.Abs(filepath.Join(dir, rel))
		if err != nil {
			continue
		}
		abs = filepath.Clean(abs)
		if seen[abs] {
			continue
		}
		seen[abs] = true
		fi, err := os.Stat(abs)
		if err != nil || fi.IsDir() || !fi.Mode().IsRegular() {
			continue
		}
		if err := allowLocalRead(abs); err != nil {
			return Resolved{}, false, err
		}
		return Resolved{ModulePath: modulePath, SourcePath: abs, Root: filepath.Dir(abs)}, true, nil
	}
	return Resolved{}, false, nil
}

// allowLocalRead permits files inside the project root unconditionally and
// requires a read capability for files that escape it. Without a project root
// (library embedding, tests) local imports keep their permissive default.
func allowLocalRead(path string) error {
	root := ProjectRoot()
	if root == "" {
		return nil
	}
	if sandbox.PathWithin(root, path) {
		return nil
	}
	return sandbox.RequireRead(path)
}

// localRelativePath converts a module path into a relative file path.
func localRelativePath(modulePath string) string {
	p := strings.TrimSpace(modulePath)
	if p == "" {
		return ""
	}
	if strings.HasSuffix(p, ".al") || strings.ContainsAny(p, `/\`) || strings.HasPrefix(p, ".") {
		return p
	}
	return strings.ReplaceAll(p, ".", string(filepath.Separator)) + ".al"
}
