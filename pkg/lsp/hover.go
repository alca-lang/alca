package lsp

import (
	"sort"
	"strings"

	"github.com/alca-lang/alca/pkg/ast"
	"github.com/alca-lang/alca/pkg/typecheck"
)

// nodesOf converts a typed node slice into []ast.Node, dropping nils.
func nodesOf[T ast.Node](items []T) []ast.Node {
	out := make([]ast.Node, 0, len(items))
	for _, it := range items {
		if any(it) != nil {
			out = append(out, it)
		}
	}
	return out
}

// childrenOf returns the direct children of an AST node in source order.
func childrenOf(n ast.Node) []ast.Node {
	var out []ast.Node
	add := func(items ...ast.Node) {
		for _, c := range items {
			if c != nil {
				out = append(out, c)
			}
		}
	}
	switch x := n.(type) {
	case *ast.Program:
		add(nodesOf(x.Statements)...)
	case *ast.BlockStatement:
		add(nodesOf(x.Statements)...)
	case *ast.ExpressionStatement:
		add(x.Expression)
	case *ast.VarDeclaration:
		add(nodesOf(x.Decorators)...)
		add(x.Name, x.TypeAnnotation, x.Value)
	case *ast.FuncDeclaration:
		add(nodesOf(x.Decorators)...)
		add(nodesOf(x.Params)...)
		add(x.ReturnType, x.Body)
	case *ast.FuncParam:
		add(x.TypeAnnotation, x.DefaultValue)
	case *ast.StructDeclaration:
		add(nodesOf(x.Decorators)...)
		add(nodesOf(x.Fields)...)
	case *ast.StructField:
		add(x.TypeAnnotation, x.DefaultValue)
	case *ast.IfStatement:
		add(x.Condition, x.Consequence)
		add(nodesOf(x.Alternatives)...)
		add(x.ElseBlock)
	case *ast.ElifClause:
		add(x.Condition, x.Body)
	case *ast.WhileStatement:
		add(x.Condition, x.Body)
	case *ast.ForStatement:
		add(x.Iterable, x.Body)
	case *ast.ForEachStatement:
		add(x.Iterable, x.Body)
	case *ast.LoopStatement:
		add(x.Body)
	case *ast.ReturnStatement:
		add(x.Value)
	case *ast.UseStatement:
		// no child expressions
	case *ast.WatchStatement:
		add(x.Target, x.Body)
	case *ast.TestStatement:
		add(x.Body)
	case *ast.MockStatement:
		add(x.Target, x.Replacement)
	case *ast.BaStatement:
		add(x.Timeout, x.Limit, x.Body)
	case *ast.BaForStatement:
		add(x.Iterable, x.Limit, x.Body)
	case *ast.ThreadsStatement:
		add(x.Body)
	case *ast.TxStatement:
		add(x.DB, x.Timeout, x.Body)
	case *ast.AssertStatement:
		add(x.Expression)
	case *ast.Decorator:
		add(nodesOf(x.Args)...)
	case *ast.MatchStatement:
		add(x.Subject)
		add(nodesOf(x.Cases)...)
		add(x.Default)
	case *ast.MatchCase:
		add(x.Pattern, x.Body)
	case *ast.ListLiteral:
		add(nodesOf(x.Elements)...)
	case *ast.MapLiteral:
		add(nodesOf(x.Pairs)...)
	case *ast.MapPair:
		add(x.Key, x.Value)
	case *ast.PrefixExpression:
		add(x.Right)
	case *ast.InfixExpression:
		add(x.Left, x.Right)
	case *ast.CallExpression:
		add(x.Function)
		add(nodesOf(x.Arguments)...)
		keys := make([]string, 0, len(x.KwArgs))
		for k := range x.KwArgs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			add(x.KwArgs[k])
		}
	case *ast.IndexExpression:
		add(x.Left, x.Index)
	case *ast.DotExpression:
		add(x.Left)
	case *ast.AssignExpression:
		add(x.Target, x.Value)
	case *ast.StreamExpression:
		add(x.Source, x.Target)
	case *ast.PipeExpression:
		add(x.Left, x.Right)
	case *ast.TimeoutExpression:
		add(x.Expression, x.Duration)
	case *ast.ConsoleLiteral:
		add(x.Prompt)
	case *ast.LambdaExpression:
		add(nodesOf(x.Params)...)
		add(x.Body)
	case *ast.ErrExpression:
		add(x.Message, x.Code)
	case *ast.RangeLiteral:
		add(x.Start, x.End)
	case *ast.StructInstantiation:
		add(nodesOf(x.Args)...)
	}
	return out
}

// nodeAt returns the innermost AST node that starts at or before the given
// 1-based position, preferring the node whose first token is closest to the
// cursor. parent is the direct parent of the returned node, when known.
func nodeAt(prog *ast.Program, line, col int) (ast.Node, ast.Node) {
	var best, bestParent ast.Node
	bestLine, bestCol := 0, 0

	var walk func(n, parent ast.Node)
	walk = func(n, parent ast.Node) {
		if n == nil {
			return
		}
		l, c := ast.NodePosition(n)
		// A node without a recorded position cannot be selected, but its
		// children may still have one: never prune the subtree. Likewise a
		// node whose first recorded token is after the cursor may still
		// contain a child that starts before it (the token of a call is its
		// "(" and the token of a member access is its ".").
		if l != 0 || c != 0 {
			if l < line || (l == line && c <= col) {
				if best == nil || l > bestLine || (l == bestLine && c >= bestCol) {
					best, bestParent = n, parent
					bestLine, bestCol = l, c
				}
			}
		}
		for _, ch := range childrenOf(n) {
			walk(ch, n)
		}
	}
	for _, st := range prog.Statements {
		walk(st, nil)
	}
	return best, bestParent
}

// alcaCode renders a markdown code fence for Alca source snippets.
func alcaCode(s string) string {
	return "```alca\n" + s + "\n```"
}

// hoverDescription renders the hover markdown for a 1-based position, or ""
// when nothing meaningful is under the cursor.
func hoverDescription(prog *ast.Program, checker *typecheck.Checker, line, col int) string {
	node, parent := nodeAt(prog, line, col)
	if node == nil {
		return ""
	}

	switch n := node.(type) {
	case *ast.Identifier:
		// A variable's name sits inside its declaration; prefer the
		// declaration so the hover shows the declared (or inferred) type.
		if vd, ok := parent.(*ast.VarDeclaration); ok && vd.Name == n {
			return describeVar(n.Value, vd, checker)
		}
		// The field name of a member access resolves through its receiver.
		if dot, ok := parent.(*ast.DotExpression); ok && dot.Field == n.Value {
			return describeDot(dot, checker)
		}
		return describeIdentifier(n, checker)
	case *ast.DotExpression:
		return describeDot(n, checker)
	case *ast.IntegerLiteral:
		return alcaCode("int")
	case *ast.FloatLiteral:
		return alcaCode("float")
	case *ast.StringLiteral:
		return alcaCode("string")
	case *ast.BooleanLiteral:
		return alcaCode("bool")
	case *ast.NullLiteral:
		return alcaCode("null")
	case *ast.VarDeclaration:
		return describeVar(n.Name.Value, n, checker)
	case *ast.FuncDeclaration:
		return alcaCode("func " + funcSignatureText(n))
	case *ast.StructDeclaration:
		return alcaCode("struct " + n.Name)
	}

	if expr, ok := node.(ast.Expression); ok && checker != nil {
		if t := checker.TypeOf(expr); t != nil {
			return alcaCode(t.String())
		}
	}
	return ""
}

// describeVar renders "name: type" for a variable declaration.
func describeVar(name string, vd *ast.VarDeclaration, checker *typecheck.Checker) string {
	if vd.TypeAnnotation != nil {
		return alcaCode(name + ": " + vd.TypeAnnotation.String())
	}
	if checker != nil && vd.Value != nil {
		if t := checker.TypeOf(vd.Value); t != nil {
			return alcaCode(name + ": " + t.String())
		}
	}
	return alcaCode(name)
}

// describeIdentifier renders the type of a named symbol.
func describeIdentifier(n *ast.Identifier, checker *typecheck.Checker) string {
	if checker == nil {
		return alcaCode(n.Value)
	}
	t := checker.TypeOf(n)
	if mt, ok := t.(*typecheck.ModuleType); ok {
		return alcaCode(n.Value + ": module " + mt.Name)
	}
	if t != nil {
		return alcaCode(n.Value + ": " + t.String())
	}
	return alcaCode(n.Value)
}

// describeDot renders a member access: std module members and struct fields
// are resolved through the type tables.
func describeDot(n *ast.DotExpression, checker *typecheck.Checker) string {
	if checker == nil {
		return alcaCode(n.Field)
	}
	left := checker.TypeOf(n.Left)
	if mt, ok := left.(*typecheck.ModuleType); ok {
		if t, ok := mt.Members[n.Field]; ok {
			return alcaCode(mt.Name + "." + n.Field + ": " + t.String())
		}
		return alcaCode(n.Field) + "\nUnknown member of standard module `" + mt.Name + "`."
	}
	if st, ok := left.(*typecheck.StructType); ok {
		if f, ok := st.Field(n.Field); ok {
			text := st.Name + "." + n.Field + ": " + f.Type.String()
			if f.HasDefault {
				text += " = <default>"
			}
			return alcaCode(text)
		}
		return alcaCode(n.Field) + "\nUnknown field of struct `" + st.Name + "`."
	}
	if t := checker.TypeOf(n); t != nil {
		return alcaCode(n.Field + ": " + t.String())
	}
	return alcaCode(n.Field)
}

// funcSignatureText renders "name(param: type, ...) -> ret" for a declared
// function.
func funcSignatureText(f *ast.FuncDeclaration) string {
	var params []string
	for _, p := range f.Params {
		s := p.Name
		if p.TypeAnnotation != nil {
			s += ": " + p.TypeAnnotation.String()
		}
		if p.DefaultValue != nil {
			s += " = " + p.DefaultValue.String()
		}
		params = append(params, s)
	}
	sig := f.Name + "(" + strings.Join(params, ", ") + ")"
	if f.ReturnType != nil {
		sig += " -> " + f.ReturnType.String()
	}
	return sig
}
