// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

//go:build nodb

package stdlib

// Built with -tags nodb: no SQL drivers are linked, db.* returns a runtime error on open.

// DBDriversLinked reports whether SQL drivers are compiled in.
const DBDriversLinked = false
