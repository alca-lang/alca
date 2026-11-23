p = r'C:\Users\geekom\Desktop\архив\Pawl\pkg\lexer\lexer.go'
lines = open(p, encoding='utf-8', newline='').read().splitlines(keepends=True)
out = []
i = 0
while i < len(lines):
    num = i + 1
    ln = lines[i]
    if num == 720:
        out.append("\t\tif l.ch == '\\\\' {\n"); i += 1; continue
    if num in (724, 729):
        esc = 'n' if num == 724 else 'r'
        out.append(f"\t\t\t\tout.WriteByte('\\{esc}')\n")
        i += 2
        continue
    if num == 735:
        out.append("\t\t\tcase '\\\\':\n"); i += 1; continue
    out.append(ln); i += 1
open(p, 'w', encoding='utf-8', newline='').write(''.join(out))
