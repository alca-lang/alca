import os, hashlib
root = r'C:\Users\geekom\Desktop\архив\Pawl'
up = os.path.join(os.environ['TEMP'], 'alca-upstream')

def read(p):
    return open(p, encoding='utf-8-sig', errors='replace').read()

def renamed(s):
    return s.replace('Alca', 'Pawl').replace('alca', 'pawl').replace('ALCA', 'PAWL').replace('an Pawl', 'a Pawl').replace('An Pawl', 'A Pawl')

same, diff = [], []
for dirpath, _, names in os.walk(os.path.join(up, 'pkg')):
    for name in names:
        if not name.endswith('.go'): continue
        p = os.path.join(dirpath, name)
        rel = os.path.relpath(p, up)
        lp = os.path.join(root, rel)
        if not os.path.exists(lp): continue
        a = renamed(read(p))
        b = read(lp)
        (same if a == b else diff).append(rel)
print('identical after rename:', len(same))
print('different:', len(diff))
for f in diff[:40]: print('DIFF', f)
