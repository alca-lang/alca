// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

#include "ir/lower.h"
#include <unordered_map>
#include <vector>
#include <stdexcept>
#include <sstream>

namespace pawl::ir {

namespace {

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

    Type annotationToIR(const std::string& typeName) const {
        if (typeName == "int") return Type::I64;
        if (typeName == "float") return Type::F64;
        if (typeName == "bool") return Type::I8;
        if (typeName == "void") return Type::Void;
        return Type::Ptr;
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
            if (irFn->returnType == Type::Void) {
                emit(Instruction{Op::Ret, {}, VReg{-1, Type::Void}});
            } else {
                VReg zero = irFn->newVReg(irFn->returnType);
                emit(Instruction{Op::ConstInt, zero, {}, {}, 0});
                emit(Instruction{Op::Ret, {}, zero});
            }
        }
    }

    void lowerStatement(const ast::Statement& stmt);
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
};

void LoweringContext::lowerStatement(const ast::Statement& stmt) {
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
    } else if (auto* s = dynamic_cast<const ast::ReturnStatement*>(&stmt)) {
        if (s->value) {
            VReg valReg = lowerExpression(*s->value);
            emit(Instruction{Op::Ret, {}, valReg});
        } else {
            emit(Instruction{Op::Ret, {}, VReg{-1, Type::Void}});
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
        for (const auto& inner : s->statements) {
            lowerStatement(*inner);
        }
    } else if (auto* s = dynamic_cast<const ast::IfStatement*>(&stmt)) {
        lowerIfStatement(*s);
    } else if (auto* s = dynamic_cast<const ast::WhileStatement*>(&stmt)) {
        lowerWhileStatement(*s);
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

VReg LoweringContext::lowerStringEq(VReg left, VReg right, bool negate) {
    VReg dst = currentFn->newVReg(Type::I8);
    Instruction call;
    call.op = Op::Call;
    call.dst = dst;
    call.callName = "pawl_str_eq";
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

VReg LoweringContext::lowerExpression(const ast::Expression& e) {
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
        Type elemIRType = Type::I64;

        const auto& chkTypes = checker.types();
        auto itColl = chkTypes.find(expr->left.get());
        if (itColl != chkTypes.end()) {
            if (dynamic_cast<typecheck::MapType*>(itColl->second.get())) {
                isMap = true;
            }
        }
        auto itElem = chkTypes.find(&e);
        if (itElem != chkTypes.end()) {
            elemIRType = typeToIR(itElem->second);
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
            if (leftReg.type == Type::Ptr || rightReg.type == Type::Ptr) {
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
            int64_t offset = 0; // string len
            const auto& chkTypes = checker.types();
            auto itT = chkTypes.find(expr->arguments[0].get());
            if (itT != chkTypes.end()) {
                if (dynamic_cast<typecheck::ListType*>(itT->second.get()) ||
                    dynamic_cast<typecheck::MapType*>(itT->second.get())) {
                    offset = 8;
                }
            }
            VReg lenDst = currentFn->newVReg(Type::I64);
            emit(Instruction{Op::Load, lenDst, args[0], {}, offset});
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
                call.callName = "pawl_str_to_int";
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

        // 2. Declare top-level functions and class methods
        for (const auto& stmt : prog->statements) {
            if (auto* s = dynamic_cast<ast::FuncDeclaration*>(stmt.get())) {
                ctx.declareFunc(s->name, s->params, s->returnType.get());
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
                Function* irFn = ctx.functions[s->name];
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

        // 4. Entry point (main)
        auto entryFn = std::make_unique<Function>();
        entryFn->name = "main";
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
            ctx.lowerStatement(*stmt);
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

} // namespace pawl::ir
