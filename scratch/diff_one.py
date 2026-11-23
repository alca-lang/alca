import os, difflib, sys
root = r'C:\Users\geekom\Desktop\архив\Pawl'
up = os.path.join(os.environ['TEMP'], 'alca-upstream')
rel = sys.argv[1]
def read(p): return open(p, encoding='utf-8-sig', errors='replace').read()
a = read(os.path.join(up, rel)).replace('Alca','Pawl').replace('alca','pawl').replace('ALCA','PAWL').replace('an Pawl','a Pawl').replace('An Pawl','A Pawl')
b = read(os.path.join(root, rel))
d = list(difflib.unified_diff(a.splitlines(), b.splitlines(), fromfile='upstream-renamed', tofile='local', n=1))
print('\n'.join(d[:80]))
