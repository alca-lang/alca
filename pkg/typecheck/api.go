// Package-level API for editor tooling: the LSP server and the CLI need read
// access to the standard module tables without duplicating them.
package typecheck

import "sort"

// StdModuleNames returns every standard module name, aliases included, sorted.
func StdModuleNames() []string {
	seen := map[string]bool{}
	var names []string
	for name := range moduleMembers {
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	for alias := range moduleAliases {
		if !seen[alias] {
			seen[alias] = true
			names = append(names, alias)
		}
	}
	sort.Strings(names)
	return names
}

// StdModuleTypes returns the member table of a standard module looked up by
// canonical name or alias, or nil when the module is unknown. The returned map
// is a copy and may be modified freely.
func StdModuleTypes(name string) map[string]Type {
	m, ok := moduleByName(name)
	if !ok {
		return nil
	}
	out := make(map[string]Type, len(m.Members))
	for member, t := range m.Members {
		out[member] = t
	}
	return out
}

// CanonicalModuleName maps a module alias ("zip", "regex") to its canonical
// name ("archive", "re"); other names are returned unchanged.
func CanonicalModuleName(name string) string {
	if canon, ok := moduleAliases[name]; ok {
		return canon
	}
	return name
}

// GlobalBuiltinTypes returns every always-available built-in function with its
// type, keyed by name. The returned map is a copy and may be modified freely.
func GlobalBuiltinTypes() map[string]Type {
	g := globalBuiltins()
	out := make(map[string]Type, len(g))
	for name, t := range g {
		out[name] = t
	}
	return out
}
