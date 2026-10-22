package typecheck

import "sort"

// StdlibModules returns the canonical standard-library module names known to
// the type checker, sorted alphabetically.
func StdlibModules() []string {
	modules := make([]string, 0, len(moduleMembers))
	for name := range moduleMembers {
		modules = append(modules, name)
	}
	sort.Strings(modules)
	return modules
}

// StdlibMembers returns the sorted member names of a canonical standard-library
// module. It reports false when the module is not in the canonical table.
func StdlibMembers(module string) ([]string, bool) {
	members, ok := moduleMembers[module]
	if !ok {
		return nil, false
	}
	names := make([]string, 0, len(members))
	for name := range members {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, true
}
