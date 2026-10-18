package main

import (
	"io"
	"os"
	"strings"
	"testing"
)

// captureOutput runs fn with stdout and stderr redirected to pipes and returns
// everything written to each stream.
func captureOutput(t *testing.T, fn func()) (string, string) {
	t.Helper()
	oldOut, oldErr := os.Stdout, os.Stderr
	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout, os.Stderr = wOut, wErr
	done := make(chan [2]string, 1)
	go func() {
		bOut, _ := io.ReadAll(rOut)
		bErr, _ := io.ReadAll(rErr)
		done <- [2]string{string(bOut), string(bErr)}
	}()

	fn()

	wOut.Close()
	wErr.Close()
	os.Stdout, os.Stderr = oldOut, oldErr
	res := <-done
	return res[0], res[1]
}

// TestREPLKeepsStateAcrossLines evaluates several snippets on one session and
// expects the interpreter values and checker symbols to persist.
func TestREPLKeepsStateAcrossLines(t *testing.T) {
	session := newREPLSession()
	out, errOut := captureOutput(t, func() {
		session.eval("x := 40;")
		session.eval("x = x + 2;") // assignments echo their value
		session.eval("println(x);")
	})
	if errOut != "" {
		t.Fatalf("unexpected diagnostics:\n%s", errOut)
	}
	if out != "42\n42\n" {
		t.Fatalf("expected the assignment echo and the println, got %q", out)
	}
}

// TestREPLFunctionsAndBlocksPersist covers multi-line snippets.
func TestREPLFunctionsAndBlocksPersist(t *testing.T) {
	session := newREPLSession()
	out, errOut := captureOutput(t, func() {
		session.eval("func twice(n: int) -> int:\n    return n * 2;")
		session.eval("total := 0;")
		session.eval("for i in 1..3:\n    total = total + i;")
		session.eval("println(twice(total));")
	})
	if errOut != "" {
		t.Fatalf("unexpected diagnostics:\n%s", errOut)
	}
	if strings.TrimSpace(out) != "12" {
		t.Fatalf("expected 12, got %q", out)
	}
}

// TestREPLEchoesExpressionValues keeps the convenient expression echo.
func TestREPLEchoesExpressionValues(t *testing.T) {
	session := newREPLSession()
	out, errOut := captureOutput(t, func() {
		session.eval("1 + 2;")
	})
	if errOut != "" {
		t.Fatalf("unexpected diagnostics:\n%s", errOut)
	}
	if strings.TrimSpace(out) != "3" {
		t.Fatalf("expected 3, got %q", out)
	}
}

// TestREPLReportsMissingSemicolon checks the strict-syntax error is shown.
func TestREPLReportsMissingSemicolon(t *testing.T) {
	session := newREPLSession()
	out, _ := captureOutput(t, func() {
		session.eval("println(1)")
	})
	if !strings.Contains(out, "expected ';'") {
		t.Fatalf("missing ';' must be reported clearly, got:\n%s", out)
	}
}

// TestREPLTypeErrorsReported checks that snippets are statically verified
// before execution.
func TestREPLTypeErrorsReported(t *testing.T) {
	session := newREPLSession()
	_, errOut := captureOutput(t, func() {
		session.eval("n: int = \"text\";")
	})
	if !strings.Contains(errOut, "cannot initialize") {
		t.Fatalf("expected a type error, got:\n%s", errOut)
	}
	// The rejected declaration must not exist afterwards in either engine
	// state, the same way it never ran.
	out, errOut2 := captureOutput(t, func() {
		session.eval("println(n);")
	})
	if !strings.Contains(errOut2, "undefined name") {
		t.Fatalf("rejected declaration leaked into the checker; stdout=%q stderr=%q", out, errOut2)
	}
}

// TestREPLRejectedRedeclarationRollsBack checks that a snippet rejected for
// redeclaring an existing variable leaves the previous value in place and the
// checker in sync.
func TestREPLRejectedRedeclarationRollsBack(t *testing.T) {
	session := newREPLSession()
	_, errOut := captureOutput(t, func() {
		session.eval("x := 1;")
		session.eval("x := \"two\";")
	})
	if !strings.Contains(errOut, "already declared") {
		t.Fatalf("expected a redeclaration error, got:\n%s", errOut)
	}
	out, errOut2 := captureOutput(t, func() {
		session.eval("println(x);")
	})
	if errOut2 != "" {
		t.Fatalf("x must still exist after the rollback:\n%s", errOut2)
	}
	if strings.TrimSpace(out) != "1" {
		t.Fatalf("expected the old value 1, got %q", out)
	}
}
