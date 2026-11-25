// Pawl bootstrap native compiler - CLI entry point.
// SPDX-License-Identifier: Apache-2.0
#include "codegen.h"
#include "front.h"
#include "link.h"

#include <cctype>
#include <cstdio>
#include <fstream>
#include <iostream>
#include <sstream>
#include <string>

static std::string readFile(const std::string& path) {
    std::ifstream f(path, std::ios::binary);
    if (!f) fail("cannot open source file '" + path + "'");
    std::ostringstream ss;
    ss << f.rdbuf();
    return ss.str();
}

static void usage() {
    std::cerr
        << "Pawl bootstrap compiler (C++ / no Go / no external linker)\n"
        << "usage: pawlc <file.al> -o <output> [--target windows|linux]\n"
        << "supported today: int, string, bool-as-int, variables, assignment,\n"
        << "  if/else, while, functions (<=4 params), return, print/println, exit.\n"
        << "not yet: imports, lists, maps, structs, classes, enums, stdlib, ARM64.\n";
}

int main(int argc, char** argv) {
    std::string input;
    std::string output;
    std::string target;

    for (int i = 1; i < argc; i++) {
        std::string a = argv[i];
        if ((a == "-o" || a == "--output") && i + 1 < argc) {
            output = argv[++i];
        } else if (a == "--target" && i + 1 < argc) {
            target = argv[++i];
        } else if (a == "-h" || a == "--help") {
            usage();
            return 0;
        } else if (!a.empty() && a[0] == '-') {
            std::cerr << "error: unknown option '" << a << "'\n";
            return 2;
        } else if (input.empty()) {
            input = a;
        } else {
            std::cerr << "error: unexpected argument '" << a << "'\n";
            return 2;
        }
    }

    if (input.empty()) {
        usage();
        return 2;
    }

    if (target.empty()) {
        if (output.size() > 4) {
            std::string ext = output.substr(output.size() - 4);
            for (auto& c : ext) c = char(std::tolower((unsigned char)c));
            target = (ext == ".exe") ? "windows" : "linux";
        } else {
            target = "windows";
        }
    }
    if (target != "windows" && target != "linux") {
        std::cerr << "error: unsupported target '" << target << "' (expected windows or linux)\n";
        return 2;
    }
    if (output.empty()) output = (target == "windows") ? "a.exe" : "a.out";

    try {
        std::string src = readFile(input);
        Program prog = parseProgram(src, input);
        if (!prog.imports.empty()) {
            std::cerr << "warning: " << prog.imports.size()
                      << " import(s) ignored: module compilation is not implemented yet\n";
        }
        Module m = compileModule(prog, target == "windows");
        if (target == "windows") writePE(m, output);
        else writeELF(m, output);

        std::printf("[OK] %s (%s, x86-64 machine code)\n", output.c_str(), target.c_str());
        return 0;
    } catch (const CompileError& e) {
        std::cerr << "error: " << e.what() << "\n";
        return 1;
    } catch (const std::exception& e) {
        std::cerr << "internal error: " << e.what() << "\n";
        return 3;
    }
}
