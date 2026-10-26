// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package apm

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCredentialsRoundTrip(t *testing.T) {
	home := t.TempDir()
	creds, err := LoadCredentials(home)
	if err != nil {
		t.Fatalf("LoadCredentials: %v", err)
	}
	if _, ok := creds.Get("http://registry.example.com"); ok {
		t.Errorf("an empty store must not report entries")
	}

	// The key is normalized: scheme/host case and a trailing slash do not matter.
	if err := creds.SetWithPassword("http://Registry.Example.com:9777/", "ann", "top_secret_pw", "alca_secret_token"); err != nil {
		t.Fatalf("SetWithPassword: %v", err)
	}
	got, ok := creds.Get("http://registry.example.com:9777")
	if !ok || got.Username != "ann" || got.Password != "top_secret_pw" || got.Token != "alca_secret_token" {
		t.Fatalf("Get = %+v ok=%v", got, ok)
	}

	// Verify that the file on disk is encrypted and does NOT contain plaintext password or token
	diskBytes, err := os.ReadFile(creds.Path())
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	diskStr := string(diskBytes)
	if !strings.Contains(diskStr, `"encrypted": true`) {
		t.Fatalf("expected encrypted: true in %s", diskStr)
	}
	if strings.Contains(diskStr, "top_secret_pw") {
		t.Fatalf("disk file contains plaintext password!")
	}
	if strings.Contains(diskStr, "alca_secret_token") {
		t.Fatalf("disk file contains plaintext token!")
	}

	// Reopen with LoadCredentials and verify decryption
	reopened, err := LoadCredentials(home)
	if err != nil {
		t.Fatal(err)
	}
	got2, ok := reopened.Get("http://registry.example.com:9777")
	if !ok || got2.Username != "ann" || got2.Password != "top_secret_pw" || got2.Token != "alca_secret_token" {
		t.Fatalf("reopened Get = %+v ok=%v", got2, ok)
	}

	if err := reopened.Delete("http://REGISTRY.example.com:9777"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, ok := reopened.Get("http://registry.example.com:9777"); ok {
		t.Errorf("Delete must forget the entry")
	}
	if err := reopened.Delete("http://never.example.com"); err != nil {
		t.Errorf("deleting an unknown registry must be a no-op: %v", err)
	}
}

func TestCredentialsLegacyV1BackwardsCompatibility(t *testing.T) {
	home := t.TempDir()
	legacyJSON := `{
  "format_version": 1,
  "registries": {
    "https://packages.example.com": {
      "username": "legacy_user",
      "token": "legacy_token"
    }
  }
}`
	if err := os.WriteFile(filepath.Join(home, CredentialsFile), []byte(legacyJSON), 0o600); err != nil {
		t.Fatal(err)
	}

	creds, err := LoadCredentials(home)
	if err != nil {
		t.Fatalf("LoadCredentials on legacy: %v", err)
	}
	entry, ok := creds.Get("https://packages.example.com")
	if !ok || entry.Username != "legacy_user" || entry.Token != "legacy_token" {
		t.Fatalf("unexpected legacy entry: %+v ok=%v", entry, ok)
	}

	// Saving should automatically upgrade to encrypted format v2
	if err := creds.Set("https://packages.example.com", "legacy_user", "updated_token"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	raw, err := os.ReadFile(creds.Path())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"encrypted": true`) {
		t.Fatalf("expected upgraded encrypted format")
	}
}

func TestCredentialsSetValidation(t *testing.T) {
	creds, err := LoadCredentials(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := creds.Set("", "ann", "alca_x"); err == nil {
		t.Errorf("an empty registry URL must be refused")
	}
	if err := creds.Set("https://packages.example.com", "ann", "  "); err == nil {
		t.Errorf("an empty token must be refused")
	}
}

func TestCredentialsFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions are not enforced on Windows")
	}
	home := t.TempDir()
	creds, err := LoadCredentials(home)
	if err != nil {
		t.Fatal(err)
	}
	if err := creds.Set("https://packages.example.com", "ann", "alca_x"); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(creds.Path())
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Errorf("credentials permissions = %o, want 600", perm)
	}
}

func TestLoadCredentialsCorrupt(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, CredentialsFile), []byte("{oops"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCredentials(home); err == nil {
		t.Errorf("a corrupt credential store must be reported")
	}
}

func TestNormalizeRegistryURL(t *testing.T) {
	cases := map[string]string{
		"":                              "",
		"   ":                           "",
		"HTTPS://Packages.Example.COM":  "https://packages.example.com",
		"https://packages.example.com/": "https://packages.example.com",
	}
	for in, want := range cases {
		if got := NormalizeRegistryURL(in); got != want {
			t.Errorf("NormalizeRegistryURL(%q) = %q, want %q", in, got, want)
		}
	}
}
