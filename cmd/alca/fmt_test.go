// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFormatFileRewritesInPlaceAndIsIdempotent exercises the 'alca fmt -w'
// code path: the file is rewritten with canonical formatting and a second run
// leaves it unchanged.
func TestFormatFileRewritesInPlaceAndIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "messy.al")
	src := "x:=1;\nif x==1:\n\tprintln( \"one\" ); // note\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	if code := formatFile(path, true); code != 0 {
		t.Fatalf("formatFile exited with code %d", code)
	}
	want := "x := 1;\nif x == 1:\n    println(\"one\"); // note\n"
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("formatted file mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}

	if code := formatFile(path, true); code != 0 {
		t.Fatalf("second formatFile exited with code %d", code)
	}
	again, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(again) != want {
		t.Fatalf("formatting is not idempotent:\n%s", again)
	}
}

// TestFormatFileRejectsBrokenSource checks that 'alca fmt' reports diagnostics
// and a non-zero exit code instead of rewriting unparsable input.
func TestFormatFileRejectsBrokenSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.al")
	src := "x := 1\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := formatFile(path, true); code == 0 {
		t.Fatal("formatFile accepted a file with a missing ';'")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != src {
		t.Fatalf("broken file must not be rewritten, got:\n%s", got)
	}
}

// TestFormatFileMigratesRanges exercises 'alca fmt --migrate-ranges -w'.
func TestFormatFileMigratesRanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ranges.al")
	src := "total := 0;\nfor i in 1..10:\n    total = total + i;\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	if code := formatFile(path, true, true); code != 0 {
		t.Fatalf("formatFile with migrate exited with code %d", code)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "total := 0;\nfor i in 1 ..= 10:\n    total = total + i;\n"
	if string(got) != want {
		t.Fatalf("range migration mismatch\n got: %q\nwant: %q", string(got), want)
	}
}
