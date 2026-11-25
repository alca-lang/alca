// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

#include "diag/diagnostic.h"

#include <algorithm>
#include <cctype>
#include <sstream>
#include <unordered_set>

namespace ulang::diag {

namespace {

const char* colorReset     = "\033[0m";
const char* colorBold      = "\033[1m";
const char* colorBoldRed   = "\033[1;31m";
const char* colorBoldGreen = "\033[1;32m";
const char* colorBoldCyan  = "\033[1;36m";
const char* colorGray      = "\033[90m";

std::string toLower(std::string s) {
    for (char& c : s) c = static_cast<char>(std::tolower(static_cast<unsigned char>(c)));
    return s;
}

std::vector<std::string> splitLines(const std::string& text) {
    std::vector<std::string> lines;
    std::string line;
    for (char c : text) {
        if (c == '\r') continue;
        if (c == '\n') {
            lines.push_back(line);
            line.clear();
        } else {
            line.push_back(c);
        }
    }
    lines.push_back(line);
    return lines;
}

std::string formatSingleError(const std::string& filename, const std::vector<std::string>& lines,
                              const std::shared_ptr<SyntaxError>& err, bool useColor) {
    const char* cRed   = useColor ? colorBoldRed   : "";
    const char* cCyan  = useColor ? colorBoldCyan  : "";
    const char* cGray  = useColor ? colorGray      : "";
    const char* cGreen = useColor ? colorBoldGreen : "";
    const char* cReset = useColor ? colorReset     : "";
    const char* cBold  = useColor ? colorBold      : "";

    std::ostringstream b;

    b << cRed << "error[SyntaxError]: " << cReset << cBold << err->message << cReset << "\n";
    b << "  " << cCyan << "-->" << cReset << " " << cGray << filename << ":" << err->line << ":" << err->column << cReset << "\n";

    int lineIdx = err->line - 1;
    if (lineIdx >= 0 && lineIdx < static_cast<int>(lines.size())) {
        std::string lineStr = std::to_string(err->line);
        size_t pad = lineStr.length();
        std::string emptyPad(pad, ' ');

        b << "  " << emptyPad << " " << cCyan << "|" << cReset << "\n";

        std::string sourceLine = lines[lineIdx];
        b << "  " << cCyan << lineStr << " |" << cReset << " " << sourceLine << "\n";

        b << "  " << emptyPad << " " << cCyan << "|" << cReset << " ";

        int col = err->column;
        if (col < 1) col = 1;
        int spaces = col - 1;
        if (spaces > static_cast<int>(sourceLine.length())) {
            spaces = static_cast<int>(sourceLine.length());
        }
        b << std::string(spaces, ' ');

        int length = err->length;
        if (length < 1) length = 1;
        b << cRed << std::string(length, '^') << cReset << "\n";

        std::string suggestion = err->suggestion;
        if (suggestion.empty()) {
            suggestion = generateSuggestion(err->message, err->token);
        }
        if (!suggestion.empty()) {
            b << "  " << emptyPad << " " << cCyan << "=" << cReset << " "
              << cGreen << "help:" << cReset << " " << suggestion << "\n";
        }
    }
    return b.str();
}

} // namespace

std::string SyntaxError::error() const {
    return filename + ":" + std::to_string(line) + ":" + std::to_string(column) + ": " + message;
}

std::string generateSuggestion(const std::string& msg, const std::string&) {
    std::string msgLower = toLower(msg);

    if (msgLower.find("no prefix parse function for :") != std::string::npos) {
        return "Unexpected ':' delimiter. In Pawl, use ':' only to end block headers (e.g. 'if cond:', 'func name():') or for variable type annotations ('x: int = 5').";
    }
    if (msgLower.find("no prefix parse function for =") != std::string::npos) {
        return "Unexpected '=' token. Check if you have an extra assignment operator or incomplete statement.";
    }
    if (msgLower.find("expected next token to be :") != std::string::npos) {
        return "Block statements ('if', 'while', 'for', 'func', 'struct', 'ba', 'threads') require a trailing ':' before the indented body.";
    }
    if (msgLower.find("no prefix parse function for indent") != std::string::npos) {
        return "Unexpected indentation. Ensure statements are aligned with 4 spaces or 1 tab per nesting level.";
    }
    if (msgLower.find("no prefix parse function for dedent") != std::string::npos) {
        return "Check block alignment. Indentation level decreased to a level that does not match any outer block.";
    }
    if (msgLower.find("expected next token to be )") != std::string::npos) {
        return "Unclosed parenthesis. Did you forget a closing ')'?";
    }
    if (msgLower.find("expected next token to be ]") != std::string::npos) {
        return "Unclosed bracket. Did you forget a closing ']'?";
    }
    if (msgLower.find("expected next token to be }") != std::string::npos) {
        return "Unclosed brace. Did you forget a closing '}'?";
    }
    if (msgLower.find("no prefix parse function for +") != std::string::npos ||
        msgLower.find("no prefix parse function for *") != std::string::npos) {
        return "Expression missing left-hand operand. Check for stray arithmetic operators.";
    }
    if (msgLower.find("unknown") != std::string::npos) {
        return "Verify the keyword spelling and variable names.";
    }
    return "";
}

std::string formatReport(const std::string& filename, const std::string& source,
                         const std::vector<std::shared_ptr<SyntaxError>>& errors,
                         bool useColor) {
    if (errors.empty()) return "";

    std::vector<std::shared_ptr<SyntaxError>> unique;
    std::unordered_set<std::string> seen;
    for (const auto& e : errors) {
        std::string key = std::to_string(e->line) + ":" + std::to_string(e->column) + ":" + e->message;
        if (seen.find(key) == seen.end()) {
            seen.insert(key);
            unique.push_back(e);
        }
    }

    auto lines = splitLines(source);
    std::ostringstream b;
    b << "\n";
    for (size_t i = 0; i < unique.size(); ++i) {
        b << formatSingleError(filename, lines, unique[i], useColor);
        if (i + 1 < unique.size()) {
            b << "\n";
        }
    }

    std::string errWord = (unique.size() != 1) ? "errors" : "error";
    std::string summary = "\nFound " + std::to_string(unique.size()) + " " + errWord + ". Execution aborted.\n";
    if (useColor) {
        summary = std::string(colorBoldRed) + summary + colorReset;
    }
    b << summary;
    return b.str();
}

} // namespace ulang::diag
