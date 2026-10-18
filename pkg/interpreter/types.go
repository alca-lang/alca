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
	ERROR_VAL    ValueType = "ERROR"
	RETURN_VAL   ValueType = "RETURN"
	BREAK_VAL    ValueType = "BREAK"
	CONTINUE_VAL ValueType = "CONTINUE"
	SECRET_VAL   ValueType = "SECRET"
	BUILTIN_VAL  ValueType = "BUILTIN"
	BLOB_VAL     ValueType = "BLOB"
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
}

// Type returns STRING_VAL.
func (v *StringValue) Type() ValueType { return STRING_VAL }

// Inspect returns the raw string content.
func (v *StringValue) Inspect() string { return v.Val }

// Clone returns an exact copy of the StringValue.
func (v *StringValue) Clone() Value { return &StringValue{Val: v.Val} }

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
}

// NewList creates a new ListValue initialized with elements.
func NewList(elements ...Value) *ListValue {
	if elements == nil {
		elements = make([]Value, 0)
	}
	return &ListValue{Elements: elements}
}

// Type returns LIST_VAL.
func (v *ListValue) Type() ValueType { return LIST_VAL }

// Inspect returns a bracketed comma-separated list of element representations.
func (v *ListValue) Inspect() string {
	v.mu.RLock()
	defer v.mu.RUnlock()

	var out strings.Builder
	out.WriteString("[")
	items := make([]string, len(v.Elements))
	for i, el := range v.Elements {
		if el == nil {
			items[i] = "null"
		} else if s, ok := el.(*StringValue); ok {
			items[i] = fmt.Sprintf("%q", s.Val)
		} else {
			items[i] = el.Inspect()
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
	v.Elements = append(v.Elements, val)
}

// Len returns the number of elements in the list.
func (v *ListValue) Len() int {
	v.mu.RLock()
	defer v.mu.RUnlock()

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

// StructDef represents the blueprint definition of a user-defined struct type.
type StructDef struct {
	Name   string
	Fields []*ast.StructField
}

// Type returns STRUCT_DEF.
func (v *StructDef) Type() ValueType { return STRUCT_DEF }

// Inspect returns a human-readable definition summary.
func (v *StructDef) Inspect() string {
	return fmt.Sprintf("<struct %s>", v.Name)
}

// Clone returns the definition itself as definitions are immutable.
func (v *StructDef) Clone() Value { return v }

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
	for _, f := range v.Def.Fields {
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

// ErrorValue represents a first-class error object in Alca.
type ErrorValue struct {
	Code    int64
	Message string
	Time    string
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

// ValuesEqual evaluates deep structural equality between two Alca values.
func ValuesEqual(a, b Value) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Handle cross-type numeric equality (e.g. 5 == 5.0).
	if a.Type() == INTEGER_VAL && b.Type() == FLOAT_VAL {
		return float64(a.(*IntegerValue).Val) == b.(*FloatValue).Val
	}
	if a.Type() == FLOAT_VAL && b.Type() == INTEGER_VAL {
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
		if len(valA.Elements) != len(valB.Elements) {
			return false
		}
		for i := range valA.Elements {
			if !ValuesEqual(valA.Elements[i], valB.Elements[i]) {
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
			if !ok || !ValuesEqual(vA, vB) {
				return false
			}
		}
		return true
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
			if !ok || !ValuesEqual(vA, vB) {
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
