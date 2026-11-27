// Copyright 2026 ULang++ Programming Language
// SPDX-License-Identifier: Apache-2.0

#include "link/pe/efi_writer.h"
#include "link/heap_helpers.h"
#include <cstring>
#include <fstream>
#include <vector>

namespace ulang::link::efi {
namespace {

inline uint32_t alignUp(uint32_t v, uint32_t a) { return (v + a - 1) / a * a; }

inline void put16(std::vector<uint8_t>& b, uint16_t v) {
    b.push_back(static_cast<uint8_t>(v));
    b.push_back(static_cast<uint8_t>(v >> 8));
}
inline void put32(std::vector<uint8_t>& b, uint32_t v) {
    for (int i = 0; i < 4; ++i) b.push_back(static_cast<uint8_t>(v >> (i * 8)));
}
inline void put64(std::vector<uint8_t>& b, uint64_t v) {
    for (int i = 0; i < 8; ++i) b.push_back(static_cast<uint8_t>(v >> (i * 8)));
}

} // namespace

bool linkEFIApp(const codegen::x64::CompiledArtifact& art, const std::string& outputPath) {
    constexpr uint32_t imageBaseLo = 0x00400000;
    constexpr uint64_t imageBase = 0x400000;
    constexpr uint32_t textRVA = 0x1000;
    constexpr uint32_t dataRVA = 0x2000;
    constexpr uint32_t bssRVA = 0x3000;
    constexpr uint32_t bssSize = 0x100000;

    // Two hidden heap slots appended to the user globals.
    std::vector<ir::GlobalVar> globals = art.globals;
    bool hasHeap = false, hasFree = false;
    for (const auto& g : globals) {
        if (g.name == "__ulang_heap") hasHeap = true;
        if (g.name == "__ulang_free") hasFree = true;
    }
    if (!hasHeap) globals.push_back({"__ulang_heap", imageBase + bssRVA});
    if (!hasFree) globals.push_back({"__ulang_free", 0});
    uint32_t heapIndex = 0, freeIndex = 0;
    for (uint32_t i = 0; i < globals.size(); ++i) {
        if (globals[i].name == "__ulang_heap") heapIndex = i;
        if (globals[i].name == "__ulang_free") freeIndex = i;
    }

    // ---- .text -------------------------------------------------------------
    std::vector<uint8_t> text;
    uint32_t target = art.entryOffset;
    auto it = art.functionRVAs.find("efi_main");
    if (it != art.functionRVAs.end()) target = it->second;
    text.push_back(0xE9); // jmp rel32 -> efi_main (funcRVA relative to code start)
    put32(text, target);

    // rodata follows the globals inside .data (same +2 convention as ELF/PE).
    uint32_t rodataRVA = dataRVA + static_cast<uint32_t>(globals.size() * 8);
    uint32_t dataSize = static_cast<uint32_t>(globals.size() * 8) + 2 +
                        static_cast<uint32_t>(art.readOnlyData.size());

    const uint32_t codeStart = static_cast<uint32_t>(text.size());
    std::vector<uint8_t> code = art.code;
    // Placeholders patched after helpers are emitted.
    std::vector<uint32_t> fixupHelperKinds;
    for (const auto& fix : art.callFixups) {
        if (fix.funcName == "ulang_alloc") fixupHelperKinds.push_back(0);
        else if (fix.funcName == "ulang_free") fixupHelperKinds.push_back(1);
        else if (fix.funcName == "ulang_str_ptr") fixupHelperKinds.push_back(2);
        else return false;
    }
    text.insert(text.end(), code.begin(), code.end());

    auto emitHelper = [&](uint32_t slotIndex, const std::vector<uint8_t>& body) -> uint32_t {
        while (text.size() % 16 != 0) text.push_back(0x90);
        uint32_t off = static_cast<uint32_t>(text.size());
        text.insert(text.end(), {0x4C, 0x8D, 0x05}); // lea r8, [rip+disp32]
        size_t dispAt = text.size();
        put32(text, 0);
        uint32_t slotRVA = dataRVA + slotIndex * 8;
        int32_t disp = static_cast<int32_t>(slotRVA) -
                       static_cast<int32_t>(textRVA + dispAt + 4);
        std::memcpy(&text[dispAt], &disp, 4);
        text.insert(text.end(), body.begin(), body.end());
        return off;
    };
    uint32_t allocHelper = emitHelper(heapIndex, emitHeapAllocHelperSlotReg());
    uint32_t freeHelper = emitHelper(heapIndex, emitHeapFreeHelperSlotReg());

    // ulang_str_ptr: RDI = literal offset -> RAX = rodataBase + 2 + offset.
    while (text.size() % 16 != 0) text.push_back(0x90);
    uint32_t strPtrHelper = static_cast<uint32_t>(text.size());
    text.insert(text.end(), {0x48, 0x8D, 0x05}); // lea rax, [rip+disp32]
    size_t strDispAt = text.size();
    put32(text, 0);
    int32_t strDisp = static_cast<int32_t>(rodataRVA + 2) -
                      static_cast<int32_t>(textRVA + strDispAt + 4);
    std::memcpy(&text[strDispAt], &strDisp, 4);
    text.insert(text.end(), {0x48, 0x01, 0xF8, 0xC3}); // add rax, rdi; ret

    // Patch call fixups now that helper offsets are known.
    uint32_t fi = 0;
    for (const auto& fix : art.callFixups) {
        uint32_t kind = fixupHelperKinds[fi++];
        uint32_t helper = kind == 0 ? allocHelper : (kind == 1 ? freeHelper : strPtrHelper);
        uint32_t at = codeStart + static_cast<uint32_t>(fix.patchOffset);
        int32_t rel = static_cast<int32_t>(helper) -
                      static_cast<int32_t>(codeStart + fix.insEnd);
        std::memcpy(&text[at], &rel, 4);
    }
    for (const auto& fix : art.dataFixups) {
        if (fix.globalIndex >= globals.size()) return false;
        uint32_t at = codeStart + static_cast<uint32_t>(fix.patchOffset);
        uint32_t globRVA = dataRVA + fix.globalIndex * 8;
        int32_t disp = static_cast<int32_t>(globRVA) -
                       static_cast<int32_t>(textRVA + codeStart + fix.insEnd);
        std::memcpy(&text[at], &disp, 4);
    }

    uint32_t textRaw = alignUp(static_cast<uint32_t>(text.size()), 0x200);
    uint32_t dataRaw = alignUp(dataSize, 0x200);

    // ---- PE image ----------------------------------------------------------
    std::vector<uint8_t> pe;
    pe.insert(pe.end(), {'M', 'Z'});
    pe.resize(0x3C, 0);
    put32(pe, 0x40);
    pe.insert(pe.end(), {'P', 'E', 0, 0});

    put16(pe, 0x8664);   // AMD64
    put16(pe, 3);        // .text, .data, .bss
    put32(pe, 0); put32(pe, 0); put32(pe, 0);
    put16(pe, 0xF0);     // SizeOfOptionalHeader
    put16(pe, 0x0022);   // EXECUTABLE_IMAGE | LARGE_ADDRESS_AWARE

    put16(pe, 0x020B);   // PE32+
    put16(pe, 0x0100);
    put32(pe, textRaw);
    put32(pe, dataRaw);
    put32(pe, 0);        // uninitialized data (bss is virtual only)
    put32(pe, textRVA);  // AddressOfEntryPoint
    put32(pe, textRVA);  // BaseOfCode
    put64(pe, imageBase);
    put32(pe, 0x1000);   // SectionAlignment
    put32(pe, 0x200);    // FileAlignment
    put16(pe, 6); put16(pe, 0);
    put16(pe, 0); put16(pe, 0);
    put16(pe, 6); put16(pe, 0);
    put32(pe, 0);
    put32(pe, alignUp(bssRVA + bssSize, 0x1000)); // SizeOfImage
    put32(pe, 0x400);    // SizeOfHeaders
    put32(pe, 0);        // CheckSum
    put16(pe, 10);       // Subsystem: EFI_APPLICATION
    put16(pe, 0);        // DllCharacteristics
    put64(pe, 0x100000); put64(pe, 0x10000);
    put64(pe, 0x100000); put64(pe, 0x10000);
    put32(pe, 0);
    put32(pe, 16);       // NumberOfRvaAndSizes
    for (int i = 0; i < 16; ++i) { put32(pe, 0); put32(pe, 0); }

    auto section = [&](const char* name, uint32_t vsize, uint32_t rva,
                       uint32_t rawSize, uint32_t rawPtr, uint32_t flags) {
        size_t len = std::strlen(name);
        for (size_t i = 0; i < 8; ++i) {
            pe.push_back(i < len ? static_cast<uint8_t>(name[i]) : 0);
        }
        put32(pe, vsize); put32(pe, rva); put32(pe, rawSize); put32(pe, rawPtr);
        put32(pe, 0); put32(pe, 0); put16(pe, 0); put16(pe, 0);
        put32(pe, flags);
    };
    section(".text", alignUp(static_cast<uint32_t>(text.size()), 0x1000),
            textRVA, textRaw, 0x400, 0x60000020);
    section(".data", alignUp(dataSize, 0x1000),
            dataRVA, dataRaw, 0x400 + textRaw, 0xC0000040);
    section(".bss", bssSize, bssRVA, 0, 0, 0xC0000080);

    while (pe.size() < 0x400) pe.push_back(0);
    pe.insert(pe.end(), text.begin(), text.end());
    while (pe.size() < 0x400 + textRaw) pe.push_back(0);
    for (const auto& g : globals) put64(pe, g.initValue);
    pe.push_back('\n');
    pe.push_back(0);
    pe.insert(pe.end(), art.readOnlyData.begin(), art.readOnlyData.end());
    while (pe.size() < 0x400 + textRaw + dataRaw) pe.push_back(0);

    std::ofstream out(outputPath, std::ios::binary);
    if (!out) return false;
    out.write(reinterpret_cast<const char*>(pe.data()), pe.size());
    return out.good();
}

} // namespace ulang::link::efi
