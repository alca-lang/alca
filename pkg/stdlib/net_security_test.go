// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package stdlib

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/alca-lang/alca/pkg/sandbox"
)

// TestHTTPRedirectChecksCapabilityPolicy verifies that an allowed URL cannot
// bounce a request to a host or port that the policy does not grant.
func TestHTTPRedirectChecksCapabilityPolicy(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("target"))
	}))
	defer target.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer redirector.Close()

	previous := sandbox.CurrentPolicy()
	defer sandbox.SetPolicy(previous)
	sandbox.SetPolicy(sandbox.Policy{
		Net: sandbox.Capability{Roots: []string{strings.TrimPrefix(redirector.URL, "http://")}},
	})

	if _, _, err := HTTPGet(redirector.URL); err == nil {
		t.Fatal("HTTPGet followed a redirect to a destination outside the policy")
	} else if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("redirect denial = %v, want a permission error", err)
	}

	// The same request must succeed when both endpoints are granted, proving
	// the first failure came from the redirect check rather than a broken test.
	sandbox.ResetPolicy()
	body, status, err := HTTPGet(redirector.URL)
	if err != nil {
		t.Fatalf("allowed redirect failed: %v", err)
	}
	if status != http.StatusOK || body != "target" {
		t.Fatalf("redirect response = %d %q, want 200 %q", status, body, "target")
	}
}
