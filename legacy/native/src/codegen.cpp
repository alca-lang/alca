// Pawl bootstrap compiler - x86-64 machine code generation.
// Emits raw instruction bytes; no assembler and no external toolchain involved.
// SPDX-License-Identifier: Apache-2.0
#include "codegen.h"

#include <algorithm>
#include <cstring>
#include <set>

namespace {

struct Var {
    Ty ty = Ty::Int;
    int off = 0;
};

struct CG {
    Module m;
    Program& prog;
    std::map<std::string, const Func*> funcs;
    std::vector<std::map<std::string, Var>> scopes;
    std::map<std::string, size_t> strOff;
    int nextSlot = 40;

    CG(Program& p, bool windows) : prog(p) {
        m.windows = windows;
        for (auto& f : p.funcs) {
            if (funcs.count(f.name)) fail("duplicate function '" + f.name + "'");
            funcs[f.name] = &f;
        }
    }

    // ---- raw emitter -------------------------------------------------------
    void u8(uint8_t v) { m.text.push_back(v); }
    void bytes(std::initializer_list<uint8_t> xs) { for (auto x : xs) m.text.push_back(x); }
    void u32(uint32_t v) { for (int i = 0; i < 4; i++) m.text.push_back(uint8_t(v >> (8 * i))); }
    void u64(uint64_t v) { for (int i = 0; i < 8; i++) m.text.push_back(uint8_t(v >> (8 * i))); }
    void align16() { while (m.text.size() % 16) m.text.push_back(0x90); }

    size_t rel32Site() { size_t at = m.text.size(); u32(0); return at; }
    void callTo(const std::string& name) { u8(0xE8); size_t at = rel32Site(); m.calls.push_back({at, name}); }
    void jmpTo(size_t at) {
        // E9 rel32, at = offset of disp32
        u8(0xE9);
        size_t site = rel32Site();
        patch32(site, int32_t(int64_t(at) - int64_t(site + 4)));
    }
    void patch32(size_t at, int32_t v) {
        for (int i = 0; i < 4; i++) m.text[at + i] = uint8_t(uint32_t(v) >> (8 * i));
    }
    size_t emitJcc(uint8_t cc) {
        bytes({0x0F, cc});
        return rel32Site();
    }
    void patchJcc(size_t site) {
        patch32(site, int32_t(int64_t(m.text.size()) - int64_t(site + 4)));
    }

    void movRAXimm(int64_t v) { bytes({0x48, 0xB8}); u64(uint64_t(v)); }
    void movRCXfromRSP(uint32_t off) { bytes({0x48, 0x8B, 0x4C, 0x24}); u8(uint8_t(off)); }
    void movRDXfromRSP(uint32_t off) { bytes({0x48, 0x8B, 0x54, 0x24}); u8(uint8_t(off)); }
    void movR8fromRSP(uint32_t off) { bytes({0x4C, 0x8B, 0x44, 0x24}); u8(uint8_t(off)); }
    void movR9fromRSP(uint32_t off) { bytes({0x4C, 0x8B, 0x4C, 0x24}); u8(uint8_t(off)); }
    void movRDIfromRSP(uint32_t off) { bytes({0x48, 0x8B, 0x7C, 0x24}); u8(uint8_t(off)); }
    void movRSIfromRSP(uint32_t off) { bytes({0x48, 0x8B, 0x74, 0x24}); u8(uint8_t(off)); }

    void movArgFromRAX() {
        if (m.windows) bytes({0x48, 0x89, 0xC1}); // mov rcx, rax
        else bytes({0x48, 0x89, 0xC7});           // mov rdi, rax
    }

    // PE runtime reference helpers
    void iatCall(const std::string& name) {
        bytes({0xFF, 0x15});
        size_t at = rel32Site();
        m.iatCalls.push_back({at, name});
    }
    void loadRCXfromRIP(std::vector<size_t>& bucket) {
        bytes({0x48, 0x8B, 0x0D});
        bucket.push_back(rel32Site());
    }
    void loadRAXfromRIP(std::vector<size_t>& bucket) {
        bytes({0x48, 0x8B, 0x05});
        bucket.push_back(rel32Site());
    }
    void storeRAXtoRIP(std::vector<size_t>& bucket) {
        bytes({0x48, 0x89, 0x05});
        bucket.push_back(rel32Site());
    }
    void storeRDXtoRIP(std::vector<size_t>& bucket) {
        bytes({0x48, 0x89, 0x15});
        bucket.push_back(rel32Site());
    }

    // ---- string pool -------------------------------------------------------
    size_t intern(const std::string& s) {
        auto it = strOff.find(s);
        if (it != strOff.end()) return it->second;
        while (m.rodata.size() % 8) m.rodata.push_back(0);
        size_t off = m.rodata.size();
        for (int i = 0; i < 8; i++) m.rodata.push_back(uint8_t(uint64_t(s.size()) >> (8 * i)));
        m.rodata.insert(m.rodata.end(), s.begin(), s.end());
        strOff[s] = off;
        return off;
    }

    // ---- scopes ------------------------------------------------------------
    void pushScope() { scopes.emplace_back(); }
    void popScope() { scopes.pop_back(); }

    Var* findVar(const std::string& name) {
        for (auto it = scopes.rbegin(); it != scopes.rend(); ++it) {
            auto f = it->find(name);
            if (f != it->end()) return &f->second;
        }
        return nullptr;
    }

    Var declare(const std::string& name, Ty ty, int line) {
        if (scopes.empty()) pushScope();
        auto& top = scopes.back();
        if (top.count(name)) fail("line " + std::to_string(line) + ": variable '" + name + "' already declared in this scope");
        Var v;
        v.ty = ty;
        v.off = nextSlot;
        nextSlot += 8;
        top[name] = v;
        return v;
    }

    // ---- type checking -----------------------------------------------------
    Ty typeOf(const Expr* e) {
        switch (e->kind) {
            case Expr::IntLit: return Ty::Int;
            case Expr::StrLit: return Ty::Str;
            case Expr::Var: {
                Var* v = findVar(e->s);
                if (!v) fail("line " + std::to_string(e->line) + ": undefined variable '" + e->s + "'");
                return v->ty;
            }
            case Expr::Call: {
                auto it = funcs.find(e->s);
                if (it == funcs.end()) fail("line " + std::to_string(e->line) + ": unknown function '" + e->s + "'");
                return it->second->ret;
            }
            case Expr::Unary: return Ty::Int;
            case Expr::Binary: return Ty::Int;
        }
        return Ty::Unknown;
    }

    // ---- expressions -------------------------------------------------------
    void genExpr(Expr* e) {
        switch (e->kind) {
            case Expr::IntLit:
                movRAXimm(e->i);
                return;
            case Expr::StrLit: {
                size_t off = intern(e->s);
                bytes({0x48, 0x8D, 0x05});
                size_t at = rel32Site();
                m.strings.push_back({at, off});
                return;
            }
            case Expr::Var: {
                Var* v = findVar(e->s);
                if (!v) fail("line " + std::to_string(e->line) + ": undefined variable '" + e->s + "'");
                // mov rax, [rbp - off]
                bytes({0x48, 0x8B, 0x85});
                u32(uint32_t(-int32_t(v->off)));
                return;
            }
            case Expr::Unary: {
                if (e->op != K::Minus) fail("unsupported unary operator");
                genExpr(e->a.get());
                bytes({0x48, 0xF7, 0xD8}); // neg rax
                return;
            }
            case Expr::Binary: {
                genBinary(e);
                return;
            }
            case Expr::Call: {
                genCall(e);
                return;
            }
        }
        fail("internal: unhandled expression");
    }

    void genBinary(Expr* e) {
        Ty lt = typeOf(e->a.get());
        Ty rt = typeOf(e->b.get());
        if (e->op == K::Plus && (lt == Ty::Str || rt == Ty::Str)) {
            fail("line " + std::to_string(e->line) + ": string concatenation is not implemented in the bootstrap backend yet");
        }
        if (lt != Ty::Int || rt != Ty::Int) {
            fail("line " + std::to_string(e->line) + ": arithmetic requires int operands");
        }

        genExpr(e->a.get());
        bytes({0x48, 0x83, 0xEC, 0x10}); // sub rsp,16
        bytes({0x48, 0x89, 0x04, 0x24}); // mov [rsp],rax
        genExpr(e->b.get());
        bytes({0x48, 0x8B, 0x0C, 0x24}); // mov rcx,[rsp]
        bytes({0x48, 0x83, 0xC4, 0x10}); // add rsp,16
        // now rcx = left, rax = right

        switch (e->op) {
            case K::Plus: bytes({0x48, 0x01, 0xC8}); break;              // add rax, rcx
            case K::Minus:
                bytes({0x48, 0x29, 0xC1});                                // sub rcx, rax
                bytes({0x48, 0x89, 0xC8});                                // mov rax, rcx
                break;
            case K::Star: bytes({0x48, 0x0F, 0xAF, 0xC1}); break;         // imul rax, rcx
            case K::Slash:
            case K::Percent:
                bytes({0x49, 0x89, 0xC0});                                // mov r8, rax
                bytes({0x48, 0x89, 0xC8});                                // mov rax, rcx
                bytes({0x48, 0x99});                                      // cqo
                bytes({0x49, 0xF7, 0xF8});                                // idiv r8
                if (e->op == K::Percent) bytes({0x48, 0x89, 0xD0});       // mov rax, rdx
                break;
            default: {
                bytes({0x48, 0x39, 0xC1});                                // cmp rcx, rax
                uint8_t cc = 0x94;
                switch (e->op) {
                    case K::Eq: cc = 0x94; break;
                    case K::Ne: cc = 0x95; break;
                    case K::Lt: cc = 0x9C; break;
                    case K::Le: cc = 0x9E; break;
                    case K::Gt: cc = 0x9F; break;
                    case K::Ge: cc = 0x9D; break;
                    default: fail("unsupported binary operator");
                }
                bytes({0x0F, cc, 0xC0});                                  // setcc al
                bytes({0x0F, 0xB6, 0xC0});                                // movzx eax, al
                break;
            }
        }
    }

    void genCall(Expr* e) {
        auto it = funcs.find(e->s);
        if (it == funcs.end()) fail("line " + std::to_string(e->line) + ": unknown function '" + e->s + "'");
        const Func* f = it->second;
        if (e->args.size() != f->params.size()) {
            fail("line " + std::to_string(e->line) + ": function '" + e->s + "' expects " +
                 std::to_string(f->params.size()) + " arguments, got " + std::to_string(e->args.size()));
        }
        for (size_t i = 0; i < e->args.size(); i++) {
            Ty at = typeOf(e->args[i].get());
            if (at != f->params[i].second) {
                fail("line " + std::to_string(e->line) + ": argument " + std::to_string(i + 1) + " of '" +
                     e->s + "' expects " + tyName(f->params[i].second) + ", got " + tyName(at));
            }
            genExpr(e->args[i].get());
            bytes({0x48, 0x83, 0xEC, 0x10}); // sub rsp,16
            bytes({0x48, 0x89, 0x04, 0x24}); // mov [rsp],rax
        }
        if (!e->args.empty()) {
            for (size_t i = 0; i < e->args.size(); i++) {
                uint32_t off = uint32_t(i * 16);
                if (m.windows) {
                    switch (i) {
                        case 0: movRCXfromRSP(off); break;
                        case 1: movRDXfromRSP(off); break;
                        case 2: movR8fromRSP(off); break;
                        case 3: movR9fromRSP(off); break;
                    }
                } else {
                    switch (i) {
                        case 0: movRDIfromRSP(off); break;
                        case 1: movRSIfromRSP(off); break;
                        case 2: movRDXfromRSP(off); break;
                        case 3: movRCXfromRSP(off); break;
                    }
                }
            }
            uint32_t n = uint32_t(e->args.size() * 16);
            bytes({0x48, 0x81, 0xC4});
            u32(n); // add rsp, n
        }
        callTo(e->s);
    }

    void printStmt(Stmt* s, bool nl) {
        if (!s->e) {
            size_t off = intern("");
            bytes({0x48, 0x8D, 0x05});
            size_t at = rel32Site();
            m.strings.push_back({at, off});
        } else {
            Ty t = typeOf(s->e.get());
            if (t == Ty::Int) {
                genExpr(s->e.get());
                movArgFromRAX();
                callTo("pawl_print_int");
                if (nl) {
                    size_t off = intern("");
                    bytes({0x48, 0x8D, 0x05});
                    size_t at = rel32Site();
                    m.strings.push_back({at, off});
                    movArgFromRAX();
                } else {
                    return;
                }
            } else if (t == Ty::Str) {
                genExpr(s->e.get());
                movArgFromRAX();
            } else {
                fail("line " + std::to_string(s->line) + ": cannot print value of type " + tyName(t));
            }
        }
        callTo(nl ? "pawl_println" : "pawl_print");
    }

    void emitExit() {
        if (m.windows) {
            bytes({0x89, 0xC1}); // mov ecx, eax
            iatCall("ExitProcess");
            u8(0xF4);            // hlt
        } else {
            bytes({0x48, 0x89, 0xC7}); // mov rdi, rax
            bytes({0xB8, 0x3C, 0x00, 0x00, 0x00}); // mov eax, 60
            bytes({0x0F, 0x05});       // syscall
            u8(0xF4);
        }
    }

    void genStmt(Stmt* s) {
        switch (s->kind) {
            case Stmt::VarDecl: {
                if (!s->e) fail("line " + std::to_string(s->line) + ": variable declaration needs a value");
                Ty t = s->hasDeclTy ? s->declTy : typeOf(s->e.get());
                if (t != Ty::Int && t != Ty::Str) {
                    fail("line " + std::to_string(s->line) + ": unsupported variable type " + tyName(t));
                }
                Ty et = typeOf(s->e.get());
                if (et != t) {
                    fail("line " + std::to_string(s->line) + ": cannot initialise " + tyName(t) +
                         " with " + tyName(et));
                }
                Var v = declare(s->name, t, s->line);
                genExpr(s->e.get());
                bytes({0x48, 0x89, 0x85}); // mov [rbp-off], rax
                u32(uint32_t(-int32_t(v.off)));
                break;
            }
            case Stmt::Assign: {
                Var* v = findVar(s->name);
                if (!v) fail("line " + std::to_string(s->line) + ": undefined variable '" + s->name + "'");
                Ty et = typeOf(s->e.get());
                if (et != v->ty) {
                    fail("line " + std::to_string(s->line) + ": cannot assign " + tyName(et) +
                         " to " + tyName(v->ty) + " variable '" + s->name + "'");
                }
                genExpr(s->e.get());
                bytes({0x48, 0x89, 0x85});
                u32(uint32_t(-int32_t(v->off)));
                break;
            }
            case Stmt::Print: printStmt(s, false); break;
            case Stmt::Println: printStmt(s, true); break;
            case Stmt::Exit:
                genExpr(s->e.get());
                emitExit();
                break;
            case Stmt::ExprStmt:
                genExpr(s->e.get());
                break;
            case Stmt::Return: {
                if (s->e) {
                    genExpr(s->e.get());
                } else {
                    bytes({0x48, 0x31, 0xC0}); // xor rax,rax
                }
                epilogue();
                break;
            }
            case Stmt::If: {
                if (typeOf(s->e.get()) != Ty::Int) fail("line " + std::to_string(s->line) + ": condition must be int/bool");
                genExpr(s->e.get());
                bytes({0x48, 0x83, 0xF8, 0x00}); // cmp rax, 0
                size_t je = emitJcc(0x84);        // je else
                pushScope();
                for (auto& st : s->body) genStmt(st.get());
                popScope();
                if (!s->els.empty()) {
                    u8(0xE9);
                    size_t jend = rel32Site();
                    patchJcc(je);
                    pushScope();
                    for (auto& st : s->els) genStmt(st.get());
                    popScope();
                    patch32(jend, int32_t(int64_t(m.text.size()) - int64_t(jend + 4)));
                } else {
                    patchJcc(je);
                }
                break;
            }
            case Stmt::While: {
                size_t start = m.text.size();
                if (typeOf(s->e.get()) != Ty::Int) fail("line " + std::to_string(s->line) + ": condition must be int/bool");
                genExpr(s->e.get());
                bytes({0x48, 0x83, 0xF8, 0x00}); // cmp rax, 0
                size_t je = emitJcc(0x84);
                pushScope();
                for (auto& st : s->body) genStmt(st.get());
                popScope();
                u8(0xE9);
                size_t jb = rel32Site();
                patch32(jb, int32_t(int64_t(start) - int64_t(jb + 4)));
                patchJcc(je);
                break;
            }
        }
    }

    void epilogue() {
        bytes({0x48, 0x89, 0xEC}); // mov rsp, rbp
        u8(0x5D);                  // pop rbp
        u8(0xC3);                  // ret
    }

    int countLocals(const std::vector<StmtPtr>& body) {
        int n = 0;
        for (auto& s : body) {
            if (s->kind == Stmt::VarDecl) n++;
            n += countLocals(s->body);
            n += countLocals(s->els);
        }
        return n;
    }

    void genFunction(const Func& f) {
        m.symbols[f.name] = uint32_t(m.text.size());
        if (m.text.size() % 16) fail("internal: function is not 16-byte aligned");

        int slots = 4 + countLocals(f.body) + 4;
        int frame = 32 + 40 + slots * 8;
        frame = (frame + 15) & ~15;

        u8(0x55);                    // push rbp
        bytes({0x48, 0x89, 0xE5});   // mov rbp, rsp
        bytes({0x48, 0x81, 0xEC});   // sub rsp, imm32
        u32(uint32_t(frame));

        scopes.clear();
        nextSlot = 40;
        pushScope();

        // store parameters
        for (size_t i = 0; i < f.params.size(); i++) {
            Var v;
            v.ty = f.params[i].second;
            v.off = int(8 * (i + 1));
            scopes.back()[f.params[i].first] = v;
            uint8_t disp = uint8_t(-v.off);
            if (m.windows) {
                switch (i) {
                    case 0: bytes({0x48, 0x89, 0x4D}); u8(disp); break; // rcx
                    case 1: bytes({0x48, 0x89, 0x55}); u8(disp); break; // rdx
                    case 2: bytes({0x4C, 0x89, 0x45}); u8(disp); break; // r8
                    case 3: bytes({0x4C, 0x89, 0x4D}); u8(disp); break; // r9
                }
            } else {
                switch (i) {
                    case 0: bytes({0x48, 0x89, 0x7D}); u8(disp); break; // rdi
                    case 1: bytes({0x48, 0x89, 0x75}); u8(disp); break; // rsi
                    case 2: bytes({0x48, 0x89, 0x55}); u8(disp); break; // rdx
                    case 3: bytes({0x48, 0x89, 0x4D}); u8(disp); break; // rcx
                }
            }
        }

        for (auto& st : f.body) genStmt(st.get());

        // implicit return 0
        bytes({0x48, 0x31, 0xC0});
        epilogue();
        popScope();
        align16();
    }

    // ---- runtime stubs -----------------------------------------------------
    void emitRuntimePE() {
        // _start
        m.symbols["_start"] = uint32_t(m.text.size());
        bytes({0x48, 0x83, 0xEC, 0x38});             // sub rsp,56
        bytes({0xB9, 0xF5, 0xFF, 0xFF, 0xFF});       // mov ecx,-11 (STD_OUTPUT_HANDLE)
        iatCall("GetStdHandle");
        storeRAXtoRIP(m.stdoutRefs);
        bytes({0x48, 0xB8});                          // movabs rax, alloc base
        m.allocInitImm.push_back(m.text.size());
        u64(0);
        storeRAXtoRIP(m.allocRefs);
        callTo("__pawl_main");
        bytes({0x89, 0xC1});                          // mov ecx,eax
        iatCall("ExitProcess");
        u8(0xF4);                                     // hlt
        align16();

        // pawl_print(const char* s): s = [u64 len][bytes]
        m.symbols["pawl_print"] = uint32_t(m.text.size());
        bytes({0x48, 0x83, 0xEC, 0x38});              // sub rsp,56
        bytes({0x4C, 0x8B, 0x01});                    // mov r8,[rcx]      len
        bytes({0x48, 0x8D, 0x51, 0x08});              // lea rdx,[rcx+8]   buf
        loadRCXfromRIP(m.stdoutRefs);                 // mov rcx,[stdout]
        bytes({0x4C, 0x8D, 0x4C, 0x24, 0x28});        // lea r9,[rsp+40]
        bytes({0x48, 0xC7, 0x44, 0x24, 0x20, 0, 0, 0, 0});
        iatCall("WriteFile");
        bytes({0x48, 0x83, 0xC4, 0x38, 0xC3});        // add rsp,56; ret
        align16();

        // pawl_println(const char* s)
        m.symbols["pawl_println"] = uint32_t(m.text.size());
        bytes({0x48, 0x83, 0xEC, 0x38});              // sub rsp,56
        callTo("pawl_print");
        loadRCXfromRIP(m.stdoutRefs);                 // mov rcx,[stdout]
        bytes({0x48, 0x8D, 0x15});                    // lea rdx,[rip+newline]
        m.newlineRefs.push_back(rel32Site());
        bytes({0x41, 0xB8, 0x02, 0x00, 0x00, 0x00});  // mov r8d,2
        bytes({0x4C, 0x8D, 0x4C, 0x24, 0x28});        // lea r9,[rsp+40]
        bytes({0x48, 0xC7, 0x44, 0x24, 0x20, 0, 0, 0, 0});
        iatCall("WriteFile");
        bytes({0x48, 0x83, 0xC4, 0x38, 0xC3});        // add rsp,56; ret
        align16();

        // pawl_alloc(size) bump allocator
        m.symbols["pawl_alloc"] = uint32_t(m.text.size());
        bytes({0x48, 0x83, 0xC1, 0x0F});              // add rcx,15
        bytes({0x48, 0x83, 0xE1, 0xF0});              // and rcx,-16
        loadRAXfromRIP(m.allocRefs);                  // mov rax,[allocPtr]
        bytes({0x48, 0x8D, 0x14, 0x08});              // lea rdx,[rax+rcx]
        storeRDXtoRIP(m.allocRefs);                   // mov [allocPtr],rdx
        u8(0xC3);
        align16();

        emitPrintInt(true);
    }

    void emitRuntimeELF() {
        // _start
        m.symbols["_start"] = uint32_t(m.text.size());
        bytes({0x48, 0x31, 0xFF});                    // xor rdi,rdi
        bytes({0xB8, 12, 0, 0, 0});                   // mov eax,12 (brk)
        bytes({0x0F, 0x05});                          // syscall
        bytes({0x49, 0x89, 0xC7});                    // mov r15,rax
        bytes({0x48, 0x89, 0xC7});                    // mov rdi,rax
        bytes({0x48, 0x81, 0xC7, 0x00, 0x00, 0x00, 0x01}); // add rdi,16MB
        bytes({0xB8, 12, 0, 0, 0});
        bytes({0x0F, 0x05});
        bytes({0x4D, 0x31, 0xF6});                    // xor r14,r14
        callTo("__pawl_main");
        bytes({0x48, 0x89, 0xC7});                    // mov rdi,rax
        bytes({0xB8, 60, 0, 0, 0});                   // mov eax,60
        bytes({0x0F, 0x05});
        u8(0xF4);
        align16();

        // pawl_print(const char* s)
        m.symbols["pawl_print"] = uint32_t(m.text.size());
        bytes({0x48, 0x8B, 0x17});                    // mov rdx,[rdi]
        bytes({0x48, 0x8D, 0x77, 0x08});              // lea rsi,[rdi+8]
        bytes({0xBF, 1, 0, 0, 0});                    // mov edi,1
        bytes({0xB8, 1, 0, 0, 0});                    // mov eax,1
        bytes({0x0F, 0x05});                          // syscall
        u8(0xC3);
        align16();

        // pawl_println(const char* s)
        m.symbols["pawl_println"] = uint32_t(m.text.size());
        callTo("pawl_print");
        bytes({0x48, 0xBE});                          // movabs rsi, newline
        u64(0x410000);
        bytes({0xBA, 1, 0, 0, 0});                    // mov edx,1
        bytes({0xBF, 1, 0, 0, 0});                    // mov edi,1
        bytes({0xB8, 1, 0, 0, 0});                    // mov eax,1
        bytes({0x0F, 0x05});
        u8(0xC3);
        align16();

        // pawl_alloc(size)
        m.symbols["pawl_alloc"] = uint32_t(m.text.size());
        bytes({0x48, 0x83, 0xC7, 0x0F});              // add rdi,15
        bytes({0x48, 0x83, 0xE7, 0xF0});              // and rdi,-16
        bytes({0x4C, 0x89, 0xF8});                    // mov rax,r15
        bytes({0x49, 0x01, 0xFF});                    // add r15,rdi
        u8(0xC3);
        align16();

        emitPrintInt(false);
    }

    void emitPrintInt(bool win) {
        m.symbols["pawl_print_int"] = uint32_t(m.text.size());
        if (win) {
            bytes({0x48, 0x81, 0xEC, 0x58, 0, 0, 0}); // sub rsp,88
            bytes({0x48, 0x89, 0xC8});                 // mov rax,rcx
            bytes({0x48, 0x8D, 0x74, 0x24, 0x48});     // lea rsi,[rsp+72]
        } else {
            bytes({0x48, 0x83, 0xEC, 0x28});           // sub rsp,40
            bytes({0x48, 0x89, 0xF8});                 // mov rax,rdi
            bytes({0x48, 0x8D, 0x74, 0x24, 0x20});     // lea rsi,[rsp+32]
        }
        bytes({0x45, 0x31, 0xC0});                     // xor r8d,r8d (sign flag)
        bytes({0x48, 0x85, 0xC0});                     // test rax,rax
        size_t jge = emitJcc(0x8D);                    // jge positive
        bytes({0x48, 0xF7, 0xD8});                     // neg rax
        bytes({0x41, 0xB8, 1, 0, 0, 0});               // mov r8d,1
        patchJcc(jge);

        size_t loop = m.text.size();
        bytes({0x48, 0x31, 0xD2});                     // xor rdx,rdx
        bytes({0x49, 0xC7, 0xC1, 10, 0, 0, 0});        // mov r9,10
        bytes({0x49, 0xF7, 0xF1});                     // div r9
        bytes({0x80, 0xC2, 0x30});                     // add dl,'0'
        bytes({0x48, 0xFF, 0xCE});                     // dec rsi
        bytes({0x88, 0x16});                           // mov [rsi],dl
        bytes({0x48, 0x85, 0xC0});                     // test rax,rax
        bytes({0x0F, 0x85});
        size_t jnz = rel32Site();
        patch32(jnz, int32_t(int64_t(loop) - int64_t(jnz + 4)));

        bytes({0x45, 0x85, 0xC0});                     // test r8d,r8d
        size_t jz = emitJcc(0x84);                     // jz done
        bytes({0x48, 0xFF, 0xCE});                     // dec rsi
        bytes({0xC6, 0x06, 0x2D});                     // mov byte [rsi],'-'
        patchJcc(jz);

        if (win) {
            bytes({0x48, 0x8D, 0x44, 0x24, 0x48});     // lea rax,[rsp+72]
            bytes({0x48, 0x29, 0xF0});                 // sub rax,rsi
            bytes({0x49, 0x89, 0xC0});                 // mov r8,rax (len)
            bytes({0x48, 0x89, 0xF2});                 // mov rdx,rsi (buf)
            loadRCXfromRIP(m.stdoutRefs);              // mov rcx,[stdout]
            bytes({0x4C, 0x8D, 0x4C, 0x24, 0x28});     // lea r9,[rsp+40]
            bytes({0x48, 0xC7, 0x44, 0x24, 0x20, 0, 0, 0, 0});
            iatCall("WriteFile");
            bytes({0x48, 0x81, 0xC4, 0x58, 0, 0, 0});  // add rsp,88
        } else {
            bytes({0x48, 0x8D, 0x44, 0x24, 0x20});     // lea rax,[rsp+32]
            bytes({0x48, 0x29, 0xF0});                 // sub rax,rsi
            bytes({0x48, 0x89, 0xC2});                 // mov rdx,rax (len)
            bytes({0xBF, 1, 0, 0, 0});                 // mov edi,1
            bytes({0xB8, 1, 0, 0, 0});                 // mov eax,1 (write)
            bytes({0x0F, 0x05});                       // syscall
            bytes({0x48, 0x83, 0xC4, 0x28});           // add rsp,40
        }
        u8(0xC3);
        align16();
    }

    void emitIntToStr(bool win) {
        static const uint8_t winBytes[] = {
            0x55, 0x48, 0x89, 0xE5, 0x53, 0x56, 0x57, 0x41, 0x54, 0x48, 0x83, 0xEC, 0x50, 0x48, 0x8D, 0x74,
            0x24, 0x40, 0xC6, 0x06, 0x00, 0x48, 0x89, 0xC8, 0x45, 0x31, 0xC0, 0x48, 0x83, 0xF8, 0x00, 0x7D,
            0x09, 0x48, 0xF7, 0xD8, 0x41, 0xB8, 0x01, 0x00, 0x00, 0x00, 0x48, 0x83, 0xF8, 0x00, 0x75, 0x08,
            0x48, 0xFF, 0xCE, 0xC6, 0x06, 0x30, 0xEB, 0x1A, 0x48, 0x31, 0xD2, 0x49, 0xC7, 0xC1, 0x0A, 0x00,
            0x00, 0x00, 0x49, 0xF7, 0xF1, 0x80, 0xC2, 0x30, 0x48, 0xFF, 0xCE, 0x88, 0x16, 0x48, 0x85, 0xC0,
            0x75, 0xE6, 0x45, 0x85, 0xC0, 0x74, 0x06, 0x48, 0xFF, 0xCE, 0xC6, 0x06, 0x2D, 0x48, 0x8D, 0x44,
            0x24, 0x40, 0x48, 0x29, 0xF0, 0x48, 0x89, 0xC3, 0x48, 0x89, 0xF7, 0x48, 0x8D, 0x4B, 0x09, 0xE8,
            0x00, 0x00, 0x00, 0x00, 0x48, 0x89, 0x18, 0x4C, 0x8D, 0x40, 0x08, 0x48, 0x31, 0xC9, 0x48, 0x39,
            0xD9, 0x7D, 0x0C, 0x8A, 0x14, 0x0F, 0x41, 0x88, 0x14, 0x08, 0x48, 0xFF, 0xC1, 0xEB, 0xEF, 0x41,
            0xC6, 0x04, 0x18, 0x00, 0x48, 0x8D, 0x65, 0xE0, 0x41, 0x5C, 0x5F, 0x5E, 0x5B, 0x5D, 0xC3, 0x90
        };
        static const uint8_t sysvBytes[] = {
            0x55, 0x48, 0x89, 0xE5, 0x53, 0x56, 0x57, 0x41, 0x54, 0x48, 0x83, 0xEC, 0x50, 0x48, 0x8D, 0x74,
            0x24, 0x40, 0xC6, 0x06, 0x00, 0x48, 0x89, 0xC8, 0x45, 0x31, 0xC0, 0x48, 0x83, 0xF8, 0x00, 0x7D,
            0x09, 0x48, 0xF7, 0xD8, 0x41, 0xB8, 0x01, 0x00, 0x00, 0x00, 0x48, 0x83, 0xF8, 0x00, 0x75, 0x08,
            0x48, 0xFF, 0xCE, 0xC6, 0x06, 0x30, 0xEB, 0x1A, 0x48, 0x31, 0xD2, 0x49, 0xC7, 0xC1, 0x0A, 0x00,
            0x00, 0x00, 0x49, 0xF7, 0xF1, 0x80, 0xC2, 0x30, 0x48, 0xFF, 0xCE, 0x88, 0x16, 0x48, 0x85, 0xC0,
            0x75, 0xE6, 0x45, 0x85, 0xC0, 0x74, 0x06, 0x48, 0xFF, 0xCE, 0xC6, 0x06, 0x2D, 0x48, 0x8D, 0x44,
            0x24, 0x40, 0x48, 0x29, 0xF0, 0x48, 0x89, 0xC3, 0x48, 0x89, 0xF7, 0x48, 0x8D, 0x7B, 0x09, 0xE8,
            0x00, 0x00, 0x00, 0x00, 0x48, 0x89, 0x18, 0x4C, 0x8D, 0x40, 0x08, 0x48, 0x31, 0xC9, 0x48, 0x39,
            0xD9, 0x7D, 0x0C, 0x8A, 0x14, 0x0F, 0x41, 0x88, 0x14, 0x08, 0x48, 0xFF, 0xC1, 0xEB, 0xEF, 0x41,
            0xC6, 0x04, 0x18, 0x00, 0x48, 0x8D, 0x65, 0xE0, 0x41, 0x5C, 0x5F, 0x5E, 0x5B, 0x5D, 0xC3, 0x90
        };
        const uint8_t* src = win ? winBytes : sysvBytes;
        size_t n = win ? sizeof(winBytes) : sizeof(sysvBytes);
        size_t off = m.text.size();
        m.symbols["pawl_int_to_str"] = uint32_t(off);
        m.text.insert(m.text.end(), src, src + n);
        // call rel32 to pawl_alloc sits at off+111, displacement at off+112
        m.calls.push_back({off + 112, "pawl_alloc"});
        align16();
    }

    Module run() {
        if (m.windows) emitRuntimePE(); else emitRuntimeELF();

        for (auto& f : prog.funcs) genFunction(f);

        Func mainFn;
        mainFn.name = "__pawl_main";
        mainFn.ret = Ty::Int;
        mainFn.body = std::move(prog.top);

        if (m.text.size() % 16) m.text.push_back(0x90);

        m.symbols["__pawl_main"] = uint32_t(m.text.size());
        m.entryMain = uint32_t(m.text.size());

        int slots = 8 + countLocals(mainFn.body);
        int frame = 32 + 40 + slots * 8;
        frame = (frame + 15) & ~15;
        u8(0x55);
        bytes({0x48, 0x89, 0xE5});
        bytes({0x48, 0x81, 0xEC});
        u32(uint32_t(frame));

        scopes.clear();
        nextSlot = 40;
        pushScope();
        for (auto& st : mainFn.body) genStmt(st.get());
        bytes({0x48, 0x31, 0xC0});
        epilogue();
        popScope();
        align16();

        return std::move(m);
    }
};

} // namespace

Module compileModule(Program& prog, bool windows) {
    CG cg(prog, windows);
    return cg.run();
}
