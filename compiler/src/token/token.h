// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0
//
// Token definitions for the Pawl programming language.
//
// This is a faithful C++17 port of pkg/token/token.go.  Token kinds keep the
// original *spelling* semantics: aliases such as LT_EQ/LTE or STREAM_OUT/
// STREAM_APPEND are the same kind in the C++ model because the Go version
// declares them as the same underlying string constant.

#pragma once

#include <cstdint>
#include <ostream>
#include <string>
#include <string_view>
#include <vector>

namespace ulang::token {

enum class Kind : std::uint16_t {
    Illegal,
    Eof,
    Newline,
    Indent,
    Dedent,
    Comment,

    Ident,
    Int,
    Float,
    String,
    FString,

    // Arithmetic / assignment
    Plus,
    Minus,
    Asterisk,
    Slash,
    Percent,
    Power,
    Assign,
    Bang,

    // Comparison
    Eq,
    StrictEq,
    NotEq,
    Lt,
    Le,
    Gt,
    Ge,

    // Streams and pipes
    StreamOut,      // ">>"   (also STREAM_APPEND)
    StreamOverwrite,// ">!"
    StreamVar,      // ">>>"  (also STREAM_CAPTURE)
    StreamIn,       // "<<"
    StreamErr,      // "?>>" (also STREAM_ERR_APPEND)
    StreamErrVar,   // "?>>>" (also STREAM_ERR_CAPTURE)
    StreamAsync,    // "&>>"
    StreamThread,   // "&&>>"
    Pipe,           // "|>"   (also PIPE_ARROW)
    Arrow,          // "->"
    TildeGt,        // "~>"   (also TILDE_ARROW)
    Tilde,          // "~"
    At,             // "@"
    PipeType,       // "|"
    Range,          // ".."   (also DOTDOT)
    RangeInclusive, // "..="

    // Delimiters
    Comma,
    Colon,
    Dot,
    LParen,
    RParen,
    LBracket,
    RBracket,
    LBrace,
    RBrace,
    Semicolon,

    // Declaration / control-flow keywords
    Func,
    Return,
    If,
    Else,
    Elif,
    While,
    For,
    Loop,
    Foreach,
    In,
    Break,
    Continue,
    And,
    Or,
    Not,
    True,
    False,
    Null,
    Struct,
    Class,
    Extends,
    Pub,
    Public,
    Private,
    Const,
    Use,
    As,
    Ba,
    Threads,
    Wait,
    Watch,
    Test,
    Mock,
    Assert,
    Defer,
    Constexpr,
    Try,
    Catch,
    Finally,
    Throw,
    Tx,
    Match,
    Case,
    Err,
    Import,
    Enum,
    Interface,
    Trait,
    Async,
    Await,

    // Type keywords
    TypeString,
    TypeInt,
    TypeFloat,
    TypeBool,
    TypeList,
    TypeMap,
    TypeBlob,
    TypeSecret,
    TypeJson,
    TypeVoid,
};

// Compatibility aliases mirroring pkg/token/token.go.  Several Go constants
// share the same string value; they are deliberately the same C++ kind.
inline constexpr Kind ILLEGAL = Kind::Illegal;
inline constexpr Kind EOF_TOKEN = Kind::Eof;
inline constexpr Kind NEWLINE = Kind::Newline;
inline constexpr Kind INDENT = Kind::Indent;
inline constexpr Kind DEDENT = Kind::Dedent;
inline constexpr Kind COMMENT = Kind::Comment;
inline constexpr Kind IDENT = Kind::Ident;
inline constexpr Kind INT = Kind::Int;
inline constexpr Kind FLOAT = Kind::Float;
inline constexpr Kind STRING = Kind::String;
inline constexpr Kind FSTRING = Kind::FString;
inline constexpr Kind PLUS = Kind::Plus;
inline constexpr Kind MINUS = Kind::Minus;
inline constexpr Kind ASTERISK = Kind::Asterisk;
inline constexpr Kind SLASH = Kind::Slash;
inline constexpr Kind PERCENT = Kind::Percent;
inline constexpr Kind POWER = Kind::Power;
inline constexpr Kind ASSIGN = Kind::Assign;
inline constexpr Kind BANG = Kind::Bang;
inline constexpr Kind EQ = Kind::Eq;
inline constexpr Kind STRICT_EQ = Kind::StrictEq;
inline constexpr Kind NOT_EQ = Kind::NotEq;
inline constexpr Kind LT = Kind::Lt;
inline constexpr Kind LT_EQ = Kind::Le;
inline constexpr Kind LTE = Kind::Le;
inline constexpr Kind GT = Kind::Gt;
inline constexpr Kind GT_EQ = Kind::Ge;
inline constexpr Kind GTE = Kind::Ge;
inline constexpr Kind STREAM_APPEND = Kind::StreamOut;
inline constexpr Kind STREAM_OUT = Kind::StreamOut;
inline constexpr Kind STREAM_OVERWRITE = Kind::StreamOverwrite;
inline constexpr Kind STREAM_CAPTURE = Kind::StreamVar;
inline constexpr Kind STREAM_VAR = Kind::StreamVar;
inline constexpr Kind STREAM_IN = Kind::StreamIn;
inline constexpr Kind STREAM_ERR_APPEND = Kind::StreamErr;
inline constexpr Kind STREAM_ERR = Kind::StreamErr;
inline constexpr Kind STREAM_ERR_CAPTURE = Kind::StreamErrVar;
inline constexpr Kind STREAM_ERR_VAR = Kind::StreamErrVar;
inline constexpr Kind STREAM_ASYNC = Kind::StreamAsync;
inline constexpr Kind STREAM_THREAD = Kind::StreamThread;
inline constexpr Kind PIPE = Kind::Pipe;
inline constexpr Kind PIPE_ARROW = Kind::Pipe;
inline constexpr Kind ARROW = Kind::Arrow;
inline constexpr Kind TILDE_ARROW = Kind::TildeGt;
inline constexpr Kind TILDE_GT = Kind::TildeGt;
inline constexpr Kind TILDE = Kind::Tilde;
inline constexpr Kind AT = Kind::At;
inline constexpr Kind PIPE_TYPE = Kind::PipeType;
inline constexpr Kind RANGE = Kind::Range;
inline constexpr Kind DOTDOT = Kind::Range;
inline constexpr Kind DOTDOTEQ = Kind::RangeInclusive;
inline constexpr Kind COMMA = Kind::Comma;
inline constexpr Kind COLON = Kind::Colon;
inline constexpr Kind DOT = Kind::Dot;
inline constexpr Kind LPAREN = Kind::LParen;
inline constexpr Kind RPAREN = Kind::RParen;
inline constexpr Kind LBRACKET = Kind::LBracket;
inline constexpr Kind RBRACKET = Kind::RBracket;
inline constexpr Kind LBRACE = Kind::LBrace;
inline constexpr Kind RBRACE = Kind::RBrace;
inline constexpr Kind SEMICOLON = Kind::Semicolon;
inline constexpr Kind FUNC = Kind::Func;
inline constexpr Kind RETURN = Kind::Return;
inline constexpr Kind IF = Kind::If;
inline constexpr Kind ELSE = Kind::Else;
inline constexpr Kind ELIF = Kind::Elif;
inline constexpr Kind WHILE = Kind::While;
inline constexpr Kind FOR = Kind::For;
inline constexpr Kind LOOP = Kind::Loop;
inline constexpr Kind FOREACH = Kind::Foreach;
inline constexpr Kind IN = Kind::In;
inline constexpr Kind BREAK = Kind::Break;
inline constexpr Kind CONTINUE = Kind::Continue;
inline constexpr Kind AND = Kind::And;
inline constexpr Kind OR = Kind::Or;
inline constexpr Kind NOT = Kind::Not;
inline constexpr Kind TRUE = Kind::True;
inline constexpr Kind FALSE = Kind::False;
inline constexpr Kind NULL_TOKEN = Kind::Null;
inline constexpr Kind STRUCT = Kind::Struct;
inline constexpr Kind CLASS = Kind::Class;
inline constexpr Kind EXTENDS = Kind::Extends;
inline constexpr Kind PUB = Kind::Pub;
inline constexpr Kind PUBLIC = Kind::Public;
inline constexpr Kind PRIVATE = Kind::Private;
inline constexpr Kind CONST = Kind::Const;
inline constexpr Kind USE = Kind::Use;
inline constexpr Kind AS = Kind::As;
inline constexpr Kind BA = Kind::Ba;
inline constexpr Kind THREADS = Kind::Threads;
inline constexpr Kind WAIT = Kind::Wait;
inline constexpr Kind WATCH = Kind::Watch;
inline constexpr Kind TEST = Kind::Test;
inline constexpr Kind MOCK = Kind::Mock;
inline constexpr Kind ASSERT = Kind::Assert;
inline constexpr Kind DEFER = Kind::Defer;
inline constexpr Kind CONSTEXPR = Kind::Constexpr;
inline constexpr Kind TRY = Kind::Try;
inline constexpr Kind CATCH = Kind::Catch;
inline constexpr Kind FINALLY = Kind::Finally;
inline constexpr Kind THROW = Kind::Throw;
inline constexpr Kind TX = Kind::Tx;
inline constexpr Kind MATCH = Kind::Match;
inline constexpr Kind CASE = Kind::Case;
inline constexpr Kind ERR = Kind::Err;
inline constexpr Kind IMPORT = Kind::Import;
inline constexpr Kind ENUM = Kind::Enum;
inline constexpr Kind INTERFACE = Kind::Interface;
inline constexpr Kind TRAIT = Kind::Trait;
inline constexpr Kind ASYNC = Kind::Async;
inline constexpr Kind AWAIT = Kind::Await;
inline constexpr Kind TYPE_STRING = Kind::TypeString;
inline constexpr Kind TYPE_INT = Kind::TypeInt;
inline constexpr Kind TYPE_FLOAT = Kind::TypeFloat;
inline constexpr Kind TYPE_BOOL = Kind::TypeBool;
inline constexpr Kind TYPE_LIST = Kind::TypeList;
inline constexpr Kind TYPE_MAP = Kind::TypeMap;
inline constexpr Kind TYPE_BLOB = Kind::TypeBlob;
inline constexpr Kind TYPE_SECRET = Kind::TypeSecret;
inline constexpr Kind TYPE_JSON = Kind::TypeJson;
inline constexpr Kind TYPE_VOID = Kind::TypeVoid;

// Duration / size suffix spellings (literal text, not token kinds).
inline constexpr std::string_view SuffixMS = "ms";
inline constexpr std::string_view SuffixS = "s";
inline constexpr std::string_view SuffixM = "m";
inline constexpr std::string_view SuffixH = "h";
inline constexpr std::string_view SuffixKB = "KB";
inline constexpr std::string_view SuffixMB = "MB";
inline constexpr std::string_view SuffixGB = "GB";

struct Token {
    Kind type = Kind::Illegal;
    std::string literal;
    int line = 0;
    int column = 0;

    // Human-readable representation, matching the Go Token.String() format:
    //   Token(TYPE, "literal", line:column)
    [[nodiscard]] std::string toString() const;
};

// The Go TokenType is a string, so this returns the canonical spelling.
[[nodiscard]] std::string_view kindName(Kind kind);

// Look up an identifier; returns Kind::Ident when it is not reserved.
[[nodiscard]] Kind lookupIdent(std::string_view ident);

[[nodiscard]] bool isKeyword(Kind kind);
[[nodiscard]] bool isTypeKeyword(Kind kind);
[[nodiscard]] bool isOperatorKind(Kind kind);

// Sorted names, for editor tooling / completion parity with the Go package.
[[nodiscard]] std::vector<std::string_view> keywordNames();
[[nodiscard]] std::vector<std::string_view> typeKeywordNames();

// Stream helpers for tests and diagnostics.
std::ostream& operator<<(std::ostream& os, Kind kind);
std::ostream& operator<<(std::ostream& os, const Token& tok);

} // namespace ulang::token
