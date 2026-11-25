// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

#include "lexer.h"

#include <utility>

namespace pawl::lexer {
namespace {

using token::Kind;
using token::Token;

inline Token make(Kind kind, std::string literal, int line, int column) {
    Token tok;
    tok.type = kind;
    tok.literal = std::move(literal);
    tok.line = line;
    tok.column = column;
    return tok;
}

} // namespace

Lexer::Lexer(std::string input) : input_(std::move(input)) {
    // Skip a UTF-8 byte order mark: editors on Windows add it by default and
    // it is not part of the program.
    if (input_.size() >= 3 && static_cast<unsigned char>(input_[0]) == 0xEF &&
        static_cast<unsigned char>(input_[1]) == 0xBB &&
        static_cast<unsigned char>(input_[2]) == 0xBF) {
        input_.erase(0, 3);
    }
    readChar();
}

std::vector<Token> Lexer::tokenize() {
    std::vector<Token> tokens;
    for (;;) {
        Token tok = nextToken();
        tokens.push_back(tok);
        if (tok.type == Kind::Eof) {
            break;
        }
    }
    return tokens;
}

Token Lexer::nextToken() {
    // 1. Return a buffered token (e.g. multiple DEDENTs) if present.
    if (!pendingTokens_.empty()) {
        Token tok = std::move(pendingTokens_.front());
        pendingTokens_.pop_front();
        return tok;
    }

    // 2. At the start of a line, handle leading whitespace and indentation.
    while (atLineStart_) {
        int spaces = 0;
        int tabs = 0;

        while (ch_ == ' ' || ch_ == '\t') {
            if (ch_ == ' ') {
                ++spaces;
            } else {
                ++tabs;
            }
            readChar();
        }

        // Empty line: skip and remain at line start.
        if (ch_ == '\r' || ch_ == '\n') {
            skipNewline();
            continue;
        }

        // Single-line comment: skip the rest of the line.
        if (ch_ == '/' && peekChar() == '/') {
            while (ch_ != '\n' && ch_ != '\r' && ch_ != 0) {
                readChar();
            }
            if (ch_ == '\r' || ch_ == '\n') {
                skipNewline();
            }
            continue;
        }

        // Block comment at line start.
        if (ch_ == '/' && peekChar() == '*') {
            skipBlockComment();
            continue;
        }

        // End of file at line start: unwind all remaining indentation levels.
        if (ch_ == 0) {
            if (indentStack_.size() > 1) {
                while (indentStack_.size() > 1) {
                    indentStack_.pop_back();
                    pendingTokens_.push_back(
                        make(Kind::Dedent, "DEDENT", line_, column_));
                }
                pendingTokens_.push_back(make(Kind::Eof, "", line_, column_));
                Token tok = std::move(pendingTokens_.front());
                pendingTokens_.pop_front();
                return tok;
            }
            return make(Kind::Eof, "", line_, column_);
        }

        // Significant code line: compute indentation level.
        // Rule: 4 spaces = 1 indent level, 1 tab = 1 indent level.
        const int currentIndent = tabs + (spaces / 4);
        const int topIndent = indentStack_.back();

        if (currentIndent > topIndent) {
            indentStack_.push_back(currentIndent);
            atLineStart_ = false;
            return make(Kind::Indent, "INDENT", line_, 1);
        }
        if (currentIndent < topIndent) {
            while (indentStack_.size() > 1 &&
                   indentStack_.back() > currentIndent) {
                indentStack_.pop_back();
                pendingTokens_.push_back(
                    make(Kind::Dedent, "DEDENT", line_, 1));
            }
            atLineStart_ = false;
            Token tok = std::move(pendingTokens_.front());
            pendingTokens_.pop_front();
            return tok;
        }
        // Indentation unchanged.
        atLineStart_ = false;
        break;
    }

    // 3. Skip horizontal whitespace within a line.
    skipWhitespace();

    Token tok;
    const int line = line_;
    const int col = column_;

    switch (ch_) {
    case '\r':
    case '\n':
        tok = make(Kind::Newline, "\n", line, col);
        skipNewline();
        atLineStart_ = true;
        return tok;

    case '=':
        if (peekChar() == '=') {
            if (peekCharN(2) == '=') {
                tok = make(Kind::StrictEq, "===", line, col);
                readChar();
                readChar();
                readChar();
            } else {
                tok = make(Kind::Eq, "==", line, col);
                readChar();
                readChar();
            }
        } else {
            tok = make(Kind::Assign, "=", line, col);
            readChar();
        }
        break;

    case '+':
        tok = make(Kind::Plus, "+", line, col);
        readChar();
        break;

    case '-':
        if (peekChar() == '>') {
            tok = make(Kind::Arrow, "->", line, col);
            readChar();
            readChar();
        } else {
            tok = make(Kind::Minus, "-", line, col);
            readChar();
        }
        break;

    case '!':
        if (peekChar() == '=') {
            tok = make(Kind::NotEq, "!=", line, col);
            readChar();
            readChar();
        } else {
            tok = make(Kind::Bang, "!", line, col);
            readChar();
        }
        break;

    case '*':
        if (peekChar() == '*') {
            tok = make(Kind::Power, "**", line, col);
            readChar();
            readChar();
        } else {
            tok = make(Kind::Asterisk, "*", line, col);
            readChar();
        }
        break;

    case '/':
        if (peekChar() == '/') {
            skipSingleLineComment();
            return nextToken();
        }
        if (peekChar() == '*') {
            skipBlockComment();
            return nextToken();
        }
        tok = make(Kind::Slash, "/", line, col);
        readChar();
        break;

    case '%':
        tok = make(Kind::Percent, "%", line, col);
        readChar();
        break;

    case '<':
        if (peekChar() == '<') {
            tok = make(Kind::StreamIn, "<<", line, col);
            readChar();
            readChar();
        } else if (peekChar() == '=') {
            tok = make(Kind::Le, "<=", line, col);
            readChar();
            readChar();
        } else {
            tok = make(Kind::Lt, "<", line, col);
            readChar();
        }
        break;

    case '>':
        if (peekChar() == '>' && peekCharN(2) == '>') {
            tok = make(Kind::StreamVar, ">>>", line, col);
            readChar();
            readChar();
            readChar();
        } else if (peekChar() == '>') {
            tok = make(Kind::StreamOut, ">>", line, col);
            readChar();
            readChar();
        } else if (peekChar() == '!') {
            tok = make(Kind::StreamOverwrite, ">!", line, col);
            readChar();
            readChar();
        } else if (peekChar() == '=') {
            tok = make(Kind::Ge, ">=", line, col);
            readChar();
            readChar();
        } else {
            tok = make(Kind::Gt, ">", line, col);
            readChar();
        }
        break;

    case '&':
        if (peekChar() == '&' && peekCharN(2) == '>' && peekCharN(3) == '>') {
            tok = make(Kind::StreamThread, "&&>>", line, col);
            readChar();
            readChar();
            readChar();
            readChar();
        } else if (peekChar() == '>' && peekCharN(2) == '>') {
            tok = make(Kind::StreamAsync, "&>>", line, col);
            readChar();
            readChar();
            readChar();
        } else {
            tok = make(Kind::Illegal, std::string(1, ch_), line, col);
            readChar();
        }
        break;

    case '?':
        if (peekChar() == '>' && peekCharN(2) == '>' && peekCharN(3) == '>') {
            tok = make(Kind::StreamErrVar, "?>>>", line, col);
            readChar();
            readChar();
            readChar();
            readChar();
        } else if (peekChar() == '>' && peekCharN(2) == '>') {
            tok = make(Kind::StreamErr, "?>>", line, col);
            readChar();
            readChar();
            readChar();
        } else {
            tok = make(Kind::Illegal, std::string(1, ch_), line, col);
            readChar();
        }
        break;

    case '|':
        if (peekChar() == '>') {
            tok = make(Kind::Pipe, "|>", line, col);
            readChar();
            readChar();
        } else {
            tok = make(Kind::PipeType, "|", line, col);
            readChar();
        }
        break;

    case '~':
        if (peekChar() == '>') {
            tok = make(Kind::TildeGt, "~>", line, col);
            readChar();
            readChar();
        } else {
            tok = make(Kind::Tilde, "~", line, col);
            readChar();
        }
        break;

    case '@':
        tok = make(Kind::At, "@", line, col);
        readChar();
        break;

    case ',':
        tok = make(Kind::Comma, ",", line, col);
        readChar();
        break;

    case ':':
        tok = make(Kind::Colon, ":", line, col);
        readChar();
        break;

    case ';':
        tok = make(Kind::Semicolon, ";", line, col);
        readChar();
        break;

    case '.':
        if (peekChar() == '.') {
            readChar();
            if (peekChar() == '=') {
                readChar();
                readChar();
                tok = make(Kind::RangeInclusive, "..=", line, col);
            } else {
                readChar();
                tok = make(Kind::Range, "..", line, col);
            }
        } else if (isDigit(peekChar())) {
            auto [literal, kind] = readFloatStartingWithDot();
            tok = make(kind, std::move(literal), line, col);
            return tok;
        } else {
            tok = make(Kind::Dot, ".", line, col);
            readChar();
        }
        break;

    case '(':
        tok = make(Kind::LParen, "(", line, col);
        readChar();
        break;

    case ')':
        tok = make(Kind::RParen, ")", line, col);
        readChar();
        break;

    case '[':
        tok = make(Kind::LBracket, "[", line, col);
        readChar();
        break;

    case ']':
        tok = make(Kind::RBracket, "]", line, col);
        readChar();
        break;

    case '{':
        tok = make(Kind::LBrace, "{", line, col);
        readChar();
        break;

    case '}':
        tok = make(Kind::RBrace, "}", line, col);
        readChar();
        break;

    case '"':
        if (peekChar() == '"' && peekCharN(2) == '"') {
            tok = make(Kind::String, readTripleString('"', false), line, col);
        } else {
            tok = make(Kind::String, readString('"'), line, col);
        }
        return tok;

    case '\'':
        if (peekChar() == '\'' && peekCharN(2) == '\'') {
            tok = make(Kind::String, readTripleString('\'', false), line, col);
        } else {
            tok = make(Kind::String, readString('\''), line, col);
        }
        return tok;

    case '`':
        tok = make(Kind::String, readRawString(), line, col);
        return tok;

    case 'r':
    case 'R':
        if ((peekChar() == 'f' || peekChar() == 'F') &&
            (peekCharN(2) == '"' || peekCharN(2) == '\'')) {
            readChar(); // consume 'r'/'R'
            readChar(); // consume 'f'/'F', ch_ is now the quote
            const char quote = ch_;
            if (peekChar() == quote && peekCharN(2) == quote) {
                tok = make(Kind::FString, readTripleString(quote, true), line, col);
            } else {
                tok = make(Kind::FString, readRawStringQuote(quote), line, col);
            }
            return tok;
        }
        if (peekChar() == '"' || peekChar() == '\'') {
            const char quote = peekChar();
            readChar(); // consume 'r'/'R', ch_ is now the quote
            if (peekChar() == quote && peekCharN(2) == quote) {
                tok = make(Kind::String, readTripleString(quote, true), line, col);
            } else {
                tok = make(Kind::String, readRawStringQuote(quote), line, col);
            }
            return tok;
        }
        {
            std::string literal = readIdentifier();
            Kind kind = token::lookupIdent(literal);
            return make(kind, std::move(literal), line, col);
        }

    case 'f':
    case 'F':
        if ((peekChar() == 'r' || peekChar() == 'R') &&
            (peekCharN(2) == '"' || peekCharN(2) == '\'')) {
            readChar(); // consume 'f'/'F'
            readChar(); // consume 'r'/'R', ch_ is now the quote
            const char quote = ch_;
            if (peekChar() == quote && peekCharN(2) == quote) {
                tok = make(Kind::FString, readTripleString(quote, true), line, col);
            } else {
                tok = make(Kind::FString, readRawStringQuote(quote), line, col);
            }
            return tok;
        }
        if (peekChar() == '"' || peekChar() == '\'') {
            const char quote = peekChar();
            readChar(); // consume 'f'/'F', ch_ is now the quote
            if (peekChar() == quote && peekCharN(2) == quote) {
                tok = make(Kind::FString, readTripleString(quote, false), line, col);
            } else {
                tok = make(Kind::FString, readString(quote), line, col);
            }
            return tok;
        }
        {
            std::string literal = readIdentifier();
            Kind kind = token::lookupIdent(literal);
            return make(kind, std::move(literal), line, col);
        }

    case 0:
        if (indentStack_.size() > 1) {
            while (indentStack_.size() > 1) {
                indentStack_.pop_back();
                pendingTokens_.push_back(
                    make(Kind::Dedent, "DEDENT", line, col));
            }
            pendingTokens_.push_back(make(Kind::Eof, "", line, col));
            tok = std::move(pendingTokens_.front());
            pendingTokens_.pop_front();
            return tok;
        }
        tok = make(Kind::Eof, "", line, col);
        break;

    default:
        if (isLetter(ch_)) {
            std::string literal = readIdentifier();
            Kind kind = token::lookupIdent(literal);
            return make(kind, std::move(literal), line, col);
        }
        if (isDigit(ch_)) {
            auto [literal, kind] = readNumber();
            return make(kind, std::move(literal), line, col);
        }
        tok = make(Kind::Illegal, std::string(1, ch_), line, col);
        readChar();
        break;
    }

    return tok;
}

void Lexer::readChar() {
    if (readPos_ >= input_.size()) {
        ch_ = 0;
    } else {
        ch_ = input_[readPos_];
    }
    pos_ = readPos_;
    ++readPos_;
    ++column_;
}

char Lexer::peekChar() const {
    if (readPos_ >= input_.size()) {
        return 0;
    }
    return input_[readPos_];
}

char Lexer::peekCharN(int n) const {
    if (n < 0) {
        return 0;
    }
    const std::size_t target = pos_ + static_cast<std::size_t>(n);
    if (target >= input_.size()) {
        return 0;
    }
    return input_[target];
}

void Lexer::skipWhitespace() {
    while (ch_ == ' ' || ch_ == '\t') {
        readChar();
    }
}

void Lexer::skipNewline() {
    if (ch_ == '\r') {
        if (peekChar() == '\n') {
            readChar();
        }
    }
    ++line_;
    column_ = 0;
    readChar();
}

void Lexer::skipSingleLineComment() {
    while (ch_ != '\n' && ch_ != '\r' && ch_ != 0) {
        readChar();
    }
}

void Lexer::skipBlockComment() {
    // Advance past '/' and '*'.
    readChar();
    readChar();
    for (;;) {
        if (ch_ == 0) {
            break;
        }
        if (ch_ == '*' && peekChar() == '/') {
            readChar(); // consume '*'
            readChar(); // consume '/'
            break;
        }
        if (ch_ == '\r') {
            if (peekChar() == '\n') {
                readChar();
            }
            ++line_;
            column_ = 0;
            readChar();
        } else if (ch_ == '\n') {
            ++line_;
            column_ = 0;
            readChar();
        } else {
            readChar();
        }
    }
}

std::string Lexer::readIdentifier() {
    const std::size_t startPos = pos_;
    while (isLetter(ch_) || isDigit(ch_)) {
        readChar();
    }
    return input_.substr(startPos, pos_ - startPos);
}

std::pair<std::string, Kind> Lexer::readNumber() {
    const std::size_t startPos = pos_;
    bool isFloat = false;

    while (isDigit(ch_)) {
        readChar();
    }

    if (ch_ == '.' && isDigit(peekChar())) {
        isFloat = true;
        readChar();
        while (isDigit(ch_)) {
            readChar();
        }
    }

    if (ch_ == 'm' && peekChar() == 's' && !isLetterOrDigit(peekCharN(2))) {
        readChar();
        readChar();
    } else if ((ch_ == 'K' || ch_ == 'k') &&
               (peekChar() == 'B' || peekChar() == 'b') &&
               !isLetterOrDigit(peekCharN(2))) {
        readChar();
        readChar();
    } else if ((ch_ == 'M' || ch_ == 'm') &&
               (peekChar() == 'B' || peekChar() == 'b') &&
               !isLetterOrDigit(peekCharN(2))) {
        readChar();
        readChar();
    } else if ((ch_ == 'G' || ch_ == 'g') &&
               (peekChar() == 'B' || peekChar() == 'b') &&
               !isLetterOrDigit(peekCharN(2))) {
        readChar();
        readChar();
    } else if ((ch_ == 's' || ch_ == 'm' || ch_ == 'h') &&
               !isLetterOrDigit(peekChar())) {
        readChar();
    }

    std::string literal = input_.substr(startPos, pos_ - startPos);
    return {std::move(literal), isFloat ? Kind::Float : Kind::Int};
}

std::pair<std::string, Kind> Lexer::readFloatStartingWithDot() {
    const std::size_t startPos = pos_;
    readChar(); // consume '.'
    while (isDigit(ch_)) {
        readChar();
    }

    if (ch_ == 'm' && peekChar() == 's' && !isLetterOrDigit(peekCharN(2))) {
        readChar();
        readChar();
    } else if ((ch_ == 'K' || ch_ == 'k') &&
               (peekChar() == 'B' || peekChar() == 'b') &&
               !isLetterOrDigit(peekCharN(2))) {
        readChar();
        readChar();
    } else if ((ch_ == 'M' || ch_ == 'm') &&
               (peekChar() == 'B' || peekChar() == 'b') &&
               !isLetterOrDigit(peekCharN(2))) {
        readChar();
        readChar();
    } else if ((ch_ == 'G' || ch_ == 'g') &&
               (peekChar() == 'B' || peekChar() == 'b') &&
               !isLetterOrDigit(peekCharN(2))) {
        readChar();
        readChar();
    } else if ((ch_ == 's' || ch_ == 'm' || ch_ == 'h') &&
               !isLetterOrDigit(peekChar())) {
        readChar();
    }

    std::string literal = input_.substr(startPos, pos_ - startPos);
    return {std::move(literal), Kind::Float};
}

std::string Lexer::readString(char quote) {
    std::string out;
    for (;;) {
        readChar();
        if (ch_ == quote || ch_ == 0) {
            break;
        }
        if (ch_ == '\\') {
            readChar();
            switch (ch_) {
            case 'n': out.push_back('\n'); break;
            case 't': out.push_back('\t'); break;
            case 'r': out.push_back('\r'); break;
            case '"': out.push_back('"'); break;
            case '\'': out.push_back('\''); break;
            case '\\': out.push_back('\\'); break;
            case '$': out.push_back('$'); break;
            default:
                out.push_back('\\');
                out.push_back(ch_);
                break;
            }
        } else {
            if (ch_ == '\n') {
                ++line_;
                column_ = 0;
            }
            out.push_back(ch_);
        }
    }
    if (ch_ == quote) {
        readChar();
    }
    return out;
}

std::string Lexer::readTripleString(char quote, bool isRaw) {
    // Consume the three opening quotes.
    readChar();
    readChar();
    readChar();

    std::string out;
    for (;;) {
        if (ch_ == 0) {
            break;
        }
        if (ch_ == quote && peekChar() == quote && peekCharN(2) == quote) {
            readChar();
            readChar();
            readChar();
            break;
        }

        if (ch_ == '\\') {
            if (isRaw) {
                if (peekChar() == quote) {
                    out.push_back('\\');
                    readChar();
                    out.push_back(ch_);
                    readChar();
                    continue;
                }
                out.push_back('\\');
                readChar();
                continue;
            }

            readChar();
            switch (ch_) {
            case 'n': out.push_back('\n'); break;
            case 't': out.push_back('\t'); break;
            case 'r': out.push_back('\r'); break;
            case 'b': out.push_back('\b'); break;
            case 'f': out.push_back('\f'); break;
            case 'a': out.push_back('\a'); break;
            case '0': out.push_back('\0'); break;
            case '"': out.push_back('"'); break;
            case '\'': out.push_back('\''); break;
            case '\\': out.push_back('\\'); break;
            case '$': out.push_back('$'); break;
            case '\r':
                if (peekChar() == '\n') {
                    readChar();
                }
                ++line_;
                column_ = 0;
                break;
            case '\n':
                ++line_;
                column_ = 0;
                break;
            default:
                out.push_back('\\');
                out.push_back(ch_);
                break;
            }
            readChar();
            continue;
        }

        if (ch_ == '\r') {
            if (peekChar() == '\n') {
                readChar();
            }
            ++line_;
            column_ = 0;
            out.push_back('\n');
            readChar();
        } else if (ch_ == '\n') {
            ++line_;
            column_ = 0;
            out.push_back('\n');
            readChar();
        } else {
            out.push_back(ch_);
            readChar();
        }
    }
    return out;
}

std::string Lexer::readRawStringQuote(char quote) {
    readChar(); // consume opening quote
    std::string out;
    for (;;) {
        if (ch_ == quote || ch_ == 0) {
            break;
        }
        if (ch_ == '\\') {
            if (peekChar() == quote) {
                out.push_back('\\');
                readChar();
                out.push_back(ch_);
                readChar();
                continue;
            }
            out.push_back('\\');
            readChar();
            continue;
        }
        if (ch_ == '\r') {
            if (peekChar() == '\n') {
                readChar();
            }
            ++line_;
            column_ = 0;
            out.push_back('\n');
            readChar();
        } else if (ch_ == '\n') {
            ++line_;
            column_ = 0;
            out.push_back('\n');
            readChar();
        } else {
            out.push_back(ch_);
            readChar();
        }
    }
    if (ch_ == quote) {
        readChar();
    }
    return out;
}

std::string Lexer::readRawString() {
    readChar(); // consume opening '`'
    std::string out;
    for (;;) {
        if (ch_ == '`' || ch_ == 0) {
            break;
        }
        if (ch_ == '\r') {
            if (peekChar() == '\n') {
                readChar();
            }
            ++line_;
            column_ = 0;
            out.push_back('\n');
            readChar();
        } else if (ch_ == '\n') {
            ++line_;
            column_ = 0;
            out.push_back('\n');
            readChar();
        } else {
            out.push_back(ch_);
            readChar();
        }
    }
    if (ch_ == '`') {
        readChar();
    }
    return out;
}

bool isLetter(char ch) {
    return ('a' <= ch && ch <= 'z') || ('A' <= ch && ch <= 'Z') || ch == '_';
}

bool isDigit(char ch) { return '0' <= ch && ch <= '9'; }

bool isLetterOrDigit(char ch) { return isLetter(ch) || isDigit(ch); }

} // namespace pawl::lexer
