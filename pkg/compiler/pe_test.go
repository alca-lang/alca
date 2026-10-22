package compiler_test

import (
	"bytes"
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/alca-lang/alca/pkg/compiler"
)

func TestPEBuilder(t *testing.T) {
	pe := compiler.NewPEBuilder()

	msg := "Hello from pure PEBuilder!\r\n"
	pe.SetConstants([]byte(msg))

	pe.Layout()

	// Compute RVAs
	getStdHandleIAT := pe.IATOffset("GetStdHandle")
	writeFileIAT := pe.IATOffset("WriteFile")
	exitProcessIAT := pe.IATOffset("ExitProcess")
	msgRVA := pe.ConstantsRVA()

	// Build .text machine code
	// Entry point is at RVA 0x1000:
	// sub rsp, 56
	// mov ecx, -11 (STD_OUTPUT_HANDLE)
	// call [rip + disp(GetStdHandle)]
	// mov rcx, rax (hStdout)
	// lea rdx, [rip + disp(msg)]
	// mov r8d, len(msg)
	// lea r9, [rsp + 48]
	// mov qword ptr [rsp + 32], 0
	// call [rip + disp(WriteFile)]
	// xor ecx, ecx
	// call [rip + disp(ExitProcess)]
	// add rsp, 56
	// ret

	text := new(bytes.Buffer)
	text.Write([]byte{0x48, 0x83, 0xEC, 0x38})       // 0x1000: sub rsp, 56
	text.Write([]byte{0xB9, 0xF5, 0xFF, 0xFF, 0xFF}) // 0x1004: mov ecx, -11

	// 0x1009: call [rip + disp] -> target getStdHandleIAT
	// RIP after instruction is 0x100F (0x1009 + 6)
	dispGetStd := int32(getStdHandleIAT) - int32(0x100F)
	text.Write([]byte{0xFF, 0x15})
	binary.Write(text, binary.LittleEndian, dispGetStd)

	// 0x100F: mov rcx, rax
	text.Write([]byte{0x48, 0x89, 0xC1})

	// 0x1012: lea rdx, [rip + disp] -> target msgRVA
	// RIP after instruction is 0x1019 (0x1012 + 7)
	dispMsg := int32(msgRVA) - int32(0x1019)
	text.Write([]byte{0x48, 0x8D, 0x15})
	binary.Write(text, binary.LittleEndian, dispMsg)

	// 0x1019: mov r8d, len(msg)
	text.WriteByte(0x41)
	text.WriteByte(0xB8)
	binary.Write(text, binary.LittleEndian, uint32(len(msg)))

	// 0x101F: lea r9, [rsp + 48]
	text.Write([]byte{0x4C, 0x8D, 0x4C, 0x24, 0x30})

	// 0x1024: mov qword ptr [rsp + 32], 0
	text.Write([]byte{0x48, 0xC7, 0x44, 0x24, 0x20, 0x00, 0x00, 0x00, 0x00})

	// 0x102D: call [rip + disp] -> target writeFileIAT
	// RIP after instruction is 0x1033 (0x102D + 6)
	dispWrite := int32(writeFileIAT) - int32(0x1033)
	text.Write([]byte{0xFF, 0x15})
	binary.Write(text, binary.LittleEndian, dispWrite)

	// 0x1033: xor ecx, ecx
	text.Write([]byte{0x31, 0xC9})

	// 0x1035: call [rip + disp] -> target exitProcessIAT
	// RIP after instruction is 0x103B (0x1035 + 6)
	dispExit := int32(exitProcessIAT) - int32(0x103B)
	text.Write([]byte{0xFF, 0x15})
	binary.Write(text, binary.LittleEndian, dispExit)

	// 0x103B: add rsp, 56; ret
	text.Write([]byte{0x48, 0x83, 0xC4, 0x38, 0xC3})

	pe.SetCode(text.Bytes())

	data, err := pe.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	tmpDir := t.TempDir()
	exePath := filepath.Join(tmpDir, "pe_builder_test.exe")
	if err := os.WriteFile(exePath, data, 0o755); err != nil {
		t.Fatalf("write exe failed: %v", err)
	}

	t.Logf("Generated PE executable size: %d bytes (%.2f KB)", len(data), float64(len(data))/1024.0)

	cmd := exec.Command(exePath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("exec failed: %v, out: %s", err, string(out))
	}
	if string(out) != msg {
		t.Fatalf("expected %q, got %q", msg, string(out))
	}
	t.Logf("SUCCESS! Output: %q", string(out))
}
