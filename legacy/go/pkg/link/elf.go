// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

package link

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"

	"github.com/pawl-lang/pawl/pkg/codegen/amd64"
	"github.com/pawl-lang/pawl/pkg/jit"
)

const (
	elfBaseAddr   uint64 = 0x400000
	elfHeaderSize uint64 = 64
	elfPHTSize    uint64 = 56
)

// LinkELF creates a standalone, static 64-bit ELF executable for Linux x86-64 with runtime.
func LinkELF(art *amd64.CompiledArtifact, outputPath string) error {
	// Prefix rodata with newline "\n\0" at offset 0
	newlineBytes := []byte("\n\x00")
	var rodataBuf bytes.Buffer
	rodataBuf.Write(newlineBytes)
	rodataBuf.Write(art.ReadOnlyData)
	rodataBytes := rodataBuf.Bytes()

	codeBuf := new(bytes.Buffer)

	// In our static ELF layout:
	// Page 0 (0x400000..0x400FFF): ELF headers
	// Page 1 (0x401000..): .text (RX)
	// Page 2 (0x402000..): .rodata (R)
	textBase := elfBaseAddr + 0x1000
	rodataBase := elfBaseAddr + 0x2000

	// 1. _start
	asm := jit.NewAssemblerAMD64()

	// Initialize bump allocator in R15 register:
	// sys_brk(0) to get current break
	asm.EmitBytes(0x48, 0x31, 0xFF) // xor rdi, rdi
	asm.EmitByte(0xB8)
	asm.EmitUint32(12)              // mov eax, 12 (sys_brk)
	asm.EmitBytes(0x0F, 0x05)       // syscall
	asm.EmitBytes(0x49, 0x89, 0xC7) // mov r15, rax (r15 = current break)

	// sys_brk(current + 16MB)
	asm.EmitBytes(0x48, 0x89, 0xC7)       // mov rdi, rax
	asm.EmitBytes(0x48, 0x81, 0xC7)       // add rdi, 0x1000000 (16MB)
	asm.EmitUint32(0x1000000)
	asm.EmitByte(0xB8)
	asm.EmitUint32(12)                    // sys_brk
	asm.EmitBytes(0x0F, 0x05)             // syscall

	// xor r14, r14 (clear lastErr)
	asm.EmitBytes(0x4D, 0x31, 0xF6)

	asm.EmitByte(0xE8) // call main
	callMainPatch := asm.CodeLen()
	asm.EmitUint32(0)

	// sys_exit(eax)
	asm.EmitBytes(0x48, 0x89, 0xC7) // mov rdi, rax
	asm.EmitByte(0xB8)
	asm.EmitUint32(60)              // mov eax, 60 (sys_exit)
	asm.EmitBytes(0x0F, 0x05)       // syscall

	for asm.CodeLen()%16 != 0 {
		asm.Nop()
	}
	startBytes, err := asm.Finalize()
	if err != nil {
		return err
	}
	codeBuf.Write(startBytes)

	// 2. pawl_print(str_ptr in RDI)
	// If RDI >= 0x10000000, it's a dynamic heap string: [rdi] = len, [rdi + 8] = data
	// Otherwise, it's an interned rodata offset: (rodataBase + 2 + rdi)
	printOff := uint32(codeBuf.Len())
	asmPrint := jit.NewAssemblerAMD64()
	// mov rax, rdi
	asmPrint.MovRegReg(jit.RAX, jit.RDI)
	// mov r8, 0x10000000
	asmPrint.MovRegImm64(jit.R8, 0x10000000)
	// cmp rax, r8
	asmPrint.CmpRegReg(jit.RAX, jit.R8)
	// jae .Lheap (+13 bytes over mov rax, imm64 and add rax, rdi)
	// In x86-64, movabs rax, imm64 is 10 bytes (48 b8 ...), add rax, rdi is 3 bytes (48 01 f8) -> 13 bytes!
	asmPrint.EmitBytes(0x73, 0x0D)
	asmPrint.MovRegImm64(jit.RAX, rodataBase+2) // string at rodataBase + 2 (after "\n\0")
	asmPrint.AddRegReg(jit.RAX, jit.RDI)

	// .Lheap:
	asmPrint.MovRegMem(jit.RDX, jit.RAX, 0) // len
	asmPrint.AddRegImm32(jit.RAX, 8)        // data
	asmPrint.MovRegReg(jit.RSI, jit.RAX)    // buf
	asmPrint.MovRegImm32(jit.RDI, 1)        // stdout (fd = 1)
	asmPrint.MovRegImm32(jit.RAX, 1)        // sys_write syscall (1)
	asmPrint.EmitBytes(0x0F, 0x05)          // syscall
	asmPrint.Ret()

	for asmPrint.CodeLen()%16 != 0 {
		asmPrint.Nop()
	}
	pBytes, _ := asmPrint.Finalize()
	codeBuf.Write(pBytes)

	// 3. pawl_println(str_ptr in RDI)
	printlnOff := uint32(codeBuf.Len())
	asmPrintln := jit.NewAssemblerAMD64()
	// call pawl_print
	asmPrintln.EmitByte(0xE8)
	callPrintDisp := int32(printOff) - int32(printlnOff+uint32(asmPrintln.CodeLen()+4))
	asmPrintln.EmitUint32(uint32(callPrintDisp))

	// print newline "\n"
	asmPrintln.MovRegImm64(jit.RSI, rodataBase) // buf = "\n"
	asmPrintln.MovRegImm32(jit.RDX, 1)          // len = 1
	asmPrintln.MovRegImm32(jit.RDI, 1)          // fd = 1
	asmPrintln.MovRegImm32(jit.RAX, 1)          // sys_write
	asmPrintln.EmitBytes(0x0F, 0x05)            // syscall
	asmPrintln.Ret()

	for asmPrintln.CodeLen()%16 != 0 {
		asmPrintln.Nop()
	}
	plBytes, _ := asmPrintln.Finalize()
	codeBuf.Write(plBytes)

	// 4. pawl_alloc(size in RDI) -> RAX
	allocOff := uint32(codeBuf.Len())
	asmAlloc := jit.NewAssemblerAMD64()
	// add rdi, 15
	asmAlloc.EmitBytes(0x48, 0x83, 0xC7, 0x0F)
	// and rdi, -16
	asmAlloc.EmitBytes(0x48, 0x83, 0xE7, 0xF0)
	// mov rax, r15 (return old bump pointer)
	asmAlloc.EmitBytes(0x4C, 0x89, 0xF8)
	// add r15, rdi (advance bump pointer)
	asmAlloc.EmitBytes(0x49, 0x01, 0xFF)
	asmAlloc.Ret()

	for asmAlloc.CodeLen()%16 != 0 {
		asmAlloc.Nop()
	}
	alBytes, _ := asmAlloc.Finalize()
	codeBuf.Write(alBytes)

	// 5. pawl_int_to_str(n in RDI) -> RAX (ptr to string)
	intToStrOff := uint32(codeBuf.Len())
	intToStrBytes := []byte{
		0x55, 0x48, 0x89, 0xE5, 0x53, 0x56, 0x57, 0x41, 0x54, 0x48, 0x83, 0xEC, 0x50, 0x48, 0x8D, 0x74,
		0x24, 0x40, 0xC6, 0x06, 0x00, 0x48, 0x89, 0xC8, 0x45, 0x31, 0xC0, 0x48, 0x83, 0xF8, 0x00, 0x7D,
		0x09, 0x48, 0xF7, 0xD8, 0x41, 0xB8, 0x01, 0x00, 0x00, 0x00, 0x48, 0x83, 0xF8, 0x00, 0x75, 0x08,
		0x48, 0xFF, 0xCE, 0xC6, 0x06, 0x30, 0xEB, 0x1A, 0x48, 0x31, 0xD2, 0x49, 0xC7, 0xC1, 0x0A, 0x00,
		0x00, 0x00, 0x49, 0xF7, 0xF1, 0x80, 0xC2, 0x30, 0x48, 0xFF, 0xCE, 0x88, 0x16, 0x48, 0x85, 0xC0,
		0x75, 0xE6, 0x45, 0x85, 0xC0, 0x74, 0x06, 0x48, 0xFF, 0xCE, 0xC6, 0x06, 0x2D, 0x48, 0x8D, 0x44,
		0x24, 0x40, 0x48, 0x29, 0xF0, 0x48, 0x89, 0xC3, 0x48, 0x89, 0xF7, 0x48, 0x8D, 0x7B, 0x09, 0xE8,
		0x00, 0x00, 0x00, 0x00, 0x48, 0x89, 0x18, 0x4C, 0x8D, 0x40, 0x08, 0x48, 0x31, 0xC9, 0x48, 0x39,
		0xD9, 0x7D, 0x0C, 0x8A, 0x14, 0x0F, 0x41, 0x88, 0x14, 0x08, 0x48, 0xFF, 0xC1, 0xEB, 0xEF, 0x41,
		0xC6, 0x04, 0x18, 0x00, 0x48, 0x8D, 0x65, 0xE0, 0x41, 0x5C, 0x5F, 0x5E, 0x5B, 0x5D, 0xC3, 0x90,
	}
	// SysV ABI: argument to pawl_alloc is RDI instead of RCX.
	// Byte at index 110 (0x6E) is: 48 8d 7b 09 (lea rdi, [rbx + 9])
	// Patch call to pawl_alloc at offset 111 (0x6F): target is allocOff
	callSiteAlloc := intToStrOff + 111 + 5
	relAlloc := int32(allocOff) - int32(callSiteAlloc)
	binary.LittleEndian.PutUint32(intToStrBytes[112:], uint32(relAlloc))
	codeBuf.Write(intToStrBytes)

	for codeBuf.Len()%16 != 0 {
		codeBuf.WriteByte(0x90)
	}

	userCodeStart := uint32(codeBuf.Len())

	// Patch _start call to main
	mainTarget := userCodeStart + art.EntryOffset
	relMain := int32(mainTarget) - int32(callMainPatch+4)
	binary.LittleEndian.PutUint32(codeBuf.Bytes()[callMainPatch:], uint32(relMain))

	// Resolve unpatched linker calls
	userCodeCopy := make([]byte, len(art.Code))
	copy(userCodeCopy, art.Code)

	for _, fix := range art.CallFixups {
		var targetHelper uint32
		switch fix.FuncName {
		case "pawl_print":
			targetHelper = printOff
		case "pawl_println":
			targetHelper = printlnOff
		case "pawl_alloc":
			targetHelper = allocOff
		case "pawl_int_to_str":
			targetHelper = intToStrOff
		default:
			return fmt.Errorf("linker: unresolved runtime symbol %q", fix.FuncName)
		}
		absCallSite := userCodeStart + uint32(fix.InsEnd)
		rel32 := int32(targetHelper) - int32(absCallSite)
		binary.LittleEndian.PutUint32(userCodeCopy[fix.PatchOffset:], uint32(rel32))
	}

	codeBuf.Write(userCodeCopy)
	textBytes := codeBuf.Bytes()

	// Build ELF with 2 LOAD segments:
	// Segment 1: Header + Text (RX)
	// Segment 2: Rodata (R)
	out := new(bytes.Buffer)

	// ELF Header (64 bytes)
	var eIdent [16]byte
	copy(eIdent[:], []byte("\x7fELF\x02\x01\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00"))
	out.Write(eIdent[:])

	_ = binary.Write(out, binary.LittleEndian, uint16(2))             // ET_EXEC
	_ = binary.Write(out, binary.LittleEndian, uint16(0x3E))          // AMD64
	_ = binary.Write(out, binary.LittleEndian, uint32(1))             // version
	_ = binary.Write(out, binary.LittleEndian, uint64(textBase))      // e_entry = _start
	_ = binary.Write(out, binary.LittleEndian, uint64(elfHeaderSize)) // e_phoff = 64
	_ = binary.Write(out, binary.LittleEndian, uint64(0))             // e_shoff = 0
	_ = binary.Write(out, binary.LittleEndian, uint32(0))             // e_flags = 0
	_ = binary.Write(out, binary.LittleEndian, uint16(64))            // e_ehsize
	_ = binary.Write(out, binary.LittleEndian, uint16(56))            // e_phentsize
	_ = binary.Write(out, binary.LittleEndian, uint16(2))             // e_phnum = 2 segments
	_ = binary.Write(out, binary.LittleEndian, uint16(64))            // e_shentsize
	_ = binary.Write(out, binary.LittleEndian, uint16(0))
	_ = binary.Write(out, binary.LittleEndian, uint16(0))

	// Program Header 1: Text (RX)
	textSegSize := uint64(0x1000 + len(textBytes))
	_ = binary.Write(out, binary.LittleEndian, uint32(1))           // PT_LOAD
	_ = binary.Write(out, binary.LittleEndian, uint32(5))           // PF_R | PF_X
	_ = binary.Write(out, binary.LittleEndian, uint64(0))           // p_offset
	_ = binary.Write(out, binary.LittleEndian, uint64(elfBaseAddr)) // p_vaddr
	_ = binary.Write(out, binary.LittleEndian, uint64(elfBaseAddr)) // p_paddr
	_ = binary.Write(out, binary.LittleEndian, uint64(textSegSize)) // p_filesz
	_ = binary.Write(out, binary.LittleEndian, uint64(textSegSize)) // p_memsz
	_ = binary.Write(out, binary.LittleEndian, uint64(0x1000))      // align

	// Program Header 2: Rodata (R)
	rodataOffset := uint64(0x2000)
	rodataSegSize := uint64(len(rodataBytes))
	_ = binary.Write(out, binary.LittleEndian, uint32(1))             // PT_LOAD
	_ = binary.Write(out, binary.LittleEndian, uint32(4))             // PF_R
	_ = binary.Write(out, binary.LittleEndian, uint64(rodataOffset))  // p_offset
	_ = binary.Write(out, binary.LittleEndian, uint64(rodataBase))    // p_vaddr
	_ = binary.Write(out, binary.LittleEndian, uint64(rodataBase))    // p_paddr
	_ = binary.Write(out, binary.LittleEndian, uint64(rodataSegSize)) // p_filesz
	_ = binary.Write(out, binary.LittleEndian, uint64(rodataSegSize)) // p_memsz
	_ = binary.Write(out, binary.LittleEndian, uint64(0x1000))        // align

	// Pad to 0x1000
	for out.Len() < 0x1000 {
		out.WriteByte(0)
	}
	out.Write(textBytes)

	// Pad to 0x2000
	for out.Len() < 0x2000 {
		out.WriteByte(0)
	}
	out.Write(rodataBytes)

	return os.WriteFile(outputPath, out.Bytes(), 0755)
}
