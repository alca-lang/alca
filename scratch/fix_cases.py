import os, sys
root = sys.argv[1]
for dirpath, _, names in os.walk(root):
    if 'scratch' in dirpath:
        continue
    for name in names:
        if not name.endswith('.go'):
            continue
        p = os.path.join(dirpath, name)
        s = open(p, encoding='utf-8-sig', newline='').read()
        lines = s.splitlines(keepends=True)
        out = []
        changed = False
        for i, ln in enumerate(lines):
            st = ln.strip()
            if (st.startswith('case ') or st == 'default') and st.endswith('{'):
                out.append(ln[:-2] + ':\n' if ln.endswith('{\n') else ln[:-1] + ':')
                changed = True
                continue
            if st == '}' and i + 1 < len(lines) and (lines[i+1].strip().startswith('case ') or lines[i+1].strip().startswith('default:')):
                changed = True
                continue
            out.append(ln)
        if changed:
            open(p, 'w', encoding='utf-8', newline='').write(''.join(out))
            print(p)
