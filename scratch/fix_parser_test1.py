p = r'C:\Users\geekom\Desktop\архив\Pawl\pkg\parser\parser_test.go'
s = open(p, encoding='utf-8', newline='').read()
s = s.replace('''func TestSemicolonIsMandatory(t *testing.T) {
	cases := map[string]string{
		"declaration":  "x: int = 5\\n",
		"expression":   "println(1)\\n",
		"return":       "func f() -> int {\\n    return 1\\n}\\n",
		"use":          "use std.math\\n",
		"struct field": "struct S {\\n    a: int\\n}\\n",
		"break":        "while true {\\n    break\\n}\\n",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			_, errs := parse(src)
			if len(errs) == 0 {
				t.Fatalf("expected a missing-';' error")
			}
			if !strings.Contains(strings.Join(errs, " "), "expected ';'") {
				t.Fatalf("wrong diagnostic: %v", errs)
			}
		})
	}
}

func TestSemicolonRecovery(t *testing.T) {
	prog, errs := parse("a: int = 1\\nb: int = 2\\nc: int = 3\\n")
	if len(errs) != 1 {
		t.Fatalf("want exactly 1 error (no cascade), got %v", errs)
	}
	if len(prog.Statements) != 3 {
		t.Fatalf("parser must recover and keep all 3 statements, got %d", len(prog.Statements))
	}
}
''', '''func TestSemicolonIsRejected(t *testing.T) {
	cases := map[string]string{
		"declaration":  "x: int = 5;\\n",
		"expression":   "println(1);\\n",
		"return":       "func f() -> int {\\n    return 1;\\n}\\n",
		"use":          "use std.math;\\n",
		"struct field": "struct S {\\n    a: int;\\n}\\n",
		"break":        "while true {\\n    break;\\n}\\n",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			_, errs := parse(src)
			if len(errs) == 0 {
				t.Fatalf("expected a semicolon-is-not-allowed error")
			}
			if !strings.Contains(strings.Join(errs, " "), "semicolons are not allowed") {
				t.Fatalf("wrong diagnostic: %v", errs)
			}
		})
	}
}

func TestNewlineTerminatedStatements(t *testing.T) {
	prog, errs := parse("a: int = 1\\nb: int = 2\\nc: int = 3\\n")
	if len(errs) != 0 {
		t.Fatalf("want no errors, got %v", errs)
	}
	if len(prog.Statements) != 3 {
		t.Fatalf("want 3 statements, got %d", len(prog.Statements))
	}
}
''')
open(p, 'w', encoding='utf-8', newline='').write(s)
print('done')
