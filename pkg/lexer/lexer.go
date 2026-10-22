// Package lexer implements the lexical scanner for the Alca programming language.
// It tokenizes source code into a stream of tokens, recognizing keywords, operators,
// literals, identifiers, comments, and Python-like indentation blocks.
package lexer

import (
	"strings"

	"github.com/alca-lang/alca/pkg/token"
)

// Lexer represents the state of lexical scanning for Alca source code.
type Lexer struct {
	input         string
	pos           int           // current position in input (points to current char)
	readPos       int           // next position to read
	ch            byte          // current character under examination
	line          int           // current line number (1-based)
	column        int           // current column number (1-based)
	indentStack   []int         // active indentation level stack
	pendingTokens []token.Token // buffered tokens waiting to be emitted (e.g. multiple DEDENTs)
	atLineStart   bool          // whether the lexer is currently at the beginning of a line
}

// New creates and initializes a new Lexer instance for the provided Alca source code.
func New(input string) *Lexer {
	// Skip a UTF-8 byte order mark: editors on Windows add it by default and
	// it is not part of the program.
	input = strings.TrimPrefix(input, "\ufeff")
	l := &Lexer{
		input:         input,
		line:          1,
		column:        0,
		indentStack:   []int{0},
		pendingTokens: make([]token.Token, 0),
		atLineStart:   true,
	}
	l.readChar()
	return l
}

// Tokenize reads all tokens from the input source until EOF and returns them as a slice.
func (l *Lexer) Tokenize() []token.Token {
	var tokens []token.Token
	for {
		tok := l.NextToken()
		tokens = append(tokens, tok)
		if tok.Type == token.EOF {
			break
		}
	}
	return tokens
}

// NextToken scans and returns the next token from the source code.
func (l *Lexer) NextToken() token.Token {
	// 1. If we have pending tokens buffered (e.g., multiple DEDENTs), return the next one.
	if len(l.pendingTokens) > 0 {
		tok := l.pendingTokens[0]
		l.pendingTokens = l.pendingTokens[1:]
		return tok
	}

	// 2. If at the start of a line, handle leading whitespace and compute indentation.
	for l.atLineStart {
		spaces := 0
		tabs := 0

		// Count leading spaces and tabs
		for l.ch == ' ' || l.ch == '\t' {
			if l.ch == ' ' {
				spaces++
			} else {
				tabs++
			}
			l.readChar()
		}

		// Empty line (carriage return / newline): skip and remain at line start
		if l.ch == '\r' || l.ch == '\n' {
			l.skipNewline()
			continue
		}

		// Single-line comment: skip line and remain at line start
		if l.ch == '/' && l.peekChar() == '/' {
			for l.ch != '\n' && l.ch != '\r' && l.ch != 0 {
				l.readChar()
			}
			if l.ch == '\r' || l.ch == '\n' {
				l.skipNewline()
			}
			continue
		}

		// Block comment at line start
		if l.ch == '/' && l.peekChar() == '*' {
			l.skipBlockComment()
			// If after block comment the line has only whitespace before newline, next iteration handles it
			continue
		}

		// End of file at line start: unwind all remaining indentation levels
		if l.ch == 0 {
			if len(l.indentStack) > 1 {
				for len(l.indentStack) > 1 {
					l.indentStack = l.indentStack[:len(l.indentStack)-1]
					l.pendingTokens = append(l.pendingTokens, token.Token{
						Type:    token.DEDENT,
						Literal: "DEDENT",
						Line:    l.line,
						Column:  l.column,
					})
				}
				l.pendingTokens = append(l.pendingTokens, token.Token{
					Type:    token.EOF,
					Literal: "",
					Line:    l.line,
					Column:  l.column,
				})
				tok := l.pendingTokens[0]
				l.pendingTokens = l.pendingTokens[1:]
				return tok
			}
			return token.Token{Type: token.EOF, Literal: "", Line: l.line, Column: l.column}
		}

		// Significant code line encountered: compute indentation level
		// Rule: 4 spaces = 1 indent level, 1 tab = 1 indent level
		currentIndent := tabs + (spaces / 4)
		topIndent := l.indentStack[len(l.indentStack)-1]

		if currentIndent > topIndent {
			l.indentStack = append(l.indentStack, currentIndent)
			l.atLineStart = false
			return token.Token{
				Type:    token.INDENT,
				Literal: "INDENT",
				Line:    l.line,
				Column:  1,
			}
		} else if currentIndent < topIndent {
			// Emit DEDENT token(s) for each level decreased
			for len(l.indentStack) > 1 && l.indentStack[len(l.indentStack)-1] > currentIndent {
				l.indentStack = l.indentStack[:len(l.indentStack)-1]
				l.pendingTokens = append(l.pendingTokens, token.Token{
					Type:    token.DEDENT,
					Literal: "DEDENT",
					Line:    l.line,
					Column:  1,
				})
			}
			l.atLineStart = false
			tok := l.pendingTokens[0]
			l.pendingTokens = l.pendingTokens[1:]
			return tok
		} else {
			// Indentation unchanged
			l.atLineStart = false
			break
		}
	}

	// 3. Skip horizontal whitespace within a line
	l.skipWhitespace()

	var tok token.Token
	line := l.line
	col := l.column

	switch l.ch {
	case '\r', '\n':
		tok = token.Token{Type: token.NEWLINE, Literal: "\n", Line: line, Column: col}
		l.skipNewline()
		l.atLineStart = true
		return tok

	case '=':
		if l.peekChar() == '=' {
			if l.peekCharN(2) == '=' {
				tok = token.Token{Type: token.STRICT_EQ, Literal: "===", Line: line, Column: col}
				l.readChar()
				l.readChar()
				l.readChar()
			} else {
				tok = token.Token{Type: token.EQ, Literal: "==", Line: line, Column: col}
				l.readChar()
				l.readChar()
			}
		} else {
			tok = token.Token{Type: token.ASSIGN, Literal: "=", Line: line, Column: col}
			l.readChar()
		}

	case '+':
		tok = token.Token{Type: token.PLUS, Literal: "+", Line: line, Column: col}
		l.readChar()

	case '-':
		if l.peekChar() == '>' {
			tok = token.Token{Type: token.ARROW, Literal: "->", Line: line, Column: col}
			l.readChar()
			l.readChar()
		} else {
			tok = token.Token{Type: token.MINUS, Literal: "-", Line: line, Column: col}
			l.readChar()
		}

	case '!':
		if l.peekChar() == '=' {
			tok = token.Token{Type: token.NOT_EQ, Literal: "!=", Line: line, Column: col}
			l.readChar()
			l.readChar()
		} else {
			tok = token.Token{Type: token.BANG, Literal: "!", Line: line, Column: col}
			l.readChar()
		}

	case '*':
		if l.peekChar() == '*' {
			tok = token.Token{Type: token.POWER, Literal: "**", Line: line, Column: col}
			l.readChar()
			l.readChar()
		} else {
			tok = token.Token{Type: token.ASTERISK, Literal: "*", Line: line, Column: col}
			l.readChar()
		}

	case '/':
		if l.peekChar() == '/' {
			l.skipSingleLineComment()
			return l.NextToken()
		} else if l.peekChar() == '*' {
			l.skipBlockComment()
			return l.NextToken()
		} else {
			tok = token.Token{Type: token.SLASH, Literal: "/", Line: line, Column: col}
			l.readChar()
		}

	case '%':
		tok = token.Token{Type: token.PERCENT, Literal: "%", Line: line, Column: col}
		l.readChar()

	case '<':
		if l.peekChar() == '<' {
			tok = token.Token{Type: token.STREAM_IN, Literal: "<<", Line: line, Column: col}
			l.readChar()
			l.readChar()
		} else if l.peekChar() == '=' {
			tok = token.Token{Type: token.LT_EQ, Literal: "<=", Line: line, Column: col}
			l.readChar()
			l.readChar()
		} else {
			tok = token.Token{Type: token.LT, Literal: "<", Line: line, Column: col}
			l.readChar()
		}

	case '>':
		if l.peekChar() == '>' && l.peekCharN(2) == '>' {
			tok = token.Token{Type: token.STREAM_VAR, Literal: ">>>", Line: line, Column: col}
			l.readChar()
			l.readChar()
			l.readChar()
		} else if l.peekChar() == '>' {
			tok = token.Token{Type: token.STREAM_OUT, Literal: ">>", Line: line, Column: col}
			l.readChar()
			l.readChar()
		} else if l.peekChar() == '!' {
			tok = token.Token{Type: token.STREAM_OVERWRITE, Literal: ">!", Line: line, Column: col}
			l.readChar()
			l.readChar()
		} else if l.peekChar() == '=' {
			tok = token.Token{Type: token.GT_EQ, Literal: ">=", Line: line, Column: col}
			l.readChar()
			l.readChar()
		} else {
			tok = token.Token{Type: token.GT, Literal: ">", Line: line, Column: col}
			l.readChar()
		}

	case '&':
		if l.peekChar() == '&' && l.peekCharN(2) == '>' && l.peekCharN(3) == '>' {
			tok = token.Token{Type: token.STREAM_THREAD, Literal: "&&>>", Line: line, Column: col}
			l.readChar()
			l.readChar()
			l.readChar()
			l.readChar()
		} else if l.peekChar() == '>' && l.peekCharN(2) == '>' {
			tok = token.Token{Type: token.STREAM_ASYNC, Literal: "&>>", Line: line, Column: col}
			l.readChar()
			l.readChar()
			l.readChar()
		} else {
			tok = token.Token{Type: token.ILLEGAL, Literal: string(l.ch), Line: line, Column: col}
			l.readChar()
		}

	case '?':
		if l.peekChar() == '>' && l.peekCharN(2) == '>' && l.peekCharN(3) == '>' {
			tok = token.Token{Type: token.STREAM_ERR_VAR, Literal: "?>>>", Line: line, Column: col}
			l.readChar()
			l.readChar()
			l.readChar()
			l.readChar()
		} else if l.peekChar() == '>' && l.peekCharN(2) == '>' {
			tok = token.Token{Type: token.STREAM_ERR, Literal: "?>>", Line: line, Column: col}
			l.readChar()
			l.readChar()
			l.readChar()
		} else {
			tok = token.Token{Type: token.ILLEGAL, Literal: string(l.ch), Line: line, Column: col}
			l.readChar()
		}

	case '|':
		if l.peekChar() == '>' {
			tok = token.Token{Type: token.PIPE, Literal: "|>", Line: line, Column: col}
			l.readChar()
			l.readChar()
		} else {
			tok = token.Token{Type: token.PIPE_TYPE, Literal: "|", Line: line, Column: col}
			l.readChar()
		}

	case '~':
		if l.peekChar() == '>' {
			tok = token.Token{Type: token.TILDE_GT, Literal: "~>", Line: line, Column: col}
			l.readChar()
			l.readChar()
		} else {
			tok = token.Token{Type: token.TILDE, Literal: "~", Line: line, Column: col}
			l.readChar()
		}

	case '@':
		tok = token.Token{Type: token.AT, Literal: "@", Line: line, Column: col}
		l.readChar()

	case ',':
		tok = token.Token{Type: token.COMMA, Literal: ",", Line: line, Column: col}
		l.readChar()

	case ':':
		tok = token.Token{Type: token.COLON, Literal: ":", Line: line, Column: col}
		l.readChar()

	case ';':
		tok = token.Token{Type: token.SEMICOLON, Literal: ";", Line: line, Column: col}
		l.readChar()

	case '.':
		if l.peekChar() == '.' {
			tok = token.Token{Type: token.RANGE, Literal: "..", Line: line, Column: col}
			l.readChar()
			l.readChar()
		} else if isDigit(l.peekChar()) {
			tok.Line = line
			tok.Column = col
			tok.Literal, tok.Type = l.readFloatStartingWithDot()
			return tok
		} else {
			tok = token.Token{Type: token.DOT, Literal: ".", Line: line, Column: col}
			l.readChar()
		}

	case '(':
		tok = token.Token{Type: token.LPAREN, Literal: "(", Line: line, Column: col}
		l.readChar()

	case ')':
		tok = token.Token{Type: token.RPAREN, Literal: ")", Line: line, Column: col}
		l.readChar()

	case '[':
		tok = token.Token{Type: token.LBRACKET, Literal: "[", Line: line, Column: col}
		l.readChar()

	case ']':
		tok = token.Token{Type: token.RBRACKET, Literal: "]", Line: line, Column: col}
		l.readChar()

	case '{':
		tok = token.Token{Type: token.LBRACE, Literal: "{", Line: line, Column: col}
		l.readChar()

	case '}':
		tok = token.Token{Type: token.RBRACE, Literal: "}", Line: line, Column: col}
		l.readChar()

	case '"':
		tok.Line = line
		tok.Column = col
		tok.Type = token.STRING
		if l.peekChar() == '"' && l.peekCharN(2) == '"' {
			tok.Literal = l.readTripleString('"', false)
		} else {
			tok.Literal = l.readString('"')
		}
		return tok

	case '\'':
		tok.Line = line
		tok.Column = col
		tok.Type = token.STRING
		if l.peekChar() == '\'' && l.peekCharN(2) == '\'' {
			tok.Literal = l.readTripleString('\'', false)
		} else {
			tok.Literal = l.readString('\'')
		}
		return tok

	case '`':
		tok.Line = line
		tok.Column = col
		tok.Type = token.STRING
		tok.Literal = l.readRawString()
		return tok

	case 'r', 'R':
		if (l.peekChar() == 'f' || l.peekChar() == 'F') && (l.peekCharN(2) == '"' || l.peekCharN(2) == '\'') {
			tok.Line = line
			tok.Column = col
			tok.Type = token.FSTRING
			l.readChar() // consume 'r'/'R'
			l.readChar() // consume 'f'/'F', l.ch is now quote
			quote := l.ch
			if l.peekChar() == quote && l.peekCharN(2) == quote {
				tok.Literal = l.readTripleString(quote, true)
			} else {
				tok.Literal = l.readRawStringQuote(quote)
			}
			return tok
		} else if l.peekChar() == '"' || l.peekChar() == '\'' {
			tok.Line = line
			tok.Column = col
			tok.Type = token.STRING
			quote := l.peekChar()
			l.readChar() // consume 'r'/'R', l.ch is now quote
			if l.peekChar() == quote && l.peekCharN(2) == quote {
				tok.Literal = l.readTripleString(quote, true)
			} else {
				tok.Literal = l.readRawStringQuote(quote)
			}
			return tok
		}
		// Not a string prefix; treat as standard identifier
		tok.Line = line
		tok.Column = col
		tok.Literal = l.readIdentifier()
		tok.Type = token.LookupIdent(tok.Literal)
		return tok

	case 'f', 'F':
		if (l.peekChar() == 'r' || l.peekChar() == 'R') && (l.peekCharN(2) == '"' || l.peekCharN(2) == '\'') {
			tok.Line = line
			tok.Column = col
			tok.Type = token.FSTRING
			l.readChar() // consume 'f'/'F'
			l.readChar() // consume 'r'/'R', l.ch is now quote
			quote := l.ch
			if l.peekChar() == quote && l.peekCharN(2) == quote {
				tok.Literal = l.readTripleString(quote, true)
			} else {
				tok.Literal = l.readRawStringQuote(quote)
			}
			return tok
		} else if l.peekChar() == '"' || l.peekChar() == '\'' {
			tok.Line = line
			tok.Column = col
			tok.Type = token.FSTRING
			quote := l.peekChar()
			l.readChar() // consume 'f'/'F', l.ch is now quote
			if l.peekChar() == quote && l.peekCharN(2) == quote {
				tok.Literal = l.readTripleString(quote, false)
			} else {
				tok.Literal = l.readString(quote)
			}
			return tok
		}
		// Not a string prefix; treat as standard identifier
		tok.Line = line
		tok.Column = col
		tok.Literal = l.readIdentifier()
		tok.Type = token.LookupIdent(tok.Literal)
		return tok

	case 0:
		// Check if there are remaining indents to dedent
		if len(l.indentStack) > 1 {
			for len(l.indentStack) > 1 {
				l.indentStack = l.indentStack[:len(l.indentStack)-1]
				l.pendingTokens = append(l.pendingTokens, token.Token{
					Type:    token.DEDENT,
					Literal: "DEDENT",
					Line:    line,
					Column:  col,
				})
			}
			l.pendingTokens = append(l.pendingTokens, token.Token{
				Type:    token.EOF,
				Literal: "",
				Line:    line,
				Column:  col,
			})
			tok = l.pendingTokens[0]
			l.pendingTokens = l.pendingTokens[1:]
			return tok
		}
		tok = token.Token{Type: token.EOF, Literal: "", Line: line, Column: col}

	default:
		if isLetter(l.ch) {
			tok.Line = line
			tok.Column = col
			tok.Literal = l.readIdentifier()
			tok.Type = token.LookupIdent(tok.Literal)
			return tok
		} else if isDigit(l.ch) {
			tok.Line = line
			tok.Column = col
			tok.Literal, tok.Type = l.readNumber()
			return tok
		} else {
			tok = token.Token{Type: token.ILLEGAL, Literal: string(l.ch), Line: line, Column: col}
			l.readChar()
		}
	}

	return tok
}

// readChar advances to the next character in input.
func (l *Lexer) readChar() {
	if l.readPos >= len(l.input) {
		l.ch = 0
	} else {
		l.ch = l.input[l.readPos]
	}
	l.pos = l.readPos
	l.readPos++
	l.column++
}

// peekChar returns the next character without advancing.
func (l *Lexer) peekChar() byte {
	if l.readPos >= len(l.input) {
		return 0
	}
	return l.input[l.readPos]
}

// peekCharN returns the character n positions ahead of current pos without advancing.
func (l *Lexer) peekCharN(n int) byte {
	target := l.pos + n
	if target >= len(l.input) || target < 0 {
		return 0
	}
	return l.input[target]
}

// skipWhitespace skips horizontal spaces and tabs.
func (l *Lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\t' {
		l.readChar()
	}
}

// skipNewline consumes carriage returns and newlines, updating line and column counters.
func (l *Lexer) skipNewline() {
	if l.ch == '\r' {
		if l.peekChar() == '\n' {
			l.readChar()
		}
	}
	l.line++
	l.column = 0
	l.readChar()
}

// skipSingleLineComment skips characters until newline or end of file.
func (l *Lexer) skipSingleLineComment() {
	for l.ch != '\n' && l.ch != '\r' && l.ch != 0 {
		l.readChar()
	}
}

// skipBlockComment skips characters until '*/' or end of file.
func (l *Lexer) skipBlockComment() {
	// Advance past '/' and '*'
	l.readChar()
	l.readChar()
	for {
		if l.ch == 0 {
			break
		}
		if l.ch == '*' && l.peekChar() == '/' {
			l.readChar() // consume '*'
			l.readChar() // consume '/'
			break
		}
		if l.ch == '\r' {
			if l.peekChar() == '\n' {
				l.readChar()
			}
			l.line++
			l.column = 0
			l.readChar()
		} else if l.ch == '\n' {
			l.line++
			l.column = 0
			l.readChar()
		} else {
			l.readChar()
		}
	}
}

// readIdentifier reads an identifier sequence (letters, digits, underscores).
func (l *Lexer) readIdentifier() string {
	startPos := l.pos
	for isLetter(l.ch) || isDigit(l.ch) {
		l.readChar()
	}
	return l.input[startPos:l.pos]
}

// readNumber reads integer or float literals, including duration and size suffixes.
func (l *Lexer) readNumber() (string, token.TokenType) {
	startPos := l.pos
	isFloat := false

	// Read integer digits
	for isDigit(l.ch) {
		l.readChar()
	}

	// Check for decimal point: must be '.' followed by a digit, not '..' (range)
	if l.ch == '.' && isDigit(l.peekChar()) {
		isFloat = true
		l.readChar() // consume '.'
		for isDigit(l.ch) {
			l.readChar()
		}
	}

	// Check duration and size suffixes (longest match first)
	// Duration: ms, s, m, h
	// Size: KB, MB, GB (case-insensitive)
	if l.ch == 'm' && l.peekChar() == 's' && !isLetterOrDigit(l.peekCharN(2)) {
		l.readChar() // consume 'm'
		l.readChar() // consume 's'
	} else if (l.ch == 'K' || l.ch == 'k') && (l.peekChar() == 'B' || l.peekChar() == 'b') && !isLetterOrDigit(l.peekCharN(2)) {
		l.readChar() // consume 'K'/'k'
		l.readChar() // consume 'B'/'b'
	} else if (l.ch == 'M' || l.ch == 'm') && (l.peekChar() == 'B' || l.peekChar() == 'b') && !isLetterOrDigit(l.peekCharN(2)) {
		l.readChar() // consume 'M'/'m'
		l.readChar() // consume 'B'/'b'
	} else if (l.ch == 'G' || l.ch == 'g') && (l.peekChar() == 'B' || l.peekChar() == 'b') && !isLetterOrDigit(l.peekCharN(2)) {
		l.readChar() // consume 'G'/'g'
		l.readChar() // consume 'B'/'b'
	} else if (l.ch == 's' || l.ch == 'm' || l.ch == 'h') && !isLetterOrDigit(l.peekChar()) {
		l.readChar() // consume single-char suffix 's', 'm', or 'h'
	}

	literal := l.input[startPos:l.pos]
	if isFloat {
		return literal, token.FLOAT
	}
	return literal, token.INT
}

// readFloatStartingWithDot handles floats written like '.5' or '.5s'.
func (l *Lexer) readFloatStartingWithDot() (string, token.TokenType) {
	startPos := l.pos
	l.readChar() // consume '.'
	for isDigit(l.ch) {
		l.readChar()
	}

	if l.ch == 'm' && l.peekChar() == 's' && !isLetterOrDigit(l.peekCharN(2)) {
		l.readChar()
		l.readChar()
	} else if (l.ch == 'K' || l.ch == 'k') && (l.peekChar() == 'B' || l.peekChar() == 'b') && !isLetterOrDigit(l.peekCharN(2)) {
		l.readChar()
		l.readChar()
	} else if (l.ch == 'M' || l.ch == 'm') && (l.peekChar() == 'B' || l.peekChar() == 'b') && !isLetterOrDigit(l.peekCharN(2)) {
		l.readChar()
		l.readChar()
	} else if (l.ch == 'G' || l.ch == 'g') && (l.peekChar() == 'B' || l.peekChar() == 'b') && !isLetterOrDigit(l.peekCharN(2)) {
		l.readChar()
		l.readChar()
	} else if (l.ch == 's' || l.ch == 'm' || l.ch == 'h') && !isLetterOrDigit(l.peekChar()) {
		l.readChar()
	}

	return l.input[startPos:l.pos], token.FLOAT
}

// readString parses string literals with escape sequences (\n, \t, \r, \", \\)
// and preserves ${expr} interpolation raw syntax.
func (l *Lexer) readString(quote byte) string {
	var out strings.Builder
	for {
		l.readChar()
		if l.ch == quote || l.ch == 0 {
			break
		}
		if l.ch == '\\' {
			l.readChar()
			switch l.ch {
			case 'n':
				out.WriteByte('\n')
			case 't':
				out.WriteByte('\t')
			case 'r':
				out.WriteByte('\r')
			case '"':
				out.WriteByte('"')
			case '\'':
				out.WriteByte('\'')
			case '\\':
				out.WriteByte('\\')
			case '$':
				out.WriteByte('$')
			default:
				out.WriteByte('\\')
				out.WriteByte(l.ch)
			}
		} else {
			if l.ch == '\n' {
				l.line++
				l.column = 0
			}
			out.WriteByte(l.ch)
		}
	}
	if l.ch == quote {
		l.readChar()
	}
	return out.String()
}

// readTripleString reads a multiline string enclosed in triple quotes (""" or ”').
// When isRaw is true, escape sequences are not interpreted (raw string).
func (l *Lexer) readTripleString(quote byte, isRaw bool) string {
	// Consume the three opening quotes
	l.readChar() // 1st
	l.readChar() // 2nd
	l.readChar() // 3rd, now l.ch is first character inside the triple string

	var out strings.Builder
	for {
		if l.ch == 0 {
			break
		}
		// Check for closing triple quotes
		if l.ch == quote && l.peekChar() == quote && l.peekCharN(2) == quote {
			l.readChar() // consume 1st
			l.readChar() // consume 2nd
			l.readChar() // consume 3rd, now l.ch is char after closing quotes
			break
		}

		if l.ch == '\\' {
			if isRaw {
				if l.peekChar() == quote {
					out.WriteByte('\\')
					l.readChar()
					out.WriteByte(l.ch)
					l.readChar()
					continue
				}
				out.WriteByte('\\')
				l.readChar()
				continue
			}

			l.readChar()
			switch l.ch {
			case 'n':
				out.WriteByte('\n')
			case 't':
				out.WriteByte('\t')
			case 'r':
				out.WriteByte('\r')
			case 'b':
				out.WriteByte('\b')
			case 'f':
				out.WriteByte('\f')
			case 'a':
				out.WriteByte('\a')
			case '0':
				out.WriteByte(0)
			case '"':
				out.WriteByte('"')
			case '\'':
				out.WriteByte('\'')
			case '\\':
				out.WriteByte('\\')
			case '$':
				out.WriteByte('$')
			case '\r':
				if l.peekChar() == '\n' {
					l.readChar()
				}
				l.line++
				l.column = 0
				// Line continuation: backslash before newline suppresses the newline
			case '\n':
				l.line++
				l.column = 0
				// Line continuation: backslash before newline suppresses the newline
			default:
				out.WriteByte('\\')
				out.WriteByte(l.ch)
			}
			l.readChar()
			continue
		}

		if l.ch == '\r' {
			if l.peekChar() == '\n' {
				l.readChar()
			}
			l.line++
			l.column = 0
			out.WriteByte('\n')
			l.readChar()
		} else if l.ch == '\n' {
			l.line++
			l.column = 0
			out.WriteByte('\n')
			l.readChar()
		} else {
			out.WriteByte(l.ch)
			l.readChar()
		}
	}
	return out.String()
}

// readRawStringQuote reads a raw string enclosed in quote (' or ") verbatim without escape processing.
func (l *Lexer) readRawStringQuote(quote byte) string {
	l.readChar() // consume opening quote
	var out strings.Builder
	for {
		if l.ch == quote || l.ch == 0 {
			break
		}
		if l.ch == '\\' {
			if l.peekChar() == quote {
				out.WriteByte('\\')
				l.readChar()
				out.WriteByte(l.ch)
				l.readChar()
				continue
			}
			out.WriteByte('\\')
			l.readChar()
			continue
		}
		if l.ch == '\r' {
			if l.peekChar() == '\n' {
				l.readChar()
			}
			l.line++
			l.column = 0
			out.WriteByte('\n')
			l.readChar()
		} else if l.ch == '\n' {
			l.line++
			l.column = 0
			out.WriteByte('\n')
			l.readChar()
		} else {
			out.WriteByte(l.ch)
			l.readChar()
		}
	}
	if l.ch == quote {
		l.readChar() // consume closing quote
	}
	return out.String()
}

// readRawString reads a raw string enclosed in backticks (`...`) verbatim without escape processing.
func (l *Lexer) readRawString() string {
	l.readChar() // consume opening '`'
	var out strings.Builder
	for {
		if l.ch == '`' || l.ch == 0 {
			break
		}
		if l.ch == '\r' {
			if l.peekChar() == '\n' {
				l.readChar()
			}
			l.line++
			l.column = 0
			out.WriteByte('\n')
			l.readChar()
		} else if l.ch == '\n' {
			l.line++
			l.column = 0
			out.WriteByte('\n')
			l.readChar()
		} else {
			out.WriteByte(l.ch)
			l.readChar()
		}
	}
	if l.ch == '`' {
		l.readChar() // consume closing '`'
	}
	return out.String()
}

// isLetter checks whether the byte is a valid identifier start/body character.
func isLetter(ch byte) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || ch == '_'
}

// isDigit checks whether the byte is an ASCII decimal digit.
func isDigit(ch byte) bool {
	return '0' <= ch && ch <= '9'
}

// isLetterOrDigit checks whether the byte is a letter or digit.
func isLetterOrDigit(ch byte) bool {
	return isLetter(ch) || isDigit(ch)
}
