// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

// Package interpreter implements the tree-walking runtime engine for the Alca programming language.
package interpreter

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/alca-lang/alca/pkg/limits"
)

// PrintHook represents a hook function for capturing or redirecting standard print output.
type PrintHook func(output string)

// MethodEvaluator represents a callback invoked by higher-order methods (e.g. map, filter) to evaluate closures.
type MethodEvaluator func(fn Value, args []Value) (Value, error)

var (
	printHookMu     sync.RWMutex
	activePrintHook PrintHook

	evaluatorMu     sync.RWMutex
	activeEvaluator MethodEvaluator
)

// SetPrintHook registers a custom hook for capturing print() / println() output.
func SetPrintHook(hook PrintHook) {
	printHookMu.Lock()
	defer printHookMu.Unlock()
	activePrintHook = hook
}

// ResetPrintHook resets print output interception back to standard os.Stdout.
func ResetPrintHook() {
	printHookMu.Lock()
	defer printHookMu.Unlock()
	activePrintHook = nil
}

// SetMethodEvaluator registers an evaluation callback for higher-order method execution.
func SetMethodEvaluator(ev MethodEvaluator) {
	evaluatorMu.Lock()
	defer evaluatorMu.Unlock()
	activeEvaluator = ev
}

// HostCallable is implemented by function values that run on another engine
// (for example a VM closure) but can still be invoked directly from host code.
type HostCallable interface {
	Call(args []Value) (Value, error)
}

// MethodProvider is implemented by host types that want to support method
// calls (obj.method(args...)) from Alca code without a circular dependency
// between the interpreter and the defining package.
type MethodProvider interface {
	CallMethod(name string, args []Value) (Value, error)
}

// IndexProvider is implemented by host types that want to support indexing
// (obj[index]) from Alca code without a circular dependency.
type IndexProvider interface {
	Index(index Value) Value
}

// Invoke calls a user-defined function value from host code (for example a GUI
// button callback). It reports an error when no interpreter is attached or the
// value is not callable.
func Invoke(fn Value, args []Value) (Value, error) {
	if fn == nil {
		return nil, errors.New("callback is nil")
	}
	if b, ok := fn.(*BuiltinFunction); ok {
		return b.Fn(args...), nil
	}
	if callable, ok := fn.(HostCallable); ok {
		return callable.Call(args)
	}
	evaluatorMu.RLock()
	ev := activeEvaluator
	evaluatorMu.RUnlock()
	if ev == nil {
		return nil, errors.New("no interpreter is attached to this process")
	}
	return ev(fn, args)
}

// BuiltinFunctionMap contains all standard global built-in functions in Alca.
var BuiltinFunctionMap = map[string]*BuiltinFunction{
	"print": {
		Name: "print",
		Fn: func(args ...Value) Value {
			parts := make([]string, len(args))
			for i, arg := range args {
				parts[i] = CoerceToString(arg)
			}
			out := strings.Join(parts, " ")
			printHookMu.RLock()
			hook := activePrintHook
			printHookMu.RUnlock()

			if hook != nil {
				hook(out)
			} else {
				fmt.Println(out)
			}
			return &StringValue{Val: out}
		},
	},
	"println": {
		Name: "println",
		Fn: func(args ...Value) Value {
			parts := make([]string, len(args))
			for i, arg := range args {
				parts[i] = CoerceToString(arg)
			}
			out := strings.Join(parts, " ")
			printHookMu.RLock()
			hook := activePrintHook
			printHookMu.RUnlock()

			if hook != nil {
				hook(out)
			} else {
				fmt.Println(out)
			}
			return &StringValue{Val: out}
		},
	},
	"len": {
		Name: "len",
		Fn: func(args ...Value) Value {
			if len(args) == 0 || args[0] == nil {
				return &IntegerValue{Val: 0}
			}
			switch val := args[0].(type) {
			case *StringValue:
				return &IntegerValue{Val: int64(len(val.RuneSlice()))}
			case *ListValue:
				return &IntegerValue{Val: int64(val.Len())}
			case *MapValue:
				return &IntegerValue{Val: int64(len(val.Pairs))}
			case *BlobValue:
				return &IntegerValue{Val: int64(len(val.Data))}
			default:
				return &IntegerValue{Val: 0}
			}
		},
	},
	"type": {
		Name: "type",
		Fn: func(args ...Value) Value {
			if len(args) == 0 || args[0] == nil {
				return &StringValue{Val: string(NULL_VAL)}
			}
			return &StringValue{Val: string(args[0].Type())}
		},
	},
	"typeof": {
		Name: "typeof",
		Fn: func(args ...Value) Value {
			if len(args) == 0 || args[0] == nil {
				return &StringValue{Val: "null"}
			}
			return &StringValue{Val: TypeNameOf(args[0])}
		},
	},
	"int": {
		Name: "int",
		Fn: func(args ...Value) Value {
			if len(args) == 0 || args[0] == nil {
				return &IntegerValue{Val: 0}
			}
			switch val := args[0].(type) {
			case *IntegerValue:
				return val
			case *FloatValue:
				return &IntegerValue{Val: int64(val.Val)}
			case *BoolValue:
				if val.Val {
					return &IntegerValue{Val: 1}
				}
				return &IntegerValue{Val: 0}
			case *StringValue:
				clean := strings.TrimSpace(val.Val)
				if i, err := strconv.ParseInt(clean, 10, 64); err == nil {
					return &IntegerValue{Val: i}
				}
				if f, err := strconv.ParseFloat(clean, 64); err == nil {
					return &IntegerValue{Val: int64(f)}
				}
				return NewError(400, fmt.Sprintf("cannot convert string %q to int", val.Val))
			default:
				return NewError(400, fmt.Sprintf("cannot convert %s to int", args[0].Type()))
			}
		},
	},
	"float": {
		Name: "float",
		Fn: func(args ...Value) Value {
			if len(args) == 0 || args[0] == nil {
				return &FloatValue{Val: 0.0}
			}
			switch val := args[0].(type) {
			case *FloatValue:
				return val
			case *IntegerValue:
				return &FloatValue{Val: float64(val.Val)}
			case *BoolValue:
				if val.Val {
					return &FloatValue{Val: 1.0}
				}
				return &FloatValue{Val: 0.0}
			case *StringValue:
				if f, err := strconv.ParseFloat(strings.TrimSpace(val.Val), 64); err == nil {
					return &FloatValue{Val: f}
				}
				return NewError(400, fmt.Sprintf("cannot convert string %q to float", val.Val))
			default:
				return NewError(400, fmt.Sprintf("cannot convert %s to float", args[0].Type()))
			}
		},
	},
	"string": {
		Name: "string",
		Fn: func(args ...Value) Value {
			if len(args) == 0 || args[0] == nil {
				return &StringValue{Val: "null"}
			}
			return &StringValue{Val: CoerceToString(args[0])}
		},
	},
	"blob": {
		Name: "blob",
		Fn: func(args ...Value) Value {
			if len(args) == 0 || args[0] == nil {
				return &BlobValue{Data: []byte{}}
			}
			switch val := args[0].(type) {
			case *BlobValue:
				return val
			case *StringValue:
				return &BlobValue{Data: []byte(val.Val)}
			case *ListValue:
				elems := val.ElementsSlice()
				bytesData := make([]byte, len(elems))
				for idx, el := range elems {
					if iv, ok := el.(*IntegerValue); ok {
						bytesData[idx] = byte(iv.Val)
					}
				}
				return &BlobValue{Data: bytesData}
			default:
				return NewError(400, fmt.Sprintf("cannot convert %s to blob", args[0].Type()))
			}
		},
	},
	"bool": {
		Name: "bool",
		Fn: func(args ...Value) Value {
			if len(args) == 0 || args[0] == nil {
				return FALSE
			}
			if IsTruthy(args[0]) {
				return TRUE
			}
			return FALSE
		},
	},
	"list": {
		Name: "list",
		Fn: func(args ...Value) Value {
			if len(args) == 0 || args[0] == nil {
				return NewList()
			}
			switch val := args[0].(type) {
			case *ListValue:
				return val.Clone()
			case *StringValue:
				runes := []rune(val.Val)
				elems := make([]Value, len(runes))
				for i, r := range runes {
					elems[i] = &StringValue{Val: string(r)}
				}
				return NewList(elems...)
			case *MapValue:
				keys := val.Keys()
				elems := make([]Value, len(keys))
				for i, k := range keys {
					elems[i] = &StringValue{Val: k}
				}
				return NewList(elems...)
			default:
				return NewList(val)
			}
		},
	},
	"append": {
		Name: "append",
		Fn: func(args ...Value) Value {
			if len(args) < 2 {
				return NewError(400, "append requires list and element arguments")
			}
			listVal, ok := args[0].(*ListValue)
			if !ok {
				return NewError(400, "first argument to append must be a list")
			}
			newList := listVal.Clone().(*ListValue)
			newList.Append(args[1])
			return newList
		},
	},
	"push": {
		Name: "push",
		Fn: func(args ...Value) Value {
			if len(args) < 2 {
				return NewError(400, "push requires list and element arguments")
			}
			listVal, ok := args[0].(*ListValue)
			if !ok {
				return NewError(400, "first argument to push must be a list")
			}
			listVal.Append(args[1])
			return listVal
		},
	},
	"pop": {
		Name: "pop",
		Fn: func(args ...Value) Value {
			if len(args) == 0 {
				return NewError(400, "pop requires a list argument")
			}
			listVal, ok := args[0].(*ListValue)
			if !ok {
				return NewError(400, "argument to pop must be a list")
			}
			listVal.Materialize()
			if len(listVal.Elements) == 0 {
				return NULL
			}
			last := listVal.Elements[len(listVal.Elements)-1]
			listVal.Elements = listVal.Elements[:len(listVal.Elements)-1]
			return last
		},
	},
	"keys": {
		Name: "keys",
		Fn: func(args ...Value) Value {
			if len(args) == 0 {
				return NewError(400, "keys requires a map argument")
			}
			mapVal, ok := args[0].(*MapValue)
			if !ok {
				return NewError(400, "argument to keys must be a map")
			}
			keys := mapVal.Keys()
			elems := make([]Value, len(keys))
			for i, k := range keys {
				elems[i] = &StringValue{Val: k}
			}
			return NewList(elems...)
		},
	},
	"values": {
		Name: "values",
		Fn: func(args ...Value) Value {
			if len(args) == 0 {
				return NewError(400, "values requires a map argument")
			}
			mapVal, ok := args[0].(*MapValue)
			if !ok {
				return NewError(400, "argument to values must be a map")
			}
			vals := mapVal.Values()
			return NewList(vals...)
		},
	},
	"range": {
		Name: "range",
		Fn: func(args ...Value) Value {
			if len(args) < 2 {
				return NewError(400, "range requires start and end arguments")
			}
			startVal, ok1 := args[0].(*IntegerValue)
			endVal, ok2 := args[1].(*IntegerValue)
			if !ok1 || !ok2 {
				return NewError(400, "range start and end must be integers")
			}
			step := int64(1)
			if len(args) >= 3 {
				if stepVal, ok3 := args[2].(*IntegerValue); ok3 && stepVal.Val != 0 {
					step = stepVal.Val
				}
			}
			elems := make([]Value, 0)
			if step > 0 {
				for i := startVal.Val; i <= endVal.Val; i += step {
					elems = append(elems, &IntegerValue{Val: i})
				}
			} else {
				for i := startVal.Val; i >= endVal.Val; i += step {
					elems = append(elems, &IntegerValue{Val: i})
				}
			}
			return NewList(elems...)
		},
	},
	"sleep": {
		Name: "sleep",
		Fn: func(args ...Value) Value {
			if len(args) == 0 {
				return NULL
			}
			ms := int64(0)
			if iv, ok := args[0].(*IntegerValue); ok {
				ms = iv.Val
			} else if fv, ok := args[0].(*FloatValue); ok {
				ms = int64(fv.Val)
			}
			if ms > 0 {
				time.Sleep(time.Duration(ms) * time.Millisecond)
			}
			return NULL
		},
	},
	"exit": {
		Name: "exit",
		Fn: func(args ...Value) Value {
			code := 0
			if len(args) > 0 {
				if iv, ok := args[0].(*IntegerValue); ok {
					code = int(iv.Val)
				}
			}
			os.Exit(code)
			return NULL
		},
	},
	"input": {
		Name: "input",
		Fn: func(args ...Value) Value {
			if len(args) > 0 && args[0] != nil {
				fmt.Print(CoerceToString(args[0]))
			}
			if line, ok := ReadStdinLine(); ok {
				return &StringValue{Val: line}
			}
			return &StringValue{Val: ""}
		},
	},
	"str_split": {
		Name: "str_split",
		Fn: func(args ...Value) Value {
			if len(args) < 2 {
				return NewError(400, "str_split requires string and separator arguments")
			}
			s := CoerceToString(args[0])
			sep := CoerceToString(args[1])
			parts := strings.Split(s, sep)
			elems := make([]Value, len(parts))
			for i, p := range parts {
				elems[i] = &StringValue{Val: p}
			}
			return NewList(elems...)
		},
	},
	"str_trim": {
		Name: "str_trim",
		Fn: func(args ...Value) Value {
			if len(args) == 0 {
				return &StringValue{Val: ""}
			}
			return &StringValue{Val: strings.TrimSpace(CoerceToString(args[0]))}
		},
	},
	"str_upper": {
		Name: "str_upper",
		Fn: func(args ...Value) Value {
			if len(args) == 0 {
				return &StringValue{Val: ""}
			}
			return &StringValue{Val: strings.ToUpper(CoerceToString(args[0]))}
		},
	},
	"str_lower": {
		Name: "str_lower",
		Fn: func(args ...Value) Value {
			if len(args) == 0 {
				return &StringValue{Val: ""}
			}
			return &StringValue{Val: strings.ToLower(CoerceToString(args[0]))}
		},
	},
	"str_contains": {
		Name: "str_contains",
		Fn: func(args ...Value) Value {
			if len(args) < 2 {
				return FALSE
			}
			s := CoerceToString(args[0])
			sub := CoerceToString(args[1])
			if strings.Contains(s, sub) {
				return TRUE
			}
			return FALSE
		},
	},
	"str_replace": {
		Name: "str_replace",
		Fn: func(args ...Value) Value {
			if len(args) < 3 {
				return NewError(400, "str_replace requires source, old, and new arguments")
			}
			s := CoerceToString(args[0])
			oldStr := CoerceToString(args[1])
			newStr := CoerceToString(args[2])
			return &StringValue{Val: strings.ReplaceAll(s, oldStr, newStr)}
		},
	},
	"str_starts_with": {
		Name: "str_starts_with",
		Fn: func(args ...Value) Value {
			if len(args) < 2 {
				return FALSE
			}
			s := CoerceToString(args[0])
			prefix := CoerceToString(args[1])
			if strings.HasPrefix(s, prefix) {
				return TRUE
			}
			return FALSE
		},
	},
	"str_ends_with": {
		Name: "str_ends_with",
		Fn: func(args ...Value) Value {
			if len(args) < 2 {
				return FALSE
			}
			s := CoerceToString(args[0])
			suffix := CoerceToString(args[1])
			if strings.HasSuffix(s, suffix) {
				return TRUE
			}
			return FALSE
		},
	},
	"abs": {
		Name: "abs",
		Fn: func(args ...Value) Value {
			if len(args) == 0 {
				return &IntegerValue{Val: 0}
			}
			switch val := args[0].(type) {
			case *IntegerValue:
				if val.Val < 0 {
					return &IntegerValue{Val: -val.Val}
				}
				return val
			case *FloatValue:
				return &FloatValue{Val: math.Abs(val.Val)}
			default:
				return NewError(400, "abs expects integer or float")
			}
		},
	},
	"max": {
		Name: "max",
		Fn: func(args ...Value) Value {
			if len(args) < 2 {
				return NewError(400, "max requires at least 2 arguments")
			}
			f1 := getNumericFloat(args[0])
			f2 := getNumericFloat(args[1])
			if f1 >= f2 {
				return args[0]
			}
			return args[1]
		},
	},
	"min": {
		Name: "min",
		Fn: func(args ...Value) Value {
			if len(args) < 2 {
				return NewError(400, "min requires at least 2 arguments")
			}
			f1 := getNumericFloat(args[0])
			f2 := getNumericFloat(args[1])
			if f1 <= f2 {
				return args[0]
			}
			return args[1]
		},
	},
	"env": {
		Name: "env",
		Fn: func(args ...Value) Value {
			if len(args) == 0 {
				return &StringValue{Val: ""}
			}
			name := CoerceToString(args[0])
			return &StringValue{Val: os.Getenv(name)}
		},
	},
	"set_env": {
		Name: "set_env",
		Fn: func(args ...Value) Value {
			if len(args) < 2 {
				return NewError(400, "set_env requires name and value arguments")
			}
			name := CoerceToString(args[0])
			val := CoerceToString(args[1])
			if err := os.Setenv(name, val); err != nil {
				return NewError(500, err.Error())
			}
			return TRUE
		},
	},
	"exec": {
		Name: "exec",
		Fn: func(args ...Value) Value {
			if len(args) == 0 {
				return &StringValue{Val: ""}
			}
			cmdStr := CoerceToString(args[0])
			out, err := RunShellCommand(cmdStr)
			if err != nil {
				return NewError(500, fmt.Sprintf("exec error: %s (%s)", err.Error(), out))
			}
			return &StringValue{Val: out}
		},
	},
	"time_now": {
		Name: "time_now",
		Fn: func(args ...Value) Value {
			return &StringValue{Val: time.Now().UTC().Format(time.RFC3339)}
		},
	},
	"json_encode": {
		Name: "json_encode",
		Fn: func(args ...Value) Value {
			if len(args) == 0 {
				return &StringValue{Val: "null"}
			}
			native := ValueToNative(args[0])
			bytes, err := json.Marshal(native)
			if err != nil {
				return NewError(500, fmt.Sprintf("json_encode error: %s", err.Error()))
			}
			return &StringValue{Val: string(bytes)}
		},
	},
	"json_decode": {
		Name: "json_decode",
		Fn: func(args ...Value) Value {
			if len(args) == 0 {
				return NULL
			}
			raw := CoerceToString(args[0])
			var parsed any
			if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
				return NewError(400, fmt.Sprintf("json_decode error: %s", err.Error()))
			}
			return NativeToValue(parsed)
		},
	},
}

// RunShellCommand executes a shell command string using cmd /c on Windows or sh -c on Unix.
func RunShellCommand(cmdStr string) (string, error) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", cmdStr)
	} else {
		cmd = exec.Command("sh", "-c", cmdStr)
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// RunShellCommandStreaming runs a shell command and copies its output to w while
// the command is still producing it, so the terminal shows the output as it
// arrives. It returns the complete output and the exit code of the command.
func RunShellCommandStreaming(cmdStr string, w io.Writer) (string, int) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", cmdStr)
	} else {
		cmd = exec.Command("sh", "-c", cmdStr)
	}

	var buf bytes.Buffer
	if w != nil {
		cmd.Stdout = io.MultiWriter(w, &buf)
		cmd.Stderr = io.MultiWriter(w, &buf)
	} else {
		cmd.Stdout = &buf
		cmd.Stderr = &buf
	}

	err := cmd.Run()
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
		} else {
			code = 1
		}
	}
	return buf.String(), code
}

func getNumericFloat(v Value) float64 {
	switch val := v.(type) {
	case *IntegerValue:
		return float64(val.Val)
	case *FloatValue:
		return val.Val
	default:
		return 0.0
	}
}

// ValueToNative converts a Alca runtime Value to generic Go native types suitable for JSON marshaling.
func ValueToNative(v Value) any {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case *NullValue:
		return nil
	case *BoolValue:
		return val.Val
	case *IntegerValue:
		return val.Val
	case *FloatValue:
		return val.Val
	case *StringValue:
		return val.Val
	case *SecretValue:
		return val.Val
	case *BlobValue:
		return val.Data
	case *ErrorValue:
		return map[string]any{
			"code":    val.Code,
			"message": val.Message,
			"time":    val.Time,
		}
	case *ListValue:
		valElems := val.ElementsSlice()
		res := make([]any, len(valElems))
		for i, el := range valElems {
			res[i] = ValueToNative(el)
		}
		return res
	case *MapValue:
		res := make(map[string]any, len(val.Pairs))
		for k, v := range val.Pairs {
			res[k] = ValueToNative(v)
		}
		return res
	case *StructInstance:
		res := make(map[string]any, len(val.Fields))
		for k, v := range val.Fields {
			res[k] = ValueToNative(v)
		}
		return res
	default:
		return val.Inspect()
	}
}

// NativeToValue recursively converts native Go parsed JSON data into Alca runtime Values.
func NativeToValue(native any) Value {
	if native == nil {
		return NULL
	}
	switch v := native.(type) {
	case bool:
		if v {
			return TRUE
		}
		return FALSE
	case float64:
		if v == math.Trunc(v) && !math.IsNaN(v) && !math.IsInf(v, 0) {
			return &IntegerValue{Val: int64(v)}
		}
		return &FloatValue{Val: v}
	case string:
		return &StringValue{Val: v}
	case []any:
		elems := make([]Value, len(v))
		for i, item := range v {
			elems[i] = NativeToValue(item)
		}
		return NewList(elems...)
	case map[string]any:
		m := NewMap()
		// Sort map keys for deterministic order
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			m.Set(k, NativeToValue(v[k]))
		}
		return m
	default:
		return &StringValue{Val: fmt.Sprintf("%v", v)}
	}
}

// CallMethod dispatches method invocation on a target object using dot notation (obj.method(args...)).
func CallMethod(obj Value, method string, args []Value) (Value, error) {
	if obj == nil {
		return nil, fmt.Errorf("cannot call method %q on null", method)
	}

	switch target := obj.(type) {
	case *StringValue:
		return callStringMethod(target, method, args)
	case *ListValue:
		return callListMethod(target, method, args)
	case *MapValue:
		return callMapMethod(target, method, args)
	case *FuncValue:
		if method == "stream" {
			return nil, fmt.Errorf("fn.stream() is not supported by the VM backend; run the program with the interpreter")
		}
		return nil, fmt.Errorf("value of type %s has no methods", obj.Type())
	case MethodProvider:
		return target.CallMethod(method, args)
	default:
		return nil, fmt.Errorf("value of type %s has no methods", obj.Type())
	}
}

func callStringMethod(s *StringValue, method string, args []Value) (Value, error) {
	switch method {
	case "split":
		sep := " "
		if len(args) > 0 {
			sep = CoerceToString(args[0])
		}
		parts := strings.Split(s.Val, sep)
		elems := make([]Value, len(parts))
		for i, p := range parts {
			elems[i] = &StringValue{Val: p}
		}
		return NewList(elems...), nil
	case "trim", "strip":
		return &StringValue{Val: strings.TrimSpace(s.Val)}, nil
	case "lstrip":
		return &StringValue{Val: strings.TrimLeft(s.Val, " \t\r\n")}, nil
	case "rstrip":
		return &StringValue{Val: strings.TrimRight(s.Val, " \t\r\n")}, nil
	case "upper":
		return &StringValue{Val: strings.ToUpper(s.Val)}, nil
	case "lower":
		return &StringValue{Val: strings.ToLower(s.Val)}, nil
	case "contains":
		if len(args) == 0 {
			return FALSE, nil
		}
		sub := CoerceToString(args[0])
		if strings.Contains(s.Val, sub) {
			return TRUE, nil
		}
		return FALSE, nil
	case "replace":
		if len(args) < 2 {
			return nil, fmt.Errorf(".replace(old, new) requires 2 arguments")
		}
		oldStr := CoerceToString(args[0])
		newStr := CoerceToString(args[1])
		return &StringValue{Val: strings.ReplaceAll(s.Val, oldStr, newStr)}, nil
	case "starts_with":
		if len(args) == 0 {
			return FALSE, nil
		}
		prefix := CoerceToString(args[0])
		if strings.HasPrefix(s.Val, prefix) {
			return TRUE, nil
		}
		return FALSE, nil
	case "ends_with":
		if len(args) == 0 {
			return FALSE, nil
		}
		suffix := CoerceToString(args[0])
		if strings.HasSuffix(s.Val, suffix) {
			return TRUE, nil
		}
		return FALSE, nil
	case "len":
		return &IntegerValue{Val: int64(len(s.RuneSlice()))}, nil
	case "last_index_of":
		if len(args) == 0 {
			return &IntegerValue{Val: -1}, nil
		}
		sub := CoerceToString(args[0])
		idx := strings.LastIndex(s.Val, sub)
		if idx < 0 {
			return &IntegerValue{Val: -1}, nil
		}
		return &IntegerValue{Val: int64(utf8.RuneCountInString(s.Val[:idx]))}, nil
	case "count":
		if len(args) == 0 {
			return &IntegerValue{Val: 0}, nil
		}
		sub := CoerceToString(args[0])
		return &IntegerValue{Val: int64(strings.Count(s.Val, sub))}, nil
	case "repeat":
		if len(args) == 0 {
			return s, nil
		}
		count := int64(0)
		if iv, ok := args[0].(*IntegerValue); ok && iv.Val > 0 {
			count = iv.Val
		}
		return &StringValue{Val: strings.Repeat(s.Val, int(count))}, nil
	case "pad_left":
		if len(args) == 0 {
			return s, nil
		}
		target := int64(0)
		if iv, ok := args[0].(*IntegerValue); ok {
			target = iv.Val
		}
		pad := " "
		if len(args) >= 2 {
			p := CoerceToString(args[1])
			if p != "" {
				pad = p
			}
		}
		cur := int64(utf8.RuneCountInString(s.Val))
		if cur >= target {
			return s, nil
		}
		needed := target - cur
		var bld strings.Builder
		for int64(utf8.RuneCountInString(bld.String())) < needed {
			bld.WriteString(pad)
		}
		padRunes := []rune(bld.String())
		padStr := bld.String()
		if int64(len(padRunes)) > needed {
			padStr = string(padRunes[:needed])
		}
		return &StringValue{Val: padStr + s.Val}, nil
	case "pad_right":
		if len(args) == 0 {
			return s, nil
		}
		target := int64(0)
		if iv, ok := args[0].(*IntegerValue); ok {
			target = iv.Val
		}
		pad := " "
		if len(args) >= 2 {
			p := CoerceToString(args[1])
			if p != "" {
				pad = p
			}
		}
		cur := int64(utf8.RuneCountInString(s.Val))
		if cur >= target {
			return s, nil
		}
		needed := target - cur
		var bld strings.Builder
		for int64(utf8.RuneCountInString(bld.String())) < needed {
			bld.WriteString(pad)
		}
		padRunes := []rune(bld.String())
		padStr := bld.String()
		if int64(len(padRunes)) > needed {
			padStr = string(padRunes[:needed])
		}
		return &StringValue{Val: s.Val + padStr}, nil
	case "pad":
		return callStringMethod(s, "pad_right", args)
	case "format":
		template := s.Val
		if len(args) == 0 {
			return s, nil
		}
		if strings.Contains(template, "{") && strings.Contains(template, "}") {
			res := template
			for i, a := range args {
				placeholder := fmt.Sprintf("{%d}", i)
				res = strings.ReplaceAll(res, placeholder, CoerceToString(a))
			}
			for _, a := range args {
				if !strings.Contains(res, "{}") {
					break
				}
				res = strings.Replace(res, "{}", CoerceToString(a), 1)
			}
			return &StringValue{Val: res}, nil
		}
		if strings.Contains(template, "%") {
			rawArgs := make([]any, len(args))
			for i, a := range args {
				switch v := a.(type) {
				case *IntegerValue:
					rawArgs[i] = v.Val
				case *FloatValue:
					rawArgs[i] = v.Val
				case *BoolValue:
					rawArgs[i] = v.Val
				case *StringValue:
					rawArgs[i] = v.Val
				default:
					rawArgs[i] = a.Inspect()
				}
			}
			return &StringValue{Val: fmt.Sprintf(template, rawArgs...)}, nil
		}
		return s, nil
	case "center":
		if len(args) == 0 {
			return s, nil
		}
		target := int64(0)
		if iv, ok := args[0].(*IntegerValue); ok {
			target = iv.Val
		}
		pad := " "
		if len(args) >= 2 {
			p := CoerceToString(args[1])
			if p != "" {
				pad = p
			}
		}
		cur := int64(utf8.RuneCountInString(s.Val))
		if cur >= target {
			return s, nil
		}
		total := target - cur
		left := total / 2
		right := total - left
		makeP := func(n int64) string {
			var b strings.Builder
			for int64(utf8.RuneCountInString(b.String())) < n {
				b.WriteString(pad)
			}
			r := []rune(b.String())
			if int64(len(r)) > n {
				return string(r[:n])
			}
			return b.String()
		}
		return &StringValue{Val: makeP(left) + s.Val + makeP(right)}, nil
	case "capitalize":
		runes := []rune(s.Val)
		if len(runes) == 0 {
			return s, nil
		}
		runes[0] = unicode.ToUpper(runes[0])
		for i := 1; i < len(runes); i++ {
			runes[i] = unicode.ToLower(runes[i])
		}
		return &StringValue{Val: string(runes)}, nil
	case "title":
		runes := []rune(s.Val)
		inWord := false
		for i, r := range runes {
			if unicode.IsSpace(r) || unicode.IsPunct(r) {
				inWord = false
			} else if !inWord {
				runes[i] = unicode.ToUpper(r)
				inWord = true
			} else {
				runes[i] = unicode.ToLower(r)
			}
		}
		return &StringValue{Val: string(runes)}, nil
	case "is_empty":
		return &BoolValue{Val: len(s.Val) == 0}, nil
	case "is_blank":
		return &BoolValue{Val: strings.TrimSpace(s.Val) == ""}, nil
	case "is_alpha":
		if len(s.Val) == 0 {
			return FALSE, nil
		}
		for _, r := range s.Val {
			if !unicode.IsLetter(r) {
				return FALSE, nil
			}
		}
		return TRUE, nil
	case "is_digit", "is_numeric":
		if len(s.Val) == 0 {
			return FALSE, nil
		}
		for _, r := range s.Val {
			if !unicode.IsDigit(r) {
				return FALSE, nil
			}
		}
		return TRUE, nil
	case "is_alphanumeric":
		if len(s.Val) == 0 {
			return FALSE, nil
		}
		for _, r := range s.Val {
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
				return FALSE, nil
			}
		}
		return TRUE, nil
	case "is_upper":
		hasCased := false
		for _, r := range s.Val {
			if unicode.IsUpper(r) {
				hasCased = true
			} else if unicode.IsLower(r) {
				return FALSE, nil
			}
		}
		return &BoolValue{Val: hasCased}, nil
	case "is_lower":
		hasCased := false
		for _, r := range s.Val {
			if unicode.IsLower(r) {
				hasCased = true
			} else if unicode.IsUpper(r) {
				return FALSE, nil
			}
		}
		return &BoolValue{Val: hasCased}, nil
	case "trim_prefix":
		if len(args) == 0 {
			return s, nil
		}
		return &StringValue{Val: strings.TrimPrefix(s.Val, CoerceToString(args[0]))}, nil
	case "trim_suffix":
		if len(args) == 0 {
			return s, nil
		}
		return &StringValue{Val: strings.TrimSuffix(s.Val, CoerceToString(args[0]))}, nil
	case "char_at":
		if len(args) == 0 {
			return &StringValue{Val: ""}, nil
		}
		idx := int64(0)
		if iv, ok := args[0].(*IntegerValue); ok {
			idx = iv.Val
		}
		runes := []rune(s.Val)
		n := int64(len(runes))
		if idx < 0 {
			idx += n
		}
		if idx < 0 || idx >= n {
			return &StringValue{Val: ""}, nil
		}
		return &StringValue{Val: string(runes[idx])}, nil
	case "lines":
		normalized := strings.ReplaceAll(s.Val, "\r\n", "\n")
		normalized = strings.ReplaceAll(normalized, "\r", "\n")
		parts := strings.Split(normalized, "\n")
		values := make([]Value, len(parts))
		for i, p := range parts {
			values[i] = &StringValue{Val: p}
		}
		return NewList(values...), nil
	case "words":
		parts := strings.Fields(s.Val)
		values := make([]Value, len(parts))
		for i, p := range parts {
			values[i] = &StringValue{Val: p}
		}
		return NewList(values...), nil
	case "reverse":
		runes := []rune(s.Val)
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		return &StringValue{Val: string(runes)}, nil
	default:
		return nil, fmt.Errorf("string has no method %q", method)
	}
}

func callListMethod(l *ListValue, method string, args []Value) (Value, error) {
	switch method {
	case "push", "append":
		if len(args) == 0 {
			return nil, fmt.Errorf(".%s() requires an element", method)
		}
		l.Append(args[0])
		return l, nil
	case "pop":
		l.Materialize()
		if len(l.Elements) == 0 {
			return NULL, nil
		}
		last := l.Elements[len(l.Elements)-1]
		l.Elements = l.Elements[:len(l.Elements)-1]
		limits.Refund(limits.CostValueSlot)
		return last, nil
	case "len":
		return &IntegerValue{Val: int64(l.Len())}, nil
	case "contains":
		if len(args) == 0 {
			return FALSE, nil
		}
		for _, el := range l.ElementsSlice() {
			if ValuesEqual(el, args[0]) {
				return TRUE, nil
			}
		}
		return FALSE, nil
	case "reverse":
		l.Materialize()
		n := len(l.Elements)
		for i := 0; i < n/2; i++ {
			l.Elements[i], l.Elements[n-1-i] = l.Elements[n-1-i], l.Elements[i]
		}
		return l, nil
	case "sort":
		l.Materialize()
		sort.SliceStable(l.Elements, func(i, j int) bool {
			a, b := l.Elements[i], l.Elements[j]
			if a.Type() == INTEGER_VAL && b.Type() == INTEGER_VAL {
				return a.(*IntegerValue).Val < b.(*IntegerValue).Val
			}
			if a.Type() == FLOAT_VAL && b.Type() == FLOAT_VAL {
				return a.(*FloatValue).Val < b.(*FloatValue).Val
			}
			return CoerceToString(a) < CoerceToString(b)
		})
		return l, nil
	case "join":
		sep := ""
		if len(args) > 0 {
			sep = CoerceToString(args[0])
		}
		elems := l.ElementsSlice()
		parts := make([]string, len(elems))
		for i, el := range elems {
			parts[i] = CoerceToString(el)
		}
		return &StringValue{Val: strings.Join(parts, sep)}, nil
	case "map":
		if len(args) == 0 {
			return nil, fmt.Errorf(".map() requires a function argument")
		}
		fn := args[0]
		elems := l.ElementsSlice()
		res := make([]Value, len(elems))
		for i, el := range elems {
			ret, err := invokeCallable(fn, []Value{el})
			if err != nil {
				return nil, err
			}
			res[i] = ret
		}
		return NewList(res...), nil
	case "filter":
		if len(args) == 0 {
			return nil, fmt.Errorf(".filter() requires a function argument")
		}
		fn := args[0]
		res := make([]Value, 0)
		for _, el := range l.ElementsSlice() {
			ret, err := invokeCallable(fn, []Value{el})
			if err != nil {
				return nil, err
			}
			if IsTruthy(ret) {
				res = append(res, el)
			}
		}
		return NewList(res...), nil
	case "reduce":
		if len(args) == 0 {
			return nil, fmt.Errorf(".reduce() requires a function argument")
		}
		fn := args[0]
		elems := l.ElementsSlice()
		var acc Value
		start := 0
		if len(args) >= 2 {
			acc = args[1]
		} else {
			if len(elems) == 0 {
				return nil, fmt.Errorf(".reduce() on empty list with no initial value")
			}
			acc = elems[0]
			start = 1
		}
		for idx := start; idx < len(elems); idx++ {
			ret, err := invokeCallable(fn, []Value{acc, elems[idx], &IntegerValue{Val: int64(idx)}})
			if err != nil {
				ret, err = invokeCallable(fn, []Value{acc, elems[idx]})
			}
			if err != nil {
				return nil, err
			}
			acc = ret
		}
		return acc, nil
	case "slice":
		if len(args) == 0 {
			return nil, fmt.Errorf(".slice() requires start index")
		}
		startVal, ok := args[0].(*IntegerValue)
		if !ok {
			return nil, fmt.Errorf(".slice() start index must be integer")
		}
		elems := l.ElementsSlice()
		n := int64(len(elems))
		start := startVal.Val
		if start < 0 {
			start += n
			if start < 0 {
				start = 0
			}
		}
		end := n
		if len(args) >= 2 {
			if endVal, ok := args[1].(*IntegerValue); ok {
				end = endVal.Val
				if end < 0 {
					end += n
				}
				if end > n {
					end = n
				}
			}
		}
		if start > end {
			start = end
		}
		if start > n {
			start = n
		}
		return NewList(elems[start:end]...), nil
	case "find":
		if len(args) == 0 {
			return nil, fmt.Errorf(".find() requires a predicate function")
		}
		fn := args[0]
		for idx, el := range l.ElementsSlice() {
			ret, err := invokeCallable(fn, []Value{el, &IntegerValue{Val: int64(idx)}})
			if err != nil {
				ret, err = invokeCallable(fn, []Value{el})
			}
			if err != nil {
				return nil, err
			}
			if IsTruthy(ret) {
				return el, nil
			}
		}
		return NULL, nil
	case "find_index":
		if len(args) == 0 {
			return nil, fmt.Errorf(".find_index() requires a predicate function")
		}
		fn := args[0]
		for idx, el := range l.ElementsSlice() {
			ret, err := invokeCallable(fn, []Value{el, &IntegerValue{Val: int64(idx)}})
			if err != nil {
				ret, err = invokeCallable(fn, []Value{el})
			}
			if err != nil {
				return nil, err
			}
			if IsTruthy(ret) {
				return &IntegerValue{Val: int64(idx)}, nil
			}
		}
		return &IntegerValue{Val: -1}, nil
	case "index_of":
		if len(args) == 0 {
			return &IntegerValue{Val: -1}, nil
		}
		for idx, el := range l.ElementsSlice() {
			if ValuesEqual(el, args[0]) {
				return &IntegerValue{Val: int64(idx)}, nil
			}
		}
		return &IntegerValue{Val: -1}, nil
	case "any":
		elems := l.ElementsSlice()
		if len(args) > 0 && args[0] != nil && args[0].Type() != NULL_VAL {
			fn := args[0]
			for idx, el := range elems {
				ret, err := invokeCallable(fn, []Value{el, &IntegerValue{Val: int64(idx)}})
				if err != nil {
					ret, err = invokeCallable(fn, []Value{el})
				}
				if err != nil {
					return nil, err
				}
				if IsTruthy(ret) {
					return TRUE, nil
				}
			}
		} else {
			for _, el := range elems {
				if IsTruthy(el) {
					return TRUE, nil
				}
			}
		}
		return FALSE, nil
	case "all":
		elems := l.ElementsSlice()
		if len(args) > 0 && args[0] != nil && args[0].Type() != NULL_VAL {
			fn := args[0]
			for idx, el := range elems {
				ret, err := invokeCallable(fn, []Value{el, &IntegerValue{Val: int64(idx)}})
				if err != nil {
					ret, err = invokeCallable(fn, []Value{el})
				}
				if err != nil {
					return nil, err
				}
				if !IsTruthy(ret) {
					return FALSE, nil
				}
			}
		} else {
			for _, el := range elems {
				if !IsTruthy(el) {
					return FALSE, nil
				}
			}
		}
		return TRUE, nil
	case "flatten":
		res := make([]Value, 0)
		for _, el := range l.ElementsSlice() {
			if sub, ok := el.(*ListValue); ok {
				res = append(res, sub.ElementsSlice()...)
			} else {
				res = append(res, el)
			}
		}
		return NewList(res...), nil
	case "unique":
		res := make([]Value, 0)
		seen := make(map[string]bool)
		for _, el := range l.ElementsSlice() {
			k := el.Inspect()
			if !seen[k] {
				seen[k] = true
				res = append(res, el)
			}
		}
		return NewList(res...), nil
	case "concat":
		if len(args) == 0 {
			return l.Clone(), nil
		}
		other, ok := args[0].(*ListValue)
		if !ok {
			return nil, fmt.Errorf(".concat() requires a list argument")
		}
		e1 := l.ElementsSlice()
		e2 := other.ElementsSlice()
		res := make([]Value, len(e1)+len(e2))
		copy(res, e1)
		copy(res[len(e1):], e2)
		return NewList(res...), nil
	default:
		return nil, fmt.Errorf("list has no method %q", method)
	}
}

func callMapMethod(m *MapValue, method string, args []Value) (Value, error) {
	if fnVal, ok := m.Get(method); ok {
		switch fnVal.(type) {
		case *BuiltinFunction, *FuncValue, HostCallable:
			return invokeCallable(fnVal, args)
		}
	}
	switch method {
	case "keys":
		keys := m.Keys()
		elems := make([]Value, len(keys))
		for i, k := range keys {
			elems[i] = &StringValue{Val: k}
		}
		return NewList(elems...), nil
	case "values":
		vals := m.Values()
		return NewList(vals...), nil
	case "has":
		if len(args) == 0 {
			return FALSE, nil
		}
		key := CoerceToString(args[0])
		if m.Has(key) {
			return TRUE, nil
		}
		return FALSE, nil
	case "delete":
		if len(args) == 0 {
			return NULL, nil
		}
		key := CoerceToString(args[0])
		m.Delete(key)
		return NULL, nil
	case "len":
		return &IntegerValue{Val: int64(m.Len())}, nil
	case "get":
		if len(args) == 0 {
			return NULL, nil
		}
		key := CoerceToString(args[0])
		if v, ok := m.Get(key); ok {
			return v, nil
		}
		if len(args) >= 2 {
			return args[1], nil
		}
		return NULL, nil
	case "set":
		if len(args) < 2 {
			return nil, fmt.Errorf(".set() requires key and value arguments")
		}
		key := CoerceToString(args[0])
		m.Set(key, args[1])
		return m, nil
	case "reverse":
		keys := m.Keys()
		res := NewMap()
		for i := len(keys) - 1; i >= 0; i-- {
			k := keys[i]
			v, _ := m.Get(k)
			res.Set(k, v)
		}
		return res, nil
	case "sort":
		keys := m.Keys()
		if len(args) > 0 && args[0] != nil && args[0].Type() != NULL_VAL {
			fn := args[0]
			var sortErr error
			sort.SliceStable(keys, func(i, j int) bool {
				if sortErr != nil {
					return false
				}
				k1, k2 := keys[i], keys[j]
				v1, _ := m.Get(k1)
				v2, _ := m.Get(k2)
				res, err := invokeCallable(fn, []Value{
					&StringValue{Val: k1}, v1,
					&StringValue{Val: k2}, v2,
				})
				if err != nil {
					res, err = invokeCallable(fn, []Value{
						&StringValue{Val: k1}, &StringValue{Val: k2},
					})
				}
				if err != nil {
					sortErr = err
					return false
				}
				if bVal, ok := res.(*BoolValue); ok {
					return bVal.Val
				}
				if iVal, ok := res.(*IntegerValue); ok {
					return iVal.Val < 0
				}
				return IsTruthy(res)
			})
			if sortErr != nil {
				return nil, sortErr
			}
		} else {
			sort.Strings(keys)
		}
		res := NewMap()
		for _, k := range keys {
			v, _ := m.Get(k)
			res.Set(k, v)
		}
		return res, nil
	case "filter":
		if len(args) == 0 {
			return nil, fmt.Errorf(".filter() requires a predicate function")
		}
		fn := args[0]
		res := NewMap()
		for _, k := range m.Keys() {
			v, _ := m.Get(k)
			ret, err := invokeCallable(fn, []Value{&StringValue{Val: k}, v})
			if err != nil {
				ret, err = invokeCallable(fn, []Value{v})
			}
			if err != nil {
				return nil, err
			}
			if IsTruthy(ret) {
				res.Set(k, v)
			}
		}
		return res, nil
	case "map":
		if len(args) == 0 {
			return nil, fmt.Errorf(".map() requires a function argument")
		}
		fn := args[0]
		res := NewMap()
		for _, k := range m.Keys() {
			v, _ := m.Get(k)
			ret, err := invokeCallable(fn, []Value{&StringValue{Val: k}, v})
			if err != nil {
				ret, err = invokeCallable(fn, []Value{v})
			}
			if err != nil {
				return nil, err
			}
			if lVal, ok := ret.(*ListValue); ok && lVal.Len() == 2 {
				s := lVal.ElementsSlice()
				res.Set(CoerceToString(s[0]), s[1])
			} else {
				res.Set(k, ret)
			}
		}
		return res, nil
	case "reduce":
		if len(args) == 0 {
			return nil, fmt.Errorf(".reduce() requires a function argument")
		}
		fn := args[0]
		keys := m.Keys()
		var acc Value
		start := 0
		if len(args) >= 2 {
			acc = args[1]
		} else {
			if len(keys) == 0 {
				return nil, fmt.Errorf(".reduce() on empty map with no initial value")
			}
			firstV, _ := m.Get(keys[0])
			acc = firstV
			start = 1
		}
		for i := start; i < len(keys); i++ {
			k := keys[i]
			v, _ := m.Get(k)
			ret, err := invokeCallable(fn, []Value{acc, &StringValue{Val: k}, v})
			if err != nil {
				ret, err = invokeCallable(fn, []Value{acc, v})
			}
			if err != nil {
				return nil, err
			}
			acc = ret
		}
		return acc, nil
	case "merge":
		if len(args) == 0 {
			return m, nil
		}
		other, ok := args[0].(*MapValue)
		if !ok {
			return nil, fmt.Errorf(".merge() requires a map argument")
		}
		res := NewMap()
		for _, k := range m.Keys() {
			v, _ := m.Get(k)
			res.Set(k, v)
		}
		for _, k := range other.Keys() {
			v, _ := other.Get(k)
			res.Set(k, v)
		}
		return res, nil
	case "entries":
		keys := m.Keys()
		res := make([]Value, len(keys))
		for i, k := range keys {
			v, _ := m.Get(k)
			res[i] = NewList(&StringValue{Val: k}, v)
		}
		return NewList(res...), nil
	case "clone":
		return m.Clone(), nil
	default:
		if fnVal, ok := m.Get(method); ok {
			return invokeCallable(fnVal, args)
		}
		return nil, fmt.Errorf("map has no method %q", method)
	}
}

func invokeCallable(fn Value, args []Value) (Value, error) {
	if b, ok := fn.(*BuiltinFunction); ok {
		return b.Fn(args...), nil
	}
	evaluatorMu.RLock()
	ev := activeEvaluator
	evaluatorMu.RUnlock()
	if ev != nil {
		return ev(fn, args)
	}
	return nil, fmt.Errorf("cannot invoke non-function value %s", fn.Type())
}
