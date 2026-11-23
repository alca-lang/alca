p = r'C:\Users\geekom\Desktop\архив\Pawl\cmd\pawl\main.go'
lines = open(p, encoding='utf-8', newline='').read().splitlines(keepends=True)
out = []
for i, ln in enumerate(lines):
    if ln.strip() == '}' and i+1 < len(lines) and lines[i+1].strip().startswith('case '):
        continue
    if ln.strip() == '}`':
        out.append('}\n'); continue
    out.append(ln)
open(p, 'w', encoding='utf-8', newline='').write(''.join(out))
