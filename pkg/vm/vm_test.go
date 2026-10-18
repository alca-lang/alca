package vm

import (
	"strings"
	"testing"

	"github.com/alca-lang/alca/pkg/diag"
	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/lexer"
	"github.com/alca-lang/alca/pkg/parser"
)

// runSource compiles and runs src on a fresh VM, returning the runtime error.
func runSource(t *testing.T, src string) error {
	t.Helper()
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if errs := p.SyntaxErrors(); len(errs) > 0 {
		t.Fatalf("syntax errors: %v", errs)
	}
	c := NewCompiler()
	if err := c.Compile(prog); err != nil {
		t.Fatalf("compile: %v", err)
	}
	machine := NewWithEnv(c.Bytecode(), interpreter.New().Env())
	return machine.Run()
}

// TestErrExpressionKeepsExplicitCode: err("msg", code=N) must raise an error
// whose code is N on the VM, matching the interpreter.
func TestErrExpressionKeepsExplicitCode(t *testing.T) {
	err := runSource(t, "err(\"boom\", code=7);\n")
	re, ok := err.(*RaisedError)
	if !ok {
		t.Fatalf("expected *RaisedError, got %T: %v", err, err)
	}
	if re.Val.Code != 7 {
		t.Fatalf("raised error code = %d, want 7 (message %q)", re.Val.Code, re.Val.Message)
	}
}

// TestRaisedErrorReportsItsOwnCodeInDiagnostics: formatting a VM runtime error
// must print the Alca error code the program used, not a fallback code.
// A regression here made `err("boom", code=7)` display "(code 400)" on the VM
// while the interpreter printed "(code 7)".
func TestRaisedErrorReportsItsOwnCodeInDiagnostics(t *testing.T) {
	err := runSource(t, "err(\"boom\", code=7);\n")
	if err == nil {
		t.Fatalf("expected a raised error")
	}
	report := diag.FormatRuntimeError("t.al", "err(\"boom\", code=7);\n", err, false)
	if !strings.Contains(report, "(code 7)") {
		t.Fatalf("diagnostic must report code 7, got:\n%s", report)
	}
	if strings.Contains(report, "(code 400)") {
		t.Fatalf("diagnostic must not use the fallback code 400, got:\n%s", report)
	}
}
