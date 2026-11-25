// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

#include "codegen/x64/codegen.h"
#include <cstring>
#include <stdexcept>

namespace pawl::codegen::x64 {

namespace {

enum Reg {
    RAX = 0, RCX = 1, RDX = 2, RBX = 3, RSP = 4, RBP = 5, RSI = 6, RDI = 7,
    R8 = 8, R9 = 9, R10 = 10, R11 = 11, R12 = 12, R13 = 13, R14 = 14, R15 = 15
};

struct FuncContext {
    const ir::Function& irFn;
    std::unordered_map<const ir::Block*, size_t> blockOffsets;
    std::vector<std::pair<size_t, const ir::Block*>> blockJumps; // patch offset -> target block
    std::unordered_map<int, int32_t> slotOffsets; // vreg id -> [rbp - disp]
    int32_t frameSize{0};
};

} // namespace

CodeGenerator::CodeGenerator(ABI abi) : abi_(abi) {}

int32_t CodeGenerator::internString(const std::string& s) {
    auto it = stringPool_.find(s);
    if (it != stringPool_.end()) {
        return it->second;
    }
    while (rodata_.size() % 8 != 0) {
        rodata_.push_back(0);
    }
    int32_t off = static_cast<int32_t>(rodata_.size());
    stringPool_[s] = off;

    // String layout: len: int64, data bytes + null
    uint64_t len = static_cast<uint64_t>(s.size());
    for (int i = 0; i < 8; ++i) rodata_.push_back(static_cast<uint8_t>(len >> (i * 8)));
    for (char c : s) rodata_.push_back(static_cast<uint8_t>(c));
    rodata_.push_back(0);
    return off;
}

CompiledArtifact CodeGenerator::generate(const ir::Program& prog) {
    code_.clear();
    rodata_.clear();
    stringPool_.clear();
    funcOffsets_.clear();
    funcRVAs_.clear();
    funcCalls_.clear();
    entryOffset_ = 0;

    for (const auto& fn : prog.functions) {
        if (fn.get() == prog.entryFn) continue;
        compileFunction(*fn);
    }

    if (prog.entryFn) {
        entryOffset_ = static_cast<uint32_t>(codeLen());
        compileFunction(*prog.entryFn);
    }

    std::vector<CallFixup> unpatched;
    for (const auto& fix : funcCalls_) {
        auto it = funcOffsets_.find(fix.funcName);
        if (it != funcOffsets_.end()) {
            int32_t rel32 = static_cast<int32_t>(it->second) - static_cast<int32_t>(fix.insEnd);
            std::memcpy(&code_[fix.patchOffset], &rel32, 4);
        } else {
            unpatched.push_back(fix);
        }
    }

    for (const auto& [name, off] : funcOffsets_) {
        funcRVAs_[name] = static_cast<uint32_t>(off);
    }

    CompiledArtifact art;
    art.code = std::move(code_);
    art.readOnlyData = std::move(rodata_);
    art.functionRVAs = std::move(funcRVAs_);
    art.entryOffset = entryOffset_;
    art.callFixups = std::move(unpatched);
    return art;
}

void CodeGenerator::compileFunction(const ir::Function& fn) {
    while (codeLen() % 16 != 0) {
        emitByte(0x90); // nop
    }
    funcOffsets_[fn.name] = codeLen();

    FuncContext ctx{fn, {}, {}, {}, 0};

    int totalVRegs = fn.nextVRegId;
    if (totalVRegs < static_cast<int>(fn.params.size())) {
        totalVRegs = static_cast<int>(fn.params.size());
    }

    for (int id = 0; id < totalVRegs; ++id) {
        ctx.slotOffsets[id] = (id + 1) * 8;
    }

    int32_t localBytes = totalVRegs * 8;
    if (abi_ == ABI::Win64) {
        localBytes += 32; // shadow store
    }
    ctx.frameSize = localBytes;
    if ((ctx.frameSize + 8) % 16 != 0) {
        ctx.frameSize += 16 - ((ctx.frameSize + 8) % 16);
    }

    // Prologue: push rbp; mov rbp, rsp; sub rsp, frameSize
    emitBytes({0x55}); // push rbp
    emitBytes({0x48, 0x89, 0xE5}); // mov rbp, rsp
    if (ctx.frameSize > 0) {
        emitBytes({0x48, 0x81, 0xEC}); // sub rsp, imm32
        emitUint32(static_cast<uint32_t>(ctx.frameSize));
    }

    // Store parameters into stack slots
    static const Reg kWin64ParamRegs[] = {RCX, RDX, R8, R9};
    static const Reg kSysVParamRegs[] = {RDI, RSI, RDX, RCX, R8, R9};
    const Reg* paramRegs = (abi_ == ABI::Win64) ? kWin64ParamRegs : kSysVParamRegs;
    size_t numParamRegs = (abi_ == ABI::Win64) ? 4 : 6;

    for (size_t i = 0; i < fn.params.size(); ++i) {
        int disp = -ctx.slotOffsets[fn.params[i].id];
        if (i < numParamRegs) {
            Reg r = paramRegs[i];
            // mov [rbp + disp32], reg
            uint8_t rex = 0x48 | ((r >= 8) ? 0x04 : 0);
            uint8_t modrm = 0x85 | ((r & 7) << 3);
            emitBytes({rex, 0x89, modrm});
            emitUint32(static_cast<uint32_t>(disp));
        } else {
            int32_t callerDisp = (abi_ == ABI::Win64) ? (48 + static_cast<int32_t>(i - 4) * 8) : (16 + static_cast<int32_t>(i - 6) * 8);
            // mov rax, [rbp + callerDisp32]
            emitBytes({0x48, 0x8B, 0x85});
            emitUint32(static_cast<uint32_t>(callerDisp));
            // mov [rbp + disp32], rax
            emitBytes({0x48, 0x89, 0x85});
            emitUint32(static_cast<uint32_t>(disp));
        }
    }

    // Instruction emit loop
    for (const auto& b : fn.blocks) {
        ctx.blockOffsets[b.get()] = codeLen();

        for (const auto& inst : b->instructions) {
            switch (inst->op) {
            case ir::Op::ConstInt: {
                int disp = -ctx.slotOffsets[inst->dst.id];
                // mov rax, imm64
                emitBytes({0x48, 0xB8});
                emitUint64(static_cast<uint64_t>(inst->intVal));
                // mov [rbp + disp32], rax
                emitBytes({0x48, 0x89, 0x85});
                emitUint32(static_cast<uint32_t>(disp));
                break;
            }
            case ir::Op::ConstBool: {
                int disp = -ctx.slotOffsets[inst->dst.id];
                // mov eax, imm32
                emitByte(0xB8);
                emitUint32(inst->intVal != 0 ? 1 : 0);
                // mov [rbp + disp32], rax
                emitBytes({0x48, 0x89, 0x85});
                emitUint32(static_cast<uint32_t>(disp));
                break;
            }
            case ir::Op::ConstStr: {
                int disp = -ctx.slotOffsets[inst->dst.id];
                int32_t strOff = internString(inst->strVal);
                // mov rax, imm64
                emitBytes({0x48, 0xB8});
                emitUint64(static_cast<uint64_t>(strOff));
                // mov [rbp + disp32], rax
                emitBytes({0x48, 0x89, 0x85});
                emitUint32(static_cast<uint32_t>(disp));
                break;
            }
            case ir::Op::ConstFloat: {
                int disp = -ctx.slotOffsets[inst->dst.id];
                uint64_t bits = 0;
                std::memcpy(&bits, &inst->floatVal, 8);
                emitBytes({0x48, 0xB8});
                emitUint64(bits);
                emitBytes({0x48, 0x89, 0x85});
                emitUint32(static_cast<uint32_t>(disp));
                break;
            }
            case ir::Op::Copy: {
                int sDisp = -ctx.slotOffsets[inst->src1.id];
                int dDisp = -ctx.slotOffsets[inst->dst.id];
                // mov rax, [rbp + sDisp32]
                emitBytes({0x48, 0x8B, 0x85});
                emitUint32(static_cast<uint32_t>(sDisp));
                // mov [rbp + dDisp32], rax
                emitBytes({0x48, 0x89, 0x85});
                emitUint32(static_cast<uint32_t>(dDisp));
                break;
            }
            case ir::Op::Add: {
                int s1 = -ctx.slotOffsets[inst->src1.id];
                int s2 = -ctx.slotOffsets[inst->src2.id];
                int dst = -ctx.slotOffsets[inst->dst.id];
                if (inst->dst.type == ir::Type::F64 || inst->src1.type == ir::Type::F64) {
                    emitBytes({0xF2, 0x0F, 0x10, 0x85}); emitUint32(static_cast<uint32_t>(s1)); // movsd xmm0,[s1]
                    emitBytes({0xF2, 0x0F, 0x58, 0x85}); emitUint32(static_cast<uint32_t>(s2)); // addsd xmm0,[s2]
                    emitBytes({0xF2, 0x0F, 0x11, 0x85}); emitUint32(static_cast<uint32_t>(dst)); // movsd [dst],xmm0
                } else {
                    emitBytes({0x48, 0x8B, 0x85}); emitUint32(static_cast<uint32_t>(s1)); // mov rax, [s1]
                    emitBytes({0x48, 0x8B, 0x8D}); emitUint32(static_cast<uint32_t>(s2)); // mov rcx, [s2]
                    emitBytes({0x48, 0x01, 0xC8});                                         // add rax, rcx
                    emitBytes({0x48, 0x89, 0x85}); emitUint32(static_cast<uint32_t>(dst)); // mov [dst], rax
                }
                break;
            }
            case ir::Op::Sub: {
                int s1 = -ctx.slotOffsets[inst->src1.id];
                int s2 = -ctx.slotOffsets[inst->src2.id];
                int dst = -ctx.slotOffsets[inst->dst.id];
                if (inst->dst.type == ir::Type::F64 || inst->src1.type == ir::Type::F64) {
                    emitBytes({0xF2, 0x0F, 0x10, 0x85}); emitUint32(static_cast<uint32_t>(s1));
                    emitBytes({0xF2, 0x0F, 0x5C, 0x85}); emitUint32(static_cast<uint32_t>(s2)); // subsd
                    emitBytes({0xF2, 0x0F, 0x11, 0x85}); emitUint32(static_cast<uint32_t>(dst));
                } else {
                    emitBytes({0x48, 0x8B, 0x85}); emitUint32(static_cast<uint32_t>(s1)); // mov rax, [s1]
                    emitBytes({0x48, 0x8B, 0x8D}); emitUint32(static_cast<uint32_t>(s2)); // mov rcx, [s2]
                    emitBytes({0x48, 0x29, 0xC8});                                         // sub rax, rcx
                    emitBytes({0x48, 0x89, 0x85}); emitUint32(static_cast<uint32_t>(dst)); // mov [dst], rax
                }
                break;
            }
            case ir::Op::Mul: {
                int s1 = -ctx.slotOffsets[inst->src1.id];
                int s2 = -ctx.slotOffsets[inst->src2.id];
                int dst = -ctx.slotOffsets[inst->dst.id];
                if (inst->dst.type == ir::Type::F64 || inst->src1.type == ir::Type::F64) {
                    emitBytes({0xF2, 0x0F, 0x10, 0x85}); emitUint32(static_cast<uint32_t>(s1));
                    emitBytes({0xF2, 0x0F, 0x59, 0x85}); emitUint32(static_cast<uint32_t>(s2)); // mulsd
                    emitBytes({0xF2, 0x0F, 0x11, 0x85}); emitUint32(static_cast<uint32_t>(dst));
                } else {
                    emitBytes({0x48, 0x8B, 0x85}); emitUint32(static_cast<uint32_t>(s1)); // mov rax, [s1]
                    emitBytes({0x48, 0x8B, 0x8D}); emitUint32(static_cast<uint32_t>(s2)); // mov rcx, [s2]
                    emitBytes({0x48, 0x0F, 0xAF, 0xC1});                                   // imul rax, rcx
                    emitBytes({0x48, 0x89, 0x85}); emitUint32(static_cast<uint32_t>(dst)); // mov [dst], rax
                }
                break;
            }
            case ir::Op::Div: {
                int s1 = -ctx.slotOffsets[inst->src1.id];
                int s2 = -ctx.slotOffsets[inst->src2.id];
                int dst = -ctx.slotOffsets[inst->dst.id];
                if (inst->dst.type == ir::Type::F64 || inst->src1.type == ir::Type::F64) {
                    emitBytes({0xF2, 0x0F, 0x10, 0x85}); emitUint32(static_cast<uint32_t>(s1));
                    emitBytes({0xF2, 0x0F, 0x5E, 0x85}); emitUint32(static_cast<uint32_t>(s2)); // divsd
                    emitBytes({0xF2, 0x0F, 0x11, 0x85}); emitUint32(static_cast<uint32_t>(dst));
                } else {
                    emitBytes({0x48, 0x8B, 0x85}); emitUint32(static_cast<uint32_t>(s1)); // mov rax, [s1]
                    emitBytes({0x48, 0x8B, 0x8D}); emitUint32(static_cast<uint32_t>(s2)); // mov rcx, [s2]
                    emitBytes({0x48, 0x99});                                               // cqo
                    emitBytes({0x48, 0xF7, 0xF9});                                         // idiv rcx
                    emitBytes({0x48, 0x89, 0x85}); emitUint32(static_cast<uint32_t>(dst)); // mov [dst], rax
                }
                break;
            }
            case ir::Op::Mod: {
                int s1 = -ctx.slotOffsets[inst->src1.id];
                int s2 = -ctx.slotOffsets[inst->src2.id];
                int dst = -ctx.slotOffsets[inst->dst.id];
                emitBytes({0x48, 0x8B, 0x85}); emitUint32(static_cast<uint32_t>(s1)); // mov rax, [s1]
                emitBytes({0x48, 0x8B, 0x8D}); emitUint32(static_cast<uint32_t>(s2)); // mov rcx, [s2]
                emitBytes({0x48, 0x99});                                               // cqo
                emitBytes({0x48, 0xF7, 0xF9});                                         // idiv rcx
                emitBytes({0x48, 0x89, 0x95}); emitUint32(static_cast<uint32_t>(dst)); // mov [dst], rdx
                break;
            }
            case ir::Op::Neg: {
                int s1 = -ctx.slotOffsets[inst->src1.id];
                int dst = -ctx.slotOffsets[inst->dst.id];
                if (inst->dst.type == ir::Type::F64 || inst->src1.type == ir::Type::F64) {
                    // Flip the IEEE-754 sign bit.
                    emitBytes({0x48, 0x8B, 0x85}); emitUint32(static_cast<uint32_t>(s1)); // mov rax, [s1]
                    emitBytes({0x48, 0xB9}); emitUint64(0x8000000000000000ULL);           // mov rcx, sign mask
                    emitBytes({0x48, 0x31, 0xC8});                                         // xor rax, rcx
                    emitBytes({0x48, 0x89, 0x85}); emitUint32(static_cast<uint32_t>(dst)); // mov [dst], rax
                } else {
                    emitBytes({0x48, 0x8B, 0x85}); emitUint32(static_cast<uint32_t>(s1)); // mov rax, [s1]
                    emitBytes({0x48, 0xF7, 0xD8});                                         // neg rax
                    emitBytes({0x48, 0x89, 0x85}); emitUint32(static_cast<uint32_t>(dst)); // mov [dst], rax
                }
                break;
            }
            case ir::Op::Not: {
                int s1 = -ctx.slotOffsets[inst->src1.id];
                int dst = -ctx.slotOffsets[inst->dst.id];
                emitBytes({0x48, 0x8B, 0x85}); emitUint32(static_cast<uint32_t>(s1)); // mov rax, [s1]
                emitBytes({0x48, 0x85, 0xC0});                                         // test rax, rax
                emitBytes({0x0F, 0x94, 0xC0});                                         // sete al
                emitBytes({0x0F, 0xB6, 0xC0});                                         // movzx eax, al
                emitBytes({0x48, 0x89, 0x85}); emitUint32(static_cast<uint32_t>(dst)); // mov [dst], rax
                break;
            }
            case ir::Op::Eq:
            case ir::Op::Ne:
            case ir::Op::Lt:
            case ir::Op::Le:
            case ir::Op::Gt:
            case ir::Op::Ge: {
                int s1 = -ctx.slotOffsets[inst->src1.id];
                int s2 = -ctx.slotOffsets[inst->src2.id];
                int dst = -ctx.slotOffsets[inst->dst.id];
                bool isFloat = inst->src1.type == ir::Type::F64 || inst->src2.type == ir::Type::F64;
                if (isFloat) {
                    emitBytes({0xF2, 0x0F, 0x10, 0x85}); emitUint32(static_cast<uint32_t>(s1)); // movsd xmm0,[s1]
                    emitBytes({0x66, 0x0F, 0x2E, 0x85}); emitUint32(static_cast<uint32_t>(s2)); // ucomisd xmm0,[s2]
                    if (inst->op == ir::Op::Ne) {
                        emitBytes({0x0F, 0x95, 0xC0}); // setne al
                        emitBytes({0x0F, 0x9A, 0xC1}); // setp cl
                        emitBytes({0x08, 0xC8});       // or al, cl
                    } else {
                        uint8_t setccCode = 0x94;      // sete
                        if (inst->op == ir::Op::Lt) setccCode = 0x92;      // setb
                        else if (inst->op == ir::Op::Le) setccCode = 0x96; // setbe
                        else if (inst->op == ir::Op::Gt) setccCode = 0x97; // seta
                        else if (inst->op == ir::Op::Ge) setccCode = 0x93; // setae
                        emitBytes({0x0F, setccCode, 0xC0}); // setcc al
                        emitBytes({0x0F, 0x9B, 0xC1});      // setnp cl
                        emitBytes({0x20, 0xC8});            // and al, cl
                    }
                    emitBytes({0x0F, 0xB6, 0xC0});                                         // movzx eax, al
                    emitBytes({0x48, 0x89, 0x85}); emitUint32(static_cast<uint32_t>(dst)); // mov [dst], rax
                    break;
                }

                emitBytes({0x48, 0x8B, 0x85}); emitUint32(static_cast<uint32_t>(s1)); // mov rax, [s1]
                emitBytes({0x48, 0x8B, 0x8D}); emitUint32(static_cast<uint32_t>(s2)); // mov rcx, [s2]
                emitBytes({0x48, 0x39, 0xC8});                                         // cmp rax, rcx

                uint8_t setccCode = 0x94; // sete
                if (inst->op == ir::Op::Ne) setccCode = 0x95; // setne
                else if (inst->op == ir::Op::Lt) setccCode = 0x9C; // setl
                else if (inst->op == ir::Op::Le) setccCode = 0x9E; // setle
                else if (inst->op == ir::Op::Gt) setccCode = 0x9F; // setg
                else if (inst->op == ir::Op::Ge) setccCode = 0x9D; // setge

                emitBytes({0x0F, setccCode, 0xC0});                                    // setcc al
                emitBytes({0x0F, 0xB6, 0xC0});                                         // movzx eax, al
                emitBytes({0x48, 0x89, 0x85}); emitUint32(static_cast<uint32_t>(dst)); // mov [dst], rax
                break;
            }
            case ir::Op::Jmp: {
                emitByte(0xE9); // jmp rel32
                ctx.blockJumps.push_back({codeLen(), inst->target});
                emitUint32(0);
                break;
            }
            case ir::Op::Branch: {
                int cDisp = -ctx.slotOffsets[inst->src1.id];
                emitBytes({0x48, 0x8B, 0x85}); emitUint32(static_cast<uint32_t>(cDisp)); // mov rax, [cDisp]
                emitBytes({0x48, 0x85, 0xC0});                                           // test rax, rax
                emitBytes({0x0F, 0x85});                                                 // jne rel32
                ctx.blockJumps.push_back({codeLen(), inst->target});
                emitUint32(0);
                emitByte(0xE9);                                                          // jmp rel32
                ctx.blockJumps.push_back({codeLen(), inst->elseTgt});
                emitUint32(0);
                break;
            }
            case ir::Op::Call: {
                for (size_t i = 0; i < inst->args.size(); ++i) {
                    int aDisp = -ctx.slotOffsets[inst->args[i].id];
                    if (i < numParamRegs) {
                        Reg r = paramRegs[i];
                        uint8_t rex = 0x48 | ((r >= 8) ? 0x04 : 0);
                        uint8_t modrm = 0x85 | ((r & 7) << 3);
                        emitBytes({rex, 0x8B, modrm});
                        emitUint32(static_cast<uint32_t>(aDisp));
                    } else {
                        emitBytes({0x48, 0x8B, 0x85});
                        emitUint32(static_cast<uint32_t>(aDisp));
                        emitBytes({0x50}); // push rax
                    }
                }

                emitByte(0xE8); // call rel32
                size_t patch = codeLen();
                emitUint32(0);
                funcCalls_.push_back({inst->callName, patch, patch + 4});

                if (inst->dst.type != ir::Type::Void && inst->dst.id >= 0) {
                    int dDisp = -ctx.slotOffsets[inst->dst.id];
                    emitBytes({0x48, 0x89, 0x85});
                    emitUint32(static_cast<uint32_t>(dDisp));
                }
                break;
            }
            case ir::Op::Print:
            case ir::Op::Println: {
                std::string rtName = (inst->op == ir::Op::Print) ? "pawl_print" : "pawl_println";
                int aDisp = -ctx.slotOffsets[inst->src1.id];
                if (abi_ == ABI::Win64) {
                    emitBytes({0x48, 0x8B, 0x8D}); // mov rcx, [rbp + aDisp]
                    emitUint32(static_cast<uint32_t>(aDisp));
                } else {
                    emitBytes({0x48, 0x8B, 0xBD}); // mov rdi, [rbp + aDisp]
                    emitUint32(static_cast<uint32_t>(aDisp));
                }
                emitByte(0xE8);
                size_t patch = codeLen();
                emitUint32(0);
                funcCalls_.push_back({rtName, patch, patch + 4});
                break;
            }
            case ir::Op::ConcatStr: {
                int s1 = -ctx.slotOffsets[inst->src1.id];
                int s2 = -ctx.slotOffsets[inst->src2.id];
                int dst = -ctx.slotOffsets[inst->dst.id];
                if (abi_ == ABI::Win64) {
                    emitBytes({0x48, 0x8B, 0x8D}); emitUint32(static_cast<uint32_t>(s1)); // mov rcx, [s1]
                    emitBytes({0x48, 0x8B, 0x95}); emitUint32(static_cast<uint32_t>(s2)); // mov rdx, [s2]
                } else {
                    emitBytes({0x48, 0x8B, 0xBD}); emitUint32(static_cast<uint32_t>(s1)); // mov rdi, [s1]
                    emitBytes({0x48, 0x8B, 0xB5}); emitUint32(static_cast<uint32_t>(s2)); // mov rsi, [s2]
                }
                emitByte(0xE8);
                size_t patch = codeLen();
                emitUint32(0);
                funcCalls_.push_back({"pawl_concat_str", patch, patch + 4});
                emitBytes({0x48, 0x89, 0x85}); emitUint32(static_cast<uint32_t>(dst)); // mov [dst], rax
                break;
            }
            case ir::Op::IntToStr: {
                int s1 = -ctx.slotOffsets[inst->src1.id];
                int dst = -ctx.slotOffsets[inst->dst.id];
                if (abi_ == ABI::Win64) {
                    emitBytes({0x48, 0x8B, 0x8D}); emitUint32(static_cast<uint32_t>(s1)); // mov rcx, [s1]
                } else {
                    emitBytes({0x48, 0x8B, 0xBD}); emitUint32(static_cast<uint32_t>(s1)); // mov rdi, [s1]
                }
                emitByte(0xE8);
                size_t patch = codeLen();
                emitUint32(0);
                funcCalls_.push_back({"pawl_int_to_str", patch, patch + 4});
                emitBytes({0x48, 0x89, 0x85}); emitUint32(static_cast<uint32_t>(dst)); // mov [dst], rax
                break;
            }
            case ir::Op::IntToFloat: {
                int s1 = -ctx.slotOffsets[inst->src1.id];
                int dst = -ctx.slotOffsets[inst->dst.id];
                emitBytes({0x48, 0x8B, 0x85}); emitUint32(static_cast<uint32_t>(s1)); // mov rax, [s1]
                emitBytes({0xF2, 0x48, 0x0F, 0x2A, 0xC0});                           // cvtsi2sd xmm0, rax
                emitBytes({0xF2, 0x0F, 0x11, 0x85}); emitUint32(static_cast<uint32_t>(dst));
                break;
            }
            case ir::Op::FloatToInt: {
                int s1 = -ctx.slotOffsets[inst->src1.id];
                int dst = -ctx.slotOffsets[inst->dst.id];
                emitBytes({0xF2, 0x0F, 0x10, 0x85}); emitUint32(static_cast<uint32_t>(s1)); // movsd xmm0,[s1]
                emitBytes({0xF2, 0x48, 0x0F, 0x2C, 0xC0});                                 // cvttsd2si rax, xmm0
                emitBytes({0x48, 0x89, 0x85}); emitUint32(static_cast<uint32_t>(dst));
                break;
            }
            case ir::Op::Alloc: {
                int s1 = -ctx.slotOffsets[inst->src1.id];
                int dst = -ctx.slotOffsets[inst->dst.id];
                if (abi_ == ABI::Win64) {
                    emitBytes({0x48, 0x8B, 0x8D}); emitUint32(static_cast<uint32_t>(s1)); // mov rcx, [s1]
                } else {
                    emitBytes({0x48, 0x8B, 0xBD}); emitUint32(static_cast<uint32_t>(s1)); // mov rdi, [s1]
                }
                emitByte(0xE8);
                size_t patch = codeLen();
                emitUint32(0);
                funcCalls_.push_back({"pawl_alloc", patch, patch + 4});
                emitBytes({0x48, 0x89, 0x85}); emitUint32(static_cast<uint32_t>(dst)); // mov [dst], rax
                break;
            }
            case ir::Op::Load: {
                int bDisp = -ctx.slotOffsets[inst->src1.id];
                int dDisp = -ctx.slotOffsets[inst->dst.id];
                emitBytes({0x48, 0x8B, 0x85}); emitUint32(static_cast<uint32_t>(bDisp)); // mov rax, [rbp + bDisp]
                if (inst->dst.type == ir::Type::I8) {
                    emitBytes({0x0F, 0xB6, 0x80}); // movzx eax, byte ptr [rax + disp32]
                    emitUint32(static_cast<uint32_t>(inst->intVal));
                } else {
                    emitBytes({0x48, 0x8B, 0x80}); // mov rax, [rax + disp32]
                    emitUint32(static_cast<uint32_t>(inst->intVal));
                }
                emitBytes({0x48, 0x89, 0x85}); emitUint32(static_cast<uint32_t>(dDisp)); // mov [rbp + dDisp], rax
                break;
            }
            case ir::Op::Store: {
                int bDisp = -ctx.slotOffsets[inst->src1.id];
                int vDisp = -ctx.slotOffsets[inst->src2.id];
                emitBytes({0x48, 0x8B, 0x85}); emitUint32(static_cast<uint32_t>(bDisp)); // mov rax, [rbp + bDisp]
                emitBytes({0x48, 0x8B, 0x8D}); emitUint32(static_cast<uint32_t>(vDisp)); // mov rcx, [rbp + vDisp]
                if (inst->src2.type == ir::Type::I8) {
                    emitBytes({0x88, 0x88}); // mov [rax + disp32], cl
                    emitUint32(static_cast<uint32_t>(inst->intVal));
                } else {
                    emitBytes({0x48, 0x89, 0x88}); // mov [rax + disp32], rcx
                    emitUint32(static_cast<uint32_t>(inst->intVal));
                }
                break;
            }
            case ir::Op::CallPtr: {
                for (size_t i = 0; i < inst->args.size(); ++i) {
                    int aDisp = -ctx.slotOffsets[inst->args[i].id];
                    if (i < numParamRegs) {
                        Reg r = paramRegs[i];
                        uint8_t rex = 0x48 | ((r >= 8) ? 0x04 : 0);
                        uint8_t modrm = 0x85 | ((r & 7) << 3);
                        emitBytes({rex, 0x8B, modrm});
                        emitUint32(static_cast<uint32_t>(aDisp));
                    } else {
                        emitBytes({0x48, 0x8B, 0x85});
                        emitUint32(static_cast<uint32_t>(aDisp));
                        emitBytes({0x50}); // push rax
                    }
                }
                int pDisp = -ctx.slotOffsets[inst->src1.id];
                emitBytes({0x48, 0x8B, 0x85}); emitUint32(static_cast<uint32_t>(pDisp)); // mov rax, [pDisp]
                emitBytes({0xFF, 0xD0}); // call rax
                if (inst->dst.type != ir::Type::Void && inst->dst.id >= 0) {
                    int dDisp = -ctx.slotOffsets[inst->dst.id];
                    emitBytes({0x48, 0x89, 0x85}); emitUint32(static_cast<uint32_t>(dDisp));
                }
                break;
            }
            case ir::Op::SetErr: {
                int sDisp = -ctx.slotOffsets[inst->src1.id];
                // mov r14, [rbp + sDisp32]
                emitBytes({0x4C, 0x8B, 0xB5});
                emitUint32(static_cast<uint32_t>(sDisp));
                break;
            }
            case ir::Op::GetErr: {
                int dDisp = -ctx.slotOffsets[inst->dst.id];
                // mov [rbp + dDisp32], r14
                emitBytes({0x4C, 0x89, 0xB5});
                emitUint32(static_cast<uint32_t>(dDisp));
                break;
            }
            case ir::Op::Ret: {
                if (inst->src1.type != ir::Type::Void && inst->src1.id >= 0) {
                    int sDisp = -ctx.slotOffsets[inst->src1.id];
                    emitBytes({0x48, 0x8B, 0x85});
                    emitUint32(static_cast<uint32_t>(sDisp));
                }
                emitBytes({0x48, 0x89, 0xEC}); // mov rsp, rbp
                emitBytes({0x5D});             // pop rbp
                emitBytes({0xC3});             // ret
                break;
            }
            default:
                break;
            }
        }
    }

    // Resolve internal jumps
    for (const auto& jmp : ctx.blockJumps) {
        size_t targetOff = ctx.blockOffsets[jmp.second];
        int32_t rel32 = static_cast<int32_t>(targetOff) - static_cast<int32_t>(jmp.first + 4);
        std::memcpy(&code_[jmp.first], &rel32, 4);
    }
}

} // namespace pawl::codegen::x64
