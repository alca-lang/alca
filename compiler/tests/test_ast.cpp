// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0
//
// Port of pkg/ast/ast_test.go to C++.

#include "ast/ast.h"
#include "microtest/microtest.h"
#include "token/token.h"

#include <string>

using namespace ulang;
using namespace ulang::ast;

MT_TEST(ast, statements) {
    // 1. Program & VarDeclaration
    auto prog = std::make_shared<Program>();
    prog->token = token::Token{token::IDENT, "x", 1, 1};

    auto varDecl = std::make_shared<VarDeclaration>();
    varDecl->token = token::Token{token::IDENT, "x", 1, 1};
    auto id = std::make_shared<Identifier>();
    id->token = token::Token{token::IDENT, "x", 1, 1};
    id->value = "x";
    varDecl->name = id;

    auto annot = std::make_shared<TypeAnnotation>();
    annot->token = token::Token{token::TYPE_INT, "int", 1, 4};
    annot->types = {"int"};
    varDecl->typeAnnotation = annot;

    auto intLit = std::make_shared<IntegerLiteral>();
    intLit->token = token::Token{token::INT, "42", 1, 10};
    intLit->value = 42;
    varDecl->value = intLit;

    prog->statements.push_back(varDecl);

    MT_CHECK_EQ(prog->tokenLiteral(), "x");
    MT_CHECK(prog->toString().find("x: int = 42") != std::string::npos);

    // 2. VarDeclaration with Decorator
    auto decDecl = std::make_shared<VarDeclaration>();
    decDecl->token = token::Token{token::IDENT, "counter", 2, 1};
    auto dec = std::make_shared<Decorator>();
    dec->token = token::Token{token::AT, "@", 1, 1};
    dec->name = "export";
    decDecl->decorators.push_back(dec);

    auto counterId = std::make_shared<Identifier>();
    counterId->token = token::Token{token::IDENT, "counter", 2, 1};
    counterId->value = "counter";
    decDecl->name = counterId;

    auto oneLit = std::make_shared<IntegerLiteral>();
    oneLit->token = token::Token{token::INT, "1", 2, 11};
    oneLit->value = 1;
    decDecl->value = oneLit;

    MT_CHECK_EQ(decDecl->tokenLiteral(), "counter");
    MT_CHECK(decDecl->toString().find("@export\ncounter = 1") != std::string::npos);

    // 3. FuncDeclaration & FuncParam
    auto fnDecl = std::make_shared<FuncDeclaration>();
    fnDecl->token = token::Token{token::FUNC, "func", 3, 1};
    fnDecl->isPublic = true;
    fnDecl->name = "add";

    auto p1 = std::make_shared<FuncParam>();
    p1->token = token::Token{token::IDENT, "a", 3, 14};
    p1->name = "a";
    auto t1 = std::make_shared<TypeAnnotation>();
    t1->types = {"int"};
    p1->typeAnnotation = t1;
    fnDecl->params.push_back(p1);

    auto p2 = std::make_shared<FuncParam>();
    p2->token = token::Token{token::IDENT, "b", 3, 22};
    p2->name = "b";
    auto t2 = std::make_shared<TypeAnnotation>();
    t2->types = {"int"};
    p2->typeAnnotation = t2;
    auto defLit = std::make_shared<IntegerLiteral>();
    defLit->token = token::Token{token::INT, "0", 3, 31};
    defLit->value = 0;
    p2->defaultValue = defLit;
    fnDecl->params.push_back(p2);

    auto retT = std::make_shared<TypeAnnotation>();
    retT->types = {"int"};
    fnDecl->returnType = retT;

    auto body = std::make_shared<BlockStatement>();
    body->token = token::Token{token::LBRACE, "{", 3, 43};
    auto retStmt = std::make_shared<ReturnStatement>();
    retStmt->token = token::Token{token::RETURN, "return", 4, 5};
    auto infix = std::make_shared<InfixExpression>();
    infix->token = token::Token{token::PLUS, "+", 4, 14};
    auto leftId = std::make_shared<Identifier>();
    leftId->value = "a";
    auto rightId = std::make_shared<Identifier>();
    rightId->value = "b";
    infix->left = leftId;
    infix->op = "+";
    infix->right = rightId;
    retStmt->value = infix;
    body->statements.push_back(retStmt);
    fnDecl->body = body;

    MT_CHECK_EQ(fnDecl->tokenLiteral(), "func");
    MT_CHECK(fnDecl->toString().find("pub func add(a: int, b: int = 0) -> int") != std::string::npos);
}

MT_TEST(ast, expressions) {
    // 1. Identifier
    auto ident = std::make_shared<Identifier>();
    ident->token = token::Token{token::IDENT, "foo", 1, 1};
    ident->value = "foo";
    MT_CHECK_EQ(ident->tokenLiteral(), "foo");
    MT_CHECK_EQ(ident->toString(), "foo");

    // 2. IntegerLiteral
    auto intLit = std::make_shared<IntegerLiteral>();
    intLit->token = token::Token{token::INT, "123", 1, 1};
    intLit->value = 123;
    MT_CHECK_EQ(intLit->tokenLiteral(), "123");
    MT_CHECK_EQ(intLit->toString(), "123");

    // 3. FloatLiteral
    auto floatLit = std::make_shared<FloatLiteral>();
    floatLit->token = token::Token{token::FLOAT, "3.14", 1, 1};
    floatLit->value = 3.14;
    MT_CHECK_EQ(floatLit->tokenLiteral(), "3.14");
    MT_CHECK_EQ(floatLit->toString(), "3.14");

    // 4. StringLiteral
    auto strLit = std::make_shared<StringLiteral>();
    strLit->token = token::Token{token::STRING, "\"hello\"", 1, 1};
    strLit->value = "hello";
    MT_CHECK_EQ(strLit->tokenLiteral(), "\"hello\"");
    MT_CHECK_EQ(strLit->toString(), "\"hello\"");

    // 5. BooleanLiteral
    auto boolLit = std::make_shared<BooleanLiteral>();
    boolLit->token = token::Token{token::TRUE, "true", 1, 1};
    boolLit->value = true;
    MT_CHECK_EQ(boolLit->tokenLiteral(), "true");
    MT_CHECK_EQ(boolLit->toString(), "true");

    // 6. NullLiteral
    auto nullLit = std::make_shared<NullLiteral>();
    nullLit->token = token::Token{token::NULL_TOKEN, "null", 1, 1};
    MT_CHECK_EQ(nullLit->tokenLiteral(), "null");
    MT_CHECK_EQ(nullLit->toString(), "null");
}
