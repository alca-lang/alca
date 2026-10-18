// Package token defines all lexical token types, keywords, operators, and literals
// for the Alca programming language.
package token

import "fmt"

// TokenType represents the classification of a lexical token in Alca.
type TokenType string

// Special token types.
const (
	// ILLEGAL represents an unrecognized or invalid character sequence.
	ILLEGAL TokenType = "ILLEGAL"
	// EOF marks the end of the input source.
	EOF TokenType = "EOF"
	// NEWLINE represents a significant statement terminator newline.
	NEWLINE TokenType = "NEWLINE"
	// INDENT represents an increase in indentation level.
	INDENT TokenType = "INDENT"
	// DEDENT represents a decrease in indentation level.
	DEDENT TokenType = "DEDENT"
	// COMMENT represents a code comment line or block.
	COMMENT TokenType = "COMMENT"
)

// Literal token types.
const (
	// IDENT represents an identifier (e.g., variable or function name).
	IDENT TokenType = "IDENT"
	// INT represents an integer literal (e.g., 42, 100ms, 5MB).
	INT TokenType = "INT"
	// FLOAT represents a floating-point literal (e.g., 3.14, 0.5s).
	FLOAT TokenType = "FLOAT"
	// STRING represents a string literal with optional interpolation.
	STRING TokenType = "STRING"
)

// Operator token types.
const (
	// PLUS represents the addition operator '+'.
	PLUS TokenType = "+"
	// MINUS represents the subtraction or negation operator '-'.
	MINUS TokenType = "-"
	// ASTERISK represents the multiplication operator '*'.
	ASTERISK TokenType = "*"
	// SLASH represents the division operator '/'.
	SLASH TokenType = "/"
	// PERCENT represents the modulo operator '%'.
	PERCENT TokenType = "%"
	// POWER represents the exponentiation operator '**'.
	POWER TokenType = "**"

	// ASSIGN represents the assignment operator '='.
	ASSIGN TokenType = "="

	// BANG represents the exclamation operator '!'.
	BANG TokenType = "!"

	// EQ represents the equality comparison operator '=='.
	EQ TokenType = "=="
	// NOT_EQ represents the inequality comparison operator '!='.
	NOT_EQ TokenType = "!="
	// LT represents the less-than comparison operator '<'.
	LT TokenType = "<"
	// GT represents the greater-than comparison operator '>'.
	GT TokenType = ">"
	// LTE represents the less-than-or-equal comparison operator '<='.
	LTE TokenType = "<="
	// LT_EQ represents the less-than-or-equal comparison operator '<='.
	LT_EQ TokenType = "<="
	// GTE represents the greater-than-or-equal comparison operator '>='.
	GTE TokenType = ">="
	// GT_EQ represents the greater-than-or-equal comparison operator '>='.
	GT_EQ TokenType = ">="

	// STREAM_APPEND represents the stream append operator '>>'.
	STREAM_APPEND TokenType = ">>"
	// STREAM_OUT represents the stream output operator '>>'.
	STREAM_OUT TokenType = ">>"
	// STREAM_OVERWRITE represents the stream overwrite operator '>!'.
	STREAM_OVERWRITE TokenType = ">!"
	// STREAM_CAPTURE represents the stream capture operator '>>>'.
	STREAM_CAPTURE TokenType = ">>>"
	// STREAM_VAR represents the stream capture operator '>>>'.
	STREAM_VAR TokenType = ">>>"
	// STREAM_IN represents the stream input operator '<<'.
	STREAM_IN TokenType = "<<"
	// STREAM_ERR_APPEND represents the stream error redirect operator '?>>'.
	STREAM_ERR_APPEND TokenType = "?>>"
	// STREAM_ERR represents the stream error redirect operator '?>>'.
	STREAM_ERR TokenType = "?>>"
	// STREAM_ERR_CAPTURE represents the stream error capture operator '?>>>'.
	STREAM_ERR_CAPTURE TokenType = "?>>>"
	// STREAM_ERR_VAR represents the stream error capture operator '?>>>'.
	STREAM_ERR_VAR TokenType = "?>>>"
	// STREAM_ASYNC represents the asynchronous stream operator '&>>'.
	STREAM_ASYNC TokenType = "&>>"
	// STREAM_THREAD represents the threaded stream operator '&&>>'.
	STREAM_THREAD TokenType = "&&>>"
	// PIPE represents the pipeline operator '|>'.
	PIPE TokenType = "|>"
	// PIPE_ARROW represents the pipeline operator '|>'.
	PIPE_ARROW TokenType = "|>"
	// ARROW represents the function return type or lambda arrow '->'.
	ARROW TokenType = "->"
	// TILDE_ARROW represents the legacy null redirect operator '~>'.
	TILDE_ARROW TokenType = "~>"
	// TILDE_GT represents the null redirect operator '~>'.
	TILDE_GT TokenType = "~>"

	// TILDE represents the timeout or async modifier '~'.
	TILDE TokenType = "~"
	// AT represents the decorator/annotation prefix '@'.
	AT TokenType = "@"
	// PIPE_TYPE represents the union type separator or bitwise pipe '|'.
	PIPE_TYPE TokenType = "|"
	// RANGE represents the range literal operator '..'.
	RANGE TokenType = ".."
	// DOTDOT represents the range literal operator '..'.
	DOTDOT TokenType = ".."
)

// Delimiter token types.
const (
	// COMMA represents the comma separator ','.
	COMMA TokenType = ","
	// COLON represents the colon delimiter ':'.
	COLON TokenType = ":"
	// DOT represents the member access delimiter '.'.
	DOT TokenType = "."
	// LPAREN represents the left parenthesis '('.
	LPAREN TokenType = "("
	// RPAREN represents the right parenthesis ')'.
	RPAREN TokenType = ")"
	// LBRACKET represents the left square bracket '['.
	LBRACKET TokenType = "["
	// RBRACKET represents the right square bracket ']'.
	RBRACKET TokenType = "]"
	// LBRACE represents the left curly brace '{'.
	LBRACE TokenType = "{"
	// RBRACE represents the right curly brace '}'.
	RBRACE TokenType = "}"
	// SEMICOLON represents the semicolon delimiter ';'.
	SEMICOLON TokenType = ";"
)

// Keyword token types.
const (
	// FUNC represents the 'func' keyword for defining functions.
	FUNC TokenType = "func"
	// SHFUNC represents the 'shfunc' keyword for shell-integrated functions.
	SHFUNC TokenType = "shfunc"
	// RETURN represents the 'return' keyword.
	RETURN TokenType = "return"
	// IF represents the 'if' keyword.
	IF TokenType = "if"
	// ELSE represents the 'else' keyword.
	ELSE TokenType = "else"
	// ELIF represents the 'elif' keyword.
	ELIF TokenType = "elif"
	// WHILE represents the 'while' keyword.
	WHILE TokenType = "while"
	// FOR represents the 'for' keyword.
	FOR TokenType = "for"
	// LOOP represents the 'loop' keyword for infinite loops.
	LOOP TokenType = "loop"
	// FOREACH represents the explicit 'foreach' keyword.
	FOREACH TokenType = "foreach"
	// IN represents the 'in' keyword for iteration.
	IN TokenType = "in"
	// BREAK represents the 'break' keyword.
	BREAK TokenType = "break"
	// CONTINUE represents the 'continue' keyword.
	CONTINUE TokenType = "continue"
	// AND represents the logical 'and' keyword.
	AND TokenType = "and"
	// OR represents the logical 'or' keyword.
	OR TokenType = "or"
	// NOT represents the logical 'not' keyword.
	NOT TokenType = "not"
	// TRUE represents the boolean 'true' keyword.
	TRUE TokenType = "true"
	// FALSE represents the boolean 'false' keyword.
	FALSE TokenType = "false"
	// NULL represents the 'null' literal keyword.
	NULL TokenType = "null"
	// STRUCT represents the 'struct' declaration keyword.
	STRUCT TokenType = "struct"
	// PUB represents the 'pub' visibility keyword.
	PUB TokenType = "pub"
	// USE represents the 'use' module import keyword.
	USE TokenType = "use"
	// AS represents the 'as' alias keyword.
	AS TokenType = "as"
	// BA represents the 'ba' background execution keyword.
	BA TokenType = "ba"
	// THREADS represents the 'threads' keyword.
	THREADS TokenType = "threads"
	// WAIT represents the 'wait' keyword for task synchronization.
	WAIT TokenType = "wait"
	// WATCH represents the 'watch' keyword for file/resource watchers.
	WATCH TokenType = "watch"
	// TEST represents the 'test' keyword for unit testing blocks.
	TEST TokenType = "test"
	// MOCK represents the 'mock' keyword for test mocking.
	MOCK TokenType = "mock"
	// ASSERT represents the 'assert' keyword for test assertions.
	ASSERT TokenType = "assert"
	// TX represents the 'tx' database transaction keyword.
	TX TokenType = "tx"
	// MATCH represents the 'match' pattern matching keyword.
	MATCH TokenType = "match"
	// CASE represents the 'case' branch keyword.
	CASE TokenType = "case"
	// ERR represents the 'err' error construction keyword.
	ERR TokenType = "err"
	// IMPORT represents the 'import' keyword.
	IMPORT TokenType = "import"
)

// Type keyword token types.
const (
	// TYPE_STRING represents the primitive type 'string'.
	TYPE_STRING TokenType = "string"
	// TYPE_INT represents the primitive type 'int'.
	TYPE_INT TokenType = "int"
	// TYPE_FLOAT represents the primitive type 'float'.
	TYPE_FLOAT TokenType = "float"
	// TYPE_BOOL represents the primitive type 'bool'.
	TYPE_BOOL TokenType = "bool"
	// TYPE_LIST represents the generic sequence type 'list'.
	TYPE_LIST TokenType = "list"
	// TYPE_MAP represents the key-value dictionary type 'map'.
	TYPE_MAP TokenType = "map"
	// TYPE_BLOB represents binary raw data type 'blob'.
	TYPE_BLOB TokenType = "blob"
	// TYPE_SECRET represents sensitive/redacted data type 'secret'.
	TYPE_SECRET TokenType = "secret"
	// TYPE_JSON represents structured JSON data type 'json'.
	TYPE_JSON TokenType = "json"
)

// Duration suffix constants parsed as part of INT/FLOAT literals.
const (
	// SuffixMS represents milliseconds duration suffix.
	SuffixMS = "ms"
	// SuffixS represents seconds duration suffix.
	SuffixS = "s"
	// SuffixM represents minutes duration suffix.
	SuffixM = "m"
	// SuffixH represents hours duration suffix.
	SuffixH = "h"
)

// Size suffix constants parsed as part of INT/FLOAT literals.
const (
	// SuffixKB represents kilobytes size suffix.
	SuffixKB = "KB"
	// SuffixMB represents megabytes size suffix.
	SuffixMB = "MB"
	// SuffixGB represents gigabytes size suffix.
	SuffixGB = "GB"
)

// Token represents a lexical token in the Alca language with position metadata.
type Token struct {
	Type    TokenType
	Literal string
	Line    int
	Column  int
}

// String returns a human-readable representation of the token for debugging.
func (t Token) String() string {
	return fmt.Sprintf("Token(%s, %q, %d:%d)", t.Type, t.Literal, t.Line, t.Column)
}

// keywords maps identifier names to their corresponding keyword TokenType.
var keywords = map[string]TokenType{
	// Keywords
	"func":     FUNC,
	"fn":       FUNC,
	"shfunc":   SHFUNC,
	"return":   RETURN,
	"if":       IF,
	"else":     ELSE,
	"elif":     ELIF,
	"while":    WHILE,
	"for":      FOR,
	"loop":     LOOP,
	"foreach":  FOREACH,
	"in":       IN,
	"break":    BREAK,
	"continue": CONTINUE,
	"and":      AND,
	"or":       OR,
	"not":      NOT,
	"true":     TRUE,
	"false":    FALSE,
	"null":     NULL,
	"struct":   STRUCT,
	"pub":      PUB,
	"use":      USE,
	"as":       AS,
	"ba":       BA,
	"threads":  THREADS,
	"wait":     WAIT,
	"watch":    WATCH,
	"test":     TEST,
	"mock":     MOCK,
	"assert":   ASSERT,
	"tx":       TX,
	"match":    MATCH,
	"case":     CASE,
	"err":      ERR,
	"import":   IMPORT,

	// Type Keywords
	"string": TYPE_STRING,
	"int":    TYPE_INT,
	"float":  TYPE_FLOAT,
	"bool":   TYPE_BOOL,
	"list":   TYPE_LIST,
	"map":    TYPE_MAP,
	"blob":   TYPE_BLOB,
	"secret": TYPE_SECRET,
	"json":   TYPE_JSON,
}

// LookupIdent checks if an identifier is a reserved keyword or type keyword.
// If it is, the corresponding TokenType is returned; otherwise, IDENT is returned.
func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return IDENT
}

// IsKeyword returns true if the token type represents a language keyword.
func IsKeyword(t TokenType) bool {
	switch t {
	case FUNC, SHFUNC, RETURN, IF, ELSE, ELIF, WHILE, FOR, LOOP, FOREACH,
		IN, BREAK, CONTINUE, AND, OR, NOT, TRUE, FALSE, NULL, STRUCT,
		PUB, USE, AS, BA, THREADS, WAIT, WATCH, TEST, MOCK, ASSERT,
		TX, MATCH, CASE, ERR, IMPORT:
		return true
	default:
		return false
	}
}

// IsTypeKeyword returns true if the token type represents a primitive or built-in type.
func IsTypeKeyword(t TokenType) bool {
	switch t {
	case TYPE_STRING, TYPE_INT, TYPE_FLOAT, TYPE_BOOL, TYPE_LIST,
		TYPE_MAP, TYPE_BLOB, TYPE_SECRET, TYPE_JSON:
		return true
	default:
		return false
	}
}
