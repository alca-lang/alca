import sys, os
sys.path.insert(0, os.path.dirname(__file__))
from migrate_embedded import decode_go

for p in sys.argv[1:]:
    s = open(p, encoding='utf-8-sig').read()
    lines = s.splitlines(keepends=True)
    out = []
    n = 0
    for ln in lines:
        if ln.count('\\n') >= 3 and '\\t' in ln:
            # This is a block of Go code that was accidentally encoded as a
            # Go string literal. Decode it back to real source lines.
            end = '\n' if ln.endswith('\n') else ''
            body = ln[:-1] if end else ln
            out.append(decode_go(body) + end)
            n += 1
        else:
            out.append(ln)
    open(p, 'w', encoding='utf-8', newline='').write(''.join(out))
    print(p, 'repaired lines:', n)
