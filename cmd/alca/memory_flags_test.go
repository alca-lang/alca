// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"

	"github.com/alca-lang/alca/pkg/limits"
)

// TestMemoryFlagStopsGrowingProgram verifies that --max-memory turns a
// program that grows without bound into a regular runtime error instead of an
// out-of-memory kill of the process.
func TestMemoryFlagStopsGrowingProgram(t *testing.T) {
	program := writeAlca(t, `
func grow() -> int:
    xs: list[int] = [];
    i := 0;
    while i < 1000000:
        push(xs, i);
        i = i + 1;
    return len(xs);

println(grow());
`)

	for _, engineFlag := range []string{"--vm", "--interp"} {
		t.Run(engineFlag, func(t *testing.T) {
			_, stderr, code := runCLI(t, engineFlag, "--max-memory=8mb", program)
			if code == 0 {
				t.Fatalf("the memory limit must stop the program, stderr:\n%s", stderr)
			}
			if !strings.Contains(stderr, "out of memory") {
				t.Fatalf("stderr must explain the memory limit, got:\n%s", stderr)
			}
		})
	}
}

// TestMemoryFlagAllowsNormalProgram verifies that a program below the limit
// runs to completion.
func TestMemoryFlagAllowsNormalProgram(t *testing.T) {
	program := writeAlca(t, "println(\"ok\");\n")
	stdout, stderr, code := runCLI(t, "--max-memory=64mb", program)
	if code != 0 {
		t.Fatalf("run failed with %d:\n%s", code, stderr)
	}
	if stdout != "ok\n" {
		t.Fatalf("stdout = %q, want %q", stdout, "ok\n")
	}
}

// TestMemoryFlagRejectsInvalidSize verifies that a malformed size is reported
// instead of being ignored.
func TestMemoryFlagRejectsInvalidSize(t *testing.T) {
	program := writeAlca(t, "println(\"ok\");\n")
	_, stderr, code := runCLI(t, "--max-memory=not-a-size", program)
	if code == 0 {
		t.Fatal("an invalid size must fail")
	}
	if !strings.Contains(stderr, "max-memory") {
		t.Fatalf("stderr must mention the flag, got:\n%s", stderr)
	}
}

// TestMemoryFlagInstallsLimit verifies that the flag reaches the shared meter.
func TestMemoryFlagInstallsLimit(t *testing.T) {
	flags, rest, err := parseMemoryFlags([]string{"--max-memory=1mb", "--max-allocation=2mb", "prog.al"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(rest) != 1 || rest[0] != "prog.al" {
		t.Fatalf("rest = %v, want [prog.al]", rest)
	}
	if flags.live != 1<<20 || flags.alloc != 2<<20 {
		t.Fatalf("flags = %+v, want 1mb live and 2mb allocation", flags)
	}

	applyMemoryFlags(flags)
	live, alloc := limits.Limits()
	if live != 1<<20 || alloc != 2<<20 {
		t.Fatalf("limits = (%d, %d), want (1mb, 2mb)", live, alloc)
	}
	limits.Reset()
	limits.SetLimits(0, 0)
}
