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
#include <fstream>

using namespace ulang;

MT_TEST(codegen, x64_generate_artifact) {
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
}

MT_TEST(codegen, x64_full_features_link) {
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
