package compiler

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
)

// Standard PE alignments
const (
	peFileAlign = 0x200  // 512 bytes on disk
	peSectAlign = 0x1000 // 4096 bytes in memory
)

// Windows Subsystem constants
const (
	SubsystemWindowsGUI uint16 = 2 // Graphical (no console window)
	SubsystemWindowsCUI uint16 = 3 // Console (opens cmd / terminal)
)

// PEBuilder builds a minimal, pure native 64-bit Windows PE32+ executable.
type PEBuilder struct {
	code       []byte
	constants  []byte
	bssSize    uint32
	imports    []string
	iatOffsets map[string]uint32 // FuncName -> RVA
	constRVA   uint32
	bssRVA     uint32
	subsystem  uint16
}

// NewPEBuilder initializes a PEBuilder with standard KERNEL32 imports.
func NewPEBuilder() *PEBuilder {
	return &PEBuilder{
		imports: []string{
			"ExitProcess",
			"GetStdHandle",
			"WriteFile",
			"GetProcessHeap",
			"HeapAlloc",
			"HeapFree",
			"GetSystemTimeAsFileTime",
		},
		iatOffsets: make(map[string]uint32),
		bssSize:    0x100000, // 1 MB uninitialized BSS memory for stack/globals
		subsystem:  SubsystemWindowsCUI,
	}
}

// SetSubsystem sets the Windows subsystem (3 = CUI/Console, 2 = GUI/NoConsole).
func (p *PEBuilder) SetSubsystem(s uint16) {
	p.subsystem = s
}

// SetNoConsole configures whether the binary runs silently without a console window.
func (p *PEBuilder) SetNoConsole(noConsole bool) {
	if noConsole {
		p.subsystem = SubsystemWindowsGUI
	} else {
		p.subsystem = SubsystemWindowsCUI
	}
}

// SetCode sets the machine code bytes for the .text section.
func (p *PEBuilder) SetCode(code []byte) {
	p.code = code
}

// SetConstants sets constant data to place in .rdata.
func (p *PEBuilder) SetConstants(c []byte) {
	p.constants = c
}

// SetBSSSize sets the virtual size of the .bss section.
func (p *PEBuilder) SetBSSSize(size uint32) {
	p.bssSize = size
}

// IATOffset returns the RVA of the IAT slot for the named imported function.
func (p *PEBuilder) IATOffset(funcName string) uint32 {
	return p.iatOffsets[funcName]
}

// ConstantsRVA returns the RVA where user constants start in .rdata.
func (p *PEBuilder) ConstantsRVA() uint32 {
	return p.constRVA
}

// BSSRVA returns the RVA where the .bss section begins.
func (p *PEBuilder) BSSRVA() uint32 {
	return p.bssRVA
}

func alignUp(n, align uint32) uint32 {
	if n%align == 0 {
		return n
	}
	return n + (align - (n % align))
}

// Layout calculates section RVAs, import table offsets, and layout before building.
func (p *PEBuilder) Layout() {
	textRVA := uint32(0x1000)
	rawTextLen := alignUp(uint32(len(p.code)), peFileAlign)
	virtTextLen := alignUp(uint32(len(p.code)), peSectAlign)
	if virtTextLen == 0 {
		virtTextLen = peSectAlign
	}

	rdataRVA := textRVA + virtTextLen

	// Calculate .rdata internal layout:
	// 1. IAT
	iatRVA := rdataRVA
	for i, fn := range p.imports {
		p.iatOffsets[fn] = iatRVA + uint32(i*8)
	}
	iatSize := uint32((len(p.imports) + 1) * 8)

	// 2. Import Directory Table (20 bytes per DLL + 20 bytes null terminator)
	idtOffset := iatSize
	idtSize := uint32(40) // 1 DLL (KERNEL32) + 1 null entry

	// 3. ILT (Import Lookup Table)
	iltOffset := alignUp(idtOffset+idtSize, 8)
	iltSize := uint32((len(p.imports) + 1) * 8)

	// 4. DLL Name ("KERNEL32.dll\0")
	dllNameOffset := iltOffset + iltSize
	dllNameSize := uint32(16) // "KERNEL32.dll\0" padded to 16 bytes

	// 5. Hint/Name entries
	hintOffset := dllNameOffset + dllNameSize

	// Accurately simulate hintBuf length
	hintLen := 0
	for _, fn := range p.imports {
		hintLen += 2 + len(fn) + 1
		if hintLen%2 != 0 {
			hintLen++
		}
	}
	if hintLen%16 != 0 {
		hintLen += 16 - (hintLen % 16)
	}

	// 6. User constants/strings
	p.constRVA = rdataRVA + hintOffset + uint32(hintLen)

	totalRData := hintOffset + uint32(hintLen) + uint32(len(p.constants))
	virtRDataLen := alignUp(totalRData, peSectAlign)
	if virtRDataLen == 0 {
		virtRDataLen = peSectAlign
	}

	// 7. BSS RVA
	p.bssRVA = rdataRVA + virtRDataLen

	_ = rawTextLen
}

// Build generates the standalone PE32+ executable bytes.
func (p *PEBuilder) Build() ([]byte, error) {
	p.Layout()

	textRVA := uint32(0x1000)
	rawTextLen := alignUp(uint32(len(p.code)), peFileAlign)
	virtTextLen := alignUp(uint32(len(p.code)), peSectAlign)
	if virtTextLen == 0 {
		virtTextLen = peSectAlign
	}

	rdataRVA := textRVA + virtTextLen

	// Build .rdata bytes
	rdata := new(bytes.Buffer)

	// 1. Calculate Hint/Name RVAs first so IAT and ILT can point to them
	iatSize := uint32((len(p.imports) + 1) * 8)
	idtOffset := iatSize
	idtSize := uint32(40)
	iltOffset := alignUp(idtOffset+idtSize, 8)
	iltSize := uint32((len(p.imports) + 1) * 8)
	dllNameOffset := iltOffset + iltSize
	dllNameRVA := rdataRVA + dllNameOffset
	dllNameSize := uint32(16)
	hintOffset := dllNameOffset + dllNameSize

	hintRVAs := make([]uint32, len(p.imports))
	hintBuf := new(bytes.Buffer)
	curHintOffset := hintOffset
	for i, fn := range p.imports {
		hintRVAs[i] = rdataRVA + curHintOffset
		binary.Write(hintBuf, binary.LittleEndian, uint16(0)) // Hint 0
		hintBuf.WriteString(fn)
		hintBuf.WriteByte(0)
		if hintBuf.Len()%2 != 0 {
			hintBuf.WriteByte(0)
		}
		curHintOffset = hintOffset + uint32(hintBuf.Len())
	}
	// Pad hintBuf to 16 bytes alignment
	if hintBuf.Len()%16 != 0 {
		hintBuf.Write(make([]byte, 16-(hintBuf.Len()%16)))
	}

	// 1. Write IAT
	for i := range p.imports {
		binary.Write(rdata, binary.LittleEndian, uint64(hintRVAs[i]))
	}
	binary.Write(rdata, binary.LittleEndian, uint64(0)) // IAT terminator

	// 2. Write Import Directory Table (20 bytes KERNEL32 + 20 bytes NULL)
	iltRVA := rdataRVA + iltOffset
	iatRVA := rdataRVA
	binary.Write(rdata, binary.LittleEndian, uint32(iltRVA))        // OriginalFirstThunk (ILT)
	binary.Write(rdata, binary.LittleEndian, uint32(0))             // TimeDateStamp
	binary.Write(rdata, binary.LittleEndian, uint32(0))             // ForwarderChain
	binary.Write(rdata, binary.LittleEndian, uint32(dllNameRVA))    // Name RVA
	binary.Write(rdata, binary.LittleEndian, uint32(iatRVA))        // FirstThunk (IAT)
	rdata.Write(make([]byte, 20))                                   // Null terminator descriptor

	// Align to iltOffset
	if uint32(rdata.Len()) < iltOffset {
		rdata.Write(make([]byte, iltOffset-uint32(rdata.Len())))
	}

	// 3. Write ILT
	for i := range p.imports {
		binary.Write(rdata, binary.LittleEndian, uint64(hintRVAs[i]))
	}
	binary.Write(rdata, binary.LittleEndian, uint64(0)) // ILT terminator

	// 4. Write DLL Name
	dllPadded := make([]byte, dllNameSize)
	copy(dllPadded, "KERNEL32.dll\x00")
	rdata.Write(dllPadded)

	// 5. Write Hint/Name entries
	rdata.Write(hintBuf.Bytes())

	// 6. Write User Constants
	if len(p.constants) > 0 {
		rdata.Write(p.constants)
	}

	rdataBytes := rdata.Bytes()
	rawRDataLen := alignUp(uint32(len(rdataBytes)), peFileAlign)
	virtRDataLen := alignUp(uint32(len(rdataBytes)), peSectAlign)
	if virtRDataLen == 0 {
		virtRDataLen = peSectAlign
	}

	// Pad rdata to file alignment
	if uint32(len(rdataBytes)) < rawRDataLen {
		rdataBytes = append(rdataBytes, make([]byte, rawRDataLen-uint32(len(rdataBytes)))...)
	}

	// Pad textCode to file alignment
	textCode := p.code
	if uint32(len(textCode)) < rawTextLen {
		textCode = append(textCode, make([]byte, rawTextLen-uint32(len(textCode)))...)
	}

	// BSS section
	bssRVA := rdataRVA + virtRDataLen
	virtBSSLen := alignUp(p.bssSize, peSectAlign)
	if virtBSSLen == 0 {
		virtBSSLen = peSectAlign
	}

	// Calculate total SizeOfImage
	sizeOfImage := bssRVA + virtBSSLen

	// Build Headers
	headers := new(bytes.Buffer)

	// DOS Header (64 bytes)
	dos := make([]byte, 64)
	dos[0] = 'M'
	dos[1] = 'Z'
	binary.LittleEndian.PutUint32(dos[0x3C:], 0x80) // e_lfanew = 0x80
	headers.Write(dos)

	// DOS Stub (64 bytes)
	dosStub := []byte("This program cannot be run in DOS mode.\r\r\n$\x00\x00\x00\x00\x00\x00\x00")
	stubPadded := make([]byte, 64)
	copy(stubPadded, dosStub)
	headers.Write(stubPadded)

	// PE Signature (4 bytes)
	headers.WriteString("PE\x00\x00")

	// COFF File Header (20 bytes)
	binary.Write(headers, binary.LittleEndian, uint16(0x8664)) // AMD64
	binary.Write(headers, binary.LittleEndian, uint16(3))      // 3 sections (.text, .rdata, .bss)
	binary.Write(headers, binary.LittleEndian, uint32(0))      // TimeDateStamp
	binary.Write(headers, binary.LittleEndian, uint32(0))      // PointerToSymbolTable
	binary.Write(headers, binary.LittleEndian, uint32(0))      // NumberOfSymbols
	binary.Write(headers, binary.LittleEndian, uint16(0xF0))   // SizeOfOptionalHeader
	binary.Write(headers, binary.LittleEndian, uint16(0x0222)) // Characteristics: EXECUTABLE_IMAGE | LARGE_ADDRESS_AWARE

	// Optional Header 64-bit (240 bytes)
	binary.Write(headers, binary.LittleEndian, uint16(0x020B)) // PE32+ magic
	headers.WriteByte(1)                                       // MajorLinkerVersion
	headers.WriteByte(0)                                       // MinorLinkerVersion
	binary.Write(headers, binary.LittleEndian, rawTextLen)     // SizeOfCode
	binary.Write(headers, binary.LittleEndian, rawRDataLen)    // SizeOfInitializedData
	binary.Write(headers, binary.LittleEndian, uint32(0))      // SizeOfUninitializedData
	binary.Write(headers, binary.LittleEndian, textRVA)        // AddressOfEntryPoint (.text RVA)
	binary.Write(headers, binary.LittleEndian, textRVA)        // BaseOfCode (.text RVA)
	binary.Write(headers, binary.LittleEndian, uint64(0x140000000)) // ImageBase
	binary.Write(headers, binary.LittleEndian, uint32(peSectAlign))  // SectionAlignment
	binary.Write(headers, binary.LittleEndian, uint32(peFileAlign))  // FileAlignment
	binary.Write(headers, binary.LittleEndian, uint16(6))            // MajorOSVersion
	binary.Write(headers, binary.LittleEndian, uint16(0))            // MinorOSVersion
	binary.Write(headers, binary.LittleEndian, uint16(0))            // MajorImageVersion
	binary.Write(headers, binary.LittleEndian, uint16(0))            // MinorImageVersion
	binary.Write(headers, binary.LittleEndian, uint16(6))            // MajorSubsystemVersion
	binary.Write(headers, binary.LittleEndian, uint16(0))            // MinorSubsystemVersion
	binary.Write(headers, binary.LittleEndian, uint32(0))            // Win32VersionValue
	binary.Write(headers, binary.LittleEndian, sizeOfImage)          // SizeOfImage
	binary.Write(headers, binary.LittleEndian, uint32(0x400))        // SizeOfHeaders
	binary.Write(headers, binary.LittleEndian, uint32(0))            // CheckSum
	binary.Write(headers, binary.LittleEndian, p.subsystem)          // Subsystem: Windows CUI (3) or GUI (2)
	binary.Write(headers, binary.LittleEndian, uint16(0x8160))       // DllCharacteristics: DYNAMIC_BASE | NX_COMPAT | TERMINAL_SERVER_AWARE
	binary.Write(headers, binary.LittleEndian, uint64(0x100000))     // SizeOfStackReserve
	binary.Write(headers, binary.LittleEndian, uint64(0x1000))       // SizeOfStackCommit
	binary.Write(headers, binary.LittleEndian, uint64(0x100000))     // SizeOfHeapReserve
	binary.Write(headers, binary.LittleEndian, uint64(0x1000))       // SizeOfHeapCommit
	binary.Write(headers, binary.LittleEndian, uint32(0))            // LoaderFlags
	binary.Write(headers, binary.LittleEndian, uint32(16))           // NumberOfRvaAndSizes

	// 16 Data Directories (8 bytes each)
	for i := 0; i < 16; i++ {
		if i == 1 {
			// Import Table directory: RVA rdataRVA + idtOffset, size 40 bytes
			binary.Write(headers, binary.LittleEndian, uint32(rdataRVA+idtOffset))
			binary.Write(headers, binary.LittleEndian, uint32(idtSize))
		} else if i == 12 {
			// IAT directory: RVA iatRVA, size iatSize
			binary.Write(headers, binary.LittleEndian, uint32(iatRVA))
			binary.Write(headers, binary.LittleEndian, uint32(iatSize))
		} else {
			binary.Write(headers, binary.LittleEndian, uint32(0))
			binary.Write(headers, binary.LittleEndian, uint32(0))
		}
	}

	// Section 1: .text (40 bytes)
	var textName [8]byte
	copy(textName[:], ".text")
	headers.Write(textName[:])
	binary.Write(headers, binary.LittleEndian, virtTextLen)      // VirtualSize
	binary.Write(headers, binary.LittleEndian, textRVA)          // VirtualAddress (RVA)
	binary.Write(headers, binary.LittleEndian, rawTextLen)      // SizeOfRawData
	binary.Write(headers, binary.LittleEndian, uint32(0x400))    // PointerToRawData
	binary.Write(headers, binary.LittleEndian, uint32(0))        // PointerToRelocations
	binary.Write(headers, binary.LittleEndian, uint32(0))        // PointerToLinenumbers
	binary.Write(headers, binary.LittleEndian, uint16(0))        // NumberOfRelocations
	binary.Write(headers, binary.LittleEndian, uint16(0))        // NumberOfLinenumbers
	binary.Write(headers, binary.LittleEndian, uint32(0x60000020)) // CNT_CODE | MEM_EXECUTE | MEM_READ

	// Section 2: .rdata (40 bytes)
	var rdataName [8]byte
	copy(rdataName[:], ".rdata")
	headers.Write(rdataName[:])
	binary.Write(headers, binary.LittleEndian, virtRDataLen)         // VirtualSize
	binary.Write(headers, binary.LittleEndian, rdataRVA)             // VirtualAddress (RVA)
	binary.Write(headers, binary.LittleEndian, rawRDataLen)          // SizeOfRawData
	binary.Write(headers, binary.LittleEndian, uint32(0x400+rawTextLen)) // PointerToRawData
	binary.Write(headers, binary.LittleEndian, uint32(0))            // PointerToRelocations
	binary.Write(headers, binary.LittleEndian, uint32(0))            // PointerToLinenumbers
	binary.Write(headers, binary.LittleEndian, uint16(0))            // NumberOfRelocations
	binary.Write(headers, binary.LittleEndian, uint16(0))            // NumberOfLinenumbers
	binary.Write(headers, binary.LittleEndian, uint32(0x40000040))    // CNT_INITIALIZED_DATA | MEM_READ

	// Section 3: .bss (40 bytes, SizeOfRawData = 0!)
	var bssName [8]byte
	copy(bssName[:], ".bss")
	headers.Write(bssName[:])
	binary.Write(headers, binary.LittleEndian, virtBSSLen)          // VirtualSize (e.g. 1 MB in RAM)
	binary.Write(headers, binary.LittleEndian, bssRVA)              // VirtualAddress (RVA)
	binary.Write(headers, binary.LittleEndian, uint32(0))           // SizeOfRawData = 0 (takes 0 bytes in file!)
	binary.Write(headers, binary.LittleEndian, uint32(0))           // PointerToRawData = 0
	binary.Write(headers, binary.LittleEndian, uint32(0))           // PointerToRelocations
	binary.Write(headers, binary.LittleEndian, uint32(0))           // PointerToLinenumbers
	binary.Write(headers, binary.LittleEndian, uint16(0))           // NumberOfRelocations
	binary.Write(headers, binary.LittleEndian, uint16(0))           // NumberOfLinenumbers
	binary.Write(headers, binary.LittleEndian, uint32(0xC0000080))  // CNT_UNINITIALIZED_DATA | MEM_READ | MEM_WRITE

	// Pad headers to 0x400
	hBytes := headers.Bytes()
	if len(hBytes) < 0x400 {
		hBytes = append(hBytes, make([]byte, 0x400-len(hBytes))...)
	}

	out := new(bytes.Buffer)
	out.Write(hBytes)
	out.Write(textCode)
	out.Write(rdataBytes)

	return out.Bytes(), nil
}

// PatchPESubsystem modifies the Subsystem field of a Windows PE binary in place.
// subsystem 2 = Windows GUI (no console), 3 = Windows CUI (console).
func PatchPESubsystem(exe []byte, subsystem uint16) bool {
	if len(exe) < 0x40 || exe[0] != 'M' || exe[1] != 'Z' {
		return false
	}
	e_lfanew := binary.LittleEndian.Uint32(exe[0x3C:])
	subsysOffset := int(e_lfanew) + 4 + 20 + 68
	if subsysOffset+2 > len(exe) {
		return false
	}
	if !bytes.Equal(exe[e_lfanew:e_lfanew+4], []byte("PE\x00\x00")) {
		return false
	}
	binary.LittleEndian.PutUint16(exe[subsysOffset:], subsystem)
	return true
}

// PatchFileSubsystem updates the Subsystem field of an on-disk Windows PE executable.
func PatchFileSubsystem(path string, subsystem uint16) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !PatchPESubsystem(data, subsystem) {
		return fmt.Errorf("invalid PE executable header")
	}
	return os.WriteFile(path, data, 0o755)
}
