// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/alca-lang/alca/pkg/compiler"
	"github.com/alca-lang/alca/pkg/vm"
)

// TestCrossBuildAllTargets exercises the complete cross-compilation path with
// real runtime stubs: every supported target is built through cmdBuild, the
// output has the target platform's executable header, and its embedded payload
// round-trips to the original source. The host artifact is executed and must
// print exactly what both engines print.
//
// The test is skipped when the runtimes have not been built (they are generated
// artifacts and are not committed). Run `go run ./cmd/alca build-runtimes` first.
func TestCrossBuildAllTargets(t *testing.T) {
	root := repoRoot(t)
	runtimes := filepath.Join(root, "runtimes")
	for _, target := range compiler.SupportedTargets {
		if _, err := os.Stat(filepath.Join(runtimes, compiler.RuntimeName(target))); err != nil {
			t.Skipf("cross runtimes not built; run `go run ./cmd/alca build-runtimes --out runtimes`")
		}
	}

	const src = "println(\"cross-ok\");\nprintln(6 * 7);\n"
	wantVM, vmCode := runCaptured(t, src, engineVM)
	wantInterp, interpCode := runCaptured(t, src, engineInterp)
	if vmCode != 0 || interpCode != 0 {
		t.Fatalf("engines failed: vm=%d interp=%d", vmCode, interpCode)
	}
	if wantVM != wantInterp {
		t.Fatalf("engines disagree: vm=%q interp=%q", wantVM, wantInterp)
	}

	dir := t.TempDir()
	input := filepath.Join(dir, "cross.al")
	if err := os.WriteFile(input, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, target := range compiler.SupportedTargets {
		target := target
		t.Run(target.String(), func(t *testing.T) {
			output := compiler.ExeName(filepath.Join(dir, "cross-"+target.OS+"-"+target.Arch), target)
			code := cmdBuild([]string{
				input,
				"--target", target.String(),
				"--runtime", filepath.Join(runtimes, compiler.RuntimeName(target)),
				"-o", output,
			})
			if code != 0 {
				t.Fatalf("cmdBuild exited with %d", code)
			}
			raw, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}
			if !hasExecutableHeader(raw, target) {
				t.Fatalf("%s output does not have a %s executable header", target, target.OS)
			}
			bundle, err := compiler.ExtractBundle(output)
			if err != nil {
				t.Fatalf("extract bundle: %v", err)
			}
			if bundle.Kind != compiler.KindBytecode {
				t.Fatalf("bundle kind = %c, want bytecode ('B')", bundle.Kind)
			}
			if bundle.Name != "cross.al" {
				t.Errorf("bundle name = %q, want cross.al", bundle.Name)
			}
			if bundle.Code == nil || len(bundle.Code.Constants) == 0 {
				t.Fatal("bytecode payload has no constants")
			}
			if !payloadContains(bundle, "cross-ok") {
				t.Error("bytecode payload does not contain the printed string")
			}
			payloadOut, payloadCode := runBundleOnVM(t, bundle)
			if payloadCode != 0 {
				t.Fatalf("decoded %s payload failed on the VM with exit code %d", target, payloadCode)
			}
			if payloadOut != wantVM {
				t.Fatalf("decoded %s payload output = %q, want %q", target, payloadOut, wantVM)
			}
			if target == compiler.HostTarget() {
				out, err := exec.Command(output).CombinedOutput()
				if err != nil {
					t.Fatalf("run host artifact: %v\n%s", err, out)
				}
				if string(out) != wantVM {
					t.Fatalf("host artifact output = %q, want %q", out, wantVM)
				}
			}
		})
	}
}

// runBundleOnVM executes a decoded bundle in-process. Cross-target bytecode is
// platform-independent, so this verifies the payload that each target would run
// without needing an emulator for that target.
func runBundleOnVM(t *testing.T, bundle *compiler.Bundle) (string, int) {
	t.Helper()
	interp := newInterpreter()
	machine := vm.NewWithEnv(bundle.Code, interp.Env())

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	done := make(chan string)
	go func() { b, _ := io.ReadAll(r); done <- string(b) }()

	err := machine.Run()
	w.Close()
	os.Stdout = old
	out := <-done
	if err != nil {
		t.Logf("decoded bundle run error: %v", err)
		return out, 1
	}
	if machine.TestFailures > 0 {
		return out, 1
	}
	return out, 0
}

// payloadContains reports whether a string constant survived serialization.
func payloadContains(bundle *compiler.Bundle, want string) bool {
	for _, c := range bundle.Code.Constants {
		if s, ok := c.(interface{ Inspect() string }); ok && s.Inspect() == want {
			return true
		}
	}
	return false
}

// hasExecutableHeader checks the magic bytes of the produced artifact.
func hasExecutableHeader(raw []byte, target compiler.Target) bool {
	switch target.OS {
	case "linux":
		return bytes.HasPrefix(raw, []byte{0x7f, 'E', 'L', 'F'})
	case "windows":
		return bytes.HasPrefix(raw, []byte{'M', 'Z'})
	case "darwin":
		// Both Mach-O 64-bit variants (x86_64 and arm64) start with cf fa ed fe.
		return bytes.HasPrefix(raw, []byte{0xcf, 0xfa, 0xed, 0xfe})
	default:
		return false
	}
}

// TestPureNativeBailoutFailsLoudly verifies that a pure native PE with no VM
// does not silently succeed when native execution deoptimizes. Division by zero
// is a runtime type/range bailout in the AMD64 backend, so the executable must
// report the missing VM fallback and exit nonzero.
func TestPureNativeBailoutFailsLoudly(t *testing.T) {
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		t.Skip("pure native PE output is Windows/amd64 only")
	}

	dir := t.TempDir()
	input := filepath.Join(dir, "bailout.al")
	const src = "a := 10;\nb := 0;\nc := a / b;\n"
	if err := os.WriteFile(input, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "bailout.exe")
	if code := cmdBuild([]string{input, "-o", output}); code != 0 {
		t.Fatalf("cmdBuild exited with %d", code)
	}

	info, err := os.Stat(output)
	if err != nil {
		t.Fatal(err)
	}
	// The pure PE is a few KB; falling back to the runtime stub would embed the
	// whole Go runtime and be far larger.
	if info.Size() > 64*1024 {
		t.Fatalf("output is %d bytes; expected a pure native PE, not the runtime stub", info.Size())
	}

	out, err := exec.Command(output).CombinedOutput()
	if err == nil {
		t.Fatalf("bailout executable succeeded silently; output: %s", out)
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("run executable: %v", err)
	}
	if code := exitErr.ExitCode(); code == 0 {
		t.Fatalf("bailout executable exited 0; output: %s", out)
	}
	if got := string(out); !strings.Contains(got, "no VM fallback") {
		t.Fatalf("bailout executable did not explain the missing VM; output: %q", got)
	}
}

func TestCollectEmbeddedResources(t *testing.T) {
	root := t.TempDir()
	assets := filepath.Join(root, "assets")
	if err := os.MkdirAll(filepath.Join(assets, "icons"), 0o755); err != nil {
		t.Fatal(err)
	}
	want := []byte{0, 1, 2, 255}
	if err := os.WriteFile(filepath.Join(assets, "icons", "app.ico"), want, 0o644); err != nil {
		t.Fatal(err)
	}
	resources, err := collectEmbeddedResources(root, []string{"assets"})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(resources["assets/icons/app.ico"], want) {
		t.Fatalf("embedded bytes = %v, want %v", resources["assets/icons/app.ico"], want)
	}
	if _, err := collectEmbeddedResources(root, []string{"../outside"}); err == nil {
		t.Fatal("expected paths outside the project directory to be rejected")
	}
}

func TestBuildEmbedsRequestedResources(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "main.al")
	if err := os.WriteFile(input, []byte(`println("embedded");`), 0o644); err != nil {
		t.Fatal(err)
	}
	asset := filepath.Join(root, "assets", "data.bin")
	if err := os.MkdirAll(filepath.Dir(asset), 0o755); err != nil {
		t.Fatal(err)
	}
	want := []byte{0, 7, 255}
	if err := os.WriteFile(asset, want, 0o644); err != nil {
		t.Fatal(err)
	}
	stub, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, "app.exe")
	if code := cmdBuild([]string{input, "--embed", "assets", "--runtime", stub, "--bytecode", "-o", output}); code != 0 {
		t.Fatalf("cmdBuild exited with %d", code)
	}
	bundle, err := compiler.ExtractBundle(output)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bundle.Resources["assets/data.bin"], want) {
		t.Fatalf("bundle resource = %v, want %v", bundle.Resources["assets/data.bin"], want)
	}
}
