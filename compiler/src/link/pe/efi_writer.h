// Copyright 2026 ULang++ Programming Language
// SPDX-License-Identifier: Apache-2.0

#pragma once

#include "codegen/x64/codegen.h"
#include <string>

namespace ulang::link::efi {

// Emits a PE32+ EFI_APPLICATION image (Subsystem 10) with no imports and no
// CRT.  The entry stub forwards ImageHandle/SystemTable to `efi_main`.
bool linkEFIApp(const codegen::x64::CompiledArtifact& art, const std::string& outputPath);

} // namespace ulang::link::efi
