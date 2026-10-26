// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

// Package version hosts the version of the Alca toolchain shared by the CLI,
// the package manager and the registry node.
package version

// Version is the semantic version of the Alca language and toolchain. It is
// recorded in package artifacts so a runtime can refuse incompatible builds.
const Version = "2.0.0"
