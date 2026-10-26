// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package typecheck_test

import (
	"strings"
	"testing"

	"github.com/alca-lang/alca/pkg/typecheck"
)

func TestVoidFunctionTypecheck(t *testing.T) {
	c := typecheck.NewChecker("<test>", "")

	// Declaring a void function and returning nothing should pass
	goodDecl := "void log_msg(msg: string) { return; }"
	if errs := snippetErrors(t, c, goodDecl); errs != "" {
		t.Fatalf("expected void function declaration to succeed, got: %s", errs)
	}

	// Calling a void function as a statement should pass
	callStmt := "log_msg(\"hello\");"
	if errs := snippetErrors(t, c, callStmt); errs != "" {
		t.Fatalf("expected calling void function statement to succeed, got: %s", errs)
	}

	// Assigning void function call to a variable should fail
	badAssign := "x: int = log_msg(\"hi\");"
	errs := snippetErrors(t, c, badAssign)
	if errs == "" || !strings.Contains(errs, "void") {
		t.Fatalf("expected void assignment error, got: %q", errs)
	}

	// Declaring a void function that returns a value should fail
	c2 := typecheck.NewChecker("<test2>", "")
	badReturn := "void bad() { return 123; }"
	errs2 := snippetErrors(t, c2, badReturn)
	if errs2 == "" || (!strings.Contains(errs2, "void") && !strings.Contains(errs2, "returns a value")) {
		t.Fatalf("expected returning value from void function to fail, got: %q", errs2)
	}
}
