// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

// Package parser implements a Pratt (top-down operator precedence) parser
// for the Alca programming language.
package parser

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/alca-lang/alca/pkg/ast"
	"github.com/alca-lang/alca/pkg/diag"
	"github.com/alca-lang/alca/pkg/lexer"
	"github.com/alca-lang/alca/pkg/token"
)

// Precedence levels for Alca expressions (from lowest to highest).
const (
	_ int = iota
	LOWEST
	STREAM      // >> >! >>> << ?>> ?>>> &>> &&>>
	PIPE        // |>
	ASSIGN      // =
	OR          // or
	AND         // and
	RANGE       // ..
	EQUALS      // == !=
	LESSGREATER // < > <= >= in
	SUM         // + -
	PRODUCT     // * / %
	POWER       // **
	PREFIX      // - not !
	CALL        // function calls ()
	INDEX       // [] .
	TIMEOUT     // ~
)

// Operator precedence lookup table mapping token types to their precedence level.
var precedences = map[token.TokenType]int{
	token.STREAM_OUT:       STREAM,
	token.STREAM_OVERWRITE: STREAM,
	token.STREAM_VAR:       STREAM,
	token.STREAM_IN:        STREAM,
	token.STREAM_ERR:       STREAM,
	token.STREAM_ERR_VAR:   STREAM,
	token.STREAM_ASYNC:     STREAM,
	token.STREAM_THREAD:    STREAM,
	token.PIPE:             PIPE,
	token.ASSIGN:           ASSIGN,
	token.OR:               OR,
	token.AND:              AND,
	token.RANGE:            RANGE,
	token.DOTDOTEQ:         RANGE,
	token.EQ:               EQUALS,
	token.STRICT_EQ:        EQUALS,
	token.NOT_EQ:           EQUALS,
	token.LT:               LESSGREATER,
	token.GT:               LESSGREATER,
	token.LT_EQ:            LESSGREATER,
	token.GT_EQ:            LESSGREATER,
	token.IN:               LESSGREATER,
	token.PLUS:             SUM,
	token.MINUS:            SUM,
	token.SLASH:            PRODUCT,
	token.ASTERISK:         PRODUCT,
	token.PERCENT:          PRODUCT,
	token.POWER:            POWER,
	token.LPAREN:           CALL,
	token.LBRACKET:         INDEX,
	token.DOT:              INDEX,
	token.TILDE:            TIMEOUT,
}

type (
	prefixParseFn func() ast.Expression
	infixParseFn  func(ast.Expression) ast.Expression
)

// Parser parses tokens from the lexer into an AST for the Alca programming language.
type Parser struct {
	l            *lexer.Lexer
	curToken     token.Token
	peekToken    token.Token
	errors       []string
	syntaxErrors []*diag.SyntaxError

	prefixParseFns map[token.TokenType]prefixParseFn
	infixParseFns  map[token.TokenType]infixParseFn
}

// New creates and initializes a new Parser instance for the given lexer.
func New(l *lexer.Lexer) *Parser {
	p := &Parser{
		l:              l,
		errors:         []string{},
		syntaxErrors:   []*diag.SyntaxError{},
		prefixParseFns: make(map[token.TokenType]prefixParseFn),
		infixParseFns:  make(map[token.TokenType]infixParseFn),
	}

	// Register prefix parse functions
	p.registerPrefix(token.IDENT, p.parseIdentifier)
	p.registerPrefix(token.INT, p.parseIntegerLiteral)
	p.registerPrefix(token.FLOAT, p.parseFloatLiteral)
	p.registerPrefix(token.STRING, p.parseStringLiteral)
	p.registerPrefix(token.FSTRING, p.parseFStringLiteral)
	p.registerPrefix(token.TRUE, p.parseBooleanLiteral)
	p.registerPrefix(token.FALSE, p.parseBooleanLiteral)
	p.registerPrefix(token.NULL, p.parseNullLiteral)
	p.registerPrefix(token.BANG, p.parsePrefixExpression)
	p.registerPrefix(token.MINUS, p.parsePrefixExpression)
	p.registerPrefix(token.NOT, p.parsePrefixExpression)
	p.registerPrefix(token.LPAREN, p.parseGroupedExpression)
	p.registerPrefix(token.LBRACKET, p.parseListLiteral)
	p.registerPrefix(token.LBRACE, p.parseMapLiteral)
	p.registerPrefix(token.ERR, p.parseErrExpression)
	p.registerPrefix(token.PIPE_TYPE, p.parseLambdaExpression)
	p.registerPrefix(token.AWAIT, p.parseAwaitExpression)
	p.registerPrefix(token.RANGE, p.parsePrefixRangeLiteral)
	p.registerPrefix(token.DOTDOT, p.parsePrefixRangeLiteral)
	p.registerPrefix(token.DOTDOTEQ, p.parsePrefixRangeLiteral)

	// Built-in type keywords as prefix identifiers
	typeTokens := []token.TokenType{
		token.TYPE_STRING, token.TYPE_INT, token.TYPE_FLOAT,
		token.TYPE_BOOL, token.TYPE_LIST, token.TYPE_MAP,
		token.TYPE_BLOB, token.TYPE_SECRET, token.TYPE_JSON,
	}
	for _, tt := range typeTokens {
		p.registerPrefix(tt, p.parseTypeIdentifier)
	}

	// Register infix parse functions
	p.registerInfix(token.PLUS, p.parseInfixExpression)
	p.registerInfix(token.MINUS, p.parseInfixExpression)
	p.registerInfix(token.ASTERISK, p.parseInfixExpression)
	p.registerInfix(token.SLASH, p.parseInfixExpression)
	p.registerInfix(token.PERCENT, p.parseInfixExpression)
	p.registerInfix(token.POWER, p.parsePowerExpression)

	p.registerInfix(token.EQ, p.parseInfixExpression)
	p.registerInfix(token.STRICT_EQ, p.parseInfixExpression)
	p.registerInfix(token.NOT_EQ, p.parseInfixExpression)
	p.registerInfix(token.LT, p.parseInfixExpression)
	p.registerInfix(token.GT, p.parseInfixExpression)
	p.registerInfix(token.LT_EQ, p.parseInfixExpression)
	p.registerInfix(token.GT_EQ, p.parseInfixExpression)
	p.registerInfix(token.LTE, p.parseInfixExpression)
	p.registerInfix(token.GTE, p.parseInfixExpression)
	p.registerInfix(token.IN, p.parseInfixExpression)

	p.registerInfix(token.AND, p.parseInfixExpression)
	p.registerInfix(token.OR, p.parseInfixExpression)

	p.registerInfix(token.ASSIGN, p.parseAssignExpression)
	p.registerInfix(token.PIPE, p.parsePipeExpression)
	p.registerInfix(token.PIPE_ARROW, p.parsePipeExpression)
	p.registerInfix(token.TILDE, p.parseTimeoutExpression)

	p.registerInfix(token.STREAM_OUT, p.parseStreamExpression)
	p.registerInfix(token.STREAM_APPEND, p.parseStreamExpression)
	p.registerInfix(token.STREAM_OVERWRITE, p.parseStreamExpression)
	p.registerInfix(token.STREAM_VAR, p.parseStreamExpression)
	p.registerInfix(token.STREAM_CAPTURE, p.parseStreamExpression)
	p.registerInfix(token.STREAM_IN, p.parseStreamExpression)
	p.registerInfix(token.STREAM_ERR, p.parseStreamExpression)
	p.registerInfix(token.STREAM_ERR_APPEND, p.parseStreamExpression)
	p.registerInfix(token.STREAM_ERR_VAR, p.parseStreamExpression)
	p.registerInfix(token.STREAM_ERR_CAPTURE, p.parseStreamExpression)
	p.registerInfix(token.STREAM_ASYNC, p.parseStreamExpression)
	p.registerInfix(token.STREAM_THREAD, p.parseStreamExpression)

	p.registerInfix(token.RANGE, p.parseRangeExpression)
	p.registerInfix(token.DOTDOT, p.parseRangeExpression)
	p.registerInfix(token.DOTDOTEQ, p.parseRangeExpression)

	p.registerInfix(token.LPAREN, p.parseCallExpression)
	p.registerInfix(token.LBRACKET, p.parseIndexExpression)
	p.registerInfix(token.DOT, p.parseDotExpression)

	// Read initial tokens skipping comments
	p.curToken = p.l.NextToken()
	for p.curToken.Type == token.COMMENT {
		p.curToken = p.l.NextToken()
	}
	p.peekToken = p.l.NextToken()
	for p.peekToken.Type == token.COMMENT {
		p.peekToken = p.l.NextToken()
	}

	return p
}

// Errors returns the list of syntax and parsing error messages accumulated.
func (p *Parser) Errors() []string {
	return p.errors
}

// SyntaxErrors returns the structured syntax error diagnostics.
func (p *Parser) SyntaxErrors() []*diag.SyntaxError {
	return p.syntaxErrors
}

// DiagnosticReport formats a beautiful, colored diagnostic report with source context and hints.
func (p *Parser) DiagnosticReport(filename string, source string, useColor bool) string {
	return diag.FormatReport(filename, source, p.syntaxErrors, useColor)
}

// ParseProgram parses the entire token stream and returns the root AST Program node.
func (p *Parser) ParseProgram() *ast.Program {
	program := &ast.Program{
		Statements: []ast.Statement{},
	}

	p.skipNewlines()

	for !p.curTokenIs(token.EOF) {
		prevTok := p.curToken
		stmt := p.parseStatement()
		if stmt != nil {
			program.Statements = append(program.Statements, stmt)
		} else {
			p.nextToken()
		}
		p.skipNewlines()
		if p.curToken == prevTok {
			p.nextToken()
			p.skipNewlines()
		}
	}

	return program
}

// -----------------------------------------------------------------------------
// Token Navigation and Helpers
// -----------------------------------------------------------------------------

func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()
	for p.peekToken.Type == token.COMMENT {
		p.peekToken = p.l.NextToken()
	}
}

func (p *Parser) curTokenIs(t token.TokenType) bool {
	return p.curToken.Type == t
}

func (p *Parser) peekTokenIs(t token.TokenType) bool {
	return p.peekToken.Type == t
}

func (p *Parser) expectPeek(t token.TokenType) bool {
	if p.peekTokenIs(t) {
		p.nextToken()
		return true
	}
	p.peekError(t)
	return false
}

func (p *Parser) peekError(t token.TokenType) {
	tok := p.peekToken
	length := len(tok.Literal)
	if length <= 0 {
		length = 1
	}

	var msg, suggestion string
	switch t {
	case token.RPAREN:
		msg = fmt.Sprintf("expected ')' to close parenthesis, got %q (%s)", tok.Literal, tok.Type)
		suggestion = "did you forget a closing ')'?"
	case token.RBRACKET:
		msg = fmt.Sprintf("expected ']' to close list or index, got %q (%s)", tok.Literal, tok.Type)
		suggestion = "did you forget a closing ']'?"
	case token.RBRACE:
		msg = fmt.Sprintf("expected '}' to close block or map, got %q (%s)", tok.Literal, tok.Type)
		suggestion = "did you forget a closing '}'?"
	case token.COLON:
		msg = fmt.Sprintf("expected ':' before block or in type annotation, got %q (%s)", tok.Literal, tok.Type)
		suggestion = "block headers and type annotations require ':'"
	case token.ASSIGN:
		msg = fmt.Sprintf("expected '=' after declaration, got %q (%s)", tok.Literal, tok.Type)
		suggestion = "variable declarations require an initial value: 'x: type = value;' or 'x := value;'"
	case token.IN:
		msg = fmt.Sprintf("expected 'in' in for-loop, got %q (%s)", tok.Literal, tok.Type)
		suggestion = "for-loops use the syntax 'for <item> in <iterable>:'"
	case token.LPAREN:
		msg = fmt.Sprintf("expected '(' after function or call, got %q (%s)", tok.Literal, tok.Type)
		suggestion = "enclose arguments or parameter lists in parentheses '(...)'"
	default:
		msg = fmt.Sprintf("expected next token to be %s, got %q (%s)", t, tok.Literal, tok.Type)
		suggestion = fmt.Sprintf("expected '%s' here", t)
	}

	p.errors = append(p.errors, fmt.Sprintf("line %d, col %d: %s", tok.Line, tok.Column, msg))
	p.syntaxErrors = append(p.syntaxErrors, &diag.SyntaxError{
		Filename:   "",
		Line:       tok.Line,
		Column:     tok.Column,
		Message:    msg,
		Token:      string(tok.Type),
		Length:     length,
		Suggestion: suggestion,
	})
}

// expectBlockColon checks for a trailing colon before an indented block.
// If the colon is missing, it reports an exact syntax error with a helpful suggestion,
// and gracefully allows block parsing to continue without cascading into spurious indent errors.
func (p *Parser) expectBlockColon(contextName string) bool {
	if p.peekTokenIs(token.COLON) {
		p.nextToken()
		return true
	}
	if p.curTokenIs(token.COLON) {
		return true
	}
	if p.peekTokenIs(token.LBRACE) || p.curTokenIs(token.LBRACE) {
		return true
	}

	col := p.curToken.Column + len(p.curToken.Literal)
	p.errors = append(p.errors, fmt.Sprintf("line %d, col %d: expected ':' after %s", p.curToken.Line, col, contextName))
	p.syntaxErrors = append(p.syntaxErrors, &diag.SyntaxError{
		Filename:   "",
		Line:       p.curToken.Line,
		Column:     col,
		Message:    fmt.Sprintf("expected ':' after %s", contextName),
		Token:      p.curToken.Literal,
		Length:     1,
		Suggestion: fmt.Sprintf("add ':' after '%s' to begin the indented block", contextName),
	})
	return false
}

func (p *Parser) noPrefixParseFnError(t token.TokenType) {
	tok := p.curToken
	length := len(tok.Literal)
	if length <= 0 {
		length = 1
	}

	var msg, suggestion string

	switch t {
	case token.RPAREN:
		msg = "unexpected closing ')' without matching '('"
		suggestion = "check for an unclosed '(' earlier, or remove this extra ')'"
	case token.RBRACKET:
		msg = "unexpected closing ']' without matching '['"
		suggestion = "check for an unclosed '[' earlier, or remove this extra ']'"
	case token.RBRACE:
		msg = "unexpected closing '}' without matching '{'"
		suggestion = "check for an unclosed '{' earlier, or remove this extra '}'"
	case token.COLON:
		msg = "unexpected ':' outside of a type annotation or block header"
		suggestion = "use ':' only at the end of block headers ('if cond:', 'func foo():') or in variable types ('x: int = 5;')"
	case token.SEMICOLON:
		msg = "unexpected ';' (empty statement)"
		suggestion = "remove the stray semicolon or provide an expression before ';'"
	case token.ASSIGN:
		msg = "unexpected '=' assignment operator"
		suggestion = "check if you meant ':=' for declaration or '==' for equality comparison"
	case token.COMMA:
		msg = "unexpected ',' outside list, map, or argument list"
		suggestion = "remove the extra comma or enclose the items in brackets or parentheses"
	case token.PLUS, token.ASTERISK, token.SLASH, token.PERCENT, token.POWER:
		msg = fmt.Sprintf("stray arithmetic operator %q without left-hand operand", tok.Literal)
		suggestion = fmt.Sprintf("provide an expression to the left of '%s'", tok.Literal)
	case token.EQ, token.NOT_EQ, token.LT, token.LT_EQ, token.GT, token.GT_EQ:
		msg = fmt.Sprintf("comparison operator %q without left-hand operand", tok.Literal)
		suggestion = fmt.Sprintf("comparison operators require a left-hand expression (e.g. 'x %s y')", tok.Literal)
	case token.AND, token.OR:
		msg = fmt.Sprintf("logical operator %q without left-hand operand", tok.Literal)
		suggestion = fmt.Sprintf("provide a boolean expression to the left of '%s'", tok.Literal)
	case token.PIPE:
		msg = "pipeline operator '|>' without left-hand value"
		suggestion = "pipeline '|>' takes a value on the left and passes it to the function on the right (e.g. 'val |> func')"
	case token.ARROW:
		msg = "unexpected '->'"
		suggestion = "'->' is used in function return type signatures (e.g. 'func foo() -> int:')"
	case token.ELSE, token.ELIF:
		msg = fmt.Sprintf("'%s' clause without a preceding 'if'", tok.Literal)
		suggestion = fmt.Sprintf("ensure this '%s' matches an 'if' at the same indentation level", tok.Literal)
	case token.CASE:
		msg = "'case' clause outside of a 'match' statement"
		suggestion = "'case' must be placed inside an indented 'match' block"
	case token.IN:
		msg = "unexpected 'in' keyword"
		suggestion = "'in' is used in 'for item in collection:' or membership check 'item in list'"
	case token.INDENT:
		msg = "unexpected indentation"
		suggestion = "indentation is only valid inside block bodies (following ':')"
	case token.DEDENT:
		msg = "unaligned dedent"
		suggestion = "check block indentation: indentation decreased to an invalid level"
	case token.ILLEGAL:
		msg = fmt.Sprintf("illegal character %q", tok.Literal)
		suggestion = "remove or replace the invalid character"
	default:
		msg = fmt.Sprintf("unexpected token %q (%s)", tok.Literal, t)
		suggestion = "check the syntax around this token"
	}

	p.errors = append(p.errors, fmt.Sprintf("line %d, col %d: %s", tok.Line, tok.Column, msg))
	p.syntaxErrors = append(p.syntaxErrors, &diag.SyntaxError{
		Filename:   "",
		Line:       tok.Line,
		Column:     tok.Column,
		Message:    msg,
		Token:      string(t),
		Length:     length,
		Suggestion: suggestion,
	})
}

func (p *Parser) peekPrecedence() int {
	if prec, ok := precedences[p.peekToken.Type]; ok {
		return prec
	}
	return LOWEST
}

func (p *Parser) curPrecedence() int {
	if prec, ok := precedences[p.curToken.Type]; ok {
		return prec
	}
	return LOWEST
}

func (p *Parser) registerPrefix(tokenType token.TokenType, fn prefixParseFn) {
	p.prefixParseFns[tokenType] = fn
}

func (p *Parser) registerInfix(tokenType token.TokenType, fn infixParseFn) {
	p.infixParseFns[tokenType] = fn
}

func (p *Parser) skipNewlines() {
	for p.curTokenIs(token.NEWLINE) {
		p.nextToken()
	}
}

func (p *Parser) skipPeekNewlines() {
	for p.peekTokenIs(token.NEWLINE) {
		p.nextToken()
	}
}

// consumeStatementEnd finishes a simple statement. Every simple statement
// (declaration, assignment, expression, return, break, continue, use, assert, ...)
// MUST end with ';'. Block statements (if/while/for/func/struct/...) end with their
// indented block and take no ';'.
//
// On entry the current token is the last token of the statement. On exit the current
// token is whatever follows the ';' (a NEWLINE, the next statement, DEDENT or EOF).
func (p *Parser) consumeStatementEnd() {
	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken() // cur = ';'
		p.nextToken() // cur = token after ';'
		return
	}

	// Missing ';' : report at the end of the statement's last token and recover
	// by skipping the remainder of the line.
	end := p.curToken
	length := len(end.Literal)
	if end.Type == token.STRING {
		length += 2 // surrounding quotes are not part of the literal
	}
	col := end.Column + length
	p.errors = append(p.errors, fmt.Sprintf("line %d, col %d: expected ';' at end of statement", end.Line, col))
	p.syntaxErrors = append(p.syntaxErrors, &diag.SyntaxError{
		Line:       end.Line,
		Column:     col,
		Message:    "expected ';' at end of statement",
		Token:      string(p.peekToken.Type),
		Length:     1,
		Suggestion: "every statement must end with ';'",
	})
	for !p.peekTokenIs(token.NEWLINE) && !p.peekTokenIs(token.EOF) && !p.peekTokenIs(token.DEDENT) {
		p.nextToken()
	}
	p.nextToken() // cur = NEWLINE / EOF / DEDENT
}

// -----------------------------------------------------------------------------
// Statement Parsing
// -----------------------------------------------------------------------------

func isNilStatement(s ast.Statement) bool {
	if s == nil {
		return true
	}
	switch v := s.(type) {
	case *ast.FuncDeclaration:
		return v == nil
	case *ast.StructDeclaration:
		return v == nil
	case *ast.EnumDeclaration:
		return v == nil
	case *ast.InterfaceDeclaration:
		return v == nil
	case *ast.ClassDeclaration:
		return v == nil
	case *ast.IfStatement:
		return v == nil
	case *ast.WhileStatement:
		return v == nil
	case *ast.ForStatement:
		return v == nil
	case *ast.ForEachStatement:
		return v == nil
	case *ast.LoopStatement:
		return v == nil
	case *ast.ReturnStatement:
		return v == nil
	case *ast.BreakStatement:
		return v == nil
	case *ast.ContinueStatement:
		return v == nil
	case *ast.UseStatement:
		return v == nil
	case *ast.WatchStatement:
		return v == nil
	case *ast.TestStatement:
		return v == nil
	case *ast.MockStatement:
		return v == nil
	case *ast.ThreadsStatement:
		return v == nil
	case *ast.TxStatement:
		return v == nil
	case *ast.WaitStatement:
		return v == nil
	case *ast.AssertStatement:
		return v == nil
	case *ast.MatchStatement:
		return v == nil
	case *ast.VarDeclaration:
		return v == nil
	case *ast.ExpressionStatement:
		return v == nil
	case *ast.BlockStatement:
		return v == nil
	}
	return false
}

func (p *Parser) parseStatement() ast.Statement {
	stmt := p.parseStatementInternal()
	if isNilStatement(stmt) {
		return nil
	}
	return stmt
}

func (p *Parser) parseStatementInternal() ast.Statement {
	var decorators []*ast.Decorator
	for p.curTokenIs(token.AT) {
		dec := p.parseDecorator()
		if dec != nil {
			decorators = append(decorators, dec)
		}
		p.skipNewlines()
	}

	visibility, isConst := p.parseVarModifiers()

	isPublic := false
	if p.curTokenIs(token.PUB) {
		isPublic = true
		p.nextToken() // past 'pub'
	}
	if visibility == "public" {
		isPublic = true
	}

	isAsync := false
	if p.curTokenIs(token.ASYNC) {
		isAsync = true
		p.nextToken() // past 'async'
		p.skipNewlines()
	}

	switch p.curToken.Type {
	case token.FUNC, token.SHFUNC, token.TYPE_VOID:
		if isConst {
			p.reportAt(p.curToken, "functions cannot be declared 'const'", "const function")
		}
		fn := p.parseFuncDeclaration(isPublic, decorators)
		if fn != nil {
			fn.IsAsync = isAsync
		}
		return fn
	case token.ENUM:
		return p.parseEnumDeclaration(isPublic, decorators)
	case token.INTERFACE, token.TRAIT:
		return p.parseInterfaceDeclaration(isPublic, decorators)
	case token.STRUCT:
		return p.parseStructDeclaration(isPublic, decorators)
	case token.CLASS:
		if visibility == "private" {
			p.reportAt(p.curToken, "classes are visible to the whole module; remove 'private'", "private class")
			visibility = ""
		}
		if isConst {
			p.reportAt(p.curToken, "classes cannot be declared 'const'", "const class")
			isConst = false
		}
		if visibility == "public" {
			isPublic = true
		}
		return p.parseClassDeclaration(isPublic, decorators)
	case token.IF:
		return p.parseIfStatement()
	case token.WHILE:
		return p.parseWhileStatement()
	case token.FOR:
		return p.parseForStatement()
	case token.FOREACH:
		return p.parseForEachStatement()
	case token.LOOP:
		return p.parseLoopStatement()
	case token.RETURN:
		return p.parseReturnStatement()
	case token.BREAK:
		return p.parseBreakStatement()
	case token.CONTINUE:
		return p.parseContinueStatement()
	case token.USE, token.IMPORT:
		return p.parseUseStatement()
	case token.WATCH:
		return p.parseWatchStatement()
	case token.TEST:
		return p.parseTestStatement()
	case token.MOCK:
		return p.parseMockStatement()
	case token.BA:
		return p.parseBaStatement()
	case token.THREADS:
		return p.parseThreadsStatement()
	case token.TX:
		return p.parseTxStatement()
	case token.WAIT:
		return p.parseWaitStatement()
	case token.ASSERT:
		return p.parseAssertStatement()
	case token.MATCH:
		return p.parseMatchStatement()
	case token.IDENT, token.TYPE_STRING, token.TYPE_INT, token.TYPE_FLOAT,
		token.TYPE_BOOL, token.TYPE_LIST, token.TYPE_MAP, token.TYPE_BLOB,
		token.TYPE_SECRET, token.TYPE_JSON:
		if token.IsTypeKeyword(p.curToken.Type) && (p.peekTokenIs(token.IDENT) || p.peekTokenIs(token.COLON)) {
			if visibility != "" || isConst {
				// Modified declarations use the requested type-first form:
				// 'public int: x = 10;', 'private string: name = "";',
				// 'const float: pi = 3.14;'.
				return p.parseTypedVarDeclaration(visibility, isConst, decorators)
			}
			// C-style declaration ('int x = 5;' or 'int: x = 5;') is not part of
			// Alca: reject with a hint and skip to end of statement to avoid cascading errors.
			p.reportAt(p.curToken, fmt.Sprintf("declare variables as 'name: %s = value;'", p.curToken.Literal),
				"C-style declarations are not supported")
			// Skip tokens until we reach the end of the statement to avoid cascading errors
			for !p.peekTokenIs(token.NEWLINE) && !p.peekTokenIs(token.EOF) && !p.peekTokenIs(token.DEDENT) {
				p.nextToken()
			}
			p.nextToken() // cur = NEWLINE / EOF / DEDENT
			return nil
		}
		if p.curTokenIs(token.IDENT) && p.peekTokenIs(token.COLON) {
			vd := p.parseVarDeclaration(decorators)
			if vd != nil {
				vd.Visibility = visibility
				vd.IsConst = isConst
			}
			return vd
		}
		if visibility != "" || isConst {
			p.reportAt(p.curToken, "write 'public int: x = 10;', 'private int: x = 10;' or 'const int: x = 10;'",
				fmt.Sprintf("modifier %q must be followed by a variable declaration", p.curToken.Literal))
			p.consumeStatementEnd()
			return nil
		}
		return p.parseExpressionStatement()
	default:
		if visibility != "" || isConst {
			p.reportAt(p.curToken, "write 'public int: x = 10;', 'private int: x = 10;' or 'const int: x = 10;'",
				fmt.Sprintf("modifier %q must be followed by a variable declaration", p.curToken.Literal))
			p.consumeStatementEnd()
			return nil
		}
		return p.parseExpressionStatement()
	}
}

// parseVarModifiers consumes the optional visibility and const modifiers of a
// variable declaration. They precede the declaration in any order, e.g.
// 'public const int: limit = 10;'.
func (p *Parser) parseVarModifiers() (visibility string, isConst bool) {
	for {
		switch p.curToken.Type {
		case token.PUBLIC:
			if visibility == "private" {
				p.reportAt(p.curToken, "keep a single visibility: 'public' or 'private'", "conflicting visibility modifiers")
			}
			visibility = "public"
			p.nextToken()
		case token.PRIVATE:
			if visibility == "public" {
				p.reportAt(p.curToken, "keep a single visibility: 'public' or 'private'", "conflicting visibility modifiers")
			}
			visibility = "private"
			p.nextToken()
		case token.CONST:
			if isConst {
				p.reportAt(p.curToken, "remove the duplicate 'const'", "duplicate 'const' modifier")
			}
			isConst = true
			p.nextToken()
		default:
			return visibility, isConst
		}
	}
}

// reportAt records a syntax error positioned at tok.
func (p *Parser) reportAt(tok token.Token, suggestion, msg string) {
	length := len(tok.Literal)
	if length <= 0 {
		length = 1
	}
	p.errors = append(p.errors, fmt.Sprintf("line %d, col %d: %s", tok.Line, tok.Column, msg))
	p.syntaxErrors = append(p.syntaxErrors, &diag.SyntaxError{
		Line:       tok.Line,
		Column:     tok.Column,
		Message:    msg,
		Token:      string(tok.Type),
		Length:     length,
		Suggestion: suggestion,
	})
}

func (p *Parser) parseDecorator() *ast.Decorator {
	tok := p.curToken // '@'
	p.nextToken()     // move to decorator name

	if !p.curTokenIs(token.IDENT) {
		p.reportAt(p.curToken, "provide a decorator name: '@memoize' or '@timeout(5s)'",
			fmt.Sprintf("expected identifier after '@', got %q", p.curToken.Literal))
		return nil
	}

	name := p.curToken.Literal
	var args []ast.Expression

	if p.peekTokenIs(token.LPAREN) {
		p.nextToken() // move to '('
		args = p.parseExpressionList(token.RPAREN)
		// parseExpressionList leaves the current token on the closing
		// parenthesis. Step past it so that a decorator with arguments ends
		// like a bare one and the declaration that follows is parsed.
		if p.curTokenIs(token.RPAREN) {
			p.nextToken()
		}
	} else {
		p.nextToken() // move past decorator name
	}

	return &ast.Decorator{
		Token: tok,
		Name:  name,
		Args:  args,
	}
}

func (p *Parser) parseVarDeclaration(decorators []*ast.Decorator) *ast.VarDeclaration {
	tok := p.curToken
	name := &ast.Identifier{Token: tok, Value: tok.Literal}
	var typeAnnotation *ast.TypeAnnotation
	var value ast.Expression

	p.nextToken() // move to ':'
	if p.peekTokenIs(token.ASSIGN) {
		// 'name := value;' -> type inferred from the initializer (fixed afterwards).
		p.nextToken() // '='
	} else {
		p.nextToken() // move to type
		typeAnnotation = p.parseTypeAnnotation()
		if !p.peekTokenIs(token.ASSIGN) {
			p.reportAt(p.curToken, "initialize it: 'name: type = value;'", "variable declaration requires an initial value")
			p.consumeStatementEnd()
			return &ast.VarDeclaration{Token: tok, Name: name, TypeAnnotation: typeAnnotation, Decorators: decorators}
		}
		p.nextToken() // '='
	}
	p.nextToken() // move to value expression
	value = p.parseExpression(LOWEST)

	p.consumeStatementEnd()

	return &ast.VarDeclaration{
		Token:          tok,
		Name:           name,
		TypeAnnotation: typeAnnotation,
		Value:          value,
		Decorators:     decorators,
	}
}

// parseTypedVarDeclaration parses the type-first declaration form used with
// modifiers:
//
//	public int: x = 10;
//	private string: name = "secret";
//	const float: pi = 3.14;
//	public int count = 0;
func (p *Parser) parseTypedVarDeclaration(visibility string, isConst bool, decorators []*ast.Decorator) *ast.VarDeclaration {
	typeTok := p.curToken
	typeAnnotation := p.parseTypeAnnotation()

	// Move from the type to the variable name. Both 'int: x' and 'int x' are
	// accepted, with the colon form being the canonical one.
	if p.peekTokenIs(token.COLON) {
		p.nextToken() // ':'
	}
	if !p.peekTokenIs(token.IDENT) && !token.IsTypeKeyword(p.peekToken.Type) {
		p.reportAt(p.curToken, fmt.Sprintf("write '%s int: x = 10;'", firstModifier(visibility, isConst)),
			fmt.Sprintf("expected variable name after type %q, got %q", typeAnnotation.String(), p.peekToken.Literal))
		p.consumeStatementEnd()
		return nil
	}
	p.nextToken() // variable name

	nameTok := p.curToken
	name := &ast.Identifier{Token: nameTok, Value: nameTok.Literal}

	var value ast.Expression
	if p.peekTokenIs(token.ASSIGN) {
		p.nextToken() // '='
		p.nextToken() // value expression
		value = p.parseExpression(LOWEST)
	} else {
		p.reportAt(nameTok, "initialize it: 'name: type = value;'", "variable declaration requires an initial value")
	}

	p.consumeStatementEnd()

	return &ast.VarDeclaration{
		Token:          typeTok,
		Name:           name,
		TypeAnnotation: typeAnnotation,
		Value:          value,
		Decorators:     decorators,
		Visibility:     visibility,
		IsConst:        isConst,
	}
}

func firstModifier(visibility string, isConst bool) string {
	switch {
	case visibility != "":
		if isConst {
			return visibility + " const"
		}
		return visibility
	case isConst:
		return "const"
	}
	return ""
}

func (p *Parser) parseFuncDeclaration(isPublic bool, decorators []*ast.Decorator) *ast.FuncDeclaration {
	tok := p.curToken // 'func', 'shfunc', or 'void'
	isShell := (tok.Type == token.SHFUNC)
	isVoidPrefix := (tok.Type == token.TYPE_VOID)

	p.nextToken() // past func/shfunc/void
	if isVoidPrefix && p.curTokenIs(token.FUNC) {
		p.nextToken() // allow optional 'func' after 'void'
	}
	if !p.curTokenIs(token.IDENT) && !token.IsTypeKeyword(p.curToken.Type) {
		p.reportAt(p.curToken, "provide a valid function name, e.g. 'func calculate():'",
			fmt.Sprintf("expected function name after '%s', got %q", tok.Literal, p.curToken.Literal))
		return nil
	}
	name := p.curToken.Literal

	if !p.expectPeek(token.LPAREN) {
		return nil
	}

	params := p.parseFuncParams()

	var returnType *ast.TypeAnnotation
	if isVoidPrefix {
		returnType = &ast.TypeAnnotation{
			Token: tok,
			Types: []string{"void"},
		}
	}
	if p.peekTokenIs(token.ARROW) {
		p.nextToken() // '->'
		p.nextToken() // start of return type
		ret := p.parseTypeAnnotation()
		if isVoidPrefix && (len(ret.Types) != 1 || ret.Types[0] != "void") {
			p.reportAt(tok, "a function declared with 'void' cannot return a non-void type", "conflicting return type")
		}
		returnType = ret
	}

	var body *ast.BlockStatement
	if p.peekTokenIs(token.LBRACE) {
		p.nextToken() // move to '{'
		body = p.parseBraceBlockStatement()
	} else if p.peekTokenIs(token.COLON) {
		p.nextToken() // move to ':'
		body = p.parseBlockStatement()
	} else if p.peekTokenIs(token.NEWLINE) {
		p.skipPeekNewlines()
		if p.peekTokenIs(token.LBRACE) {
			p.nextToken()
			body = p.parseBraceBlockStatement()
		} else if p.peekTokenIs(token.COLON) {
			p.nextToken()
			body = p.parseBlockStatement()
		} else if p.curTokenIs(token.LBRACE) {
			body = p.parseBraceBlockStatement()
		} else if p.curTokenIs(token.COLON) {
			body = p.parseBlockStatement()
		} else {
			p.reportAt(p.curToken, "provide '{' to start the function body",
				"expected '{' after function signature")
			return nil
		}
	} else if p.curTokenIs(token.LBRACE) {
		body = p.parseBraceBlockStatement()
	} else if p.curTokenIs(token.COLON) {
		body = p.parseBlockStatement()
	} else {
		p.reportAt(p.curToken, "provide '{' to start the function body",
			"expected '{' after function signature")
		return nil
	}

	return &ast.FuncDeclaration{
		Token:      tok,
		Name:       name,
		Params:     params,
		ReturnType: returnType,
		Body:       body,
		IsPublic:   isPublic,
		IsShell:    isShell,
		Decorators: decorators,
	}
}

func (p *Parser) parseFuncParams() []*ast.FuncParam {
	var params []*ast.FuncParam

	p.skipPeekNewlines()
	if p.peekTokenIs(token.RPAREN) {
		p.nextToken()
		return params
	}

	p.nextToken() // move to first param name
	p.skipNewlines()

	for !p.curTokenIs(token.RPAREN) && !p.curTokenIs(token.EOF) {
		paramTok := p.curToken
		paramName := paramTok.Literal
		param := &ast.FuncParam{
			Token: paramTok,
			Name:  paramName,
		}

		if p.peekTokenIs(token.COLON) {
			p.nextToken() // ':'
			p.nextToken() // start of type
			param.TypeAnnotation = p.parseTypeAnnotation()
		} else if p.peekTokenIs(token.IDENT) || token.IsTypeKeyword(p.peekToken.Type) {
			p.reportAt(p.peekToken, fmt.Sprintf("write parameter types with a colon: '%s: %s'", paramName, p.peekToken.Literal),
				fmt.Sprintf("missing ':' before type annotation for parameter '%s'", paramName))
			p.nextToken() // move to type
			param.TypeAnnotation = p.parseTypeAnnotation()
		}

		if p.peekTokenIs(token.ASSIGN) {
			p.nextToken() // '='
			p.nextToken() // default value
			param.DefaultValue = p.parseExpression(LOWEST)
		}

		params = append(params, param)

		p.skipPeekNewlines()
		if p.peekTokenIs(token.COMMA) {
			p.nextToken() // ','
			p.skipPeekNewlines()
			if p.peekTokenIs(token.RPAREN) {
				p.nextToken() // trailing comma before ')'
				return params
			}
			p.nextToken() // next param name
			p.skipNewlines()
		} else {
			break
		}
	}

	p.skipPeekNewlines()
	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	return params
}

func (p *Parser) parseStructDeclaration(isPublic bool, decorators []*ast.Decorator) *ast.StructDeclaration {
	structTok := p.curToken
	p.nextToken() // past 'struct'

	if !p.curTokenIs(token.IDENT) {
		p.reportAt(p.curToken, "provide a struct name, e.g. 'struct User:'",
			fmt.Sprintf("expected struct name after 'struct', got %q", p.curToken.Literal))
		return nil
	}

	name := p.curToken.Literal

	var isBraced bool
	p.skipPeekNewlines()
	if p.peekTokenIs(token.LBRACE) {
		p.nextToken()
		isBraced = true
		p.nextToken()
	} else if p.curTokenIs(token.LBRACE) {
		isBraced = true
		p.nextToken()
	} else {
		p.expectBlockColon("struct " + name)
		if p.curTokenIs(token.COLON) {
			p.nextToken()
		}
		p.skipNewlines()
		if p.curTokenIs(token.INDENT) {
			p.nextToken()
		}
	}

	p.skipNewlines()

	var fields []*ast.StructField

	isEnd := func() bool {
		if isBraced {
			return p.curTokenIs(token.RBRACE) || p.curTokenIs(token.EOF)
		}
		return p.curTokenIs(token.DEDENT) || p.curTokenIs(token.EOF)
	}

	for !isEnd() {
		prevTok := p.curToken
		if isBraced {
			p.skipBraceBlockTrivia()
		} else {
			p.skipNewlines()
		}
		if isEnd() {
			break
		}

		fieldVisibility, fieldIsConst := p.parseVarModifiers()
		fieldTok := p.curToken
		fieldName := fieldTok.Literal

		var typeAnnot *ast.TypeAnnotation
		var defaultVal ast.Expression

		if token.IsTypeKeyword(fieldTok.Type) && (p.peekTokenIs(token.IDENT) || p.peekTokenIs(token.COLON)) {
			// Type-first field with a modifier: 'private int: x = 10;'.
			typeAnnot = p.parseTypeAnnotation()
			if p.peekTokenIs(token.COLON) {
				p.nextToken()
			}
			if !p.peekTokenIs(token.IDENT) {
				p.reportAt(fieldTok, "write 'private int: field = value;'", "expected a field name after the type")
				p.consumeStatementEnd()
				continue
			}
			p.nextToken() // field name
			fieldTok = p.curToken
			fieldName = fieldTok.Literal
		} else if p.peekTokenIs(token.COLON) {
			p.nextToken() // ':'
			p.nextToken() // type start
			typeAnnot = p.parseTypeAnnotation()
		}

		if p.peekTokenIs(token.ASSIGN) {
			p.nextToken() // '='
			p.nextToken() // default value
			defaultVal = p.parseExpression(LOWEST)
		}

		fields = append(fields, &ast.StructField{
			Token:          fieldTok,
			Name:           fieldName,
			TypeAnnotation: typeAnnot,
			DefaultValue:   defaultVal,
			Visibility:     fieldVisibility,
			IsConst:        fieldIsConst,
		})

		p.consumeStatementEnd()
		if isBraced {
			p.skipBraceBlockTrivia()
		} else {
			p.skipNewlines()
		}
		if p.curToken == prevTok {
			p.nextToken()
			if isBraced {
				p.skipBraceBlockTrivia()
			} else {
				p.skipNewlines()
			}
		}
	}

	if isBraced {
		if p.curTokenIs(token.RBRACE) {
			p.nextToken()
		}
	} else if p.curTokenIs(token.DEDENT) {
		p.nextToken()
	}

	return &ast.StructDeclaration{
		Token:      structTok,
		Name:       name,
		Fields:     fields,
		IsPublic:   isPublic,
		Decorators: decorators,
	}
}

func (p *Parser) parseClassDeclaration(isPublic bool, decorators []*ast.Decorator) *ast.ClassDeclaration {
	classTok := p.curToken
	p.nextToken() // past 'class'

	if !p.curTokenIs(token.IDENT) {
		p.reportAt(p.curToken, "provide a class name, e.g. 'class User:'",
			fmt.Sprintf("expected class name after 'class', got %q", p.curToken.Literal))
		return nil
	}

	name := p.curToken.Literal

	var parent string
	var parentTok token.Token
	if p.peekTokenIs(token.EXTENDS) {
		p.nextToken() // 'extends'
		p.nextToken() // base class name
		if !p.curTokenIs(token.IDENT) {
			p.reportAt(p.curToken, "write 'class Child extends Parent:'", "expected a base class name after 'extends'")
		} else {
			parent = p.curToken.Literal
			parentTok = p.curToken
		}
	}

	var isBraced bool
	p.skipPeekNewlines()
	if p.peekTokenIs(token.LBRACE) {
		p.nextToken()
		isBraced = true
		p.nextToken()
	} else if p.curTokenIs(token.LBRACE) {
		isBraced = true
		p.nextToken()
	} else {
		p.expectBlockColon("class " + name)
		if p.curTokenIs(token.COLON) {
			p.nextToken()
		}
		p.skipNewlines()
		if p.curTokenIs(token.INDENT) {
			p.nextToken()
		}
	}

	class := &ast.ClassDeclaration{
		Token:       classTok,
		Name:        name,
		Parent:      parent,
		ParentToken: parentTok,
		IsPublic:    isPublic,
		Decorators:  decorators,
	}

	isEnd := func() bool {
		if isBraced {
			return p.curTokenIs(token.RBRACE) || p.curTokenIs(token.EOF)
		}
		return p.curTokenIs(token.DEDENT) || p.curTokenIs(token.EOF)
	}

	for !isEnd() {
		prevTok := p.curToken
		if isBraced {
			p.skipBraceBlockTrivia()
		} else {
			p.skipNewlines()
		}
		if isEnd() {
			break
		}

		visibility, isConst := p.parseVarModifiers()
		if p.curTokenIs(token.PUB) {
			visibility = "public"
			p.nextToken()
		}
		switch p.curToken.Type {
		case token.FUNC, token.SHFUNC, token.TYPE_VOID:
			if isConst {
				p.reportAt(p.curToken, "methods cannot be declared 'const'", "const method")
			}
			if p.curTokenIs(token.SHFUNC) {
				p.reportAt(p.curToken, "methods are declared with 'func'", "shell functions cannot be class methods")
			}
			method := p.parseFuncDeclaration(visibility == "public", nil)
			if method != nil {
				class.Methods = append(class.Methods, &ast.MethodDeclaration{
					Token:      method.Token,
					Visibility: visibility,
					Func:       method,
				})
			}
		case token.STRUCT:
			if isConst {
				p.reportAt(p.curToken, "nested structs cannot be declared 'const'", "const struct")
			}
			nested := p.parseStructDeclaration(visibility == "public", nil)
			if nested != nil {
				class.NestedStructs = append(class.NestedStructs, nested)
			}
		default:
			field := p.parseClassField(visibility, isConst)
			if field != nil {
				class.Fields = append(class.Fields, field)
			}
		}

		if isBraced {
			p.skipBraceBlockTrivia()
		} else {
			p.skipNewlines()
		}
		if p.curToken == prevTok {
			p.nextToken()
			if isBraced {
				p.skipBraceBlockTrivia()
			} else {
				p.skipNewlines()
			}
		}
	}

	if isBraced {
		if p.curTokenIs(token.RBRACE) {
			p.nextToken()
		}
	} else if p.curTokenIs(token.DEDENT) {
		p.nextToken()
	}

	return class
}

func (p *Parser) parseEnumDeclaration(isPublic bool, decorators []*ast.Decorator) *ast.EnumDeclaration {
	enumTok := p.curToken
	p.nextToken() // past 'enum'

	if !p.curTokenIs(token.IDENT) {
		p.reportAt(p.curToken, "provide an enum name, e.g. 'enum Status {'", "expected enum name after 'enum'")
		return nil
	}
	name := p.curToken.Literal

	var isBraced bool
	p.skipPeekNewlines()
	if p.peekTokenIs(token.LBRACE) {
		p.nextToken()
		isBraced = true
		p.nextToken()
	} else if p.curTokenIs(token.LBRACE) {
		isBraced = true
		p.nextToken()
	} else {
		p.expectBlockColon("enum " + name)
		if p.curTokenIs(token.COLON) {
			p.nextToken()
		}
		p.skipNewlines()
		if p.curTokenIs(token.INDENT) {
			p.nextToken()
		}
	}

	var variants []*ast.EnumVariant
	isEnd := func() bool {
		if isBraced {
			return p.curTokenIs(token.RBRACE) || p.curTokenIs(token.EOF)
		}
		return p.curTokenIs(token.DEDENT) || p.curTokenIs(token.EOF)
	}

	for !isEnd() {
		if isBraced {
			p.skipBraceBlockTrivia()
		} else {
			p.skipNewlines()
		}
		if isEnd() {
			break
		}

		if !p.curTokenIs(token.IDENT) {
			p.nextToken()
			continue
		}

		varTok := p.curToken
		varName := varTok.Literal
		var varVal ast.Expression

		if p.peekTokenIs(token.ASSIGN) {
			p.nextToken() // '='
			p.nextToken()
			varVal = p.parseExpression(LOWEST)
		}

		variants = append(variants, &ast.EnumVariant{
			Token: varTok,
			Name:  varName,
			Value: varVal,
		})

		if p.peekTokenIs(token.COMMA) || p.peekTokenIs(token.SEMICOLON) {
			p.nextToken()
		}
		p.nextToken()
	}

	if isBraced {
		if p.curTokenIs(token.RBRACE) {
			p.nextToken()
		}
	} else if p.curTokenIs(token.DEDENT) {
		p.nextToken()
	}

	return &ast.EnumDeclaration{
		Token:      enumTok,
		Name:       name,
		Variants:   variants,
		IsPublic:   isPublic,
		Decorators: decorators,
	}
}

func (p *Parser) parseInterfaceDeclaration(isPublic bool, decorators []*ast.Decorator) *ast.InterfaceDeclaration {
	ifaceTok := p.curToken
	p.nextToken() // past 'interface' / 'trait'

	if !p.curTokenIs(token.IDENT) {
		p.reportAt(p.curToken, "provide an interface name, e.g. 'interface Greeter {'", "expected interface name")
		return nil
	}
	name := p.curToken.Literal

	var isBraced bool
	p.skipPeekNewlines()
	if p.peekTokenIs(token.LBRACE) {
		p.nextToken()
		isBraced = true
		p.nextToken()
	} else if p.curTokenIs(token.LBRACE) {
		isBraced = true
		p.nextToken()
	} else {
		p.expectBlockColon("interface " + name)
		if p.curTokenIs(token.COLON) {
			p.nextToken()
		}
		p.skipNewlines()
		if p.curTokenIs(token.INDENT) {
			p.nextToken()
		}
	}

	var methods []*ast.MethodSignature
	isEnd := func() bool {
		if isBraced {
			return p.curTokenIs(token.RBRACE) || p.curTokenIs(token.EOF)
		}
		return p.curTokenIs(token.DEDENT) || p.curTokenIs(token.EOF)
	}

	for !isEnd() {
		if isBraced {
			p.skipBraceBlockTrivia()
		} else {
			p.skipNewlines()
		}
		if isEnd() {
			break
		}

		if p.curTokenIs(token.PUB) {
			p.nextToken()
		}

		if p.curTokenIs(token.FUNC) {
			fnTok := p.curToken
			p.nextToken() // past 'fn'/'func'
			if !p.curTokenIs(token.IDENT) {
				p.nextToken()
				continue
			}
			mName := p.curToken.Literal
			if !p.expectPeek(token.LPAREN) {
				continue
			}
			params := p.parseFuncParams()
			var retType *ast.TypeAnnotation
			if p.peekTokenIs(token.ARROW) {
				p.nextToken() // '->'
				p.nextToken()
				retType = p.parseTypeAnnotation()
			}
			methods = append(methods, &ast.MethodSignature{
				Token:      fnTok,
				Name:       mName,
				Params:     params,
				ReturnType: retType,
			})
			p.consumeStatementEnd()
		} else {
			p.nextToken()
		}
	}

	if isBraced {
		if p.curTokenIs(token.RBRACE) {
			p.nextToken()
		}
	} else if p.curTokenIs(token.DEDENT) {
		p.nextToken()
	}

	return &ast.InterfaceDeclaration{
		Token:      ifaceTok,
		Name:       name,
		Methods:    methods,
		IsPublic:   isPublic,
		Decorators: decorators,
	}
}

func (p *Parser) parseAwaitExpression() ast.Expression {
	tok := p.curToken // 'await'
	p.nextToken()
	expr := p.parseExpression(PREFIX)
	return &ast.AwaitExpression{
		Token:      tok,
		Expression: expr,
	}
}

// parseClassField parses one field line inside a class body. It accepts the
// same forms as struct fields, including 'public int: x = 10;'.
func (p *Parser) parseClassField(visibility string, isConst bool) *ast.StructField {
	fieldTok := p.curToken
	fieldName := fieldTok.Literal

	var typeAnnot *ast.TypeAnnotation
	var defaultVal ast.Expression

	if token.IsTypeKeyword(fieldTok.Type) && (p.peekTokenIs(token.IDENT) || p.peekTokenIs(token.COLON)) {
		typeAnnot = p.parseTypeAnnotation()
		if p.peekTokenIs(token.COLON) {
			p.nextToken()
		}
		if !p.peekTokenIs(token.IDENT) {
			p.reportAt(fieldTok, "write 'private int: field = value;'", "expected a field name after the type")
			p.consumeStatementEnd()
			return nil
		}
		p.nextToken()
		fieldTok = p.curToken
		fieldName = fieldTok.Literal
	} else if p.peekTokenIs(token.COLON) {
		p.nextToken() // ':'
		p.nextToken() // type start
		typeAnnot = p.parseTypeAnnotation()
	}

	if p.peekTokenIs(token.ASSIGN) {
		p.nextToken() // '='
		p.nextToken() // default value
		defaultVal = p.parseExpression(LOWEST)
	}

	field := &ast.StructField{
		Token:          fieldTok,
		Name:           fieldName,
		TypeAnnotation: typeAnnot,
		DefaultValue:   defaultVal,
		Visibility:     visibility,
		IsConst:        isConst,
	}
	p.consumeStatementEnd()
	return field
}

func (p *Parser) parseBlockStatement() *ast.BlockStatement {
	if p.peekTokenIs(token.LBRACE) {
		p.nextToken()
	}
	if p.curTokenIs(token.LBRACE) {
		return p.parseBraceBlockStatement()
	}

	block := &ast.BlockStatement{
		Token:      p.curToken,
		Statements: []ast.Statement{},
	}

	if p.peekTokenIs(token.COLON) {
		p.nextToken()
	}
	if p.curTokenIs(token.COLON) {
		p.nextToken()
	}

	// A body written on the same physical line as the ':' forms a single-line
	// block. Otherwise the body is either indented (an INDENT token) or empty:
	// comment lines are invisible to the lexer, so a comment-only body emits
	// no INDENT at all. An unindented token after the header line (e.g. else:)
	// is never treated as this block's body.
	if p.curTokenIs(token.INDENT) || p.curTokenIs(token.NEWLINE) ||
		p.curTokenIs(token.DEDENT) || p.curTokenIs(token.EOF) || p.peekTokenIs(token.NEWLINE) {
		if !p.curTokenIs(token.INDENT) && p.peekTokenIs(token.NEWLINE) {
			p.nextToken()
		}
		p.skipNewlines()

		if p.curTokenIs(token.INDENT) {
			p.nextToken() // enter block

			for !p.curTokenIs(token.DEDENT) && !p.curTokenIs(token.EOF) {
				prevTok := p.curToken
				p.skipNewlines()
				if p.curTokenIs(token.DEDENT) || p.curTokenIs(token.EOF) {
					break
				}

				stmt := p.parseStatement()
				if stmt != nil {
					block.Statements = append(block.Statements, stmt)
				} else {
					p.nextToken()
				}

				p.skipNewlines()
				if p.curToken == prevTok {
					p.nextToken()
					p.skipNewlines()
				}
			}

			if p.curTokenIs(token.DEDENT) {
				p.nextToken() // consume DEDENT
			}
			return block
		}

		// No INDENT token follows: the body is empty. A comment-only body is
		// legitimate (comments emit no tokens), but a real statement at the
		// header's indentation level is not part of this block, and the
		// continuation keywords belong to the enclosing statement.
		switch p.curToken.Type {
		case token.DEDENT, token.EOF, token.ELIF, token.ELSE, token.CASE:
			return block
		}

		tok := block.Token
		p.errors = append(p.errors, fmt.Sprintf("line %d, col %d: expected an indented block", tok.Line, tok.Column))
		p.syntaxErrors = append(p.syntaxErrors, &diag.SyntaxError{
			Line:       tok.Line,
			Column:     tok.Column,
			Message:    "expected an indented block",
			Token:      tok.Literal,
			Length:     len(tok.Literal),
			Suggestion: "indent the block body by 4 spaces or 1 tab",
		})
		return block
	}

	// Single-line block statement
	stmt := p.parseStatement()
	if stmt != nil {
		block.Statements = append(block.Statements, stmt)
	}

	return block
}

// parseBraceBlockStatement parses a block enclosed in curly braces '{' ... '}'.
func (p *Parser) parseBraceBlockStatement() *ast.BlockStatement {
	block := &ast.BlockStatement{
		Token:      p.curToken, // token.LBRACE
		Statements: []ast.Statement{},
	}

	p.nextToken() // move past '{'

	for !p.curTokenIs(token.RBRACE) && !p.curTokenIs(token.EOF) {
		p.skipBraceBlockTrivia()
		if p.curTokenIs(token.RBRACE) || p.curTokenIs(token.EOF) {
			break
		}

		prevTok := p.curToken
		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		} else {
			p.nextToken()
		}

		p.skipBraceBlockTrivia()
		if p.curToken == prevTok {
			p.nextToken()
			p.skipBraceBlockTrivia()
		}
	}

	if p.curTokenIs(token.RBRACE) {
		p.nextToken() // consume '}'
	} else {
		p.reportAt(p.curToken, "close the function body with '}'", "expected '}' at end of function body")
	}

	return block
}

// skipBraceBlockTrivia advances past newlines, semicolons, and indentation tokens within a braced block.
func (p *Parser) skipBraceBlockTrivia() {
	for p.curTokenIs(token.NEWLINE) || p.curTokenIs(token.INDENT) || p.curTokenIs(token.DEDENT) || p.curTokenIs(token.SEMICOLON) {
		p.nextToken()
	}
}

func (p *Parser) parseIfStatement() *ast.IfStatement {
	ifTok := p.curToken
	if p.peekTokenIs(token.COLON) {
		p.reportAt(p.peekToken, "provide a boolean condition, e.g. 'if x > 0:'", "missing condition in 'if' statement")
		p.nextToken() // move to ':'
		consequence := p.parseBlockStatement()
		return &ast.IfStatement{
			Token:        ifTok,
			Condition:    &ast.BooleanLiteral{Token: ifTok, Value: false},
			Consequence:  consequence,
			Alternatives: []*ast.ElifClause{},
		}
	}
	p.nextToken() // move past 'if' to condition

	cond := p.parseExpression(LOWEST)

	p.expectBlockColon("if condition")

	consequence := p.parseBlockStatement()

	stmt := &ast.IfStatement{
		Token:        ifTok,
		Condition:    cond,
		Consequence:  consequence,
		Alternatives: []*ast.ElifClause{},
	}

	p.skipNewlines()

	for p.curTokenIs(token.ELIF) {
		elifTok := p.curToken
		if p.peekTokenIs(token.COLON) {
			p.reportAt(p.peekToken, "provide a boolean condition, e.g. 'elif x > 0:'", "missing condition in 'elif' statement")
			p.nextToken() // move to ':'
			elifBody := p.parseBlockStatement()
			stmt.Alternatives = append(stmt.Alternatives, &ast.ElifClause{
				Token:     elifTok,
				Condition: &ast.BooleanLiteral{Token: elifTok, Value: false},
				Body:      elifBody,
			})
			p.skipNewlines()
			continue
		}
		p.nextToken() // past 'elif' to condition

		elifCond := p.parseExpression(LOWEST)
		p.expectBlockColon("elif condition")
		elifBody := p.parseBlockStatement()

		stmt.Alternatives = append(stmt.Alternatives, &ast.ElifClause{
			Token:     elifTok,
			Condition: elifCond,
			Body:      elifBody,
		})

		p.skipNewlines()
	}

	if p.curTokenIs(token.ELSE) {
		p.nextToken() // past 'else'
		p.expectBlockColon("else")
		stmt.ElseBlock = p.parseBlockStatement()
	}

	return stmt
}

func (p *Parser) parseWhileStatement() *ast.WhileStatement {
	tok := p.curToken
	if p.peekTokenIs(token.COLON) {
		p.reportAt(p.peekToken, "provide a loop condition, e.g. 'while running:'", "missing condition in 'while' statement")
		p.nextToken() // move to ':'
		body := p.parseBlockStatement()
		return &ast.WhileStatement{
			Token:     tok,
			Condition: &ast.BooleanLiteral{Token: tok, Value: false},
			Body:      body,
		}
	}
	p.nextToken() // condition

	cond := p.parseExpression(LOWEST)

	p.expectBlockColon("while condition")

	body := p.parseBlockStatement()

	return &ast.WhileStatement{
		Token:     tok,
		Condition: cond,
		Body:      body,
	}
}

func (p *Parser) parseForStatement() *ast.ForStatement {
	forTok := p.curToken
	p.nextToken() // variable name
	if !p.curTokenIs(token.IDENT) {
		p.reportAt(p.curToken, "specify a variable name: 'for item in collection:'", "expected loop variable name after 'for'")
		return nil
	}

	varName := p.curToken.Literal
	if p.peekTokenIs(token.COMMA) {
		p.nextToken() // ','
		varName += ", "
		p.nextToken() // next ident
		varName += p.curToken.Literal
	}

	if !p.expectPeek(token.IN) {
		return nil
	}

	p.nextToken() // iterable start
	iterable := p.parseExpression(LOWEST)

	p.expectBlockColon("for expression")

	body := p.parseBlockStatement()

	return &ast.ForStatement{
		Token:    forTok,
		Variable: varName,
		Iterable: iterable,
		Body:     body,
	}
}

func (p *Parser) parseForEachStatement() *ast.ForEachStatement {
	forEachTok := p.curToken
	p.nextToken() // variable name
	if !p.curTokenIs(token.IDENT) {
		p.reportAt(p.curToken, "specify a variable name: 'foreach item in collection:'", "expected loop variable name after 'foreach'")
		return nil
	}

	varName := p.curToken.Literal
	if p.peekTokenIs(token.COMMA) {
		p.nextToken() // ','
		varName += ", "
		p.nextToken() // next ident
		varName += p.curToken.Literal
	}

	if !p.expectPeek(token.IN) {
		return nil
	}

	p.nextToken() // iterable start
	iterable := p.parseExpression(LOWEST)

	p.expectBlockColon("foreach expression")

	body := p.parseBlockStatement()

	return &ast.ForEachStatement{
		Token:    forEachTok,
		Variable: varName,
		Iterable: iterable,
		Body:     body,
	}
}

func (p *Parser) parseLoopStatement() *ast.LoopStatement {
	tok := p.curToken
	p.expectBlockColon("loop")

	body := p.parseBlockStatement()

	return &ast.LoopStatement{
		Token: tok,
		Body:  body,
	}
}

func (p *Parser) parseReturnStatement() *ast.ReturnStatement {
	tok := p.curToken
	stmt := &ast.ReturnStatement{Token: tok}

	if p.peekTokenIs(token.SEMICOLON) || p.peekTokenIs(token.NEWLINE) || p.peekTokenIs(token.EOF) || p.peekTokenIs(token.DEDENT) || p.peekTokenIs(token.RBRACE) {
		p.consumeStatementEnd()
		return stmt
	}

	p.nextToken()
	stmt.Value = p.parseExpression(LOWEST)
	p.consumeStatementEnd()

	return stmt
}

func (p *Parser) parseBreakStatement() *ast.BreakStatement {
	tok := p.curToken
	p.consumeStatementEnd()
	return &ast.BreakStatement{Token: tok}
}

func (p *Parser) parseContinueStatement() *ast.ContinueStatement {
	tok := p.curToken
	p.consumeStatementEnd()
	return &ast.ContinueStatement{Token: tok}
}

func (p *Parser) parseUseStatement() *ast.UseStatement {
	useTok := p.curToken
	p.nextToken() // past 'use'

	var modulePath string
	var imports []string
	var alias string

	if p.curTokenIs(token.STRING) {
		modulePath = p.curToken.Literal
	} else {
		modulePath = p.curToken.Literal
		for p.peekTokenIs(token.DOT) || p.peekTokenIs(token.SLASH) {
			p.nextToken()
			modulePath += p.curToken.Literal
			if p.peekTokenIs(token.IDENT) || token.IsTypeKeyword(p.peekToken.Type) || token.IsKeyword(p.peekToken.Type) {
				p.nextToken()
				modulePath += p.curToken.Literal
			}
		}
	}

	// Check for (ident1, ident2, ...)
	if p.peekTokenIs(token.LPAREN) {
		p.nextToken() // '('
		p.skipPeekNewlines()
		for !p.peekTokenIs(token.RPAREN) && !p.peekTokenIs(token.EOF) {
			p.nextToken()
			p.skipNewlines()
			imports = append(imports, p.curToken.Literal)
			p.skipPeekNewlines()
			if p.peekTokenIs(token.COMMA) {
				p.nextToken() // ','
				p.skipPeekNewlines()
			} else {
				break
			}
		}
		if !p.expectPeek(token.RPAREN) {
			return nil
		}
	}

	// Check for as alias
	if p.peekTokenIs(token.AS) {
		p.nextToken() // 'as'
		p.nextToken() // alias IDENT
		alias = p.curToken.Literal
	}

	p.consumeStatementEnd()

	return &ast.UseStatement{
		Token:      useTok,
		ModulePath: modulePath,
		Imports:    imports,
		Alias:      alias,
	}
}

func (p *Parser) parseWatchStatement() *ast.WatchStatement {
	watchTok := p.curToken
	p.nextToken() // target start

	target := p.parseExpression(LOWEST)

	if p.peekTokenIs(token.COLON) {
		p.nextToken()
	}

	body := p.parseBlockStatement()

	return &ast.WatchStatement{
		Token:  watchTok,
		Target: target,
		Body:   body,
	}
}

func (p *Parser) parseTestStatement() *ast.TestStatement {
	testTok := p.curToken
	p.nextToken() // name (string or ident)

	name := p.curToken.Literal

	if p.peekTokenIs(token.COLON) {
		p.nextToken()
	}

	body := p.parseBlockStatement()

	return &ast.TestStatement{
		Token: testTok,
		Name:  name,
		Body:  body,
	}
}

func (p *Parser) parseMockStatement() *ast.MockStatement {
	mockTok := p.curToken
	var target ast.Expression
	var replacement ast.Expression

	if p.peekTokenIs(token.LPAREN) {
		p.nextToken() // '('
		p.nextToken() // target
		target = p.parseExpression(LOWEST)
		if !p.expectPeek(token.COMMA) {
			return nil
		}
		p.nextToken() // replacement
		replacement = p.parseExpression(LOWEST)
		if !p.expectPeek(token.RPAREN) {
			return nil
		}
	} else {
		p.nextToken() // target
		target = p.parseExpression(ASSIGN)
		if p.peekTokenIs(token.ASSIGN) || p.peekTokenIs(token.COMMA) {
			p.nextToken() // '=' or ','
			p.nextToken() // replacement
			replacement = p.parseExpression(LOWEST)
		}
	}

	p.consumeStatementEnd()

	return &ast.MockStatement{
		Token:       mockTok,
		Target:      target,
		Replacement: replacement,
	}
}

func (p *Parser) parseBaStatement() ast.Statement {
	baTok := p.curToken
	var timeout ast.Expression
	var memLimit string
	var limit ast.Expression

	// Check for parenthesized options: ba(limit=10, timeout=5s, mem="50MB")
	if p.peekTokenIs(token.LPAREN) {
		p.nextToken() // '('
		p.nextToken() // first inside
		p.skipNewlines()

		for !p.curTokenIs(token.RPAREN) && !p.curTokenIs(token.EOF) {
			p.skipNewlines()
			if p.curTokenIs(token.RPAREN) {
				break
			}

			optName := p.curToken.Literal
			if p.peekTokenIs(token.ASSIGN) {
				p.nextToken() // '='
				p.nextToken() // value
				switch optName {
				case "limit":
					limit = p.parseExpression(LOWEST)
				case "timeout":
					timeout = p.parseExpression(LOWEST)
				case "mem":
					memLimit = p.curToken.Literal
				default:
					p.parseExpression(LOWEST)
				}
			}

			p.skipPeekNewlines()
			if p.peekTokenIs(token.COMMA) {
				p.nextToken() // ','
				p.skipPeekNewlines()
				p.nextToken()
			} else {
				break
			}
		}

		p.skipPeekNewlines()
		if !p.curTokenIs(token.RPAREN) {
			p.expectPeek(token.RPAREN)
		}
	} else {
		// Options without parentheses: e.g. ba limit=10 for x in items:
		for !p.peekTokenIs(token.FOR) && !p.peekTokenIs(token.COLON) && !p.peekTokenIs(token.NEWLINE) && !p.peekTokenIs(token.EOF) {
			p.nextToken()
			optName := p.curToken.Literal
			if p.peekTokenIs(token.ASSIGN) {
				p.nextToken() // '='
				p.nextToken() // value
				switch optName {
				case "limit":
					limit = p.parseExpression(LOWEST)
				case "timeout":
					timeout = p.parseExpression(LOWEST)
				case "mem":
					memLimit = p.curToken.Literal
				default:
					p.parseExpression(LOWEST)
				}
			}
			if p.peekTokenIs(token.COMMA) {
				p.nextToken()
			}
		}
	}

	// Check if followed by 'for'
	if p.peekTokenIs(token.FOR) {
		p.nextToken() // 'for'
		p.nextToken() // variable name
		varName := p.curToken.Literal
		if p.peekTokenIs(token.COMMA) {
			p.nextToken()
			varName += ", "
			p.nextToken()
			varName += p.curToken.Literal
		}

		if !p.expectPeek(token.IN) {
			return nil
		}

		p.nextToken() // start of iterable
		iterable := p.parseExpression(LOWEST)

		if p.peekTokenIs(token.COLON) {
			p.nextToken()
		}

		body := p.parseBlockStatement()

		return &ast.BaForStatement{
			Token:    baTok,
			Variable: varName,
			Iterable: iterable,
			Body:     body,
			Limit:    limit,
		}
	}

	if p.peekTokenIs(token.COLON) {
		p.nextToken()
	}

	body := p.parseBlockStatement()

	return &ast.BaStatement{
		Token:    baTok,
		Body:     body,
		Timeout:  timeout,
		MemLimit: memLimit,
		Limit:    limit,
	}
}

func (p *Parser) parseThreadsStatement() *ast.ThreadsStatement {
	tok := p.curToken
	if p.peekTokenIs(token.COLON) {
		p.nextToken()
	}
	body := p.parseBlockStatement()
	return &ast.ThreadsStatement{
		Token: tok,
		Body:  body,
	}
}

func (p *Parser) parseTxStatement() *ast.TxStatement {
	txTok := p.curToken
	var db ast.Expression
	var timeout ast.Expression

	if p.peekTokenIs(token.LPAREN) {
		p.nextToken() // '('
		p.nextToken() // db
		db = p.parseExpression(LOWEST)

		if p.peekTokenIs(token.COMMA) {
			p.nextToken() // ','
			p.nextToken() // timeout
			timeout = p.parseExpression(LOWEST)
		}

		if !p.expectPeek(token.RPAREN) {
			return nil
		}
	} else if !p.peekTokenIs(token.COLON) && !p.peekTokenIs(token.NEWLINE) {
		p.nextToken()
		db = p.parseExpression(LOWEST)
	}

	if p.peekTokenIs(token.COLON) {
		p.nextToken()
	}

	body := p.parseBlockStatement()

	return &ast.TxStatement{
		Token:   txTok,
		DB:      db,
		Timeout: timeout,
		Body:    body,
	}
}

func (p *Parser) parseWaitStatement() *ast.WaitStatement {
	tok := p.curToken
	p.consumeStatementEnd()
	return &ast.WaitStatement{Token: tok}
}

func (p *Parser) parseAssertStatement() *ast.AssertStatement {
	tok := p.curToken
	p.nextToken() // past 'assert'

	if p.curTokenIs(token.SEMICOLON) {
		p.reportAt(tok, "provide an expression to assert, e.g. 'assert x > 0;'", "missing condition in 'assert' statement")
		p.consumeStatementEnd()
		return &ast.AssertStatement{Token: tok}
	}

	var expr ast.Expression
	if p.curTokenIs(token.LPAREN) {
		p.nextToken() // inside '('
		expr = p.parseExpression(LOWEST)
		p.expectPeek(token.RPAREN)
	} else {
		expr = p.parseExpression(LOWEST)
	}

	p.consumeStatementEnd()

	return &ast.AssertStatement{
		Token:      tok,
		Expression: expr,
	}
}

func (p *Parser) parseMatchStatement() *ast.MatchStatement {
	matchTok := p.curToken
	p.nextToken() // past 'match'

	if p.curTokenIs(token.COLON) {
		p.reportAt(p.curToken, "provide a match expression, e.g. 'match status:'", "missing expression in 'match' statement")
	}

	subject := p.parseExpression(LOWEST)

	var isBraced bool
	p.skipPeekNewlines()
	if p.peekTokenIs(token.LBRACE) {
		p.nextToken()
		isBraced = true
		p.nextToken()
	} else if p.curTokenIs(token.LBRACE) {
		isBraced = true
		p.nextToken()
	} else {
		if p.peekTokenIs(token.COLON) {
			p.nextToken()
		}
		if p.curTokenIs(token.COLON) {
			p.nextToken()
		}
		p.skipNewlines()
		if p.curTokenIs(token.INDENT) {
			p.nextToken()
		}
	}

	p.skipNewlines()

	var cases []*ast.MatchCase
	var defaultBlock *ast.BlockStatement

	isEnd := func() bool {
		if isBraced {
			return p.curTokenIs(token.RBRACE) || p.curTokenIs(token.EOF)
		}
		return p.curTokenIs(token.DEDENT) || p.curTokenIs(token.EOF)
	}

	for !isEnd() {
		if isBraced {
			p.skipBraceBlockTrivia()
		} else {
			p.skipNewlines()
		}
		if isEnd() {
			break
		}

		if p.curTokenIs(token.CASE) {
			caseTok := p.curToken
			p.nextToken() // past 'case'

			if p.curTokenIs(token.COLON) {
				p.reportAt(p.curToken, "provide a pattern to match, e.g. 'case 200:'", "missing pattern in 'case' clause")
			}

			pattern := p.parseExpression(LOWEST)
			if p.peekTokenIs(token.COLON) {
				p.nextToken()
			}
			caseBody := p.parseBlockStatement()

			cases = append(cases, &ast.MatchCase{
				Token:   caseTok,
				Pattern: pattern,
				Body:    caseBody,
			})
		} else if p.curTokenIs(token.ELSE) {
			p.nextToken() // past 'else'
			if p.peekTokenIs(token.COLON) {
				p.nextToken()
			}
			defaultBlock = p.parseBlockStatement()
		} else {
			p.nextToken()
		}
		if isBraced {
			p.skipBraceBlockTrivia()
		} else {
			p.skipNewlines()
		}
	}

	if isBraced {
		if p.curTokenIs(token.RBRACE) {
			p.nextToken()
		}
	} else if p.curTokenIs(token.DEDENT) {
		p.nextToken()
	}

	return &ast.MatchStatement{
		Token:   matchTok,
		Subject: subject,
		Cases:   cases,
		Default: defaultBlock,
	}
}

func (p *Parser) parseExpressionStatement() *ast.ExpressionStatement {
	stmt := &ast.ExpressionStatement{Token: p.curToken}
	stmt.Expression = p.parseExpression(LOWEST)
	if stmt.Expression == nil {
		// Suppress cascading "expected ';'" error on the invalid expression.
		// Skip tokens until end of statement / newline / EOF.
		for !p.curTokenIs(token.SEMICOLON) && !p.curTokenIs(token.NEWLINE) && !p.curTokenIs(token.EOF) && !p.curTokenIs(token.DEDENT) {
			p.nextToken()
		}
		if p.curTokenIs(token.SEMICOLON) {
			p.nextToken()
		}
		return nil
	}
	p.consumeStatementEnd()
	return stmt
}

// -----------------------------------------------------------------------------
// Type Annotation Parsing
// -----------------------------------------------------------------------------

func (p *Parser) parseTypeAnnotation() *ast.TypeAnnotation {
	return p.parseTypeAnnotationWithUnion(true)
}

func (p *Parser) parseTypeAnnotationWithUnion(allowUnion bool) *ast.TypeAnnotation {
	tok := p.curToken
	var types []string

	for {
		typeStr := p.curToken.Literal

		for p.peekTokenIs(token.DOT) {
			p.nextToken() // '.'
			typeStr += "."
			if p.peekTokenIs(token.IDENT) || token.IsTypeKeyword(p.peekToken.Type) {
				p.nextToken()
				typeStr += p.curToken.Literal
			}
		}

		if p.peekTokenIs(token.LBRACKET) {
			p.nextToken() // '['
			typeStr += "["
			p.nextToken() // inside '['
			for !p.curTokenIs(token.RBRACKET) && !p.curTokenIs(token.EOF) {
				typeStr += p.curToken.Literal
				if p.peekTokenIs(token.COMMA) {
					p.nextToken()
					typeStr += ", "
				}
				p.nextToken()
			}
			if p.curTokenIs(token.RBRACKET) {
				typeStr += "]"
			}
		}

		types = append(types, typeStr)

		if allowUnion && p.peekTokenIs(token.PIPE_TYPE) {
			p.nextToken() // '|'
			p.nextToken() // next type
		} else {
			break
		}
	}

	return &ast.TypeAnnotation{
		Token: tok,
		Types: types,
	}
}

// -----------------------------------------------------------------------------
// Expression Parsing (Pratt)
// -----------------------------------------------------------------------------

func (p *Parser) parseExpression(precedence int) ast.Expression {
	p.skipNewlines()

	prefix := p.prefixParseFns[p.curToken.Type]
	if prefix == nil {
		p.noPrefixParseFnError(p.curToken.Type)
		return nil
	}
	leftExp := prefix()

	for !p.peekTokenIs(token.NEWLINE) && !p.peekTokenIs(token.EOF) && precedence < p.peekPrecedence() {
		infix := p.infixParseFns[p.peekToken.Type]
		if infix == nil {
			return leftExp
		}

		p.nextToken()

		leftExp = infix(leftExp)
	}

	return leftExp
}

func (p *Parser) parseIdentifier() ast.Expression {
	if p.curToken.Literal == "console" && p.peekTokenIs(token.LPAREN) {
		tok := p.curToken
		p.nextToken() // '('
		p.nextToken() // start of prompt
		var prompt ast.Expression
		if !p.curTokenIs(token.RPAREN) {
			prompt = p.parseExpression(LOWEST)
			p.expectPeek(token.RPAREN)
		}
		return &ast.ConsoleLiteral{
			Token:  tok,
			Prompt: prompt,
		}
	}
	return &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
}

func (p *Parser) parseTypeIdentifier() ast.Expression {
	return &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
}

func (p *Parser) parseIntegerLiteral() ast.Expression {
	lit := &ast.IntegerLiteral{Token: p.curToken}

	raw := p.curToken.Literal
	cleaned := raw
	for _, suffix := range []string{"ms", "s", "m", "h", "KB", "MB", "GB"} {
		if strings.HasSuffix(raw, suffix) {
			cleaned = strings.TrimSuffix(raw, suffix)
			break
		}
	}

	val, err := strconv.ParseInt(cleaned, 0, 64)
	if err != nil {
		fVal, fErr := strconv.ParseFloat(cleaned, 64)
		if fErr == nil {
			val = int64(fVal)
		} else {
			msg := fmt.Sprintf("line %d, col %d: could not parse %q as integer",
				p.curToken.Line, p.curToken.Column, p.curToken.Literal)
			p.errors = append(p.errors, msg)
			return nil
		}
	}

	lit.Value = val
	return lit
}

func (p *Parser) parseFloatLiteral() ast.Expression {
	lit := &ast.FloatLiteral{Token: p.curToken}

	raw := p.curToken.Literal
	cleaned := raw
	for _, suffix := range []string{"ms", "s", "m", "h", "KB", "MB", "GB"} {
		if strings.HasSuffix(raw, suffix) {
			cleaned = strings.TrimSuffix(raw, suffix)
			break
		}
	}

	val, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		msg := fmt.Sprintf("line %d, col %d: could not parse %q as float",
			p.curToken.Line, p.curToken.Column, p.curToken.Literal)
		p.errors = append(p.errors, msg)
		return nil
	}

	lit.Value = val
	return lit
}

func (p *Parser) parseStringLiteral() ast.Expression {
	return &ast.StringLiteral{Token: p.curToken, Value: p.curToken.Literal}
}

type fstringPart struct {
	isExpr bool
	text   string
}

func (p *Parser) parseFStringLiteral() ast.Expression {
	tok := p.curToken
	s := tok.Literal
	var parts []fstringPart
	var lit strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '{' {
			if i+1 < len(s) && s[i+1] == '{' {
				lit.WriteByte('{')
				i += 2
				continue
			}
			if lit.Len() > 0 {
				parts = append(parts, fstringPart{isExpr: false, text: lit.String()})
				lit.Reset()
			}
			i++ // skip '{'
			start := i
			depth := 1
			for i < len(s) && depth > 0 {
				ch := s[i]
				if ch == '"' || ch == '\'' || ch == '`' {
					q := ch
					i++
					for i < len(s) && s[i] != q {
						if s[i] == '\\' && i+1 < len(s) {
							i += 2
							continue
						}
						i++
					}
					if i < len(s) {
						i++ // consume closing quote
					}
					continue
				}
				if ch == '{' {
					depth++
				} else if ch == '}' {
					depth--
					if depth == 0 {
						break
					}
				}
				i++
			}
			if depth != 0 {
				p.reportAt(tok, "f-string", "unclosed '{' in f-string")
				return nil
			}
			exprStr := s[start:i]
			i++ // skip closing '}'
			parts = append(parts, fstringPart{isExpr: true, text: strings.TrimSpace(exprStr)})
			continue
		}
		if s[i] == '}' {
			if i+1 < len(s) && s[i+1] == '}' {
				lit.WriteByte('}')
				i += 2
				continue
			}
			p.reportAt(tok, "f-string", "single '}' encountered in f-string; use '}}' for literal brace")
			return nil
		}
		lit.WriteByte(s[i])
		i++
	}
	if lit.Len() > 0 {
		parts = append(parts, fstringPart{isExpr: false, text: lit.String()})
	}

	if len(parts) == 0 {
		return &ast.StringLiteral{Token: tok, Value: ""}
	}

	var exprs []ast.Expression
	for _, part := range parts {
		if !part.isExpr {
			exprs = append(exprs, &ast.StringLiteral{Token: tok, Value: part.text})
		} else {
			if part.text == "" {
				p.reportAt(tok, "f-string", "empty expression in f-string '{}'")
				return nil
			}
			subLex := lexer.New(part.text)
			subP := New(subLex)
			expr := subP.parseExpression(LOWEST)
			if len(subP.Errors()) > 0 {
				p.reportAt(tok, "f-string", fmt.Sprintf("invalid expression in f-string '{%s}': %s", part.text, strings.Join(subP.Errors(), ", ")))
				return nil
			}
			exprs = append(exprs, &ast.CallExpression{
				Token:     tok,
				Function:  &ast.Identifier{Token: token.Token{Type: token.IDENT, Literal: "string", Line: tok.Line, Column: tok.Column}, Value: "string"},
				Arguments: []ast.Expression{expr},
			})
		}
	}

	result := exprs[0]
	for j := 1; j < len(exprs); j++ {
		result = &ast.InfixExpression{
			Token:    tok,
			Left:     result,
			Operator: "+",
			Right:    exprs[j],
		}
	}
	return result
}

func (p *Parser) parseBooleanLiteral() ast.Expression {
	return &ast.BooleanLiteral{Token: p.curToken, Value: p.curTokenIs(token.TRUE)}
}

func (p *Parser) parseNullLiteral() ast.Expression {
	return &ast.NullLiteral{Token: p.curToken}
}

func (p *Parser) parsePrefixExpression() ast.Expression {
	expression := &ast.PrefixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal,
	}

	p.nextToken()

	expression.Right = p.parseExpression(PREFIX)

	return expression
}

func (p *Parser) parseGroupedExpression() ast.Expression {
	openTok := p.curToken
	p.nextToken()
	p.skipNewlines()

	exp := p.parseExpression(LOWEST)

	p.skipNewlines()
	if !p.expectPeek(token.RPAREN) {
		if len(p.syntaxErrors) > 0 {
			p.syntaxErrors[len(p.syntaxErrors)-1].Suggestion = fmt.Sprintf("unclosed '(' from line %d, col %d; add a closing ')'", openTok.Line, openTok.Column)
		}
		return nil
	}

	return exp
}

func (p *Parser) parseListLiteral() ast.Expression {
	list := &ast.ListLiteral{Token: p.curToken}
	list.Elements = p.parseExpressionList(token.RBRACKET)
	return list
}

func (p *Parser) parseMapLiteral() ast.Expression {
	mapLit := &ast.MapLiteral{Token: p.curToken}
	mapLit.Pairs = []*ast.MapPair{}

	p.nextToken() // past '{'
	p.skipNewlines()

	closed := false
	if p.curTokenIs(token.RBRACE) {
		closed = true
	}

	for !p.curTokenIs(token.RBRACE) && !p.curTokenIs(token.EOF) {
		p.skipNewlines()
		if p.curTokenIs(token.RBRACE) {
			closed = true
			break
		}

		key := p.parseExpression(LOWEST)
		if key == nil {
			return nil
		}

		if !p.expectPeek(token.COLON) {
			return nil
		}
		colonTok := p.curToken

		p.nextToken() // past ':' to value
		p.skipNewlines()

		val := p.parseExpression(LOWEST)
		if val == nil {
			return nil
		}

		mapLit.Pairs = append(mapLit.Pairs, &ast.MapPair{
			Token: colonTok,
			Key:   key,
			Value: val,
		})

		p.skipPeekNewlines()
		if p.peekTokenIs(token.COMMA) {
			p.nextToken() // ','
			p.skipPeekNewlines()
			if p.peekTokenIs(token.RBRACE) {
				p.nextToken() // '}'
				closed = true
				break
			}
			p.nextToken() // next key
		} else {
			break
		}
	}

	p.skipPeekNewlines()
	if !closed {
		if !p.expectPeek(token.RBRACE) {
			if len(p.syntaxErrors) > 0 {
				p.syntaxErrors[len(p.syntaxErrors)-1].Suggestion = fmt.Sprintf("unclosed '{' from line %d, col %d; add a closing '}'", mapLit.Token.Line, mapLit.Token.Column)
			}
		}
	}

	return mapLit
}

func (p *Parser) parseErrExpression() ast.Expression {
	tok := p.curToken // 'err'
	if !p.peekTokenIs(token.LPAREN) {
		return &ast.ErrExpression{Token: tok}
	}

	p.nextToken() // '('
	p.nextToken() // message start

	var msg ast.Expression
	var code ast.Expression

	if !p.curTokenIs(token.RPAREN) {
		msg = p.parseExpression(LOWEST)
		if p.peekTokenIs(token.COMMA) {
			p.nextToken() // ','
			p.nextToken() // code
			code = p.parseExpression(LOWEST)
			// 'code=1' is a keyword argument, not an assignment to a variable named 'code'.
			if as, ok := code.(*ast.AssignExpression); ok {
				if id, ok := as.Target.(*ast.Identifier); ok && id.Value == "code" {
					code = as.Value
				}
			}
		}
		p.expectPeek(token.RPAREN)
	}

	return &ast.ErrExpression{
		Token:   tok,
		Message: msg,
		Code:    code,
	}
}

func (p *Parser) parseLambdaExpression() ast.Expression {
	tok := p.curToken // '|'
	p.nextToken()     // past '|'

	var params []*ast.FuncParam
	for !p.curTokenIs(token.PIPE_TYPE) && !p.curTokenIs(token.EOF) {
		paramTok := p.curToken
		paramName := paramTok.Literal
		param := &ast.FuncParam{
			Token: paramTok,
			Name:  paramName,
		}

		if p.peekTokenIs(token.COLON) {
			p.nextToken() // ':'
			p.nextToken() // type
			param.TypeAnnotation = p.parseTypeAnnotationWithUnion(false)
		}

		if p.peekTokenIs(token.ASSIGN) {
			p.nextToken() // '='
			p.nextToken() // default
			param.DefaultValue = p.parseExpression(LOWEST)
		}

		params = append(params, param)

		if p.peekTokenIs(token.COMMA) {
			p.nextToken() // ','
			p.nextToken() // next param
		} else {
			if p.peekTokenIs(token.PIPE_TYPE) {
				p.nextToken() // move to '|'
			}
			break
		}
	}

	if !p.curTokenIs(token.PIPE_TYPE) {
		p.peekError(token.PIPE_TYPE)
		return nil
	}

	p.nextToken() // past closing '|' to body
	body := p.parseExpression(LOWEST)

	return &ast.LambdaExpression{
		Token:  tok,
		Params: params,
		Body:   body,
	}
}

func (p *Parser) parsePrefixRangeLiteral() ast.Expression {
	tok := p.curToken // '..' or '..='
	inclusive := (tok.Type == token.DOTDOTEQ)
	p.nextToken()
	end := p.parseExpression(RANGE)
	return &ast.RangeLiteral{
		Token:     tok,
		Start:     nil,
		End:       end,
		Inclusive: inclusive,
	}
}

func (p *Parser) parseExpressionList(end token.TokenType) []ast.Expression {
	var list []ast.Expression

	p.skipPeekNewlines()
	if p.peekTokenIs(end) {
		p.nextToken()
		return list
	}

	p.nextToken() // move past delimiter to first expr
	p.skipNewlines()
	if p.curTokenIs(end) {
		return list
	}

	list = append(list, p.parseExpression(LOWEST))

	for p.peekTokenIs(token.COMMA) {
		p.nextToken() // ','
		p.skipPeekNewlines()
		if p.peekTokenIs(end) {
			p.nextToken()
			return list
		}
		p.nextToken() // next expr
		p.skipNewlines()
		if p.curTokenIs(end) {
			return list
		}
		list = append(list, p.parseExpression(LOWEST))
	}

	p.skipPeekNewlines()
	if !p.expectPeek(end) {
		return nil
	}

	return list
}

func (p *Parser) parseInfixExpression(left ast.Expression) ast.Expression {
	expression := &ast.InfixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal,
		Left:     left,
	}

	precedence := p.curPrecedence()
	p.nextToken()
	p.skipNewlines()
	expression.Right = p.parseExpression(precedence)

	return expression
}

func (p *Parser) parsePowerExpression(left ast.Expression) ast.Expression {
	tok := p.curToken
	p.nextToken()
	p.skipNewlines()
	right := p.parseExpression(POWER - 1)
	return &ast.InfixExpression{
		Token:    tok,
		Left:     left,
		Operator: tok.Literal,
		Right:    right,
	}
}

func (p *Parser) parseAssignExpression(left ast.Expression) ast.Expression {
	tok := p.curToken
	p.nextToken()
	p.skipNewlines()
	right := p.parseExpression(ASSIGN - 1)
	return &ast.AssignExpression{
		Token:  tok,
		Target: left,
		Value:  right,
	}
}

func (p *Parser) parsePipeExpression(left ast.Expression) ast.Expression {
	tok := p.curToken
	p.nextToken()
	p.skipNewlines()
	right := p.parseExpression(PIPE)
	return &ast.PipeExpression{
		Token: tok,
		Left:  left,
		Right: right,
	}
}

func (p *Parser) parseTimeoutExpression(left ast.Expression) ast.Expression {
	tok := p.curToken
	p.nextToken()
	p.skipNewlines()
	duration := p.parseExpression(TIMEOUT)
	return &ast.TimeoutExpression{
		Token:      tok,
		Expression: left,
		Expr:       left,
		Duration:   duration,
	}
}

func (p *Parser) parseStreamExpression(left ast.Expression) ast.Expression {
	tok := p.curToken
	op := tok.Literal
	p.nextToken()
	p.skipNewlines()
	target := p.parseExpression(STREAM)
	return &ast.StreamExpression{
		Token:    tok,
		Source:   left,
		Operator: op,
		Target:   target,
	}
}

func (p *Parser) parseRangeExpression(left ast.Expression) ast.Expression {
	tok := p.curToken // '..' or '..='
	inclusive := (tok.Type == token.DOTDOTEQ)
	p.nextToken()
	p.skipNewlines()
	right := p.parseExpression(RANGE)
	return &ast.RangeLiteral{
		Token:     tok,
		Start:     left,
		End:       right,
		Inclusive: inclusive,
	}
}

func (p *Parser) parseCallExpression(fn ast.Expression) ast.Expression {
	tok := p.curToken // '('
	args, kwargs := p.parseCallArguments()
	return &ast.CallExpression{
		Token:     tok,
		Function:  fn,
		Arguments: args,
		KwArgs:    kwargs,
	}
}

func (p *Parser) parseCallArguments() ([]ast.Expression, map[string]ast.Expression) {
	args := []ast.Expression{}
	kwargs := make(map[string]ast.Expression)

	p.skipPeekNewlines()
	if p.peekTokenIs(token.RPAREN) {
		p.nextToken() // ')'
		return args, kwargs
	}

	p.nextToken() // first arg
	p.skipNewlines()

	for !p.curTokenIs(token.RPAREN) && !p.curTokenIs(token.EOF) {
		if p.curTokenIs(token.IDENT) && p.peekTokenIs(token.ASSIGN) {
			kwName := p.curToken.Literal
			p.nextToken() // '='
			p.nextToken() // value
			val := p.parseExpression(LOWEST)
			kwargs[kwName] = val
		} else {
			arg := p.parseExpression(LOWEST)
			if assign, ok := arg.(*ast.AssignExpression); ok {
				if ident, isIdent := assign.Target.(*ast.Identifier); isIdent {
					kwargs[ident.Value] = assign.Value
				} else {
					args = append(args, arg)
				}
			} else {
				args = append(args, arg)
			}
		}

		p.skipPeekNewlines()
		if p.peekTokenIs(token.COMMA) {
			p.nextToken() // ','
			p.skipPeekNewlines()
			if p.peekTokenIs(token.RPAREN) {
				p.nextToken() // ')'
				return args, kwargs
			}
			p.nextToken() // next arg
			p.skipNewlines()
		} else {
			break
		}
	}

	p.skipPeekNewlines()
	if !p.expectPeek(token.RPAREN) {
		return nil, nil
	}

	return args, kwargs
}

func (p *Parser) parseIndexExpression(left ast.Expression) ast.Expression {
	tok := p.curToken // '['
	p.nextToken()     // past '['
	p.skipNewlines()

	index := p.parseExpression(LOWEST)

	p.skipNewlines()
	if !p.expectPeek(token.RBRACKET) {
		if len(p.syntaxErrors) > 0 {
			p.syntaxErrors[len(p.syntaxErrors)-1].Suggestion = fmt.Sprintf("unclosed '[' from line %d, col %d; add a closing ']'", tok.Line, tok.Column)
		}
		return nil
	}

	return &ast.IndexExpression{
		Token: tok,
		Left:  left,
		Index: index,
	}
}

func (p *Parser) parseDotExpression(left ast.Expression) ast.Expression {
	tok := p.curToken // '.'
	p.nextToken()     // field identifier

	if !p.curTokenIs(token.IDENT) && !token.IsKeyword(p.curToken.Type) && !token.IsTypeKeyword(p.curToken.Type) {
		p.reportAt(p.curToken, "provide a valid field or method name, e.g. 'obj.property'",
			fmt.Sprintf("expected identifier after '.', got %q", p.curToken.Literal))
		return nil
	}

	fieldName := p.curToken.Literal
	return &ast.DotExpression{
		Token: tok,
		Left:  left,
		Field: fieldName,
	}
}
