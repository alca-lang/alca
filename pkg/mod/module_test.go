package mod

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/alca-lang/alca/pkg/sandbox"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBindingName(t *testing.T) {
	cases := map[string]string{
		"std.math":     "math",
		"http":         "http",
		"http.client":  "client",
		"helper":       "helper",
		"models.user":  "user",
		"dir/file.al":  "file",
		`dir\file.al`:  "file",
		"./helpers.al": "helpers",
		"":             "",
	}
	for in, want := range cases {
		if got := BindingName(in); got != want {
			t.Errorf("BindingName(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestResolveLocalRelativeToImporterNotCWD is the regression test for the bug
// where a program launched from another directory could not `use` a module that
// sits next to it.
func TestResolveLocalRelativeToImporterNotCWD(t *testing.T) {
	base := t.TempDir()
	sub := filepath.Join(base, "proj", "sub")
	writeFile(t, filepath.Join(sub, "helper.al"), "func greet() -> string:\n    return \"hi\";\n")
	main := filepath.Join(sub, "main.al")
	writeFile(t, main, "use helper;\n")

	// Run from an unrelated working directory.
	t.Chdir(t.TempDir())
	SetProjectRoot("")
	defer SetProjectRoot("")

	res, found, err := Resolve("helper", main)
	if err != nil {
		t.Fatalf("Resolve returned an error: %v", err)
	}
	if !found {
		t.Fatalf("module next to the importing file was not found")
	}
	want := filepath.Join(sub, "helper.al")
	if res.SourcePath != want {
		t.Errorf("SourcePath = %q, want %q", res.SourcePath, want)
	}
	if res.Root != sub {
		t.Errorf("Root = %q, want %q", res.Root, sub)
	}
}

func TestResolveDottedPathBecomesFile(t *testing.T) {
	base := t.TempDir()
	writeFile(t, filepath.Join(base, "models", "user.al"), "x := 1;\n")
	main := filepath.Join(base, "main.al")
	writeFile(t, main, "use models.user;\n")

	res, found, err := Resolve("models.user", main)
	if err != nil || !found {
		t.Fatalf("Resolve failed: found=%v err=%v", found, err)
	}
	if want := filepath.Join(base, "models", "user.al"); res.SourcePath != want {
		t.Errorf("SourcePath = %q, want %q", res.SourcePath, want)
	}
}

func TestResolveCWD(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "shared.al"), "x := 1;\n")
	t.Chdir(dir)
	SetProjectRoot("")
	defer SetProjectRoot("")

	res, found, err := Resolve("shared", filepath.Join(t.TempDir(), "elsewhere.al"))
	if err != nil || !found {
		t.Fatalf("Resolve from CWD failed: found=%v err=%v", found, err)
	}
	if want := filepath.Join(dir, "shared.al"); res.SourcePath != want {
		t.Errorf("SourcePath = %q, want %q", res.SourcePath, want)
	}
}

func TestResolveBundledWins(t *testing.T) {
	file := filepath.Join(t.TempDir(), "pkg.al")
	writeFile(t, file, "x := 1;\n")
	SetBundled(map[string]string{"pkg": file})
	defer SetBundled(nil)

	res, found, err := Resolve("pkg", "anything.al")
	if err != nil || !found {
		t.Fatalf("bundled module not resolved: found=%v err=%v", found, err)
	}
	if res.SourcePath != file {
		t.Errorf("SourcePath = %q, want %q", res.SourcePath, file)
	}

	// Bundled overrides are cleared when nil is passed.
	SetBundled(nil)
	if _, found, _ := Resolve("pkg", "anything.al"); found {
		t.Errorf("bundled override survived SetBundled(nil)")
	}
}

// TestProjectRootPermissionGate locks in the capability rule: imports inside the
// project root load without flags, files that escape the root need a read
// capability, and the failure is a permission error (not a silent 404).
func TestProjectRootPermissionGate(t *testing.T) {
	sandbox.SetPolicy(sandbox.DenyAll())
	defer sandbox.ResetPolicy()

	parent := t.TempDir()
	root := filepath.Join(parent, "proj")
	outside := filepath.Join(parent, "outside")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(outside, "ext.al"), "x := 1;\n")
	writeFile(t, filepath.Join(root, "inside.al"), "x := 1;\n")
	SetProjectRoot(root)
	defer SetProjectRoot("")

	// A file inside the project root loads under deny-all.
	res, found, err := Resolve("inside", filepath.Join(root, "prog.al"))
	if err != nil || !found {
		t.Fatalf("inside-root import failed: found=%v err=%v", found, err)
	}
	if res.SourcePath == "" {
		t.Fatalf("inside-root import resolved without a source path")
	}

	// A file outside the root is denied without a capability.
	_, found, err = Resolve("../outside/ext.al", filepath.Join(root, "prog.al"))
	if found || err == nil {
		t.Fatalf("outside-root import should have been denied, found=%v err=%v", found, err)
	}
	if !errors.Is(err, os.ErrPermission) {
		t.Errorf("denial should wrap os.ErrPermission, got %v", err)
	}

	// Granting a read root for the outside directory allows it.
	sandbox.SetPolicy(sandbox.Policy{Read: sandbox.Capability{Roots: []string{outside}}})
	resolved, found, err := Resolve("../outside/ext.al", filepath.Join(root, "prog.al"))
	if err != nil || !found {
		t.Fatalf("outside-root import failed with a read capability: found=%v err=%v", found, err)
	}
	if resolved.SourcePath != filepath.Join(outside, "ext.al") {
		t.Errorf("SourcePath = %q, want %q", resolved.SourcePath, filepath.Join(outside, "ext.al"))
	}
}

func TestResolveNoResolverNoLocal(t *testing.T) {
	SetResolver(nil)
	if _, found, err := Resolve("nonexistentpkg", filepath.Join(t.TempDir(), "p.al")); found || err != nil {
		t.Fatalf("expected not found without an error, got found=%v err=%v", found, err)
	}
}
