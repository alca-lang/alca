// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package android

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"hash/adler32"
	"testing"
)

func TestBuildLauncherDEXDeepValidation(t *testing.T) {
	className := "alca.runtime.MainActivity"
	dex, err := BuildLauncherDEX(className)
	if err != nil {
		t.Fatalf("BuildLauncherDEX failed: %v", err)
	}

	if len(dex) < 0x70 {
		t.Fatalf("DEX too short: %d", len(dex))
	}

	// 1. Magic
	if string(dex[:8]) != "dex\n035\x00" {
		t.Fatalf("bad magic: %q", dex[:8])
	}

	// 2. Checksum and signature
	fileSize := binary.LittleEndian.Uint32(dex[0x20:0x24])
	if int(fileSize) != len(dex) {
		t.Fatalf("recorded size %d != actual %d", fileSize, len(dex))
	}

	sha := sha1.Sum(dex[32:])
	if !bytes.Equal(sha[:], dex[12:32]) {
		t.Fatalf("SHA-1 mismatch")
	}

	adler := adler32.Checksum(dex[12:])
	fileAdler := binary.LittleEndian.Uint32(dex[8:12])
	if adler != fileAdler {
		t.Fatalf("Adler32 mismatch: got %x, want %x", fileAdler, adler)
	}

	// Read header fields
	stringIdsSize := binary.LittleEndian.Uint32(dex[0x38:0x3c])
	stringIdsOff := binary.LittleEndian.Uint32(dex[0x3c:0x40])
	typeIdsSize := binary.LittleEndian.Uint32(dex[0x40:0x44])
	typeIdsOff := binary.LittleEndian.Uint32(dex[0x44:0x48])
	protoIdsSize := binary.LittleEndian.Uint32(dex[0x48:0x4c])
	protoIdsOff := binary.LittleEndian.Uint32(dex[0x4c:0x50])
	methodIdsSize := binary.LittleEndian.Uint32(dex[0x58:0x5c])
	methodIdsOff := binary.LittleEndian.Uint32(dex[0x5c:0x60])
	classDefsSize := binary.LittleEndian.Uint32(dex[0x60:0x64])
	classDefsOff := binary.LittleEndian.Uint32(dex[0x64:0x68])
	mapOff := binary.LittleEndian.Uint32(dex[0x34:0x38])

	// Validate proto section offset
	if protoIdsOff != typeIdsOff+typeIdsSize*4 {
		t.Fatalf("unexpected protoIdsOff: %d", protoIdsOff)
	}

	// Validate method section offset
	if methodIdsOff != protoIdsOff+protoIdsSize*12 {
		t.Fatalf("unexpected methodIdsOff: %d", methodIdsOff)
	}

	// 3. Validate Strings
	strings := make([]string, stringIdsSize)
	for i := uint32(0); i < stringIdsSize; i++ {
		off := binary.LittleEndian.Uint32(dex[stringIdsOff+i*4 : stringIdsOff+i*4+4])
		// Read uleb128 string length
		pos := off
		var strLen uint32
		var shift uint
		for {
			b := dex[pos]
			pos++
			strLen |= uint32(b&0x7F) << shift
			if b&0x80 == 0 {
				break
			}
			shift += 7
		}
		strData := string(dex[pos : pos+strLen])
		strings[i] = strData
		if i > 0 && strings[i] <= strings[i-1] {
			t.Fatalf("strings not sorted: [%d]=%q <= [%d]=%q", i, strings[i], i-1, strings[i-1])
		}
	}

	// 4. Validate Types
	types := make([]string, typeIdsSize)
	for i := uint32(0); i < typeIdsSize; i++ {
		strIdx := binary.LittleEndian.Uint32(dex[typeIdsOff+i*4 : typeIdsOff+i*4+4])
		if strIdx >= stringIdsSize {
			t.Fatalf("type %d has invalid strIdx %d", i, strIdx)
		}
		types[i] = strings[strIdx]
		if i > 0 && strIdx <= binary.LittleEndian.Uint32(dex[typeIdsOff+(i-1)*4:typeIdsOff+(i-1)*4+4]) {
			t.Fatalf("type_ids not sorted by string_id")
		}
	}

	// Verify required types exist
	findType := func(name string) int {
		for i, t := range types {
			if t == name {
				return i
			}
		}
		return -1
	}

	actIdx := findType("Lalca/runtime/MainActivity;")
	if actIdx < 0 {
		t.Fatalf("MainActivity type not found in DEX")
	}
	superIdx := findType("Landroid/app/Activity;")
	if superIdx < 0 {
		t.Fatalf("Activity superclass type not found in DEX")
	}

	// 5. Validate Class Def
	if classDefsSize != 1 {
		t.Fatalf("expected 1 class def, got %d", classDefsSize)
	}
	cdClassIdx := binary.LittleEndian.Uint32(dex[classDefsOff : classDefsOff+4])
	cdSuperIdx := binary.LittleEndian.Uint32(dex[classDefsOff+8 : classDefsOff+12])
	classDataOff := binary.LittleEndian.Uint32(dex[classDefsOff+24 : classDefsOff+28])

	if int(cdClassIdx) != actIdx {
		t.Fatalf("class_idx mismatch: got %d (%s), want %d", cdClassIdx, types[cdClassIdx], actIdx)
	}
	if int(cdSuperIdx) != superIdx {
		t.Fatalf("superclass mismatch: got %d, want %d", cdSuperIdx, superIdx)
	}
	if classDataOff == 0 {
		t.Fatalf("class_data_off is 0")
	}

	// 6. Validate Map List
	mapCount := binary.LittleEndian.Uint32(dex[mapOff : mapOff+4])
	if mapCount < 8 {
		t.Fatalf("map count too small: %d", mapCount)
	}

	t.Logf("DEX successfully verified! Size=%d bytes, Strings=%d, Types=%d, Protos=%d, Methods=%d",
		len(dex), stringIdsSize, typeIdsSize, protoIdsSize, methodIdsSize)
}
