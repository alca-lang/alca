p = r'C:\Users\geekom\Desktop\архив\Pawl\pkg\lexer\lexer.go'
s = open(p, encoding='utf-8', newline='').read()
s = s.replace("out.WriteByte('\\')", "out.WriteByte('\\\\')")
open(p, 'w', encoding='utf-8', newline='').write(s)
