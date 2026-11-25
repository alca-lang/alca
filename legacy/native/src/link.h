// Pawl bootstrap compiler - built-in PE32+ and ELF64 writers.
// No system linker, no object files, no external toolchain.
// SPDX-License-Identifier: Apache-2.0
#pragma once

#include "codegen.h"

#include <string>

void writePE(Module& m, const std::string& path);
void writeELF(Module& m, const std::string& path);
