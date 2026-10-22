package stdlib

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/alca-lang/alca/pkg/interpreter"
)

// RegisterStringBuiltins registers Unicode-aware helpers in std.string.
func RegisterStringBuiltins(env *interpreter.Environment) {
	module := interpreter.NewMap()
	register := func(name string, fn func(...interpreter.Value) interpreter.Value) {
		module.Set(name, &interpreter.BuiltinFunction{Name: name, Fn: fn})
	}
	getString := func(args []interpreter.Value, index int, name string) (string, interpreter.Value) {
		if len(args) <= index {
			return "", interpreter.NewError(400, fmt.Sprintf("string.%s requires more arguments", name))
		}
		value, ok := args[index].(*interpreter.StringValue)
		if !ok {
			return "", interpreter.NewError(400, fmt.Sprintf("string.%s expects string arguments", name))
		}
		return value.Val, nil
	}
	register("length", func(args ...interpreter.Value) interpreter.Value {
		text, errVal := getString(args, 0, "length")
		if errVal != nil {
			return errVal
		}
		return &interpreter.IntegerValue{Val: int64(utf8.RuneCountInString(text))}
	})
	register("lower", func(args ...interpreter.Value) interpreter.Value {
		text, errVal := getString(args, 0, "lower")
		if errVal != nil {
			return errVal
		}
		return &interpreter.StringValue{Val: strings.ToLower(text)}
	})
	register("upper", func(args ...interpreter.Value) interpreter.Value {
		text, errVal := getString(args, 0, "upper")
		if errVal != nil {
			return errVal
		}
		return &interpreter.StringValue{Val: strings.ToUpper(text)}
	})
	register("trim", func(args ...interpreter.Value) interpreter.Value {
		text, errVal := getString(args, 0, "trim")
		if errVal != nil {
			return errVal
		}
		return &interpreter.StringValue{Val: strings.TrimSpace(text)}
	})
	register("trim_left", func(args ...interpreter.Value) interpreter.Value {
		text, errVal := getString(args, 0, "trim_left")
		if errVal != nil {
			return errVal
		}
		return &interpreter.StringValue{Val: strings.TrimLeftFunc(text, func(r rune) bool {
			return strings.ContainsRune(" \t\n\r\v\f", r)
		})}
	})
	register("trim_right", func(args ...interpreter.Value) interpreter.Value {
		text, errVal := getString(args, 0, "trim_right")
		if errVal != nil {
			return errVal
		}
		return &interpreter.StringValue{Val: strings.TrimRightFunc(text, func(r rune) bool {
			return strings.ContainsRune(" \t\n\r\v\f", r)
		})}
	})
	register("contains", func(args ...interpreter.Value) interpreter.Value {
		text, errVal := getString(args, 0, "contains")
		if errVal != nil {
			return errVal
		}
		part, errVal := getString(args, 1, "contains")
		if errVal != nil {
			return errVal
		}
		if strings.Contains(text, part) {
			return interpreter.TRUE
		}
		return interpreter.FALSE
	})
	register("index_of", func(args ...interpreter.Value) interpreter.Value {
		text, errVal := getString(args, 0, "index_of")
		if errVal != nil {
			return errVal
		}
		part, errVal := getString(args, 1, "index_of")
		if errVal != nil {
			return errVal
		}
		index := strings.Index(text, part)
		if index < 0 {
			return &interpreter.IntegerValue{Val: -1}
		}
		return &interpreter.IntegerValue{Val: int64(utf8.RuneCountInString(text[:index]))}
	})
	register("starts_with", func(args ...interpreter.Value) interpreter.Value {
		text, errVal := getString(args, 0, "starts_with")
		if errVal != nil {
			return errVal
		}
		prefix, errVal := getString(args, 1, "starts_with")
		if errVal != nil {
			return errVal
		}
		if strings.HasPrefix(text, prefix) {
			return interpreter.TRUE
		}
		return interpreter.FALSE
	})
	register("ends_with", func(args ...interpreter.Value) interpreter.Value {
		text, errVal := getString(args, 0, "ends_with")
		if errVal != nil {
			return errVal
		}
		suffix, errVal := getString(args, 1, "ends_with")
		if errVal != nil {
			return errVal
		}
		if strings.HasSuffix(text, suffix) {
			return interpreter.TRUE
		}
		return interpreter.FALSE
	})
	register("replace", func(args ...interpreter.Value) interpreter.Value {
		text, errVal := getString(args, 0, "replace")
		if errVal != nil {
			return errVal
		}
		old, errVal := getString(args, 1, "replace")
		if errVal != nil {
			return errVal
		}
		replacement, errVal := getString(args, 2, "replace")
		if errVal != nil {
			return errVal
		}
		return &interpreter.StringValue{Val: strings.ReplaceAll(text, old, replacement)}
	})
	register("split", func(args ...interpreter.Value) interpreter.Value {
		text, errVal := getString(args, 0, "split")
		if errVal != nil {
			return errVal
		}
		separator, errVal := getString(args, 1, "split")
		if errVal != nil {
			return errVal
		}
		parts := strings.Split(text, separator)
		values := make([]interpreter.Value, len(parts))
		for i, part := range parts {
			values[i] = &interpreter.StringValue{Val: part}
		}
		return interpreter.NewList(values...)
	})
	register("join", func(args ...interpreter.Value) interpreter.Value {
		if len(args) != 2 {
			return interpreter.NewError(400, "string.join requires (list[string], separator)")
		}
		values, ok := args[0].(*interpreter.ListValue)
		if !ok {
			return interpreter.NewError(400, "string.join first argument must be a list")
		}
		separator, ok := args[1].(*interpreter.StringValue)
		if !ok {
			return interpreter.NewError(400, "string.join separator must be a string")
		}
		parts := make([]string, len(values.ElementsSlice()))
		for i, value := range values.ElementsSlice() {
			text, ok := value.(*interpreter.StringValue)
			if !ok {
				return interpreter.NewError(400, "string.join list items must be strings")
			}
			parts[i] = text.Val
		}
		return &interpreter.StringValue{Val: strings.Join(parts, separator.Val)}
	})
	register("slice", func(args ...interpreter.Value) interpreter.Value {
		if len(args) != 3 {
			return interpreter.NewError(400, "string.slice requires (text, start, end)")
		}
		text, errVal := getString(args, 0, "slice")
		if errVal != nil {
			return errVal
		}
		start, startOK := args[1].(*interpreter.IntegerValue)
		end, endOK := args[2].(*interpreter.IntegerValue)
		if !startOK || !endOK {
			return interpreter.NewError(400, "string.slice start and end must be integers")
		}
		runes := []rune(text)
		length := int64(len(runes))
		from, to := start.Val, end.Val
		if from < 0 {
			from += length
		}
		if to < 0 {
			to += length
		}
		if from < 0 || to < from || to > length {
			return interpreter.NewError(400, "string.slice indices are out of range")
		}
		return &interpreter.StringValue{Val: string(runes[from:to])}
	})
	register("runes", func(args ...interpreter.Value) interpreter.Value {
		text, errVal := getString(args, 0, "runes")
		if errVal != nil {
			return errVal
		}
		runes := []rune(text)
		values := make([]interpreter.Value, len(runes))
		for i, r := range runes {
			values[i] = &interpreter.StringValue{Val: string(r)}
		}
		return interpreter.NewList(values...)
	})
	env.Set("std.string", module)
	env.Set("str", module)
}
