// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func newTestAccountStore(t *testing.T) *AccountStore {
	t.Helper()
	store, err := NewAccountStore(filepath.Join(t.TempDir(), "accounts.json"))
	if err != nil {
		t.Fatalf("NewAccountStore: %v", err)
	}
	return store
}

func TestAccountStoreRegisterAndToken(t *testing.T) {
	store := newTestAccountStore(t)

	acct, token, err := store.Register("Ann", "ann@example.com", "supersecret")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if acct.Username != "ann" {
		t.Errorf("username = %q, want the canonical lowercase form", acct.Username)
	}
	if !strings.HasPrefix(token, TokenPrefix) || len(token) < 20 {
		t.Errorf("token = %q", token)
	}
	if len(acct.Tokens) != 1 || acct.Tokens[0].Label != "registration" {
		t.Errorf("tokens = %+v", acct.Tokens)
	}

	// The plaintext token resolves back to its account and record.
	found, tok, err := store.FindToken(token)
	if err != nil {
		t.Fatalf("FindToken: %v", err)
	}
	if found.Username != "ann" || tok.ID != acct.Tokens[0].ID {
		t.Errorf("FindToken = %+v %+v", found, tok)
	}
	if tok.Prefix == token || len(tok.Prefix) >= len(token) {
		t.Errorf("display prefix %q must be shorter than the token", tok.Prefix)
	}
	if _, _, err := store.FindToken("alca_not-a-real-token"); !errors.Is(err, ErrTokenNotFound) {
		t.Errorf("unknown token error = %v", err)
	}

	// Plaintext secrets never reach the file.
	data, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{token, "supersecret"} {
		if strings.Contains(string(data), secret) {
			t.Errorf("accounts.json leaks %q", secret)
		}
	}
	var doc AccountFile
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("accounts.json is not valid JSON: %v", err)
	}
	if doc.Accounts["ann"] == nil || doc.Accounts["ann"].Password.Algo != PasswordAlgo {
		t.Errorf("stored account = %+v", doc.Accounts["ann"])
	}
	if doc.Accounts["ann"].Password.Iterations != PasswordIterations {
		t.Errorf("iteration count = %d", doc.Accounts["ann"].Password.Iterations)
	}

	// A reopened store accepts the same token.
	reopened, err := NewAccountStore(store.Path())
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := reopened.FindToken(token); err != nil {
		t.Errorf("token must survive a restart: %v", err)
	}
	if reopened.Count() != 1 {
		t.Errorf("Count = %d, want 1", reopened.Count())
	}
}

func TestAccountStoreFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions are not enforced on Windows")
	}
	store := newTestAccountStore(t)
	if _, _, err := store.Register("ann", "ann@example.com", "supersecret"); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(store.Path())
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Errorf("accounts.json permissions = %o, want 600", perm)
	}
}

func TestAccountStoreDuplicatesAndValidation(t *testing.T) {
	store := newTestAccountStore(t)
	if _, _, err := store.Register("ann", "ann@example.com", "supersecret"); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if _, _, err := store.Register("ANN", "other@example.com", "supersecret"); !errors.Is(err, ErrUsernameTaken) {
		t.Errorf("duplicate username error = %v", err)
	}
	if _, _, err := store.Register("bob", "ANN@example.com", "supersecret"); !errors.Is(err, ErrEmailTaken) {
		t.Errorf("duplicate email error = %v", err)
	}

	cases := []struct {
		name              string
		user, email, pass string
	}{
		{"short username", "ab", "bob@example.com", "supersecret"},
		{"invalid username", "bob!", "bob@example.com", "supersecret"},
		{"invalid email", "bob", "not-an-email", "supersecret"},
		{"short password", "bob", "bob@example.com", "short"},
		{"blank password", "bob", "bob@example.com", "         "},
	}
	for _, c := range cases {
		if _, _, err := store.Register(c.user, c.email, c.pass); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%s: error = %v, want ErrInvalidInput", c.name, err)
		}
	}
}

func TestAccountStoreAuthenticateIssueRevoke(t *testing.T) {
	store := newTestAccountStore(t)
	_, registrationToken, err := store.Register("ann", "ann@example.com", "supersecret")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := store.Authenticate("ann", "wrong-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("wrong password error = %v", err)
	}
	if _, err := store.Authenticate("ghost", "supersecret"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("unknown user error = %v", err)
	}
	acct, err := store.Authenticate("Ann", "supersecret")
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}

	// Login issues an extra token; both work until one is revoked.
	loginToken, _, err := store.IssueToken(acct.Username, "login")
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}
	if loginToken == registrationToken {
		t.Errorf("IssueToken must return a fresh value")
	}
	snapshot, err := store.Snapshot("ann")
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Tokens) != 2 {
		t.Errorf("token count = %d, want 2", len(snapshot.Tokens))
	}

	if _, err := store.RevokeToken(loginToken); err != nil {
		t.Fatalf("RevokeToken: %v", err)
	}
	if _, _, err := store.FindToken(loginToken); !errors.Is(err, ErrTokenNotFound) {
		t.Errorf("revoked token error = %v", err)
	}
	if _, _, err := store.FindToken(registrationToken); err != nil {
		t.Errorf("other tokens must keep working: %v", err)
	}
	if _, err := store.RevokeToken("alca_nope"); !errors.Is(err, ErrTokenNotFound) {
		t.Errorf("revoking an unknown token error = %v", err)
	}

	// A failed password check never mutates the stored account.
	accountAfter, _ := store.Snapshot("ann")
	if len(accountAfter.Tokens) != 1 {
		t.Errorf("token count after revoke = %d, want 1", len(accountAfter.Tokens))
	}
}

func TestAccountStoreTouchToken(t *testing.T) {
	store := newTestAccountStore(t)
	_, token, err := store.Register("ann", "ann@example.com", "supersecret")
	if err != nil {
		t.Fatal(err)
	}
	acct, tok, err := store.FindToken(token)
	if err != nil {
		t.Fatal(err)
	}
	if tok.LastUsedAt != "" {
		t.Errorf("a fresh token should have no last-used timestamp")
	}
	if err := store.TouchToken(acct.Username, tok.ID); err != nil {
		t.Fatalf("TouchToken: %v", err)
	}
	_, touched, err := store.FindToken(token)
	if err != nil {
		t.Fatal(err)
	}
	if touched.LastUsedAt == "" {
		t.Errorf("last-used timestamp was not recorded")
	}
	if err := store.TouchToken("ghost", "id"); !errors.Is(err, ErrAccountNotFound) {
		t.Errorf("TouchToken for an unknown account error = %v", err)
	}
}

func TestAccountValidationHelpers(t *testing.T) {
	if got, err := CanonicalUsername("  Ann-1  "); err != nil || got != "ann-1" {
		t.Errorf("CanonicalUsername = %q, %v", got, err)
	}
	if _, err := CanonicalUsername("a"); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("one-character username error = %v", err)
	}
	if err := ValidateEmail("ann@example.com"); err != nil {
		t.Errorf("valid email rejected: %v", err)
	}
	for _, bad := range []string{"", "nope", "a@b", "a b@example.com"} {
		if err := ValidateEmail(bad); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("ValidateEmail(%q) error = %v", bad, err)
		}
	}
	if err := ValidatePassword("long enough"); err != nil {
		t.Errorf("valid password rejected: %v", err)
	}
	if err := ValidatePassword(strings.Repeat("x", 300)); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("overlong password error = %v", err)
	}
}

func TestAccountStoreRejectsNewerFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "accounts.json")
	if err := os.WriteFile(path, []byte(`{"format_version": 99, "accounts": {}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewAccountStore(path); err == nil {
		t.Errorf("a newer accounts format must be reported instead of silently used")
	}
}
