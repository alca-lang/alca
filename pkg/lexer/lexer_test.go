package lexer_test

import (
	"testing"

	"github.com/alca-lang/alca/pkg/lexer"
	"github.com/alca-lang/alca/pkg/token"
)

func TestBasicOperatorsAndDelimiters(t *testing.T) {
	input := `= + - ! * / % ** == != < <= > >= , : ; . ( ) [ ] { } |> | -> ~ ~> @ ..`

	tests := []struct {
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{token.ASSIGN, "="},
		{token.PLUS, "+"},
		{token.MINUS, "-"},
		{token.BANG, "!"},
		{token.ASTERISK, "*"},
		{token.SLASH, "/"},
		{token.PERCENT, "%"},
		{token.POWER, "**"},
		{token.EQ, "=="},
		{token.NOT_EQ, "!="},
		{token.LT, "<"},
		{token.LT_EQ, "<="},
		{token.GT, ">"},
		{token.GT_EQ, ">="},
		{token.COMMA, ","},
		{token.COLON, ":"},
		{token.SEMICOLON, ";"},
		{token.DOT, "."},
		{token.LPAREN, "("},
		{token.RPAREN, ")"},
		{token.LBRACKET, "["},
		{token.RBRACKET, "]"},
		{token.LBRACE, "{"},
		{token.RBRACE, "}"},
		{token.PIPE, "|>"},
		{token.PIPE_TYPE, "|"},
		{token.ARROW, "->"},
		{token.TILDE, "~"},
		{token.TILDE_GT, "~>"},
		{token.AT, "@"},
		{token.RANGE, ".."},
		{token.EOF, ""},
	}

	l := lexer.New(input)
	for i, tt := range tests {
		tok := l.NextToken()
		if tok.Type != tt.expectedType {
			t.Fatalf("test[%d] - token type wrong. expected=%q, got=%q (literal=%q)",
				i, tt.expectedType, tok.Type, tok.Literal)
		}
		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("test[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestStreamOperators(t *testing.T) {
	input := `>> >! >>> << ?>> ?>>> &>> &&>>`

	tests := []struct {
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{token.STREAM_OUT, ">>"},
		{token.STREAM_OVERWRITE, ">!"},
		{token.STREAM_VAR, ">>>"},
		{token.STREAM_IN, "<<"},
		{token.STREAM_ERR, "?>>"},
		{token.STREAM_ERR_VAR, "?>>>"},
		{token.STREAM_ASYNC, "&>>"},
		{token.STREAM_THREAD, "&&>>"},
		{token.EOF, ""},
	}

	l := lexer.New(input)
	for i, tt := range tests {
		tok := l.NextToken()
		if tok.Type != tt.expectedType {
			t.Fatalf("test[%d] - stream operator token type wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}
		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("test[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestKeywordsAndTypes(t *testing.T) {
	input := `func shfunc return if else elif while for loop foreach in break continue and or not true false null struct pub use as ba threads wait watch test mock assert tx match case err import string int float bool list map blob secret json`

	tests := []struct {
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{token.FUNC, "func"},
		{token.SHFUNC, "shfunc"},
		{token.RETURN, "return"},
		{token.IF, "if"},
		{token.ELSE, "else"},
		{token.ELIF, "elif"},
		{token.WHILE, "while"},
		{token.FOR, "for"},
		{token.LOOP, "loop"},
		{token.FOREACH, "foreach"},
		{token.IN, "in"},
		{token.BREAK, "break"},
		{token.CONTINUE, "continue"},
		{token.AND, "and"},
		{token.OR, "or"},
		{token.NOT, "not"},
		{token.TRUE, "true"},
		{token.FALSE, "false"},
		{token.NULL, "null"},
		{token.STRUCT, "struct"},
		{token.PUB, "pub"},
		{token.USE, "use"},
		{token.AS, "as"},
		{token.BA, "ba"},
		{token.THREADS, "threads"},
		{token.WAIT, "wait"},
		{token.WATCH, "watch"},
		{token.TEST, "test"},
		{token.MOCK, "mock"},
		{token.ASSERT, "assert"},
		{token.TX, "tx"},
		{token.MATCH, "match"},
		{token.CASE, "case"},
		{token.ERR, "err"},
		{token.IMPORT, "import"},

		{token.TYPE_STRING, "string"},
		{token.TYPE_INT, "int"},
		{token.TYPE_FLOAT, "float"},
		{token.TYPE_BOOL, "bool"},
		{token.TYPE_LIST, "list"},
		{token.TYPE_MAP, "map"},
		{token.TYPE_BLOB, "blob"},
		{token.TYPE_SECRET, "secret"},
		{token.TYPE_JSON, "json"},
		{token.EOF, ""},
	}

	l := lexer.New(input)
	for i, tt := range tests {
		tok := l.NextToken()
		if tok.Type != tt.expectedType {
			t.Fatalf("test[%d] - keyword token type wrong. expected=%q, got=%q (literal=%q)",
				i, tt.expectedType, tok.Type, tok.Literal)
		}
		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("test[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestNumbersAndSuffixes(t *testing.T) {
	input := `42 3.14 100ms 5s 10m 2h 500KB 16MB 2GB 0.5s 1.5MB 1..10`

	tests := []struct {
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{token.INT, "42"},
		{token.FLOAT, "3.14"},
		{token.INT, "100ms"},
		{token.INT, "5s"},
		{token.INT, "10m"},
		{token.INT, "2h"},
		{token.INT, "500KB"},
		{token.INT, "16MB"},
		{token.INT, "2GB"},
		{token.FLOAT, "0.5s"},
		{token.FLOAT, "1.5MB"},
		{token.INT, "1"},
		{token.RANGE, ".."},
		{token.INT, "10"},
		{token.EOF, ""},
	}

	l := lexer.New(input)
	for i, tt := range tests {
		tok := l.NextToken()
		if tok.Type != tt.expectedType {
			t.Fatalf("test[%d] - number type wrong. expected=%q, got=%q (literal=%q)",
				i, tt.expectedType, tok.Type, tok.Literal)
		}
		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("test[%d] - number literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestStringsAndInterpolation(t *testing.T) {
	input := `"hello world" "with\nnewline" "hello ${name}!" 'single quote'`

	tests := []struct {
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{token.STRING, "hello world"},
		{token.STRING, "with\nnewline"},
		{token.STRING, "hello ${name}!"},
		{token.STRING, "single quote"},
		{token.EOF, ""},
	}

	l := lexer.New(input)
	for i, tt := range tests {
		tok := l.NextToken()
		if tok.Type != tt.expectedType {
			t.Fatalf("test[%d] - string type wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}
		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("test[%d] - string literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestIndentationAndDedent(t *testing.T) {
	input := "if true:\n    x = 1\n    y = 2\nz = 3"

	tests := []struct {
		expectedType token.TokenType
		expectedLit  string
	}{
		{token.IF, "if"},
		{token.TRUE, "true"},
		{token.COLON, ":"},
		{token.NEWLINE, "\n"},
		{token.INDENT, "INDENT"},
		{token.IDENT, "x"},
		{token.ASSIGN, "="},
		{token.INT, "1"},
		{token.NEWLINE, "\n"},
		{token.IDENT, "y"},
		{token.ASSIGN, "="},
		{token.INT, "2"},
		{token.NEWLINE, "\n"},
		{token.DEDENT, "DEDENT"},
		{token.IDENT, "z"},
		{token.ASSIGN, "="},
		{token.INT, "3"},
		{token.EOF, ""},
	}

	l := lexer.New(input)
	for i, tt := range tests {
		tok := l.NextToken()
		if tok.Type != tt.expectedType {
			t.Fatalf("test[%d] - wrong type. expected=%q, got=%q (literal=%q)",
				i, tt.expectedType, tok.Type, tok.Literal)
		}
		if tt.expectedLit != "" && tok.Literal != tt.expectedLit {
			t.Fatalf("test[%d] - wrong literal. expected=%q, got=%q",
				i, tt.expectedLit, tok.Literal)
		}
	}
}

func TestNestedIndentationAndEOFDedent(t *testing.T) {
	input := "if a:\n    if b:\n        c = 1"

	tests := []struct {
		expectedType token.TokenType
	}{
		{token.IF},
		{token.IDENT},
		{token.COLON},
		{token.NEWLINE},
		{token.INDENT},
		{token.IF},
		{token.IDENT},
		{token.COLON},
		{token.NEWLINE},
		{token.INDENT},
		{token.IDENT},
		{token.ASSIGN},
		{token.INT},
		{token.DEDENT},
		{token.DEDENT},
		{token.EOF},
	}

	l := lexer.New(input)
	for i, tt := range tests {
		tok := l.NextToken()
		if tok.Type != tt.expectedType {
			t.Fatalf("test[%d] - wrong type. expected=%q, got=%q (literal=%q)",
				i, tt.expectedType, tok.Type, tok.Literal)
		}
	}
}

func TestCommentsAndEmptyLines(t *testing.T) {
	input := `// Initial comment
fn myFunc():
    // Comment inside function

    x = 10 // trailing comment
    /* multi
       line
       comment */
    return x`

	l := lexer.New(input)
	tokens := l.Tokenize()

	expectedTypes := []token.TokenType{
		token.FUNC,
		token.IDENT,
		token.LPAREN,
		token.RPAREN,
		token.COLON,
		token.NEWLINE,
		token.INDENT,
		token.IDENT,
		token.ASSIGN,
		token.INT,
		token.NEWLINE,
		token.RETURN,
		token.IDENT,
		token.DEDENT,
		token.EOF,
	}

	if len(tokens) != len(expectedTypes) {
		t.Fatalf("expected %d tokens, got %d tokens: %#v", len(expectedTypes), len(tokens), tokens)
	}

	for i, expected := range expectedTypes {
		if tokens[i].Type != expected {
			t.Fatalf("token[%d] expected %q, got %q (literal=%q)",
				i, expected, tokens[i].Type, tokens[i].Literal)
		}
	}
}

func TestWindowsLineEndings(t *testing.T) {
	input := "x = 1\r\ny = 2\r\n"

	l := lexer.New(input)
	tokens := l.Tokenize()

	expectedTypes := []token.TokenType{
		token.IDENT,
		token.ASSIGN,
		token.INT,
		token.NEWLINE,
		token.IDENT,
		token.ASSIGN,
		token.INT,
		token.NEWLINE,
		token.EOF,
	}

	if len(tokens) != len(expectedTypes) {
		t.Fatalf("expected %d tokens, got %d", len(expectedTypes), len(tokens))
	}

	for i, expected := range expectedTypes {
		if tokens[i].Type != expected {
			t.Fatalf("token[%d] expected %q, got %q", i, expected, tokens[i].Type)
		}
	}
}

func TestTabIndentation(t *testing.T) {
	input := "if true:\n\tx = 1\n\ty = 2\nz = 3"

	l := lexer.New(input)
	tokens := l.Tokenize()

	expectedTypes := []token.TokenType{
		token.IF,
		token.TRUE,
		token.COLON,
		token.NEWLINE,
		token.INDENT,
		token.IDENT,
		token.ASSIGN,
		token.INT,
		token.NEWLINE,
		token.IDENT,
		token.ASSIGN,
		token.INT,
		token.NEWLINE,
		token.DEDENT,
		token.IDENT,
		token.ASSIGN,
		token.INT,
		token.EOF,
	}

	if len(tokens) != len(expectedTypes) {
		t.Fatalf("expected %d tokens, got %d", len(expectedTypes), len(tokens))
	}

	for i, expected := range expectedTypes {
		if tokens[i].Type != expected {
			t.Fatalf("token[%d] expected %q, got %q", i, expected, tokens[i].Type)
		}
	}
}

func TestLineAndColumnPositions(t *testing.T) {
	input := "a = 10\nb = 20"

	l := lexer.New(input)

	tok := l.NextToken()
	if tok.Literal != "a" || tok.Line != 1 || tok.Column != 1 {
		t.Fatalf("unexpected pos for 'a': %d:%d, literal=%q", tok.Line, tok.Column, tok.Literal)
	}

	tok = l.NextToken()
	if tok.Literal != "=" || tok.Line != 1 || tok.Column != 3 {
		t.Fatalf("unexpected pos for '=': %d:%d, literal=%q", tok.Line, tok.Column, tok.Literal)
	}

	tok = l.NextToken()
	if tok.Literal != "10" || tok.Line != 1 || tok.Column != 5 {
		t.Fatalf("unexpected pos for '10': %d:%d, literal=%q", tok.Line, tok.Column, tok.Literal)
	}

	tok = l.NextToken()
	if tok.Type != token.NEWLINE || tok.Line != 1 || tok.Column != 7 {
		t.Fatalf("unexpected pos for NEWLINE: %d:%d", tok.Line, tok.Column)
	}

	tok = l.NextToken()
	if tok.Literal != "b" || tok.Line != 2 || tok.Column != 1 {
		t.Fatalf("unexpected pos for 'b': %d:%d, literal=%q", tok.Line, tok.Column, tok.Literal)
	}
}

func TestFloatDotPrefix(t *testing.T) {
	input := ".5 .75s"

	l := lexer.New(input)
	tok1 := l.NextToken()
	if tok1.Type != token.FLOAT || tok1.Literal != ".5" {
		t.Fatalf("expected .5 FLOAT, got %q (%q)", tok1.Type, tok1.Literal)
	}

	tok2 := l.NextToken()
	if tok2.Type != token.FLOAT || tok2.Literal != ".75s" {
		t.Fatalf("expected .75s FLOAT, got %q (%q)", tok2.Type, tok2.Literal)
	}
}

func TestTripleQuotedStrings(t *testing.T) {
	input := `html = """
<html>
    <button onclick="alert('hi')">Click</button>
</html>
"""`
	l := lexer.New(input)
	tok1 := l.NextToken() // ident 'html'
	if tok1.Literal != "html" {
		t.Fatalf("expected ident 'html', got %v", tok1)
	}
	tok2 := l.NextToken() // '='
	if tok2.Type != token.ASSIGN {
		t.Fatalf("expected '=', got %v", tok2)
	}
	tok3 := l.NextToken() // STRING
	if tok3.Type != token.STRING {
		t.Fatalf("expected STRING token, got %v", tok3)
	}
	expectedContent := "\n<html>\n    <button onclick=\"alert('hi')\">Click</button>\n</html>\n"
	if tok3.Literal != expectedContent {
		t.Fatalf("expected literal %q, got %q", expectedContent, tok3.Literal)
	}
}

func TestRawBacktickStrings(t *testing.T) {
	input := "pat = `\\b\\d{1,3}\\.\\d{1,3}\\b`"
	l := lexer.New(input)
	_ = l.NextToken() // pat
	_ = l.NextToken() // =
	tok := l.NextToken()
	if tok.Type != token.STRING {
		t.Fatalf("expected STRING token, got %v", tok)
	}
	expected := `\b\d{1,3}\.\d{1,3}\b`
	if tok.Literal != expected {
		t.Fatalf("expected raw literal %q, got %q", expected, tok.Literal)
	}
}
