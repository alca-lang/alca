// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

#pragma once

#include "ir.h"
#include "ast/ast.h"
#include "typecheck/typecheck.h"
#include <string>

namespace ulang::ir {

struct LowerResult {
    std::unique_ptr<Program> program;
    std::string error;

    bool ok() const { return error.empty() && program != nullptr; }
};

LowerResult lower(const std::shared_ptr<ast::Program>& prog, const typecheck::Checker& checker);

} // namespace ulang::ir
