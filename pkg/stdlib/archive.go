// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

// Package stdlib provides the Archive standard library module for the Alca programming language.
// It supports creating and extracting ZIP and TAR.GZ archives with built-in path-traversal protection.
package stdlib

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/sandbox"
)

// maxExtractedBytes caps the total number of bytes a single extraction call
// will write to disk. It bounds the damage of a decompression bomb (a tiny
// archive that expands to fill the disk); extraction aborts once the budget is
// exhausted. It is a variable so tests can lower it.
var maxExtractedBytes int64 = 1 << 30 // 1 GiB

// ZipCreate archives a file or directory into a ZIP file.
func ZipCreate(srcPath, destZip string) (err error) {
	if err := sandbox.RequireRead(srcPath); err != nil {
		return err
	}
	if err := sandbox.RequireWrite(destZip); err != nil {
		return err
	}
	zipFile, err := os.Create(destZip)
	if err != nil {
		return err
	}
	archive := zip.NewWriter(zipFile)
	defer func() {
		// archive.Close flushes the central directory; zipFile.Close flushes
		// buffered bytes. Both can fail (e.g. disk full) and would otherwise be
		// silently dropped, so surface the first error that occurs.
		if cerr := archive.Close(); cerr != nil && err == nil {
			err = cerr
		}
		if cerr := zipFile.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return err
	}

	baseDir := ""
	if srcInfo.IsDir() {
		baseDir = filepath.Clean(srcPath)
	} else {
		baseDir = filepath.Dir(srcPath)
	}

	return filepath.Walk(srcPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(baseDir, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}

		// Use forward slashes for zip internal paths
		zipPath := filepath.ToSlash(relPath)

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = zipPath
		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}

		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		return err
	})
}

// ZipExtract safely unpacks a ZIP archive into the destination directory.
func ZipExtract(zipPath, destDir string) error {
	if err := sandbox.RequireRead(zipPath); err != nil {
		return err
	}
	if err := sandbox.RequireWrite(destDir); err != nil {
		return err
	}
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	cleanDest := filepath.Clean(destDir)
	remaining := maxExtractedBytes

	for _, f := range r.File {
		targetPath := filepath.Join(cleanDest, f.Name)
		cleanTarget := filepath.Clean(targetPath)

		// Security: Prevent Zip Slip path traversal vulnerability
		if cleanTarget != cleanDest && !strings.HasPrefix(cleanTarget, cleanDest+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path in zip archive: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(cleanTarget, f.Mode()); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(cleanTarget), 0755); err != nil {
			return err
		}

		outFile, err := os.OpenFile(cleanTarget, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		// Read one extra byte past the remaining budget so an over-large entry
		// is detected rather than silently truncated.
		written, err := io.Copy(outFile, io.LimitReader(rc, remaining+1))
		rc.Close()
		if cerr := outFile.Close(); cerr != nil && err == nil {
			err = cerr
		}
		if err != nil {
			return err
		}
		remaining -= written
		if remaining < 0 {
			return fmt.Errorf("zip archive expands beyond the %d byte limit", maxExtractedBytes)
		}
	}

	return nil
}

// TarGzCreate packages a file or directory into a compressed .tar.gz archive.
func TarGzCreate(srcPath, destTarGz string) (err error) {
	if err := sandbox.RequireRead(srcPath); err != nil {
		return err
	}
	if err := sandbox.RequireWrite(destTarGz); err != nil {
		return err
	}
	tarFile, err := os.Create(destTarGz)
	if err != nil {
		return err
	}
	gw := gzip.NewWriter(tarFile)
	tw := tar.NewWriter(gw)
	defer func() {
		// Close in inner-to-outer order: tw writes the tar end-of-archive
		// marker, gw writes the gzip footer (including the CRC), and tarFile
		// flushes the file. Each can fail and would otherwise be dropped.
		if cerr := tw.Close(); cerr != nil && err == nil {
			err = cerr
		}
		if cerr := gw.Close(); cerr != nil && err == nil {
			err = cerr
		}
		if cerr := tarFile.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return err
	}

	baseDir := ""
	if srcInfo.IsDir() {
		baseDir = filepath.Clean(srcPath)
	} else {
		baseDir = filepath.Dir(srcPath)
	}

	return filepath.Walk(srcPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(baseDir, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}

		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relPath)

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(tw, file)
		return err
	})
}

// TarGzExtract safely unpacks a .tar.gz archive into destDir.
func TarGzExtract(tarGzPath, destDir string) error {
	if err := sandbox.RequireRead(tarGzPath); err != nil {
		return err
	}
	if err := sandbox.RequireWrite(destDir); err != nil {
		return err
	}
	file, err := os.Open(tarGzPath)
	if err != nil {
		return err
	}
	defer file.Close()

	gr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	cleanDest := filepath.Clean(destDir)
	remaining := maxExtractedBytes

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		targetPath := filepath.Join(cleanDest, header.Name)
		cleanTarget := filepath.Clean(targetPath)

		// Security: Prevent Tar Slip path traversal
		if cleanTarget != cleanDest && !strings.HasPrefix(cleanTarget, cleanDest+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path in tar archive: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(cleanTarget, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(cleanTarget), 0755); err != nil {
				return err
			}
			outFile, err := os.OpenFile(cleanTarget, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, header.FileInfo().Mode())
			if err != nil {
				return err
			}
			written, err := io.Copy(outFile, io.LimitReader(tr, remaining+1))
			if cerr := outFile.Close(); cerr != nil && err == nil {
				err = cerr
			}
			if err != nil {
				return err
			}
			remaining -= written
			if remaining < 0 {
				return fmt.Errorf("tar archive expands beyond the %d byte limit", maxExtractedBytes)
			}
		}
	}
	return nil
}

// RegisterArchiveBuiltins registers the "std.archive" and "archive" modules into the interpreter environment.
func RegisterArchiveBuiltins(env *interpreter.Environment) {
	archiveModule := interpreter.NewMap()

	register := func(name string, fn func(args ...interpreter.Value) interpreter.Value) {
		bf := &interpreter.BuiltinFunction{Name: name, Fn: fn}
		archiveModule.Set(name, bf)
	}

	// zip_create(src, zip_file) -> bool
	register("zip_create", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "archive.zip_create requires (src_path, dest_zip)")
		}
		src := interpreter.CoerceToString(args[0])
		dest := interpreter.CoerceToString(args[1])
		if err := ZipCreate(src, dest); err != nil {
			return handleIOError("zip_create", src+" -> "+dest, err)
		}
		return interpreter.TRUE
	})

	// zip_extract(zip_file, dest_dir) -> bool
	register("zip_extract", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "archive.zip_extract requires (zip_file, dest_dir)")
		}
		zipPath := interpreter.CoerceToString(args[0])
		dest := interpreter.CoerceToString(args[1])
		if err := ZipExtract(zipPath, dest); err != nil {
			return handleIOError("zip_extract", zipPath+" -> "+dest, err)
		}
		return interpreter.TRUE
	})

	// tar_create(src, tar_gz_file) -> bool
	register("tar_create", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "archive.tar_create requires (src_path, dest_tar_gz)")
		}
		src := interpreter.CoerceToString(args[0])
		dest := interpreter.CoerceToString(args[1])
		if err := TarGzCreate(src, dest); err != nil {
			return handleIOError("tar_create", src+" -> "+dest, err)
		}
		return interpreter.TRUE
	})

	// tar_extract(tar_gz_file, dest_dir) -> bool
	register("tar_extract", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "archive.tar_extract requires (tar_gz_file, dest_dir)")
		}
		tarPath := interpreter.CoerceToString(args[0])
		dest := interpreter.CoerceToString(args[1])
		if err := TarGzExtract(tarPath, dest); err != nil {
			return handleIOError("tar_extract", tarPath+" -> "+dest, err)
		}
		return interpreter.TRUE
	})

	// list(archive_path) -> list of maps {name, size, is_dir}
	register("list", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewList()
		}
		path := interpreter.CoerceToString(args[0])
		if err := sandbox.RequireRead(path); err != nil {
			return handleIOError("archive_list", path, err)
		}
		var entries []interpreter.Value

		if strings.HasSuffix(strings.ToLower(path), ".zip") {
			r, err := zip.OpenReader(path)
			if err != nil {
				return interpreter.NewError(500, err.Error())
			}
			defer r.Close()

			for _, f := range r.File {
				m := interpreter.NewMap()
				m.Set("name", &interpreter.StringValue{Val: f.Name})
				m.Set("size", &interpreter.IntegerValue{Val: int64(f.UncompressedSize64)})
				if f.FileInfo().IsDir() {
					m.Set("is_dir", interpreter.TRUE)
				} else {
					m.Set("is_dir", interpreter.FALSE)
				}
				entries = append(entries, m)
			}
		} else if strings.HasSuffix(strings.ToLower(path), ".tar.gz") || strings.HasSuffix(strings.ToLower(path), ".tgz") {
			f, err := os.Open(path)
			if err != nil {
				return interpreter.NewError(500, err.Error())
			}
			defer f.Close()

			gr, err := gzip.NewReader(f)
			if err != nil {
				return interpreter.NewError(500, err.Error())
			}
			defer gr.Close()

			tr := tar.NewReader(gr)
			for {
				h, err := tr.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					return interpreter.NewError(500, err.Error())
				}
				m := interpreter.NewMap()
				m.Set("name", &interpreter.StringValue{Val: h.Name})
				m.Set("size", &interpreter.IntegerValue{Val: h.Size})
				if h.Typeflag == tar.TypeDir {
					m.Set("is_dir", interpreter.TRUE)
				} else {
					m.Set("is_dir", interpreter.FALSE)
				}
				entries = append(entries, m)
			}
		}
		return interpreter.NewList(entries...)
	})

	env.Set("archive", archiveModule)
	env.Set("std.archive", archiveModule)
	env.Set("zip", archiveModule)
	env.Set("std.zip", archiveModule)
}
