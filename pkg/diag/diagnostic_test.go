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
