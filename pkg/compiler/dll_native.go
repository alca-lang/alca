// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package compiler

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alca-lang/alca/pkg/ast"
	"github.com/alca-lang/alca/pkg/jit"
	"github.com/alca-lang/alca/pkg/lexer"
	"github.com/alca-lang/alca/pkg/parser"
)

// BuildPureNativeDLL constructs a genuine 64-bit Windows PE DLL with the specified
// exported functions directly in pure native machine code, with zero external compilers.
func BuildPureNativeDLL(outputPath string, dllName string, exports map[string][]byte) error {
	if len(exports) == 0 {
		return fmt.Errorf("cannot build DLL without exported functions")
	}

	if dllName == "" {
		dllName = filepath.Base(outputPath)
	}

	pe := NewPEBuilder()
	pe.SetDLL(true, dllName)

	text := new(bytes.Buffer)

	// 1. DllMain entry point at offset 0
	// DllMain(HINSTANCE hinstDLL, DWORD fdwReason, LPVOID lpReserved)
	// mov eax, 1 (return TRUE)
	// ret
	text.Write([]byte{0xB8, 0x01, 0x00, 0x00, 0x00, 0xC3})

	// Pad to 16 bytes alignment
	for text.Len()%16 != 0 {
		text.WriteByte(0x90) // nop
	}

	// 2. Append each exported function
	for name, code := range exports {
		offset := uint32(text.Len())
		pe.AddExport(name, offset)
		text.Write(code)

		// Ensure 16-byte alignment between functions
		for text.Len()%16 != 0 {
			text.WriteByte(0x90) // nop
		}
	}

	pe.SetCode(text.Bytes())

	dllBytes, err := pe.Build()
	if err != nil {
		return fmt.Errorf("failed to build PE DLL: %w", err)
	}

	if err := os.WriteFile(outputPath, dllBytes, 0755); err != nil {
		return fmt.Errorf("failed to write DLL file %q: %w", outputPath, err)
	}

	return nil
}

// BuildNativeDLLFromAlca compiles an Alca source file into a pure native Windows DLL.
// Any function marked with 'pub' is exported in the DLL's Export Directory Table.
func BuildNativeDLLFromAlca(alcaSource string, outputPath string) error {
	p := parser.New(lexer.New(alcaSource))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return fmt.Errorf("parser errors: %s", strings.Join(p.Errors(), "; "))
	}

	exports := make(map[string][]byte)

	for _, stmt := range prog.Statements {
		fn, ok := stmt.(*ast.FuncDeclaration)
		if !ok || !fn.IsPublic {
			continue
		}

		code, err := CompileFuncDeclarationToAMD64(fn)
		if err != nil {
			return fmt.Errorf("failed to compile exported function %q: %w", fn.Name, err)
		}
		exports[fn.Name] = code
	}

	if len(exports) == 0 {
		return fmt.Errorf("no 'pub' functions found to export in DLL (use 'pub func name(...)')")
	}

	return BuildPureNativeDLL(outputPath, filepath.Base(outputPath), exports)
}

// CompileFuncDeclarationToAMD64 compiles an AST function into native Windows x64 machine code
// adhering to the Microsoft x64 calling convention (args in RCX, RDX, R8, R9; return in RAX).
func CompileFuncDeclarationToAMD64(fn *ast.FuncDeclaration) ([]byte, error) {
	asm := jit.NewAssemblerAMD64()

	// Microsoft x64 Calling Convention:
	// 1st arg: RCX
	// 2nd arg: RDX
	// 3rd arg: R8
	// 4th arg: R9
	paramRegs := []jit.Register{jit.RCX, jit.RDX, jit.R8, jit.R9}
	paramMap := make(map[string]jit.Register)
	for i, param := range fn.Params {
		if i < len(paramRegs) {
			paramMap[param.Name] = paramRegs[i]
		}
	}

	// Local variable stack frame
	// [RBP - 8], [RBP - 16], ...
	locals := make(map[string]int32)
	localCount := int32(0)

	// Function Prologue: setup stack frame
	asm.Push(jit.RBP)
	asm.MovRegReg(jit.RBP, jit.RSP)

	// Save incoming parameters onto stack for reliable local access
	for _, param := range fn.Params {
		localCount++
		disp := -8 * localCount
		locals[param.Name] = disp
		if reg, ok := paramMap[param.Name]; ok {
			asm.MovMemReg(jit.RBP, disp, reg)
		}
	}

	// Allocate initial shadow/local space (aligned to 16 bytes)
	asm.SubRegImm32(jit.RSP, 64)

	var compileExpr func(e ast.Expression) error
	compileExpr = func(e ast.Expression) error {
		switch expr := e.(type) {
		case *ast.IntegerLiteral:
			asm.MovRegImm64(jit.RAX, uint64(expr.Value))
			return nil

		case *ast.BooleanLiteral:
			val := int32(0)
			if expr.Value {
				val = 1
			}
			asm.MovRegImm32(jit.RAX, val)
			return nil

		case *ast.Identifier:
			if disp, ok := locals[expr.Value]; ok {
				asm.MovRegMem(jit.RAX, jit.RBP, disp)
				return nil
			}
			return fmt.Errorf("undefined identifier %q", expr.Value)

		case *ast.PrefixExpression:
			if err := compileExpr(expr.Right); err != nil {
				return err
			}
			switch expr.Operator {
			case "-":
				asm.NegReg(jit.RAX)
			case "!":
				asm.CmpRegImm32(jit.RAX, 0)
				asm.Setcc(jit.CondE, jit.RAX)
				asm.MovzxByte(jit.RAX, jit.RAX)
			case "~":
				asm.NotReg(jit.RAX)
			default:
				return fmt.Errorf("unsupported prefix operator %q", expr.Operator)
			}
			return nil

		case *ast.InfixExpression:
			// Evaluate Left into RAX
			if err := compileExpr(expr.Left); err != nil {
				return err
			}
			// Push Left onto stack
			asm.Push(jit.RAX)

			// Evaluate Right into RAX
			if err := compileExpr(expr.Right); err != nil {
				return err
			}
			// Pop Left into R10
			asm.Pop(jit.R10)

			// Now R10 has Left, RAX has Right
			switch expr.Operator {
			case "+":
				asm.AddRegReg(jit.R10, jit.RAX)
				asm.MovRegReg(jit.RAX, jit.R10)
			case "-":
				asm.SubRegReg(jit.R10, jit.RAX)
				asm.MovRegReg(jit.RAX, jit.R10)
			case "*":
				asm.ImulRegReg(jit.R10, jit.RAX)
				asm.MovRegReg(jit.RAX, jit.R10)
			case "/":
				// RAX = Left, RCX = Right
				asm.MovRegReg(jit.RCX, jit.RAX) // Right in RCX
				asm.MovRegReg(jit.RAX, jit.R10) // Left in RAX
				asm.Cqo()                       // sign extend RAX into RDX:RAX
				asm.IdivReg(jit.RCX)            // RAX = RAX / RCX
			case "%":
				asm.MovRegReg(jit.RCX, jit.RAX) // Right in RCX
				asm.MovRegReg(jit.RAX, jit.R10) // Left in RAX
				asm.Cqo()
				asm.IdivReg(jit.RCX)
				asm.MovRegReg(jit.RAX, jit.RDX) // remainder in RDX
			case "&":
				asm.AndRegReg(jit.R10, jit.RAX)
				asm.MovRegReg(jit.RAX, jit.R10)
			case "|":
				asm.OrRegReg(jit.R10, jit.RAX)
				asm.MovRegReg(jit.RAX, jit.R10)
			case "^":
				asm.XorRegReg(jit.R10, jit.RAX)
				asm.MovRegReg(jit.RAX, jit.R10)
			case "==":
				asm.CmpRegReg(jit.R10, jit.RAX)
				asm.Setcc(jit.CondE, jit.RAX)
				asm.MovzxByte(jit.RAX, jit.RAX)
			case "!=":
				asm.CmpRegReg(jit.R10, jit.RAX)
				asm.Setcc(jit.CondNE, jit.RAX)
				asm.MovzxByte(jit.RAX, jit.RAX)
			case "<":
				asm.CmpRegReg(jit.R10, jit.RAX)
				asm.Setcc(jit.CondL, jit.RAX)
				asm.MovzxByte(jit.RAX, jit.RAX)
			case "<=":
				asm.CmpRegReg(jit.R10, jit.RAX)
				asm.Setcc(jit.CondLE, jit.RAX)
				asm.MovzxByte(jit.RAX, jit.RAX)
			case ">":
				asm.CmpRegReg(jit.R10, jit.RAX)
				asm.Setcc(jit.CondG, jit.RAX)
				asm.MovzxByte(jit.RAX, jit.RAX)
			case ">=":
				asm.CmpRegReg(jit.R10, jit.RAX)
				asm.Setcc(jit.CondGE, jit.RAX)
				asm.MovzxByte(jit.RAX, jit.RAX)
			default:
				return fmt.Errorf("unsupported operator %q in native DLL function", expr.Operator)
			}
			return nil

		default:
			return fmt.Errorf("unsupported expression type %T in native DLL function", e)
		}
	}

	// Compile body statements
	if fn.Body != nil {
		for _, stmt := range fn.Body.Statements {
			switch s := stmt.(type) {
			case *ast.VarDeclaration:
				if s.Value != nil && s.Name != nil {
					if err := compileExpr(s.Value); err != nil {
						return nil, err
					}
					localCount++
					disp := -8 * localCount
					locals[s.Name.Value] = disp
					asm.MovMemReg(jit.RBP, disp, jit.RAX)
				}
			case *ast.ExpressionStatement:
				if assign, ok := s.Expression.(*ast.AssignExpression); ok {
					if targetIdent, isIdent := assign.Target.(*ast.Identifier); isIdent {
						if disp, ok := locals[targetIdent.Value]; ok {
							if err := compileExpr(assign.Value); err != nil {
								return nil, err
							}
							asm.MovMemReg(jit.RBP, disp, jit.RAX)
						} else {
							return nil, fmt.Errorf("undefined variable %q", targetIdent.Value)
						}
					}
				}
			case *ast.ReturnStatement:
				if s.Value != nil {
					if err := compileExpr(s.Value); err != nil {
						return nil, err
					}
				} else {
					asm.MovRegImm32(jit.RAX, 0)
				}
				// Epilogue
				asm.MovRegReg(jit.RSP, jit.RBP)
				asm.Pop(jit.RBP)
				asm.Ret()
				return asm.Finalize()
			}
		}
	}

	// Default epilogue if no explicit return
	asm.MovRegImm32(jit.RAX, 0)
	asm.MovRegReg(jit.RSP, jit.RBP)
	asm.Pop(jit.RBP)
	asm.Ret()

	return asm.Finalize()
}
