// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

#include "typecheck/typecheck.h"
#include "dunder/dunder.h"
#include "mod/mod.h"

#include <algorithm>
#include <sstream>

namespace pawl::typecheck {

std::pair<std::shared_ptr<Symbol>, TypePtr> Scope::lookup(const std::string& name) {
    for (Scope* sc = this; sc != nullptr; sc = sc->outer_.get()) {
        auto itNarrow = sc->narrowed.find(name);
        if (itNarrow != sc->narrowed.end()) {
            auto sym = sc->lookupLocalChain(name);
            if (sym) return {sym, itNarrow->second};
        }
        auto itSym = sc->syms.find(name);
        if (itSym != sc->syms.end()) {
            return {itSym->second, itSym->second->typ};
        }
    }
    return {nullptr, nullptr};
}

std::shared_ptr<Symbol> Scope::lookupLocalChain(const std::string& name) {
    for (Scope* sc = this; sc != nullptr; sc = sc->outer_.get()) {
        auto it = sc->syms.find(name);
        if (it != sc->syms.end()) return it->second;
    }
    return nullptr;
}

void Scope::clearNarrow(const std::string& name) {
    for (Scope* sc = this; sc != nullptr; sc = sc->outer_.get()) {
        sc->narrowed.erase(name);
        if (sc->syms.find(name) != sc->syms.end()) return;
    }
}

std::vector<std::string> Scope::names() const {
    std::vector<std::string> out;
    for (const Scope* sc = this; sc != nullptr; sc = sc->outer_.get()) {
        for (const auto& pair : sc->syms) {
            out.push_back(pair.first);
        }
    }
    return out;
}

Checker::Checker(std::string filename, std::string source)
    : filename_(std::move(filename)), source_(std::move(source)) {
    global_ = std::make_shared<Scope>();
}

TypePtr Checker::typeOf(const ast::ExpressionPtr& e) const {
    if (!e) return nullptr;
    auto it = types_.find(e.get());
    if (it != types_.end()) return it->second;
    return nullptr;
}

void Checker::addError(const token::Token& tok, const std::string& msg, const std::string& suggestion) {
    int length = static_cast<int>(tok.literal.length());
    if (length <= 0) length = 1;
    auto err = std::make_shared<diag::SyntaxError>();
    err->filename = filename_;
    err->line = tok.line;
    err->column = tok.column;
    err->message = msg;
    err->token = tok.literal;
    err->length = length;
    err->suggestion = suggestion;
    errors_.push_back(err);
}

void Checker::initGlobal() {
    registerType("int", getInt());
    registerType("float", getFloat());
    registerType("string", getString());
    registerType("bool", getBool());
    registerType("null", getNull());
    registerType("void", getVoid());
    registerType("blob", getBlob());
    registerType("secret", getSecret());
    registerType("json", getJSON());
    registerType("error", getErrorType());

    // Register built-in functions
    for (const auto& pair : globalBuiltins()) {
        auto sym = std::make_shared<Symbol>();
        sym->kind = SymKind::Builtin;
        sym->typ = pair.second;
        sym->declared = pair.second;
        global_->syms[pair.first] = sym;
    }

    // Register built-in functions that have dynamic/polymorphic signatures
    for (const auto& name : {"len", "keys", "values", "push", "append", "pop", "range", "list"}) {
        auto dynFunc = std::make_shared<FuncType>();
        dynFunc->dynamic = true;
        dynFunc->ret = getJSON();
        auto sym = std::make_shared<Symbol>();
        sym->kind = SymKind::Builtin;
        sym->typ = dynFunc;
        sym->declared = dynFunc;
        global_->syms[name] = sym;
    }

    // Register dunder constants
    for (const auto& pair : dunder::registry()) {
        auto sym = std::make_shared<Symbol>();
        sym->kind = SymKind::Var;
        sym->typ = parseType(pair.second.type);
        sym->declared = sym->typ;
        sym->isConst = true;
        global_->syms[pair.first] = sym;
    }

    // Register std module
    auto stdMod = stdNamespace();
    auto symStd = std::make_shared<Symbol>();
    symStd->kind = SymKind::Module;
    symStd->typ = stdMod;
    symStd->declared = stdMod;
    global_->syms["std"] = symStd;
}

void Checker::registerType(const std::string& name, const TypePtr& t) {
    userTypes_[name] = t;
    if (auto s = std::dynamic_pointer_cast<StructType>(t)) {
        structs_[name] = s;
    }
}

TypePtr Checker::resolve(const std::shared_ptr<ast::TypeAnnotation>& annot) {
    if (!annot) return getInvalid();
    std::vector<TypePtr> ts;
    for (const auto& typeStr : annot->types) {
        ts.push_back(parseType(typeStr, &userTypes_));
    }
    return newUnion(ts);
}

void Checker::hoist(const std::vector<ast::StatementPtr>& statements, const std::shared_ptr<Scope>& sc) {
    for (const auto& s : statements) {
        if (!s) continue;
        if (auto sd = std::dynamic_pointer_cast<ast::StructDeclaration>(s)) {
            if (hoistedStructs_.insert(sd.get()).second) {
                auto structType = std::make_shared<StructType>();
                structType->name = sd->name;
                registerType(sd->name, structType);

                auto sym = std::make_shared<Symbol>();
                sym->kind = SymKind::Struct;
                sym->typ = structType;
                sym->declared = structType;
                sc->syms[sd->name] = sym;
            }
        } else if (auto cd = std::dynamic_pointer_cast<ast::ClassDeclaration>(s)) {
            if (hoistedClasses_.insert(cd.get()).second) {
                auto classType = std::make_shared<StructType>();
                classType->name = cd->name;
                classType->isClass = true;
                registerType(cd->name, classType);

                auto sym = std::make_shared<Symbol>();
                sym->kind = SymKind::Class;
                sym->typ = classType;
                sym->declared = classType;
                sc->syms[cd->name] = sym;
            }
        } else if (auto ed = std::dynamic_pointer_cast<ast::EnumDeclaration>(s)) {
            auto enumType = std::make_shared<EnumType>();
            enumType->name = ed->name;
            for (const auto& v : ed->variants) {
                enumType->variants[v->name] = enumType;
            }
            registerType(ed->name, enumType);
            auto sym = std::make_shared<Symbol>();
            sym->kind = SymKind::Var;
            sym->typ = enumType;
            sym->declared = enumType;
            sc->syms[ed->name] = sym;
        } else if (auto fn = std::dynamic_pointer_cast<ast::FuncDeclaration>(s)) {
            auto sig = funcSig(fn, sc);
            auto sym = std::make_shared<Symbol>();
            sym->kind = SymKind::Func;
            sym->typ = sig;
            sym->declared = sig;
            sc->syms[fn->name] = sym;
        }
    }

    // Pass 2: populate fields
    for (const auto& s : statements) {
        if (!s) continue;
        if (auto sd = std::dynamic_pointer_cast<ast::StructDeclaration>(s)) {
            addTypeFields(sd, sc);
        } else if (auto cd = std::dynamic_pointer_cast<ast::ClassDeclaration>(s)) {
            if (hoistedClassFields_.insert(cd.get()).second) {
                auto classType = structs_[cd->name];
                if (!cd->parent.empty()) {
                    auto itP = structs_.find(cd->parent);
                    if (itP != structs_.end()) classType->parent = itP->second;
                }
                for (const auto& f : cd->fields) {
                    Field field;
                    field.name = f->name;
                    field.type = f->typeAnnotation ? resolve(f->typeAnnotation) : getAny();
                    field.hasDefault = (f->defaultValue != nullptr);
                    field.visibility = f->visibility;
                    field.isConst = f->isConst;
                    field.owner = classType;
                    classType->fields.push_back(field);
                }
                addClassMethods(cd, sc);
            }
        }
    }
}

void Checker::addTypeFields(const std::shared_ptr<ast::StructDeclaration>& decl, const std::shared_ptr<Scope>&) {
    auto structType = structs_[decl->name];
    if (!structType) return;
    structType->fields.clear();
    for (const auto& f : decl->fields) {
        Field field;
        field.name = f->name;
        field.type = f->typeAnnotation ? resolve(f->typeAnnotation) : getAny();
        field.hasDefault = (f->defaultValue != nullptr);
        field.visibility = f->visibility;
        field.isConst = f->isConst;
        field.owner = structType;
        structType->fields.push_back(field);
    }
}

void Checker::addClassMethods(const std::shared_ptr<ast::ClassDeclaration>& decl, const std::shared_ptr<Scope>& sc) {
    auto classType = structs_[decl->name];
    if (!classType) return;
    for (const auto& m : decl->methods) {
        if (!m || !m->func) continue;
        auto sig = funcSig(m->func, sc);
        if (auto fSig = std::dynamic_pointer_cast<FuncType>(sig)) {
            auto method = std::make_shared<Method>();
            method->name = m->func->name;
            method->signature = fSig;
            method->visibility = m->visibility;
            method->owner = classType;
            classType->methods[m->func->name] = method;
        }
    }
}

TypePtr Checker::funcSig(const std::shared_ptr<ast::FuncDeclaration>& fn, const std::shared_ptr<Scope>&) {
    auto f = std::make_shared<FuncType>();
    f->ret = fn->returnType ? resolve(fn->returnType) : getVoid();
    for (const auto& p : fn->params) {
        Param param;
        param.name = p->name;
        param.type = p->typeAnnotation ? resolve(p->typeAnnotation) : getAny();
        param.optional = (p->defaultValue != nullptr);
        f->params.push_back(param);
    }
    return f;
}

std::vector<std::shared_ptr<diag::SyntaxError>> Checker::check(const std::shared_ptr<ast::Program>& program) {
    initGlobal();
    hoist(program->statements, global_);
    stmts(program->statements, global_);
    return errors_;
}

void Checker::stmts(const std::vector<ast::StatementPtr>& statements, const std::shared_ptr<Scope>& sc) {
    for (const auto& s : statements) {
        stmt(s, sc);
    }
}

void Checker::block(const std::shared_ptr<ast::BlockStatement>& b, const std::shared_ptr<Scope>& sc) {
    if (!b) return;
    auto inner = std::make_shared<Scope>(sc);
    hoist(b->statements, inner);
    stmts(b->statements, inner);
}

void Checker::condition(const ast::ExpressionPtr& cond, const std::shared_ptr<Scope>& sc) {
    if (!cond) return;
    auto t = fit(cond, getBool(), sc);
    if (t != getBool() && t != getJSON() && t != getInvalid()) {
        addError(cond->tokenLiteral().empty() ? token::Token{} : token::Token{token::IDENT, cond->tokenLiteral(), 1, 1},
                 "condition must be bool, got " + t->toString(), "use a boolean expression");
    }
}

void Checker::stmt(const ast::StatementPtr& s, const std::shared_ptr<Scope>& sc) {
    if (!s) return;
    if (auto vd = std::dynamic_pointer_cast<ast::VarDeclaration>(s)) {
        varDecl(vd, sc);
    } else if (auto es = std::dynamic_pointer_cast<ast::ExpressionStatement>(s)) {
        expr(es->expression, sc);
    } else if (auto ifStmt = std::dynamic_pointer_cast<ast::IfStatement>(s)) {
        condition(ifStmt->condition, sc);
        block(ifStmt->consequence, sc);
        for (const auto& alt : ifStmt->alternatives) {
            if (alt) {
                condition(alt->condition, sc);
                block(alt->body, sc);
            }
        }
        if (ifStmt->elseBlock) block(ifStmt->elseBlock, sc);
    } else if (auto wh = std::dynamic_pointer_cast<ast::WhileStatement>(s)) {
        condition(wh->condition, sc);
        loop_++;
        block(wh->body, sc);
        loop_--;
    } else if (auto fs = std::dynamic_pointer_cast<ast::ForStatement>(s)) {
        auto iterT = expr(fs->iterable, sc);
        auto inner = std::make_shared<Scope>(sc);
        TypePtr elemT = getAny();
        if (auto lt = std::dynamic_pointer_cast<ListType>(iterT)) elemT = lt->elem();
        auto sym = std::make_shared<Symbol>();
        sym->kind = SymKind::Var;
        sym->typ = elemT;
        sym->declared = elemT;
        inner->syms[fs->variable] = sym;
        loop_++;
        block(fs->body, inner);
        loop_--;
    } else if (auto as = std::dynamic_pointer_cast<ast::AssertStatement>(s)) {
        condition(as->condition, sc);
    } else if (auto ret = std::dynamic_pointer_cast<ast::ReturnStatement>(s)) {
        TypePtr valT = ret->value ? expr(ret->value, sc) : getVoid();
        if (fn_) {
            if (!assignable(fn_->ret, valT)) {
                addError(ret->token, "cannot return " + valT->toString() + " from function returning " + fn_->ret->toString(),
                         "change the return value or the function return type");
            }
        }
    } else if (auto useStmt = std::dynamic_pointer_cast<ast::UseStatement>(s)) {
        std::string modPath = useStmt->path;
        if (modPath.rfind("std.", 0) == 0) modPath = modPath.substr(4);
        auto mod = moduleByName(modPath);
        std::string bindAs = useStmt->alias.empty() ? mod::bindingName(useStmt->path) : useStmt->alias;
        auto sym = std::make_shared<Symbol>();
        sym->kind = SymKind::Module;
        sym->typ = mod ? std::static_pointer_cast<Type>(mod) : std::static_pointer_cast<Type>(getJSON());
        sym->declared = sym->typ;
        sc->syms[bindAs] = sym;
    } else if (auto fn = std::dynamic_pointer_cast<ast::FuncDeclaration>(s)) {
        funcBody(fn, sc);
    }
}

void Checker::varDecl(const std::shared_ptr<ast::VarDeclaration>& vd, const std::shared_ptr<Scope>& sc) {
    if (!vd || !vd->name) return;
    std::string name = vd->name->value;

    TypePtr declT;
    if (vd->typeAnnotation) {
        declT = resolve(vd->typeAnnotation);
    }

    TypePtr valT;
    if (vd->value) {
        valT = declT ? fit(vd->value, declT, sc) : expr(vd->value, sc);
    } else {
        valT = declT ? declT : getInvalid();
    }

    TypePtr finalT = declT ? declT : valT;
    if (declT && vd->value && !assignable(declT, valT)) {
        addError(vd->token, "cannot assign " + valT->toString() + " to " + declT->toString(),
                 "types must match");
    }

    auto sym = std::make_shared<Symbol>();
    sym->kind = SymKind::Var;
    sym->typ = finalT;
    sym->declared = finalT;
    sym->isConst = vd->isConst;
    sc->syms[name] = sym;
}

void Checker::funcBody(const std::shared_ptr<ast::FuncDeclaration>& fn, const std::shared_ptr<Scope>& sc) {
    auto inner = std::make_shared<Scope>(sc);
    auto oldFn = fn_;
    auto fCtx = std::make_shared<FuncCtx>();
    fCtx->name = fn->name;
    fCtx->ret = fn->returnType ? resolve(fn->returnType) : getVoid();
    fn_ = fCtx;

    for (const auto& p : fn->params) {
        auto pType = p->typeAnnotation ? resolve(p->typeAnnotation) : getAny();
        auto sym = std::make_shared<Symbol>();
        sym->kind = SymKind::Param;
        sym->typ = pType;
        sym->declared = pType;
        inner->syms[p->name] = sym;
    }

    private_++;
    block(fn->body, inner);
    private_--;
    fn_ = oldFn;
}

TypePtr Checker::fit(const ast::ExpressionPtr& e, const TypePtr&, const std::shared_ptr<Scope>& sc) {
    return expr(e, sc);
}

TypePtr Checker::expr(const ast::ExpressionPtr& e, const std::shared_ptr<Scope>& sc) {
    if (!e) return getInvalid();
    auto t = expr0(e, sc);
    types_[e.get()] = t;
    return t;
}

TypePtr Checker::expr0(const ast::ExpressionPtr& e, const std::shared_ptr<Scope>& sc) {
    if (auto id = std::dynamic_pointer_cast<ast::Identifier>(e)) {
        return evalIdentifier(id, sc);
    }
    if (std::dynamic_pointer_cast<ast::IntegerLiteral>(e)) return getInt();
    if (std::dynamic_pointer_cast<ast::FloatLiteral>(e)) return getFloat();
    if (std::dynamic_pointer_cast<ast::StringLiteral>(e)) return getString();
    if (std::dynamic_pointer_cast<ast::BooleanLiteral>(e)) return getBool();
    if (std::dynamic_pointer_cast<ast::NullLiteral>(e)) return getNull();

    if (auto pre = std::dynamic_pointer_cast<ast::PrefixExpression>(e)) {
        auto rightT = expr(pre->right, sc);
        if (pre->op == "not" || pre->op == "!") return getBool();
        if (pre->op == "-" || pre->op == "+") {
            if (rightT == getInt() || rightT == getFloat()) return rightT;
            return getInt();
        }
        return rightT;
    }

    if (auto list = std::dynamic_pointer_cast<ast::ListLiteral>(e)) {
        return evalListLiteral(list, sc);
    }
    if (auto map = std::dynamic_pointer_cast<ast::MapLiteral>(e)) {
        return evalMapLiteral(map, sc);
    }
    if (auto in = std::dynamic_pointer_cast<ast::InfixExpression>(e)) {
        return evalInfix(in, sc);
    }
    if (auto as = std::dynamic_pointer_cast<ast::AssignExpression>(e)) {
        return evalAssign(as, sc);
    }
    if (auto idx = std::dynamic_pointer_cast<ast::IndexExpression>(e)) {
        return evalIndex(idx, sc);
    }
    if (auto dot = std::dynamic_pointer_cast<ast::DotExpression>(e)) {
        return evalDot(dot, sc);
    }
    if (auto call = std::dynamic_pointer_cast<ast::CallExpression>(e)) {
        return evalCall(call, sc);
    }
    if (auto errExp = std::dynamic_pointer_cast<ast::ErrExpression>(e)) {
        if (errExp->message) expr(errExp->message, sc);
        if (errExp->code) expr(errExp->code, sc);
        return getNever();
    }
    if (auto range = std::dynamic_pointer_cast<ast::RangeLiteral>(e)) {
        if (range->start) expr(range->start, sc);
        if (range->end) expr(range->end, sc);
        return std::make_shared<ListType>(getInt());
    }
    if (auto stream = std::dynamic_pointer_cast<ast::StreamExpression>(e)) {
        auto srcT = expr(stream->source, sc);
        if (stream->op == "?>>>") {
            if (stream->target) {
                expr(stream->target, sc);
            }
            return srcT;
        }
        if (stream->target) expr(stream->target, sc);
        return srcT;
    }
    return getJSON();
}

TypePtr Checker::evalIdentifier(const std::shared_ptr<ast::Identifier>& id, const std::shared_ptr<Scope>& sc) {
    auto [sym, t] = sc->lookup(id->value);
    if (!sym) {
        addError(id->token, "undefined variable \"" + id->value + "\"", "declare it before use");
        return getInvalid();
    }
    return t;
}

TypePtr Checker::evalListLiteral(const std::shared_ptr<ast::ListLiteral>& list, const std::shared_ptr<Scope>& sc) {
    if (list->elements.empty()) return std::make_shared<ListType>(getNever());
    std::vector<TypePtr> ts;
    for (const auto& elem : list->elements) {
        ts.push_back(expr(elem, sc));
    }
    return std::make_shared<ListType>(newUnion(ts));
}

TypePtr Checker::evalMapLiteral(const std::shared_ptr<ast::MapLiteral>& map, const std::shared_ptr<Scope>& sc) {
    if (map->pairs.empty()) return std::make_shared<MapType>(getNever(), getNever());
    std::vector<TypePtr> ks, vs;
    for (const auto& p : map->pairs) {
        if (p) {
            ks.push_back(expr(p->key, sc));
            vs.push_back(expr(p->value, sc));
        }
    }
    return std::make_shared<MapType>(newUnion(ks), newUnion(vs));
}

TypePtr Checker::evalInfix(const std::shared_ptr<ast::InfixExpression>& in, const std::shared_ptr<Scope>& sc) {
    auto lt = expr(in->left, sc);
    auto rt = expr(in->right, sc);

    const std::string& op = in->op;
    if (op == "==" || op == "!=" || op == "===" || op == "!==") return getBool();
    if (op == "<" || op == "<=" || op == ">" || op == ">=") return getBool();
    if (op == "and" || op == "or") return getBool();

    if (op == "+") {
        if (lt == getString() && rt == getString()) return getString();
        if (lt == getFloat() && rt == getFloat()) return getFloat();
        if (lt == getInt() && rt == getInt()) return getInt();
        if (lt == getString() || rt == getString()) return getString();
        return lt;
    }
    if (op == "-" || op == "*" || op == "%" || op == "**" || op == "/") {
        if (lt == getFloat() || rt == getFloat()) return getFloat();
        return getInt();
    }
    return getJSON();
}

TypePtr Checker::evalAssign(const std::shared_ptr<ast::AssignExpression>& as, const std::shared_ptr<Scope>& sc) {
    auto lt = expr(as->target, sc);
    auto rt = expr(as->value, sc);
    if (!assignable(lt, rt)) {
        addError(as->token, "cannot assign " + rt->toString() + " to " + lt->toString(), "types must match");
    }
    return lt;
}

TypePtr Checker::evalIndex(const std::shared_ptr<ast::IndexExpression>& idx, const std::shared_ptr<Scope>& sc) {
    auto lt = expr(idx->left, sc);
    auto it = expr(idx->index, sc);

    if (auto listT = std::dynamic_pointer_cast<ListType>(lt)) {
        return listT->elem();
    }
    if (auto mapT = std::dynamic_pointer_cast<MapType>(lt)) {
        return mapT->val();
    }
    if (lt == getString()) return getString();
    return getJSON();
}

TypePtr Checker::evalDot(const std::shared_ptr<ast::DotExpression>& dot, const std::shared_ptr<Scope>& sc) {
    auto lt = expr(dot->left, sc);
    if (auto mod = std::dynamic_pointer_cast<ModuleType>(lt)) {
        auto it = mod->members.find(dot->field);
        if (it != mod->members.end()) return it->second;
        addError(dot->token, "module \"" + mod->name + "\" has no member \"" + dot->field + "\"", "check spelling");
        return getInvalid();
    }
    if (auto en = std::dynamic_pointer_cast<EnumType>(lt)) {
        auto it = en->variants.find(dot->field);
        if (it != en->variants.end()) return it->second;
        addError(dot->token, "enum \"" + en->name + "\" has no variant \"" + dot->field + "\"", "check variant name");
        return getInvalid();
    }
    if (auto s = std::dynamic_pointer_cast<StructType>(lt)) {
        auto f = s->lookupField(dot->field);
        if (f) return f->type;
        auto m = s->lookupMethod(dot->field);
        if (m) return m->signature;
        addError(dot->token, "type \"" + s->name + "\" has no field \"" + dot->field + "\"", "check field name");
        return getInvalid();
    }
    return getJSON();
}

TypePtr Checker::evalCall(const std::shared_ptr<ast::CallExpression>& call, const std::shared_ptr<Scope>& sc) {
    auto fnT = expr(call->function, sc);
    for (const auto& arg : call->arguments) {
        expr(arg, sc);
    }
    for (const auto& pair : call->kwargs) {
        expr(pair.second, sc);
    }
    if (auto f = std::dynamic_pointer_cast<FuncType>(fnT)) {
        return f->ret ? f->ret : getVoid();
    }
    if (auto s = std::dynamic_pointer_cast<StructType>(fnT)) {
        return s;
    }
    if (auto ov = std::dynamic_pointer_cast<Overloads>(fnT)) {
        if (!ov->sigs.empty() && ov->sigs[0]->ret) return ov->sigs[0]->ret;
    }
    return getJSON();
}

} // namespace pawl::typecheck
