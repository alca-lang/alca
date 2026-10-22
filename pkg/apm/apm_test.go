package apm

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeLib creates a minimal valid library directory and returns its path.
func writeLib(t *testing.T, dir, name, version, source string, deps string) string {
	t.Helper()
	manifest := "[package]\nname = \"" + name + "\"\nversion = \"" + version + "\"\nentry = \"src/main.al\"\n\n[build]\ncompile = true\noptimization = 2\n"
	if deps != "" {
		manifest += "\n[dependencies]\n" + deps + "\n"
	}
	mustWrite(t, filepath.Join(dir, "alca.toml"), manifest)
	mustWrite(t, filepath.Join(dir, "src", "main.al"), source)
	return dir
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestParseSpec(t *testing.T) {
	name, c, err := ParseSpec("jsonlib")
	if err != nil || name != "jsonlib" || !c.Satisfies(mustVersion(t, "9.9.9")) {
		t.Fatalf("ParseSpec(jsonlib) = %q, %v, %v", name, c, err)
	}
	_, c, err = ParseSpec("jsonlib@^1.2")
	if err != nil || !c.Satisfies(mustVersion(t, "1.9.0")) || c.Satisfies(mustVersion(t, "2.0.0")) {
		t.Fatalf("ParseSpec constraint wrong: %v, %v", c, err)
	}
	for _, bad := range []string{"", "Bad Name", "ok@not-a-version"} {
		if _, _, err := ParseSpec(bad); err == nil {
			t.Errorf("ParseSpec(%q) should fail", bad)
		}
	}
}

func mustVersion(t *testing.T, s string) Version {
	t.Helper()
	v, err := ParseVersion(s)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestPackUnpackRoundTrip(t *testing.T) {
	dir := writeLib(t, t.TempDir(), "jsonlib", "1.0.0", "func greet() -> string:\n    return \"hi\";\n", "")
	mustWrite(t, filepath.Join(dir, "README.md"), "# jsonlib\n")

	archive, err := PackDir(dir)
	if err != nil {
		t.Fatalf("PackDir: %v", err)
	}
	dst := t.TempDir()
	if err := UnpackArchive(archive, dst); err != nil {
		t.Fatalf("UnpackArchive: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dst, "src", "main.al"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "greet") {
		t.Errorf("unpacked source is wrong: %q", got)
	}
	if _, err := os.Stat(filepath.Join(dst, "README.md")); err != nil {
		t.Errorf("README missing after unpack: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "alca.toml")); err != nil {
		t.Errorf("manifest missing after unpack: %v", err)
	}
}

func TestPackExcludesArtifactsAndVCS(t *testing.T) {
	dir := writeLib(t, t.TempDir(), "jsonlib", "1.0.0", "x := 1;\n", "")
	mustWrite(t, filepath.Join(dir, ".alca", "artifacts", "windows-amd64.bin"), "binary")
	mustWrite(t, filepath.Join(dir, ".git", "config"), "secret")
	archive, err := PackDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	dst := t.TempDir()
	if err := UnpackArchive(archive, dst); err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{".alca", ".git"} {
		if _, err := os.Stat(filepath.Join(dst, forbidden)); err == nil {
			t.Errorf("%s should not be packed", forbidden)
		}
	}
}

func TestUnpackRejectsTraversalAndLinks(t *testing.T) {
	makeArchive := func(hdr *tar.Header, body string) []byte {
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		tw := tar.NewWriter(zw)
		if hdr.Typeflag == tar.TypeReg {
			hdr.Size = int64(len(body))
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if body != "" {
			if _, err := tw.Write([]byte(body)); err != nil {
				t.Fatal(err)
			}
		}
		tw.Close()
		zw.Close()
		return buf.Bytes()
	}
	cases := []struct {
		name string
		hdr  *tar.Header
		body string
	}{
		{"parent escape", &tar.Header{Name: "../evil.txt", Mode: 0o644, Typeflag: tar.TypeReg}, "evil"},
		{"absolute path", &tar.Header{Name: "/evil.txt", Mode: 0o644, Typeflag: tar.TypeReg}, "evil"},
		{"symlink", &tar.Header{Name: "link", Linkname: "/etc/passwd", Mode: 0o777, Typeflag: tar.TypeSymlink}, ""},
	}
	for _, c := range cases {
		dst := t.TempDir()
		if err := UnpackArchive(makeArchive(c.hdr, c.body), dst); err == nil {
			t.Errorf("%s: expected rejection", c.name)
		}
	}
}

func TestArtifactBuildLoadAndStaleness(t *testing.T) {
	t.Setenv(EnvHome, t.TempDir())
	dir := writeLib(t, t.TempDir(), "jsonlib", "1.0.0", "greeting := \"hello\";\n\nfunc greet(n: string) -> string:\n    return greeting + n;\n", "")
	m, err := ValidatePackageDir(dir)
	if err != nil {
		t.Fatalf("ValidatePackageDir: %v", err)
	}
	meta, err := BuildArtifact(dir, m, "windows", "amd64")
	if err != nil {
		t.Fatalf("BuildArtifact: %v", err)
	}
	if meta.SHA256 == "" || meta.SourceHash == "" {
		t.Fatalf("artifact metadata is incomplete: %+v", meta)
	}
	if _, ok := meta.Exports["greet"]; !ok {
		t.Errorf("greet missing from exports: %v", meta.Exports)
	}

	code, loaded, err := LoadArtifact(dir, "windows", "amd64")
	if err != nil || code == nil {
		t.Fatalf("LoadArtifact: %v", err)
	}
	if loaded.SHA256 != meta.SHA256 {
		t.Errorf("loaded metadata differs from built metadata")
	}

	// Wrong platform is incompatible.
	if _, _, err := LoadArtifact(dir, "linux", "amd64"); !errors.Is(err, ErrArtifactIncompatible) {
		t.Errorf("expected ErrArtifactIncompatible, got %v", err)
	}

	// Changing the source makes the artifact stale.
	mustWrite(t, filepath.Join(dir, "src", "main.al"), "func greet(n: string) -> string:\n    return \"bye \" + n;\n")
	if _, _, err := LoadArtifact(dir, "windows", "amd64"); !errors.Is(err, ErrArtifactStale) {
		t.Errorf("expected ErrArtifactStale, got %v", err)
	}
}

func TestArtifactTamperDetected(t *testing.T) {
	t.Setenv(EnvHome, t.TempDir())
	dir := writeLib(t, t.TempDir(), "jsonlib", "1.0.0", "func f() -> int:\n    return 1;\n", "")
	m, err := ValidatePackageDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BuildArtifact(dir, m, "windows", "amd64"); err != nil {
		t.Fatal(err)
	}
	binRel, _ := ArtifactPaths("windows", "amd64")
	binPath := filepath.Join(dir, filepath.FromSlash(binRel))
	if err := os.WriteFile(binPath, []byte("corrupted"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadArtifact(dir, "windows", "amd64"); !errors.Is(err, ErrArtifactIncompatible) {
		t.Errorf("corrupted artifact should be rejected, got %v", err)
	}
}

func TestLocalInstallRemoveAndDependents(t *testing.T) {
	home := t.TempDir()
	t.Setenv(EnvHome, home)
	reg, err := NewRegistry(home)
	if err != nil {
		t.Fatal(err)
	}
	in := NewInstaller(reg, InstallOptions{Out: io.Discard})

	depDir := writeLib(t, t.TempDir(), "depi", "1.0.0", "func value() -> int:\n    return 41;\n", "")
	if _, err := in.InstallLocal(depDir); err != nil {
		t.Fatalf("InstallLocal(dep): %v", err)
	}

	mainDir := writeLib(t, t.TempDir(), "libmain", "1.0.0",
		"use depi;\n\nfunc answer() -> int:\n    return depi.value() + 1;\n",
		"depi = \"^1.0.0\"\n")
	if _, err := in.InstallLocal(mainDir); err != nil {
		t.Fatalf("InstallLocal(main): %v", err)
	}

	list, err := in.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("installed packages = %d, want 2", len(list))
	}

	// depi is a dependency of libmain: removal without --force must fail.
	if _, err := in.Remove("depi", false); err == nil {
		t.Fatalf("removing a dependency without --force should fail")
	}
	if _, err := in.Remove("libmain", false); err != nil {
		t.Fatalf("Remove(libmain): %v", err)
	}
	if _, err := in.Remove("depi", false); err != nil {
		t.Fatalf("Remove(depi) after dependent removal: %v", err)
	}
	if list, _ := in.List(); len(list) != 0 {
		t.Errorf("registry should be empty, got %v", list)
	}
}

func TestInstallLocalRejectsMissingDependency(t *testing.T) {
	t.Setenv(EnvHome, t.TempDir())
	reg, err := NewRegistry(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	in := NewInstaller(reg, InstallOptions{Out: io.Discard})
	dir := writeLib(t, t.TempDir(), "needssomething", "1.0.0",
		"use missingdep;\n\nfunc f() -> int:\n    return missingdep.value();\n",
		"missingdep = \"^1.0.0\"\n")
	_, err = in.InstallLocal(dir)
	if err == nil {
		t.Fatalf("install with a missing dependency should fail")
	}
	if !strings.Contains(err.Error(), "missingdep") {
		t.Errorf("error should mention the missing dependency: %v", err)
	}
}

func TestInstallLocalRejectsInvalidStructure(t *testing.T) {
	t.Setenv(EnvHome, t.TempDir())
	reg, err := NewRegistry(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	in := NewInstaller(reg, InstallOptions{Out: io.Discard})

	// No manifest at all.
	if _, err := in.InstallLocal(t.TempDir()); err == nil {
		t.Errorf("installing a directory without alca.toml should fail")
	}

	// Manifest points at a missing entry.
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "alca.toml"), "[package]\nname = \"broken\"\nversion = \"1.0.0\"\nentry = \"src/main.al\"\n")
	if _, err := in.InstallLocal(dir); err == nil {
		t.Errorf("installing a package with a missing entry should fail")
	}
}

func TestInstallLocalRejectsTypeErrors(t *testing.T) {
	t.Setenv(EnvHome, t.TempDir())
	reg, err := NewRegistry(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	in := NewInstaller(reg, InstallOptions{Out: io.Discard})
	dir := writeLib(t, t.TempDir(), "brokentypes", "1.0.0", "func f() -> int:\n    return \"not an int\";\n", "")
	if _, err := in.InstallLocal(dir); err == nil {
		t.Errorf("installing a package with type errors should fail")
	}
}

func TestResolveInstalledModule(t *testing.T) {
	home := t.TempDir()
	t.Setenv(EnvHome, home)
	reg, err := NewRegistry(home)
	if err != nil {
		t.Fatal(err)
	}
	in := NewInstaller(reg, InstallOptions{Out: io.Discard})
	dir := writeLib(t, t.TempDir(), "jsonlib", "1.0.0", "func greet() -> string:\n    return \"hi\";\n", "")
	mustWrite(t, filepath.Join(dir, "src", "extra.al"), "func shout(t: string) -> string:\n    return t + \"!\";\n")
	if _, err := in.InstallLocal(dir); err != nil {
		t.Fatal(err)
	}

	res, found, err := ResolveInstalledModule("jsonlib", filepath.Join(t.TempDir(), "prog.al"))
	if err != nil || !found {
		t.Fatalf("resolve package entry: found=%v err=%v", found, err)
	}
	if res.Version != "1.0.0" || res.Package != "jsonlib" {
		t.Errorf("resolved identity: %+v", res)
	}
	if !strings.HasSuffix(filepath.ToSlash(res.SourcePath), "src/main.al") {
		t.Errorf("entry path = %q", res.SourcePath)
	}
	if res.ArtifactPath == "" {
		t.Errorf("installed package should expose a compiled artifact")
	}

	res, found, err = ResolveInstalledModule("jsonlib.extra", "prog.al")
	if err != nil || !found {
		t.Fatalf("resolve submodule: found=%v err=%v", found, err)
	}
	if !strings.HasSuffix(filepath.ToSlash(res.SourcePath), "src/extra.al") {
		t.Errorf("submodule path = %q", res.SourcePath)
	}

	if _, found, _ := ResolveInstalledModule("unknownpkg", "prog.al"); found {
		t.Errorf("unknown package must not resolve")
	}
	if _, _, err := ResolveInstalledModule("jsonlib.nope", "prog.al"); err == nil {
		t.Errorf("missing submodule should produce a clear error")
	}
}

func TestSourceHashChangesWithSources(t *testing.T) {
	dir := writeLib(t, t.TempDir(), "h", "1.0.0", "x := 1;\n", "")
	first, err := SourceHash(dir)
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(dir, "src", "other.al"), "y := 2;\n")
	second, err := SourceHash(dir)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Errorf("source hash did not change after adding a file")
	}
}

func TestCheckSecureRegistryURL(t *testing.T) {
	if err := CheckSecureRegistryURL("https://packages.example.com", false); err != nil {
		t.Errorf("https must be accepted: %v", err)
	}
	if err := CheckSecureRegistryURL("http://localhost:8787", false); err != nil {
		t.Errorf("loopback http must be accepted: %v", err)
	}
	if err := CheckSecureRegistryURL("http://packages.example.com", false); err == nil {
		t.Errorf("public plain http must be refused")
	}
	if err := CheckSecureRegistryURL("http://packages.example.com", true); err != nil {
		t.Errorf("explicit insecure override must be accepted: %v", err)
	}
	if err := CheckSecureRegistryURL("ftp://x", false); err == nil {
		t.Errorf("unsupported schemes must be refused")
	}
}

func TestResolveRegistryURLPrecedence(t *testing.T) {
	defer func(old string) { DefaultRegistry = old }(DefaultRegistry)
	t.Setenv(EnvRegistry, "")

	// A build without a stamped node stays offline.
	DefaultRegistry = ""
	if got := ResolveRegistryURL(""); got != "" {
		t.Errorf("unstamped build without env = %q, want empty", got)
	}

	// The node compiled into the binary is the fallback.
	DefaultRegistry = "https://builtin.example.com"
	if got := ResolveRegistryURL(""); got != "https://builtin.example.com" {
		t.Errorf("built-in default = %q", got)
	}

	// An explicit flag wins over everything.
	if got := ResolveRegistryURL("https://flag.example.com"); got != "https://flag.example.com" {
		t.Errorf("flag should win: %q", got)
	}

	// $ALCA_REGISTRY wins over the built-in default, but not over the flag.
	t.Setenv(EnvRegistry, "https://env.example.com")
	if got := ResolveRegistryURL(""); got != "https://env.example.com" {
		t.Errorf("env should win over built-in: %q", got)
	}
	if got := ResolveRegistryURL("https://flag.example.com"); got != "https://flag.example.com" {
		t.Errorf("flag should win over env: %q", got)
	}

	// Whitespace-only values count as unset.
	if got := ResolveRegistryURL("   "); got != "https://env.example.com" {
		t.Errorf("blank flag = %q", got)
	}
	if got := DefaultRegistryURL(); got != "https://builtin.example.com" {
		t.Errorf("DefaultRegistryURL = %q", got)
	}
	DefaultRegistry = "  https://padded.example.com  "
	if got := DefaultRegistryURL(); got != "https://padded.example.com" {
		t.Errorf("DefaultRegistryURL should trim: %q", got)
	}
}
