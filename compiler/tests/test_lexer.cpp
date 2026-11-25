// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0
//
// Port of pkg/lexer/lexer_test.go plus a few regression cases.

#include "lexer/lexer.h"
#include "microtest/microtest.h"
#include "token/token.h"

#include <string>
#include <utility>
#include <vector>

using ulang::lexer::Lexer;
using ulang::token::Kind;
using ulang::token::Token;

namespace {

void expectTypes(const std::string& input, const std::vector<Kind>& expected) {
    Lexer lexer(input);
    const auto tokens = lexer.tokenize();
    MT_CHECK_EQ(tokens.size(), expected.size());
    for (std::size_t i = 0; i < expected.size(); ++i) {
        MT_CHECK_EQ(tokens[i].type, expected[i]);
    }
}

} // namespace

MT_TEST(lexer, basic_operators_and_delimiters) {
    const std::string input =
        "= + - ! * / % ** == === != < <= > >= , :\n . ( ) [ ] { } |> | -> ~ ~> @ ..";
    const std::vector<std::pair<Kind, std::string>> expected = {
        {ulang::token::ASSIGN, "="},   {ulang::token::PLUS, "+"},
        {ulang::token::MINUS, "-"},    {ulang::token::BANG, "!"},
        {ulang::token::ASTERISK, "*"}, {ulang::token::SLASH, "/"},
        {ulang::token::PERCENT, "%"},  {ulang::token::POWER, "**"},
        {ulang::token::EQ, "=="},      {ulang::token::STRICT_EQ, "==="},
        {ulang::token::NOT_EQ, "!="},  {ulang::token::LT, "<"},
        {ulang::token::LT_EQ, "<="},   {ulang::token::GT, ">"},
        {ulang::token::GT_EQ, ">="},   {ulang::token::COMMA, ","},
        {ulang::token::COLON, ":"},    {ulang::token::NEWLINE, "\n"},
        {ulang::token::DOT, "."},      {ulang::token::LPAREN, "("},
        {ulang::token::RPAREN, ")"},   {ulang::token::LBRACKET, "["},
        {ulang::token::RBRACKET, "]"}, {ulang::token::LBRACE, "{"},
        {ulang::token::RBRACE, "}"},   {ulang::token::PIPE, "|>"},
        {ulang::token::PIPE_TYPE, "|"},{ulang::token::ARROW, "->"},
        {ulang::token::TILDE, "~"},    {ulang::token::TILDE_GT, "~>"},
        {ulang::token::AT, "@"},       {ulang::token::RANGE, ".."},
        {ulang::token::EOF_TOKEN, ""},
    };

    Lexer lexer(input);
    for (std::size_t i = 0; i < expected.size(); ++i) {
        const Token tok = lexer.nextToken();
        MT_CHECK_EQ(tok.type, expected[i].first);
        MT_CHECK_EQ(tok.literal, expected[i].second);
    }
}

MT_TEST(lexer, stream_operators) {
    const std::string input = ">> >! >>> << ?>> ?>>> &>> &&>>";
    const std::vector<std::pair<Kind, std::string>> expected = {
        {ulang::token::STREAM_OUT, ">>"},
        {ulang::token::STREAM_OVERWRITE, ">!"},
        {ulang::token::STREAM_VAR, ">>>"},
        {ulang::token::STREAM_IN, "<<"},
        {ulang::token::STREAM_ERR, "?>>"},
        {ulang::token::STREAM_ERR_VAR, "?>>>"},
        {ulang::token::STREAM_ASYNC, "&>>"},
        {ulang::token::STREAM_THREAD, "&&>>"},
        {ulang::token::EOF_TOKEN, ""},
    };
    Lexer lexer(input);
    for (const auto& item : expected) {
        const Token tok = lexer.nextToken();
        MT_CHECK_EQ(tok.type, item.first);
        MT_CHECK_EQ(tok.literal, item.second);
    }
}

MT_TEST(lexer, keywords_and_types) {
    const std::string input =
        "func return if else elif while for loop foreach in break continue and or "
        "not true false null struct pub use as ba threads wait watch test mock "
        "assert tx match case err import string int float bool list map blob "
        "secret json";
    const std::vector<std::pair<Kind, std::string>> expected = {
        {ulang::token::FUNC, "func"},
        {ulang::token::RETURN, "return"},
        {ulang::token::IF, "if"},
        {ulang::token::ELSE, "else"},
        {ulang::token::ELIF, "elif"},
        {ulang::token::WHILE, "while"},
        {ulang::token::FOR, "for"},
        {ulang::token::LOOP, "loop"},
        {ulang::token::FOREACH, "foreach"},
        {ulang::token::IN, "in"},
        {ulang::token::BREAK, "break"},
        {ulang::token::CONTINUE, "continue"},
        {ulang::token::AND, "and"},
        {ulang::token::OR, "or"},
        {ulang::token::NOT, "not"},
        {ulang::token::TRUE, "true"},
        {ulang::token::FALSE, "false"},
        {ulang::token::NULL_TOKEN, "null"},
        {ulang::token::STRUCT, "struct"},
        {ulang::token::PUB, "pub"},
        {ulang::token::USE, "use"},
        {ulang::token::AS, "as"},
        {ulang::token::BA, "ba"},
        {ulang::token::THREADS, "threads"},
        {ulang::token::WAIT, "wait"},
        {ulang::token::WATCH, "watch"},
        {ulang::token::TEST, "test"},
        {ulang::token::MOCK, "mock"},
        {ulang::token::ASSERT, "assert"},
        {ulang::token::TX, "tx"},
        {ulang::token::MATCH, "match"},
        {ulang::token::CASE, "case"},
        {ulang::token::ERR, "err"},
        {ulang::token::IMPORT, "import"},
        {ulang::token::TYPE_STRING, "string"},
        {ulang::token::TYPE_INT, "int"},
        {ulang::token::TYPE_FLOAT, "float"},
        {ulang::token::TYPE_BOOL, "bool"},
        {ulang::token::TYPE_LIST, "list"},
        {ulang::token::TYPE_MAP, "map"},
        {ulang::token::TYPE_BLOB, "blob"},
        {ulang::token::TYPE_SECRET, "secret"},
        {ulang::token::TYPE_JSON, "json"},
        {ulang::token::EOF_TOKEN, ""},
    };
    Lexer lexer(input);
    for (const auto& item : expected) {
        const Token tok = lexer.nextToken();
        MT_CHECK_EQ(tok.type, item.first);
        MT_CHECK_EQ(tok.literal, item.second);
    }
}

MT_TEST(lexer, numbers_and_suffixes) {
    const std::string input =
        "42 3.14 100ms 5s 10m 2h 500KB 16MB 2GB 0.5s 1.5MB 1..10";
    const std::vector<std::pair<Kind, std::string>> expected = {
        {ulang::token::INT, "42"},     {ulang::token::FLOAT, "3.14"},
        {ulang::token::INT, "100ms"},  {ulang::token::INT, "5s"},
        {ulang::token::INT, "10m"},    {ulang::token::INT, "2h"},
        {ulang::token::INT, "500KB"},  {ulang::token::INT, "16MB"},
        {ulang::token::INT, "2GB"},    {ulang::token::FLOAT, "0.5s"},
        {ulang::token::FLOAT, "1.5MB"},{ulang::token::INT, "1"},
        {ulang::token::RANGE, ".."},   {ulang::token::INT, "10"},
        {ulang::token::EOF_TOKEN, ""},
    };
    Lexer lexer(input);
    for (const auto& item : expected) {
        const Token tok = lexer.nextToken();
        MT_CHECK_EQ(tok.type, item.first);
        MT_CHECK_EQ(tok.literal, item.second);
    }
}

MT_TEST(lexer, strings_and_interpolation) {
    const std::string input =
        "\"hello world\" \"with\\nnewline\" \"hello ${name}!\" 'single quote'";
    const std::vector<std::pair<Kind, std::string>> expected = {
        {ulang::token::STRING, "hello world"},
        {ulang::token::STRING, "with\nnewline"},
        {ulang::token::STRING, "hello ${name}!"},
        {ulang::token::STRING, "single quote"},
        {ulang::token::EOF_TOKEN, ""},
    };
    Lexer lexer(input);
    for (const auto& item : expected) {
        const Token tok = lexer.nextToken();
        MT_CHECK_EQ(tok.type, item.first);
        MT_CHECK_EQ(tok.literal, item.second);
    }
}

MT_TEST(lexer, indentation_and_dedent) {
    const std::string input = "if true {\n    x = 1\n    y = 2\n}\nz = 3";
    expectTypes(input, {
        ulang::token::IF, ulang::token::TRUE, ulang::token::LBRACE,
        ulang::token::NEWLINE, ulang::token::INDENT, ulang::token::IDENT,
        ulang::token::ASSIGN, ulang::token::INT, ulang::token::NEWLINE,
        ulang::token::IDENT, ulang::token::ASSIGN, ulang::token::INT,
        ulang::token::NEWLINE, ulang::token::DEDENT, ulang::token::RBRACE,
        ulang::token::NEWLINE, ulang::token::IDENT, ulang::token::ASSIGN,
        ulang::token::INT, ulang::token::EOF_TOKEN,
    });
}

MT_TEST(lexer, nested_indentation_and_eof_dedent) {
    const std::string input = "if a {\n    if b {\n        c = 1    \n    }\n}\n";
    expectTypes(input, {
        ulang::token::IF, ulang::token::IDENT, ulang::token::LBRACE,
        ulang::token::NEWLINE, ulang::token::INDENT, ulang::token::IF,
        ulang::token::IDENT, ulang::token::LBRACE, ulang::token::NEWLINE,
        ulang::token::INDENT, ulang::token::IDENT, ulang::token::ASSIGN,
        ulang::token::INT, ulang::token::NEWLINE, ulang::token::DEDENT,
        ulang::token::RBRACE, ulang::token::NEWLINE, ulang::token::DEDENT,
        ulang::token::RBRACE, ulang::token::NEWLINE, ulang::token::EOF_TOKEN,
    });
}

MT_TEST(lexer, comments_and_empty_lines) {
    const std::string input = R"(// Initial comment
fn myFunc() {
    // Comment inside function

    x = 10 // trailing comment
    /* multi
       line
       comment */
    return x
}
)";
    expectTypes(input, {
        ulang::token::FUNC, ulang::token::IDENT, ulang::token::LPAREN,
        ulang::token::RPAREN, ulang::token::LBRACE, ulang::token::NEWLINE,
        ulang::token::INDENT, ulang::token::IDENT, ulang::token::ASSIGN,
        ulang::token::INT, ulang::token::NEWLINE, ulang::token::RETURN,
        ulang::token::IDENT, ulang::token::NEWLINE, ulang::token::DEDENT,
        ulang::token::RBRACE, ulang::token::NEWLINE, ulang::token::EOF_TOKEN,
    });
}

MT_TEST(lexer, windows_line_endings) {
    const std::string input = "x = 1\r\ny = 2\r\n";
    expectTypes(input, {
        ulang::token::IDENT, ulang::token::ASSIGN, ulang::token::INT,
        ulang::token::NEWLINE, ulang::token::IDENT, ulang::token::ASSIGN,
        ulang::token::INT, ulang::token::NEWLINE, ulang::token::EOF_TOKEN,
    });
}

MT_TEST(lexer, tab_indentation) {
    const std::string input = "if true {\n\tx = 1\n\ty = 2\n}\nz = 3";
    expectTypes(input, {
        ulang::token::IF, ulang::token::TRUE, ulang::token::LBRACE,
        ulang::token::NEWLINE, ulang::token::INDENT, ulang::token::IDENT,
        ulang::token::ASSIGN, ulang::token::INT, ulang::token::NEWLINE,
        ulang::token::IDENT, ulang::token::ASSIGN, ulang::token::INT,
        ulang::token::NEWLINE, ulang::token::DEDENT, ulang::token::RBRACE,
        ulang::token::NEWLINE, ulang::token::IDENT, ulang::token::ASSIGN,
        ulang::token::INT, ulang::token::EOF_TOKEN,
    });
}

MT_TEST(lexer, line_and_column_positions) {
    const std::string input = "a = 10\nb = 20";
    Lexer lexer(input);

    Token tok = lexer.nextToken();
    MT_CHECK_EQ(tok.literal, std::string("a"));
    MT_CHECK_EQ(tok.line, 1);
    MT_CHECK_EQ(tok.column, 1);

    tok = lexer.nextToken();
    MT_CHECK_EQ(tok.literal, std::string("="));
    MT_CHECK_EQ(tok.line, 1);
    MT_CHECK_EQ(tok.column, 3);

    tok = lexer.nextToken();
    MT_CHECK_EQ(tok.literal, std::string("10"));
    MT_CHECK_EQ(tok.line, 1);
    MT_CHECK_EQ(tok.column, 5);

    tok = lexer.nextToken();
    MT_CHECK_EQ(tok.type, ulang::token::NEWLINE);
    MT_CHECK_EQ(tok.line, 1);
    MT_CHECK_EQ(tok.column, 7);

    tok = lexer.nextToken();
    MT_CHECK_EQ(tok.literal, std::string("b"));
    MT_CHECK_EQ(tok.line, 2);
    MT_CHECK_EQ(tok.column, 1);
}

MT_TEST(lexer, float_dot_prefix) {
    Lexer lexer(".5 .75s");
    Token tok = lexer.nextToken();
    MT_CHECK_EQ(tok.type, ulang::token::FLOAT);
    MT_CHECK_EQ(tok.literal, std::string(".5"));
    tok = lexer.nextToken();
    MT_CHECK_EQ(tok.type, ulang::token::FLOAT);
    MT_CHECK_EQ(tok.literal, std::string(".75s"));
}

MT_TEST(lexer, triple_quoted_strings) {
    const std::string input = "html = \"\"\"\n<html>\n    <button onclick=\"alert('hi')\">Click</button>\n</html>\n\"\"\"";
    Lexer lexer(input);
    MT_CHECK_EQ(lexer.nextToken().literal, std::string("html"));
    MT_CHECK_EQ(lexer.nextToken().type, ulang::token::ASSIGN);
    const Token tok = lexer.nextToken();
    MT_CHECK_EQ(tok.type, ulang::token::STRING);
    MT_CHECK_EQ(tok.literal,
                std::string("\n<html>\n    <button onclick=\"alert('hi')\">Click</button>\n</html>\n"));
}

MT_TEST(lexer, raw_backtick_strings) {
    const std::string input = "pat = `\\b\\d{1,3}\\.\\d{1,3}\\b`";
    Lexer lexer(input);
    MT_CHECK_EQ(lexer.nextToken().literal, std::string("pat"));
    MT_CHECK_EQ(lexer.nextToken().type, ulang::token::ASSIGN);
    const Token tok = lexer.nextToken();
    MT_CHECK_EQ(tok.type, ulang::token::STRING);
    MT_CHECK_EQ(tok.literal, std::string("\\b\\d{1,3}\\.\\d{1,3}\\b"));
}

MT_TEST(lexer, multiline_raw_and_fstring_tokens) {
    struct Case {
        const char* name;
        std::string input;
        Kind wantType;
        std::string wantLiteral;
    };
    const std::vector<Case> cases = {
        {"triple single quote", "'''line1\nline2'''", ulang::token::STRING,
         "line1\nline2"},
        {"raw triple double quotes", R"(r"""hello\nworld""")",
         ulang::token::STRING, R"(hello\nworld)"},
        {"raw triple single quotes", R"(r'''hello\tworld''')",
         ulang::token::STRING, R"(hello\tworld)"},
        {"raw single quote string", R"(r'C:\Users\foo\bar')",
         ulang::token::STRING, R"(C:\Users\foo\bar)"},
        {"raw double quote string", R"(r"C:\Windows\System32")",
         ulang::token::STRING, R"(C:\Windows\System32)"},
        {"fstring single quotes", R"(f"val = {x}")", ulang::token::FSTRING,
         R"(val = {x})"},
        {"fstring triple quotes", "f\"\"\"hello {name}\nworld\"\"\"",
         ulang::token::FSTRING, "hello {name}\nworld"},
        {"raw fstring rf", R"(rf"path: {p}\n")", ulang::token::FSTRING,
         R"(path: {p}\n)"},
        {"raw fstring fr", R"(fr"path: {p}\t")", ulang::token::FSTRING,
         R"(path: {p}\t)"},
    };

    for (const auto& tc : cases) {
        Lexer lexer(tc.input);
        const Token tok = lexer.nextToken();
        MT_CHECK_EQ(tok.type, tc.wantType);
        MT_CHECK_EQ(tok.literal, tc.wantLiteral);
    }
}

MT_TEST(lexer, bom_is_skipped) {
    const std::string input = "\xEF\xBB\xBFx = 1";
    expectTypes(input, {
        ulang::token::IDENT, ulang::token::ASSIGN, ulang::token::INT,
        ulang::token::EOF_TOKEN,
    });
}

MT_TEST(lexer, illegal_operators) {
    expectTypes("&& ?", {
        ulang::token::ILLEGAL, ulang::token::ILLEGAL, ulang::token::ILLEGAL,
        ulang::token::EOF_TOKEN,
    });
}

MT_TEST(lexer, range_inclusive) {
    Lexer lexer("1..=10");
    MT_CHECK_EQ(lexer.nextToken().literal, std::string("1"));
    MT_CHECK_EQ(lexer.nextToken().type, ulang::token::DOTDOTEQ);
    MT_CHECK_EQ(lexer.nextToken().literal, std::string("10"));
}
