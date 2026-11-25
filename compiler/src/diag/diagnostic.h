// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

#pragma once

#include <string>
#include <vector>
#include <memory>
#include <cstdint>

namespace ulang::diag {

struct SyntaxError {
    std::string filename;
    int line = 0;
    int column = 0;
    std::string message;
    std::string token;
    int length = 1;
    std::string suggestion;

    std::string error() const;
};

std::string formatReport(const std::string& filename, const std::string& source,
                         const std::vector<std::shared_ptr<SyntaxError>>& errors,
                         bool useColor = false);

std::string generateSuggestion(const std::string& msg, const std::string& tok);

} // namespace ulang::diag
