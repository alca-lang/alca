p = r'C:\Users\geekom\Desktop\архив\Pawl\pkg\lexer\lexer.go'
lines = open(p, encoding='utf-8', newline='').read().splitlines(keepends=True)
for i, ln in enumerate(lines):
    if ln.strip() == '}' and i+1 < len(lines) and lines[i+1].strip() == '' and i+2 < len(lines) and lines[i+2].strip() == 'return tok':
        del lines[i]
        break
open(p, 'w', encoding='utf-8', newline='').write(''.join(lines))
