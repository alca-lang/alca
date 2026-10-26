// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package stdlib

import (
	"fmt"
	"strings"
	"unicode"
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

	register("trim_prefix", func(args ...interpreter.Value) interpreter.Value {
		text, errVal := getString(args, 0, "trim_prefix")
		if errVal != nil {
			return errVal
		}
		prefix, errVal := getString(args, 1, "trim_prefix")
		if errVal != nil {
			return errVal
		}
		return &interpreter.StringValue{Val: strings.TrimPrefix(text, prefix)}
	})

	register("trim_suffix", func(args ...interpreter.Value) interpreter.Value {
		text, errVal := getString(args, 0, "trim_suffix")
		if errVal != nil {
			return errVal
		}
		suffix, errVal := getString(args, 1, "trim_suffix")
		if errVal != nil {
			return errVal
		}
		return &interpreter.StringValue{Val: strings.TrimSuffix(text, suffix)}
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

	register("last_index_of", func(args ...interpreter.Value) interpreter.Value {
		text, errVal := getString(args, 0, "last_index_of")
		if errVal != nil {
			return errVal
		}
		part, errVal := getString(args, 1, "last_index_of")
		if errVal != nil {
			return errVal
		}
		index := strings.LastIndex(text, part)
		if index < 0 {
			return &interpreter.IntegerValue{Val: -1}
		}
		return &interpreter.IntegerValue{Val: int64(utf8.RuneCountInString(text[:index]))}
	})

	register("count", func(args ...interpreter.Value) interpreter.Value {
		text, errVal := getString(args, 0, "count")
		if errVal != nil {
			return errVal
		}
		substr, errVal := getString(args, 1, "count")
		if errVal != nil {
			return errVal
		}
		return &interpreter.IntegerValue{Val: int64(strings.Count(text, substr))}
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

	register("replace_n", func(args ...interpreter.Value) interpreter.Value {
		text, errVal := getString(args, 0, "replace_n")
		if errVal != nil {
			return errVal
		}
		old, errVal := getString(args, 1, "replace_n")
		if errVal != nil {
			return errVal
		}
		replacement, errVal := getString(args, 2, "replace_n")
		if errVal != nil {
			return errVal
		}
		if len(args) < 4 {
			return &interpreter.StringValue{Val: strings.ReplaceAll(text, old, replacement)}
		}
		nVal, ok := args[3].(*interpreter.IntegerValue)
		if !ok {
			return interpreter.NewError(400, "string.replace_n count argument must be integer")
		}
		return &interpreter.StringValue{Val: strings.Replace(text, old, replacement, int(nVal.Val))}
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

	register("repeat", func(args ...interpreter.Value) interpreter.Value {
		text, errVal := getString(args, 0, "repeat")
		if errVal != nil {
			return errVal
		}
		if len(args) < 2 {
			return interpreter.NewError(400, "string.repeat requires count argument")
		}
		countVal, ok := args[1].(*interpreter.IntegerValue)
		if !ok || countVal.Val < 0 {
			return interpreter.NewError(400, "string.repeat count must be non-negative integer")
		}
		return &interpreter.StringValue{Val: strings.Repeat(text, int(countVal.Val))}
	})

	register("pad_left", func(args ...interpreter.Value) interpreter.Value {
		text, errVal := getString(args, 0, "pad_left")
		if errVal != nil {
			return errVal
		}
		if len(args) < 2 {
			return interpreter.NewError(400, "string.pad_left requires width argument")
		}
		widthVal, ok := args[1].(*interpreter.IntegerValue)
		if !ok {
			return interpreter.NewError(400, "string.pad_left width must be integer")
		}
		pad := " "
		if len(args) >= 3 {
			p, err := getString(args, 2, "pad_left")
			if err != nil || p == "" {
				pad = " "
			} else {
				pad = p
			}
		}
		curLen := int64(utf8.RuneCountInString(text))
		target := widthVal.Val
		if curLen >= target {
			return &interpreter.StringValue{Val: text}
		}
		needed := target - curLen
		var b strings.Builder
		for int64(utf8.RuneCountInString(b.String())) < needed {
			b.WriteString(pad)
		}
		padStr := b.String()
		runes := []rune(padStr)
		if int64(len(runes)) > needed {
			padStr = string(runes[:needed])
		}
		return &interpreter.StringValue{Val: padStr + text}
	})

	register("pad_right", func(args ...interpreter.Value) interpreter.Value {
		text, errVal := getString(args, 0, "pad_right")
		if errVal != nil {
			return errVal
		}
		if len(args) < 2 {
			return interpreter.NewError(400, "string.pad_right requires width argument")
		}
		widthVal, ok := args[1].(*interpreter.IntegerValue)
		if !ok {
			return interpreter.NewError(400, "string.pad_right width must be integer")
		}
		pad := " "
		if len(args) >= 3 {
			p, err := getString(args, 2, "pad_right")
			if err != nil || p == "" {
				pad = " "
			} else {
				pad = p
			}
		}
		curLen := int64(utf8.RuneCountInString(text))
		target := widthVal.Val
		if curLen >= target {
			return &interpreter.StringValue{Val: text}
		}
		needed := target - curLen
		var b strings.Builder
		for int64(utf8.RuneCountInString(b.String())) < needed {
			b.WriteString(pad)
		}
		padStr := b.String()
		runes := []rune(padStr)
		if int64(len(runes)) > needed {
			padStr = string(runes[:needed])
		}
		return &interpreter.StringValue{Val: text + padStr}
	})

	register("center", func(args ...interpreter.Value) interpreter.Value {
		text, errVal := getString(args, 0, "center")
		if errVal != nil {
			return errVal
		}
		if len(args) < 2 {
			return interpreter.NewError(400, "string.center requires width argument")
		}
		widthVal, ok := args[1].(*interpreter.IntegerValue)
		if !ok {
			return interpreter.NewError(400, "string.center width must be integer")
		}
		pad := " "
		if len(args) >= 3 {
			p, err := getString(args, 2, "center")
			if err != nil || p == "" {
				pad = " "
			} else {
				pad = p
			}
		}
		curLen := int64(utf8.RuneCountInString(text))
		target := widthVal.Val
		if curLen >= target {
			return &interpreter.StringValue{Val: text}
		}
		totalPad := target - curLen
		leftPad := totalPad / 2
		rightPad := totalPad - leftPad

		makePad := func(needed int64) string {
			var b strings.Builder
			for int64(utf8.RuneCountInString(b.String())) < needed {
				b.WriteString(pad)
			}
			runes := []rune(b.String())
			if int64(len(runes)) > needed {
				return string(runes[:needed])
			}
			return b.String()
		}
		return &interpreter.StringValue{Val: makePad(leftPad) + text + makePad(rightPad)}
	})

	register("capitalize", func(args ...interpreter.Value) interpreter.Value {
		text, errVal := getString(args, 0, "capitalize")
		if errVal != nil {
			return errVal
		}
		runes := []rune(text)
		if len(runes) == 0 {
			return &interpreter.StringValue{Val: ""}
		}
		runes[0] = unicode.ToUpper(runes[0])
		for i := 1; i < len(runes); i++ {
			runes[i] = unicode.ToLower(runes[i])
		}
		return &interpreter.StringValue{Val: string(runes)}
	})

	register("title", func(args ...interpreter.Value) interpreter.Value {
		text, errVal := getString(args, 0, "title")
		if errVal != nil {
			return errVal
		}
		runes := []rune(text)
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
		return &interpreter.StringValue{Val: string(runes)}
	})

	register("is_empty", func(args ...interpreter.Value) interpreter.Value {
		text, errVal := getString(args, 0, "is_empty")
		if errVal != nil {
			return errVal
		}
		if len(text) == 0 {
			return interpreter.TRUE
		}
		return interpreter.FALSE
	})

	register("is_blank", func(args ...interpreter.Value) interpreter.Value {
		text, errVal := getString(args, 0, "is_blank")
		if errVal != nil {
			return errVal
		}
		if strings.TrimSpace(text) == "" {
			return interpreter.TRUE
		}
		return interpreter.FALSE
	})

	register("is_alpha", func(args ...interpreter.Value) interpreter.Value {
		text, errVal := getString(args, 0, "is_alpha")
		if errVal != nil {
			return errVal
		}
		if len(text) == 0 {
			return interpreter.FALSE
		}
		for _, r := range text {
			if !unicode.IsLetter(r) {
				return interpreter.FALSE
			}
		}
		return interpreter.TRUE
	})

	register("is_digit", func(args ...interpreter.Value) interpreter.Value {
		text, errVal := getString(args, 0, "is_digit")
		if errVal != nil {
			return errVal
		}
		if len(text) == 0 {
			return interpreter.FALSE
		}
		for _, r := range text {
			if !unicode.IsDigit(r) {
				return interpreter.FALSE
			}
		}
		return interpreter.TRUE
	})

	register("is_numeric", func(args ...interpreter.Value) interpreter.Value {
		text, errVal := getString(args, 0, "is_numeric")
		if errVal != nil {
			return errVal
		}
		if len(text) == 0 {
			return interpreter.FALSE
		}
		for _, r := range text {
			if !unicode.IsDigit(r) {
				return interpreter.FALSE
			}
		}
		return interpreter.TRUE
	})

	register("is_alphanumeric", func(args ...interpreter.Value) interpreter.Value {
		text, errVal := getString(args, 0, "is_alphanumeric")
		if errVal != nil {
			return errVal
		}
		if len(text) == 0 {
			return interpreter.FALSE
		}
		for _, r := range text {
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
				return interpreter.FALSE
			}
		}
		return interpreter.TRUE
	})

	register("is_upper", func(args ...interpreter.Value) interpreter.Value {
		text, errVal := getString(args, 0, "is_upper")
		if errVal != nil {
			return errVal
		}
		hasCased := false
		for _, r := range text {
			if unicode.IsUpper(r) {
				hasCased = true
			} else if unicode.IsLower(r) {
				return interpreter.FALSE
			}
		}
		if hasCased {
			return interpreter.TRUE
		}
		return interpreter.FALSE
	})

	register("is_lower", func(args ...interpreter.Value) interpreter.Value {
		text, errVal := getString(args, 0, "is_lower")
		if errVal != nil {
			return errVal
		}
		hasCased := false
		for _, r := range text {
			if unicode.IsLower(r) {
				hasCased = true
			} else if unicode.IsUpper(r) {
				return interpreter.FALSE
			}
		}
		if hasCased {
			return interpreter.TRUE
		}
		return interpreter.FALSE
	})

	register("char_at", func(args ...interpreter.Value) interpreter.Value {
		text, errVal := getString(args, 0, "char_at")
		if errVal != nil {
			return errVal
		}
		if len(args) < 2 {
			return interpreter.NewError(400, "string.char_at requires index")
		}
		idxVal, ok := args[1].(*interpreter.IntegerValue)
		if !ok {
			return interpreter.NewError(400, "string.char_at index must be integer")
		}
		runes := []rune(text)
		n := int64(len(runes))
		idx := idxVal.Val
		if idx < 0 {
			idx += n
		}
		if idx < 0 || idx >= n {
			return &interpreter.StringValue{Val: ""}
		}
		return &interpreter.StringValue{Val: string(runes[idx])}
	})

	register("code_point_at", func(args ...interpreter.Value) interpreter.Value {
		text, errVal := getString(args, 0, "code_point_at")
		if errVal != nil {
			return errVal
		}
		if len(args) < 2 {
			return interpreter.NewError(400, "string.code_point_at requires index")
		}
		idxVal, ok := args[1].(*interpreter.IntegerValue)
		if !ok {
			return interpreter.NewError(400, "string.code_point_at index must be integer")
		}
		runes := []rune(text)
		n := int64(len(runes))
		idx := idxVal.Val
		if idx < 0 {
			idx += n
		}
		if idx < 0 || idx >= n {
			return &interpreter.IntegerValue{Val: -1}
		}
		return &interpreter.IntegerValue{Val: int64(runes[idx])}
	})

	register("from_code_point", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "string.from_code_point requires code point integer")
		}
		cpVal, ok := args[0].(*interpreter.IntegerValue)
		if !ok {
			return interpreter.NewError(400, "string.from_code_point expects integer code point")
		}
		return &interpreter.StringValue{Val: string(rune(cpVal.Val))}
	})

	register("lines", func(args ...interpreter.Value) interpreter.Value {
		text, errVal := getString(args, 0, "lines")
		if errVal != nil {
			return errVal
		}
		normalized := strings.ReplaceAll(text, "\r\n", "\n")
		normalized = strings.ReplaceAll(normalized, "\r", "\n")
		parts := strings.Split(normalized, "\n")
		values := make([]interpreter.Value, len(parts))
		for i, p := range parts {
			values[i] = &interpreter.StringValue{Val: p}
		}
		return interpreter.NewList(values...)
	})

	register("words", func(args ...interpreter.Value) interpreter.Value {
		text, errVal := getString(args, 0, "words")
		if errVal != nil {
			return errVal
		}
		parts := strings.Fields(text)
		values := make([]interpreter.Value, len(parts))
		for i, p := range parts {
			values[i] = &interpreter.StringValue{Val: p}
		}
		return interpreter.NewList(values...)
	})

	register("reverse", func(args ...interpreter.Value) interpreter.Value {
		text, errVal := getString(args, 0, "reverse")
		if errVal != nil {
			return errVal
		}
		runes := []rune(text)
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		return &interpreter.StringValue{Val: string(runes)}
	})

	// format(template, args...) -> string
	register("format", func(args ...interpreter.Value) interpreter.Value {
		if len(args) == 0 {
			return &interpreter.StringValue{Val: ""}
		}
		tpl, errVal := getString(args, 0, "format")
		if errVal != nil {
			return errVal
		}
		return &interpreter.StringValue{Val: formatStringTemplate(tpl, args[1:])}
	})

	// pad(str, target_len, pad_char?, side?: "left" | "right" | "both") -> string
	register("pad", func(args ...interpreter.Value) interpreter.Value {
		text, errVal := getString(args, 0, "pad")
		if errVal != nil {
			return errVal
		}
		target := 0
		if len(args) >= 2 {
			if iv, ok := args[1].(*interpreter.IntegerValue); ok {
				target = int(iv.Val)
			}
		}
		pad := " "
		if len(args) >= 3 {
			pad = interpreter.CoerceToString(args[2])
			if pad == "" {
				pad = " "
			}
		}
		side := "right"
		if len(args) >= 4 {
			side = strings.ToLower(strings.TrimSpace(interpreter.CoerceToString(args[3])))
		}
		cur := utf8.RuneCountInString(text)
		if cur >= target {
			return &interpreter.StringValue{Val: text}
		}
		needed := target - cur
		makePad := func(cnt int) string {
			if cnt <= 0 {
				return ""
			}
			var bld strings.Builder
			for utf8.RuneCountInString(bld.String()) < cnt {
				bld.WriteString(pad)
			}
			r := []rune(bld.String())
			if len(r) > cnt {
				return string(r[:cnt])
			}
			return bld.String()
		}

		switch side {
		case "left":
			return &interpreter.StringValue{Val: makePad(needed) + text}
		case "both", "center":
			leftLen := needed / 2
			rightLen := needed - leftLen
			return &interpreter.StringValue{Val: makePad(leftLen) + text + makePad(rightLen)}
		default: // "right"
			return &interpreter.StringValue{Val: text + makePad(needed)}
		}
	})

	env.Set("std.string", module)
	env.Set("std.str", module)
	env.Set("str", module)
}

func formatStringTemplate(template string, args []interpreter.Value) string {
	if len(args) == 0 {
		return template
	}
	// Check for {0}, {1} or {}
	if strings.Contains(template, "{") && strings.Contains(template, "}") {
		res := template
		for i, a := range args {
			placeholder := fmt.Sprintf("{%d}", i)
			res = strings.ReplaceAll(res, placeholder, interpreter.CoerceToString(a))
		}
		for _, a := range args {
			if !strings.Contains(res, "{}") {
				break
			}
			res = strings.Replace(res, "{}", interpreter.CoerceToString(a), 1)
		}
		return res
	}
	// Fallback to sprintf if % present
	if strings.Contains(template, "%") {
		rawArgs := make([]any, len(args))
		for i, a := range args {
			switch v := a.(type) {
			case *interpreter.IntegerValue:
				rawArgs[i] = v.Val
			case *interpreter.FloatValue:
				rawArgs[i] = v.Val
			case *interpreter.BoolValue:
				rawArgs[i] = v.Val
			case *interpreter.StringValue:
				rawArgs[i] = v.Val
			default:
				rawArgs[i] = a.Inspect()
			}
		}
		return fmt.Sprintf(template, rawArgs...)
	}
	return template
}
