// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package interpreter

import (
	"errors"
	"os"
	"testing"

	"github.com/alca-lang/alca/pkg/sandbox"
)

func TestAsyncStreamReportsSandboxDenial(t *testing.T) {
	previous := sandbox.CurrentPolicy()
	defer sandbox.SetPolicy(previous)
	sandbox.SetPolicy(sandbox.DenyAll())

	for _, operator := range []string{"&>>", "&&>>"} {
		if _, err := HandleStream(&StringValue{Val: "content"}, operator, &StringValue{Val: "blocked.txt"}, nil); err != nil {
			t.Fatalf("%s returned before its background operation: %v", operator, err)
		}
	}

	err := WaitAll()
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("WaitAll() error = %v, want a permission error", err)
	}
}

func TestAsyncStreamReportsWriteFailure(t *testing.T) {
	previous := sandbox.CurrentPolicy()
	defer sandbox.SetPolicy(previous)
	sandbox.SetPolicy(sandbox.AllowAll())

	dir := t.TempDir()
	target := dir + string(os.PathSeparator) + "directory"
	if err := os.Mkdir(target, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := HandleStream(&StringValue{Val: "content"}, "&>>", &StringValue{Val: target}, nil); err != nil {
		t.Fatal(err)
	}
	if err := WaitAll(); err == nil {
		t.Fatal("WaitAll() did not report a failed file write")
	}
}
