// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package stdlib

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

// TestZipExtractRejectsDecompressionBomb lowers the extraction budget and
// checks that an archive expanding past it is rejected rather than written to
// disk. Without the budget a tiny archive could fill the filesystem.
func TestZipExtractRejectsDecompressionBomb(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "bomb.zip")

	// Build a zip with a single entry larger than the budget below.
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	zw := zip.NewWriter(f)
	entry, err := zw.Create("big.txt")
	if err != nil {
		t.Fatalf("create entry: %v", err)
	}
	if _, err := entry.Write(make([]byte, 4096)); err != nil {
		t.Fatalf("write entry: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close file: %v", err)
	}

	// Lower the budget so the entry no longer fits.
	saved := maxExtractedBytes
	maxExtractedBytes = 1024
	defer func() { maxExtractedBytes = saved }()

	dest := filepath.Join(dir, "out")
	if err := ZipExtract(zipPath, dest); err == nil {
		t.Fatal("expected extraction past the budget to fail, got nil")
	}
}

// TestTarGzExtractRejectsDecompressionBomb mirrors the zip bomb check for the
// tar.gz path.
func TestTarGzExtractRejectsDecompressionBomb(t *testing.T) {
	dir := t.TempDir()
	tarPath := filepath.Join(dir, "bomb.tar.gz")

	f, err := os.Create(tarPath)
	if err != nil {
		t.Fatalf("create tar: %v", err)
	}
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	header := &tar.Header{Name: "big.txt", Mode: 0644, Size: 4096, Typeflag: tar.TypeReg}
	if err := tw.WriteHeader(header); err != nil {
		t.Fatalf("write tar header: %v", err)
	}
	if _, err := tw.Write(make([]byte, 4096)); err != nil {
		t.Fatalf("write tar entry: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close file: %v", err)
	}

	saved := maxExtractedBytes
	maxExtractedBytes = 1024
	defer func() { maxExtractedBytes = saved }()

	dest := filepath.Join(dir, "out")
	if err := TarGzExtract(tarPath, dest); err == nil {
		t.Fatal("expected extraction past the budget to fail, got nil")
	}
}
