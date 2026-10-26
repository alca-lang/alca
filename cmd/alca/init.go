// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func cmdInit(args []string) int {
	name := "myapp"
	targetDir := "."
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		targetDir = strings.TrimSpace(args[0])
		name = filepath.Base(targetDir)
	}

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "error creating directory %s: %v\n", targetDir, err)
		return 1
	}

	mainPath := filepath.Join(targetDir, "main.al")
	if _, err := os.Stat(mainPath); os.IsNotExist(err) {
		mainContent := fmt.Sprintf(`// %s - Alca Application
use std.io;

println("Hello from %s!");
`, name, name)
		if err := os.WriteFile(mainPath, []byte(mainContent), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "error writing %s: %v\n", mainPath, err)
			return 1
		}
	}

	tomlPath := filepath.Join(targetDir, "alca.toml")
	if _, err := os.Stat(tomlPath); os.IsNotExist(err) {
		tomlContent := fmt.Sprintf(`[package]
name = %q
version = "0.1.0"
entry = "main.al"
`, name)
		_ = os.WriteFile(tomlPath, []byte(tomlContent), 0o644)
	}

	fmt.Printf("✓ Initialized Alca project %q in %s\n", name, targetDir)
	return 0
}
