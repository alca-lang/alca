package vm

import (
	"fmt"

	"github.com/alca-lang/alca/pkg/ast"
	"github.com/alca-lang/alca/pkg/interpreter"
)

// ClassTemplate is the compile-time description of a class. The VM turns it
// into a runtime definition when the class declaration is executed, because
// the base class is only known at that point.
type ClassTemplate struct {
	Name    string
	Fields  []*ast.StructField
	Methods []*ClassTemplateMethod
	// ParentGlobal is the global slot of the base class, or -1 when the class
	// has no base class.
	ParentGlobal int
	// Nested are the struct declarations inside the class body.
	Nested []*ast.StructDeclaration
}

// ClassTemplateMethod is one method of a class template.
type ClassTemplateMethod struct {
	Name       string
	Visibility string
	Fn         *CompiledFunction
}

// Type returns STRUCT_DEF so a class template can be stored like any other
// definition value.
func (t *ClassTemplate) Type() interpreter.ValueType { return interpreter.STRUCT_DEF }

// Inspect returns a short description of the class template.
func (t *ClassTemplate) Inspect() string { return fmt.Sprintf("<class %s>", t.Name) }

// Clone returns the template itself; templates are immutable.
func (t *ClassTemplate) Clone() interpreter.Value { return t }

// ClassMethod is a compiled method body. Binding it to a receiver produces a
// closure that carries the receiver and the super proxy as its first two
// upvalues, which is how a method reaches `self` and `super`.
type ClassMethod struct {
	Fn     *CompiledFunction
	Parent *interpreter.StructDef
}

// Bind returns a callable that runs the method with the given receiver.
func (m *ClassMethod) Bind(receiver interpreter.Value) interpreter.Value {
	inst, _ := receiver.(*interpreter.StructInstance)
	closure := &CompiledFunction{
		Instructions:  m.Fn.Instructions,
		NumLocals:     m.Fn.NumLocals,
		NumParameters: m.Fn.NumParameters,
		Name:          m.Fn.Name,
		NumFree:       2,
		Upvalues: []*Upvalue{
			{stackIndex: -1, closed: FromValue(inst)},
			{stackIndex: -1, closed: FromValue(&interpreter.SuperValue{Instance: inst, Start: m.Parent})},
		},
		Pool:    m.Fn.Pool,
		Globals: m.Fn.Globals,
		// The closure shares the code of the method, so the JIT compiles it
		// once for every receiver of the class.
		Template: m.Fn,
	}
	return closure
}

// BuildClassDef turns a class template into the runtime definition of a class.
// parent is the definition of the base class, or nil when the class has none.
// It is exported so the JIT tiers can build the same definition.
func BuildClassDef(tmpl *ClassTemplate, parent *interpreter.StructDef) *interpreter.StructDef {
	return buildClassDef(tmpl, parent)
}

// buildClassDef turns a class template into the runtime definition of a class.
// parent is the definition of the base class, or nil when the class has none.
func buildClassDef(tmpl *ClassTemplate, parent *interpreter.StructDef) *interpreter.StructDef {
	def := &interpreter.StructDef{
		Name:    tmpl.Name,
		Fields:  tmpl.Fields,
		Parent:  parent,
		IsClass: true,
		Methods: map[string]*interpreter.MethodDef{},
		Nested:  map[string]*interpreter.StructDef{},
	}
	for _, m := range tmpl.Methods {
		if m == nil || m.Fn == nil {
			continue
		}
		def.Methods[m.Name] = &interpreter.MethodDef{
			Name:       m.Name,
			Visibility: m.Visibility,
			Owner:      def,
			Compiled:   &ClassMethod{Fn: m.Fn, Parent: parent},
		}
	}
	return def
}
