// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0
//
// MIGRATION-ONLY Go oracle. It exists solely to produce golden token dumps for
// differential testing while the compiler is being ported from Go to C++.
// It is deleted together with pkg/** in Stage 9.

package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pawl-lang/pawl/pkg/lexer"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: lexdump <repo-root> <corpus-manifest> <golden-dir>")
		os.Exit(2)
	}
	root, manifest, goldenDir := os.Args[1], os.Args[2], os.Args[3]

	manifestData, err := os.ReadFile(manifest)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read manifest: %v\n", err)
		os.Exit(1)
	}

	count := 0
	for _, rel := range strings.Split(strings.ReplaceAll(string(manifestData), "\r\n", "\n"), "\n") {
		if rel == "" || strings.HasPrefix(rel, "#") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			fmt.Fprintf(os.Stderr, "read %s: %v\n", rel, err)
			os.Exit(1)
		}

		l := lexer.New(string(src))
		var b strings.Builder
		for _, tok := range l.Tokenize() {
			fmt.Fprintf(&b, "%s\t%d\t%d\t%s\n", tok.Type, tok.Line, tok.Column, hex.EncodeToString([]byte(tok.Literal)))
		}

		outPath := filepath.Join(goldenDir, filepath.FromSlash(rel)+".tokens")
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "mkdir %s: %v\n", outPath, err)
			os.Exit(1)
		}
		if err := os.WriteFile(outPath, []byte(b.String()), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "write %s: %v\n", outPath, err)
			os.Exit(1)
		}
		count++
	}
	fmt.Fprintf(os.Stderr, "lexdump: wrote %d golden files\n", count)
}
