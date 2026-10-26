// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

// Package vm provides compile-time AST and bytecode peephole optimization passes for Alca.
package vm

import (
	"github.com/alca-lang/alca/pkg/interpreter"

	"github.com/alca-lang/alca/pkg/ast"
)

// Optimizer applies constant folding, dead code elimination, and peephole optimizations.
type Optimizer struct {
	inlineable map[string]*ast.FuncDeclaration
}

// NewOptimizer creates a new Optimizer instance.
func NewOptimizer() *Optimizer {
	return &Optimizer{}
}

// OptimizeAST walks an AST program tree and applies compile-time constant folding,
// dead code elimination, and function inlining.
func (opt *Optimizer) OptimizeAST(program *ast.Program) *ast.Program {
	if program == nil {
		return nil
	}
	// Collect inlineable functions first (including nested ones) so that a
	// function can be inlined even when it is defined after its call site.
	opt.inlineable = make(map[string]*ast.FuncDeclaration)
	shadowed := make(map[string]bool)
	collectShadowedNames(program, shadowed)
	collectInlineable(program, opt.inlineable, shadowed)

	optimizedStmts := make([]ast.Statement, 0, len(program.Statements))
	for _, stmt := range program.Statements {
		optStmt := opt.optimizeStatement(stmt)
		if optStmt != nil {
			optimizedStmts = append(optimizedStmts, optStmt)
		}
	}
	return &ast.Program{Statements: optimizedStmts}
}

// collectInlineable walks the program and records functions whose body is a single
// pure return expression (no calls, no assignments, no side effects). Such
// functions can be inlined at call sites without changing observable behaviour.
func collectInlineable(node ast.Node, out map[string]*ast.FuncDeclaration, shadowed map[string]bool) {
	if node == nil {
		return
	}
	switch n := node.(type) {
	case *ast.Program:
		for _, s := range n.Statements {
			collectInlineable(s, out, shadowed)
		}
	case *ast.FuncDeclaration:
		if n.Body != nil {
			if len(n.Body.Statements) == 1 {
				if ret, ok := n.Body.Statements[0].(*ast.ReturnStatement); ok && ret.Value != nil {
					if !shadowed[n.Name] && !containsCall(ret.Value) && !containsAssign(ret.Value) && onlyReferencesParams(ret.Value, n.Params) {
						out[n.Name] = n
					}
				}
			}
			collectInlineable(n.Body, out, shadowed)
		}
	case *ast.BlockStatement:
		for _, s := range n.Statements {
			collectInlineable(s, out, shadowed)
		}
	case *ast.IfStatement:
		if n == nil {
			return
		}
		if n.Consequence != nil {
			collectInlineable(n.Consequence, out, shadowed)
		}
		if n.ElseBlock != nil {
			collectInlineable(n.ElseBlock, out, shadowed)
		}
		for _, alt := range n.Alternatives {
			if alt != nil {
				collectInlineable(alt, out, shadowed)
			}
		}
	case *ast.WhileStatement:
		if n == nil {
			return
		}
		if n.Body != nil {
			collectInlineable(n.Body, out, shadowed)
		}
	case *ast.ForStatement:
		if n == nil {
			return
		}
		if n.Body != nil {
			collectInlineable(n.Body, out, shadowed)
		}
	case *ast.ExpressionStatement:
		if n == nil {
			return
		}
		collectInlineable(n.Expression, out, shadowed)
	case *ast.VarDeclaration:
		if n == nil {
			return
		}
		if n.Value != nil {
			collectInlineable(n.Value, out, shadowed)
		}
	case *ast.ReturnStatement:
		if n == nil {
			return
		}
		if n.Value != nil {
			collectInlineable(n.Value, out, shadowed)
		}
	}
}

func containsCall(expr ast.Expression) bool {
	if expr == nil {
		return false
	}
	switch e := expr.(type) {
	case *ast.CallExpression:
		return true
	case *ast.InfixExpression:
		return containsCall(e.Left) || containsCall(e.Right)
	case *ast.PrefixExpression:
		return containsCall(e.Right)
	case *ast.ListLiteral:
		for _, el := range e.Elements {
			if containsCall(el) {
				return true
			}
		}
	case *ast.MapLiteral:
		for _, p := range e.Pairs {
			if containsCall(p.Key) || containsCall(p.Value) {
				return true
			}
		}
	case *ast.IndexExpression:
		return containsCall(e.Left) || containsCall(e.Index)
	}
	return false
}

func containsAssign(expr ast.Expression) bool {
	if expr == nil {
		return false
	}
	switch e := expr.(type) {
	case *ast.AssignExpression:
		return true
	case *ast.InfixExpression:
		return containsAssign(e.Left) || containsAssign(e.Right)
	case *ast.PrefixExpression:
		return containsAssign(e.Right)
	case *ast.ListLiteral:
		for _, el := range e.Elements {
			if containsAssign(el) {
				return true
			}
		}
	case *ast.MapLiteral:
		for _, p := range e.Pairs {
			if containsAssign(p.Key) || containsAssign(p.Value) {
				return true
			}
		}
	case *ast.IndexExpression:
		return containsAssign(e.Left) || containsAssign(e.Index)
	}
	return false
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
		if s.Body != nil {
			if optBody, ok := opt.optimizeStatement(s.Body).(*ast.BlockStatement); ok {
				s.Body = optBody
			}
		}
		return s

	case *ast.ForStatement:
		if s.Iterable != nil {
			s.Iterable = opt.optimizeExpression(s.Iterable)
		}
		if s.Body != nil {
			if optBody, ok := opt.optimizeStatement(s.Body).(*ast.BlockStatement); ok {
				s.Body = optBody
			}
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

	case *ast.AssignExpression:
		e.Target = opt.optimizeExpression(e.Target)
		e.Value = opt.optimizeExpression(e.Value)
		return e

	case *ast.CallExpression:
		e.Function = opt.optimizeExpression(e.Function)
		for i, arg := range e.Arguments {
			e.Arguments[i] = opt.optimizeExpression(arg)
		}
		// Inline simple, pure functions at call sites.
		if id, ok := e.Function.(*ast.Identifier); ok {
			if fn, ok := opt.inlineable[id.Value]; ok && len(e.Arguments) == len(fn.Params) {
				ret := fn.Body.Statements[0].(*ast.ReturnStatement)
				return opt.substituteParams(ret.Value, fn.Params, e.Arguments)
			}
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
		case "===":
			return &ast.BooleanLiteral{Value: l == r, Token: e.Token}
		case "!==":
			return &ast.BooleanLiteral{Value: l != r, Token: e.Token}
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

	// Constant float arithmetic folding
	leftFloat, leftIsFloat := e.Left.(*ast.FloatLiteral)
	rightFloat, rightIsFloat := e.Right.(*ast.FloatLiteral)
	if leftIsFloat && rightIsFloat {
		l := leftFloat.Value
		r := rightFloat.Value
		switch e.Operator {
		case "+":
			return &ast.FloatLiteral{Value: l + r, Token: e.Token}
		case "-":
			return &ast.FloatLiteral{Value: l - r, Token: e.Token}
		case "*":
			return &ast.FloatLiteral{Value: l * r, Token: e.Token}
		case "/":
			if r != 0 {
				return &ast.FloatLiteral{Value: l / r, Token: e.Token}
			}
		case "==":
			return &ast.BooleanLiteral{Value: l == r, Token: e.Token}
		case "===":
			return &ast.BooleanLiteral{Value: l == r, Token: e.Token}
		case "!==":
			return &ast.BooleanLiteral{Value: l != r, Token: e.Token}
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

	// Constant string concatenation & comparison folding
	leftStr, leftIsStr := e.Left.(*ast.StringLiteral)
	rightStr, rightIsStr := e.Right.(*ast.StringLiteral)
	if leftIsStr && rightIsStr {
		switch e.Operator {
		case "+":
			return &ast.StringLiteral{Value: leftStr.Value + rightStr.Value, Token: e.Token}
		case "==":
			return &ast.BooleanLiteral{Value: leftStr.Value == rightStr.Value, Token: e.Token}
		case "===":
			return &ast.BooleanLiteral{Value: leftStr.Value == rightStr.Value, Token: e.Token}
		case "!==":
			return &ast.BooleanLiteral{Value: leftStr.Value != rightStr.Value, Token: e.Token}
		case "!=":
			return &ast.BooleanLiteral{Value: leftStr.Value != rightStr.Value, Token: e.Token}
		}
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
		case "===":
			return &ast.BooleanLiteral{Value: leftBool.Value == rightBool.Value, Token: e.Token}
		case "!==":
			return &ast.BooleanLiteral{Value: leftBool.Value != rightBool.Value, Token: e.Token}
		case "!=":
			return &ast.BooleanLiteral{Value: leftBool.Value != rightBool.Value, Token: e.Token}
		}
	}

	return e
}

// substituteParams returns a copy of expr with each parameter identifier replaced
// by the corresponding argument expression. The tree is rebuilt so the original
// (shared) return expression is never mutated.
func (opt *Optimizer) substituteParams(expr ast.Expression, params []*ast.FuncParam, args []ast.Expression) ast.Expression {
	if expr == nil {
		return nil
	}
	switch e := expr.(type) {
	case *ast.Identifier:
		for i, p := range params {
			if p.Name == e.Value {
				// Share the (already optimized) argument read-only.
				return args[i]
			}
		}
		return e
	case *ast.IntegerLiteral, *ast.FloatLiteral, *ast.StringLiteral, *ast.BooleanLiteral, *ast.NullLiteral:
		return e
	case *ast.InfixExpression:
		return &ast.InfixExpression{
			Token:    e.Token,
			Operator: e.Operator,
			Left:     opt.substituteParams(e.Left, params, args),
			Right:    opt.substituteParams(e.Right, params, args),
		}
	case *ast.PrefixExpression:
		return &ast.PrefixExpression{
			Token:    e.Token,
			Operator: e.Operator,
			Right:    opt.substituteParams(e.Right, params, args),
		}
	case *ast.IndexExpression:
		return &ast.IndexExpression{
			Token: e.Token,
			Left:  opt.substituteParams(e.Left, params, args),
			Index: opt.substituteParams(e.Index, params, args),
		}
	case *ast.ListLiteral:
		out := &ast.ListLiteral{Token: e.Token}
		for _, el := range e.Elements {
			out.Elements = append(out.Elements, opt.substituteParams(el, params, args))
		}
		return out
	default:
		return e
	}
}

type peepholeInst struct {
	op         Opcode
	operands   []int
	origOffset int
	origLen    int
	kept       bool
	newOffset  int
}

func jumpTargetOperandIndex(op Opcode) int {
	switch op {
	case OpJump, OpJumpIfFalse, OpTryBegin:
		return 0
	case OpLoopGlobalLtInc, OpLoopGlobalLeInc, OpLoopLocalLtInc, OpLoopLocalLeInc:
		return 2
	case OpLoopRangeLocal, OpLoopRangeGlobal:
		return 3
	case OpLoopListLocal, OpLoopListGlobal:
		return 4
	default:
		return -1
	}
}

// OptimizePeephole safely optimizes the bytecode stream with full jump-target relocation.
// It removes redundant jumps, eliminates OpDup+OpPop, and converts OpSetLocal+OpGetLocal into OpDup+OpSetLocal.
func (opt *Optimizer) OptimizePeephole(ins Instructions) Instructions {
	if len(ins) == 0 {
		return ins
	}

	var decoded []*peepholeInst
	offset := 0
	for offset < len(ins) {
		op := Opcode(ins[offset])
		def, err := Lookup(byte(op))
		if err != nil {
			return ins // on any unknown opcode, don't modify
		}
		operands, read := ReadOperands(def, ins[offset+1:])
		origLen := 1 + read
		decoded = append(decoded, &peepholeInst{
			op:         op,
			operands:   operands,
			origOffset: offset,
			origLen:    origLen,
			kept:       true,
		})
		offset += origLen
	}

	// Collect all jump targets in original offsets
	jumpTargets := make(map[int]bool)
	for _, inst := range decoded {
		targetIdx := jumpTargetOperandIndex(inst.op)
		if targetIdx >= 0 && targetIdx < len(inst.operands) {
			jumpTargets[inst.operands[targetIdx]] = true
		}
	}

	changed := false
	for i := 0; i < len(decoded); i++ {
		inst := decoded[i]
		if !inst.kept {
			continue
		}

		// Pattern 1: Redundant OpJump to immediately following instruction
		if inst.op == OpJump && len(inst.operands) > 0 {
			nextOffset := inst.origOffset + inst.origLen
			if inst.operands[0] == nextOffset {
				inst.kept = false
				changed = true
				continue
			}
		}

		if i+1 < len(decoded) && decoded[i+1].kept {
			nextInst := decoded[i+1]

			// Pattern 2: OpSetLocal X followed by OpGetLocal X -> OpDup + OpSetLocal X
			if inst.op == OpSetLocal && nextInst.op == OpGetLocal &&
				len(inst.operands) > 0 && len(nextInst.operands) > 0 &&
				inst.operands[0] == nextInst.operands[0] &&
				!jumpTargets[nextInst.origOffset] {

				localIdx := inst.operands[0]
				inst.op = OpDup
				inst.operands = nil
				nextInst.op = OpSetLocal
				nextInst.operands = []int{localIdx}
				changed = true
			}

			// Pattern 3: OpDup followed by OpPop -> eliminate both
			if inst.op == OpDup && nextInst.op == OpPop && !jumpTargets[nextInst.origOffset] {
				inst.kept = false
				nextInst.kept = false
				changed = true
			}
		}
	}

	if !changed {
		return ins
	}

	// Compute new offsets for all kept instructions
	newOffset := 0
	for _, inst := range decoded {
		if !inst.kept {
			continue
		}
		inst.newOffset = newOffset
		def, _ := Lookup(byte(inst.op))
		instLen := 1
		for _, w := range def.OperandWidths {
			instLen += w
		}
		newOffset += instLen
	}

	// Build relocation map: origOffset -> newOffset
	targetMap := make(map[int]int)
	targetMap[len(ins)] = newOffset
	nextNew := newOffset
	for i := len(decoded) - 1; i >= 0; i-- {
		d := decoded[i]
		if d.kept {
			nextNew = d.newOffset
		}
		targetMap[d.origOffset] = nextNew
	}

	// Relocate jump targets and assemble output bytecode
	out := make([]byte, 0, newOffset)
	for _, inst := range decoded {
		if !inst.kept {
			continue
		}
		targetIdx := jumpTargetOperandIndex(inst.op)
		if targetIdx >= 0 && targetIdx < len(inst.operands) {
			oldTarget := inst.operands[targetIdx]
			if mapped, ok := targetMap[oldTarget]; ok {
				inst.operands[targetIdx] = mapped
			}
		}
		bytes, err := MakeChecked(inst.op, inst.operands...)
		if err != nil {
			return ins
		}
		out = append(out, bytes...)
	}

	return out
}

func onlyReferencesParams(expr ast.Expression, params []*ast.FuncParam) bool {
	if expr == nil {
		return true
	}
	paramSet := make(map[string]bool, len(params))
	for _, p := range params {
		paramSet[p.Name] = true
	}
	var walk func(ast.Expression) bool
	walk = func(e ast.Expression) bool {
		if e == nil {
			return true
		}
		switch v := e.(type) {
		case *ast.Identifier:
			return paramSet[v.Value]
		case *ast.IntegerLiteral, *ast.FloatLiteral, *ast.StringLiteral, *ast.BooleanLiteral, *ast.NullLiteral:
			return true
		case *ast.InfixExpression:
			return walk(v.Left) && walk(v.Right)
		case *ast.PrefixExpression:
			return walk(v.Right)
		case *ast.IndexExpression:
			return walk(v.Left) && walk(v.Index)
		case *ast.ListLiteral:
			for _, el := range v.Elements {
				if !walk(el) {
					return false
				}
			}
			return true
		default:
			return false
		}
	}
	return walk(expr)
}

// collectShadowedNames records every name that is bound by something other than
// a function declaration: a local variable, an assignment target, a parameter
// or a loop variable. A function whose name appears here may be shadowed at some
// call site, so it must not be inlined.
func collectShadowedNames(node ast.Node, out map[string]bool) {
	if node == nil {
		return
	}
	switch n := node.(type) {
	case *ast.Program:
		if n == nil {
			return
		}
		for _, s := range n.Statements {
			collectShadowedNames(s, out)
		}
	case *ast.BlockStatement:
		if n == nil {
			return
		}
		for _, s := range n.Statements {
			collectShadowedNames(s, out)
		}
	case *ast.FuncDeclaration:
		if n == nil {
			return
		}
		for _, p := range n.Params {
			out[p.Name] = true
		}
		if n.Body != nil {
			collectShadowedNames(n.Body, out)
		}
	case *ast.VarDeclaration:
		if n == nil {
			return
		}
		if n.Name != nil {
			out[n.Name.Value] = true
		}
		if n.Value != nil {
			collectShadowedNames(n.Value, out)
		}
	case *ast.AssignExpression:
		if n == nil {
			return
		}
		if name := assignmentRootName(n.Target); name != "" {
			out[name] = true
		}
		if n.Value != nil {
			collectShadowedNames(n.Value, out)
		}
	case *ast.DotExpression:
		if n != nil {
			collectShadowedNames(n.Left, out)
		}
	case *ast.IndexExpression:
		if n != nil {
			collectShadowedNames(n.Left, out)
			collectShadowedNames(n.Index, out)
		}
	case *ast.InfixExpression:
		if n != nil {
			collectShadowedNames(n.Left, out)
			collectShadowedNames(n.Right, out)
		}
	case *ast.PrefixExpression:
		if n != nil {
			collectShadowedNames(n.Right, out)
		}
	case *ast.CallExpression:
		if n != nil {
			collectShadowedNames(n.Function, out)
			for _, arg := range n.Arguments {
				collectShadowedNames(arg, out)
			}
			for _, arg := range n.KwArgs {
				collectShadowedNames(arg, out)
			}
		}
	case *ast.ListLiteral:
		if n != nil {
			for _, element := range n.Elements {
				collectShadowedNames(element, out)
			}
		}
	case *ast.MapLiteral:
		if n != nil {
			for _, pair := range n.Pairs {
				if pair != nil {
					collectShadowedNames(pair.Key, out)
					collectShadowedNames(pair.Value, out)
				}
			}
		}
	case *ast.ForStatement:
		if n == nil {
			return
		}
		if n.Variable != "" {
			out[n.Variable] = true
		}
		if n.Body != nil {
			collectShadowedNames(n.Body, out)
		}
	case *ast.IfStatement:
		if n == nil {
			return
		}
		if n.Consequence != nil {
			collectShadowedNames(n.Consequence, out)
		}
		if n.ElseBlock != nil {
			collectShadowedNames(n.ElseBlock, out)
		}
		for _, alt := range n.Alternatives {
			if alt != nil {
				collectShadowedNames(alt, out)
			}
		}

	case *ast.WhileStatement:
		if n == nil {
			return
		}
		if n.Body != nil {
			collectShadowedNames(n.Body, out)
		}
	case *ast.ReturnStatement:
		if n == nil {
			return
		}
		if n.Value != nil {
			collectShadowedNames(n.Value, out)
		}
	case *ast.ExpressionStatement:
		if n == nil {
			return
		}
		if n.Expression != nil {
			collectShadowedNames(n.Expression, out)
		}
	}
}

func assignmentRootName(target ast.Expression) string {
	switch n := target.(type) {
	case *ast.Identifier:
		return n.Value
	case *ast.DotExpression:
		return assignmentRootName(n.Left)
	case *ast.IndexExpression:
		return assignmentRootName(n.Left)
	default:
		return ""
	}
}
