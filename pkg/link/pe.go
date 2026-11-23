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

// LinkPE builds a 64-bit Windows PE executable using the compiled AMD64 code.
// The entry point calls main() and then calls ExitProcess(eax).
func LinkPE(art *amd64.CompiledArtifact, outputPath string, noConsole bool) error {
	pe := NewPEBuilder()
	pe.SetNoConsole(noConsole)
	pe.Layout() // compute RVAs for imports (ExitProcess etc.)

	// We wrap the compiled code with a _start bootstrap:
	// _start:
	//   sub rsp, 40               ; 32 bytes shadow space + 8 bytes alignment
	//   call main                 ; call pawl main()
	//   mov ecx, eax              ; exit code in ecx for ExitProcess(UINT uExitCode)
	//   call [rip + disp(ExitProcess)]
	//   hlt                       ; should never reach here

	bootstrap := jit.NewAssemblerAMD64()
	bootstrap.SubRegImm32(jit.RSP, 40)

	// call main (entryOffset relative to _start after bootstrap call instruction)
	bootstrap.EmitByte(0xE8)
	callPatch := bootstrap.CodeLen()
	bootstrap.EmitUint32(0) // placeholder

	// mov ecx, eax
	// 89 C1
	bootstrap.EmitByte(0x89)
	bootstrap.EmitByte(0xC1)

	// call [rip + disp(ExitProcess)]
	// FF 15 disp32
	bootstrap.EmitByte(0xFF)
	bootstrap.EmitByte(0x15)

	bootLen := bootstrap.CodeLen() + 4 // after disp32
	dispExit := int32(pe.IATOffset("ExitProcess")) - int32(0x1000+bootLen)
	bootstrap.EmitUint32(uint32(dispExit))

	// hlt
	bootstrap.EmitByte(0xF4)

	// Align bootstrap to 16 bytes before appending user code
	for bootstrap.CodeLen()%16 != 0 {
		bootstrap.Nop()
	}

	startCode, err := bootstrap.Finalize()
	if err != nil {
		return err
	}

	// Calculate rel32 to main function:
	// main is located at len(startCode) + art.EntryOffset
	// rel32 = target - (callPatch + 4)
	targetMain := int32(len(startCode) + int(art.EntryOffset))
	relMain := targetMain - int32(callPatch+4)
	binary.LittleEndian.PutUint32(startCode[callPatch:], uint32(relMain))

	// Combine bootstrap + user code
	fullText := new(bytes.Buffer)
	fullText.Write(startCode)
	fullText.Write(art.Code)

	pe.SetCode(fullText.Bytes())

	peBytes, err := pe.Build()
	if err != nil {
		return fmt.Errorf("failed to build PE: %w", err)
	}

	return os.WriteFile(outputPath, peBytes, 0755)
}
