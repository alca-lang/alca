// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

// Package stdlib provides the Formats standard library module for the Alca programming language.
// It supports CSV with header mappings, INI/CONF configuration files, and deep JSONPath dot-notation queries.
package stdlib

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/sandbox"
)

// --- CSV Implementation ---

func parseCSVReader(r io.Reader, hasHeader bool) (interpreter.Value, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1 // flexible fields per record

	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	if len(records) == 0 {
		return interpreter.NewList(), nil
	}

	if hasHeader && len(records) > 0 {
		headers := records[0]
		var rows []interpreter.Value

		for _, record := range records[1:] {
			m := interpreter.NewMap()
			for i, val := range record {
				colName := ""
				if i < len(headers) {
					colName = strings.TrimSpace(headers[i])
				} else {
					colName = fmt.Sprintf("col_%d", i)
				}
				m.Set(colName, &interpreter.StringValue{Val: val})
			}
			rows = append(rows, m)
		}
		return interpreter.NewList(rows...), nil
	}

	// Raw rows without header mapping
	var rows []interpreter.Value
	for _, record := range records {
		var rowItems []interpreter.Value
		for _, val := range record {
			rowItems = append(rowItems, &interpreter.StringValue{Val: val})
		}
		rows = append(rows, interpreter.NewList(rowItems...))
	}
	return interpreter.NewList(rows...), nil
}

func stringifyCSV(data interpreter.Value, explicitHeaders []string) (string, error) {
	lv, ok := data.(*interpreter.ListValue)
	if !ok {
		return "", fmt.Errorf("csv data must be a list")
	}

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	if len(lv.ElementsSlice()) == 0 {
		return "", nil
	}

	// Check if elements are maps
	if _, isMap := lv.ElementsSlice()[0].(*interpreter.MapValue); isMap {
		headers := explicitHeaders
		if len(headers) == 0 {
			// Auto-discover headers from first map
			firstMap := lv.ElementsSlice()[0].(*interpreter.MapValue)
			headers = firstMap.Order
		}

		if err := writer.Write(headers); err != nil {
			return "", err
		}

		for _, elem := range lv.ElementsSlice() {
			if m, ok := elem.(*interpreter.MapValue); ok {
				row := make([]string, len(headers))
				for i, h := range headers {
					if val, ok := m.Get(h); ok {
						row[i] = interpreter.CoerceToString(val)
					} else {
						row[i] = ""
					}
				}
				if err := writer.Write(row); err != nil {
					return "", err
				}
			}
		}
	} else {
		// List of lists
		for _, elem := range lv.ElementsSlice() {
			if rowList, ok := elem.(*interpreter.ListValue); ok {
				row := make([]string, len(rowList.ElementsSlice()))
				for i, cell := range rowList.ElementsSlice() {
					row[i] = interpreter.CoerceToString(cell)
				}
				if err := writer.Write(row); err != nil {
					return "", err
				}
			}
		}
	}

	writer.Flush()
	return buf.String(), writer.Error()
}

// --- INI Implementation ---

func parseINI(content string) *interpreter.MapValue {
	rootMap := interpreter.NewMap()
	currentSectionName := ""
	currentSectionMap := interpreter.NewMap()
	rootMap.Set("", currentSectionMap)

	lines := strings.Split(content, "\n")
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		// Section header [section_name]
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSectionName = strings.TrimSpace(line[1 : len(line)-1])
			if existing, ok := rootMap.Get(currentSectionName); ok {
				currentSectionMap = existing.(*interpreter.MapValue)
			} else {
				currentSectionMap = interpreter.NewMap()
				rootMap.Set(currentSectionName, currentSectionMap)
			}
			continue
		}

		// Key-value pair (separated by = or :)
		var k, v string
		if idx := strings.IndexAny(line, "=:"); idx != -1 {
			k = strings.TrimSpace(line[:idx])
			v = strings.TrimSpace(line[idx+1:])
			// Strip wrapping quotes
			if (strings.HasPrefix(v, `"`) && strings.HasSuffix(v, `"`)) ||
				(strings.HasPrefix(v, `'`) && strings.HasSuffix(v, `'`)) {
				if len(v) >= 2 {
					v = v[1 : len(v)-1]
				}
			}
			currentSectionMap.Set(k, &interpreter.StringValue{Val: v})
		}
	}

	return rootMap
}

func stringifyINI(val *interpreter.MapValue) string {
	var sb strings.Builder

	// First print top-level / default section
	if defaultSec, ok := val.Get(""); ok {
		if secMap, ok := defaultSec.(*interpreter.MapValue); ok {
			for _, k := range secMap.Order {
				if v, ok := secMap.Get(k); ok {
					sb.WriteString(fmt.Sprintf("%s = %s\n", k, interpreter.CoerceToString(v)))
				}
			}
		}
	}

	// Then print each named section
	for _, secName := range val.Order {
		if secName == "" {
			continue
		}
		if secVal, ok := val.Get(secName); ok {
			if secMap, ok := secVal.(*interpreter.MapValue); ok {
				if sb.Len() > 0 {
					sb.WriteString("\n")
				}
				sb.WriteString(fmt.Sprintf("[%s]\n", secName))
				for _, k := range secMap.Order {
					if v, ok := secMap.Get(k); ok {
						sb.WriteString(fmt.Sprintf("%s = %s\n", k, interpreter.CoerceToString(v)))
					}
				}
			}
		}
	}

	return sb.String()
}

// --- JSON Query Implementation ---

// convertAlcaValueToGo converts an Alca value into plain Go data (maps, slices,
// scalars) so it can be traversed by queryJSON. Alca's Inspect() output uses
// unquoted map keys and would not survive a JSON string round-trip.
func convertAlcaValueToGo(v interpreter.Value) interface{} {
	switch val := v.(type) {
	case nil:
		return nil
	case *interpreter.MapValue:
		out := make(map[string]interface{}, val.Len())
		for _, k := range val.Keys() {
			item, _ := val.Get(k)
			out[k] = convertAlcaValueToGo(item)
		}
		return out
	case *interpreter.ListValue:
		out := make([]interface{}, val.Len())
		for i, el := range val.ElementsSlice() {
			out[i] = convertAlcaValueToGo(el)
		}
		return out
	case *interpreter.StringValue:
		return val.Val
	case *interpreter.IntegerValue:
		return val.Val
	case *interpreter.FloatValue:
		return val.Val
	case *interpreter.BoolValue:
		return val.Val
	case *interpreter.NullValue:
		return nil
	default:
		return interpreter.CoerceToString(v)
	}
}

func convertGoValueToAlca(val interface{}) interpreter.Value {
	if val == nil {
		return interpreter.NULL
	}
	switch v := val.(type) {
	case bool:
		if v {
			return interpreter.TRUE
		}
		return interpreter.FALSE
	case int:
		return &interpreter.IntegerValue{Val: int64(v)}
	case int64:
		return &interpreter.IntegerValue{Val: v}
	case float64:
		if v == float64(int64(v)) {
			return &interpreter.IntegerValue{Val: int64(v)}
		}
		return &interpreter.FloatValue{Val: v}
	case string:
		return &interpreter.StringValue{Val: v}
	case []interface{}:
		items := make([]interpreter.Value, len(v))
		for i, elem := range v {
			items[i] = convertGoValueToAlca(elem)
		}
		return interpreter.NewList(items...)
	case map[string]interface{}:
		m := interpreter.NewMap()
		for k, elem := range v {
			m.Set(k, convertGoValueToAlca(elem))
		}
		return m
	default:
		return &interpreter.StringValue{Val: fmt.Sprintf("%v", v)}
	}
}

// queryJSON navigates an arbitrary JSON data structure using dot-separated path tokens with list indexing (e.g., "users[0].name")
func queryJSON(root interface{}, path string) interface{} {
	if path == "" {
		return root
	}

	// Parse tokens: "users[0].name" -> ["users", "[0]", "name"]
	var tokens []string
	var current strings.Builder

	for i := 0; i < len(path); i++ {
		ch := path[i]
		if ch == '.' {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		} else if ch == '[' {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			idxEnd := strings.IndexByte(path[i:], ']')
			if idxEnd != -1 {
				tokens = append(tokens, path[i:i+idxEnd+1])
				i += idxEnd
			}
		} else {
			current.WriteByte(ch)
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	curr := root
	for _, tok := range tokens {
		if curr == nil {
			return nil
		}
		if strings.HasPrefix(tok, "[") && strings.HasSuffix(tok, "]") {
			// Array index
			idxStr := tok[1 : len(tok)-1]
			idx, err := strconv.Atoi(idxStr)
			if err != nil {
				return nil
			}
			slice, ok := curr.([]interface{})
			if !ok || idx < 0 || idx >= len(slice) {
				return nil
			}
			curr = slice[idx]
		} else {
			// Map key
			m, ok := curr.(map[string]interface{})
			if !ok {
				return nil
			}
			curr = m[tok]
		}
	}
	return curr
}

// RegisterFormatsBuiltins registers "std.formats", "csv", "ini", and "std.json" into the environment.
func RegisterFormatsBuiltins(env *interpreter.Environment) {
	formatsModule := interpreter.NewMap()
	csvModule := interpreter.NewMap()
	iniModule := interpreter.NewMap()
	jsonModule := interpreter.NewMap()

	// --- CSV Builtins ---

	csvReadFn := func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "csv.read requires file path or content string")
		}
		target := interpreter.CoerceToString(args[0])
		hasHeader := true
		if len(args) >= 2 {
			hasHeader = interpreter.IsTruthy(args[1])
		}

		var r io.Reader
		if strings.Contains(target, "\n") || len(target) > 260 {
			r = strings.NewReader(target)
			res, err := parseCSVReader(r, hasHeader)
			if err != nil {
				return interpreter.NewError(500, err.Error())
			}
			return res
		}

		fi, statErr := os.Stat(target)
		if statErr != nil && !os.IsNotExist(statErr) {
			// On Windows, strings with special chars return ERROR_INVALID_NAME; treat as string
			r = strings.NewReader(target)
			res, err := parseCSVReader(r, hasHeader)
			if err != nil {
				return handleIOError("csv_read", target, statErr)
			}
			return res
		}
		if statErr == nil && fi.IsDir() {
			return interpreter.NewError(400, "csv.read target is a directory")
		}
		if statErr == nil {
			if err := sandbox.RequireRead(target); err != nil {
				return handleIOError("csv_read", target, err)
			}
			f, err := os.Open(target)
			if err != nil {
				return handleIOError("csv_read", target, err)
			}
			defer f.Close()
			res, err := parseCSVReader(f, hasHeader)
			if err != nil {
				return interpreter.NewError(500, err.Error())
			}
			return res
		}

		r = strings.NewReader(target)
		res, err := parseCSVReader(r, hasHeader)
		if err != nil {
			return interpreter.NewError(500, err.Error())
		}
		return res
	}
	csvModule.Set("read", &interpreter.BuiltinFunction{Name: "read", Fn: csvReadFn})
	csvModule.Set("parse", &interpreter.BuiltinFunction{Name: "parse", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "csv.parse requires CSV content")
		}
		hasHeader := len(args) < 2 || interpreter.IsTruthy(args[1])
		result, err := parseCSVReader(strings.NewReader(interpreter.CoerceToString(args[0])), hasHeader)
		if err != nil {
			return interpreter.NewError(400, "invalid CSV: "+err.Error())
		}
		return result
	}})

	csvWriteFn := func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "csv.write requires (path, data, headers?)")
		}
		path := interpreter.CoerceToString(args[0])
		data := args[1]
		var headers []string
		if len(args) >= 3 {
			if hList, ok := args[2].(*interpreter.ListValue); ok {
				for _, h := range hList.ElementsSlice() {
					headers = append(headers, interpreter.CoerceToString(h))
				}
			}
		}

		content, err := stringifyCSV(data, headers)
		if err != nil {
			return interpreter.NewError(500, err.Error())
		}
		if err := sandbox.RequireWrite(path); err != nil {
			return handleIOError("csv_write", path, err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return handleIOError("csv_write", path, err)
		}
		return interpreter.TRUE
	}
	csvModule.Set("write", &interpreter.BuiltinFunction{Name: "write", Fn: csvWriteFn})

	csvStringifyFn := func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return &interpreter.StringValue{Val: ""}
		}
		data := args[0]
		var headers []string
		if len(args) >= 2 {
			if hList, ok := args[1].(*interpreter.ListValue); ok {
				for _, h := range hList.ElementsSlice() {
					headers = append(headers, interpreter.CoerceToString(h))
				}
			}
		}
		s, err := stringifyCSV(data, headers)
		if err != nil {
			return interpreter.NewError(500, err.Error())
		}
		return &interpreter.StringValue{Val: s}
	}
	csvModule.Set("stringify", &interpreter.BuiltinFunction{Name: "stringify", Fn: csvStringifyFn})

	// --- INI Builtins ---

	iniParseFn := func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewMap()
		}
		target := interpreter.CoerceToString(args[0])
		content := target
		fi, statErr := os.Stat(target)
		if statErr != nil && !os.IsNotExist(statErr) {
			return handleIOError("ini_read", target, statErr)
		}
		if statErr == nil && fi.IsDir() {
			return interpreter.NewError(400, "ini.read target is a directory")
		}
		if statErr == nil {
			if err := sandbox.RequireRead(target); err != nil {
				return handleIOError("ini_read", target, err)
			}
			data, err := os.ReadFile(target)
			if err != nil {
				return handleIOError("ini_read", target, err)
			}
			content = string(data)
		}
		return parseINI(content)
	}
	iniModule.Set("parse", &interpreter.BuiltinFunction{Name: "parse", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) != 1 {
			return interpreter.NewError(400, "ini.parse requires INI content")
		}
		return parseINI(interpreter.CoerceToString(args[0]))
	}})
	iniModule.Set("read", &interpreter.BuiltinFunction{Name: "read", Fn: iniParseFn})

	iniWriteFn := func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "ini.write requires (path, map_data)")
		}
		path := interpreter.CoerceToString(args[0])
		m, ok := args[1].(*interpreter.MapValue)
		if !ok {
			return interpreter.NewError(400, "ini.write requires a map")
		}
		content := stringifyINI(m)
		if err := sandbox.RequireWrite(path); err != nil {
			return handleIOError("ini_write", path, err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return handleIOError("ini_write", path, err)
		}
		return interpreter.TRUE
	}
	iniModule.Set("write", &interpreter.BuiltinFunction{Name: "write", Fn: iniWriteFn})

	iniStringifyFn := func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return &interpreter.StringValue{Val: ""}
		}
		m, ok := args[0].(*interpreter.MapValue)
		if !ok {
			return interpreter.NewError(400, "ini.stringify requires a map")
		}
		return &interpreter.StringValue{Val: stringifyINI(m)}
	}
	iniModule.Set("stringify", &interpreter.BuiltinFunction{Name: "stringify", Fn: iniStringifyFn})

	// --- JSON Builtins ---

	jsonQueryFn := func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NULL
		}
		target := args[0]
		path := interpreter.CoerceToString(args[1])

		var rawData interface{}

		if sv, ok := target.(*interpreter.StringValue); ok {
			if err := json.Unmarshal([]byte(sv.Val), &rawData); err != nil {
				return interpreter.NewError(400, "invalid JSON: "+err.Error())
			}
		} else {
			// Already an Alca map/list/scalar: convert it natively so paths
			// like "a[2]" keep working on decoded values.
			rawData = convertAlcaValueToGo(target)
		}

		res := queryJSON(rawData, path)
		return convertGoValueToAlca(res)
	}
	jsonModule.Set("query", &interpreter.BuiltinFunction{Name: "query", Fn: jsonQueryFn})

	jsonModule.Set("parse", &interpreter.BuiltinFunction{Name: "parse", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) != 1 {
			return interpreter.NewError(400, "json.parse requires one JSON string")
		}
		text, ok := args[0].(*interpreter.StringValue)
		if !ok {
			return interpreter.NewError(400, "json.parse input must be a string")
		}
		var raw interface{}
		if err := json.Unmarshal([]byte(text.Val), &raw); err != nil {
			return interpreter.NewError(400, "invalid JSON: "+err.Error())
		}
		return convertGoValueToAlca(raw)
	}})
	jsonEncodeFn := func(args ...interpreter.Value) interpreter.Value {
		if len(args) != 1 {
			return interpreter.NewError(400, "json.encode requires one value")
		}
		data, err := json.Marshal(convertAlcaValueToGo(args[0]))
		if err != nil {
			return interpreter.NewError(400, "JSON encoding failed: "+err.Error())
		}
		return &interpreter.StringValue{Val: string(data)}
	}
	jsonModule.Set("encode", &interpreter.BuiltinFunction{Name: "encode", Fn: jsonEncodeFn})
	jsonModule.Set("stringify", &interpreter.BuiltinFunction{Name: "stringify", Fn: jsonEncodeFn})
	jsonModule.Set("validate", &interpreter.BuiltinFunction{Name: "validate", Fn: func(args ...interpreter.Value) interpreter.Value {
		if len(args) != 1 {
			return interpreter.NewError(400, "json.validate requires one JSON string")
		}
		text, ok := args[0].(*interpreter.StringValue)
		if !ok {
			return interpreter.NewError(400, "json.validate input must be a string")
		}
		if json.Valid([]byte(text.Val)) {
			return interpreter.TRUE
		}
		return interpreter.FALSE
	}})

	jsonPrettifyFn := func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return &interpreter.StringValue{Val: ""}
		}
		var raw []byte
		if text, ok := args[0].(*interpreter.StringValue); ok {
			raw = []byte(text.Val)
		} else {
			var err error
			raw, err = json.Marshal(convertAlcaValueToGo(args[0]))
			if err != nil {
				return interpreter.NewError(400, "JSON encoding failed: "+err.Error())
			}
		}
		indent := "  "
		if len(args) >= 2 {
			indent = interpreter.CoerceToString(args[1])
		}

		var buf bytes.Buffer
		if err := json.Indent(&buf, raw, "", indent); err != nil {
			return interpreter.NewError(400, "invalid JSON: "+err.Error())
		}
		return &interpreter.StringValue{Val: buf.String()}
	}
	jsonModule.Set("prettify", &interpreter.BuiltinFunction{Name: "prettify", Fn: jsonPrettifyFn})
	legacyJSONPrettifyFn := func(args ...interpreter.Value) interpreter.Value {
		if len(args) > 0 {
			if _, ok := args[0].(*interpreter.StringValue); !ok {
				return &interpreter.StringValue{Val: interpreter.CoerceToString(args[0])}
			}
		}
		return jsonPrettifyFn(args...)
	}

	// Register submodules into Formats module
	formatsModule.Set("csv", csvModule)
	formatsModule.Set("ini", iniModule)
	formatsModule.Set("json", jsonModule)

	// Register in Environment
	env.Set("formats", formatsModule)
	env.Set("std.formats", formatsModule)

	env.Set("csv", csvModule)
	env.Set("std.csv", csvModule)

	env.Set("ini", iniModule)
	env.Set("std.ini", iniModule)
	env.Set("json", jsonModule)
	env.Set("std.json", jsonModule)

	env.Set("json_query", &interpreter.BuiltinFunction{Name: "json_query", Fn: jsonQueryFn})
	env.Set("json_prettify", &interpreter.BuiltinFunction{Name: "json_prettify", Fn: legacyJSONPrettifyFn})
}
