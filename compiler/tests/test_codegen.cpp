// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

#include "lexer/lexer.h"
#include "microtest/microtest.h"
#include "parser/parser.h"
#include "typecheck/typecheck.h"
#include "ir/ir.h"
#include "ir/lower.h"
#include "codegen/x64/codegen.h"
#include "link/pe/pe_writer.h"
#include "link/elf/elf_writer.h"
#include <filesystem>
#include <fstream>

using namespace ulang;

static void ensureBuildDir() {
    std::filesystem::create_directories("compiler/build_bin");
}

MT_TEST(codegen, x64_generate_artifact) {
    ensureBuildDir();
    std::string source = 
        "func safe_add(a: int, b: int) -> int {\n"
        "    return a + b\n"
        "}\n"
        "res := safe_add(10, 20)\n"
        "println(res)\n";

    auto lex = std::make_unique<lexer::Lexer>(source);
    parser::Parser p(std::move(lex));
    auto astProg = p.parseProgram();
    MT_CHECK_EQ(p.syntaxErrors().size(), 0u);

    typecheck::Checker tc("test.al", source);
    auto errs = tc.check(astProg);
    MT_CHECK_EQ(errs.size(), 0u);

    auto irRes = ir::lower(astProg, tc);
    MT_CHECK_EQ(irRes.ok(), true);

    codegen::x64::CodeGenerator cg(codegen::x64::ABI::Win64);
    auto art = cg.generate(*irRes.program);
    MT_CHECK(art.code.size() > 0);
    MT_CHECK(art.functionRVAs.find("safe_add") != art.functionRVAs.end());

    for (const auto& fix : art.callFixups) {
        std::printf("fixup: %s\n", fix.funcName.c_str());
    }
    bool peOk = link::pe::linkPE(art, "compiler/build_bin/test_out.exe", false);
    MT_CHECK_EQ(peOk, true);

    bool elfOk = link::elf::linkELF(art, "compiler/build_bin/test_out.elf");
    MT_CHECK_EQ(elfOk, true);

#ifdef _WIN32
    // End-to-end execution verification: run the compiled binary and check stdout
    FILE* pipe = _popen("compiler\\build_bin\\test_out.exe", "r");
    MT_CHECK(pipe != nullptr);
    char buf[128];
    std::string runOut;
    while (std::fgets(buf, sizeof(buf), pipe)) {
        runOut += buf;
    }
    int exitCode = _pclose(pipe);
    MT_CHECK_EQ(exitCode, 0);
    MT_CHECK(runOut.find("30") != std::string::npos);
#endif
}

MT_TEST(codegen, globals_data_section_and_indirect_call) {
    ensureBuildDir();
    std::string source =
        "count := 0\n"
        "func bump() {\n"
        "    count = count + 1\n"
        "}\n"
        "func main() {\n"
        "    f := bump\n"
        "    f()\n"
        "    f()\n"
        "    if count == 2 { println(\"GLOBAL_OK\") }\n"
        "    else { println(\"GLOBAL_BAD\") }\n"
        "}\n";

    auto lex = std::make_unique<lexer::Lexer>(source);
    parser::Parser p(std::move(lex));
    auto astProg = p.parseProgram();
    MT_CHECK_EQ(p.syntaxErrors().size(), 0u);

    typecheck::Checker tc("globals.al", source);
    auto errs = tc.check(astProg);
    MT_CHECK_EQ(errs.size(), 0u);

    auto irRes = ir::lower(astProg, tc);
    MT_CHECK(irRes.ok());
    codegen::x64::CodeGenerator cg(codegen::x64::ABI::Win64);
    auto art = cg.generate(*irRes.program);
    MT_CHECK(art.globals.size() >= 3u); // count + two hidden exception slots
    MT_CHECK(art.dataFixups.size() >= 3u);

    MT_CHECK_EQ(link::pe::linkPE(art, "compiler/build_bin/globals.exe", false), true);
    MT_CHECK_EQ(link::elf::linkELF(art, "compiler/build_bin/globals.elf"), true);
#ifdef _WIN32
    FILE* pipe = _popen("compiler\\build_bin\\globals.exe", "r");
    MT_CHECK(pipe != nullptr);
    char buf[128];
    std::string runOut;
    while (std::fgets(buf, sizeof(buf), pipe)) runOut += buf;
    MT_CHECK_EQ(_pclose(pipe), 0);
    MT_CHECK(runOut.find("GLOBAL_OK") != std::string::npos);
#endif
}

MT_TEST(codegen, generic_and_overload_execution) {
    ensureBuildDir();
    std::string source =
        "func identity[T](x: T) -> T {\n"
        "    return x\n"
        "}\n"
        "func twice(a: int) -> int {\n"
        "    return a + a\n"
        "}\n"
        "func twice(a: float) -> float {\n"
        "    return a + a\n"
        "}\n"
        "func main() {\n"
        "    a := identity(42)\n"
        "    b := identity(3.5)\n"
        "    x := twice(2)\n"
        "    y := twice(2.5)\n"
        "    println(\"GENERICS_OK\")\n"
        "}\n";

    auto lex = std::make_unique<lexer::Lexer>(source);
    parser::Parser p(std::move(lex));
    auto astProg = p.parseProgram();
    MT_CHECK_EQ(p.syntaxErrors().size(), 0u);

    typecheck::Checker tc("generics.al", source);
    auto errs = tc.check(astProg);
    MT_CHECK_EQ(errs.size(), 0u);

    auto irRes = ir::lower(astProg, tc);
    MT_CHECK(irRes.ok());
    codegen::x64::CodeGenerator cg(codegen::x64::ABI::Win64);
    auto art = cg.generate(*irRes.program);
    MT_CHECK(art.functionRVAs.count("identity_int") != 0u);
    MT_CHECK(art.functionRVAs.count("identity_float") != 0u);
    MT_CHECK_EQ(link::pe::linkPE(art, "compiler/build_bin/generics.exe", false), true);
#ifdef _WIN32
    FILE* pipe = _popen("compiler\\build_bin\\generics.exe", "r");
    MT_CHECK(pipe != nullptr);
    char buf[128];
    std::string runOut;
    while (std::fgets(buf, sizeof(buf), pipe)) runOut += buf;
    MT_CHECK_EQ(_pclose(pipe), 0);
    MT_CHECK(runOut.find("GENERICS_OK") != std::string::npos);
#endif
}

MT_TEST(codegen, heap_free_reuse_and_defer) {
    ensureBuildDir();
    std::string source =
        "use std.mem\n"
        "func work() {\n"
        "    p := mem.alloc(64)\n"
        "    mem.write_u64(p, 4242)\n"
        "    mem.free(p)\n"
        "    q := mem.alloc(64)\n"
        "    defer mem.free(q)\n"
        "    r := mem.read_u64(q)\n"
        "    if q == p and r == 4242 { println(\"HEAP_DEFER_OK\") }\n"
        "    else { println(\"HEAP_DEFER_BAD\") }\n"
        "}\n"
        "func main() {\n"
        "    work()\n"
        "}\n";

    auto lex = std::make_unique<lexer::Lexer>(source);
    parser::Parser p(std::move(lex));
    auto astProg = p.parseProgram();
    MT_CHECK_EQ(p.syntaxErrors().size(), 0u);

    typecheck::Checker tc("heap.al", source);
    auto errs = tc.check(astProg);
    MT_CHECK_EQ(errs.size(), 0u);

    auto irRes = ir::lower(astProg, tc);
    MT_CHECK(irRes.ok());
    codegen::x64::CodeGenerator cg(codegen::x64::ABI::Win64);
    auto art = cg.generate(*irRes.program);
    bool hasFree = false;
    for (const auto& fix : art.callFixups) {
        if (fix.funcName == "ulang_free") hasFree = true;
    }
    MT_CHECK(hasFree);
    MT_CHECK_EQ(link::pe::linkPE(art, "compiler/build_bin/heap.exe", false), true);
#ifdef _WIN32
    FILE* pipe = _popen("compiler\\build_bin\\heap.exe", "r");
    MT_CHECK(pipe != nullptr);
    char buf[128];
    std::string runOut;
    while (std::fgets(buf, sizeof(buf), pipe)) runOut += buf;
    MT_CHECK_EQ(_pclose(pipe), 0);
    MT_CHECK(runOut.find("HEAP_DEFER_OK") != std::string::npos);
#endif
}

MT_TEST(codegen, implicit_destructor_execution) {
    ensureBuildDir();
    std::string source =
        "struct A {\n"
        "    id: int\n"
        "}\n"
        "func ~A(self: A) {\n"
        "    println(\"DTOR\")\n"
        "}\n"
        "func f() -> int {\n"
        "    a := A(1)\n"
        "    return 7\n"
        "}\n"
        "func main() {\n"
        "    x := f()\n"
        "    println(\"AFTER\")\n"
        "}\n";

    auto lex = std::make_unique<lexer::Lexer>(source);
    parser::Parser p(std::move(lex));
    auto astProg = p.parseProgram();
    MT_CHECK_EQ(p.syntaxErrors().size(), 0u);

    typecheck::Checker tc("dtor.al", source);
    auto errs = tc.check(astProg);
    MT_CHECK_EQ(errs.size(), 0u);

    auto irRes = ir::lower(astProg, tc);
    MT_CHECK(irRes.ok());
    codegen::x64::CodeGenerator cg(codegen::x64::ABI::Win64);
    auto art = cg.generate(*irRes.program);
    MT_CHECK_EQ(link::pe::linkPE(art, "compiler/build_bin/dtor.exe", false), true);
#ifdef _WIN32
    FILE* pipe = _popen("compiler\\build_bin\\dtor.exe", "r");
    MT_CHECK(pipe != nullptr);
    char buf[128];
    std::string runOut;
    while (std::fgets(buf, sizeof(buf), pipe)) runOut += buf;
    MT_CHECK_EQ(_pclose(pipe), 0);
    size_t d = runOut.find("DTOR");
    size_t a = runOut.find("AFTER");
    MT_CHECK(d != std::string::npos && a != std::string::npos && d < a);
#endif
}

MT_TEST(codegen, block_scoped_destructor_execution) {
    ensureBuildDir();
    std::string source =
        "struct A {\n"
        "    id: int\n"
        "}\n"
        "func ~A(self: A) {\n"
        "    println(\"DTOR\")\n"
        "}\n"
        "func main() {\n"
        "    if true {\n"
        "        a := A(1)\n"
        "        println(\"INNER\")\n"
        "    }\n"
        "    println(\"OUTER\")\n"
        "}\n";

    auto lex = std::make_unique<lexer::Lexer>(source);
    parser::Parser p(std::move(lex));
    auto astProg = p.parseProgram();
    MT_CHECK_EQ(p.syntaxErrors().size(), 0u);

    typecheck::Checker tc("scope_dtor.al", source);
    auto errs = tc.check(astProg);
    MT_CHECK_EQ(errs.size(), 0u);

    auto irRes = ir::lower(astProg, tc);
    MT_CHECK(irRes.ok());
    codegen::x64::CodeGenerator cg(codegen::x64::ABI::Win64);
    auto art = cg.generate(*irRes.program);
    MT_CHECK_EQ(link::pe::linkPE(art, "compiler/build_bin/scope_dtor.exe", false), true);
#ifdef _WIN32
    FILE* pipe = _popen("compiler\\build_bin\\scope_dtor.exe", "r");
    MT_CHECK(pipe != nullptr);
    char buf[128];
    std::string runOut;
    while (std::fgets(buf, sizeof(buf), pipe)) runOut += buf;
    MT_CHECK_EQ(_pclose(pipe), 0);
    size_t inner = runOut.find("INNER");
    size_t dtor = runOut.find("DTOR");
    size_t outer = runOut.find("OUTER");
    MT_CHECK(inner != std::string::npos && dtor != std::string::npos &&
             outer != std::string::npos && inner < dtor && dtor < outer);
#endif
}

MT_TEST(codegen, exception_catch_and_unwind_execution) {
    ensureBuildDir();
    std::string source =
        "struct A {\n"
        "    id: int\n"
        "}\n"
        "func ~A(self: A) {\n"
        "    println(\"DTOR\")\n"
        "}\n"
        "func raise() {\n"
        "    a := A(1)\n"
        "    throw \"boom\"\n"
        "}\n"
        "func main() {\n"
        "    try {\n"
        "        raise()\n"
        "    } catch (e) {\n"
        "        println(\"CAUGHT\")\n"
        "    }\n"
        "    println(\"DONE\")\n"
        "}\n";

    auto lex = std::make_unique<lexer::Lexer>(source);
    parser::Parser p(std::move(lex));
    auto astProg = p.parseProgram();
    MT_CHECK_EQ(p.syntaxErrors().size(), 0u);

    typecheck::Checker tc("exceptions.al", source);
    auto errs = tc.check(astProg);
    MT_CHECK_EQ(errs.size(), 0u);

    auto irRes = ir::lower(astProg, tc);
    MT_CHECK(irRes.ok());
    codegen::x64::CodeGenerator cg(codegen::x64::ABI::Win64);
    auto art = cg.generate(*irRes.program);
    MT_CHECK_EQ(link::pe::linkPE(art, "compiler/build_bin/exceptions.exe", false), true);
#ifdef _WIN32
    FILE* pipe = _popen("compiler\\build_bin\\exceptions.exe", "r");
    MT_CHECK(pipe != nullptr);
    char buf[128];
    std::string runOut;
    while (std::fgets(buf, sizeof(buf), pipe)) runOut += buf;
    MT_CHECK_EQ(_pclose(pipe), 0);
    size_t d = runOut.find("DTOR");
    size_t c = runOut.find("CAUGHT");
    size_t done = runOut.find("DONE");
    MT_CHECK(d != std::string::npos && c != std::string::npos && done != std::string::npos &&
             d < c && c < done);
#endif
}

MT_TEST(codegen, typed_catch_and_finally_execution) {
    ensureBuildDir();
    std::string source =
        "struct Boom {\n"
        "    id: int\n"
        "}\n"
        "func main() {\n"
        "    try {\n"
        "        throw Boom(1)\n"
        "    } catch (e: Boom) {\n"
        "        println(\"TYPED_CAUGHT\")\n"
        "    } finally {\n"
        "        println(\"FINALLY\")\n"
        "    }\n"
        "    println(\"DONE\")\n"
        "}\n";

    auto lex = std::make_unique<lexer::Lexer>(source);
    parser::Parser p(std::move(lex));
    auto astProg = p.parseProgram();
    MT_CHECK_EQ(p.syntaxErrors().size(), 0u);

    typecheck::Checker tc("typed.al", source);
    auto errs = tc.check(astProg);
    MT_CHECK_EQ(errs.size(), 0u);

    auto irRes = ir::lower(astProg, tc);
    MT_CHECK(irRes.ok());
    codegen::x64::CodeGenerator cg(codegen::x64::ABI::Win64);
    auto art = cg.generate(*irRes.program);
    MT_CHECK_EQ(link::pe::linkPE(art, "compiler/build_bin/typed.exe", false), true);
#ifdef _WIN32
    FILE* pipe = _popen("compiler\\build_bin\\typed.exe", "r");
    MT_CHECK(pipe != nullptr);
    char buf[128];
    std::string runOut;
    while (std::fgets(buf, sizeof(buf), pipe)) runOut += buf;
    MT_CHECK_EQ(_pclose(pipe), 0);
    size_t c = runOut.find("TYPED_CAUGHT");
    size_t f = runOut.find("FINALLY");
    size_t d = runOut.find("DONE");
    MT_CHECK(c != std::string::npos && f != std::string::npos && d != std::string::npos &&
             c < f && f < d);
#endif
}

MT_TEST(codegen, operator_overload_execution) {
    ensureBuildDir();
    std::string source =
        "struct Vec {\n"
        "    x: int\n"
        "    y: int\n"
        "}\n"
        "func +(a: Vec, b: Vec) -> Vec {\n"
        "    return Vec(a.x + b.x, a.y + b.y)\n"
        "}\n"
        "func main() {\n"
        "    a := Vec(1, 2)\n"
        "    b := Vec(3, 4)\n"
        "    c := a + b\n"
        "    if c.x == 4 and c.y == 6 { println(\"OP_OK\") }\n"
        "    else { println(\"OP_BAD\") }\n"
        "}\n";

    auto lex = std::make_unique<lexer::Lexer>(source);
    parser::Parser p(std::move(lex));
    auto astProg = p.parseProgram();
    MT_CHECK_EQ(p.syntaxErrors().size(), 0u);

    typecheck::Checker tc("operators.al", source);
    auto errs = tc.check(astProg);
    MT_CHECK_EQ(errs.size(), 0u);

    auto irRes = ir::lower(astProg, tc);
    MT_CHECK(irRes.ok());
    codegen::x64::CodeGenerator cg(codegen::x64::ABI::Win64);
    auto art = cg.generate(*irRes.program);
    MT_CHECK_EQ(link::pe::linkPE(art, "compiler/build_bin/operators.exe", false), true);
#ifdef _WIN32
    FILE* pipe = _popen("compiler\\build_bin\\operators.exe", "r");
    MT_CHECK(pipe != nullptr);
    char buf[128];
    std::string runOut;
    while (std::fgets(buf, sizeof(buf), pipe)) runOut += buf;
    MT_CHECK_EQ(_pclose(pipe), 0);
    MT_CHECK(runOut.find("OP_OK") != std::string::npos);
#endif
}

MT_TEST(codegen, x64_full_features_link) {
    ensureBuildDir();
    std::string source = 
        "struct Server {\n"
        "    ip: string\n"
        "    port: int = 80\n"
        "}\n"
        "func safe_div(a: int, b: int) -> int {\n"
        "    if b === 0 {\n"
        "        err(\"zero\", code=1)\n"
        "        return 0\n"
        "    }\n"
        "    return a / b\n"
        "}\n"
        "srv := Server(\"127.0.0.1\", 8080)\n"
        "greeting := \"srv: \" + srv.ip + \":\" + string(srv.port)\n"
        "println(greeting)\n"
        "failure: error | null = null\n"
        "safe_div(10, 0) ?>>> failure\n"
        "if failure != null {\n"
        "    println(\"caught: \" + failure.msg)\n"
        "}\n"
        "sum := 0\n"
        "for i in 1..=5 {\n"
        "    sum = sum + i\n"
        "}\n"
        "println(sum)\n";

    auto lex = std::make_unique<lexer::Lexer>(source);
    parser::Parser p(std::move(lex));
    auto astProg = p.parseProgram();
    MT_CHECK_EQ(p.syntaxErrors().size(), 0u);

    typecheck::Checker tc("full_test.al", source);
    auto errs = tc.check(astProg);
    MT_CHECK_EQ(errs.size(), 0u);

    auto irRes = ir::lower(astProg, tc);
    MT_CHECK_EQ(irRes.ok(), true);

    codegen::x64::CodeGenerator cg(codegen::x64::ABI::Win64);
    auto art = cg.generate(*irRes.program);
    MT_CHECK(art.code.size() > 0);

    bool peOk = link::pe::linkPE(art, "compiler/build_bin/full_test.exe", false);
    MT_CHECK_EQ(peOk, true);

    bool elfOk = link::elf::linkELF(art, "compiler/build_bin/full_test.elf");
    MT_CHECK_EQ(elfOk, true);
}
