// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

#pragma once

#include <cstdint>
#include <string>
#include <vector>
#include <unordered_map>
#include "ir/ir.h"

namespace ulang::codegen::x64 {

enum class ABI {
    Win64,
    SysV
};

struct CallFixup {
    std::string funcName;
    size_t patchOffset{0};
    size_t insEnd{0};
};

struct CompiledArtifact {
    std::vector<uint8_t> code;
    std::vector<uint8_t> readOnlyData;
    std::unordered_map<std::string, uint32_t> functionRVAs;
    uint32_t entryOffset{0};
    std::vector<CallFixup> callFixups;
};

class CodeGenerator {
public:
    explicit CodeGenerator(ABI abi);

    CompiledArtifact generate(const ir::Program& prog);

private:
    ABI abi_;
    std::vector<uint8_t> code_;
    std::vector<uint8_t> rodata_;
    std::unordered_map<std::string, int32_t> stringPool_;
    std::unordered_map<std::string, size_t> funcOffsets_;
    std::unordered_map<std::string, uint32_t> funcRVAs_;
    std::vector<CallFixup> funcCalls_;
    uint32_t entryOffset_{0};

    int32_t internString(const std::string& s);
    void compileFunction(const ir::Function& fn);

    // Emitter primitives
    void emitByte(uint8_t b) { code_.push_back(b); }
    void emitBytes(std::initializer_list<uint8_t> bytes) {
        code_.insert(code_.end(), bytes.begin(), bytes.end());
    }
    void emitUint32(uint32_t v) {
        for (int i = 0; i < 4; ++i) code_.push_back(static_cast<uint8_t>(v >> (i * 8)));
    }
    void emitUint64(uint64_t v) {
        for (int i = 0; i < 8; ++i) code_.push_back(static_cast<uint8_t>(v >> (i * 8)));
    }
    size_t codeLen() const { return code_.size(); }
};

} // namespace ulang::codegen::x64
