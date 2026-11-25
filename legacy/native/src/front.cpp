// Pawl bootstrap compiler - lexer and recursive-descent parser.
// SPDX-License-Identifier: Apache-2.0
#include "front.h"

#include <algorithm>
#include <cctype>
#include <cstdlib>
#include <unordered_map>

const char* tyName(Ty t) {
    switch (t) {
        case Ty::Int: return "int";
        case Ty::Str: return "string";
        case Ty::Void: return "void";
        default: return "unknown";
    }
}

namespace {

struct Lexer {
    const std::string& src;
    std::string file;
    size_t p = 0;
    int line = 1;
    std::vector<Tok> out;

    Lexer(const std::string& s, std::string f) : src(s), file(std::move(f)) {}

    char peek(size_t n = 0) const { return p + n < src.size() ? src[p + n] : '\0'; }
    void adv() {
        if (p < src.size()) {
            if (src[p] == '\n') line++;
            p++;
        }
    }

    [[noreturn]] void err(const std::string& m) const {
        throw CompileError(file + ":" + std::to_string(line) + ": " + m);
    }

    void skip() {
        for (;;) {
            while (p < src.size() && std::isspace((unsigned char)peek())) adv();
            if (peek() == '/' && peek(1) == '/') {
                while (p < src.size() && peek() != '\n') adv();
                continue;
            }
            if (peek() == '/' && peek(1) == '*') {
                adv(); adv();
                while (p < src.size() && !(peek() == '*' && peek(1) == '/')) adv();
                if (p < src.size()) { adv(); adv(); }
                continue;
            }
            break;
        }
    }

    static bool identStart(char c) { return std::isalpha((unsigned char)c) || c == '_'; }
    static bool identPart(char c) { return std::isalnum((unsigned char)c) || c == '_'; }

    Tok make(K k, std::string s = {}) {
        Tok t; t.k = k; t.s = std::move(s); t.line = line; return t;
    }

    void lex() {
        static const std::unordered_map<std::string, K> kw = {
            {"fn", K::KwFn}, {"if", K::KwIf}, {"else", K::KwElse},
            {"while", K::KwWhile}, {"return", K::KwReturn},
            {"print", K::KwPrint}, {"println", K::KwPrintln},
            {"exit", K::KwExit}, {"pub", K::KwPub}, {"use", K::KwUse},
            {"true", K::KwTrue}, {"false", K::KwFalse},
        };

        for (;;) {
            skip();
            int startLine = line;
            if (p >= src.size()) break;
            char c = peek();

            if (identStart(c)) {
                std::string s;
                while (p < src.size() && identPart(peek())) { s += peek(); adv(); }
                auto it = kw.find(s);
                Tok t = make(it == kw.end() ? K::Ident : it->second, s);
                t.line = startLine;
                out.push_back(t);
                continue;
            }
            if (std::isdigit((unsigned char)c)) {
                int64_t v = 0;
                while (p < src.size() && (std::isdigit((unsigned char)peek()) || peek() == '_')) {
                    if (peek() != '_') v = v * 10 + (peek() - '0');
                    adv();
                }
                Tok t = make(K::IntLit);
                t.i = v; t.line = startLine;
                out.push_back(t);
                continue;
            }
            if (c == '"') {
                adv();
                std::string s;
                while (p < src.size() && peek() != '"') {
                    char ch = peek();
                    if (ch == '\\') {
                        adv();
                        char e = peek();
                        switch (e) {
                            case 'n': s += '\n'; break;
                            case 'r': s += '\r'; break;
                            case 't': s += '\t'; break;
                            case '0': s += '\0'; break;
                            case '"': s += '"'; break;
                            case '\\': s += '\\'; break;
                            default: s += e; break;
                        }
                        adv();
                    } else {
                        s += ch;
                        adv();
                    }
                }
                if (p >= src.size()) err("unterminated string literal");
                adv();
                Tok t = make(K::StrLit, s);
                t.line = startLine;
                out.push_back(t);
                continue;
            }

            auto two = [&](char a, char b) { return c == a && peek(1) == b; };
            Tok t;
            if (two(':', '=')) { adv(); adv(); t = make(K::Decl); }
            else if (two('-', '>')) { adv(); adv(); t = make(K::Arrow); }
            else if (two('=', '=')) { adv(); adv(); t = make(K::Eq); }
            else if (two('!', '=')) { adv(); adv(); t = make(K::Ne); }
            else if (two('<', '=')) { adv(); adv(); t = make(K::Le); }
            else if (two('>', '=')) { adv(); adv(); t = make(K::Ge); }
            else {
                adv();
                switch (c) {
                    case '(': t = make(K::LParen); break;
                    case ')': t = make(K::RParen); break;
                    case '{': t = make(K::LBrace); break;
                    case '}': t = make(K::RBrace); break;
                    case ',': t = make(K::Comma); break;
                    case ';': t = make(K::Semi); break;
                    case ':': t = make(K::Colon); break;
                    case '.': t = make(K::Dot); break;
                    case '+': t = make(K::Plus); break;
                    case '-': t = make(K::Minus); break;
                    case '*': t = make(K::Star); break;
                    case '/': t = make(K::Slash); break;
                    case '%': t = make(K::Percent); break;
                    case '=': t = make(K::Assign); break;
                    case '<': t = make(K::Lt); break;
                    case '>': t = make(K::Gt); break;
                    default: err(std::string("unexpected character '") + c + "'");
                }
            }
            t.line = startLine;
            out.push_back(t);
        }
        Tok e = make(K::End);
        e.line = line;
        out.push_back(e);
    }
};

struct Parser {
    std::vector<Tok> ts;
    size_t pos = 0;
    std::string file;
    Program prog;

    Parser(std::vector<Tok> tokens, std::string f) : ts(std::move(tokens)), file(std::move(f)) {}

    const Tok& cur() const { return ts[pos]; }
    const Tok& next(size_t n = 1) const { return ts[std::min(pos + n, ts.size() - 1)]; }
    bool at(K k) const { return cur().k == k; }
    Tok take() { return ts[pos++]; }
    bool eat(K k) { if (at(k)) { pos++; return true; } return false; }

    [[noreturn]] void err(const std::string& m) const {
        throw CompileError(file + ":" + std::to_string(cur().line) + ": " + m);
    }

    Tok expect(K k, const char* what) {
        if (!at(k)) err(std::string("expected ") + what);
        return take();
    }

    Ty parseType() {
        if (at(K::Ident)) {
            std::string t = cur().s;
            take();
            if (t == "int") return Ty::Int;
            if (t == "string") return Ty::Str;
            if (t == "void") return Ty::Void;
            if (t == "bool") return Ty::Int; // represented as int for now
            err("type '" + t + "' is not supported by the bootstrap backend yet");
        }
        err("expected type name");
    }

    ExprPtr mkInt(int64_t v, int line) {
        auto e = std::make_unique<Expr>();
        e->kind = Expr::IntLit; e->i = v; e->ty = Ty::Int; e->line = line;
        return e;
    }
    ExprPtr mkStr(const std::string& s, int line) {
        auto e = std::make_unique<Expr>();
        e->kind = Expr::StrLit; e->s = s; e->ty = Ty::Str; e->line = line;
        return e;
    }

    ExprPtr parsePrimary() {
        int line = cur().line;
        if (at(K::IntLit)) { int64_t v = take().i; return mkInt(v, line); }
        if (at(K::StrLit)) { std::string s = take().s; return mkStr(s, line); }
        if (at(K::KwTrue)) { take(); return mkInt(1, line); }
        if (at(K::KwFalse)) { take(); return mkInt(0, line); }
        if (at(K::LParen)) {
            take();
            ExprPtr e = parseExpr();
            expect(K::RParen, "')'");
            return e;
        }
        if (at(K::Ident)) {
            std::string name = take().s;
            if (eat(K::LParen)) {
                auto call = std::make_unique<Expr>();
                call->kind = Expr::Call; call->s = name; call->line = line;
                if (!at(K::RParen)) {
                    for (;;) {
                        call->args.push_back(parseExpr());
                        if (!eat(K::Comma)) break;
                    }
                }
                expect(K::RParen, "')'");
                return call;
            }
            if (at(K::Dot)) err("module member access is not implemented yet");
            auto v = std::make_unique<Expr>();
            v->kind = Expr::Var; v->s = name; v->line = line;
            return v;
        }
        err("expected expression");
    }

    ExprPtr parseUnary() {
        if (at(K::Minus)) {
            int line = take().line;
            auto e = std::make_unique<Expr>();
            e->kind = Expr::Unary; e->op = K::Minus; e->a = parseUnary(); e->ty = Ty::Int; e->line = line;
            return e;
        }
        return parsePrimary();
    }

    ExprPtr parseMul() {
        ExprPtr left = parseUnary();
        while (at(K::Star) || at(K::Slash) || at(K::Percent)) {
            int line = cur().line;
            K op = take().k;
            auto e = std::make_unique<Expr>();
            e->kind = Expr::Binary; e->op = op; e->a = std::move(left); e->b = parseUnary();
            e->ty = Ty::Int; e->line = line;
            left = std::move(e);
        }
        return left;
    }

    ExprPtr parseAdd() {
        ExprPtr left = parseMul();
        while (at(K::Plus) || at(K::Minus)) {
            int line = cur().line;
            K op = take().k;
            auto e = std::make_unique<Expr>();
            e->kind = Expr::Binary; e->op = op; e->a = std::move(left); e->b = parseMul();
            e->ty = Ty::Int; e->line = line;
            left = std::move(e);
        }
        return left;
    }

    ExprPtr parseCmp() {
        ExprPtr left = parseAdd();
        while (at(K::Eq) || at(K::Ne) || at(K::Lt) || at(K::Le) || at(K::Gt) || at(K::Ge)) {
            int line = cur().line;
            K op = take().k;
            auto e = std::make_unique<Expr>();
            e->kind = Expr::Binary; e->op = op; e->a = std::move(left); e->b = parseAdd();
            e->ty = Ty::Int; e->line = line;
            left = std::move(e);
        }
        return left;
    }

    ExprPtr parseExpr() { return parseCmp(); }

    std::vector<StmtPtr> parseBlock() {
        expect(K::LBrace, "'{'");
        std::vector<StmtPtr> body;
        while (!at(K::RBrace)) {
            if (at(K::End)) err("unexpected end of file inside block");
            body.push_back(parseStmt());
        }
        expect(K::RBrace, "'}'");
        return body;
    }

    StmtPtr parseStmt() {
        int line = cur().line;
        if (at(K::KwIf)) {
            take();
            auto s = std::make_unique<Stmt>();
            s->kind = Stmt::If; s->line = line;
            s->e = parseExpr();
            s->body = parseBlock();
            if (eat(K::KwElse)) {
                if (at(K::KwIf)) {
                    s->els.push_back(parseStmt());
                } else {
                    s->els = parseBlock();
                }
            }
            return s;
        }
        if (at(K::KwWhile)) {
            take();
            auto s = std::make_unique<Stmt>();
            s->kind = Stmt::While; s->line = line;
            s->e = parseExpr();
            s->body = parseBlock();
            return s;
        }
        if (at(K::KwReturn)) {
            take();
            auto s = std::make_unique<Stmt>();
            s->kind = Stmt::Return; s->line = line;
            if (!at(K::Semi)) s->e = parseExpr();
            eat(K::Semi);
            return s;
        }
        if (at(K::KwPrint) || at(K::KwPrintln)) {
            bool nl = at(K::KwPrintln);
            take();
            expect(K::LParen, "'('");
            auto s = std::make_unique<Stmt>();
            s->kind = nl ? Stmt::Println : Stmt::Print; s->line = line;
            if (!at(K::RParen)) {
                s->e = parseExpr();
                if (eat(K::Comma)) err("multiple print arguments are not supported yet");
            }
            expect(K::RParen, "')'");
            eat(K::Semi);
            return s;
        }
        if (at(K::KwExit)) {
            take();
            expect(K::LParen, "'('");
            auto s = std::make_unique<Stmt>();
            s->kind = Stmt::Exit; s->line = line;
            s->e = parseExpr();
            expect(K::RParen, "')'");
            eat(K::Semi);
            return s;
        }
        if (at(K::Ident)) {
            // name := expr | name: Type = expr | name = expr
            if (next().k == K::Decl) {
                std::string name = take().s;
                take();
                auto s = std::make_unique<Stmt>();
                s->kind = Stmt::VarDecl; s->name = name; s->line = line;
                s->e = parseExpr();
                eat(K::Semi);
                return s;
            }
            if (next().k == K::Colon) {
                std::string name = take().s;
                take();
                auto s = std::make_unique<Stmt>();
                s->kind = Stmt::VarDecl; s->name = name; s->line = line;
                s->declTy = parseType(); s->hasDeclTy = true;
                expect(K::Assign, "'='");
                s->e = parseExpr();
                eat(K::Semi);
                return s;
            }
            if (next().k == K::Assign) {
                std::string name = take().s;
                take();
                auto s = std::make_unique<Stmt>();
                s->kind = Stmt::Assign; s->name = name; s->line = line;
                s->e = parseExpr();
                eat(K::Semi);
                return s;
            }
        }
        auto s = std::make_unique<Stmt>();
        s->kind = Stmt::ExprStmt; s->line = line;
        s->e = parseExpr();
        eat(K::Semi);
        return s;
    }

    void parseUse() {
        int line = cur().line;
        take();
        std::string path;
        if (at(K::StrLit)) {
            path = take().s;
        } else if (at(K::Ident)) {
            path = take().s;
            while (eat(K::Dot)) {
                path += ".";
                path += expect(K::Ident, "module name").s;
            }
        } else {
            err("expected module path after 'use'");
        }
        (void)line;
        eat(K::Semi);
        prog.imports.push_back(path);
    }

    void parseFunc() {
        bool isPub = eat(K::KwPub);
        (void)isPub;
        expect(K::KwFn, "'fn'");
        Func f;
        f.line = cur().line;
        f.name = expect(K::Ident, "function name").s;
        expect(K::LParen, "'('");
        if (!at(K::RParen)) {
            for (;;) {
                std::string pname = expect(K::Ident, "parameter name").s;
                Ty pty = Ty::Int;
                if (eat(K::Colon)) pty = parseType();
                f.params.emplace_back(pname, pty);
                if (eat(K::Assign)) {
                    // default value: parse and ignore for now
                    parseExpr();
                }
                if (!eat(K::Comma)) break;
            }
        }
        expect(K::RParen, "')'");
        if (eat(K::Arrow)) f.ret = parseType();
        f.body = parseBlock();
        if (f.params.size() > 4) err("bootstrap backend supports at most 4 parameters");
        prog.funcs.push_back(std::move(f));
    }

    Program parse() {
        while (!at(K::End)) {
            if (at(K::KwUse)) { parseUse(); continue; }
            if (at(K::KwPub) && next().k == K::KwFn) { parseFunc(); continue; }
            if (at(K::KwFn)) { parseFunc(); continue; }
            prog.top.push_back(parseStmt());
        }
        return std::move(prog);
    }
};

} // namespace

Program parseProgram(const std::string& src, const std::string& file) {
    Lexer lx(src, file);
    lx.lex();
    Parser p(std::move(lx.out), file);
    return p.parse();
}
