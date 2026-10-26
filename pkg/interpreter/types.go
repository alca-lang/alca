// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

// Package interpreter implements the tree-walking runtime engine for the Alca programming language.
package interpreter

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alca-lang/alca/pkg/ast"
	"github.com/alca-lang/alca/pkg/limits"
)

// ValueType represents the runtime category of a Alca value.
type ValueType string

// ValueType enumerations for all supported runtime values.
const (
	INTEGER_VAL  ValueType = "INTEGER"
	FLOAT_VAL    ValueType = "FLOAT"
	STRING_VAL   ValueType = "STRING"
	BOOL_VAL     ValueType = "BOOL"
	NULL_VAL     ValueType = "NULL"
	LIST_VAL     ValueType = "LIST"
	MAP_VAL      ValueType = "MAP"
	FUNC_VAL     ValueType = "FUNCTION"
	SHFUNC_VAL   ValueType = "SHFUNCTION"
	STRUCT_DEF   ValueType = "STRUCT_DEF"
	STRUCT_INST  ValueType = "STRUCT_INSTANCE"
	SUPER_VAL    ValueType = "SUPER"
	ERROR_VAL    ValueType = "ERROR"
	RETURN_VAL   ValueType = "RETURN"
	BREAK_VAL    ValueType = "BREAK"
	CONTINUE_VAL ValueType = "CONTINUE"
	SECRET_VAL   ValueType = "SECRET"
	BUILTIN_VAL  ValueType = "BUILTIN"
	BLOB_VAL     ValueType = "BLOB"
	ENUM_DEF     ValueType = "ENUM_DEF"
	ENUM_VAL     ValueType = "ENUM_VAL"
)

// Value is the core interface implemented by every runtime Alca value.
type Value interface {
	// Type returns the ValueType identifier.
	Type() ValueType
	// Inspect returns a string representation suitable for debugging or output.
	Inspect() string
	// Clone returns a deep copy of the value.
	Clone() Value
}

// Global singletons for immutable literals and control signals.
var (
	// NULL is the singleton representation of a null value.
	NULL = &NullValue{}
	// TRUE is the singleton representation of boolean true.
	TRUE = &BoolValue{Val: true}
	// FALSE is the singleton representation of boolean false.
	FALSE = &BoolValue{Val: false}
	// BREAK is the singleton break control signal.
	BREAK = &BreakSignal{}
	// CONTINUE is the singleton continue control signal.
	CONTINUE = &ContinueSignal{}
)

// -----------------------------------------------------------------------------
// Concrete Value Implementations
// -----------------------------------------------------------------------------

// IntegerValue wraps a signed 64-bit integer.
type IntegerValue struct {
	Val int64
}

var intCache [1153]*IntegerValue

func init() {
	for i := range intCache {
		intCache[i] = &IntegerValue{Val: int64(i - 128)}
	}
}

// NewInteger returns a cached or new IntegerValue for the given int64.
func NewInteger(v int64) *IntegerValue {
	if v >= -128 && v <= 1024 {
		return intCache[v+128]
	}
	return &IntegerValue{Val: v}
}

// Type returns INTEGER_VAL.
func (v *IntegerValue) Type() ValueType { return INTEGER_VAL }

// Inspect returns the decimal string representation of the integer.
func (v *IntegerValue) Inspect() string { return fmt.Sprintf("%d", v.Val) }

// Clone returns an exact copy of the IntegerValue.
func (v *IntegerValue) Clone() Value { return &IntegerValue{Val: v.Val} }

// FloatValue wraps a 64-bit IEEE 754 floating point number.
type FloatValue struct {
	Val float64
}

// Type returns FLOAT_VAL.
func (v *FloatValue) Type() ValueType { return FLOAT_VAL }

// Inspect returns the floating-point string representation.
func (v *FloatValue) Inspect() string {
	s := strconv.FormatFloat(v.Val, 'f', -1, 64)
	if !strings.ContainsRune(s, '.') {
		s += ".0"
	}
	return s
}

// Clone returns an exact copy of the FloatValue.
func (v *FloatValue) Clone() Value { return &FloatValue{Val: v.Val} }

// StringValue wraps a UTF-8 string.
type StringValue struct {
	Val string
	// runes caches the decoded characters of the string so that indexing and
	// iterating it do not re-decode the UTF-8 bytes on every access. The cache
	// is guarded because a string value can be shared between threads.
	mu    sync.Mutex
	runes []rune
}

// RuneSlice returns the string's characters, decoding them on first use.
func (v *StringValue) RuneSlice() []rune {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.runes == nil {
		v.runes = []rune(v.Val)
	}
	return v.runes
}

// Type returns STRING_VAL.
func (v *StringValue) Type() ValueType { return STRING_VAL }

// Inspect returns the raw string content.
func (v *StringValue) Inspect() string { return v.Val }

// Clone returns an exact copy of the StringValue.
func (v *StringValue) Clone() Value { return &StringValue{Val: v.Val} }

// asciiStrings holds the interned single-character strings for the ASCII range.
// Iterating a string yields one character per step, and interning them removes
// one allocation per character.
var asciiStrings = func() [128]*StringValue {
	var table [128]*StringValue
	for i := range table {
		table[i] = &StringValue{Val: string(rune(i))}
	}
	return table
}()

// SingleRuneString returns the interned single-character string for r. ASCII
// characters share one preallocated value; other characters allocate.
func SingleRuneString(r rune) *StringValue {
	if r >= 0 && r < 128 {
		return asciiStrings[r]
	}
	return &StringValue{Val: string(r)}
}

// BoolValue wraps a boolean true or false.
type BoolValue struct {
	Val bool
}

// Type returns BOOL_VAL.
func (v *BoolValue) Type() ValueType { return BOOL_VAL }

// Inspect returns "true" or "false".
func (v *BoolValue) Inspect() string { return strconv.FormatBool(v.Val) }

// Clone returns an exact copy of the BoolValue.
func (v *BoolValue) Clone() Value {
	if v.Val {
		return TRUE
	}
	return FALSE
}

// NullValue represents the absence of any value.
type NullValue struct{}

// Type returns NULL_VAL.
func (v *NullValue) Type() ValueType { return NULL_VAL }

// Inspect returns "null".
func (v *NullValue) Inspect() string { return "null" }

// Clone returns NULL.
func (v *NullValue) Clone() Value { return NULL }

// ListValue wraps an ordered slice of values with thread-safe synchronization.
type ListValue struct {
	mu       sync.RWMutex
	Elements []Value
	// Ints holds unboxed int64 elements when the list contains only ints.
	// When non-nil, Elements is nil and Ints is authoritative.
	Ints []int64
	// Runes holds the characters of a string that is being iterated. The list
	// then stands for one single-character string per rune, which avoids
	// allocating a StringValue per character. When non-nil, Elements and Ints
	// are nil and Runes is authoritative.
	Runes []rune
}

// ElementsSlice returns the list's elements as []Value, converting from the
// unboxed Ints or Runes representations when necessary.
func (v *ListValue) ElementsSlice() []Value {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if v.Ints != nil {
		out := make([]Value, len(v.Ints))
		for i, val := range v.Ints {
			out[i] = &IntegerValue{Val: val}
		}
		return out
	}
	if v.Runes != nil {
		out := make([]Value, len(v.Runes))
		for i, r := range v.Runes {
			out[i] = &StringValue{Val: string(r)}
		}
		return out
	}
	return v.Elements
}

// IntSlice returns the list's elements as []int64 if it holds unboxed ints,
// or nil otherwise.
func (v *ListValue) IntSlice() []int64 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.Ints
}

// RuneSlice returns the list's characters as []rune if it holds a rune-backed
// string, or nil otherwise.
func (v *ListValue) RuneSlice() []rune {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.Runes
}

// NewList creates a new ListValue initialized with elements.
func NewList(elements ...Value) *ListValue {
	if elements == nil {
		elements = make([]Value, 0)
	}
	limits.Charge(int64(len(elements)) * limits.CostValueSlot)
	return &ListValue{Elements: elements}
}

// NewIntList creates a new ListValue holding unboxed int64 elements.
func NewIntList(elements []int64) *ListValue {
	if elements == nil {
		elements = make([]int64, 0)
	}
	limits.Charge(int64(len(elements)) * limits.CostValueSlot)
	return &ListValue{Ints: elements}
}

// NewRuneList creates a new ListValue that stands for the characters of a
// string: one single-character string per rune.
func NewRuneList(runes []rune) *ListValue {
	if runes == nil {
		runes = make([]rune, 0)
	}
	limits.Charge(int64(len(runes)) * limits.CostValueSlot)
	return &ListValue{Runes: runes}
}

// Type returns LIST_VAL.
func (v *ListValue) Type() ValueType { return LIST_VAL }

// Inspect returns a bracketed comma-separated list of element representations.
func (v *ListValue) Inspect() string {
	v.mu.RLock()
	defer v.mu.RUnlock()

	var out strings.Builder
	out.WriteString("[")
	var items []string
	if v.Ints != nil {
		items = make([]string, len(v.Ints))
		for i, val := range v.Ints {
			items[i] = (&IntegerValue{Val: val}).Inspect()
		}
	} else if v.Runes != nil {
		items = make([]string, len(v.Runes))
		for i, r := range v.Runes {
			items[i] = fmt.Sprintf("%q", string(r))
		}
	} else {
		items = make([]string, len(v.Elements))
		for i, el := range v.Elements {
			if el == nil {
				items[i] = "null"
			} else if s, ok := el.(*StringValue); ok {
				items[i] = fmt.Sprintf("%q", s.Val)
			} else {
				items[i] = el.Inspect()
			}
		}
	}
	out.WriteString(strings.Join(items, ", "))
	out.WriteString("]")
	return out.String()
}

// Clone returns a deep copy of the ListValue.
func (v *ListValue) Clone() Value {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if v.Ints != nil {
		copied := make([]int64, len(v.Ints))
		copy(copied, v.Ints)
		return &ListValue{Ints: copied}
	}
	if v.Runes != nil {
		copied := make([]rune, len(v.Runes))
		copy(copied, v.Runes)
		return &ListValue{Runes: copied}
	}
	copied := make([]Value, len(v.Elements))
	for i, el := range v.Elements {
		if el != nil {
			copied[i] = el.Clone()
		} else {
			copied[i] = NULL
		}
	}
	return &ListValue{Elements: copied}
}

// Append appends a value to the list.
func (v *ListValue) Append(val Value) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if val == nil {
		val = NULL
	}
	v.materializeLocked()
	v.Elements = append(v.Elements, val)
	limits.Charge(limits.CostValueSlot)
}

// materializeLocked converts an unboxed int list into the boxed representation
// in place. It must be called before any operation that mutates or reorders
// the element slice.
func (v *ListValue) materializeLocked() {
	if v.Ints != nil {
		elems := make([]Value, len(v.Ints))
		for i, val := range v.Ints {
			elems[i] = &IntegerValue{Val: val}
		}
		v.Elements = elems
		v.Ints = nil
		limits.Charge(int64(len(elems)) * limits.CostBoxedValue)
		return
	}
	if v.Runes != nil {
		elems := make([]Value, len(v.Runes))
		for i, r := range v.Runes {
			elems[i] = &StringValue{Val: string(r)}
		}
		v.Elements = elems
		v.Runes = nil
		limits.Charge(int64(len(elems)) * limits.CostBoxedValue)
	}
}

// Materialize converts an unboxed int list into the boxed representation so
// that code operating on the element slice sees every element.
func (v *ListValue) Materialize() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.materializeLocked()
}

// Len returns the number of elements in the list.
func (v *ListValue) Len() int {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if v.Ints != nil {
		return len(v.Ints)
	}
	if v.Runes != nil {
		return len(v.Runes)
	}
	return len(v.Elements)
}

// MapValue wraps an ordered key-value dictionary with preserved key insertion sequence and thread-safe access.
type MapValue struct {
	mu    sync.RWMutex
	Pairs map[string]Value
	Order []string
}

// NewMap creates an empty initialized MapValue.
func NewMap() *MapValue {
	return &MapValue{
		Pairs: make(map[string]Value),
		Order: make([]string, 0),
	}
}

// Type returns MAP_VAL.
func (v *MapValue) Type() ValueType { return MAP_VAL }

// Inspect returns a brace-enclosed comma-separated key-value sequence.
func (v *MapValue) Inspect() string {
	v.mu.RLock()
	defer v.mu.RUnlock()

	var out strings.Builder
	out.WriteString("{")
	items := make([]string, 0, len(v.Order))
	for _, k := range v.Order {
		val := v.Pairs[k]
		valStr := "null"
		if val != nil {
			if s, ok := val.(*StringValue); ok {
				valStr = fmt.Sprintf("%q", s.Val)
			} else {
				valStr = val.Inspect()
			}
		}
		items = append(items, fmt.Sprintf("%s: %s", k, valStr))
	}
	out.WriteString(strings.Join(items, ", "))
	out.WriteString("}")
	return out.String()
}

// Clone returns a deep copy of the MapValue.
func (v *MapValue) Clone() Value {
	v.mu.RLock()
	defer v.mu.RUnlock()

	newMap := NewMap()
	for _, k := range v.Order {
		val := v.Pairs[k]
		if val != nil {
			newMap.Set(k, val.Clone())
		} else {
			newMap.Set(k, NULL)
		}
	}
	return newMap
}

// Get looks up a key in the map.
func (v *MapValue) Get(key string) (Value, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	val, ok := v.Pairs[key]
	return val, ok
}

// Set adds or updates a key-value pair, preserving insertion order for new keys.
func (v *MapValue) Set(key string, val Value) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if val == nil {
		val = NULL
	}
	if _, exists := v.Pairs[key]; !exists {
		v.Order = append(v.Order, key)
		limits.Charge(limits.CostMapEntry + int64(len(key))*limits.CostStringByte)
	}
	v.Pairs[key] = val
}

// Delete removes a key and updates the order slice.
func (v *MapValue) Delete(key string) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if _, exists := v.Pairs[key]; !exists {
		return
	}
	delete(v.Pairs, key)
	for i, k := range v.Order {
		if k == key {
			v.Order = append(v.Order[:i], v.Order[i+1:]...)
			break
		}
	}
	limits.Refund(limits.CostMapEntry + int64(len(key))*limits.CostStringByte)
}

// Has returns true if the map contains the specified key.
func (v *MapValue) Has(key string) bool {
	v.mu.RLock()
	defer v.mu.RUnlock()

	_, exists := v.Pairs[key]
	return exists
}

// Keys returns a slice of all keys in insertion order.
func (v *MapValue) Keys() []string {
	v.mu.RLock()
	defer v.mu.RUnlock()

	res := make([]string, len(v.Order))
	copy(res, v.Order)
	return res
}

// Values returns a slice of all values in insertion order.
func (v *MapValue) Values() []Value {
	v.mu.RLock()
	defer v.mu.RUnlock()

	res := make([]Value, len(v.Order))
	for i, k := range v.Order {
		res[i] = v.Pairs[k]
	}
	return res
}

// Len returns the count of key-value pairs.
func (v *MapValue) Len() int {
	v.mu.RLock()
	defer v.mu.RUnlock()

	return len(v.Pairs)
}

// FuncValue represents a user-defined function closure or shell function.
type FuncValue struct {
	Name       string
	Params     []*ast.FuncParam
	Body       *ast.BlockStatement
	Env        *Environment
	ReturnType *ast.TypeAnnotation
	IsShell    bool
}

// Type returns SHFUNC_VAL if declared as a shell function, or FUNC_VAL otherwise.
func (v *FuncValue) Type() ValueType {
	if v.IsShell {
		return SHFUNC_VAL
	}
	return FUNC_VAL
}

// Inspect returns a concise representation of the function signature.
func (v *FuncValue) Inspect() string {
	prefix := "func"
	if v.IsShell {
		prefix = "shfunc"
	}
	name := v.Name
	if name == "" {
		name = "<anonymous>"
	}
	return fmt.Sprintf("<%s %s>", prefix, name)
}

// Clone creates a shallow closure copy referencing the environment.
func (v *FuncValue) Clone() Value {
	return &FuncValue{
		Name:       v.Name,
		Params:     v.Params,
		Body:       v.Body,
		Env:        v.Env,
		ReturnType: v.ReturnType,
		IsShell:    v.IsShell,
	}
}

// MethodDef is a method attached to a class definition at runtime.
type MethodDef struct {
	Name       string
	Decl       *ast.FuncDeclaration
	Visibility string
	Owner      *StructDef
	// Compiled is set by a runtime that compiles the method body ahead of
	// time. The tree-walking interpreter evaluates the AST declaration; the
	// bytecode VM compiles the body and binds it to the receiver. Both use the
	// same class model, so a class behaves the same on every engine.
	Compiled CompiledMethod
}

// CompiledMethod is a method body that a runtime compiled ahead of time.
// Bind returns a callable that runs the method with the given receiver.
type CompiledMethod interface {
	Bind(receiver Value) Value
}

// StructDef represents the blueprint definition of a user-defined struct or
// class. Classes add a base-class link and a method table.
type StructDef struct {
	Name    string
	Fields  []*ast.StructField
	Parent  *StructDef
	Methods map[string]*MethodDef
	IsClass bool
	Env     *Environment
	Nested  map[string]*StructDef
}

// Type returns STRUCT_DEF.
func (v *StructDef) Type() ValueType { return STRUCT_DEF }

// Inspect returns a human-readable definition summary.
func (v *StructDef) Inspect() string {
	if v.IsClass {
		return fmt.Sprintf("<class %s>", v.Name)
	}
	return fmt.Sprintf("<struct %s>", v.Name)
}

// Clone returns the definition itself as definitions are immutable.
func (v *StructDef) Clone() Value { return v }

// AllFields returns inherited fields first, followed by the class's own fields.
func (v *StructDef) AllFields() []*ast.StructField {
	var chain []*StructDef
	for cur := v; cur != nil; cur = cur.Parent {
		chain = append(chain, cur)
	}
	var out []*ast.StructField
	for i := len(chain) - 1; i >= 0; i-- {
		out = append(out, chain[i].Fields...)
	}
	return out
}

// LookupField walks the inheritance chain.
func (v *StructDef) LookupField(name string) (*ast.StructField, bool) {
	for cur := v; cur != nil; cur = cur.Parent {
		for _, f := range cur.Fields {
			if f.Name == name {
				return f, true
			}
		}
	}
	return nil, false
}

// LookupMethod walks the inheritance chain and returns the most derived method.
func (v *StructDef) LookupMethod(name string) *MethodDef {
	for cur := v; cur != nil; cur = cur.Parent {
		if m, ok := cur.Methods[name]; ok {
			return m
		}
	}
	return nil
}

// Inherits reports whether v is parent or derives from it.
func (v *StructDef) Inherits(parent *StructDef) bool {
	for cur := v; cur != nil; cur = cur.Parent {
		if cur == parent || cur.Name == parent.Name {
			return true
		}
	}
	return false
}

// EnumDef represents an enumeration definition.
type EnumDef struct {
	Name     string
	Variants map[string]*EnumValue
	IsPublic bool
}

func (ed *EnumDef) Type() ValueType { return ENUM_DEF }
func (ed *EnumDef) Inspect() string { return fmt.Sprintf("<enum %s>", ed.Name) }
func (ed *EnumDef) Clone() Value    { return ed }

// EnumValue represents a specific variant of an enum.
type EnumValue struct {
	EnumName string
	Name     string
	Tag      int64
	Val      Value
}

func (ev *EnumValue) Type() ValueType { return ENUM_VAL }
func (ev *EnumValue) Inspect() string { return fmt.Sprintf("%s.%s", ev.EnumName, ev.Name) }
func (ev *EnumValue) Clone() Value {
	var cVal Value
	if ev.Val != nil {
		cVal = ev.Val.Clone()
	}
	return &EnumValue{
		EnumName: ev.EnumName,
		Name:     ev.Name,
		Tag:      ev.Tag,
		Val:      cVal,
	}
}

// SuperValue is the runtime value of 'super' inside a method. It carries the
// receiver and the class at which method lookup must start.
type SuperValue struct {
	Instance *StructInstance
	Start    *StructDef
}

// Type returns SUPER_VAL.
func (v *SuperValue) Type() ValueType { return SUPER_VAL }

// Inspect returns a short super-class description.
func (v *SuperValue) Inspect() string {
	if v.Start == nil {
		return "<super>"
	}
	return fmt.Sprintf("<super %s>", v.Start.Name)
}

// Clone returns the immutable super proxy.
func (v *SuperValue) Clone() Value { return v }

// StructInstance represents an instantiated struct with concrete field values.
type StructInstance struct {
	Def    *StructDef
	Fields map[string]Value
}

// Type returns STRUCT_INST.
func (v *StructInstance) Type() ValueType { return STRUCT_INST }

// Inspect returns a struct literal formatted string.
func (v *StructInstance) Inspect() string {
	var out strings.Builder
	out.WriteString(v.Def.Name)
	out.WriteString("{")
	fields := make([]string, 0, len(v.Fields))
	for _, f := range v.Def.AllFields() {
		val := v.Fields[f.Name]
		valStr := "null"
		if val != nil {
			if s, ok := val.(*StringValue); ok {
				valStr = fmt.Sprintf("%q", s.Val)
			} else {
				valStr = val.Inspect()
			}
		}
		fields = append(fields, fmt.Sprintf("%s: %s", f.Name, valStr))
	}
	out.WriteString(strings.Join(fields, ", "))
	out.WriteString("}")
	return out.String()
}

// Clone returns a deep copy of the struct instance.
func (v *StructInstance) Clone() Value {
	newFields := make(map[string]Value, len(v.Fields))
	for k, val := range v.Fields {
		if val != nil {
			newFields[k] = val.Clone()
		} else {
			newFields[k] = NULL
		}
	}
	return &StructInstance{
		Def:    v.Def,
		Fields: newFields,
	}
}

// Standard error code categories
const (
	ErrCodeInvalidArg   int64 = 400
	ErrCodeUnauthorized int64 = 401
	ErrCodePermission   int64 = 403
	ErrCodeNotFound     int64 = 404
	ErrCodeUnsupported  int64 = 405
	ErrCodeTimeout      int64 = 408
	ErrCodeInternal     int64 = 500
	ErrCodeIO           int64 = 502
)

// ErrorValue represents a first-class error object in Alca.
type ErrorValue struct {
	Code    int64
	Message string
	Time    string
}

// Category returns a human-readable symbolic string category for the error code.
func (v *ErrorValue) Category() string {
	switch v.Code {
	case ErrCodeInvalidArg:
		return "invalid_argument"
	case ErrCodeUnauthorized:
		return "unauthorized"
	case ErrCodePermission:
		return "permission_denied"
	case ErrCodeNotFound:
		return "not_found"
	case ErrCodeUnsupported:
		return "unsupported"
	case ErrCodeTimeout:
		return "timeout"
	case ErrCodeInternal:
		return "internal_error"
	case ErrCodeIO:
		return "io_error"
	default:
		return "error"
	}
}

// CallMethod implements MethodProvider for ErrorValue.
func (v *ErrorValue) CallMethod(name string, args []Value) (Value, error) {
	switch name {
	case "code":
		return &IntegerValue{Val: v.Code}, nil
	case "message":
		return &StringValue{Val: v.Message}, nil
	case "category":
		return &StringValue{Val: v.Category()}, nil
	case "time":
		return &StringValue{Val: v.Time}, nil
	default:
		return nil, fmt.Errorf("error has no method %q", name)
	}
}

// NewError creates an ErrorValue with current UTC timestamp.
func NewError(code int64, message string) *ErrorValue {
	return &ErrorValue{
		Code:    code,
		Message: message,
		Time:    time.Now().UTC().Format(time.RFC3339),
	}
}

// Type returns ERROR_VAL.
func (v *ErrorValue) Type() ValueType { return ERROR_VAL }

// Inspect returns a formatted error representation.
func (v *ErrorValue) Inspect() string {
	return fmt.Sprintf("Error(%d: %s at %s)", v.Code, v.Message, v.Time)
}

// Clone returns an exact copy of the ErrorValue.
func (v *ErrorValue) Clone() Value {
	return &ErrorValue{
		Code:    v.Code,
		Message: v.Message,
		Time:    v.Time,
	}
}

// ReturnValue is an internal control flow wrapper indicating an explicit return statement.
type ReturnValue struct {
	Val Value
}

// Type returns RETURN_VAL.
func (v *ReturnValue) Type() ValueType { return RETURN_VAL }

// Inspect forwards the inspection to the contained value.
func (v *ReturnValue) Inspect() string {
	if v.Val != nil {
		return v.Val.Inspect()
	}
	return "null"
}

// Clone returns a cloned ReturnValue.
func (v *ReturnValue) Clone() Value {
	if v.Val != nil {
		return &ReturnValue{Val: v.Val.Clone()}
	}
	return &ReturnValue{Val: NULL}
}

// BreakSignal is an internal control flow signal indicating a loop break.
type BreakSignal struct{}

// Type returns BREAK_VAL.
func (v *BreakSignal) Type() ValueType { return BREAK_VAL }

// Inspect returns "break".
func (v *BreakSignal) Inspect() string { return "break" }

// Clone returns BREAK.
func (v *BreakSignal) Clone() Value { return BREAK }

// ContinueSignal is an internal control flow signal indicating a loop continuation.
type ContinueSignal struct{}

// Type returns CONTINUE_VAL.
func (v *ContinueSignal) Type() ValueType { return CONTINUE_VAL }

// Inspect returns "continue".
func (v *ContinueSignal) Inspect() string { return "continue" }

// Clone returns CONTINUE.
func (v *ContinueSignal) Clone() Value { return CONTINUE }

// SecretValue represents sensitive data that is masked when inspected.
type SecretValue struct {
	Val string
}

// Type returns SECRET_VAL.
func (v *SecretValue) Type() ValueType { return SECRET_VAL }

// Inspect returns the redaction placeholder to prevent sensitive leaks in logs.
func (v *SecretValue) Inspect() string { return "***REDACTED***" }

// Clone returns an exact copy of the SecretValue.
func (v *SecretValue) Clone() Value { return &SecretValue{Val: v.Val} }

// BlobValue represents raw binary data.
type BlobValue struct {
	Data []byte
}

// Type returns BLOB_VAL.
func (v *BlobValue) Type() ValueType { return BLOB_VAL }

// Inspect returns byte count summary.
func (v *BlobValue) Inspect() string {
	return fmt.Sprintf("<blob %d bytes>", len(v.Data))
}

// Clone returns a deep copy of the underlying byte slice.
func (v *BlobValue) Clone() Value {
	return &BlobValue{Data: bytes.Clone(v.Data)}
}

// BuiltinFunction represents an interpreter built-in function implemented in Go.
type BuiltinFunction struct {
	Name string
	Fn   func(args ...Value) Value
}

// Type returns BUILTIN_VAL.
func (v *BuiltinFunction) Type() ValueType { return BUILTIN_VAL }

// Inspect returns the builtin function signature identifier.
func (v *BuiltinFunction) Inspect() string {
	return fmt.Sprintf("<builtin %s>", v.Name)
}

// Clone returns the immutable builtin reference.
func (v *BuiltinFunction) Clone() Value { return v }

// -----------------------------------------------------------------------------
// Value Helper Functions
// -----------------------------------------------------------------------------

// IsTruthy evaluates if a Alca value evaluates to true in boolean contexts.
// In Alca, null, false, 0, 0.0, and "" are falsy; everything else is truthy.
func IsTruthy(v Value) bool {
	if v == nil {
		return false
	}
	switch val := v.(type) {
	case *NullValue:
		return false
	case *BoolValue:
		return val.Val
	case *IntegerValue:
		return val.Val != 0
	case *FloatValue:
		return val.Val != 0.0
	case *StringValue:
		return val.Val != ""
	default:
		return true
	}
}

// ValuesEqual evaluates deep equality while allowing numeric int/float equality.
func ValuesEqual(a, b Value) bool {
	return valuesEqual(a, b, false)
}

// StrictEqual requires matching runtime types recursively as well as equal values.
func StrictEqual(a, b Value) bool {
	return valuesEqual(a, b, true)
}

// LooseEqual compares primitive values after limited numeric coercion.
func LooseEqual(a, b Value) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if a.Type() == b.Type() {
		return ValuesEqual(a, b)
	}
	if left, ok := equalityNumber(a); ok {
		if right, ok := equalityNumber(b); ok {
			return left == right
		}
	}
	return false
}

func equalityNumber(value Value) (float64, bool) {
	switch v := value.(type) {
	case *IntegerValue:
		return float64(v.Val), true
	case *FloatValue:
		return v.Val, true
	case *BoolValue:
		if v.Val {
			return 1, true
		}
		return 0, true
	case *StringValue:
		text := strings.TrimSpace(v.Val)
		if text == "" {
			return 0, true
		}
		number, err := strconv.ParseFloat(text, 64)
		return number, err == nil
	default:
		return 0, false
	}
}

func valuesEqual(a, b Value, strict bool) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Handle cross-type numeric equality (e.g. 5 == 5.0).
	if !strict && a.Type() == INTEGER_VAL && b.Type() == FLOAT_VAL {
		return float64(a.(*IntegerValue).Val) == b.(*FloatValue).Val
	}
	if !strict && a.Type() == FLOAT_VAL && b.Type() == INTEGER_VAL {
		return a.(*FloatValue).Val == float64(b.(*IntegerValue).Val)
	}

	if a.Type() != b.Type() {
		return false
	}

	switch valA := a.(type) {
	case *NullValue:
		return true
	case *BoolValue:
		return valA.Val == b.(*BoolValue).Val
	case *IntegerValue:
		return valA.Val == b.(*IntegerValue).Val
	case *FloatValue:
		return valA.Val == b.(*FloatValue).Val
	case *StringValue:
		return valA.Val == b.(*StringValue).Val
	case *SecretValue:
		return valA.Val == b.(*SecretValue).Val
	case *BlobValue:
		return bytes.Equal(valA.Data, b.(*BlobValue).Data)
	case *ErrorValue:
		valB := b.(*ErrorValue)
		return valA.Code == valB.Code && valA.Message == valB.Message
	case *ListValue:
		valB := b.(*ListValue)
		aElems := valA.ElementsSlice()
		bElems := valB.ElementsSlice()
		if len(aElems) != len(bElems) {
			return false
		}
		for i := range aElems {
			if !valuesEqual(aElems[i], bElems[i], strict) {
				return false
			}
		}
		return true
	case *MapValue:
		valB := b.(*MapValue)
		if len(valA.Pairs) != len(valB.Pairs) {
			return false
		}
		for k, vA := range valA.Pairs {
			vB, ok := valB.Pairs[k]
			if !ok || !valuesEqual(vA, vB, strict) {
				return false
			}
		}
		return true
	case *EnumValue:
		valB := b.(*EnumValue)
		return valA.EnumName == valB.EnumName && valA.Name == valB.Name
	case *StructInstance:
		valB := b.(*StructInstance)
		if valA.Def.Name != valB.Def.Name {
			return false
		}
		if len(valA.Fields) != len(valB.Fields) {
			return false
		}
		for k, vA := range valA.Fields {
			vB, ok := valB.Fields[k]
			if !ok || !valuesEqual(vA, vB, strict) {
				return false
			}
		}
		return true
	case *StructDef:
		return valA.Name == b.(*StructDef).Name
	case *FuncValue:
		return valA == b.(*FuncValue)
	case *BuiltinFunction:
		return valA.Name == b.(*BuiltinFunction).Name
	default:
		return a.Inspect() == b.Inspect()
	}
}

// Concat joins two strings and accounts for the result on the shared memory
// meter. The joined string is a fresh allocation, so it counts against the
// allocation budget: a concatenation loop allocates quadratically, and the
// budget is what keeps such a loop from exhausting the process.
func Concat(a, b string) string {
	joined := a + b
	limits.ChargeTransient(int64(len(joined)) * limits.CostStringByte)
	return joined
}

// CoerceToString converts any runtime value into a string representation.
// Unlike Inspect(), StringValue returns its raw string without quotes.
func CoerceToString(v Value) string {
	if v == nil {
		return "null"
	}
	switch val := v.(type) {
	case *StringValue:
		return val.Val
	case *ReturnValue:
		return CoerceToString(val.Val)
	default:
		return val.Inspect()
	}
}
