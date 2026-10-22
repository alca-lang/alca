package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alca-lang/alca/pkg/apm"
)

// writeText writes a file, creating parent directories.
func writeText(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// installLibrary writes a source library and installs it into home, so `use
// <name>` resolves through the APM registry exactly like an end-user install.
func installLibrary(t *testing.T, home, name, source string) {
	t.Helper()
	dir := t.TempDir()
	writeText(t, filepath.Join(dir, "alca.toml"),
		"[package]\nname = \""+name+"\"\nversion = \"1.0.0\"\ndescription = \"integration test library\"\nentry = \"src/main.al\"\n\n[build]\ncompile = true\noptimization = 2\n")
	writeText(t, filepath.Join(dir, "src", "main.al"), source)
	reg, err := apm.NewRegistry(home)
	if err != nil {
		t.Fatal(err)
	}
	in := apm.NewInstaller(reg, apm.InstallOptions{Out: io.Discard})
	if _, err := in.InstallLocal(dir); err != nil {
		t.Fatalf("installing %s: %v", name, err)
	}
}

// TestAPMLibraryBothEngines runs a program importing an installed library on
// both engines; the outputs must match. The VM path loads the compiled artifact,
// the interpreter path loads the sources, so this is the artifact/parity check.
func TestAPMLibraryBothEngines(t *testing.T) {
	home := t.TempDir()
	t.Setenv(apm.EnvHome, home)
	installLibrary(t, home, "jsonlib", `
greeting := "hello ";
counter := 0;

func greet(name: string) -> string:
    return greeting + name;

func add(a: int, b: int) -> int:
    return a + b;

func bump() -> int:
    counter = counter + 1;
    return counter;

func current() -> int:
    return counter;
`)
	// A submodule of the same package.
	installSubmodule(t, home, "jsonlib", "extra.al", "func shout(text: string) -> string:\n    return text + \"!\";\n")

	src := `
use jsonlib;
use jsonlib.extra;

println(jsonlib.greet("bob"));
println(jsonlib.add(2, 3));
println(jsonlib.bump());
println(jsonlib.bump());
println(jsonlib.current());
println(extra.shout("hey"));
`
	want := "hello bob\n5\n1\n2\n2\nhey!\n"

	vmOut, vmCode := runCaptured(t, src, engineVM)
	interpOut, interpCode := runCaptured(t, src, engineInterp)
	if vmCode != 0 || interpCode != 0 {
		t.Fatalf("exit codes: vm=%d interp=%d", vmCode, interpCode)
	}
	if interpOut != want {
		t.Errorf("interpreter output:\n%s\nwant:\n%s", interpOut, want)
	}
	if vmOut != want {
		t.Errorf("vm output:\n%s\nwant:\n%s", vmOut, want)
	}
	if vmOut != interpOut {
		t.Errorf("engines disagree:\nvm:     %q\ninterp: %q", vmOut, interpOut)
	}
}

// installSubmodule adds one module file to an installed package.
func installSubmodule(t *testing.T, home, name, file, source string) {
	t.Helper()
	reg, err := apm.NewRegistry(home)
	if err != nil {
		t.Fatal(err)
	}
	pkgDir := filepath.Join(reg.PackagesDir(), name, "1.0.0", "src")
	writeText(t, filepath.Join(pkgDir, file), source)
}

// TestAPMMissingLibraryTeachesInstall verifies the error path of an unknown
// import: both engines report it, and the message points at apm install.
func TestAPMMissingLibraryTeachesInstall(t *testing.T) {
	t.Setenv(apm.EnvHome, t.TempDir())
	file := writeAlca(t, "use definitelynotinstalled;\nprintln(\"never\");\n")

	_, stderr, code := runCLI(t, "--interp", file)
	if code == 0 {
		t.Fatalf("running an unknown import should fail")
	}
	if !strings.Contains(stderr, "apm install definitelynotinstalled") {
		t.Errorf("stderr = %q, want an apm install hint", stderr)
	}

	// The VM path must surface an error too, not silently succeed.
	if _, _, vmCode := runCLI(t, "--vm", file); vmCode == 0 {
		t.Errorf("the VM should also fail on an unknown import")
	}
}

// TestAPMStandaloneBuildIsSelfContained builds an executable importing an
// installed library and runs it with an empty ALCA_HOME: the sources travel in
// the binary.
func TestAPMStandaloneBuildIsSelfContained(t *testing.T) {
	home := t.TempDir()
	t.Setenv(apm.EnvHome, home)
	installLibrary(t, home, "liba", `
func triple(n: int) -> int:
    return n * 3;
`)

	src := "use liba;\nprintln(liba.triple(14));\n"
	output := buildStandalone(t, src)

	// Run against an empty registry home: the artifact must not need it.
	t.Setenv(apm.EnvHome, t.TempDir())
	stdout, stderr, code := runArtifact(t, output)
	if code != 0 {
		t.Fatalf("standalone run failed (%d): %s", code, stderr)
	}
	if stdout != "42\n" {
		t.Errorf("standalone output = %q, want %q", stdout, "42\n")
	}
}
