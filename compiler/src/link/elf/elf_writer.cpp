// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

#include "link/elf/elf_writer.h"
#include "link/heap_helpers.h"
#include <fstream>
#include <iostream>
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

// Emits a freestanding print/println helper.  RDI is a string handle: either a
// literal offset (< 0x10000000) or a heap pointer.  The string layout is
// { u64 length; bytes... }.  Output goes to the VGA text buffer and COM1 so it
// is visible both on screen and in QEMU's serial log.  R10/R11 hold the string
// pointer and the VGA cursor slot; a separate slot at payloadVa+8 keeps the
// cursor across calls.  strBase is patched to rodataVa+2 after layout.
std::vector<uint8_t> emitRawPrintHelper(uint64_t strBase, uint64_t cursorVa, bool newline) {
    std::vector<uint8_t> b;
    auto e = [&](std::initializer_list<uint8_t> xs) {
        b.insert(b.end(), xs.begin(), xs.end());
    };
    auto mark = [&]() { return static_cast<int>(b.size()); };
    auto patch8 = [&](int at, int target) {
        int32_t rel = target - (at + 1);
        b[at] = static_cast<uint8_t>(static_cast<int8_t>(rel));
    };

    e({0x48, 0x89, 0xF8});                                // mov rax, rdi
    e({0x49, 0xC7, 0xC0, 0, 0, 0, 0x10});                 // mov r8, 0x10000000
    e({0x4C, 0x39, 0xC0});                                // cmp rax, r8
    e({0x73, 0}); int jHeap = mark() - 1;                  // jae .Lheap
    e({0x49, 0xBA}); for (int i = 0; i < 8; ++i) b.push_back(static_cast<uint8_t>(strBase >> (i * 8)));
    e({0x4C, 0x01, 0xD0});                                // add rax, r10
    int heap = mark(); b[jHeap] = static_cast<uint8_t>(heap - (jHeap + 1));

    e({0x49, 0x89, 0xC2});                                // mov r10, rax (string ptr)
    e({0x49, 0x8B, 0x0A});                                // mov rcx, [r10] (len)
    e({0x49, 0x8D, 0x42, 0x08});                          // lea rax, [r10+8] (data ptr)
    e({0x48, 0x85, 0xC9});                                // test rcx, rcx
    e({0x74, 0}); int jDone = mark() - 1;                  // jz .Ldone
    int loopStart = mark();

    e({0x44, 0x0F, 0xB6, 0x08});                          // movzx r9d, byte [rax]
    e({0x48, 0xFF, 0xC0});                                // inc rax
    // --- VGA text buffer (r10 reused as the cell address) ---
    e({0x49, 0xBB}); for (int i = 0; i < 8; ++i) b.push_back(static_cast<uint8_t>(cursorVa >> (i * 8)));
    e({0x4F, 0x8B, 0x03});                                // mov r8, [r11] (cursor)
    e({0x49, 0x81, 0xF8, 0xD0, 0x07, 0x00, 0x00});        // cmp r8, 2000
    e({0x73, 0}); int jNoVga = mark() - 1;                // jae .Lno_vga
    e({0x49, 0xBA, 0x00, 0x80, 0x0B, 0, 0, 0, 0, 0});     // movabs r10, 0xB8000
    e({0x49, 0xD1, 0xE0});                                // shl r8, 1
    e({0x4D, 0x01, 0xC2});                                // add r10, r8
    e({0x45, 0x88, 0x0A});                                // mov [r10], r9b
    e({0x41, 0xC6, 0x42, 0x01, 0x0F});                    // mov byte [r10+1], 0x0F
    e({0x49, 0xFF, 0x03});                                // inc qword [r11]
    int noVga = mark(); b[jNoVga] = static_cast<uint8_t>(noVga - (jNoVga + 1));

    // --- COM1 serial port (DX is now free; AL keeps only the low byte) ---
    e({0x66, 0xBA, 0xF8, 0x03});                          // mov dx, 0x3F8
    e({0x50});                                            // push rax (save data ptr)
    e({0x44, 0x88, 0xC8});                                // mov al, r9b
    e({0xEE});                                            // out dx, al
    e({0x58});                                            // pop rax
    e({0x48, 0xFF, 0xC9});                                // dec rcx
    e({0x75, 0}); int jLoop = mark() - 1;                  // jnz .Lloop
    patch8(jLoop, loopStart);
    int done = mark(); b[jDone] = static_cast<uint8_t>(done - (jDone + 1));

    if (newline) {
        e({0x66, 0xBA, 0xF8, 0x03});                      // mov dx, 0x3F8
        e({0xB8, 0x0A, 0, 0, 0});                         // mov eax, 10
        e({0xEE});                                        // out dx, al
        e({0x49, 0xBB}); for (int i = 0; i < 8; ++i) b.push_back(static_cast<uint8_t>(cursorVa >> (i * 8)));
        e({0x4F, 0x8B, 0x03});                            // mov rax, [r11]
        e({0x31, 0xD2});                                  // xor edx, edx
        e({0xB9, 0x50, 0x00, 0x00, 0x00});                // mov ecx, 80
        e({0x48, 0xF7, 0xF1});                            // div rcx
        e({0x48, 0x29, 0xD0});                            // sub rax, rdx
        e({0x48, 0x83, 0xC0, 0x50});                      // add rax, 80
        e({0x48, 0x3D, 0xD0, 0x07, 0x00, 0x00});          // cmp rax, 2000
        e({0x72, 0}); int jWrap = mark() - 1;             // jb .Lok
        e({0x31, 0xC0});                                  // xor eax, eax
        int ok = mark(); b[jWrap] = static_cast<uint8_t>(ok - (jWrap + 1));
        e({0x4F, 0x89, 0x03});                            // mov [r11], rax
    }
    e({0xC3});                                            // ret
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

    const uint32_t dataFileOff = alignUp(static_cast<uint32_t>(0x2000 + rodata.size()), 0x1000u);
    const uint64_t dataBase = elfBaseAddr + dataFileOff;
    // Two hidden slots at the end: bump pointer and free-list head.
    const uint64_t heapSlotVa = dataBase + art.globals.size() * 8;
    const uint64_t freeSlotVa = heapSlotVa + 8;
    const uint64_t dataSize = (art.globals.size() + 2) * 8;

    std::vector<uint8_t> text;

    // 1. _start
    // sys_brk(0) -> mov r15, rax
    text.insert(text.end(), {0x48, 0x31, 0xFF, 0xB8, 12, 0, 0, 0, 0x0F, 0x05, 0x49, 0x89, 0xC7});
    // sys_brk(current + 16MB)
    text.insert(text.end(), {0x48, 0x89, 0xC7, 0x48, 0x81, 0xC7, 0, 0, 0, 1, 0xB8, 12, 0, 0, 0, 0x0F, 0x05});
    // xor r14, r14
    text.insert(text.end(), {0x4D, 0x31, 0xF6});
    // heap slot = brk returned above
    text.insert(text.end(), {0x48, 0xB8}); put64(text, heapSlotVa);
    text.insert(text.end(), {0x4C, 0x89, 0x38}); // mov [rax], r15
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

    // 4. ulang_alloc(size in RDI) -> RAX (free-list aware)
    uint32_t allocOff = static_cast<uint32_t>(text.size());
    auto allocBytes = emitHeapAllocHelper(heapSlotVa, freeSlotVa);
    text.insert(text.end(), allocBytes.begin(), allocBytes.end());

    while (text.size() % 16 != 0) text.push_back(0x90);

    // 4b. ulang_free(ptr in RDI)
    uint32_t freeOff = static_cast<uint32_t>(text.size());
    auto freeBytes = emitHeapFreeHelper(freeSlotVa);
    text.insert(text.end(), freeBytes.begin(), freeBytes.end());

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
        else if (fix.funcName == "ulang_free") targetHelper = freeOff;
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
    for (const auto& fix : art.dataFixups) {
        if (fix.globalIndex >= art.globals.size()) return false;
        uint64_t codeVa = textBase + userCodeStart + fix.insEnd;
        uint64_t globVa = dataBase + 8ull * fix.globalIndex;
        int32_t disp = static_cast<int32_t>(globVa - codeVa);
        std::memcpy(&userCodeCopy[fix.patchOffset], &disp, 4);
    }
    text.insert(text.end(), userCodeCopy.begin(), userCodeCopy.end());

    // PT_LOAD segments: .text (RX), .rodata (R), optional .data (RW)
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
    put16(elf, art.globals.empty() ? 2 : 3); // e_phnum
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

    if (!art.globals.empty()) {
        // Program Header 3: .data (RW)
        put32(elf, 1);      // PT_LOAD
        put32(elf, 6);      // PF_R | PF_W
        put64(elf, dataFileOff);
        put64(elf, dataBase);
        put64(elf, dataBase);
        put64(elf, dataSize);
        put64(elf, dataSize);
        put64(elf, 0x1000);
    }

    // Pad to 0x1000
    while (elf.size() < 0x1000) elf.push_back(0);
    elf.insert(elf.end(), text.begin(), text.end());

    // Pad to 0x2000
    while (elf.size() < 0x2000) elf.push_back(0);
    elf.insert(elf.end(), rodata.begin(), rodata.end());

    if (!art.globals.empty()) {
        while (elf.size() < dataFileOff) elf.push_back(0);
        for (const auto& g : art.globals) put64(elf, g.initValue);
        put64(elf, 0); // heap slot is initialized from brk in _start
        put64(elf, 0); // free list head
    }

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

bool linkMultibootELF(const codegen::x64::CompiledArtifact& art, const std::string& outputPath,
                      const std::string& arch) {
    // QEMU -kernel rejects ELFCLASS64. Emit ELF32 (EM_386) + Multiboot2 header +
    // a 32-bit trampoline that enters long mode, then runs 64-bit ULang++ code.
    if (arch == "x86_32") {
        std::cerr << "ulang++ link: --arch x86_32 is not yet implemented\n";
        return false;
    }
    if (arch != "x86_64") {
        std::cerr << "ulang++ link: unknown --arch '" << arch << "' (expected x86_64 or x86_32)\n";
        return false;
    }

    constexpr uint32_t loadBase = 0x100000;
    constexpr uint32_t fileOff = 0x1000;

    constexpr uint32_t offGdt = 0x100;
    constexpr uint32_t offGdtr = 0x118;
    constexpr uint32_t offTramp = 0x120;
    constexpr uint32_t offLong = 0x200;
    constexpr uint32_t offPml4 = 0x1000;
    constexpr uint32_t offPdpt = 0x2000;
    constexpr uint32_t offPd = 0x3000;
    constexpr uint32_t offPayload = 0x4000;
    constexpr uint32_t stackVa = 0x1F0000;

    const uint32_t gdtVa = loadBase + offGdt;
    const uint32_t gdtrVa = loadBase + offGdtr;
    const uint32_t trampVa = loadBase + offTramp;
    const uint32_t longVa = loadBase + offLong;
    const uint32_t pml4Va = loadBase + offPml4;
    const uint32_t pdptVa = loadBase + offPdpt;
    const uint32_t pdVa = loadBase + offPd;
    const uint32_t payloadVa = loadBase + offPayload;

    auto ensure = [](std::vector<uint8_t>& b, size_t n) {
        if (b.size() < n) b.resize(n, 0);
    };
    auto put8At = [&](std::vector<uint8_t>& b, size_t off, uint8_t v) {
        ensure(b, off + 1);
        b[off] = v;
    };
    auto put16At = [&](std::vector<uint8_t>& b, size_t off, uint16_t v) {
        put8At(b, off, static_cast<uint8_t>(v));
        put8At(b, off + 1, static_cast<uint8_t>(v >> 8));
    };
    auto put32At = [&](std::vector<uint8_t>& b, size_t off, uint32_t v) {
        for (int i = 0; i < 4; ++i) put8At(b, off + i, static_cast<uint8_t>(v >> (i * 8)));
    };
    auto put64At = [&](std::vector<uint8_t>& b, size_t off, uint64_t v) {
        for (int i = 0; i < 8; ++i) put8At(b, off + i, static_cast<uint8_t>(v >> (i * 8)));
    };
    auto putBytesAt = [&](std::vector<uint8_t>& b, size_t off, const std::vector<uint8_t>& bytes) {
        ensure(b, off + bytes.size());
        std::memcpy(b.data() + off, bytes.data(), bytes.size());
    };

    std::vector<uint8_t> img(offPayload, 0);

    // ---- Multiboot1 (QEMU -kernel only recognizes MB1, not MB2) ----
    // Must sit within the first 8192 bytes of the file (here: file off 0x1000).
    {
        constexpr uint32_t mb1Magic = 0x1BADB002u;
        constexpr uint32_t mb1Flags = 0;
        put32At(img, 0, mb1Magic);
        put32At(img, 4, mb1Flags);
        put32At(img, 8, static_cast<uint32_t>(-(mb1Magic + mb1Flags)));
    }

    // ---- Multiboot2 header (GRUB2 / modern bootloaders; magic 0xE85250D6) ----
    size_t addrLoadEndOff = 0;
    size_t addrBssEndOff = 0;
    {
        // 8-byte aligned, after Multiboot1
        size_t mb2Off = 16;
        std::vector<uint8_t> hdr;
        put32(hdr, 0xE85250D6);
        put32(hdr, 0); // i386
        put32(hdr, 0); // length
        put32(hdr, 0); // checksum

        // Tag 1: information request (cmdline, bootloader name, mmap)
        put16(hdr, 1);
        put16(hdr, 0);
        put32(hdr, 20);
        put32(hdr, 1);
        put32(hdr, 2);
        put32(hdr, 6);
        while (hdr.size() % 8 != 0) hdr.push_back(0);

        // Tag 2: address
        put16(hdr, 2);
        put16(hdr, 0);
        put32(hdr, 24);
        put32(hdr, loadBase + static_cast<uint32_t>(mb2Off)); // header_addr
        put32(hdr, loadBase); // load_addr
        addrLoadEndOff = mb2Off + hdr.size();
        put32(hdr, 0); // load_end_addr
        addrBssEndOff = mb2Off + hdr.size();
        put32(hdr, 0); // bss_end_addr
        while (hdr.size() % 8 != 0) hdr.push_back(0);

        // Tag 3: entry address
        put16(hdr, 3);
        put16(hdr, 0);
        put32(hdr, 12);
        put32(hdr, trampVa);
        while (hdr.size() % 8 != 0) hdr.push_back(0);

        // Tag 4: console flags (EGA text preferred)
        put16(hdr, 4);
        put16(hdr, 0);
        put32(hdr, 12);
        put32(hdr, 1);
        while (hdr.size() % 8 != 0) hdr.push_back(0);

        // Tag 0: end
        put16(hdr, 0);
        put16(hdr, 0);
        put32(hdr, 8);

        uint32_t hdrLen = static_cast<uint32_t>(hdr.size());
        std::memcpy(&hdr[8], &hdrLen, 4);
        uint32_t checksum = static_cast<uint32_t>(-(0xE85250D6u + 0u + hdrLen));
        std::memcpy(&hdr[12], &checksum, 4);
        putBytesAt(img, mb2Off, hdr);
    }

    // ---- GDT + GDTR ----
    put64At(img, offGdt + 0, 0);
    put64At(img, offGdt + 8, 0x00AF9A000000FFFFull);  // 64-bit code
    put64At(img, offGdt + 16, 0x00CF92000000FFFFull); // data
    put16At(img, offGdtr, 23);
    put32At(img, offGdtr + 2, gdtVa);

    // ---- Page tables: identity-map 0..8MiB with 2MiB pages ----
    put64At(img, offPml4, static_cast<uint64_t>(pdptVa) | 0x03);
    put64At(img, offPdpt, static_cast<uint64_t>(pdVa) | 0x03);
    for (uint32_t i = 0; i < 4; ++i) {
        put64At(img, offPd + i * 8, (static_cast<uint64_t>(i) << 21) | 0x83);
    }

    // ---- 32-bit trampoline ----
    {
        std::vector<uint8_t> t;
        t.insert(t.end(), {0x0F, 0x01, 0x15});
        put32(t, gdtrVa); // lgdt [gdtr]
        t.insert(t.end(), {0x0F, 0x20, 0xE0}); // mov eax, cr4
        t.insert(t.end(), {0x83, 0xC8, 0x20}); // or eax, PAE
        t.insert(t.end(), {0x0F, 0x22, 0xE0}); // mov cr4, eax
        t.push_back(0xB8);
        put32(t, pml4Va); // mov eax, pml4
        t.insert(t.end(), {0x0F, 0x22, 0xD8}); // mov cr3, eax
        t.push_back(0xB9);
        put32(t, 0xC0000080u); // mov ecx, EFER
        t.insert(t.end(), {0x0F, 0x32}); // rdmsr
        t.insert(t.end(), {0x0D, 0x00, 0x01, 0x00, 0x00}); // or eax, LME
        t.insert(t.end(), {0x0F, 0x30}); // wrmsr
        t.insert(t.end(), {0x0F, 0x20, 0xC0}); // mov eax, cr0
        t.insert(t.end(), {0x0D, 0x01, 0x00, 0x00, 0x80}); // or eax, PG|PE
        t.insert(t.end(), {0x0F, 0x22, 0xC0}); // mov cr0, eax
        t.push_back(0xEA);
        put32(t, longVa);
        put16(t, 0x08); // jmp 0x08:long
        putBytesAt(img, offTramp, t);
    }

    // ---- Freestanding payload: helpers + user code + rodata ----
    std::vector<uint8_t> payload;
    auto alignPad = [&](size_t a, uint8_t fill = 0x90) {
        while (payload.size() % a != 0) payload.push_back(fill);
    };

    put64(payload, 0); // heap_ptr slot
    put64(payload, 0); // VGA cursor slot
    put64(payload, 0); // free-list head slot
    const uint32_t heapPtrVa = payloadVa;
    const uint32_t cursorVa = payloadVa + 8;
    const uint32_t freePtrVa = payloadVa + 16;

    auto appendHelper = [&](const std::vector<uint8_t>& bytes) -> uint32_t {
        alignPad(16);
        uint32_t off = static_cast<uint32_t>(payload.size());
        payload.insert(payload.end(), bytes.begin(), bytes.end());
        return off;
    };

    // Helpers use rodataBase+2 convention; emit with base 0 then patch immediates (==2).
    uint32_t strPtrOff = appendHelper(emitStrPtrHelper(0));
    uint32_t strEqOff = appendHelper(emitStrEqHelper(0));
    uint32_t strToIntOff = appendHelper(emitStrToIntHelper(0));

    alignPad(16);
    uint32_t allocOff = static_cast<uint32_t>(payload.size());
    {
        auto ab = emitHeapAllocHelper(heapPtrVa, freePtrVa);
        payload.insert(payload.end(), ab.begin(), ab.end());
    }

    alignPad(16);
    uint32_t freeOff = static_cast<uint32_t>(payload.size());
    {
        auto fb = emitHeapFreeHelper(freePtrVa);
        payload.insert(payload.end(), fb.begin(), fb.end());
    }

    auto emitRet0 = [&]() -> uint32_t {
        alignPad(16);
        uint32_t off = static_cast<uint32_t>(payload.size());
        payload.insert(payload.end(), {0x48, 0x31, 0xC0, 0xC3});
        return off;
    };
    uint32_t printOff = appendHelper(emitRawPrintHelper(2, cursorVa, false));
    uint32_t printlnOff = appendHelper(emitRawPrintHelper(2, cursorVa, true));
    // String formatting still needs a full allocator-backed runtime; keep the
    // honest no-op stubs rather than emitting garbage.
    uint32_t concatOff = emitRet0();
    uint32_t intToStrOff = emitRet0();

    alignPad(16);
    uint32_t userCodeOff = static_cast<uint32_t>(payload.size());
    std::vector<uint8_t> userCode = art.code;
    for (const auto& fix : art.callFixups) {
        uint32_t helper = 0;
        if (fix.funcName == "ulang_print") helper = printOff;
        else if (fix.funcName == "ulang_println") helper = printlnOff;
        else if (fix.funcName == "ulang_alloc") helper = allocOff;
        else if (fix.funcName == "ulang_free") helper = freeOff;
        else if (fix.funcName == "ulang_concat_str") helper = concatOff;
        else if (fix.funcName == "ulang_str_eq") helper = strEqOff;
        else if (fix.funcName == "ulang_str_ptr") helper = strPtrOff;
        else if (fix.funcName == "ulang_str_to_int") helper = strToIntOff;
        else if (fix.funcName == "ulang_int_to_str") helper = intToStrOff;
        else {
            std::cerr << "ulang++ link: unresolved runtime '" << fix.funcName << "'\n";
            return false;
        }
        int32_t rel32 = static_cast<int32_t>(helper) -
                        static_cast<int32_t>(userCodeOff + static_cast<uint32_t>(fix.insEnd));
        std::memcpy(&userCode[fix.patchOffset], &rel32, 4);
    }
    const uint32_t dataOff = alignUp(userCodeOff + static_cast<uint32_t>(userCode.size()), 8u);
    const uint64_t dataVa = payloadVa + dataOff;
    for (const auto& fix : art.dataFixups) {
        if (fix.globalIndex >= art.globals.size()) return false;
        uint64_t codeVa = payloadVa + userCodeOff + fix.insEnd;
        uint64_t globVa = dataVa + 8ull * fix.globalIndex;
        int32_t disp = static_cast<int32_t>(globVa - codeVa);
        std::memcpy(&userCode[fix.patchOffset], &disp, 4);
    }
    payload.insert(payload.end(), userCode.begin(), userCode.end());

    alignPad(8, 0);
    for (const auto& g : art.globals) put64(payload, g.initValue);

    alignPad(8, 0);
    uint32_t rodataOff = static_cast<uint32_t>(payload.size());
    payload.push_back('\n');
    payload.push_back(0);
    payload.insert(payload.end(), art.readOnlyData.begin(), art.readOnlyData.end());
    const uint64_t rodataVa = payloadVa + rodataOff;

    auto patchHelperBases = [&](uint32_t helperOff, uint32_t helperEnd) {
        for (size_t i = helperOff; i + 10 <= helperEnd && i + 10 <= payload.size(); ++i) {
            if (payload[i] == 0x48 && payload[i + 1] == 0xB8) {
                uint64_t cur = 0;
                std::memcpy(&cur, &payload[i + 2], 8);
                if (cur == 2) {
                    uint64_t val = rodataVa + 2;
                    std::memcpy(&payload[i + 2], &val, 8);
                }
            }
        }
    };
    patchHelperBases(strPtrOff, strEqOff);
    patchHelperBases(strEqOff, strToIntOff);
    patchHelperBases(strToIntOff, allocOff);

    // print/println carry their literal base in `movabs r10, imm64` (49 BA).
    auto patchPrintBases = [&](uint32_t helperOff, uint32_t helperEnd) {
        for (size_t i = helperOff; i + 10 <= helperEnd && i + 10 <= payload.size(); ++i) {
            if (payload[i] == 0x49 && payload[i + 1] == 0xBA) {
                uint64_t cur = 0;
                std::memcpy(&cur, &payload[i + 2], 8);
                if (cur == 2) {
                    uint64_t val = rodataVa + 2;
                    std::memcpy(&payload[i + 2], &val, 8);
                }
            }
        }
    };
    patchPrintBases(printOff, printlnOff);
    patchPrintBases(printlnOff, concatOff);

    uint64_t heapInit = static_cast<uint64_t>((payloadVa + payload.size() + 15) & ~15ull);
    for (int i = 0; i < 8; ++i) {
        payload[i] = static_cast<uint8_t>((heapInit >> (i * 8)) & 0xFF);
    }

    uint32_t kernelOff = art.entryOffset;
    auto itKm = art.functionRVAs.find("kernel_main");
    if (itKm != art.functionRVAs.end()) {
        kernelOff = itKm->second;
    } else {
        auto itMain = art.functionRVAs.find("main");
        if (itMain != art.functionRVAs.end()) kernelOff = itMain->second;
    }
    const uint32_t kernelVa = payloadVa + userCodeOff + kernelOff;

    // ---- 64-bit long-mode stub ----
    {
        std::vector<uint8_t> s;
        s.insert(s.end(), {0x66, 0xB8, 0x10, 0x00}); // mov ax, 0x10
        s.insert(s.end(), {0x8E, 0xD8});             // mov ds, ax
        s.insert(s.end(), {0x8E, 0xC0});             // mov es, ax
        s.insert(s.end(), {0x8E, 0xD0});             // mov ss, ax
        s.insert(s.end(), {0x8E, 0xE0});             // mov fs, ax
        s.insert(s.end(), {0x8E, 0xE8});             // mov gs, ax
        s.insert(s.end(), {0x48, 0xBC});
        put64(s, stackVa); // mov rsp, stack
        s.insert(s.end(), {0x31, 0xED}); // xor ebp, ebp
        s.push_back(0xE8);
        size_t callPatch = s.size();
        put32(s, 0);
        s.push_back(0xF4);
        s.push_back(0xEB);
        s.push_back(0xFD);
        int32_t rel = static_cast<int32_t>(kernelVa) - static_cast<int32_t>(longVa + callPatch + 4);
        std::memcpy(&s[callPatch], &rel, 4);
        putBytesAt(img, offLong, s);
    }

    img.resize(offPayload, 0);
    img.insert(img.end(), payload.begin(), payload.end());

    put32At(img, addrLoadEndOff, loadBase + static_cast<uint32_t>(img.size()));
    put32At(img, addrBssEndOff, stackVa);

    // ---- ELF32 ET_EXEC ----
    std::vector<uint8_t> elf;
    elf.insert(elf.end(), {0x7F, 'E', 'L', 'F', 1, 1, 1, 0});
    elf.resize(16, 0);
    put16(elf, 2);       // ET_EXEC
    put16(elf, 3);       // EM_386
    put32(elf, 1);
    put32(elf, trampVa); // e_entry
    put32(elf, 52);
    put32(elf, 0);
    put32(elf, 0);
    put16(elf, 52);
    put16(elf, 32);
    put16(elf, 1);
    put16(elf, 40);
    put16(elf, 0);
    put16(elf, 0);

    put32(elf, 1); // PT_LOAD
    put32(elf, fileOff);
    put32(elf, loadBase);
    put32(elf, loadBase);
    put32(elf, static_cast<uint32_t>(img.size()));
    put32(elf, stackVa - loadBase);
    put32(elf, 7);
    put32(elf, 0x1000);

    while (elf.size() < fileOff) elf.push_back(0);
    elf.insert(elf.end(), img.begin(), img.end());

    std::ofstream out(outputPath, std::ios::binary);
    if (!out) return false;
    out.write(reinterpret_cast<const char*>(elf.data()), static_cast<std::streamsize>(elf.size()));
    return true;
}

} // namespace ulang::link::elf
