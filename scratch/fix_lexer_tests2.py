p = r'C:\Users\geekom\Desktop\архив\Pawl\pkg\lexer\lexer_test.go'
s = open(p, encoding='utf-8', newline='').read()
# basic operators: restore colon + newline pair
s = s.replace('\t\t{token.LBRACE, "{"},\n\t\t{token.NEWLINE, "\\n"},\n\t\t{token.DOT, "."},',
              '\t\t{token.COLON, ":"},\n\t\t{token.NEWLINE, "\\n"},\n\t\t{token.DOT, "."},')
# indentation test: add rbrace + newline after dedent
s = s.replace('''		{token.NEWLINE, "\\n"},
		{token.DEDENT, "DEDENT"},
		{token.IDENT, "z"},''', '''		{token.NEWLINE, "\\n"},
		{token.DEDENT, "DEDENT"},
		{token.RBRACE, "}"},
		{token.NEWLINE, "\\n"},
		{token.IDENT, "z"},''')
# nested test
s = s.replace('''		{token.IDENT},
		{token.ASSIGN},
		{token.INT},
		{token.DEDENT},
		{token.DEDENT},
		{token.EOF},''', '''		{token.IDENT},
		{token.ASSIGN},
		{token.INT},
		{token.NEWLINE},
		{token.DEDENT},
		{token.RBRACE},
		{token.NEWLINE},
		{token.DEDENT},
		{token.RBRACE},
		{token.NEWLINE},
		{token.EOF},''')
open(p, 'w', encoding='utf-8', newline='').write(s)
print('done')
