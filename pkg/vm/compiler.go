// Package vm implements the Bytecode compiler for the Alca programming language.
package vm

import (
	"encoding/binary"
	"fmt"

	"github.com/alca-lang/alca/pkg/ast"
	"github.com/alca-lang/alca/pkg/interpreter"
)

// Bytecode packages compiled bytecode instructions and constant pool values.
type Bytecode struct {
	Instructions Instructions
	Constants    []interpreter.Value
}

// EmittedInstruction records an emitted opcode and its byte offset.
type EmittedInstruction struct {
	Opcode   Opcode
	Position int
}

// CompilationScope holds the bytecode instructions and state for a function scope.
type CompilationScope struct {
	instructions        Instructions
	lastInstruction     EmittedInstruction
	previousInstruction EmittedInstruction
}

type loopContext struct {
	breakJumps    []int
	continueJumps []int
}

// Compiler compiles an AST program into optimized bytecode instructions.
type Compiler struct {
	constants   []interpreter.Value
	symbolTable *SymbolTable
	scopes      []CompilationScope
	scopeIndex  int
	optimizer   *Optimizer
	loopStack   []*loopContext

	funcs      map[string]*ast.FuncDeclaration
	structs    map[string]*ast.StructDeclaration
	tmpCounter int
	assignStmt *ast.AssignExpression
	err        error // first deferred compile error (e.g. jump overflow)
}

// NewCompiler creates a new Compiler.
func NewCompiler() *Compiler {
	mainScope := CompilationScope{
		instructions:        Instructions{},
		lastInstruction:     EmittedInstruction{},
		previousInstruction: EmittedInstruction{},
	}

	return &Compiler{
		constants:   []interpreter.Value{},
		symbolTable: NewSymbolTable(),
		scopes:      []CompilationScope{mainScope},
		scopeIndex:  0,
		optimizer:   NewOptimizer(),
		loopStack:   []*loopContext{},
		funcs:       map[string]*ast.FuncDeclaration{},
		structs:     map[string]*ast.StructDeclaration{},
	}
}

func (c *Compiler) currentLoop() *loopContext {
	if len(c.loopStack) == 0 {
		return nil
	}
	return c.loopStack[len(c.loopStack)-1]
}

// Bytecode returns the finalized compiled bytecode after running peephole optimizations.
func (c *Compiler) Bytecode() *Bytecode {
	rawIns := c.currentInstructions()
	optIns := c.optimizer.OptimizePeephole(rawIns)

	return &Bytecode{
		Instructions: optIns,
		Constants:    c.constants,
	}
}

func (c *Compiler) currentInstructions() Instructions {
	return c.scopes[c.scopeIndex].instructions
}

func (c *Compiler) enterScope() {
	scope := CompilationScope{
		instructions:        Instructions{},
		lastInstruction:     EmittedInstruction{},
		previousInstruction: EmittedInstruction{},
	}
	c.scopes = append(c.scopes, scope)
	c.scopeIndex++
	c.symbolTable = NewEnclosedSymbolTable(c.symbolTable)
}

func (c *Compiler) leaveScope() Instructions {
	ins := c.currentInstructions()
	c.scopes = c.scopes[:len(c.scopes)-1]
	c.scopeIndex--
	if c.symbolTable.Outer != nil {
		c.symbolTable = c.symbolTable.Outer
	}
	return c.optimizer.OptimizePeephole(ins)
}

func (c *Compiler) addConstant(val interpreter.Value) int {
	c.constants = append(c.constants, val)
	return len(c.constants) - 1
}

func (c *Compiler) emit(op Opcode, operands ...int) int {
	ins := Make(op, operands...)
	pos := c.addInstruction(ins)

	c.setLastInstruction(op, pos)
	return pos
}

func (c *Compiler) addInstruction(ins []byte) int {
	posNewInstruction := len(c.currentInstructions())
	updatedInstructions := append(c.currentInstructions(), ins...)
	c.scopes[c.scopeIndex].instructions = updatedInstructions
	return posNewInstruction
}

func (c *Compiler) setLastInstruction(op Opcode, pos int) {
	previous := c.scopes[c.scopeIndex].lastInstruction
	last := EmittedInstruction{Opcode: op, Position: pos}

	c.scopes[c.scopeIndex].previousInstruction = previous
	c.scopes[c.scopeIndex].lastInstruction = last
}

func (c *Compiler) lastInstructionIs(op Opcode) bool {
	if len(c.currentInstructions()) == 0 {
		return false
	}
	return c.scopes[c.scopeIndex].lastInstruction.Opcode == op
}

func (c *Compiler) removeLastPop() {
	last := c.scopes[c.scopeIndex].lastInstruction
	previous := c.scopes[c.scopeIndex].previousInstruction

	old := c.currentInstructions()
	c.scopes[c.scopeIndex].instructions = old[:last.Position]
	c.scopes[c.scopeIndex].lastInstruction = previous
}

func (c *Compiler) changeOperand(opPos int, operand int) {
	op := Opcode(c.currentInstructions()[opPos])
	newInstruction := Make(op, operand)
	for i := 0; i < len(newInstruction); i++ {
		c.scopes[c.scopeIndex].instructions[opPos+i] = newInstruction[i]
	}
}

// Compile traverses an AST node and compiles it into bytecode.
func (c *Compiler) Compile(node ast.Node) error {
	if handled, err := c.compileExt(node); handled {
		return err
	}
	switch n := node.(type) {
	case *ast.Program:
		// Run AST constant folding and dead code elimination first
		optProgram := c.optimizer.OptimizeAST(n)
		for _, s := range optProgram.Statements {
			if err := c.Compile(s); err != nil {
				return err
			}
		}

	case *ast.ExpressionStatement:
		if err := c.Compile(n.Expression); err != nil {
			return err
		}
		if !c.lastInstructionIs(OpSetGlobal) && !c.lastInstructionIs(OpSetLocal) &&
			!c.lastInstructionIs(OpSetField) && !c.lastInstructionIs(OpSetIndex) &&
			!c.lastInstructionIs(OpIncGlobal) && !c.lastInstructionIs(OpIncLocal) {
			c.emit(OpPop)
		}

	case *ast.BlockStatement:
		for _, s := range n.Statements {
			if err := c.Compile(s); err != nil {
				return err
			}
		}

	case *ast.VarDeclaration:
		symbol, alreadyDefined := c.symbolTable.Resolve(n.Name.Value)
		if !alreadyDefined {
			symbol = c.symbolTable.Define(n.Name.Value)
		}

		if n.Value != nil {
			if alreadyDefined {
				if inf, ok := n.Value.(*ast.InfixExpression); ok && inf.Operator == "+" {
					var otherExpr ast.Expression
					// Only 'x = x + e' maps onto in-place add; 'x = e + x' differs for strings/lists.
					if idL, okL := inf.Left.(*ast.Identifier); okL && idL.Value == n.Name.Value {
						otherExpr = inf.Right
					}

					if otherExpr != nil {
						if intLit, okLit := otherExpr.(*ast.IntegerLiteral); okLit && intLit.Value == 1 {
							if symbol.Scope == GlobalScope {
								c.emit(OpIncGlobal, symbol.Index)
							} else {
								c.emit(OpIncLocal, symbol.Index)
							}
							return nil
						}
						if err := c.Compile(otherExpr); err != nil {
							return err
						}
						if symbol.Scope == GlobalScope {
							c.emit(OpAddGlobal, symbol.Index)
						} else {
							c.emit(OpAddLocal, symbol.Index)
						}
						return nil
					}
				} else if inf, ok := n.Value.(*ast.InfixExpression); ok && inf.Operator == "-" {
					if idL, okL := inf.Left.(*ast.Identifier); okL && idL.Value == n.Name.Value {
						if err := c.Compile(inf.Right); err != nil {
							return err
						}
						if symbol.Scope == GlobalScope {
							c.emit(OpSubGlobal, symbol.Index)
						} else {
							c.emit(OpSubLocal, symbol.Index)
						}
						return nil
					}
				}
			}

			if err := c.Compile(n.Value); err != nil {
				return err
			}
		} else {
			c.emit(OpNull)
		}

		if symbol.Scope == GlobalScope {
			c.emit(OpSetGlobal, symbol.Index)
		} else {
			c.emit(OpSetLocal, symbol.Index)
		}

	case *ast.ReturnStatement:
		if n.Value != nil {
			if err := c.Compile(n.Value); err != nil {
				return err
			}
			c.emit(OpReturnValue)
		} else {
			c.emit(OpReturn)
		}

	case *ast.IfStatement:
		if len(n.Alternatives) > 0 {
			return c.Compile(desugarElif(n))
		}
		if err := c.Compile(n.Condition); err != nil {
			return err
		}
		// Jump past consequence if false (placeholder address)
		jumpIfFalsePos := c.emit(OpJumpIfFalse, 9999)

		if err := c.Compile(n.Consequence); err != nil {
			return err
		}

		// Jump over alternative/else block
		jumpOverElsePos := c.emit(OpJump, 9999)

		afterConsequencePos := len(c.currentInstructions())
		c.changeOperand(jumpIfFalsePos, afterConsequencePos)

		if n.ElseBlock != nil {
			if err := c.Compile(n.ElseBlock); err != nil {
				return err
			}
		} else {
			c.emit(OpNull)
		}

		afterElsePos := len(c.currentInstructions())
		c.changeOperand(jumpOverElsePos, afterElsePos)

	case *ast.WhileStatement:
		return c.compileWhile(n)

	case *ast.ForStatement:
		return c.compileFor(n.Variable, n.Iterable, n.Body)

	case *ast.ForEachStatement:
		return c.compileFor(n.Variable, n.Iterable, n.Body)

	case *ast.LoopStatement:
		return c.compileLoop(n)

	case *ast.BreakStatement:
		if loop := c.currentLoop(); loop != nil {
			pos := c.emit(OpJump, 9999)
			loop.breakJumps = append(loop.breakJumps, pos)
		}

	case *ast.ContinueStatement:
		if loop := c.currentLoop(); loop != nil {
			pos := c.emit(OpJump, 9999)
			loop.continueJumps = append(loop.continueJumps, pos)
		}

	case *ast.UseStatement:
		idx := c.addConstant(&UseStatementValue{Stmt: n})
		c.emit(OpUse, idx)

	case *ast.StructDeclaration:
		sDef := &interpreter.StructDef{
			Name:   n.Name,
			Fields: n.Fields,
		}
		constIdx := c.addConstant(sDef)
		c.emit(OpConstant, constIdx)
		symbol := c.symbolTable.Define(n.Name)
		if symbol.Scope == GlobalScope {
			c.emit(OpSetGlobal, symbol.Index)
		} else {
			c.emit(OpSetLocal, symbol.Index)
		}

	case *ast.FuncDeclaration:
		fnSymbol := c.symbolTable.Define(n.Name)

		c.enterScope()
		for _, p := range n.Params {
			c.symbolTable.Define(p.Name)
		}

		if err := c.Compile(n.Body); err != nil {
			return err
		}

		if !c.lastInstructionIs(OpReturnValue) {
			c.emit(OpReturn)
		}

		numLocals := c.symbolTable.numDefinitions
		instructions := c.leaveScope()

		compiledFn := &CompiledFunction{
			Instructions:  instructions,
			NumLocals:     numLocals,
			NumParameters: len(n.Params),
			Name:          n.Name,
		}

		fnConstantIdx := c.addConstant(compiledFn)
		c.emit(OpConstant, fnConstantIdx)

		if fnSymbol.Scope == GlobalScope {
			c.emit(OpSetGlobal, fnSymbol.Index)
		} else {
			c.emit(OpSetLocal, fnSymbol.Index)
		}

	case *ast.IntegerLiteral:
		val := &interpreter.IntegerValue{Val: n.Value}
		c.emit(OpConstant, c.addConstant(val))

	case *ast.FloatLiteral:
		val := &interpreter.FloatValue{Val: n.Value}
		c.emit(OpConstant, c.addConstant(val))

	case *ast.StringLiteral:
		val := &interpreter.StringValue{Val: n.Value}
		c.emit(OpConstant, c.addConstant(val))

	case *ast.BooleanLiteral:
		if n.Value {
			c.emit(OpTrue)
		} else {
			c.emit(OpFalse)
		}

	case *ast.NullLiteral:
		c.emit(OpNull)

	case *ast.PrefixExpression:
		if err := c.Compile(n.Right); err != nil {
			return err
		}
		switch n.Operator {
		case "not", "!":
			c.emit(OpNot)
		case "-":
			c.emit(OpNegate)
		default:
			return fmt.Errorf("unknown prefix operator: %s", n.Operator)
		}

	case *ast.InfixExpression:
		if intLit, ok := n.Right.(*ast.IntegerLiteral); ok {
			switch n.Operator {
			case "%":
				if err := c.Compile(n.Left); err != nil {
					return err
				}
				constIdx := c.addConstant(&interpreter.IntegerValue{Val: intLit.Value})
				c.emit(OpModConst, constIdx)
				return nil
			case "*":
				if err := c.Compile(n.Left); err != nil {
					return err
				}
				constIdx := c.addConstant(&interpreter.IntegerValue{Val: intLit.Value})
				c.emit(OpMulConst, constIdx)
				return nil
			case "+":
				if err := c.Compile(n.Left); err != nil {
					return err
				}
				constIdx := c.addConstant(&interpreter.IntegerValue{Val: intLit.Value})
				c.emit(OpAddConst, constIdx)
				return nil
			}
		}

		if err := c.Compile(n.Left); err != nil {
			return err
		}
		if err := c.Compile(n.Right); err != nil {
			return err
		}

		switch n.Operator {
		case "+":
			c.emit(OpAdd)
		case "-":
			c.emit(OpSub)
		case "*":
			c.emit(OpMul)
		case "/":
			c.emit(OpDiv)
		case "%":
			c.emit(OpMod)
		case "**":
			c.emit(OpPow)
		case "==":
			c.emit(OpEqual)
		case "!=":
			c.emit(OpNotEqual)
		case ">":
			c.emit(OpGreaterThan)
		case ">=":
			c.emit(OpGreaterEqual)
		case "<":
			c.emit(OpLessThan)
		case "<=":
			c.emit(OpLessEqual)
		case "in":
			c.emit(OpIn)
		default:
			return fmt.Errorf("unknown infix operator: %s", n.Operator)
		}

	case *ast.Identifier:
		symbol, ok := c.symbolTable.Resolve(n.Value)
		if !ok {
			// Resolve as builtin/stdlib/environment symbol
			idx := c.addConstant(&interpreter.StringValue{Val: n.Value})
			c.emit(OpGetBuiltin, idx)
		} else {
			if symbol.Scope == GlobalScope {
				c.emit(OpGetGlobal, symbol.Index)
			} else if symbol.Scope == BuiltinScope {
				c.emit(OpGetBuiltin, symbol.Index)
			} else {
				c.emit(OpGetLocal, symbol.Index)
			}
		}

	case *ast.RangeLiteral:
		if err := c.Compile(n.Start); err != nil {
			return err
		}
		if err := c.Compile(n.End); err != nil {
			return err
		}
		c.emit(OpRange)

	case *ast.AssignExpression:
		if ident, ok := n.Target.(*ast.Identifier); ok {
			symbol, ok := c.symbolTable.Resolve(ident.Value)
			if !ok {
				symbol = c.symbolTable.Define(ident.Value)
			}

			// Compound assignment optimization: ident = ident + expr or ident = expr + ident
			if inf, ok := n.Value.(*ast.InfixExpression); ok && inf.Operator == "+" {
				var otherExpr ast.Expression
				// Only 'x = x + e' maps onto in-place add; 'x = e + x' differs for strings/lists.
				if idL, okL := inf.Left.(*ast.Identifier); okL && idL.Value == ident.Value {
					otherExpr = inf.Right
				}

				if otherExpr != nil {
					if intLit, okLit := otherExpr.(*ast.IntegerLiteral); okLit && intLit.Value == 1 {
						if symbol.Scope == GlobalScope {
							c.emit(OpIncGlobal, symbol.Index)
						} else {
							c.emit(OpIncLocal, symbol.Index)
						}
						return nil
					}
					if err := c.Compile(otherExpr); err != nil {
						return err
					}
					if symbol.Scope == GlobalScope {
						c.emit(OpAddGlobal, symbol.Index)
					} else {
						c.emit(OpAddLocal, symbol.Index)
					}
					return nil
				}
			} else if inf, ok := n.Value.(*ast.InfixExpression); ok && inf.Operator == "-" {
				if idL, okL := inf.Left.(*ast.Identifier); okL && idL.Value == ident.Value {
					if err := c.Compile(inf.Right); err != nil {
						return err
					}
					if symbol.Scope == GlobalScope {
						c.emit(OpSubGlobal, symbol.Index)
					} else {
						c.emit(OpSubLocal, symbol.Index)
					}
					return nil
				}
			}

			if err := c.Compile(n.Value); err != nil {
				return err
			}
			if symbol.Scope == GlobalScope {
				c.emit(OpSetGlobal, symbol.Index)
			} else {
				c.emit(OpSetLocal, symbol.Index)
			}
			return nil
		} else if dot, ok := n.Target.(*ast.DotExpression); ok {
			if err := c.Compile(n.Value); err != nil {
				return err
			}
			if err := c.Compile(dot.Left); err != nil {
				return err
			}
			nameIdx := c.addConstant(&interpreter.StringValue{Val: dot.Field})
			c.emit(OpSetField, nameIdx)
		} else if idxExpr, ok := n.Target.(*ast.IndexExpression); ok {
			if err := c.Compile(n.Value); err != nil {
				return err
			}
			if err := c.Compile(idxExpr.Left); err != nil {
				return err
			}
			if err := c.Compile(idxExpr.Index); err != nil {
				return err
			}
			c.emit(OpSetIndex)
		}

	case *ast.ListLiteral:
		for _, el := range n.Elements {
			if err := c.Compile(el); err != nil {
				return err
			}
		}
		c.emit(OpList, len(n.Elements))

	case *ast.MapLiteral:
		for _, pair := range n.Pairs {
			var keyStr string
			if ident, ok := pair.Key.(*ast.Identifier); ok {
				keyStr = ident.Value
			}
			if keyStr != "" {
				idx := c.addConstant(&interpreter.StringValue{Val: keyStr})
				c.emit(OpConstant, idx)
			} else {
				if err := c.Compile(pair.Key); err != nil {
					return err
				}
			}
			if err := c.Compile(pair.Value); err != nil {
				return err
			}
		}
		c.emit(OpMap, len(n.Pairs))

	case *ast.IndexExpression:
		if err := c.Compile(n.Left); err != nil {
			return err
		}
		if err := c.Compile(n.Index); err != nil {
			return err
		}
		c.emit(OpIndex)

	case *ast.DotExpression:
		if err := c.Compile(n.Left); err != nil {
			return err
		}
		nameIdx := c.addConstant(&interpreter.StringValue{Val: n.Field})
		c.emit(OpGetField, nameIdx)

	case *ast.CallExpression:
		if err := c.Compile(n.Function); err != nil {
			return err
		}
		for _, arg := range n.Arguments {
			if err := c.Compile(arg); err != nil {
				return err
			}
		}
		c.emit(OpCall, len(n.Arguments))

	case *ast.StreamExpression:
		if err := c.Compile(n.Source); err != nil {
			return err
		}
		if err := c.Compile(n.Target); err != nil {
			return err
		}

		var opByte byte
		switch n.Operator {
		case ">>":
			opByte = 1
		case ">!":
			opByte = 2
		case ">>>":
			opByte = 3
		case "<<":
			opByte = 4
		case "?>>":
			opByte = 5
		case "?>>>":
			opByte = 6
		case "&>>":
			opByte = 7
		case "&&>>":
			opByte = 8
		case "|>":
			opByte = 9
		default:
			opByte = 1
		}
		c.emit(OpStream, int(opByte))
	}

	return nil
}

// Helpers for encoding
func encodeUint16(val int) []byte {
	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, uint16(val))
	return buf
}

func (c *Compiler) compileWhile(n *ast.WhileStatement) error {
	// The former "counting while" fast path is removed: `continue` skipped the
	// increment (infinite loop) and it mis-handled non-int counters.
	if inf, ok := n.Condition.(*ast.InfixExpression); ok && inf.Operator == "<" && false {
		if id, okId := inf.Left.(*ast.Identifier); okId {
			if intLit, okLit := inf.Right.(*ast.IntegerLiteral); okLit {
				if symbol, okSym := c.symbolTable.Resolve(id.Value); okSym && symbol.Scope == GlobalScope {
					if n.Body != nil && len(n.Body.Statements) > 0 {
						lastStmt := n.Body.Statements[len(n.Body.Statements)-1]
						isCounterInc := false
						if expStmt, okExp := lastStmt.(*ast.ExpressionStatement); okExp {
							if assign, okAssign := expStmt.Expression.(*ast.AssignExpression); okAssign {
								if targetId, okT := assign.Target.(*ast.Identifier); okT && targetId.Value == id.Value {
									if addExp, okAdd := assign.Value.(*ast.InfixExpression); okAdd && addExp.Operator == "+" {
										if addL, okL := addExp.Left.(*ast.Identifier); okL && addL.Value == id.Value {
											if oneLit, ok1 := addExp.Right.(*ast.IntegerLiteral); ok1 && oneLit.Value == 1 {
												isCounterInc = true
											}
										}
									}
								}
							}
						} else if varDecl, okVar := lastStmt.(*ast.VarDeclaration); okVar && varDecl.Name != nil && varDecl.Name.Value == id.Value {
							if addExp, okAdd := varDecl.Value.(*ast.InfixExpression); okAdd && addExp.Operator == "+" {
								if addL, okL := addExp.Left.(*ast.Identifier); okL && addL.Value == id.Value {
									if oneLit, ok1 := addExp.Right.(*ast.IntegerLiteral); ok1 && oneLit.Value == 1 {
										isCounterInc = true
									}
								} else if addR, okR := addExp.Right.(*ast.Identifier); okR && addR.Value == id.Value {
									if oneLit, ok1 := addExp.Left.(*ast.IntegerLiteral); ok1 && oneLit.Value == 1 {
										isCounterInc = true
									}
								}
							}
						}

						if isCounterInc {
							return c.compileOptimizedWhileCountLoop(symbol.Index, intLit.Value, n.Body)
						}
					}
				}
			}
		}
	}

	ctx := &loopContext{}
	c.loopStack = append(c.loopStack, ctx)

	loopStartPos := len(c.currentInstructions())
	if err := c.Compile(n.Condition); err != nil {
		return err
	}
	jumpOutPos := c.emit(OpJumpIfFalse, 9999)

	if err := c.Compile(n.Body); err != nil {
		return err
	}

	c.emit(OpJump, loopStartPos)
	loopEndPos := len(c.currentInstructions())
	c.changeOperand(jumpOutPos, loopEndPos)

	for _, brk := range ctx.breakJumps {
		c.changeOperand(brk, loopEndPos)
	}
	for _, cnt := range ctx.continueJumps {
		c.changeOperand(cnt, loopStartPos)
	}

	c.loopStack = c.loopStack[:len(c.loopStack)-1]
	return nil
}

func (c *Compiler) compileOptimizedWhileCountLoop(varIndex int, limit int64, body *ast.BlockStatement) error {
	constIdx := c.addConstant(&interpreter.IntegerValue{Val: limit})

	// Initial check before loop: if var >= limit, jump to loopEnd
	c.emit(OpGetGlobal, varIndex)
	c.emit(OpConstant, constIdx)
	c.emit(OpLessThan)
	jumpOutPos := c.emit(OpJumpIfFalse, 9999)

	ctx := &loopContext{}
	c.loopStack = append(c.loopStack, ctx)

	bodyStartPos := len(c.currentInstructions())

	// Compile body statements EXCEPT the last one (which is counter = counter + 1)
	for i := 0; i < len(body.Statements)-1; i++ {
		if err := c.Compile(body.Statements[i]); err != nil {
			return err
		}
	}

	// Emit OpLoopGlobalLtInc: increments var and jumps back to bodyStart if var < limit!
	c.emit(OpLoopGlobalLtInc, varIndex, constIdx, bodyStartPos)

	loopEndPos := len(c.currentInstructions())
	c.changeOperand(jumpOutPos, loopEndPos)

	for _, brk := range ctx.breakJumps {
		c.changeOperand(brk, loopEndPos)
	}
	for _, cnt := range ctx.continueJumps {
		c.changeOperand(cnt, bodyStartPos)
	}

	c.loopStack = c.loopStack[:len(c.loopStack)-1]
	return nil
}

func (c *Compiler) compileForRange(variable string, start, end ast.Expression, body *ast.BlockStatement) error {
	// Initialize variable = start
	if err := c.Compile(start); err != nil {
		return err
	}
	varSymbol := c.symbolTable.Define(variable)
	if varSymbol.Scope == GlobalScope {
		c.emit(OpSetGlobal, varSymbol.Index)
	} else {
		c.emit(OpSetLocal, varSymbol.Index)
	}

	// Store end into @end_<variable>
	if err := c.Compile(end); err != nil {
		return err
	}
	endSymbol := c.symbolTable.Define("@end_" + variable)
	if endSymbol.Scope == GlobalScope {
		c.emit(OpSetGlobal, endSymbol.Index)
	} else {
		c.emit(OpSetLocal, endSymbol.Index)
	}

	if varSymbol.Scope == GlobalScope && endSymbol.Scope == GlobalScope {
		// Initial check before loop: if variable > @end_<variable>, jump to loopEnd
		c.emit(OpGetGlobal, varSymbol.Index)
		c.emit(OpGetGlobal, endSymbol.Index)
		c.emit(OpLessEqual)
		jumpOut := c.emit(OpJumpIfFalse, 9999)

		ctx := &loopContext{}
		c.loopStack = append(c.loopStack, ctx)

		bodyStart := len(c.currentInstructions())
		if err := c.Compile(body); err != nil {
			return err
		}

		// Emit OpLoopGlobalLeInc: increments var and loops back to bodyStart if var <= end!
		contPos := len(c.currentInstructions())
		c.emit(OpLoopGlobalLeInc, varSymbol.Index, endSymbol.Index, bodyStart)
		loopEnd := len(c.currentInstructions())
		c.changeOperand(jumpOut, loopEnd)

		for _, brk := range ctx.breakJumps {
			c.changeOperand(brk, loopEnd)
		}
		for _, cnt := range ctx.continueJumps {
			c.changeOperand(cnt, contPos)
		}

		c.loopStack = c.loopStack[:len(c.loopStack)-1]
		return nil
	}

	ctx := &loopContext{}
	c.loopStack = append(c.loopStack, ctx)

	loopStart := len(c.currentInstructions())

	// Condition: variable <= @end_<variable>
	if varSymbol.Scope == GlobalScope {
		c.emit(OpGetGlobal, varSymbol.Index)
	} else {
		c.emit(OpGetLocal, varSymbol.Index)
	}
	if endSymbol.Scope == GlobalScope {
		c.emit(OpGetGlobal, endSymbol.Index)
	} else {
		c.emit(OpGetLocal, endSymbol.Index)
	}
	c.emit(OpLessEqual)

	jumpOut := c.emit(OpJumpIfFalse, 9999)

	// Body
	if err := c.Compile(body); err != nil {
		return err
	}

	// Increment variable: variable++  (continue lands here)
	contPos := len(c.currentInstructions())
	if varSymbol.Scope == GlobalScope {
		c.emit(OpIncGlobal, varSymbol.Index)
	} else {
		c.emit(OpIncLocal, varSymbol.Index)
	}

	c.emit(OpJump, loopStart)
	loopEnd := len(c.currentInstructions())
	c.changeOperand(jumpOut, loopEnd)

	for _, brk := range ctx.breakJumps {
		c.changeOperand(brk, loopEnd)
	}
	for _, cnt := range ctx.continueJumps {
		c.changeOperand(cnt, contPos)
	}

	c.loopStack = c.loopStack[:len(c.loopStack)-1]
	return nil
}

func (c *Compiler) compileFor(variable string, iterable ast.Expression, body *ast.BlockStatement) error {
	if rng, ok := iterable.(*ast.RangeLiteral); ok {
		// The specialised counter loop only ascends (var <= end, var += 1).
		// Use it when the compiler can prove an ascending literal range; for
		// dynamic bounds or a descending literal range fall through to the
		// generic iterator, whose OpRange list matches the interpreter's
		// EvalRange semantics (inclusive, direction-aware) exactly.
		startLit, startOK := rng.Start.(*ast.IntegerLiteral)
		endLit, endOK := rng.End.(*ast.IntegerLiteral)
		if startOK && endOK && startLit.Value <= endLit.Value {
			return c.compileForRange(variable, rng.Start, rng.End, body)
		}
	}

	c.emit(OpGetBuiltin, c.addConstant(&interpreter.StringValue{Val: "@iter"}))
	if err := c.Compile(iterable); err != nil {
		return err
	}
	c.emit(OpCall, 1)
	iterSymbol := c.symbolTable.Define("@iter_" + variable)
	if iterSymbol.Scope == GlobalScope {
		c.emit(OpSetGlobal, iterSymbol.Index)
	} else {
		c.emit(OpSetLocal, iterSymbol.Index)
	}

	c.emit(OpConstant, c.addConstant(&interpreter.IntegerValue{Val: 0}))
	idxSymbol := c.symbolTable.Define("@idx_" + variable)
	if idxSymbol.Scope == GlobalScope {
		c.emit(OpSetGlobal, idxSymbol.Index)
	} else {
		c.emit(OpSetLocal, idxSymbol.Index)
	}

	ctx := &loopContext{}
	c.loopStack = append(c.loopStack, ctx)

	loopStart := len(c.currentInstructions())

	// Push index
	if idxSymbol.Scope == GlobalScope {
		c.emit(OpGetGlobal, idxSymbol.Index)
	} else {
		c.emit(OpGetLocal, idxSymbol.Index)
	}

	// Push len(iterable)
	lenIdx := c.addConstant(&interpreter.StringValue{Val: "len"})
	c.emit(OpGetBuiltin, lenIdx)
	if iterSymbol.Scope == GlobalScope {
		c.emit(OpGetGlobal, iterSymbol.Index)
	} else {
		c.emit(OpGetLocal, iterSymbol.Index)
	}
	c.emit(OpCall, 1)

	// index < len(iterable)
	c.emit(OpLessThan)

	jumpOut := c.emit(OpJumpIfFalse, 9999)

	// variable = iterable[index]
	if iterSymbol.Scope == GlobalScope {
		c.emit(OpGetGlobal, iterSymbol.Index)
	} else {
		c.emit(OpGetLocal, iterSymbol.Index)
	}
	if idxSymbol.Scope == GlobalScope {
		c.emit(OpGetGlobal, idxSymbol.Index)
	} else {
		c.emit(OpGetLocal, idxSymbol.Index)
	}
	c.emit(OpIndex)

	varSymbol := c.symbolTable.Define(variable)
	if varSymbol.Scope == GlobalScope {
		c.emit(OpSetGlobal, varSymbol.Index)
	} else {
		c.emit(OpSetLocal, varSymbol.Index)
	}

	// Loop body
	if err := c.Compile(body); err != nil {
		return err
	}

	continuePos := len(c.currentInstructions())

	// index = index + 1
	if idxSymbol.Scope == GlobalScope {
		c.emit(OpGetGlobal, idxSymbol.Index)
	} else {
		c.emit(OpGetLocal, idxSymbol.Index)
	}
	oneIdx := c.addConstant(&interpreter.IntegerValue{Val: 1})
	c.emit(OpConstant, oneIdx)
	c.emit(OpAdd)
	if idxSymbol.Scope == GlobalScope {
		c.emit(OpSetGlobal, idxSymbol.Index)
	} else {
		c.emit(OpSetLocal, idxSymbol.Index)
	}

	c.emit(OpJump, loopStart)
	loopEndPos := len(c.currentInstructions())
	c.changeOperand(jumpOut, loopEndPos)

	for _, brk := range ctx.breakJumps {
		c.changeOperand(brk, loopEndPos)
	}
	for _, cnt := range ctx.continueJumps {
		c.changeOperand(cnt, continuePos)
	}

	c.loopStack = c.loopStack[:len(c.loopStack)-1]
	return nil
}

func (c *Compiler) compileLoop(n *ast.LoopStatement) error {
	ctx := &loopContext{}
	c.loopStack = append(c.loopStack, ctx)

	loopStartPos := len(c.currentInstructions())
	if err := c.Compile(n.Body); err != nil {
		return err
	}
	c.emit(OpJump, loopStartPos)
	loopEndPos := len(c.currentInstructions())

	for _, brk := range ctx.breakJumps {
		c.changeOperand(brk, loopEndPos)
	}
	for _, cnt := range ctx.continueJumps {
		c.changeOperand(cnt, loopStartPos)
	}

	c.loopStack = c.loopStack[:len(c.loopStack)-1]
	return nil
}

// desugarElif rewrites "if a: A elif b: B else: E" into nested if/else statements
// so the single if/else code path handles every branch.
func desugarElif(n *ast.IfStatement) *ast.IfStatement {
	tail := n.ElseBlock
	for i := len(n.Alternatives) - 1; i >= 0; i-- {
		alt := n.Alternatives[i]
		inner := &ast.IfStatement{Token: n.Token, Condition: alt.Condition, Consequence: alt.Body, ElseBlock: tail}
		tail = &ast.BlockStatement{Token: n.Token, Statements: []ast.Statement{inner}}
	}
	return &ast.IfStatement{Token: n.Token, Condition: n.Condition, Consequence: n.Consequence, ElseBlock: tail}
}
