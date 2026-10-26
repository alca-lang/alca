// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

// Package interpreter implements the tree-walking runtime engine for the Alca programming language.
package interpreter

import (
	"sync"

	"github.com/alca-lang/alca/pkg/limits"
)

// Environment manages variable scopes, bindings, and constants in Alca.
// Thread-safe access is guarded with a reader-writer mutex, avoiding map allocations on updates.
type Environment struct {
	store  map[string]Value
	outer  *Environment
	consts map[string]bool
	mu     sync.RWMutex
}

// NewEnvironment creates a root-level environment scope.
func NewEnvironment() *Environment {
	return &Environment{
		store:  make(map[string]Value),
		consts: make(map[string]bool),
		outer:  nil,
	}
}

// NewEnclosedEnvironment creates a new nested child scope enclosing an outer environment.
func NewEnclosedEnvironment(outer *Environment) *Environment {
	return &Environment{
		store:  make(map[string]Value),
		consts: make(map[string]bool),
		outer:  outer,
	}
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
	if val == nil {
		val = NULL
	}
	e.mu.Lock()
	if e.store == nil {
		e.store = make(map[string]Value)
	}
	_, existed := e.store[name]
	e.store[name] = val
	e.mu.Unlock()
	if !existed {
		limits.Charge(limits.CostBinding)
	}
	return val
}

// Update walks up the scope hierarchy to modify an existing variable binding.
func (e *Environment) Update(name string, val Value) (Value, bool) {
	e.mu.Lock()
	if e.store != nil {
		if _, ok := e.store[name]; ok {
			if e.consts != nil && e.consts[name] {
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
	}
	e.mu.Unlock()

	if e.outer != nil {
		return e.outer.Update(name, val)
	}
	return nil, false
}

// SetConst binds an immutable constant in the current scope.
func (e *Environment) SetConst(name string, val Value) {
	if val == nil {
		val = NULL
	}
	e.mu.Lock()
	if e.store == nil {
		e.store = make(map[string]Value)
	}
	e.store[name] = val
	if e.consts == nil {
		e.consts = make(map[string]bool)
	}
	e.consts[name] = true
	e.mu.Unlock()
}

// IsConst checks whether the nearest binding of name is immutable. Scoping
// stops at the first environment that defines the name, so a local variable can
// legally shadow an outer constant.
func (e *Environment) IsConst(name string) bool {
	e.mu.RLock()
	if _, ok := e.store[name]; ok {
		isConst := e.consts != nil && e.consts[name]
		e.mu.RUnlock()
		return isConst
	}
	e.mu.RUnlock()
	if e.outer != nil {
		return e.outer.IsConst(name)
	}
	return false
}

// Delete removes a variable binding from the closest scope that defines it.
func (e *Environment) Delete(name string) {
	e.mu.Lock()
	if e.store != nil {
		if _, ok := e.store[name]; ok {
			delete(e.store, name)
			if e.consts != nil {
				delete(e.consts, name)
			}
			e.mu.Unlock()
			return
		}
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
