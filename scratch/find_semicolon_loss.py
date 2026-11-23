import os, difflib, glob

up = os.path.expandvars(r'C:\Users\geekom\AppData\Local\Temp\alca-upstream')
loc = r'C:\Users\geekom\Desktop\архив\Pawl'

def norm(s):
    return (s.replace('github.com/alca-lang/alca', 'github.com/pawl-lang/pawl')
             .replace('Alca', 'Pawl').replace('alca', 'pawl'))

for lf in glob.glob(os.path.join(loc, '**', '*.go'), recursive=True):
    rel = os.path.relpath(lf, loc)
    uf = os.path.join(up, rel)
    if not os.path.exists(uf):
        continue
    u = [norm(x) for x in open(uf, encoding='utf-8', errors='replace').read().splitlines()]
    l = open(lf, encoding='utf-8', errors='replace').read().splitlines()
    sm = difflib.SequenceMatcher(None, u, l)
    out = []
    for tag, i1, i2, j1, j2 in sm.get_opcodes():
        if tag == 'replace' and (i2 - i1) == (j2 - j1):
            for k in range(i2 - i1):
                a, b = u[i1+k], l[j1+k]
                if ';' in a and b == a.replace(';', ''):
                    out.append((j1+k+1, a.strip(), b.strip()))
    if out:
        print('==', rel)
        for lineno, a, b in out:
            print(f'  {lineno}: {a!r} -> {b!r}')
