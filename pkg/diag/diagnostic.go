// Package diag provides rich, human-friendly compiler diagnostic formatting
// with source code previews, line numbers, caret pointers, and actionable hints.
package diag

import (
	"fmt"
	"strconv"
	"strings"
)

// ANSI color escape codes for terminal formatting.
const (
	colorReset     = "\033[0m"
	colorBold      = "\033[1m"
	colorRed       = "\033[31m"
	colorBoldRed   = "\033[1;31m"
	colorGreen     = "\033[32m"
	colorBoldGreen = "\033[1;32m"
	colorYellow    = "\033[33m"
	colorBoldYel   = "\033[1;33m"
	colorCyan      = "\033[36m"
	colorBoldCyan  = "\033[1;36m"
	colorGray      = "\033[90m"
)

// SyntaxError represents a structured syntax or parse error with location info.
type SyntaxError struct {
	Filename   string
	Line       int
	Column     int
	Message    string
	Token      string
	Length     int
	Suggestion string
}

// Error implements the error interface.
func (e *SyntaxError) Error() string {
	return fmt.Sprintf("%s:%d:%d: %s", e.Filename, e.Line, e.Column, e.Message)
}

// FormatReport formats a slice of syntax errors into a clear, beautiful diagnostic report.
func FormatReport(filename string, source string, errors []*SyntaxError, useColor bool) string {
	if len(errors) == 0 {
		return ""
	}

	// Deduplicate errors on same line and column
	var unique []*SyntaxError
	seen := make(map[string]bool)
	for _, e := range errors {
		key := fmt.Sprintf("%d:%d:%s", e.Line, e.Column, e.Message)
		if !seen[key] {
			seen[key] = true
			unique = append(unique, e)
		}
	}

	lines := strings.Split(source, "\n")
	var b strings.Builder

	b.WriteString("\n")
	for i, err := range unique {
		b.WriteString(formatSingleError(filename, lines, err, useColor))
		if i < len(unique)-1 {
			b.WriteString("\n")
		}
	}

	errWord := "error"
	if len(unique) != 1 {
		errWord = "errors"
	}
	summary := fmt.Sprintf("\nFound %d %s. Execution aborted.\n", len(unique), errWord)
	if useColor {
		summary = colorBoldRed + summary + colorReset
	}
	b.WriteString(summary)

	return b.String()
}

func formatSingleError(filename string, lines []string, err *SyntaxError, useColor bool) string {
	var b strings.Builder

	cRed := colorBoldRed
	cCyan := colorBoldCyan
	cGray := colorGray
	cYel := colorBoldYel
	cGreen := colorBoldGreen
	cReset := colorReset
	cBold := colorBold

	if !useColor {
		cRed, cCyan, cGray, cYel, cGreen, cReset, cBold = "", "", "", "", "", "", ""
	}

	// Header: error[E001]: <message>
	b.WriteString(fmt.Sprintf("%serror: %s%s%s\n", cRed, cReset, cBold, err.Message))
	b.WriteString(fmt.Sprintf("  %s-->%s %s%s:%d:%d%s\n", cCyan, cReset, cGray, filename, err.Line, err.Column, cReset))

	// Separator line
	b.WriteString(fmt.Sprintf("   %s|%s\n", cCyan, cReset))

	// Display context lines: 1 line before, the error line, and 1 line after
	startLine := err.Line - 1
	if startLine < 1 {
		startLine = 1
	}
	endLine := err.Line + 1
	if endLine > len(lines) {
		endLine = len(lines)
	}

	for l := startLine; l <= endLine; l++ {
		lineIdx := l - 1
		lineContent := ""
		if lineIdx >= 0 && lineIdx < len(lines) {
			lineContent = strings.TrimRight(lines[lineIdx], "\r")
		}

		if l == err.Line {
			// Error line with marker
			b.WriteString(fmt.Sprintf("%s%2d |%s %s\n", cCyan, l, cReset, lineContent))

			// Underline / Caret
			padLen := err.Column - 1
			if padLen < 0 {
				padLen = 0
			}

			// If the line has leading tabs, preserve width
			tabAdjustedPad := ""
			for ci, ch := range lineContent {
				if ci >= padLen {
					break
				}
				if ch == '\t' {
					tabAdjustedPad += "    "
				} else {
					tabAdjustedPad += " "
				}
			}

			length := err.Length
			if length <= 0 {
				length = 1
			}
			carets := strings.Repeat("^", length)

			b.WriteString(fmt.Sprintf("   %s|%s %s%s%s %s%s\n",
				cCyan, cReset, tabAdjustedPad, cYel, carets, err.Message, cReset))
		} else {
			// Context line
			b.WriteString(fmt.Sprintf("%s%2d |%s %s%s%s\n", cGray, l, cReset, cGray, lineContent, cReset))
		}
	}

	b.WriteString(fmt.Sprintf("   %s|%s\n", cCyan, cReset))

	// Suggestion / Help hint
	suggestion := err.Suggestion
	if suggestion == "" {
		suggestion = generateSuggestion(err.Message, err.Token)
	}
	if suggestion != "" {
		b.WriteString(fmt.Sprintf("   %s=%s %shelp:%s %s\n", cGreen, cReset, cGreen, cReset, suggestion))
	}

	return b.String()
}

// generateSuggestion infers helpful tips based on the error message and token.
func generateSuggestion(msg, tok string) string {
	msgLower := strings.ToLower(msg)

	switch {
	case strings.Contains(msgLower, "no prefix parse function for :"):
		return "Unexpected ':' delimiter. In Alca, use ':' only to end block headers (e.g. 'if cond:', 'func name():') or for variable type annotations ('x: int = 5')."
	case strings.Contains(msgLower, "no prefix parse function for ="):
		return "Unexpected '=' token. Check if you have an extra assignment operator or incomplete statement."
	case strings.Contains(msgLower, "expected next token to be :"):
		return "Block statements ('if', 'while', 'for', 'func', 'struct', 'ba', 'threads') require a trailing ':' before the indented body."
	case strings.Contains(msgLower, "no prefix parse function for indent"):
		return "Unexpected indentation. Ensure statements are aligned with 4 spaces or 1 tab per nesting level."
	case strings.Contains(msgLower, "no prefix parse function for dedent"):
		return "Check block alignment. Indentation level decreased to a level that does not match any outer block."
	case strings.Contains(msgLower, "expected next token to be )"):
		return "Unclosed parenthesis. Did you forget a closing ')'?"
	case strings.Contains(msgLower, "expected next token to be ]"):
		return "Unclosed bracket. Did you forget a closing ']'?"
	case strings.Contains(msgLower, "expected next token to be }"):
		return "Unclosed brace. Did you forget a closing '}'?"
	case strings.Contains(msgLower, "no prefix parse function for +") || strings.Contains(msgLower, "no prefix parse function for *"):
		return "Expression missing left-hand operand. Check for stray arithmetic operators."
	case strings.Contains(msgLower, "unknown"):
		return "Verify the keyword spelling and variable names."
	default:
		return ""
	}
}

// RuntimeError represents an execution failure within the Alca language runtime.
type RuntimeError struct {
	Message    string
	Code       int64
	Filename   string
	Line       int
	Column     int
	Suggestion string
}

func (e *RuntimeError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("%s:%d:%d: runtime error: %s (code %d)", e.Filename, e.Line, e.Column, e.Message, e.Code)
	}
	return fmt.Sprintf("runtime error: %s (code %d)", e.Message, e.Code)
}

// CleanErrorMessage translates raw Go OS/runtime error messages into clean, human-friendly Alca errors.
func CleanErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	raw := err.Error()

	// Strip common Go prefixes
	raw = strings.TrimPrefix(raw, "alca runtime error: ")
	raw = strings.TrimPrefix(raw, "Runtime error: ")
	raw = strings.TrimPrefix(raw, "VM runtime error: ")
	raw = strings.TrimPrefix(raw, "Bytecode compile error: ")

	lower := strings.ToLower(raw)

	switch {
	case strings.Contains(lower, "the system cannot find the file specified") || strings.Contains(lower, "no such file or directory"):
		return "file does not exist"
	case strings.Contains(lower, "the system cannot find the path specified"):
		return "directory path does not exist"
	case strings.Contains(lower, "access is denied") || strings.Contains(lower, "permission denied"):
		return "permission denied (insufficient privileges)"
	case strings.Contains(lower, "executable file not found in %path%") || strings.Contains(lower, "executable file not found in $path"):
		return "command or executable not found in system PATH"
	case strings.Contains(lower, "is a directory"):
		return "target is a directory, expected a file"
	case strings.Contains(lower, "not a directory"):
		return "target is a file, expected a directory"
	case strings.Contains(lower, "file exists"):
		return "target file already exists"
	case strings.Contains(lower, "no such host"):
		return "DNS lookup failed (host not found)"
	case strings.Contains(lower, "connection refused") || strings.Contains(lower, "connectex"):
		return "network connection refused by target host"
	case strings.Contains(lower, "address already in use"):
		return "network port/address is already in use"
	case strings.Contains(lower, "integer divide by zero") || strings.Contains(lower, "division by zero"):
		return "division by zero"
	case strings.Contains(lower, "modulo by zero"):
		return "modulo by zero"
	case strings.Contains(lower, "nil pointer dereference") || strings.Contains(lower, "invalid memory address"):
		return "null pointer reference (attempted to access member on null)"
	case strings.Contains(lower, "index out of range"):
		return "index out of bounds"
	case strings.Contains(lower, "stack overflow"):
		return "call stack overflow (infinite recursion detected)"
	default:
		// Remove Go-specific file path decorators like "open C:\...:"
		if idx := strings.LastIndex(raw, ": "); idx != -1 && idx+2 < len(raw) {
			tail := raw[idx+2:]
			if len(tail) > 3 {
				return tail
			}
		}
		return raw
	}
}

// FormatRuntimeError formats any runtime execution error in Alca compiler diagnostic style.
func FormatRuntimeError(filename string, source string, err error, useColor bool) string {
	if err == nil {
		return ""
	}

	cRed := colorBoldRed
	cCyan := colorBoldCyan
	cGray := colorGray
	cYel := colorBoldYel
	cGreen := colorBoldGreen
	cReset := colorReset
	cBold := colorBold

	if !useColor {
		cRed, cCyan, cGray, cYel, cGreen, cReset, cBold = "", "", "", "", "", "", ""
	}

	line := 0
	col := 0
	code := int64(400)
	msg := CleanErrorMessage(err)
	suggestion := ""

	if rtErr, ok := err.(*RuntimeError); ok {
		line = rtErr.Line
		col = rtErr.Column
		code = rtErr.Code
		msg = rtErr.Message
		suggestion = rtErr.Suggestion
	} else {
		// Try to extract line from error text like "foo.al:12:4: ..."
		str := err.Error()
		if parts := strings.Split(str, ":"); len(parts) >= 3 {
			if l, e := strconv.Atoi(parts[1]); e == nil {
				line = l
			}
			if c, e := strconv.Atoi(parts[2]); e == nil {
				col = c
			}
		}
		// If line still 0, try to locate relevant line in source
		if line == 0 && source != "" {
			lines := strings.Split(source, "\n")
			for i, l := range lines {
				if strings.Contains(msg, "division by zero") && (strings.Contains(l, "/ 0") || strings.Contains(l, "/0")) {
					line = i + 1
					col = strings.Index(l, "/") + 1
					break
				}
			}
		}
	}

	if suggestion == "" {
		suggestion = generateRuntimeSuggestion(msg)
	}

	var b strings.Builder
	b.WriteString("\n")

	// Header: error[RuntimeError]: <message>
	b.WriteString(fmt.Sprintf("%serror[RuntimeError]: %s%s%s (code %d)\n", cRed, cReset, cBold, msg, code))
	if line > 0 {
		b.WriteString(fmt.Sprintf("  %s-->%s %s%s:%d:%d%s\n", cCyan, cReset, cGray, filename, line, col, cReset))
	} else {
		b.WriteString(fmt.Sprintf("  %s-->%s %s%s%s\n", cCyan, cReset, cGray, filename, cReset))
	}

	// Code preview
	if line > 0 && source != "" {
		lines := strings.Split(source, "\n")
		b.WriteString(fmt.Sprintf("   %s|%s\n", cCyan, cReset))

		startLine := line - 1
		if startLine < 1 {
			startLine = 1
		}
		endLine := line + 1
		if endLine > len(lines) {
			endLine = len(lines)
		}

		for l := startLine; l <= endLine; l++ {
			lineIdx := l - 1
			lineContent := ""
			if lineIdx >= 0 && lineIdx < len(lines) {
				lineContent = strings.TrimRight(lines[lineIdx], "\r")
			}

			if l == line {
				b.WriteString(fmt.Sprintf("%s%2d |%s %s\n", cCyan, l, cReset, lineContent))
				padLen := col - 1
				if padLen < 0 {
					padLen = 0
				}
				tabAdjustedPad := ""
				for ci, ch := range lineContent {
					if ci >= padLen {
						break
					}
					if ch == '\t' {
						tabAdjustedPad += "    "
					} else {
						tabAdjustedPad += " "
					}
				}
				b.WriteString(fmt.Sprintf("   %s|%s %s%s^ %s%s\n", cCyan, cReset, tabAdjustedPad, cYel, msg, cReset))
			} else {
				b.WriteString(fmt.Sprintf("%s%2d |%s %s%s%s\n", cGray, l, cReset, cGray, lineContent, cReset))
			}
		}
		b.WriteString(fmt.Sprintf("   %s|%s\n", cCyan, cReset))
	}

	if suggestion != "" {
		b.WriteString(fmt.Sprintf("   %s=%s %shelp:%s %s\n", cGreen, cReset, cGreen, cReset, suggestion))
	}
	b.WriteString("\n")

	return b.String()
}

// FormatIOReport formats source file read or write errors without exposing Go errors.
func FormatIOReport(filename string, op string, target string, err error, useColor bool) string {
	cRed := colorBoldRed
	cCyan := colorBoldCyan
	cGray := colorGray
	cGreen := colorBoldGreen
	cReset := colorReset
	cBold := colorBold

	if !useColor {
		cRed, cCyan, cGray, cGreen, cReset, cBold = "", "", "", "", "", ""
	}

	cleanMsg := CleanErrorMessage(err)
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("%serror[IOError]: %s%scannot %s '%s'\n", cRed, cReset, cBold, op, target))
	b.WriteString(fmt.Sprintf("  %s-->%s %s%s%s\n", cCyan, cReset, cGray, target, cReset))
	b.WriteString(fmt.Sprintf("   %s|%s\n", cCyan, cReset))
	b.WriteString(fmt.Sprintf("   %s=%s %scause:%s %s\n", cCyan, cReset, cBold, cReset, cleanMsg))

	suggestion := "verify that the file path exists and is accessible"
	if strings.Contains(cleanMsg, "permission denied") {
		suggestion = "check read/write permissions or run with elevated privileges"
	}
	b.WriteString(fmt.Sprintf("   %s=%s %shelp:%s %s\n\n", cGreen, cReset, cGreen, cReset, suggestion))
	return b.String()
}

// FormatPanicReport catches any unhandled Go panic and displays it as a Alca engine failure.
func FormatPanicReport(filename string, r interface{}, useColor bool) string {
	cRed := colorBoldRed
	cCyan := colorBoldCyan
	cGray := colorGray
	cGreen := colorBoldGreen
	cReset := colorReset
	cBold := colorBold

	if !useColor {
		cRed, cCyan, cGray, cGreen, cReset, cBold = "", "", "", "", "", ""
	}

	detail := fmt.Sprintf("%v", r)
	if err, ok := r.(error); ok {
		detail = CleanErrorMessage(err)
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("%serror[EngineFault]: %s%sunexpected runtime condition in Alca engine\n", cRed, cReset, cBold))
	b.WriteString(fmt.Sprintf("  %s-->%s %s%s%s\n", cCyan, cReset, cGray, filename, cReset))
	b.WriteString(fmt.Sprintf("   %s|%s\n", cCyan, cReset))
	b.WriteString(fmt.Sprintf("   %s=%s %sdetail:%s %s\n", cCyan, cReset, cBold, cReset, detail))
	b.WriteString(fmt.Sprintf("   %s=%s %shelp:%s this indicates an unhandled condition; execution safely halted\n\n", cGreen, cReset, cGreen, cReset))
	return b.String()
}

// FormatCompilerError formats native compilation errors in Alca style.
func FormatCompilerError(filename string, err error, useColor bool) string {
	cRed := colorBoldRed
	cCyan := colorBoldCyan
	cGray := colorGray
	cGreen := colorBoldGreen
	cReset := colorReset
	cBold := colorBold

	if !useColor {
		cRed, cCyan, cGray, cGreen, cReset, cBold = "", "", "", "", "", ""
	}

	cleanMsg := CleanErrorMessage(err)
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("%serror[BuildError]: %s%scompilation to native executable failed\n", cRed, cReset, cBold))
	b.WriteString(fmt.Sprintf("  %s-->%s %s%s%s\n", cCyan, cReset, cGray, filename, cReset))
	b.WriteString(fmt.Sprintf("   %s|%s\n", cCyan, cReset))
	b.WriteString(fmt.Sprintf("   %s=%s %scause:%s %s\n", cCyan, cReset, cBold, cReset, cleanMsg))
	b.WriteString(fmt.Sprintf("   %s=%s %shelp:%s ensure the target output path is writable and disk space is available\n\n", cGreen, cReset, cGreen, cReset))
	return b.String()
}

func generateRuntimeSuggestion(msg string) string {
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "division by zero"):
		return "Check denominator value before dividing; division by zero is prohibited in Alca."
	case strings.Contains(lower, "undefined identifier"):
		return "Variable or function is not defined in current scope. Check spelling or declare it before use."
	case strings.Contains(lower, "index out of bounds") || strings.Contains(lower, "index out of range"):
		return "Index exceeds list length. Verify list bounds using len() before indexing."
	case strings.Contains(lower, "null pointer") || strings.Contains(lower, "access member on null"):
		return "Attempted to access field or call method on 'null'. Check if object was properly initialized."
	case strings.Contains(lower, "permission denied") || strings.Contains(lower, "access denied"):
		return "Operation was rejected by OS security policy. Verify file/resource permissions."
	case strings.Contains(lower, "stack overflow"):
		return "Recursion depth exceeded. Ensure recursive functions have a terminating base case."
	default:
		return "Review the logic on the referenced line."
	}
}
