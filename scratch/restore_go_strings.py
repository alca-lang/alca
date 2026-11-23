import os, difflib, glob

up = os.path.expandvars(r'C:\Users\geekom\AppData\Local\Temp\alca-upstream')
loc = r'C:\Users\geekom\Desktop\архив\Pawl'

def norm(s):
    return (s.replace('github.com/alca-lang/alca', 'github.com/pawl-lang/pawl')
             .replace('Alca', 'Pawl').replace('alca', 'pawl'))

changed = 0
for lf in glob.glob(os.path.join(loc, '**', '*.go'), recursive=True):
    rel = os.path.relpath(lf, loc)
    if rel.endswith('_test.go'):
        continue
    if rel == os.path.join('pkg', 'parser', 'parser.go'):
        continue
    uf = os.path.join(up, rel)
    if not os.path.exists(uf):
        continue
    u = [norm(x) for x in open(uf, encoding='utf-8', errors='replace').read().splitlines()]
    with open(lf, encoding='utf-8', newline='') as f:
        raw = f.read()
    l = raw.splitlines()
    sm = difflib.SequenceMatcher(None, u, l)
    for tag, i1, i2, j1, j2 in sm.get_opcodes():
        if tag != 'replace' or (i2 - i1) != (j2 - j1):
            continue
        for k in range(i2 - i1):
            a, b = u[i1+k], l[j1+k]
            if b == a.replace(';', '\\n') or b == a.replace(':', ' {'):
                l[j1+k] = a
                changed += 1
    if l != raw.splitlines():
        with open(lf, 'w', encoding='utf-8', newline='') as f:
            f.write('\n'.join(l) + ('\n' if raw.endswith('\n') else ''))
        print('fixed', rel)
print('lines changed:', changed)
