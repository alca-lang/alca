p = r'C:\Users\geekom\Desktop\архив\Pawl\pkg\parser\parser.go'
s = open(p, encoding='utf-8', newline='').read()
s = s.replace('for j := 1;\n\tj < len(exprs);\n\tj++ { }', 'for j := 1; j < len(exprs); j++ {')
s = s.replace('if id, ok := as.Target.(*ast.Identifier);\n\t\t\t\tok && id.Value ==  }"code" {',
              'if id, ok := as.Target.(*ast.Identifier); ok && id.Value == "code" {')
s = s.replace('if ident, isIdent := assign.Target.(*ast.Identifier);\n\t\t\t\tisIdent { }',
              'if ident, isIdent := assign.Target.(*ast.Identifier); isIdent {')
s = s.replace('\n}"\n', '\n')
open(p, 'w', encoding='utf-8', newline='').write(s)
