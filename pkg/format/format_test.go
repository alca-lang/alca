// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package format

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alca-lang/alca/pkg/lexer"
	"github.com/alca-lang/alca/pkg/parser"
)

// repoRoot returns the repository root relative to this package's directory.
func repoRoot(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..")
}

// programText parses src and returns the AST rendering, failing the test on
// syntax errors.
func programText(t *testing.T, name, src string) string {
	t.Helper()
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if len(p.SyntaxErrors()) > 0 {
		t.Fatalf("%s: unexpected syntax errors:\n%s", name, p.DiagnosticReport(name, src, false))
	}
	return prog.String()
}

// shippedValidFiles lists every .al file that must parse (excluding the
// intentionally invalid fixtures).
func shippedValidFiles(t *testing.T) []string {
	t.Helper()
	root := repoRoot(t)
	var files []string
	for _, dir := range []string{"examples", "benchmarks", "tests"} {
		base := filepath.Join(root, dir)
		if _, err := os.Stat(base); err != nil {
			continue
		}
		err := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".al") {
				return nil
			}
			if strings.Contains(filepath.ToSlash(path), "/invalid/") {
				return nil
			}
			files = append(files, path)
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", base, err)
		}
	}
	if len(files) == 0 {
		t.Fatal("no shipped .al files found")
	}
	return files
}

// TestFormatPreservesShippedPrograms formats every shipped source file and
// verifies that the formatted text parses to exactly the same program, that
// formatting is idempotent, and that no comment text is lost.
func TestFormatPreservesShippedPrograms(t *testing.T) {
	for _, path := range shippedValidFiles(t) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			src := string(data)

			formatted, err := FormatSource(path, src)
			if err != nil {
				t.Fatalf("FormatSource(%s) failed: %v", path, err)
			}
			if got, want := programText(t, path, formatted), programText(t, path, src); got != want {
				t.Fatalf("%s: formatted program differs from the original", path)
			}

			twice, err := FormatSource(path, formatted)
			if err != nil {
				t.Fatalf("re-formatting %s failed: %v", path, err)
			}
			if twice != formatted {
				t.Fatalf("%s: formatting is not idempotent", path)
			}

			outLines := map[string]bool{}
			for _, ln := range strings.Split(formatted, "\n") {
				outLines[strings.TrimSpace(ln)] = true
			}
			for _, ln := range strings.Split(src, "\n") {
				trimmed := strings.TrimSpace(ln)
				if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*") {
					if !outLines[trimmed] {
						t.Errorf("%s: comment line %q was not preserved", path, trimmed)
					}
				}
			}
		})
	}
}

// TestFormatCanonicalSpacing pins down the canonical style of the formatter.
func TestFormatCanonicalSpacing(t *testing.T) {
	input := "func  f( a: int,b: int)->int:\n" +
		"\treturn a+b;\n" +
		"x:=f(1,2);\n" +
		"if x==3:\n" +
		"\t\tprintln( \"ok\" );\n" +
		"vals:list[int]=[1,2, 3];\n" +
		"m := { \"a\":1 , \"b\" : 2 };\n" +
		"for  v  in  vals:\n" +
		"\tm[\"a\"]=v+1;\n" +
		"n := -5;\n" +
		"r := 1..10;\n" +
		"b: bool = not (n==0);\n" +
		"err(\"boom\", code=7);\n"
	want := "func f(a: int, b: int) -> int:\n" +
		"    return a + b;\n" +
		"x := f(1, 2);\n" +
		"if x == 3:\n" +
		"    println(\"ok\");\n" +
		"vals: list[int] = [1, 2, 3];\n" +
		"m := {\"a\": 1, \"b\": 2};\n" +
		"for v in vals:\n" +
		"    m[\"a\"] = v + 1;\n" +
		"n := -5;\n" +
		"r := 1..10;\n" +
		"b: bool = not (n == 0);\n" +
		"err(\"boom\", code=7);\n"

	got, err := Format(input)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}
	if got != want {
		t.Fatalf("unexpected formatting:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// TestFormatPreservesRawTextOfStringsAndComments checks that everything that
// lives inside string literals or comments survives byte-for-byte.
func TestFormatPreservesRawTextOfStringsAndComments(t *testing.T) {
	input := "func f():\n" +
		"    html := \"\"\"\n" +
		"  <div>\n" +
		"    spaced  // not a comment\n" +
		"  </div>\n" +
		"\"\"\";\n" +
		"    println(html);\n" +
		"    pat := `a  // b /* c */`;\n" +
		"    println(pat); // trailing comment\n"
	got, err := Format(input)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}
	for _, frag := range []string{
		"\"\"\"\n  <div>\n    spaced  // not a comment\n  </div>\n\"\"\";",
		"`a  // b /* c */`",
		"// trailing comment",
	} {
		if !strings.Contains(got, frag) {
			t.Errorf("formatted output lost %q:\n%s", frag, got)
		}
	}
	if _, err := Format(got); err != nil {
		t.Fatalf("re-formatting failed: %v", err)
	}
}

// TestFormatRejectsInvalidSyntax checks that invalid fixtures report a
// SyntaxError instead of producing output.
func TestFormatRejectsInvalidSyntax(t *testing.T) {
	for _, name := range []string{"parse_missing_semicolon.al", "parse_missing_colon.al"} {
		path := filepath.Join(repoRoot(t), "tests", "invalid", name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		out, err := FormatSource(path, string(data))
		if err == nil {
			t.Fatalf("%s: expected a formatting error, got output:\n%s", name, out)
		}
		if _, ok := err.(*SyntaxError); !ok {
			t.Fatalf("%s: expected *SyntaxError, got %T: %v", name, err, err)
		}
	}
}

// TestFormatTypeInvalidFilesStillFormat checks that files which are valid
// syntax but fail the type checker are formatted normally.
func TestFormatTypeInvalidFilesStillFormat(t *testing.T) {
	for _, name := range []string{"type_bad_assignment.al", "type_bad_return.al", "type_bad_argument.al"} {
		path := filepath.Join(repoRoot(t), "tests", "invalid", name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := FormatSource(path, string(data)); err != nil {
			t.Fatalf("%s: expected successful formatting, got %v", name, err)
		}
	}
}

// TestFormatKeepsProgramWithMultilineStringInBlock verifies that re-indenting
// the statement opening a triple-quoted string does not touch the string body.
func TestFormatKeepsProgramWithMultilineStringInBlock(t *testing.T) {
	input := "func f():\n" +
		"\ts := \"\"\"\n" +
		"line one\n" +
		"  line two\n" +
		"\"\"\";\n" +
		"\treturn s;\n"
	got, err := Format(input)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}
	want := "func f():\n" +
		"    s := \"\"\"\n" +
		"line one\n" +
		"  line two\n" +
		"\"\"\";\n" +
		"    return s;\n"
	if got != want {
		t.Fatalf("unexpected formatting:\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}
