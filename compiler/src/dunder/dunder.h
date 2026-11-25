// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

#pragma once

#include <string>
#include <string_view>
#include <unordered_map>

namespace pawl::dunder {

struct Def {
    std::string type; // "string", "int", "bool" or "list[string]"
    bool positional = false;
};

// Known dunder variables and their types
const std::unordered_map<std::string, Def>& registry();

// IsReserved reports whether name is a dunder (__*__).
bool isReserved(std::string_view name);

// IsPositional reports whether the value is resolved from the source token position.
bool isPositional(std::string_view name);

} // namespace pawl::dunder
