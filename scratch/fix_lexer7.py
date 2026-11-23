p = r'C:\Users\geekom\Desktop\архив\Pawl\pkg\lexer\lexer.go'
lines = open(p, encoding='utf-8', newline='').read().splitlines(keepends=True)
# remove the stray closing brace right after the backtick case
for i, ln in enumerate(lines):
    if ln.strip() == "case '`':":
        assert lines[i+6].strip() == '}', repr(lines[i+6])
        del lines[i+6]
        break
open(p, 'w', encoding='utf-8', newline='').write(''.join(lines))
