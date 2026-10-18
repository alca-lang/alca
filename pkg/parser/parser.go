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
	token.EQ:               EQUALS,
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
	p.registerPrefix(token.RANGE, p.parsePrefixRangeLiteral)
	p.registerPrefix(token.DOTDOT, p.parsePrefixRangeLiteral)

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
	msg := fmt.Sprintf("line %d, col %d: expected next token to be %s, got %s instead",
		p.peekToken.Line, p.peekToken.Column, t, p.peekToken.Type)
	p.errors = append(p.errors, msg)

	length := len(p.peekToken.Literal)
	if length <= 0 {
		length = 1
	}
	p.syntaxErrors = append(p.syntaxErrors, &diag.SyntaxError{
		Filename: "",
		Line:     p.peekToken.Line,
		Column:   p.peekToken.Column,
		Message:  fmt.Sprintf("expected next token to be %s, got %q (%s)", t, p.peekToken.Literal, p.peekToken.Type),
		Token:    string(p.peekToken.Type),
		Length:   length,
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
	msg := fmt.Sprintf("line %d, col %d: no prefix parse function for %s found",
		p.curToken.Line, p.curToken.Column, t)
	p.errors = append(p.errors, msg)

	length := len(p.curToken.Literal)
	if length <= 0 {
		length = 1
	}
	p.syntaxErrors = append(p.syntaxErrors, &diag.SyntaxError{
		Filename: "",
		Line:     p.curToken.Line,
		Column:   p.curToken.Column,
		Message:  fmt.Sprintf("unexpected token %q (%s)", p.curToken.Literal, t),
		Token:    string(t),
		Length:   length,
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

func (p *Parser) parseStatement() ast.Statement {
	var decorators []*ast.Decorator
	for p.curTokenIs(token.AT) {
		dec := p.parseDecorator()
		if dec != nil {
			decorators = append(decorators, dec)
		}
		p.skipNewlines()
	}

	isPublic := false
	if p.curTokenIs(token.PUB) {
		isPublic = true
		p.nextToken() // past 'pub'
	}

	switch p.curToken.Type {
	case token.FUNC, token.SHFUNC:
		return p.parseFuncDeclaration(isPublic, decorators)
	case token.STRUCT:
		return p.parseStructDeclaration(isPublic, decorators)
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
		if token.IsTypeKeyword(p.curToken.Type) && p.peekTokenIs(token.IDENT) {
			// C-style declaration ('int x = 5;') is not part of Alca: reject with a hint, keep parsing.
			p.reportAt(p.curToken, fmt.Sprintf("declare variables as 'name: %s = value;'", p.curToken.Literal),
				"C-style declarations are not supported")
			return p.parseCStyleVarDeclaration(decorators)
		}
		if p.curTokenIs(token.IDENT) && p.peekTokenIs(token.COLON) {
			return p.parseVarDeclaration(decorators)
		}
		return p.parseExpressionStatement()
	default:
		return p.parseExpressionStatement()
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
		p.peekError(token.IDENT)
		return nil
	}

	name := p.curToken.Literal
	var args []ast.Expression

	if p.peekTokenIs(token.LPAREN) {
		p.nextToken() // move to '('
		args = p.parseExpressionList(token.RPAREN)
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

func (p *Parser) parseCStyleVarDeclaration(decorators []*ast.Decorator) *ast.VarDeclaration {
	typeTok := p.curToken
	typeAnnotation := &ast.TypeAnnotation{
		Token: typeTok,
		Types: []string{typeTok.Literal},
	}
	p.nextToken() // move to variable name (IDENT)

	nameTok := p.curToken
	name := &ast.Identifier{Token: nameTok, Value: nameTok.Literal}

	var value ast.Expression
	if p.peekTokenIs(token.ASSIGN) {
		p.nextToken() // move to '='
		p.nextToken() // move to value expression
		value = p.parseExpression(LOWEST)
	}

	p.consumeStatementEnd()

	return &ast.VarDeclaration{
		Token:          typeTok,
		Name:           name,
		TypeAnnotation: typeAnnotation,
		Value:          value,
		Decorators:     decorators,
	}
}

func (p *Parser) parseFuncDeclaration(isPublic bool, decorators []*ast.Decorator) *ast.FuncDeclaration {
	tok := p.curToken // 'func' or 'shfunc'
	isShell := (tok.Type == token.SHFUNC)

	p.nextToken() // past func/shfunc
	name := p.curToken.Literal

	if !p.expectPeek(token.LPAREN) {
		return nil
	}

	params := p.parseFuncParams()

	var returnType *ast.TypeAnnotation
	if p.peekTokenIs(token.ARROW) {
		p.nextToken() // '->'
		p.nextToken() // start of return type
		returnType = p.parseTypeAnnotation()
	}

	p.expectBlockColon("function signature")

	body := p.parseBlockStatement()

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
		p.peekError(token.IDENT)
		return nil
	}

	name := p.curToken.Literal

	p.expectBlockColon("struct " + name)
	if p.curTokenIs(token.COLON) {
		p.nextToken()
	}

	p.skipNewlines()

	var fields []*ast.StructField

	if p.curTokenIs(token.INDENT) {
		p.nextToken() // enter struct body

		for !p.curTokenIs(token.DEDENT) && !p.curTokenIs(token.EOF) {
			prevTok := p.curToken
			p.skipNewlines()
			if p.curTokenIs(token.DEDENT) || p.curTokenIs(token.EOF) {
				break
			}

			fieldTok := p.curToken
			fieldName := fieldTok.Literal

			var typeAnnot *ast.TypeAnnotation
			var defaultVal ast.Expression

			if p.peekTokenIs(token.COLON) {
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
			})

			p.consumeStatementEnd()
			p.skipNewlines()
			if p.curToken == prevTok {
				p.nextToken()
				p.skipNewlines()
			}
		}

		if p.curTokenIs(token.DEDENT) {
			p.nextToken()
		}
	}

	return &ast.StructDeclaration{
		Token:      structTok,
		Name:       name,
		Fields:     fields,
		IsPublic:   isPublic,
		Decorators: decorators,
	}
}

func (p *Parser) parseBlockStatement() *ast.BlockStatement {
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
	} else {
		// Single-line block statement
		if !p.curTokenIs(token.NEWLINE) && !p.curTokenIs(token.EOF) {
			stmt := p.parseStatement()
			if stmt != nil {
				block.Statements = append(block.Statements, stmt)
			}
		}
	}

	return block
}

func (p *Parser) parseIfStatement() *ast.IfStatement {
	ifTok := p.curToken
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

	if p.peekTokenIs(token.SEMICOLON) || p.peekTokenIs(token.NEWLINE) || p.peekTokenIs(token.EOF) || p.peekTokenIs(token.DEDENT) {
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
			if p.peekTokenIs(token.IDENT) {
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

	subject := p.parseExpression(LOWEST)

	if p.peekTokenIs(token.COLON) {
		p.nextToken()
	}
	if p.curTokenIs(token.COLON) {
		p.nextToken()
	}

	p.skipNewlines()

	var cases []*ast.MatchCase
	var defaultBlock *ast.BlockStatement

	if p.curTokenIs(token.INDENT) {
		p.nextToken() // enter match body

		for !p.curTokenIs(token.DEDENT) && !p.curTokenIs(token.EOF) {
			p.skipNewlines()
			if p.curTokenIs(token.DEDENT) || p.curTokenIs(token.EOF) {
				break
			}

			if p.curTokenIs(token.CASE) {
				caseTok := p.curToken
				p.nextToken() // past 'case'

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
			p.skipNewlines()
		}

		if p.curTokenIs(token.DEDENT) {
			p.nextToken()
		}
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
	p.nextToken()
	p.skipNewlines()

	exp := p.parseExpression(LOWEST)

	p.skipNewlines()
	if !p.expectPeek(token.RPAREN) {
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

	for !p.curTokenIs(token.RBRACE) && !p.curTokenIs(token.EOF) {
		p.skipNewlines()
		if p.curTokenIs(token.RBRACE) {
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
			p.nextToken() // next key or '}'
		} else {
			break
		}
	}

	p.skipPeekNewlines()
	if !p.curTokenIs(token.RBRACE) {
		p.expectPeek(token.RBRACE)
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
	tok := p.curToken // '..'
	p.nextToken()
	end := p.parseExpression(RANGE)
	return &ast.RangeLiteral{
		Token: tok,
		Start: nil,
		End:   end,
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
	tok := p.curToken
	p.nextToken()
	p.skipNewlines()
	right := p.parseExpression(RANGE)
	return &ast.RangeLiteral{
		Token: tok,
		Start: left,
		End:   right,
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

	fieldName := p.curToken.Literal
	return &ast.DotExpression{
		Token: tok,
		Left:  left,
		Field: fieldName,
	}
}
