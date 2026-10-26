// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package registry_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alca-lang/alca/pkg/apm"
	"github.com/alca-lang/alca/pkg/registry"
)

// openNode starts a node with self-service registration enabled.
func openNode(t *testing.T, tokens []registry.Token, mutate func(*registry.Config)) *httptest.Server {
	t.Helper()
	ts, _ := startNode(t, tokens, func(c *registry.Config) {
		c.AllowRegistration = true
		if mutate != nil {
			mutate(c)
		}
	})
	return ts
}

// registerAccount creates an account through the public client.
func registerAccount(t *testing.T, ts *httptest.Server, username, email string) apm.AuthResponse {
	t.Helper()
	res, err := apm.NewClient(ts.URL, apm.ClientOptions{}).Register(username, email, "supersecret")
	if err != nil {
		t.Fatalf("register %s: %v", username, err)
	}
	if res.Token == "" {
		t.Fatalf("register %s returned no token", username)
	}
	return *res
}

func TestAccountsRegisterPublishLogout(t *testing.T) {
	t.Setenv(apm.EnvHome, t.TempDir())
	ts := openNode(t, nil, nil)

	res := registerAccount(t, ts, "ann", "ann@example.com")
	if res.Username != "ann" || res.Email != "ann@example.com" {
		t.Errorf("register response = %+v", res)
	}
	if res.Message == "" {
		t.Errorf("register response should tell the user to store the token")
	}
	if !strings.Contains(res.Message, "not shown") {
		t.Errorf("register message = %q", res.Message)
	}

	client := apm.NewClient(ts.URL, apm.ClientOptions{Token: res.Token})
	acct, err := client.Account()
	if err != nil {
		t.Fatalf("Account: %v", err)
	}
	if acct.Username != "ann" || acct.Email != "ann@example.com" || len(acct.Tokens) != 1 {
		t.Errorf("account = %+v", acct)
	}
	if !strings.HasPrefix(res.Token, acct.Tokens[0].Prefix) {
		t.Errorf("prefix %q does not identify the token", acct.Tokens[0].Prefix)
	}
	if len(acct.Tokens[0].Prefix) >= len(res.Token) {
		t.Errorf("the account response must only expose a token prefix")
	}

	// An account token publishes.
	lib := writeLib(t, "annlib", "1.0.0", "func f() -> int:\n    return 1;\n", "")
	if _, err := apm.Publish(apm.PublishOptions{Dir: lib, RegistryURL: ts.URL, Token: res.Token, Out: io.Discard}); err != nil {
		t.Fatalf("publish with an account token: %v", err)
	}

	// Login issues a fresh one-time token that also publishes.
	login, err := apm.NewClient(ts.URL, apm.ClientOptions{}).Login("ann", "supersecret")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if login.Token == res.Token {
		t.Errorf("login must issue a new token, not re-show the old one")
	}
	second := writeLib(t, "annlib2", "1.0.0", "func f() -> int:\n    return 2;\n", "")
	if _, err := apm.Publish(apm.PublishOptions{Dir: second, RegistryURL: ts.URL, Token: login.Token, Out: io.Discard}); err != nil {
		t.Fatalf("publish with the login token: %v", err)
	}
	if _, err := apm.NewClient(ts.URL, apm.ClientOptions{}).Login("ann", "wrong-password"); err == nil {
		t.Errorf("login with a wrong password must fail")
	}

	// Logout revokes only the presented token.
	if err := client.Logout(); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	next := writeLib(t, "annlib", "1.1.0", "func f() -> int:\n    return 5;\n", "")
	if _, err := apm.Publish(apm.PublishOptions{Dir: next, RegistryURL: ts.URL, Token: res.Token, Out: io.Discard}); err == nil {
		t.Errorf("a revoked token must not publish")
	}
	if _, err := apm.Publish(apm.PublishOptions{Dir: next, RegistryURL: ts.URL, Token: login.Token, Out: io.Discard}); err != nil {
		t.Errorf("other tokens must keep working: %v", err)
	}
}

func TestRegistrationClosedAndReadOnly(t *testing.T) {
	closed, _ := startNode(t, publisher(t), nil)
	if _, err := apm.NewClient(closed.URL, apm.ClientOptions{}).Register("ann", "ann@example.com", "supersecret"); err == nil || !strings.Contains(err.Error(), "registration is disabled") {
		t.Errorf("registration on a closed node: %v", err)
	}

	readonly := openNode(t, nil, func(c *registry.Config) { c.ReadOnly = true })
	if _, err := apm.NewClient(readonly.URL, apm.ClientOptions{}).Register("ann", "ann@example.com", "supersecret"); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Errorf("registration on a read-only node: %v", err)
	}
	if _, err := apm.NewClient(readonly.URL, apm.ClientOptions{}).Login("ann", "supersecret"); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Errorf("login on a read-only node: %v", err)
	}
}

func TestRegistrationValidation(t *testing.T) {
	ts := openNode(t, nil, nil)
	client := apm.NewClient(ts.URL, apm.ClientOptions{})

	if _, err := client.Register("ab", "bob@example.com", "supersecret"); err == nil {
		t.Errorf("short username must be refused")
	}
	if _, err := client.Register("bob", "not-an-email", "supersecret"); err == nil {
		t.Errorf("invalid email must be refused")
	}
	if _, err := client.Register("bob", "bob@example.com", "short"); err == nil {
		t.Errorf("short password must be refused")
	}

	registerAccount(t, ts, "ann", "ann@example.com")
	if _, err := client.Register("ANN", "other@example.com", "supersecret"); err == nil || !strings.Contains(err.Error(), "already taken") {
		t.Errorf("duplicate username: %v", err)
	}
	if _, err := client.Register("bob", "ANN@example.com", "supersecret"); err == nil || !strings.Contains(err.Error(), "already registered") {
		t.Errorf("duplicate email: %v", err)
	}
}

func TestAccountTokenCannotBeUsedAsOperator(t *testing.T) {
	ts := openNode(t, publisher(t), nil)
	acct := registerAccount(t, ts, "ann", "ann@example.com")

	// The account is not a node operator.
	got, err := apm.NewClient(ts.URL, apm.ClientOptions{Token: acct.Token}).Account()
	if err != nil {
		t.Fatalf("Account: %v", err)
	}
	if got.Operator {
		t.Errorf("an account token must not report itself as a node operator")
	}
	if len(got.Tokens) != 1 {
		t.Errorf("account tokens = %d, want 1", len(got.Tokens))
	}

	// Operator tokens live in the server config and cannot be revoked remotely.
	op := apm.NewClient(ts.URL, apm.ClientOptions{Token: testToken})
	opAcct, err := op.Account()
	if err != nil {
		t.Fatalf("operator Account: %v", err)
	}
	if !opAcct.Operator {
		t.Errorf("a configured node token should act as an operator")
	}
	if err := op.Logout(); err == nil || !strings.Contains(err.Error(), "cannot be revoked") {
		t.Errorf("operator logout: %v", err)
	}
}

func TestPackageOwnership(t *testing.T) {
	ts := openNode(t, publisher(t), nil)
	ann := registerAccount(t, ts, "ann", "ann@example.com")
	bob := registerAccount(t, ts, "bob", "bob@example.com")

	annLib := writeLib(t, "shared", "1.0.0", "func f() -> int:\n    return 1;\n", "")
	if _, err := apm.Publish(apm.PublishOptions{Dir: annLib, RegistryURL: ts.URL, Token: ann.Token, Out: io.Discard}); err != nil {
		t.Fatalf("ann publishes: %v", err)
	}

	// Another account cannot publish under someone else's package name.
	takeover := writeLib(t, "shared", "2.0.0", "func f() -> int:\n    return 2;\n", "")
	if _, err := apm.Publish(apm.PublishOptions{Dir: takeover, RegistryURL: ts.URL, Token: bob.Token, Out: io.Discard}); err == nil || !strings.Contains(err.Error(), "owned by ann") {
		t.Errorf("bob publishing ann's package: %v", err)
	}

	// A package of its own is fine.
	own := writeLib(t, "boblib", "1.0.0", "func f() -> int:\n    return 3;\n", "")
	if _, err := apm.Publish(apm.PublishOptions{Dir: own, RegistryURL: ts.URL, Token: bob.Token, Out: io.Discard}); err != nil {
		t.Errorf("bob publishes his own package: %v", err)
	}

	// The owner can publish further versions.
	next := writeLib(t, "shared", "1.1.0", "func f() -> int:\n    return 4;\n", "")
	if _, err := apm.Publish(apm.PublishOptions{Dir: next, RegistryURL: ts.URL, Token: ann.Token, Out: io.Discard}); err != nil {
		t.Errorf("ann publishes a new version: %v", err)
	}

	// A node operator token can publish anywhere.
	if _, err := apm.Publish(apm.PublishOptions{Dir: takeover, RegistryURL: ts.URL, Token: testToken, Out: io.Discard}); err != nil {
		t.Errorf("operator publishes shared 2.0.0: %v", err)
	}
}

func TestAccountEndpointRequiresToken(t *testing.T) {
	ts := openNode(t, nil, nil)
	resp, err := http.Get(ts.URL + apm.APIBasePath + "/account")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}
