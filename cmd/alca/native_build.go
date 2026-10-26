// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alca-lang/alca/pkg/compiler"
	"github.com/alca-lang/alca/pkg/stdlib"
)

// compileNativeLibrary builds a C or assembly source into a shared library and
// returns the bytes of the library.
func compileNativeLibrary(source string) ([]byte, error) {
	dir, err := os.MkdirTemp("", "alca-cc-")
	if err != nil {
		return nil, fmt.Errorf("cannot create the build directory: %w", err)
	}
	defer os.RemoveAll(dir)

	name := strings.TrimSuffix(filepath.Base(source), filepath.Ext(source))
	library := filepath.Join(dir, name+stdlib.NativeLibraryExtension())

	outPath, err := stdlib.CompileSharedLibrary(source, library)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(outPath)
}

func findNativeCompiler() (string, error) {
	return stdlib.FindNativeCompiler(false)
}

// cmdBuildDLL implements `alca dll <source> [-o output.dll]`.
func cmdBuildDLL(args []string) int {
	var source, output string
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "-o" && i+1 < len(args):
			output = args[i+1]
			i++
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(os.Stderr, "unknown dll option %q\n", a)
			return 1
		default:
			source = a
		}
	}
	if source == "" {
		fmt.Fprintln(os.Stderr, "Usage: alca dll <source.al|source.c> [-o output.dll]")
		return 1
	}

	ext := strings.ToLower(filepath.Ext(source))
	if ext == ".al" {
		data, err := os.ReadFile(source)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		if output == "" {
			output = strings.TrimSuffix(source, filepath.Ext(source)) + ".dll"
		}
		absOutput, err := filepath.Abs(output)
		if err != nil {
			absOutput = output
		}
		if err := compiler.BuildNativeDLLFromAlca(string(data), absOutput); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		fmt.Printf("Compiled pure native DLL: %s\n", absOutput)
		return 0
	}

	outPath, err := stdlib.CompileSharedLibrary(source, output)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	fmt.Printf("Compiled shared library: %s\n", outPath)
	return 0
}
