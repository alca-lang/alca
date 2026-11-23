p = r'C:\Users\geekom\Desktop\архив\Pawl\pkg\parser\parser.go'
lines = open(p, encoding='utf-8', newline='').read().splitlines(keepends=True)
del lines[2636]  # line 2637 (1-based), stray closing brace
open(p, 'w', encoding='utf-8', newline='').write(''.join(lines))
