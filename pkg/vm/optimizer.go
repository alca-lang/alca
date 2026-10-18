// Package vm provides compile-time AST and bytecode peephole optimization passes for Alca.
package vm

import (
	"github.com/alca-lang/alca/pkg/interpreter"

	"github.com/alca-lang/alca/pkg/ast"
)

// Optimizer applies constant folding, dead code elimination, and peephole optimizations.
type Optimizer struct{}

// NewOptimizer creates a new Optimizer instance.
func NewOptimizer() *Optimizer {
	return &Optimizer{}
}

// OptimizeAST walks an AST program tree and applies compile-time constant folding and dead code elimination.
func (opt *Optimizer) OptimizeAST(program *ast.Program) *ast.Program {
	if program == nil {
		return nil
	}

	optimizedStmts := make([]ast.Statement, 0, len(program.Statements))
	for _, stmt := range program.Statements {
		optStmt := opt.optimizeStatement(stmt)
		if optStmt != nil {
			optimizedStmts = append(optimizedStmts, optStmt)
		}
	}
	return &ast.Program{Statements: optimizedStmts}
}

func (opt *Optimizer) optimizeStatement(stmt ast.Statement) ast.Statement {
	switch s := stmt.(type) {
	case *ast.ExpressionStatement:
		s.Expression = opt.optimizeExpression(s.Expression)
		return s

	case *ast.VarDeclaration:
		if s.Value != nil {
			s.Value = opt.optimizeExpression(s.Value)
		}
		return s

	case *ast.ReturnStatement:
		if s.Value != nil {
			s.Value = opt.optimizeExpression(s.Value)
		}
		return s

	case *ast.IfStatement:
		s.Condition = opt.optimizeExpression(s.Condition)
		// Dead-code elimination if condition is a constant boolean
		// Only eliminate when there are no elif branches: dropping them would
		// silently change program behaviour.
		if boolLit, ok := s.Condition.(*ast.BooleanLiteral); ok && len(s.Alternatives) == 0 {
			if boolLit.Value {
				// Condition is always true: inline consequence block
				return s.Consequence
			}
			// Condition is always false
			if s.ElseBlock != nil {
				return s.ElseBlock
			}
			return nil // Statement eliminated
		}
		return s

	case *ast.WhileStatement:
		s.Condition = opt.optimizeExpression(s.Condition)
		if boolLit, ok := s.Condition.(*ast.BooleanLiteral); ok && !boolLit.Value {
			return nil // Loop condition is always false, eliminate loop
		}
		return s

	case *ast.BlockStatement:
		optStmts := make([]ast.Statement, 0, len(s.Statements))
		for _, sub := range s.Statements {
			res := opt.optimizeStatement(sub)
			if res != nil {
				optStmts = append(optStmts, res)
			}
		}
		s.Statements = optStmts
		return s

	case *ast.FuncDeclaration:
		if s.Body != nil {
			if optBody, ok := opt.optimizeStatement(s.Body).(*ast.BlockStatement); ok {
				s.Body = optBody
			}
		}
		return s

	default:
		return stmt
	}
}

func (opt *Optimizer) optimizeExpression(expr ast.Expression) ast.Expression {
	if expr == nil {
		return nil
	}

	switch e := expr.(type) {
	case *ast.PrefixExpression:
		e.Right = opt.optimizeExpression(e.Right)
		return opt.foldPrefix(e)

	case *ast.InfixExpression:
		e.Left = opt.optimizeExpression(e.Left)
		e.Right = opt.optimizeExpression(e.Right)
		return opt.foldInfix(e)

	case *ast.ListLiteral:
		for i, el := range e.Elements {
			e.Elements[i] = opt.optimizeExpression(el)
		}
		return e

	case *ast.MapLiteral:
		for _, pair := range e.Pairs {
			pair.Key = opt.optimizeExpression(pair.Key)
			pair.Value = opt.optimizeExpression(pair.Value)
		}
		return e

	case *ast.CallExpression:
		e.Function = opt.optimizeExpression(e.Function)
		for i, arg := range e.Arguments {
			e.Arguments[i] = opt.optimizeExpression(arg)
		}
		return e

	default:
		return expr
	}
}

// foldPrefix evaluates constant prefix expressions at compile time.
func (opt *Optimizer) foldPrefix(e *ast.PrefixExpression) ast.Expression {
	switch e.Operator {
	case "not", "!":
		if b, ok := e.Right.(*ast.BooleanLiteral); ok {
			return &ast.BooleanLiteral{Value: !b.Value, Token: e.Token}
		}
	case "-":
		if iv, ok := e.Right.(*ast.IntegerLiteral); ok {
			return &ast.IntegerLiteral{Value: -iv.Value, Token: e.Token}
		}
		if fv, ok := e.Right.(*ast.FloatLiteral); ok {
			return &ast.FloatLiteral{Value: -fv.Value, Token: e.Token}
		}
	}
	return e
}

// foldInfix evaluates constant arithmetic, comparison, and boolean infix expressions at compile time.
func (opt *Optimizer) foldInfix(e *ast.InfixExpression) ast.Expression {
	leftInt, leftIsInt := e.Left.(*ast.IntegerLiteral)
	rightInt, rightIsInt := e.Right.(*ast.IntegerLiteral)

	if leftIsInt && rightIsInt {
		l := leftInt.Value
		r := rightInt.Value
		switch e.Operator {
		case "+":
			return &ast.IntegerLiteral{Value: l + r, Token: e.Token}
		case "-":
			return &ast.IntegerLiteral{Value: l - r, Token: e.Token}
		case "*":
			return &ast.IntegerLiteral{Value: l * r, Token: e.Token}
		case "/":
			if r != 0 {
				return &ast.IntegerLiteral{Value: l / r, Token: e.Token}
			}
		case "%":
			if r != 0 {
				return &ast.IntegerLiteral{Value: l % r, Token: e.Token}
			}
		case "**":
			if r >= 0 {
				return &ast.IntegerLiteral{Value: interpreter.IntPow(l, r), Token: e.Token}
			}
		case "==":
			return &ast.BooleanLiteral{Value: l == r, Token: e.Token}
		case "!=":
			return &ast.BooleanLiteral{Value: l != r, Token: e.Token}
		case "<":
			return &ast.BooleanLiteral{Value: l < r, Token: e.Token}
		case "<=":
			return &ast.BooleanLiteral{Value: l <= r, Token: e.Token}
		case ">":
			return &ast.BooleanLiteral{Value: l > r, Token: e.Token}
		case ">=":
			return &ast.BooleanLiteral{Value: l >= r, Token: e.Token}
		}
	}

	// Constant string concatenation folding: "foo" + "bar" -> "foobar"
	leftStr, leftIsStr := e.Left.(*ast.StringLiteral)
	rightStr, rightIsStr := e.Right.(*ast.StringLiteral)
	if leftIsStr && rightIsStr && e.Operator == "+" {
		return &ast.StringLiteral{Value: leftStr.Value + rightStr.Value, Token: e.Token}
	}

	// Constant boolean logical folding
	leftBool, leftIsBool := e.Left.(*ast.BooleanLiteral)
	rightBool, rightIsBool := e.Right.(*ast.BooleanLiteral)
	if leftIsBool && rightIsBool {
		switch e.Operator {
		case "and":
			return &ast.BooleanLiteral{Value: leftBool.Value && rightBool.Value, Token: e.Token}
		case "or":
			return &ast.BooleanLiteral{Value: leftBool.Value || rightBool.Value, Token: e.Token}
		case "==":
			return &ast.BooleanLiteral{Value: leftBool.Value == rightBool.Value, Token: e.Token}
		case "!=":
			return &ast.BooleanLiteral{Value: leftBool.Value != rightBool.Value, Token: e.Token}
		}
	}

	return e
}

// OptimizePeephole is intentionally the identity transformation.
//
// Jump operands in Alca bytecode are absolute byte offsets. The previous
// implementation shortened the instruction stream (SetLocal+GetLocal -> Dup+SetLocal,
// removal of jump-to-next) without relocating those offsets, which silently
// corrupted control flow for any jump crossing an optimized site. Constant folding
// and dead-branch elimination are done safely on the AST instead (OptimizeAST).
func (opt *Optimizer) OptimizePeephole(ins Instructions) Instructions {
	return ins
}
