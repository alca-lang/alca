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
		typeInfo:      typeInfo,
		irProg:        &Program{},
		functions:     make(map[string]*Function),
		structLayouts: make(map[string]*structLayout),
		enums:         make(map[string]map[string]int64),
	}
	return builder.lowerProgram(prog)
}

type structLayout struct {
	name        string
	isClass     bool
	fields      map[string]int64 // field name -> byte offset
	fieldTypes  map[string]Type  // field name -> IR type
	totalSize   int64
	methodVCols map[string]int   // method name -> vtable index (0-based)
}

type loweringContext struct {
	typeInfo      *typecheck.Result
	irProg        *Program
	functions     map[string]*Function
	structLayouts map[string]*structLayout
	enums         map[string]map[string]int64 // enumName -> variantName -> tag

	currentFn    *Function
	currentBlock *Block
	locals       map[string]VReg
	tmpCounter   int

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
	// 1. Build struct and class layouts
	if err := l.buildLayouts(prog); err != nil {
		return nil, err
	}

	// 2. Declare top-level functions and class methods
	for _, stmt := range prog.Statements {
		switch s := stmt.(type) {
		case *ast.FuncDeclaration:
			l.declareFunc(s.Name, s.Params, s.ReturnType)
		case *ast.ClassDeclaration:
			for _, m := range s.Methods {
				if m != nil && m.Func != nil {
					mName := s.Name + "." + m.Func.Name
					l.declareMethod(mName, s.Name, m.Func.Params, m.Func.ReturnType)
				}
			}
		}
	}

	// 3. Lower top-level functions and class methods
	for _, stmt := range prog.Statements {
		switch s := stmt.(type) {
		case *ast.FuncDeclaration:
			irFn := l.functions[s.Name]
			if err := l.lowerFunction(irFn, s.Params, s.Body.Statements, nil); err != nil {
				return nil, err
			}
		case *ast.ClassDeclaration:
			for _, m := range s.Methods {
				if m != nil && m.Func != nil {
					mName := s.Name + "." + m.Func.Name
					irFn := l.functions[mName]
					if err := l.lowerFunction(irFn, m.Func.Params, m.Func.Body.Statements, s); err != nil {
						return nil, err
					}
				}
			}
		}
	}

	// 4. Entry point (main)
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
		if _, ok := stmt.(*ast.ClassDeclaration); ok {
			continue
		}
		if _, ok := stmt.(*ast.StructDeclaration); ok {
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

func (l *loweringContext) buildLayouts(prog *ast.Program) error {
	// Register built-in error struct layout: code: i64 (off 0), msg: ptr (off 8)
	errLayout := &structLayout{
		name:       "error",
		isClass:    false,
		fields:     map[string]int64{"code": 0, "msg": 8, "message": 8},
		fieldTypes: map[string]Type{"code": TypeI64, "msg": TypePtr, "message": TypePtr},
		totalSize:  16,
	}
	l.structLayouts["error"] = errLayout

	for _, stmt := range prog.Statements {
		switch s := stmt.(type) {
		case *ast.EnumDeclaration:
			variants := make(map[string]int64)
			for idx, v := range s.Variants {
				var tag int64 = int64(idx)
				if v.Value != nil {
					if intVal, ok := v.Value.(*ast.IntegerLiteral); ok {
						tag = intVal.Value
					}
				}
				variants[v.Name] = tag
			}
			l.enums[s.Name] = variants

		case *ast.StructDeclaration:
			layout := &structLayout{
				name:       s.Name,
				isClass:    false,
				fields:     make(map[string]int64),
				fieldTypes: make(map[string]Type),
			}
			var offset int64 = 0
			for _, f := range s.Fields {
				fType := TypeI64
				if f.TypeAnnotation != nil && len(f.TypeAnnotation.Types) > 0 {
					fType = l.annotationToIR(f.TypeAnnotation.Types[0])
				}
				layout.fields[f.Name] = offset
				layout.fieldTypes[f.Name] = fType
				offset += 8
			}
			layout.totalSize = offset
			l.structLayouts[s.Name] = layout

		case *ast.ClassDeclaration:
			layout := &structLayout{
				name:        s.Name,
				isClass:     true,
				fields:      make(map[string]int64),
				fieldTypes:  make(map[string]Type),
				methodVCols: make(map[string]int),
			}
			// Header: vtable pointer at offset 0
			var offset int64 = 8

			// Inherited fields from parent if any
			if s.Parent != "" {
				if parentLayout, ok := l.structLayouts[s.Parent]; ok {
					for fName, fOff := range parentLayout.fields {
						layout.fields[fName] = fOff
						layout.fieldTypes[fName] = parentLayout.fieldTypes[fName]
						if fOff+8 > offset {
							offset = fOff + 8
						}
					}
					for mName, vIdx := range parentLayout.methodVCols {
						layout.methodVCols[mName] = vIdx
					}
				}
			}

			// Subclass fields
			for _, f := range s.Fields {
				fType := TypeI64
				if f.TypeAnnotation != nil && len(f.TypeAnnotation.Types) > 0 {
					fType = l.annotationToIR(f.TypeAnnotation.Types[0])
				}
				layout.fields[f.Name] = offset
				layout.fieldTypes[f.Name] = fType
				offset += 8
			}
			layout.totalSize = offset

			// Methods
			for _, m := range s.Methods {
				if m != nil && m.Func != nil {
					mName := m.Func.Name
					if _, exists := layout.methodVCols[mName]; !exists {
						layout.methodVCols[mName] = len(layout.methodVCols)
					}
				}
			}

			l.structLayouts[s.Name] = layout
		}
	}
	return nil
}

func (l *loweringContext) declareFunc(name string, params []*ast.FuncParam, retAnn *ast.TypeAnnotation) {
	retType := TypeVoid
	if retAnn != nil && len(retAnn.Types) > 0 {
		retType = l.annotationToIR(retAnn.Types[0])
	}
	irFn := &Function{
		Name:       name,
		ReturnType: retType,
	}
	for i, p := range params {
		pType := TypeI64
		if p.TypeAnnotation != nil && len(p.TypeAnnotation.Types) > 0 {
			pType = l.annotationToIR(p.TypeAnnotation.Types[0])
		}
		irFn.Params = append(irFn.Params, VReg{ID: i, Type: pType})
	}
	irFn.NextVRegID = len(irFn.Params)
	l.functions[name] = irFn
	l.irProg.Functions = append(l.irProg.Functions, irFn)
}

func (l *loweringContext) declareMethod(mName, className string, params []*ast.FuncParam, retAnn *ast.TypeAnnotation) {
	retType := TypeVoid
	if retAnn != nil && len(retAnn.Types) > 0 {
		retType = l.annotationToIR(retAnn.Types[0])
	}
	irFn := &Function{
		Name:       mName,
		ReturnType: retType,
	}
	// Param 0 is self
	irFn.Params = append(irFn.Params, VReg{ID: 0, Type: TypePtr})
	for i, p := range params {
		pType := TypeI64
		if p.TypeAnnotation != nil && len(p.TypeAnnotation.Types) > 0 {
			pType = l.annotationToIR(p.TypeAnnotation.Types[0])
		}
		irFn.Params = append(irFn.Params, VReg{ID: i + 1, Type: pType})
	}
	irFn.NextVRegID = len(irFn.Params)
	l.functions[mName] = irFn
	l.irProg.Functions = append(l.irProg.Functions, irFn)
}

func (l *loweringContext) lowerFunction(irFn *Function, params []*ast.FuncParam, body []ast.Statement, cls *ast.ClassDeclaration) error {
	l.currentFn = irFn
	l.locals = make(map[string]VReg)

	entry := irFn.NewBlock("entry")
	l.currentBlock = entry

	if cls != nil {
		// self is param 0
		l.locals["self"] = irFn.Params[0]
		for i, p := range params {
			l.locals[p.Name] = irFn.Params[i+1]
		}
	} else {
		for i, p := range params {
			l.locals[p.Name] = irFn.Params[i]
		}
	}

	for _, stmt := range body {
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

func (l *loweringContext) isTerminated(b *Block) bool {
	if len(b.Instructions) == 0 {
		return false
	}
	last := b.Instructions[len(b.Instructions)-1].Op
	return last == OpRet || last == OpJmp || last == OpBranch
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

func (l *loweringContext) typeToIR(t typecheck.Type) Type {
	switch t {
	case typecheck.Int:
		return TypeI64
	case typecheck.Float:
		return TypeF64
	case typecheck.Bool:
		return TypeI8
	case typecheck.Void:
		return TypeVoid
	default:
		return TypePtr
	}
}

func (l *loweringContext) lowerStatement(stmt ast.Statement) error {
	switch s := stmt.(type) {
	case *ast.VarDeclaration:
		if se, ok := s.Value.(*ast.StreamExpression); ok && (se.Operator == ">>>" || se.Operator == "?>>>") {
			// e.g. res := validate(-10) ?>>> e
			valReg, err := l.lowerExpression(se.Source)
			if err != nil {
				return err
			}
			if targetIdent, ok := se.Target.(*ast.Identifier); ok {
				tReg, exists := l.locals[targetIdent.Value]
				if !exists {
					tReg = l.currentFn.NewVReg(TypePtr)
					l.locals[targetIdent.Value] = tReg
				}
				if se.Operator == "?>>>" {
					l.emit(&Instruction{Op: OpGetErr, Dst: tReg})
				} else {
					l.emit(&Instruction{Op: OpCopy, Dst: tReg, Src1: valReg})
				}
			}
			localReg := l.currentFn.NewVReg(valReg.Type)
			l.emit(&Instruction{Op: OpCopy, Dst: localReg, Src1: valReg})
			l.locals[s.Name.Value] = localReg
			return nil
		}

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
		if se, ok := s.Expression.(*ast.StreamExpression); ok && (se.Operator == ">>>" || se.Operator == "?>>>") {
			return l.lowerStreamCapture(se)
		}
		_, err := l.lowerExpression(s.Expression)
		return err

	case *ast.AssertStatement:
		condReg, err := l.lowerExpression(s.Expression)
		if err != nil {
			return err
		}
		okBlock := l.currentFn.NewBlock("assert_ok")
		failBlock := l.currentFn.NewBlock("assert_fail")
		l.emit(&Instruction{Op: OpBranch, Src1: condReg, Target: okBlock, ElseTgt: failBlock})

		l.currentBlock = failBlock
		errCode := l.currentFn.NewVReg(TypeI64)
		l.emit(&Instruction{Op: OpConstInt, Dst: errCode, IntVal: 1})
		l.emit(&Instruction{Op: OpRet, Src1: errCode})

		l.currentBlock = okBlock
		return nil

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

	case *ast.MatchStatement:
		return l.lowerMatchStatement(s)

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

	case *ast.EnumDeclaration, *ast.StructDeclaration, *ast.ClassDeclaration, *ast.UseStatement:
		// Already processed in buildLayouts, imports and declarations pass
		return nil

	default:
		return fmt.Errorf("statement %T not supported yet in native backend", stmt)
	}
}

func (l *loweringContext) lowerStreamCapture(se *ast.StreamExpression) error {
	id, ok := se.Target.(*ast.Identifier)
	if !ok {
		return fmt.Errorf("capture stream target must be variable identifier")
	}

	resReg, err := l.lowerExpression(se.Source)
	if err != nil {
		return err
	}

	if se.Operator == ">>>" {
		targetReg, exists := l.locals[id.Value]
		if !exists {
			targetReg = l.currentFn.NewVReg(resReg.Type)
			l.locals[id.Value] = targetReg
		}
		l.emit(&Instruction{Op: OpCopy, Dst: targetReg, Src1: resReg})
		return nil
	}

	// Operator is "?>>>" (error capture)
	targetReg, exists := l.locals[id.Value]
	if !exists {
		targetReg = l.currentFn.NewVReg(TypePtr)
		l.locals[id.Value] = targetReg
	}
	l.emit(&Instruction{Op: OpGetErr, Dst: targetReg})
	return nil
}

func (l *loweringContext) lowerMatchStatement(s *ast.MatchStatement) error {
	l.tmpCounter++
	tmpName := fmt.Sprintf("@match_%d", l.tmpCounter)
	tmpIdent := &ast.Identifier{Token: s.Token, Value: tmpName}
	decl := &ast.VarDeclaration{Token: s.Token, Name: tmpIdent, Value: s.Subject}
	if err := l.lowerStatement(decl); err != nil {
		return err
	}

	var chain *ast.IfStatement
	for i, cs := range s.Cases {
		cond := &ast.InfixExpression{Token: cs.Token, Left: tmpIdent, Operator: "==", Right: cs.Pattern}
		if i == 0 {
			chain = &ast.IfStatement{Token: s.Token, Condition: cond, Consequence: cs.Body, ElseBlock: s.Default}
		} else {
			chain.Alternatives = append(chain.Alternatives, &ast.ElifClause{Token: cs.Token, Condition: cond, Body: cs.Body})
		}
	}

	if chain != nil {
		return l.lowerIfStatement(chain)
	} else if s.Default != nil {
		return l.lowerStatement(s.Default)
	}
	return nil
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
	if len(s.Alternatives) > 0 {
		// Desugar elif into nested IfStatement in elseBlock
		nestedIf := &ast.IfStatement{
			Token:        s.Alternatives[0].Token,
			Condition:    s.Alternatives[0].Condition,
			Consequence:  s.Alternatives[0].Body,
			Alternatives: s.Alternatives[1:],
			ElseBlock:    s.ElseBlock,
		}
		if err := l.lowerIfStatement(nestedIf); err != nil {
			return err
		}
	} else if s.ElseBlock != nil {
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
	if rangeLit, ok := s.Iterable.(*ast.RangeLiteral); ok {
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

	// For loop over a list expression: for x in list_expr
	listReg, err := l.lowerExpression(s.Iterable)
	if err != nil {
		return err
	}

	elemIRType := TypeI64
	if l.typeInfo != nil && l.typeInfo.Types != nil {
		if t, ok := l.typeInfo.Types[s.Iterable]; ok {
			if lt, ok := t.(*typecheck.ListType); ok {
				elemIRType = l.typeToIR(lt.Elem)
			}
		}
	}

	// Read len: load listReg[8]
	lenReg := l.currentFn.NewVReg(TypeI64)
	l.emit(&Instruction{Op: OpLoad, Dst: lenReg, Src1: listReg, IntVal: 8})

	// Read dataPtr: load listReg[0]
	dataPtrReg := l.currentFn.NewVReg(TypePtr)
	l.emit(&Instruction{Op: OpLoad, Dst: dataPtrReg, Src1: listReg, IntVal: 0})

	idxReg := l.currentFn.NewVReg(TypeI64)
	l.emit(&Instruction{Op: OpConstInt, Dst: idxReg, IntVal: 0})

	elemReg := l.currentFn.NewVReg(elemIRType)
	l.locals[s.Variable] = elemReg

	condBlock := l.currentFn.NewBlock("for_list_cond")
	bodyBlock := l.currentFn.NewBlock("for_list_body")
	stepBlock := l.currentFn.NewBlock("for_list_step")
	afterBlock := l.currentFn.NewBlock("for_list_after")

	l.emit(&Instruction{Op: OpJmp, Target: condBlock})

	l.currentBlock = condBlock
	cmpReg := l.currentFn.NewVReg(TypeI8)
	l.emit(&Instruction{Op: OpLt, Dst: cmpReg, Src1: idxReg, Src2: lenReg})
	l.emit(&Instruction{Op: OpBranch, Src1: cmpReg, Target: bodyBlock, ElseTgt: afterBlock})

	l.loopContinue = append(l.loopContinue, stepBlock)
	l.loopBreak = append(l.loopBreak, afterBlock)

	l.currentBlock = bodyBlock
	// offset = idx * 8
	scaleReg := l.currentFn.NewVReg(TypeI64)
	l.emit(&Instruction{Op: OpConstInt, Dst: scaleReg, IntVal: 8})
	byteOffReg := l.currentFn.NewVReg(TypeI64)
	l.emit(&Instruction{Op: OpMul, Dst: byteOffReg, Src1: idxReg, Src2: scaleReg})
	itemAddrReg := l.currentFn.NewVReg(TypePtr)
	l.emit(&Instruction{Op: OpAdd, Dst: itemAddrReg, Src1: dataPtrReg, Src2: byteOffReg})
	l.emit(&Instruction{Op: OpLoad, Dst: elemReg, Src1: itemAddrReg, IntVal: 0})

	if err := l.lowerStatement(s.Body); err != nil {
		return err
	}
	if !l.isTerminated(l.currentBlock) {
		l.emit(&Instruction{Op: OpJmp, Target: stepBlock})
	}

	l.currentBlock = stepBlock
	oneReg := l.currentFn.NewVReg(TypeI64)
	l.emit(&Instruction{Op: OpConstInt, Dst: oneReg, IntVal: 1})
	nextIdxReg := l.currentFn.NewVReg(TypeI64)
	l.emit(&Instruction{Op: OpAdd, Dst: nextIdxReg, Src1: idxReg, Src2: oneReg})
	l.emit(&Instruction{Op: OpCopy, Dst: idxReg, Src1: nextIdxReg})
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

	case *ast.NullLiteral:
		dst := l.currentFn.NewVReg(TypePtr)
		l.emit(&Instruction{Op: OpConstInt, Dst: dst, IntVal: 0})
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

	case *ast.ListLiteral:
		n := int64(len(expr.Elements))
		capVal := n
		if capVal < 4 {
			capVal = 4
		}
		// 1. Allocate backing array of capVal * 8 bytes
		capBytes := l.currentFn.NewVReg(TypeI64)
		l.emit(&Instruction{Op: OpConstInt, Dst: capBytes, IntVal: capVal * 8})
		dataPtr := l.currentFn.NewVReg(TypePtr)
		l.emit(&Instruction{Op: OpAlloc, Dst: dataPtr, Src1: capBytes})

		// 2. Store elements
		for i, el := range expr.Elements {
			elReg, err := l.lowerExpression(el)
			if err != nil {
				return VReg{}, err
			}
			l.emit(&Instruction{Op: OpStore, Src1: dataPtr, IntVal: int64(i * 8), Src2: elReg})
		}

		// 3. Allocate List header (24 bytes: data_ptr, len, cap)
		hdrSize := l.currentFn.NewVReg(TypeI64)
		l.emit(&Instruction{Op: OpConstInt, Dst: hdrSize, IntVal: 24})
		listHdr := l.currentFn.NewVReg(TypePtr)
		l.emit(&Instruction{Op: OpAlloc, Dst: listHdr, Src1: hdrSize})

		lenReg := l.currentFn.NewVReg(TypeI64)
		l.emit(&Instruction{Op: OpConstInt, Dst: lenReg, IntVal: n})
		capReg := l.currentFn.NewVReg(TypeI64)
		l.emit(&Instruction{Op: OpConstInt, Dst: capReg, IntVal: capVal})

		l.emit(&Instruction{Op: OpStore, Src1: listHdr, IntVal: 0, Src2: dataPtr})
		l.emit(&Instruction{Op: OpStore, Src1: listHdr, IntVal: 8, Src2: lenReg})
		l.emit(&Instruction{Op: OpStore, Src1: listHdr, IntVal: 16, Src2: capReg})
		return listHdr, nil

	case *ast.MapLiteral:
		// Map layout: header (24 bytes: buckets_ptr, len, cap)
		// Each bucket is 16 bytes: key (8 bytes), val (8 bytes)
		n := int64(len(expr.Pairs))
		capVal := n
		if capVal < 4 {
			capVal = 4
		}
		// Allocate buckets: capVal * 16 bytes
		bucketsBytes := l.currentFn.NewVReg(TypeI64)
		l.emit(&Instruction{Op: OpConstInt, Dst: bucketsBytes, IntVal: capVal * 16})
		bucketsPtr := l.currentFn.NewVReg(TypePtr)
		l.emit(&Instruction{Op: OpAlloc, Dst: bucketsPtr, Src1: bucketsBytes})

		// Store initial pairs
		for i, p := range expr.Pairs {
			kReg, err := l.lowerExpression(p.Key)
			if err != nil {
				return VReg{}, err
			}
			vReg, err := l.lowerExpression(p.Value)
			if err != nil {
				return VReg{}, err
			}
			l.emit(&Instruction{Op: OpStore, Src1: bucketsPtr, IntVal: int64(i*16), Src2: kReg})
			l.emit(&Instruction{Op: OpStore, Src1: bucketsPtr, IntVal: int64(i*16 + 8), Src2: vReg})
		}

		// Allocate map header (24 bytes: buckets_ptr, len, cap)
		mapHdrSize := l.currentFn.NewVReg(TypeI64)
		l.emit(&Instruction{Op: OpConstInt, Dst: mapHdrSize, IntVal: 24})
		mapHdr := l.currentFn.NewVReg(TypePtr)
		l.emit(&Instruction{Op: OpAlloc, Dst: mapHdr, Src1: mapHdrSize})

		lenReg := l.currentFn.NewVReg(TypeI64)
		l.emit(&Instruction{Op: OpConstInt, Dst: lenReg, IntVal: n})
		capReg := l.currentFn.NewVReg(TypeI64)
		l.emit(&Instruction{Op: OpConstInt, Dst: capReg, IntVal: capVal})

		l.emit(&Instruction{Op: OpStore, Src1: mapHdr, IntVal: 0, Src2: bucketsPtr})
		l.emit(&Instruction{Op: OpStore, Src1: mapHdr, IntVal: 8, Src2: lenReg})
		l.emit(&Instruction{Op: OpStore, Src1: mapHdr, IntVal: 16, Src2: capReg})
		return mapHdr, nil

	case *ast.IndexExpression:
		leftReg, err := l.lowerExpression(expr.Left)
		if err != nil {
			return VReg{}, err
		}
		idxReg, err := l.lowerExpression(expr.Index)
		if err != nil {
			return VReg{}, err
		}

		isMap := false
		if l.typeInfo != nil && l.typeInfo.Types != nil {
			if t, ok := l.typeInfo.Types[expr.Left]; ok {
				if _, okMap := t.(*typecheck.MapType); okMap {
					isMap = true
				}
			}
		}

		elemIRType := TypeI64
		if l.typeInfo != nil && l.typeInfo.Types != nil {
			if t, ok := l.typeInfo.Types[expr]; ok {
				elemIRType = l.typeToIR(t)
			}
		}

		if isMap {
			// Linear probe lookup in map buckets
			// map header: [0] = buckets_ptr, [8] = len, [16] = cap
			bucketsPtr := l.currentFn.NewVReg(TypePtr)
			l.emit(&Instruction{Op: OpLoad, Dst: bucketsPtr, Src1: leftReg, IntVal: 0})
			mapLen := l.currentFn.NewVReg(TypeI64)
			l.emit(&Instruction{Op: OpLoad, Dst: mapLen, Src1: leftReg, IntVal: 8})

			loopIdx := l.currentFn.NewVReg(TypeI64)
			l.emit(&Instruction{Op: OpConstInt, Dst: loopIdx, IntVal: 0})

			resVal := l.currentFn.NewVReg(elemIRType)
			l.emit(&Instruction{Op: OpConstInt, Dst: resVal, IntVal: 0})

			condBlock := l.currentFn.NewBlock("map_lookup_cond")
			bodyBlock := l.currentFn.NewBlock("map_lookup_body")
			stepBlock := l.currentFn.NewBlock("map_lookup_step")
			afterBlock := l.currentFn.NewBlock("map_lookup_after")

			l.emit(&Instruction{Op: OpJmp, Target: condBlock})

			l.currentBlock = condBlock
			cmpLt := l.currentFn.NewVReg(TypeI8)
			l.emit(&Instruction{Op: OpLt, Dst: cmpLt, Src1: loopIdx, Src2: mapLen})
			l.emit(&Instruction{Op: OpBranch, Src1: cmpLt, Target: bodyBlock, ElseTgt: afterBlock})

			l.currentBlock = bodyBlock
			// bucket addr = bucketsPtr + loopIdx * 16
			sixteen := l.currentFn.NewVReg(TypeI64)
			l.emit(&Instruction{Op: OpConstInt, Dst: sixteen, IntVal: 16})
			bOff := l.currentFn.NewVReg(TypeI64)
			l.emit(&Instruction{Op: OpMul, Dst: bOff, Src1: loopIdx, Src2: sixteen})
			bAddr := l.currentFn.NewVReg(TypePtr)
			l.emit(&Instruction{Op: OpAdd, Dst: bAddr, Src1: bucketsPtr, Src2: bOff})

			currKey := l.currentFn.NewVReg(idxReg.Type)
			l.emit(&Instruction{Op: OpLoad, Dst: currKey, Src1: bAddr, IntVal: 0})

			keyMatch := l.currentFn.NewVReg(TypeI8)
			l.emit(&Instruction{Op: OpEq, Dst: keyMatch, Src1: currKey, Src2: idxReg})

			foundBlock := l.currentFn.NewBlock("map_lookup_found")
			l.emit(&Instruction{Op: OpBranch, Src1: keyMatch, Target: foundBlock, ElseTgt: stepBlock})

			l.currentBlock = foundBlock
			l.emit(&Instruction{Op: OpLoad, Dst: resVal, Src1: bAddr, IntVal: 8})
			l.emit(&Instruction{Op: OpJmp, Target: afterBlock})

			l.currentBlock = stepBlock
			one := l.currentFn.NewVReg(TypeI64)
			l.emit(&Instruction{Op: OpConstInt, Dst: one, IntVal: 1})
			nextIdx := l.currentFn.NewVReg(TypeI64)
			l.emit(&Instruction{Op: OpAdd, Dst: nextIdx, Src1: loopIdx, Src2: one})
			l.emit(&Instruction{Op: OpCopy, Dst: loopIdx, Src1: nextIdx})
			l.emit(&Instruction{Op: OpJmp, Target: condBlock})

			l.currentBlock = afterBlock
			return resVal, nil
		}

		// List indexing
		// Read dataPtr = load leftReg[0]
		dataPtr := l.currentFn.NewVReg(TypePtr)
		l.emit(&Instruction{Op: OpLoad, Dst: dataPtr, Src1: leftReg, IntVal: 0})

		scaleReg := l.currentFn.NewVReg(TypeI64)
		l.emit(&Instruction{Op: OpConstInt, Dst: scaleReg, IntVal: 8})
		byteOff := l.currentFn.NewVReg(TypeI64)
		l.emit(&Instruction{Op: OpMul, Dst: byteOff, Src1: idxReg, Src2: scaleReg})
		addrReg := l.currentFn.NewVReg(TypePtr)
		l.emit(&Instruction{Op: OpAdd, Dst: addrReg, Src1: dataPtr, Src2: byteOff})

		resReg := l.currentFn.NewVReg(elemIRType)
		l.emit(&Instruction{Op: OpLoad, Dst: resReg, Src1: addrReg, IntVal: 0})
		return resReg, nil

	case *ast.DotExpression:
		// Check if expr.Left is an enum identifier (e.g. Color.Red)
		if ident, ok := expr.Left.(*ast.Identifier); ok {
			if variants, isEnum := l.enums[ident.Value]; isEnum {
				if tag, hasVariant := variants[expr.Field]; hasVariant {
					dst := l.currentFn.NewVReg(TypeI64)
					l.emit(&Instruction{Op: OpConstInt, Dst: dst, IntVal: tag})
					return dst, nil
				}
			}
		}

		leftReg, err := l.lowerExpression(expr.Left)
		if err != nil {
			return VReg{}, err
		}
		var typeName string
		if l.typeInfo != nil && l.typeInfo.Types != nil {
			if t, ok := l.typeInfo.Types[expr.Left]; ok {
				if st, ok := t.(*typecheck.StructType); ok {
					typeName = st.Name
				}
			}
		}
		layout := l.structLayouts[typeName]
		if layout == nil {
			return VReg{}, fmt.Errorf("unknown struct/class for dot access %s.%s", typeName, expr.Field)
		}
		offset, ok := layout.fields[expr.Field]
		if !ok {
			return VReg{}, fmt.Errorf("field %s not found in %s", expr.Field, typeName)
		}
		fType := layout.fieldTypes[expr.Field]
		resReg := l.currentFn.NewVReg(fType)
		l.emit(&Instruction{Op: OpLoad, Dst: resReg, Src1: leftReg, IntVal: offset})
		return resReg, nil

	case *ast.StructInstantiation:
		layout := l.structLayouts[expr.Name]
		if layout == nil {
			return VReg{}, fmt.Errorf("unknown struct/class %s", expr.Name)
		}
		sizeReg := l.currentFn.NewVReg(TypeI64)
		l.emit(&Instruction{Op: OpConstInt, Dst: sizeReg, IntVal: layout.totalSize})
		objReg := l.currentFn.NewVReg(TypePtr)
		l.emit(&Instruction{Op: OpAlloc, Dst: objReg, Src1: sizeReg})

		// Fill fields in order
		offset := int64(0)
		if layout.isClass {
			offset = 8 // skip vtable
		}
		for _, arg := range expr.Args {
			ar, err := l.lowerExpression(arg)
			if err != nil {
				return VReg{}, err
			}
			l.emit(&Instruction{Op: OpStore, Src1: objReg, IntVal: offset, Src2: ar})
			offset += 8
		}
		return objReg, nil

	case *ast.AssignExpression:
		valReg, err := l.lowerExpression(expr.Value)
		if err != nil {
			return VReg{}, err
		}

		switch tgt := expr.Target.(type) {
		case *ast.Identifier:
			localReg, exists := l.locals[tgt.Value]
			if !exists {
				localReg = l.currentFn.NewVReg(valReg.Type)
				l.locals[tgt.Value] = localReg
			}
			l.emit(&Instruction{Op: OpCopy, Dst: localReg, Src1: valReg})
			return localReg, nil

		case *ast.DotExpression:
			objReg, err := l.lowerExpression(tgt.Left)
			if err != nil {
				return VReg{}, err
			}
			var typeName string
			if l.typeInfo != nil && l.typeInfo.Types != nil {
				if t, ok := l.typeInfo.Types[tgt.Left]; ok {
					if st, ok := t.(*typecheck.StructType); ok {
						typeName = st.Name
					}
				}
			}
			layout := l.structLayouts[typeName]
			if layout == nil {
				return VReg{}, fmt.Errorf("unknown struct/class for dot assignment %s.%s", typeName, tgt.Field)
			}
			offset, ok := layout.fields[tgt.Field]
			if !ok {
				return VReg{}, fmt.Errorf("field %s not found in %s", tgt.Field, typeName)
			}
			l.emit(&Instruction{Op: OpStore, Src1: objReg, IntVal: offset, Src2: valReg})
			return valReg, nil

		case *ast.IndexExpression:
			collReg, err := l.lowerExpression(tgt.Left)
			if err != nil {
				return VReg{}, err
			}
			idxReg, err := l.lowerExpression(tgt.Index)
			if err != nil {
				return VReg{}, err
			}

			isMap := false
			if l.typeInfo != nil && l.typeInfo.Types != nil {
				if t, ok := l.typeInfo.Types[tgt.Left]; ok {
					if _, okMap := t.(*typecheck.MapType); okMap {
						isMap = true
					}
				}
			}

			if isMap {
				// Map assignment: if key exists in [0..len), update bucket[8].
				// Else append at bucket[len]: store key at [len*16], val at [len*16+8], len++
				bucketsPtr := l.currentFn.NewVReg(TypePtr)
				l.emit(&Instruction{Op: OpLoad, Dst: bucketsPtr, Src1: collReg, IntVal: 0})
				mapLen := l.currentFn.NewVReg(TypeI64)
				l.emit(&Instruction{Op: OpLoad, Dst: mapLen, Src1: collReg, IntVal: 8})

				loopIdx := l.currentFn.NewVReg(TypeI64)
				l.emit(&Instruction{Op: OpConstInt, Dst: loopIdx, IntVal: 0})

				condBlock := l.currentFn.NewBlock("map_store_cond")
				bodyBlock := l.currentFn.NewBlock("map_store_body")
				stepBlock := l.currentFn.NewBlock("map_store_step")
				appendBlock := l.currentFn.NewBlock("map_store_append")
				doneBlock := l.currentFn.NewBlock("map_store_done")

				l.emit(&Instruction{Op: OpJmp, Target: condBlock})

				l.currentBlock = condBlock
				cmpLt := l.currentFn.NewVReg(TypeI8)
				l.emit(&Instruction{Op: OpLt, Dst: cmpLt, Src1: loopIdx, Src2: mapLen})
				l.emit(&Instruction{Op: OpBranch, Src1: cmpLt, Target: bodyBlock, ElseTgt: appendBlock})

				l.currentBlock = bodyBlock
				sixteen := l.currentFn.NewVReg(TypeI64)
				l.emit(&Instruction{Op: OpConstInt, Dst: sixteen, IntVal: 16})
				bOff := l.currentFn.NewVReg(TypeI64)
				l.emit(&Instruction{Op: OpMul, Dst: bOff, Src1: loopIdx, Src2: sixteen})
				bAddr := l.currentFn.NewVReg(TypePtr)
				l.emit(&Instruction{Op: OpAdd, Dst: bAddr, Src1: bucketsPtr, Src2: bOff})

				currKey := l.currentFn.NewVReg(idxReg.Type)
				l.emit(&Instruction{Op: OpLoad, Dst: currKey, Src1: bAddr, IntVal: 0})

				keyMatch := l.currentFn.NewVReg(TypeI8)
				l.emit(&Instruction{Op: OpEq, Dst: keyMatch, Src1: currKey, Src2: idxReg})

				foundBlock := l.currentFn.NewBlock("map_store_found")
				l.emit(&Instruction{Op: OpBranch, Src1: keyMatch, Target: foundBlock, ElseTgt: stepBlock})

				l.currentBlock = foundBlock
				l.emit(&Instruction{Op: OpStore, Src1: bAddr, IntVal: 8, Src2: valReg})
				l.emit(&Instruction{Op: OpJmp, Target: doneBlock})

				l.currentBlock = stepBlock
				one := l.currentFn.NewVReg(TypeI64)
				l.emit(&Instruction{Op: OpConstInt, Dst: one, IntVal: 1})
				nextIdx := l.currentFn.NewVReg(TypeI64)
				l.emit(&Instruction{Op: OpAdd, Dst: nextIdx, Src1: loopIdx, Src2: one})
				l.emit(&Instruction{Op: OpCopy, Dst: loopIdx, Src1: nextIdx})
				l.emit(&Instruction{Op: OpJmp, Target: condBlock})

				l.currentBlock = appendBlock
				// bucket addr = bucketsPtr + mapLen * 16
				appBOff := l.currentFn.NewVReg(TypeI64)
				l.emit(&Instruction{Op: OpMul, Dst: appBOff, Src1: mapLen, Src2: sixteen})
				appBAddr := l.currentFn.NewVReg(TypePtr)
				l.emit(&Instruction{Op: OpAdd, Dst: appBAddr, Src1: bucketsPtr, Src2: appBOff})

				l.emit(&Instruction{Op: OpStore, Src1: appBAddr, IntVal: 0, Src2: idxReg})
				l.emit(&Instruction{Op: OpStore, Src1: appBAddr, IntVal: 8, Src2: valReg})

				newLen := l.currentFn.NewVReg(TypeI64)
				l.emit(&Instruction{Op: OpAdd, Dst: newLen, Src1: mapLen, Src2: one})
				l.emit(&Instruction{Op: OpStore, Src1: collReg, IntVal: 8, Src2: newLen})
				l.emit(&Instruction{Op: OpJmp, Target: doneBlock})

				l.currentBlock = doneBlock
				return valReg, nil
			}

			// List store
			dataPtr := l.currentFn.NewVReg(TypePtr)
			l.emit(&Instruction{Op: OpLoad, Dst: dataPtr, Src1: collReg, IntVal: 0})
			scaleReg := l.currentFn.NewVReg(TypeI64)
			l.emit(&Instruction{Op: OpConstInt, Dst: scaleReg, IntVal: 8})
			byteOff := l.currentFn.NewVReg(TypeI64)
			l.emit(&Instruction{Op: OpMul, Dst: byteOff, Src1: idxReg, Src2: scaleReg})
			addrReg := l.currentFn.NewVReg(TypePtr)
			l.emit(&Instruction{Op: OpAdd, Dst: addrReg, Src1: dataPtr, Src2: byteOff})
			l.emit(&Instruction{Op: OpStore, Src1: addrReg, IntVal: 0, Src2: valReg})
			return valReg, nil

		default:
			return VReg{}, fmt.Errorf("assignment to target %T not supported", expr.Target)
		}

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
		// Check for stdlib module calls: module.func(...)
		if dot, ok := expr.Function.(*ast.DotExpression); ok {
			if ident, ok := dot.Left.(*ast.Identifier); ok {
				modName := ident.Value
				fnName := dot.Field

				if modName == "math" {
					var args []VReg
					for _, a := range expr.Arguments {
						ar, err := l.lowerExpression(a)
						if err != nil {
							return VReg{}, err
						}
						args = append(args, ar)
					}

					switch fnName {
					case "min":
						if len(args) == 2 {
							dst := l.currentFn.NewVReg(args[0].Type)
							cmp := l.currentFn.NewVReg(TypeI8)
							l.emit(&Instruction{Op: OpLt, Dst: cmp, Src1: args[0], Src2: args[1]})
							thenB := l.currentFn.NewBlock("min_then")
							elseB := l.currentFn.NewBlock("min_else")
							mergeB := l.currentFn.NewBlock("min_merge")
							l.emit(&Instruction{Op: OpBranch, Src1: cmp, Target: thenB, ElseTgt: elseB})

							l.currentBlock = thenB
							l.emit(&Instruction{Op: OpCopy, Dst: dst, Src1: args[0]})
							l.emit(&Instruction{Op: OpJmp, Target: mergeB})

							l.currentBlock = elseB
							l.emit(&Instruction{Op: OpCopy, Dst: dst, Src1: args[1]})
							l.emit(&Instruction{Op: OpJmp, Target: mergeB})

							l.currentBlock = mergeB
							return dst, nil
						}
					case "max":
						if len(args) == 2 {
							dst := l.currentFn.NewVReg(args[0].Type)
							cmp := l.currentFn.NewVReg(TypeI8)
							l.emit(&Instruction{Op: OpGt, Dst: cmp, Src1: args[0], Src2: args[1]})
							thenB := l.currentFn.NewBlock("max_then")
							elseB := l.currentFn.NewBlock("max_else")
							mergeB := l.currentFn.NewBlock("max_merge")
							l.emit(&Instruction{Op: OpBranch, Src1: cmp, Target: thenB, ElseTgt: elseB})

							l.currentBlock = thenB
							l.emit(&Instruction{Op: OpCopy, Dst: dst, Src1: args[0]})
							l.emit(&Instruction{Op: OpJmp, Target: mergeB})

							l.currentBlock = elseB
							l.emit(&Instruction{Op: OpCopy, Dst: dst, Src1: args[1]})
							l.emit(&Instruction{Op: OpJmp, Target: mergeB})

							l.currentBlock = mergeB
							return dst, nil
						}
					case "abs":
						if len(args) == 1 {
							dst := l.currentFn.NewVReg(args[0].Type)
							zero := l.currentFn.NewVReg(args[0].Type)
							l.emit(&Instruction{Op: OpConstInt, Dst: zero, IntVal: 0})
							cmp := l.currentFn.NewVReg(TypeI8)
							l.emit(&Instruction{Op: OpLt, Dst: cmp, Src1: args[0], Src2: zero})
							thenB := l.currentFn.NewBlock("abs_neg")
							elseB := l.currentFn.NewBlock("abs_pos")
							mergeB := l.currentFn.NewBlock("abs_merge")
							l.emit(&Instruction{Op: OpBranch, Src1: cmp, Target: thenB, ElseTgt: elseB})

							l.currentBlock = thenB
							l.emit(&Instruction{Op: OpNeg, Dst: dst, Src1: args[0]})
							l.emit(&Instruction{Op: OpJmp, Target: mergeB})

							l.currentBlock = elseB
							l.emit(&Instruction{Op: OpCopy, Dst: dst, Src1: args[0]})
							l.emit(&Instruction{Op: OpJmp, Target: mergeB})

							l.currentBlock = mergeB
							return dst, nil
						}
					}
				}
			}

			// Method call: receiver.method(...)
			rcvReg, err := l.lowerExpression(dot.Left)
			if err != nil {
				return VReg{}, err
			}
			var typeName string
			if l.typeInfo != nil && l.typeInfo.Types != nil {
				if t, ok := l.typeInfo.Types[dot.Left]; ok {
					if st, ok := t.(*typecheck.StructType); ok {
						typeName = st.Name
					}
				}
			}
			mFullName := typeName + "." + dot.Field
			targetFn, exists := l.functions[mFullName]
			if !exists {
				// Check parent class methods
				if layout := l.structLayouts[typeName]; layout != nil {
					for cur := layout.name; cur != ""; {
						if curFn, has := l.functions[cur+"."+dot.Field]; has {
							targetFn = curFn
							mFullName = cur + "." + dot.Field
							exists = true
							break
						}
						break
					}
				}
			}
			if !exists {
				return VReg{}, fmt.Errorf("method %s not found on %s", dot.Field, typeName)
			}

			var callArgs []VReg
			callArgs = append(callArgs, rcvReg) // self
			for _, a := range expr.Arguments {
				ar, err := l.lowerExpression(a)
				if err != nil {
					return VReg{}, err
				}
				callArgs = append(callArgs, ar)
			}

			dst := l.currentFn.NewVReg(targetFn.ReturnType)
			l.emit(&Instruction{
				Op:       OpCall,
				Dst:      dst,
				CallName: mFullName,
				Args:     callArgs,
			})
			return dst, nil
		}

		// 2. Direct function call: name(...)
		fnIdent, ok := expr.Function.(*ast.Identifier)
		if !ok {
			return VReg{}, fmt.Errorf("indirect calls not supported yet")
		}

		fnName := fnIdent.Value

		// Struct/Class instantiation via CallExpression
		if _, exists := l.structLayouts[fnName]; exists {
			si := &ast.StructInstantiation{
				Token: expr.Token,
				Name:  fnName,
				Args:  expr.Arguments,
			}
			return l.lowerExpression(si)
		}

		var args []VReg
		for _, a := range expr.Arguments {
			ar, err := l.lowerExpression(a)
			if err != nil {
				return VReg{}, err
			}
			args = append(args, ar)
		}

		// Built-in: exit(code)
		if fnName == "exit" && len(args) == 1 {
			l.emit(&Instruction{
				Op:   OpRet,
				Src1: args[0],
			})
			return VReg{ID: -1, Type: TypeVoid}, nil
		}

		// Built-in: print(...)
		if fnName == "print" {
			for _, a := range args {
				argToPrint := a
				if a.Type == TypeI64 {
					strReg := l.currentFn.NewVReg(TypePtr)
					l.emit(&Instruction{Op: OpIntToStr, Dst: strReg, Src1: a})
					argToPrint = strReg
				}
				l.emit(&Instruction{Op: OpPrint, Src1: argToPrint})
			}
			return VReg{ID: -1, Type: TypeVoid}, nil
		}

		// Built-in: println(...)
		if fnName == "println" {
			for _, a := range args {
				argToPrint := a
				if a.Type == TypeI64 {
					strReg := l.currentFn.NewVReg(TypePtr)
					l.emit(&Instruction{Op: OpIntToStr, Dst: strReg, Src1: a})
					argToPrint = strReg
				}
				l.emit(&Instruction{Op: OpPrintln, Src1: argToPrint})
			}
			if len(args) == 0 {
				emptyStr := l.currentFn.NewVReg(TypePtr)
				l.emit(&Instruction{Op: OpConstStr, Dst: emptyStr, StrVal: ""})
				l.emit(&Instruction{Op: OpPrintln, Src1: emptyStr})
			}
			return VReg{ID: -1, Type: TypeVoid}, nil
		}

		// Built-in: len(x)
		if fnName == "len" && len(args) == 1 {
			// Length is at offset 0 for string, offset 8 for list and map
			argExpr := expr.Arguments[0]
			offset := int64(0) // default string len
			if l.typeInfo != nil && l.typeInfo.Types != nil {
				if t, ok := l.typeInfo.Types[argExpr]; ok {
					if _, isList := t.(*typecheck.ListType); isList {
						offset = 8
					} else if _, isMap := t.(*typecheck.MapType); isMap {
						offset = 8
					}
				}
			}
			lenDst := l.currentFn.NewVReg(TypeI64)
			l.emit(&Instruction{Op: OpLoad, Dst: lenDst, Src1: args[0], IntVal: offset})
			return lenDst, nil
		}

		// Casts
		if fnName == "int" && len(args) == 1 {
			return args[0], nil
		}
		if fnName == "float" && len(args) == 1 {
			return args[0], nil
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

	case *ast.ErrExpression:
		// Build error struct on heap: layout (code: i64, msg: ptr)
		// err(message, code)
		var msgReg VReg
		if expr.Message != nil {
			mr, err := l.lowerExpression(expr.Message)
			if err != nil {
				return VReg{}, err
			}
			msgReg = mr
		} else {
			msgReg = l.currentFn.NewVReg(TypePtr)
			l.emit(&Instruction{Op: OpConstStr, Dst: msgReg, StrVal: "error"})
		}

		var codeReg VReg
		if expr.Code != nil {
			cr, err := l.lowerExpression(expr.Code)
			if err != nil {
				return VReg{}, err
			}
			codeReg = cr
		} else {
			codeReg = l.currentFn.NewVReg(TypeI64)
			l.emit(&Instruction{Op: OpConstInt, Dst: codeReg, IntVal: 500})
		}

		errObj := l.currentFn.NewVReg(TypePtr)
		sizeReg := l.currentFn.NewVReg(TypeI64)
		l.emit(&Instruction{Op: OpConstInt, Dst: sizeReg, IntVal: 16})
		l.emit(&Instruction{Op: OpAlloc, Dst: errObj, Src1: sizeReg})

		// fields: code at 0, msg at 8
		l.emit(&Instruction{Op: OpStore, Src1: errObj, IntVal: 0, Src2: codeReg})
		l.emit(&Instruction{Op: OpStore, Src1: errObj, IntVal: 8, Src2: msgReg})

		// Set global error slot
		l.emit(&Instruction{Op: OpSetErr, Src1: errObj})
		return errObj, nil

	default:
		return VReg{}, fmt.Errorf("expression %T not supported in native backend yet", e)
	}
}
