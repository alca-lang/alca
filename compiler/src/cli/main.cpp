// Copyright 2026 ULang++ Programming Language
// SPDX-License-Identifier: Apache-2.0
//
// CLI scaffolding: lex, parse, version, build, run.

#include <cstdio>
#include <fstream>
#include <iostream>
#include <iterator>
#include <string>
#include <string_view>
#include <vector>
#include <unordered_set>

#include "ast/ast.h"
#include "codegen/x64/codegen.h"
#include "diag/diagnostic.h"
#include "ir/ir.h"
#include "ir/lower.h"
#include "lexer/lexer.h"
#include "link/elf/elf_writer.h"
#include "link/pe/pe_writer.h"
#include "link/pe/efi_writer.h"
#include "parser/parser.h"
#include "token/token.h"
#include "typecheck/typecheck.h"
#include "mod/mod.h"

namespace {

void loadImportedModules(std::shared_ptr<ulang::ast::Program> prog, const std::string& currentFilePath, std::unordered_set<std::string>& loaded) {
    if (!prog) return;
    std::vector<std::string> toLoad;
    for (const auto& stmt : prog->statements) {
        if (auto* u = dynamic_cast<ulang::ast::UseStatement*>(stmt.get())) {
            auto res = ulang::mod::resolve(u->path, currentFilePath);
            if (res && loaded.find(res->sourcePath) == loaded.end()) {
                toLoad.push_back(res->sourcePath);
                loaded.insert(res->sourcePath);
            }
        }
    }
    for (const auto& modFile : toLoad) {
        std::ifstream f(modFile);
        if (!f.is_open()) continue;
        std::string modSrc((std::istreambuf_iterator<char>(f)), std::istreambuf_iterator<char>());
        auto modLex = std::make_unique<ulang::lexer::Lexer>(modSrc);
        ulang::parser::Parser modP(std::move(modLex));
        auto modProg = modP.parseProgram();
        if (modProg && modP.syntaxErrors().empty()) {
            loadImportedModules(modProg, modFile, loaded);
            for (auto& s : modProg->statements) {
                prog->statements.push_back(std::move(s));
            }
        }
    }
}

constexpr std::string_view kVersion = "0.1.0-native (ULang++)";

int usage() {
    std::cerr
        << "usage: ulang++ <command> [options]\n"
        << "\n"
        << "commands:\n"
        << "  build [-o <out>] [--target <t>] [--arch x86_64|x86_32] <file>  compile a .upp file\n"
        << "  run <file>                            compile and run a .upp file\n"
        << "  lex [--dump] <file>                   tokenize a .upp file\n"
        << "  parse <file>                          parse and print AST of a .upp file\n"
        << "  check <file>                          typecheck a .upp file\n"
        << "  ir <file>                             emit ULang++ IR for a .upp file\n"
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

void dumpTokens(const std::vector<ulang::token::Token>& tokens) {
    for (const auto& tok : tokens) {
        std::cout << ulang::token::kindName(tok.type) << '\t' << tok.line << '\t'
                  << tok.column << '\t' << hexEncode(tok.literal) << '\n';
    }
}

void printTokens(const std::vector<ulang::token::Token>& tokens) {
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
            std::cerr << "ulang++ lex: unknown option '" << args[i] << "'\n";
            return 2;
        } else if (path.empty()) {
            path = args[i];
        } else {
            std::cerr << "ulang++ lex: expected exactly one input file\n";
            return 2;
        }
    }
    if (path.empty()) {
        return usage();
    }

    std::string source;
    if (!readFile(path, source)) {
        std::cerr << "ulang++ lex: cannot read '" << path << "'\n";
        return 1;
    }

    ulang::lexer::Lexer lexer(std::move(source));
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
            std::cerr << "ulang++ parse: unknown option '" << args[i] << "'\n";
            return 2;
        } else if (path.empty()) {
            path = args[i];
        } else {
            std::cerr << "ulang++ parse: expected exactly one input file\n";
            return 2;
        }
    }
    if (path.empty()) {
        return usage();
    }

    std::string source;
    if (!readFile(path, source)) {
        std::cerr << "ulang++ parse: cannot read '" << path << "'\n";
        return 1;
    }

    auto lex = std::make_unique<ulang::lexer::Lexer>(source);
    ulang::parser::Parser p(std::move(lex));
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
            std::cerr << "ulang++ check: unknown option '" << args[i] << "'\n";
            return 2;
        } else if (path.empty()) {
            path = args[i];
        } else {
            std::cerr << "ulang++ check: expected exactly one input file\n";
            return 2;
        }
    }
    if (path.empty()) {
        return usage();
    }

    std::string source;
    if (!readFile(path, source)) {
        std::cerr << "ulang++ check: cannot read '" << path << "'\n";
        return 1;
    }

    auto lex = std::make_unique<ulang::lexer::Lexer>(source);
    ulang::parser::Parser p(std::move(lex));
    auto prog = p.parseProgram();

    if (!p.syntaxErrors().empty()) {
        std::cerr << p.diagnosticReport(path, source, false);
        return 1;
    }

    std::unordered_set<std::string> loadedFiles;
    loadedFiles.insert(path);
    loadImportedModules(prog, path, loadedFiles);

    ulang::typecheck::Checker c(path, source);
    auto errs = c.check(prog);
    if (!errs.empty()) {
        std::cerr << ulang::diag::formatReport(path, source, errs, false);
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
            std::cerr << "ulang++ ir: unknown option '" << args[i] << "'\n";
            return 2;
        } else if (path.empty()) {
            path = args[i];
        } else {
            std::cerr << "ulang++ ir: expected exactly one input file\n";
            return 2;
        }
    }
    if (path.empty()) {
        return usage();
    }

    std::string source;
    if (!readFile(path, source)) {
        std::cerr << "ulang++ ir: cannot read '" << path << "'\n";
        return 1;
    }

    auto lex = std::make_unique<ulang::lexer::Lexer>(source);
    ulang::parser::Parser p(std::move(lex));
    auto prog = p.parseProgram();

    if (!p.syntaxErrors().empty()) {
        std::cerr << p.diagnosticReport(path, source, false);
        return 1;
    }

    std::unordered_set<std::string> loadedFiles;
    loadedFiles.insert(path);
    loadImportedModules(prog, path, loadedFiles);

    ulang::typecheck::Checker c(path, source);
    auto errs = c.check(prog);
    if (!errs.empty()) {
        std::cerr << ulang::diag::formatReport(path, source, errs, false);
        return 1;
    }

    auto irRes = ulang::ir::lower(prog, c);
    if (!irRes.ok()) {
        std::cerr << "ulang++ ir error: " << irRes.error << '\n';
        return 1;
    }

    std::cout << irRes.program->toString();
    return 0;
}

int handleBuild(const std::vector<std::string>& args) {
    std::string path;
    std::string outPath;
    std::string target = "windows/amd64";
    std::string arch = "x86_64";
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
        } else if (args[i] == "--arch" && i + 1 < args.size()) {
            arch = args[++i];
        } else if (!args[i].empty() && args[i][0] == '-' && args[i] != "-") {
            std::cerr << "ulang++ build: unknown option '" << args[i] << "'\n";
            return 2;
        } else if (path.empty()) {
            path = args[i];
        } else {
            std::cerr << "ulang++ build: expected exactly one input file\n";
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
        std::cerr << "ulang++ build: cannot read '" << path << "'\n";
        return 1;
    }

    auto lex = std::make_unique<ulang::lexer::Lexer>(source);
    ulang::parser::Parser p(std::move(lex));
    auto prog = p.parseProgram();
    if (!p.syntaxErrors().empty()) {
        std::cerr << p.diagnosticReport(path, source, false);
        return 1;
    }

    std::unordered_set<std::string> loadedFiles;
    loadedFiles.insert(path);
    loadImportedModules(prog, path, loadedFiles);

    ulang::typecheck::Checker c(path, source);
    auto errs = c.check(prog);
    if (!errs.empty()) {
        std::cerr << ulang::diag::formatReport(path, source, errs, false);
        return 1;
    }

    auto irRes = ulang::ir::lower(prog, c);
    if (!irRes.ok()) {
        std::cerr << "ulang++ build error: " << irRes.error << '\n';
        return 1;
    }

    bool isEfi = (target.find("efi") != std::string::npos || target.find("uefi") != std::string::npos);
    bool isWin = (target.find("windows") != std::string::npos);
    bool isFreestanding = (target.find("baremetal") != std::string::npos || target.find("freestanding") != std::string::npos || target.find("kernel") != std::string::npos);
    bool isRawBin = (target.find("raw") != std::string::npos || target.find("bin") != std::string::npos);

    ulang::codegen::x64::ABI abi = (isWin || isEfi) ? ulang::codegen::x64::ABI::Win64 : ulang::codegen::x64::ABI::SysV;
    ulang::codegen::x64::CodeGenerator cg(abi);
    auto art = cg.generate(*irRes.program);

    bool ok = false;
    if (isEfi) {
        ok = ulang::link::efi::linkEFIApp(art, outPath);
    } else if (isFreestanding) {
        if (isRawBin) {
            ok = ulang::link::elf::linkRawBinary(art, outPath);
        } else {
            ok = ulang::link::elf::linkMultibootELF(art, outPath, arch);
        }
    } else if (isWin) {
        ok = ulang::link::pe::linkPE(art, outPath, false);
    } else {
        ok = ulang::link::elf::linkELF(art, outPath);
    }

    if (!ok) {
        std::cerr << "ulang++ build error: failed to write output file '" << outPath << "'\n";
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
    std::string tmpExe = "temp_ulang_run.exe";
#ifndef _WIN32
    tmpExe = "./temp_ulang_run";
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
        std::cout << "ulang++ " << kVersion << '\n';
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

    std::cerr << "ulang++: unknown command '" << command << "'\n";
    return usage();
}
