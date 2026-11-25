// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

#include "dunder/dunder.h"

namespace ulang::dunder {

const std::unordered_map<std::string, Def>& registry() {
    static const std::unordered_map<std::string, Def> kRegistry = {
        {"__file__",    {"string", false}},
        {"__dir__",     {"string", false}},
        {"__dirname__", {"string", false}},
        {"__name__",    {"string", false}},
        {"__version__", {"string", false}},
        {"__line__",    {"int", true}},
        {"__column__",  {"int", true}},
        {"__os__",      {"string", false}},
        {"__arch__",    {"string", false}},
        {"__exe__",     {"string", false}},
        {"__argv__",    {"list[string]", false}},
        {"__compiled__",{"bool", false}},
    };
    return kRegistry;
}

bool isReserved(std::string_view name) {
    return name.size() > 4 && name.rfind("__", 0) == 0 &&
           name.compare(name.size() - 2, 2, "__") == 0;
}

bool isPositional(std::string_view name) {
    const auto& reg = registry();
    auto it = reg.find(std::string(name));
    return it != reg.end() && it->second.positional;
}

} // namespace ulang::dunder
