// Pawl bootstrap compiler - frontend types (no Go).
// SPDX-License-Identifier: Apache-2.0
#pragma once

#include <cstdint>
#include <memory>
#include <stdexcept>
#include <string>
#include <utility>
#include <vector>

struct CompileError : std::runtime_error {
    explicit CompileError(const std::string& m) : std::runtime_error(m) {}
};

[[noreturn]] inline void fail(const std::string& m) { throw CompileError(m); }

enum class K {
    End, IntLit, StrLit, Ident,
    KwFn, KwIf, KwElse, KwWhile, KwReturn, KwPrint, KwPrintln, KwExit, KwPub,
    KwUse, KwTrue, KwFalse,
    LParen, RParen, LBrace, RBrace, Comma, Semi, Colon, Arrow, Dot,
    Plus, Minus, Star, Slash, Percent, Assign, Decl, Eq, Ne, Lt, Le, Gt, Ge
};

struct Tok {
    K k = K::End;
    std::string s;
    int64_t i = 0;
    int line = 1;
};

enum class Ty { Int, Str, Void, Unknown };

const char* tyName(Ty t);

struct Expr;
struct Stmt;
using ExprPtr = std::unique_ptr<Expr>;
using StmtPtr = std::unique_ptr<Stmt>;

struct Expr {
    enum Kind { IntLit, StrLit, Var, Call, Unary, Binary } kind = IntLit;
    int64_t i = 0;
    std::string s;
    K op = K::End;
    std::vector<ExprPtr> args;
    ExprPtr a, b;
    Ty ty = Ty::Unknown;
    int line = 1;
};

struct Stmt {
    enum Kind { VarDecl, Assign, Print, Println, Exit, If, While, Return, ExprStmt } kind = ExprStmt;
    std::string name;
    Ty declTy = Ty::Unknown;   // explicit annotation, if any
    bool hasDeclTy = false;
    ExprPtr e;                 // value / condition / return value
    std::vector<StmtPtr> body; // then-branch / loop body
    std::vector<StmtPtr> els;  // else-branch
    int line = 1;
};

struct Func {
    std::string name;
    std::vector<std::pair<std::string, Ty>> params;
    Ty ret = Ty::Void;
    std::vector<StmtPtr> body;
    int line = 1;
};

struct Program {
    std::vector<std::string> imports;
    std::vector<Func> funcs;
    std::vector<StmtPtr> top;
};

// Parse a .al source file. Throws CompileError on syntax errors.
Program parseProgram(const std::string& src, const std::string& file);
