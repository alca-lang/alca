// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

//go:build !amd64

package compiler

import (
	"errors"

	"github.com/alca-lang/alca/pkg/ast"
)

// BuildPureNativeDLL reports unsupported on non-x86-64 hosts.
func BuildPureNativeDLL(outputPath string, dllName string, exports map[string][]byte) error {
	return errors.New("pure native PE DLL generation is only supported on amd64 architectures")
}

// BuildNativeDLLFromAlca reports unsupported on non-x86-64 hosts.
func BuildNativeDLLFromAlca(alcaSource string, outputPath string) error {
	return errors.New("pure native PE DLL generation is only supported on amd64 architectures")
}

// CompileFuncDeclarationToAMD64 reports unsupported on non-x86-64 hosts.
func CompileFuncDeclarationToAMD64(fn *ast.FuncDeclaration) ([]byte, error) {
	return nil, errors.New("native AMD64 compilation is only supported on amd64 architectures")
}
