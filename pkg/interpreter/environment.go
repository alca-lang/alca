// Package interpreter implements the tree-walking runtime engine for the Alca programming language.
package interpreter

import (
	"sync"
)

// Environment manages variable scopes, bindings, and constants in Alca.
type Environment struct {
	store  map[string]Value
	outer  *Environment
	consts map[string]bool
	mu     sync.RWMutex
}

// NewEnvironment creates a new root-level environment scope.
func NewEnvironment() *Environment {
	return &Environment{
		store:  make(map[string]Value),
		outer:  nil,
		consts: make(map[string]bool),
	}
}

// NewEnclosedEnvironment creates a new nested child scope enclosing an outer environment.
func NewEnclosedEnvironment(outer *Environment) *Environment {
	env := NewEnvironment()
	env.outer = outer
	return env
}

// Get looks up a variable binding starting in the current scope and traversing upwards.
func (e *Environment) Get(name string) (Value, bool) {
	e.mu.RLock()
	val, ok := e.store[name]
	e.mu.RUnlock()

	if ok {
		return val, true
	}
	if e.outer != nil {
		return e.outer.Get(name)
	}
	return nil, false
}

// Set binds or rebinds a variable name in the current local scope.
func (e *Environment) Set(name string, val Value) Value {
	e.mu.Lock()
	defer e.mu.Unlock()

	if val == nil {
		val = NULL
	}
	e.store[name] = val
	return val
}

// Update walks up the scope hierarchy to modify an existing variable binding.
// Returns false if the variable is not found in any scope or is a constant.
func (e *Environment) Update(name string, val Value) (Value, bool) {
	e.mu.Lock()
	if _, ok := e.store[name]; ok {
		if e.consts[name] {
			e.mu.Unlock()
			return nil, false
		}
		if val == nil {
			val = NULL
		}
		e.store[name] = val
		e.mu.Unlock()
		return val, true
	}
	e.mu.Unlock()

	if e.outer != nil {
		return e.outer.Update(name, val)
	}
	return nil, false
}

// SetConst binds an immutable constant in the current scope.
func (e *Environment) SetConst(name string, val Value) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if val == nil {
		val = NULL
	}
	e.store[name] = val
	e.consts[name] = true
}

// IsConst checks whether a variable name is designated as a constant.
func (e *Environment) IsConst(name string) bool {
	e.mu.RLock()
	isConst, ok := e.consts[name]
	e.mu.RUnlock()

	if ok {
		return isConst
	}
	if e.outer != nil {
		return e.outer.IsConst(name)
	}
	return false
}

// Delete removes a variable binding from the closest scope that defines it.
// This enables instant memory release upon null assignment.
func (e *Environment) Delete(name string) {
	e.mu.Lock()
	if _, ok := e.store[name]; ok {
		delete(e.store, name)
		delete(e.consts, name)
		e.mu.Unlock()
		return
	}
	e.mu.Unlock()

	if e.outer != nil {
		e.outer.Delete(name)
	}
}

// Outer returns the enclosing parent environment, or nil if this is the root.
func (e *Environment) Outer() *Environment {
	return e.outer
}

// All returns a snapshot copy of all bindings in the current local scope.
func (e *Environment) All() map[string]Value {
	e.mu.RLock()
	defer e.mu.RUnlock()

	copied := make(map[string]Value, len(e.store))
	for k, v := range e.store {
		copied[k] = v
	}
	return copied
}
