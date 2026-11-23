import sys
p = sys.argv[1]
lines = open(p, encoding='utf-8', newline='').read().splitlines()
for i, ln in enumerate(lines):
    if ln.rstrip().endswith("'") and i + 1 < len(lines) and lines[i+1].strip() in ("' {", "' && l.ch != 0 {"):
        print(i+1, repr(ln), '||', repr(lines[i+1]))
    if ln.strip().startswith('//') and i + 1 < len(lines) and lines[i+1].strip().startswith(', '):
        print('COMMENT', i+1, repr(ln), '||', repr(lines[i+1]))
