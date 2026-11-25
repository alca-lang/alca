// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

#pragma once

#include "codegen/x64/codegen.h"
#include <string>

namespace pawl::link::pe {

bool linkPE(const codegen::x64::CompiledArtifact& art, const std::string& outputPath, bool noConsole = false);

} // namespace pawl::link::pe
