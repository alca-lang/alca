p = r'C:\Users\geekom\Desktop\архив\Pawl\pkg\lexer\lexer.go'
lines = open(p, encoding='utf-8', newline='').read().splitlines(keepends=True)
for i, ln in enumerate(lines):
    if ln.strip() == "if l.ch == '\r' {".strip() or ln.strip() == "if l.ch == '\n' {":
        pass
    if "if l.ch == '" in ln and ln.count('\n') > 1:
        lines[i] = "\tif l.ch == '\\r' {\n"
    if "if l.peekChar() == '" in ln and ln.count('\n') > 1:
        lines[i] = "\t\tif l.peekChar() == '\\n' {\n"
open(p, 'w', encoding='utf-8', newline='').write(''.join(lines))
