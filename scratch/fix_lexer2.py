p = r'C:\Users\geekom\Desktop\архив\Pawl\pkg\lexer\lexer.go'
lines = open(p, encoding='utf-8', newline='').read().splitlines(keepends=True)
# indices are 0-based; rebuild by removing known broken line groups
out = []
i = 0
while i < len(lines):
    ln = lines[i]
    if ln.startswith('\tif l.ch == "') and i + 1 < len(lines) and lines[i+1].strip() == "' {":
        out.append("\tif l.ch == '\\r' {\n"); i += 2; continue
    if ln.startswith('\t\tif l.peekChar() == "') and i + 1 < len(lines) and lines[i+1].strip() == "' {":
        out.append("\t\tif l.peekChar() == '\\n' {\n"); i += 2; continue
    if ln.startswith('\tfor l.ch != "') and i + 2 < len(lines) and lines[i+1].strip() == "' && l.ch != \"" and lines[i+2].strip() == "' && l.ch != 0 {":
        out.append("\tfor l.ch != '\\n' && l.ch != '\\r' && l.ch != 0 {\n"); i += 3; continue
    out.append(ln); i += 1
open(p, 'w', encoding='utf-8', newline='').write(''.join(out))
