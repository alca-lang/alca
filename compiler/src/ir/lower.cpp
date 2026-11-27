// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

#include "ir/lower.h"
#include <cctype>
#include <functional>
#include <iterator>
#include <unordered_map>
#include <unordered_set>
#include <vector>
#include <stdexcept>
#include <sstream>

namespace ulang::ir {

namespace {

static int64_t typeTagHash(const std::string& s) {
    uint64_t h = 1469598103934665603ull;
    for (unsigned char c : s) { h ^= c; h *= 1099511628211ull; }
    return static_cast<int64_t>(h & 0x7FFFFFFF);
}

struct StructLayout {
    std::string name;
    bool isClass{false};
    std::unordered_map<std::string, int64_t> fields; // field name -> byte offset
    std::unordered_map<std::string, Type> fieldTypes; // field name -> IR type
    int64_t totalSize{0};
    std::unordered_map<std::string, int> methodVCols; // method name -> vtable index
};

class LoweringContext {
public:
    LoweringContext(const typecheck::Checker& chk)
        : checker(chk) {
        irProg = std::make_unique<Program>();
    }

    const typecheck::Checker& checker;
    std::unique_ptr<Program> irProg;
    std::unordered_map<std::string, Function*> functions;
    std::unordered_map<std::string, StructLayout> structLayouts;
    std::unordered_map<std::string, std::unordered_map<std::string, int64_t>> enums; // enumName -> variantName -> tag

    Function* currentFn{nullptr};
    Block* currentBlock{nullptr};
    std::unordered_map<std::string, VReg> locals;

    // Top-level `NAME := literal` declarations (mostly hardware constants in
    // std/*.upp) are visible to the typechecker in the global scope, but IR
    // locals are per-function.  Substitute their literals at each use site so
    // module functions can reference them from any body.  Names that are ever
    // assigned are deliberately excluded: those are mutable globals, which
    // this backend does not support (see ADR-0004).
    std::unordered_map<std::string, const ast::Expression*> moduleConsts;
    std::unordered_map<std::string, int64_t> constexprValues;
    std::unordered_set<std::string> assignedNames;
    // Mutable top-level variables: name -> slot index in irProg->globals.
    std::unordered_map<std::string, uint32_t> globalIndex;

    // Generic functions and overloads are resolved at call sites and lowered
    // on demand into mangled, monomorphized IR functions.
    std::unordered_map<std::string, std::vector<const ast::FuncDeclaration*>> funcDecls;
    std::unordered_map<std::string, std::string> typeSubst;
    std::unordered_map<std::string, Function*> specializations;

    // RAII: expressions registered by `defer`, replayed before every Ret.
    std::vector<ast::ExpressionPtr> deferredExprs;
    bool loweringDeferred{false};
    // Implicit destructors: locals holding objects with `func ~Type(self)`.
    std::unordered_map<std::string, const ast::FuncDeclaration*> destructors;
    // One frame per lexical block; index 0 is the function body itself.
    std::vector<std::vector<std::pair<std::string, std::string>>> destructibleScopes;
    // Snapshot of active objects at each explicit/implicit Ret instruction.
    std::unordered_map<const Instruction*, std::vector<std::pair<std::string, std::string>>> retDestructors;
    std::vector<Block*> catchTargets;
    std::unordered_map<Block*, int64_t> catchTypeHashes;
    int tmpCounter{0};

    std::vector<Block*> loopContinue;
    std::vector<Block*> loopBreak;

    void emit(std::unique_ptr<Instruction> inst) {
        if (!currentBlock) return;
        currentBlock->instructions.push_back(std::move(inst));
    }

    void emit(Instruction inst) {
        emit(std::make_unique<Instruction>(std::move(inst)));
    }

    bool isTerminated(const Block* b) const {
        if (!b || b->instructions.empty()) return false;
        Op last = b->instructions.back()->op;
        return last == Op::Ret || last == Op::Jmp || last == Op::Branch;
    }

    std::string applyTypeSubst(std::string s) const {
        for (const auto& [from, to] : typeSubst) {
            if (from.empty()) continue;
            size_t pos = 0;
            while ((pos = s.find(from, pos)) != std::string::npos) {
                bool left = pos == 0 || !std::isalnum(static_cast<unsigned char>(s[pos - 1]));
                size_t end = pos + from.size();
                bool right = end >= s.size() || !std::isalnum(static_cast<unsigned char>(s[end]));
                if (left && right) {
                    s.replace(pos, from.size(), to);
                    pos += to.size();
                } else {
                    pos = end;
                }
            }
        }
        return s;
    }

    Type annotationToIR(const std::string& typeName) const {
        const std::string n = applyTypeSubst(typeName);
        if (n == "int") return Type::I64;
        if (n == "float") return Type::F64;
        if (n == "bool") return Type::I8;
        if (n == "void") return Type::Void;
        return Type::Ptr;
    }

    static std::string irTypeToName(Type t) {
        if (t == Type::I64) return "int";
        if (t == Type::F64) return "float";
        if (t == Type::I8) return "bool";
        if (t == Type::Void) return "void";
        return "ptr";
    }

    // Resolve a call that is not a plain declared function: pick a matching
    // overload/generic declaration, infer type arguments, and emit a mangled
    // monomorphized copy of its body.
    Function* resolveSpecialCall(const std::string& name, const std::vector<VReg>& args) {
        auto it = funcDecls.find(name);
        if (it == funcDecls.end()) return nullptr;
        for (const ast::FuncDeclaration* decl : it->second) {
            if (decl->params.size() != args.size()) continue;

            std::unordered_map<std::string, std::string> saved = typeSubst;
            for (size_t i = 0; i < decl->typeParams.size() && i < args.size(); ++i) {
                typeSubst[decl->typeParams[i]] = irTypeToName(args[i].type);
            }

            bool match = true;
            for (size_t i = 0; i < decl->params.size(); ++i) {
                const auto& ann = decl->params[i]->typeAnnotation;
                Type expected = ann && !ann->types.empty()
                              ? annotationToIR(ann->types[0]) : Type::I64;
                if (expected != args[i].type && expected != Type::Ptr) {
                    match = false;
                    break;
                }
            }
            if (!match) {
                typeSubst = saved;
                continue;
            }

            std::vector<std::string> argNames;
            for (const auto& a : args) argNames.push_back(irTypeToName(a.type));
            std::string mangled = name;
            for (const auto& n : argNames) mangled += "_" + n;
            auto found = functions.find(mangled);
            if (found != functions.end()) {
                typeSubst = saved;
                return found->second;
            }
            auto cached = specializations.find(mangled);
            if (cached != specializations.end()) {
                typeSubst = saved;
                return cached->second;
            }

            declareFunc(mangled, decl->params, decl->returnType.get());
            Function* fn = functions[mangled];
            specializations[mangled] = fn;

            Function* savedFn = currentFn;
            Block* savedBlock = currentBlock;
            auto savedLocals = locals;
            auto savedContinue = loopContinue;
            auto savedBreak = loopBreak;
            lowerFunctionBody(fn, decl->params, decl->body ? decl->body->statements
                                                          : std::vector<ast::StatementPtr>{}, false);
            currentFn = savedFn;
            currentBlock = savedBlock;
            locals = std::move(savedLocals);
            loopContinue = std::move(savedContinue);
            loopBreak = std::move(savedBreak);
            typeSubst = saved;
            return fn;
        }
        return nullptr;
    }

    Type typeToIR(const std::shared_ptr<typecheck::Type>& t) const {
        if (!t) return Type::I64;
        std::string s = t->toString();
        if (s == "int") return Type::I64;
        if (s == "float") return Type::F64;
        if (s == "bool") return Type::I8;
        if (s == "void") return Type::Void;
        return Type::Ptr;
    }

    void buildLayouts(const ast::Program& prog) {
        // Built-in error struct layout: code: i64 (off 0), msg: ptr (off 8)
        StructLayout errLayout;
        errLayout.name = "error";
        errLayout.isClass = false;
        errLayout.fields["code"] = 0;
        errLayout.fields["msg"] = 8;
        errLayout.fields["message"] = 8;
        errLayout.fieldTypes["code"] = Type::I64;
        errLayout.fieldTypes["msg"] = Type::Ptr;
        errLayout.fieldTypes["message"] = Type::Ptr;
        errLayout.totalSize = 16;
        structLayouts["error"] = errLayout;

        for (const auto& stmt : prog.statements) {
            if (auto* s = dynamic_cast<ast::EnumDeclaration*>(stmt.get())) {
                std::unordered_map<std::string, int64_t> variants;
                for (size_t idx = 0; idx < s->variants.size(); ++idx) {
                    int64_t tag = static_cast<int64_t>(idx);
                    if (s->variants[idx]->value) {
                        if (auto* il = dynamic_cast<ast::IntegerLiteral*>(s->variants[idx]->value.get())) {
                            tag = il->value;
                        }
                    }
                    variants[s->variants[idx]->name] = tag;
                }
                enums[s->name] = std::move(variants);
            } else if (auto* s = dynamic_cast<ast::StructDeclaration*>(stmt.get())) {
                StructLayout layout;
                layout.name = s->name;
                layout.isClass = false;
                int64_t offset = 0;
                for (const auto& f : s->fields) {
                    Type fType = Type::I64;
                    if (f->typeAnnotation && !f->typeAnnotation->types.empty()) {
                        fType = annotationToIR(f->typeAnnotation->types[0]);
                    }
                    layout.fields[f->name] = offset;
                    layout.fieldTypes[f->name] = fType;
                    offset += 8;
                }
                layout.totalSize = offset;
                structLayouts[s->name] = std::move(layout);
            } else if (auto* s = dynamic_cast<ast::ClassDeclaration*>(stmt.get())) {
                StructLayout layout;
                layout.name = s->name;
                layout.isClass = true;
                int64_t offset = 8; // offset 0 is vtable ptr

                if (!s->parent.empty()) {
                    auto pIt = structLayouts.find(s->parent);
                    if (pIt != structLayouts.end()) {
                        for (const auto& [fName, fOff] : pIt->second.fields) {
                            layout.fields[fName] = fOff;
                            layout.fieldTypes[fName] = pIt->second.fieldTypes.at(fName);
                            if (fOff + 8 > offset) {
                                offset = fOff + 8;
                            }
                        }
                        for (const auto& [mName, vIdx] : pIt->second.methodVCols) {
                            layout.methodVCols[mName] = vIdx;
                        }
                    }
                }

                for (const auto& f : s->fields) {
                    Type fType = Type::I64;
                    if (f->typeAnnotation && !f->typeAnnotation->types.empty()) {
                        fType = annotationToIR(f->typeAnnotation->types[0]);
                    }
                    layout.fields[f->name] = offset;
                    layout.fieldTypes[f->name] = fType;
                    offset += 8;
                }
                layout.totalSize = offset;

                for (const auto& m : s->methods) {
                    if (m && m->func) {
                        const std::string& mName = m->func->name;
                        if (layout.methodVCols.find(mName) == layout.methodVCols.end()) {
                            layout.methodVCols[mName] = static_cast<int>(layout.methodVCols.size());
                        }
                    }
                }
                structLayouts[s->name] = std::move(layout);
            }
        }
    }

    void declareFunc(const std::string& name, const std::vector<std::shared_ptr<ast::FuncParam>>& params, const ast::TypeAnnotation* retAnn) {
        Type retType = Type::Void;
        if (retAnn && !retAnn->types.empty()) {
            retType = annotationToIR(retAnn->types[0]);
        }
        auto fn = std::make_unique<Function>();
        fn->name = name;
        fn->returnType = retType;
        for (size_t i = 0; i < params.size(); ++i) {
            Type pType = Type::I64;
            if (params[i]->typeAnnotation && !params[i]->typeAnnotation->types.empty()) {
                pType = annotationToIR(params[i]->typeAnnotation->types[0]);
            }
            fn->params.push_back(VReg{static_cast<int>(i), pType});
        }
        fn->nextVRegId = static_cast<int>(fn->params.size());
        Function* ptr = fn.get();
        functions[name] = ptr;
        irProg->functions.push_back(std::move(fn));
    }

    void declareMethod(const std::string& mName, const std::vector<std::shared_ptr<ast::FuncParam>>& params, const ast::TypeAnnotation* retAnn) {
        Type retType = Type::Void;
        if (retAnn && !retAnn->types.empty()) {
            retType = annotationToIR(retAnn->types[0]);
        }
        auto fn = std::make_unique<Function>();
        fn->name = mName;
        fn->returnType = retType;
        // Param 0 is self (ptr)
        fn->params.push_back(VReg{0, Type::Ptr});
        for (size_t i = 0; i < params.size(); ++i) {
            Type pType = Type::I64;
            if (params[i]->typeAnnotation && !params[i]->typeAnnotation->types.empty()) {
                pType = annotationToIR(params[i]->typeAnnotation->types[0]);
            }
            fn->params.push_back(VReg{static_cast<int>(i + 1), pType});
        }
        fn->nextVRegId = static_cast<int>(fn->params.size());
        Function* ptr = fn.get();
        functions[mName] = ptr;
        irProg->functions.push_back(std::move(fn));
    }

    void lowerFunctionBody(Function* irFn, const std::vector<std::shared_ptr<ast::FuncParam>>& params, const std::vector<ast::StatementPtr>& body, bool isMethod) {
        auto savedDefers = deferredExprs;
        auto savedScopes = destructibleScopes;
        auto savedCatchTargets = catchTargets;
        deferredExprs.clear();
        catchTargets.clear();
        destructibleScopes.clear();
        destructibleScopes.push_back({}); // function-body frame
        currentFn = irFn;
        locals.clear();

        Block* entry = irFn->newBlock("entry");
        currentBlock = entry;

        if (isMethod) {
            locals["self"] = irFn->params[0];
            for (size_t i = 0; i < params.size(); ++i) {
                locals[params[i]->name] = irFn->params[i + 1];
            }
        } else {
            for (size_t i = 0; i < params.size(); ++i) {
                locals[params[i]->name] = irFn->params[i];
            }
        }

        for (const auto& stmt : body) {
            lowerStatement(*stmt);
        }

        if (currentBlock && !isTerminated(currentBlock)) {
            auto snapshot = snapshotActiveDestructors();
            if (irFn->returnType == Type::Void) {
                auto inst = std::make_unique<Instruction>(Instruction{Op::Ret, {}, VReg{-1, Type::Void}});
                retDestructors[inst.get()] = std::move(snapshot);
                emit(std::move(inst));
            } else {
                VReg zero = irFn->newVReg(irFn->returnType);
                emit(Instruction{Op::ConstInt, zero, {}, {}, 0});
                auto inst = std::make_unique<Instruction>(Instruction{Op::Ret, {}, zero});
                retDestructors[inst.get()] = std::move(snapshot);
                emit(std::move(inst));
            }
        }
        emitDeferredExits(irFn);
        deferredExprs = std::move(savedDefers);
        destructibleScopes = std::move(savedScopes);
        catchTargets = std::move(savedCatchTargets);
    }

    void lowerStatement(const ast::Statement& stmt);
    void emitDeferredExits(ir::Function* fn);
    void emitDestructorCalls(const std::vector<std::pair<std::string, std::string>>& objects);
    std::vector<std::pair<std::string, std::string>> snapshotActiveDestructors() const;
    void collectModuleConstants(const ast::Program& prog);
    static bool isConstInitializer(const ast::Expression& expr);
    static int64_t evalLiteralInit(const ast::Expression& expr);
    static int64_t evalConstExpr(const ast::Expression& expr, bool& ok);
    VReg lowerExpression(const ast::Expression& expr);
    void lowerIfStatement(const ast::IfStatement& s);
    void lowerWhileStatement(const ast::WhileStatement& s);
    void lowerForStatement(const ast::ForStatement& s);
    void lowerMatchStatement(const ast::MatchStatement& s);
    void lowerStreamCapture(const ast::StreamExpression& se);
    VReg lowerListConcat(VReg left, VReg right, Type elemType);
    VReg lowerListEq(VReg left, VReg right, Type elemType, bool negate);
    VReg lowerStringEq(VReg left, VReg right, bool negate);
    VReg lowerBoolToStr(VReg value);
    VReg emitIntConst(int64_t value);
    VReg emitResolveStr(VReg value);
    VReg emitByteLoad(VReg str, VReg index);
    void emitByteStore(VReg str, VReg index, VReg value);
    VReg emitSubstring(VReg str, VReg start, VReg count);
    VReg emitStringSearch(VReg haystack, VReg needle, VReg from);
    VReg emitStringTransform(VReg str, bool toUpper);
    VReg emitStringTrim(VReg str);
    VReg emitStringSplit(VReg str, VReg sep);
    VReg emitStringJoin(VReg list, VReg sep);
    void emitCopyString(VReg dst, VReg dstIndex, VReg src, VReg count);
};

void LoweringContext::lowerStatement(const ast::Statement& stmt) {
    if (auto* th = dynamic_cast<const ast::ThrowStatement*>(&stmt)) {
        VReg val = th->value ? lowerExpression(*th->value) : VReg{-1, Type::Void};
        if (val.id >= 0) {
            emit(Instruction{Op::SetErr, {}, val});
            std::string typeName = "string";
            auto it = checker.types().find(th->value.get());
            if (it != checker.types().end()) typeName = it->second->toString();
            Instruction tag;
            tag.op = Op::SetErrType;
            tag.intVal = typeTagHash(typeName);
            emit(std::move(tag));
        }
        auto snapshot = snapshotActiveDestructors();
        if (!catchTargets.empty()) {
            emitDestructorCalls(snapshot);
            Block* target = catchTargets.back();
            auto thIt = catchTypeHashes.find(target);
            if (thIt != catchTypeHashes.end()) {
                VReg tag = currentFn->newVReg(Type::I64);
                VReg want = currentFn->newVReg(Type::I64);
                VReg match = currentFn->newVReg(Type::I8);
                emit(Instruction{Op::GetErrType, tag});
                emit(Instruction{Op::ConstInt, want, {}, {}, thIt->second});
                emit(Instruction{Op::Eq, match, tag, want});
                Instruction branch;
                branch.op = Op::Branch;
                branch.src1 = match;
                branch.target = target;
                branch.elseTgt = currentFn->newBlock("unhandled");
                emit(std::move(branch));
            } else {
                Instruction jmp;
                jmp.op = Op::Jmp;
                jmp.target = target;
                emit(std::move(jmp));
            }
        } else {
            auto inst = std::make_unique<Instruction>(Instruction{Op::Ret, {}, VReg{-1, Type::Void}});
            retDestructors[inst.get()] = std::move(snapshot);
            emit(std::move(inst));
        }
        if (currentFn) currentBlock = currentFn->newBlock("after_throw");
        return;
    }
    if (auto* tr = dynamic_cast<const ast::TryStatement*>(&stmt)) {
        size_t firstTryBlock = currentFn->blocks.size();
        Block* tryBlock = currentFn->newBlock("try_body");
        Block* catchBlock = currentFn->newBlock("catch_body");
        Block* endBlock = currentFn->newBlock("try_end");
        Block* finallyBlock = tr->finallyBody ? currentFn->newBlock("finally_body") : nullptr;
        Instruction enter;
        enter.op = Op::Jmp;
        enter.target = tryBlock;
        emit(std::move(enter));

        if (!tr->catchType.empty()) catchTypeHashes[catchBlock] = typeTagHash(tr->catchType);
        catchTargets.push_back(catchBlock);
        currentBlock = tryBlock;
        if (tr->tryBody) lowerStatement(*tr->tryBody);
        Block* afterTry = currentBlock;

        // Insert an error check after every call made inside the try body.
        std::unordered_map<Block*, Block*> tails;
        for (size_t bi = firstTryBlock; bi < currentFn->blocks.size(); ++bi) {
            Block* b = currentFn->blocks[bi].get();
            if (b == catchBlock || b == endBlock || b == finallyBlock) continue;
            auto old = std::move(b->instructions);
            b->instructions.clear();
            Block* tail = b;
            for (auto& inst : old) {
                bool isCall = inst && (inst->op == Op::Call || inst->op == Op::CallPtr);
                tail->instructions.push_back(std::move(inst));
                if (!isCall) continue;
                VReg err = currentFn->newVReg(Type::I64);
                VReg zero = currentFn->newVReg(Type::I64);
                VReg cond = currentFn->newVReg(Type::I8);
                tail->instructions.push_back(std::make_unique<Instruction>(Instruction{Op::GetErr, err}));
                tail->instructions.push_back(std::make_unique<Instruction>(Instruction{Op::ConstInt, zero, {}, {}, 0}));
                tail->instructions.push_back(std::make_unique<Instruction>(Instruction{Op::Ne, cond, err, zero}));
                Block* cont = currentFn->newBlock("try_cont");
                if (tr->catchType.empty()) {
                    Instruction branch;
                    branch.op = Op::Branch;
                    branch.src1 = cond;
                    branch.target = catchBlock;
                    branch.elseTgt = cont;
                    tail->instructions.push_back(std::make_unique<Instruction>(std::move(branch)));
                } else {
                    // Typed catch: only enter the handler when the stored tag
                    // matches the declared exception type.
                    Block* typeCheck = currentFn->newBlock("type_check");
                    Instruction flagBranch;
                    flagBranch.op = Op::Branch;
                    flagBranch.src1 = cond;
                    flagBranch.target = typeCheck;
                    flagBranch.elseTgt = cont;
                    tail->instructions.push_back(std::make_unique<Instruction>(std::move(flagBranch)));

                    VReg tag = currentFn->newVReg(Type::I64);
                    VReg want = currentFn->newVReg(Type::I64);
                    VReg match = currentFn->newVReg(Type::I8);
                    typeCheck->instructions.push_back(std::make_unique<Instruction>(Instruction{Op::GetErrType, tag}));
                    typeCheck->instructions.push_back(std::make_unique<Instruction>(Instruction{Op::ConstInt, want, {}, {}, typeTagHash(tr->catchType)}));
                    typeCheck->instructions.push_back(std::make_unique<Instruction>(Instruction{Op::Eq, match, tag, want}));
                    Instruction typeBranch;
                    typeBranch.op = Op::Branch;
                    typeBranch.src1 = match;
                    typeBranch.target = catchBlock;
                    typeBranch.elseTgt = cont;
                    typeCheck->instructions.push_back(std::make_unique<Instruction>(std::move(typeBranch)));
                }
                tail = cont;
            }
            tails[b] = tail;
        }
        if (auto it = tails.find(afterTry); it != tails.end()) afterTry = it->second;
        currentBlock = afterTry;
        Instruction toEnd;
        toEnd.op = Op::Jmp;
        toEnd.target = finallyBlock ? finallyBlock : endBlock;
        emit(std::move(toEnd));

        // Catch handler: consume the raised value and clear the error slot.
        currentBlock = catchBlock;
        emit(Instruction{Op::ClearErr});
        if (!tr->catchParam.empty()) {
            VReg err = currentFn->newVReg(Type::Ptr);
            emit(Instruction{Op::GetErrValue, err});
            locals[tr->catchParam] = err;
        }
        if (tr->catchBody) lowerStatement(*tr->catchBody);
        catchTargets.pop_back();
        if (currentBlock && !isTerminated(currentBlock)) {
            Instruction je;
            je.op = Op::Jmp;
            je.target = finallyBlock ? finallyBlock : endBlock;
            emit(std::move(je));
        }
        if (finallyBlock) {
            currentBlock = finallyBlock;
            if (tr->finallyBody) lowerStatement(*tr->finallyBody);
            if (currentBlock && !isTerminated(currentBlock)) {
                Instruction jf;
                jf.op = Op::Jmp;
                jf.target = endBlock;
                emit(std::move(jf));
            }
        }
        currentBlock = endBlock;
        return;
    }
    if (auto* df = dynamic_cast<const ast::DeferStatement*>(&stmt)) {
        if (df->expression && !loweringDeferred) deferredExprs.push_back(df->expression);
        return;
    }
    if (auto* s = dynamic_cast<const ast::VarDeclaration*>(&stmt)) {
        if (auto* se = dynamic_cast<ast::StreamExpression*>(s->value.get())) {
            if (se->op == ">>>" || se->op == "?>>>") {
                if (se->op == "?>>>") {
                    VReg noErr = currentFn->newVReg(Type::Ptr);
                    emit(Instruction{Op::ConstInt, noErr, {}, {}, 0});
                    emit(Instruction{Op::SetErr, {}, noErr});
                }
                VReg valReg = lowerExpression(*se->source);
                if (auto* targetIdent = dynamic_cast<ast::Identifier*>(se->target.get())) {
                    auto it = locals.find(targetIdent->value);
                    VReg tReg;
                    if (it == locals.end()) {
                        tReg = currentFn->newVReg(Type::Ptr);
                        locals[targetIdent->value] = tReg;
                    } else {
                        tReg = it->second;
                    }
                    if (se->op == "?>>>") {
                        emit(Instruction{Op::GetErr, tReg});
                    } else {
                        emit(Instruction{Op::Copy, tReg, valReg});
                    }
                }
                VReg localReg = currentFn->newVReg(valReg.type);
                emit(Instruction{Op::Copy, localReg, valReg});
                locals[s->name->value] = localReg;
                return;
            }
        }

        VReg valReg = lowerExpression(*s->value);
        VReg localReg = currentFn->newVReg(valReg.type);
        emit(Instruction{Op::Copy, localReg, valReg});
        locals[s->name->value] = localReg;

        // Track locals whose type has a destructor, so they can be cleaned up
        // on every exit path of the function.
        if (s->name && s->value && !loweringDeferred) {
            std::string typeName;
            if (auto* si = dynamic_cast<ast::StructInstantiation*>(s->value.get())) {
                typeName = si->name;
            } else if (auto* ce = dynamic_cast<ast::CallExpression*>(s->value.get())) {
                if (auto* id = dynamic_cast<ast::Identifier*>(ce->function.get())) {
                    typeName = id->value;
                }
            }
            if (typeName.empty()) {
                auto it = checker.types().find(s->value.get());
                if (it != checker.types().end()) {
                    if (auto* st = dynamic_cast<typecheck::StructType*>(it->second.get())) {
                        typeName = st->name;
                    }
                }
            }
            if (destructors.count(typeName) && !destructibleScopes.empty()) {
                destructibleScopes.back().push_back({s->name->value, typeName});
            }
        }
    } else if (auto* s = dynamic_cast<const ast::ReturnStatement*>(&stmt)) {
        auto snapshot = snapshotActiveDestructors();
        if (s->value) {
            VReg valReg = lowerExpression(*s->value);
            auto inst = std::make_unique<Instruction>(Instruction{Op::Ret, {}, valReg});
            retDestructors[inst.get()] = std::move(snapshot);
            emit(std::move(inst));
        } else {
            auto inst = std::make_unique<Instruction>(Instruction{Op::Ret, {}, VReg{-1, Type::Void}});
            retDestructors[inst.get()] = std::move(snapshot);
            emit(std::move(inst));
        }
    } else if (auto* s = dynamic_cast<const ast::ExpressionStatement*>(&stmt)) {
        if (auto* se = dynamic_cast<ast::StreamExpression*>(s->expression.get())) {
            if (se->op == ">>>" || se->op == "?>>>") {
                lowerStreamCapture(*se);
                return;
            }
        }
        lowerExpression(*s->expression);
    } else if (auto* s = dynamic_cast<const ast::AssertStatement*>(&stmt)) {
        VReg condReg = lowerExpression(*s->condition);
        Block* okBlock = currentFn->newBlock("assert_ok");
        Block* failBlock = currentFn->newBlock("assert_fail");
        Instruction brInst;
        brInst.op = Op::Branch;
        brInst.src1 = condReg;
        brInst.target = okBlock;
        brInst.elseTgt = failBlock;
        emit(std::move(brInst));

        currentBlock = failBlock;
        VReg errCode = currentFn->newVReg(Type::I64);
        emit(Instruction{Op::ConstInt, errCode, {}, {}, 1});
        emit(Instruction{Op::Ret, {}, errCode});

        currentBlock = okBlock;
    } else if (auto* s = dynamic_cast<const ast::BlockStatement*>(&stmt)) {
        destructibleScopes.push_back({});
        for (const auto& inner : s->statements) {
            lowerStatement(*inner);
        }
        // Objects declared in this block are destroyed at its end, in reverse
        // construction order (whether the block falls through or returns).
        auto frame = std::move(destructibleScopes.back());
        destructibleScopes.pop_back();
        emitDestructorCalls(frame);
    } else if (auto* s = dynamic_cast<const ast::IfStatement*>(&stmt)) {
        lowerIfStatement(*s);
    } else if (auto* s = dynamic_cast<const ast::WhileStatement*>(&stmt)) {
        lowerWhileStatement(*s);
    } else if (auto* s = dynamic_cast<const ast::LoopStatement*>(&stmt)) {
        Block* bodyBlock = currentFn->newBlock("loop_body");
        Block* afterBlock = currentFn->newBlock("loop_after");

        Instruction jmpEntry;
        jmpEntry.op = Op::Jmp;
        jmpEntry.target = bodyBlock;
        emit(std::move(jmpEntry));

        loopContinue.push_back(bodyBlock);
        loopBreak.push_back(afterBlock);

        currentBlock = bodyBlock;
        if (s->body) {
            lowerStatement(*s->body);
        }
        if (!isTerminated(currentBlock)) {
            Instruction jmpLoop;
            jmpLoop.op = Op::Jmp;
            jmpLoop.target = bodyBlock;
            emit(std::move(jmpLoop));
        }

        loopContinue.pop_back();
        loopBreak.pop_back();
        currentBlock = afterBlock;
    } else if (auto* s = dynamic_cast<const ast::ForStatement*>(&stmt)) {
        lowerForStatement(*s);
    } else if (auto* s = dynamic_cast<const ast::MatchStatement*>(&stmt)) {
        lowerMatchStatement(*s);
    } else if (dynamic_cast<const ast::BreakStatement*>(&stmt)) {
        if (loopBreak.empty()) {
            throw std::runtime_error("break outside of loop");
        }
        Block* target = loopBreak.back();
        Instruction jmpInst;
        jmpInst.op = Op::Jmp;
        jmpInst.target = target;
        emit(std::move(jmpInst));
        currentBlock = currentFn->newBlock("after_break");
    } else if (dynamic_cast<const ast::ContinueStatement*>(&stmt)) {
        if (loopContinue.empty()) {
            throw std::runtime_error("continue outside of loop");
        }
        Block* target = loopContinue.back();
        Instruction jmpInst;
        jmpInst.op = Op::Jmp;
        jmpInst.target = target;
        emit(std::move(jmpInst));
        currentBlock = currentFn->newBlock("after_continue");
    } else if (dynamic_cast<const ast::EnumDeclaration*>(&stmt) ||
               dynamic_cast<const ast::StructDeclaration*>(&stmt) ||
               dynamic_cast<const ast::ClassDeclaration*>(&stmt) ||
               dynamic_cast<const ast::UseStatement*>(&stmt) ||
               dynamic_cast<const ast::FuncDeclaration*>(&stmt)) {
        // Handled in layout / declaration passes or imports
    } else {
        throw std::runtime_error("statement not supported yet in native backend");
    }
}

void LoweringContext::lowerStreamCapture(const ast::StreamExpression& se) {
    auto* id = dynamic_cast<ast::Identifier*>(se.target.get());
    if (!id) {
        throw std::runtime_error("capture stream target must be variable identifier");
    }

    if (se.op == "?>>>") {
        VReg noErr = currentFn->newVReg(Type::Ptr);
        emit(Instruction{Op::ConstInt, noErr, {}, {}, 0});
        emit(Instruction{Op::SetErr, {}, noErr});
    }
    VReg resReg = lowerExpression(*se.source);
    if (se.op == ">>>") {
        auto it = locals.find(id->value);
        VReg targetReg;
        if (it == locals.end()) {
            targetReg = currentFn->newVReg(resReg.type);
            locals[id->value] = targetReg;
        } else {
            targetReg = it->second;
        }
        emit(Instruction{Op::Copy, targetReg, resReg});
        return;
    }

    // ?>>>
    auto it = locals.find(id->value);
    VReg targetReg;
    if (it == locals.end()) {
        targetReg = currentFn->newVReg(Type::Ptr);
        locals[id->value] = targetReg;
    } else {
        targetReg = it->second;
    }
    emit(Instruction{Op::GetErr, targetReg});
}

void LoweringContext::lowerIfStatement(const ast::IfStatement& s) {
    VReg condReg = lowerExpression(*s.condition);
    Block* thenBlock = currentFn->newBlock("if_then");
    Block* elseBlock = currentFn->newBlock("if_else");
    Block* mergeBlock = currentFn->newBlock("if_merge");

    Instruction brInst;
    brInst.op = Op::Branch;
    brInst.src1 = condReg;
    brInst.target = thenBlock;
    brInst.elseTgt = elseBlock;
    emit(std::move(brInst));

    currentBlock = thenBlock;
    if (s.consequence) {
        lowerStatement(*s.consequence);
    }
    if (!isTerminated(currentBlock)) {
        Instruction jmpInst;
        jmpInst.op = Op::Jmp;
        jmpInst.target = mergeBlock;
        emit(std::move(jmpInst));
    }

    currentBlock = elseBlock;
    if (!s.alternatives.empty()) {
        ast::IfStatement nestedIf;
        nestedIf.token = s.alternatives[0]->token;
        nestedIf.condition = s.alternatives[0]->condition;
        nestedIf.consequence = s.alternatives[0]->body;
        for (size_t i = 1; i < s.alternatives.size(); ++i) {
            nestedIf.alternatives.push_back(s.alternatives[i]);
        }
        nestedIf.elseBlock = s.elseBlock;

        lowerIfStatement(nestedIf);
    } else if (s.elseBlock) {
        lowerStatement(*s.elseBlock);
    }

    if (!isTerminated(currentBlock)) {
        Instruction jmpInst;
        jmpInst.op = Op::Jmp;
        jmpInst.target = mergeBlock;
        emit(std::move(jmpInst));
    }

    currentBlock = mergeBlock;
}

void LoweringContext::lowerWhileStatement(const ast::WhileStatement& s) {
    Block* condBlock = currentFn->newBlock("while_cond");
    Block* bodyBlock = currentFn->newBlock("while_body");
    Block* afterBlock = currentFn->newBlock("while_after");

    Instruction jmpEntry;
    jmpEntry.op = Op::Jmp;
    jmpEntry.target = condBlock;
    emit(std::move(jmpEntry));

    currentBlock = condBlock;
    VReg condReg = lowerExpression(*s.condition);
    Instruction brInst;
    brInst.op = Op::Branch;
    brInst.src1 = condReg;
    brInst.target = bodyBlock;
    brInst.elseTgt = afterBlock;
    emit(std::move(brInst));

    loopContinue.push_back(condBlock);
    loopBreak.push_back(afterBlock);

    currentBlock = bodyBlock;
    if (s.body) {
        lowerStatement(*s.body);
    }
    if (!isTerminated(currentBlock)) {
        Instruction jmpLoop;
        jmpLoop.op = Op::Jmp;
        jmpLoop.target = condBlock;
        emit(std::move(jmpLoop));
    }

    loopContinue.pop_back();
    loopBreak.pop_back();

    currentBlock = afterBlock;
}

void LoweringContext::lowerForStatement(const ast::ForStatement& s) {
    if (auto* rangeLit = dynamic_cast<ast::RangeLiteral*>(s.iterable.get())) {
        VReg startReg = lowerExpression(*rangeLit->start);
        VReg endReg = lowerExpression(*rangeLit->end);

        VReg iReg = currentFn->newVReg(Type::I64);
        emit(Instruction{Op::Copy, iReg, startReg});
        locals[s.variable] = iReg;

        Block* condBlock = currentFn->newBlock("for_cond");
        Block* bodyBlock = currentFn->newBlock("for_body");
        Block* stepBlock = currentFn->newBlock("for_step");
        Block* afterBlock = currentFn->newBlock("for_after");

        Instruction jmpEntry;
        jmpEntry.op = Op::Jmp;
        jmpEntry.target = condBlock;
        emit(std::move(jmpEntry));

        currentBlock = condBlock;
        VReg cmpReg = currentFn->newVReg(Type::I8);
        Op cmpOp = rangeLit->inclusive ? Op::Le : Op::Lt;
        emit(Instruction{cmpOp, cmpReg, iReg, endReg});

        Instruction brInst;
        brInst.op = Op::Branch;
        brInst.src1 = cmpReg;
        brInst.target = bodyBlock;
        brInst.elseTgt = afterBlock;
        emit(std::move(brInst));

        loopContinue.push_back(stepBlock);
        loopBreak.push_back(afterBlock);

        currentBlock = bodyBlock;
        if (s.body) {
            lowerStatement(*s.body);
        }
        if (!isTerminated(currentBlock)) {
            Instruction jmpStep;
            jmpStep.op = Op::Jmp;
            jmpStep.target = stepBlock;
            emit(std::move(jmpStep));
        }

        currentBlock = stepBlock;
        VReg oneReg = currentFn->newVReg(Type::I64);
        emit(Instruction{Op::ConstInt, oneReg, {}, {}, 1});
        VReg nextIReg = currentFn->newVReg(Type::I64);
        emit(Instruction{Op::Add, nextIReg, iReg, oneReg});
        emit(Instruction{Op::Copy, iReg, nextIReg});

        Instruction jmpCond;
        jmpCond.op = Op::Jmp;
        jmpCond.target = condBlock;
        emit(std::move(jmpCond));

        loopContinue.pop_back();
        loopBreak.pop_back();

        currentBlock = afterBlock;
        return;
    }

    // For loop over list
    VReg listReg = lowerExpression(*s.iterable);
    Type elemIRType = Type::I64;
    const auto& chkTypes = checker.types();
    auto itT = chkTypes.find(s.iterable.get());
    if (itT != chkTypes.end()) {
        if (auto* lt = dynamic_cast<typecheck::ListType*>(itT->second.get())) {
            elemIRType = typeToIR(lt->elem());
        }
    }

    // Read len: load listReg[8]
    VReg lenReg = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Load, lenReg, listReg, {}, 8});

    // Read dataPtr: load listReg[0]
    VReg dataPtrReg = currentFn->newVReg(Type::Ptr);
    emit(Instruction{Op::Load, dataPtrReg, listReg, {}, 0});

    VReg idxReg = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::ConstInt, idxReg, {}, {}, 0});

    VReg elemReg = currentFn->newVReg(elemIRType);
    locals[s.variable] = elemReg;

    Block* condBlock = currentFn->newBlock("for_list_cond");
    Block* bodyBlock = currentFn->newBlock("for_list_body");
    Block* stepBlock = currentFn->newBlock("for_list_step");
    Block* afterBlock = currentFn->newBlock("for_list_after");

    Instruction jmpEntry;
    jmpEntry.op = Op::Jmp;
    jmpEntry.target = condBlock;
    emit(std::move(jmpEntry));

    currentBlock = condBlock;
    VReg cmpReg = currentFn->newVReg(Type::I8);
    emit(Instruction{Op::Lt, cmpReg, idxReg, lenReg});

    Instruction brInst;
    brInst.op = Op::Branch;
    brInst.src1 = cmpReg;
    brInst.target = bodyBlock;
    brInst.elseTgt = afterBlock;
    emit(std::move(brInst));

    loopContinue.push_back(stepBlock);
    loopBreak.push_back(afterBlock);

    currentBlock = bodyBlock;
    VReg scaleReg = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::ConstInt, scaleReg, {}, {}, 8});
    VReg byteOffReg = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Mul, byteOffReg, idxReg, scaleReg});
    VReg itemAddrReg = currentFn->newVReg(Type::Ptr);
    emit(Instruction{Op::Add, itemAddrReg, dataPtrReg, byteOffReg});
    emit(Instruction{Op::Load, elemReg, itemAddrReg, {}, 0});

    if (s.body) {
        lowerStatement(*s.body);
    }
    if (!isTerminated(currentBlock)) {
        Instruction jmpStep;
        jmpStep.op = Op::Jmp;
        jmpStep.target = stepBlock;
        emit(std::move(jmpStep));
    }

    currentBlock = stepBlock;
    VReg oneReg = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::ConstInt, oneReg, {}, {}, 1});
    VReg nextIdxReg = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Add, nextIdxReg, idxReg, oneReg});
    emit(Instruction{Op::Copy, idxReg, nextIdxReg});

    Instruction jmpCond;
    jmpCond.op = Op::Jmp;
    jmpCond.target = condBlock;
    emit(std::move(jmpCond));

    loopContinue.pop_back();
    loopBreak.pop_back();

    currentBlock = afterBlock;
}

void LoweringContext::lowerMatchStatement(const ast::MatchStatement& s) {
    tmpCounter++;
    std::string tmpName = "@match_" + std::to_string(tmpCounter);

    VReg subjReg = lowerExpression(*s.subject);
    locals[tmpName] = subjReg;

    Block* afterMatch = currentFn->newBlock("match_after");

    for (const auto& cs : s.cases) {
        Block* caseTestBlock = currentBlock;
        Block* caseBodyBlock = currentFn->newBlock("match_case_body");
        Block* nextCaseBlock = currentFn->newBlock("match_case_next");

        currentBlock = caseTestBlock;
        VReg patReg = lowerExpression(*cs->pattern);
        VReg cmpReg = currentFn->newVReg(Type::I8);
        emit(Instruction{Op::Eq, cmpReg, subjReg, patReg});

        Instruction brInst;
        brInst.op = Op::Branch;
        brInst.src1 = cmpReg;
        brInst.target = caseBodyBlock;
        brInst.elseTgt = nextCaseBlock;
        emit(std::move(brInst));

        currentBlock = caseBodyBlock;
        if (cs->body) {
            lowerStatement(*cs->body);
        }
        if (!isTerminated(currentBlock)) {
            Instruction jmpAfter;
            jmpAfter.op = Op::Jmp;
            jmpAfter.target = afterMatch;
            emit(std::move(jmpAfter));
        }

        currentBlock = nextCaseBlock;
    }

    if (s.defaultCase) {
        lowerStatement(*s.defaultCase);
    }
    if (!isTerminated(currentBlock)) {
        Instruction jmpAfter;
        jmpAfter.op = Op::Jmp;
        jmpAfter.target = afterMatch;
        emit(std::move(jmpAfter));
    }

    currentBlock = afterMatch;
}

VReg LoweringContext::lowerListConcat(VReg left, VReg right, Type elemType) {
    VReg leftLen = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Load, leftLen, left, {}, 8});
    VReg rightLen = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Load, rightLen, right, {}, 8});
    VReg totalLen = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Add, totalLen, leftLen, rightLen});

    VReg elemSize = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::ConstInt, elemSize, {}, {}, 8});
    VReg dataBytes = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Mul, dataBytes, totalLen, elemSize});
    VReg data = currentFn->newVReg(Type::Ptr);
    emit(Instruction{Op::Alloc, data, dataBytes});

    auto copySource = [&](VReg list, VReg count, VReg dstIndex) {
        VReg srcData = currentFn->newVReg(Type::Ptr);
        emit(Instruction{Op::Load, srcData, list, {}, 0});
        VReg idx = currentFn->newVReg(Type::I64);
        emit(Instruction{Op::ConstInt, idx, {}, {}, 0});

        Block* cond = currentFn->newBlock("list_concat_cond");
        Block* body = currentFn->newBlock("list_concat_body");
        Block* done = currentFn->newBlock("list_concat_done");
        Instruction toCond;
        toCond.op = Op::Jmp;
        toCond.target = cond;
        emit(std::move(toCond));

        currentBlock = cond;
        VReg more = currentFn->newVReg(Type::I8);
        emit(Instruction{Op::Lt, more, idx, count});
        Instruction br;
        br.op = Op::Branch;
        br.src1 = more;
        br.target = body;
        br.elseTgt = done;
        emit(std::move(br));

        currentBlock = body;
        VReg off = currentFn->newVReg(Type::I64);
        emit(Instruction{Op::Mul, off, idx, elemSize});
        VReg srcAddr = currentFn->newVReg(Type::Ptr);
        emit(Instruction{Op::Add, srcAddr, srcData, off});
        VReg value = currentFn->newVReg(elemType);
        emit(Instruction{Op::Load, value, srcAddr, {}, 0});

        VReg dstOff = currentFn->newVReg(Type::I64);
        emit(Instruction{Op::Mul, dstOff, dstIndex, elemSize});
        VReg dstAddr = currentFn->newVReg(Type::Ptr);
        emit(Instruction{Op::Add, dstAddr, data, dstOff});
        emit(Instruction{Op::Store, {}, dstAddr, value, 0});

        VReg one = currentFn->newVReg(Type::I64);
        emit(Instruction{Op::ConstInt, one, {}, {}, 1});
        VReg nextSrc = currentFn->newVReg(Type::I64);
        emit(Instruction{Op::Add, nextSrc, idx, one});
        emit(Instruction{Op::Copy, idx, nextSrc});
        VReg nextDst = currentFn->newVReg(Type::I64);
        emit(Instruction{Op::Add, nextDst, dstIndex, one});
        emit(Instruction{Op::Copy, dstIndex, nextDst});

        Instruction back;
        back.op = Op::Jmp;
        back.target = cond;
        emit(std::move(back));

        currentBlock = done;
    };

    VReg dstIndex = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::ConstInt, dstIndex, {}, {}, 0});
    copySource(left, leftLen, dstIndex);
    copySource(right, rightLen, dstIndex);

    VReg headerBytes = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::ConstInt, headerBytes, {}, {}, 24});
    VReg header = currentFn->newVReg(Type::Ptr);
    emit(Instruction{Op::Alloc, header, headerBytes});
    emit(Instruction{Op::Store, {}, header, data, 0});
    emit(Instruction{Op::Store, {}, header, totalLen, 8});
    emit(Instruction{Op::Store, {}, header, totalLen, 16});
    return header;
}

VReg LoweringContext::lowerBoolToStr(VReg value) {
    VReg dst = currentFn->newVReg(Type::Ptr);
    Block* trueBlock = currentFn->newBlock("bool_true");
    Block* falseBlock = currentFn->newBlock("bool_false");
    Block* doneBlock = currentFn->newBlock("bool_done");

    Instruction br;
    br.op = Op::Branch;
    br.src1 = value;
    br.target = trueBlock;
    br.elseTgt = falseBlock;
    emit(std::move(br));

    auto emitText = [&](const char* text) {
        VReg str = currentFn->newVReg(Type::Ptr);
        Instruction cs;
        cs.op = Op::ConstStr;
        cs.dst = str;
        cs.strVal = text;
        emit(std::move(cs));
        emit(Instruction{Op::Copy, dst, str});
        Instruction jmp;
        jmp.op = Op::Jmp;
        jmp.target = doneBlock;
        emit(std::move(jmp));
    };

    currentBlock = trueBlock;
    emitText("true");
    currentBlock = falseBlock;
    emitText("false");
    currentBlock = doneBlock;
    return dst;
}

VReg LoweringContext::emitIntConst(int64_t value) {
    VReg reg = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::ConstInt, reg, {}, {}, value});
    return reg;
}

VReg LoweringContext::emitResolveStr(VReg value) {
    VReg resolved = currentFn->newVReg(Type::Ptr);
    Instruction call;
    call.op = Op::Call;
    call.dst = resolved;
    call.callName = "ulang_str_ptr";
    call.args = {value};
    emit(std::move(call));
    return resolved;
}

VReg LoweringContext::emitByteLoad(VReg str, VReg index) {
    VReg addr = currentFn->newVReg(Type::Ptr);
    emit(Instruction{Op::Add, addr, str, index});
    VReg byte = currentFn->newVReg(Type::I8);
    emit(Instruction{Op::Load, byte, addr, {}, 8});
    return byte;
}

void LoweringContext::emitByteStore(VReg str, VReg index, VReg value) {
    VReg addr = currentFn->newVReg(Type::Ptr);
    emit(Instruction{Op::Add, addr, str, index});
    emit(Instruction{Op::Store, {}, addr, value, 8});
}

VReg LoweringContext::emitSubstring(VReg str, VReg start, VReg count) {
    str = emitResolveStr(str);
    VReg allocSize = currentFn->newVReg(Type::I64);
    VReg overhead = emitIntConst(9);
    emit(Instruction{Op::Add, allocSize, count, overhead});
    VReg buffer = currentFn->newVReg(Type::Ptr);
    emit(Instruction{Op::Alloc, buffer, allocSize});
    emit(Instruction{Op::Store, {}, buffer, count, 0});

    VReg index = currentFn->newVReg(Type::I64);
    VReg zero = emitIntConst(0);
    VReg one = emitIntConst(1);
    emit(Instruction{Op::Copy, index, zero});

    Block* condBlock = currentFn->newBlock("substr_cond");
    Block* bodyBlock = currentFn->newBlock("substr_body");
    Block* doneBlock = currentFn->newBlock("substr_done");
    Instruction toCond;
    toCond.op = Op::Jmp;
    toCond.target = condBlock;
    emit(std::move(toCond));

    currentBlock = condBlock;
    VReg more = currentFn->newVReg(Type::I8);
    emit(Instruction{Op::Lt, more, index, count});
    Instruction condBranch;
    condBranch.op = Op::Branch;
    condBranch.src1 = more;
    condBranch.target = bodyBlock;
    condBranch.elseTgt = doneBlock;
    emit(std::move(condBranch));

    currentBlock = bodyBlock;
    VReg sourceIndex = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Add, sourceIndex, start, index});
    VReg byte = emitByteLoad(str, sourceIndex);
    emitByteStore(buffer, index, byte);
    VReg next = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Add, next, index, one});
    emit(Instruction{Op::Copy, index, next});
    Instruction back;
    back.op = Op::Jmp;
    back.target = condBlock;
    emit(std::move(back));

    currentBlock = doneBlock;
    VReg endAddr = currentFn->newVReg(Type::Ptr);
    emit(Instruction{Op::Add, endAddr, buffer, count});
    VReg nul = currentFn->newVReg(Type::I8);
    VReg byteZero = emitIntConst(0);
    emit(Instruction{Op::Copy, nul, byteZero});
    emit(Instruction{Op::Store, {}, endAddr, nul, 8});
    return buffer;
}

VReg LoweringContext::emitStringSearch(VReg haystack, VReg needle, VReg from) {
    haystack = emitResolveStr(haystack);
    needle = emitResolveStr(needle);
    VReg result = currentFn->newVReg(Type::I64);
    VReg minusOne = emitIntConst(-1);
    emit(Instruction{Op::Copy, result, minusOne});

    VReg hayLen = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Load, hayLen, haystack, {}, 0});
    VReg needleLen = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Load, needleLen, needle, {}, 0});
    VReg zero = emitIntConst(0);
    VReg one = emitIntConst(1);
    VReg index = currentFn->newVReg(Type::I64);
    VReg offset = currentFn->newVReg(Type::I64);

    Block* checkLimit = currentFn->newBlock("str_find_limit");
    Block* outerInit = currentFn->newBlock("str_find_outer_init");
    Block* outerCond = currentFn->newBlock("str_find_outer_cond");
    Block* innerInit = currentFn->newBlock("str_find_inner_init");
    Block* innerCond = currentFn->newBlock("str_find_inner_cond");
    Block* innerBody = currentFn->newBlock("str_find_inner_body");
    Block* innerNext = currentFn->newBlock("str_find_inner_next");
    Block* found = currentFn->newBlock("str_find_found");
    Block* mismatch = currentFn->newBlock("str_find_mismatch");
    Block* done = currentFn->newBlock("str_find_done");

    VReg fromValid = currentFn->newVReg(Type::I8);
    emit(Instruction{Op::Ge, fromValid, from, zero});
    Instruction brFrom;
    brFrom.op = Op::Branch;
    brFrom.src1 = fromValid;
    brFrom.target = checkLimit;
    brFrom.elseTgt = done;
    emit(std::move(brFrom));

    currentBlock = checkLimit;
    VReg limit = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Sub, limit, hayLen, needleLen});
    VReg limitValid = currentFn->newVReg(Type::I8);
    emit(Instruction{Op::Le, limitValid, from, limit});
    Instruction brLimit;
    brLimit.op = Op::Branch;
    brLimit.src1 = limitValid;
    brLimit.target = outerInit;
    brLimit.elseTgt = done;
    emit(std::move(brLimit));

    currentBlock = outerInit;
    emit(Instruction{Op::Copy, index, from});
    Instruction toOuter;
    toOuter.op = Op::Jmp;
    toOuter.target = outerCond;
    emit(std::move(toOuter));

    currentBlock = outerCond;
    VReg outerMore = currentFn->newVReg(Type::I8);
    emit(Instruction{Op::Le, outerMore, index, limit});
    Instruction brOuter;
    brOuter.op = Op::Branch;
    brOuter.src1 = outerMore;
    brOuter.target = innerInit;
    brOuter.elseTgt = done;
    emit(std::move(brOuter));

    currentBlock = innerInit;
    emit(Instruction{Op::Copy, offset, zero});
    Instruction toInner;
    toInner.op = Op::Jmp;
    toInner.target = innerCond;
    emit(std::move(toInner));

    currentBlock = innerCond;
    VReg innerMore = currentFn->newVReg(Type::I8);
    emit(Instruction{Op::Lt, innerMore, offset, needleLen});
    Instruction brInner;
    brInner.op = Op::Branch;
    brInner.src1 = innerMore;
    brInner.target = innerBody;
    brInner.elseTgt = found;
    emit(std::move(brInner));

    currentBlock = innerBody;
    VReg hayIndex = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Add, hayIndex, index, offset});
    VReg hayByte = emitByteLoad(haystack, hayIndex);
    VReg needleByte = emitByteLoad(needle, offset);
    VReg equal = currentFn->newVReg(Type::I8);
    emit(Instruction{Op::Eq, equal, hayByte, needleByte});
    Instruction brEqual;
    brEqual.op = Op::Branch;
    brEqual.src1 = equal;
    brEqual.target = innerNext;
    brEqual.elseTgt = mismatch;
    emit(std::move(brEqual));

    currentBlock = innerNext;
    VReg nextOffset = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Add, nextOffset, offset, one});
    emit(Instruction{Op::Copy, offset, nextOffset});
    Instruction backInner;
    backInner.op = Op::Jmp;
    backInner.target = innerCond;
    emit(std::move(backInner));

    currentBlock = mismatch;
    VReg nextIndex = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Add, nextIndex, index, one});
    emit(Instruction{Op::Copy, index, nextIndex});
    Instruction backOuter;
    backOuter.op = Op::Jmp;
    backOuter.target = outerCond;
    emit(std::move(backOuter));

    currentBlock = found;
    emit(Instruction{Op::Copy, result, index});
    Instruction toDone;
    toDone.op = Op::Jmp;
    toDone.target = done;
    emit(std::move(toDone));

    currentBlock = done;
    return result;
}

VReg LoweringContext::emitStringTransform(VReg str, bool toUpper) {
    str = emitResolveStr(str);
    VReg length = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Load, length, str, {}, 0});
    VReg allocSize = currentFn->newVReg(Type::I64);
    VReg overhead = emitIntConst(9);
    emit(Instruction{Op::Add, allocSize, length, overhead});
    VReg buffer = currentFn->newVReg(Type::Ptr);
    emit(Instruction{Op::Alloc, buffer, allocSize});
    emit(Instruction{Op::Store, {}, buffer, length, 0});

    VReg index = currentFn->newVReg(Type::I64);
    VReg zero = emitIntConst(0);
    VReg one = emitIntConst(1);
    VReg caseDelta = emitIntConst(32);
    VReg lowerA = emitIntConst('a');
    VReg lowerZ = emitIntConst('z');
    VReg upperA = emitIntConst('A');
    VReg upperZ = emitIntConst('Z');
    emit(Instruction{Op::Copy, index, zero});

    Block* condBlock = currentFn->newBlock("str_xform_cond");
    Block* bodyBlock = currentFn->newBlock("str_xform_body");
    Block* checkUpper = currentFn->newBlock("str_xform_check_upper");
    Block* checkLower = currentFn->newBlock("str_xform_check_lower");
    Block* convertBlock = currentFn->newBlock("str_xform_convert");
    Block* storeBlock = currentFn->newBlock("str_xform_store");
    Block* doneBlock = currentFn->newBlock("str_xform_done");

    Instruction toCond;
    toCond.op = Op::Jmp;
    toCond.target = condBlock;
    emit(std::move(toCond));

    currentBlock = condBlock;
    VReg more = currentFn->newVReg(Type::I8);
    emit(Instruction{Op::Lt, more, index, length});
    Instruction brCond;
    brCond.op = Op::Branch;
    brCond.src1 = more;
    brCond.target = bodyBlock;
    brCond.elseTgt = doneBlock;
    emit(std::move(brCond));

    currentBlock = bodyBlock;
    VReg byte = emitByteLoad(str, index);
    VReg byteReg = currentFn->newVReg(Type::I8);
    emit(Instruction{Op::Copy, byteReg, byte});
    if (toUpper) {
        VReg ge = currentFn->newVReg(Type::I8);
        emit(Instruction{Op::Ge, ge, byteReg, lowerA});
        Instruction brGe;
        brGe.op = Op::Branch;
        brGe.src1 = ge;
        brGe.target = checkUpper;
        brGe.elseTgt = storeBlock;
        emit(std::move(brGe));
    } else {
        VReg ge = currentFn->newVReg(Type::I8);
        emit(Instruction{Op::Ge, ge, byteReg, upperA});
        Instruction brGe;
        brGe.op = Op::Branch;
        brGe.src1 = ge;
        brGe.target = checkLower;
        brGe.elseTgt = storeBlock;
        emit(std::move(brGe));
    }

    currentBlock = toUpper ? checkUpper : checkLower;
    VReg le = currentFn->newVReg(Type::I8);
    if (toUpper) emit(Instruction{Op::Le, le, byteReg, lowerZ});
    else emit(Instruction{Op::Le, le, byteReg, upperZ});
    Instruction brLe;
    brLe.op = Op::Branch;
    brLe.src1 = le;
    brLe.target = convertBlock;
    brLe.elseTgt = storeBlock;
    emit(std::move(brLe));

    currentBlock = convertBlock;
    VReg converted = currentFn->newVReg(Type::I8);
    if (toUpper) emit(Instruction{Op::Sub, converted, byteReg, caseDelta});
    else emit(Instruction{Op::Add, converted, byteReg, caseDelta});
    emit(Instruction{Op::Copy, byteReg, converted});
    Instruction toStore;
    toStore.op = Op::Jmp;
    toStore.target = storeBlock;
    emit(std::move(toStore));

    currentBlock = storeBlock;
    emitByteStore(buffer, index, byteReg);
    VReg next = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Add, next, index, one});
    emit(Instruction{Op::Copy, index, next});
    Instruction back;
    back.op = Op::Jmp;
    back.target = condBlock;
    emit(std::move(back));

    currentBlock = doneBlock;
    VReg endAddr = currentFn->newVReg(Type::Ptr);
    emit(Instruction{Op::Add, endAddr, buffer, length});
    VReg nul = currentFn->newVReg(Type::I8);
    VReg byteZero = emitIntConst(0);
    emit(Instruction{Op::Copy, nul, byteZero});
    emit(Instruction{Op::Store, {}, endAddr, nul, 8});
    return buffer;
}

VReg LoweringContext::emitStringTrim(VReg str) {
    str = emitResolveStr(str);
    VReg length = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Load, length, str, {}, 0});
    VReg zero = emitIntConst(0);
    VReg one = emitIntConst(1);
    VReg space = emitIntConst(' ');
    VReg first = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Copy, first, zero});

    Block* firstCond = currentFn->newBlock("trim_first_cond");
    Block* firstBody = currentFn->newBlock("trim_first_body");
    Block* firstDone = currentFn->newBlock("trim_first_done");
    Instruction toFirst;
    toFirst.op = Op::Jmp;
    toFirst.target = firstCond;
    emit(std::move(toFirst));

    currentBlock = firstCond;
    VReg firstMore = currentFn->newVReg(Type::I8);
    emit(Instruction{Op::Lt, firstMore, first, length});
    Instruction brFirst;
    brFirst.op = Op::Branch;
    brFirst.src1 = firstMore;
    brFirst.target = firstBody;
    brFirst.elseTgt = firstDone;
    emit(std::move(brFirst));

    currentBlock = firstBody;
    VReg firstByte = emitByteLoad(str, first);
    VReg firstSpace = currentFn->newVReg(Type::I8);
    emit(Instruction{Op::Eq, firstSpace, firstByte, space});
    Block* firstAdvance = currentFn->newBlock("trim_first_advance");
    Instruction brFirstSpace;
    brFirstSpace.op = Op::Branch;
    brFirstSpace.src1 = firstSpace;
    brFirstSpace.target = firstAdvance;
    brFirstSpace.elseTgt = firstDone;
    emit(std::move(brFirstSpace));

    currentBlock = firstAdvance;
    VReg nextFirst = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Add, nextFirst, first, one});
    emit(Instruction{Op::Copy, first, nextFirst});
    Instruction backFirst;
    backFirst.op = Op::Jmp;
    backFirst.target = firstCond;
    emit(std::move(backFirst));

    currentBlock = firstDone;
    VReg last = currentFn->newVReg(Type::I64);
    VReg lastStart = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Sub, lastStart, length, one});
    emit(Instruction{Op::Copy, last, lastStart});

    Block* lastCond = currentFn->newBlock("trim_last_cond");
    Block* lastBody = currentFn->newBlock("trim_last_body");
    Block* lastDone = currentFn->newBlock("trim_last_done");
    Block* emptyDone = currentFn->newBlock("trim_empty");
    Instruction toLast;
    toLast.op = Op::Jmp;
    toLast.target = lastCond;
    emit(std::move(toLast));

    currentBlock = lastCond;
    VReg lastValid = currentFn->newVReg(Type::I8);
    emit(Instruction{Op::Ge, lastValid, last, first});
    Instruction brLast;
    brLast.op = Op::Branch;
    brLast.src1 = lastValid;
    brLast.target = lastBody;
    brLast.elseTgt = emptyDone;
    emit(std::move(brLast));

    currentBlock = lastBody;
    VReg lastByte = emitByteLoad(str, last);
    VReg lastSpace = currentFn->newVReg(Type::I8);
    emit(Instruction{Op::Eq, lastSpace, lastByte, space});
    Block* lastAdvance = currentFn->newBlock("trim_last_advance");
    Instruction brLastSpace;
    brLastSpace.op = Op::Branch;
    brLastSpace.src1 = lastSpace;
    brLastSpace.target = lastAdvance;
    brLastSpace.elseTgt = lastDone;
    emit(std::move(brLastSpace));

    currentBlock = lastAdvance;
    VReg prevLast = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Sub, prevLast, last, one});
    emit(Instruction{Op::Copy, last, prevLast});
    Instruction backLast;
    backLast.op = Op::Jmp;
    backLast.target = lastCond;
    emit(std::move(backLast));

    currentBlock = lastDone;
    VReg count = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Sub, count, last, first});
    emit(Instruction{Op::Add, count, count, one});
    VReg result = emitSubstring(str, first, count);
    Block* finish = currentFn->newBlock("trim_finish");
    Instruction toFinish;
    toFinish.op = Op::Jmp;
    toFinish.target = finish;
    emit(std::move(toFinish));

    currentBlock = emptyDone;
    VReg empty = emitSubstring(str, zero, zero);
    emit(Instruction{Op::Copy, result, empty});
    Instruction toFinishEmpty;
    toFinishEmpty.op = Op::Jmp;
    toFinishEmpty.target = finish;
    emit(std::move(toFinishEmpty));

    currentBlock = finish;
    return result;
}

void LoweringContext::emitCopyString(VReg dst, VReg dstIndex, VReg src, VReg count) {
    VReg index = currentFn->newVReg(Type::I64);
    VReg zero = emitIntConst(0);
    VReg one = emitIntConst(1);
    emit(Instruction{Op::Copy, index, zero});

    Block* condBlock = currentFn->newBlock("copy_cond");
    Block* bodyBlock = currentFn->newBlock("copy_body");
    Block* doneBlock = currentFn->newBlock("copy_done");
    Instruction toCond;
    toCond.op = Op::Jmp;
    toCond.target = condBlock;
    emit(std::move(toCond));

    currentBlock = condBlock;
    VReg more = currentFn->newVReg(Type::I8);
    emit(Instruction{Op::Lt, more, index, count});
    Instruction branch;
    branch.op = Op::Branch;
    branch.src1 = more;
    branch.target = bodyBlock;
    branch.elseTgt = doneBlock;
    emit(std::move(branch));

    currentBlock = bodyBlock;
    VReg byte = emitByteLoad(src, index);
    VReg outIndex = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Add, outIndex, dstIndex, index});
    emitByteStore(dst, outIndex, byte);
    VReg next = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Add, next, index, one});
    emit(Instruction{Op::Copy, index, next});
    Instruction back;
    back.op = Op::Jmp;
    back.target = condBlock;
    emit(std::move(back));

    currentBlock = doneBlock;
}

VReg LoweringContext::emitStringSplit(VReg str, VReg sep) {
    str = emitResolveStr(str);
    sep = emitResolveStr(sep);
    VReg strLen = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Load, strLen, str, {}, 0});
    VReg sepLen = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Load, sepLen, sep, {}, 0});
    VReg zero = emitIntConst(0);
    VReg one = emitIntConst(1);
    VReg result = currentFn->newVReg(Type::Ptr);

    Block* emptySepBlock = currentFn->newBlock("split_empty_sep");
    Block* generalBlock = currentFn->newBlock("split_general");
    Block* doneBlock = currentFn->newBlock("split_done");
    VReg sepEmpty = currentFn->newVReg(Type::I8);
    emit(Instruction{Op::Eq, sepEmpty, sepLen, zero});
    Instruction choose;
    choose.op = Op::Branch;
    choose.src1 = sepEmpty;
    choose.target = emptySepBlock;
    choose.elseTgt = generalBlock;
    emit(std::move(choose));

    currentBlock = emptySepBlock;
    VReg oneBytes = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::ConstInt, oneBytes, {}, {}, 8});
    VReg singleData = currentFn->newVReg(Type::Ptr);
    emit(Instruction{Op::Alloc, singleData, oneBytes});
    emit(Instruction{Op::Store, {}, singleData, str, 0});
    VReg singleHdrSize = emitIntConst(24);
    VReg singleHdr = currentFn->newVReg(Type::Ptr);
    emit(Instruction{Op::Alloc, singleHdr, singleHdrSize});
    emit(Instruction{Op::Store, {}, singleHdr, singleData, 0});
    emit(Instruction{Op::Store, {}, singleHdr, one, 8});
    emit(Instruction{Op::Store, {}, singleHdr, one, 16});
    emit(Instruction{Op::Copy, result, singleHdr});
    Instruction emptyToDone;
    emptyToDone.op = Op::Jmp;
    emptyToDone.target = doneBlock;
    emit(std::move(emptyToDone));

    currentBlock = generalBlock;
    VReg count = currentFn->newVReg(Type::I64);
    VReg pos = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Copy, count, zero});
    emit(Instruction{Op::Copy, pos, zero});

    Block* countCond = currentFn->newBlock("split_count_cond");
    Block* countDone = currentFn->newBlock("split_count_done");
    Block* countStep = currentFn->newBlock("split_count_step");
    Instruction toCount;
    toCount.op = Op::Jmp;
    toCount.target = countCond;
    emit(std::move(toCount));

    currentBlock = countCond;
    VReg foundCount = emitStringSearch(str, sep, pos);
    VReg noMoreCount = currentFn->newVReg(Type::I8);
    VReg minusOne = emitIntConst(-1);
    emit(Instruction{Op::Eq, noMoreCount, foundCount, minusOne});
    Instruction countBranch;
    countBranch.op = Op::Branch;
    countBranch.src1 = noMoreCount;
    countBranch.target = countDone;
    countBranch.elseTgt = countStep;
    emit(std::move(countBranch));

    currentBlock = countStep;
    VReg countNext = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Add, countNext, count, one});
    emit(Instruction{Op::Copy, count, countNext});
    VReg afterSepCount = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Add, afterSepCount, foundCount, sepLen});
    emit(Instruction{Op::Copy, pos, afterSepCount});
    Instruction countBack;
    countBack.op = Op::Jmp;
    countBack.target = countCond;
    emit(std::move(countBack));

    currentBlock = countDone;
    VReg parts = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Add, parts, count, one});
    VReg dataBytes = currentFn->newVReg(Type::I64);
    VReg eight = emitIntConst(8);
    emit(Instruction{Op::Mul, dataBytes, parts, eight});
    VReg data = currentFn->newVReg(Type::Ptr);
    emit(Instruction{Op::Alloc, data, dataBytes});

    VReg part = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Copy, part, zero});
    emit(Instruction{Op::Copy, pos, zero});

    Block* splitCond = currentFn->newBlock("split_cond");
    Block* splitBody = currentFn->newBlock("split_body");
    Block* splitFinal = currentFn->newBlock("split_final");
    Instruction toSplit;
    toSplit.op = Op::Jmp;
    toSplit.target = splitCond;
    emit(std::move(toSplit));

    currentBlock = splitCond;
    VReg foundSplit = emitStringSearch(str, sep, pos);
    VReg noMoreSplit = currentFn->newVReg(Type::I8);
    emit(Instruction{Op::Eq, noMoreSplit, foundSplit, minusOne});
    Instruction splitBranch;
    splitBranch.op = Op::Branch;
    splitBranch.src1 = noMoreSplit;
    splitBranch.target = splitFinal;
    splitBranch.elseTgt = splitBody;
    emit(std::move(splitBranch));

    currentBlock = splitBody;
    VReg pieceLen = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Sub, pieceLen, foundSplit, pos});
    VReg piece = emitSubstring(str, pos, pieceLen);
    VReg partOff = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Mul, partOff, part, eight});
    VReg partAddr = currentFn->newVReg(Type::Ptr);
    emit(Instruction{Op::Add, partAddr, data, partOff});
    emit(Instruction{Op::Store, {}, partAddr, piece, 0});
    VReg partNext = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Add, partNext, part, one});
    emit(Instruction{Op::Copy, part, partNext});
    VReg posNext = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Add, posNext, foundSplit, sepLen});
    emit(Instruction{Op::Copy, pos, posNext});
    Instruction splitBack;
    splitBack.op = Op::Jmp;
    splitBack.target = splitCond;
    emit(std::move(splitBack));

    currentBlock = splitFinal;
    VReg tailLen = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Sub, tailLen, strLen, pos});
    VReg tail = emitSubstring(str, pos, tailLen);
    VReg tailOff = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Mul, tailOff, part, eight});
    VReg tailAddr = currentFn->newVReg(Type::Ptr);
    emit(Instruction{Op::Add, tailAddr, data, tailOff});
    emit(Instruction{Op::Store, {}, tailAddr, tail, 0});
    VReg finalParts = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Add, finalParts, part, one});

    VReg hdrSize = emitIntConst(24);
    VReg hdr = currentFn->newVReg(Type::Ptr);
    emit(Instruction{Op::Alloc, hdr, hdrSize});
    emit(Instruction{Op::Store, {}, hdr, data, 0});
    emit(Instruction{Op::Store, {}, hdr, finalParts, 8});
    emit(Instruction{Op::Store, {}, hdr, parts, 16});
    emit(Instruction{Op::Copy, result, hdr});
    Instruction splitToDone;
    splitToDone.op = Op::Jmp;
    splitToDone.target = doneBlock;
    emit(std::move(splitToDone));

    currentBlock = doneBlock;
    return result;
}

VReg LoweringContext::emitStringJoin(VReg list, VReg sep) {
    sep = emitResolveStr(sep);
    VReg data = currentFn->newVReg(Type::Ptr);
    emit(Instruction{Op::Load, data, list, {}, 0});
    VReg count = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Load, count, list, {}, 8});
    VReg sepLen = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Load, sepLen, sep, {}, 0});
    VReg zero = emitIntConst(0);
    VReg one = emitIntConst(1);
    VReg eight = emitIntConst(8);
    VReg total = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Copy, total, zero});

    Block* emptyBlock = currentFn->newBlock("join_empty");
    Block* calcInit = currentFn->newBlock("join_calc_init");
    Block* calcCond = currentFn->newBlock("join_calc_cond");
    Block* calcBody = currentFn->newBlock("join_calc_body");
    Block* calcDone = currentFn->newBlock("join_calc_done");
    Block* doneBlock = currentFn->newBlock("join_done");
    VReg noItems = currentFn->newVReg(Type::I8);
    emit(Instruction{Op::Eq, noItems, count, zero});
    Instruction choose;
    choose.op = Op::Branch;
    choose.src1 = noItems;
    choose.target = emptyBlock;
    choose.elseTgt = calcInit;
    emit(std::move(choose));

    VReg result = currentFn->newVReg(Type::Ptr);

    currentBlock = emptyBlock;
    VReg emptyBytes = emitIntConst(9);
    VReg emptyBuf = currentFn->newVReg(Type::Ptr);
    emit(Instruction{Op::Alloc, emptyBuf, emptyBytes});
    emit(Instruction{Op::Store, {}, emptyBuf, zero, 0});
    emit(Instruction{Op::Store, {}, emptyBuf, zero, 8});
    emit(Instruction{Op::Copy, result, emptyBuf});
    Instruction emptyDone;
    emptyDone.op = Op::Jmp;
    emptyDone.target = doneBlock;
    emit(std::move(emptyDone));

    currentBlock = calcInit;
    VReg index = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Copy, index, zero});
    Instruction toCalc;
    toCalc.op = Op::Jmp;
    toCalc.target = calcCond;
    emit(std::move(toCalc));

    currentBlock = calcCond;
    VReg calcMore = currentFn->newVReg(Type::I8);
    emit(Instruction{Op::Lt, calcMore, index, count});
    Instruction calcBranch;
    calcBranch.op = Op::Branch;
    calcBranch.src1 = calcMore;
    calcBranch.target = calcBody;
    calcBranch.elseTgt = calcDone;
    emit(std::move(calcBranch));

    currentBlock = calcBody;
    VReg itemOff = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Mul, itemOff, index, eight});
    VReg itemAddr = currentFn->newVReg(Type::Ptr);
    emit(Instruction{Op::Add, itemAddr, data, itemOff});
    VReg item = currentFn->newVReg(Type::Ptr);
    emit(Instruction{Op::Load, item, itemAddr, {}, 0});
    VReg itemResolved = emitResolveStr(item);
    VReg itemLen = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Load, itemLen, itemResolved, {}, 0});
    emit(Instruction{Op::Add, total, total, itemLen});
    VReg calcNext = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Add, calcNext, index, one});
    emit(Instruction{Op::Copy, index, calcNext});
    Instruction calcBack;
    calcBack.op = Op::Jmp;
    calcBack.target = calcCond;
    emit(std::move(calcBack));

    currentBlock = calcDone;
    VReg gapCount = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Sub, gapCount, count, one});
    VReg gapBytes = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Mul, gapBytes, gapCount, sepLen});
    emit(Instruction{Op::Add, total, total, gapBytes});
    VReg allocSize = currentFn->newVReg(Type::I64);
    VReg overhead = emitIntConst(9);
    emit(Instruction{Op::Add, allocSize, total, overhead});
    VReg buffer = currentFn->newVReg(Type::Ptr);
    emit(Instruction{Op::Alloc, buffer, allocSize});
    emit(Instruction{Op::Store, {}, buffer, total, 0});
    emit(Instruction{Op::Copy, result, buffer});

    VReg outPos = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Copy, outPos, zero});
    emit(Instruction{Op::Copy, index, zero});

    Block* copyCond = currentFn->newBlock("join_copy_cond");
    Block* copyBody = currentFn->newBlock("join_copy_body");
    Block* sepBlock = currentFn->newBlock("join_sep");
    Block* copyStep = currentFn->newBlock("join_copy_step");
    Block* copyDone = currentFn->newBlock("join_copy_done");
    Instruction toCopy;
    toCopy.op = Op::Jmp;
    toCopy.target = copyCond;
    emit(std::move(toCopy));

    currentBlock = copyCond;
    VReg copyMore = currentFn->newVReg(Type::I8);
    emit(Instruction{Op::Lt, copyMore, index, count});
    Instruction copyBranch;
    copyBranch.op = Op::Branch;
    copyBranch.src1 = copyMore;
    copyBranch.target = copyBody;
    copyBranch.elseTgt = copyDone;
    emit(std::move(copyBranch));

    currentBlock = copyBody;
    VReg outItemOff = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Mul, outItemOff, index, eight});
    VReg outItemAddr = currentFn->newVReg(Type::Ptr);
    emit(Instruction{Op::Add, outItemAddr, data, outItemOff});
    VReg outItem = currentFn->newVReg(Type::Ptr);
    emit(Instruction{Op::Load, outItem, outItemAddr, {}, 0});
    VReg outItemResolved = emitResolveStr(outItem);
    VReg outItemLen = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Load, outItemLen, outItemResolved, {}, 0});
    emitCopyString(buffer, outPos, outItemResolved, outItemLen);
    VReg afterItem = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Add, afterItem, outPos, outItemLen});
    emit(Instruction{Op::Copy, outPos, afterItem});
    VReg hasMore = currentFn->newVReg(Type::I8);
    VReg lastIndex = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Sub, lastIndex, count, one});
    emit(Instruction{Op::Lt, hasMore, index, lastIndex});
    Instruction sepBranch;
    sepBranch.op = Op::Branch;
    sepBranch.src1 = hasMore;
    sepBranch.target = sepBlock;
    sepBranch.elseTgt = copyStep;
    emit(std::move(sepBranch));

    currentBlock = sepBlock;
    emitCopyString(buffer, outPos, sep, sepLen);
    VReg afterSep = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Add, afterSep, outPos, sepLen});
    emit(Instruction{Op::Copy, outPos, afterSep});
    Instruction sepToStep;
    sepToStep.op = Op::Jmp;
    sepToStep.target = copyStep;
    emit(std::move(sepToStep));

    currentBlock = copyStep;
    VReg nextIndex = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Add, nextIndex, index, one});
    emit(Instruction{Op::Copy, index, nextIndex});
    Instruction copyBack;
    copyBack.op = Op::Jmp;
    copyBack.target = copyCond;
    emit(std::move(copyBack));

    currentBlock = copyDone;
    VReg endAddr = currentFn->newVReg(Type::Ptr);
    emit(Instruction{Op::Add, endAddr, buffer, total});
    VReg nul = currentFn->newVReg(Type::I8);
    VReg byteZero = emitIntConst(0);
    emit(Instruction{Op::Copy, nul, byteZero});
    emit(Instruction{Op::Store, {}, endAddr, nul, 8});
    Instruction toDone;
    toDone.op = Op::Jmp;
    toDone.target = doneBlock;
    emit(std::move(toDone));

    currentBlock = doneBlock;
    return result;
}

VReg LoweringContext::lowerStringEq(VReg left, VReg right, bool negate) {
    VReg dst = currentFn->newVReg(Type::I8);
    Instruction call;
    call.op = Op::Call;
    call.dst = dst;
    call.callName = "ulang_str_eq";
    call.args = {left, right};
    emit(std::move(call));
    if (!negate) return dst;

    VReg inverted = currentFn->newVReg(Type::I8);
    emit(Instruction{Op::Not, inverted, dst});
    return inverted;
}

VReg LoweringContext::lowerListEq(VReg left, VReg right, Type elemType, bool negate) {
    VReg dst = currentFn->newVReg(Type::I8);

    VReg lenLeft = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Load, lenLeft, left, {}, 8});
    VReg lenRight = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Load, lenRight, right, {}, 8});
    VReg sameLen = currentFn->newVReg(Type::I8);
    emit(Instruction{Op::Eq, sameLen, lenLeft, lenRight});

    Block* bodyBlock = currentFn->newBlock("list_eq_body");
    Block* unequalBlock = currentFn->newBlock("list_eq_unequal");
    Block* doneBlock = currentFn->newBlock("list_eq_done");

    Instruction lenBranch;
    lenBranch.op = Op::Branch;
    lenBranch.src1 = sameLen;
    lenBranch.target = bodyBlock;
    lenBranch.elseTgt = unequalBlock;
    emit(std::move(lenBranch));

    currentBlock = bodyBlock;
    VReg dataLeft = currentFn->newVReg(Type::Ptr);
    emit(Instruction{Op::Load, dataLeft, left, {}, 0});
    VReg dataRight = currentFn->newVReg(Type::Ptr);
    emit(Instruction{Op::Load, dataRight, right, {}, 0});
    VReg idx = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::ConstInt, idx, {}, {}, 0});
    VReg elemSize = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::ConstInt, elemSize, {}, {}, 8});
    VReg one = currentFn->newVReg(Type::I8);
    emit(Instruction{Op::ConstInt, one, {}, {}, 1});
    emit(Instruction{Op::Copy, dst, one});

    Block* condBlock = currentFn->newBlock("list_eq_cond");
    Block* elemBlock = currentFn->newBlock("list_eq_elem");
    Block* stepBlock = currentFn->newBlock("list_eq_step");

    Instruction toCond;
    toCond.op = Op::Jmp;
    toCond.target = condBlock;
    emit(std::move(toCond));

    currentBlock = condBlock;
    VReg more = currentFn->newVReg(Type::I8);
    emit(Instruction{Op::Lt, more, idx, lenLeft});
    Instruction condBranch;
    condBranch.op = Op::Branch;
    condBranch.src1 = more;
    condBranch.target = elemBlock;
    condBranch.elseTgt = doneBlock;
    emit(std::move(condBranch));

    currentBlock = elemBlock;
    VReg off = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Mul, off, idx, elemSize});
    VReg addrLeft = currentFn->newVReg(Type::Ptr);
    emit(Instruction{Op::Add, addrLeft, dataLeft, off});
    VReg addrRight = currentFn->newVReg(Type::Ptr);
    emit(Instruction{Op::Add, addrRight, dataRight, off});
    VReg elemLeft = currentFn->newVReg(elemType);
    emit(Instruction{Op::Load, elemLeft, addrLeft, {}, 0});
    VReg elemRight = currentFn->newVReg(elemType);
    emit(Instruction{Op::Load, elemRight, addrRight, {}, 0});
    VReg matches = currentFn->newVReg(Type::I8);
    emit(Instruction{Op::Eq, matches, elemLeft, elemRight});

    Instruction matchBranch;
    matchBranch.op = Op::Branch;
    matchBranch.src1 = matches;
    matchBranch.target = stepBlock;
    matchBranch.elseTgt = unequalBlock;
    emit(std::move(matchBranch));

    currentBlock = stepBlock;
    VReg next = currentFn->newVReg(Type::I64);
    emit(Instruction{Op::Add, next, idx, one});
    emit(Instruction{Op::Copy, idx, next});
    Instruction back;
    back.op = Op::Jmp;
    back.target = condBlock;
    emit(std::move(back));

    currentBlock = unequalBlock;
    VReg zero = currentFn->newVReg(Type::I8);
    emit(Instruction{Op::ConstInt, zero, {}, {}, 0});
    emit(Instruction{Op::Copy, dst, zero});
    Instruction toDone;
    toDone.op = Op::Jmp;
    toDone.target = doneBlock;
    emit(std::move(toDone));

    currentBlock = doneBlock;
    if (!negate) return dst;

    VReg inverted = currentFn->newVReg(Type::I8);
    emit(Instruction{Op::Not, inverted, dst});
    return inverted;
}

bool LoweringContext::isConstInitializer(const ast::Expression& e) {
    if (dynamic_cast<const ast::IntegerLiteral*>(&e) ||
        dynamic_cast<const ast::StringLiteral*>(&e)  ||
        dynamic_cast<const ast::FloatLiteral*>(&e)   ||
        dynamic_cast<const ast::BooleanLiteral*>(&e)) {
        return true;
    }
    if (auto* p = dynamic_cast<const ast::PrefixExpression*>(&e)) {
        return (p->op == "-" || p->op == "+") && p->right &&
               isConstInitializer(*p->right);
    }
    if (auto* in = dynamic_cast<const ast::InfixExpression*>(&e)) {
        return in->left && in->right &&
               isConstInitializer(*in->left) && isConstInitializer(*in->right);
    }
    // Identifiers are intentionally rejected: no recursive substitution.
    return false;
}

std::vector<std::pair<std::string, std::string>> LoweringContext::snapshotActiveDestructors() const {
    std::vector<std::pair<std::string, std::string>> out;
    for (auto frame = destructibleScopes.rbegin(); frame != destructibleScopes.rend(); ++frame) {
        for (auto obj = frame->rbegin(); obj != frame->rend(); ++obj) {
            out.push_back(*obj);
        }
    }
    return out;
}

void LoweringContext::emitDestructorCalls(
        const std::vector<std::pair<std::string, std::string>>& objects) {
    for (const auto& [name, type] : objects) {
        auto lit = locals.find(name);
        if (lit == locals.end()) continue;
        Instruction call;
        call.op = Op::Call;
        call.callName = type;
        call.args = {lit->second};
        emit(std::move(call));
    }
}

void LoweringContext::emitDeferredExits(ir::Function* fn) {
    if (deferredExprs.empty() && retDestructors.empty()) return;
    for (auto& blockPtr : fn->blocks) {
        Block* b = blockPtr.get();
        for (size_t i = 0; i < b->instructions.size();) {
            if (b->instructions[i]->op != Op::Ret) {
                ++i;
                continue;
            }
            Function* savedFn = currentFn;
            Block* savedBlock = currentBlock;
            Block temp;
            currentFn = fn;
            currentBlock = &temp;
            loweringDeferred = true;
            for (const auto& expr : deferredExprs) lowerExpression(*expr);
            loweringDeferred = false;
            // Objects alive at this return (innermost scope first), reversed.
            auto rd = retDestructors.find(b->instructions[i].get());
            if (rd != retDestructors.end()) emitDestructorCalls(rd->second);
            currentFn = savedFn;
            currentBlock = savedBlock;
            size_t count = temp.instructions.size();
            b->instructions.insert(
                b->instructions.begin() + static_cast<long>(i),
                std::make_move_iterator(temp.instructions.begin()),
                std::make_move_iterator(temp.instructions.end()));
            i += count + 1;
        }
    }
}

int64_t LoweringContext::evalConstExpr(const ast::Expression& e, bool& ok) {
    if (auto* n = dynamic_cast<const ast::IntegerLiteral*>(&e)) return n->value;
    if (auto* b = dynamic_cast<const ast::BooleanLiteral*>(&e)) return b->value ? 1 : 0;
    if (auto* p = dynamic_cast<const ast::PrefixExpression*>(&e)) {
        if (!p->right) { ok = false; return 0; }
        int64_t v = evalConstExpr(*p->right, ok);
        if (!ok) return 0;
        if (p->op == "-") return -v;
        if (p->op == "+") return v;
        ok = false;
        return 0;
    }
    if (auto* in = dynamic_cast<const ast::InfixExpression*>(&e)) {
        if (!in->left || !in->right) { ok = false; return 0; }
        int64_t l = evalConstExpr(*in->left, ok);
        int64_t r = evalConstExpr(*in->right, ok);
        if (!ok) return 0;
        if (in->op == "+") return l + r;
        if (in->op == "-") return l - r;
        if (in->op == "*") return l * r;
        if (in->op == "/" && r != 0) return l / r;
        if (in->op == "%" && r != 0) return l % r;
        ok = false;
        return 0;
    }
    // Shifts are lexed as stream expressions when both operands are integers.
    if (auto* se = dynamic_cast<const ast::StreamExpression*>(&e)) {
        if (!se->source || !se->target ||
            (se->op != "<<" && se->op != ">>")) { ok = false; return 0; }
        int64_t l = evalConstExpr(*se->source, ok);
        int64_t r = evalConstExpr(*se->target, ok);
        if (!ok) return 0;
        if (se->op == "<<") return static_cast<int64_t>(static_cast<uint64_t>(l) << (r & 63));
        return static_cast<int64_t>(static_cast<uint64_t>(l) >> (r & 63));
    }
    ok = false;
    return 0;
}

int64_t LoweringContext::evalLiteralInit(const ast::Expression& e) {
    if (auto* n = dynamic_cast<const ast::IntegerLiteral*>(&e)) return n->value;
    if (auto* b = dynamic_cast<const ast::BooleanLiteral*>(&e)) return b->value ? 1 : 0;
    if (auto* p = dynamic_cast<const ast::PrefixExpression*>(&e)) {
        if (p->right && p->op == "-") return -evalLiteralInit(*p->right);
        return evalLiteralInit(*p->right);
    }
    if (auto* in = dynamic_cast<const ast::InfixExpression*>(&e)) {
        if (!in->left || !in->right) return 0;
        int64_t l = evalLiteralInit(*in->left);
        int64_t r = evalLiteralInit(*in->right);
        if (in->op == "+") return l + r;
        if (in->op == "-") return l - r;
        if (in->op == "*") return l * r;
        if (in->op == "/" && r != 0) return l / r;
        if (in->op == "%" && r != 0) return l % r;
    }
    return 0;
}

void LoweringContext::collectModuleConstants(const ast::Program& prog) {
    // Pass 1: find names that are written anywhere.  Those are mutable state,
    // not constants -- e.g. io.vga_row in std/io.upp.
    std::function<void(const ast::Statement&)> scanAssigned =
        [&](const ast::Statement& s) {
        if (auto* es = dynamic_cast<const ast::ExpressionStatement*>(&s)) {
            if (es->expression) {
                if (auto* as = dynamic_cast<ast::AssignExpression*>(es->expression.get())) {
                    if (as->target) {
                        if (auto* id = dynamic_cast<ast::Identifier*>(as->target.get())) {
                            assignedNames.insert(id->value);
                        }
                    }
                }
            }
        } else if (auto* b = dynamic_cast<const ast::BlockStatement*>(&s)) {
            for (const auto& st : b->statements) if (st) scanAssigned(*st);
        } else if (auto* i = dynamic_cast<const ast::IfStatement*>(&s)) {
            if (i->consequence) scanAssigned(*i->consequence);
            for (const auto& a : i->alternatives) if (a && a->body) scanAssigned(*a->body);
            if (i->elseBlock) scanAssigned(*i->elseBlock);
        } else if (auto* w = dynamic_cast<const ast::WhileStatement*>(&s)) {
            if (w->body) scanAssigned(*w->body);
        } else if (auto* f = dynamic_cast<const ast::ForStatement*>(&s)) {
            if (f->body) scanAssigned(*f->body);
        } else if (auto* f = dynamic_cast<const ast::ForEachStatement*>(&s)) {
            if (f->body) scanAssigned(*f->body);
        } else if (auto* l = dynamic_cast<const ast::LoopStatement*>(&s)) {
            if (l->body) scanAssigned(*l->body);
        } else if (auto* m = dynamic_cast<const ast::MatchStatement*>(&s)) {
            for (const auto& c : m->cases) if (c && c->body) scanAssigned(*c->body);
            if (m->defaultCase) scanAssigned(*m->defaultCase);
        } else if (auto* fn = dynamic_cast<const ast::FuncDeclaration*>(&s)) {
            if (fn->body) {
                for (const auto& st : fn->body->statements) if (st) scanAssigned(*st);
            }
        } else if (auto* cls = dynamic_cast<const ast::ClassDeclaration*>(&s)) {
            for (const auto& m : cls->methods) {
                if (m && m->func && m->func->body) {
                    for (const auto& st : m->func->body->statements) if (st) scanAssigned(*st);
                }
            }
        }
    };
    for (const auto& st : prog.statements) if (st) scanAssigned(*st);

    // Pass 2: literal-only, never-assigned top-level declarations are inlined
    // as constants.  Everything else becomes a real mutable global slot so it
    // can be shared across functions (and have a static initial value).
    for (const auto& st : prog.statements) {
        auto* vd = dynamic_cast<ast::VarDeclaration*>(st.get());
        if (!vd || !vd->name) continue;
        if (vd->isConstexpr && vd->value) {
            bool ok = true;
            int64_t v = evalConstExpr(*vd->value, ok);
            if (ok) {
                constexprValues[vd->name->value] = v;
                continue;
            }
        }
        bool lit = vd->value && isConstInitializer(*vd->value);
        if (lit && !assignedNames.count(vd->name->value)) {
            moduleConsts[vd->name->value] = vd->value.get();
            continue;
        }
        GlobalVar g;
        g.name = vd->name->value;
        g.initValue = lit ? static_cast<uint64_t>(evalLiteralInit(*vd->value)) : 0;
        globalIndex[g.name] = static_cast<uint32_t>(irProg->globals.size());
        irProg->globals.push_back(std::move(g));
    }
}

VReg LoweringContext::lowerExpression(const ast::Expression& e) {
    // `<<` / `>>` are lexed as stream operators.  When both operands are
    // integers they are unambiguously bitwise shifts (used heavily by hardware
    // register code); keep stream semantics for every other operand type.
    if (auto* se = dynamic_cast<const ast::StreamExpression*>(&e)) {
        if ((se->op == "<<" || se->op == ">>") && se->source && se->target) {
            auto isInt = [&](const ast::Expression* x) {
                auto it = checker.types().find(x);
                if (it == checker.types().end()) return false;
                auto* basic = dynamic_cast<typecheck::Basic*>(it->second.get());
                return basic && basic->name() == "int";
            };
            if (isInt(se->source.get()) && isInt(se->target.get())) {
                VReg left = lowerExpression(*se->source);
                VReg right = lowerExpression(*se->target);
                VReg dst = currentFn->newVReg(Type::I64);
                emit(Instruction{se->op == "<<" ? Op::Shl : Op::Shr, dst, left, right});
                return dst;
            }
        }
    }
    if (auto* expr = dynamic_cast<const ast::IntegerLiteral*>(&e)) {
        VReg dst = currentFn->newVReg(Type::I64);
        emit(Instruction{Op::ConstInt, dst, {}, {}, expr->value});
        return dst;
    }
    if (dynamic_cast<const ast::NullLiteral*>(&e)) {
        VReg dst = currentFn->newVReg(Type::Ptr);
        emit(Instruction{Op::ConstInt, dst, {}, {}, 0});
        return dst;
    }
    if (auto* expr = dynamic_cast<const ast::StringLiteral*>(&e)) {
        VReg dst = currentFn->newVReg(Type::Ptr);
        Instruction inst;
        inst.op = Op::ConstStr;
        inst.dst = dst;
        inst.strVal = expr->value;
        emit(std::move(inst));
        return dst;
    }
    if (auto* expr = dynamic_cast<const ast::FloatLiteral*>(&e)) {
        VReg dst = currentFn->newVReg(Type::F64);
        Instruction inst;
        inst.op = Op::ConstFloat;
        inst.dst = dst;
        inst.floatVal = expr->value;
        emit(std::move(inst));
        return dst;
    }
    if (auto* expr = dynamic_cast<const ast::BooleanLiteral*>(&e)) {
        VReg dst = currentFn->newVReg(Type::I8);
        emit(Instruction{Op::ConstBool, dst, {}, {}, expr->value ? 1 : 0});
        return dst;
    }
    if (auto* expr = dynamic_cast<const ast::Identifier*>(&e)) {
        auto it = locals.find(expr->value);
        if (it == locals.end()) {
            auto cv = constexprValues.find(expr->value);
            if (cv != constexprValues.end()) {
                VReg dst = currentFn->newVReg(Type::I64);
                emit(Instruction{Op::ConstInt, dst, {}, {}, cv->second});
                return dst;
            }
            auto c = moduleConsts.find(expr->value);
            if (c != moduleConsts.end()) return lowerExpression(*c->second);
            if (functions.count(expr->value)) {
                VReg dst = currentFn->newVReg(Type::Ptr);
                Instruction in;
                in.op = Op::FuncAddr;
                in.dst = dst;
                in.callName = expr->value;
                emit(std::move(in));
                return dst;
            }
            auto g = globalIndex.find(expr->value);
            if (g != globalIndex.end()) {
                VReg dst = currentFn->newVReg(Type::I64);
                Instruction in;
                in.op = Op::LoadGlobal;
                in.dst = dst;
                in.callName = expr->value;
                in.intVal = 8;
                emit(std::move(in));
                return dst;
            }
            throw std::runtime_error("undefined local variable " + expr->value);
        }
        return it->second;
    }
    if (auto* expr = dynamic_cast<const ast::ListLiteral*>(&e)) {
        int64_t n = static_cast<int64_t>(expr->elements.size());
        int64_t capVal = n < 4 ? 4 : n;

        VReg capBytes = currentFn->newVReg(Type::I64);
        emit(Instruction{Op::ConstInt, capBytes, {}, {}, capVal * 8});
        VReg dataPtr = currentFn->newVReg(Type::Ptr);
        emit(Instruction{Op::Alloc, dataPtr, capBytes});

        for (size_t i = 0; i < expr->elements.size(); ++i) {
            VReg elReg = lowerExpression(*expr->elements[i]);
            emit(Instruction{Op::Store, {}, dataPtr, elReg, static_cast<int64_t>(i * 8)});
        }

        VReg hdrSize = currentFn->newVReg(Type::I64);
        emit(Instruction{Op::ConstInt, hdrSize, {}, {}, 24});
        VReg listHdr = currentFn->newVReg(Type::Ptr);
        emit(Instruction{Op::Alloc, listHdr, hdrSize});

        VReg lenReg = currentFn->newVReg(Type::I64);
        emit(Instruction{Op::ConstInt, lenReg, {}, {}, n});
        VReg capReg = currentFn->newVReg(Type::I64);
        emit(Instruction{Op::ConstInt, capReg, {}, {}, capVal});

        emit(Instruction{Op::Store, {}, listHdr, dataPtr, 0});
        emit(Instruction{Op::Store, {}, listHdr, lenReg, 8});
        emit(Instruction{Op::Store, {}, listHdr, capReg, 16});
        return listHdr;
    }
    if (auto* expr = dynamic_cast<const ast::MapLiteral*>(&e)) {
        int64_t n = static_cast<int64_t>(expr->pairs.size());
        int64_t capVal = n < 4 ? 4 : n;

        VReg bucketsBytes = currentFn->newVReg(Type::I64);
        emit(Instruction{Op::ConstInt, bucketsBytes, {}, {}, capVal * 16});
        VReg bucketsPtr = currentFn->newVReg(Type::Ptr);
        emit(Instruction{Op::Alloc, bucketsPtr, bucketsBytes});

        for (size_t i = 0; i < expr->pairs.size(); ++i) {
            VReg kReg = lowerExpression(*expr->pairs[i]->key);
            VReg vReg = lowerExpression(*expr->pairs[i]->value);
            emit(Instruction{Op::Store, {}, bucketsPtr, kReg, static_cast<int64_t>(i * 16)});
            emit(Instruction{Op::Store, {}, bucketsPtr, vReg, static_cast<int64_t>(i * 16 + 8)});
        }

        VReg mapHdrSize = currentFn->newVReg(Type::I64);
        emit(Instruction{Op::ConstInt, mapHdrSize, {}, {}, 24});
        VReg mapHdr = currentFn->newVReg(Type::Ptr);
        emit(Instruction{Op::Alloc, mapHdr, mapHdrSize});

        VReg lenReg = currentFn->newVReg(Type::I64);
        emit(Instruction{Op::ConstInt, lenReg, {}, {}, n});
        VReg capReg = currentFn->newVReg(Type::I64);
        emit(Instruction{Op::ConstInt, capReg, {}, {}, capVal});

        emit(Instruction{Op::Store, {}, mapHdr, bucketsPtr, 0});
        emit(Instruction{Op::Store, {}, mapHdr, lenReg, 8});
        emit(Instruction{Op::Store, {}, mapHdr, capReg, 16});
        return mapHdr;
    }
    if (auto* expr = dynamic_cast<const ast::IndexExpression*>(&e)) {
        VReg leftReg = lowerExpression(*expr->left);
        VReg idxReg = lowerExpression(*expr->index);

        bool isMap = false;
        bool isString = false;
        Type elemIRType = Type::I64;

        const auto& chkTypes = checker.types();
        auto itColl = chkTypes.find(expr->left.get());
        if (itColl != chkTypes.end()) {
            if (dynamic_cast<typecheck::MapType*>(itColl->second.get())) {
                isMap = true;
            } else if (auto* basic = dynamic_cast<typecheck::Basic*>(itColl->second.get())) {
                if (basic->name() == "string") isString = true;
            }
        }
        auto itElem = chkTypes.find(&e);
        if (itElem != chkTypes.end()) {
            elemIRType = typeToIR(itElem->second);
        }

        if (isString) {
            VReg resolved = emitResolveStr(leftReg);
            return emitByteLoad(resolved, idxReg);
        }

        if (isMap) {
            VReg bucketsPtr = currentFn->newVReg(Type::Ptr);
            emit(Instruction{Op::Load, bucketsPtr, leftReg, {}, 0});
            VReg mapLen = currentFn->newVReg(Type::I64);
            emit(Instruction{Op::Load, mapLen, leftReg, {}, 8});

            VReg loopIdx = currentFn->newVReg(Type::I64);
            emit(Instruction{Op::ConstInt, loopIdx, {}, {}, 0});

            VReg resVal = currentFn->newVReg(elemIRType);
            emit(Instruction{Op::ConstInt, resVal, {}, {}, 0});

            Block* condBlock = currentFn->newBlock("map_lookup_cond");
            Block* bodyBlock = currentFn->newBlock("map_lookup_body");
            Block* stepBlock = currentFn->newBlock("map_lookup_step");
            Block* afterBlock = currentFn->newBlock("map_lookup_after");

            Instruction jmpCond;
            jmpCond.op = Op::Jmp;
            jmpCond.target = condBlock;
            emit(std::move(jmpCond));

            currentBlock = condBlock;
            VReg cmpLt = currentFn->newVReg(Type::I8);
            emit(Instruction{Op::Lt, cmpLt, loopIdx, mapLen});

            Instruction brInst;
            brInst.op = Op::Branch;
            brInst.src1 = cmpLt;
            brInst.target = bodyBlock;
            brInst.elseTgt = afterBlock;
            emit(std::move(brInst));

            currentBlock = bodyBlock;
            VReg sixteen = currentFn->newVReg(Type::I64);
            emit(Instruction{Op::ConstInt, sixteen, {}, {}, 16});
            VReg bOff = currentFn->newVReg(Type::I64);
            emit(Instruction{Op::Mul, bOff, loopIdx, sixteen});
            VReg bAddr = currentFn->newVReg(Type::Ptr);
            emit(Instruction{Op::Add, bAddr, bucketsPtr, bOff});

            VReg currKey = currentFn->newVReg(idxReg.type);
            emit(Instruction{Op::Load, currKey, bAddr, {}, 0});

            VReg keyMatch = currentFn->newVReg(Type::I8);
            emit(Instruction{Op::Eq, keyMatch, currKey, idxReg});

            Block* foundBlock = currentFn->newBlock("map_lookup_found");
            Instruction brMatch;
            brMatch.op = Op::Branch;
            brMatch.src1 = keyMatch;
            brMatch.target = foundBlock;
            brMatch.elseTgt = stepBlock;
            emit(std::move(brMatch));

            currentBlock = foundBlock;
            emit(Instruction{Op::Load, resVal, bAddr, {}, 8});
            Instruction jmpAfter;
            jmpAfter.op = Op::Jmp;
            jmpAfter.target = afterBlock;
            emit(std::move(jmpAfter));

            currentBlock = stepBlock;
            VReg one = currentFn->newVReg(Type::I64);
            emit(Instruction{Op::ConstInt, one, {}, {}, 1});
            VReg nextIdx = currentFn->newVReg(Type::I64);
            emit(Instruction{Op::Add, nextIdx, loopIdx, one});
            emit(Instruction{Op::Copy, loopIdx, nextIdx});

            Instruction jmpBack;
            jmpBack.op = Op::Jmp;
            jmpBack.target = condBlock;
            emit(std::move(jmpBack));

            currentBlock = afterBlock;
            return resVal;
        }

        // List indexing
        VReg dataPtr = currentFn->newVReg(Type::Ptr);
        emit(Instruction{Op::Load, dataPtr, leftReg, {}, 0});

        VReg scaleReg = currentFn->newVReg(Type::I64);
        emit(Instruction{Op::ConstInt, scaleReg, {}, {}, 8});
        VReg byteOff = currentFn->newVReg(Type::I64);
        emit(Instruction{Op::Mul, byteOff, idxReg, scaleReg});
        VReg addrReg = currentFn->newVReg(Type::Ptr);
        emit(Instruction{Op::Add, addrReg, dataPtr, byteOff});

        VReg resReg = currentFn->newVReg(elemIRType);
        emit(Instruction{Op::Load, resReg, addrReg, {}, 0});
        return resReg;
    }
    if (auto* expr = dynamic_cast<const ast::DotExpression*>(&e)) {
        if (auto* ident = dynamic_cast<ast::Identifier*>(expr->left.get())) {
            if (ident->value == "math") {
                if (expr->field == "PI" || expr->field == "pi") {
                    VReg dst = currentFn->newVReg(Type::F64);
                    Instruction inst;
                    inst.op = Op::ConstFloat;
                    inst.dst = dst;
                    inst.floatVal = 3.14159265358979323846;
                    emit(std::move(inst));
                    return dst;
                }
                if (expr->field == "E" || expr->field == "e") {
                    VReg dst = currentFn->newVReg(Type::F64);
                    Instruction inst;
                    inst.op = Op::ConstFloat;
                    inst.dst = dst;
                    inst.floatVal = 2.71828182845904523536;
                    emit(std::move(inst));
                    return dst;
                }
            }
            auto eIt = enums.find(ident->value);
            if (eIt != enums.end()) {
                auto vIt = eIt->second.find(expr->field);
                if (vIt != eIt->second.end()) {
                    VReg dst = currentFn->newVReg(Type::I64);
                    emit(Instruction{Op::ConstInt, dst, {}, {}, vIt->second});
                    return dst;
                }
            }
        }

        VReg leftReg = lowerExpression(*expr->left);
        std::string typeName;
        const auto& chkTypes = checker.types();
        auto itT = chkTypes.find(expr->left.get());
        if (itT != chkTypes.end() && itT->second) {
            if (auto* st = dynamic_cast<typecheck::StructType*>(itT->second.get())) {
                typeName = st->name;
            } else if (auto* un = dynamic_cast<typecheck::UnionType*>(itT->second.get())) {
                for (const auto& m : un->types) {
                    if (auto* st = dynamic_cast<typecheck::StructType*>(m.get())) {
                        typeName = st->name;
                        break;
                    }
                    if (m->toString() == "error") {
                        typeName = "error";
                        break;
                    }
                }
            } else if (itT->second->toString() == "error") {
                typeName = "error";
            }
        }
        auto lIt = structLayouts.find(typeName);
        if (lIt == structLayouts.end()) {
            throw std::runtime_error("unknown struct/class for dot access " + typeName + "." + expr->field);
        }
        auto fIt = lIt->second.fields.find(expr->field);
        if (fIt == lIt->second.fields.end()) {
            throw std::runtime_error("field " + expr->field + " not found in " + typeName);
        }
        Type fType = lIt->second.fieldTypes.at(expr->field);
        VReg resReg = currentFn->newVReg(fType);
        emit(Instruction{Op::Load, resReg, leftReg, {}, fIt->second});
        return resReg;
    }
    if (auto* expr = dynamic_cast<const ast::StructInstantiation*>(&e)) {
        auto lIt = structLayouts.find(expr->name);
        if (lIt == structLayouts.end()) {
            throw std::runtime_error("unknown struct/class " + expr->name);
        }
        VReg sizeReg = currentFn->newVReg(Type::I64);
        emit(Instruction{Op::ConstInt, sizeReg, {}, {}, lIt->second.totalSize});
        VReg objReg = currentFn->newVReg(Type::Ptr);
        emit(Instruction{Op::Alloc, objReg, sizeReg});

        int64_t offset = lIt->second.isClass ? 8 : 0;
        for (const auto& arg : expr->args) {
            VReg ar = lowerExpression(*arg);
            emit(Instruction{Op::Store, {}, objReg, ar, offset});
            offset += 8;
        }
        return objReg;
    }
    if (auto* expr = dynamic_cast<const ast::AssignExpression*>(&e)) {
        VReg valReg = lowerExpression(*expr->value);

        if (auto* tgt = dynamic_cast<ast::Identifier*>(expr->target.get())) {
            if (globalIndex.count(tgt->value)) {
                Instruction in;
                in.op = Op::StoreGlobal;
                in.src1 = valReg;
                in.callName = tgt->value;
                in.intVal = 8;
                emit(std::move(in));
                return valReg;
            }
            auto it = locals.find(tgt->value);
            VReg localReg;
            if (it == locals.end()) {
                localReg = currentFn->newVReg(valReg.type);
                locals[tgt->value] = localReg;
            } else {
                localReg = it->second;
            }
            emit(Instruction{Op::Copy, localReg, valReg});
            return localReg;
        }
        if (auto* tgt = dynamic_cast<ast::DotExpression*>(expr->target.get())) {
            VReg objReg = lowerExpression(*tgt->left);
            std::string typeName;
            const auto& chkTypes = checker.types();
            auto itT = chkTypes.find(tgt->left.get());
            if (itT != chkTypes.end()) {
                if (auto* st = dynamic_cast<typecheck::StructType*>(itT->second.get())) {
                    typeName = st->name;
                }
            }
            auto lIt = structLayouts.find(typeName);
            if (lIt == structLayouts.end()) {
                throw std::runtime_error("unknown struct/class for dot assignment " + typeName + "." + tgt->field);
            }
            auto fIt = lIt->second.fields.find(tgt->field);
            if (fIt == lIt->second.fields.end()) {
                throw std::runtime_error("field " + tgt->field + " not found in " + typeName);
            }
            emit(Instruction{Op::Store, {}, objReg, valReg, fIt->second});
            return valReg;
        }
        if (auto* tgt = dynamic_cast<ast::IndexExpression*>(expr->target.get())) {
            VReg collReg = lowerExpression(*tgt->left);
            VReg idxReg = lowerExpression(*tgt->index);

            bool isMap = false;
            const auto& chkTypes = checker.types();
            auto itT = chkTypes.find(tgt->left.get());
            if (itT != chkTypes.end()) {
                if (dynamic_cast<typecheck::MapType*>(itT->second.get())) {
                    isMap = true;
                }
            }

            if (isMap) {
                VReg bucketsPtr = currentFn->newVReg(Type::Ptr);
                emit(Instruction{Op::Load, bucketsPtr, collReg, {}, 0});
                VReg mapLen = currentFn->newVReg(Type::I64);
                emit(Instruction{Op::Load, mapLen, collReg, {}, 8});

                VReg loopIdx = currentFn->newVReg(Type::I64);
                emit(Instruction{Op::ConstInt, loopIdx, {}, {}, 0});

                Block* condBlock = currentFn->newBlock("map_store_cond");
                Block* bodyBlock = currentFn->newBlock("map_store_body");
                Block* stepBlock = currentFn->newBlock("map_store_step");
                Block* appendBlock = currentFn->newBlock("map_store_append");
                Block* doneBlock = currentFn->newBlock("map_store_done");

                Instruction jmpCond;
                jmpCond.op = Op::Jmp;
                jmpCond.target = condBlock;
                emit(std::move(jmpCond));

                currentBlock = condBlock;
                VReg cmpLt = currentFn->newVReg(Type::I8);
                emit(Instruction{Op::Lt, cmpLt, loopIdx, mapLen});

                Instruction brInst;
                brInst.op = Op::Branch;
                brInst.src1 = cmpLt;
                brInst.target = bodyBlock;
                brInst.elseTgt = appendBlock;
                emit(std::move(brInst));

                currentBlock = bodyBlock;
                VReg sixteen = currentFn->newVReg(Type::I64);
                emit(Instruction{Op::ConstInt, sixteen, {}, {}, 16});
                VReg bOff = currentFn->newVReg(Type::I64);
                emit(Instruction{Op::Mul, bOff, loopIdx, sixteen});
                VReg bAddr = currentFn->newVReg(Type::Ptr);
                emit(Instruction{Op::Add, bAddr, bucketsPtr, bOff});

                VReg currKey = currentFn->newVReg(idxReg.type);
                emit(Instruction{Op::Load, currKey, bAddr, {}, 0});

                VReg keyMatch = currentFn->newVReg(Type::I8);
                emit(Instruction{Op::Eq, keyMatch, currKey, idxReg});

                Block* foundBlock = currentFn->newBlock("map_store_found");
                Instruction brMatch;
                brMatch.op = Op::Branch;
                brMatch.src1 = keyMatch;
                brMatch.target = foundBlock;
                brMatch.elseTgt = stepBlock;
                emit(std::move(brMatch));

                currentBlock = foundBlock;
                emit(Instruction{Op::Store, {}, bAddr, valReg, 8});
                Instruction jmpDone;
                jmpDone.op = Op::Jmp;
                jmpDone.target = doneBlock;
                emit(std::move(jmpDone));

                currentBlock = stepBlock;
                VReg one = currentFn->newVReg(Type::I64);
                emit(Instruction{Op::ConstInt, one, {}, {}, 1});
                VReg nextIdx = currentFn->newVReg(Type::I64);
                emit(Instruction{Op::Add, nextIdx, loopIdx, one});
                emit(Instruction{Op::Copy, loopIdx, nextIdx});

                Instruction jmpBack;
                jmpBack.op = Op::Jmp;
                jmpBack.target = condBlock;
                emit(std::move(jmpBack));

                currentBlock = appendBlock;
                // Append is reached directly from the loop condition, so it
                // must create its own constants instead of reusing values
                // defined in the loop body or step blocks.
                VReg appSixteen = currentFn->newVReg(Type::I64);
                emit(Instruction{Op::ConstInt, appSixteen, {}, {}, 16});
                VReg appOne = currentFn->newVReg(Type::I64);
                emit(Instruction{Op::ConstInt, appOne, {}, {}, 1});
                VReg appBOff = currentFn->newVReg(Type::I64);
                emit(Instruction{Op::Mul, appBOff, mapLen, appSixteen});
                VReg appBAddr = currentFn->newVReg(Type::Ptr);
                emit(Instruction{Op::Add, appBAddr, bucketsPtr, appBOff});

                emit(Instruction{Op::Store, {}, appBAddr, idxReg, 0});
                emit(Instruction{Op::Store, {}, appBAddr, valReg, 8});

                VReg newLen = currentFn->newVReg(Type::I64);
                emit(Instruction{Op::Add, newLen, mapLen, appOne});
                emit(Instruction{Op::Store, {}, collReg, newLen, 8});

                Instruction jmpAppendDone;
                jmpAppendDone.op = Op::Jmp;
                jmpAppendDone.target = doneBlock;
                emit(std::move(jmpAppendDone));

                currentBlock = doneBlock;
                return valReg;
            }

            // List store
            VReg dataPtr = currentFn->newVReg(Type::Ptr);
            emit(Instruction{Op::Load, dataPtr, collReg, {}, 0});
            VReg scaleReg = currentFn->newVReg(Type::I64);
            emit(Instruction{Op::ConstInt, scaleReg, {}, {}, 8});
            VReg byteOff = currentFn->newVReg(Type::I64);
            emit(Instruction{Op::Mul, byteOff, idxReg, scaleReg});
            VReg addrReg = currentFn->newVReg(Type::Ptr);
            emit(Instruction{Op::Add, addrReg, dataPtr, byteOff});
            emit(Instruction{Op::Store, {}, addrReg, valReg, 0});
            return valReg;
        }

        throw std::runtime_error("assignment to target not supported");
    }
    if (auto* expr = dynamic_cast<const ast::PrefixExpression*>(&e)) {
        VReg r = lowerExpression(*expr->right);
        VReg dst = currentFn->newVReg(r.type);
        if (expr->op == "-") {
            emit(Instruction{Op::Neg, dst, r});
        } else if (expr->op == "!") {
            emit(Instruction{Op::Not, dst, r});
        } else {
            throw std::runtime_error("unsupported prefix op " + expr->op);
        }
        return dst;
    }
    if (auto* expr = dynamic_cast<const ast::InfixExpression*>(&e)) {
        // Logical operators short-circuit, so the right operand must only be
        // lowered in the branch that actually evaluates it.
        if (expr->op == "and" || expr->op == "or") {
            VReg leftReg = lowerExpression(*expr->left);
            VReg dst = currentFn->newVReg(Type::I8);
            Block* rhsBlock = currentFn->newBlock(expr->op == "and" ? "and_rhs" : "or_rhs");
            Block* mergeBlock = currentFn->newBlock(expr->op == "and" ? "and_merge" : "or_merge");

            if (expr->op == "and") {
                // false -> merge with 0; true -> evaluate rhs.
                VReg zero = currentFn->newVReg(Type::I8);
                emit(Instruction{Op::ConstInt, zero, {}, {}, 0});
                emit(Instruction{Op::Copy, dst, zero});
                Instruction br;
                br.op = Op::Branch;
                br.src1 = leftReg;
                br.target = rhsBlock;
                br.elseTgt = mergeBlock;
                emit(std::move(br));
            } else {
                // true -> merge with 1; false -> evaluate rhs.
                VReg one = currentFn->newVReg(Type::I8);
                emit(Instruction{Op::ConstInt, one, {}, {}, 1});
                emit(Instruction{Op::Copy, dst, one});
                Instruction br;
                br.op = Op::Branch;
                br.src1 = leftReg;
                br.target = mergeBlock;
                br.elseTgt = rhsBlock;
                emit(std::move(br));
            }

            currentBlock = rhsBlock;
            VReg rightReg = lowerExpression(*expr->right);
            emit(Instruction{Op::Copy, dst, rightReg});
            Instruction jmp;
            jmp.op = Op::Jmp;
            jmp.target = mergeBlock;
            emit(std::move(jmp));

            currentBlock = mergeBlock;
            return dst;
        }

        VReg leftReg = lowerExpression(*expr->left);
        VReg rightReg = lowerExpression(*expr->right);

        // User-defined operator: func +(a: Foo, b: Foo) -> Foo
        if (funcDecls.count(expr->op) &&
            (leftReg.type == Type::Ptr || rightReg.type == Type::Ptr)) {
            if (Function* opFn = resolveSpecialCall(expr->op, {leftReg, rightReg})) {
                VReg dst = currentFn->newVReg(opFn->returnType);
                Instruction callInst;
                callInst.op = Op::Call;
                callInst.dst = dst;
                callInst.callName = opFn->name;
                callInst.args = {leftReg, rightReg};
                emit(std::move(callInst));
                return dst;
            }
        }

        if (expr->op == "in") {
            const auto& chkTypes = checker.types();
            auto itT = chkTypes.find(expr->right.get());
            if (itT == chkTypes.end() || !itT->second) {
                throw std::runtime_error("membership test requires a list or map");
            }

            VReg dst = currentFn->newVReg(Type::I8);
            if (auto* listT = dynamic_cast<typecheck::ListType*>(itT->second.get())) {
                Type elemType = typeToIR(listT->elem());
                VReg dataPtr = currentFn->newVReg(Type::Ptr);
                emit(Instruction{Op::Load, dataPtr, rightReg, {}, 0});
                VReg lenReg = currentFn->newVReg(Type::I64);
                emit(Instruction{Op::Load, lenReg, rightReg, {}, 8});
                VReg idx = currentFn->newVReg(Type::I64);
                emit(Instruction{Op::ConstInt, idx, {}, {}, 0});
                VReg zero = currentFn->newVReg(Type::I8);
                emit(Instruction{Op::ConstInt, zero, {}, {}, 0});
                emit(Instruction{Op::Copy, dst, zero});

                Block* cond = currentFn->newBlock("in_list_cond");
                Block* body = currentFn->newBlock("in_list_body");
                Block* found = currentFn->newBlock("in_list_found");
                Block* step = currentFn->newBlock("in_list_step");
                Block* done = currentFn->newBlock("in_list_done");

                Instruction toCond;
                toCond.op = Op::Jmp;
                toCond.target = cond;
                emit(std::move(toCond));

                currentBlock = cond;
                VReg more = currentFn->newVReg(Type::I8);
                emit(Instruction{Op::Lt, more, idx, lenReg});
                Instruction br;
                br.op = Op::Branch;
                br.src1 = more;
                br.target = body;
                br.elseTgt = done;
                emit(std::move(br));

                currentBlock = body;
                VReg scale = currentFn->newVReg(Type::I64);
                emit(Instruction{Op::ConstInt, scale, {}, {}, 8});
                VReg off = currentFn->newVReg(Type::I64);
                emit(Instruction{Op::Mul, off, idx, scale});
                VReg addr = currentFn->newVReg(Type::Ptr);
                emit(Instruction{Op::Add, addr, dataPtr, off});
                VReg elem = currentFn->newVReg(elemType);
                emit(Instruction{Op::Load, elem, addr, {}, 0});
                VReg match = currentFn->newVReg(Type::I8);
                emit(Instruction{Op::Eq, match, elem, leftReg});
                Instruction brMatch;
                brMatch.op = Op::Branch;
                brMatch.src1 = match;
                brMatch.target = found;
                brMatch.elseTgt = step;
                emit(std::move(brMatch));

                currentBlock = found;
                VReg one = currentFn->newVReg(Type::I8);
                emit(Instruction{Op::ConstInt, one, {}, {}, 1});
                emit(Instruction{Op::Copy, dst, one});
                Instruction toDone;
                toDone.op = Op::Jmp;
                toDone.target = done;
                emit(std::move(toDone));

                currentBlock = step;
                VReg inc = currentFn->newVReg(Type::I64);
                emit(Instruction{Op::ConstInt, inc, {}, {}, 1});
                VReg next = currentFn->newVReg(Type::I64);
                emit(Instruction{Op::Add, next, idx, inc});
                emit(Instruction{Op::Copy, idx, next});
                Instruction back;
                back.op = Op::Jmp;
                back.target = cond;
                emit(std::move(back));

                currentBlock = done;
                return dst;
            }

            if (auto* mapT = dynamic_cast<typecheck::MapType*>(itT->second.get())) {
                Type keyType = typeToIR(mapT->key());
                VReg bucketsPtr = currentFn->newVReg(Type::Ptr);
                emit(Instruction{Op::Load, bucketsPtr, rightReg, {}, 0});
                VReg lenReg = currentFn->newVReg(Type::I64);
                emit(Instruction{Op::Load, lenReg, rightReg, {}, 8});
                VReg idx = currentFn->newVReg(Type::I64);
                emit(Instruction{Op::ConstInt, idx, {}, {}, 0});
                VReg zero = currentFn->newVReg(Type::I8);
                emit(Instruction{Op::ConstInt, zero, {}, {}, 0});
                emit(Instruction{Op::Copy, dst, zero});

                Block* cond = currentFn->newBlock("in_map_cond");
                Block* body = currentFn->newBlock("in_map_body");
                Block* found = currentFn->newBlock("in_map_found");
                Block* step = currentFn->newBlock("in_map_step");
                Block* done = currentFn->newBlock("in_map_done");

                Instruction toCond;
                toCond.op = Op::Jmp;
                toCond.target = cond;
                emit(std::move(toCond));

                currentBlock = cond;
                VReg more = currentFn->newVReg(Type::I8);
                emit(Instruction{Op::Lt, more, idx, lenReg});
                Instruction br;
                br.op = Op::Branch;
                br.src1 = more;
                br.target = body;
                br.elseTgt = done;
                emit(std::move(br));

                currentBlock = body;
                VReg scale = currentFn->newVReg(Type::I64);
                emit(Instruction{Op::ConstInt, scale, {}, {}, 16});
                VReg off = currentFn->newVReg(Type::I64);
                emit(Instruction{Op::Mul, off, idx, scale});
                VReg addr = currentFn->newVReg(Type::Ptr);
                emit(Instruction{Op::Add, addr, bucketsPtr, off});
                VReg key = currentFn->newVReg(keyType);
                emit(Instruction{Op::Load, key, addr, {}, 0});
                VReg match = currentFn->newVReg(Type::I8);
                emit(Instruction{Op::Eq, match, key, leftReg});
                Instruction brMatch;
                brMatch.op = Op::Branch;
                brMatch.src1 = match;
                brMatch.target = found;
                brMatch.elseTgt = step;
                emit(std::move(brMatch));

                currentBlock = found;
                VReg one = currentFn->newVReg(Type::I8);
                emit(Instruction{Op::ConstInt, one, {}, {}, 1});
                emit(Instruction{Op::Copy, dst, one});
                Instruction toDone;
                toDone.op = Op::Jmp;
                toDone.target = done;
                emit(std::move(toDone));

                currentBlock = step;
                VReg inc = currentFn->newVReg(Type::I64);
                emit(Instruction{Op::ConstInt, inc, {}, {}, 1});
                VReg next = currentFn->newVReg(Type::I64);
                emit(Instruction{Op::Add, next, idx, inc});
                emit(Instruction{Op::Copy, idx, next});
                Instruction back;
                back.op = Op::Jmp;
                back.target = cond;
                emit(std::move(back));

                currentBlock = done;
                return dst;
            }

            throw std::runtime_error("membership test requires a list or map");
        }

        Op op = Op::Nop;
        Type resType = leftReg.type;

        if (expr->op == "+") {
            const auto& chkTypes = checker.types();
            auto itL = chkTypes.find(expr->left.get());
            auto itR = chkTypes.find(expr->right.get());
            auto* listL = (itL != chkTypes.end()) ? dynamic_cast<typecheck::ListType*>(itL->second.get()) : nullptr;
            auto* listR = (itR != chkTypes.end()) ? dynamic_cast<typecheck::ListType*>(itR->second.get()) : nullptr;
            if (listL || listR) {
                Type elemType = listL ? typeToIR(listL->elem()) : typeToIR(listR->elem());
                return lowerListConcat(leftReg, rightReg, elemType);
            }
            auto isStringType = [&](const typecheck::TypePtr& t) {
                auto* basic = t ? dynamic_cast<typecheck::Basic*>(t.get()) : nullptr;
                return basic && basic->name() == "string";
            };
            bool strings = (itL != chkTypes.end() && isStringType(itL->second)) ||
                           (itR != chkTypes.end() && isStringType(itR->second));
            if (strings) {
                op = Op::ConcatStr;
                resType = Type::Ptr;
            } else {
                op = Op::Add;
            }
        } else if (expr->op == "-") {
            op = Op::Sub;
        } else if (expr->op == "*") {
            op = Op::Mul;
        } else if (expr->op == "/") {
            op = Op::Div;
        } else if (expr->op == "%") {
            op = Op::Mod;
        } else if (expr->op == "<<") {
            op = Op::Shl;
        } else if (expr->op == ">>") {
            op = Op::Shr;
        } else if (expr->op == "==" || expr->op == "===" || expr->op == "!=" || expr->op == "!==") {
            bool negate = expr->op == "!=" || expr->op == "!==";
            const auto& chkTypes = checker.types();
            auto itL = chkTypes.find(expr->left.get());
            auto itR = chkTypes.find(expr->right.get());
            auto* listL = (itL != chkTypes.end()) ? dynamic_cast<typecheck::ListType*>(itL->second.get()) : nullptr;
            auto* listR = (itR != chkTypes.end()) ? dynamic_cast<typecheck::ListType*>(itR->second.get()) : nullptr;
            if (listL || listR) {
                Type elemType = listL ? typeToIR(listL->elem()) : typeToIR(listR->elem());
                return lowerListEq(leftReg, rightReg, elemType, negate);
            }

            auto isStringType = [](const typecheck::TypePtr& t) {
                auto* basic = dynamic_cast<typecheck::Basic*>(t.get());
                return basic && basic->name() == "string";
            };
            if (itL != chkTypes.end() && itR != chkTypes.end() &&
                isStringType(itL->second) && isStringType(itR->second)) {
                return lowerStringEq(leftReg, rightReg, negate);
            }

            op = negate ? Op::Ne : Op::Eq;
            resType = Type::I8;
        } else if (expr->op == "<") {
            op = Op::Lt;
            resType = Type::I8;
        } else if (expr->op == "<=") {
            op = Op::Le;
            resType = Type::I8;
        } else if (expr->op == ">") {
            op = Op::Gt;
            resType = Type::I8;
        } else if (expr->op == ">=") {
            op = Op::Ge;
            resType = Type::I8;
        } else {
            throw std::runtime_error("unsupported infix op " + expr->op);
        }

        VReg dst = currentFn->newVReg(resType);
        emit(Instruction{op, dst, leftReg, rightReg});
        return dst;
    }
    if (auto* expr = dynamic_cast<const ast::CallExpression*>(&e)) {
        // 1. Stdlib module calls: module.func(...)
        if (auto* dot = dynamic_cast<ast::DotExpression*>(expr->function.get())) {
            if (auto* ident = dynamic_cast<ast::Identifier*>(dot->left.get())) {
                const std::string& modName = ident->value;
                const std::string& fnName = dot->field;

                // Check if this is a custom imported module function: modName.fnName
                std::string fullCustomName = modName + "." + fnName;
                auto customFnIt = functions.find(fullCustomName);
                if (customFnIt == functions.end()) {
                    customFnIt = functions.find(fnName);
                }
                if (customFnIt != functions.end()) {
                    std::vector<VReg> args;
                    for (const auto& a : expr->arguments) {
                        args.push_back(lowerExpression(*a));
                    }
                    Function* targetFn = customFnIt->second;
                    VReg dst = currentFn->newVReg(targetFn->returnType);
                    Instruction callInst;
                    callInst.op = Op::Call;
                    callInst.dst = dst;
                    callInst.callName = customFnIt->first;
                    callInst.args = std::move(args);
                    emit(std::move(callInst));
                    return dst;
                }

                if (modName == "cpu") {
                    std::vector<VReg> args;
                    for (const auto& a : expr->arguments) {
                        args.push_back(lowerExpression(*a));
                    }
                    if (fnName == "cli") {
                        emit(Instruction{Op::CpuCli});
                        VReg nullReg = currentFn->newVReg(Type::I64);
                        emit(Instruction{Op::ConstInt, nullReg, {}, {}, 0});
                        return nullReg;
                    }
                    if (fnName == "sti") {
                        emit(Instruction{Op::CpuSti});
                        VReg nullReg = currentFn->newVReg(Type::I64);
                        emit(Instruction{Op::ConstInt, nullReg, {}, {}, 0});
                        return nullReg;
                    }
                    if (fnName == "hlt") {
                        emit(Instruction{Op::CpuHlt});
                        VReg nullReg = currentFn->newVReg(Type::I64);
                        emit(Instruction{Op::ConstInt, nullReg, {}, {}, 0});
                        return nullReg;
                    }
                    if (fnName == "pause") {
                        emit(Instruction{Op::CpuPause});
                        VReg nullReg = currentFn->newVReg(Type::I64);
                        emit(Instruction{Op::ConstInt, nullReg, {}, {}, 0});
                        return nullReg;
                    }
                    if ((fnName == "in8" || fnName == "in16" || fnName == "in32") && args.size() == 1) {
                        int bits = (fnName == "in8") ? 8 : ((fnName == "in16") ? 16 : 32);
                        VReg dst = currentFn->newVReg(Type::I64);
                        Instruction inst;
                        inst.op = Op::CpuIn;
                        inst.dst = dst;
                        inst.src1 = args[0];
                        inst.intVal = bits;
                        emit(std::move(inst));
                        return dst;
                    }
                    if ((fnName == "out8" || fnName == "out16" || fnName == "out32") && args.size() == 2) {
                        int bits = (fnName == "out8") ? 8 : ((fnName == "out16") ? 16 : 32);
                        Instruction inst;
                        inst.op = Op::CpuOut;
                        inst.src1 = args[0];
                        inst.src2 = args[1];
                        inst.intVal = bits;
                        emit(std::move(inst));
                        VReg nullReg = currentFn->newVReg(Type::I64);
                        emit(Instruction{Op::ConstInt, nullReg, {}, {}, 0});
                        return nullReg;
                    }
                    if (fnName == "read_cr0" || fnName == "read_cr2" || fnName == "read_cr3") {
                        int crNum = (fnName == "read_cr0") ? 0 : ((fnName == "read_cr2") ? 2 : 3);
                        VReg dst = currentFn->newVReg(Type::I64);
                        Instruction inst;
                        inst.op = Op::CpuReadCr;
                        inst.dst = dst;
                        inst.intVal = crNum;
                        emit(std::move(inst));
                        return dst;
                    }
                    if ((fnName == "write_cr0" || fnName == "write_cr3") && args.size() == 1) {
                        int crNum = (fnName == "write_cr0") ? 0 : 3;
                        Instruction inst;
                        inst.op = Op::CpuWriteCr;
                        inst.src1 = args[0];
                        inst.intVal = crNum;
                        emit(std::move(inst));
                        VReg nullReg = currentFn->newVReg(Type::I64);
                        emit(Instruction{Op::ConstInt, nullReg, {}, {}, 0});
                        return nullReg;
                    }
                    if (fnName == "invlpg" && args.size() == 1) {
                        Instruction inst;
                        inst.op = Op::CpuInvlpg;
                        inst.src1 = args[0];
                        emit(std::move(inst));
                        VReg nullReg = currentFn->newVReg(Type::I64);
                        emit(Instruction{Op::ConstInt, nullReg, {}, {}, 0});
                        return nullReg;
                    }
                    if (fnName == "rdmsr" && args.size() == 1) {
                        VReg dst = currentFn->newVReg(Type::I64);
                        Instruction inst;
                        inst.op = Op::CpuRdmsr;
                        inst.dst = dst;
                        inst.src1 = args[0];
                        emit(std::move(inst));
                        return dst;
                    }
                    if (fnName == "wrmsr" && args.size() == 2) {
                        Instruction inst;
                        inst.op = Op::CpuWrmsr;
                        inst.src1 = args[0];
                        inst.src2 = args[1];
                        emit(std::move(inst));
                        VReg nullReg = currentFn->newVReg(Type::I64);
                        emit(Instruction{Op::ConstInt, nullReg, {}, {}, 0});
                        return nullReg;
                    }
                    if ((fnName == "cpuid" || fnName == "cpuid_eax" ||
                         fnName == "cpuid_ebx" || fnName == "cpuid_ecx" ||
                         fnName == "cpuid_edx") && args.size() == 2) {
                        int outReg = (fnName == "cpuid_ebx") ? 1 :
                                     ((fnName == "cpuid_ecx") ? 2 :
                                     ((fnName == "cpuid_edx") ? 3 : 0));
                        VReg dst = currentFn->newVReg(Type::I64);
                        Instruction inst;
                        inst.op = Op::CpuCpuid;
                        inst.dst = dst;
                        inst.src1 = args[0];
                        inst.src2 = args[1];
                        inst.intVal = outReg;
                        emit(std::move(inst));
                        return dst;
                    }
                    if (fnName == "lidt" && args.size() == 1) {
                        Instruction inst;
                        inst.op = Op::CpuLidt;
                        inst.src1 = args[0];
                        emit(std::move(inst));
                        VReg nullReg = currentFn->newVReg(Type::I64);
                        emit(Instruction{Op::ConstInt, nullReg, {}, {}, 0});
                        return nullReg;
                    }
                    if (fnName == "iret") {
                        emit(Instruction{Op::CpuIret});
                        VReg nullReg = currentFn->newVReg(Type::I64);
                        emit(Instruction{Op::ConstInt, nullReg, {}, {}, 0});
                        return nullReg;
                    }
                }

                if (modName == "mem") {
                    std::vector<VReg> args;
                    for (const auto& a : expr->arguments) {
                        args.push_back(lowerExpression(*a));
                    }
                    if (fnName == "free" && args.size() == 1) {
                        Instruction callInst;
                        callInst.op = Op::Call;
                        callInst.callName = "ulang_free";
                        callInst.args = {args[0]};
                        emit(std::move(callInst));
                        VReg done = currentFn->newVReg(Type::I8);
                        emit(Instruction{Op::ConstBool, done, {}, {}, 1});
                        return done;
                    }
                    if (fnName == "alloc" && args.size() == 1) {
                        VReg dst = currentFn->newVReg(Type::Ptr);
                        Instruction callInst;
                        callInst.op = Op::Call;
                        callInst.dst = dst;
                        callInst.callName = "ulang_alloc";
                        callInst.args = {args[0]};
                        emit(std::move(callInst));
                        return dst;
                    }
                    if (fnName == "read_u8" || fnName == "read_i8" ||
                        fnName == "read_u16" || fnName == "read_i16" ||
                        fnName == "read_u32" || fnName == "read_i32" ||
                        fnName == "read_u64" || fnName == "read_i64") {
                        int bits = (fnName.find("8") != std::string::npos) ? 8 :
                                   ((fnName.find("16") != std::string::npos) ? 16 :
                                   ((fnName.find("32") != std::string::npos) ? 32 : 64));
                        VReg addr = args[0];
                        if (args.size() == 2) {
                            VReg sum = currentFn->newVReg(Type::I64);
                            emit(Instruction{Op::Add, sum, args[0], args[1]});
                            addr = sum;
                        }
                        VReg dst = currentFn->newVReg(Type::I64);
                        Instruction inst;
                        inst.op = Op::RawLoad;
                        inst.dst = dst;
                        inst.src1 = addr;
                        inst.intVal = bits;
                        emit(std::move(inst));
                        return dst;
                    }
                    if (fnName == "write_u8" || fnName == "write_i8" ||
                        fnName == "write_u16" || fnName == "write_i16" ||
                        fnName == "write_u32" || fnName == "write_i32" ||
                        fnName == "write_u64" || fnName == "write_i64") {
                        int bits = (fnName.find("8") != std::string::npos) ? 8 :
                                   ((fnName.find("16") != std::string::npos) ? 16 :
                                   ((fnName.find("32") != std::string::npos) ? 32 : 64));
                        VReg addr = args[0];
                        VReg val = args[1];
                        if (args.size() == 3) {
                            // mem.write_u8(base, val, offset)
                            VReg sum = currentFn->newVReg(Type::I64);
                            emit(Instruction{Op::Add, sum, args[0], args[2]});
                            addr = sum;
                        }
                        Instruction inst;
                        inst.op = Op::RawStore;
                        inst.src1 = addr;
                        inst.src2 = val;
                        inst.intVal = bits;
                        emit(std::move(inst));
                        VReg trueReg = currentFn->newVReg(Type::I8);
                        emit(Instruction{Op::ConstInt, trueReg, {}, {}, 1});
                        return trueReg;
                    }
                }

                if (modName == "math") {
                    std::vector<VReg> args;
                    for (const auto& a : expr->arguments) {
                        args.push_back(lowerExpression(*a));
                    }

                    if (fnName == "min" && args.size() == 2) {
                        VReg dst = currentFn->newVReg(args[0].type);
                        VReg cmp = currentFn->newVReg(Type::I8);
                        emit(Instruction{Op::Lt, cmp, args[0], args[1]});
                        Block* thenB = currentFn->newBlock("min_then");
                        Block* elseB = currentFn->newBlock("min_else");
                        Block* mergeB = currentFn->newBlock("min_merge");

                        Instruction brInst;
                        brInst.op = Op::Branch;
                        brInst.src1 = cmp;
                        brInst.target = thenB;
                        brInst.elseTgt = elseB;
                        emit(std::move(brInst));

                        currentBlock = thenB;
                        emit(Instruction{Op::Copy, dst, args[0]});
                        Instruction j1; j1.op = Op::Jmp; j1.target = mergeB; emit(std::move(j1));

                        currentBlock = elseB;
                        emit(Instruction{Op::Copy, dst, args[1]});
                        Instruction j2; j2.op = Op::Jmp; j2.target = mergeB; emit(std::move(j2));

                        currentBlock = mergeB;
                        return dst;
                    }
                    if (fnName == "max" && args.size() == 2) {
                        VReg dst = currentFn->newVReg(args[0].type);
                        VReg cmp = currentFn->newVReg(Type::I8);
                        emit(Instruction{Op::Gt, cmp, args[0], args[1]});
                        Block* thenB = currentFn->newBlock("max_then");
                        Block* elseB = currentFn->newBlock("max_else");
                        Block* mergeB = currentFn->newBlock("max_merge");

                        Instruction brInst;
                        brInst.op = Op::Branch;
                        brInst.src1 = cmp;
                        brInst.target = thenB;
                        brInst.elseTgt = elseB;
                        emit(std::move(brInst));

                        currentBlock = thenB;
                        emit(Instruction{Op::Copy, dst, args[0]});
                        Instruction j1; j1.op = Op::Jmp; j1.target = mergeB; emit(std::move(j1));

                        currentBlock = elseB;
                        emit(Instruction{Op::Copy, dst, args[1]});
                        Instruction j2; j2.op = Op::Jmp; j2.target = mergeB; emit(std::move(j2));

                        currentBlock = mergeB;
                        return dst;
                    }
                    if (fnName == "abs" && args.size() == 1) {
                        VReg dst = currentFn->newVReg(args[0].type);
                        VReg zero = currentFn->newVReg(args[0].type);
                        emit(Instruction{Op::ConstInt, zero, {}, {}, 0});
                        VReg cmp = currentFn->newVReg(Type::I8);
                        emit(Instruction{Op::Lt, cmp, args[0], zero});
                        Block* thenB = currentFn->newBlock("abs_neg");
                        Block* elseB = currentFn->newBlock("abs_pos");
                        Block* mergeB = currentFn->newBlock("abs_merge");

                        Instruction brInst;
                        brInst.op = Op::Branch;
                        brInst.src1 = cmp;
                        brInst.target = thenB;
                        brInst.elseTgt = elseB;
                        emit(std::move(brInst));

                        currentBlock = thenB;
                        emit(Instruction{Op::Neg, dst, args[0]});
                        Instruction j1; j1.op = Op::Jmp; j1.target = mergeB; emit(std::move(j1));

                        currentBlock = elseB;
                        emit(Instruction{Op::Copy, dst, args[0]});
                        Instruction j2; j2.op = Op::Jmp; j2.target = mergeB; emit(std::move(j2));

                        currentBlock = mergeB;
                        return dst;
                    }

                    auto toFloat = [&](VReg value) -> VReg {
                        if (value.type == Type::F64) return value;
                        VReg converted = currentFn->newVReg(Type::F64);
                        emit(Instruction{Op::IntToFloat, converted, value});
                        return converted;
                    };

                    if (args.size() == 1 &&
                        (fnName == "sqrt" || fnName == "sin" || fnName == "cos" ||
                         fnName == "tan" || fnName == "asin" || fnName == "acos" ||
                         fnName == "atan" || fnName == "log" || fnName == "log10" ||
                         fnName == "floor" || fnName == "ceil" || fnName == "round")) {
                        VReg input = toFloat(args[0]);
                        VReg dst = currentFn->newVReg(Type::F64);
                        Instruction inst;
                        inst.op = Op::MathUnary;
                        inst.dst = dst;
                        inst.src1 = input;
                        inst.callName = fnName;
                        emit(std::move(inst));
                        return dst;
                    }

                    if (args.size() == 2 && (fnName == "pow" || fnName == "hypot")) {
                        VReg left = toFloat(args[0]);
                        VReg right = toFloat(args[1]);
                        VReg dst = currentFn->newVReg(Type::F64);
                        Instruction inst;
                        inst.op = Op::MathBinary;
                        inst.dst = dst;
                        inst.src1 = left;
                        inst.src2 = right;
                        inst.callName = fnName;
                        emit(std::move(inst));
                        return dst;
                    }

                    if (fnName == "clamp" && args.size() == 3) {
                        Type ty = args[0].type;
                        VReg result = currentFn->newVReg(ty);
                        emit(Instruction{Op::Copy, result, args[0]});

                        Block* checkHigh = currentFn->newBlock("clamp_high");
                        Block* setLow = currentFn->newBlock("clamp_low");
                        Block* setHigh = currentFn->newBlock("clamp_set_high");
                        Block* done = currentFn->newBlock("clamp_done");

                        VReg below = currentFn->newVReg(Type::I8);
                        emit(Instruction{Op::Lt, below, result, args[1]});
                        Instruction br1;
                        br1.op = Op::Branch;
                        br1.src1 = below;
                        br1.target = setLow;
                        br1.elseTgt = checkHigh;
                        emit(std::move(br1));

                        currentBlock = setLow;
                        emit(Instruction{Op::Copy, result, args[1]});
                        Instruction j1;
                        j1.op = Op::Jmp;
                        j1.target = checkHigh;
                        emit(std::move(j1));

                        currentBlock = checkHigh;
                        VReg above = currentFn->newVReg(Type::I8);
                        emit(Instruction{Op::Gt, above, result, args[2]});
                        Instruction br2;
                        br2.op = Op::Branch;
                        br2.src1 = above;
                        br2.target = setHigh;
                        br2.elseTgt = done;
                        emit(std::move(br2));

                        currentBlock = setHigh;
                        emit(Instruction{Op::Copy, result, args[2]});
                        Instruction j2;
                        j2.op = Op::Jmp;
                        j2.target = done;
                        emit(std::move(j2));

                        currentBlock = done;
                        return result;
                    }

                    if (fnName == "deg2rad" && args.size() == 1) {
                        VReg input = toFloat(args[0]);
                        VReg pi = currentFn->newVReg(Type::F64);
                        Instruction piInst;
                        piInst.op = Op::ConstFloat;
                        piInst.dst = pi;
                        piInst.floatVal = 3.14159265358979323846;
                        emit(std::move(piInst));
                        VReg divisor = currentFn->newVReg(Type::F64);
                        Instruction divInst;
                        divInst.op = Op::ConstFloat;
                        divInst.dst = divisor;
                        divInst.floatVal = 180.0;
                        emit(std::move(divInst));
                        VReg factor = currentFn->newVReg(Type::F64);
                        emit(Instruction{Op::Div, factor, pi, divisor});
                        VReg dst = currentFn->newVReg(Type::F64);
                        emit(Instruction{Op::Mul, dst, input, factor});
                        return dst;
                    }

                    if (fnName == "rad2deg" && args.size() == 1) {
                        VReg input = toFloat(args[0]);
                        VReg pi = currentFn->newVReg(Type::F64);
                        Instruction piInst;
                        piInst.op = Op::ConstFloat;
                        piInst.dst = pi;
                        piInst.floatVal = 3.14159265358979323846;
                        emit(std::move(piInst));
                        VReg factor = currentFn->newVReg(Type::F64);
                        Instruction factorInst;
                        factorInst.op = Op::ConstFloat;
                        factorInst.dst = factor;
                        factorInst.floatVal = 180.0;
                        emit(std::move(factorInst));
                        VReg dst = currentFn->newVReg(Type::F64);
                        emit(Instruction{Op::Mul, dst, input, factor});
                        emit(Instruction{Op::Div, dst, dst, pi});
                        return dst;
                    }
                }

                if (modName == "string") {
                    std::vector<VReg> args;
                    for (const auto& a : expr->arguments) {
                        args.push_back(lowerExpression(*a));
                    }

                    if (fnName == "lower" && args.size() == 1) {
                        return emitStringTransform(args[0], false);
                    }
                    if (fnName == "upper" && args.size() == 1) {
                        return emitStringTransform(args[0], true);
                    }
                    if (fnName == "trim" && args.size() == 1) {
                        return emitStringTrim(args[0]);
                    }
                    if (fnName == "contains" && args.size() == 2) {
                        VReg zero = emitIntConst(0);
                        VReg foundIdx = emitStringSearch(args[0], args[1], zero);
                        VReg minusOne = emitIntConst(-1);
                        VReg dst = currentFn->newVReg(Type::I8);
                        emit(Instruction{Op::Ne, dst, foundIdx, minusOne});
                        return dst;
                    }
                    if (fnName == "index_of" && (args.size() == 2 || args.size() == 3)) {
                        VReg from = (args.size() == 3) ? args[2] : emitIntConst(0);
                        return emitStringSearch(args[0], args[1], from);
                    }
                    if (fnName == "starts_with" && args.size() == 2) {
                        VReg zero = emitIntConst(0);
                        VReg foundIdx = emitStringSearch(args[0], args[1], zero);
                        VReg dst = currentFn->newVReg(Type::I8);
                        emit(Instruction{Op::Eq, dst, foundIdx, zero});
                        return dst;
                    }
                    if (fnName == "ends_with" && args.size() == 2) {
                        VReg hay = emitResolveStr(args[0]);
                        VReg needle = emitResolveStr(args[1]);
                        VReg hayLen = currentFn->newVReg(Type::I64);
                        emit(Instruction{Op::Load, hayLen, hay, {}, 0});
                        VReg needleLen = currentFn->newVReg(Type::I64);
                        emit(Instruction{Op::Load, needleLen, needle, {}, 0});
                        VReg fits = currentFn->newVReg(Type::I8);
                        emit(Instruction{Op::Le, fits, needleLen, hayLen});

                        VReg dst = currentFn->newVReg(Type::I8);
                        VReg zero = emitIntConst(0);
                        emit(Instruction{Op::Copy, dst, zero});

                        Block* checkBlock = currentFn->newBlock("ends_check");
                        Block* falseBlock = currentFn->newBlock("ends_false");
                        Block* doneBlock = currentFn->newBlock("ends_done");
                        Instruction brFits;
                        brFits.op = Op::Branch;
                        brFits.src1 = fits;
                        brFits.target = checkBlock;
                        brFits.elseTgt = falseBlock;
                        emit(std::move(brFits));

                        currentBlock = checkBlock;
                        VReg from = currentFn->newVReg(Type::I64);
                        emit(Instruction{Op::Sub, from, hayLen, needleLen});
                        VReg foundIdx = emitStringSearch(args[0], args[1], from);
                        VReg matches = currentFn->newVReg(Type::I8);
                        emit(Instruction{Op::Eq, matches, foundIdx, from});
                        emit(Instruction{Op::Copy, dst, matches});
                        Instruction toDone;
                        toDone.op = Op::Jmp;
                        toDone.target = doneBlock;
                        emit(std::move(toDone));

                        currentBlock = falseBlock;
                        emit(Instruction{Op::Copy, dst, zero});
                        Instruction toDoneFalse;
                        toDoneFalse.op = Op::Jmp;
                        toDoneFalse.target = doneBlock;
                        emit(std::move(toDoneFalse));

                        currentBlock = doneBlock;
                        return dst;
                    }
                    if (fnName == "substring" && (args.size() == 2 || args.size() == 3)) {
                        VReg start = args[1];
                        VReg count;
                        if (args.size() == 3) {
                            count = args[2];
                        } else {
                            VReg resolved = emitResolveStr(args[0]);
                            VReg total = currentFn->newVReg(Type::I64);
                            emit(Instruction{Op::Load, total, resolved, {}, 0});
                            count = currentFn->newVReg(Type::I64);
                            emit(Instruction{Op::Sub, count, total, start});
                        }
                        return emitSubstring(args[0], start, count);
                    }
                    if (fnName == "length" && args.size() == 1) {
                        VReg resolved = emitResolveStr(args[0]);
                        VReg dst = currentFn->newVReg(Type::I64);
                        emit(Instruction{Op::Load, dst, resolved, {}, 0});
                        return dst;
                    }
                    if (fnName == "split" && args.size() == 2) {
                        return emitStringSplit(args[0], args[1]);
                    }
                    if (fnName == "join" && args.size() == 2) {
                        return emitStringJoin(args[0], args[1]);
                    }
                    if (fnName == "replace" && args.size() == 3) {
                        VReg parts = emitStringSplit(args[0], args[1]);
                        return emitStringJoin(parts, args[2]);
                    }
                }
            }

            // Method call: receiver.method(...)
            VReg rcvReg = lowerExpression(*dot->left);
            std::string typeName;
            const auto& chkTypes = checker.types();
            auto itT = chkTypes.find(dot->left.get());
            if (itT != chkTypes.end()) {
                if (auto* st = dynamic_cast<typecheck::StructType*>(itT->second.get())) {
                    typeName = st->name;
                }
            }
            std::string mFullName = typeName + "." + dot->field;
            auto fIt = functions.find(mFullName);
            if (fIt == functions.end()) {
                auto lIt = structLayouts.find(typeName);
                if (lIt != structLayouts.end()) {
                    for (std::string cur = lIt->second.name; !cur.empty(); ) {
                        auto curIt = functions.find(cur + "." + dot->field);
                        if (curIt != functions.end()) {
                            fIt = curIt;
                            mFullName = cur + "." + dot->field;
                            break;
                        }
                        break;
                    }
                }
            }
            if (fIt == functions.end()) {
                throw std::runtime_error("method " + dot->field + " not found on " + typeName);
            }

            std::vector<VReg> callArgs;
            callArgs.push_back(rcvReg); // self
            for (const auto& a : expr->arguments) {
                callArgs.push_back(lowerExpression(*a));
            }

            Function* targetFn = fIt->second;
            VReg dst = currentFn->newVReg(targetFn->returnType);
            Instruction callInst;
            callInst.op = Op::Call;
            callInst.dst = dst;
            callInst.callName = mFullName;
            callInst.args = std::move(callArgs);
            emit(std::move(callInst));
            return dst;
        }

        // Indirect call through any expression evaluating to a function
        // address: table[i](), obj.callback(), etc.
        if (!dynamic_cast<ast::Identifier*>(expr->function.get())) {
            VReg fnPtr = lowerExpression(*expr->function);
            std::vector<VReg> indArgs;
            for (const auto& a : expr->arguments) indArgs.push_back(lowerExpression(*a));
            VReg dst = currentFn->newVReg(Type::I64);
            Instruction callInst;
            callInst.op = Op::CallPtr;
            callInst.src1 = fnPtr;
            callInst.dst = dst;
            callInst.args = std::move(indArgs);
            emit(std::move(callInst));
            return dst;
        }

        // Direct function call
        auto* fnIdent = dynamic_cast<ast::Identifier*>(expr->function.get());
        if (!fnIdent) {
            throw std::runtime_error("indirect calls not supported yet");
        }

        const std::string& fnName = fnIdent->value;

        // Struct/Class instantiation via CallExpression
        if (structLayouts.find(fnName) != structLayouts.end()) {
            ast::StructInstantiation si;
            si.token = expr->token;
            si.name = fnName;
            for (const auto& a : expr->arguments) {
                si.args.push_back(a);
            }
            return lowerExpression(si);
        }

        std::vector<VReg> args;
        for (const auto& a : expr->arguments) {
            args.push_back(lowerExpression(*a));
        }

        if (fnName == "exit" && args.size() == 1) {
            emit(Instruction{Op::Ret, {}, args[0]});
            return VReg{-1, Type::Void};
        }

        if (fnName == "print") {
            for (const auto& a : args) {
                VReg argToPrint = a;
                if (a.type == Type::I8) {
                    argToPrint = lowerBoolToStr(a);
                } else if (a.type == Type::I64 || a.type == Type::F64) {
                    VReg strReg = currentFn->newVReg(Type::Ptr);
                    emit(Instruction{Op::IntToStr, strReg, a});
                    argToPrint = strReg;
                }
                emit(Instruction{Op::Print, {}, argToPrint});
            }
            return VReg{-1, Type::Void};
        }

        if (fnName == "println") {
            for (const auto& a : args) {
                VReg argToPrint = a;
                if (a.type == Type::I8) {
                    argToPrint = lowerBoolToStr(a);
                } else if (a.type == Type::I64 || a.type == Type::F64) {
                    VReg strReg = currentFn->newVReg(Type::Ptr);
                    emit(Instruction{Op::IntToStr, strReg, a});
                    argToPrint = strReg;
                }
                emit(Instruction{Op::Println, {}, argToPrint});
            }
            if (args.empty()) {
                VReg emptyStr = currentFn->newVReg(Type::Ptr);
                Instruction cs;
                cs.op = Op::ConstStr;
                cs.dst = emptyStr;
                cs.strVal = "";
                emit(std::move(cs));
                emit(Instruction{Op::Println, {}, emptyStr});
            }
            return VReg{-1, Type::Void};
        }

        if (fnName == "len" && args.size() == 1) {
            int64_t offset = 0; // string len at header+0
            VReg src = args[0];
            const auto& chkTypes = checker.types();
            auto itT = chkTypes.find(expr->arguments[0].get());
            if (itT != chkTypes.end()) {
                if (dynamic_cast<typecheck::ListType*>(itT->second.get()) ||
                    dynamic_cast<typecheck::MapType*>(itT->second.get())) {
                    offset = 8;
                } else if (auto* basic = dynamic_cast<typecheck::Basic*>(itT->second.get())) {
                    if (basic->name() == "string") {
                        src = emitResolveStr(src);
                        offset = 0;
                    }
                }
            }
            VReg lenDst = currentFn->newVReg(Type::I64);
            emit(Instruction{Op::Load, lenDst, src, {}, offset});
            return lenDst;
        }

        if (fnName == "int" && args.size() == 1) {
            if (auto* lit = dynamic_cast<const ast::StringLiteral*>(expr->arguments[0].get())) {
                try {
                    int64_t value = std::stoll(lit->value);
                    VReg dst = currentFn->newVReg(Type::I64);
                    emit(Instruction{Op::ConstInt, dst, {}, {}, value});
                    return dst;
                } catch (...) {
                    // Fall through to the runtime conversion path.
                }
            }
            bool argIsString = false;
            const auto& chkTypes = checker.types();
            auto itArg = chkTypes.find(expr->arguments[0].get());
            if (itArg != chkTypes.end()) {
                auto* basic = dynamic_cast<typecheck::Basic*>(itArg->second.get());
                argIsString = basic && basic->name() == "string";
            }
            if (argIsString) {
                VReg dst = currentFn->newVReg(Type::I64);
                Instruction call;
                call.op = Op::Call;
                call.dst = dst;
                call.callName = "ulang_str_to_int";
                call.args = {args[0]};
                emit(std::move(call));
                return dst;
            }
            if (args[0].type == Type::F64) {
                VReg dst = currentFn->newVReg(Type::I64);
                emit(Instruction{Op::FloatToInt, dst, args[0]});
                return dst;
            }
            return args[0];
        }
        if (fnName == "float" && args.size() == 1) {
            if (auto* lit = dynamic_cast<const ast::StringLiteral*>(expr->arguments[0].get())) {
                try {
                    double value = std::stod(lit->value);
                    VReg dst = currentFn->newVReg(Type::F64);
                    Instruction inst;
                    inst.op = Op::ConstFloat;
                    inst.dst = dst;
                    inst.floatVal = value;
                    emit(std::move(inst));
                    return dst;
                } catch (...) {
                    // Fall through to the runtime conversion path.
                }
            }
            if (args[0].type == Type::F64) {
                return args[0];
            }
            VReg dst = currentFn->newVReg(Type::F64);
            emit(Instruction{Op::IntToFloat, dst, args[0]});
            return dst;
        }
        if (fnName == "string" && args.size() == 1) {
            if (args[0].type == Type::Ptr) {
                return args[0];
            }
            VReg dst = currentFn->newVReg(Type::Ptr);
            emit(Instruction{Op::IntToStr, dst, args[0]});
            return dst;
        }

        // Indirect call through a function-valued local/global: f := bump; f()
        if (!functions.count(fnName)) {
            auto lit = locals.find(fnName);
            if (lit != locals.end() &&
                (lit->second.type == Type::Ptr || lit->second.type == Type::I64)) {
                VReg dst = currentFn->newVReg(Type::I64);
                Instruction callInst;
                callInst.op = Op::CallPtr;
                callInst.src1 = lit->second;
                callInst.dst = dst;
                callInst.args = std::move(args);
                emit(std::move(callInst));
                return dst;
            }
        }

        // Generic / overloaded call: infer a specialization from argument types.
        if (!functions.count(fnName)) {
            if (Function* sp = resolveSpecialCall(fnName, args)) {
                VReg dst = currentFn->newVReg(sp->returnType);
                Instruction callInst;
                callInst.op = Op::Call;
                callInst.dst = dst;
                callInst.callName = sp->name;
                callInst.args = args;
                emit(std::move(callInst));
                return dst;
            }
        }

        Type retType = Type::Void;
        auto fIt = functions.find(fnName);
        if (fIt != functions.end()) {
            retType = fIt->second->returnType;
        }

        VReg dst = currentFn->newVReg(retType);
        Instruction callInst;
        callInst.op = Op::Call;
        callInst.dst = dst;
        callInst.callName = fnName;
        callInst.args = std::move(args);
        emit(std::move(callInst));
        return dst;
    }
    if (auto* expr = dynamic_cast<const ast::ErrExpression*>(&e)) {
        VReg msgReg;
        if (expr->message) {
            msgReg = lowerExpression(*expr->message);
        } else {
            msgReg = currentFn->newVReg(Type::Ptr);
            Instruction cs;
            cs.op = Op::ConstStr;
            cs.dst = msgReg;
            cs.strVal = "error";
            emit(std::move(cs));
        }

        VReg codeReg;
        if (expr->code) {
            codeReg = lowerExpression(*expr->code);
        } else {
            codeReg = currentFn->newVReg(Type::I64);
            emit(Instruction{Op::ConstInt, codeReg, {}, {}, 500});
        }

        VReg errObj = currentFn->newVReg(Type::Ptr);
        VReg sizeReg = currentFn->newVReg(Type::I64);
        emit(Instruction{Op::ConstInt, sizeReg, {}, {}, 16});
        emit(Instruction{Op::Alloc, errObj, sizeReg});

        emit(Instruction{Op::Store, {}, errObj, codeReg, 0});
        emit(Instruction{Op::Store, {}, errObj, msgReg, 8});

        emit(Instruction{Op::SetErr, {}, errObj});
        return errObj;
    }

    throw std::runtime_error("expression not supported in native backend yet");
}

} // namespace

LowerResult lower(const std::shared_ptr<ast::Program>& prog, const typecheck::Checker& checker) {
    try {
        LoweringContext ctx(checker);
        ctx.buildLayouts(*prog);
        ctx.collectModuleConstants(*prog);

        // 2a. Collect function declarations.  Overloaded and generic functions
        // are not declared eagerly; they are monomorphized at call sites.
        for (const auto& stmt : prog->statements) {
            if (auto* s = dynamic_cast<ast::FuncDeclaration*>(stmt.get())) {
                ctx.funcDecls[s->name].push_back(s);
                if (s->isDestructor) ctx.destructors[s->name] = s;
            }
        }

        // 2b. Declare plain functions and class methods
        for (const auto& stmt : prog->statements) {
            if (auto* s = dynamic_cast<ast::FuncDeclaration*>(stmt.get())) {
                auto it = ctx.funcDecls.find(s->name);
                if (it->second.size() == 1 && s->typeParams.empty()) {
                    ctx.declareFunc(s->name, s->params, s->returnType.get());
                }
            } else if (auto* s = dynamic_cast<ast::ClassDeclaration*>(stmt.get())) {
                for (const auto& m : s->methods) {
                    if (m && m->func) {
                        std::string mName = s->name + "." + m->func->name;
                        ctx.declareMethod(mName, m->func->params, m->func->returnType.get());
                    }
                }
            }
        }

        // 3. Lower top-level functions and class methods
        for (const auto& stmt : prog->statements) {
            if (auto* s = dynamic_cast<ast::FuncDeclaration*>(stmt.get())) {
                auto fIt = ctx.functions.find(s->name);
                if (fIt == ctx.functions.end()) continue; // generic/overloaded
                Function* irFn = fIt->second;
                ctx.lowerFunctionBody(irFn, s->params, s->body->statements, false);
            } else if (auto* s = dynamic_cast<ast::ClassDeclaration*>(stmt.get())) {
                for (const auto& m : s->methods) {
                    if (m && m->func) {
                        std::string mName = s->name + "." + m->func->name;
                        Function* irFn = ctx.functions[mName];
                        ctx.lowerFunctionBody(irFn, m->func->params, m->func->body->statements, true);
                    }
                }
            }
        }

        // 4. Entry point (main, or kernel_main for freestanding kernels)
        Function* userMain = nullptr;
        auto itMain = ctx.functions.find("main");
        if (itMain != ctx.functions.end()) {
            userMain = itMain->second;
        } else {
            auto itKernel = ctx.functions.find("kernel_main");
            if (itKernel != ctx.functions.end()) {
                userMain = itKernel->second;
            }
        }

        auto entryFn = std::make_unique<Function>();
        entryFn->name = userMain ? "__ulang_top_level" : "main";
        entryFn->returnType = Type::I64;
        Function* entryPtr = entryFn.get();
        ctx.currentFn = entryPtr;
        ctx.locals.clear();

        Block* entryBlock = entryPtr->newBlock("entry");
        ctx.currentBlock = entryBlock;

        for (const auto& stmt : prog->statements) {
            if (dynamic_cast<ast::FuncDeclaration*>(stmt.get()) ||
                dynamic_cast<ast::ClassDeclaration*>(stmt.get()) ||
                dynamic_cast<ast::StructDeclaration*>(stmt.get())) {
                continue;
            }
            // Mutable globals live in the data section with their static
            // initializer; do not also create an entry-only local for them.
            if (auto* vd = dynamic_cast<ast::VarDeclaration*>(stmt.get())) {
                if (vd->isConstexpr || (vd->name && ctx.globalIndex.count(vd->name->value))) {
                    // Literal initializers are baked into the data section;
                    // computed ones must still run in the entry block.
                    if (vd->value && !LoweringContext::isConstInitializer(*vd->value)) {
                        VReg val = ctx.lowerExpression(*vd->value);
                        Instruction in;
                        in.op = Op::StoreGlobal;
                        in.src1 = val;
                        in.callName = vd->name->value;
                        in.intVal = 8;
                        ctx.emit(std::move(in));
                    }
                    continue;
                }
            }
            ctx.lowerStatement(*stmt);
        }

        if (userMain) {
            // Call user's func main()
            Instruction callInst;
            callInst.op = Op::Call;
            callInst.callName = userMain->name;
            ctx.emit(std::move(callInst));
        }

        if (ctx.currentBlock && !ctx.isTerminated(ctx.currentBlock)) {
            VReg zeroReg = entryPtr->newVReg(Type::I64);
            ctx.emit(Instruction{Op::ConstInt, zeroReg, {}, {}, 0});
            ctx.emit(Instruction{Op::Ret, {}, zeroReg});
        }

        ctx.irProg->entryFn = entryPtr;
        ctx.irProg->functions.push_back(std::move(entryFn));

        LowerResult res;
        res.program = std::move(ctx.irProg);
        return res;
    } catch (const std::exception& ex) {
        LowerResult res;
        res.error = ex.what();
        return res;
    }
}

} // namespace ulang::ir
