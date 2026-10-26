// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package interpreter

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/alca-lang/alca/pkg/sandbox"
)

// TestStreamRedirectChecksCapabilityPolicy locks in the same redirect policy
// for stream writes that HTTP builtins use.
func TestStreamRedirectChecksCapabilityPolicy(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer redirector.Close()

	previous := sandbox.CurrentPolicy()
	defer sandbox.SetPolicy(previous)
	sandbox.SetPolicy(sandbox.Policy{
		Net: sandbox.Capability{Roots: []string{strings.TrimPrefix(redirector.URL, "http://")}},
	})

	if err := writeToNetwork(redirector.URL, "data"); err == nil {
		t.Fatal("stream write followed a redirect outside the policy")
	} else if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("redirect denial = %v, want a permission error", err)
	}

	sandbox.ResetPolicy()
	if err := writeToNetwork(redirector.URL, "data"); err != nil {
		t.Fatalf("allowed stream redirect failed: %v", err)
	}
}
