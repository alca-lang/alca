// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package vm

// SymbolScope categorizes the lexical scope in which a symbol is bound.
type SymbolScope string

const (
	GlobalScope  SymbolScope = "GLOBAL"
	LocalScope   SymbolScope = "LOCAL"
	BuiltinScope SymbolScope = "BUILTIN"
	// FreeScope marks a variable that is local to an enclosing function and is
	// captured as an upvalue of the current function.
	FreeScope SymbolScope = "FREE"
)

// Symbol represents a resolved named identifier bound to a memory location.
type Symbol struct {
	Name  string
	Scope SymbolScope
	Index int
}

// SymbolTable maintains lexical scoping and variable index assignments.
type SymbolTable struct {
	Outer          *SymbolTable
	store          map[string]Symbol
	numDefinitions int
	// FreeSymbols records the upvalues captured by the function whose scope this
	// table represents, in capture order. Each entry is the symbol as seen from
	// the enclosing function (a local slot, or one of its own upvalues).
	FreeSymbols []Symbol
}

// NewSymbolTable creates a root global symbol table.
func NewSymbolTable() *SymbolTable {
	return &SymbolTable{
		store: make(map[string]Symbol),
	}
}

// NewEnclosedSymbolTable creates a nested local scope symbol table.
func NewEnclosedSymbolTable(outer *SymbolTable) *SymbolTable {
	s := NewSymbolTable()
	s.Outer = outer
	return s
}

// Define binds a variable in the current scope and returns its Symbol.
func (s *SymbolTable) Define(name string) Symbol {
	if existing, ok := s.store[name]; ok {
		return existing
	}
	symbol := Symbol{Name: name, Index: s.numDefinitions}
	if s.Outer == nil {
		symbol.Scope = GlobalScope
	} else {
		symbol.Scope = LocalScope
	}

	s.store[name] = symbol
	s.numDefinitions++
	return symbol
}

// DefineGlobal binds a name in the program-wide symbol table, regardless of the
// table's current nesting. Public declarations use this so every function sees
// the same global slot.
func (s *SymbolTable) DefineGlobal(name string) Symbol {
	root := s
	for root.Outer != nil {
		root = root.Outer
	}
	return root.Define(name)
}

// DefineBuiltin registers a pre-defined global builtin symbol at a designated index.
func (s *SymbolTable) DefineBuiltin(index int, name string) Symbol {
	symbol := Symbol{Name: name, Index: index, Scope: BuiltinScope}
	s.store[name] = symbol
	return symbol
}

// GlobalNames returns global symbol names ordered by slot index. The slice has
// one entry per global slot; concurrently-defined names occupy their own slot
// and never collide, so the order matches the VM's globals array exactly.
func (s *SymbolTable) GlobalNames() []string {
	names := make([]string, s.numDefinitions)
	for name, sym := range s.store {
		if sym.Scope == GlobalScope && sym.Index >= 0 && sym.Index < len(names) {
			names[sym.Index] = name
		}
	}
	return names
}

// Resolve looks up a symbol in the current table and its enclosing tables.
// A variable that is local to an enclosing function is captured as an upvalue of
// the current function (FreeScope) and recorded in this table's FreeSymbols, so
// closures see the enclosing frame's slot by reference exactly like the
// tree-walking interpreter.
func (s *SymbolTable) Resolve(name string) (Symbol, bool) {
	if obj, ok := s.store[name]; ok {
		return obj, true
	}
	if s.Outer == nil {
		return Symbol{}, false
	}
	obj, ok := s.Outer.Resolve(name)
	if !ok {
		return Symbol{}, false
	}
	if obj.Scope == GlobalScope || obj.Scope == BuiltinScope {
		// Globals and builtins are reachable from any frame without captures.
		return obj, true
	}
	return s.defineFree(obj), true
}

// defineFree records original as an upvalue of this function, reusing the same
// upvalue slot when the name was already captured.
func (s *SymbolTable) defineFree(original Symbol) Symbol {
	for i, sym := range s.FreeSymbols {
		if sym.Name == original.Name {
			return Symbol{Name: sym.Name, Index: i, Scope: FreeScope}
		}
	}
	s.FreeSymbols = append(s.FreeSymbols, original)
	return Symbol{Name: original.Name, Index: len(s.FreeSymbols) - 1, Scope: FreeScope}
}
