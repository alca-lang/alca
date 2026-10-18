package token

import (
	"testing"
)

func TestLookupIdent(t *testing.T) {
	tests := []struct {
		input    string
		expected TokenType
	}{
		// Keywords
		{"func", FUNC},
		{"shfunc", SHFUNC},
		{"return", RETURN},
		{"if", IF},
		{"else", ELSE},
		{"elif", ELIF},
		{"while", WHILE},
		{"for", FOR},
		{"loop", LOOP},
		{"foreach", FOREACH},
		{"in", IN},
		{"break", BREAK},
		{"continue", CONTINUE},
		{"and", AND},
		{"or", OR},
		{"not", NOT},
		{"true", TRUE},
		{"false", FALSE},
		{"null", NULL},
		{"struct", STRUCT},
		{"pub", PUB},
		{"use", USE},
		{"as", AS},
		{"ba", BA},
		{"threads", THREADS},
		{"wait", WAIT},
		{"watch", WATCH},
		{"test", TEST},
		{"mock", MOCK},
		{"assert", ASSERT},
		{"tx", TX},
		{"match", MATCH},
		{"case", CASE},
		{"err", ERR},
		{"import", IMPORT},

		// Type Keywords
		{"string", TYPE_STRING},
		{"int", TYPE_INT},
		{"float", TYPE_FLOAT},
		{"bool", TYPE_BOOL},
		{"list", TYPE_LIST},
		{"map", TYPE_MAP},
		{"blob", TYPE_BLOB},
		{"secret", TYPE_SECRET},
		{"json", TYPE_JSON},

		// Identifiers
		{"myVar", IDENT},
		{"calculate_total", IDENT},
		{"Foo123", IDENT},
	}

	for _, tt := range tests {
		tok := LookupIdent(tt.input)
		if tok != tt.expected {
			t.Errorf("LookupIdent(%q) = %q, want %q", tt.input, tok, tt.expected)
		}
	}
}

func TestIsKeyword(t *testing.T) {
	if !IsKeyword(FUNC) {
		t.Errorf("expected FUNC to be recognized as keyword")
	}
	if !IsKeyword(MATCH) {
		t.Errorf("expected MATCH to be recognized as keyword")
	}
	if IsKeyword(IDENT) {
		t.Errorf("expected IDENT not to be recognized as keyword")
	}
	if IsKeyword(TYPE_STRING) {
		t.Errorf("expected TYPE_STRING not to be recognized as standard statement keyword")
	}
}

func TestIsTypeKeyword(t *testing.T) {
	if !IsTypeKeyword(TYPE_STRING) {
		t.Errorf("expected TYPE_STRING to be recognized as type keyword")
	}
	if !IsTypeKeyword(TYPE_JSON) {
		t.Errorf("expected TYPE_JSON to be recognized as type keyword")
	}
	if IsTypeKeyword(FUNC) {
		t.Errorf("expected FUNC not to be recognized as type keyword")
	}
	if IsTypeKeyword(IDENT) {
		t.Errorf("expected IDENT not to be recognized as type keyword")
	}
}

func TestTokenString(t *testing.T) {
	tok := Token{
		Type:    IDENT,
		Literal: "foobar",
		Line:    12,
		Column:  5,
	}
	expected := `Token(IDENT, "foobar", 12:5)`
	if tok.String() != expected {
		t.Errorf("tok.String() = %q, want %q", tok.String(), expected)
	}
}

func TestSuffixConstants(t *testing.T) {
	if SuffixMS != "ms" || SuffixS != "s" || SuffixM != "m" || SuffixH != "h" {
		t.Errorf("duration suffix constants mismatch")
	}
	if SuffixKB != "KB" || SuffixMB != "MB" || SuffixGB != "GB" {
		t.Errorf("size suffix constants mismatch")
	}
}
