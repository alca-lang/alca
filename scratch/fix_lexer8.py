p = r'C:\Users\geekom\Desktop\архив\Pawl\pkg\lexer\lexer.go'
lines = open(p, encoding='utf-8', newline='').read().splitlines(keepends=True)
out = []
i = 0
while i < len(lines):
    ln = lines[i]
    stripped = ln.strip()
    if (stripped.startswith('case ') or stripped == 'default') and stripped.endswith('{'):
        out.append(ln.replace('{', ':', 1)); i += 1; continue
    if stripped.startswith('Literal:') and stripped.endswith('}'):
        nxt = lines[i+1].strip() if i+1 < len(lines) else ''
        if nxt.startswith('"') and nxt.endswith(','):
            indent = ln[:len(ln)-len(ln.lstrip())]
            out.append(indent + 'Literal: ' + nxt + '\n')
            i += 2
            continue
    out.append(ln); i += 1
open(p, 'w', encoding='utf-8', newline='').write(''.join(out))
