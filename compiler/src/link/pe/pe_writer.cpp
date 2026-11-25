// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

#include "link/pe/pe_writer.h"
#include <fstream>
#include <vector>
#include <cstring>
#include <algorithm>
#include <stdexcept>
#include <map>

namespace ulang::link::pe {

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

// Emits ulang_concat_str helper in machine bytes (calls ulang_alloc)
std::vector<uint8_t> emitConcatStrHelper(uint32_t allocOff, uint32_t myOff, uint32_t constRVA) {
    // RCX = s1 (rodata off or heap ptr), RDX = s2 (rodata off or heap ptr)
    // returns RAX = heap ptr to new string [len:8][bytes...][0]
    std::vector<uint8_t> b;
    // push rbp; mov rbp, rsp; push rbx; push rsi; push rdi; push r12; push r13; push r14; push r15; sub rsp, 40
    b.insert(b.end(), {0x55, 0x48, 0x89, 0xE5, 0x53, 0x56, 0x57, 0x41, 0x54, 0x41, 0x55, 0x41, 0x56, 0x41, 0x57, 0x48, 0x83, 0xEC, 0x28});

    // 1. Resolve s1 (RCX) -> R14 (pointer to [len:8][bytes])
    b.insert(b.end(), {0x48, 0x89, 0xC8});                 // mov rax, rcx
    b.insert(b.end(), {0x49, 0xB8, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x00}); // mov r8, 0x10000000
    b.insert(b.end(), {0x4C, 0x39, 0xC0});                 // cmp rax, r8
    b.insert(b.end(), {0x73, 0x0A});                       // jae .Ls1_heap (+10 bytes to mov r14, rax)
    uint32_t cur = 0x1000 + myOff + static_cast<uint32_t>(b.size()) + 7;
    int32_t d1 = static_cast<int32_t>(constRVA + 8) - static_cast<int32_t>(cur);
    b.insert(b.end(), {0x48, 0x8D, 0x05});                 // lea rax, [rip + d1]
    put32(b, static_cast<uint32_t>(d1));
    b.insert(b.end(), {0x48, 0x01, 0xC8});                 // add rax, rcx
    // .Ls1_heap:
    b.insert(b.end(), {0x49, 0x89, 0xC6});                 // mov r14, rax (r14 = s1 ptr)

    // 2. Resolve s2 (RDX) -> R15 (pointer to [len:8][bytes])
    b.insert(b.end(), {0x48, 0x89, 0xD0});                 // mov rax, rdx
    b.insert(b.end(), {0x4C, 0x39, 0xC0});                 // cmp rax, r8
    b.insert(b.end(), {0x73, 0x0A});                       // jae .Ls2_heap (+10 bytes to mov r15, rax)
    cur = 0x1000 + myOff + static_cast<uint32_t>(b.size()) + 7;
    int32_t d2 = static_cast<int32_t>(constRVA + 8) - static_cast<int32_t>(cur);
    b.insert(b.end(), {0x48, 0x8D, 0x05});                 // lea rax, [rip + d2]
    put32(b, static_cast<uint32_t>(d2));
    b.insert(b.end(), {0x48, 0x01, 0xD0});                 // add rax, rdx
    // .Ls2_heap:
    b.insert(b.end(), {0x49, 0x89, 0xC7});                 // mov r15, rax (r15 = s2 ptr)

    // 3. Read lengths: rbx = [r14] (len1), rsi = [r15] (len2)
    b.insert(b.end(), {0x49, 0x8B, 0x1E});                 // mov rbx, [r14]
    b.insert(b.end(), {0x49, 0x8B, 0x37});                 // mov rsi, [r15]
    b.insert(b.end(), {0x4C, 0x8D, 0x24, 0x33});           // lea r12, [rbx + rsi] (totalLen)

    // 4. Alloc size = (totalLen + 8 + 1 + 15) & ~15
    b.insert(b.end(), {0x49, 0x8D, 0x4C, 0x24, 0x18});     // lea rcx, [r12 + 24]
    b.insert(b.end(), {0x48, 0x83, 0xE1, 0xF0});           // and rcx, -16
    // call ulang_alloc (rel32)
    b.push_back(0xE8);
    size_t callOff = b.size();
    put32(b, 0);
    int32_t rel = static_cast<int32_t>(allocOff) - static_cast<int32_t>(myOff + callOff + 4);
    std::memcpy(&b[callOff], &rel, 4);

    b.insert(b.end(), {0x49, 0x89, 0xC5});                 // mov r13, rax (newStr)
    b.insert(b.end(), {0x4D, 0x89, 0x65, 0x00});           // mov [r13], r12 (store totalLen)

    // 5. Copy s1: memcpy(dst = r13 + 8, src = r14 + 8, n = rbx)
    b.insert(b.end(), {0x49, 0x8D, 0x7D, 0x08});           // lea rdi, [r13 + 8]
    b.insert(b.end(), {0x49, 0x8D, 0x76, 0x08});           // lea rsi, [r14 + 8]
    b.insert(b.end(), {0x48, 0x89, 0xD9});                 // mov rcx, rbx
    b.insert(b.end(), {0xF3, 0xA4});                       // rep movsb (rdi now points to s2 dst)

    // 6. Copy s2: memcpy(dst = rdi, src = r15 + 8, n = [r15])
    b.insert(b.end(), {0x49, 0x8D, 0x77, 0x08});           // lea rsi, [r15 + 8]
    b.insert(b.end(), {0x49, 0x8B, 0x0F});                 // mov rcx, [r15]
    b.insert(b.end(), {0xF3, 0xA4});                       // rep movsb

    // 7. Null terminate
    b.insert(b.end(), {0xC6, 0x07, 0x00});                 // mov byte ptr [rdi], 0

    // Return newStr (r13)
    b.insert(b.end(), {0x4C, 0x89, 0xE8});                 // mov rax, r13
    b.insert(b.end(), {0x48, 0x83, 0xC4, 0x28});           // add rsp, 40
    b.insert(b.end(), {0x41, 0x5F, 0x41, 0x5E, 0x41, 0x5D, 0x41, 0x5C, 0x5F, 0x5E, 0x5B, 0x5D, 0xC3});
    return b;
}

// Emits ulang_str_eq (RCX = s1, RDX = s2) -> RAX = 0/1 in machine bytes.
std::vector<uint8_t> emitStrEqHelper(uint32_t myOff, uint32_t constRVA) {
    std::vector<uint8_t> b;
    b.insert(b.end(), {0x53, 0x56, 0x57});                 // push rbx; push rsi; push rdi
    b.insert(b.end(), {0x48, 0x89, 0xC8});                 // mov rax, rcx
    b.insert(b.end(), {0x49, 0xB8}); put64(b, 0x10000000); // mov r8, 0x10000000
    b.insert(b.end(), {0x4C, 0x39, 0xC0});                 // cmp rax, r8
    b.push_back(0x73);                                     // jae .Ls1_heap
    size_t j1 = b.size(); b.push_back(0);
    uint32_t cur = 0x1000 + myOff + static_cast<uint32_t>(b.size()) + 7;
    int32_t d1 = static_cast<int32_t>(constRVA + 8) - static_cast<int32_t>(cur);
    b.insert(b.end(), {0x48, 0x8D, 0x05});                 // lea rax, [rip + d1]
    put32(b, static_cast<uint32_t>(d1));
    b.insert(b.end(), {0x48, 0x01, 0xC8});                 // add rax, rcx
    size_t s1Heap = b.size();
    b[j1] = static_cast<uint8_t>(s1Heap - (j1 + 1));
    b.insert(b.end(), {0x48, 0x89, 0xC6});                 // mov rsi, rax

    b.insert(b.end(), {0x48, 0x89, 0xD0});                 // mov rax, rdx
    b.insert(b.end(), {0x4C, 0x39, 0xC0});                 // cmp rax, r8
    b.push_back(0x73);                                     // jae .Ls2_heap
    size_t j2 = b.size(); b.push_back(0);
    cur = 0x1000 + myOff + static_cast<uint32_t>(b.size()) + 7;
    int32_t d2 = static_cast<int32_t>(constRVA + 8) - static_cast<int32_t>(cur);
    b.insert(b.end(), {0x48, 0x8D, 0x05});                 // lea rax, [rip + d2]
    put32(b, static_cast<uint32_t>(d2));
    b.insert(b.end(), {0x48, 0x01, 0xD0});                 // add rax, rdx
    size_t s2Heap = b.size();
    b[j2] = static_cast<uint8_t>(s2Heap - (j2 + 1));
    b.insert(b.end(), {0x48, 0x89, 0xC7});                 // mov rdi, rax

    b.insert(b.end(), {0x48, 0x8B, 0x0E});                 // mov rcx, [rsi]
    b.insert(b.end(), {0x48, 0x3B, 0x0F});                 // cmp rcx, [rdi]
    b.push_back(0x75);                                     // jne .Lneq
    size_t jNe = b.size(); b.push_back(0);
    b.insert(b.end(), {0x48, 0x83, 0xC6, 0x08});           // add rsi, 8
    b.insert(b.end(), {0x48, 0x83, 0xC7, 0x08});           // add rdi, 8
    b.insert(b.end(), {0x31, 0xC0});                       // xor eax, eax
    b.insert(b.end(), {0xF3, 0xA6});                       // repe cmpsb
    b.insert(b.end(), {0x0F, 0x94, 0xC0});                 // sete al
    b.insert(b.end(), {0x0F, 0xB6, 0xC0});                 // movzx eax, al
    b.push_back(0xEB);                                     // jmp .Ldone
    size_t jDone = b.size(); b.push_back(0);
    size_t neq = b.size();
    b[jNe] = static_cast<uint8_t>(neq - (jNe + 1));
    b.insert(b.end(), {0x31, 0xC0});                       // xor eax, eax
    size_t done = b.size();
    b[jDone] = static_cast<uint8_t>(done - (jDone + 1));
    b.insert(b.end(), {0x5F, 0x5E, 0x5B, 0xC3});           // pop rdi; pop rsi; pop rbx; ret
    return b;
}

// Emits ulang_str_ptr (RCX = literal offset or heap pointer) -> RAX = real pointer.
std::vector<uint8_t> emitStrPtrHelper(uint32_t myOff, uint32_t constRVA) {
    std::vector<uint8_t> b;
    b.insert(b.end(), {0x48, 0x89, 0xC8});                 // mov rax, rcx
    b.insert(b.end(), {0x49, 0xB8}); put64(b, 0x10000000); // mov r8, 0x10000000
    b.insert(b.end(), {0x4C, 0x39, 0xC0});                 // cmp rax, r8
    b.push_back(0x73);                                     // jae done
    size_t jDone = b.size(); b.push_back(0);
    uint32_t cur = 0x1000 + myOff + static_cast<uint32_t>(b.size()) + 7;
    int32_t disp = static_cast<int32_t>(constRVA + 8) - static_cast<int32_t>(cur);
    b.insert(b.end(), {0x48, 0x8D, 0x05});                 // lea rax, [rip + disp]
    put32(b, static_cast<uint32_t>(disp));
    b.insert(b.end(), {0x48, 0x01, 0xC8});                 // add rax, rcx
    size_t done = b.size();
    b[jDone] = static_cast<uint8_t>(done - (jDone + 1));
    b.push_back(0xC3);                                     // ret
    return b;
}

// Emits ulang_str_to_int (RCX = string) -> RAX = parsed integer.
std::vector<uint8_t> emitStrToIntHelper(uint32_t myOff, uint32_t constRVA) {
    std::vector<uint8_t> b;
    b.push_back(0x56);                                     // push rsi
    b.insert(b.end(), {0x48, 0x89, 0xC8});                 // mov rax, rcx
    b.insert(b.end(), {0x49, 0xBA}); put64(b, 0x10000000); // mov r10, 0x10000000
    b.insert(b.end(), {0x4C, 0x39, 0xD0});                 // cmp rax, r10
    b.push_back(0x73);                                     // jae .Lheap
    size_t jHeap = b.size(); b.push_back(0);
    uint32_t cur = 0x1000 + myOff + static_cast<uint32_t>(b.size()) + 7;
    int32_t disp = static_cast<int32_t>(constRVA + 8) - static_cast<int32_t>(cur);
    b.insert(b.end(), {0x48, 0x8D, 0x05});                 // lea rax, [rip + disp]
    put32(b, static_cast<uint32_t>(disp));
    b.insert(b.end(), {0x48, 0x01, 0xC8});                 // add rax, rcx
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
    int32_t loopBack = static_cast<int32_t>(digits) - static_cast<int32_t>(jLoop + 1);
    b[jLoop] = static_cast<uint8_t>(loopBack);

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
    b.insert(b.end(), {0x5E, 0xC3});                       // pop rsi; ret
    return b;
}

} // namespace

bool linkPE(const codegen::x64::CompiledArtifact& art, const std::string& outputPath, bool noConsole) {
    constexpr uint32_t peFileAlign = 0x200;
    constexpr uint32_t peSectAlign = 0x1000;
    constexpr uint64_t imageBase = 0x140000000ULL;

    std::vector<std::string> imports = {
        "ExitProcess", "GetStdHandle", "WriteFile"
    };

    std::vector<uint8_t> rdata;
    // Newline bytes "\r\n\0\0\0\0\0\0"
    for (char c : "\r\n") rdata.push_back(static_cast<uint8_t>(c));
    while (rdata.size() < 8) rdata.push_back(0);
    rdata.insert(rdata.end(), art.readOnlyData.begin(), art.readOnlyData.end());

    // Compute IAT layout
    uint32_t nImports = static_cast<uint32_t>(imports.size());
    uint32_t iatSize = (nImports + 1) * 8;
    uint32_t idtOffset = iatSize;
    uint32_t idtSize = 40; // 20 bytes KERNEL32, 20 bytes null terminator
    uint32_t iltOffset = alignUp(idtOffset + idtSize, 8);
    uint32_t iltSize = (nImports + 1) * 8;
    uint32_t dllNameOffset = iltOffset + iltSize;
    const std::string dllName = "KERNEL32.dll";
    uint32_t dllNameSize = static_cast<uint32_t>(dllName.size()) + 1;
    uint32_t hintOffset = dllNameOffset + dllNameSize;
    if (hintOffset % 2 != 0) hintOffset++;

    std::vector<uint32_t> hintOffsets;
    uint32_t curHint = hintOffset;
    for (const auto& fn : imports) {
        hintOffsets.push_back(curHint);
        curHint += 2 + static_cast<uint32_t>(fn.size()) + 1;
        if (curHint % 2 != 0) curHint++;
    }
    uint32_t hdrArea = alignUp(curHint, 16);

    uint32_t textRVA = 0x1000;
    // .text grows with the user program and the embedded runtime helpers, so
    // reserve a virtual window before .rdata. RIP-relative references inside
    // the runtime are patched while .text is emitted, before its final size is
    // known; a fixed window keeps the layout deterministic and avoids
    // overlapping .rdata for larger programs.
    constexpr uint32_t textVirtReserve = 0x100000;
    uint32_t rdataRVA = textRVA + textVirtReserve;
    uint32_t constRVA = rdataRVA + hdrArea;
    uint32_t bssRVA = rdataRVA + alignUp(hdrArea + static_cast<uint32_t>(rdata.size()), peSectAlign);
    uint32_t hStdoutRVA = bssRVA;
    uint32_t allocPtrRVA = bssRVA + 8;

    std::map<std::string, uint32_t> iatMap;
    for (size_t i = 0; i < imports.size(); ++i) {
        iatMap[imports[i]] = rdataRVA + static_cast<uint32_t>(i * 8);
    }

    std::vector<uint8_t> text;

    // 1. _start
    // sub rsp, 56
    text.insert(text.end(), {0x48, 0x83, 0xEC, 0x38});
    // mov ecx, -11 (STD_OUTPUT_HANDLE)
    text.insert(text.end(), {0xB9, 0xF5, 0xFF, 0xFF, 0xFF});
    // call [rip + disp(GetStdHandle)]
    uint32_t curPos = textRVA + static_cast<uint32_t>(text.size()) + 6;
    int32_t dispGetStd = static_cast<int32_t>(iatMap["GetStdHandle"]) - static_cast<int32_t>(curPos);
    text.insert(text.end(), {0xFF, 0x15});
    put32(text, static_cast<uint32_t>(dispGetStd));

    // mov [rip + disp(hStdoutRVA)], rax
    curPos = textRVA + static_cast<uint32_t>(text.size()) + 7;
    int32_t dispH = static_cast<int32_t>(hStdoutRVA) - static_cast<int32_t>(curPos);
    text.insert(text.end(), {0x48, 0x89, 0x05});
    put32(text, static_cast<uint32_t>(dispH));

    // allocPtr = bssRVA + 0x10000
    curPos = textRVA + static_cast<uint32_t>(text.size()) + 7;
    int32_t dispAllocInit = static_cast<int32_t>(bssRVA + 0x10000) - static_cast<int32_t>(curPos);
    text.insert(text.end(), {0x48, 0x8D, 0x05});
    put32(text, static_cast<uint32_t>(dispAllocInit));
    curPos = textRVA + static_cast<uint32_t>(text.size()) + 7;
    int32_t dispAllocPtr = static_cast<int32_t>(allocPtrRVA) - static_cast<int32_t>(curPos);
    text.insert(text.end(), {0x48, 0x89, 0x05});
    put32(text, static_cast<uint32_t>(dispAllocPtr));

    // xor r14, r14
    text.insert(text.end(), {0x4D, 0x31, 0xF6});

    // call main (rel32)
    text.push_back(0xE8);
    size_t callMainPatch = text.size();
    put32(text, 0);

    // mov ecx, eax; call [rip + disp(ExitProcess)]
    text.insert(text.end(), {0x89, 0xC1});
    curPos = textRVA + static_cast<uint32_t>(text.size()) + 6;
    int32_t dispExit = static_cast<int32_t>(iatMap["ExitProcess"]) - static_cast<int32_t>(curPos);
    text.insert(text.end(), {0xFF, 0x15});
    put32(text, static_cast<uint32_t>(dispExit));
    text.push_back(0xF4); // hlt

    while (text.size() % 16 != 0) text.push_back(0x90);

    // 2. ulang_print(str in RCX)
    uint32_t printOff = static_cast<uint32_t>(text.size());
    text.insert(text.end(), {0x48, 0x83, 0xEC, 0x38}); // sub rsp, 56
    text.insert(text.end(), {0x48, 0x89, 0xC8});       // mov rax, rcx
    text.insert(text.end(), {0x49, 0xC7, 0xC0, 0x00, 0x00, 0x00, 0x10}); // mov r8, 0x10000000
    text.insert(text.end(), {0x4C, 0x39, 0xC0});       // cmp rax, r8
    text.insert(text.end(), {0x73, 0x0A});             // jae .Lheap (+10 bytes)
    curPos = textRVA + static_cast<uint32_t>(text.size()) + 7;
    int32_t dispStrBase = static_cast<int32_t>(constRVA + 8) - static_cast<int32_t>(curPos);
    text.insert(text.end(), {0x48, 0x8D, 0x05});
    put32(text, static_cast<uint32_t>(dispStrBase));
    text.insert(text.end(), {0x48, 0x01, 0xC8});       // add rax, rcx

    // .Lheap:
    text.insert(text.end(), {0x4C, 0x8B, 0x00});       // mov r8, [rax] (len)
    text.insert(text.end(), {0x48, 0x8D, 0x50, 0x08}); // lea rdx, [rax + 8] (data)
    curPos = textRVA + static_cast<uint32_t>(text.size()) + 7;
    int32_t dispOut = static_cast<int32_t>(hStdoutRVA) - static_cast<int32_t>(curPos);
    text.insert(text.end(), {0x48, 0x8B, 0x0D});       // mov rcx, [hStdout]
    put32(text, static_cast<uint32_t>(dispOut));
    text.insert(text.end(), {0x4C, 0x8D, 0x4C, 0x24, 0x28}); // lea r9, [rsp + 40]
    text.insert(text.end(), {0x48, 0xC7, 0x44, 0x24, 0x20, 0x00, 0x00, 0x00, 0x00}); // [rsp+32]=0
    curPos = textRVA + static_cast<uint32_t>(text.size()) + 6;
    int32_t dispW = static_cast<int32_t>(iatMap["WriteFile"]) - static_cast<int32_t>(curPos);
    text.insert(text.end(), {0xFF, 0x15});             // call WriteFile
    put32(text, static_cast<uint32_t>(dispW));
    text.insert(text.end(), {0x48, 0x83, 0xC4, 0x38, 0xC3}); // add rsp, 56; ret

    while (text.size() % 16 != 0) text.push_back(0x90);

    // 3. ulang_println(str in RCX)
    uint32_t printlnOff = static_cast<uint32_t>(text.size());
    text.insert(text.end(), {0x48, 0x83, 0xEC, 0x38}); // sub rsp, 56
    text.push_back(0xE8);
    int32_t relPrint = static_cast<int32_t>(printOff) - static_cast<int32_t>(text.size() + 4);
    put32(text, static_cast<uint32_t>(relPrint));
    curPos = textRVA + static_cast<uint32_t>(text.size()) + 7;
    int32_t dispOut2 = static_cast<int32_t>(hStdoutRVA) - static_cast<int32_t>(curPos);
    text.insert(text.end(), {0x48, 0x8B, 0x0D});
    put32(text, static_cast<uint32_t>(dispOut2));
    curPos = textRVA + static_cast<uint32_t>(text.size()) + 7;
    int32_t dispNL = static_cast<int32_t>(constRVA) - static_cast<int32_t>(curPos);
    text.insert(text.end(), {0x48, 0x8D, 0x15});
    put32(text, static_cast<uint32_t>(dispNL));
    text.insert(text.end(), {0x41, 0xB8}); put32(text, 2); // mov r8d, 2
    text.insert(text.end(), {0x4C, 0x8D, 0x4C, 0x24, 0x28});
    text.insert(text.end(), {0x48, 0xC7, 0x44, 0x24, 0x20, 0x00, 0x00, 0x00, 0x00});
    curPos = textRVA + static_cast<uint32_t>(text.size()) + 6;
    int32_t dispW2 = static_cast<int32_t>(iatMap["WriteFile"]) - static_cast<int32_t>(curPos);
    text.insert(text.end(), {0xFF, 0x15});
    put32(text, static_cast<uint32_t>(dispW2));
    text.insert(text.end(), {0x48, 0x83, 0xC4, 0x38, 0xC3});

    while (text.size() % 16 != 0) text.push_back(0x90);

    // 4. ulang_alloc(size in RCX) -> RAX
    uint32_t allocOff = static_cast<uint32_t>(text.size());
    text.insert(text.end(), {0x48, 0x83, 0xC1, 0x0F}); // add rcx, 15
    text.insert(text.end(), {0x48, 0x83, 0xE1, 0xF0}); // and rcx, -16
    curPos = textRVA + static_cast<uint32_t>(text.size()) + 7;
    int32_t dispAllocRead = static_cast<int32_t>(allocPtrRVA) - static_cast<int32_t>(curPos);
    text.insert(text.end(), {0x48, 0x8B, 0x05});
    put32(text, static_cast<uint32_t>(dispAllocRead));
    text.insert(text.end(), {0x48, 0x8D, 0x14, 0x08}); // lea rdx, [rax + rcx]
    curPos = textRVA + static_cast<uint32_t>(text.size()) + 7;
    int32_t dispAllocWrite = static_cast<int32_t>(allocPtrRVA) - static_cast<int32_t>(curPos);
    text.insert(text.end(), {0x48, 0x89, 0x15});
    put32(text, static_cast<uint32_t>(dispAllocWrite));
    text.push_back(0xC3);

    while (text.size() % 16 != 0) text.push_back(0x90);

    // 5. ulang_concat_str(s1 in RCX, s2 in RDX) -> RAX
    uint32_t concatOff = static_cast<uint32_t>(text.size());
    auto concatBytes = emitConcatStrHelper(allocOff, concatOff, constRVA);
    text.insert(text.end(), concatBytes.begin(), concatBytes.end());

    while (text.size() % 16 != 0) text.push_back(0x90);

    // 6. ulang_str_eq(s1 in RCX, s2 in RDX) -> RAX
    uint32_t strEqOff = static_cast<uint32_t>(text.size());
    auto strEqBytes = emitStrEqHelper(strEqOff, constRVA);
    text.insert(text.end(), strEqBytes.begin(), strEqBytes.end());

    // 7. ulang_str_ptr(s in RCX) -> RAX
    uint32_t strPtrOff = static_cast<uint32_t>(text.size());
    auto strPtrBytes = emitStrPtrHelper(strPtrOff, constRVA);
    text.insert(text.end(), strPtrBytes.begin(), strPtrBytes.end());

    // 8. ulang_str_to_int(s in RCX) -> RAX
    uint32_t strToIntOff = static_cast<uint32_t>(text.size());
    auto strToIntBytes = emitStrToIntHelper(strToIntOff, constRVA);
    text.insert(text.end(), strToIntBytes.begin(), strToIntBytes.end());

    while (text.size() % 16 != 0) text.push_back(0x90);

    // 8. ulang_int_to_str(n in RCX) -> RAX
    uint32_t intToStrOff = static_cast<uint32_t>(text.size());
    std::vector<uint8_t> intToStrBytes = {
        0x55, 0x48, 0x89, 0xE5, 0x53, 0x56, 0x57, 0x41, 0x54, 0x48, 0x83, 0xEC, 0x50, 0x48, 0x8D, 0x74,
        0x24, 0x40, 0xC6, 0x06, 0x00, 0x48, 0x89, 0xC8, 0x45, 0x31, 0xC0, 0x48, 0x83, 0xF8, 0x00, 0x7D,
        0x09, 0x48, 0xF7, 0xD8, 0x41, 0xB8, 0x01, 0x00, 0x00, 0x00, 0x48, 0x83, 0xF8, 0x00, 0x75, 0x08,
        0x48, 0xFF, 0xCE, 0xC6, 0x06, 0x30, 0xEB, 0x1A, 0x48, 0x31, 0xD2, 0x49, 0xC7, 0xC1, 0x0A, 0x00,
        0x00, 0x00, 0x49, 0xF7, 0xF1, 0x80, 0xC2, 0x30, 0x48, 0xFF, 0xCE, 0x88, 0x16, 0x48, 0x85, 0xC0,
        0x75, 0xE6, 0x45, 0x85, 0xC0, 0x74, 0x06, 0x48, 0xFF, 0xCE, 0xC6, 0x06, 0x2D, 0x48, 0x8D, 0x44,
        0x24, 0x40, 0x48, 0x29, 0xF0, 0x48, 0x89, 0xC3, 0x48, 0x89, 0xF7, 0x48, 0x8D, 0x4B, 0x09, 0xE8,
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

    // Patch _start call to main
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
        else {
            std::printf("linkPE: unknown fixup %s\n", fix.funcName.c_str());
            return false;
        }

        uint32_t absCallSite = userCodeStart + static_cast<uint32_t>(fix.insEnd);
        int32_t rel32 = static_cast<int32_t>(targetHelper) - static_cast<int32_t>(absCallSite);
        std::memcpy(&userCodeCopy[fix.patchOffset], &rel32, 4);
    }
    text.insert(text.end(), userCodeCopy.begin(), userCodeCopy.end());

    // Finalize PE binary structure
    uint32_t textRawLen = alignUp(static_cast<uint32_t>(text.size()), peFileAlign);
    // Keep .text virtually contiguous with .rdata. The reserved window is
    // zero-filled by the loader past the raw bytes and avoids leaving a hole
    // between the two sections.
    uint32_t textVirtLen = textVirtReserve;
    if (text.size() > textVirtReserve) {
        std::printf("linkPE: .text exceeds reserved virtual window\n");
        return false;
    }

    // Build rdata contents
    std::vector<uint8_t> rdataSection(hdrArea + rdata.size(), 0);
    // IAT
    for (size_t i = 0; i < imports.size(); ++i) {
        uint64_t v = rdataRVA + hintOffsets[i];
        std::memcpy(&rdataSection[i * 8], &v, 8);
    }
    // IDT (at idtOffset)
    uint32_t idtILT = rdataRVA + iltOffset;
    uint32_t idtName = rdataRVA + dllNameOffset;
    uint32_t idtIAT = rdataRVA;
    std::memcpy(&rdataSection[idtOffset + 0], &idtILT, 4);
    std::memcpy(&rdataSection[idtOffset + 12], &idtName, 4);
    std::memcpy(&rdataSection[idtOffset + 16], &idtIAT, 4);
    // ILT
    for (size_t i = 0; i < imports.size(); ++i) {
        uint64_t v = rdataRVA + hintOffsets[i];
        std::memcpy(&rdataSection[iltOffset + i * 8], &v, 8);
    }
    // DLL Name
    std::memcpy(&rdataSection[dllNameOffset], dllName.c_str(), dllName.size() + 1);
    // Hints
    for (size_t i = 0; i < imports.size(); ++i) {
        std::memcpy(&rdataSection[hintOffsets[i] + 2], imports[i].c_str(), imports[i].size() + 1);
    }
    // Constants
    std::memcpy(&rdataSection[hdrArea], rdata.data(), rdata.size());

    uint32_t rdataRawLen = alignUp(static_cast<uint32_t>(rdataSection.size()), peFileAlign);
    uint32_t rdataVirtLen = alignUp(static_cast<uint32_t>(rdataSection.size()), peSectAlign);
    uint32_t bssVirtLen = 0x200000; // 2MB

    // Full file assembly
    std::vector<uint8_t> pe;

    // DOS Header (64 bytes)
    pe.insert(pe.end(), {'M', 'Z'});
    pe.resize(0x3C, 0);
    put32(pe, 0x40); // e_lfanew = 0x40

    // PE Signature (4 bytes)
    pe.insert(pe.end(), {'P', 'E', 0, 0});

    // COFF Header (20 bytes)
    put16(pe, 0x8664); // AMD64
    put16(pe, 3);      // 3 sections (.text, .rdata, .bss)
    put32(pe, 0);      // TimeDateStamp
    put32(pe, 0);      // PointerToSymbolTable
    put32(pe, 0);      // NumberOfSymbols
    put16(pe, 0xF0);   // SizeOfOptionalHeader
    put16(pe, 0x0022); // Characteristics: EXECUTABLE_IMAGE | LARGE_ADDRESS_AWARE

    // Optional Header (PE32+, 240 bytes)
    put16(pe, 0x020B); // PE32+
    put16(pe, 0x0100); // LinkerVersion 1.0
    put32(pe, textRawLen);
    put32(pe, rdataRawLen);
    put32(pe, 0);      // SizeOfUninitializedData
    put32(pe, 0x1000); // AddressOfEntryPoint (_start)
    put32(pe, 0x1000); // BaseOfCode
    put64(pe, imageBase);
    put32(pe, peSectAlign);
    put32(pe, peFileAlign);
    put16(pe, 6); put16(pe, 0); // OS Version 6.0
    put16(pe, 0); put16(pe, 0); // Image Version
    put16(pe, 6); put16(pe, 0); // Subsystem Version 6.0
    put32(pe, 0);               // Win32VersionValue
    uint32_t sizeOfImage = bssRVA + bssVirtLen;
    put32(pe, sizeOfImage);
    put32(pe, 0x400);           // SizeOfHeaders
    put32(pe, 0);               // CheckSum
    put16(pe, noConsole ? 2 : 3); // Subsystem: 2=GUI, 3=CUI
    put16(pe, 0x8160);          // DllCharacteristics: HIGH_ENTROPY_VA | DYNAMIC_BASE | NX_COMPAT | TERMINAL_SERVER_AWARE
    put64(pe, 0x100000);        // SizeOfStackReserve (1MB)
    put64(pe, 0x10000);         // SizeOfStackCommit (64KB)
    put64(pe, 0x100000);        // SizeOfHeapReserve
    put64(pe, 0x10000);         // SizeOfHeapCommit
    put32(pe, 0);               // LoaderFlags
    put32(pe, 16);              // NumberOfRvaAndSizes

    // 16 Data Directories
    for (int i = 0; i < 16; ++i) {
        if (i == 1) { // Import Directory
            put32(pe, rdataRVA + idtOffset);
            put32(pe, idtSize);
        } else if (i == 12) { // IAT Directory
            put32(pe, rdataRVA);
            put32(pe, iatSize);
        } else {
            put32(pe, 0);
            put32(pe, 0);
        }
    }

    // Section Headers (3 * 40 bytes)
    // 1. .text
    for (char c : ".text") pe.push_back(static_cast<uint8_t>(c));
    while (pe.size() % 40 != (0x40 + 4 + 20 + 0xF0 + 8) % 40) pe.push_back(0); // name 8 bytes
    put32(pe, textVirtLen);
    put32(pe, 0x1000);
    put32(pe, textRawLen);
    put32(pe, 0x400); // Raw offset
    put32(pe, 0); put32(pe, 0); put16(pe, 0); put16(pe, 0);
    put32(pe, 0x60000020); // CNT_CODE | MEM_EXECUTE | MEM_READ

    // 2. .rdata
    for (char c : ".rdata") pe.push_back(static_cast<uint8_t>(c));
    while (pe.size() % 40 != (0x40 + 4 + 20 + 0xF0 + 40 + 8) % 40) pe.push_back(0);
    put32(pe, rdataVirtLen);
    put32(pe, rdataRVA);
    put32(pe, rdataRawLen);
    put32(pe, 0x400 + textRawLen);
    put32(pe, 0); put32(pe, 0); put16(pe, 0); put16(pe, 0);
    put32(pe, 0x40000040); // CNT_INITIALIZED_DATA | MEM_READ

    // 3. .bss
    for (char c : ".bss") pe.push_back(static_cast<uint8_t>(c));
    while (pe.size() % 40 != (0x40 + 4 + 20 + 0xF0 + 80 + 8) % 40) pe.push_back(0);
    put32(pe, bssVirtLen);
    put32(pe, bssRVA);
    put32(pe, 0);
    put32(pe, 0);
    put32(pe, 0); put32(pe, 0); put16(pe, 0); put16(pe, 0);
    put32(pe, 0xC0000080); // CNT_UNINITIALIZED_DATA | MEM_READ | MEM_WRITE

    // Pad headers to 0x400
    while (pe.size() < 0x400) pe.push_back(0);

    // .text section
    pe.insert(pe.end(), text.begin(), text.end());
    while (pe.size() < 0x400 + textRawLen) pe.push_back(0);

    // .rdata section
    pe.insert(pe.end(), rdataSection.begin(), rdataSection.end());
    while (pe.size() < 0x400 + textRawLen + rdataRawLen) pe.push_back(0);

    // Write file
    std::ofstream out(outputPath, std::ios::binary);
    if (!out) {
        std::printf("linkPE: cannot open output %s\n", outputPath.c_str());
        return false;
    }
    out.write(reinterpret_cast<const char*>(pe.data()), pe.size());
    return true;
}

} // namespace ulang::link::pe
