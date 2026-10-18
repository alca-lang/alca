package vm

// SymbolScope categorizes the lexical scope in which a symbol is bound.
type SymbolScope string

const (
	GlobalScope  SymbolScope = "GLOBAL"
	LocalScope   SymbolScope = "LOCAL"
	BuiltinScope SymbolScope = "BUILTIN"
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

// DefineBuiltin registers a pre-defined global builtin symbol at a designated index.
func (s *SymbolTable) DefineBuiltin(index int, name string) Symbol {
	symbol := Symbol{Name: name, Index: index, Scope: BuiltinScope}
	s.store[name] = symbol
	return symbol
}

// Resolve looks up a symbol name in current and enclosing scopes.
func (s *SymbolTable) Resolve(name string) (Symbol, bool) {
	obj, ok := s.store[name]
	if !ok && s.Outer != nil {
		return s.Outer.Resolve(name)
	}
	return obj, ok
}
