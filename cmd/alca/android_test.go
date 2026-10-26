// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestCmdAndroidWorkflow(t *testing.T) {
	tmpDir := t.TempDir()
	projDir := filepath.Join(tmpDir, "MobileApp")

	// 1. Test init
	code := cmdAndroidInit([]string{projDir})
	if code != 0 {
		t.Fatalf("cmdAndroidInit returned code %d", code)
	}

	// Verify required structure
	if _, err := os.Stat(filepath.Join(projDir, "AndroidManifest.xml")); err != nil {
		t.Fatalf("missing AndroidManifest.xml: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projDir, "src", "main.al")); err != nil {
		t.Fatalf("missing src/main.al: %v", err)
	}

	// 2. Test check
	code = cmdAndroidCheck([]string{projDir})
	if code != 0 {
		t.Fatalf("cmdAndroidCheck returned code %d", code)
	}

	// 3. Test build
	outAPK := filepath.Join(tmpDir, "output.apk")
	code = cmdAndroidBuild([]string{"-o", outAPK, projDir})
	if code != 0 {
		t.Fatalf("cmdAndroidBuild returned code %d", code)
	}

	if _, err := os.Stat(outAPK); err != nil {
		t.Fatalf("expected APK at %s, not found: %v", outAPK, err)
	}

	// 4. Verify contents inside APK
	zr, err := zip.OpenReader(outAPK)
	if err != nil {
		t.Fatalf("failed to open generated APK: %v", err)
	}
	defer zr.Close()

	hasManifest := false
	hasDex := false
	hasCert := false
	for _, f := range zr.File {
		if f.Name == "AndroidManifest.xml" {
			hasManifest = true
		}
		if f.Name == "classes.dex" {
			hasDex = true
		}
		if f.Name == "META-INF/CERT.RSA" {
			hasCert = true
		}
	}

	if !hasManifest || !hasDex || !hasCert {
		t.Fatalf("APK missing critical components: manifest=%v, dex=%v, cert=%v", hasManifest, hasDex, hasCert)
	}
}

func TestCmdBuildWithAPKFlag(t *testing.T) {
	tmpDir := t.TempDir()
	projDir := filepath.Join(tmpDir, "FlagApp")

	code := cmdAndroidInit([]string{projDir})
	if code != 0 {
		t.Fatalf("cmdAndroidInit returned code %d", code)
	}

	outAPK := filepath.Join(tmpDir, "flag_test.apk")
	code = cmdBuild([]string{"--apk", "-o", outAPK, projDir})
	if code != 0 {
		t.Fatalf("cmdBuild with --apk returned code %d", code)
	}

	if _, err := os.Stat(outAPK); err != nil {
		t.Fatalf("expected APK at %s, not found: %v", outAPK, err)
	}
}
