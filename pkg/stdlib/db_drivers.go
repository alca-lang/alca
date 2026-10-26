// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

//go:build !nodb

package stdlib

// Pure-Go SQL drivers (no CGO). Build with `-tags nodb` to exclude them
// (smaller binary, no third-party modules required).
import (
	_ "github.com/lib/pq"  // PostgreSQL
	_ "modernc.org/sqlite" // SQLite
)

// DBDriversLinked reports whether SQL drivers are compiled in.
const DBDriversLinked = true
