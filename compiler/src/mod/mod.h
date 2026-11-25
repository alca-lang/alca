// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

#pragma once

#include <functional>
#include <optional>
#include <string>
#include <string_view>
#include <unordered_map>

namespace pawl::mod {

struct Resolved {
    std::string modulePath;
    std::string sourcePath;
    std::string artifactPath;
    std::string package;
    std::string version;
    std::string root;
};

// BindingName returns the identifier a `use` statement binds by default
std::string bindingName(std::string_view modulePath);

// SetProjectRoot sets the directory containing the project
void setProjectRoot(const std::string& dir);
std::string projectRoot();

// SetBundled installs source overrides
void setBundled(const std::unordered_map<std::string, std::string>& files);

// Resolve maps a module path written in source to a loadable file.
std::optional<Resolved> resolve(const std::string& modulePath, const std::string& fromFile);

} // namespace pawl::mod
