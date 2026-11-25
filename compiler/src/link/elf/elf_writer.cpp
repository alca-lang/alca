// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

#include "link/elf/elf_writer.h"
#include <fstream>
#include <vector>
#include <cstring>
#include <stdexcept>

namespace pawl::link::elf {

namespace {

inline uint32_t alignUp(uint32_t val, uint32_t align) {
    return (val + align - 1) / align * align;
}

inline void put16(std::vector<uint8_t>& buf, uint16_t val) {
    buf.push_back(static_cast<uint8_t>(val));
    buf.push_back(static_cast<uint8_t>(val >> 8));
}

inline void put32(std::vector<uint8_t>& buf, uint32_t val) {
    for (int i = 0; i < 4; ++i) buf.push_back(static_cast<uint8_t>(val >> (i * 8)));
}

inline void put64(std::vector<uint8_t>& buf, uint64_t val) {
    for (int i = 0; i < 8; ++i) buf.push_back(static_cast<uint8_t>(val >> (i * 8)));
}

// Emits pawl_str_eq (RDI = s1, RSI = s2) -> RAX = 0/1 in machine bytes.
std::vector<uint8_t> emitStrEqHelper(uint64_t rodataBase) {
    std::vector<uint8_t> b;
    b.insert(b.end(), {0x48, 0x89, 0xF8});                     // mov rax, rdi
    b.insert(b.end(), {0x49, 0xC7, 0xC0, 0, 0, 0, 0x10});      // mov r8, 0x10000000
    b.insert(b.end(), {0x4C, 0x39, 0xC0});                     // cmp rax, r8
    b.push_back(0x73);                                         // jae .Ls1_heap
    size_t j1 = b.size(); b.push_back(0);
    b.insert(b.end(), {0x48, 0xB8}); put64(b, rodataBase + 2); // movabs rax, literal base
    b.insert(b.end(), {0x48, 0x01, 0xF8});                     // add rax, rdi
    size_t s1Heap = b.size();
    b[j1] = static_cast<uint8_t>(s1Heap - (j1 + 1));
    b.insert(b.end(), {0x49, 0x89, 0xC2});                     // mov r10, rax

    b.insert(b.end(), {0x48, 0x89, 0xF0});                     // mov rax, rsi
    b.insert(b.end(), {0x4C, 0x39, 0xC0});                     // cmp rax, r8
    b.push_back(0x73);                                         // jae .Ls2_heap
    size_t j2 = b.size(); b.push_back(0);
    b.insert(b.end(), {0x48, 0xB8}); put64(b, rodataBase + 2); // movabs rax, literal base
    b.insert(b.end(), {0x48, 0x01, 0xF0});                     // add rax, rsi
    size_t s2Heap = b.size();
    b[j2] = static_cast<uint8_t>(s2Heap - (j2 + 1));
    b.insert(b.end(), {0x49, 0x89, 0xC3});                     // mov r11, rax

    b.insert(b.end(), {0x49, 0x8B, 0x0A});                     // mov rcx, [r10]
    b.insert(b.end(), {0x49, 0x3B, 0x0B});                     // cmp rcx, [r11]
    b.push_back(0x75);                                         // jne .Lneq
    size_t jNe = b.size(); b.push_back(0);
    b.insert(b.end(), {0x49, 0x8D, 0x72, 0x08});               // lea rsi, [r10 + 8]
    b.insert(b.end(), {0x49, 0x8D, 0x7B, 0x08});               // lea rdi, [r11 + 8]
    b.insert(b.end(), {0x31, 0xC0});                           // xor eax, eax
    b.insert(b.end(), {0xF3, 0xA6});                           // repe cmpsb
    b.insert(b.end(), {0x0F, 0x94, 0xC0});                     // sete al
    b.insert(b.end(), {0x0F, 0xB6, 0xC0});                     // movzx eax, al
    b.push_back(0xEB);                                         // jmp .Ldone
    size_t jDone = b.size(); b.push_back(0);
    size_t neq = b.size();
    b[jNe] = static_cast<uint8_t>(neq - (jNe + 1));
    b.insert(b.end(), {0x31, 0xC0});                           // xor eax, eax
    size_t done = b.size();
    b[jDone] = static_cast<uint8_t>(done - (jDone + 1));
    b.push_back(0xC3);                                         // ret
    return b;
}

// Emits pawl_str_to_int (RDI = string) -> RAX = parsed integer.
std::vector<uint8_t> emitStrToIntHelper(uint64_t rodataBase) {
    std::vector<uint8_t> b;
    b.insert(b.end(), {0x48, 0x89, 0xF8});                 // mov rax, rdi
    b.insert(b.end(), {0x49, 0xC7, 0xC0, 0, 0, 0, 0x10});  // mov r8, 0x10000000
    b.insert(b.end(), {0x4C, 0x39, 0xC0});                 // cmp rax, r8
    b.push_back(0x73);                                     // jae .Lheap
    size_t jHeap = b.size(); b.push_back(0);
    b.insert(b.end(), {0x48, 0xB8}); put64(b, rodataBase + 2); // movabs rax, literal base
    b.insert(b.end(), {0x48, 0x01, 0xF8});                 // add rax, rdi
    size_t heap = b.size();
    b[jHeap] = static_cast<uint8_t>(heap - (jHeap + 1));

    b.insert(b.end(), {0x48, 0x89, 0xC6});                 // mov rsi, rax
    b.insert(b.end(), {0x48, 0x8B, 0x0E});                 // mov rcx, [rsi] (len)
    b.insert(b.end(), {0x48, 0x8D, 0x56, 0x08});           // lea rdx, [rsi + 8] (data)
    b.insert(b.end(), {0x31, 0xC0});                       // xor eax, eax
    b.insert(b.end(), {0x45, 0x31, 0xC0});                 // xor r8d, r8d (negative)
    b.insert(b.end(), {0x48, 0x85, 0xC9});                 // test rcx, rcx
    b.push_back(0x74);                                     // jz .Ldone
    size_t jEmpty = b.size(); b.push_back(0);
    b.insert(b.end(), {0x44, 0x0F, 0xB6, 0x0A});           // movzx r9d, byte [rdx]
    b.insert(b.end(), {0x41, 0x80, 0xF9, 0x2D});           // cmp r9b, '-'
    b.push_back(0x75);                                     // jne .Ldigits
    size_t jSign = b.size(); b.push_back(0);
    b.insert(b.end(), {0x41, 0xB8, 0x01, 0x00, 0x00, 0x00}); // mov r8d, 1
    b.insert(b.end(), {0x48, 0xFF, 0xC2});                 // inc rdx
    b.insert(b.end(), {0x48, 0xFF, 0xC9});                 // dec rcx
    size_t digits = b.size();
    b[jSign] = static_cast<uint8_t>(digits - (jSign + 1));

    b.insert(b.end(), {0x48, 0x85, 0xC9});                 // test rcx, rcx
    b.push_back(0x74);                                     // jz .Lfinish
    size_t jDigitsEnd = b.size(); b.push_back(0);
    b.insert(b.end(), {0x44, 0x0F, 0xB6, 0x0A});           // movzx r9d, byte [rdx]
    b.insert(b.end(), {0x41, 0x83, 0xE9, 0x30});           // sub r9d, '0'
    b.insert(b.end(), {0x41, 0x83, 0xF9, 0x09});           // cmp r9d, 9
    b.push_back(0x77);                                     // ja .Lfinish
    size_t jNonDigit = b.size(); b.push_back(0);
    b.insert(b.end(), {0x48, 0x6B, 0xC0, 0x0A});           // imul rax, rax, 10
    b.insert(b.end(), {0x4C, 0x01, 0xC8});                 // add rax, r9
    b.insert(b.end(), {0x48, 0xFF, 0xC2});                 // inc rdx
    b.insert(b.end(), {0x48, 0xFF, 0xC9});                 // dec rcx
    b.push_back(0xEB);                                     // jmp .Ldigits
    size_t jLoop = b.size(); b.push_back(0);
    b[jLoop] = static_cast<uint8_t>(static_cast<int32_t>(digits) - static_cast<int32_t>(jLoop + 1));

    size_t finish = b.size();
    b[jDigitsEnd] = static_cast<uint8_t>(finish - (jDigitsEnd + 1));
    b[jNonDigit] = static_cast<uint8_t>(finish - (jNonDigit + 1));
    b.insert(b.end(), {0x45, 0x85, 0xC0});                 // test r8d, r8d
    b.push_back(0x74);                                     // jz .Ldone
    size_t jSignDone = b.size(); b.push_back(0);
    b.insert(b.end(), {0x48, 0xF7, 0xD8});                 // neg rax
    size_t done = b.size();
    b[jEmpty] = static_cast<uint8_t>(done - (jEmpty + 1));
    b[jSignDone] = static_cast<uint8_t>(done - (jSignDone + 1));
    b.push_back(0xC3);                                     // ret
    return b;
}

} // namespace

bool linkELF(const codegen::x64::CompiledArtifact& art, const std::string& outputPath) {
    constexpr uint64_t elfBaseAddr = 0x400000;
    uint64_t textBase = elfBaseAddr + 0x1000;
    uint64_t rodataBase = elfBaseAddr + 0x2000;

    std::vector<uint8_t> rodata;
    rodata.push_back('\n');
    rodata.push_back(0);
    rodata.insert(rodata.end(), art.readOnlyData.begin(), art.readOnlyData.end());

    std::vector<uint8_t> text;

    // 1. _start
    // sys_brk(0) -> mov r15, rax
    text.insert(text.end(), {0x48, 0x31, 0xFF, 0xB8, 12, 0, 0, 0, 0x0F, 0x05, 0x49, 0x89, 0xC7});
    // sys_brk(current + 16MB)
    text.insert(text.end(), {0x48, 0x89, 0xC7, 0x48, 0x81, 0xC7, 0, 0, 0, 1, 0xB8, 12, 0, 0, 0, 0x0F, 0x05});
    // xor r14, r14
    text.insert(text.end(), {0x4D, 0x31, 0xF6});
    // call main
    text.push_back(0xE8);
    size_t callMainPatch = text.size();
    put32(text, 0);
    // sys_exit(eax)
    text.insert(text.end(), {0x48, 0x89, 0xC7, 0xB8, 60, 0, 0, 0, 0x0F, 0x05});

    while (text.size() % 16 != 0) text.push_back(0x90);

    // 2. pawl_print(str_ptr in RDI)
    uint32_t printOff = static_cast<uint32_t>(text.size());
    text.insert(text.end(), {0x48, 0x89, 0xF8}); // mov rax, rdi
    text.insert(text.end(), {0x49, 0xC7, 0xC0, 0, 0, 0, 0x10}); // mov r8, 0x10000000
    text.insert(text.end(), {0x4C, 0x39, 0xC0}); // cmp rax, r8
    text.insert(text.end(), {0x73, 0x0D});       // jae .Lheap (+13 bytes)
    text.insert(text.end(), {0x48, 0xB8});       // movabs rax, imm64
    put64(text, rodataBase + 2);
    text.insert(text.end(), {0x48, 0x01, 0xF8}); // add rax, rdi
    // .Lheap:
    text.insert(text.end(), {0x48, 0x8B, 0x10}); // mov rdx, [rax] (len)
    text.insert(text.end(), {0x48, 0x83, 0xC0, 0x08}); // add rax, 8
    text.insert(text.end(), {0x48, 0x89, 0xC6}); // mov rsi, rax
    text.insert(text.end(), {0xBF, 1, 0, 0, 0}); // mov edi, 1 (stdout)
    text.insert(text.end(), {0xB8, 1, 0, 0, 0}); // mov eax, 1 (sys_write)
    text.insert(text.end(), {0x0F, 0x05, 0xC3}); // syscall; ret

    while (text.size() % 16 != 0) text.push_back(0x90);

    // 3. pawl_println(str_ptr in RDI)
    uint32_t printlnOff = static_cast<uint32_t>(text.size());
    text.insert(text.end(), {0x57});             // push rdi
    text.push_back(0xE8);                        // call pawl_print
    int32_t relP = static_cast<int32_t>(printOff) - static_cast<int32_t>(text.size() + 4);
    put32(text, static_cast<uint32_t>(relP));
    text.insert(text.end(), {0x5F});             // pop rdi
    // write newline
    text.insert(text.end(), {0x48, 0xBE});       // movabs rsi, rodataBase
    put64(text, rodataBase);
    text.insert(text.end(), {0xBA, 1, 0, 0, 0}); // mov edx, 1
    text.insert(text.end(), {0xBF, 1, 0, 0, 0}); // mov edi, 1
    text.insert(text.end(), {0xB8, 1, 0, 0, 0}); // mov eax, 1
    text.insert(text.end(), {0x0F, 0x05, 0xC3}); // syscall; ret

    while (text.size() % 16 != 0) text.push_back(0x90);

    // 4. pawl_alloc(size in RDI) -> RAX
    uint32_t allocOff = static_cast<uint32_t>(text.size());
    text.insert(text.end(), {0x48, 0x83, 0xC7, 0x0F}); // add rdi, 15
    text.insert(text.end(), {0x48, 0x83, 0xE7, 0xF0}); // and rdi, -16
    text.insert(text.end(), {0x49, 0x89, 0xF8});       // mov r8, r15
    text.insert(text.end(), {0x49, 0x01, 0xFF});       // add r15, rdi
    text.insert(text.end(), {0x4C, 0x89, 0xC0, 0xC3}); // mov rax, r8; ret

    while (text.size() % 16 != 0) text.push_back(0x90);

    // 5. pawl_concat_str(s1 in RDI, s2 in RSI) -> RAX
    uint32_t concatOff = static_cast<uint32_t>(text.size());
    // Use bump allocator R15
    text.insert(text.end(), {
        0x55, 0x48, 0x89, 0xE5, 0x53, 0x56, 0x57, 0x41, 0x54, 0x41, 0x55,
        0x48, 0x8B, 0x1F,       // mov rbx, [rdi] (len1)
        0x48, 0x8B, 0x2E,       // mov rbp, [rsi] (len2)
        0x4D, 0x8D, 0x24, 0x2B, // lea r12, [rbx + rbp] (totLen)
        0x49, 0x89, 0xF8,       // mov r8, r15 (newStr)
        0x4D, 0x89, 0x20,       // mov [r8], r12
        0x4D, 0x8D, 0x78, 0x08, // lea r15, [r8 + 8] (dst)
        0x48, 0x8D, 0x77, 0x08, // lea rsi, [rdi + 8] (src1)
        0x48, 0x89, 0xD9,       // mov rcx, rbx
        0xF3, 0xA4,             // rep movsb
        0x48, 0x8D, 0x76, 0x08, // lea rsi, [rsi + 8] (src2)
        0x48, 0x89, 0xE9,       // mov rcx, rbp
        0xF3, 0xA4,             // rep movsb
        0xC6, 0x07, 0x00,       // mov byte ptr [rdi], 0
        0x49, 0x8D, 0x5C, 0x24, 0x18, // lea rbx, [r12 + 24]
        0x48, 0x83, 0xE3, 0xF0, // and rbx, -16
        0x4D, 0x8D, 0x3C, 0x18, // lea r15, [r8 + rbx] (advance R15)
        0x4C, 0x89, 0xC0,       // mov rax, r8
        0x41, 0x5D, 0x41, 0x5C, 0x5F, 0x5E, 0x5B, 0x5D, 0xC3
    });

    while (text.size() % 16 != 0) text.push_back(0x90);

    // 6. pawl_str_eq(s1 in RDI, s2 in RSI) -> RAX
    uint32_t strEqOff = static_cast<uint32_t>(text.size());
    auto strEqBytes = emitStrEqHelper(rodataBase);
    text.insert(text.end(), strEqBytes.begin(), strEqBytes.end());

    // 7. pawl_str_to_int(s in RDI) -> RAX
    uint32_t strToIntOff = static_cast<uint32_t>(text.size());
    auto strToIntBytes = emitStrToIntHelper(rodataBase);
    text.insert(text.end(), strToIntBytes.begin(), strToIntBytes.end());

    while (text.size() % 16 != 0) text.push_back(0x90);

    // 8. pawl_int_to_str(n in RDI) -> RAX
    uint32_t intToStrOff = static_cast<uint32_t>(text.size());
    // Convert n in RDI, alloc via R15
    std::vector<uint8_t> intToStrBytes = {
        0x55, 0x48, 0x89, 0xE5, 0x53, 0x56, 0x57, 0x41, 0x54, 0x48, 0x83, 0xEC, 0x50, 0x48, 0x8D, 0x74,
        0x24, 0x40, 0xC6, 0x06, 0x00, 0x48, 0x89, 0xF8, 0x45, 0x31, 0xC0, 0x48, 0x83, 0xF8, 0x00, 0x7D,
        0x09, 0x48, 0xF7, 0xD8, 0x41, 0xB8, 0x01, 0x00, 0x00, 0x00, 0x48, 0x83, 0xF8, 0x00, 0x75, 0x08,
        0x48, 0xFF, 0xCE, 0xC6, 0x06, 0x30, 0xEB, 0x1A, 0x48, 0x31, 0xD2, 0x49, 0xC7, 0xC1, 0x0A, 0x00,
        0x00, 0x00, 0x49, 0xF7, 0xF1, 0x80, 0xC2, 0x30, 0x48, 0xFF, 0xCE, 0x88, 0x16, 0x48, 0x85, 0xC0,
        0x75, 0xE6, 0x45, 0x85, 0xC0, 0x74, 0x06, 0x48, 0xFF, 0xCE, 0xC6, 0x06, 0x2D, 0x48, 0x8D, 0x44,
        0x24, 0x40, 0x48, 0x29, 0xF0, 0x48, 0x89, 0xC3, 0x48, 0x89, 0xF7, 0x48, 0x8D, 0x7B, 0x09, 0xE8,
        0x00, 0x00, 0x00, 0x00, 0x48, 0x89, 0x18, 0x4C, 0x8D, 0x40, 0x08, 0x48, 0x31, 0xC9, 0x48, 0x39,
        0xD9, 0x7D, 0x0C, 0x8A, 0x14, 0x0F, 0x41, 0x88, 0x14, 0x08, 0x48, 0xFF, 0xC1, 0xEB, 0xEF, 0x41,
        0xC6, 0x04, 0x18, 0x00, 0x48, 0x8D, 0x65, 0xE0, 0x41, 0x5C, 0x5F, 0x5E, 0x5B, 0x5D, 0xC3, 0x90
    };
    int32_t relAlloc = static_cast<int32_t>(allocOff) - static_cast<int32_t>(intToStrOff + 111 + 5);
    std::memcpy(&intToStrBytes[112], &relAlloc, 4);
    text.insert(text.end(), intToStrBytes.begin(), intToStrBytes.end());

    while (text.size() % 16 != 0) text.push_back(0x90);

    // 7. User code
    uint32_t userCodeStart = static_cast<uint32_t>(text.size());
    uint32_t mainTarget = userCodeStart + art.entryOffset;
    int32_t relMain = static_cast<int32_t>(mainTarget) - static_cast<int32_t>(callMainPatch + 4);
    std::memcpy(&text[callMainPatch], &relMain, 4);

    std::vector<uint8_t> userCodeCopy = art.code;
    for (const auto& fix : art.callFixups) {
        uint32_t targetHelper = 0;
        if (fix.funcName == "pawl_print") targetHelper = printOff;
        else if (fix.funcName == "pawl_println") targetHelper = printlnOff;
        else if (fix.funcName == "pawl_alloc") targetHelper = allocOff;
        else if (fix.funcName == "pawl_concat_str") targetHelper = concatOff;
        else if (fix.funcName == "pawl_str_eq") targetHelper = strEqOff;
        else if (fix.funcName == "pawl_str_to_int") targetHelper = strToIntOff;
        else if (fix.funcName == "pawl_int_to_str") targetHelper = intToStrOff;
        else return false;

        uint32_t absCallSite = userCodeStart + static_cast<uint32_t>(fix.insEnd);
        int32_t rel32 = static_cast<int32_t>(targetHelper) - static_cast<int32_t>(absCallSite);
        std::memcpy(&userCodeCopy[fix.patchOffset], &rel32, 4);
    }
    text.insert(text.end(), userCodeCopy.begin(), userCodeCopy.end());

    // 2 PT_LOAD segments: Page 1 (.text, RX) and Page 2 (.rodata, R)
    uint64_t textFileSize = text.size();
    uint64_t rodataFileSize = rodata.size();

    std::vector<uint8_t> elf;

    // ELF Header (64 bytes)
    elf.insert(elf.end(), {0x7F, 'E', 'L', 'F', 2, 1, 1, 0}); // 64-bit, LittleEndian, version 1, System V
    elf.resize(16, 0);
    put16(elf, 2);      // ET_EXEC
    put16(elf, 0x3E);   // EM_X86_64
    put32(elf, 1);      // EV_CURRENT
    put64(elf, textBase); // Entry point
    put64(elf, 64);     // e_phoff (immediately after ELF header)
    put64(elf, 0);      // e_shoff
    put32(elf, 0);      // e_flags
    put16(elf, 64);     // e_ehsize
    put16(elf, 56);     // e_phentsize
    put16(elf, 2);      // e_phnum (2 PT_LOADs)
    put16(elf, 64);     // e_shentsize
    put16(elf, 0);      // e_shnum
    put16(elf, 0);      // e_shstrndx

    // Program Header 1: .text (RX)
    put32(elf, 1);      // PT_LOAD
    put32(elf, 5);      // PF_R | PF_X
    put64(elf, 0x1000); // p_offset
    put64(elf, textBase); // p_vaddr
    put64(elf, textBase); // p_paddr
    put64(elf, textFileSize); // p_filesz
    put64(elf, textFileSize); // p_memsz
    put64(elf, 0x1000); // p_align

    // Program Header 2: .rodata (R)
    put32(elf, 1);      // PT_LOAD
    put32(elf, 4);      // PF_R
    put64(elf, 0x2000); // p_offset
    put64(elf, rodataBase); // p_vaddr
    put64(elf, rodataBase); // p_paddr
    put64(elf, rodataFileSize); // p_filesz
    put64(elf, rodataFileSize); // p_memsz
    put64(elf, 0x1000); // p_align

    // Pad to 0x1000
    while (elf.size() < 0x1000) elf.push_back(0);
    elf.insert(elf.end(), text.begin(), text.end());

    // Pad to 0x2000
    while (elf.size() < 0x2000) elf.push_back(0);
    elf.insert(elf.end(), rodata.begin(), rodata.end());

    std::ofstream out(outputPath, std::ios::binary);
    if (!out) return false;
    out.write(reinterpret_cast<const char*>(elf.data()), elf.size());
    return true;
}

} // namespace pawl::link::elf
