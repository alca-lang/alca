// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

#include "mod/mod.h"

#include <algorithm>
#include <filesystem>
#include <mutex>
#include <unordered_set>
#ifdef _WIN32
#include <windows.h>
#endif

namespace ulang::mod {

namespace {

std::mutex gMutex;
std::string gProjectRoot;
std::unordered_map<std::string, std::string> gBundled;

std::filesystem::path getExecutableDir() {
#ifdef _WIN32
    char buffer[MAX_PATH];
    DWORD len = GetModuleFileNameA(NULL, buffer, MAX_PATH);
    if (len > 0 && len < MAX_PATH) {
        return std::filesystem::path(buffer).parent_path();
    }
#endif
    return std::filesystem::current_path();
}

std::string trimSpace(std::string_view s) {
    size_t start = 0;
    while (start < s.size() && std::isspace(static_cast<unsigned char>(s[start]))) ++start;
    size_t end = s.size();
    while (end > start && std::isspace(static_cast<unsigned char>(s[end - 1]))) --end;
    return std::string(s.substr(start, end - start));
}

std::string localRelativePath(std::string_view modulePath) {
    std::string p = trimSpace(modulePath);
    if (p.empty()) return "";
    if (p.front() == '"' && p.back() == '"' && p.size() >= 2) {
        p = p.substr(1, p.size() - 2);
    }
    if (p.size() >= 3 && p.compare(p.size() - 3, 3, ".upp") == 0) return p;
    if (p.find('/') != std::string::npos || p.find('\\') != std::string::npos || p[0] == '.') {
        return p;
    }
    std::string rel = p;
    for (char& c : rel) {
        if (c == '.') c = '/';
    }
    return rel + ".upp";
}

} // namespace

std::string bindingName(std::string_view modulePath) {
    std::string p = trimSpace(modulePath);
    if (p.empty()) return "";
    if (p.front() == '"' && p.back() == '"' && p.size() >= 2) {
        p = p.substr(1, p.size() - 2);
    }
    if ((p.size() >= 3 && p.compare(p.size() - 3, 3, ".upp") == 0) ||
        p.find('/') != std::string::npos || p.find('\\') != std::string::npos) {
        std::filesystem::path fp(p);
        return fp.stem().string();
    }
    size_t lastDot = p.rfind('.');
    if (lastDot != std::string::npos && lastDot + 1 < p.size()) {
        return p.substr(lastDot + 1);
    }
    return p;
}

void setProjectRoot(const std::string& dir) {
    std::lock_guard<std::mutex> lock(gMutex);
    gProjectRoot = dir;
}

std::string projectRoot() {
    std::lock_guard<std::mutex> lock(gMutex);
    return gProjectRoot;
}

void setBundled(const std::unordered_map<std::string, std::string>& files) {
    std::lock_guard<std::mutex> lock(gMutex);
    gBundled = files;
}

std::optional<Resolved> resolve(const std::string& modulePath, const std::string& fromFile) {
    std::string trimmed = trimSpace(modulePath);
    if (trimmed.empty()) return std::nullopt;

    {
        std::lock_guard<std::mutex> lock(gMutex);
        auto it = gBundled.find(trimmed);
        if (it != gBundled.end()) {
            Resolved r;
            r.modulePath = trimmed;
            r.sourcePath = it->second;
            return r;
        }
    }

    std::string rel = localRelativePath(trimmed);
    if (rel.empty()) return std::nullopt;

    std::vector<std::filesystem::path> dirs;
    if (!fromFile.empty() && fromFile[0] != '<') {
        std::error_code ec;
        auto p = std::filesystem::absolute(fromFile, ec);
        if (!ec) {
            dirs.push_back(p.parent_path());
            // Check repo root if fromFile is inside a subdirectory
            if (p.parent_path().has_parent_path()) {
                dirs.push_back(p.parent_path().parent_path());
            }
        }
    }
    dirs.push_back(std::filesystem::current_path());

    // Check compiler binary adjacent standard library directories:
    auto exeDir = getExecutableDir();
    dirs.push_back(exeDir / "std");
    dirs.push_back(exeDir.parent_path() / "std");
    dirs.push_back(exeDir.parent_path() / "lib" / "ulang" / "std");

    // Check ProgramData/ULangLibs/libs and legacy ProgramData/PawlLibs/libs
    const char* progData = std::getenv("ProgramData");
    if (progData && *progData) {
        dirs.push_back(std::filesystem::path(progData) / "ULangLibs" / "libs");
        dirs.push_back(std::filesystem::path(progData) / "ULangLibs" / "std");
    }
#ifdef _WIN32
    dirs.push_back(std::filesystem::path("C:/ProgramData/ULangLibs/libs"));
    dirs.push_back(std::filesystem::path("C:/ProgramData/ULangLibs/std"));
#else
    dirs.push_back(std::filesystem::path("/usr/local/share/ULangLibs/libs"));
    dirs.push_back(std::filesystem::path("/usr/local/share/ULangLibs/std"));
    dirs.push_back(std::filesystem::path("/var/lib/ULangLibs/libs"));
    dirs.push_back(std::filesystem::path("/var/lib/ULangLibs/std"));
#endif

    std::unordered_set<std::string> seen;
    for (const auto& dir : dirs) {
        std::error_code ec;
        // First try standard rel (e.g. std/io.upp, mylib.upp)
        auto abs = std::filesystem::weakly_canonical(dir / rel, ec);
        if (!ec) {
            std::string absStr = abs.string();
            if (seen.find(absStr) == seen.end()) {
                seen.insert(absStr);
                if (std::filesystem::is_regular_file(abs, ec)) {
                    Resolved res;
                    res.modulePath = trimmed;
                    res.sourcePath = absStr;
                    res.root = abs.parent_path().string();
                    return res;
                }
            }
        }

        // If trimmed started with std. or std/, also try directly inside dir (dir / io.upp)
        // when dir is already a std/ directory
        if (rel.rfind("std/", 0) == 0 || rel.rfind("std\\", 0) == 0) {
            std::string sub = rel.substr(4);
            auto absSub = std::filesystem::weakly_canonical(dir / sub, ec);
            if (!ec) {
                std::string absSubStr = absSub.string();
                if (seen.find(absSubStr) == seen.end()) {
                    seen.insert(absSubStr);
                    if (std::filesystem::is_regular_file(absSub, ec)) {
                        Resolved res;
                        res.modulePath = trimmed;
                        res.sourcePath = absSubStr;
                        res.root = absSub.parent_path().string();
                        return res;
                    }
                }
            }
        }
    }

    return std::nullopt;
}

} // namespace ulang::mod
