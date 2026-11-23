import os
root = r'C:\Users\geekom\Desktop\архив\Pawl'
up = os.path.join(os.environ['TEMP'], 'alca-upstream')

def rename(s):
    return (s.replace('Alca', 'Pawl').replace('alca', 'pawl').replace('ALCA', 'PAWL')
             .replace('an Pawl', 'a Pawl').replace('An Pawl', 'A Pawl'))

n = 0
for dirpath, _, names in os.walk(up):
    if '.git' in dirpath: continue
    for name in names:
        if not name.endswith('.go'): continue
        p = os.path.join(dirpath, name)
        rel = os.path.relpath(p, up)
        rel = rel.replace('cmd\\alca\\', 'cmd\\pawl\\')
        dst = os.path.join(root, rel)
        s = rename(open(p, encoding='utf-8-sig').read())
        os.makedirs(os.path.dirname(dst), exist_ok=True)
        open(dst, 'w', encoding='utf-8', newline='').write(s)
        n += 1
print('restored', n, 'Go files')
