// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

#pragma once

#include <string>
#include <unordered_map>
#include <vector>

namespace ulang::typecheck {

const std::unordered_map<std::string, std::unordered_map<std::string, std::string>>& getModuleMembers();
const std::unordered_map<std::string, std::string>& getModuleAliases();
const std::unordered_map<std::string, std::string>& getGlobalSigs();

} // namespace ulang::typecheck
