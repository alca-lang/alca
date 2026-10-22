package apm

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/alca-lang/alca/pkg/ast"
	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/mod"
	"github.com/alca-lang/alca/pkg/vm"
)

// Enable wires APM into the language engines: `use pkg` resolves against the
// local registry, and the VM runs a compatible compiled artifact when one is
// installed. Call once per process (cmd/alca does it at init). The hooks look up
// $ALCA_HOME on every call, so tests and tools can repoint the registry.
func Enable() {
	mod.SetResolver(ResolveInstalledModule)
	vm.SetModuleLoader(LoadCompiledModule)
}

// ResolveInstalledModule resolves a module path against installed packages:
// `use http` maps to the package entry, `use http.client` to a file next to the
// entry. It implements mod.Resolver.
func ResolveInstalledModule(modulePath, fromFile string) (mod.Resolved, bool, error) {
	modulePath = strings.TrimSpace(modulePath)
	if modulePath == "" || strings.HasPrefix(modulePath, "std.") || modulePath == "std" {
		return mod.Resolved{}, false, nil
	}
	segs := strings.Split(modulePath, ".")
	name := segs[0]
	if !ValidName(name) {
		return mod.Resolved{}, false, nil
	}
	reg, err := NewRegistry("")
	if err != nil {
		// A broken home directory must not turn every unknown import into a
		// hard error; the engines report the module as not found.
		return mod.Resolved{}, false, nil
	}
	idx, err := reg.LoadIndex()
	if err != nil {
		return mod.Resolved{}, false, err
	}
	ve, ok, err := idx.Installed(name)
	if err != nil {
		return mod.Resolved{}, false, err
	}
	if !ok {
		return mod.Resolved{}, false, nil
	}
	root := filepath.Join(reg.Dir(), filepath.FromSlash(ve.Path))
	entry := filepath.Join(root, filepath.FromSlash(ve.Entry))
	source := entry
	if len(segs) > 1 {
		sub := filepath.Join(segs[1:]...) + ".al"
		source = filepath.Join(filepath.Dir(entry), filepath.FromSlash(sub))
		if !fileExists(source) {
			return mod.Resolved{}, false, fmt.Errorf("package %s %s has no module %q (expected %s)",
				name, ve.Version, modulePath, filepath.ToSlash(source))
		}
	}
	if !fileExists(source) {
		return mod.Resolved{}, false, fmt.Errorf("package %s %s is installed but its entry %s is missing; reinstall it with 'apm install %s'",
			name, ve.Version, ve.Entry, name)
	}
	res := mod.Resolved{
		ModulePath: modulePath,
		SourcePath: source,
		Root:       root,
		Package:    name,
		Version:    ve.Version,
	}
	if len(segs) == 1 {
		goos, goarch := HostTarget()
		binRel, _ := ArtifactPaths(goos, goarch)
		binPath := filepath.Join(root, filepath.FromSlash(binRel))
		if fileExists(binPath) {
			res.ArtifactPath = binPath
		}
	}
	return res, true, nil
}

// LoadCompiledModule is the VM's module loader hook: it runs a compatible
// compiled artifact and binds its exports exactly like a source import. It
// returns false to let the VM fall back to the tree-walking import path when no
// artifact is available, compatible or runnable.
func LoadCompiledModule(stmt *ast.UseStatement, fromFile string, env *interpreter.Environment) bool {
	if stmt == nil || env == nil {
		return false
	}
	res, found, err := ResolveInstalledModule(stmt.ModulePath, fromFile)
	if err != nil || !found || res.ArtifactPath == "" || res.Root == "" {
		return false
	}
	goos, goarch := HostTarget()
	code, meta, err := LoadArtifact(res.Root, goos, goarch)
	if err != nil {
		return false // incompatible, stale or corrupt: use the sources
	}
	sub := vm.NewWithEnv(code, env)
	sub.SetSourceFile(filepath.Join(res.Root, filepath.FromSlash(meta.Entry)))
	if err := sub.Run(); err != nil {
		return false
	}
	modMap := interpreter.NewMap()
	for name, idx := range meta.Exports {
		modMap.Set(name, sub.GlobalAt(idx))
	}
	interpreter.BindModule(env, stmt, modMap)
	return true
}
