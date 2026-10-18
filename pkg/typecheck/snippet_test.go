package typecheck_test

import (
	"strings"
	"testing"

	"github.com/alca-lang/alca/pkg/ast"
	"github.com/alca-lang/alca/pkg/lexer"
	"github.com/alca-lang/alca/pkg/parser"
	"github.com/alca-lang/alca/pkg/typecheck"
)

// parseForSnippets parses one REPL-style snippet.
func parseForSnippets(t *testing.T, src string) *ast.Program {
	t.Helper()
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if errs := p.SyntaxErrors(); len(errs) > 0 {
		t.Fatalf("unexpected syntax errors in %q: %v", src, errs[0].Message)
	}
	return prog
}

// snippetErrors runs an incremental check and renders the diagnostics.
func snippetErrors(t *testing.T, c *typecheck.Checker, src string) string {
	t.Helper()
	var msgs []string
	for _, e := range c.CheckSnippet(parseForSnippets(t, src)) {
		msgs = append(msgs, e.Message)
	}
	return strings.Join(msgs, "\n")
}

// TestCheckSnippetKeepsDeclarations verifies that a checker used across REPL
// snippets remembers variables, functions and structs declared earlier.
func TestCheckSnippetKeepsDeclarations(t *testing.T) {
	c := typecheck.NewChecker("<repl>", "")

	if got := snippetErrors(t, c, "x: int = 1;"); got != "" {
		t.Fatalf("declaring x failed: %s", got)
	}
	if got := snippetErrors(t, c, "func twice(n: int) -> int:\n    return n * 2;"); got != "" {
		t.Fatalf("declaring twice failed: %s", got)
	}
	if got := snippetErrors(t, c, "struct P:\n    n: int;"); got != "" {
		t.Fatalf("declaring P failed: %s", got)
	}
	if got := snippetErrors(t, c, "x = twice(x);"); got != "" {
		t.Fatalf("using earlier declarations failed: %s", got)
	}
	if got := snippetErrors(t, c, "p := P(x);"); got != "" {
		t.Fatalf("using struct P failed: %s", got)
	}

	// Redeclaration must be rejected exactly like in a file.
	if got := snippetErrors(t, c, "x := 2;"); !strings.Contains(got, "already declared") {
		t.Fatalf("expected a redeclaration error, got %q", got)
	}
}

// TestCheckSnippetRestoreRollsBackFailedSnippet verifies Snapshot/Restore:
// declarations from a snippet that failed later in the same check must not
// leak into the next snippet, while earlier declarations survive.
func TestCheckSnippetRestoreRollsBackFailedSnippet(t *testing.T) {
	c := typecheck.NewChecker("<repl>", "")
	if got := snippetErrors(t, c, "keep: int = 7;"); got != "" {
		t.Fatalf("declaring keep failed: %s", got)
	}

	snap := c.Snapshot()
	bad := "leak := 1;\nstruct Temp:\n    f: int;\nbad: int = \"no\";"
	if got := snippetErrors(t, c, bad); !strings.Contains(got, "cannot initialize") {
		t.Fatalf("expected the bad snippet to fail, got %q", got)
	}
	c.Restore(snap)

	if got := snippetErrors(t, c, "println(leak);"); !strings.Contains(got, "undefined name") {
		t.Fatalf("rolled-back variable must not exist, got %q", got)
	}
	if got := snippetErrors(t, c, "t := Temp(1);"); !strings.Contains(got, "undefined name") {
		t.Fatalf("rolled-back struct must not exist, got %q", got)
	}
	if got := snippetErrors(t, c, "keep = keep + 1;"); got != "" {
		t.Fatalf("earlier declarations must survive the rollback: %s", got)
	}
}
