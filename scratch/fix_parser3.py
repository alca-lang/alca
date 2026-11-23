p = r'C:\Users\geekom\Desktop\архив\Pawl\pkg\parser\parser.go'
lines = open(p, encoding='utf-8', newline='').read().splitlines(keepends=True)
lines = [ln for i, ln in enumerate(lines) if i+1 not in (2626, 3094)]
open(p, 'w', encoding='utf-8', newline='').write(''.join(lines))
