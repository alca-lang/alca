// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

package ir

import (
	"fmt"

	"github.com/pawl-lang/pawl/pkg/ast"
	"github.com/pawl-lang/pawl/pkg/typecheck"
)

// Lower converts a checked Pawl AST into an IR Program.
func Lower(prog *ast.Program, typeInfo *typecheck.Result) (*Program, error) {
	builder := &loweringContext{
		typeInfo:  typeInfo,
		irProg:    &Program{},
		functions: make(map[string]*Function),
	}
	return builder.lowerProgram(prog)
}

type loweringContext struct {
	typeInfo  *typecheck.Result
	irProg    *Program
	functions map[string]*Function

	currentFn    *Function
	currentBlock *Block
	locals       map[string]VReg

	// Loops for break/continue
	loopContinue []*Block
	loopBreak    []*Block
}

func (l *loweringContext) emit(inst *Instruction) {
	if l.currentBlock == nil {
		return
	}
	l.currentBlock.Instructions = append(l.currentBlock.Instructions, inst)
}

func (l *loweringContext) lowerProgram(prog *ast.Program) (*Program, error) {
	// 1. Declare functions
	for _, stmt := range prog.Statements {
		if fnDecl, ok := stmt.(*ast.FuncDeclaration); ok {
			retType := l.lookupReturnType(fnDecl)
			irFn := &Function{
				Name:       fnDecl.Name,
				ReturnType: retType,
			}
			for i, p := range fnDecl.Params {
				pType := TypeI64
				if p.TypeAnnotation != nil && len(p.TypeAnnotation.Types) > 0 {
					pType = l.annotationToIR(p.TypeAnnotation.Types[0])
				}
				v := VReg{ID: i, Type: pType}
				irFn.Params = append(irFn.Params, v)
			}
			irFn.NextVRegID = len(irFn.Params)
			l.functions[fnDecl.Name] = irFn
			l.irProg.Functions = append(l.irProg.Functions, irFn)
		}
	}

	// 2. Lower user functions
	for _, stmt := range prog.Statements {
		if fnDecl, ok := stmt.(*ast.FuncDeclaration); ok {
			irFn := l.functions[fnDecl.Name]
			if err := l.lowerFunction(irFn, fnDecl); err != nil {
				return nil, err
			}
		}
	}

	// 3. Entry point (main)
	entryFn := &Function{
		Name:       "main",
		ReturnType: TypeI64,
	}
	l.currentFn = entryFn
	l.locals = make(map[string]VReg)
	entryBlock := entryFn.NewBlock("entry")
	l.currentBlock = entryBlock

	for _, stmt := range prog.Statements {
		if _, ok := stmt.(*ast.FuncDeclaration); ok {
			continue
		}
		if err := l.lowerStatement(stmt); err != nil {
			return nil, err
		}
	}

	if l.currentBlock != nil && !l.isTerminated(l.currentBlock) {
		zeroReg := entryFn.NewVReg(TypeI64)
		l.emit(&Instruction{Op: OpConstInt, Dst: zeroReg, IntVal: 0})
		l.emit(&Instruction{Op: OpRet, Src1: zeroReg})
	}

	l.irProg.Functions = append(l.irProg.Functions, entryFn)
	l.irProg.EntryFn = entryFn

	return l.irProg, nil
}

func (l *loweringContext) isTerminated(b *Block) bool {
	if len(b.Instructions) == 0 {
		return false
	}
	last := b.Instructions[len(b.Instructions)-1].Op
	return last == OpRet || last == OpJmp || last == OpBranch
}

func (l *loweringContext) lookupReturnType(fnDecl *ast.FuncDeclaration) Type {
	if fnDecl.ReturnType != nil && len(fnDecl.ReturnType.Types) > 0 {
		return l.annotationToIR(fnDecl.ReturnType.Types[0])
	}
	return TypeVoid
}

func (l *loweringContext) annotationToIR(typeName string) Type {
	switch typeName {
	case "int":
		return TypeI64
	case "float":
		return TypeF64
	case "bool":
		return TypeI8
	case "void":
		return TypeVoid
	default:
		return TypePtr
	}
}

func (l *loweringContext) lowerFunction(irFn *Function, decl *ast.FuncDeclaration) error {
	l.currentFn = irFn
	l.locals = make(map[string]VReg)

	entry := irFn.NewBlock("entry")
	l.currentBlock = entry

	for i, p := range decl.Params {
		paramVReg := irFn.Params[i]
		l.locals[p.Name] = paramVReg
	}

	for _, stmt := range decl.Body.Statements {
		if err := l.lowerStatement(stmt); err != nil {
			return err
		}
	}

	if l.currentBlock != nil && !l.isTerminated(l.currentBlock) {
		if irFn.ReturnType == TypeVoid {
			l.emit(&Instruction{Op: OpRet, Src1: VReg{ID: -1, Type: TypeVoid}})
		} else {
			zero := irFn.NewVReg(irFn.ReturnType)
			l.emit(&Instruction{Op: OpConstInt, Dst: zero, IntVal: 0})
			l.emit(&Instruction{Op: OpRet, Src1: zero})
		}
	}
	return nil
}

func (l *loweringContext) lowerStatement(stmt ast.Statement) error {
	switch s := stmt.(type) {
	case *ast.VarDeclaration:
		valReg, err := l.lowerExpression(s.Value)
		if err != nil {
			return err
		}
		localReg := l.currentFn.NewVReg(valReg.Type)
		l.emit(&Instruction{Op: OpCopy, Dst: localReg, Src1: valReg})
		l.locals[s.Name.Value] = localReg
		return nil

	case *ast.ReturnStatement:
		if s.Value != nil {
			valReg, err := l.lowerExpression(s.Value)
			if err != nil {
				return err
			}
			l.emit(&Instruction{Op: OpRet, Src1: valReg})
		} else {
			l.emit(&Instruction{Op: OpRet, Src1: VReg{ID: -1, Type: TypeVoid}})
		}
		return nil

	case *ast.ExpressionStatement:
		_, err := l.lowerExpression(s.Expression)
		return err

	case *ast.BlockStatement:
		for _, inner := range s.Statements {
			if err := l.lowerStatement(inner); err != nil {
				return err
			}
		}
		return nil

	case *ast.IfStatement:
		return l.lowerIfStatement(s)

	case *ast.WhileStatement:
		return l.lowerWhileStatement(s)

	case *ast.ForStatement:
		return l.lowerForStatement(s)

	case *ast.BreakStatement:
		if len(l.loopBreak) == 0 {
			return fmt.Errorf("break outside of loop")
		}
		target := l.loopBreak[len(l.loopBreak)-1]
		l.emit(&Instruction{Op: OpJmp, Target: target})
		l.currentBlock = l.currentFn.NewBlock("after_break")
		return nil

	case *ast.ContinueStatement:
		if len(l.loopContinue) == 0 {
			return fmt.Errorf("continue outside of loop")
		}
		target := l.loopContinue[len(l.loopContinue)-1]
		l.emit(&Instruction{Op: OpJmp, Target: target})
		l.currentBlock = l.currentFn.NewBlock("after_continue")
		return nil

	default:
		return fmt.Errorf("statement %T not supported yet in stage 2", stmt)
	}
}

func (l *loweringContext) lowerIfStatement(s *ast.IfStatement) error {
	condReg, err := l.lowerExpression(s.Condition)
	if err != nil {
		return err
	}

	thenBlock := l.currentFn.NewBlock("if_then")
	elseBlock := l.currentFn.NewBlock("if_else")
	mergeBlock := l.currentFn.NewBlock("if_merge")

	l.emit(&Instruction{
		Op:      OpBranch,
		Src1:    condReg,
		Target:  thenBlock,
		ElseTgt: elseBlock,
	})

	l.currentBlock = thenBlock
	if err := l.lowerStatement(s.Consequence); err != nil {
		return err
	}
	if !l.isTerminated(l.currentBlock) {
		l.emit(&Instruction{Op: OpJmp, Target: mergeBlock})
	}

	l.currentBlock = elseBlock
	if s.ElseBlock != nil {
		if err := l.lowerStatement(s.ElseBlock); err != nil {
			return err
		}
	}
	if !l.isTerminated(l.currentBlock) {
		l.emit(&Instruction{Op: OpJmp, Target: mergeBlock})
	}

	l.currentBlock = mergeBlock
	return nil
}

func (l *loweringContext) lowerWhileStatement(s *ast.WhileStatement) error {
	condBlock := l.currentFn.NewBlock("while_cond")
	bodyBlock := l.currentFn.NewBlock("while_body")
	afterBlock := l.currentFn.NewBlock("while_after")

	l.emit(&Instruction{Op: OpJmp, Target: condBlock})

	l.currentBlock = condBlock
	condReg, err := l.lowerExpression(s.Condition)
	if err != nil {
		return err
	}
	l.emit(&Instruction{
		Op:      OpBranch,
		Src1:    condReg,
		Target:  bodyBlock,
		ElseTgt: afterBlock,
	})

	l.loopContinue = append(l.loopContinue, condBlock)
	l.loopBreak = append(l.loopBreak, afterBlock)

	l.currentBlock = bodyBlock
	if err := l.lowerStatement(s.Body); err != nil {
		return err
	}
	if !l.isTerminated(l.currentBlock) {
		l.emit(&Instruction{Op: OpJmp, Target: condBlock})
	}

	l.loopContinue = l.loopContinue[:len(l.loopContinue)-1]
	l.loopBreak = l.loopBreak[:len(l.loopBreak)-1]

	l.currentBlock = afterBlock
	return nil
}

func (l *loweringContext) lowerForStatement(s *ast.ForStatement) error {
	rangeLit, ok := s.Iterable.(*ast.RangeLiteral)
	if !ok {
		return fmt.Errorf("for loop only supports integer range literals in stage 2")
	}

	startReg, err := l.lowerExpression(rangeLit.Start)
	if err != nil {
		return err
	}
	endReg, err := l.lowerExpression(rangeLit.End)
	if err != nil {
		return err
	}

	iReg := l.currentFn.NewVReg(TypeI64)
	l.emit(&Instruction{Op: OpCopy, Dst: iReg, Src1: startReg})
	l.locals[s.Variable] = iReg

	condBlock := l.currentFn.NewBlock("for_cond")
	bodyBlock := l.currentFn.NewBlock("for_body")
	stepBlock := l.currentFn.NewBlock("for_step")
	afterBlock := l.currentFn.NewBlock("for_after")

	l.emit(&Instruction{Op: OpJmp, Target: condBlock})

	l.currentBlock = condBlock
	cmpReg := l.currentFn.NewVReg(TypeI8)
	cmpOp := OpLt
	if rangeLit.Inclusive {
		cmpOp = OpLe
	}
	l.emit(&Instruction{Op: cmpOp, Dst: cmpReg, Src1: iReg, Src2: endReg})
	l.emit(&Instruction{Op: OpBranch, Src1: cmpReg, Target: bodyBlock, ElseTgt: afterBlock})

	l.loopContinue = append(l.loopContinue, stepBlock)
	l.loopBreak = append(l.loopBreak, afterBlock)

	l.currentBlock = bodyBlock
	if err := l.lowerStatement(s.Body); err != nil {
		return err
	}
	if !l.isTerminated(l.currentBlock) {
		l.emit(&Instruction{Op: OpJmp, Target: stepBlock})
	}

	l.currentBlock = stepBlock
	oneReg := l.currentFn.NewVReg(TypeI64)
	l.emit(&Instruction{Op: OpConstInt, Dst: oneReg, IntVal: 1})
	nextIReg := l.currentFn.NewVReg(TypeI64)
	l.emit(&Instruction{Op: OpAdd, Dst: nextIReg, Src1: iReg, Src2: oneReg})
	l.emit(&Instruction{Op: OpCopy, Dst: iReg, Src1: nextIReg})
	l.emit(&Instruction{Op: OpJmp, Target: condBlock})

	l.loopContinue = l.loopContinue[:len(l.loopContinue)-1]
	l.loopBreak = l.loopBreak[:len(l.loopBreak)-1]

	l.currentBlock = afterBlock
	return nil
}

func (l *loweringContext) lowerExpression(e ast.Expression) (VReg, error) {
	switch expr := e.(type) {
	case *ast.IntegerLiteral:
		dst := l.currentFn.NewVReg(TypeI64)
		l.emit(&Instruction{Op: OpConstInt, Dst: dst, IntVal: expr.Value})
		return dst, nil

	case *ast.StringLiteral:
		dst := l.currentFn.NewVReg(TypePtr)
		l.emit(&Instruction{Op: OpConstStr, Dst: dst, StrVal: expr.Value})
		return dst, nil

	case *ast.FloatLiteral:
		dst := l.currentFn.NewVReg(TypeF64)
		l.emit(&Instruction{Op: OpConstFloat, Dst: dst, FloatVal: expr.Value})
		return dst, nil

	case *ast.BooleanLiteral:
		dst := l.currentFn.NewVReg(TypeI8)
		bVal := int64(0)
		if expr.Value {
			bVal = 1
		}
		l.emit(&Instruction{Op: OpConstBool, Dst: dst, IntVal: bVal})
		return dst, nil

	case *ast.Identifier:
		v, exists := l.locals[expr.Value]
		if !exists {
			return VReg{}, fmt.Errorf("undefined local variable %s", expr.Value)
		}
		return v, nil

	case *ast.AssignExpression:
		valReg, err := l.lowerExpression(expr.Value)
		if err != nil {
			return VReg{}, err
		}
		if ident, ok := expr.Target.(*ast.Identifier); ok {
			localReg, exists := l.locals[ident.Value]
			if !exists {
				localReg = l.currentFn.NewVReg(valReg.Type)
				l.locals[ident.Value] = localReg
			}
			l.emit(&Instruction{Op: OpCopy, Dst: localReg, Src1: valReg})
			return localReg, nil
		}
		return VReg{}, fmt.Errorf("assignment to non-identifier not supported in stage 2")

	case *ast.PrefixExpression:
		r, err := l.lowerExpression(expr.Right)
		if err != nil {
			return VReg{}, err
		}
		dst := l.currentFn.NewVReg(r.Type)
		switch expr.Operator {
		case "-":
			l.emit(&Instruction{Op: OpNeg, Dst: dst, Src1: r})
		case "!":
			l.emit(&Instruction{Op: OpNot, Dst: dst, Src1: r})
		default:
			return VReg{}, fmt.Errorf("unsupported prefix op %s", expr.Operator)
		}
		return dst, nil

	case *ast.InfixExpression:
		leftReg, err := l.lowerExpression(expr.Left)
		if err != nil {
			return VReg{}, err
		}
		rightReg, err := l.lowerExpression(expr.Right)
		if err != nil {
			return VReg{}, err
		}

		var op Op
		resType := leftReg.Type

		switch expr.Operator {
		case "+":
			if leftReg.Type == TypePtr || rightReg.Type == TypePtr {
				op = OpConcatStr
				resType = TypePtr
			} else {
				op = OpAdd
			}
		case "-":
			op = OpSub
		case "*":
			op = OpMul
		case "/":
			op = OpDiv
		case "%":
			op = OpMod
		case "==":
			op = OpEq
			resType = TypeI8
		case "!=":
			op = OpNe
			resType = TypeI8
		case "<":
			op = OpLt
			resType = TypeI8
		case "<=":
			op = OpLe
			resType = TypeI8
		case ">":
			op = OpGt
			resType = TypeI8
		case ">=":
			op = OpGe
			resType = TypeI8
		default:
			return VReg{}, fmt.Errorf("unsupported infix op %s", expr.Operator)
		}

		dst := l.currentFn.NewVReg(resType)
		l.emit(&Instruction{Op: op, Dst: dst, Src1: leftReg, Src2: rightReg})
		return dst, nil

	case *ast.CallExpression:
		fnIdent, ok := expr.Function.(*ast.Identifier)
		if !ok {
			return VReg{}, fmt.Errorf("indirect calls not supported yet in stage 2")
		}

		var args []VReg
		for _, a := range expr.Arguments {
			ar, err := l.lowerExpression(a)
			if err != nil {
				return VReg{}, err
			}
			args = append(args, ar)
		}

		fnName := fnIdent.Value

		// Special handling for exit(code)
		if fnName == "exit" && len(args) == 1 {
			l.emit(&Instruction{
				Op:   OpRet,
				Src1: args[0],
			})
			return VReg{ID: -1, Type: TypeVoid}, nil
		}

		if fnName == "print" {
			for _, a := range args {
				l.emit(&Instruction{Op: OpPrint, Src1: a})
			}
			return VReg{ID: -1, Type: TypeVoid}, nil
		}

		if fnName == "println" {
			for _, a := range args {
				l.emit(&Instruction{Op: OpPrintln, Src1: a})
			}
			if len(args) == 0 {
				emptyStr := l.currentFn.NewVReg(TypePtr)
				l.emit(&Instruction{Op: OpConstStr, Dst: emptyStr, StrVal: ""})
				l.emit(&Instruction{Op: OpPrintln, Src1: emptyStr})
			}
			return VReg{ID: -1, Type: TypeVoid}, nil
		}

		if fnName == "string" && len(args) == 1 {
			if args[0].Type == TypePtr {
				return args[0], nil
			}
			dst := l.currentFn.NewVReg(TypePtr)
			l.emit(&Instruction{Op: OpIntToStr, Dst: dst, Src1: args[0]})
			return dst, nil
		}

		var retType Type = TypeVoid
		if targetFn, exists := l.functions[fnName]; exists {
			retType = targetFn.ReturnType
		}

		dst := l.currentFn.NewVReg(retType)
		l.emit(&Instruction{
			Op:       OpCall,
			Dst:      dst,
			CallName: fnName,
			Args:     args,
		})
		return dst, nil

	default:
		return VReg{}, fmt.Errorf("expression %T not supported in stage 2 IR", e)
	}
}
