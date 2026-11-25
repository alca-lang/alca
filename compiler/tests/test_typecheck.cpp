// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0
//
// Port of pkg/typecheck/typecheck_test.go to C++.

#include "lexer/lexer.h"
#include "microtest/microtest.h"
#include "parser/parser.h"
#include "typecheck/typecheck.h"

#include <memory>
#include <string>
#include <vector>

using namespace pawl;
using namespace pawl::typecheck;

namespace {

std::vector<std::string> check(const std::string& src) {
    auto l = std::make_unique<lexer::Lexer>(src);
    parser::Parser p(std::move(l));
    auto prog = p.parseProgram();
    MT_CHECK_EQ(p.syntaxErrors().size(), 0u);

    Checker c("test.al", src);
    auto errs = c.check(prog);

    std::vector<std::string> msgs;
    for (const auto& e : errs) {
        msgs.push_back(e->message + " | " + e->suggestion);
    }
    return msgs;
}

} // namespace

MT_TEST(typecheck, annotated_and_inferred) {
    const std::string src =
        "x: int = 5\n"
        "y := 7\n"
        "z: float = 2.0\n"
        "println(x + y)\n";
    auto errs = check(src);
    MT_CHECK_EQ(errs.size(), 0u);
}

MT_TEST(typecheck, functions_and_structs) {
    const std::string src =
        "struct P {\n"
        "    name: string\n"
        "    age: int = 18\n"
        "}\n"
        "func older(p: P) -> int {\n"
        "    return p.age + 1\n"
        "}\n"
        "q := P(\"a\")\n"
        "n: int = older(q)\n";
    auto errs = check(src);
    MT_CHECK_EQ(errs.size(), 0u);
}

MT_TEST(typecheck, assert_condition_is_checked) {
    const std::string src =
        "values := [1, 2, 3]\n"
        "assert len(values) == 3\n"
        "assert values[0] == 1\n";
    auto errs = check(src);
    MT_CHECK_EQ(errs.size(), 0u);
}

MT_TEST(typecheck, type_mismatch_detected) {
    const std::string src =
        "x: int = \"hello\"\n";
    auto errs = check(src);
    MT_CHECK(errs.size() > 0u);
}

MT_TEST(typecheck, undefined_variable_detected) {
    const std::string src =
        "println(unknown_var)\n";
    auto errs = check(src);
    MT_CHECK(errs.size() > 0u);
}
