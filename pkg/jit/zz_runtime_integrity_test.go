// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package jit_test

import (
	"testing"
	"time"
)

// TestZZRuntimeIntegrity runs last in this package (the file name sorts last,
// and Go executes tests in file order) and verifies that the Go runtime's own
// data structures are still intact after every other test in the package has
// executed.
//
// The native backend writes stack slots directly, so an out-of-bounds write
// there silently corrupts unrelated heap objects. The most visible symptom is
// a standard-library table losing an entry, which shows up as a spurious
// "unknown unit" failure from time.ParseDuration.
func TestZZRuntimeIntegrity(t *testing.T) {
	if _, err := time.ParseDuration("1h30m"); err != nil {
		t.Fatalf("time.ParseDuration is broken after the package tests: %v", err)
	}
	if _, err := time.ParseDuration("100ms"); err != nil {
		t.Fatalf("time.ParseDuration is broken after the package tests: %v", err)
	}
	if _, err := time.Parse(time.RFC3339, "2020-01-02T03:04:05Z"); err != nil {
		t.Fatalf("time.Parse is broken after the package tests: %v", err)
	}
}
