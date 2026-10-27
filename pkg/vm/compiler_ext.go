// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package vm

import (
	"fmt"

	"github.com/alca-lang/alca/pkg/ast"
	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/token"
)

// UnsupportedError reports a language feature the bytecode backend cannot compile.
// Callers fall back to the tree-walking interpreter for such programs.
type UnsupportedError struct {
	Feature string
	Line    int
}

func (e *UnsupportedError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("line %d: %s is not supported by the VM backend", e.Line, e.Feature)
	}
	return fmt.Sprintf("%s is not supported by the VM backend", e.Feature)
}

func unsupported(feature string, tok token.Token) error {
	return &UnsupportedError{Feature: feature, Line: tok.Line}
}

// lookup resolves a variable through the lexical scopes. A variable local to an
// enclosing function is captured as an upvalue of the current function (see
// SymbolTable.Resolve): the VM creates closures over live stack slots, matching
// the interpreter's by-reference closures.
func (c *Compiler) lookup(name string) (Symbol, bool) {
	return c.symbolTable.Resolve(name)
}

func (c *Compiler) emitSet(sym Symbol) {
	switch sym.Scope {
	case GlobalScope:
		c.emit(OpSetGlobal, sym.Index)
	case FreeScope:
		c.emit(OpSetFree, sym.Index)
	default:
		c.emit(OpSetLocal, sym.Index)
	}
}

func (c *Compiler) emitGet(sym Symbol) {
	switch sym.Scope {
	case GlobalScope:
		c.emit(OpGetGlobal, sym.Index)
	case BuiltinScope:
		c.emit(OpGetBuiltin, sym.Index)
	case FreeScope:
		c.emit(OpGetFree, sym.Index)
	default:
		c.emit(OpGetLocal, sym.Index)
	}
}

// canInPlace reports whether a target symbol supports the specialised in-place
// arithmetic opcodes. Upvalues need the generic read/modify/write path because
// the captured slot may live in another frame or be closed over.
func canInPlace(sym Symbol) bool {
	return sym.Scope == GlobalScope || sym.Scope == LocalScope
}

// captureDescriptors turns the free symbols recorded while compiling a function
// body into the per-upvalue capture descriptions used by OpClosure.
func captureDescriptors(free []Symbol) []UpvalueDesc {
	if len(free) == 0 {
		return nil
	}
	descs := make([]UpvalueDesc, len(free))
	for i, sym := range free {
		descs[i] = UpvalueDesc{Index: sym.Index, IsLocal: sym.Scope == LocalScope}
	}
	return descs
}

// emitFunctionValue pushes a compiled function as a value: OpClosure when the
// function captures upvalues of the current frame, plain OpConstant otherwise.
func (c *Compiler) emitFunctionValue(fn *CompiledFunction) {
	idx := c.addConstant(fn)
	if fn.NumFree > 0 {
		c.emit(OpClosure, idx)
		return
	}
	c.emit(OpConstant, idx)
}

// predeclareLocals defines every variable bound anywhere in a function body
// before the body is compiled. All blocks share the function scope, nested
// functions are compiled first (hoisting), and the interpreter resolves names at
// run time: pre-declaring keeps slot indices stable and lets hoisted functions
// capture locals that are declared textually later in the body.
func (c *Compiler) predeclareLocals(body *ast.BlockStatement) {
	if body == nil {
		return
	}
	for _, st := range body.Statements {
		c.predeclareStatement(st)
	}
}

func (c *Compiler) predeclareStatement(st ast.Statement) {
	switch n := st.(type) {
	case *ast.VarDeclaration:
		if n.Name != nil {
			if n.Visibility == "public" {
				// Public declarations live in the program-wide table, never in
				// the enclosing function's local slots. Declaring a local here
				// would make references resolve to an uninitialized slot.
				c.symbolTable.DefineGlobal(n.Name.Value)
			} else {
				c.symbolTable.Define(n.Name.Value)
			}
		}
	case *ast.FuncDeclaration:
		c.symbolTable.Define(n.Name)
	case *ast.StructDeclaration:
		c.symbolTable.Define(n.Name)
	case *ast.EnumDeclaration:
		c.symbolTable.Define(n.Name)
	case *ast.ForStatement:
		if n.Variable != "" {
			c.symbolTable.Define(n.Variable)
		}
		c.predeclareLocals(n.Body)
	case *ast.ForEachStatement:
		if n.Variable != "" {
			c.symbolTable.Define(n.Variable)
		}
		c.predeclareLocals(n.Body)
	case *ast.WhileStatement:
		c.predeclareLocals(n.Body)
	case *ast.LoopStatement:
		c.predeclareLocals(n.Body)
	case *ast.IfStatement:
		c.predeclareLocals(n.Consequence)
		for _, alt := range n.Alternatives {
			if alt != nil {
				c.predeclareLocals(alt.Body)
			}
		}
		c.predeclareLocals(n.ElseBlock)
	case *ast.MatchStatement:
		for _, cs := range n.Cases {
			if cs != nil {
				c.predeclareLocals(cs.Body)
			}
		}
		c.predeclareLocals(n.Default)
	}
}

func (c *Compiler) emitJump(op Opcode) int { return c.emit(op, 9999) }

// patch points the jump at pos to the current end of the instruction stream.
func (c *Compiler) patch(pos int) {
	target := len(c.currentInstructions())
	if target > 0xFFFF {
		if c.err == nil {
			c.err = fmt.Errorf("program too large: jump target %d exceeds 65535 bytes of bytecode in one function", target)
		}
	}
	c.changeOperand(pos, target)
}

func isDecl(st ast.Statement) bool {
	switch st.(type) {
	case *ast.FuncDeclaration, *ast.StructDeclaration, *ast.ClassDeclaration, *ast.EnumDeclaration, *ast.InterfaceDeclaration:
		return true
	}
	return false
}

// compileStatements compiles a statement list. Function and struct declarations are
// emitted first so they can be called before their textual position (the type checker
// allows this).
func (c *Compiler) compileStatements(stmts []ast.Statement) error {
	for _, st := range stmts {
		if isDecl(st) {
			if err := c.Compile(st); err != nil {
				return err
			}
		}
	}
	for _, st := range stmts {
		if !isDecl(st) {
			if err := c.Compile(st); err != nil {
				return err
			}
		}
	}
	return c.err
}

// collectDecls records top-level functions/structs (used to bind keyword/default arguments)
// and pre-declares top-level variables so hoisted function bodies resolve them as globals.
func (c *Compiler) collectDecls(stmts []ast.Statement) error {
	for _, st := range stmts {
		switch d := st.(type) {
		case *ast.FuncDeclaration:
			if d.IsShell {
				return unsupported("shell functions (shfunc)", d.Token)
			}
			c.funcs[d.Name] = d
			c.symbolTable.Define(d.Name)
		case *ast.StructDeclaration:
			c.structs[d.Name] = d
			c.symbolTable.Define(d.Name)
		case *ast.ClassDeclaration:
			c.symbolTable.Define(d.Name)
		case *ast.EnumDeclaration:
			c.symbolTable.Define(d.Name)
		case *ast.VarDeclaration:
			if d.Name != nil {
				c.symbolTable.Define(d.Name.Value)
			}
		}
	}
	return nil
}

// compileExt handles the node kinds that need stricter or additional semantics than the
// legacy switch in Compile. It reports handled=false for everything else.
func (c *Compiler) compileExt(node ast.Node) (bool, error) {
	switch n := node.(type) {

	case *ast.Program:
		prog := c.optimizer.OptimizeAST(n)
		c.mathAliases = make(map[string]bool)
		c.stdlibAliases = make(map[string]string)
		shadowedNames := make(map[string]bool)
		collectShadowedNames(prog, shadowedNames)
		for _, stmt := range prog.Statements {
			use, ok := stmt.(*ast.UseStatement)
			if !ok || len(use.Imports) > 0 {
				continue
			}
			alias := use.Alias
			if alias == "" {
				alias = StdlibBindingName(use.ModulePath)
			}
			if alias == "" || shadowedNames[alias] || moduleAliasEscapes(prog, alias) {
				continue
			}
			if use.ModulePath == "math" || use.ModulePath == "std.math" {
				c.mathAliases[alias] = true
			}
			if SupportsPureStdlibMembers(use.ModulePath) {
				c.stdlibAliases[alias] = use.ModulePath
			}
		}
		if err := c.collectDecls(prog.Statements); err != nil {
			return true, err
		}
		return true, c.compileStatements(prog.Statements)

	case *ast.BlockStatement:
		return true, c.compileStatements(n.Statements)

	case *ast.FuncDeclaration:
		if n.IsShell {
			return true, unsupported("shell functions (shfunc)", n.Token)
		}
		if n.Body != nil {
			// nested function declarations need their own binding info
			if _, known := c.funcs[n.Name]; !known {
				c.funcs[n.Name] = n
			}
		}
		return false, nil

	case *ast.IfStatement:
		return true, c.compileIf(n)

	case *ast.MatchStatement:
		return true, c.Compile(c.desugarMatch(n))

	case *ast.ExpressionStatement:
		if se, ok := n.Expression.(*ast.StreamExpression); ok && (se.Operator == ">>>" || se.Operator == "?>>>") {
			return true, c.compileCapture(se)
		}
		// Stack discipline is decided by the node kind, never by peeking at the last
		// emitted opcode: assignments are stack-neutral, every other expression leaves
		// exactly one value that must be popped.
		if as, ok := n.Expression.(*ast.AssignExpression); ok {
			c.assignStmt = as
			return true, c.Compile(as)
		}
		if err := c.Compile(n.Expression); err != nil {
			return true, err
		}
		c.emit(OpPop)
		return true, nil

	case *ast.AssignExpression:
		if c.assignStmt != n {
			// The assignment is used as a value. The set opcodes consume the
			// value they store, so the value is duplicated first and the
			// assignment leaves its result on the stack.
			c.assignAsValue = true
			return false, nil
		}
		c.assignStmt = nil
		return false, nil

	case *ast.InfixExpression:
		if n.Operator == "and" || n.Operator == "or" {
			return true, c.compileLogical(n)
		}
		return false, nil

	case *ast.StreamExpression:
		if n.Operator == "?>>" {
			return true, c.compileTryStream(n)
		}
		if n.Operator == ">>>" || n.Operator == "?>>>" {
			return true, unsupported("capture stream ('"+n.Operator+"') used as a value", n.Token)
		}
		return false, nil

	case *ast.CallExpression:
		args, err := c.bindArgs(n)
		if err != nil {
			return true, err
		}
		if len(n.KwArgs) == 0 {
			if dot, ok := n.Function.(*ast.DotExpression); ok {
				root, memberPath := moduleCallPath(dot)
				if root != nil {
					if _, shadowed := c.symbolTable.Resolve(root.Value); !shadowed {
						if c.mathAliases[root.Value] {
							if id, arity, supported := MathBuiltinID(memberPath); supported && len(args) == arity {
								for _, arg := range args {
									if err := c.Compile(arg); err != nil {
										return true, err
									}
								}
								c.emit(OpMathBuiltin, int(id))
								return true, nil
							}
						}
						// Pure module calls are fused so the VM and JIT engines
						// can invoke the registered member without materializing
						// the module map and doing a generic field lookup.
						if modulePath, ok := c.stdlibAliases[root.Value]; ok && IsPureStdlibMember(modulePath, memberPath) {
							for _, arg := range args {
								if err := c.Compile(arg); err != nil {
									return true, err
								}
							}
							aliasIdx := c.addConstant(&interpreter.StringValue{Val: root.Value})
							memberIdx := c.addConstant(&interpreter.StringValue{Val: memberPath})
							c.emit(OpStdlibCall, aliasIdx, memberIdx, len(args))
							return true, nil
						}
					}
				}
			}
		}
		// A bare call to an always-available conversion builtin that does not
		// resolve to a user binding lowers to a boxing-free opcode: the runtime
		// name lookup the VM would otherwise do always selects the builtin.
		if id, ok := n.Function.(*ast.Identifier); ok && len(args) == 1 {
			if _, bound := c.symbolTable.Resolve(id.Value); !bound {
				if bid, ok := builtin1ID(id.Value); ok {
					// Fused conversion of a plain local/global operand.
					if arg, ok := args[0].(*ast.Identifier); ok {
						if sym, ok := c.symbolTable.Resolve(arg.Value); ok {
							switch sym.Scope {
							case LocalScope:
								c.emit(OpConvLocal, bid, sym.Index)
								return true, nil
							case GlobalScope:
								c.emit(OpConvGlobal, bid, sym.Index)
								return true, nil
							}
						}
					}
					if err := c.Compile(args[0]); err != nil {
						return true, err
					}
					c.emit(OpCallBuiltin1, bid)
					return true, nil
				}
			}
		}
		if err := c.Compile(n.Function); err != nil {
			return true, err
		}
		for _, a := range args {
			if err := c.Compile(a); err != nil {
				return true, err
			}
		}
		c.emit(OpCall, len(args))
		return true, nil

	case *ast.Identifier:
		if n.Value == "__line__" {
			lineVal := int64(n.Token.Line)
			if lineVal <= 0 {
				lineVal = 1
			}
			idx := c.addConstant(&interpreter.IntegerValue{Val: lineVal})
			c.emit(OpConstant, idx)
			return true, nil
		}
		sym, ok := c.lookup(n.Value)
		if !ok {
			c.emit(OpGetBuiltin, c.addConstant(&interpreter.StringValue{Val: n.Value}))
			return true, nil
		}
		c.emitGet(sym)
		return true, nil

	case *ast.ErrExpression:
		if n.Message != nil {
			if err := c.Compile(n.Message); err != nil {
				return true, err
			}
		} else {
			c.emit(OpConstant, c.addConstant(&interpreter.StringValue{Val: "error"}))
		}
		if n.Code != nil {
			if err := c.Compile(n.Code); err != nil {
				return true, err
			}
		} else {
			c.emit(OpNull)
		}
		c.emit(OpMakeError)
		c.emit(OpRaise)
		c.emit(OpNull) // unreachable; keeps the expression's stack effect uniform
		return true, nil

	case *ast.AssertStatement:
		if err := c.Compile(n.Expression); err != nil {
			return true, err
		}
		msg := fmt.Sprintf("assertion failed (line %d)", n.Token.Line)
		c.emit(OpAssert, c.addConstant(&interpreter.StringValue{Val: msg}))
		return true, nil

	case *ast.TestStatement:
		return true, c.compileTest(n)

	case *ast.PipeExpression:
		if err := c.Compile(n.Right); err != nil {
			return true, err
		}
		if err := c.Compile(n.Left); err != nil {
			return true, err
		}
		c.emit(OpCall, 1)
		return true, nil

	case *ast.ConsoleLiteral:
		c.emit(OpGetBuiltin, c.addConstant(&interpreter.StringValue{Val: "input"}))
		argc := 0
		if n.Prompt != nil {
			if err := c.Compile(n.Prompt); err != nil {
				return true, err
			}
			argc = 1
		}
		c.emit(OpCall, argc)
		return true, nil

	case *ast.LambdaExpression:
		return true, c.compileLambda(n)

	case *ast.BaStatement:
		c.emit(OpGetBuiltin, c.addConstant(&interpreter.StringValue{Val: "@ba"}))
		if err := c.compileBlockClosure(n.Body); err != nil {
			return true, err
		}
		c.emit(OpCall, 1)
		c.emit(OpPop)
		return true, nil

	case *ast.BaForStatement:
		return true, unsupported("'ba for' loops", n.Token)

	case *ast.ThreadsStatement:
		c.emit(OpGetBuiltin, c.addConstant(&interpreter.StringValue{Val: "@threads"}))
		count := 0
		if n.Body != nil {
			for _, s := range n.Body.Statements {
				if s == nil {
					continue
				}
				if err := c.compileBlockClosure(&ast.BlockStatement{Statements: []ast.Statement{s}}); err != nil {
					return true, err
				}
				count++
			}
		}
		c.emit(OpCall, count)
		c.emit(OpPop)
		return true, nil

	case *ast.TxStatement:
		return true, c.compileTx(n)
	case *ast.WatchStatement:
		return true, unsupported("'watch' blocks", n.Token)
	case *ast.MockStatement:
		return true, c.compileMock(n)
	case *ast.WaitStatement:
		c.emit(OpWait)
		return true, nil
	case *ast.TimeoutExpression:
		c.emit(OpGetBuiltin, c.addConstant(&interpreter.StringValue{Val: "@timeout"}))
		inner := n.Expression
		if inner == nil {
			inner = n.Expr
		}
		lambda := &ast.LambdaExpression{
			Token: n.Token,
			Body:  inner,
		}
		if err := c.compileLambda(lambda); err != nil {
			return true, err
		}
		dur := n.Duration
		if dur == nil {
			dur = &ast.IntegerLiteral{Value: 0}
		}
		if err := c.Compile(dur); err != nil {
			return true, err
		}
		c.emit(OpCall, 2)
		return true, nil
	case *ast.StructInstantiation:
		return true, c.compileStructInstantiation(n)
	case *ast.ClassDeclaration:
		return true, c.compileClass(n)
	case *ast.EnumDeclaration:
		variants := make(map[string]*interpreter.EnumValue)
		enumDef := &interpreter.EnumDef{
			Name:     n.Name,
			Variants: variants,
		}
		for idx, v := range n.Variants {
			var tag int64 = int64(idx)
			if v.Value != nil {
				if intVal, ok := v.Value.(*ast.IntegerLiteral); ok {
					tag = intVal.Value
				}
			}
			ev := &interpreter.EnumValue{
				EnumName: n.Name,
				Name:     v.Name,
				Tag:      tag,
			}
			variants[v.Name] = ev
		}
		constIdx := c.addConstant(enumDef)
		c.emit(OpConstant, constIdx)
		symbol, ok := c.symbolTable.Resolve(n.Name)
		if !ok {
			symbol = c.symbolTable.Define(n.Name)
		}
		if symbol.Scope == GlobalScope {
			c.emit(OpSetGlobal, symbol.Index)
		} else {
			c.emit(OpSetLocal, symbol.Index)
		}
		return true, nil
	case *ast.InterfaceDeclaration:
		return true, nil
	}
	return false, nil
}

// compileStructInstantiation compiles the `Name(args)` form of a struct
// creation. It is a call of the struct value with the given arguments.
func (c *Compiler) compileStructInstantiation(n *ast.StructInstantiation) error {
	symbol, ok := c.symbolTable.Resolve(n.Name)
	if !ok {
		return unsupported(fmt.Sprintf("struct %q is not defined", n.Name), n.Token)
	}
	c.emitGet(symbol)
	for _, arg := range n.Args {
		if err := c.Compile(arg); err != nil {
			return err
		}
	}
	c.emit(OpCall, len(n.Args))
	return c.err
}

// compileIf compiles if / elif / else as a stack-neutral statement.
func (c *Compiler) compileIf(n *ast.IfStatement) error {
	if len(n.Alternatives) > 0 {
		n = desugarElif(n)
	}
	if err := c.Compile(n.Condition); err != nil {
		return err
	}
	jumpFalse := c.emitJump(OpJumpIfFalse)
	if err := c.Compile(n.Consequence); err != nil {
		return err
	}
	if n.ElseBlock == nil {
		c.patch(jumpFalse)
		return c.err
	}
	jumpEnd := c.emitJump(OpJump)
	c.patch(jumpFalse)
	if err := c.Compile(n.ElseBlock); err != nil {
		return err
	}
	c.patch(jumpEnd)
	return c.err
}

// desugarMatch turns a match statement into an if/elif chain over a hidden temporary.
func (c *Compiler) desugarMatch(n *ast.MatchStatement) ast.Statement {
	c.tmpCounter++
	name := fmt.Sprintf("@match_%d", c.tmpCounter)
	tmp := &ast.Identifier{Token: n.Token, Value: name}
	decl := &ast.VarDeclaration{Token: n.Token, Name: tmp, Value: n.Subject}

	var chain *ast.IfStatement
	for i, cs := range n.Cases {
		cond := &ast.InfixExpression{Token: cs.Token, Left: tmp, Operator: "==", Right: cs.Pattern}
		if i == 0 {
			chain = &ast.IfStatement{Token: n.Token, Condition: cond, Consequence: cs.Body, ElseBlock: n.Default}
		} else {
			chain.Alternatives = append(chain.Alternatives, &ast.ElifClause{Token: cs.Token, Condition: cond, Body: cs.Body})
		}
	}
	stmts := []ast.Statement{decl}
	if chain != nil {
		stmts = append(stmts, chain)
	} else if n.Default != nil {
		stmts = append(stmts, n.Default)
	}
	return &ast.BlockStatement{Token: n.Token, Statements: stmts}
}

// compileCapture compiles "expr >>> var" and "expr ?>>> var" as statements.
func (c *Compiler) compileCapture(se *ast.StreamExpression) error {
	id, ok := se.Target.(*ast.Identifier)
	if !ok {
		return unsupported("capture stream target that is not a variable", se.Token)
	}
	sym, found := c.lookup(id.Value)
	if !found {
		sym = c.symbolTable.Define(id.Value)
	}
	c.assignedNames[id.Value] = true
	if se.Operator == ">>>" {
		if err := c.Compile(se.Source); err != nil {
			return err
		}
		c.emitSet(sym)
		return c.err
	}
	// ?>>> : run the source; if it raises, store the error value in the variable.
	try := c.emitJump(OpTryBegin)
	if err := c.Compile(se.Source); err != nil {
		return err
	}
	c.emit(OpPop)
	c.emit(OpEndTry)
	skip := c.emitJump(OpJump)
	c.patch(try)
	c.emitSet(sym) // the unwinder pushed the error value
	c.patch(skip)
	return c.err
}

// compileTryStream compiles "src ?>> target": on error, append the error to target.
func (c *Compiler) compileTryStream(n *ast.StreamExpression) error {
	try := c.emitJump(OpTryBegin)
	if err := c.Compile(n.Source); err != nil {
		return err
	}
	c.emit(OpEndTry)
	skip := c.emitJump(OpJump)
	c.patch(try)
	if err := c.Compile(n.Target); err != nil {
		return err
	}
	c.emit(OpStream, 5)
	c.patch(skip)
	return c.err
}

// compileTest runs a test body under an error handler and reports PASS/FAIL.
func (c *Compiler) compileTest(n *ast.TestStatement) error {
	name := c.addConstant(&interpreter.StringValue{Val: n.Name})
	try := c.emitJump(OpTryBegin)
	if err := c.Compile(n.Body); err != nil {
		return err
	}
	c.emit(OpEndTry)
	c.emit(OpTestPass, name)
	skip := c.emitJump(OpJump)
	c.patch(try)
	c.emit(OpTestFail, name)
	c.patch(skip)
	return c.err
}

func (c *Compiler) compileLambda(n *ast.LambdaExpression) error {
	c.enterScope()
	for _, p := range n.Params {
		c.symbolTable.Define(p.Name)
	}
	if err := c.Compile(n.Body); err != nil {
		return err
	}
	c.emit(OpReturnValue)
	numLocals := c.symbolTable.numDefinitions
	free := c.symbolTable.FreeSymbols
	ins := c.leaveScope()
	var defaults []interpreter.Value
	for _, p := range n.Params {
		if p.DefaultValue != nil {
			defaults = append(defaults, interpreter.New().Eval(p.DefaultValue))
		}
	}
	fn := &CompiledFunction{
		Instructions:  ins,
		NumLocals:     numLocals,
		NumParameters: len(n.Params),
		Defaults:      defaults,
		Name:          "<lambda>",
		NumFree:       len(free),
		UpvalueDescs:  captureDescriptors(free),
	}
	c.emitFunctionValue(fn)
	return c.err
}

func (c *Compiler) compileBlockClosure(block *ast.BlockStatement) error {
	c.enterScope()
	if block != nil {
		for _, s := range block.Statements {
			if err := c.Compile(s); err != nil {
				return err
			}
		}
	}
	c.emit(OpReturn)
	numLocals := c.symbolTable.numDefinitions
	free := c.symbolTable.FreeSymbols
	ins := c.leaveScope()
	fn := &CompiledFunction{
		Instructions:  ins,
		NumLocals:     numLocals,
		NumParameters: 0,
		Name:          "<closure>",
		NumFree:       len(free),
		UpvalueDescs:  captureDescriptors(free),
	}
	c.emitFunctionValue(fn)
	return c.err
}

// bindArgs resolves keyword arguments and default values for calls to known
// user functions and struct constructors, producing a positional argument list.
func (c *Compiler) bindArgs(n *ast.CallExpression) ([]ast.Expression, error) {
	type param struct {
		name string
		def  ast.Expression
	}
	var params []param
	if id, ok := n.Function.(*ast.Identifier); ok {
		if f, ok := c.funcs[id.Value]; ok {
			for _, p := range f.Params {
				params = append(params, param{p.Name, p.DefaultValue})
			}
		} else if st, ok := c.structs[id.Value]; ok {
			for _, f := range st.Fields {
				params = append(params, param{f.Name, f.DefaultValue})
			}
		}
	}
	if params == nil {
		if len(n.KwArgs) > 0 {
			return nil, unsupported("keyword arguments in this call", n.Token)
		}
		return n.Arguments, nil
	}
	if len(n.Arguments) > len(params) {
		return n.Arguments, nil
	}
	out := make([]ast.Expression, len(params))
	copy(out, n.Arguments)
	for k, v := range n.KwArgs {
		idx := -1
		for i, p := range params {
			if p.name == k {
				idx = i
			}
		}
		if idx < 0 || out[idx] != nil {
			return nil, unsupported(fmt.Sprintf("unknown or duplicate keyword argument '%s'", k), n.Token)
		}
		out[idx] = v
	}
	// fill defaults; trailing positions without a value are an error at runtime (arity)
	last := -1
	for i := range out {
		if out[i] == nil {
			out[i] = params[i].def
		}
		if out[i] != nil {
			last = i
		}
	}
	for i := 0; i <= last; i++ {
		if out[i] == nil {
			return nil, unsupported("call with a missing required argument", n.Token)
		}
	}
	return out[:last+1], nil
}

// compileLogical compiles short-circuit 'and' / 'or' (operands are bool by the type checker).
func (c *Compiler) compileLogical(n *ast.InfixExpression) error {
	if err := c.Compile(n.Left); err != nil {
		return err
	}
	jumpFalse := c.emitJump(OpJumpIfFalse)
	if n.Operator == "and" {
		if err := c.Compile(n.Right); err != nil {
			return err
		}
		jumpEnd := c.emitJump(OpJump)
		c.patch(jumpFalse)
		c.emit(OpFalse)
		c.patch(jumpEnd)
		return c.err
	}
	c.emit(OpTrue)
	jumpEnd := c.emitJump(OpJump)
	c.patch(jumpFalse)
	if err := c.Compile(n.Right); err != nil {
		return err
	}
	c.patch(jumpEnd)
	return c.err
}
