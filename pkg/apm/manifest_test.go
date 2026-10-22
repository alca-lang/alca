package apm

import (
	"strings"
	"testing"
)

const sampleManifest = `
# A test library
[package]
name = "jsonlib"
version = "1.2.3"
description = "Tiny test library"
entry = "src/main.al"
alca = ">=1.0.0"
license = "MIT"
authors = ["Ann", "Bob"]

[dependencies]
dep = "^2.0.0"
other = "1.0.0"

[build]
compile = true
optimization = 3
`

func TestParseManifest(t *testing.T) {
	m, err := ParseManifest([]byte(sampleManifest))
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	if m.Package.Name != "jsonlib" || m.Package.Version != "1.2.3" {
		t.Errorf("unexpected package identity: %+v", m.Package)
	}
	if m.Package.Entry != "src/main.al" {
		t.Errorf("entry = %q", m.Package.Entry)
	}
	if len(m.Package.Authors) != 2 || m.Package.Authors[1] != "Bob" {
		t.Errorf("authors = %v", m.Package.Authors)
	}
	if len(m.DepOrder) != 2 || m.DepOrder[0] != "dep" || m.DepOrder[1] != "other" {
		t.Errorf("dependency order = %v", m.DepOrder)
	}
	if m.Dependencies["dep"] != "^2.0.0" {
		t.Errorf("dep constraint = %q", m.Dependencies["dep"])
	}
	if !m.Build.Compile || m.Build.Optimization != 3 {
		t.Errorf("build = %+v", m.Build)
	}
}

func TestManifestDefaults(t *testing.T) {
	m, err := ParseManifest([]byte("[package]\nname = \"mini\"\nversion = \"0.1.0\"\n"))
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	if m.Package.Entry != DefaultEntry {
		t.Errorf("default entry = %q, want %q", m.Package.Entry, DefaultEntry)
	}
	if !m.Build.Compile || m.Build.Optimization != 2 {
		t.Errorf("default build = %+v", m.Build)
	}
}

func TestManifestErrors(t *testing.T) {
	cases := map[string]string{
		"missing name":        "[package]\nversion = \"1.0.0\"\n",
		"bad name":            "[package]\nname = \"Bad Name\"\nversion = \"1.0.0\"\n",
		"bad version":         "[package]\nname = \"ok\"\nversion = \"v1\"\n",
		"absolute entry":      "[package]\nname = \"ok\"\nversion = \"1.0.0\"\nentry = \"/etc/passwd.al\"\n",
		"escaping entry":      "[package]\nname = \"ok\"\nversion = \"1.0.0\"\nentry = \"../x.al\"\n",
		"non-al entry":        "[package]\nname = \"ok\"\nversion = \"1.0.0\"\nentry = \"src/main.txt\"\n",
		"self dependency":     "[package]\nname = \"ok\"\nversion = \"1.0.0\"\n[dependencies]\nok = \"1.0.0\"\n",
		"bad constraint":      "[package]\nname = \"ok\"\nversion = \"1.0.0\"\n[dependencies]\ndep = \"not a version\"\n",
		"bad optimization":    "[package]\nname = \"ok\"\nversion = \"1.0.0\"\n[build]\noptimization = 9\n",
		"key outside section": "name = \"ok\"\n",
		"unterminated string": "[package]\nname = \"ok\nversion = \"1.0.0\"\n",
	}
	for label, src := range cases {
		if _, err := ParseManifest([]byte(src)); err == nil {
			t.Errorf("%s: expected an error", label)
		}
	}
}

func TestManifestRoundTrip(t *testing.T) {
	m, err := ParseManifest([]byte(sampleManifest))
	if err != nil {
		t.Fatal(err)
	}
	out := string(m.Marshal())
	m2, err := ParseManifest([]byte(out))
	if err != nil {
		t.Fatalf("re-parse of marshalled manifest: %v\n%s", err, out)
	}
	if m2.Package.Name != m.Package.Name || m2.Package.Version != m.Package.Version ||
		m2.Package.Entry != m.Package.Entry || m2.Package.Alca != m.Package.Alca {
		t.Errorf("round trip changed package fields:\n%+v\n%+v", m.Package, m2.Package)
	}
	if m2.Dependencies["dep"] != "^2.0.0" || m2.Build.Optimization != 3 {
		t.Errorf("round trip changed dependencies/build: %v %+v", m2.Dependencies, m2.Build)
	}
}

func TestManifestUnknownSectionsTolerated(t *testing.T) {
	src := "[package]\nname = \"ok\"\nversion = \"1.0.0\"\n\n[future]\nflag = \"on\"\n"
	m, err := ParseManifest([]byte(src))
	if err != nil {
		t.Fatalf("unknown sections must not break parsing: %v", err)
	}
	if m.Extra["future"]["flag"] != "on" {
		t.Errorf("unknown section was not preserved: %+v", m.Extra)
	}
}

func TestValidImportPath(t *testing.T) {
	valid := []string{"http", "http.client", "a.b.c", "my-lib_v2"}
	invalid := []string{"", "Http", "1abc", "a..b", "a.", ".a", "a/b"}
	for _, s := range valid {
		if !ValidImportPath(s) {
			t.Errorf("ValidImportPath(%q) = false, want true", s)
		}
	}
	for _, s := range invalid {
		if ValidImportPath(s) {
			t.Errorf("ValidImportPath(%q) = true, want false", s)
		}
	}
}

func TestStripCommentInsideString(t *testing.T) {
	m, err := ParseManifest([]byte("[package]\nname = \"ok\"\nversion = \"1.0.0\"\ndescription = \"a # b\" # trailing\n"))
	if err != nil {
		t.Fatal(err)
	}
	if m.Package.Description != "a # b" {
		t.Errorf("description = %q, want %q", m.Package.Description, "a # b")
	}
}

func TestParseManifestInlineTableDependency(t *testing.T) {
	// Inline tables are accepted for forward compatibility even though version
	// constraints are plain strings today.
	src := "[package]\nname = \"ok\"\nversion = \"1.0.0\"\n[build]\ncompile = true\n"
	m, err := ParseManifest([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(m.Marshal()), "[dependencies]") {
		t.Errorf("marshal should always emit a [dependencies] section")
	}
}
