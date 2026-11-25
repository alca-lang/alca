// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0
//
// Differential harness: tokenizes the checked-in corpus with the C++ lexer and
// compares the result against golden dumps produced by the frozen Go oracle.
//
// Golden format (one token per line, LF endings):
//   <TOKEN-TYPE>\t<line>\t<column>\t<lowercase-hex-literal>
//
// The golden files live under compiler/tests/golden/<corpus-relative>.tokens
// and are regenerated with tools/oracle (migration-only Go tooling).

#include "lexer/lexer.h"
#include "microtest/microtest.h"
#include "token/token.h"

#include <algorithm>
#include <cstdlib>
#include <filesystem>
#include <fstream>
#include <sstream>
#include <string>
#include <vector>

namespace {

#ifndef PAWL_REPO_ROOT
#define PAWL_REPO_ROOT ""
#endif

std::string repoRoot() {
    if (const char* env = std::getenv("PAWL_REPO_ROOT")) {
        if (*env != '\0') {
            return env;
        }
    }
    if (PAWL_REPO_ROOT[0] != '\0') {
        return PAWL_REPO_ROOT;
    }

    // Fall back to walking up from the current working directory.  This keeps
    // the test runnable from an IDE or a plain shell without extra defines.
    std::error_code ec;
    std::filesystem::path dir = std::filesystem::current_path(ec);
    for (int i = 0; i < 8 && !ec; ++i) {
        if (std::filesystem::exists(dir / "compiler" / "tests" / "corpus.txt",
                                    ec)) {
            return dir.string();
        }
        if (std::filesystem::exists(dir / "tests" / "corpus.txt", ec) &&
            std::filesystem::exists(dir / "src" / "lexer", ec)) {
            return dir.parent_path().string();
        }
        if (!dir.has_parent_path() || dir.parent_path() == dir) {
            break;
        }
        dir = dir.parent_path();
    }
    return ".";
}

std::string readFile(const std::string& path, bool& ok) {
    std::ifstream in(path, std::ios::binary);
    if (!in) {
        ok = false;
        return {};
    }
    std::ostringstream ss;
    ss << in.rdbuf();
    ok = true;
    return ss.str();
}

std::string hexEncode(const std::string& bytes) {
    static constexpr char kHex[] = "0123456789abcdef";
    std::string out;
    out.reserve(bytes.size() * 2);
    for (unsigned char c : bytes) {
        out.push_back(kHex[c >> 4]);
        out.push_back(kHex[c & 0x0F]);
    }
    return out;
}

std::string dumpTokens(const std::vector<ulang::token::Token>& tokens) {
    std::ostringstream out;
    for (const auto& tok : tokens) {
        out << ulang::token::kindName(tok.type) << '\t' << tok.line << '\t'
            << tok.column << '\t' << hexEncode(tok.literal) << '\n';
    }
    return out.str();
}

std::vector<std::string> splitLines(std::string text) {
    if (!text.empty() && text.back() == '\n') {
        text.pop_back();
    }
    std::vector<std::string> lines;
    std::string current;
    for (char c : text) {
        if (c == '\n') {
            if (!current.empty() && current.back() == '\r') {
                current.pop_back();
            }
            lines.push_back(current);
            current.clear();
        } else {
            current.push_back(c);
        }
    }
    if (!current.empty()) {
        lines.push_back(current);
    }
    return lines;
}

std::string firstDifference(const std::vector<std::string>& actual,
                            const std::vector<std::string>& expected) {
    const std::size_t n = std::max(actual.size(), expected.size());
    for (std::size_t i = 0; i < n; ++i) {
        const std::string a = i < actual.size() ? actual[i] : "<missing>";
        const std::string e = i < expected.size() ? expected[i] : "<missing>";
        if (a != e) {
            std::ostringstream msg;
            msg << "line " << (i + 1) << "\n  expected: " << e
                << "\n  actual:   " << a;
            return msg.str();
        }
    }
    return {};
}

} // namespace

MT_TEST(differential, corpus_matches_go_oracle) {
    const std::string root = repoRoot();
    const std::string manifestPath =
        root + "/compiler/tests/corpus.txt";

    bool ok = false;
    const std::string manifest = readFile(manifestPath, ok);
    MT_CHECK(ok && "cannot read compiler/tests/corpus.txt");

    std::size_t checked = 0;
    std::istringstream lines(manifest);
    std::string relative;
    while (std::getline(lines, relative)) {
        if (!relative.empty() && relative.back() == '\r') {
            relative.pop_back();
        }
        if (relative.empty() || relative[0] == '#') {
            continue;
        }

        const std::string source =
            readFile(root + "/" + relative, ok);
        if (!ok) {
            MT_FAIL("cannot read corpus file " + relative);
        }
        const std::string golden = readFile(
            root + "/compiler/tests/golden/" + relative + ".tokens", ok);
        if (!ok) {
            MT_FAIL("missing golden file for " + relative);
        }

        ulang::lexer::Lexer lexer(source);
        const std::string actual = dumpTokens(lexer.tokenize());
        const auto actualLines = splitLines(actual);
        const auto expectedLines = splitLines(golden);
        const std::string diff = firstDifference(actualLines, expectedLines);
        if (!diff.empty()) {
            MT_FAIL("token mismatch in " + relative + "\n" + diff);
        }
        ++checked;
    }

    MT_CHECK(checked > 0 && "corpus manifest is empty");
}
