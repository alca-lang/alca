// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package apm

import (
	"os"
	"strings"
)

// DefaultRegistry is the registry node URL compiled into this build of the
// toolchain. Release builders stamp it with:
//
//	go build -ldflags \
//	  "-X github.com/alca-lang/alca/pkg/apm.DefaultRegistry=https://packages.example.com" \
//	  ./cmd/apm
//
// It is consulted only when neither --registry nor $ALCA_REGISTRY is set, so
// an unstamped build keeps the offline-by-default behaviour.
var DefaultRegistry = "https://pkg.alca-lang.org"

// DefaultRegistryURL returns the registry node URL baked into this binary at
// build time, or "" when none was stamped in.
func DefaultRegistryURL() string { return strings.TrimSpace(DefaultRegistry) }

// ResolveRegistryURL applies the registry URL precedence used by the CLI:
// the explicit flag value wins over $ALCA_REGISTRY, which wins over the node
// compiled into the binary. An empty result means the operation is offline.
func ResolveRegistryURL(flagValue string) string {
	if v := strings.TrimSpace(flagValue); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv(EnvRegistry)); v != "" {
		return v
	}
	return DefaultRegistryURL()
}
