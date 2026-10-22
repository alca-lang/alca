package apm

import (
	"os"
	"path/filepath"
	"runtime"
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
	if err := creds.Set("http://Registry.Example.com:9777/", "ann", "alca_secret"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, ok := creds.Get("http://registry.example.com:9777")
	if !ok || got.Username != "ann" || got.Token != "alca_secret" {
		t.Fatalf("Get = %+v ok=%v", got, ok)
	}

	reopened, err := LoadCredentials(home)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := reopened.Get("http://registry.example.com:9777"); !ok {
		t.Errorf("credentials must persist across loads")
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
		"http://91.92.34.94:9777":       "http://91.92.34.94:9777",
		"http://91.92.34.94:9777/":      "http://91.92.34.94:9777",
		"not a url/":                    "not a url",
	}
	for in, want := range cases {
		if got := NormalizeRegistryURL(in); got != want {
			t.Errorf("NormalizeRegistryURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCredentialsPathUsesHome(t *testing.T) {
	home := t.TempDir()
	path, err := CredentialsPath(home)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(path) != home || filepath.Base(path) != CredentialsFile {
		t.Errorf("CredentialsPath = %q", path)
	}

	t.Setenv(EnvHome, home)
	fromEnv, err := CredentialsPath("")
	if err != nil {
		t.Fatal(err)
	}
	if fromEnv != path {
		t.Errorf("CredentialsPath with $%s = %q, want %q", EnvHome, fromEnv, path)
	}
}
