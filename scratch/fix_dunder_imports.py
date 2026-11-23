import os
root = r'C:\Users\geekom\Desktop\архив\Pawl'
for f in ('pkg/vm/compiler.go', 'pkg/vm/compiler_ext.go', 'pkg/vm/vm.go'):
    p = os.path.join(root, f)
    s = open(p, encoding='utf-8', newline='').read()
    lines = s.splitlines(keepends=True)
    # dedupe dunder import
    out, seen = [], False
    for ln in lines:
        if 'pkg/dunder' in ln:
            if seen: continue
            seen = True
        out.append(ln)
    s = ''.join(out)
    if f.endswith('vm.go') and '"os"' not in s:
        s = s.replace('import (\n', 'import (\n\t"os"\n', 1)
    open(p, 'w', encoding='utf-8', newline='').write(s)

p = os.path.join(root, 'pkg/vm/compiler.go')
s = open(p, encoding='utf-8', newline='').read()
s = s.replace("c.err = fmt.Errorf(\"cannot assign to read-only built-in '%s'\", ident.Value)\n\t\t\t\treturn", "c.err = fmt.Errorf(\"cannot assign to read-only built-in '%s'\", ident.Value)\n\t\t\t\treturn c.err")
s = s.replace("c.err = fmt.Errorf(\"cannot declare read-only built-in '%s'\", n.Name.Value)\n\t\t\treturn", "c.err = fmt.Errorf(\"cannot declare read-only built-in '%s'\", n.Name.Value)\n\t\t\treturn c.err")
open(p, 'w', encoding='utf-8', newline='').write(s)
