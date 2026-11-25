// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

#pragma once

#include <memory>
#include <string>
#include <unordered_map>
#include <unordered_set>
#include <vector>

#include "ast/ast.h"
#include "diag/diagnostic.h"
#include "token/token.h"
#include "typecheck/types.h"

namespace pawl::typecheck {

enum class SymKind {
    Var,
    Param,
    Func,
    Struct,
    Class,
    Enum,
    Interface,
    Module,
    Builtin
};

struct Symbol {
    SymKind kind = SymKind::Var;
    TypePtr typ;
    TypePtr declared; // declared type of a variable (typ may be narrowed)
    bool isConst = false;
};

class Scope {
public:
    explicit Scope(std::shared_ptr<Scope> outer = nullptr) : outer_(std::move(outer)) {}

    std::pair<std::shared_ptr<Symbol>, TypePtr> lookup(const std::string& name);
    std::shared_ptr<Symbol> lookupLocalChain(const std::string& name);
    void clearNarrow(const std::string& name);
    std::vector<std::string> names() const;

    std::unordered_map<std::string, std::shared_ptr<Symbol>> syms;
    std::unordered_map<std::string, TypePtr> narrowed;
    std::shared_ptr<Scope> outer_;
};

struct FuncCtx {
    std::string name;
    TypePtr ret;
};

class Checker {
public:
    Checker(std::string filename, std::string source);

    std::vector<std::shared_ptr<diag::SyntaxError>> check(const std::shared_ptr<ast::Program>& program);

    const std::vector<std::shared_ptr<diag::SyntaxError>>& errors() const { return errors_; }
    TypePtr typeOf(const ast::ExpressionPtr& e) const;
    const std::unordered_map<const ast::Expression*, TypePtr>& types() const { return types_; }

    void addError(const token::Token& tok, const std::string& msg, const std::string& suggestion);

private:
    void initGlobal();
    void hoist(const std::vector<ast::StatementPtr>& stmts, const std::shared_ptr<Scope>& sc);
    void registerType(const std::string& name, const TypePtr& t);
    void addTypeFields(const std::shared_ptr<ast::StructDeclaration>& decl, const std::shared_ptr<Scope>& sc);
    void addClassMethods(const std::shared_ptr<ast::ClassDeclaration>& decl, const std::shared_ptr<Scope>& sc);
    TypePtr funcSig(const std::shared_ptr<ast::FuncDeclaration>& fn, const std::shared_ptr<Scope>& sc);
    TypePtr resolve(const std::shared_ptr<ast::TypeAnnotation>& annot);

    void stmts(const std::vector<ast::StatementPtr>& stmts, const std::shared_ptr<Scope>& sc);
    void block(const std::shared_ptr<ast::BlockStatement>& b, const std::shared_ptr<Scope>& sc);
    void condition(const ast::ExpressionPtr& cond, const std::shared_ptr<Scope>& sc);
    void stmt(const ast::StatementPtr& s, const std::shared_ptr<Scope>& sc);
    void varDecl(const std::shared_ptr<ast::VarDeclaration>& vd, const std::shared_ptr<Scope>& sc);
    void funcBody(const std::shared_ptr<ast::FuncDeclaration>& fn, const std::shared_ptr<Scope>& sc);

    TypePtr fit(const ast::ExpressionPtr& e, const TypePtr& expected, const std::shared_ptr<Scope>& sc);
    TypePtr expr(const ast::ExpressionPtr& e, const std::shared_ptr<Scope>& sc);
    TypePtr expr0(const ast::ExpressionPtr& e, const std::shared_ptr<Scope>& sc);

    TypePtr evalIdentifier(const std::shared_ptr<ast::Identifier>& id, const std::shared_ptr<Scope>& sc);
    TypePtr evalListLiteral(const std::shared_ptr<ast::ListLiteral>& list, const std::shared_ptr<Scope>& sc);
    TypePtr evalMapLiteral(const std::shared_ptr<ast::MapLiteral>& map, const std::shared_ptr<Scope>& sc);
    TypePtr evalInfix(const std::shared_ptr<ast::InfixExpression>& in, const std::shared_ptr<Scope>& sc);
    TypePtr evalAssign(const std::shared_ptr<ast::AssignExpression>& as, const std::shared_ptr<Scope>& sc);
    TypePtr evalIndex(const std::shared_ptr<ast::IndexExpression>& idx, const std::shared_ptr<Scope>& sc);
    TypePtr evalDot(const std::shared_ptr<ast::DotExpression>& dot, const std::shared_ptr<Scope>& sc);
    TypePtr evalCall(const std::shared_ptr<ast::CallExpression>& call, const std::shared_ptr<Scope>& sc);

    std::string filename_;
    std::string source_;
    std::vector<std::shared_ptr<diag::SyntaxError>> errors_;
    std::unordered_map<std::string, std::shared_ptr<StructType>> structs_;
    std::unordered_map<std::string, TypePtr> userTypes_;
    std::unordered_map<const ast::Expression*, TypePtr> types_;
    std::shared_ptr<Scope> global_;
    std::shared_ptr<FuncCtx> fn_;
    std::shared_ptr<StructType> class_;
    int loop_ = 0;
    int private_ = 0;
    std::unordered_set<std::string> imported_;
    std::unordered_set<const ast::StructDeclaration*> hoistedStructs_;
    std::unordered_set<const ast::ClassDeclaration*> hoistedClasses_;
    std::unordered_set<const ast::ClassDeclaration*> hoistedClassFields_;
};

} // namespace pawl::typecheck
