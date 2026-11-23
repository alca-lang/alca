// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

// Package dunder defines the read-only built-in variables that both execution
// engines make available to every Pawl program.
package dunder

import "strings"

// Known names and their declared types. Positional values are resolved from
// the token at the point of use rather than from the environment.
type Def struct {
	Type       string // "string", "int", "bool" or "list[string]"
	Positional bool
}

// Registry is the single source of truth for dunder definitions.
var Registry = map[string]Def{
	"__file__":    {Type: "string"},
	"__dir__":     {Type: "string"},
	"__dirname__": {Type: "string"},
	"__name__":    {Type: "string"},
	"__version__": {Type: "string"},
	"__line__":    {Type: "int", Positional: true},
	"__column__":  {Type: "int", Positional: true},
	"__os__":      {Type: "string"},
	"__arch__":    {Type: "string"},
	"__exe__":     {Type: "string"},
	"__argv__":    {Type: "list[string]"},
	"__compiled__": {Type: "bool"},
}

// IsReserved reports whether name is a dunder. All __*__ identifiers are
// reserved, including ones this version does not define yet, so user code can
// never shadow a future built-in.
func IsReserved(name string) bool {
	return len(name) > 4 && strings.HasPrefix(name, "__") && strings.HasSuffix(name, "__")
}

// IsPositional reports whether the value must be read from the source token.
func IsPositional(name string) bool {
	d, ok := Registry[name]
	return ok && d.Positional
}
