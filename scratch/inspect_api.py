import json, re

d = json.load(open('scratch/api.json', encoding='utf-8'))
kw = set('and or not if else elif while for in return break continue fn func pub struct class enum interface use as import from to try catch throw error null true false async await match case default self new delete type const let var static private public'.split())
names = []
for m, v in d['methods'].items():
    req = [f for f in v.get('fields', []) if f.get('required')]
    names.append((m, [f['name'] for f in req]))
for m, req in names:
    bad = [r for r in req if r in kw]
    if bad:
        print('KEYWORD', m, bad)
print('methods', len(names))
print('max req', max(len(r) for _, r in names))

def snake(s):
    return re.sub(r'(?<!^)(?=[A-Z])', '_', s).lower()

seen = {}
for m, _ in names:
    s = snake(m)
    if s in seen:
        print('DUP', s, seen[s], m)
    seen[s] = m
for m, r in names:
    print(f'{m}: {len(r)} {r}')
