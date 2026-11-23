p = r'C:\Users\geekom\Desktop\архив\Pawl\pkg\parser\parser.go'
s = open(p, encoding='utf-8', newline='').read()
s = s.replace('{=', ':=')
s = s.replace("if s[i] == '\\' && i+1 < len(s) {", "if s[i] == '\\\\' && i+1 < len(s) {")
# repair semicolon-split `if x, ok := ...; ok {` constructs
s = s.replace(');\n\t\t\tok { }', '); ok {')
s = s.replace(');\n\t\t\t\tok { }', '); ok {')
s = s.replace('ok {= as.Target.(*ast.Identifier);\n\t\t\t\tok && id.Value ==  }"code" {',
              'ok := as.Target.(*ast.Identifier); ok && id.Value == "code" {')
s = s.replace('if id, ok {= as.Target.(*ast.Identifier);\n\t\t\t\tok && id.Value ==  }"code" {',
              'if id, ok := as.Target.(*ast.Identifier); ok && id.Value == "code" {')
s = s.replace('for _, part {= range parts { }', 'for _, part := range parts {')
open(p, 'w', encoding='utf-8', newline='').write(s)
