// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

#pragma once

#include <functional>
#include <map>
#include <memory>
#include <string>
#include <vector>

#include "ast/ast.h"
#include "diag/diagnostic.h"
#include "lexer/lexer.h"
#include "token/token.h"

namespace ulang::parser {

// Precedence levels for Pawl expressions (from lowest to highest).
enum Precedence {
    LOWEST = 1,
    STREAM,      // >> >! >>> << ?>> ?>>> &>> &&>>
    PIPE,        // |>
    ASSIGN,      // =
    OR,          // or
    AND,         // and
    RANGE,       // ..
    EQUALS,      // == !=
    LESSGREATER, // < > <= >= in
    SUM,         // + -
    PRODUCT,     // * / %
    POWER,       // **
    PREFIX,      // - not !
    CALL,        // function calls ()
    INDEX,       // [] .
    TIMEOUT      // ~
};

class Parser {
public:
    explicit Parser(std::unique_ptr<lexer::Lexer> lexer);
    explicit Parser(lexer::Lexer* lexer);

    std::shared_ptr<ast::Program> parseProgram();

    const std::vector<std::string>& errors() const { return errors_; }
    const std::vector<std::shared_ptr<diag::SyntaxError>>& syntaxErrors() const { return syntaxErrors_; }
    std::string diagnosticReport(const std::string& filename, const std::string& source, bool useColor = false) const;

private:
    using PrefixParseFn = std::function<ast::ExpressionPtr()>;
    using InfixParseFn = std::function<ast::ExpressionPtr(ast::ExpressionPtr)>;

    void nextToken();
    bool curTokenIs(token::Kind t) const;
    bool peekTokenIs(token::Kind t) const;
    bool expectPeek(token::Kind t);
    void peekError(token::Kind t);
    bool expectBlockBrace(const std::string& contextName);
    void noPrefixParseFnError(token::Kind t);

    int peekPrecedence() const;
    int curPrecedence() const;

    void registerPrefix(token::Kind tokenType, PrefixParseFn fn);
    void registerInfix(token::Kind tokenType, InfixParseFn fn);

    void skipNewlines();
    void skipPeekNewlines();
    void consumeStatementEnd();

    ast::StatementPtr parseStatement();
    ast::StatementPtr parseStatementInternal();
    void parseVarModifiers(std::string& visibility, bool& isConst);
    void reportAt(const token::Token& tok, const std::string& suggestion, const std::string& msg);
    std::shared_ptr<ast::Decorator> parseDecorator();
    std::shared_ptr<ast::VarDeclaration> parseVarDeclaration(const std::vector<std::shared_ptr<ast::Decorator>>& decorators);
    std::shared_ptr<ast::VarDeclaration> parseTypedVarDeclaration(const std::string& visibility, bool isConst,
                                                                 const std::vector<std::shared_ptr<ast::Decorator>>& decorators);
    std::shared_ptr<ast::FuncDeclaration> parseFuncDeclaration(bool isPublic, const std::vector<std::shared_ptr<ast::Decorator>>& decorators);
    std::vector<std::shared_ptr<ast::FuncParam>> parseFuncParams();
    std::shared_ptr<ast::StructDeclaration> parseStructDeclaration(bool isPublic, const std::vector<std::shared_ptr<ast::Decorator>>& decorators);
    std::shared_ptr<ast::ClassDeclaration> parseClassDeclaration(bool isPublic, const std::vector<std::shared_ptr<ast::Decorator>>& decorators);
    std::shared_ptr<ast::EnumDeclaration> parseEnumDeclaration(bool isPublic, const std::vector<std::shared_ptr<ast::Decorator>>& decorators);
    std::shared_ptr<ast::InterfaceDeclaration> parseInterfaceDeclaration(bool isPublic, const std::vector<std::shared_ptr<ast::Decorator>>& decorators);
    ast::ExpressionPtr parseAwaitExpression();
    std::shared_ptr<ast::StructField> parseClassField(const std::string& visibility, bool isConst);
    std::shared_ptr<ast::BlockStatement> parseBlockStatement();
    std::shared_ptr<ast::BlockStatement> parseBraceBlockStatement();
    void skipBraceBlockTrivia();

    std::shared_ptr<ast::IfStatement> parseIfStatement();
    std::shared_ptr<ast::WhileStatement> parseWhileStatement();
    std::shared_ptr<ast::ForStatement> parseForStatement();
    std::shared_ptr<ast::ForEachStatement> parseForEachStatement();
    std::shared_ptr<ast::LoopStatement> parseLoopStatement();
    std::shared_ptr<ast::ReturnStatement> parseReturnStatement();
    std::shared_ptr<ast::BreakStatement> parseBreakStatement();
    std::shared_ptr<ast::ContinueStatement> parseContinueStatement();
    std::shared_ptr<ast::UseStatement> parseUseStatement();
    std::shared_ptr<ast::WatchStatement> parseWatchStatement();
    std::shared_ptr<ast::TestStatement> parseTestStatement();
    std::shared_ptr<ast::MockStatement> parseMockStatement();
    ast::StatementPtr parseBaStatement();
    std::shared_ptr<ast::ThreadsStatement> parseThreadsStatement();
    std::shared_ptr<ast::TxStatement> parseTxStatement();
    std::shared_ptr<ast::WaitStatement> parseWaitStatement();
    std::shared_ptr<ast::AssertStatement> parseAssertStatement();
    std::shared_ptr<ast::MatchStatement> parseMatchStatement();
    std::shared_ptr<ast::ExpressionStatement> parseExpressionStatement();

    std::shared_ptr<ast::TypeAnnotation> parseTypeAnnotation();
    std::shared_ptr<ast::TypeAnnotation> parseTypeAnnotationWithUnion(bool allowUnion);

    ast::ExpressionPtr parseExpression(int precedence);
    ast::ExpressionPtr parseIdentifier();
    ast::ExpressionPtr parseTypeIdentifier();
    ast::ExpressionPtr parseIntegerLiteral();
    ast::ExpressionPtr parseFloatLiteral();
    ast::ExpressionPtr parseStringLiteral();
    ast::ExpressionPtr parseFStringLiteral();
    ast::ExpressionPtr parseBooleanLiteral();
    ast::ExpressionPtr parseNullLiteral();
    ast::ExpressionPtr parsePrefixExpression();
    ast::ExpressionPtr parseGroupedExpression();
    ast::ExpressionPtr parseListLiteral();
    ast::ExpressionPtr parseMapLiteral();
    ast::ExpressionPtr parseErrExpression();
    ast::ExpressionPtr parseLambdaExpression();
    ast::ExpressionPtr parsePrefixRangeLiteral();
    std::vector<ast::ExpressionPtr> parseExpressionList(token::Kind end);
    ast::ExpressionPtr parseInfixExpression(ast::ExpressionPtr left);
    ast::ExpressionPtr parsePowerExpression(ast::ExpressionPtr left);
    ast::ExpressionPtr parseAssignExpression(ast::ExpressionPtr left);
    ast::ExpressionPtr parsePipeExpression(ast::ExpressionPtr left);
    ast::ExpressionPtr parseTimeoutExpression(ast::ExpressionPtr left);
    ast::ExpressionPtr parseStreamExpression(ast::ExpressionPtr left);
    ast::ExpressionPtr parseRangeExpression(ast::ExpressionPtr left);
    ast::ExpressionPtr parseCallExpression(ast::ExpressionPtr fn);
    std::pair<std::vector<ast::ExpressionPtr>, std::map<std::string, ast::ExpressionPtr>> parseCallArguments();
    ast::ExpressionPtr parseIndexExpression(ast::ExpressionPtr left);
    ast::ExpressionPtr parseDotExpression(ast::ExpressionPtr left);
    void skipExpressionContinuation();

    std::unique_ptr<lexer::Lexer> ownedLexer_;
    lexer::Lexer* l_ = nullptr;
    token::Token curToken_;
    token::Token peekToken_;
    std::vector<std::string> errors_;
    std::vector<std::shared_ptr<diag::SyntaxError>> syntaxErrors_;
    bool constexprModifier_ = false;
    int continuationDedents_ = 0;

    std::map<token::Kind, PrefixParseFn> prefixParseFns_;
    std::map<token::Kind, InfixParseFn> infixParseFns_;
    std::map<token::Kind, int> precedences_;
};

} // namespace ulang::parser
