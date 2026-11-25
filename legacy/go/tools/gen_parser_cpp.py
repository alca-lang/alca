# Copyright 2026 Pawl Programming Language
# SPDX-License-Identifier: Apache-2.0

import os

parser_cpp = r'''// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

#include "parser/parser.h"

#include <algorithm>
#include <charconv>
#include <sstream>

namespace pawl::parser {

namespace {

bool isNilStatement(const ast::StatementPtr& s) {
    return s == nullptr;
}

std::string firstModifier(const std::string& visibility, bool isConst) {
    if (!visibility.empty()) {
        if (isConst) return visibility + " const";
        return visibility;
    }
    if (isConst) return "const";
    return "";
}

struct FStringPart {
    bool isExpr = false;
    std::string text;
};

std::string trimSpace(const std::string& s) {
    size_t start = 0;
    while (start < s.size() && std::isspace(static_cast<unsigned char>(s[start]))) ++start;
    size_t end = s.size();
    while (end > start && std::isspace(static_cast<unsigned char>(s[end - 1]))) --end;
    return s.substr(start, end - start);
}

} // namespace

Parser::Parser(std::unique_ptr<lexer::Lexer> lexer)
    : ownedLexer_(std::move(lexer)), l_(ownedLexer_.get()) {
    
    // Register precedences
    precedences_[token::STREAM_OUT]       = STREAM;
    precedences_[token::STREAM_APPEND]    = STREAM;
    precedences_[token::STREAM_OVERWRITE] = STREAM;
    precedences_[token::STREAM_VAR]       = STREAM;
    precedences_[token::STREAM_CAPTURE]   = STREAM;
    precedences_[token::STREAM_IN]        = STREAM;
    precedences_[token::STREAM_ERR]       = STREAM;
    precedences_[token::STREAM_ERR_APPEND]= STREAM;
    precedences_[token::STREAM_ERR_VAR]   = STREAM;
    precedences_[token::STREAM_ERR_CAPTURE]= STREAM;
    precedences_[token::STREAM_ASYNC]     = STREAM;
    precedences_[token::STREAM_THREAD]    = STREAM;

    precedences_[token::PIPE]             = PIPE;
    precedences_[token::PIPE_ARROW]       = PIPE;
    precedences_[token::ASSIGN]           = ASSIGN;
    precedences_[token::OR]               = OR;
    precedences_[token::AND]              = AND;
    precedences_[token::RANGE]            = RANGE;
    precedences_[token::DOTDOT]           = RANGE;
    precedences_[token::DOTDOTEQ]         = RANGE;
    precedences_[token::EQ]               = EQUALS;
    precedences_[token::STRICT_EQ]        = EQUALS;
    precedences_[token::NOT_EQ]           = EQUALS;
    precedences_[token::LT]               = LESSGREATER;
    precedences_[token::GT]               = LESSGREATER;
    precedences_[token::LT_EQ]            = LESSGREATER;
    precedences_[token::GT_EQ]            = LESSGREATER;
    precedences_[token::LTE]              = LESSGREATER;
    precedences_[token::GTE]              = LESSGREATER;
    precedences_[token::IN]               = LESSGREATER;
    precedences_[token::PLUS]             = SUM;
    precedences_[token::MINUS]            = SUM;
    precedences_[token::SLASH]            = PRODUCT;
    precedences_[token::ASTERISK]         = PRODUCT;
    precedences_[token::PERCENT]          = PRODUCT;
    precedences_[token::POWER]            = POWER;
    precedences_[token::LPAREN]           = CALL;
    precedences_[token::LBRACKET]         = INDEX;
    precedences_[token::DOT]              = INDEX;
    precedences_[token::TILDE]            = TIMEOUT;

    // Register prefix parse functions
    registerPrefix(token::IDENT, [this] { return parseIdentifier(); });
    registerPrefix(token::INT, [this] { return parseIntegerLiteral(); });
    registerPrefix(token::FLOAT, [this] { return parseFloatLiteral(); });
    registerPrefix(token::STRING, [this] { return parseStringLiteral(); });
    registerPrefix(token::FSTRING, [this] { return parseFStringLiteral(); });
    registerPrefix(token::TRUE, [this] { return parseBooleanLiteral(); });
    registerPrefix(token::FALSE, [this] { return parseBooleanLiteral(); });
    registerPrefix(token::NULL_TOKEN, [this] { return parseNullLiteral(); });
    registerPrefix(token::BANG, [this] { return parsePrefixExpression(); });
    registerPrefix(token::MINUS, [this] { return parsePrefixExpression(); });
    registerPrefix(token::NOT, [this] { return parsePrefixExpression(); });
    registerPrefix(token::LPAREN, [this] { return parseGroupedExpression(); });
    registerPrefix(token::LBRACKET, [this] { return parseListLiteral(); });
    registerPrefix(token::LBRACE, [this] { return parseMapLiteral(); });
    registerPrefix(token::ERR, [this] { return parseErrExpression(); });
    registerPrefix(token::PIPE_TYPE, [this] { return parseLambdaExpression(); });
    registerPrefix(token::AWAIT, [this] { return parseAwaitExpression(); });
    registerPrefix(token::RANGE, [this] { return parsePrefixRangeLiteral(); });
    registerPrefix(token::DOTDOT, [this] { return parsePrefixRangeLiteral(); });
    registerPrefix(token::DOTDOTEQ, [this] { return parsePrefixRangeLiteral(); });

    // Built-in type keywords as prefix identifiers
    token::Kind typeTokens[] = {
        token::TYPE_STRING, token::TYPE_INT, token::TYPE_FLOAT,
        token::TYPE_BOOL, token::TYPE_LIST, token::TYPE_MAP,
        token::TYPE_BLOB, token::TYPE_SECRET, token::TYPE_JSON
    };
    for (auto tt : typeTokens) {
        registerPrefix(tt, [this] { return parseTypeIdentifier(); });
    }

    // Register infix parse functions
    registerInfix(token::PLUS, [this](auto left) { return parseInfixExpression(left); });
    registerInfix(token::MINUS, [this](auto left) { return parseInfixExpression(left); });
    registerInfix(token::ASTERISK, [this](auto left) { return parseInfixExpression(left); });
    registerInfix(token::SLASH, [this](auto left) { return parseInfixExpression(left); });
    registerInfix(token::PERCENT, [this](auto left) { return parseInfixExpression(left); });
    registerInfix(token::POWER, [this](auto left) { return parsePowerExpression(left); });

    registerInfix(token::EQ, [this](auto left) { return parseInfixExpression(left); });
    registerInfix(token::STRICT_EQ, [this](auto left) { return parseInfixExpression(left); });
    registerInfix(token::NOT_EQ, [this](auto left) { return parseInfixExpression(left); });
    registerInfix(token::LT, [this](auto left) { return parseInfixExpression(left); });
    registerInfix(token::GT, [this](auto left) { return parseInfixExpression(left); });
    registerInfix(token::LT_EQ, [this](auto left) { return parseInfixExpression(left); });
    registerInfix(token::GT_EQ, [this](auto left) { return parseInfixExpression(left); });
    registerInfix(token::LTE, [this](auto left) { return parseInfixExpression(left); });
    registerInfix(token::GTE, [this](auto left) { return parseInfixExpression(left); });
    registerInfix(token::IN, [this](auto left) { return parseInfixExpression(left); });

    registerInfix(token::AND, [this](auto left) { return parseInfixExpression(left); });
    registerInfix(token::OR, [this](auto left) { return parseInfixExpression(left); });

    registerInfix(token::ASSIGN, [this](auto left) { return parseAssignExpression(left); });
    registerInfix(token::PIPE, [this](auto left) { return parsePipeExpression(left); });
    registerInfix(token::PIPE_ARROW, [this](auto left) { return parsePipeExpression(left); });
    registerInfix(token::TILDE, [this](auto left) { return parseTimeoutExpression(left); });

    token::Kind streamTokens[] = {
        token::STREAM_OUT, token::STREAM_APPEND, token::STREAM_OVERWRITE,
        token::STREAM_VAR, token::STREAM_CAPTURE, token::STREAM_IN,
        token::STREAM_ERR, token::STREAM_ERR_APPEND, token::STREAM_ERR_VAR,
        token::STREAM_ERR_CAPTURE, token::STREAM_ASYNC, token::STREAM_THREAD
    };
    for (auto st : streamTokens) {
        registerInfix(st, [this](auto left) { return parseStreamExpression(left); });
    }

    registerInfix(token::RANGE, [this](auto left) { return parseRangeExpression(left); });
    registerInfix(token::DOTDOT, [this](auto left) { return parseRangeExpression(left); });
    registerInfix(token::DOTDOTEQ, [this](auto left) { return parseRangeExpression(left); });

    registerInfix(token::LPAREN, [this](auto left) { return parseCallExpression(left); });
    registerInfix(token::LBRACKET, [this](auto left) { return parseIndexExpression(left); });
    registerInfix(token::DOT, [this](auto left) { return parseDotExpression(left); });

    // Read initial tokens skipping comments
    curToken_ = l_->nextToken();
    while (curToken_.type == token::COMMENT) {
        curToken_ = l_->nextToken();
    }
    peekToken_ = l_->nextToken();
    while (peekToken_.type == token::COMMENT) {
        peekToken_ = l_->nextToken();
    }
}

Parser::Parser(lexer::Lexer* lexer) : Parser(std::unique_ptr<lexer::Lexer>(nullptr)) {
    l_ = lexer;
    curToken_ = l_->nextToken();
    while (curToken_.type == token::COMMENT) {
        curToken_ = l_->nextToken();
    }
    peekToken_ = l_->nextToken();
    while (peekToken_.type == token::COMMENT) {
        peekToken_ = l_->nextToken();
    }
}

std::string Parser::diagnosticReport(const std::string& filename, const std::string& source, bool useColor) const {
    return diag::formatReport(filename, source, syntaxErrors_, useColor);
}

std::shared_ptr<ast::Program> Parser::parseProgram() {
    auto program = std::make_shared<ast::Program>();
    skipNewlines();

    while (!curTokenIs(token::EOF_TOKEN)) {
        auto prevTok = curToken_;
        auto stmt = parseStatement();
        if (stmt) {
            program->statements.push_back(stmt);
        } else {
            nextToken();
        }
        skipNewlines();
        if (curToken_.type == prevTok.type && curToken_.literal == prevTok.literal &&
            curToken_.line == prevTok.line && curToken_.column == prevTok.column) {
            nextToken();
            skipNewlines();
        }
    }

    return program;
}

void Parser::nextToken() {
    curToken_ = peekToken_;
    peekToken_ = l_->nextToken();
    while (peekToken_.type == token::COMMENT) {
        peekToken_ = l_->nextToken();
    }
}

bool Parser::curTokenIs(token::Kind t) const {
    return curToken_.type == t;
}

bool Parser::peekTokenIs(token::Kind t) const {
    return peekToken_.type == t;
}

bool Parser::expectPeek(token::Kind t) {
    if (peekTokenIs(t)) {
        nextToken();
        return true;
    }
    peekError(t);
    return false;
}

void Parser::peekError(token::Kind t) {
    auto tok = peekToken_;
    int length = static_cast<int>(tok.literal.length());
    if (length <= 0) length = 1;

    std::string msg, suggestion;
    switch (t) {
    case token::RPAREN:
        msg = "expected ')' to close parenthesis, got \"" + tok.literal + "\" (" + std::string(token::kindName(tok.type) + ")";
        suggestion = "did you forget a closing ')'?";
        break;
    case token::RBRACKET:
        msg = "expected ']' to close list or index, got \"" + tok.literal + "\" (" + std::string(token::kindName(tok.type) + ")";
        suggestion = "did you forget a closing ']'?";
        break;
    case token::RBRACE:
        msg = "expected '}' to close block or map, got \"" + tok.literal + "\" (" + std::string(token::kindName(tok.type) + ")";
        suggestion = "did you forget a closing '}'?";
        break;
    case token::COLON:
        msg = "expected ':' before block or in type annotation, got \"" + tok.literal + "\" (" + std::string(token::kindName(tok.type) + ")";
        suggestion = "block headers and type annotations require ':'";
        break;
    case token::ASSIGN:
        msg = "expected '=' after declaration, got \"" + tok.literal + "\" (" + std::string(token::kindName(tok.type) + ")";
        suggestion = "variable declarations require an initial value: 'x: type = value;' or 'x := value;'";
        break;
    case token::IN:
        msg = "expected 'in' in for-loop, got \"" + tok.literal + "\" (" + std::string(token::kindName(tok.type) + ")";
        suggestion = "for-loops use the syntax 'for <item> in <iterable>:'";
        break;
    case token::LPAREN:
        msg = "expected '(' after function or call, got \"" + tok.literal + "\" (" + std::string(token::kindName(tok.type) + ")";
        suggestion = "enclose arguments or parameter lists in parentheses '(...)'";
        break;
    default:
        msg = "expected next token to be " + std::string(std::string(token::kindName(t)) + ", got \"" + tok.literal + "\" (" + std::string(token::kindName(tok.type) + ")";
        suggestion = "expected '" + std::string(std::string(token::kindName(t)) + "' here";
        break;
    }

    errors_.push_back("line " + std::to_string(tok.line) + ", col " + std::to_string(tok.column) + ": " + msg);
    auto err = std::make_shared<diag::SyntaxError>();
    err->filename = "";
    err->line = tok.line;
    err->column = tok.column;
    err->message = msg;
    err->token = std::string(token::kindName(tok.type);
    err->length = length;
    err->suggestion = suggestion;
    syntaxErrors_.push_back(err);
}

bool Parser::expectBlockBrace(const std::string& contextName) {
    if (peekTokenIs(token::LBRACE)) {
        nextToken();
        return true;
    }
    if (curTokenIs(token::LBRACE)) {
        return true;
    }
    if (peekTokenIs(token::COLON)) {
        reportAt(peekToken_, "use '{' to open a block, not ':'", "expected '{' after " + contextName);
        nextToken();
        return true;
    }
    if (curTokenIs(token::COLON)) {
        reportAt(curToken_, "use '{' to open a block, not ':'", "expected '{' after " + contextName);
        return true;
    }

    int col = curToken_.column + static_cast<int>(curToken_.literal.length());
    errors_.push_back("line " + std::to_string(curToken_.line) + ", col " + std::to_string(col) + ": expected '{' after " + contextName);
    auto err = std::make_shared<diag::SyntaxError>();
    err->filename = "";
    err->line = curToken_.line;
    err->column = col;
    err->message = "expected '{' after " + contextName;
    err->token = curToken_.literal;
    err->length = 1;
    err->suggestion = "add '{' after '" + contextName + "' to begin the block";
    syntaxErrors_.push_back(err);
    return false;
}

void Parser::noPrefixParseFnError(token::Kind t) {
    auto tok = curToken_;
    int length = static_cast<int>(tok.literal.length());
    if (length <= 0) length = 1;

    std::string msg, suggestion;
    switch (t) {
    case token::RPAREN:
        msg = "unexpected closing ')' without matching '('";
        suggestion = "check for an unclosed '(' earlier, or remove this extra ')'";
        break;
    case token::RBRACKET:
        msg = "unexpected closing ']' without matching '['";
        suggestion = "check for an unclosed '[' earlier, or remove this extra ']'";
        break;
    case token::RBRACE:
        msg = "unexpected closing '}' without matching '{'";
        suggestion = "check for an unclosed '{' earlier, or remove this extra '}'";
        break;
    case token::COLON:
        msg = "unexpected ':' outside of a type annotation or block header";
        suggestion = "use ':' only at the end of block headers ('if cond:', 'func foo():') or in variable types ('x: int = 5;')";
        break;
    case token::SEMICOLON:
        msg = "unexpected ';' (empty statement)";
        suggestion = "remove the stray semicolon or provide an expression before ';'";
        break;
    case token::ASSIGN:
        msg = "unexpected '=' assignment operator";
        suggestion = "check if you meant ':=' for declaration or '==' for comparison";
        break;
    case token::COMMA:
        msg = "unexpected comma ',' outside of a list or argument sequence";
        suggestion = "commas separate items in lists '[a, b]', arguments 'f(a, b)', or tuples";
        break;
    case token::IN:
        msg = "unexpected 'in' keyword";
        suggestion = "'in' is used in 'for item in collection:' or membership check 'item in list'";
        break;
    case token::INDENT:
        msg = "unexpected indentation";
        suggestion = "indentation is only valid inside block bodies (following ':')";
        break;
    case token::DEDENT:
        msg = "unaligned dedent";
        suggestion = "check block indentation: indentation decreased to an invalid level";
        break;
    case token::ILLEGAL:
        msg = "illegal character \"" + tok.literal + "\"";
        suggestion = "remove or replace the invalid character";
        break;
    default:
        msg = "unexpected token \"" + tok.literal + "\" (" + std::string(std::string(token::kindName(t)) + ")";
        suggestion = "check the syntax around this token";
        break;
    }

    errors_.push_back("line " + std::to_string(tok.line) + ", col " + std::to_string(tok.column) + ": " + msg);
    auto err = std::make_shared<diag::SyntaxError>();
    err->filename = "";
    err->line = tok.line;
    err->column = tok.column;
    err->message = msg;
    err->token = std::string(std::string(token::kindName(t));
    err->length = length;
    err->suggestion = suggestion;
    syntaxErrors_.push_back(err);
}

int Parser::peekPrecedence() const {
    auto it = precedences_.find(peekToken_.type);
    if (it != precedences_.end()) return it->second;
    return LOWEST;
}

int Parser::curPrecedence() const {
    auto it = precedences_.find(curToken_.type);
    if (it != precedences_.end()) return it->second;
    return LOWEST;
}

void Parser::registerPrefix(token::Kind tokenType, PrefixParseFn fn) {
    prefixParseFns_[tokenType] = std::move(fn);
}

void Parser::registerInfix(token::Kind tokenType, InfixParseFn fn) {
    infixParseFns_[tokenType] = std::move(fn);
}

void Parser::skipNewlines() {
    while (curTokenIs(token::NEWLINE)) {
        nextToken();
    }
}

void Parser::skipPeekNewlines() {
    while (peekTokenIs(token::NEWLINE)) {
        nextToken();
    }
}

void Parser::consumeStatementEnd() {
    if (peekTokenIs(token::SEMICOLON)) {
        reportAt(peekToken_, "remove the ';', statements end at the newline", "semicolons are not allowed in Pawl");
        nextToken(); // cur = ';'
        nextToken(); // cur = token after ';'
        return;
    }
    if (peekTokenIs(token::NEWLINE) || peekTokenIs(token::EOF_TOKEN) ||
        peekTokenIs(token::DEDENT) || peekTokenIs(token::RBRACE)) {
        nextToken(); // cur = terminator
        return;
    }

    auto end = curToken_;
    int length = static_cast<int>(end.literal.length());
    if (end.type == token::STRING) {
        length += 2;
    }
    int col = end.column + length;
    errors_.push_back("line " + std::to_string(end.line) + ", col " + std::to_string(col) + ": expected end of statement (newline)");
    auto err = std::make_shared<diag::SyntaxError>();
    err->line = end.line;
    err->column = col;
    err->message = "expected end of statement (newline)";
    err->token = std::string(std::string(token::kindName(peekToken_.type));
    err->length = 1;
    err->suggestion = "put each statement on its own line, without a trailing ';'";
    syntaxErrors_.push_back(err);

    while (!peekTokenIs(token::NEWLINE) && !peekTokenIs(token::EOF_TOKEN) &&
           !peekTokenIs(token::DEDENT) && !peekTokenIs(token::RBRACE)) {
        nextToken();
    }
    nextToken();
}

ast::StatementPtr Parser::parseStatement() {
    auto stmt = parseStatementInternal();
    if (isNilStatement(stmt)) return nullptr;
    return stmt;
}

ast::StatementPtr Parser::parseStatementInternal() {
    std::vector<std::shared_ptr<ast::Decorator>> decorators;
    while (curTokenIs(token::AT)) {
        if (peekTokenIs(token::BA)) {
            nextToken();
            return parseBaStatement();
        }
        if (peekTokenIs(token::THREADS)) {
            nextToken();
            return parseThreadsStatement();
        }
        auto dec = parseDecorator();
        if (dec) decorators.push_back(dec);
        skipNewlines();
    }

    std::string visibility;
    bool isConst = false;
    parseVarModifiers(visibility, isConst);

    bool isPublic = false;
    if (curTokenIs(token::PUB)) {
        isPublic = true;
        nextToken();
    }
    if (visibility == "public") {
        isPublic = true;
    }

    bool isAsync = false;
    if (curTokenIs(token::ASYNC)) {
        isAsync = true;
        nextToken();
        skipNewlines();
    }

    switch (curToken_.type) {
    case token::FUNC:
    case token::TYPE_VOID: {
        if (isConst) {
            reportAt(curToken_, "functions cannot be declared 'const'", "const function");
        }
        auto fn = parseFuncDeclaration(isPublic, decorators);
        if (fn) fn->isAsync = isAsync;
        return fn;
    }
    case token::ENUM:
        return parseEnumDeclaration(isPublic, decorators);
    case token::INTERFACE:
    case token::TRAIT:
        return parseInterfaceDeclaration(isPublic, decorators);
    case token::STRUCT:
        return parseStructDeclaration(isPublic, decorators);
    case token::CLASS: {
        if (visibility == "private") {
            reportAt(curToken_, "classes are visible to the whole module; remove 'private'", "private class");
            visibility = "";
        }
        if (isConst) {
            reportAt(curToken_, "classes cannot be declared 'const'", "const class");
            isConst = false;
        }
        if (visibility == "public") isPublic = true;
        return parseClassDeclaration(isPublic, decorators);
    }
    case token::IF:
        return parseIfStatement();
    case token::WHILE:
        return parseWhileStatement();
    case token::FOR:
        return parseForStatement();
    case token::FOREACH:
        return parseForEachStatement();
    case token::LOOP:
        return parseLoopStatement();
    case token::RETURN:
        return parseReturnStatement();
    case token::BREAK:
        return parseBreakStatement();
    case token::CONTINUE:
        return parseContinueStatement();
    case token::USE:
    case token::IMPORT:
        return parseUseStatement();
    case token::WATCH:
        return parseWatchStatement();
    case token::TEST:
        return parseTestStatement();
    case token::MOCK:
        return parseMockStatement();
    case token::BA:
        return parseBaStatement();
    case token::THREADS:
        return parseThreadsStatement();
    case token::TX:
        return parseTxStatement();
    case token::WAIT:
        return parseWaitStatement();
    case token::ASSERT:
        return parseAssertStatement();
    case token::MATCH:
        return parseMatchStatement();
    case token::IDENT:
    case token::TYPE_STRING:
    case token::TYPE_INT:
    case token::TYPE_FLOAT:
    case token::TYPE_BOOL:
    case token::TYPE_LIST:
    case token::TYPE_MAP:
    case token::TYPE_BLOB:
    case token::TYPE_SECRET:
    case token::TYPE_JSON: {
        if (token::isTypeKeyword(curToken_.type) && (peekTokenIs(token::IDENT) || peekTokenIs(token::COLON))) {
            if (!visibility.empty() || isConst) {
                return parseTypedVarDeclaration(visibility, isConst, decorators);
            }
            reportAt(curToken_, "declare variables as 'name: " + curToken_.literal + " = value;'",
                     "C-style declarations are not supported");
            while (!peekTokenIs(token::NEWLINE) && !peekTokenIs(token::EOF_TOKEN) && !peekTokenIs(token::DEDENT)) {
                nextToken();
            }
            nextToken();
            return nullptr;
        }
        if (curTokenIs(token::IDENT) && peekTokenIs(token::COLON)) {
            auto vd = parseVarDeclaration(decorators);
            if (vd) {
                vd->visibility = visibility;
                vd->isConst = isConst;
            }
            return vd;
        }
        if (!visibility.empty() || isConst) {
            reportAt(curToken_, "write 'public int: x = 10;', 'private int: x = 10;' or 'const int: x = 10;'",
                     "modifier \"" + curToken_.literal + "\" must be followed by a variable declaration");
            consumeStatementEnd();
            return nullptr;
        }
        return parseExpressionStatement();
    }
    default:
        if (!visibility.empty() || isConst) {
            reportAt(curToken_, "write 'public int: x = 10;', 'private int: x = 10;' or 'const int: x = 10;'",
                     "modifier \"" + curToken_.literal + "\" must be followed by a variable declaration");
            consumeStatementEnd();
            return nullptr;
        }
        return parseExpressionStatement();
    }
}

void Parser::parseVarModifiers(std::string& visibility, bool& isConst) {
    for (;;) {
        switch (curToken_.type) {
        case token::PUBLIC:
            if (visibility == "private") {
                reportAt(curToken_, "keep a single visibility: 'public' or 'private'", "conflicting visibility modifiers");
            }
            visibility = "public";
            nextToken();
            break;
        case token::PRIVATE:
            if (visibility == "public") {
                reportAt(curToken_, "keep a single visibility: 'public' or 'private'", "conflicting visibility modifiers");
            }
            visibility = "private";
            nextToken();
            break;
        case token::CONST:
            if (isConst) {
                reportAt(curToken_, "remove the duplicate 'const'", "duplicate 'const' modifier");
            }
            isConst = true;
            nextToken();
            break;
        default:
            return;
        }
    }
}

void Parser::reportAt(const token::Token& tok, const std::string& suggestion, const std::string& msg) {
    int length = static_cast<int>(tok.literal.length());
    if (length <= 0) length = 1;
    errors_.push_back("line " + std::to_string(tok.line) + ", col " + std::to_string(tok.column) + ": " + msg);
    auto err = std::make_shared<diag::SyntaxError>();
    err->line = tok.line;
    err->column = tok.column;
    err->message = msg;
    err->token = std::string(token::kindName(tok.type);
    err->length = length;
    err->suggestion = suggestion;
    syntaxErrors_.push_back(err);
}

std::shared_ptr<ast::Decorator> Parser::parseDecorator() {
    auto tok = curToken_; // '@'
    nextToken();

    if (!curTokenIs(token::IDENT)) {
        reportAt(curToken_, "provide a decorator name: '@memoize' or '@timeout(5s)'",
                 "expected identifier after '@', got \"" + curToken_.literal + "\"");
        return nullptr;
    }

    std::string name = curToken_.literal;
    ast::ExpressionPtr arg;

    if (peekTokenIs(token::LPAREN)) {
        nextToken(); // '('
        auto args = parseExpressionList(token::RPAREN);
        if (!args.empty()) arg = args[0];
        if (curTokenIs(token::RPAREN)) {
            nextToken();
        }
    } else {
        nextToken();
    }

    auto dec = std::make_shared<ast::Decorator>();
    dec->token = tok;
    dec->name = name;
    dec->arg = arg;
    return dec;
}

std::shared_ptr<ast::VarDeclaration> Parser::parseVarDeclaration(const std::vector<std::shared_ptr<ast::Decorator>>& decorators) {
    auto tok = curToken_;
    auto name = std::make_shared<ast::Identifier>();
    name->token = tok;
    name->value = tok.literal;

    std::shared_ptr<ast::TypeAnnotation> typeAnnotation;
    ast::ExpressionPtr value;

    nextToken(); // ':'
    if (peekTokenIs(token::ASSIGN)) {
        nextToken(); // '='
    } else {
        nextToken(); // type
        typeAnnotation = parseTypeAnnotation();
        if (!peekTokenIs(token::ASSIGN)) {
            reportAt(curToken_, "initialize it: 'name: type = value;'", "variable declaration requires an initial value");
            consumeStatementEnd();
            auto vd = std::make_shared<ast::VarDeclaration>();
            vd->token = tok;
            vd->name = name;
            vd->typeAnnotation = typeAnnotation;
            vd->decorators = decorators;
            return vd;
        }
        nextToken(); // '='
    }
    nextToken(); // value expression
    value = parseExpression(LOWEST);

    consumeStatementEnd();

    auto vd = std::make_shared<ast::VarDeclaration>();
    vd->token = tok;
    vd->name = name;
    vd->typeAnnotation = typeAnnotation;
    vd->value = value;
    vd->decorators = decorators;
    return vd;
}

std::shared_ptr<ast::VarDeclaration> Parser::parseTypedVarDeclaration(const std::string& visibility, bool isConst,
                                                                     const std::vector<std::shared_ptr<ast::Decorator>>& decorators) {
    auto typeTok = curToken_;
    auto typeAnnotation = parseTypeAnnotation();

    if (peekTokenIs(token::COLON)) {
        nextToken();
    }
    if (!peekTokenIs(token::IDENT) && !token::isTypeKeyword(peekToken_.type)) {
        reportAt(curToken_, "write '" + firstModifier(visibility, isConst) + " int: x = 10;'",
                 "expected variable name after type \"" + typeAnnotation->toString() + "\", got \"" + peekToken_.literal + "\"");
        consumeStatementEnd();
        return nullptr;
    }
    nextToken(); // variable name

    auto nameTok = curToken_;
    auto name = std::make_shared<ast::Identifier>();
    name->token = nameTok;
    name->value = nameTok.literal;

    ast::ExpressionPtr value;
    if (peekTokenIs(token::ASSIGN)) {
        nextToken(); // '='
        nextToken(); // value expression
        value = parseExpression(LOWEST);
    } else {
        reportAt(nameTok, "initialize it: 'name: type = value;'", "variable declaration requires an initial value");
    }

    consumeStatementEnd();

    auto vd = std::make_shared<ast::VarDeclaration>();
    vd->token = typeTok;
    vd->name = name;
    vd->typeAnnotation = typeAnnotation;
    vd->value = value;
    vd->decorators = decorators;
    vd->visibility = visibility;
    vd->isConst = isConst;
    return vd;
}

std::shared_ptr<ast::FuncDeclaration> Parser::parseFuncDeclaration(bool isPublic, const std::vector<std::shared_ptr<ast::Decorator>>& decorators) {
    auto tok = curToken_;
    bool isVoidPrefix = (tok.type == token::TYPE_VOID);

    nextToken(); // past func/void
    if (isVoidPrefix && curTokenIs(token::FUNC)) {
        nextToken();
    }
    if (!curTokenIs(token::IDENT) && !token::isTypeKeyword(curToken_.type)) {
        reportAt(curToken_, "provide a valid function name, e.g. 'func calculate():'",
                 "expected function name after '" + tok.literal + "', got \"" + curToken_.literal + "\"");
        return nullptr;
    }
    std::string name = curToken_.literal;

    if (!expectPeek(token::LPAREN)) {
        return nullptr;
    }

    auto params = parseFuncParams();

    std::shared_ptr<ast::TypeAnnotation> returnType;
    if (isVoidPrefix) {
        returnType = std::make_shared<ast::TypeAnnotation>();
        returnType->token = tok;
        returnType->types = {"void"};
    }
    if (peekTokenIs(token::ARROW)) {
        nextToken(); // '->'
        nextToken(); // type start
        auto ret = parseTypeAnnotation();
        if (isVoidPrefix && (ret->types.size() != 1 || ret->types[0] != "void")) {
            reportAt(tok, "a function declared with 'void' cannot return a non-void type", "conflicting return type");
        }
        returnType = ret;
    }

    std::shared_ptr<ast::BlockStatement> body;
    if (peekTokenIs(token::LBRACE)) {
        nextToken();
        body = parseBraceBlockStatement();
    } else if (peekTokenIs(token::COLON)) {
        reportAt(peekToken_, "use '{' to open a function body, not ':'", "expected '{' after function signature");
        nextToken();
        body = parseBlockStatement();
    } else if (peekTokenIs(token::NEWLINE)) {
        skipPeekNewlines();
        if (peekTokenIs(token::LBRACE)) {
            nextToken();
            body = parseBraceBlockStatement();
        } else if (peekTokenIs(token::COLON)) {
            reportAt(peekToken_, "use '{' to open a function body, not ':'", "expected '{' after function signature");
            nextToken();
            body = parseBlockStatement();
        } else if (curTokenIs(token::LBRACE)) {
            body = parseBraceBlockStatement();
        } else if (curTokenIs(token::COLON)) {
            reportAt(curToken_, "use '{' to open a function body, not ':'", "expected '{' after function signature");
            body = parseBlockStatement();
        } else {
            reportAt(curToken_, "provide '{' to start the function body", "expected '{' after function signature");
            return nullptr;
        }
    } else if (curTokenIs(token::LBRACE)) {
        body = parseBraceBlockStatement();
    } else if (curTokenIs(token::COLON)) {
        reportAt(curToken_, "use '{' to open a function body, not ':'", "expected '{' after function signature");
        body = parseBlockStatement();
    } else {
        reportAt(curToken_, "provide '{' to start the function body", "expected '{' after function signature");
        return nullptr;
    }

    auto fn = std::make_shared<ast::FuncDeclaration>();
    fn->token = tok;
    fn->name = name;
    fn->params = params;
    fn->returnType = returnType;
    fn->body = body;
    fn->isPublic = isPublic;
    fn->decorators = decorators;
    return fn;
}

std::vector<std::shared_ptr<ast::FuncParam>> Parser::parseFuncParams() {
    std::vector<std::shared_ptr<ast::FuncParam>> params;

    skipPeekNewlines();
    if (peekTokenIs(token::RPAREN)) {
        nextToken();
        return params;
    }

    nextToken(); // first param name
    skipNewlines();

    while (!curTokenIs(token::RPAREN) && !curTokenIs(token::EOF_TOKEN)) {
        auto paramTok = curToken_;
        std::string paramName = paramTok.literal;
        auto param = std::make_shared<ast::FuncParam>();
        param->token = paramTok;
        param->name = paramName;

        if (peekTokenIs(token::COLON)) {
            nextToken(); // ':'
            nextToken(); // type start
            param->typeAnnotation = parseTypeAnnotation();
        } else if (peekTokenIs(token::IDENT) || token::isTypeKeyword(peekToken_.type)) {
            reportAt(peekToken_, "write parameter types with a colon: '" + paramName + ": " + peekToken_.literal + "'",
                     "missing ':' before type annotation for parameter '" + paramName + "'");
            nextToken();
            param->typeAnnotation = parseTypeAnnotation();
        }

        if (peekTokenIs(token::ASSIGN)) {
            nextToken(); // '='
            nextToken(); // default value
            param->defaultValue = parseExpression(LOWEST);
        }

        params.push_back(param);

        skipPeekNewlines();
        if (peekTokenIs(token::COMMA)) {
            nextToken(); // ','
            skipPeekNewlines();
            if (peekTokenIs(token::RPAREN)) {
                nextToken(); // trailing comma
                return params;
            }
            nextToken(); // next param
            skipNewlines();
        } else {
            break;
        }
    }

    skipPeekNewlines();
    if (!expectPeek(token::RPAREN)) {
        return {};
    }

    return params;
}

std::shared_ptr<ast::StructDeclaration> Parser::parseStructDeclaration(bool isPublic, const std::vector<std::shared_ptr<ast::Decorator>>& decorators) {
    auto structTok = curToken_;
    nextToken(); // past 'struct'

    if (!curTokenIs(token::IDENT)) {
        reportAt(curToken_, "provide a struct name, e.g. 'struct User:'",
                 "expected struct name after 'struct', got \"" + curToken_.literal + "\"");
        return nullptr;
    }

    std::string name = curToken_.literal;
    bool isBraced = false;

    skipPeekNewlines();
    if (peekTokenIs(token::LBRACE)) {
        nextToken();
        isBraced = true;
        nextToken();
    } else if (curTokenIs(token::LBRACE)) {
        isBraced = true;
        nextToken();
    } else {
        expectBlockBrace("struct " + name);
        if (curTokenIs(token::COLON)) nextToken();
        skipNewlines();
        if (curTokenIs(token::INDENT)) nextToken();
    }

    skipNewlines();

    std::vector<std::shared_ptr<ast::StructField>> fields;

    auto isEnd = [&] {
        if (isBraced) {
            return curTokenIs(token::RBRACE) || curTokenIs(token::EOF_TOKEN);
        }
        return curTokenIs(token::DEDENT) || curTokenIs(token::EOF_TOKEN);
    };

    while (!isEnd()) {
        auto prevTok = curToken_;
        if (isBraced) {
            skipBraceBlockTrivia();
        } else {
            skipNewlines();
        }
        if (isEnd()) break;

        std::string fieldVisibility;
        bool fieldIsConst = false;
        parseVarModifiers(fieldVisibility, fieldIsConst);
        auto fieldTok = curToken_;
        std::string fieldName = fieldTok.literal;

        std::shared_ptr<ast::TypeAnnotation> typeAnnot;
        ast::ExpressionPtr defaultVal;

        if (token::isTypeKeyword(fieldTok.type) && (peekTokenIs(token::IDENT) || peekTokenIs(token::COLON))) {
            typeAnnot = parseTypeAnnotation();
            if (peekTokenIs(token::COLON)) nextToken();
            if (!peekTokenIs(token::IDENT)) {
                reportAt(fieldTok, "write 'private int: field = value;'", "expected a field name after the type");
                consumeStatementEnd();
                continue;
            }
            nextToken();
            fieldTok = curToken_;
            fieldName = fieldTok.literal;
        } else if (peekTokenIs(token::COLON)) {
            nextToken(); // ':'
            nextToken(); // type start
            typeAnnot = parseTypeAnnotation();
        }

        if (peekTokenIs(token::ASSIGN)) {
            nextToken(); // '='
            nextToken(); // default value
            defaultVal = parseExpression(LOWEST);
        }

        auto field = std::make_shared<ast::StructField>();
        field->token = fieldTok;
        field->name = fieldName;
        field->typeAnnotation = typeAnnot;
        field->defaultValue = defaultVal;
        field->visibility = fieldVisibility;
        field->isConst = fieldIsConst;
        fields.push_back(field);

        consumeStatementEnd();
        if (isBraced) {
            skipBraceBlockTrivia();
        } else {
            skipNewlines();
        }
        if (curToken_.type == prevTok.type && curToken_.literal == prevTok.literal &&
            curToken_.line == prevTok.line && curToken_.column == prevTok.column) {
            nextToken();
            if (isBraced) {
                skipBraceBlockTrivia();
            } else {
                skipNewlines();
            }
        }
    }

    if (isBraced) {
        if (curTokenIs(token::RBRACE)) nextToken();
    } else if (curTokenIs(token::DEDENT)) {
        nextToken();
    }

    auto sd = std::make_shared<ast::StructDeclaration>();
    sd->token = structTok;
    sd->name = name;
    sd->fields = fields;
    sd->isPublic = isPublic;
    sd->decorators = decorators;
    return sd;
}

std::shared_ptr<ast::ClassDeclaration> Parser::parseClassDeclaration(bool isPublic, const std::vector<std::shared_ptr<ast::Decorator>>& decorators) {
    auto classTok = curToken_;
    nextToken(); // past 'class'

    if (!curTokenIs(token::IDENT)) {
        reportAt(curToken_, "provide a class name, e.g. 'class User:'",
                 "expected class name after 'class', got \"" + curToken_.literal + "\"");
        return nullptr;
    }

    std::string name = curToken_.literal;
    std::string parent;
    token::Token parentTok;

    if (peekTokenIs(token::EXTENDS)) {
        nextToken(); // 'extends'
        nextToken(); // base class name
        if (!curTokenIs(token::IDENT)) {
            reportAt(curToken_, "write 'class Child extends Parent:'", "expected a base class name after 'extends'");
        } else {
            parent = curToken_.literal;
            parentTok = curToken_;
        }
    }

    bool isBraced = false;
    skipPeekNewlines();
    if (peekTokenIs(token::LBRACE)) {
        nextToken();
        isBraced = true;
        nextToken();
    } else if (curTokenIs(token::LBRACE)) {
        isBraced = true;
        nextToken();
    } else {
        expectBlockBrace("class " + name);
        if (curTokenIs(token::COLON)) nextToken();
        skipNewlines();
        if (curTokenIs(token::INDENT)) nextToken();
    }

    auto classDecl = std::make_shared<ast::ClassDeclaration>();
    classDecl->token = classTok;
    classDecl->name = name;
    classDecl->parent = parent;
    classDecl->parentToken = parentTok;
    classDecl->isPublic = isPublic;
    classDecl->decorators = decorators;

    auto isEnd = [&] {
        if (isBraced) {
            return curTokenIs(token::RBRACE) || curTokenIs(token::EOF_TOKEN);
        }
        return curTokenIs(token::DEDENT) || curTokenIs(token::EOF_TOKEN);
    };

    while (!isEnd()) {
        auto prevTok = curToken_;
        if (isBraced) {
            skipBraceBlockTrivia();
        } else {
            skipNewlines();
        }
        if (isEnd()) break;

        std::string visibility;
        bool isConst = false;
        parseVarModifiers(visibility, isConst);

        bool memberIsPublic = (visibility == "public");
        if (curTokenIs(token::PUB)) {
            memberIsPublic = true;
            nextToken();
        }

        if (curTokenIs(token::STRUCT)) {
            auto s = parseStructDeclaration(memberIsPublic, {});
            if (s) classDecl->nestedStructs.push_back(s);
        } else if (curTokenIs(token::FUNC) || curTokenIs(token::TYPE_VOID)) {
            auto fn = parseFuncDeclaration(memberIsPublic, {});
            if (fn) {
                auto m = std::make_shared<ast::MethodDeclaration>();
                m->token = fn->token;
                m->visibility = visibility;
                m->func = fn;
                classDecl->methods.push_back(m);
            }
        } else if (curTokenIs(token::IDENT) || token::isTypeKeyword(curToken_.type)) {
            auto f = parseClassField(visibility, isConst);
            if (f) classDecl->fields.push_back(f);
        } else {
            nextToken();
        }

        if (isBraced) {
            skipBraceBlockTrivia();
        } else {
            skipNewlines();
        }
        if (curToken_.type == prevTok.type && curToken_.literal == prevTok.literal &&
            curToken_.line == prevTok.line && curToken_.column == prevTok.column) {
            nextToken();
            if (isBraced) {
                skipBraceBlockTrivia();
            } else {
                skipNewlines();
            }
        }
    }

    if (isBraced) {
        if (curTokenIs(token::RBRACE)) nextToken();
    } else if (curTokenIs(token::DEDENT)) {
        nextToken();
    }

    return classDecl;
}

std::shared_ptr<ast::EnumDeclaration> Parser::parseEnumDeclaration(bool isPublic, const std::vector<std::shared_ptr<ast::Decorator>>& decorators) {
    auto enumTok = curToken_;
    nextToken(); // past 'enum'

    if (!curTokenIs(token::IDENT)) {
        reportAt(curToken_, "provide an enum name, e.g. 'enum Status {'", "expected enum name");
        return nullptr;
    }
    std::string name = curToken_.literal;

    bool isBraced = false;
    skipPeekNewlines();
    if (peekTokenIs(token::LBRACE)) {
        nextToken();
        isBraced = true;
        nextToken();
    } else if (curTokenIs(token::LBRACE)) {
        isBraced = true;
        nextToken();
    } else {
        expectBlockBrace("enum " + name);
        if (curTokenIs(token::COLON)) nextToken();
        skipNewlines();
        if (curTokenIs(token::INDENT)) nextToken();
    }

    std::vector<std::shared_ptr<ast::EnumVariant>> variants;
    auto isEnd = [&] {
        if (isBraced) {
            return curTokenIs(token::RBRACE) || curTokenIs(token::EOF_TOKEN);
        }
        return curTokenIs(token::DEDENT) || curTokenIs(token::EOF_TOKEN);
    };

    while (!isEnd()) {
        if (isBraced) {
            skipBraceBlockTrivia();
        } else {
            skipNewlines();
        }
        if (isEnd()) break;

        if (!curTokenIs(token::IDENT)) {
            nextToken();
            continue;
        }

        auto varTok = curToken_;
        std::string varName = varTok.literal;
        ast::ExpressionPtr varVal;

        if (peekTokenIs(token::ASSIGN)) {
            nextToken(); // '='
            nextToken();
            varVal = parseExpression(LOWEST);
        }

        auto variant = std::make_shared<ast::EnumVariant>();
        variant->token = varTok;
        variant->name = varName;
        variant->value = varVal;
        variants.push_back(variant);

        if (peekTokenIs(token::COMMA) || peekTokenIs(token::SEMICOLON)) {
            nextToken();
        }
        nextToken();
    }

    if (isBraced) {
        if (curTokenIs(token::RBRACE)) nextToken();
    } else if (curTokenIs(token::DEDENT)) {
        nextToken();
    }

    auto ed = std::make_shared<ast::EnumDeclaration>();
    ed->token = enumTok;
    ed->name = name;
    ed->variants = variants;
    ed->isPublic = isPublic;
    ed->decorators = decorators;
    return ed;
}

std::shared_ptr<ast::InterfaceDeclaration> Parser::parseInterfaceDeclaration(bool isPublic, const std::vector<std::shared_ptr<ast::Decorator>>& decorators) {
    auto ifaceTok = curToken_;
    nextToken(); // past 'interface'/'trait'

    if (!curTokenIs(token::IDENT)) {
        reportAt(curToken_, "provide an interface name, e.g. 'interface Greeter {'", "expected interface name");
        return nullptr;
    }
    std::string name = curToken_.literal;

    bool isBraced = false;
    skipPeekNewlines();
    if (peekTokenIs(token::LBRACE)) {
        nextToken();
        isBraced = true;
        nextToken();
    } else if (curTokenIs(token::LBRACE)) {
        isBraced = true;
        nextToken();
    } else {
        expectBlockBrace("interface " + name);
        if (curTokenIs(token::COLON)) nextToken();
        skipNewlines();
        if (curTokenIs(token::INDENT)) nextToken();
    }

    std::vector<std::shared_ptr<ast::MethodSignature>> methods;
    auto isEnd = [&] {
        if (isBraced) {
            return curTokenIs(token::RBRACE) || curTokenIs(token::EOF_TOKEN);
        }
        return curTokenIs(token::DEDENT) || curTokenIs(token::EOF_TOKEN);
    };

    while (!isEnd()) {
        if (isBraced) {
            skipBraceBlockTrivia();
        } else {
            skipNewlines();
        }
        if (isEnd()) break;

        if (curTokenIs(token::PUB)) nextToken();

        if (curTokenIs(token::FUNC)) {
            auto fnTok = curToken_;
            nextToken();
            if (!curTokenIs(token::IDENT)) {
                nextToken();
                continue;
            }
            std::string mName = curToken_.literal;
            if (!expectPeek(token::LPAREN)) continue;
            auto params = parseFuncParams();
            std::shared_ptr<ast::TypeAnnotation> retType;
            if (peekTokenIs(token::ARROW)) {
                nextToken(); // '->'
                nextToken();
                retType = parseTypeAnnotation();
            }
            auto sig = std::make_shared<ast::MethodSignature>();
            sig->token = fnTok;
            sig->name = mName;
            sig->params = params;
            sig->returnType = retType;
            methods.push_back(sig);
            consumeStatementEnd();
        } else {
            nextToken();
        }
    }

    if (isBraced) {
        if (curTokenIs(token::RBRACE)) nextToken();
    } else if (curTokenIs(token::DEDENT)) {
        nextToken();
    }

    auto iface = std::make_shared<ast::InterfaceDeclaration>();
    iface->token = ifaceTok;
    iface->name = name;
    iface->methods = methods;
    iface->isPublic = isPublic;
    iface->decorators = decorators;
    return iface;
}

ast::ExpressionPtr Parser::parseAwaitExpression() {
    auto tok = curToken_; // 'await'
    nextToken();
    auto expr = parseExpression(PREFIX);
    auto awaitExp = std::make_shared<ast::AwaitExpression>();
    awaitExp->token = tok;
    awaitExp->expression = expr;
    return awaitExp;
}

std::shared_ptr<ast::StructField> Parser::parseClassField(const std::string& visibility, bool isConst) {
    auto fieldTok = curToken_;
    std::string fieldName = fieldTok.literal;

    std::shared_ptr<ast::TypeAnnotation> typeAnnot;
    ast::ExpressionPtr defaultVal;

    if (token::isTypeKeyword(fieldTok.type) && (peekTokenIs(token::IDENT) || peekTokenIs(token::COLON))) {
        typeAnnot = parseTypeAnnotation();
        if (peekTokenIs(token::COLON)) nextToken();
        if (!peekTokenIs(token::IDENT)) {
            reportAt(fieldTok, "write 'private int: field = value;'", "expected a field name after the type");
            consumeStatementEnd();
            return nullptr;
        }
        nextToken();
        fieldTok = curToken_;
        fieldName = fieldTok.literal;
    } else if (peekTokenIs(token::COLON)) {
        nextToken(); // ':'
        nextToken(); // type start
        typeAnnot = parseTypeAnnotation();
    }

    if (peekTokenIs(token::ASSIGN)) {
        nextToken(); // '='
        nextToken(); // default value
        defaultVal = parseExpression(LOWEST);
    }

    auto field = std::make_shared<ast::StructField>();
    field->token = fieldTok;
    field->name = fieldName;
    field->typeAnnotation = typeAnnot;
    field->defaultValue = defaultVal;
    field->visibility = visibility;
    field->isConst = isConst;
    consumeStatementEnd();
    return field;
}

std::shared_ptr<ast::BlockStatement> Parser::parseBlockStatement() {
    if (peekTokenIs(token::LBRACE)) nextToken();
    if (curTokenIs(token::LBRACE)) return parseBraceBlockStatement();

    auto block = std::make_shared<ast::BlockStatement>();
    block->token = curToken_;

    if (peekTokenIs(token::COLON)) nextToken();
    if (curTokenIs(token::COLON)) nextToken();

    if (curTokenIs(token::INDENT) || curTokenIs(token::NEWLINE) ||
        curTokenIs(token::DEDENT) || curTokenIs(token::EOF_TOKEN) || peekTokenIs(token::NEWLINE)) {
        if (!curTokenIs(token::INDENT) && peekTokenIs(token::NEWLINE)) nextToken();
        skipNewlines();

        if (curTokenIs(token::INDENT)) {
            nextToken();

            while (!curTokenIs(token::DEDENT) && !curTokenIs(token::EOF_TOKEN)) {
                auto prevTok = curToken_;
                skipNewlines();
                if (curTokenIs(token::DEDENT) || curTokenIs(token::EOF_TOKEN)) break;

                auto stmt = parseStatement();
                if (stmt) {
                    block->statements.push_back(stmt);
                } else {
                    nextToken();
                }

                skipNewlines();
                if (curToken_.type == prevTok.type && curToken_.literal == prevTok.literal &&
                    curToken_.line == prevTok.line && curToken_.column == prevTok.column) {
                    nextToken();
                    skipNewlines();
                }
            }

            if (curTokenIs(token::DEDENT)) nextToken();
            return block;
        }

        switch (curToken_.type) {
        case token::DEDENT:
        case token::EOF_TOKEN:
        case token::ELIF:
        case token::ELSE:
        case token::CASE:
            return block;
        default:
            break;
        }

        auto tok = block->token;
        errors_.push_back("line " + std::to_string(tok.line) + ", col " + std::to_string(tok.column) + ": expected an indented block");
        auto err = std::make_shared<diag::SyntaxError>();
        err->line = tok.line;
        err->column = tok.column;
        err->message = "expected an indented block";
        err->token = tok.literal;
        err->length = static_cast<int>(tok.literal.length());
        err->suggestion = "indent the block body by 4 spaces or 1 tab";
        syntaxErrors_.push_back(err);
        return block;
    }

    auto stmt = parseStatement();
    if (stmt) block->statements.push_back(stmt);
    return block;
}

std::shared_ptr<ast::BlockStatement> Parser::parseBraceBlockStatement() {
    auto block = std::make_shared<ast::BlockStatement>();
    block->token = curToken_; // '{'
    nextToken(); // past '{'

    while (!curTokenIs(token::RBRACE) && !curTokenIs(token::EOF_TOKEN)) {
        skipBraceBlockTrivia();
        if (curTokenIs(token::RBRACE) || curTokenIs(token::EOF_TOKEN)) break;

        auto prevTok = curToken_;
        auto stmt = parseStatement();
        if (stmt) {
            block->statements.push_back(stmt);
        } else {
            nextToken();
        }

        skipBraceBlockTrivia();
        if (curToken_.type == prevTok.type && curToken_.literal == prevTok.literal &&
            curToken_.line == prevTok.line && curToken_.column == prevTok.column) {
            nextToken();
            skipBraceBlockTrivia();
        }
    }

    if (curTokenIs(token::RBRACE)) {
        nextToken();
    } else {
        reportAt(curToken_, "close the function body with '}'", "expected '}' at end of function body");
    }

    return block;
}

void Parser::skipBraceBlockTrivia() {
    while (curTokenIs(token::NEWLINE) || curTokenIs(token::INDENT) ||
           curTokenIs(token::DEDENT) || curTokenIs(token::SEMICOLON)) {
        nextToken();
    }
}

std::shared_ptr<ast::IfStatement> Parser::parseIfStatement() {
    auto ifTok = curToken_;
    if (peekTokenIs(token::COLON) || peekTokenIs(token::LBRACE)) {
        reportAt(peekToken_, "provide a boolean condition, e.g. 'if x > 0 {'", "missing condition in 'if' statement");
        nextToken(); // ':'
        auto consequence = parseBlockStatement();
        auto stmt = std::make_shared<ast::IfStatement>();
        stmt->token = ifTok;
        auto boolLit = std::make_shared<ast::BooleanLiteral>();
        boolLit->token = ifTok;
        boolLit->value = false;
        stmt->condition = boolLit;
        stmt->consequence = consequence;
        return stmt;
    }
    nextToken(); // past 'if'

    auto cond = parseExpression(LOWEST);
    expectBlockBrace("if condition");
    auto consequence = parseBlockStatement();

    auto stmt = std::make_shared<ast::IfStatement>();
    stmt->token = ifTok;
    stmt->condition = cond;
    stmt->consequence = consequence;

    skipNewlines();

    while (curTokenIs(token::ELIF)) {
        auto elifTok = curToken_;
        if (peekTokenIs(token::COLON) || peekTokenIs(token::LBRACE)) {
            reportAt(peekToken_, "provide a boolean condition, e.g. 'elif x > 0 {'", "missing condition in 'elif' statement");
            nextToken();
            auto elifBody = parseBlockStatement();
            auto clause = std::make_shared<ast::ElifClause>();
            clause->token = elifTok;
            auto boolLit = std::make_shared<ast::BooleanLiteral>();
            boolLit->token = elifTok;
            boolLit->value = false;
            clause->condition = boolLit;
            clause->body = elifBody;
            stmt->alternatives.push_back(clause);
            skipNewlines();
            continue;
        }
        nextToken(); // past 'elif'

        auto elifCond = parseExpression(LOWEST);
        expectBlockBrace("elif condition");
        auto elifBody = parseBlockStatement();

        auto clause = std::make_shared<ast::ElifClause>();
        clause->token = elifTok;
        clause->condition = elifCond;
        clause->body = elifBody;
        stmt->alternatives.push_back(clause);

        skipNewlines();
    }

    if (curTokenIs(token::ELSE)) {
        nextToken(); // past 'else'
        expectBlockBrace("else");
        stmt->elseBlock = parseBlockStatement();
    }

    return stmt;
}

std::shared_ptr<ast::WhileStatement> Parser::parseWhileStatement() {
    auto tok = curToken_;
    if (peekTokenIs(token::COLON) || peekTokenIs(token::LBRACE)) {
        reportAt(peekToken_, "provide a loop condition, e.g. 'while running {'", "missing condition in 'while' statement");
        nextToken();
        auto body = parseBlockStatement();
        auto stmt = std::make_shared<ast::WhileStatement>();
        stmt->token = tok;
        auto boolLit = std::make_shared<ast::BooleanLiteral>();
        boolLit->token = tok;
        boolLit->value = false;
        stmt->condition = boolLit;
        stmt->body = body;
        return stmt;
    }
    nextToken(); // condition start

    auto cond = parseExpression(LOWEST);
    expectBlockBrace("while condition");
    auto body = parseBlockStatement();

    auto stmt = std::make_shared<ast::WhileStatement>();
    stmt->token = tok;
    stmt->condition = cond;
    stmt->body = body;
    return stmt;
}

std::shared_ptr<ast::ForStatement> Parser::parseForStatement() {
    auto forTok = curToken_;
    nextToken(); // variable
    if (!curTokenIs(token::IDENT)) {
        reportAt(curToken_, "specify a variable name: 'for item in collection:'", "expected loop variable name after 'for'");
        return nullptr;
    }

    std::string varName = curToken_.literal;
    if (peekTokenIs(token::COMMA)) {
        nextToken(); // ','
        varName += ", ";
        nextToken(); // next ident
        varName += curToken_.literal;
    }

    if (!expectPeek(token::IN)) return nullptr;

    nextToken(); // iterable start
    auto iterable = parseExpression(LOWEST);
    expectBlockBrace("for expression");
    auto body = parseBlockStatement();

    auto stmt = std::make_shared<ast::ForStatement>();
    stmt->token = forTok;
    stmt->variable = varName;
    stmt->iterable = iterable;
    stmt->body = body;
    return stmt;
}

std::shared_ptr<ast::ForEachStatement> Parser::parseForEachStatement() {
    auto forEachTok = curToken_;
    nextToken(); // variable
    if (!curTokenIs(token::IDENT)) {
        reportAt(curToken_, "specify a variable name: 'foreach item in collection:'", "expected loop variable name after 'foreach'");
        return nullptr;
    }

    std::string varName = curToken_.literal;
    if (peekTokenIs(token::COMMA)) {
        nextToken();
        varName += ", ";
        nextToken();
        varName += curToken_.literal;
    }

    if (!expectPeek(token::IN)) return nullptr;

    nextToken();
    auto iterable = parseExpression(LOWEST);
    expectBlockBrace("foreach expression");
    auto body = parseBlockStatement();

    auto stmt = std::make_shared<ast::ForEachStatement>();
    stmt->token = forEachTok;
    stmt->variable = varName;
    stmt->iterable = iterable;
    stmt->body = body;
    return stmt;
}

std::shared_ptr<ast::LoopStatement> Parser::parseLoopStatement() {
    auto tok = curToken_;
    expectBlockBrace("loop");
    auto body = parseBlockStatement();

    auto stmt = std::make_shared<ast::LoopStatement>();
    stmt->token = tok;
    stmt->body = body;
    return stmt;
}

std::shared_ptr<ast::ReturnStatement> Parser::parseReturnStatement() {
    auto tok = curToken_;
    auto stmt = std::make_shared<ast::ReturnStatement>();
    stmt->token = tok;

    if (peekTokenIs(token::SEMICOLON) || peekTokenIs(token::NEWLINE) ||
        peekTokenIs(token::EOF_TOKEN) || peekTokenIs(token::DEDENT) || peekTokenIs(token::RBRACE)) {
        consumeStatementEnd();
        return stmt;
    }

    nextToken();
    stmt->value = parseExpression(LOWEST);
    consumeStatementEnd();
    return stmt;
}

std::shared_ptr<ast::BreakStatement> Parser::parseBreakStatement() {
    auto tok = curToken_;
    consumeStatementEnd();
    auto stmt = std::make_shared<ast::BreakStatement>();
    stmt->token = tok;
    return stmt;
}

std::shared_ptr<ast::ContinueStatement> Parser::parseContinueStatement() {
    auto tok = curToken_;
    consumeStatementEnd();
    auto stmt = std::make_shared<ast::ContinueStatement>();
    stmt->token = tok;
    return stmt;
}

std::shared_ptr<ast::UseStatement> Parser::parseUseStatement() {
    auto useTok = curToken_;
    nextToken(); // past 'use'

    std::string modulePath;
    std::vector<std::string> symbols;
    std::string alias;

    if (curTokenIs(token::STRING)) {
        modulePath = curToken_.literal;
    } else {
        modulePath = curToken_.literal;
        while (peekTokenIs(token::DOT) || peekTokenIs(token::SLASH)) {
            nextToken();
            modulePath += curToken_.literal;
            if (peekTokenIs(token::IDENT) || token::isTypeKeyword(peekToken_.type) || token::isKeyword(peekToken_.type)) {
                nextToken();
                modulePath += curToken_.literal;
            }
        }
    }

    if (peekTokenIs(token::LPAREN)) {
        nextToken(); // '('
        skipPeekNewlines();
        while (!peekTokenIs(token::RPAREN) && !peekTokenIs(token::EOF_TOKEN)) {
            nextToken();
            skipNewlines();
            symbols.push_back(curToken_.literal);
            skipPeekNewlines();
            if (peekTokenIs(token::COMMA)) {
                nextToken();
                skipPeekNewlines();
            } else {
                break;
            }
        }
        if (!expectPeek(token::RPAREN)) return nullptr;
    }

    if (peekTokenIs(token::AS)) {
        nextToken(); // 'as'
        nextToken(); // alias
        alias = curToken_.literal;
    }

    consumeStatementEnd();

    auto stmt = std::make_shared<ast::UseStatement>();
    stmt->token = useTok;
    stmt->path = modulePath;
    stmt->symbols = symbols;
    stmt->alias = alias;
    return stmt;
}

std::shared_ptr<ast::WatchStatement> Parser::parseWatchStatement() {
    auto watchTok = curToken_;
    nextToken(); // target start
    auto target = parseExpression(LOWEST);
    if (peekTokenIs(token::COLON)) nextToken();
    auto body = parseBlockStatement();

    auto stmt = std::make_shared<ast::WatchStatement>();
    stmt->token = watchTok;
    stmt->target = std::dynamic_pointer_cast<ast::Identifier>(target);
    stmt->handler = body;
    return stmt;
}

std::shared_ptr<ast::TestStatement> Parser::parseTestStatement() {
    auto testTok = curToken_;
    nextToken();
    std::string name = curToken_.literal;
    if (peekTokenIs(token::COLON)) nextToken();
    auto body = parseBlockStatement();

    auto stmt = std::make_shared<ast::TestStatement>();
    stmt->token = testTok;
    stmt->name = name;
    stmt->body = body;
    return stmt;
}

std::shared_ptr<ast::MockStatement> Parser::parseMockStatement() {
    auto mockTok = curToken_;
    ast::ExpressionPtr target;
    ast::ExpressionPtr replacement;

    if (peekTokenIs(token::LPAREN)) {
        nextToken(); // '('
        nextToken();
        target = parseExpression(LOWEST);
        if (!expectPeek(token::COMMA)) return nullptr;
        nextToken();
        replacement = parseExpression(LOWEST);
        if (!expectPeek(token::RPAREN)) return nullptr;
    } else {
        nextToken();
        target = parseExpression(ASSIGN);
        if (peekTokenIs(token::ASSIGN) || peekTokenIs(token::COMMA)) {
            nextToken();
            nextToken();
            replacement = parseExpression(LOWEST);
        }
    }

    consumeStatementEnd();

    auto stmt = std::make_shared<ast::MockStatement>();
    stmt->token = mockTok;
    stmt->target = std::dynamic_pointer_cast<ast::Identifier>(target);
    stmt->body = nullptr;
    return stmt;
}

ast::StatementPtr Parser::parseBaStatement() {
    auto baTok = curToken_;

    if (peekTokenIs(token::LPAREN)) {
        nextToken();
        nextToken();
        skipNewlines();
        while (!curTokenIs(token::RPAREN) && !curTokenIs(token::EOF_TOKEN)) {
            skipNewlines();
            if (curTokenIs(token::RPAREN)) break;
            if (peekTokenIs(token::ASSIGN)) {
                nextToken();
                nextToken();
                parseExpression(LOWEST);
            }
            skipPeekNewlines();
            if (peekTokenIs(token::COMMA)) {
                nextToken();
                skipPeekNewlines();
                nextToken();
            } else {
                break;
            }
        }
        skipPeekNewlines();
        if (!curTokenIs(token::RPAREN)) {
            expectPeek(token::RPAREN);
        }
    } else {
        while (!peekTokenIs(token::FOR) && !peekTokenIs(token::LBRACE) &&
               !peekTokenIs(token::COLON) && !peekTokenIs(token::NEWLINE) && !peekTokenIs(token::EOF_TOKEN)) {
            nextToken();
            if (peekTokenIs(token::ASSIGN)) {
                nextToken();
                nextToken();
                parseExpression(LOWEST);
            }
            if (peekTokenIs(token::COMMA)) nextToken();
        }
    }

    if (peekTokenIs(token::FOR)) {
        nextToken(); // 'for'
        nextToken(); // variable
        std::string varName = curToken_.literal;
        if (peekTokenIs(token::COMMA)) {
            nextToken();
            varName += ", ";
            nextToken();
            varName += curToken_.literal;
        }

        if (!expectPeek(token::IN)) return nullptr;

        nextToken();
        auto iterable = parseExpression(LOWEST);
        if (peekTokenIs(token::COLON)) nextToken();
        auto body = parseBlockStatement();

        auto baFor = std::make_shared<ast::BaForStatement>();
        baFor->token = baTok;
        baFor->variable = varName;
        baFor->iterable = iterable;
        baFor->body = body;
        return baFor;
    }

    if (peekTokenIs(token::COLON)) nextToken();
    auto body = parseBlockStatement();

    auto stmt = std::make_shared<ast::BaStatement>();
    stmt->token = baTok;
    stmt->body = body;
    return stmt;
}

std::shared_ptr<ast::ThreadsStatement> Parser::parseThreadsStatement() {
    auto tok = curToken_;
    nextToken(); // past 'threads'

    if (curTokenIs(token::LPAREN)) {
        int depth = 0;
        while (!curTokenIs(token::EOF_TOKEN)) {
            if (curTokenIs(token::LPAREN)) {
                depth++;
            } else if (curTokenIs(token::RPAREN)) {
                depth--;
                if (depth == 0) {
                    nextToken();
                    break;
                }
            }
            nextToken();
        }
    } else {
        while (!peekTokenIs(token::LBRACE) && !peekTokenIs(token::COLON) &&
               !peekTokenIs(token::NEWLINE) && !peekTokenIs(token::EOF_TOKEN)) {
            nextToken();
        }
    }

    skipNewlines();
    auto body = parseBlockStatement();
    auto stmt = std::make_shared<ast::ThreadsStatement>();
    stmt->token = tok;
    stmt->body = body;
    return stmt;
}

std::shared_ptr<ast::TxStatement> Parser::parseTxStatement() {
    auto txTok = curToken_;

    if (peekTokenIs(token::LPAREN)) {
        nextToken(); // '('
        nextToken();
        parseExpression(LOWEST);
        if (peekTokenIs(token::COMMA)) {
            nextToken();
            nextToken();
            parseExpression(LOWEST);
        }
        if (!expectPeek(token::RPAREN)) return nullptr;
    } else if (!peekTokenIs(token::LBRACE) && !peekTokenIs(token::COLON) && !peekTokenIs(token::NEWLINE)) {
        nextToken();
        parseExpression(LOWEST);
    }

    if (peekTokenIs(token::COLON)) nextToken();
    auto body = parseBlockStatement();

    auto stmt = std::make_shared<ast::TxStatement>();
    stmt->token = txTok;
    stmt->body = body;
    return stmt;
}

std::shared_ptr<ast::WaitStatement> Parser::parseWaitStatement() {
    auto tok = curToken_;
    consumeStatementEnd();
    auto stmt = std::make_shared<ast::WaitStatement>();
    stmt->token = tok;
    return stmt;
}

std::shared_ptr<ast::AssertStatement> Parser::parseAssertStatement() {
    auto tok = curToken_;
    nextToken(); // past 'assert'

    if (curTokenIs(token::SEMICOLON)) {
        reportAt(tok, "provide an expression to assert, e.g. 'assert x > 0;'", "missing condition in 'assert' statement");
        consumeStatementEnd();
        auto stmt = std::make_shared<ast::AssertStatement>();
        stmt->token = tok;
        return stmt;
    }

    ast::ExpressionPtr expr;
    if (curTokenIs(token::LPAREN)) {
        nextToken();
        expr = parseExpression(LOWEST);
        expectPeek(token::RPAREN);
    } else {
        expr = parseExpression(LOWEST);
    }

    consumeStatementEnd();

    auto stmt = std::make_shared<ast::AssertStatement>();
    stmt->token = tok;
    stmt->condition = expr;
    return stmt;
}

std::shared_ptr<ast::MatchStatement> Parser::parseMatchStatement() {
    auto matchTok = curToken_;
    nextToken(); // past 'match'

    if (curTokenIs(token::COLON) || curTokenIs(token::LBRACE)) {
        reportAt(curToken_, "provide a match expression, e.g. 'match status {'", "missing expression in 'match' statement");
    }

    auto subject = parseExpression(LOWEST);

    bool isBraced = false;
    skipPeekNewlines();
    if (peekTokenIs(token::LBRACE)) {
        nextToken();
        isBraced = true;
        nextToken();
    } else if (curTokenIs(token::LBRACE)) {
        isBraced = true;
        nextToken();
    } else {
        if (peekTokenIs(token::COLON)) nextToken();
        if (curTokenIs(token::COLON)) nextToken();
        skipNewlines();
        if (curTokenIs(token::INDENT)) nextToken();
    }

    skipNewlines();

    std::vector<std::shared_ptr<ast::MatchCase>> cases;
    std::shared_ptr<ast::BlockStatement> defaultBlock;

    auto isEnd = [&] {
        if (isBraced) {
            return curTokenIs(token::RBRACE) || curTokenIs(token::EOF_TOKEN);
        }
        return curTokenIs(token::DEDENT) || curTokenIs(token::EOF_TOKEN);
    };

    while (!isEnd()) {
        if (isBraced) {
            skipBraceBlockTrivia();
        } else {
            skipNewlines();
        }
        if (isEnd()) break;

        if (curTokenIs(token::CASE)) {
            auto caseTok = curToken_;
            nextToken();

            if (curTokenIs(token::COLON) || curTokenIs(token::LBRACE)) {
                reportAt(curToken_, "provide a pattern to match, e.g. 'case 200 {'", "missing pattern in 'case' clause");
            }

            auto pattern = parseExpression(LOWEST);
            if (peekTokenIs(token::COLON)) nextToken();
            auto caseBody = parseBlockStatement();

            auto mc = std::make_shared<ast::MatchCase>();
            mc->token = caseTok;
            mc->pattern = pattern;
            mc->body = caseBody;
            cases.push_back(mc);
        } else if (curTokenIs(token::ELSE)) {
            nextToken();
            if (peekTokenIs(token::COLON)) nextToken();
            defaultBlock = parseBlockStatement();
        } else {
            nextToken();
        }

        if (isBraced) {
            skipBraceBlockTrivia();
        } else {
            skipNewlines();
        }
    }

    if (isBraced) {
        if (curTokenIs(token::RBRACE)) nextToken();
    } else if (curTokenIs(token::DEDENT)) {
        nextToken();
    }

    auto stmt = std::make_shared<ast::MatchStatement>();
    stmt->token = matchTok;
    stmt->subject = subject;
    stmt->cases = cases;
    stmt->defaultCase = defaultBlock;
    return stmt;
}

std::shared_ptr<ast::ExpressionStatement> Parser::parseExpressionStatement() {
    auto stmt = std::make_shared<ast::ExpressionStatement>();
    stmt->token = curToken_;
    stmt->expression = parseExpression(LOWEST);
    if (!stmt->expression) {
        while (!curTokenIs(token::SEMICOLON) && !curTokenIs(token::NEWLINE) &&
               !curTokenIs(token::EOF_TOKEN) && !curTokenIs(token::DEDENT)) {
            nextToken();
        }
        if (curTokenIs(token::SEMICOLON)) nextToken();
        return nullptr;
    }
    consumeStatementEnd();
    return stmt;
}

std::shared_ptr<ast::TypeAnnotation> Parser::parseTypeAnnotation() {
    return parseTypeAnnotationWithUnion(true);
}

std::shared_ptr<ast::TypeAnnotation> Parser::parseTypeAnnotationWithUnion(bool allowUnion) {
    auto tok = curToken_;
    std::vector<std::string> types;

    for (;;) {
        std::string typeStr = curToken_.literal;

        while (peekTokenIs(token::DOT)) {
            nextToken(); // '.'
            typeStr += ".";
            if (peekTokenIs(token::IDENT) || token::isTypeKeyword(peekToken_.type)) {
                nextToken();
                typeStr += curToken_.literal;
            }
        }

        if (peekTokenIs(token::LBRACKET)) {
            nextToken(); // '['
            typeStr += "[";
            nextToken();
            while (!curTokenIs(token::RBRACKET) && !curTokenIs(token::EOF_TOKEN)) {
                typeStr += curToken_.literal;
                if (peekTokenIs(token::COMMA)) {
                    nextToken();
                    typeStr += ", ";
                }
                nextToken();
            }
            if (curTokenIs(token::RBRACKET)) {
                typeStr += "]";
            }
        }

        types.push_back(typeStr);

        if (allowUnion && peekTokenIs(token::PIPE_TYPE)) {
            nextToken(); // '|'
            nextToken(); // next type
        } else {
            break;
        }
    }

    auto annot = std::make_shared<ast::TypeAnnotation>();
    annot->token = tok;
    annot->types = types;
    return annot;
}

ast::ExpressionPtr Parser::parseExpression(int precedence) {
    skipNewlines();

    auto it = prefixParseFns_.find(curToken_.type);
    if (it == prefixParseFns_.end()) {
        noPrefixParseFnError(curToken_.type);
        return nullptr;
    }
    auto leftExp = it->second();

    while (!peekTokenIs(token::NEWLINE) && !peekTokenIs(token::EOF_TOKEN) && precedence < peekPrecedence()) {
        auto infixIt = infixParseFns_.find(peekToken_.type);
        if (infixIt == infixParseFns_.end()) return leftExp;

        nextToken();
        leftExp = infixIt->second(leftExp);
    }

    return leftExp;
}

ast::ExpressionPtr Parser::parseIdentifier() {
    if (curToken_.literal == "console" && peekTokenIs(token::LPAREN)) {
        auto tok = curToken_;
        nextToken(); // '('
        nextToken(); // prompt start
        ast::ExpressionPtr prompt;
        if (!curTokenIs(token::RPAREN)) {
            prompt = parseExpression(LOWEST);
            expectPeek(token::RPAREN);
        }
        auto cl = std::make_shared<ast::ConsoleLiteral>();
        cl->token = tok;
        cl->prompt = prompt;
        return cl;
    }
    auto id = std::make_shared<ast::Identifier>();
    id->token = curToken_;
    id->value = curToken_.literal;
    return id;
}

ast::ExpressionPtr Parser::parseTypeIdentifier() {
    auto id = std::make_shared<ast::Identifier>();
    id->token = curToken_;
    id->value = curToken_.literal;
    return id;
}

ast::ExpressionPtr Parser::parseIntegerLiteral() {
    auto lit = std::make_shared<ast::IntegerLiteral>();
    lit->token = curToken_;

    std::string raw = curToken_.literal;
    std::string cleaned = raw;
    const char* suffixes[] = {"ms", "s", "m", "h", "KB", "MB", "GB"};
    for (const char* suffix : suffixes) {
        size_t slen = std::string_view(suffix).length();
        if (raw.length() >= slen && raw.compare(raw.length() - slen, slen, suffix) == 0) {
            cleaned = raw.substr(0, raw.length() - slen);
            break;
        }
    }

    try {
        size_t idx = 0;
        int base = 10;
        if (cleaned.rfind("0x", 0) == 0 || cleaned.rfind("0X", 0) == 0) base = 16;
        else if (cleaned.rfind("0b", 0) == 0 || cleaned.rfind("0B", 0) == 0) {
            cleaned = cleaned.substr(2);
            base = 2;
        } else if (cleaned.rfind("0o", 0) == 0 || cleaned.rfind("0O", 0) == 0) {
            cleaned = cleaned.substr(2);
            base = 8;
        }
        lit->value = std::stoll(cleaned, &idx, base);
    } catch (...) {
        try {
            double fVal = std::stod(cleaned);
            lit->value = static_cast<int64_t>(fVal);
        } catch (...) {
            errors_.push_back("line " + std::to_string(curToken_.line) + ", col " + std::to_string(curToken_.column) +
                              ": could not parse \"" + curToken_.literal + "\" as integer");
            return nullptr;
        }
    }

    return lit;
}

ast::ExpressionPtr Parser::parseFloatLiteral() {
    auto lit = std::make_shared<ast::FloatLiteral>();
    lit->token = curToken_;

    std::string raw = curToken_.literal;
    std::string cleaned = raw;
    const char* suffixes[] = {"ms", "s", "m", "h", "KB", "MB", "GB"};
    for (const char* suffix : suffixes) {
        size_t slen = std::string_view(suffix).length();
        if (raw.length() >= slen && raw.compare(raw.length() - slen, slen, suffix) == 0) {
            cleaned = raw.substr(0, raw.length() - slen);
            break;
        }
    }

    try {
        lit->value = std::stod(cleaned);
    } catch (...) {
        errors_.push_back("line " + std::to_string(curToken_.line) + ", col " + std::to_string(curToken_.column) +
                          ": could not parse \"" + curToken_.literal + "\" as float");
        return nullptr;
    }

    return lit;
}

ast::ExpressionPtr Parser::parseStringLiteral() {
    auto lit = std::make_shared<ast::StringLiteral>();
    lit->token = curToken_;
    lit->value = curToken_.literal;
    return lit;
}

ast::ExpressionPtr Parser::parseFStringLiteral() {
    auto tok = curToken_;
    const std::string& s = tok.literal;
    std::vector<FStringPart> parts;
    std::string lit;
    size_t i = 0;

    while (i < s.length()) {
        if (s[i] == '{') {
            if (i + 1 < s.length() && s[i + 1] == '{') {
                lit.push_back('{');
                i += 2;
                continue;
            }
            if (!lit.empty()) {
                parts.push_back({false, lit});
                lit.clear();
            }
            i++; // skip '{'
            size_t start = i;
            int depth = 1;
            while (i < s.length() && depth > 0) {
                char ch = s[i];
                if (ch == '"' || ch == '\'' || ch == '`') {
                    char q = ch;
                    i++;
                    while (i < s.length() && s[i] != q) {
                        if (s[i] == '\\' && i + 1 < s.length()) i += 2;
                        else i++;
                    }
                    if (i < s.length()) i++;
                    continue;
                }
                if (ch == '{') depth++;
                else if (ch == '}') {
                    depth--;
                    if (depth == 0) break;
                }
                i++;
            }
            if (depth != 0) {
                reportAt(tok, "f-string", "unclosed '{' in f-string");
                return nullptr;
            }
            std::string exprStr = s.substr(start, i - start);
            i++; // skip '}'
            parts.push_back({true, trimSpace(exprStr)});
            continue;
        }
        if (s[i] == '}') {
            if (i + 1 < s.length() && s[i + 1] == '}') {
                lit.push_back('}');
                i += 2;
                continue;
            }
            reportAt(tok, "f-string", "single '}' encountered in f-string; use '}}' for literal brace");
            return nullptr;
        }
        lit.push_back(s[i]);
        i++;
    }
    if (!lit.empty()) {
        parts.push_back({false, lit});
    }

    if (parts.empty()) {
        auto strLit = std::make_shared<ast::StringLiteral>();
        strLit->token = tok;
        strLit->value = "";
        return strLit;
    }

    std::vector<ast::ExpressionPtr> exprs;
    for (const auto& part : parts) {
        if (!part.isExpr) {
            auto strLit = std::make_shared<ast::StringLiteral>();
            strLit->token = tok;
            strLit->value = part.text;
            exprs.push_back(strLit);
        } else {
            if (part.text.empty()) {
                reportAt(tok, "f-string", "empty expression in f-string '{}'");
                return nullptr;
            }
            auto subLex = std::make_unique<lexer::Lexer>(part.text);
            Parser subP(std::move(subLex));
            auto expr = subP.parseExpression(LOWEST);
            if (!subP.errors().empty()) {
                reportAt(tok, "f-string", "invalid expression in f-string '{" + part.text + "}'");
                return nullptr;
            }
            auto id = std::make_shared<ast::Identifier>();
            id->token = token::Token{token::IDENT, "string", tok.line, tok.column};
            id->value = "string";
            auto call = std::make_shared<ast::CallExpression>();
            call->token = tok;
            call->function = id;
            call->arguments = {expr};
            exprs.push_back(call);
        }
    }

    if (exprs.size() == 1) return exprs[0];

    auto left = exprs[0];
    for (size_t idx = 1; idx < exprs.size(); ++idx) {
        auto infix = std::make_shared<ast::InfixExpression>();
        infix->token = token::Token{token::PLUS, "+", tok.line, tok.column};
        infix->left = left;
        infix->op = "+";
        infix->right = exprs[idx];
        left = infix;
    }
    return left;
}

ast::ExpressionPtr Parser::parseBooleanLiteral() {
    auto lit = std::make_shared<ast::BooleanLiteral>();
    lit->token = curToken_;
    lit->value = curTokenIs(token::TRUE);
    return lit;
}

ast::ExpressionPtr Parser::parseNullLiteral() {
    auto lit = std::make_shared<ast::NullLiteral>();
    lit->token = curToken_;
    return lit;
}

ast::ExpressionPtr Parser::parsePrefixExpression() {
    auto tok = curToken_;
    std::string op = curToken_.literal;
    nextToken();
    auto right = parseExpression(PREFIX);

    auto exp = std::make_shared<ast::PrefixExpression>();
    exp->token = tok;
    exp->op = op;
    exp->right = right;
    return exp;
}

ast::ExpressionPtr Parser::parseGroupedExpression() {
    auto openTok = curToken_;
    nextToken();
    skipNewlines();

    auto exp = parseExpression(LOWEST);
    skipNewlines();

    if (!expectPeek(token::RPAREN)) {
        if (!syntaxErrors_.empty()) {
            syntaxErrors_.back()->suggestion = "unclosed '(' from line " + std::to_string(openTok.line) +
                                                ", col " + std::to_string(openTok.column) + "; add a closing ')'";
        }
        return nullptr;
    }

    return exp;
}

ast::ExpressionPtr Parser::parseListLiteral() {
    auto list = std::make_shared<ast::ListLiteral>();
    list->token = curToken_;
    list->elements = parseExpressionList(token::RBRACKET);
    return list;
}

ast::ExpressionPtr Parser::parseMapLiteral() {
    auto mapLit = std::make_shared<ast::MapLiteral>();
    mapLit->token = curToken_;

    nextToken(); // past '{'
    skipNewlines();

    bool closed = false;
    if (curTokenIs(token::RBRACE)) closed = true;

    while (!curTokenIs(token::RBRACE) && !curTokenIs(token::EOF_TOKEN)) {
        skipNewlines();
        if (curTokenIs(token::RBRACE)) {
            closed = true;
            break;
        }

        auto key = parseExpression(LOWEST);
        if (!key) return nullptr;

        if (!expectPeek(token::COLON)) return nullptr;
        auto colonTok = curToken_;

        nextToken(); // past ':'
        skipNewlines();

        auto val = parseExpression(LOWEST);
        if (!val) return nullptr;

        auto pair = std::make_shared<ast::MapPair>();
        pair->token = colonTok;
        pair->key = key;
        pair->value = val;
        mapLit->pairs.push_back(pair);

        skipPeekNewlines();
        if (peekTokenIs(token::COMMA)) {
            nextToken();
            skipPeekNewlines();
            if (peekTokenIs(token::RBRACE)) {
                nextToken(); // '}'
                closed = true;
                break;
            }
            nextToken(); // next key
        } else {
            break;
        }
    }

    if (!closed) {
        skipPeekNewlines();
        if (!expectPeek(token::RBRACE)) {
            if (!syntaxErrors_.empty()) {
                syntaxErrors_.back()->suggestion = "unclosed '{' from line " + std::to_string(mapLit->token.line) +
                                                    ", col " + std::to_string(mapLit->token.column) + "; add a closing '}'";
            }
        }
    }

    return mapLit;
}

ast::ExpressionPtr Parser::parseErrExpression() {
    auto tok = curToken_;
    if (!peekTokenIs(token::LPAREN)) {
        auto errExp = std::make_shared<ast::ErrExpression>();
        errExp->token = tok;
        return errExp;
    }

    nextToken(); // '('
    nextToken(); // msg start

    ast::ExpressionPtr msg;
    ast::ExpressionPtr code;

    if (!curTokenIs(token::RPAREN)) {
        msg = parseExpression(LOWEST);
        if (peekTokenIs(token::COMMA)) {
            nextToken(); // ','
            nextToken(); // code
            code = parseExpression(LOWEST);
            if (auto as = std::dynamic_pointer_cast<ast::AssignExpression>(code)) {
                if (auto id = std::dynamic_pointer_cast<ast::Identifier>(as->target)) {
                    if (id->value == "code") {
                        code = as->value;
                    }
                }
            }
        }
        expectPeek(token::RPAREN);
    }

    auto errExp = std::make_shared<ast::ErrExpression>();
    errExp->token = tok;
    errExp->message = msg;
    errExp->code = code;
    return errExp;
}

ast::ExpressionPtr Parser::parseLambdaExpression() {
    auto tok = curToken_; // '|'
    nextToken();

    std::vector<std::shared_ptr<ast::FuncParam>> params;
    while (!curTokenIs(token::PIPE_TYPE) && !curTokenIs(token::EOF_TOKEN)) {
        auto paramTok = curToken_;
        std::string paramName = paramTok.literal;
        auto param = std::make_shared<ast::FuncParam>();
        param->token = paramTok;
        param->name = paramName;

        if (peekTokenIs(token::COLON)) {
            nextToken(); // ':'
            nextToken();
            param->typeAnnotation = parseTypeAnnotationWithUnion(false);
        }

        if (peekTokenIs(token::ASSIGN)) {
            nextToken(); // '='
            nextToken();
            param->defaultValue = parseExpression(LOWEST);
        }

        params.push_back(param);

        if (peekTokenIs(token::COMMA)) {
            nextToken();
            nextToken();
        } else {
            if (peekTokenIs(token::PIPE_TYPE)) {
                nextToken();
            }
            break;
        }
    }

    if (!curTokenIs(token::PIPE_TYPE)) {
        peekError(token::PIPE_TYPE);
        return nullptr;
    }

    nextToken(); // past '|'
    auto body = parseExpression(LOWEST);

    auto lambda = std::make_shared<ast::LambdaExpression>();
    lambda->token = tok;
    lambda->params = params;
    lambda->body = body;
    return lambda;
}

ast::ExpressionPtr Parser::parsePrefixRangeLiteral() {
    auto tok = curToken_;
    bool inclusive = (tok.type == token::DOTDOTEQ);
    nextToken();
    auto end = parseExpression(RANGE);

    auto range = std::make_shared<ast::RangeLiteral>();
    range->token = tok;
    range->start = nullptr;
    range->end = end;
    range->inclusive = inclusive;
    return range;
}

std::vector<ast::ExpressionPtr> Parser::parseExpressionList(token::Kind end) {
    std::vector<ast::ExpressionPtr> list;

    skipPeekNewlines();
    if (peekTokenIs(end)) {
        nextToken();
        return list;
    }

    nextToken();
    skipNewlines();
    if (curTokenIs(end)) return list;

    list.push_back(parseExpression(LOWEST));

    while (peekTokenIs(token::COMMA)) {
        nextToken(); // ','
        skipPeekNewlines();
        if (peekTokenIs(end)) {
            nextToken();
            return list;
        }
        nextToken();
        skipNewlines();
        if (curTokenIs(end)) return list;
        list.push_back(parseExpression(LOWEST));
    }

    skipPeekNewlines();
    if (!expectPeek(end)) return {};

    return list;
}

ast::ExpressionPtr Parser::parseInfixExpression(ast::ExpressionPtr left) {
    auto tok = curToken_;
    std::string op = curToken_.literal;
    int precedence = curPrecedence();

    nextToken();
    skipNewlines();
    auto right = parseExpression(precedence);

    auto infix = std::make_shared<ast::InfixExpression>();
    infix->token = tok;
    infix->left = left;
    infix->op = op;
    infix->right = right;
    return infix;
}

ast::ExpressionPtr Parser::parsePowerExpression(ast::ExpressionPtr left) {
    auto tok = curToken_;
    nextToken();
    skipNewlines();
    auto right = parseExpression(POWER - 1);

    auto infix = std::make_shared<ast::InfixExpression>();
    infix->token = tok;
    infix->left = left;
    infix->op = tok.literal;
    infix->right = right;
    return infix;
}

ast::ExpressionPtr Parser::parseAssignExpression(ast::ExpressionPtr left) {
    auto tok = curToken_;
    nextToken();
    skipNewlines();
    auto right = parseExpression(ASSIGN - 1);

    auto assign = std::make_shared<ast::AssignExpression>();
    assign->token = tok;
    assign->target = left;
    assign->value = right;
    return assign;
}

ast::ExpressionPtr Parser::parsePipeExpression(ast::ExpressionPtr left) {
    auto tok = curToken_;
    nextToken();
    skipNewlines();
    auto right = parseExpression(PIPE);

    auto pipe = std::make_shared<ast::PipeExpression>();
    pipe->token = tok;
    pipe->left = left;
    pipe->right = right;
    return pipe;
}

ast::ExpressionPtr Parser::parseTimeoutExpression(ast::ExpressionPtr left) {
    auto tok = curToken_;
    nextToken();
    skipNewlines();
    auto duration = parseExpression(TIMEOUT);

    auto timeout = std::make_shared<ast::TimeoutExpression>();
    timeout->token = tok;
    timeout->expression = left;
    timeout->duration = duration;
    return timeout;
}

ast::ExpressionPtr Parser::parseStreamExpression(ast::ExpressionPtr left) {
    auto tok = curToken_;
    std::string op = tok.literal;
    nextToken();
    skipNewlines();
    auto target = parseExpression(STREAM);

    auto stream = std::make_shared<ast::StreamExpression>();
    stream->token = tok;
    stream->source = left;
    stream->op = op;
    stream->target = target;
    return stream;
}

ast::ExpressionPtr Parser::parseRangeExpression(ast::ExpressionPtr left) {
    auto tok = curToken_;
    bool inclusive = (tok.type == token::DOTDOTEQ);
    nextToken();
    skipNewlines();
    auto right = parseExpression(RANGE);

    auto range = std::make_shared<ast::RangeLiteral>();
    range->token = tok;
    range->start = left;
    range->end = right;
    range->inclusive = inclusive;
    return range;
}

ast::ExpressionPtr Parser::parseCallExpression(ast::ExpressionPtr fn) {
    auto tok = curToken_;
    auto [args, kwargs] = parseCallArguments();

    auto call = std::make_shared<ast::CallExpression>();
    call->token = tok;
    call->function = fn;
    call->arguments = args;
    call->kwargs = kwargs;
    return call;
}

std::pair<std::vector<ast::ExpressionPtr>, std::map<std::string, ast::ExpressionPtr>> Parser::parseCallArguments() {
    std::vector<ast::ExpressionPtr> args;
    std::map<std::string, ast::ExpressionPtr> kwargs;

    skipPeekNewlines();
    if (peekTokenIs(token::RPAREN)) {
        nextToken();
        return {args, kwargs};
    }

    nextToken(); // first arg
    skipNewlines();

    while (!curTokenIs(token::RPAREN) && !curTokenIs(token::EOF_TOKEN)) {
        if (curTokenIs(token::IDENT) && peekTokenIs(token::ASSIGN)) {
            std::string kwName = curToken_.literal;
            nextToken(); // '='
            nextToken(); // value
            auto val = parseExpression(LOWEST);
            kwargs[kwName] = val;
        } else {
            auto arg = parseExpression(LOWEST);
            if (auto assign = std::dynamic_pointer_cast<ast::AssignExpression>(arg)) {
                if (auto ident = std::dynamic_pointer_cast<ast::Identifier>(assign->target)) {
                    kwargs[ident->value] = assign->value;
                } else {
                    args.push_back(arg);
                }
            } else {
                args.push_back(arg);
            }
        }

        skipPeekNewlines();
        if (peekTokenIs(token::COMMA)) {
            nextToken();
            skipPeekNewlines();
            if (peekTokenIs(token::RPAREN)) {
                nextToken();
                return {args, kwargs};
            }
            nextToken();
            skipNewlines();
        } else {
            break;
        }
    }

    skipPeekNewlines();
    if (!expectPeek(token::RPAREN)) {
        return {{}, {}};
    }

    return {args, kwargs};
}

ast::ExpressionPtr Parser::parseIndexExpression(ast::ExpressionPtr left) {
    auto tok = curToken_;
    nextToken(); // past '['
    skipNewlines();

    auto index = parseExpression(LOWEST);
    skipNewlines();

    if (!expectPeek(token::RBRACKET)) {
        if (!syntaxErrors_.empty()) {
            syntaxErrors_.back()->suggestion = "unclosed '[' from line " + std::to_string(tok.line) +
                                                ", col " + std::to_string(tok.column) + "; add a closing ']'";
        }
        return nullptr;
    }

    auto idx = std::make_shared<ast::IndexExpression>();
    idx->token = tok;
    idx->left = left;
    idx->index = index;
    return idx;
}

ast::ExpressionPtr Parser::parseDotExpression(ast::ExpressionPtr left) {
    auto tok = curToken_;
    nextToken(); // past '.'

    if (!curTokenIs(token::IDENT) && !token::isKeyword(curToken_.type) && !token::isTypeKeyword(curToken_.type)) {
        reportAt(curToken_, "provide a valid field or method name, e.g. 'obj.property'",
                 "expected identifier after '.', got \"" + curToken_.literal + "\"");
        return nullptr;
    }

    std::string fieldName = curToken_.literal;
    auto dot = std::make_shared<ast::DotExpression>();
    dot->token = tok;
    dot->left = left;
    dot->field = fieldName;
    return dot;
}

} // namespace pawl::parser
'''

with open('compiler/src/parser/parser.cpp', 'w', encoding='utf-8') as f:
    f.write(parser_cpp)
print("Wrote parser.cpp successfully.")
