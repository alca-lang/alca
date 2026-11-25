// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

#include "lexer/lexer.h"
#include "microtest/microtest.h"
#include "parser/parser.h"
#include "typecheck/typecheck.h"
#include "ir/ir.h"
#include "ir/lower.h"

using namespace pawl;

MT_TEST(ir, simple_arithmetic) {
    std::string source = "x := 10 + 20\n";
    auto lex = std::make_unique<lexer::Lexer>(source);
    parser::Parser p(std::move(lex));
    auto astProg = p.parseProgram();
    MT_CHECK_EQ(p.syntaxErrors().size(), 0u);

    typecheck::Checker tc("test.al", source);
    auto errs = tc.check(astProg);
    MT_CHECK_EQ(errs.size(), 0u);

    auto irRes = ir::lower(astProg, tc);
    MT_CHECK_EQ(irRes.ok(), true);
    MT_CHECK(irRes.program != nullptr);

    std::string irStr = irRes.program->toString();
    MT_CHECK(irStr.find("const_int 10") != std::string::npos);
    MT_CHECK(irStr.find("const_int 20") != std::string::npos);
    MT_CHECK(irStr.find("add") != std::string::npos);
}

MT_TEST(ir, functions_and_calls) {
    std::string source = 
        "func add(a: int, b: int) -> int {\n"
        "    return a + b\n"
        "}\n"
        "res := add(5, 7)\n";

    auto lex = std::make_unique<lexer::Lexer>(source);
    parser::Parser p(std::move(lex));
    auto astProg = p.parseProgram();
    MT_CHECK_EQ(p.syntaxErrors().size(), 0u);

    typecheck::Checker tc("test.al", source);
    auto errs = tc.check(astProg);
    MT_CHECK_EQ(errs.size(), 0u);

    auto irRes = ir::lower(astProg, tc);
    MT_CHECK_EQ(irRes.ok(), true);

    std::string irStr = irRes.program->toString();
    MT_CHECK(irStr.find("fn @add(v0:i64, v1:i64) -> i64") != std::string::npos);
    MT_CHECK(irStr.find("call @add(v") != std::string::npos);
}

MT_TEST(ir, control_flow) {
    std::string source = 
        "x := 1\n"
        "if x > 0 {\n"
        "    x = x + 1\n"
        "} else {\n"
        "    x = 0\n"
        "}\n";

    auto lex = std::make_unique<lexer::Lexer>(source);
    parser::Parser p(std::move(lex));
    auto astProg = p.parseProgram();
    MT_CHECK_EQ(p.syntaxErrors().size(), 0u);

    typecheck::Checker tc("test.al", source);
    auto errs = tc.check(astProg);
    MT_CHECK_EQ(errs.size(), 0u);

    auto irRes = ir::lower(astProg, tc);
    MT_CHECK_EQ(irRes.ok(), true);

    std::string irStr = irRes.program->toString();
    MT_CHECK(irStr.find("branch") != std::string::npos);
    MT_CHECK(irStr.find("if_then") != std::string::npos);
    MT_CHECK(irStr.find("if_else") != std::string::npos);
    MT_CHECK(irStr.find("if_merge") != std::string::npos);
}

MT_TEST(ir, string_equality_uses_runtime_helper) {
    std::string source =
        "a := \"hello\"\n"
        "b := \"hel\" + \"lo\"\n"
        "same := a == b\n";

    auto lex = std::make_unique<lexer::Lexer>(source);
    parser::Parser p(std::move(lex));
    auto astProg = p.parseProgram();
    MT_CHECK_EQ(p.syntaxErrors().size(), 0u);

    typecheck::Checker tc("test.al", source);
    auto errs = tc.check(astProg);
    MT_CHECK_EQ(errs.size(), 0u);

    auto irRes = ir::lower(astProg, tc);
    MT_CHECK_EQ(irRes.ok(), true);
    MT_CHECK(irRes.program != nullptr);
    MT_CHECK(irRes.program->toString().find("call @pawl_str_eq") != std::string::npos);
}

MT_TEST(ir, list_equality_uses_element_loop) {
    std::string source =
        "a := [1, 2, 3]\n"
        "b := [1, 2] + [3]\n"
        "same := a == b\n";

    auto lex = std::make_unique<lexer::Lexer>(source);
    parser::Parser p(std::move(lex));
    auto astProg = p.parseProgram();
    MT_CHECK_EQ(p.syntaxErrors().size(), 0u);

    typecheck::Checker tc("test.al", source);
    auto errs = tc.check(astProg);
    MT_CHECK_EQ(errs.size(), 0u);

    auto irRes = ir::lower(astProg, tc);
    MT_CHECK_EQ(irRes.ok(), true);
    MT_CHECK(irRes.program != nullptr);
    std::string irStr = irRes.program->toString();
    MT_CHECK(irStr.find("list_eq_cond") != std::string::npos);
    MT_CHECK(irStr.find("list_eq_elem") != std::string::npos);
}

MT_TEST(ir, assert_condition_is_lowered) {
    std::string source =
        "values := [1, 2, 3]\n"
        "assert len(values) == 3\n";

    auto lex = std::make_unique<lexer::Lexer>(source);
    parser::Parser p(std::move(lex));
    auto astProg = p.parseProgram();
    MT_CHECK_EQ(p.syntaxErrors().size(), 0u);

    typecheck::Checker tc("test.al", source);
    auto errs = tc.check(astProg);
    MT_CHECK_EQ(errs.size(), 0u);

    auto irRes = ir::lower(astProg, tc);
    MT_CHECK_EQ(irRes.ok(), true);
    MT_CHECK(irRes.program != nullptr);
    MT_CHECK(irRes.program->toString().find("assert_fail") != std::string::npos);
}

MT_TEST(ir, list_and_for_loop) {
    std::string source = 
        "total := 0\n"
        "for i in 1..5 {\n"
        "    total = total + i\n"
        "}\n";

    auto lex = std::make_unique<lexer::Lexer>(source);
    parser::Parser p(std::move(lex));
    auto astProg = p.parseProgram();
    MT_CHECK_EQ(p.syntaxErrors().size(), 0u);

    typecheck::Checker tc("test.al", source);
    auto errs = tc.check(astProg);
    MT_CHECK_EQ(errs.size(), 0u);

    auto irRes = ir::lower(astProg, tc);
    MT_CHECK_EQ(irRes.ok(), true);

    std::string irStr = irRes.program->toString();
    MT_CHECK(irStr.find("for_cond") != std::string::npos);
    MT_CHECK(irStr.find("for_body") != std::string::npos);
    MT_CHECK(irStr.find("for_step") != std::string::npos);
}
