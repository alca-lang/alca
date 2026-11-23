import os, sys
sys.path.insert(0, os.path.dirname(__file__))
from migrate_embedded import decode_go

root = sys.argv[1]
bad = []
for dirpath, _, names in os.walk(root):
    for name in names:
        if not name.endswith('.go'):
            continue
        p = os.path.join(dirpath, name)
        s = open(p, encoding='utf-8-sig').read()
        lines = s.splitlines(keepends=True)
        suspect = 0
        for ln in lines:
            if ln.count('\\n') >= 3 and ('\\t' in ln or 'tok.' in ln or 'p.' in ln or 'func ' in ln):
                suspect += 1
        if suspect:
            bad.append((p, suspect))
for p, n in bad:
    print(p, n)
