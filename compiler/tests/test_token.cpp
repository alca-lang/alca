// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0
//
// Port of pkg/token/token_test.go.

#include "microtest/microtest.h"
#include "token/token.h"

#include <algorithm>
#include <string>
#include <vector>

using namespace ulang;

MT_TEST(token, lookup_ident) {
    struct Item {
        const char* input;
        token::Kind expected;
    };
    const std::vector<Item> items = {
        {"func", token::FUNC},       {"return", token::RETURN},
        {"if", token::IF},           {"else", token::ELSE},
        {"elif", token::ELIF},       {"while", token::WHILE},
        {"for", token::FOR},         {"loop", token::LOOP},
        {"foreach", token::FOREACH}, {"in", token::IN},
        {"break", token::BREAK},     {"continue", token::CONTINUE},
        {"and", token::AND},         {"or", token::OR},
        {"not", token::NOT},         {"true", token::TRUE},
        {"false", token::FALSE},     {"null", token::NULL_TOKEN},
        {"struct", token::STRUCT},   {"pub", token::PUB},
        {"use", token::USE},         {"as", token::AS},
        {"ba", token::BA},           {"threads", token::THREADS},
        {"wait", token::WAIT},       {"watch", token::WATCH},
        {"test", token::TEST},       {"mock", token::MOCK},
        {"assert", token::ASSERT},   {"tx", token::TX},
        {"match", token::MATCH},     {"case", token::CASE},
        {"err", token::ERR},         {"import", token::IMPORT},
        {"string", token::TYPE_STRING}, {"int", token::TYPE_INT},
        {"float", token::TYPE_FLOAT},   {"bool", token::TYPE_BOOL},
        {"list", token::TYPE_LIST},     {"map", token::TYPE_MAP},
        {"blob", token::TYPE_BLOB},     {"secret", token::TYPE_SECRET},
        {"json", token::TYPE_JSON},
        {"myVar", token::IDENT},     {"calculate_total", token::IDENT},
        {"Foo123", token::IDENT},
    };
    for (const auto& item : items) {
        MT_CHECK_EQ(token::lookupIdent(item.input), item.expected);
    }
}

MT_TEST(token, is_keyword_and_type_keyword) {
    MT_CHECK(token::isKeyword(token::FUNC));
    MT_CHECK(token::isKeyword(token::MATCH));
    MT_CHECK(!token::isKeyword(token::IDENT));
    MT_CHECK(!token::isKeyword(token::TYPE_STRING));

    MT_CHECK(token::isTypeKeyword(token::TYPE_STRING));
    MT_CHECK(token::isTypeKeyword(token::TYPE_JSON));
    MT_CHECK(!token::isTypeKeyword(token::FUNC));
    MT_CHECK(!token::isTypeKeyword(token::IDENT));
}

MT_TEST(token, token_string) {
    token::Token tok;
    tok.type = token::IDENT;
    tok.literal = "foobar";
    tok.line = 12;
    tok.column = 5;
    MT_CHECK_EQ(tok.toString(), std::string("Token(IDENT, \"foobar\", 12:5)"));
}

MT_TEST(token, suffixes) {
    MT_CHECK_EQ(std::string(token::SuffixMS), std::string("ms"));
    MT_CHECK_EQ(std::string(token::SuffixS), std::string("s"));
    MT_CHECK_EQ(std::string(token::SuffixM), std::string("m"));
    MT_CHECK_EQ(std::string(token::SuffixH), std::string("h"));
    MT_CHECK_EQ(std::string(token::SuffixKB), std::string("KB"));
    MT_CHECK_EQ(std::string(token::SuffixMB), std::string("MB"));
    MT_CHECK_EQ(std::string(token::SuffixGB), std::string("GB"));
}

MT_TEST(token, keyword_names_sorted_and_split) {
    const auto names = token::keywordNames();
    MT_CHECK(!names.empty());
    MT_CHECK(std::is_sorted(names.begin(), names.end()));
    MT_CHECK(std::find(names.begin(), names.end(), "func") != names.end());
    MT_CHECK(std::find(names.begin(), names.end(), "string") == names.end());

    const auto types = token::typeKeywordNames();
    MT_CHECK(!types.empty());
    MT_CHECK(std::is_sorted(types.begin(), types.end()));
    MT_CHECK(std::find(types.begin(), types.end(), "string") != types.end());
    MT_CHECK(std::find(types.begin(), types.end(), "func") == types.end());
}
