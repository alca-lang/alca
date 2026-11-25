// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0
//
// Full lexical scanner for Pawl.  Faithful C++17 port of pkg/lexer/lexer.go.

#pragma once

#include <cstddef>
#include <deque>
#include <string>
#include <vector>

#include "token/token.h"

namespace pawl::lexer {

class Lexer {
public:
    explicit Lexer(std::string input);

    // Reads every token up to and including EOF.
    [[nodiscard]] std::vector<token::Token> tokenize();

    // Scans and returns the next token.
    [[nodiscard]] token::Token nextToken();

private:
    void readChar();
    [[nodiscard]] char peekChar() const;
    [[nodiscard]] char peekCharN(int n) const;
    void skipWhitespace();
    void skipNewline();
    void skipSingleLineComment();
    void skipBlockComment();

    [[nodiscard]] std::string readIdentifier();
    [[nodiscard]] std::pair<std::string, token::Kind> readNumber();
    [[nodiscard]] std::pair<std::string, token::Kind> readFloatStartingWithDot();
    [[nodiscard]] std::string readString(char quote);
    [[nodiscard]] std::string readTripleString(char quote, bool isRaw);
    [[nodiscard]] std::string readRawStringQuote(char quote);
    [[nodiscard]] std::string readRawString();

    std::string input_;
    std::size_t pos_ = 0;      // current position (points at ch_)
    std::size_t readPos_ = 0;  // next position to read
    char ch_ = 0;              // current byte under examination
    int line_ = 1;
    int column_ = 0;
    std::vector<int> indentStack_{0};
    std::deque<token::Token> pendingTokens_;
    bool atLineStart_ = true;
};

[[nodiscard]] bool isLetter(char ch);
[[nodiscard]] bool isDigit(char ch);
[[nodiscard]] bool isLetterOrDigit(char ch);

} // namespace pawl::lexer
