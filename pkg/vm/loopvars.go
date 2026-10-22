package vm

import "github.com/alca-lang/alca/pkg/ast"

// writesLoopVariable reports whether compiling/executing the loop body could
// assign to the named loop variable, including from nested functions and
// lambdas (which may be invoked while the loop is active and capture the
// variable by reference). The counter fast path in compileForRangeDynamic is
// only sound when this returns false: otherwise the body's writes would change the
// iteration count, while the interpreter keeps iterating a fixed item list.
func writesLoopVariable(body *ast.BlockStatement, name string) bool {
	if body == nil {
		return false
	}
	for _, st := range body.Statements {
		if statementWritesVariable(st, name) {
			return true
		}
	}
	return false
}

func statementWritesVariable(st ast.Statement, name string) bool {
	switch n := st.(type) {
	case *ast.ExpressionStatement:
		return expressionWritesVariable(n.Expression, name)
	case *ast.VarDeclaration:
		if n.Name != nil && n.Name.Value == name {
			return true
		}
		return expressionWritesVariable(n.Value, name)
	case *ast.BlockStatement:
		return writesLoopVariable(n, name)
	case *ast.IfStatement:
		if expressionWritesVariable(n.Condition, name) || writesLoopVariable(n.Consequence, name) {
			return true
		}
		for _, alt := range n.Alternatives {
			if alt != nil && (expressionWritesVariable(alt.Condition, name) || writesLoopVariable(alt.Body, name)) {
				return true
			}
		}
		return writesLoopVariable(n.ElseBlock, name)
	case *ast.WhileStatement:
		return expressionWritesVariable(n.Condition, name) || writesLoopVariable(n.Body, name)
	case *ast.ForStatement:
		if n.Variable == name {
			return true
		}
		return expressionWritesVariable(n.Iterable, name) || writesLoopVariable(n.Body, name)
	case *ast.ForEachStatement:
		if n.Variable == name {
			return true
		}
		return expressionWritesVariable(n.Iterable, name) || writesLoopVariable(n.Body, name)
	case *ast.LoopStatement:
		return writesLoopVariable(n.Body, name)
	case *ast.ReturnStatement:
		return expressionWritesVariable(n.Value, name)
	case *ast.MatchStatement:
		if expressionWritesVariable(n.Subject, name) {
			return true
		}
		for _, cs := range n.Cases {
			if cs != nil && (expressionWritesVariable(cs.Pattern, name) || writesLoopVariable(cs.Body, name)) {
				return true
			}
		}
		return writesLoopVariable(n.Default, name)
	case *ast.FuncDeclaration:
		return writesLoopVariable(n.Body, name)
	case *ast.StructDeclaration:
		return false
	case *ast.ClassDeclaration:
		return false
	case *ast.BreakStatement, *ast.ContinueStatement, *ast.UseStatement:
		return false
	case *ast.AssertStatement:
		return expressionWritesVariable(n.Expression, name)
	case *ast.TestStatement:
		return writesLoopVariable(n.Body, name)
	default:
		// Unsupported/unknown statement kinds are conservatively reported as writes.
		return true
	}
}

func expressionWritesVariable(expr ast.Expression, name string) bool {
	switch e := expr.(type) {
	case nil:
		return false
	case *ast.Identifier, *ast.IntegerLiteral, *ast.FloatLiteral, *ast.StringLiteral,
		*ast.BooleanLiteral, *ast.NullLiteral, *ast.ConsoleLiteral, *ast.TypeAnnotation:
		return false
	case *ast.PrefixExpression:
		return expressionWritesVariable(e.Right, name)
	case *ast.RangeLiteral:
		return expressionWritesVariable(e.Start, name) || expressionWritesVariable(e.End, name)
	case *ast.InfixExpression:
		return expressionWritesVariable(e.Left, name) || expressionWritesVariable(e.Right, name)
	case *ast.ListLiteral:
		for _, el := range e.Elements {
			if expressionWritesVariable(el, name) {
				return true
			}
		}
		return false
	case *ast.MapLiteral:
		for _, p := range e.Pairs {
			if p != nil && (expressionWritesVariable(p.Key, name) || expressionWritesVariable(p.Value, name)) {
				return true
			}
		}
		return false
	case *ast.CallExpression:
		if expressionWritesVariable(e.Function, name) {
			return true
		}
		for _, a := range e.Arguments {
			if expressionWritesVariable(a, name) {
				return true
			}
		}
		for _, v := range e.KwArgs {
			if expressionWritesVariable(v, name) {
				return true
			}
		}
		return false
	case *ast.IndexExpression:
		return expressionWritesVariable(e.Left, name) || expressionWritesVariable(e.Index, name)
	case *ast.DotExpression:
		return expressionWritesVariable(e.Left, name)
	case *ast.AssignExpression:
		if id, ok := e.Target.(*ast.Identifier); ok && id.Value == name {
			return true
		}
		return expressionWritesVariable(e.Target, name) || expressionWritesVariable(e.Value, name)
	case *ast.StreamExpression:
		if id, ok := e.Target.(*ast.Identifier); ok && id.Value == name {
			return true
		}
		return true
	case *ast.PipeExpression:
		return expressionWritesVariable(e.Left, name) || expressionWritesVariable(e.Right, name)
	case *ast.TimeoutExpression:
		return expressionWritesVariable(e.Expression, name)
	case *ast.LambdaExpression:
		return expressionWritesVariable(e.Body, name)
	case *ast.ErrExpression:
		return expressionWritesVariable(e.Message, name) || expressionWritesVariable(e.Code, name)
	case *ast.StructInstantiation:
		for _, a := range e.Args {
			if expressionWritesVariable(a, name) {
				return true
			}
		}
		return false
	default:
		// Unknown expression kinds are conservatively reported as writes.
		return true
	}
}
