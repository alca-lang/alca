// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"strings"
	"testing"
)

// TestREPLAcceptsIndentedSnippet covers a statement pasted with the leading
// whitespace of the line it came from: the REPL must not report an INDENT
// error, because indentation only carries meaning inside a block.
func TestREPLAcceptsIndentedSnippet(t *testing.T) {
	session := newREPLSession()
	out, errOut := captureOutput(t, func() {
		session.eval("    result: int = 6 * 7;\n")
	})
	if errOut != "" {
		t.Fatalf("unexpected diagnostics:\n%s", errOut)
	}
	if strings.Contains(out, "INDENT") {
		t.Fatalf("leading whitespace must not produce an INDENT error:\n%s", out)
	}
}

// TestREPLAcceptsIndentedBlock covers a block whose header and body carry a
// common leading indentation.
func TestREPLAcceptsIndentedBlock(t *testing.T) {
	session := newREPLSession()
	out, errOut := captureOutput(t, func() {
		session.eval("    i: int = 0;\n    while i < 5:\n        i = i + 1;\n")
	})
	if errOut != "" {
		t.Fatalf("unexpected diagnostics:\n%s", errOut)
	}
	if strings.Contains(out, "INDENT") {
		t.Fatalf("an indented block must not produce an INDENT error:\n%s", out)
	}
}

// TestREPLIndentedBlockKeepsRelativeIndentation makes sure dedenting the
// snippet does not flatten the block: the body must stay one level deeper
// than its header.
func TestREPLIndentedBlockKeepsRelativeIndentation(t *testing.T) {
	session := newREPLSession()
	out, errOut := captureOutput(t, func() {
		session.eval("    total: int = 0;\n    for i in 1..=3:\n        total = total + i;\n    println(total);\n")
	})
	if errOut != "" {
		t.Fatalf("unexpected diagnostics:\n%s", errOut)
	}
	if strings.TrimSpace(out) != "6" {
		t.Fatalf("expected 6, got %q", out)
	}
}

// TestREPLMetaCommands covers the REPL's own commands, which are handled
// before the input reaches the parser.
func TestREPLMetaCommands(t *testing.T) {
	for _, cmd := range []string{"help", "clear"} {
		t.Run(cmd, func(t *testing.T) {
			session := newREPLSession()
			out, errOut := captureOutput(t, func() {
				res, ok := session.command(cmd)
				if !ok {
					t.Fatalf("%q must be a REPL command", cmd)
				}
				fmt.Print(res)
			})
			if errOut != "" {
				t.Fatalf("unexpected diagnostics:\n%s", errOut)
			}
			if strings.Contains(out, "expected ';'") {
				t.Fatalf("%q must be handled by the REPL, not the parser:\n%s", cmd, out)
			}
		})
	}
}

// TestREPLRejectsCStyleDeclaration checks that the type-first form
// ("int: x = 5;") reports the Alca declaration order instead of a bare
// "expected ';'" complaint.
func TestREPLRejectsCStyleDeclaration(t *testing.T) {
	session := newREPLSession()
	out, _ := captureOutput(t, func() {
		session.eval("int: a = 10;")
	})
	if !strings.Contains(out, "name: int = value;") {
		t.Fatalf("the error must show the Alca declaration order:\n%s", out)
	}
}

func TestHasUnclosedTripleQuote(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{`s := """hello`, true},
		{`s := """hello""";`, false},
		{`s := '''hello`, true},
		{`s := '''hello''';`, false},
		{`s := "normal string";`, false},
		{`r"""unclosed raw`, true},
		{`r"""closed raw""";`, false},
		{`f"""unclosed fstring {x}`, true},
		{`// """ comment`, false},
	}
	for _, tc := range tests {
		got := hasUnclosedTripleQuote(tc.input)
		if got != tc.want {
			t.Errorf("hasUnclosedTripleQuote(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}
