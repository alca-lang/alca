// Package stdlib provides the CLI standard library module for the Alca programming language.
// It supports command-line argument parsing, ANSI terminal colors, interactive prompts, secure password entry, and progress bars.
package stdlib

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/alca-lang/alca/pkg/interpreter"
)

var (
	scriptCommandLineArgs []string
)

// SetScriptArgs stores the command line arguments passed specifically to the Alca user script.
func SetScriptArgs(args []string) {
	scriptCommandLineArgs = args
}

// GetScriptArgs retrieves the stored command line arguments for the running script.
func GetScriptArgs() []string {
	if scriptCommandLineArgs == nil {
		return []string{}
	}
	return scriptCommandLineArgs
}

var ansiColors = map[string]string{
	"reset":     "\x1b[0m",
	"bold":      "\x1b[1m",
	"dim":       "\x1b[2m",
	"italic":    "\x1b[3m",
	"underline": "\x1b[4m",
	"black":     "\x1b[30m",
	"red":       "\x1b[31m",
	"green":     "\x1b[32m",
	"yellow":    "\x1b[33m",
	"blue":      "\x1b[34m",
	"magenta":   "\x1b[35m",
	"cyan":      "\x1b[36m",
	"white":     "\x1b[37m",
	"gray":      "\x1b[90m",
	"bg_black":  "\x1b[40m",
	"bg_red":    "\x1b[41m",
	"bg_green":  "\x1b[42m",
	"bg_yellow": "\x1b[43m",
	"bg_blue":   "\x1b[44m",
	"bg_cyan":   "\x1b[46m",
	"bg_white":  "\x1b[47m",
}

// parseCommandLineFlags parses command line arguments using a specification schema.
func parseCommandLineFlags(rawArgs []string, spec *interpreter.MapValue) *interpreter.MapValue {
	result := interpreter.NewMap()
	var positional []interpreter.Value

	// Initialize defaults from spec
	type optInfo struct {
		key      string
		optType  string
		defVal   interpreter.Value
		flag     string
		longFlag string
	}

	options := make(map[string]*optInfo)

	for _, k := range spec.Order {
		v, _ := spec.Get(k)
		info := &optInfo{
			key:      k,
			optType:  "string",
			flag:     "-" + k[:1],
			longFlag: "--" + k,
		}

		if subMap, ok := v.(*interpreter.MapValue); ok {
			if tVal, ok := subMap.Get("type"); ok {
				info.optType = interpreter.CoerceToString(tVal)
			}
			if dVal, ok := subMap.Get("default"); ok {
				info.defVal = dVal
			}
			if fVal, ok := subMap.Get("flag"); ok {
				info.flag = interpreter.CoerceToString(fVal)
			}
			if lVal, ok := subMap.Get("long"); ok {
				info.longFlag = interpreter.CoerceToString(lVal)
			}
		} else {
			info.defVal = v
			switch v.(type) {
			case *interpreter.BoolValue:
				info.optType = "bool"
			case *interpreter.IntegerValue:
				info.optType = "int"
			case *interpreter.FloatValue:
				info.optType = "float"
			default:
				info.optType = "string"
			}
		}

		if info.defVal != nil {
			result.Set(info.key, info.defVal)
		} else {
			if info.optType == "bool" {
				result.Set(info.key, interpreter.FALSE)
			} else {
				result.Set(info.key, interpreter.NULL)
			}
		}

		options[info.flag] = info
		options[info.longFlag] = info
	}

	// Parse arguments
	for i := 0; i < len(rawArgs); i++ {
		arg := rawArgs[i]

		if strings.HasPrefix(arg, "-") {
			// Check for --key=value
			eqIdx := strings.IndexByte(arg, '=')
			flagPart := arg
			inlineVal := ""
			hasInlineVal := false
			if eqIdx != -1 {
				flagPart = arg[:eqIdx]
				inlineVal = arg[eqIdx+1:]
				hasInlineVal = true
			}

			if opt, found := options[flagPart]; found {
				if opt.optType == "bool" {
					if hasInlineVal {
						boolVal := inlineVal == "true" || inlineVal == "1"
						if boolVal {
							result.Set(opt.key, interpreter.TRUE)
						} else {
							result.Set(opt.key, interpreter.FALSE)
						}
					} else {
						result.Set(opt.key, interpreter.TRUE)
					}
				} else {
					valStr := inlineVal
					if !hasInlineVal {
						if i+1 < len(rawArgs) && !strings.HasPrefix(rawArgs[i+1], "-") {
							valStr = rawArgs[i+1]
							i++
						}
					}
					switch opt.optType {
					case "int":
						iv, _ := strconv.ParseInt(valStr, 10, 64)
						result.Set(opt.key, &interpreter.IntegerValue{Val: iv})
					case "float":
						fv, _ := strconv.ParseFloat(valStr, 64)
						result.Set(opt.key, &interpreter.FloatValue{Val: fv})
					default:
						result.Set(opt.key, &interpreter.StringValue{Val: valStr})
					}
				}
				continue
			}
		}

		positional = append(positional, &interpreter.StringValue{Val: arg})
	}

	result.Set("_args", interpreter.NewList(positional...))
	return result
}

// RegisterCLIBuiltins registers the "std.cli" and "cli" modules into the interpreter environment.
func RegisterCLIBuiltins(env *interpreter.Environment) {
	cliModule := interpreter.NewMap()

	register := func(name string, fn func(args ...interpreter.Value) interpreter.Value) {
		bf := &interpreter.BuiltinFunction{Name: name, Fn: fn}
		cliModule.Set(name, bf)
	}

	// args() -> list of strings
	register("args", func(args ...interpreter.Value) interpreter.Value {
		raw := GetScriptArgs()
		vals := make([]interpreter.Value, len(raw))
		for i, a := range raw {
			vals[i] = &interpreter.StringValue{Val: a}
		}
		return interpreter.NewList(vals...)
	})

	// color(color_name, text) -> colored text string
	register("color", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			if len(args) == 1 {
				return args[0]
			}
			return &interpreter.StringValue{Val: ""}
		}
		colorName := strings.ToLower(interpreter.CoerceToString(args[0]))
		text := interpreter.CoerceToString(args[1])

		code, ok := ansiColors[colorName]
		if !ok {
			return &interpreter.StringValue{Val: text}
		}
		return &interpreter.StringValue{Val: code + text + ansiColors["reset"]}
	})

	// parse(spec_map) -> parsed options map
	register("parse", func(args ...interpreter.Value) interpreter.Value {
		spec := interpreter.NewMap()
		if len(args) >= 1 {
			if sm, ok := args[0].(*interpreter.MapValue); ok {
				spec = sm
			}
		}
		return parseCommandLineFlags(GetScriptArgs(), spec)
	})

	// prompt(message, default_val?) -> string
	register("prompt", func(args ...interpreter.Value) interpreter.Value {
		msg := "Input: "
		defVal := ""
		if len(args) >= 1 {
			msg = interpreter.CoerceToString(args[0])
		}
		if len(args) >= 2 {
			defVal = interpreter.CoerceToString(args[1])
			if defVal != "" {
				msg += fmt.Sprintf(" [%s]", defVal)
			}
		}
		fmt.Print(msg)
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			val := scanner.Text()
			if strings.TrimSpace(val) == "" && defVal != "" {
				return &interpreter.StringValue{Val: defVal}
			}
			return &interpreter.StringValue{Val: val}
		}
		return &interpreter.StringValue{Val: defVal}
	})

	// prompt_password(message) -> string (hidden input)
	register("prompt_password", func(args ...interpreter.Value) interpreter.Value {
		msg := "Password: "
		if len(args) >= 1 {
			msg = interpreter.CoerceToString(args[0])
		}
		fmt.Print(msg)
		pwd, err := readPasswordSecure()
		if err != nil {
			return interpreter.NewError(500, err.Error())
		}
		return &interpreter.StringValue{Val: pwd}
	})

	// confirm(message, default_bool?) -> bool
	register("confirm", func(args ...interpreter.Value) interpreter.Value {
		msg := "Proceed?"
		defBool := false
		if len(args) >= 1 {
			msg = interpreter.CoerceToString(args[0])
		}
		if len(args) >= 2 {
			defBool = interpreter.IsTruthy(args[1])
		}

		hint := " [y/N]: "
		if defBool {
			hint = " [Y/n]: "
		}
		fmt.Print(msg + hint)

		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			ans := strings.ToLower(strings.TrimSpace(scanner.Text()))
			if ans == "" {
				if defBool {
					return interpreter.TRUE
				}
				return interpreter.FALSE
			}
			if ans == "y" || ans == "yes" || ans == "true" || ans == "1" || ans == "д" || ans == "да" {
				return interpreter.TRUE
			}
		}
		return interpreter.FALSE
	})

	// progress_bar(current, total, width?, prefix?) -> string
	register("progress_bar", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return &interpreter.StringValue{Val: ""}
		}
		curr := 0.0
		total := 100.0
		switch v := args[0].(type) {
		case *interpreter.IntegerValue:
			curr = float64(v.Val)
		case *interpreter.FloatValue:
			curr = v.Val
		}
		switch v := args[1].(type) {
		case *interpreter.IntegerValue:
			total = float64(v.Val)
		case *interpreter.FloatValue:
			total = v.Val
		}

		width := 30
		if len(args) >= 3 {
			if iv, ok := args[2].(*interpreter.IntegerValue); ok && iv.Val > 0 {
				width = int(iv.Val)
			}
		}

		prefix := ""
		if len(args) >= 4 {
			prefix = interpreter.CoerceToString(args[3]) + " "
		}

		ratio := 0.0
		if total > 0 {
			ratio = curr / total
			if ratio > 1.0 {
				ratio = 1.0
			}
			if ratio < 0.0 {
				ratio = 0.0
			}
		}

		filled := int(math.Round(ratio * float64(width)))
		bar := strings.Repeat("=", filled)
		if filled < width {
			bar += ">" + strings.Repeat(" ", width-filled-1)
		}

		pct := math.Round(ratio*1000) / 10
		res := fmt.Sprintf("%s[%s] %.1f%% (%.0f/%.0f)", prefix, bar, pct, curr, total)
		return &interpreter.StringValue{Val: res}
	})

	env.Set("cli", cliModule)
	env.Set("std.cli", cliModule)
}
