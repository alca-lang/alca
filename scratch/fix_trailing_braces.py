import os, sys
sys.path.insert(0, r'C:\Users\geekom\Desktop\архив\Pawl\scratch')
import migrate_embedded as me

def fix_content(s):
    out = []
    for ln in s.splitlines(keepends=True):
        body = ln.rstrip('\r\n')
        nl = ln[len(body):]
        if body.endswith('}') and '{' not in body and body.strip() and not body.strip().startswith('}'):
            ws = body[:len(body)-len(body.lstrip())]
            if '\t' in ws:
                close_ws = ws[:-1]
            else:
                close_ws = ws[:-4] if len(ws) >= 4 else ''
            out.append(body[:-1] + nl)
            out.append(close_ws + '}' + nl)
        else:
            out.append(ln)
    return ''.join(out)

def migrate_go_fixed(src):
    # reuse the string-literal scanner from migrate_embedded but apply fix_content
    out = []; i = 0; n = len(src)
    while i < n:
        c = src[i]
        if c == '/' and i+1 < n and src[i+1] == '/':
            j = src.find('\n', i); j = n if j < 0 else j
            out.append(src[i:j]); i = j; continue
        if c == '/' and i+1 < n and src[i+1] == '*':
            j = src.find('*/', i+2); j = n if j < 0 else j+2
            out.append(src[i:j]); i = j; continue
        if c == "'":
            j = i+1
            while j < n and src[j] != "'":
                j += 2 if src[j] == '\\' else 1
            j = min(j+1, n)
            out.append(src[i:j]); i = j; continue
        if c == '`':
            j = src.find('`', i+1); j = n if j < 0 else j+1
            raw = src[i+1:j-1]
            if me.looks_like_pawl(raw): raw = fix_content(raw)
            out.append('`'+raw+'`'); i = j; continue
        if c == '"':
            j = i+1
            while j < n:
                if src[j] == '\\': j += 2; continue
                if src[j] == '"': j += 1; break
                j += 1
            lit = src[i+1:j-1]; dec = me.decode_go(lit)
            if me.looks_like_pawl(dec): lit = me.encode_go(fix_content(dec))
            out.append('"'+lit+'"'); i = j; continue
        out.append(c); i += 1
    return ''.join(out)

root = sys.argv[1]
changed = 0
for dirpath, _, names in os.walk(root):
    if 'scratch' in dirpath: continue
    for name in names:
        if not name.endswith('.go'): continue
        p = os.path.join(dirpath, name)
        s = open(p, encoding='utf-8-sig', newline='').read()
        d = migrate_go_fixed(s)
        if s != d:
            open(p, 'w', encoding='utf-8', newline='').write(d)
            changed += 1
print('fixed', changed, 'files')
