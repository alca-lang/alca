// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package android

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"hash/crc32"
	"sort"
	"time"
)

// ZipEntry represents a file to be placed into a zipaligned APK.
type ZipEntry struct {
	Name     string
	Data     []byte
	Compress bool
}

// BuildAlignedZip creates a deterministic, 4-byte zipaligned ZIP archive from a map of entries.
func BuildAlignedZip(entries map[string][]byte) ([]byte, error) {
	var names []string
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)

	var entryList []ZipEntry
	// Priority order: META-INF first, then AndroidManifest.xml, classes.dex, then others
	isMeta := func(n string) bool { return stringsHasPrefix(n, "META-INF/") }
	for _, n := range names {
		if isMeta(n) {
			entryList = append(entryList, ZipEntry{
				Name:     n,
				Data:     entries[n],
				Compress: false, // META-INF signatures are stored uncompressed
			})
		}
	}
	for _, n := range names {
		if !isMeta(n) {
			compress := true
			// Uncompressed extensions for assets and resources
			if stringsHasSuffix(n, ".png") || stringsHasSuffix(n, ".jpg") || stringsHasSuffix(n, ".arsc") || stringsHasSuffix(n, ".so") {
				compress = false
			}
			entryList = append(entryList, ZipEntry{
				Name:     n,
				Data:     entries[n],
				Compress: compress,
			})
		}
	}

	return PackAlignedZip(entryList)
}

// PackAlignedZip packs a list of entries into a 4-byte zipaligned ZIP archive.
func PackAlignedZip(entries []ZipEntry) ([]byte, error) {
	var bodyBuf bytes.Buffer
	var cdBuf bytes.Buffer

	// Fixed mod time for deterministic builds
	fixedTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	modTime := uint16(fixedTime.Hour()<<11 | fixedTime.Minute()<<5 | fixedTime.Second()/2)
	modDate := uint16((fixedTime.Year()-1980)<<9 | int(fixedTime.Month())<<5 | fixedTime.Day())

	for _, entry := range entries {
		nameBytes := []byte(entry.Name)
		nameLen := uint16(len(nameBytes))
		crc := crc32.ChecksumIEEE(entry.Data)
		uncompSize := uint32(len(entry.Data))

		var compData []byte
		var method uint16 = 0 // Store

		if entry.Compress {
			var defBuf bytes.Buffer
			fw, err := flate.NewWriter(&defBuf, flate.BestCompression)
			if err == nil {
				_, _ = fw.Write(entry.Data)
				_ = fw.Close()
				if defBuf.Len() < len(entry.Data) {
					compData = defBuf.Bytes()
					method = 8 // Deflate
				}
			}
		}

		if compData == nil {
			compData = entry.Data
			method = 0
		}
		compSize := uint32(len(compData))

		localHeaderOffset := uint32(bodyBuf.Len())

		// Calculate 4-byte alignment padding for STORED entries
		var extra []byte
		if method == 0 {
			dataOffset := localHeaderOffset + 30 + uint32(nameLen)
			padding := (4 - (dataOffset % 4)) % 4
			if padding > 0 {
				extra = make([]byte, padding)
			}
		}
		extraLen := uint16(len(extra))

		// 1. Write Local File Header (30 bytes)
		_ = binary.Write(&bodyBuf, binary.LittleEndian, uint32(0x04034b50))
		_ = binary.Write(&bodyBuf, binary.LittleEndian, uint16(20)) // version needed
		_ = binary.Write(&bodyBuf, binary.LittleEndian, uint16(0))  // flags
		_ = binary.Write(&bodyBuf, binary.LittleEndian, method)
		_ = binary.Write(&bodyBuf, binary.LittleEndian, modTime)
		_ = binary.Write(&bodyBuf, binary.LittleEndian, modDate)
		_ = binary.Write(&bodyBuf, binary.LittleEndian, crc)
		_ = binary.Write(&bodyBuf, binary.LittleEndian, compSize)
		_ = binary.Write(&bodyBuf, binary.LittleEndian, uncompSize)
		_ = binary.Write(&bodyBuf, binary.LittleEndian, nameLen)
		_ = binary.Write(&bodyBuf, binary.LittleEndian, extraLen)
		bodyBuf.Write(nameBytes)
		if extraLen > 0 {
			bodyBuf.Write(extra)
		}

		// 2. Write File Data
		bodyBuf.Write(compData)

		// 3. Write Central Directory Header (46 bytes)
		_ = binary.Write(&cdBuf, binary.LittleEndian, uint32(0x02014b50))
		_ = binary.Write(&cdBuf, binary.LittleEndian, uint16(0x0314)) // version made by (Unix 2.0)
		_ = binary.Write(&cdBuf, binary.LittleEndian, uint16(20))     // version needed
		_ = binary.Write(&cdBuf, binary.LittleEndian, uint16(0))      // flags
		_ = binary.Write(&cdBuf, binary.LittleEndian, method)
		_ = binary.Write(&cdBuf, binary.LittleEndian, modTime)
		_ = binary.Write(&cdBuf, binary.LittleEndian, modDate)
		_ = binary.Write(&cdBuf, binary.LittleEndian, crc)
		_ = binary.Write(&cdBuf, binary.LittleEndian, compSize)
		_ = binary.Write(&cdBuf, binary.LittleEndian, uncompSize)
		_ = binary.Write(&cdBuf, binary.LittleEndian, nameLen)
		_ = binary.Write(&cdBuf, binary.LittleEndian, extraLen)
		_ = binary.Write(&cdBuf, binary.LittleEndian, uint16(0))          // comment len
		_ = binary.Write(&cdBuf, binary.LittleEndian, uint16(0))          // disk number
		_ = binary.Write(&cdBuf, binary.LittleEndian, uint16(0))          // internal attrs
		_ = binary.Write(&cdBuf, binary.LittleEndian, uint32(0x81a40000)) // external attrs (0644)
		_ = binary.Write(&cdBuf, binary.LittleEndian, localHeaderOffset)
		cdBuf.Write(nameBytes)
		if extraLen > 0 {
			cdBuf.Write(extra)
		}
	}

	cdOffset := uint32(bodyBuf.Len())
	cdSize := uint32(cdBuf.Len())
	entryCount := uint16(len(entries))

	// 4. Write End of Central Directory (EOCD, 22 bytes)
	var eocdBuf bytes.Buffer
	_ = binary.Write(&eocdBuf, binary.LittleEndian, uint32(0x06054b50))
	_ = binary.Write(&eocdBuf, binary.LittleEndian, uint16(0)) // disk number
	_ = binary.Write(&eocdBuf, binary.LittleEndian, uint16(0)) // start disk
	_ = binary.Write(&eocdBuf, binary.LittleEndian, entryCount)
	_ = binary.Write(&eocdBuf, binary.LittleEndian, entryCount)
	_ = binary.Write(&eocdBuf, binary.LittleEndian, cdSize)
	_ = binary.Write(&eocdBuf, binary.LittleEndian, cdOffset)
	_ = binary.Write(&eocdBuf, binary.LittleEndian, uint16(0)) // comment len

	var finalZip bytes.Buffer
	finalZip.Write(bodyBuf.Bytes())
	finalZip.Write(cdBuf.Bytes())
	finalZip.Write(eocdBuf.Bytes())

	return finalZip.Bytes(), nil
}

func stringsHasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func stringsHasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}
