// Copyright 2026 ULang++ Programming Language
// SPDX-License-Identifier: Apache-2.0

#pragma once

#include <cstring>
#include <iostream>
#include <string>

#ifdef _WIN32
#ifndef WIN32_LEAN_AND_MEAN
#define WIN32_LEAN_AND_MEAN
#endif
#include <windows.h>
#endif

namespace ulang::cli {

inline constexpr const char* kRed = "\033[31m";
inline constexpr const char* kYellow = "\033[33m";
inline constexpr const char* kGreen = "\033[32m";
inline constexpr const char* kReset = "\033[0m";

inline void enableAnsi() {
#ifdef _WIN32
    HANDLE h = GetStdHandle(STD_OUTPUT_HANDLE);
    if (h != INVALID_HANDLE_VALUE) {
        DWORD mode = 0;
        if (GetConsoleMode(h, &mode)) {
            SetConsoleMode(h, mode | ENABLE_VIRTUAL_TERMINAL_PROCESSING);
        }
    }
    h = GetStdHandle(STD_ERROR_HANDLE);
    if (h != INVALID_HANDLE_VALUE) {
        DWORD mode = 0;
        if (GetConsoleMode(h, &mode)) {
            SetConsoleMode(h, mode | ENABLE_VIRTUAL_TERMINAL_PROCESSING);
        }
    }
#endif
}

inline void printError(const std::string& s) {
    std::cerr << kRed << s << kReset << '\n';
}

inline void printWarn(const std::string& s) {
    std::cerr << kYellow << s << kReset << '\n';
}

inline void printOk(const std::string& s) {
    std::cout << kGreen << s << kReset << '\n';
}

inline void printInfo(const std::string& s) {
    std::cout << s << '\n';
}

// Diagnostic blocks can contain either errors or warnings; pick the colour.
inline void printDiagnostic(const std::string& s) {
    if (s.find("warning") != std::string::npos ||
        s.find("Warning") != std::string::npos) {
        printWarn(s);
    } else {
        printError(s);
    }
}

class ColorStreamBuf : public std::streambuf {
public:
    explicit ColorStreamBuf(std::streambuf* sink) : sink_(sink) {}
    ~ColorStreamBuf() override { flushLine(); }

protected:
    int overflow(int c) override {
        if (c == traits_type::eof()) return traits_type::not_eof(c);
        char ch = static_cast<char>(c);
        line_.push_back(ch);
        if (ch == '\n') flushLine();
        return c;
    }

    int sync() override {
        flushLine();
        return sink_ ? sink_->pubsync() : -1;
    }

private:
    void flushLine() {
        if (line_.empty() || !sink_) return;
        const char* color = nullptr;
        if (line_.find("FATAL: no input file") != std::string::npos) {
            // Exact no-argument message stays plain text.
        } else if (line_.find("warning") != std::string::npos ||
            line_.find("Warning") != std::string::npos) {
            color = kYellow;
        } else if (line_.find("[OK]") != std::string::npos ||
                   line_.find("OK:") != std::string::npos) {
            color = kGreen;
        } else if (line_.find("FATAL") != std::string::npos ||
                   line_.find("error") != std::string::npos ||
                   line_.find("Error") != std::string::npos ||
                   line_.find("cannot ") != std::string::npos ||
                   line_.find("unknown ") != std::string::npos ||
                   line_.find("missing ") != std::string::npos ||
                   line_.find("failed") != std::string::npos) {
            color = kRed;
        }
        if (color) sink_->sputn(color, static_cast<std::streamsize>(std::strlen(color)));
        sink_->sputn(line_.data(), static_cast<std::streamsize>(line_.size()));
        if (color) sink_->sputn(kReset, static_cast<std::streamsize>(std::strlen(kReset)));
        line_.clear();
    }

    std::streambuf* sink_;
    std::string line_;
};

inline void installColors() {
    enableAnsi();
    static ColorStreamBuf outBuf(std::cout.rdbuf());
    static ColorStreamBuf errBuf(std::cerr.rdbuf());
    static bool installed = false;
    if (!installed) {
        std::cout.rdbuf(&outBuf);
        std::cerr.rdbuf(&errBuf);
        installed = true;
    }
}

} // namespace ulang::cli
