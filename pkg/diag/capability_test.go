package diag

import (
	"errors"
	"strings"
	"testing"
)

// TestCleanErrorMessageKeepsCapabilityHint: sandbox denials explain which flag
// grants access. The generic OS-permission translation used to discard that
// hint, leaving users with only "insufficient privileges".
func TestCleanErrorMessageKeepsCapabilityHint(t *testing.T) {
	raw := "permission denied: read access to \"/tmp/x\" is not allowed (pass --allow-read[=<path>])"
	if got := CleanErrorMessage(errors.New(raw)); !strings.Contains(got, "--allow-read") {
		t.Fatalf("capability hint was dropped: %q", got)
	}

	report := FormatRuntimeError("x.al", "", errors.New(raw), false)
	if !strings.Contains(report, "--allow-read") {
		t.Fatalf("runtime report must keep the capability hint:\n%s", report)
	}
	if !strings.Contains(report, "Grant read access") {
		t.Fatalf("runtime report must suggest the flag:\n%s", report)
	}
}
