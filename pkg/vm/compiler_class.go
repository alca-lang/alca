package vm

import (
	"github.com/alca-lang/alca/pkg/ast"
)

// compileTx compiles a transaction block. The block runs in the frame of the
// function; the VM snapshots the local slots, so that the changes of the block
// are discarded when it fails.
func (c *Compiler) compileTx(n *ast.TxStatement) error {
	c.emit(OpTxBegin)
	if err := c.Compile(n.Body); err != nil {
		return err
	}
	c.emit(OpTxEnd)
	return c.err
}

// compileMock compiles a mock statement. A mock replaces the binding of the
// mocked name, which is what assigning the replacement to that binding does on
// the tree-walking interpreter.
func (c *Compiler) compileMock(n *ast.MockStatement) error {
	id, ok := n.Target.(*ast.Identifier)
	if !ok {
		return unsupported("a mock of a computed target", n.Token)
	}
	symbol, ok := c.symbolTable.Resolve(id.Value)
	if !ok {
		symbol = c.symbolTable.Define(id.Value)
	}
	if err := c.Compile(n.Replacement); err != nil {
		return err
	}
	c.emitSet(symbol)
	return c.err
}

// emitDecorators emits the decorators of a declaration. The decorators that
// have an effect on the program (@oom_guard, @error_log and @drop_on_oom) are
// applied when the declaration is executed; the others are ignored, exactly
// like the tree-walking interpreter ignores them.
//
// The arguments of a decorator have to be literals: the VM applies the
// decorator when the declaration runs, and a literal is the only form that
// both engines can evaluate the same way.
func (c *Compiler) emitDecorators(decorators []*ast.Decorator, slot int) {
	for _, d := range decorators {
		if d == nil {
			continue
		}
		args := make([]string, 0, len(d.Args))
		for _, arg := range d.Args {
			literal, ok := arg.(*ast.StringLiteral)
			if !ok {
				return
			}
			args = append(args, literal.Value)
		}
		spec := &DecoratorSpec{Name: d.Name, Args: args, Slot: slot}
		c.emit(OpApplyDecorator, c.addConstant(spec))
	}
}

// compileClass compiles a class declaration.
//
// The class definition itself is built when the declaration runs, because the
// base class is only known at that point. The compiled method bodies travel
// with the declaration in a template: every receiver of the class shares that
// code, and the receiver is bound to the method when it is called.
func (c *Compiler) compileClass(n *ast.ClassDeclaration) error {
	// Nested types are ordinary struct declarations in the scope of the class.
	for _, nested := range n.NestedStructs {
		if nested == nil {
			continue
		}
		if err := c.Compile(nested); err != nil {
			return err
		}
	}

	methods := make([]*ClassTemplateMethod, 0, len(n.Methods))
	for _, m := range n.Methods {
		if m == nil || m.Func == nil {
			continue
		}
		fn, err := c.compileMethod(n.Name, m)
		if err != nil {
			return err
		}
		methods = append(methods, &ClassTemplateMethod{
			Name:       m.Func.Name,
			Visibility: m.Visibility,
			Fn:         fn,
		})
	}

	parentGlobal := -1
	if n.Parent != "" {
		sym, ok := c.symbolTable.Resolve(n.Parent)
		if !ok || sym.Scope != GlobalScope {
			return unsupported("the base class must be a class declared at the top level", n.ParentToken)
		}
		parentGlobal = sym.Index
	}

	symbol := c.symbolTable.Define(n.Name)
	template := &ClassTemplate{
		Name:         n.Name,
		Fields:       n.Fields,
		Methods:      methods,
		ParentGlobal: parentGlobal,
	}
	// The base class is addressed by its global slot; zero means "no base".
	c.emit(OpMakeClass, c.addConstant(template), parentGlobal+1)
	c.emitSet(symbol)
	return c.err
}

// compileMethod compiles the body of a method.
//
// `self` and `super` are the first two upvalues of a method: the receiver and
// the proxy that reaches the base class. Everything else the body captures is
// resolved the same way as in a closure.
func (c *Compiler) compileMethod(className string, m *ast.MethodDeclaration) (*CompiledFunction, error) {
	c.enterScope()
	c.symbolTable.store["self"] = Symbol{Name: "self", Index: 0, Scope: FreeScope}
	c.symbolTable.store["super"] = Symbol{Name: "super", Index: 1, Scope: FreeScope}
	c.symbolTable.FreeSymbols = []Symbol{
		{Name: "self", Index: 0, Scope: FreeScope},
		{Name: "super", Index: 1, Scope: FreeScope},
	}
	for _, p := range m.Func.Params {
		c.symbolTable.Define(p.Name)
	}
	c.predeclareLocals(m.Func.Body)
	if len(m.Func.Params) > 0xFF {
		c.leaveScope()
		return nil, c.err
	}
	if err := c.Compile(m.Func.Body); err != nil {
		c.leaveScope()
		return nil, err
	}
	if !c.lastInstructionIs(OpReturnValue) {
		c.emit(OpReturn)
	}
	numLocals := c.symbolTable.numDefinitions
	free := c.symbolTable.FreeSymbols
	instructions := c.leaveScope()
	return &CompiledFunction{
		Instructions:  instructions,
		NumLocals:     numLocals,
		NumParameters: len(m.Func.Params),
		Name:          className + "." + m.Func.Name,
		NumFree:       len(free),
	}, nil
}
