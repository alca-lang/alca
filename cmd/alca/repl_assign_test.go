// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"strings"
	"testing"
)

// TestREPLAssignsToUndeclaredName covers a snippet that assigns to a name that
// was never declared: the REPL creates the binding, so the snippet check must
// accept it and remember the name for the following snippets.
func TestREPLAssignsToUndeclaredName(t *testing.T) {
	session := newREPLSession()
	out, errOut := captureOutput(t, func() {
		session.eval("a = 10;\n")
		session.eval("println(a);\n")
	})
	if errOut != "" {
		t.Fatalf("unexpected diagnostics:\n%s", errOut)
	}
	if !strings.Contains(out, "10") {
		t.Fatalf("expected the assigned value to be printed, got %q", out)
	}
}

// TestREPLAssignmentKeepsTheDeclaredType checks that the name introduced by an
// assignment is remembered with its type, so a later incompatible assignment
// is still reported.
func TestREPLAssignmentKeepsTheDeclaredType(t *testing.T) {
	session := newREPLSession()
	_, errOut := captureOutput(t, func() {
		session.eval("a = 10;\n")
		session.eval("a = \"text\";\n")
	})
	if !strings.Contains(errOut, "cannot assign") {
		t.Fatalf("expected a type error on the second assignment, got:\n%s", errOut)
	}
}

// TestREPLHintForUndeclaredName checks that a one-letter name is not corrected
// to an unrelated longer builtin.
func TestREPLHintForUndeclaredName(t *testing.T) {
	session := newREPLSession()
	_, errOut := captureOutput(t, func() {
		session.eval("a = 10;\n")
	})
	if strings.Contains(errOut, "did you mean 'abs'") {
		t.Fatalf("a one-letter name must not be corrected to 'abs':\n%s", errOut)
	}
}

// TestREPLHelpListsCommands checks that "help" documents the REPL commands.
func TestREPLHelpListsCommands(t *testing.T) {
	session := newREPLSession()
	out, _ := captureOutput(t, func() {
		res, _ := session.command("help")
		fmt.Print(res)
	})
	for _, want := range []string{"help", "clear", "exit"} {
		if !strings.Contains(out, want) {
			t.Fatalf("help output must mention %q:\n%s", want, out)
		}
	}
}
