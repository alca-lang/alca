// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

package link

import (
	"bytes"
	"encoding/binary"
	"os"

	"github.com/pawl-lang/pawl/pkg/codegen/amd64"
	"github.com/pawl-lang/pawl/pkg/jit"
)

// Standard ELF64 constants
const (
	elfBaseAddr   uint64 = 0x400000
	elfHeaderSize uint64 = 64
	elfPHTSize    uint64 = 56
)

// LinkELF creates a standalone, static 64-bit ELF executable for Linux x86-64.
// It contains a _start entry point that calls main() and invokes sys_exit (syscall 60).
func LinkELF(art *amd64.CompiledArtifact, outputPath string) error {
	// _start bootstrap:
	//   call main
	//   mov rdi, rax     ; exit code
	//   mov eax, 60      ; sys_exit syscall number
	//   syscall
	bootstrap := jit.NewAssemblerAMD64()

	// call main (rel32)
	bootstrap.EmitByte(0xE8)
	callPatch := bootstrap.CodeLen()
	bootstrap.EmitUint32(0) // placeholder

	// mov rdi, rax -> 48 89 C7
	bootstrap.EmitBytes(0x48, 0x89, 0xC7)

	// mov eax, 60 -> B8 3C 00 00 00
	bootstrap.EmitByte(0xB8)
	bootstrap.EmitUint32(60)

	// syscall -> 0F 05
	bootstrap.EmitBytes(0x0F, 0x05)

	for bootstrap.CodeLen()%16 != 0 {
		bootstrap.Nop()
	}

	startCode, err := bootstrap.Finalize()
	if err != nil {
		return err
	}

	targetMain := int32(len(startCode) + int(art.EntryOffset))
	relMain := targetMain - int32(callPatch+4)
	binary.LittleEndian.PutUint32(startCode[callPatch:], uint32(relMain))

	textBuf := new(bytes.Buffer)
	textBuf.Write(startCode)
	textBuf.Write(art.Code)
	textBytes := textBuf.Bytes()

	// ELF Layout:
	// 0x00..0x3F: ELF Header (64 bytes)
	// 0x40..0x77: Program Header (56 bytes, 1 load segment: RX)
	// Pad to 0x1000 page boundary
	// 0x1000..: .text code segment

	fileHdrSize := elfHeaderSize + elfPHTSize
	fileOffset := uint64(0x1000)
	vaddr := elfBaseAddr + fileOffset

	out := new(bytes.Buffer)

	// 1. ELF Header (64 bytes)
	// e_ident: \x7fELF, 64-bit (2), little-endian (1), version 1, sysv (0)
	var eIdent [16]byte
	copy(eIdent[:], []byte("\x7fELF\x02\x01\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00"))
	out.Write(eIdent[:])

	_ = binary.Write(out, binary.LittleEndian, uint16(2))             // e_type = ET_EXEC
	_ = binary.Write(out, binary.LittleEndian, uint16(0x3E))          // e_machine = AMD64 (62)
	_ = binary.Write(out, binary.LittleEndian, uint32(1))             // e_version = 1
	_ = binary.Write(out, binary.LittleEndian, uint64(vaddr))         // e_entry = _start virtual address
	_ = binary.Write(out, binary.LittleEndian, uint64(elfHeaderSize)) // e_phoff = 64
	_ = binary.Write(out, binary.LittleEndian, uint64(0))             // e_shoff = 0 (no section headers needed for static execution)
	_ = binary.Write(out, binary.LittleEndian, uint32(0))             // e_flags = 0
	_ = binary.Write(out, binary.LittleEndian, uint16(64))            // e_ehsize = 64
	_ = binary.Write(out, binary.LittleEndian, uint16(56))            // e_phentsize = 56
	_ = binary.Write(out, binary.LittleEndian, uint16(1))             // e_phnum = 1 (LOAD segment)
	_ = binary.Write(out, binary.LittleEndian, uint16(64))            // e_shentsize = 64
	_ = binary.Write(out, binary.LittleEndian, uint16(0))             // e_shnum = 0
	_ = binary.Write(out, binary.LittleEndian, uint16(0))             // e_shstrndx = 0

	// 2. Program Header Table (1 entry = 56 bytes)
	// PT_LOAD: Read + Execute (PF_R | PF_X = 5)
	_ = binary.Write(out, binary.LittleEndian, uint32(1))                  // p_type = PT_LOAD
	_ = binary.Write(out, binary.LittleEndian, uint32(5))                  // p_flags = PF_R | PF_X
	_ = binary.Write(out, binary.LittleEndian, uint64(0))                  // p_offset = 0 (load from beginning of file)
	_ = binary.Write(out, binary.LittleEndian, uint64(elfBaseAddr))        // p_vaddr
	_ = binary.Write(out, binary.LittleEndian, uint64(elfBaseAddr))        // p_paddr
	segSize := fileOffset + uint64(len(textBytes))
	_ = binary.Write(out, binary.LittleEndian, uint64(segSize))            // p_filesz
	_ = binary.Write(out, binary.LittleEndian, uint64(segSize))            // p_memsz
	_ = binary.Write(out, binary.LittleEndian, uint64(0x1000))             // p_align = 4KB

	// Pad header to fileOffset (0x1000)
	for uint64(out.Len()) < fileOffset {
		out.WriteByte(0)
	}

	// 3. .text section
	out.Write(textBytes)

	_ = fileHdrSize
	return os.WriteFile(outputPath, out.Bytes(), 0755)
}
