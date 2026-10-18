// Package format implements the canonical source formatter for Alca programs.
//
// The formatter is line based: Alca requires every simple statement to be
// terminated with ';' and does not support expressions spanning several
// physical lines, so each logical line can be re-indented and re-spaced
// independently. Comments are not part of the abstract syntax tree (the lexer
// skips them), therefore the formatter works on the raw source bytes and the
// token stream instead of the AST: raw slices keep comments, string escapes
// and triple-quoted string bodies byte-for-byte identical.
//
// Format is conservative: the formatted output is re-parsed and compared with
// the original program's AST string. If the canonical spacing would change the
// program, the formatter falls back to a mode that only normalizes leading
// indentation and trailing whitespace. If even that fails, an error is
// returned and the input is left untouched by callers.
package format

import (
	"fmt"
	"strings"

	"github.com/alca-lang/alca/pkg/lexer"
	"github.com/alca-lang/alca/pkg/parser"
	"github.com/alca-lang/alca/pkg/token"
)

// SyntaxError reports that the input could not be formatted because it does
// not parse. The embedded report is ready to be printed for the user.
type SyntaxError struct {
	Report string
}

// Error returns the diagnostic report without the trailing newline.
func (e *SyntaxError) Error() string {
	return strings.TrimRight(e.Report, "\n")
}

// Format formats src using "<input>" as the diagnostic file name.
func Format(src string) (string, error) {
	return FormatSource("<input>", src)
}

// FormatSource formats src, using name in diagnostic messages. The returned
// source is guaranteed to parse to the same program as the input (identical
// AST string) and to be idempotent: formatting it again yields the same text.
func FormatSource(name, src string) (string, error) {
	l := lexer.New(src)
	p := parser.New(l)
	p.ParseProgram()
	if len(p.SyntaxErrors()) > 0 {
		return "", &SyntaxError{Report: p.DiagnosticReport(name, src, false)}
	}

	toks := lexer.New(src).Tokenize()
	for _, t := range toks {
		if t.Type == token.ILLEGAL {
			return "", &SyntaxError{Report: fmt.Sprintf("error: unexpected character %q at %s:%d:%d\n", t.Literal, name, t.Line, t.Column)}
		}
	}

	if out := render(src, toks, true); sameProgram(name, src, out) {
		return out, nil
	}
	// Canonical spacing did not round-trip (unusual construct): fall back to
	// re-indentation only, which leaves every token's raw text untouched.
	if out := render(src, toks, false); sameProgram(name, src, out) {
		return out, nil
	}
	return "", fmt.Errorf("%s: cannot format without changing the program", name)
}

// sameProgram reports whether both sources parse and yield the same AST.
func sameProgram(name, a, b string) bool {
	parse := func(s string) *astProgram {
		pa := parser.New(lexer.New(s))
		prog := pa.ParseProgram()
		if len(pa.SyntaxErrors()) > 0 {
			return nil
		}
		return &astProgram{text: prog.String()}
	}
	x, y := parse(a), parse(b)
	return x != nil && y != nil && x.text == y.text
}

// astProgram is a tiny wrapper used by sameProgram to compare parsed programs.
type astProgram struct {
	text string
}

// lineInfo describes one physical source line.
type lineInfo struct {
	start int // byte offset of the first character of the line
	end   int // byte offset just past the last character (before the newline)
}

// group is a logical line: the tokens between two statement terminators.
// Usually start == end; triple-quoted strings make a group span several lines.
type group struct {
	toks  []token.Token // all tokens of the logical line, in source order
	start int           // first physical line of the group (1-based)
	end   int           // last physical line of the group (1-based)
	depth int           // indentation depth at the start of the group
}

const indentUnit = "    " // one indentation level (4 spaces)

// render produces the formatted source. When normalize is false only leading
// indentation and trailing whitespace are touched.
func render(src string, toks []token.Token, normalize bool) string {
	eol := "\n"
	if strings.Contains(src, "\r\n") {
		eol = "\r\n"
	}

	raw := strings.Split(src, "\n")
	infos := make([]lineInfo, len(raw))
	pos := 0
	for i, line := range raw {
		line = strings.TrimRight(line, "\r")
		raw[i] = line
		infos[i] = lineInfo{start: pos, end: pos + len(line)}
		pos += len(line) + 1 // the newline byte
	}

	// Split the token stream into logical lines and record indentation depth.
	var groups []group
	lineDepth := map[int]int{}
	depth := 0
	var cur []token.Token
	flush := func() {
		if len(cur) == 0 {
			return
		}
		groups = append(groups, group{
			toks:  cur,
			start: cur[0].Line,
			end:   cur[len(cur)-1].Line,
			depth: lineDepth[cur[0].Line],
		})
		cur = nil
	}
	for _, t := range toks {
		switch t.Type {
		case token.INDENT:
			depth++
		case token.DEDENT:
			depth--
		case token.NEWLINE, token.EOF:
			flush()
		case token.COMMENT:
			// The lexer never emits comment tokens, but ignore them if one
			// ever shows up: they carry no program meaning.
		default:
			if len(cur) == 0 {
				lineDepth[t.Line] = depth
			}
			cur = append(cur, t)
		}
	}
	flush()

	// Comment-only lines are indented like the next block of code, so doc
	// comments stay attached to what they describe.
	nextDepth := make([]int, len(raw)+2)
	d := 0
	for ln := len(raw); ln >= 1; ln-- {
		if v, ok := lineDepth[ln]; ok {
			d = v
		}
		nextDepth[ln] = d
	}

	res := make([]string, 0, len(raw))
	gi := 0
	for ln := 1; ln <= len(raw); ln++ {
		if gi < len(groups) && groups[gi].start == ln {
			g := groups[gi]
			res = append(res, renderGroup(g, raw, infos, src, normalize))
			for l := ln + 1; l <= g.end; l++ {
				res = append(res, raw[l-1]) // string / comment continuation
			}
			ln = g.end
			gi++
			continue
		}
		trimmed := strings.TrimSpace(raw[ln-1])
		switch {
		case trimmed == "":
			res = append(res, "")
		case strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*"):
			res = append(res, strings.Repeat(indentUnit, nextDepth[ln])+trimmed)
		default:
			// Continuation of a block comment or string: keep verbatim.
			res = append(res, raw[ln-1])
		}
	}

	out := strings.Join(res, eol)
	if !strings.HasSuffix(out, eol) {
		out += eol
	}
	return out
}

// renderGroup renders the first physical line of a logical line. Lines after
// the first (inside a triple-quoted string) are emitted verbatim by the caller.
func renderGroup(g group, raw []string, infos []lineInfo, src string, normalize bool) string {
	indent := strings.Repeat(indentUnit, g.depth)

	// Only tokens that begin on the group's first line belong to that line;
	// tokens after a multiline string are part of the verbatim tail.
	var toks []token.Token
	for _, t := range g.toks {
		if t.Line == g.start {
			toks = append(toks, t)
		}
	}
	if len(toks) == 0 {
		return indent
	}

	ln := toks[0].Line
	lineStart := infos[ln-1].start
	lineEnd := infos[ln-1].end

	if !normalize {
		start := lineStart + toks[0].Column - 1
		return indent + strings.TrimRight(src[start:lineEnd], " \t")
	}

	roles := pipeRoles(toks)
	var sb strings.Builder
	sb.WriteString(indent)
	for i, t := range toks {
		start := lineStart + t.Column - 1
		end := lineEnd
		if i+1 < len(toks) && toks[i+1].Line == ln {
			end = lineStart + toks[i+1].Column - 1
		}
		if i > 0 && needsSpace(toks, roles, i) {
			sb.WriteString(" ")
		}
		sb.WriteString(strings.TrimRight(src[start:end], " \t\r"))
	}
	return strings.TrimRight(sb.String(), " \t")
}

// pipeRole classifies '|' tokens: union type separator, lambda opener or
// lambda closer.
type pipeRole int

const (
	pipeUnion pipeRole = iota // "int | null"
	pipeOpen                  // "|x, y|" opening pipe
	pipeClose                 // "|x, y|" closing pipe
)

// pipeRoles classifies every '|' on a line. A '|' that appears where an
// expression can start opens a lambda parameter list; the next '|' closes it.
func pipeRoles(toks []token.Token) []pipeRole {
	roles := make([]pipeRole, len(toks))
	open := false
	for i, t := range toks {
		if t.Type != token.PIPE_TYPE {
			continue
		}
		if !open && pipeStartsLambda(toks, i) {
			roles[i] = pipeOpen
			open = true
		} else if open {
			roles[i] = pipeClose
			open = false
		} else {
			roles[i] = pipeUnion
		}
	}
	return roles
}

// pipeStartsLambda reports whether the '|' at index i can begin a lambda
// parameter list, i.e. it does not follow a value.
func pipeStartsLambda(toks []token.Token, i int) bool {
	if i == 0 {
		return true
	}
	return !isValueLike(toks[i-1].Type)
}

// needsSpace reports whether a space must separate toks[i-1] and toks[i].
func needsSpace(toks []token.Token, roles []pipeRole, i int) bool {
	cur := toks[i]
	prev := toks[i-1]
	hasPrev2 := i >= 2
	var prev2 token.Token
	if hasPrev2 {
		prev2 = toks[i-2]
	}
	hasPrev3 := i >= 3
	var prev3 token.Token
	if hasPrev3 {
		prev3 = toks[i-3]
	}

	// Never a space before closing delimiters and tight punctuation.
	switch cur.Type {
	case token.RPAREN, token.RBRACKET, token.RBRACE, token.COMMA, token.SEMICOLON,
		token.DOT, token.RANGE:
		return false
	case token.COLON:
		// The ':' of an inferred declaration is lexed as COLON followed by
		// ASSIGN; ':=' is written with a space before and none inside.
		return i+1 < len(toks) && toks[i+1].Type == token.ASSIGN
	}

	// No space between the ':' and '=' of ':='.
	if prev.Type == token.COLON && cur.Type == token.ASSIGN {
		return false
	}

	// Never a space after opening delimiters and prefix operators.
	switch prev.Type {
	case token.LPAREN, token.LBRACKET, token.LBRACE, token.DOT, token.AT:
		return false
	case token.RANGE:
		return false
	case token.BANG:
		return false
	case token.MINUS:
		if unaryContext(toks, i-1) {
			return false
		}
	}

	// Keyword arguments are written tight: f(code=1). An '=' directly after
	// an identifier that follows '(' or ',' is a keyword argument; parameter
	// defaults (name: T = value) keep the spaces.
	if cur.Type == token.ASSIGN && isCallContextValue(prev, prev2, hasPrev2) {
		return false
	}
	if prev.Type == token.ASSIGN && hasPrev3 && isCallContextValue(prev2, prev3, true) {
		return false
	}

	// A space always follows ':', ',' and ';'.
	switch prev.Type {
	case token.COLON, token.COMMA, token.SEMICOLON:
		return true
	}

	// Calls and index expressions stay tight: f(x), xs[0], list[string].
	if cur.Type == token.LPAREN || cur.Type == token.LBRACKET {
		if isValueLike(prev.Type) {
			return false
		}
		if prev.Type == token.MINUS && unaryContext(toks, i-1) {
			return false
		}
		return true
	}

	// Lambda pipes are tight on the inside, union type separators are not.
	if cur.Type == token.PIPE_TYPE {
		return roles[i] != pipeClose
	}
	if prev.Type == token.PIPE_TYPE {
		return roles[i-1] != pipeOpen
	}

	return true
}

// isCallContextValue reports whether cur is an identifier directly following
// '(' or ',' on the same logical line, which makes it a keyword argument name.
func isCallContextValue(cur, before token.Token, hasBefore bool) bool {
	if !hasBefore || cur.Type != token.IDENT {
		return false
	}
	return before.Type == token.LPAREN || before.Type == token.COMMA
}

// unaryContext reports whether the token at index i is used as a prefix
// operator rather than a binary one (e.g. -1 versus a - b).
func unaryContext(toks []token.Token, i int) bool {
	if i == 0 {
		return true
	}
	switch toks[i-1].Type {
	case token.LPAREN, token.LBRACKET, token.LBRACE, token.COMMA, token.COLON,
		token.SEMICOLON, token.RETURN, token.ASSIGN, token.EQ, token.NOT_EQ,
		token.LT, token.GT, token.LT_EQ, token.GT_EQ, token.PLUS, token.MINUS,
		token.ASTERISK, token.SLASH, token.PERCENT, token.POWER, token.AND,
		token.OR, token.NOT, token.IN, token.ARROW, token.PIPE,
		token.TILDE, token.TILDE_GT, token.RANGE, token.AT,
		token.STREAM_IN, token.STREAM_OUT, token.STREAM_OVERWRITE, token.STREAM_VAR,
		token.STREAM_ERR, token.STREAM_ERR_VAR, token.STREAM_ASYNC, token.STREAM_THREAD:
		return true
	}
	return false
}

// isValueLike reports whether a token can end a value expression, so a
// following '(' or '[' binds to it without a space.
func isValueLike(t token.TokenType) bool {
	switch t {
	case token.IDENT, token.INT, token.FLOAT, token.STRING, token.TRUE, token.FALSE,
		token.NULL, token.ERR, token.RPAREN, token.RBRACKET, token.RBRACE,
		token.TYPE_STRING, token.TYPE_INT, token.TYPE_FLOAT, token.TYPE_BOOL,
		token.TYPE_LIST, token.TYPE_MAP, token.TYPE_BLOB, token.TYPE_SECRET,
		token.TYPE_JSON:
		return true
	}
	return false
}
