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

using pawl::lexer::Lexer;
using pawl::token::Kind;
using pawl::token::Token;

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
        {pawl::token::ASSIGN, "="},   {pawl::token::PLUS, "+"},
        {pawl::token::MINUS, "-"},    {pawl::token::BANG, "!"},
        {pawl::token::ASTERISK, "*"}, {pawl::token::SLASH, "/"},
        {pawl::token::PERCENT, "%"},  {pawl::token::POWER, "**"},
        {pawl::token::EQ, "=="},      {pawl::token::STRICT_EQ, "==="},
        {pawl::token::NOT_EQ, "!="},  {pawl::token::LT, "<"},
        {pawl::token::LT_EQ, "<="},   {pawl::token::GT, ">"},
        {pawl::token::GT_EQ, ">="},   {pawl::token::COMMA, ","},
        {pawl::token::COLON, ":"},    {pawl::token::NEWLINE, "\n"},
        {pawl::token::DOT, "."},      {pawl::token::LPAREN, "("},
        {pawl::token::RPAREN, ")"},   {pawl::token::LBRACKET, "["},
        {pawl::token::RBRACKET, "]"}, {pawl::token::LBRACE, "{"},
        {pawl::token::RBRACE, "}"},   {pawl::token::PIPE, "|>"},
        {pawl::token::PIPE_TYPE, "|"},{pawl::token::ARROW, "->"},
        {pawl::token::TILDE, "~"},    {pawl::token::TILDE_GT, "~>"},
        {pawl::token::AT, "@"},       {pawl::token::RANGE, ".."},
        {pawl::token::EOF_TOKEN, ""},
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
        {pawl::token::STREAM_OUT, ">>"},
        {pawl::token::STREAM_OVERWRITE, ">!"},
        {pawl::token::STREAM_VAR, ">>>"},
        {pawl::token::STREAM_IN, "<<"},
        {pawl::token::STREAM_ERR, "?>>"},
        {pawl::token::STREAM_ERR_VAR, "?>>>"},
        {pawl::token::STREAM_ASYNC, "&>>"},
        {pawl::token::STREAM_THREAD, "&&>>"},
        {pawl::token::EOF_TOKEN, ""},
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
        {pawl::token::FUNC, "func"},
        {pawl::token::RETURN, "return"},
        {pawl::token::IF, "if"},
        {pawl::token::ELSE, "else"},
        {pawl::token::ELIF, "elif"},
        {pawl::token::WHILE, "while"},
        {pawl::token::FOR, "for"},
        {pawl::token::LOOP, "loop"},
        {pawl::token::FOREACH, "foreach"},
        {pawl::token::IN, "in"},
        {pawl::token::BREAK, "break"},
        {pawl::token::CONTINUE, "continue"},
        {pawl::token::AND, "and"},
        {pawl::token::OR, "or"},
        {pawl::token::NOT, "not"},
        {pawl::token::TRUE, "true"},
        {pawl::token::FALSE, "false"},
        {pawl::token::NULL_TOKEN, "null"},
        {pawl::token::STRUCT, "struct"},
        {pawl::token::PUB, "pub"},
        {pawl::token::USE, "use"},
        {pawl::token::AS, "as"},
        {pawl::token::BA, "ba"},
        {pawl::token::THREADS, "threads"},
        {pawl::token::WAIT, "wait"},
        {pawl::token::WATCH, "watch"},
        {pawl::token::TEST, "test"},
        {pawl::token::MOCK, "mock"},
        {pawl::token::ASSERT, "assert"},
        {pawl::token::TX, "tx"},
        {pawl::token::MATCH, "match"},
        {pawl::token::CASE, "case"},
        {pawl::token::ERR, "err"},
        {pawl::token::IMPORT, "import"},
        {pawl::token::TYPE_STRING, "string"},
        {pawl::token::TYPE_INT, "int"},
        {pawl::token::TYPE_FLOAT, "float"},
        {pawl::token::TYPE_BOOL, "bool"},
        {pawl::token::TYPE_LIST, "list"},
        {pawl::token::TYPE_MAP, "map"},
        {pawl::token::TYPE_BLOB, "blob"},
        {pawl::token::TYPE_SECRET, "secret"},
        {pawl::token::TYPE_JSON, "json"},
        {pawl::token::EOF_TOKEN, ""},
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
        {pawl::token::INT, "42"},     {pawl::token::FLOAT, "3.14"},
        {pawl::token::INT, "100ms"},  {pawl::token::INT, "5s"},
        {pawl::token::INT, "10m"},    {pawl::token::INT, "2h"},
        {pawl::token::INT, "500KB"},  {pawl::token::INT, "16MB"},
        {pawl::token::INT, "2GB"},    {pawl::token::FLOAT, "0.5s"},
        {pawl::token::FLOAT, "1.5MB"},{pawl::token::INT, "1"},
        {pawl::token::RANGE, ".."},   {pawl::token::INT, "10"},
        {pawl::token::EOF_TOKEN, ""},
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
        {pawl::token::STRING, "hello world"},
        {pawl::token::STRING, "with\nnewline"},
        {pawl::token::STRING, "hello ${name}!"},
        {pawl::token::STRING, "single quote"},
        {pawl::token::EOF_TOKEN, ""},
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
        pawl::token::IF, pawl::token::TRUE, pawl::token::LBRACE,
        pawl::token::NEWLINE, pawl::token::INDENT, pawl::token::IDENT,
        pawl::token::ASSIGN, pawl::token::INT, pawl::token::NEWLINE,
        pawl::token::IDENT, pawl::token::ASSIGN, pawl::token::INT,
        pawl::token::NEWLINE, pawl::token::DEDENT, pawl::token::RBRACE,
        pawl::token::NEWLINE, pawl::token::IDENT, pawl::token::ASSIGN,
        pawl::token::INT, pawl::token::EOF_TOKEN,
    });
}

MT_TEST(lexer, nested_indentation_and_eof_dedent) {
    const std::string input = "if a {\n    if b {\n        c = 1    \n    }\n}\n";
    expectTypes(input, {
        pawl::token::IF, pawl::token::IDENT, pawl::token::LBRACE,
        pawl::token::NEWLINE, pawl::token::INDENT, pawl::token::IF,
        pawl::token::IDENT, pawl::token::LBRACE, pawl::token::NEWLINE,
        pawl::token::INDENT, pawl::token::IDENT, pawl::token::ASSIGN,
        pawl::token::INT, pawl::token::NEWLINE, pawl::token::DEDENT,
        pawl::token::RBRACE, pawl::token::NEWLINE, pawl::token::DEDENT,
        pawl::token::RBRACE, pawl::token::NEWLINE, pawl::token::EOF_TOKEN,
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
        pawl::token::FUNC, pawl::token::IDENT, pawl::token::LPAREN,
        pawl::token::RPAREN, pawl::token::LBRACE, pawl::token::NEWLINE,
        pawl::token::INDENT, pawl::token::IDENT, pawl::token::ASSIGN,
        pawl::token::INT, pawl::token::NEWLINE, pawl::token::RETURN,
        pawl::token::IDENT, pawl::token::NEWLINE, pawl::token::DEDENT,
        pawl::token::RBRACE, pawl::token::NEWLINE, pawl::token::EOF_TOKEN,
    });
}

MT_TEST(lexer, windows_line_endings) {
    const std::string input = "x = 1\r\ny = 2\r\n";
    expectTypes(input, {
        pawl::token::IDENT, pawl::token::ASSIGN, pawl::token::INT,
        pawl::token::NEWLINE, pawl::token::IDENT, pawl::token::ASSIGN,
        pawl::token::INT, pawl::token::NEWLINE, pawl::token::EOF_TOKEN,
    });
}

MT_TEST(lexer, tab_indentation) {
    const std::string input = "if true {\n\tx = 1\n\ty = 2\n}\nz = 3";
    expectTypes(input, {
        pawl::token::IF, pawl::token::TRUE, pawl::token::LBRACE,
        pawl::token::NEWLINE, pawl::token::INDENT, pawl::token::IDENT,
        pawl::token::ASSIGN, pawl::token::INT, pawl::token::NEWLINE,
        pawl::token::IDENT, pawl::token::ASSIGN, pawl::token::INT,
        pawl::token::NEWLINE, pawl::token::DEDENT, pawl::token::RBRACE,
        pawl::token::NEWLINE, pawl::token::IDENT, pawl::token::ASSIGN,
        pawl::token::INT, pawl::token::EOF_TOKEN,
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
    MT_CHECK_EQ(tok.type, pawl::token::NEWLINE);
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
    MT_CHECK_EQ(tok.type, pawl::token::FLOAT);
    MT_CHECK_EQ(tok.literal, std::string(".5"));
    tok = lexer.nextToken();
    MT_CHECK_EQ(tok.type, pawl::token::FLOAT);
    MT_CHECK_EQ(tok.literal, std::string(".75s"));
}

MT_TEST(lexer, triple_quoted_strings) {
    const std::string input = "html = \"\"\"\n<html>\n    <button onclick=\"alert('hi')\">Click</button>\n</html>\n\"\"\"";
    Lexer lexer(input);
    MT_CHECK_EQ(lexer.nextToken().literal, std::string("html"));
    MT_CHECK_EQ(lexer.nextToken().type, pawl::token::ASSIGN);
    const Token tok = lexer.nextToken();
    MT_CHECK_EQ(tok.type, pawl::token::STRING);
    MT_CHECK_EQ(tok.literal,
                std::string("\n<html>\n    <button onclick=\"alert('hi')\">Click</button>\n</html>\n"));
}

MT_TEST(lexer, raw_backtick_strings) {
    const std::string input = "pat = `\\b\\d{1,3}\\.\\d{1,3}\\b`";
    Lexer lexer(input);
    MT_CHECK_EQ(lexer.nextToken().literal, std::string("pat"));
    MT_CHECK_EQ(lexer.nextToken().type, pawl::token::ASSIGN);
    const Token tok = lexer.nextToken();
    MT_CHECK_EQ(tok.type, pawl::token::STRING);
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
        {"triple single quote", "'''line1\nline2'''", pawl::token::STRING,
         "line1\nline2"},
        {"raw triple double quotes", R"(r"""hello\nworld""")",
         pawl::token::STRING, R"(hello\nworld)"},
        {"raw triple single quotes", R"(r'''hello\tworld''')",
         pawl::token::STRING, R"(hello\tworld)"},
        {"raw single quote string", R"(r'C:\Users\foo\bar')",
         pawl::token::STRING, R"(C:\Users\foo\bar)"},
        {"raw double quote string", R"(r"C:\Windows\System32")",
         pawl::token::STRING, R"(C:\Windows\System32)"},
        {"fstring single quotes", R"(f"val = {x}")", pawl::token::FSTRING,
         R"(val = {x})"},
        {"fstring triple quotes", "f\"\"\"hello {name}\nworld\"\"\"",
         pawl::token::FSTRING, "hello {name}\nworld"},
        {"raw fstring rf", R"(rf"path: {p}\n")", pawl::token::FSTRING,
         R"(path: {p}\n)"},
        {"raw fstring fr", R"(fr"path: {p}\t")", pawl::token::FSTRING,
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
        pawl::token::IDENT, pawl::token::ASSIGN, pawl::token::INT,
        pawl::token::EOF_TOKEN,
    });
}

MT_TEST(lexer, illegal_operators) {
    expectTypes("&& ?", {
        pawl::token::ILLEGAL, pawl::token::ILLEGAL, pawl::token::ILLEGAL,
        pawl::token::EOF_TOKEN,
    });
}

MT_TEST(lexer, range_inclusive) {
    Lexer lexer("1..=10");
    MT_CHECK_EQ(lexer.nextToken().literal, std::string("1"));
    MT_CHECK_EQ(lexer.nextToken().type, pawl::token::DOTDOTEQ);
    MT_CHECK_EQ(lexer.nextToken().literal, std::string("10"));
}
