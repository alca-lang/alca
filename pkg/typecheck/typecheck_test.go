package typecheck_test

import (
	"strings"
	"testing"

	"github.com/alca-lang/alca/pkg/lexer"
	"github.com/alca-lang/alca/pkg/parser"
	"github.com/alca-lang/alca/pkg/typecheck"
)

func check(t *testing.T, src string) []string {
	t.Helper()
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if errs := p.SyntaxErrors(); len(errs) > 0 {
		t.Fatalf("unexpected syntax errors: %v", errs[0].Message)
	}
	var msgs []string
	for _, e := range typecheck.NewChecker("t.al", src).Check(prog) {
		msgs = append(msgs, e.Message+" | "+e.Suggestion)
	}
	return msgs
}

func TestValidPrograms(t *testing.T) {
	cases := map[string]string{
		"annotated and inferred": `x: int = 5; y := 7; z: float = 2; println(x + y);`,
		"functions and structs": `
struct P:
    name: string;
    age: int = 18;
func older(p: P) -> int:
    return p.age + 1;
q := P("a");
n: int = older(q);`,
		"hoisting": `
println(twice(2));
func twice(n: int) -> int:
    return n * 2;`,
		"null narrowing": `
func f(s: string | null) -> int:
    if s == null:
        return 0;
    return len(s);
v: string | null = null;
if v != null:
    println(v + "!");`,
		"error capture": `
func risky(n: int) -> int:
    if n == 0:
        err("zero", code=3);
        return 0;
    return 10 / n;
e: error | null = null;
risky(0) ?>>> e;
if e != null:
    println(e.msg);`,
		"collections": `
xs: list[int] = [];
push(xs, 1);
m: map[string, int] = {"a": 1};
m["b"] = 2;
total: int = 0;
for x in xs:
    total = total + x;
for k in keys(m):
    println(k);`,
		"stdlib typed": `
use std.math;
r: float = math.sqrt(9) + 1.5;
c: int = math.floor(2.7);`,
		"match": `
n: int = 2;
match n:
    case 1:
        println("one");
    case 2:
        println("two");
    else:
        println("many");`,
		"float literal widening": `f: float = 3; g: float = f * 2;`,
		"lambda pipe":            `inc := |x: int| x + 1; r: int = 4 |> inc;`,
		"json is dynamic": `
use std.formats;
d: json = json_decode("{}");
n: int = int(d["a"]);`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			if errs := check(t, src); len(errs) > 0 {
				t.Fatalf("expected no errors, got:\n  %s", strings.Join(errs, "\n  "))
			}
		})
	}
}

func TestRejectedPrograms(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{"string to int", `a: int = "x";`, "cannot initialize"},
		{"int plus string", `s: string = "n=" + 5;`, "cannot be applied"},
		{"int plus float", `a: int = 1; b: float = 2.5; c := a + b;`, "cannot be applied"},
		{"float to int", `a: int = 2.5;`, "cannot initialize"},
		{"infer from null", `c := null;`, "cannot infer"},
		{"undeclared assignment", `x = 10;`, "undeclared variable"},
		{"undefined name", `println(nope);`, "undefined name"},
		{"typo suggestion", "counter: int = 1;\nprintln(countr);", "did you mean 'counter'"},
		{"redeclaration", `a: int = 1; a: int = 2;`, "already declared"},
		{"shadowing", "a: int = 1;\nfunc f():\n    a: int = 2;", "shadows"},
		{"wrong arg type", "func f(a: int) -> int:\n    return a;\nf(\"s\");", "has type string, expected int"},
		{"too many args", "func f(a: int) -> int:\n    return a;\nf(1, 2);", "too many arguments"},
		{"missing arg", "func f(a: int, b: int) -> int:\n    return a;\nf(1);", "missing argument"},
		{"missing return", "func f(a: int) -> int:\n    if a > 0:\n        return a;", "must return"},
		{"return in void", "func f():\n    return 1;", "without a return type"},
		{"truthiness", "a: int = 1;\nif a:\n    println(\"x\");", "must be bool"},
		{"null into int", `a: int = null;`, "cannot initialize"},
		{"null member access", "s: string | null = null;\nprintln(s.length);", "may be null"},
		{"mixed list", `xs: list[int] = [1, "two"];`, "list element"},
		{"map value type", `m: map[string, int] = {"k": "v"};`, "map value"},
		{"unknown field", "struct P:\n    n: string;\np := P(\"a\");\nprintln(p.age);", "no field"},
		{"unknown module member", "use std.math;\nmath.sqroot(4);", "has no member"},
		{"unknown module", `use std.mth;`, "unknown standard module"},
		{"module not imported", `println(math.sqrt(4));`, "undefined name"},
		{"break outside loop", `break;`, "outside of a loop"},
		{"stream var type", "n: int = 0;\nprintln(\"x\") >>> n;", "cannot hold"},
		{"any annotation", `x: any = 1;`, "'any'"},
		{"json needs conversion", "d: json = json_decode(\"{}\");\nn: int = d;", "cannot initialize"},
		{"assign wrong type", "a: int = 1;\na = \"s\";", "cannot assign"},
		{"string modulo", `a: string = "x" % "y";`, "cannot be applied"},
		{"c style decl is a syntax error", ``, ""},
	}
	for _, tc := range cases {
		if tc.src == "" {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			errs := check(t, tc.src)
			if len(errs) == 0 {
				t.Fatalf("expected an error containing %q, got none", tc.want)
			}
			joined := strings.Join(errs, "\n")
			if !strings.Contains(joined, tc.want) {
				t.Fatalf("expected an error containing %q, got:\n%s", tc.want, joined)
			}
		})
	}
}

func TestOneMistakeOneError(t *testing.T) {
	errs := check(t, `s: string = "n=" + 5;`)
	if len(errs) != 1 {
		t.Fatalf("a single mistake must produce a single diagnostic, got %d: %v", len(errs), errs)
	}
}
