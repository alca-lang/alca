// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package android

import (
	"archive/zip"
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"hash/adler32"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildLauncherDEX(t *testing.T) {
	dexData, err := BuildLauncherDEX("alca.runtime.MainActivity")
	if err != nil {
		t.Fatalf("BuildLauncherDEX failed: %v", err)
	}

	if len(dexData) < 112 {
		t.Fatalf("DEX too short: %d bytes", len(dexData))
	}

	// Verify magic
	if string(dexData[:8]) != "dex\n035\x00" {
		t.Fatalf("invalid DEX magic: %q", string(dexData[:8]))
	}

	// Verify SHA-1
	sig := sha1.Sum(dexData[32:])
	if !bytes.Equal(sig[:], dexData[12:32]) {
		t.Fatalf("DEX SHA-1 signature mismatch")
	}

	// Verify Adler32
	chk := adler32.Checksum(dexData[12:])
	fileChk := binary.LittleEndian.Uint32(dexData[8:12])
	if chk != fileChk {
		t.Fatalf("DEX Adler32 checksum mismatch: got %x, expected %x", fileChk, chk)
	}

	// Verify file size recorded in header
	fileSize := binary.LittleEndian.Uint32(dexData[0x20:0x24])
	if int(fileSize) != len(dexData) {
		t.Fatalf("DEX recorded size %d != actual %d", fileSize, len(dexData))
	}
}

func TestCompileAXML(t *testing.T) {
	xmlText := `<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android"
    package="com.example.test"
    android:versionCode="1"
    android:versionName="1.0.0">
    <uses-sdk android:minSdkVersion="21" android:targetSdkVersion="34" />
    <application android:label="Test App">
        <activity android:name="alca.runtime.MainActivity" android:exported="true">
            <intent-filter>
                <action android:name="android.intent.action.MAIN" />
                <category android:name="android.intent.category.LAUNCHER" />
            </intent-filter>
        </activity>
    </application>
</manifest>`

	axml, err := CompileAXML([]byte(xmlText))
	if err != nil {
		t.Fatalf("CompileAXML failed: %v", err)
	}

	if len(axml) < 8 {
		t.Fatalf("AXML too short: %d bytes", len(axml))
	}

	// Verify chunk type RES_XML_TYPE (0x0003)
	chunkType := binary.LittleEndian.Uint16(axml[0:2])
	if chunkType != ResXMLType {
		t.Fatalf("expected chunk type 0x0003, got 0x%04x", chunkType)
	}

	totalSize := binary.LittleEndian.Uint32(axml[4:8])
	if int(totalSize) != len(axml) {
		t.Fatalf("AXML total size mismatch: recorded %d, actual %d", totalSize, len(axml))
	}
}

func TestScaffold(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "alca_android_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	err = Scaffold(tmpDir, "CoolApp")
	if err != nil {
		t.Fatalf("Scaffold failed: %v", err)
	}

	expectedFiles := []string{
		"alca.toml",
		"AndroidManifest.xml",
		filepath.Join("src", "main.al"),
		filepath.Join("res", "values", "strings.xml"),
		filepath.Join("res", "mipmap", "ic_launcher.png"),
		filepath.Join("assets", "index.html"),
		filepath.Join("assets", "style.css"),
		filepath.Join("assets", "app.js"),
	}

	for _, rel := range expectedFiles {
		p := filepath.Join(tmpDir, rel)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("missing expected scaffold file: %s", rel)
		}
	}
}

func TestLoadConfigFull(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "alca_config_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	tomlContent := `[package]
name = "awesome_app"
version = "2.3.4"
description = "Awesome Mobile Experience"
entry = "src/main.al"

[android]
package = "org.alca.awesome"
label = "Awesome App"
version_code = 42
version_name = "2.3.4"
min_sdk = 26
target_sdk = 34
orientation = "portrait"
permissions = ["android.permission.CAMERA", "android.permission.VIBRATE"]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "alca.toml"), []byte(tomlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.PackageName != "org.alca.awesome" {
		t.Errorf("expected package org.alca.awesome, got %s", cfg.PackageName)
	}
	if cfg.VersionCode != 42 {
		t.Errorf("expected version_code 42, got %d", cfg.VersionCode)
	}
	if cfg.MinSDK != 26 {
		t.Errorf("expected min_sdk 26, got %d", cfg.MinSDK)
	}
	if cfg.TargetSDK != 34 {
		t.Errorf("expected target_sdk 34, got %d", cfg.TargetSDK)
	}
	if cfg.Orientation != "portrait" {
		t.Errorf("expected orientation portrait, got %s", cfg.Orientation)
	}
	if len(cfg.Permissions) != 2 || cfg.Permissions[0] != "android.permission.CAMERA" {
		t.Errorf("unexpected permissions: %v (item 0: %q)", cfg.Permissions, cfg.Permissions[0])
	}
}

func TestBuildAlignedZipAndV2Signature(t *testing.T) {
	entries := map[string][]byte{
		"AndroidManifest.xml": []byte("manifest contents"),
		"classes.dex":         []byte("dex contents"),
		"res/icon.png":        []byte("uncompressed image data"),
	}

	alignedZip, err := BuildAlignedZip(entries)
	if err != nil {
		t.Fatalf("BuildAlignedZip failed: %v", err)
	}

	// Verify uncompressed entries are 4-byte aligned
	r, err := zip.NewReader(bytes.NewReader(alignedZip), int64(len(alignedZip)))
	if err != nil {
		t.Fatalf("zip.NewReader failed: %v", err)
	}

	for _, f := range r.File {
		if f.Method == zip.Store {
			// Offset of local header
			offset, _ := f.DataOffset()
			if offset%4 != 0 {
				t.Errorf("file %s is STORED but data offset %d is not 4-byte aligned!", f.Name, offset)
			}
		}
	}

	// Apply APK Signature Scheme v2
	v2SignedAPK, err := ApplyAPKSignatureSchemeV2(alignedZip)
	if err != nil {
		t.Fatalf("ApplyAPKSignatureSchemeV2 failed: %v", err)
	}

	if !bytes.Contains(v2SignedAPK, []byte("APK Sig Block 42")) {
		t.Fatalf("missing APK Signing Block magic marker in signed APK")
	}

	// Verify the signed APK can still be read by standard ZIP readers
	r2, err := zip.NewReader(bytes.NewReader(v2SignedAPK), int64(len(v2SignedAPK)))
	if err != nil {
		t.Fatalf("zip reader failed on v2 signed APK: %v", err)
	}
	if len(r2.File) != len(entries) {
		t.Fatalf("expected %d files in APK, got %d", len(entries), len(r2.File))
	}
}

func TestBuildAPKEndToEnd(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "alca_android_build_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// 1. Scaffold project
	err = Scaffold(tmpDir, "SampleMobileApp")
	if err != nil {
		t.Fatalf("Scaffold failed: %v", err)
	}

	// 2. Validate
	if err := ValidateProject(tmpDir); err != nil {
		t.Fatalf("ValidateProject failed: %v", err)
	}

	// 3. Build APK
	outAPK := filepath.Join(tmpDir, "sample.apk")
	resPath, err := BuildAPK(BuildOptions{
		ProjectDir: tmpDir,
		OutputPath: outAPK,
	})
	if err != nil {
		t.Fatalf("BuildAPK failed: %v", err)
	}

	if resPath != outAPK {
		t.Fatalf("expected APK path %s, got %s", outAPK, resPath)
	}

	fi, err := os.Stat(outAPK)
	if err != nil {
		t.Fatalf("APK file not found: %v", err)
	}

	if fi.Size() == 0 {
		t.Fatalf("APK file is empty")
	}

	// 4. Verify APK v2 signature block exists in the file
	data, err := os.ReadFile(outAPK)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("APK Sig Block 42")) {
		t.Fatalf("APK missing APK Signature Scheme v2 block!")
	}

	// 5. Inspect ZIP contents of generated APK
	zr, err := zip.OpenReader(outAPK)
	if err != nil {
		t.Fatalf("reading generated APK as ZIP: %v", err)
	}
	defer zr.Close()

	foundFiles := make(map[string]bool)
	for _, f := range zr.File {
		foundFiles[f.Name] = true
	}

	requiredInAPK := []string{
		"AndroidManifest.xml",
		"classes.dex",
		"META-INF/MANIFEST.MF",
		"META-INF/CERT.SF",
		"META-INF/CERT.RSA",
		"assets/index.html",
		"assets/style.css",
		"assets/app.js",
		"assets/alca/main.al",
		"res/mipmap/ic_launcher.png",
		"res/values/strings.xml",
	}

	for _, req := range requiredInAPK {
		if !foundFiles[req] {
			t.Errorf("expected %s in generated APK, but was not found", req)
		}
	}
}

func TestContactsAppManifestAXML(t *testing.T) {
	manifestPath := filepath.Join("..", "..", "contacts_app", "AndroidManifest.xml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Skip("contacts_app manifest not found, skipping")
	}

	axml, err := CompileAXML(data)
	if err != nil {
		t.Fatalf("CompileAXML on contacts_app failed: %v", err)
	}

	if len(axml) < 8 {
		t.Fatalf("AXML too short: %d", len(axml))
	}
	t.Logf("Compiled contacts_app AXML successfully, size=%d bytes", len(axml))
}

func TestSyncManifestPermissions(t *testing.T) {
	manifest := []byte(`<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android"
    package="com.test.app">
    <uses-permission android:name="android.permission.CAMERA" />
    <uses-permission android:name="android.permission.RECORD_AUDIO" />
    <application android:label="Test">
    </application>
</manifest>`)

	newPerms := []string{
		"android.permission.READ_CONTACTS",
		"android.permission.INTERNET",
	}

	synced := syncManifestPermissions(manifest, newPerms)
	syncedStr := string(synced)

	// Old permissions must be removed
	if strings.Contains(syncedStr, "android.permission.CAMERA") {
		t.Errorf("expected CAMERA to be removed")
	}
	if strings.Contains(syncedStr, "android.permission.RECORD_AUDIO") {
		t.Errorf("expected RECORD_AUDIO to be removed")
	}

	// New permissions must be present
	if !strings.Contains(syncedStr, "android.permission.READ_CONTACTS") {
		t.Errorf("expected READ_CONTACTS to be present")
	}
	if !strings.Contains(syncedStr, "android.permission.INTERNET") {
		t.Errorf("expected INTERNET to be present")
	}

	// Extracted permissions must match newPerms
	extracted := extractManifestPermissions(synced)
	if len(extracted) != 2 {
		t.Fatalf("expected 2 extracted permissions, got %d: %v", len(extracted), extracted)
	}
	if extracted[0] != "android.permission.READ_CONTACTS" || extracted[1] != "android.permission.INTERNET" {
		t.Errorf("extracted permissions mismatch: %v", extracted)
	}
}

func TestBuildLauncherDEXDynamicPermissions(t *testing.T) {
	// 1. With zero permissions
	dex0, err := BuildLauncherDEX("alca.runtime.MainActivity")
	if err != nil {
		t.Fatalf("dex0 failed: %v", err)
	}
	if bytes.Contains(dex0, []byte("android.permission.READ_CONTACTS")) {
		t.Errorf("dex0 should not contain READ_CONTACTS")
	}

	// 2. With install-time permissions only (should NOT emit requestPermissions)
	dexInstallOnly, err := BuildLauncherDEX("alca.runtime.MainActivity", "INTERNET", "VIBRATE")
	if err != nil {
		t.Fatalf("dexInstallOnly failed: %v", err)
	}
	if bytes.Contains(dexInstallOnly, []byte("android.permission.INTERNET")) {
		t.Errorf("dexInstallOnly should filter out install-time permissions from DEX")
	}

	// 3. With dangerous permissions
	dexPerms, err := BuildLauncherDEX("alca.runtime.MainActivity",
		"android.permission.READ_CONTACTS",
		"android.permission.WRITE_CONTACTS",
		"android.permission.INTERNET", // Should be filtered out of DEX
	)
	if err != nil {
		t.Fatalf("dexPerms failed: %v", err)
	}
	if !bytes.Contains(dexPerms, []byte("android.permission.READ_CONTACTS")) {
		t.Errorf("dexPerms must contain READ_CONTACTS")
	}
	if !bytes.Contains(dexPerms, []byte("android.permission.WRITE_CONTACTS")) {
		t.Errorf("dexPerms must contain WRITE_CONTACTS")
	}
	if bytes.Contains(dexPerms, []byte("android.permission.INTERNET")) {
		t.Errorf("dexPerms should not contain install-time INTERNET")
	}
}
