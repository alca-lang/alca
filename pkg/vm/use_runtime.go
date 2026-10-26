// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package vm

import (
	"github.com/alca-lang/alca/pkg/ast"
	"github.com/alca-lang/alca/pkg/interpreter"
)

// ExecuteUseStatement performs the runtime effect of one compiled `use`
// statement. It is shared by the bytecode VM and the JIT runtime-helper tier so
// an import has identical resolution, module-loader, and environment effects in
// both paths.
func ExecuteUseStatement(env *interpreter.Environment, useVal *UseStatementValue, sourceFile string) {
	if env == nil || useVal == nil || useVal.Stmt == nil {
		return
	}
	if load := currentModuleLoader(); load != nil && load(useVal.Stmt, sourceFile, env) {
		return
	}
	interp := interpreter.New()
	interp.SetFilename(sourceFile)
	_ = interp.ExecuteInEnv(&ast.Program{Statements: []ast.Statement{useVal.Stmt}}, env)
}
