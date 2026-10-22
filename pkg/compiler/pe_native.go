package compiler

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"

	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/vm"
)

// BuildNativePE generates a standalone, pure native 64-bit Windows PE executable
// containing direct machine instructions with zero Go runtime overhead (~2-5 KB).
func BuildNativePE(bundle *Bundle, outputPath string, noConsole bool) error {
	if len(bundle.NativeCode) == 0 {
		return fmt.Errorf("bundle does not contain native machine code")
	}

	pe := NewPEBuilder()
	pe.SetNoConsole(noConsole)

	// 1. Serialize Constant Pool (40 bytes per StackSlot). A pure native PE has
	// no Go runtime behind its constant slots, so only unboxed primitives can be
	// represented. Reject everything else explicitly instead of serializing an
	// unusable slot and failing silently at run time.
	constPoolBuf := new(bytes.Buffer)
	for _, k := range bundle.Code.Constants {
		slot := make([]byte, 40)
		switch v := k.(type) {
		case *interpreter.IntegerValue:
			slot[0] = byte(vm.SlotInt)
			binary.LittleEndian.PutUint64(slot[8:], uint64(v.Val))
		case *interpreter.FloatValue:
			slot[0] = byte(vm.SlotFloat)
			binary.LittleEndian.PutUint64(slot[16:], math.Float64bits(v.Val))
		case *interpreter.BoolValue:
			slot[0] = byte(vm.SlotBool)
			if v.Val {
				binary.LittleEndian.PutUint64(slot[8:], 1)
			}
		case *interpreter.NullValue:
			slot[0] = byte(vm.SlotNull)
		default:
			return fmt.Errorf("pure native PE cannot represent constant of type %T; use the runtime stub", k)
		}
		constPoolBuf.Write(slot)
	}

	// Diagnostic messages used when native execution stops without a VM to
	// resume it. They live after the constant slots so their RVAs are stable.
	bailoutMsg := []byte("alca: native execution stopped: the program reached a deoptimization point, but this pure native build has no VM fallback\r\n")
	bailoutMsgOffset := constPoolBuf.Len()
	constPoolBuf.Write(bailoutMsg)
	runtimeMsg := []byte("alca: native execution failed at runtime; this pure native build has no VM fallback\r\n")
	runtimeMsgOffset := constPoolBuf.Len()
	constPoolBuf.Write(runtimeMsg)

	// Layout .rdata constants
	pe.SetConstants(constPoolBuf.Bytes())
	pe.Layout()

	// 2. Generate _start Entry Point in .text
	// _start sets up NativeContext in .bss, initializes stack and globals,
	// and invokes the native machine instructions.
	text := new(bytes.Buffer)

	// BSS offsets:
	// 0x0000: NativeContext (64 bytes)
	// 0x0040: Globals (4096 * 40 bytes = 163840 bytes)
	// 0x28040: Stack (16384 * 40 bytes = 655360 bytes)
	// 0xC8040: hStdout (8 bytes)

	const (
		bssCtxOffset     = 0x0000
		bssGlobalsOffset = 0x0040
		bssStackOffset   = 0x28040
		bssHStdoutOffset = 0xC8040
	)

	// Prologue: 32-byte shadow space, the fifth Win32 argument at [rsp+32],
	// and a 16-byte local area that Win32 calls cannot clobber.
	text.Write([]byte{0x48, 0x83, 0xEC, 0x38}) // sub rsp, 56

	// GetStdHandle(STD_OUTPUT_HANDLE = -11)
	text.Write([]byte{0xB9, 0xF5, 0xFF, 0xFF, 0xFF}) // mov ecx, -11
	// call [rip + disp(GetStdHandle)]
	text.Write([]byte{0xFF, 0x15})
	dispGetStd := int32(pe.IATOffset("GetStdHandle")) - int32(0x1000+text.Len()+4)
	binary.Write(text, binary.LittleEndian, dispGetStd)

	// mov [rip + disp(hStdout)], rax
	text.Write([]byte{0x48, 0x89, 0x05})
	dispHStdout := int32(pe.BSSRVA()+bssHStdoutOffset) - int32(0x1000+text.Len()+4)
	binary.Write(text, binary.LittleEndian, dispHStdout)

	// Setup NativeContext in .bss:
	// ctx.StackPtr = ImageBase + bssRVA + bssStackOffset
	// ctx.GlobalsPtr = ImageBase + bssRVA + bssGlobalsOffset
	// ctx.PoolPtr = ImageBase + constRVA
	// ctx.BasePointer = 0
	// ctx.SP = 0
	// ctx.EntryOffset = 0

	// lea r12, [rip + disp(bssCtx)]
	text.Write([]byte{0x4C, 0x8D, 0x25})
	dispCtx := int32(pe.BSSRVA()+bssCtxOffset) - int32(0x1000+text.Len()+4)
	binary.Write(text, binary.LittleEndian, dispCtx)

	// lea rax, [rip + disp(bssStack)]
	text.Write([]byte{0x48, 0x8D, 0x05})
	dispStack := int32(pe.BSSRVA()+bssStackOffset) - int32(0x1000+text.Len()+4)
	binary.Write(text, binary.LittleEndian, dispStack)
	// mov [r12 + 0], rax (ctx.StackPtr)
	text.Write([]byte{0x49, 0x89, 0x44, 0x24, 0x00})

	// lea rax, [rip + disp(bssGlobals)]
	text.Write([]byte{0x48, 0x8D, 0x05})
	dispGlobals := int32(pe.BSSRVA()+bssGlobalsOffset) - int32(0x1000+text.Len()+4)
	binary.Write(text, binary.LittleEndian, dispGlobals)
	// mov [r12 + 8], rax (ctx.GlobalsPtr)
	text.Write([]byte{0x49, 0x89, 0x44, 0x24, 0x08})

	// lea rax, [rip + disp(constPool)]
	text.Write([]byte{0x48, 0x8D, 0x05})
	dispPool := int32(pe.ConstantsRVA()) - int32(0x1000+text.Len()+4)
	binary.Write(text, binary.LittleEndian, dispPool)
	// mov [r12 + 16], rax (ctx.PoolPtr)
	text.Write([]byte{0x49, 0x89, 0x44, 0x24, 0x10})

	// mov qword ptr [r12 + 24], 0 (ctx.BasePointer)
	text.Write([]byte{0x49, 0xC7, 0x44, 0x24, 0x18, 0x00, 0x00, 0x00, 0x00})
	// mov qword ptr [r12 + 32], 0 (ctx.SP)
	text.Write([]byte{0x49, 0xC7, 0x44, 0x24, 0x20, 0x00, 0x00, 0x00, 0x00})
	// mov qword ptr [r12 + 88], 16384 (ctx.StackLimit): the .bss stack holds
	// 16384 slots, and the native code refuses to push past that limit.
	text.Write([]byte{0x49, 0xC7, 0x44, 0x24, 0x58, 0x00, 0x40, 0x00, 0x00})
	// mov qword ptr [r12 + 56], 0 (ctx.EntryOffset)
	text.Write([]byte{0x49, 0xC7, 0x44, 0x24, 0x38, 0x00, 0x00, 0x00, 0x00})

	// Pass ctx in RAX
	text.Write([]byte{0x4C, 0x89, 0xE0}) // mov rax, r12

	// Call the native machine code (located right after this wrapper). The
	// displacement is patched once the wrapper and error handler are complete.
	callProgOffset := text.Len()
	text.Write([]byte{0xE8, 0x00, 0x00, 0x00, 0x00}) // placeholder call rel32

	// The native body records its status in ctx.Status (StatusSuccess=0,
	// StatusBailout=1, StatusError=2). A pure native PE has no VM to resume,
	// so a deoptimization or runtime fault must be reported on stderr and
	// turned into a nonzero exit status instead of being silently ignored.
	text.Write([]byte{0x41, 0x8B, 0x44, 0x24, 0x28}) // mov eax, [r12+40]
	text.Write([]byte{0x85, 0xC0})                   // test eax, eax
	text.Write([]byte{0x0F, 0x85, 0, 0, 0, 0})       // jne error path (patched below)
	statusJumpDisp := text.Len() - 4

	// Success: ExitProcess(0)
	text.Write([]byte{0x31, 0xC9}) // xor ecx, ecx
	text.Write([]byte{0xFF, 0x15})
	dispExit := int32(pe.IATOffset("ExitProcess")) - int32(0x1000+text.Len()+4)
	binary.Write(text, binary.LittleEndian, dispExit)

	errorPath := text.Len()
	relJump := int32(errorPath) - int32(statusJumpDisp+4)
	binary.LittleEndian.PutUint32(text.Bytes()[statusJumpDisp:], uint32(relJump))

	// Error path: report the failure on stderr, then exit with status 70.
	// Keep the status in a callee-saved register: the shadow space belongs to
	// the Win32 calls and must not be used for live data across them.
	text.Write([]byte{0x41, 0x89, 0xC5}) // mov r13d, eax (status)

	// GetStdHandle(STD_ERROR_HANDLE = -12)
	text.Write([]byte{0xB9, 0xF4, 0xFF, 0xFF, 0xFF}) // mov ecx, -12
	text.Write([]byte{0xFF, 0x15})
	dispStderr := int32(pe.IATOffset("GetStdHandle")) - int32(0x1000+text.Len()+4)
	binary.Write(text, binary.LittleEndian, dispStderr)
	text.Write([]byte{0x48, 0x89, 0xC1}) // mov rcx, rax (hStderr)

	text.Write([]byte{0x41, 0x83, 0xFD, 0x01}) // cmp r13d, 1 (StatusBailout)
	text.Write([]byte{0x75, 0x00})             // jne runtime message (patched below)
	noBailoutJump := text.Len() - 1

	// Bailout message: lea rdx, [rip + msg]; mov r8d, len(msg)
	writeRIPLea := func(op []byte, rva int) {
		text.Write(op)
		disp := int32(rva) - int32(0x1000+text.Len()+4)
		binary.Write(text, binary.LittleEndian, disp)
	}
	writeMsgLen := func(n int) {
		text.Write([]byte{0x41, 0xB8}) // mov r8d, imm32
		binary.Write(text, binary.LittleEndian, uint32(n))
	}
	writeRIPLea([]byte{0x48, 0x8D, 0x15}, int(pe.ConstantsRVA())+bailoutMsgOffset)
	writeMsgLen(len(bailoutMsg))
	text.Write([]byte{0xEB, 0x00}) // jmp write (patched below)
	skipRuntimeMsgJump := text.Len() - 1

	runtimeMsgLabel := text.Len()
	text.Bytes()[noBailoutJump] = byte(runtimeMsgLabel - (noBailoutJump + 1))
	writeRIPLea([]byte{0x48, 0x8D, 0x15}, int(pe.ConstantsRVA())+runtimeMsgOffset)
	writeMsgLen(len(runtimeMsg))

	writeLabel := text.Len()
	text.Bytes()[skipRuntimeMsgJump] = byte(writeLabel - (skipRuntimeMsgJump + 1))

	// WriteFile(hStderr, msg, len, &written, NULL)
	text.Write([]byte{0x4C, 0x8D, 0x4C, 0x24, 0x28})                         // lea r9, [rsp+40]
	text.Write([]byte{0x48, 0xC7, 0x44, 0x24, 0x20, 0x00, 0x00, 0x00, 0x00}) // mov qword [rsp+32], 0
	text.Write([]byte{0xFF, 0x15})
	dispWrite := int32(pe.IATOffset("WriteFile")) - int32(0x1000+text.Len()+4)
	binary.Write(text, binary.LittleEndian, dispWrite)

	// ExitProcess(70): EX_SOFTWARE-like nonzero status for "no VM fallback".
	text.Write([]byte{0xB9, 0x46, 0x00, 0x00, 0x00}) // mov ecx, 70
	text.Write([]byte{0xFF, 0x15})
	dispExitErr := int32(pe.IATOffset("ExitProcess")) - int32(0x1000+text.Len()+4)
	binary.Write(text, binary.LittleEndian, dispExitErr)

	text.Write([]byte{0x48, 0x83, 0xC4, 0x38}) // add rsp, 56
	text.WriteByte(0xC3)                       // ret (unreachable)

	// Target of callProgOffset is current text.Len()
	targetProgOffset := text.Len()
	relDisp := int32(targetProgOffset) - int32(callProgOffset+5)
	binary.LittleEndian.PutUint32(text.Bytes()[callProgOffset+1:], uint32(relDisp))

	// Append the program's native machine code bytes!
	text.Write(bundle.NativeCode)

	pe.SetCode(text.Bytes())

	// Build the PE file bytes
	exeBytes, err := pe.Build()
	if err != nil {
		return fmt.Errorf("failed to generate PE executable: %w", err)
	}

	// Write out the file
	if err := os.WriteFile(outputPath, exeBytes, 0o755); err != nil {
		return fmt.Errorf("failed to write %q: %w", outputPath, err)
	}

	return nil
}
