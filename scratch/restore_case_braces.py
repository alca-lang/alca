import os, sys
root = sys.argv[1]
def indent(s):
    return len(s) - len(s.lstrip('\t'))
for dirpath, _, names in os.walk(root):
    if 'scratch' in dirpath: continue
    for name in names:
        if not name.endswith('.go'): continue
        p = os.path.join(dirpath, name)
        lines = open(p, encoding='utf-8-sig', newline='').read().splitlines(keepends=True)
        out = []
        changed = False
        for i, ln in enumerate(lines):
            st = ln.strip()
            if st.startswith('case ') or st.startswith('default:'):
                # find previous non-blank emitted line
                j = len(out) - 1
                while j >= 0 and out[j].strip() == '': j -= 1
                if j >= 0:
                    pi = indent(out[j])
                    ci = indent(ln)
                    if pi > ci and out[j].strip() not in ('{', '}'):
                        out.append('\t' * (ci + 1) + '}\n')
                        changed = True
            out.append(ln)
        if changed:
            print(p)
            open(p, 'w', encoding='utf-8', newline='').write(''.join(out))
