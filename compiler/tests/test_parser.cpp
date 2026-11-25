// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0
//
// Port of pkg/parser/parser_test.go to C++.

#include "ast/ast.h"
#include "lexer/lexer.h"
#include "microtest/microtest.h"
#include "parser/parser.h"

#include <memory>
#include <string>
#include <vector>

using namespace pawl;
using namespace pawl::ast;
using namespace pawl::parser;

namespace {

std::pair<std::shared_ptr<Program>, std::vector<std::string>> parse(const std::string& src) {
    auto l = std::make_unique<lexer::Lexer>(src);
    Parser p(std::move(l));
    auto prog = p.parseProgram();
    std::vector<std::string> msgs;
    for (const auto& e : p.syntaxErrors()) {
        msgs.push_back(e->message);
    }
    return {prog, msgs};
}

std::shared_ptr<Program> mustParse(const std::string& src) {
    auto [prog, errs] = parse(src);
    MT_CHECK_EQ(errs.size(), 0u);
    return prog;
}

} // namespace

MT_TEST(parser, annotated_and_inferred_declarations) {
    auto prog = mustParse("y: int = 10\nname: string | null = \"alice\"\nz := 3.5\n");
    MT_CHECK_EQ(prog->statements.size(), 3u);

    auto y = std::dynamic_pointer_cast<VarDeclaration>(prog->statements[0]);
    MT_CHECK(y != nullptr);
    MT_CHECK(y->typeAnnotation != nullptr);
    MT_CHECK_EQ(y->typeAnnotation->types.size(), 1u);
    MT_CHECK_EQ(y->typeAnnotation->types[0], "int");

    auto n = std::dynamic_pointer_cast<VarDeclaration>(prog->statements[1]);
    MT_CHECK(n != nullptr);
    MT_CHECK(n->typeAnnotation != nullptr);
    MT_CHECK_EQ(n->typeAnnotation->types.size(), 2u);
    MT_CHECK_EQ(n->typeAnnotation->types[0], "string");
    MT_CHECK_EQ(n->typeAnnotation->types[1], "null");

    auto z = std::dynamic_pointer_cast<VarDeclaration>(prog->statements[2]);
    MT_CHECK(z != nullptr);
    MT_CHECK(z->typeAnnotation == nullptr);
    MT_CHECK(z->value != nullptr);
}

MT_TEST(parser, plain_assignment_is_not_a_declaration) {
    auto prog = mustParse("x = 5\n");
    MT_CHECK_EQ(prog->statements.size(), 1u);
    auto es = std::dynamic_pointer_cast<ExpressionStatement>(prog->statements[0]);
    MT_CHECK(es != nullptr);
    auto as = std::dynamic_pointer_cast<AssignExpression>(es->expression);
    MT_CHECK(as != nullptr);
}

MT_TEST(parser, semicolon_is_rejected) {
    const std::vector<std::string> cases = {
        "x: int = 5;\n",
        "println(1);\n",
        "func f() -> int {\n    return 1;\n}\n;\n",
        "use std.math;\n",
        "break;\n"
    };
    for (const auto& src : cases) {
        auto [prog, errs] = parse(src);
        MT_CHECK(errs.size() > 0);
    }
}

MT_TEST(parser, newline_terminated_statements) {
    auto [prog, errs] = parse("a: int = 1\nb: int = 2\nc: int = 3\n");
    MT_CHECK_EQ(errs.size(), 0u);
    MT_CHECK_EQ(prog->statements.size(), 3u);
}

MT_TEST(parser, blocks_do_not_take_semicolons) {
    const std::string src =
        "func f(a: int) -> int {\n"
        "    if a > 0 {\n"
        "        return a\n"
        "    }\n"
        "    elif a == 0 {\n"
        "        return 0\n"
        "    }\n"
        "    else {\n"
        "        return -a\n"
        "    }\n"
        "}\n"
        "while false {\n"
        "    break\n"
        "}\n"
        "for i in 0..3 {\n"
        "    println(i)\n"
        "}\n";
    mustParse(src);
}

MT_TEST(parser, c_style_declaration_rejected) {
    auto [prog, errs] = parse("int x = 5\n");
    MT_CHECK(errs.size() > 0);
}

MT_TEST(parser, declaration_needs_initializer) {
    auto [prog, errs] = parse("x: int\n");
    MT_CHECK(errs.size() > 0);
}

MT_TEST(parser, struct_and_match) {
    const std::string src =
        "struct Point {\n"
        "    x: int\n"
        "    y: int\n"
        "}\n"
        "match status {\n"
        "    case 200 {\n"
        "        println(\"OK\")\n"
        "    }\n"
        "    else {\n"
        "        println(\"Error\")\n"
        "    }\n"
        "}\n";
    mustParse(src);
}
