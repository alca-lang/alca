// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0
//
// CLI scaffolding: lex, parse, version.

#include <cstdio>
#include <fstream>
#include <iostream>
#include <iterator>
#include <string>
#include <string_view>
#include <vector>

#include "ast/ast.h"
#include "codegen/x64/codegen.h"
#include "diag/diagnostic.h"
#include "ir/ir.h"
#include "ir/lower.h"
#include "lexer/lexer.h"
#include "link/elf/elf_writer.h"
#include "link/pe/pe_writer.h"
#include "parser/parser.h"
#include "token/token.h"
#include "typecheck/typecheck.h"

namespace {

constexpr std::string_view kVersion = "0.1.0-native";

int usage() {
    std::cerr
        << "usage: pawl <command> [options]\n"
        << "\n"
        << "commands:\n"
        << "  build [-o <out>] [--target <t>] <file> compile a .al file directly to native binary\n"
        << "  run <file>                            compile and run a .al file\n"
        << "  lex [--dump] <file>                   tokenize a .al file\n"
        << "  parse <file>                          parse and print AST of a .al file\n"
        << "  check <file>                          typecheck a .al file\n"
        << "  ir <file>                             emit Pawl IR for a .al file\n"
        << "  version                               print compiler version\n";
    return 2;
}

bool readFile(const std::string& path, std::string& out) {
    std::ifstream in(path, std::ios::binary);
    if (!in) {
        return false;
    }
    out.assign(std::istreambuf_iterator<char>(in),
               std::istreambuf_iterator<char>());
    return true;
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

void dumpTokens(const std::vector<pawl::token::Token>& tokens) {
    for (const auto& tok : tokens) {
        std::cout << pawl::token::kindName(tok.type) << '\t' << tok.line << '\t'
                  << tok.column << '\t' << hexEncode(tok.literal) << '\n';
    }
}

void printTokens(const std::vector<pawl::token::Token>& tokens) {
    for (const auto& tok : tokens) {
        std::cout << tok.toString() << '\n';
    }
}

int handleLex(const std::vector<std::string>& args) {
    bool dump = false;
    std::string path;
    for (std::size_t i = 1; i < args.size(); ++i) {
        if (args[i] == "--dump") {
            dump = true;
        } else if (args[i] == "-h" || args[i] == "--help") {
            usage();
            return 0;
        } else if (!args[i].empty() && args[i][0] == '-' && args[i] != "-") {
            std::cerr << "pawl lex: unknown option '" << args[i] << "'\n";
            return 2;
        } else if (path.empty()) {
            path = args[i];
        } else {
            std::cerr << "pawl lex: expected exactly one input file\n";
            return 2;
        }
    }
    if (path.empty()) {
        return usage();
    }

    std::string source;
    if (!readFile(path, source)) {
        std::cerr << "pawl lex: cannot read '" << path << "'\n";
        return 1;
    }

    pawl::lexer::Lexer lexer(std::move(source));
    const auto tokens = lexer.tokenize();
    if (dump) {
        dumpTokens(tokens);
    } else {
        printTokens(tokens);
    }
    return 0;
}

int handleParse(const std::vector<std::string>& args) {
    std::string path;
    for (std::size_t i = 1; i < args.size(); ++i) {
        if (args[i] == "-h" || args[i] == "--help") {
            usage();
            return 0;
        } else if (!args[i].empty() && args[i][0] == '-' && args[i] != "-") {
            std::cerr << "pawl parse: unknown option '" << args[i] << "'\n";
            return 2;
        } else if (path.empty()) {
            path = args[i];
        } else {
            std::cerr << "pawl parse: expected exactly one input file\n";
            return 2;
        }
    }
    if (path.empty()) {
        return usage();
    }

    std::string source;
    if (!readFile(path, source)) {
        std::cerr << "pawl parse: cannot read '" << path << "'\n";
        return 1;
    }

    auto lex = std::make_unique<pawl::lexer::Lexer>(source);
    pawl::parser::Parser p(std::move(lex));
    auto prog = p.parseProgram();

    if (!p.syntaxErrors().empty()) {
        std::cerr << p.diagnosticReport(path, source, false);
        return 1;
    }

    if (prog) {
        std::cout << prog->toString();
    }
    return 0;
}

int handleCheck(const std::vector<std::string>& args) {
    std::string path;
    for (std::size_t i = 1; i < args.size(); ++i) {
        if (args[i] == "-h" || args[i] == "--help") {
            usage();
            return 0;
        } else if (!args[i].empty() && args[i][0] == '-' && args[i] != "-") {
            std::cerr << "pawl check: unknown option '" << args[i] << "'\n";
            return 2;
        } else if (path.empty()) {
            path = args[i];
        } else {
            std::cerr << "pawl check: expected exactly one input file\n";
            return 2;
        }
    }
    if (path.empty()) {
        return usage();
    }

    std::string source;
    if (!readFile(path, source)) {
        std::cerr << "pawl check: cannot read '" << path << "'\n";
        return 1;
    }

    auto lex = std::make_unique<pawl::lexer::Lexer>(source);
    pawl::parser::Parser p(std::move(lex));
    auto prog = p.parseProgram();

    if (!p.syntaxErrors().empty()) {
        std::cerr << p.diagnosticReport(path, source, false);
        return 1;
    }

    pawl::typecheck::Checker c(path, source);
    auto errs = c.check(prog);
    if (!errs.empty()) {
        std::cerr << pawl::diag::formatReport(path, source, errs, false);
        return 1;
    }

    std::cout << "OK: " << path << " passed typecheck\n";
    return 0;
}

int handleIR(const std::vector<std::string>& args) {
    std::string path;
    for (std::size_t i = 1; i < args.size(); ++i) {
        if (args[i] == "-h" || args[i] == "--help") {
            usage();
            return 0;
        } else if (!args[i].empty() && args[i][0] == '-' && args[i] != "-") {
            std::cerr << "pawl ir: unknown option '" << args[i] << "'\n";
            return 2;
        } else if (path.empty()) {
            path = args[i];
        } else {
            std::cerr << "pawl ir: expected exactly one input file\n";
            return 2;
        }
    }
    if (path.empty()) {
        return usage();
    }

    std::string source;
    if (!readFile(path, source)) {
        std::cerr << "pawl ir: cannot read '" << path << "'\n";
        return 1;
    }

    auto lex = std::make_unique<pawl::lexer::Lexer>(source);
    pawl::parser::Parser p(std::move(lex));
    auto prog = p.parseProgram();

    if (!p.syntaxErrors().empty()) {
        std::cerr << p.diagnosticReport(path, source, false);
        return 1;
    }

    pawl::typecheck::Checker c(path, source);
    auto errs = c.check(prog);
    if (!errs.empty()) {
        std::cerr << pawl::diag::formatReport(path, source, errs, false);
        return 1;
    }

    auto irRes = pawl::ir::lower(prog, c);
    if (!irRes.ok()) {
        std::cerr << "pawl ir error: " << irRes.error << '\n';
        return 1;
    }

    std::cout << irRes.program->toString();
    return 0;
}

int handleBuild(const std::vector<std::string>& args) {
    std::string path;
    std::string outPath;
    std::string target = "windows/amd64";
#ifndef _WIN32
    target = "linux/amd64";
#endif

    for (std::size_t i = 1; i < args.size(); ++i) {
        if (args[i] == "-h" || args[i] == "--help") {
            usage();
            return 0;
        } else if (args[i] == "-o" && i + 1 < args.size()) {
            outPath = args[++i];
        } else if (args[i] == "--target" && i + 1 < args.size()) {
            target = args[++i];
        } else if (!args[i].empty() && args[i][0] == '-' && args[i] != "-") {
            std::cerr << "pawl build: unknown option '" << args[i] << "'\n";
            return 2;
        } else if (path.empty()) {
            path = args[i];
        } else {
            std::cerr << "pawl build: expected exactly one input file\n";
            return 2;
        }
    }
    if (path.empty()) {
        return usage();
    }

    if (outPath.empty()) {
        std::string base = path;
        size_t lastDot = base.find_last_of('.');
        if (lastDot != std::string::npos) {
            base = base.substr(0, lastDot);
        }
        if (target.find("windows") != std::string::npos) {
            outPath = base + ".exe";
        } else {
            outPath = base;
        }
    }

    std::string source;
    if (!readFile(path, source)) {
        std::cerr << "pawl build: cannot read '" << path << "'\n";
        return 1;
    }

    auto lex = std::make_unique<pawl::lexer::Lexer>(source);
    pawl::parser::Parser p(std::move(lex));
    auto prog = p.parseProgram();
    if (!p.syntaxErrors().empty()) {
        std::cerr << p.diagnosticReport(path, source, false);
        return 1;
    }

    pawl::typecheck::Checker c(path, source);
    auto errs = c.check(prog);
    if (!errs.empty()) {
        std::cerr << pawl::diag::formatReport(path, source, errs, false);
        return 1;
    }

    auto irRes = pawl::ir::lower(prog, c);
    if (!irRes.ok()) {
        std::cerr << "pawl build error: " << irRes.error << '\n';
        return 1;
    }

    bool isWin = (target.find("windows") != std::string::npos);
    pawl::codegen::x64::ABI abi = isWin ? pawl::codegen::x64::ABI::Win64 : pawl::codegen::x64::ABI::SysV;
    pawl::codegen::x64::CodeGenerator cg(abi);
    auto art = cg.generate(*irRes.program);

    bool ok = false;
    if (isWin) {
        ok = pawl::link::pe::linkPE(art, outPath, false);
    } else {
        ok = pawl::link::elf::linkELF(art, outPath);
    }

    if (!ok) {
        std::cerr << "pawl build error: failed to write output file '" << outPath << "'\n";
        return 1;
    }

    std::cout << "[OK] compiled " << path << " -> " << outPath << '\n';
    return 0;
}

int handleRun(const std::vector<std::string>& args) {
    if (args.size() < 2) {
        return usage();
    }
    std::string path = args[1];
    std::string tmpExe = "temp_pawl_run.exe";
#ifndef _WIN32
    tmpExe = "./temp_pawl_run";
#endif

    std::vector<std::string> buildArgs = {"build", "-o", tmpExe, path};
    int bRes = handleBuild(buildArgs);
    if (bRes != 0) {
        return bRes;
    }

    int runCode = std::system(tmpExe.c_str());
    std::remove(tmpExe.c_str());
    return runCode;
}

} // namespace

int main(int argc, char** argv) {
    std::vector<std::string> args(argv + 1, argv + argc);
    if (args.empty()) {
        return usage();
    }

    const std::string command = args[0];
    if (command == "version" || command == "--version") {
        std::cout << "pawl " << kVersion << '\n';
        return 0;
    }
    if (command == "-h" || command == "--help" || command == "help") {
        usage();
        return 0;
    }
    if (command == "build") {
        return handleBuild(args);
    }
    if (command == "run") {
        return handleRun(args);
    }
    if (command == "lex") {
        return handleLex(args);
    }
    if (command == "parse") {
        return handleParse(args);
    }
    if (command == "check") {
        return handleCheck(args);
    }
    if (command == "ir") {
        return handleIR(args);
    }

    std::cerr << "pawl: unknown command '" << command << "'\n";
    return usage();
}
