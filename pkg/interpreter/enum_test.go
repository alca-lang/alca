// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package interpreter

import (
	"strings"
	"testing"

	"github.com/alca-lang/alca/pkg/lexer"
	"github.com/alca-lang/alca/pkg/parser"
)

func TestEnumEvaluation(t *testing.T) {
	src := `
enum Status {
    Pending,
    Active = 10,
    Done,
}

s1 := Status.Pending;
s2 := Status.Active;
s3 := Status.Done;

match s2 {
    case Status.Pending:
        println("pending");
    case Status.Active:
        println("active");
    else:
        println("other");
}

println(s1 == Status.Pending);
println(s1 == Status.Active);
println(s2.tag);
`
	l := lexer.New(src)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	var out strings.Builder
	SetPrintHook(func(s string) { out.WriteString(s + "\n") })
	defer ResetPrintHook()

	interp := New()
	interp.Eval(prog)

	got := strings.TrimSpace(out.String())
	want := "active\ntrue\nfalse\n10"
	if got != want {
		t.Errorf("got output:\n%s\nwant:\n%s", got, want)
	}
}
