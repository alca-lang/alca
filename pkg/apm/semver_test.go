// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package apm

import "testing"

func TestParseVersionAndCompare(t *testing.T) {
	ordered := []string{"1.0.0", "1.0.1", "1.1.0-alpha", "1.1.0-alpha.1", "1.1.0-beta", "1.1.0", "2.0.0"}
	for i := 0; i < len(ordered)-1; i++ {
		a, err := ParseVersion(ordered[i])
		if err != nil {
			t.Fatalf("ParseVersion(%q): %v", ordered[i], err)
		}
		b, err := ParseVersion(ordered[i+1])
		if err != nil {
			t.Fatalf("ParseVersion(%q): %v", ordered[i+1], err)
		}
		if a.Compare(b) >= 0 {
			t.Errorf("%s should sort before %s", ordered[i], ordered[i+1])
		}
		if b.Compare(a) <= 0 {
			t.Errorf("%s should sort after %s", ordered[i+1], ordered[i])
		}
	}
	for _, bad := range []string{"", "x", "1.x.0", "1..0", "-1.0.0", "1.0.0-"} {
		if _, err := ParseVersion(bad); err == nil {
			t.Errorf("ParseVersion(%q) should fail", bad)
		}
	}
}

func TestPartialVersions(t *testing.T) {
	v, err := ParseVersion("1.2")
	if err != nil || v.String() != "1.2.0" {
		t.Fatalf("ParseVersion(1.2) = %v, %v", v, err)
	}
	v, err = ParseVersion("3")
	if err != nil || v.String() != "3.0.0" {
		t.Fatalf("ParseVersion(3) = %v, %v", v, err)
	}
}

func TestConstraints(t *testing.T) {
	type check struct {
		constraint string
		version    string
		want       bool
	}
	checks := []check{
		{"", "0.0.1", true},
		{"*", "9.9.9", true},
		{"1.2.3", "1.2.3", true},
		{"=1.2.3", "1.2.3", true},
		{"1.2.3", "1.2.4", false},
		{"^1.2.3", "1.9.0", true},
		{"^1.2.3", "1.2.2", false},
		{"^1.2.3", "2.0.0", false},
		{"^0.2.3", "0.2.9", true},
		{"^0.2.3", "0.3.0", false},
		{"^0.0.3", "0.0.3", true},
		{"^0.0.3", "0.0.4", false},
		{"~1.2.3", "1.2.9", true},
		{"~1.2.3", "1.3.0", false},
		{">=1.0.0 <2.0.0", "1.5.0", true},
		{">=1.0.0 <2.0.0", "2.0.0", false},
		{">=1.0.0, <2.0.0", "1.0.0", true},
		{">1.0.0", "1.0.0", false},
		{"<=1.0.0", "1.0.0", true},
		{"<=1.0.0", "1.0.1", false},
		{"^1.0.0-alpha", "1.0.0-beta", true},
	}
	for _, c := range checks {
		constraint, err := ParseConstraint(c.constraint)
		if err != nil {
			t.Errorf("ParseConstraint(%q): %v", c.constraint, err)
			continue
		}
		v, err := ParseVersion(c.version)
		if err != nil {
			t.Fatal(err)
		}
		if got := constraint.Satisfies(v); got != c.want {
			t.Errorf("%q.Satisfies(%s) = %v, want %v", c.constraint, c.version, got, c.want)
		}
	}
	for _, bad := range []string{">=", "^", ">=", "=abc"} {
		if _, err := ParseConstraint(bad); err == nil {
			t.Errorf("ParseConstraint(%q) should fail", bad)
		}
	}
}
