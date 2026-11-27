// Copyright 2026 ULang++ Programming Language
// SPDX-License-Identifier: Apache-2.0

#pragma once

#include <cstdint>
#include <vector>

namespace ulang::link {

inline void heapPut64(std::vector<uint8_t>& b, uint64_t v) {
    for (int i = 0; i < 8; ++i) b.push_back(static_cast<uint8_t>(v >> (i * 8)));
}

// First-fit free-list allocator.  Each chunk has a 16-byte header:
//   [size:u64][next:u64][ payload ... ]
// `heapVa` is a slot holding the bump pointer; `freeVa` holds the list head.
// RDI = requested size, RAX = aligned payload pointer.
inline std::vector<uint8_t> emitHeapAllocHelper(uint64_t heapVa, uint64_t freeVa) {
    std::vector<uint8_t> b;
    auto e = [&](std::initializer_list<uint8_t> xs) { b.insert(b.end(), xs); };
    auto mark = [&]() { return static_cast<int>(b.size()); };
    auto patch8 = [&](int at, int target) {
        b[at] = static_cast<uint8_t>(static_cast<int8_t>(target - (at + 1)));
    };

    e({0x48, 0x83, 0xC7, 0x10}); // add rdi, 16 (chunk header)
    e({0x48, 0x83, 0xE7, 0xF0}); // and rdi, -16

    // r8 = free list head
    e({0x48, 0xB8}); heapPut64(b, freeVa); // movabs rax, freeVa
    e({0x4C, 0x8B, 0x00});                 // mov r8, [rax]
    int scan = mark();
    e({0x4D, 0x85, 0xC0});                 // test r8, r8
    e({0x74, 0}); int jBump = mark() - 1;   // jz .Lbump
    e({0x4D, 0x8B, 0x10});                 // mov r10, [r8] (chunk size)
    e({0x49, 0x39, 0xFA});                 // cmp r10, rdi
    e({0x73, 0}); int jReuse = mark() - 1;  // jae .Lreuse
    e({0x4D, 0x8B, 0x40, 0x08});           // mov r8, [r8+8] (next)
    e({0xEB, 0}); int jScan = mark() - 1;   // jmp .Lscan
    patch8(jScan, scan);

    // Reuse: unlink r8 and return r8+16
    int reuse = mark(); b[jReuse] = static_cast<uint8_t>(reuse - (jReuse + 1));
    e({0x4D, 0x8B, 0x48, 0x08});           // mov r9, [r8+8]
    e({0x48, 0xB8}); heapPut64(b, freeVa); // movabs rax, freeVa
    e({0x4C, 0x89, 0x08});                 // mov [rax], r9
    e({0x49, 0x8D, 0x40, 0x10});           // lea rax, [r8+16]
    e({0xC3});                             // ret

    // Bump path: carve a new chunk from the heap
    int bump = mark(); b[jBump] = static_cast<uint8_t>(bump - (jBump + 1));
    e({0x48, 0xB8}); heapPut64(b, heapVa); // movabs rax, heapVa
    e({0x4C, 0x8B, 0x10});                 // mov r10, [rax] (old heap)
    e({0x4D, 0x89, 0xD3});                 // mov r11, r10
    e({0x49, 0x01, 0xFB});                 // add r11, rdi
    e({0x4C, 0x89, 0x18});                 // mov [rax], r11 (new heap)
    e({0x49, 0x89, 0x3A});                 // mov [r10], rdi (chunk size)
    e({0x49, 0x8D, 0x42, 0x10});           // lea rax, [r10+16]
    e({0xC3});                             // ret
    return b;
}

// RDI = payload pointer (0 is ignored).  Pushes its chunk onto the free list.
inline std::vector<uint8_t> emitHeapFreeHelper(uint64_t freeVa) {
    std::vector<uint8_t> b;
    auto e = [&](std::initializer_list<uint8_t> xs) { b.insert(b.end(), xs); };
    auto mark = [&]() { return static_cast<int>(b.size()); };

    e({0x48, 0x85, 0xFF});                 // test rdi, rdi
    e({0x74, 0}); int jDone = mark() - 1;   // jz .Ldone
    e({0x4C, 0x8D, 0x57, 0xF0});           // lea r10, [rdi-16]
    e({0x48, 0xB8}); heapPut64(b, freeVa); // movabs rax, freeVa
    e({0x4C, 0x8B, 0x18});                 // mov r11, [rax]
    e({0x4D, 0x89, 0x5A, 0x08});           // mov [r10+8], r11
    e({0x4C, 0x89, 0x10});                 // mov [rax], r10
    int done = mark(); b[jDone] = static_cast<uint8_t>(done - (jDone + 1));
    e({0xC3});                             // ret
    return b;
}

// Same allocator with R8 pointing at the two slot words [heap, free_head].
// Used by the PE writer, where image-relative addressing must be RIP-based.
inline std::vector<uint8_t> emitHeapAllocHelperSlotReg() {
    std::vector<uint8_t> b;
    auto e = [&](std::initializer_list<uint8_t> xs) { b.insert(b.end(), xs); };
    auto mark = [&]() { return static_cast<int>(b.size()); };

    e({0x48, 0x83, 0xC7, 0x10});       // add rdi, 16
    e({0x48, 0x83, 0xE7, 0xF0});       // and rdi, -16
    int scan = mark();
    e({0x4D, 0x8B, 0x48, 0x08});       // mov r9, [r8+8] (free head)
    e({0x4D, 0x85, 0xC9});             // test r9, r9
    e({0x74, 0}); int jBump = mark() - 1;
    e({0x4D, 0x8B, 0x11});             // mov r10, [r9] (chunk size)
    e({0x49, 0x39, 0xFA});             // cmp r10, rdi
    e({0x73, 0}); int jReuse = mark() - 1;
    e({0x4D, 0x8B, 0x49, 0x08});       // mov r9, [r9+8] (next)
    e({0xEB, 0}); int jScan = mark() - 1;
    b[jScan] = static_cast<uint8_t>(static_cast<int8_t>(scan - (jScan + 1)));

    int reuse = mark(); b[jReuse] = static_cast<uint8_t>(reuse - (jReuse + 1));
    e({0x4D, 0x8B, 0x51, 0x08});       // mov r10, [r9+8]
    e({0x4D, 0x89, 0x50, 0x08});       // mov [r8+8], r10
    e({0x49, 0x8D, 0x41, 0x10});       // lea rax, [r9+16]
    e({0xC3});

    int bump = mark(); b[jBump] = static_cast<uint8_t>(bump - (jBump + 1));
    e({0x4D, 0x8B, 0x10});             // mov r10, [r8] (old heap)
    e({0x4D, 0x89, 0xD3});             // mov r11, r10
    e({0x49, 0x01, 0xFB});             // add r11, rdi
    e({0x4D, 0x89, 0x18});             // mov [r8], r11
    e({0x49, 0x89, 0x3A});             // mov [r10], rdi
    e({0x49, 0x8D, 0x42, 0x10});       // lea rax, [r10+16]
    e({0xC3});
    return b;
}

inline std::vector<uint8_t> emitHeapFreeHelperSlotReg() {
    std::vector<uint8_t> b;
    auto e = [&](std::initializer_list<uint8_t> xs) { b.insert(b.end(), xs); };
    auto mark = [&]() { return static_cast<int>(b.size()); };

    e({0x48, 0x85, 0xFF});             // test rdi, rdi
    e({0x74, 0}); int jDone = mark() - 1;
    e({0x4C, 0x8D, 0x57, 0xF0});       // lea r10, [rdi-16]
    e({0x4D, 0x8B, 0x48, 0x08});       // mov r9, [r8+8]
    e({0x4D, 0x89, 0x4A, 0x08});       // mov [r10+8], r9
    e({0x4D, 0x89, 0x50, 0x08});       // mov [r8+8], r10
    int done = mark(); b[jDone] = static_cast<uint8_t>(done - (jDone + 1));
    e({0xC3});
    return b;
}

} // namespace ulang::link
