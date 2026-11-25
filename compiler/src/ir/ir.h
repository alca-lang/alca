// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

#pragma once

#include <cstdint>
#include <string>
#include <string_view>
#include <vector>
#include <memory>

namespace ulang::ir {

enum class Type {
    Void,
    I8,
    I64,
    F64,
    Ptr
};

std::string_view typeToString(Type t);

struct VReg {
    int id{-1};
    Type type{Type::Void};

    bool operator==(const VReg& o) const noexcept {
        return id == o.id && type == o.type;
    }
    bool operator!=(const VReg& o) const noexcept {
        return !(*this == o);
    }
    std::string toString() const;
};

enum class Op {
    Nop,
    ConstInt,
    ConstFloat,
    ConstBool,
    Copy,
    // Arithmetic
    Add,
    Sub,
    Mul,
    Div,
    Mod,
    // Negation
    Neg,
    // Bitwise / Logical
    Not,
    // Comparisons
    Eq,
    Ne,
    Lt,
    Le,
    Gt,
    Ge,
    // Control flow
    Jmp,
    Branch,
    Call,
    Ret,
    // String & I/O
    ConstStr,
    Print,
    Println,
    ConcatStr,
    IntToStr,
    IntToFloat,
    FloatToInt,
    MathUnary,
    MathBinary,
    // Memory & Objects (Stage 4)
    Alloc,
    Load,
    Store,
    CallPtr,
    // Error handling (Stage 5)
    SetErr,
    GetErr,
    // Bare-Metal & OS Dev (Stage 8)
    CpuCli,
    CpuSti,
    CpuHlt,
    CpuPause,
    CpuIn,
    CpuOut,
    CpuReadCr,
    CpuWriteCr,
    CpuInvlpg,
    RawLoad,
    RawStore
};

std::string_view opToString(Op op);

struct Block;

struct Instruction {
    Op op{Op::Nop};
    VReg dst{};
    VReg src1{};
    VReg src2{};
    int64_t intVal{0};
    double floatVal{0.0};
    std::string strVal{};
    Block* target{nullptr};
    Block* elseTgt{nullptr};
    std::string callName{};
    std::vector<VReg> args{};

    std::string toString() const;
};

struct Block {
    std::string name;
    std::vector<std::unique_ptr<Instruction>> instructions;
};

struct Function {
    std::string name;
    std::vector<VReg> params;
    Type returnType{Type::Void};
    std::vector<std::unique_ptr<Block>> blocks;
    int nextVRegId{0};

    VReg newVReg(Type t) {
        return VReg{nextVRegId++, t};
    }

    Block* newBlock(const std::string& prefix) {
        auto b = std::make_unique<Block>();
        b->name = prefix + "_" + std::to_string(blocks.size());
        Block* ptr = b.get();
        blocks.push_back(std::move(b));
        return ptr;
    }

    std::string toString() const;
};

struct Program {
    std::vector<std::unique_ptr<Function>> functions;
    Function* entryFn{nullptr};

    std::string toString() const;
};

} // namespace ulang::ir
