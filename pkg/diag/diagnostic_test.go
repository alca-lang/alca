// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package diag_test

import (
	"strings"
	"testing"

	"github.com/alca-lang/alca/pkg/diag"
)

func TestFormatReport(t *testing.T) {
	source := `func calculate(x: int):
    y = : 5 + 10
    return y`

	errs := []*diag.SyntaxError{
		{
			Filename:   "test.al",
			Line:       2,
			Column:     9,
			Message:    "unexpected token ':'",
			Token:      ":",
			Length:     1,
			Suggestion: "Did you mean an assignment '=' or a type annotation?",
		},
	}

	report := diag.FormatReport("test.al", source, errs, false)

	if !strings.Contains(report, "test.al:2:9") {
		t.Errorf("expected report to contain filename and line, got:\n%s", report)
	}
	if !strings.Contains(report, "y = : 5 + 10") {
		t.Errorf("expected report to contain source line, got:\n%s", report)
	}
	if !strings.Contains(report, "^") {
		t.Errorf("expected report to contain caret, got:\n%s", report)
	}
	if !strings.Contains(report, "help:") {
		t.Errorf("expected report to contain help hint, got:\n%s", report)
	}
}

// TestFormatUnsupportedFeatureDoesNotBlameBuild: when the user explicitly runs
// a program with --vm and the backend rejects a construct, the diagnostic must
// name the unsupported feature and suggest the interpreter, not reuse the
// native-build template ("compilation to native executable failed", writable
// output path, disk space) which describes an operation that never happened.
func TestFormatUnsupportedFeatureDoesNotBlameBuild(t *testing.T) {
	report := diag.FormatUnsupportedFeature("app.al", "threads blocks", 3, false)
	for _, want := range []string{"error[UnsupportedFeature]", "threads blocks", "app.al:3", "help:", "--interp"} {
		if !strings.Contains(report, want) {
			t.Errorf("report must contain %q, got:\n%s", want, report)
		}
	}
	for _, unwanted := range []string{"BuildError", "native executable", "output path", "disk space"} {
		if strings.Contains(report, unwanted) {
			t.Errorf("report must not contain %q, got:\n%s", unwanted, report)
		}
	}
}
