// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package stdlib

import (
	"net"
	"path/filepath"
	"testing"

	"github.com/alca-lang/alca/pkg/sandbox"
)

func TestSQLiteDSNPath(t *testing.T) {
	cases := []struct {
		dsn      string
		path     string
		readOnly bool
	}{
		{":memory:", "", false},
		{"file::memory:?cache=shared", "", false},
		{"file:test.db?mode=memory", "", false},
		{filepath.Join("data", "test.db"), filepath.Join("data", "test.db"), false},
		{"file:test.db", "test.db", false},
		{"file:test.db?mode=ro", "test.db", true},
		{"file:test.db?immutable=1", "test.db", true},
	}
	for _, tc := range cases {
		path, readOnly := sqliteDSNPath(tc.dsn)
		if path != tc.path || readOnly != tc.readOnly {
			t.Errorf("sqliteDSNPath(%q) = %q, %v; want %q, %v", tc.dsn, path, readOnly, tc.path, tc.readOnly)
		}
	}
}

func TestSQLiteDSNRequiresReadAndWrite(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "app.db")
	previous := sandbox.CurrentPolicy()
	defer sandbox.SetPolicy(previous)

	sandbox.SetPolicy(sandbox.Policy{Read: sandbox.Capability{Roots: []string{root}}})
	if err := requireSQLiteDSN(path); err == nil {
		t.Fatal("read-only grant must not open a writable SQLite database")
	}

	sandbox.SetPolicy(sandbox.Policy{
		Read:  sandbox.Capability{Roots: []string{root}},
		Write: sandbox.Capability{Roots: []string{root}},
	})
	if err := requireSQLiteDSN(path); err != nil {
		t.Fatalf("read+write grant was rejected: %v", err)
	}

	sandbox.SetPolicy(sandbox.Policy{Read: sandbox.Capability{Roots: []string{root}}})
	if err := requireSQLiteDSN("file:" + filepath.ToSlash(path) + "?mode=ro"); err != nil {
		t.Fatalf("read-only DSN with a read grant was rejected: %v", err)
	}
}

func TestPostgresDSNRequiresNetworkGrant(t *testing.T) {
	previous := sandbox.CurrentPolicy()
	defer sandbox.SetPolicy(previous)

	hostPort := net.JoinHostPort("db.example.com", "5432")
	sandbox.SetPolicy(sandbox.Policy{Net: sandbox.Capability{Roots: []string{hostPort}}})
	for _, dsn := range []string{
		"postgres://db.example.com/app",
		"postgresql://user:pass@db.example.com/app",
		"host=db.example.com port=5432 dbname=app",
	} {
		if err := requirePostgresDSN(dsn); err != nil {
			t.Errorf("requirePostgresDSN(%q) rejected an allowed host: %v", dsn, err)
		}
	}

	sandbox.SetPolicy(sandbox.Policy{Net: sandbox.Capability{Roots: []string{net.JoinHostPort("other.example.com", "5432")}}})
	if err := requirePostgresDSN("postgres://db.example.com/app"); err == nil {
		t.Fatal("postgres DSN outside the network policy was allowed")
	}
}
