import sys
p = sys.argv[1]
s = open(p, encoding='utf-8-sig').read()
for i, ln in enumerate(s.splitlines(), 1):
    if ln.count('\\n') >= 3 and '\\t' in ln:
        print(i, repr(ln[:300]))
