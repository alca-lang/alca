package registry_test

import (
	"crypto/rand"
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

const testToken = "test-token-123"

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeLib creates a publishable library directory.
func writeLib(t *testing.T, name, version, source, deps string) string {
	t.Helper()
	dir := t.TempDir()
	manifest := "[package]\nname = \"" + name + "\"\nversion = \"" + version + "\"\ndescription = \"test library\"\nentry = \"src/main.al\"\n\n[build]\ncompile = true\noptimization = 2\n"
	if deps != "" {
		manifest += "\n[dependencies]\n" + deps + "\n"
	}
	writeFile(t, filepath.Join(dir, "alca.toml"), manifest)
	writeFile(t, filepath.Join(dir, "src", "main.al"), source)
	return dir
}

func startNode(t *testing.T, tokens []registry.Token, mutate func(*registry.Config)) (*httptest.Server, string) {
	t.Helper()
	dataDir := t.TempDir()
	cfg := registry.DefaultConfig()
	cfg.DataDir = dataDir
	cfg.Tokens = tokens
	if mutate != nil {
		mutate(&cfg)
	}
	srv, err := registry.New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("registry.New: %v", err)
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts, dataDir
}

func publisher(t *testing.T, token ...string) []registry.Token {
	t.Helper()
	tok := testToken
	if len(token) > 0 {
		tok = token[0]
	}
	return []registry.Token{{Token: tok, Author: "tester", Scopes: []string{"publish"}}}
}

func TestPublishInstallRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv(apm.EnvHome, home)

	ts, dataDir := startNode(t, publisher(t), nil)
	lib := writeLib(t, "jsonlib", "1.0.0",
		"greeting := \"hi \";\n\nfunc greet(n: string) -> string:\n    return greeting + n;\n", "")

	res, err := apm.Publish(apm.PublishOptions{Dir: lib, RegistryURL: ts.URL, Token: testToken, Out: io.Discard})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if res.Name != "jsonlib" || res.Version != "1.0.0" || res.SHA256 == "" {
		t.Fatalf("publish result incomplete: %+v", res)
	}
	if len(res.Artifacts) != 1 {
		t.Fatalf("expected one host artifact, got %+v", res.Artifacts)
	}
	published := res.Artifacts[0]
	if published.GOOS != runtime.GOOS || published.GOARCH != runtime.GOARCH {
		t.Fatalf("artifact targets %s/%s", published.GOOS, published.GOARCH)
	}
	if published.SourceHash == "" {
		t.Errorf("published artifact has no source hash")
	}

	// The node stores archive, metadata and build on disk.
	versionDir := filepath.Join(dataDir, "packages", "jsonlib", "1.0.0")
	for _, f := range []string{"package.tar.gz", "meta.json", filepath.Join("artifacts", runtime.GOOS+"-"+runtime.GOARCH+".bin")} {
		if _, err := os.Stat(filepath.Join(versionDir, f)); err != nil {
			t.Errorf("node storage is missing %s: %v", f, err)
		}
	}

	// Install into a clean home through the public installer.
	reg, err := apm.NewRegistry(home)
	if err != nil {
		t.Fatal(err)
	}
	in := apm.NewInstaller(reg, apm.InstallOptions{RegistryURL: ts.URL, Out: io.Discard})
	pkgs, err := in.Install("jsonlib")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if len(pkgs) != 1 || pkgs[0].Name != "jsonlib" || pkgs[0].Version != "1.0.0" {
		t.Fatalf("install result: %+v", pkgs)
	}
	if !strings.HasPrefix(pkgs[0].Origin, "remote:") {
		t.Errorf("origin = %q, want a remote origin", pkgs[0].Origin)
	}
	if len(pkgs[0].Artifacts) != 1 {
		t.Fatalf("installed package has no compiled build: %+v", pkgs[0].Artifacts)
	}
	if pkgs[0].Artifacts[0].SHA256 != published.SHA256 {
		t.Errorf("installed build sha %s != published %s", pkgs[0].Artifacts[0].SHA256, published.SHA256)
	}

	// The downloaded build must load: same toolchain, matching sources.
	code, meta, err := apm.LoadArtifact(pkgs[0].Path, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatalf("LoadArtifact: %v", err)
	}
	if code == nil || meta.SHA256 != published.SHA256 {
		t.Errorf("loaded artifact metadata mismatch: %+v", meta)
	}
	if _, ok := meta.Exports["greet"]; !ok {
		t.Errorf("exports = %v, want greet", meta.Exports)
	}

	// Installing again is a no-op: nothing is downloaded and no error is raised.
	again, err := in.Install("jsonlib@^1.0.0")
	if err != nil {
		t.Fatalf("reinstall: %v", err)
	}
	if len(again) != 0 {
		t.Errorf("reinstall should not plan any work, got %+v", again)
	}
	if shown, err := in.Show("jsonlib"); err != nil || len(shown) != 1 || shown[0].Version != "1.0.0" {
		t.Errorf("Show after install: %+v, %v", shown, err)
	}

	// Unknown versions fail clearly.
	if _, err := in.Install("jsonlib@9.9.9"); err == nil {
		t.Errorf("installing an unpublished version should fail")
	}
}

func TestPublishIdempotentAndImmutable(t *testing.T) {
	t.Setenv(apm.EnvHome, t.TempDir())
	ts, _ := startNode(t, publisher(t), nil)
	lib := writeLib(t, "imlib", "1.0.0", "func f() -> int:\n    return 1;\n", "")

	first, err := apm.Publish(apm.PublishOptions{Dir: lib, RegistryURL: ts.URL, Token: testToken, Out: io.Discard})
	if err != nil {
		t.Fatal(err)
	}
	if first.AlreadyStored {
		t.Errorf("first publish must not report AlreadyStored")
	}

	same, err := apm.Publish(apm.PublishOptions{Dir: lib, RegistryURL: ts.URL, Token: testToken, Out: io.Discard})
	if err != nil {
		t.Fatalf("republishing identical content: %v", err)
	}
	if !same.AlreadyStored {
		t.Errorf("publishing identical content should report AlreadyStored")
	}

	// Same version, different content: refused.
	writeFile(t, filepath.Join(lib, "src", "main.al"), "func f() -> int:\n    return 2;\n")
	if _, err := apm.Publish(apm.PublishOptions{Dir: lib, RegistryURL: ts.URL, Token: testToken, Out: io.Discard}); err == nil {
		t.Errorf("republishing a version with different content should fail")
	}
}

func TestPublishAuth(t *testing.T) {
	t.Setenv(apm.EnvHome, t.TempDir())
	lib := writeLib(t, "authlib", "1.0.0", "func f() -> int:\n    return 1;\n", "")

	ts, _ := startNode(t, publisher(t), nil)
	if _, err := apm.Publish(apm.PublishOptions{Dir: lib, RegistryURL: ts.URL, Token: "wrong", Out: io.Discard}); err == nil {
		t.Errorf("publish with a wrong token should fail")
	}
	if _, err := apm.Publish(apm.PublishOptions{Dir: lib, RegistryURL: ts.URL, Out: io.Discard}); err == nil {
		t.Errorf("publish without a token should fail")
	}

	// A node without tokens refuses publishing entirely.
	ts2, _ := startNode(t, nil, nil)
	if _, err := apm.Publish(apm.PublishOptions{Dir: lib, RegistryURL: ts2.URL, Token: testToken, Out: io.Discard}); err == nil {
		t.Errorf("a node with no configured tokens must refuse publishing")
	}

	// Read-only nodes refuse writes too.
	ts3, _ := startNode(t, publisher(t), func(c *registry.Config) { c.ReadOnly = true })
	if _, err := apm.Publish(apm.PublishOptions{Dir: lib, RegistryURL: ts3.URL, Token: testToken, Out: io.Discard}); err == nil {
		t.Errorf("read-only nodes must refuse publishing")
	}
}

func TestDependencyInstallFromRegistry(t *testing.T) {
	home := t.TempDir()
	t.Setenv(apm.EnvHome, home)
	ts, _ := startNode(t, publisher(t), nil)

	dep := writeLib(t, "libdep", "1.0.0", "func value() -> int:\n    return 41;\n", "")
	if _, err := apm.Publish(apm.PublishOptions{Dir: dep, RegistryURL: ts.URL, Token: testToken, Out: io.Discard}); err != nil {
		t.Fatalf("publish dep: %v", err)
	}
	reg, err := apm.NewRegistry(home)
	if err != nil {
		t.Fatal(err)
	}
	in := apm.NewInstaller(reg, apm.InstallOptions{RegistryURL: ts.URL, Out: io.Discard})
	if _, err := in.Install("libdep"); err != nil {
		t.Fatalf("install dep: %v", err)
	}

	user := writeLib(t, "libuser", "1.1.0",
		"use libdep;\n\nfunc answer() -> int:\n    return libdep.value() + 1;\n",
		"libdep = \"^1.0.0\"\n")
	if _, err := apm.Publish(apm.PublishOptions{Dir: user, RegistryURL: ts.URL, Token: testToken, Out: io.Discard}); err != nil {
		t.Fatalf("publish user: %v", err)
	}

	// Fresh home: installing libuser pulls libdep automatically.
	freshHome := t.TempDir()
	t.Setenv(apm.EnvHome, freshHome)
	freshReg, err := apm.NewRegistry(freshHome)
	if err != nil {
		t.Fatal(err)
	}
	fresh := apm.NewInstaller(freshReg, apm.InstallOptions{RegistryURL: ts.URL, Out: io.Discard})
	pkgs, err := fresh.Install("libuser")
	if err != nil {
		t.Fatalf("install libuser: %v", err)
	}
	names := map[string]string{}
	for _, p := range pkgs {
		names[p.Name] = p.Version
	}
	if names["libuser"] != "1.1.0" || names["libdep"] != "1.0.0" {
		t.Fatalf("installed set = %v, want libuser 1.1.0 and libdep 1.0.0", names)
	}

	// Unsatisfiable constraints are reported.
	if _, err := fresh.Install("libuser@^2.0.0"); err == nil {
		t.Errorf("installing an unsatisfiable constraint should fail")
	}
}

func TestListAndVersions(t *testing.T) {
	t.Setenv(apm.EnvHome, t.TempDir())
	ts, _ := startNode(t, publisher(t), nil)

	for _, v := range []string{"1.0.0", "1.0.1"} {
		lib := writeLib(t, "alpha-one", v, "func f() -> int:\n    return 1;\n", "")
		if _, err := apm.Publish(apm.PublishOptions{Dir: lib, RegistryURL: ts.URL, Token: testToken, Out: io.Discard}); err != nil {
			t.Fatalf("publish alpha-one %s: %v", v, err)
		}
	}
	other := writeLib(t, "beta", "0.1.0", "func f() -> int:\n    return 2;\n", "")
	if _, err := apm.Publish(apm.PublishOptions{Dir: other, RegistryURL: ts.URL, Token: testToken, Out: io.Discard}); err != nil {
		t.Fatalf("publish beta: %v", err)
	}

	client := apm.NewClient(ts.URL, apm.ClientOptions{})
	list, err := client.List("", 20, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if list.Total != 2 {
		t.Errorf("List total = %d, want 2", list.Total)
	}
	search, err := client.List("alpha", 20, 0)
	if err != nil {
		t.Fatalf("List(search): %v", err)
	}
	if search.Total != 1 || search.Packages[0].Name != "alpha-one" {
		t.Errorf("search result = %+v", search)
	}

	versions, err := client.Versions("alpha-one")
	if err != nil {
		t.Fatalf("Versions: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("Versions = %d entries, want 2", len(versions))
	}
	info, err := client.Version("alpha-one", "1.0.1")
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if info.SHA256 == "" || len(info.Artifacts) != 1 {
		t.Errorf("version info incomplete: %+v", info)
	}
	if _, err := client.Version("alpha-one", "9.9.9"); err == nil {
		t.Errorf("unknown version should fail")
	}
	if _, err := client.Versions("nosuchpkg"); err == nil {
		t.Errorf("unknown package should fail")
	}

	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("healthz status = %d", resp.StatusCode)
	}
}

func TestDownloadRejectsCorruptedArchive(t *testing.T) {
	t.Setenv(apm.EnvHome, t.TempDir())
	ts, dataDir := startNode(t, publisher(t), nil)
	lib := writeLib(t, "corruptme", "1.0.0", "func f() -> int:\n    return 1;\n", "")
	res, err := apm.Publish(apm.PublishOptions{Dir: lib, RegistryURL: ts.URL, Token: testToken, Out: io.Discard})
	if err != nil {
		t.Fatal(err)
	}

	archivePath := filepath.Join(dataDir, "packages", "corruptme", "1.0.0", "package.tar.gz")
	writeFile(t, archivePath, "not the archive you asked for")

	client := apm.NewClient(ts.URL, apm.ClientOptions{})
	if _, err := client.Download("corruptme", "1.0.0", res.SHA256); err == nil {
		t.Errorf("download must fail when the served archive does not match its checksum")
	}
}

func TestOversizedPublishRejected(t *testing.T) {
	t.Setenv(apm.EnvHome, t.TempDir())
	ts, _ := startNode(t, publisher(t), func(c *registry.Config) { c.MaxUploadBytes = 4096 })
	lib := writeLib(t, "huge", "1.0.0", "func f() -> int:\n    return 1;\n", "")
	// Incompressible padding: gzip cannot shrink it below the upload cap.
	pad := make([]byte, 32*1024)
	if _, err := rand.Read(pad); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(lib, "padding.bin"), string(pad))
	if _, err := apm.Publish(apm.PublishOptions{Dir: lib, RegistryURL: ts.URL, Token: testToken, Out: io.Discard}); err == nil {
		t.Errorf("publishing an oversized package should fail")
	}
}
