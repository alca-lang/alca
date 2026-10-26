// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package stdlib

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// NativeCompilerNames returns candidate compiler names in order of preference.
func NativeCompilerNames(forCpp bool) []string {
	var names []string
	if forCpp {
		if cxx := strings.TrimSpace(os.Getenv("CXX")); cxx != "" {
			names = append(names, cxx)
		}
		switch runtime.GOOS {
		case "windows":
			names = append(names, "clang++", "g++", "clang", "gcc", "cl")
		default:
			names = append(names, "c++", "clang++", "g++")
		}
	} else {
		if cc := strings.TrimSpace(os.Getenv("CC")); cc != "" {
			names = append(names, cc)
		}
		switch runtime.GOOS {
		case "windows":
			names = append(names, "clang", "gcc", "cl")
		default:
			names = append(names, "cc", "clang", "gcc")
		}
	}
	return names
}

// FindNativeCompiler locates a working C/C++ compiler executable.
func FindNativeCompiler(forCpp bool) (string, error) {
	for _, name := range NativeCompilerNames(forCpp) {
		path, err := exec.LookPath(name)
		if err == nil {
			return path, nil
		}
	}
	compType := "C"
	if forCpp {
		compType = "C/C++"
	}
	return "", fmt.Errorf("no %s compiler found (install gcc, clang or MSVC cl)", compType)
}

// CompileSharedLibrary compiles a native source file (.c, .cpp, .cc, .cxx, .s, .asm)
// into a shared library (.dll on Windows, .so on Linux, .dylib on macOS).
func CompileSharedLibrary(sourcePath string, outputPath string) (string, error) {
	ext := strings.ToLower(filepath.Ext(sourcePath))
	isCpp := ext == ".cpp" || ext == ".cc" || ext == ".cxx"
	isC := ext == ".c" || ext == ".s" || ext == ".asm"

	if !isCpp && !isC {
		return "", fmt.Errorf("source file must be .c, .cpp, .cc, .s, or .asm, got %q", sourcePath)
	}

	sourceAbs, err := filepath.Abs(sourcePath)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(sourceAbs); err != nil {
		return "", fmt.Errorf("source file %q does not exist: %w", sourcePath, err)
	}

	if outputPath == "" {
		outputPath = strings.TrimSuffix(sourceAbs, filepath.Ext(sourceAbs)) + NativeLibraryExtension()
	}
	outputAbs, err := filepath.Abs(outputPath)
	if err != nil {
		outputAbs = outputPath
	}

	outDir := filepath.Dir(outputAbs)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "", fmt.Errorf("cannot create output directory %q: %w", outDir, err)
	}

	compiler, err := FindNativeCompiler(isCpp)
	if err != nil {
		return "", err
	}

	var args []string
	compilerBase := strings.ToLower(filepath.Base(compiler))
	if compilerBase == "cl" || compilerBase == "cl.exe" {
		args = []string{"/nologo", "/LD", "/O2", "/Fe:" + outputAbs, sourceAbs}
	} else {
		args = []string{"-shared", "-fPIC", "-O2", "-o", outputAbs, sourceAbs}
	}

	cmd := exec.Command(compiler, args...)
	cmd.Dir = outDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s compilation failed: %w\n%s", compiler, err, strings.TrimSpace(string(out)))
	}

	if _, err := os.Stat(outputAbs); err != nil {
		return "", fmt.Errorf("%s succeeded but did not produce %q:\n%s", compiler, outputAbs, strings.TrimSpace(string(out)))
	}

	return outputAbs, nil
}

// CompileSharedLibraryFromSource takes source code text, writes it to a temporary file,
// compiles it into a shared library, and returns the path to the library.
func CompileSharedLibraryFromSource(sourceCode string, outputPath string, isCpp bool) (string, error) {
	ext := ".c"
	if isCpp {
		ext = ".cpp"
	}
	tmpFile, err := os.CreateTemp("", "alca-src-*"+ext)
	if err != nil {
		return "", fmt.Errorf("cannot create temp source file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(sourceCode); err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("cannot write temp source code: %w", err)
	}
	tmpFile.Close()

	if outputPath == "" {
		tmpDir := os.TempDir()
		outputPath = filepath.Join(tmpDir, fmt.Sprintf("alca_lib_%d%s", os.Getpid(), NativeLibraryExtension()))
	}

	return CompileSharedLibrary(tmpFile.Name(), outputPath)
}
