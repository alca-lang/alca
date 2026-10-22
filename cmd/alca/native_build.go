package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/alca-lang/alca/pkg/stdlib"
)

// nativeCompilerNames lists the C compilers the build tries, in order. The
// environment variable CC has the priority.
func nativeCompilerNames() []string {
	var names []string
	if cc := strings.TrimSpace(os.Getenv("CC")); cc != "" {
		names = append(names, cc)
	}
	switch runtime.GOOS {
	case "windows":
		names = append(names, "clang", "gcc", "cl")
	default:
		names = append(names, "cc", "clang", "gcc")
	}
	return names
}

// compileNativeLibrary builds a C or assembly source into a shared library and
// returns the bytes of the library.
//
// The library is built for the host, because the C compiler of the host is the
// only one that is available. A program that is built for another target has
// to bring a library that was compiled for that target with --native-lib.
func compileNativeLibrary(source string) ([]byte, error) {
	ext := strings.ToLower(filepath.Ext(source))
	if ext != ".c" && ext != ".s" && ext != ".asm" {
		return nil, fmt.Errorf("a native source must be a C or assembly file, got %q", source)
	}
	compiler, err := findNativeCompiler()
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "alca-cc-")
	if err != nil {
		return nil, fmt.Errorf("cannot create the build directory: %w", err)
	}
	defer os.RemoveAll(dir)

	name := strings.TrimSuffix(filepath.Base(source), filepath.Ext(source))
	library := filepath.Join(dir, name+stdlib.NativeLibraryExtension())

	var args []string
	if filepath.Base(compiler) == "cl" {
		// The Microsoft compiler writes a DLL and an import library.
		args = []string{"/nologo", "/LD", "/O2", "/Fe:" + library, source}
	} else {
		args = []string{"-shared", "-fPIC", "-O2", "-o", library, source}
	}
	command := exec.Command(compiler, args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s failed: %w\n%s", compiler, err, strings.TrimSpace(string(output)))
	}
	data, err := os.ReadFile(library)
	if err != nil {
		return nil, fmt.Errorf("%s did not produce a library:\n%s", compiler, strings.TrimSpace(string(output)))
	}
	return data, nil
}

// findNativeCompiler returns the C compiler of the host.
func findNativeCompiler() (string, error) {
	for _, name := range nativeCompilerNames() {
		path, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		return path, nil
	}
	return "", fmt.Errorf("no C compiler found for a native library; install one or use --native-lib with a library that was compiled already")
}
