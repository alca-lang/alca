// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

#include "link/elf/elf_writer.h"
#include <fstream>
#include <vector>
#include <cstring>
#include <stdexcept>

namespace ulang::link::elf {

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

// Emits ulang_str_eq (RDI = s1, RSI = s2) -> RAX = 0/1 in machine bytes.
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

// Emits ulang_str_ptr (RDI = literal offset or heap pointer) -> RAX = real pointer.
std::vector<uint8_t> emitStrPtrHelper(uint64_t rodataBase) {
    std::vector<uint8_t> b;
    b.insert(b.end(), {0x48, 0x89, 0xF8});                // mov rax, rdi
    b.insert(b.end(), {0x49, 0xC7, 0xC0, 0, 0, 0, 0x10}); // mov r8, 0x10000000
    b.insert(b.end(), {0x4C, 0x39, 0xC0});                // cmp rax, r8
    b.push_back(0x73);                                    // jae done
    size_t jDone = b.size(); b.push_back(0);
    b.insert(b.end(), {0x48, 0xB8}); put64(b, rodataBase + 2); // movabs rax, literal base
    b.insert(b.end(), {0x48, 0x01, 0xF8});                // add rax, rdi
    size_t done = b.size();
    b[jDone] = static_cast<uint8_t>(done - (jDone + 1));
    b.push_back(0xC3);                                    // ret
    return b;
}

// Emits ulang_str_to_int (RDI = string) -> RAX = parsed integer.
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

    // 2. ulang_print(str_ptr in RDI)
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

    // 3. ulang_println(str_ptr in RDI)
    uint32_t printlnOff = static_cast<uint32_t>(text.size());
    text.insert(text.end(), {0x57});             // push rdi
    text.push_back(0xE8);                        // call ulang_print
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

    // 4. ulang_alloc(size in RDI) -> RAX
    uint32_t allocOff = static_cast<uint32_t>(text.size());
    text.insert(text.end(), {0x48, 0x83, 0xC7, 0x0F}); // add rdi, 15
    text.insert(text.end(), {0x48, 0x83, 0xE7, 0xF0}); // and rdi, -16
    text.insert(text.end(), {0x4C, 0x89, 0xF8});       // mov rax, r15
    text.insert(text.end(), {0x49, 0x01, 0xFF});       // add r15, rdi
    text.insert(text.end(), {0xC3});                   // ret

    while (text.size() % 16 != 0) text.push_back(0x90);

    // 5. ulang_concat_str(s1 in RDI, s2 in RSI) -> RAX
    uint32_t concatOff = static_cast<uint32_t>(text.size());
    std::vector<uint8_t> concatBytes = {
        0x55, 0x48, 0x89, 0xE5, 0x53, 0x41, 0x54, 0x41, 0x55, 0x56, 0x57, 0x48, 0x8B, 0x1F, 0x4C, 0x8B,
        0x26, 0x4E, 0x8D, 0x2C, 0x23, 0x4C, 0x89, 0xF8, 0x4C, 0x89, 0x28, 0x4C, 0x8D, 0x40, 0x08, 0x48,
        0x8D, 0x77, 0x08, 0x48, 0x31, 0xC9, 0x48, 0x39, 0xD9, 0x7D, 0x0C, 0x8A, 0x14, 0x0E, 0x41, 0x88,
        0x14, 0x08, 0x48, 0xFF, 0xC1, 0xEB, 0xEF, 0x5F, 0x5E, 0x56, 0x57, 0x48, 0x8D, 0x76, 0x08, 0x4D,
        0x8D, 0x0C, 0x18, 0x48, 0x31, 0xC9, 0x4C, 0x39, 0xE1, 0x7D, 0x0C, 0x8A, 0x14, 0x0E, 0x41, 0x88,
        0x14, 0x09, 0x48, 0xFF, 0xC1, 0xEB, 0xEF, 0x43, 0xC6, 0x04, 0x28, 0x00, 0x49, 0x8D, 0x5D, 0x18,
        0x48, 0x83, 0xE3, 0xF0, 0x49, 0x01, 0xDF, 0x5F, 0x5E, 0x41, 0x5D, 0x41, 0x5C, 0x5B, 0x5D, 0xC3
    };
    text.insert(text.end(), concatBytes.begin(), concatBytes.end());

    while (text.size() % 16 != 0) text.push_back(0x90);

    // 6. ulang_str_eq(s1 in RDI, s2 in RSI) -> RAX
    uint32_t strEqOff = static_cast<uint32_t>(text.size());
    auto strEqBytes = emitStrEqHelper(rodataBase);
    text.insert(text.end(), strEqBytes.begin(), strEqBytes.end());

    // 7. ulang_str_ptr(s in RDI) -> RAX
    uint32_t strPtrOff = static_cast<uint32_t>(text.size());
    auto strPtrBytes = emitStrPtrHelper(rodataBase);
    text.insert(text.end(), strPtrBytes.begin(), strPtrBytes.end());

    // 8. ulang_str_to_int(s in RDI) -> RAX
    uint32_t strToIntOff = static_cast<uint32_t>(text.size());
    auto strToIntBytes = emitStrToIntHelper(rodataBase);
    text.insert(text.end(), strToIntBytes.begin(), strToIntBytes.end());

    while (text.size() % 16 != 0) text.push_back(0x90);

    // 9. ulang_int_to_str(n in RDI) -> RAX
    uint32_t intToStrOff = static_cast<uint32_t>(text.size());
    std::vector<uint8_t> intToStrBytes = {
        0x55, 0x48, 0x89, 0xE5, 0x53, 0x41, 0x54, 0x41, 0x55, 0x48, 0x83, 0xEC, 0x28, 0x48, 0x89, 0xF8,
        0x48, 0x8D, 0x4D, 0xFF, 0xC6, 0x01, 0x00, 0x49, 0xC7, 0xC4, 0x00, 0x00, 0x00, 0x00, 0x49, 0xC7,
        0xC5, 0x00, 0x00, 0x00, 0x00, 0x48, 0x83, 0xF8, 0x00, 0x7D, 0x0A, 0x48, 0xF7, 0xD8, 0x49, 0xC7,
        0xC5, 0x01, 0x00, 0x00, 0x00, 0x48, 0x83, 0xF8, 0x00, 0x75, 0x0B, 0x48, 0xFF, 0xC9, 0xC6, 0x01,
        0x30, 0x49, 0xFF, 0xC4, 0xEB, 0x1D, 0x48, 0x31, 0xD2, 0x48, 0xC7, 0xC3, 0x0A, 0x00, 0x00, 0x00,
        0x48, 0xF7, 0xF3, 0x80, 0xC2, 0x30, 0x48, 0xFF, 0xC9, 0x88, 0x11, 0x49, 0xFF, 0xC4, 0x48, 0x85,
        0xC0, 0x75, 0xE3, 0x4D, 0x85, 0xED, 0x74, 0x09, 0x48, 0xFF, 0xC9, 0xC6, 0x01, 0x2D, 0x49, 0xFF,
        0xC4, 0x4C, 0x89, 0xF8, 0x4C, 0x89, 0x20, 0x4C, 0x8D, 0x40, 0x08, 0x48, 0x31, 0xD2, 0x4C, 0x39,
        0xE2, 0x7D, 0x0C, 0x8A, 0x1C, 0x11, 0x41, 0x88, 0x1C, 0x10, 0x48, 0xFF, 0xC2, 0xEB, 0xEF, 0x43,
        0xC6, 0x04, 0x20, 0x00, 0x49, 0x8D, 0x5C, 0x24, 0x18, 0x48, 0x83, 0xE3, 0xF0, 0x49, 0x01, 0xDF,
        0x48, 0x83, 0xC4, 0x28, 0x41, 0x5D, 0x41, 0x5C, 0x5B, 0x5D, 0xC3
    };
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
        if (fix.funcName == "ulang_print") targetHelper = printOff;
        else if (fix.funcName == "ulang_println") targetHelper = printlnOff;
        else if (fix.funcName == "ulang_alloc") targetHelper = allocOff;
        else if (fix.funcName == "ulang_concat_str") targetHelper = concatOff;
        else if (fix.funcName == "ulang_str_eq") targetHelper = strEqOff;
        else if (fix.funcName == "ulang_str_ptr") targetHelper = strPtrOff;
        else if (fix.funcName == "ulang_str_to_int") targetHelper = strToIntOff;
        else if (fix.funcName == "ulang_int_to_str") targetHelper = intToStrOff;
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

bool linkRawBinary(const codegen::x64::CompiledArtifact& art, const std::string& outputPath) {
    std::vector<uint8_t> bin;
    if (art.entryOffset > 0) {
        bin.push_back(0xE9);
        int32_t rel = static_cast<int32_t>(art.entryOffset) - 5;
        put32(bin, static_cast<uint32_t>(rel));
    }
    bin.insert(bin.end(), art.code.begin(), art.code.end());

    std::ofstream out(outputPath, std::ios::binary);
    if (!out) return false;
    out.write(reinterpret_cast<const char*>(bin.data()), bin.size());
    return true;
}

bool linkMultibootELF(const codegen::x64::CompiledArtifact& art, const std::string& outputPath) {
    // 32-bit Multiboot1 / Multiboot2 header compatible ELF (for GRUB, QEMU -kernel)
    // Multiboot 1 Magic: 0x1BADB002, flags = 0x00, checksum = -(magic + flags)
    constexpr uint32_t mbMagic = 0x1BADB002;
    constexpr uint32_t mbFlags = 0x00000000;
    constexpr uint32_t mbChecksum = -(mbMagic + mbFlags);

    std::vector<uint8_t> text;
    // Multiboot header must be within first 8KB of OS image
    put32(text, mbMagic);
    put32(text, mbFlags);
    put32(text, mbChecksum);
    while (text.size() % 16 != 0) text.push_back(0x90);

    // Entry point: jmp to user entry
    uint32_t codeStart = static_cast<uint32_t>(text.size());
    text.insert(text.end(), art.code.begin(), art.code.end());

    uint64_t loadBase = 0x100000; // Standard 1MB load address for x86 kernels
    uint64_t entryPoint = loadBase + 0x1000 + codeStart + art.entryOffset;

    std::vector<uint8_t> elf;
    // ELF Header (64-bit ELF executable)
    elf.insert(elf.end(), {0x7F, 'E', 'L', 'F', 2, 1, 1, 0});
    elf.resize(16, 0);
    put16(elf, 2);          // ET_EXEC
    put16(elf, 0x3E);       // EM_X86_64
    put32(elf, 1);          // EV_CURRENT
    put64(elf, entryPoint); // Entry point
    put64(elf, 64);         // e_phoff
    put64(elf, 0);          // e_shoff
    put32(elf, 0);          // e_flags
    put16(elf, 64);         // e_ehsize
    put16(elf, 56);         // e_phentsize
    put16(elf, 1);          // e_phnum (1 PT_LOAD)
    put16(elf, 64);         // e_shentsize
    put16(elf, 0);          // e_shnum
    put16(elf, 0);          // e_shstrndx

    // Program Header 1: .text + rodata (RWE)
    uint64_t fileSize = text.size();
    put32(elf, 1);          // PT_LOAD
    put32(elf, 7);          // PF_R | PF_W | PF_X
    put64(elf, 0x1000);     // p_offset
    put64(elf, loadBase + 0x1000); // p_vaddr
    put64(elf, loadBase + 0x1000); // p_paddr
    put64(elf, fileSize);   // p_filesz
    put64(elf, fileSize);   // p_memsz
    put64(elf, 0x1000);     // p_align

    // Pad to 0x1000
    while (elf.size() < 0x1000) elf.push_back(0);
    elf.insert(elf.end(), text.begin(), text.end());

    std::ofstream out(outputPath, std::ios::binary);
    if (!out) return false;
    out.write(reinterpret_cast<const char*>(elf.data()), elf.size());
    return true;
}

} // namespace ulang::link::elf
