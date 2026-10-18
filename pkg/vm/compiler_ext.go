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

// lookup resolves a variable and rejects captures of another function's locals
// (closures are not implemented; silently reading the wrong frame slot would be a miscompile).
func (c *Compiler) lookup(name string, tok token.Token) (Symbol, bool, error) {
	depth := 0
	for t := c.symbolTable; t != nil; t = t.Outer {
		if sym, ok := t.store[name]; ok {
			if sym.Scope == LocalScope && depth > 0 {
				return sym, true, unsupported(fmt.Sprintf("capturing the local variable '%s' in a nested function/lambda", name), tok)
			}
			return sym, true, nil
		}
		depth++
	}
	return Symbol{}, false, nil
}

func (c *Compiler) emitSet(sym Symbol) {
	if sym.Scope == GlobalScope {
		c.emit(OpSetGlobal, sym.Index)
	} else {
		c.emit(OpSetLocal, sym.Index)
	}
}

func (c *Compiler) emitJump(op Opcode) int { return c.emit(op, 9999) }

// patch points the jump at pos to the current end of the instruction stream.
func (c *Compiler) patch(pos int) {
	target := len(c.currentInstructions())
	if target > 0xFFFF {
		c.err = fmt.Errorf("program too large: jump target %d exceeds 65535 bytes of bytecode in one function", target)
	}
	c.changeOperand(pos, target)
}

func isDecl(st ast.Statement) bool {
	switch st.(type) {
	case *ast.FuncDeclaration, *ast.StructDeclaration:
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
			if len(d.Decorators) > 0 {
				return unsupported("function decorators", d.Token)
			}
			if d.IsShell {
				return unsupported("shell functions (shfunc)", d.Token)
			}
			c.funcs[d.Name] = d
			c.symbolTable.Define(d.Name)
		case *ast.StructDeclaration:
			c.structs[d.Name] = d
			c.symbolTable.Define(d.Name)
		case *ast.VarDeclaration:
			if len(d.Decorators) > 0 {
				return unsupported("variable decorators", d.Token)
			}
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
		if err := c.collectDecls(prog.Statements); err != nil {
			return true, err
		}
		return true, c.compileStatements(prog.Statements)

	case *ast.BlockStatement:
		return true, c.compileStatements(n.Statements)

	case *ast.FuncDeclaration:
		if len(n.Decorators) > 0 {
			return true, unsupported("function decorators", n.Token)
		}
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
			return true, unsupported("an assignment used as a value", n.Token)
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
		sym, ok, err := c.lookup(n.Value, n.Token)
		if err != nil {
			return true, err
		}
		if !ok {
			c.emit(OpGetBuiltin, c.addConstant(&interpreter.StringValue{Val: n.Value}))
			return true, nil
		}
		switch sym.Scope {
		case GlobalScope:
			c.emit(OpGetGlobal, sym.Index)
		case BuiltinScope:
			c.emit(OpGetBuiltin, sym.Index)
		default:
			c.emit(OpGetLocal, sym.Index)
		}
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
		return true, unsupported("'ba' blocks", n.Token)
	case *ast.BaForStatement:
		return true, unsupported("'ba for' loops", n.Token)
	case *ast.ThreadsStatement:
		return true, unsupported("'threads' blocks", n.Token)
	case *ast.TxStatement:
		return true, unsupported("'tx' blocks", n.Token)
	case *ast.WatchStatement:
		return true, unsupported("'watch' blocks", n.Token)
	case *ast.MockStatement:
		return true, unsupported("'mock'", n.Token)
	case *ast.WaitStatement:
		return true, unsupported("'wait'", n.Token)
	case *ast.TimeoutExpression:
		return true, unsupported("timeout expressions", n.Token)
	case *ast.StructInstantiation:
		return true, unsupported("struct instantiation node", n.Token)
	}
	return false, nil
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
	sym, found, err := c.lookup(id.Value, id.Token)
	if err != nil {
		return err
	}
	if !found {
		sym = c.symbolTable.Define(id.Value)
	}
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
	ins := c.leaveScope()
	fn := &CompiledFunction{Instructions: ins, NumLocals: numLocals, NumParameters: len(n.Params), Name: "<lambda>"}
	c.emit(OpConstant, c.addConstant(fn))
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
