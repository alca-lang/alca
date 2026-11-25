// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

#include "token.h"

#include <algorithm>
#include <array>
#include <cstdio>
#include <string>
#include <string_view>
#include <vector>

namespace ulang::token {
namespace {

struct KeywordEntry {
    std::string_view name;
    Kind kind;
};

// Kept verbatim from pkg/token/token.go.  Aliases are included.
constexpr KeywordEntry kKeywords[] = {
    {"func", FUNC},
    {"fn", FUNC},
    {"return", RETURN},
    {"if", IF},
    {"else", ELSE},
    {"elif", ELIF},
    {"while", WHILE},
    {"for", FOR},
    {"loop", LOOP},
    {"foreach", FOREACH},
    {"in", IN},
    {"break", BREAK},
    {"continue", CONTINUE},
    {"and", AND},
    {"or", OR},
    {"not", NOT},
    {"true", TRUE},
    {"false", FALSE},
    {"null", NULL_TOKEN},
    {"struct", STRUCT},
    {"class", CLASS},
    {"extends", EXTENDS},
    {"pub", PUB},
    {"public", PUBLIC},
    {"private", PRIVATE},
    {"const", CONST},
    {"use", USE},
    {"as", AS},
    {"ba", BA},
    {"threads", THREADS},
    {"wait", WAIT},
    {"watch", WATCH},
    {"test", TEST},
    {"mock", MOCK},
    {"assert", ASSERT},
    {"tx", TX},
    {"match", MATCH},
    {"case", CASE},
    {"err", ERR},
    {"import", IMPORT},
    {"enum", ENUM},
    {"interface", INTERFACE},
    {"trait", TRAIT},
    {"async", ASYNC},
    {"await", AWAIT},

    {"string", TYPE_STRING},
    {"int", TYPE_INT},
    {"float", TYPE_FLOAT},
    {"bool", TYPE_BOOL},
    {"list", TYPE_LIST},
    {"map", TYPE_MAP},
    {"blob", TYPE_BLOB},
    {"secret", TYPE_SECRET},
    {"json", TYPE_JSON},
    {"void", TYPE_VOID},
};

[[nodiscard]] std::string_view canonicalName(Kind kind) {
    switch (kind) {
    case Kind::Illegal: return "ILLEGAL";
    case Kind::Eof: return "EOF";
    case Kind::Newline: return "NEWLINE";
    case Kind::Indent: return "INDENT";
    case Kind::Dedent: return "DEDENT";
    case Kind::Comment: return "COMMENT";
    case Kind::Ident: return "IDENT";
    case Kind::Int: return "INT";
    case Kind::Float: return "FLOAT";
    case Kind::String: return "STRING";
    case Kind::FString: return "FSTRING";
    case Kind::Plus: return "+";
    case Kind::Minus: return "-";
    case Kind::Asterisk: return "*";
    case Kind::Slash: return "/";
    case Kind::Percent: return "%";
    case Kind::Power: return "**";
    case Kind::Assign: return "=";
    case Kind::Bang: return "!";
    case Kind::Eq: return "==";
    case Kind::StrictEq: return "===";
    case Kind::NotEq: return "!=";
    case Kind::Lt: return "<";
    case Kind::Le: return "<=";
    case Kind::Gt: return ">";
    case Kind::Ge: return ">=";
    case Kind::StreamOut: return ">>";
    case Kind::StreamOverwrite: return ">!";
    case Kind::StreamVar: return ">>>";
    case Kind::StreamIn: return "<<";
    case Kind::StreamErr: return "?>>";
    case Kind::StreamErrVar: return "?>>>";
    case Kind::StreamAsync: return "&>>";
    case Kind::StreamThread: return "&&>>";
    case Kind::Pipe: return "|>";
    case Kind::Arrow: return "->";
    case Kind::TildeGt: return "~>";
    case Kind::Tilde: return "~";
    case Kind::At: return "@";
    case Kind::PipeType: return "|";
    case Kind::Range: return "..";
    case Kind::RangeInclusive: return "..=";
    case Kind::Comma: return ",";
    case Kind::Colon: return ":";
    case Kind::Dot: return ".";
    case Kind::LParen: return "(";
    case Kind::RParen: return ")";
    case Kind::LBracket: return "[";
    case Kind::RBracket: return "]";
    case Kind::LBrace: return "{";
    case Kind::RBrace: return "}";
    case Kind::Semicolon: return ";";
    case Kind::Func: return "func";
    case Kind::Return: return "return";
    case Kind::If: return "if";
    case Kind::Else: return "else";
    case Kind::Elif: return "elif";
    case Kind::While: return "while";
    case Kind::For: return "for";
    case Kind::Loop: return "loop";
    case Kind::Foreach: return "foreach";
    case Kind::In: return "in";
    case Kind::Break: return "break";
    case Kind::Continue: return "continue";
    case Kind::And: return "and";
    case Kind::Or: return "or";
    case Kind::Not: return "not";
    case Kind::True: return "true";
    case Kind::False: return "false";
    case Kind::Null: return "null";
    case Kind::Struct: return "struct";
    case Kind::Class: return "class";
    case Kind::Extends: return "extends";
    case Kind::Pub: return "pub";
    case Kind::Public: return "public";
    case Kind::Private: return "private";
    case Kind::Const: return "const";
    case Kind::Use: return "use";
    case Kind::As: return "as";
    case Kind::Ba: return "ba";
    case Kind::Threads: return "threads";
    case Kind::Wait: return "wait";
    case Kind::Watch: return "watch";
    case Kind::Test: return "test";
    case Kind::Mock: return "mock";
    case Kind::Assert: return "assert";
    case Kind::Tx: return "tx";
    case Kind::Match: return "match";
    case Kind::Case: return "case";
    case Kind::Err: return "err";
    case Kind::Import: return "import";
    case Kind::Enum: return "enum";
    case Kind::Interface: return "interface";
    case Kind::Trait: return "trait";
    case Kind::Async: return "async";
    case Kind::Await: return "await";
    case Kind::TypeString: return "string";
    case Kind::TypeInt: return "int";
    case Kind::TypeFloat: return "float";
    case Kind::TypeBool: return "bool";
    case Kind::TypeList: return "list";
    case Kind::TypeMap: return "map";
    case Kind::TypeBlob: return "blob";
    case Kind::TypeSecret: return "secret";
    case Kind::TypeJson: return "json";
    case Kind::TypeVoid: return "void";
    }
    return "ILLEGAL";
}

// Approximates Go's %q for a byte string.  Printable ASCII is preserved,
// standard escapes are used for control characters, and high bytes (UTF-8
// payload) are passed through unchanged.
[[nodiscard]] std::string quoteGo(std::string_view s) {
    std::string out;
    out.reserve(s.size() + 2);
    out.push_back('"');
    for (unsigned char c : s) {
        switch (c) {
        case '"': out += "\\\""; break;
        case '\\': out += "\\\\"; break;
        case '\a': out += "\\a"; break;
        case '\b': out += "\\b"; break;
        case '\f': out += "\\f"; break;
        case '\n': out += "\\n"; break;
        case '\r': out += "\\r"; break;
        case '\t': out += "\\t"; break;
        case '\v': out += "\\v"; break;
        default:
            if (c < 0x20 || c == 0x7f) {
                char buf[5];
                std::snprintf(buf, sizeof(buf), "\\x%02x", static_cast<unsigned>(c));
                out += buf;
            } else {
                out.push_back(static_cast<char>(c));
            }
        }
    }
    out.push_back('"');
    return out;
}

} // namespace

std::string_view kindName(Kind kind) { return canonicalName(kind); }

Kind lookupIdent(std::string_view ident) {
    if (ident.empty()) {
        return Kind::Ident;
    }
    for (const auto& entry : kKeywords) {
        if (entry.name == ident) {
            return entry.kind;
        }
    }
    return Kind::Ident;
}

bool isKeyword(Kind kind) {
    switch (kind) {
    case FUNC: case RETURN: case IF: case ELSE: case ELIF: case WHILE: case FOR:
    case LOOP: case FOREACH: case IN: case BREAK: case CONTINUE: case AND:
    case OR: case NOT: case TRUE: case FALSE: case NULL_TOKEN: case STRUCT:
    case CLASS: case EXTENDS: case PUB: case PUBLIC: case PRIVATE: case CONST:
    case USE: case AS: case BA: case THREADS: case WAIT: case WATCH: case TEST:
    case MOCK: case ASSERT: case TX: case MATCH: case CASE: case ERR:
    case IMPORT:
        return true;
    default:
        return false;
    }
}

bool isTypeKeyword(Kind kind) {
    switch (kind) {
    case TYPE_STRING: case TYPE_INT: case TYPE_FLOAT: case TYPE_BOOL:
    case TYPE_LIST: case TYPE_MAP: case TYPE_BLOB: case TYPE_SECRET:
    case TYPE_JSON: case TYPE_VOID:
        return true;
    default:
        return false;
    }
}

std::vector<std::string_view> keywordNames() {
    std::vector<std::string_view> names;
    names.reserve(sizeof(kKeywords) / sizeof(kKeywords[0]));
    for (const auto& entry : kKeywords) {
        if (!entry.name.empty() && !isTypeKeyword(entry.kind)) {
            names.push_back(entry.name);
        }
    }
    std::sort(names.begin(), names.end());
    return names;
}

std::vector<std::string_view> typeKeywordNames() {
    std::vector<std::string_view> names;
    names.reserve(16);
    for (const auto& entry : kKeywords) {
        if (!entry.name.empty() && isTypeKeyword(entry.kind)) {
            names.push_back(entry.name);
        }
    }
    std::sort(names.begin(), names.end());
    return names;
}

std::string Token::toString() const {
    std::string out = "Token(";
    out += canonicalName(type);
    out += ", ";
    out += quoteGo(literal);
    out += ", ";
    out += std::to_string(line);
    out += ':';
    out += std::to_string(column);
    out += ')';
    return out;
}

std::ostream& operator<<(std::ostream& os, Kind kind) {
    return os << canonicalName(kind);
}

std::ostream& operator<<(std::ostream& os, const Token& tok) {
    return os << tok.toString();
}

} // namespace ulang::token
