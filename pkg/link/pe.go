// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

package link

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"

	"github.com/pawl-lang/pawl/pkg/codegen/amd64"
)

// LinkPE builds a 64-bit Windows PE executable with minimal I/O runtime.
func LinkPE(art *amd64.CompiledArtifact, outputPath string, noConsole bool) error {
	pe := NewPEBuilder()
	pe.SetNoConsole(noConsole)

	newlineBytes := []byte("\r\n\x00\x00\x00\x00\x00\x00")
	var rdataBuf bytes.Buffer
	rdataBuf.Write(newlineBytes)
	rdataBuf.Write(art.ReadOnlyData)

	pe.SetConstants(rdataBuf.Bytes())
	pe.SetBSSSize(0x200000) // 2 MB BSS

	// Temporarily set a dummy code of 4096 bytes so Layout computes RVAs assuming 0x1000 virtTextLen
	pe.SetCode(make([]byte, 4096))
	pe.Layout()

	rdataRVA := pe.ConstantsRVA()
	bssRVA := pe.BSSRVA()
	textRVA := uint32(0x1000)
	hStdoutRVA := bssRVA

	text := new(bytes.Buffer)

	// 1. _start at 0x1000:
	// sub rsp, 56
	text.Write([]byte{0x48, 0x83, 0xEC, 0x38})

	// mov ecx, -11
	text.Write([]byte{0xB9, 0xF5, 0xFF, 0xFF, 0xFF})

	// call [rip + disp(GetStdHandle)]
	curPos := textRVA + uint32(text.Len()) + 6
	dispGetStd := int32(pe.IATOffset("GetStdHandle")) - int32(curPos)
	text.Write([]byte{0xFF, 0x15})
	_ = binary.Write(text, binary.LittleEndian, dispGetStd)

	// mov [rip + disp(hStdoutRVA)], rax
	curPos = textRVA + uint32(text.Len()) + 7
	dispH := int32(hStdoutRVA) - int32(curPos)
	text.Write([]byte{0x48, 0x89, 0x05})
	_ = binary.Write(text, binary.LittleEndian, dispH)

	// call main (rel32)
	text.WriteByte(0xE8)
	callMainPatch := text.Len()
	_ = binary.Write(text, binary.LittleEndian, uint32(0))

	// ExitProcess(eax) -> mov ecx, eax
	text.Write([]byte{0x89, 0xC1})
	curPos = textRVA + uint32(text.Len()) + 6
	dispExit := int32(pe.IATOffset("ExitProcess")) - int32(curPos)
	text.Write([]byte{0xFF, 0x15})
	_ = binary.Write(text, binary.LittleEndian, dispExit)
	text.WriteByte(0xF4) // hlt

	// Align to 16 bytes
	for text.Len()%16 != 0 {
		text.WriteByte(0x90)
	}

	// 2. pawl_print(str_ptr in RCX)
	// interned string is at (rdataRVA + 8 + rcx)
	printOff := uint32(text.Len())
	// sub rsp, 56
	text.Write([]byte{0x48, 0x83, 0xEC, 0x38})

	// lea rax, [rip + disp(rdataRVA + 8)]
	curPos = textRVA + uint32(text.Len()) + 7
	dispStrBase := int32(rdataRVA+8) - int32(curPos)
	text.Write([]byte{0x48, 0x8D, 0x05})
	_ = binary.Write(text, binary.LittleEndian, dispStrBase)

	// add rax, rcx
	text.Write([]byte{0x48, 0x01, 0xC8})

	// mov r8, [rax] (len)
	text.Write([]byte{0x4C, 0x8B, 0x00})

	// lea rdx, [rax + 8] (data)
	text.Write([]byte{0x48, 0x8D, 0x50, 0x08})

	// mov rcx, [rip + disp(hStdoutRVA)]
	curPos = textRVA + uint32(text.Len()) + 7
	dispOut := int32(hStdoutRVA) - int32(curPos)
	text.Write([]byte{0x48, 0x8B, 0x0D})
	_ = binary.Write(text, binary.LittleEndian, dispOut)

	// lea r9, [rsp + 40]
	text.Write([]byte{0x4C, 0x8D, 0x4C, 0x24, 0x28})

	// mov qword ptr [rsp + 32], 0
	text.Write([]byte{0x48, 0xC7, 0x44, 0x24, 0x20, 0x00, 0x00, 0x00, 0x00})

	// call [rip + disp(WriteFile)]
	curPos = textRVA + uint32(text.Len()) + 6
	dispW := int32(pe.IATOffset("WriteFile")) - int32(curPos)
	text.Write([]byte{0xFF, 0x15})
	_ = binary.Write(text, binary.LittleEndian, dispW)

	// add rsp, 56; ret
	text.Write([]byte{0x48, 0x83, 0xC4, 0x38, 0xC3})

	for text.Len()%16 != 0 {
		text.WriteByte(0x90)
	}

	// 3. pawl_println(str_ptr in RCX)
	printlnOff := uint32(text.Len())
	// sub rsp, 56
	text.Write([]byte{0x48, 0x83, 0xEC, 0x38})

	// call pawl_print (rel32)
	text.WriteByte(0xE8)
	relPrint := int32(printOff) - int32(uint32(text.Len())+4)
	_ = binary.Write(text, binary.LittleEndian, relPrint)

	// mov rcx, [rip + disp(hStdoutRVA)]
	curPos = textRVA + uint32(text.Len()) + 7
	dispOut2 := int32(hStdoutRVA) - int32(curPos)
	text.Write([]byte{0x48, 0x8B, 0x0D})
	_ = binary.Write(text, binary.LittleEndian, dispOut2)

	// lea rdx, [rip + disp(rdataRVA)] (points to "\r\n")
	curPos = textRVA + uint32(text.Len()) + 7
	dispNL := int32(rdataRVA) - int32(curPos)
	text.Write([]byte{0x48, 0x8D, 0x15})
	_ = binary.Write(text, binary.LittleEndian, dispNL)

	// mov r8d, 2
	text.WriteByte(0x41)
	text.WriteByte(0xB8)
	_ = binary.Write(text, binary.LittleEndian, uint32(2))

	// lea r9, [rsp + 40]
	text.Write([]byte{0x4C, 0x8D, 0x4C, 0x24, 0x28})

	// mov qword ptr [rsp + 32], 0
	text.Write([]byte{0x48, 0xC7, 0x44, 0x24, 0x20, 0x00, 0x00, 0x00, 0x00})

	// call [rip + disp(WriteFile)]
	curPos = textRVA + uint32(text.Len()) + 6
	dispW2 := int32(pe.IATOffset("WriteFile")) - int32(curPos)
	text.Write([]byte{0xFF, 0x15})
	_ = binary.Write(text, binary.LittleEndian, dispW2)

	// add rsp, 56; ret
	text.Write([]byte{0x48, 0x83, 0xC4, 0x38, 0xC3})

	for text.Len()%16 != 0 {
		text.WriteByte(0x90)
	}

	// 4. User code
	userCodeStart := uint32(text.Len())

	// Patch _start call to main
	mainTarget := userCodeStart + art.EntryOffset
	relMain := int32(mainTarget) - int32(callMainPatch+4)
	binary.LittleEndian.PutUint32(text.Bytes()[callMainPatch:], uint32(relMain))

	userCodeCopy := make([]byte, len(art.Code))
	copy(userCodeCopy, art.Code)

	for _, fix := range art.CallFixups {
		var targetHelper uint32
		switch fix.FuncName {
		case "pawl_print":
			targetHelper = printOff
		case "pawl_println":
			targetHelper = printlnOff
		default:
			return fmt.Errorf("linker: unresolved runtime symbol %q", fix.FuncName)
		}

		absCallSite := userCodeStart + uint32(fix.InsEnd)
		rel32 := int32(targetHelper) - int32(absCallSite)
		binary.LittleEndian.PutUint32(userCodeCopy[fix.PatchOffset:], uint32(rel32))
	}

	text.Write(userCodeCopy)

	pe.SetCode(text.Bytes())
	peBytes, err := pe.Build()
	if err != nil {
		return err
	}
	return os.WriteFile(outputPath, peBytes, 0755)
}
