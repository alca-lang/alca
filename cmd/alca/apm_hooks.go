package main

import (
	"github.com/alca-lang/alca/pkg/apm"
)

// init wires APM into the language engines for every alca invocation:
// `use pkg` resolves against the local package registry, and the VM uses a
// compatible compiled artifact when one is installed, falling back to sources.
func init() {
	apm.Enable()
}
