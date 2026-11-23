p = r'C:\Users\geekom\Desktop\архив\Pawl\pkg\lexer\lexer.go'
lines = open(p, encoding='utf-8', newline='').read().splitlines(keepends=True)
out = []
for ln in lines:
    if ln.strip() == 'default {':
        out.append(ln.replace('{', ':', 1)); continue
    out.append(ln)
# remove the extra closing brace right before `return tok` at end of NextToken
for i, ln in enumerate(out):
    if ln.strip() == 'return tok' and i > 0 and out[i-1].strip() == '}' and out[i-2].strip() == '}':
        del out[i-1]
        break
open(p, 'w', encoding='utf-8', newline='').write(''.join(out))
