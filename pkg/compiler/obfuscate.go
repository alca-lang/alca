// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package compiler

import (
	"fmt"
	"strings"

	"github.com/alca-lang/alca/pkg/ast"
	"github.com/alca-lang/alca/pkg/token"
)

// preservedIdentifiers lists names that should never be obfuscated because they
// are part of the runtime, standard library, or external interfaces.
var preservedIdentifiers = map[string]bool{
	"main":        true,
	"init":        true,
	"self":        true,
	"this":        true,
	"super":       true,
	"true":        true,
	"false":       true,
	"null":        true,
	"std":         true,
	"io":          true,
	"audio":       true,
	"image":       true,
	"encoding":    true,
	"marshal":     true,
	"libtorch":    true,
	"math":        true,
	"net":         true,
	"http":        true,
	"sys":         true,
	"time":        true,
	"win32":       true,
	"gui":         true,
	"ffi":         true,
	"crypto":      true,
	"re":          true,
	"regex":       true,
	"string":      true,
	"str":         true,
	"list":        true,
	"map":         true,
	"int":         true,
	"float":       true,
	"bool":        true,
	"json":        true,
	"print":       true,
	"println":     true,
	"len":         true,
	"input":       true,
	"sleep":       true,
	"exit":        true,
	"min":         true,
	"max":         true,
	"abs":         true,
	"__file__":    true,
	"__dir__":     true,
	"__line__":    true,
	"__version__": true,
}

// Obfuscator traverses and anonymizes user identifiers and strips debug metadata.
type Obfuscator struct {
	counter   int
	mangled   map[string]string
	protected map[string]bool
}

// NewObfuscator initializes a new obfuscation context.
func NewObfuscator() *Obfuscator {
	return &Obfuscator{
		mangled:   make(map[string]string),
		protected: make(map[string]bool),
	}
}

func (o *Obfuscator) getMangledName(orig string) string {
	if orig == "" || preservedIdentifiers[orig] || o.protected[orig] {
		return orig
	}
	if strings.HasPrefix(orig, "_0x") {
		return orig
	}
	if m, ok := o.mangled[orig]; ok {
		return m
	}
	o.counter++
	m := fmt.Sprintf("_0x%04x", o.counter)
	o.mangled[orig] = m
	return m
}

// ObfuscateProgram strips debug symbols, comments and anonymizes identifiers in the AST.
func ObfuscateProgram(prog *ast.Program) *ast.Program {
	if prog == nil {
		return nil
	}
	obf := NewObfuscator()

	// Pass 1: Collect exported symbols and top-level module names
	for _, stmt := range prog.Statements {
		if use, ok := stmt.(*ast.UseStatement); ok {
			obf.protected[use.Alias] = true
			for _, imp := range use.Imports {
				obf.protected[imp] = true
			}
		}
	}

	// Pass 2: Anonymize AST nodes
	for i := range prog.Statements {
		prog.Statements[i] = obf.obfuscateStatement(prog.Statements[i])
	}

	return prog
}

func (o *Obfuscator) stripToken(tok *token.Token) {
	if tok != nil {
		tok.Line = 0
		tok.Column = 0
	}
}

func (o *Obfuscator) obfuscateStatement(s ast.Statement) ast.Statement {
	if s == nil {
		return nil
	}
	switch stmt := s.(type) {
	case *ast.VarDeclaration:
		o.stripToken(&stmt.Token)
		if stmt.Name != nil {
			stmt.Name.Value = o.getMangledName(stmt.Name.Value)
		}
		stmt.Value = o.obfuscateExpression(stmt.Value)
		return stmt

	case *ast.FuncDeclaration:
		o.stripToken(&stmt.Token)
		if stmt.Name != "main" {
			stmt.Name = o.getMangledName(stmt.Name)
		}
		for i := range stmt.Params {
			stmt.Params[i].Name = o.getMangledName(stmt.Params[i].Name)
			if stmt.Params[i].DefaultValue != nil {
				stmt.Params[i].DefaultValue = o.obfuscateExpression(stmt.Params[i].DefaultValue)
			}
		}
		stmt.Body = o.obfuscateBlockStatement(stmt.Body)
		return stmt

	case *ast.ExpressionStatement:
		o.stripToken(&stmt.Token)
		stmt.Expression = o.obfuscateExpression(stmt.Expression)
		return stmt

	case *ast.BlockStatement:
		return o.obfuscateBlockStatement(stmt)

	case *ast.IfStatement:
		o.stripToken(&stmt.Token)
		stmt.Condition = o.obfuscateExpression(stmt.Condition)
		stmt.Consequence = o.obfuscateBlockStatement(stmt.Consequence)
		for i := range stmt.Alternatives {
			if stmt.Alternatives[i] != nil {
				stmt.Alternatives[i].Condition = o.obfuscateExpression(stmt.Alternatives[i].Condition)
				stmt.Alternatives[i].Body = o.obfuscateBlockStatement(stmt.Alternatives[i].Body)
			}
		}
		if stmt.ElseBlock != nil {
			stmt.ElseBlock = o.obfuscateBlockStatement(stmt.ElseBlock)
		}
		return stmt

	case *ast.WhileStatement:
		o.stripToken(&stmt.Token)
		stmt.Condition = o.obfuscateExpression(stmt.Condition)
		stmt.Body = o.obfuscateBlockStatement(stmt.Body)
		return stmt

	case *ast.ForStatement:
		o.stripToken(&stmt.Token)
		stmt.Variable = o.getMangledName(stmt.Variable)
		stmt.Iterable = o.obfuscateExpression(stmt.Iterable)
		stmt.Body = o.obfuscateBlockStatement(stmt.Body)
		return stmt

	case *ast.ForEachStatement:
		o.stripToken(&stmt.Token)
		stmt.Variable = o.getMangledName(stmt.Variable)
		stmt.Iterable = o.obfuscateExpression(stmt.Iterable)
		stmt.Body = o.obfuscateBlockStatement(stmt.Body)
		return stmt

	case *ast.ReturnStatement:
		o.stripToken(&stmt.Token)
		if stmt.Value != nil {
			stmt.Value = o.obfuscateExpression(stmt.Value)
		}
		return stmt

	default:
		return s
	}
}

func (o *Obfuscator) obfuscateBlockStatement(b *ast.BlockStatement) *ast.BlockStatement {
	if b == nil {
		return nil
	}
	o.stripToken(&b.Token)
	for i := range b.Statements {
		b.Statements[i] = o.obfuscateStatement(b.Statements[i])
	}
	return b
}

func (o *Obfuscator) obfuscateExpression(e ast.Expression) ast.Expression {
	if e == nil {
		return nil
	}
	switch expr := e.(type) {
	case *ast.Identifier:
		o.stripToken(&expr.Token)
		expr.Value = o.getMangledName(expr.Value)
		return expr

	case *ast.CallExpression:
		o.stripToken(&expr.Token)
		expr.Function = o.obfuscateExpression(expr.Function)
		for i := range expr.Arguments {
			expr.Arguments[i] = o.obfuscateExpression(expr.Arguments[i])
		}
		return expr

	case *ast.InfixExpression:
		o.stripToken(&expr.Token)
		expr.Left = o.obfuscateExpression(expr.Left)
		expr.Right = o.obfuscateExpression(expr.Right)
		return expr

	case *ast.PrefixExpression:
		o.stripToken(&expr.Token)
		expr.Right = o.obfuscateExpression(expr.Right)
		return expr

	case *ast.AssignExpression:
		o.stripToken(&expr.Token)
		expr.Target = o.obfuscateExpression(expr.Target)
		expr.Value = o.obfuscateExpression(expr.Value)
		return expr

	case *ast.DotExpression:
		o.stripToken(&expr.Token)
		expr.Left = o.obfuscateExpression(expr.Left)
		// Don't mangle the right side member/field name of a dot access
		return expr

	case *ast.IndexExpression:
		o.stripToken(&expr.Token)
		expr.Left = o.obfuscateExpression(expr.Left)
		expr.Index = o.obfuscateExpression(expr.Index)
		return expr

	case *ast.ListLiteral:
		o.stripToken(&expr.Token)
		for i := range expr.Elements {
			expr.Elements[i] = o.obfuscateExpression(expr.Elements[i])
		}
		return expr

	case *ast.MapLiteral:
		o.stripToken(&expr.Token)
		for i := range expr.Pairs {
			expr.Pairs[i].Key = o.obfuscateExpression(expr.Pairs[i].Key)
			expr.Pairs[i].Value = o.obfuscateExpression(expr.Pairs[i].Value)
		}
		return expr

	case *ast.LambdaExpression:
		o.stripToken(&expr.Token)
		for i := range expr.Params {
			expr.Params[i].Name = o.getMangledName(expr.Params[i].Name)
		}
		expr.Body = o.obfuscateExpression(expr.Body)
		return expr

	default:
		return e
	}
}
