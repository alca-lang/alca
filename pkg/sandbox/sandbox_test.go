// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package sandbox

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultPolicyDeniesEverything(t *testing.T) {
	old := CurrentPolicy()
	defer SetPolicy(old)

	SetPolicy(DenyAll())
	for _, err := range []error{
		RequireRead("file.txt"),
		RequireWrite("file.txt"),
		RequireNet("example.com:443"),
	} {
		if err == nil {
			t.Fatal("deny-all policy granted access")
		}
		if !errors.Is(err, os.ErrPermission) {
			t.Fatalf("%v must classify as a permission error", err)
		}
		if !strings.Contains(err.Error(), "permission denied") {
			t.Fatalf("error must mention permission denied: %v", err)
		}
	}
}

func TestPathRootsAllowSubtreeOnly(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "sub", "file.txt")
	outside := filepath.Join(t.TempDir(), "file.txt")
	policy := Policy{Read: Capability{Roots: []string{root}}, Write: Capability{Roots: []string{root}}}
	if !policy.Read.AllowsPath(inside) {
		t.Fatalf("%q should be inside %q", inside, root)
	}
	if policy.Read.AllowsPath(outside) {
		t.Fatalf("%q must not be inside %q", outside, root)
	}
	if policy.Read.AllowsPath(filepath.Join(root, "..", filepath.Base(outside))) {
		t.Fatal("a path escaping the root with .. must be denied")
	}
	if policy.Write.AllowsPath(root) == false {
		t.Fatal("the root itself must be writable")
	}
}

func TestNetworkRulesMatchHostAndOptionalPort(t *testing.T) {
	hostOnly := Capability{Roots: []string{"example.com"}}
	if !hostOnly.AllowsHost("example.com:443") || !hostOnly.AllowsHost("example.com") {
		t.Fatal("host-only rule must ignore the port")
	}
	if hostOnly.AllowsHost("api.example.com:443") {
		t.Fatal("host-only rule must not grant subdomains")
	}

	withPort := Capability{Roots: []string{"example.com:443"}}
	if !withPort.AllowsHost("example.com:443") {
		t.Fatal("exact host:port rule must match")
	}
	if withPort.AllowsHost("example.com:80") {
		t.Fatal("exact host:port rule must not grant another port")
	}

	wildcard := Capability{Roots: []string{"*.example.com"}}
	if !wildcard.AllowsHost("api.example.com:443") {
		t.Fatal("wildcard subdomain rule must match")
	}
	if wildcard.AllowsHost("example.com:443") {
		t.Fatal("wildcard subdomain rule must not match the bare domain")
	}
}

func TestURLRulesUseTheURLPort(t *testing.T) {
	policy := Policy{Net: Capability{Roots: []string{"example.com:8443"}}}
	old := CurrentPolicy()
	defer SetPolicy(old)
	SetPolicy(policy)

	if err := RequireURL("https://example.com:8443/api"); err != nil {
		t.Fatalf("allowed URL rejected: %v", err)
	}
	if err := RequireURL("https://example.com/api"); err == nil {
		t.Fatal("URL on a different port must be denied")
	}
}

func TestParseArgs(t *testing.T) {
	dir := t.TempDir()
	policy, rest, err := ParseArgs([]string{
		"--allow-read=" + dir,
		"--allow-net=example.com:443",
		"program.al",
		"--vm",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rest) != 2 || rest[0] != "program.al" || rest[1] != "--vm" {
		t.Fatalf("rest = %#v", rest)
	}
	if !policy.Read.AllowsPath(filepath.Join(dir, "a.txt")) {
		t.Fatal("read flag was not applied")
	}
	if policy.Write.AllowsPath(filepath.Join(dir, "a.txt")) {
		t.Fatal("read flag must not grant write access")
	}
	if !policy.Net.AllowsHost("example.com:443") || policy.Net.AllowsHost("example.com:80") {
		t.Fatal("network flag was not applied correctly")
	}
}

func TestParseArgsAllowAllAndSeparator(t *testing.T) {
	policy, rest, err := ParseArgs([]string{"--allow-all", "--", "program.al", "--flag"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rest) != 2 || rest[0] != "program.al" {
		t.Fatalf("rest = %#v", rest)
	}
	old := CurrentPolicy()
	defer SetPolicy(old)
	SetPolicy(policy)
	if err := RequireRead(filepath.Join(t.TempDir(), "x")); err != nil {
		t.Fatalf("--allow-all must permit reads: %v", err)
	}
	if err := RequireNet("anything.example:1"); err != nil {
		t.Fatalf("--allow-all must permit network: %v", err)
	}
}

func TestRequireAllCapabilities(t *testing.T) {
	old := CurrentPolicy()
	defer SetPolicy(old)

	SetPolicy(Policy{Read: Capability{All: true}, Write: Capability{All: true}})
	if err := RequireAllCapabilities("native code"); err == nil || !errors.Is(err, os.ErrPermission) {
		t.Fatalf("partial unrestricted policy must be denied, got %v", err)
	}
	SetPolicy(AllowAll())
	if err := RequireAllCapabilities("native code"); err != nil {
		t.Fatalf("allow-all policy must pass: %v", err)
	}
}

func TestParseArgsRejectsUnknownAllowFlag(t *testing.T) {
	if _, _, err := ParseArgs([]string{"--allow-everything"}); err == nil {
		t.Fatal("unknown capability flag must be rejected")
	}
	if _, _, err := ParseArgs([]string{"--allow-read="}); err == nil {
		t.Fatal("an empty path must be rejected")
	}
}

func TestRunEnvFFICapabilities(t *testing.T) {
	old := CurrentPolicy()
	defer SetPolicy(old)

	// 1. DenyAll denies all
	SetPolicy(DenyAll())
	if err := RequireRun("curl"); err == nil || !errors.Is(err, os.ErrPermission) {
		t.Errorf("RequireRun must be denied by default, got %v", err)
	}
	if err := RequireEnv("MY_KEY"); err == nil || !errors.Is(err, os.ErrPermission) {
		t.Errorf("RequireEnv must be denied by default, got %v", err)
	}
	if err := RequireFFI("user32.dll"); err == nil || !errors.Is(err, os.ErrPermission) {
		t.Errorf("RequireFFI must be denied by default, got %v", err)
	}

	// 2. Granular flags
	policy, _, err := ParseArgs([]string{"--allow-run=git,curl", "--allow-env=ALCA_ENV", "--allow-ffi=user32.dll"})
	if err != nil {
		t.Fatalf("ParseArgs failed: %v", err)
	}
	SetPolicy(policy)

	if err := RequireRun("curl"); err != nil {
		t.Errorf("RequireRun(curl) must be allowed: %v", err)
	}
	if err := RequireRun("sh"); err == nil {
		t.Errorf("RequireRun(sh) must be denied")
	}

	if err := RequireEnv("ALCA_ENV"); err != nil {
		t.Errorf("RequireEnv(ALCA_ENV) must be allowed: %v", err)
	}
	if err := RequireEnv("SECRET_TOKEN"); err == nil {
		t.Errorf("RequireEnv(SECRET_TOKEN) must be denied")
	}

	if err := RequireFFI("user32.dll"); err != nil {
		t.Errorf("RequireFFI(user32.dll) must be allowed: %v", err)
	}
	if err := RequireFFI("kernel32.dll"); err == nil {
		t.Errorf("RequireFFI(kernel32.dll) must be denied")
	}

	// 3. AllowAll grants all
	SetPolicy(AllowAll())
	if err := RequireRun("anything"); err != nil {
		t.Errorf("AllowAll must permit Run: %v", err)
	}
	if err := RequireEnv("anything"); err != nil {
		t.Errorf("AllowAll must permit Env: %v", err)
	}
	if err := RequireFFI("anything.so"); err != nil {
		t.Errorf("AllowAll must permit FFI: %v", err)
	}
}
