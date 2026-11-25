// Pawl bootstrap compiler - x86-64 code generation (no assembler, no Go).
// SPDX-License-Identifier: Apache-2.0
#pragma once

#include "front.h"

#include <cstdint>
#include <map>
#include <string>
#include <vector>

struct Module {
    bool windows = true;
    std::vector<uint8_t> text;
    std::vector<uint8_t> rodata; // string pool: [u64 length][bytes], 8-byte aligned
    std::map<std::string, uint32_t> symbols;
    uint32_t entryMain = 0;

    struct CallPatch { size_t at; std::string target; };
    struct StrPatch { size_t at; size_t off; };

    std::vector<CallPatch> calls;               // rel32 sites (offset of disp32)
    std::vector<StrPatch> strings;              // RIP-relative string refs
    std::vector<std::pair<size_t, std::string>> iatCalls; // PE IAT call sites
    std::vector<size_t> stdoutRefs;             // PE [rip+stdout] disp32 sites
    std::vector<size_t> allocRefs;              // PE [rip+allocPtr] disp32 sites
    std::vector<size_t> newlineRefs;            // PE println newline disp32 sites
    std::vector<size_t> allocInitImm;           // PE movabs imm64 sites
};

// Compile the parsed program to machine code for the requested platform.
// The program is consumed because top-level statements are moved into the
// synthetic entry function.
Module compileModule(Program& prog, bool windows);
