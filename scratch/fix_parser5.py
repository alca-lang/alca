p = r'C:\Users\geekom\Desktop\архив\Pawl\pkg\parser\parser.go'
lines = open(p, encoding='utf-8', newline='').read().splitlines(keepends=True)
# insert the brace closing `for _, part := range parts` right after the else block
idx = next(i for i, ln in enumerate(lines) if 'result := exprs[0]' in ln)
lines.insert(idx, '\t}\n')
open(p, 'w', encoding='utf-8', newline='').write(''.join(lines))
