package apm_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alca-lang/alca/pkg/apm"
)

func TestLockfileRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "apm.lock")

	lock := apm.NewLockfile()
	lock.UpdateFromInstalled([]apm.InstalledPackage{
		{
			Name:    "json-helper",
			Version: "1.2.3",
			Dependencies: map[string]string{
				"core": "^1.0.0",
			},
		},
	})

	if err := lock.Write(lockPath); err != nil {
		t.Fatalf("failed to write lockfile: %v", err)
	}

	loaded, err := apm.ReadLockfile(lockPath)
	if err != nil {
		t.Fatalf("failed to read lockfile: %v", err)
	}

	if loaded.Version != 1 {
		t.Errorf("expected version 1, got %d", loaded.Version)
	}
	pkg, ok := loaded.Packages["json-helper"]
	if !ok {
		t.Fatalf("expected package 'json-helper' in lockfile")
	}
	if pkg.Version != "1.2.3" {
		t.Errorf("expected version '1.2.3', got %q", pkg.Version)
	}
	if pkg.Dependencies["core"] != "^1.0.0" {
		t.Errorf("expected dep core='^1.0.0', got %q", pkg.Dependencies["core"])
	}
}

func TestFindLockfilePath(t *testing.T) {
	tmpDir := t.TempDir()
	tomlPath := filepath.Join(tmpDir, "alca.toml")
	if err := os.WriteFile(tomlPath, []byte("[package]\nname = \"test\"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	subDir := filepath.Join(tmpDir, "src", "sub")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	lockPath, inProj := apm.FindLockfilePath(subDir)
	if !inProj {
		t.Fatalf("expected to find project root from subDir")
	}
	expected := filepath.Join(tmpDir, "apm.lock")
	if lockPath != expected {
		t.Fatalf("expected lock path %q, got %q", expected, lockPath)
	}
}
