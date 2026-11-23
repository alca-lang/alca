p = r'C:\Users\geekom\Desktop\архив\Pawl\pkg\lexer\lexer.go'
lines = open(p, encoding='utf-8', newline='').read().splitlines(keepends=True)

# line number (1-based) -> replacement for the broken first line
fix = {
    613: "\t\tif l.ch == '\\n' {\n",
    615: "\t\t\tif l.peekChar() == '\\r' {\n",
    622: "\t\t} else if l.ch == '\\r' {\n",
    749: "\t\t\tif l.ch == '\\n' {\n",
}
out = []
i = 0
while i < len(lines):
    num = i + 1
    if num in fix:
        out.append(fix[num])
        # skip the stray "' {" continuation line(s)
        i += 1
        while i < len(lines) and lines[i].strip() in ("' {",):
            i += 1
        continue
    if num == 714:
        out.append("// readString parses string literals with escape sequences (\\n, \\t, \\r, \\\", \\\\)\n")
        i += 1
        while i < len(lines) and (lines[i].strip().startswith(', ') or lines[i].strip().startswith("', ")):
            i += 1
        continue
    out.append(lines[i]); i += 1
open(p, 'w', encoding='utf-8', newline='').write(''.join(out))
