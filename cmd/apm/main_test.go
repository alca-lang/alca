// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/alca-lang/alca/pkg/apm"
	"github.com/alca-lang/alca/pkg/registry"
)

// runAPM executes the CLI with captured stdout/stderr.
func runAPM(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	oldOut, oldErr := os.Stdout, os.Stderr
	outR, outW, _ := os.Pipe()
	errR, errW, _ := os.Pipe()
	os.Stdout, os.Stderr = outW, errW
	outDone := make(chan string)
	errDone := make(chan string)
	go func() { b, _ := io.ReadAll(outR); outDone <- string(b) }()
	go func() { b, _ := io.ReadAll(errR); errDone <- string(b) }()

	code = run(args)

	outW.Close()
	errW.Close()
	os.Stdout, os.Stderr = oldOut, oldErr
	return <-outDone, <-errDone, code
}

func writeLocalLib(t *testing.T, name, version string) string {
	t.Helper()
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "alca.toml"),
		"[package]\nname = \""+name+"\"\nversion = \""+version+"\"\nentry = \"src/main.al\"\n\n[build]\ncompile = true\n")
	writeTestFile(t, filepath.Join(dir, "src", "main.al"),
		"func greet(n: string) -> string:\n    return \"hi \" + n;\n")
	return dir
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCLIInstallLocalShowRemove(t *testing.T) {
	home := t.TempDir()
	lib := writeLocalLib(t, "clilib", "2.3.4")

	out, errOut, code := runAPM(t, "install", "-u", lib, "--home", home)
	if code != 0 {
		t.Fatalf("install -u failed (%d): %s%s", code, out, errOut)
	}
	if !strings.Contains(out, "clilib") || !strings.Contains(out, "2.3.4") {
		t.Errorf("install output = %q", out)
	}
	if !strings.Contains(out, "compiled") {
		t.Errorf("install should mention the compiled build: %q", out)
	}

	out, errOut, code = runAPM(t, "show", "--home", home)
	if code != 0 || !strings.Contains(out, "clilib") {
		t.Fatalf("show failed (%d): %s%s", code, out, errOut)
	}

	out, errOut, code = runAPM(t, "show", "clilib", "--home", home)
	if code != 0 || !strings.Contains(out, "2.3.4") || !strings.Contains(out, runtime.GOOS+"/"+runtime.GOARCH) {
		t.Fatalf("show detail failed (%d): %s%s", code, out, errOut)
	}

	out, errOut, code = runAPM(t, "remove", "clilib", "--home", home)
	if code != 0 {
		t.Fatalf("remove failed (%d): %s%s", code, out, errOut)
	}
	out, _, code = runAPM(t, "show", "--home", home)
	if code != 0 || !strings.Contains(out, "No libraries") {
		t.Fatalf("show after remove: (%d) %q", code, out)
	}
}

func TestCLIInstallFromRegistry(t *testing.T) {
	home := t.TempDir()

	cfg := registry.DefaultConfig()
	cfg.DataDir = t.TempDir()
	cfg.Tokens = []registry.Token{{Token: "cli-token", Author: "tester", Scopes: []string{"publish"}}}
	srv, err := registry.New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	lib := writeLocalLib(t, "remotecli", "1.0.0")
	t.Setenv(apm.EnvHome, t.TempDir()) // publishing validates against an isolated home
	if _, err := apm.Publish(apm.PublishOptions{Dir: lib, RegistryURL: ts.URL, Token: "cli-token", Out: io.Discard}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	out, errOut, code := runAPM(t, "install", "remotecli", "--registry", ts.URL, "--home", home)
	if code != 0 {
		t.Fatalf("install from registry failed (%d): %s%s", code, out, errOut)
	}
	if !strings.Contains(out, "remotecli") || !strings.Contains(out, "1.0.0") {
		t.Errorf("install output = %q", out)
	}
	if strings.Contains(errOut, "error") {
		t.Errorf("unexpected stderr: %q", errOut)
	}

	// Versioned install of an unknown version fails with a clear message.
	_, errOut, code = runAPM(t, "install", "remotecli@9.9.9", "--registry", ts.URL, "--home", home)
	if code != 1 || !strings.Contains(errOut, "9.9.9") {
		t.Errorf("unknown version: code=%d stderr=%q", code, errOut)
	}
}

func TestCLIUsesBuiltInRegistry(t *testing.T) {
	defer func(old string) { apm.DefaultRegistry = old }(apm.DefaultRegistry)

	home := t.TempDir()
	t.Setenv(apm.EnvHome, t.TempDir()) // publishing validates against an isolated home
	t.Setenv(apm.EnvRegistry, "")      // prove the compiled-in node is what is used
	apm.DefaultRegistry = ""

	cfg := registry.DefaultConfig()
	cfg.DataDir = t.TempDir()
	cfg.Tokens = []registry.Token{{Token: "builtin-token", Author: "tester", Scopes: []string{"publish"}}}
	srv, err := registry.New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	lib := writeLocalLib(t, "builtincli", "0.5.0")
	if _, err := apm.Publish(apm.PublishOptions{Dir: lib, RegistryURL: ts.URL, Token: "builtin-token", Out: io.Discard}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	// A build with a stamped node installs from it without --registry.
	apm.DefaultRegistry = ts.URL
	out, errOut, code := runAPM(t, "install", "builtincli", "--home", home)
	if code != 0 {
		t.Fatalf("install via built-in registry failed (%d): %s%s", code, out, errOut)
	}
	if !strings.Contains(out, "builtincli") || !strings.Contains(out, "0.5.0") {
		t.Errorf("install output = %q", out)
	}

	// Help mentions the built-in node without revealing raw server IP.
	help, _, code := runAPM(t, "help")
	if code != 0 || !strings.Contains(help, "built in") {
		t.Errorf("help should mention the built-in registry (%d):\n%s", code, help)
	}

	// An explicit --registry still wins over the compiled-in value.
	other := httptest.NewServer(srv.Handler())
	defer other.Close()
	_, errOut, code = runAPM(t, "install", "builtincli", "--registry", other.URL, "--home", t.TempDir())
	if code != 0 {
		t.Fatalf("install with explicit --registry failed (%d): %s", code, errOut)
	}
}

func TestCLIAccountFlow(t *testing.T) {
	t.Setenv(apm.EnvEnablePublish, "1")
	home := t.TempDir()

	cfg := registry.DefaultConfig()
	cfg.DataDir = t.TempDir()
	cfg.AllowRegistration = true
	srv, err := registry.New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Registration prints the one-time token and saves it locally.
	out, errOut, code := runAPM(t, "register", "ann", "ann@example.com",
		"--password", "supersecret", "--registry", ts.URL, "--home", home)
	if code != 0 {
		t.Fatalf("register failed (%d): %s%s", code, out, errOut)
	}
	if !strings.Contains(out, "shown only once") || !strings.Contains(out, "alca_") {
		t.Errorf("register output = %q", out)
	}
	creds, err := apm.LoadCredentials(home)
	if err != nil {
		t.Fatal(err)
	}
	cred, ok := creds.Get(ts.URL)
	if !ok || cred.Username != "ann" || !strings.HasPrefix(cred.Token, "alca_") {
		t.Fatalf("saved credential = %+v ok=%v", cred, ok)
	}

	// The saved token identifies the account.
	out, errOut, code = runAPM(t, "whoami", "--registry", ts.URL, "--home", home)
	if code != 0 || !strings.Contains(out, "ann") || !strings.Contains(out, "ann@example.com") {
		t.Fatalf("whoami failed (%d): %s%s", code, out, errOut)
	}

	// Publishing without --token reuses the saved credential.
	lib := writeLocalLib(t, "cliauth", "1.0.0")
	out, errOut, code = runAPM(t, "publish", lib, "--registry", ts.URL, "--home", home)
	if code != 0 {
		t.Fatalf("publish with saved credentials failed (%d): %s%s", code, out, errOut)
	}
	if !strings.Contains(out, "Published cliauth 1.0.0") || !strings.Contains(out, "saved token of ann") {
		t.Errorf("publish output = %q", out)
	}

	// Login issues another token for the same account.
	out, errOut, code = runAPM(t, "login", "ann", "--password", "supersecret", "--registry", ts.URL, "--home", home)
	if code != 0 {
		t.Fatalf("login failed (%d): %s%s", code, out, errOut)
	}

	// Logout revokes the saved token and forgets it locally.
	out, errOut, code = runAPM(t, "logout", "--registry", ts.URL, "--home", home)
	if code != 0 {
		t.Fatalf("logout failed (%d): %s%s", code, out, errOut)
	}
	if _, errOut, code = runAPM(t, "whoami", "--registry", ts.URL, "--home", home); code == 0 {
		t.Errorf("whoami after logout must fail, got: %s", errOut)
	}
}

func TestCLIAccountErrors(t *testing.T) {
	home := t.TempDir()
	t.Setenv(apm.EnvRegistry, "")
	oldDefault := apm.DefaultRegistry
	apm.DefaultRegistry = ""
	t.Cleanup(func() { apm.DefaultRegistry = oldDefault })

	// Without a registry, account commands cannot know where to connect.
	if _, errOut, code := runAPM(t, "register", "ann", "ann@example.com", "--password", "supersecret", "--home", home); code == 0 {
		t.Errorf("register without a registry must fail: %s", errOut)
	}
	if _, errOut, code := runAPM(t, "whoami", "--home", home); code == 0 {
		t.Errorf("whoami without a registry must fail: %s", errOut)
	}

	// Too many positional arguments is a usage error.
	ts := httptest.NewServer(http.NotFoundHandler())
	defer ts.Close()
	if _, _, code := runAPM(t, "register", "a", "b", "c", "--registry", ts.URL, "--home", home); code != 2 {
		t.Errorf("register with three positional arguments: code = %d, want 2", code)
	}
}

func TestCLIUsageErrors(t *testing.T) {
	home := t.TempDir()
	cases := []struct {
		name string
		args []string
		code int
	}{
		{"no arguments", nil, 2},
		{"unknown command", []string{"frobnicate"}, 2},
		{"install without arguments", []string{"install", "--home", home}, 2},
		{"local plus positional", []string{"install", "-u", home, "pkg", "--home", home}, 2},
		{"unknown flag", []string{"show", "--nope", "--home", home}, 2},
		{"offline missing package", []string{"install", "doesnotexist", "--home", home}, 1},
		{"remove missing package", []string{"remove", "doesnotexist", "--home", home}, 1},
		{"show missing package", []string{"show", "doesnotexist", "--home", home}, 1},
	}
	for _, c := range cases {
		if _, _, code := runAPM(t, c.args...); code != c.code {
			t.Errorf("%s: exit code = %d, want %d", c.name, code, c.code)
		}
	}
}

func TestCLIPublishIsGated(t *testing.T) {
	home := t.TempDir()
	lib := writeLocalLib(t, "gated", "1.0.0")

	_, errOut, code := runAPM(t, "publish", lib, "--home", home)
	if code != 2 {
		t.Fatalf("publish without %s should be refused, got code %d (%s)", apm.EnvEnablePublish, code, errOut)
	}
	if !strings.Contains(errOut, "not enabled") {
		t.Errorf("stderr = %q", errOut)
	}

	// The command stays out of the help text until it is enabled for real.
	help, _, _ := runAPM(t, "help")
	if strings.Contains(help, "publish") {
		t.Errorf("help must not advertise publish:\n%s", help)
	}
}

func TestCLIPublishEndToEnd(t *testing.T) {
	t.Setenv(apm.EnvEnablePublish, "1")
	t.Setenv(apm.EnvHome, t.TempDir())

	cfg := registry.DefaultConfig()
	cfg.DataDir = t.TempDir()
	cfg.Tokens = []registry.Token{{Token: "pub-token", Author: "tester", Scopes: []string{"publish"}}}
	srv, err := registry.New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	lib := writeLocalLib(t, "publib", "0.1.0")
	out, errOut, code := runAPM(t, "publish", lib, "--registry", ts.URL, "--token", "pub-token")
	if code != 0 {
		t.Fatalf("publish failed (%d): %s%s", code, out, errOut)
	}
	if !strings.Contains(out, "Published publib 0.1.0") {
		t.Errorf("publish output = %q", out)
	}

	// Republishing the identical directory is idempotent.
	out, _, code = runAPM(t, "publish", lib, "--registry", ts.URL, "--token", "pub-token")
	if code != 0 || !strings.Contains(out, "Already published") {
		t.Errorf("republish: code=%d out=%q", code, out)
	}

	// Without a token the CLI reports the missing credential.
	_, errOut, code = runAPM(t, "publish", lib, "--registry", ts.URL, "--token", "")
	if code != 1 || !strings.Contains(errOut, "token") {
		t.Errorf("publish without token: code=%d stderr=%q", code, errOut)
	}
}

func TestCLIInstallPackageNotInRegistryRedMessageAndProgress(t *testing.T) {
	home := t.TempDir()

	cfg := registry.DefaultConfig()
	cfg.DataDir = t.TempDir()
	cfg.Tokens = []registry.Token{{Token: "test-token", Author: "tester", Scopes: []string{"publish"}}}
	srv, err := registry.New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	t.Setenv(apm.EnvHome, t.TempDir())
	t.Setenv(apm.EnvEnablePublish, "1")

	// 1. Missing library check: must print in RED that library does not exist
	out, errOut, code := runAPM(t, "install", "nosuchlibrary", "--registry", ts.URL, "--home", home)
	if code != 1 {
		t.Fatalf("expected code 1, got %d (out=%s, err=%s)", code, out, errOut)
	}
	if !strings.Contains(errOut, "\033[31m") || !strings.Contains(errOut, "not found in registry") {
		t.Fatalf("expected red error saying library is not in registry, got: %q", errOut)
	}

	// 2. Publish and install with progress bar (speed and size)
	lib := writeLocalLib(t, "proglib", "1.0.0")
	if _, err := apm.Publish(apm.PublishOptions{Dir: lib, RegistryURL: ts.URL, Token: "test-token", Out: io.Discard}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	out, errOut, code = runAPM(t, "install", "proglib", "--registry", ts.URL, "--home", home)
	if code != 0 {
		t.Fatalf("install failed (%d): %s%s", code, out, errOut)
	}
	// Check for small progress bar and size formatting
	if !strings.Contains(out, "[") || !strings.Contains(out, "%") || !strings.Contains(out, "B") {
		t.Errorf("expected progress bar with size/speed in output: %q", out)
	}
}

func TestCLIInit(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	out, _, code := runAPM(t, "init", "my-awesome-lib")
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	if !strings.Contains(out, "my-awesome-lib") {
		t.Errorf("expected 'my-awesome-lib' in output: %q", out)
	}

	manifestPath := filepath.Join(tmpDir, "alca.toml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("alca.toml not created: %v", err)
	}
	if !strings.Contains(string(data), `name = "my-awesome-lib"`) {
		t.Errorf("alca.toml missing package name: %s", string(data))
	}

	mainAl := filepath.Join(tmpDir, "src", "main.al")
	if _, err := os.Stat(mainAl); err != nil {
		t.Fatalf("src/main.al not created: %v", err)
	}

	// Second init in same dir should fail
	_, errOut, code2 := runAPM(t, "init", "another-lib")
	if code2 != 1 {
		t.Fatalf("expected exit code 1 for duplicate init, got %d", code2)
	}
	if !strings.Contains(errOut, "already exists") {
		t.Errorf("expected 'already exists' in stderr: %q", errOut)
	}
}
