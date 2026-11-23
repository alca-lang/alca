#!/usr/bin/env python3
"""Migrate Pawl source embedded in Go string literals, Markdown fences and JSON snippets."""
import argparse
import os
import re
from migrate_syntax import transform as transform_pawl

GO_ESCAPES = {'n': '\n', 'r': '\r', 't': '\t', '"': '"', '\\': '\\', 'a': '\a',
              'b': '\b', 'f': '\f', 'v': '\v', '0': '\0'}


def decode_go(s: str) -> str:
    out = []
    i = 0
    while i < len(s):
        if s[i] != '\\':
            out.append(s[i]); i += 1; continue
        i += 1
        if i >= len(s):
            out.append('\\'); break
        c = s[i]
        if c in GO_ESCAPES:
            out.append(GO_ESCAPES[c]); i += 1
        elif c == 'x' and i + 2 < len(s):
            try:
                out.append(chr(int(s[i + 1:i + 3], 16))); i += 3
            except ValueError:
                out.append('\\' + c); i += 1
        elif c in 'uU' and i + (4 if c == 'u' else 8) < len(s):
            n = 4 if c == 'u' else 8
            try:
                out.append(chr(int(s[i + 1:i + 1 + n], 16))); i += 1 + n
            except ValueError:
                out.append('\\' + c); i += 1
        elif c.isdigit() and i + 2 < len(s):
            out.append(chr(int(s[i:i + 3], 8))); i += 3
        else:
            out.append('\\' + c); i += 1
    return ''.join(out)


def encode_go(s: str) -> str:
    out = []
    for ch in s:
        if ch == '\\': out.append('\\\\')
        elif ch == '"': out.append('\\"')
        elif ch == '\n': out.append('\\n')
        elif ch == '\r': out.append('\\r')
        elif ch == '\t': out.append('\\t')
        else: out.append(ch)
    return ''.join(out)


def looks_like_pawl(s: str) -> bool:
    if not s or s.strip() in (';', ':', '{', '}'):
        return False
    markers = ('func ', 'fn ', 'if ', 'while ', 'for ', 'foreach ', 'println(',
               'use ', 'assert ', 'return ', 'struct ', 'class ', 'match ',
               'case ', 'else', 'enum ', 'test ', ':=', ': int', ': string',
               ': bool', ': float', ': map', ': list', '->', 'const ', 'pub ')
    if any(m in s for m in markers):
        return True
    # simple assignment/declaration statements such as "x = 5;"
    return bool(re.search(r'^\s*[A-Za-z_]\w*\s*(=|:=|:\s*\w)', s))


def migrate_go(src: str):
    """Find Go string literals and migrate Pawl content in place."""
    out = []
    i = 0
    n = len(src)
    while i < n:
        c = src[i]
        if c == '/' and i + 1 < n and src[i + 1] == '/':
            j = src.find('\n', i)
            j = n if j < 0 else j
            out.append(src[i:j]); i = j; continue
        if c == '/' and i + 1 < n and src[i + 1] == '*':
            j = src.find('*/', i + 2)
            j = n if j < 0 else j + 2
            out.append(src[i:j]); i = j; continue
        # Go rune literal: skip it so a quote inside it is not mistaken for a string start
        if c == "'":
            j = i + 1
            while j < n:
                if src[j] == '\\':
                    j += 2; continue
                if src[j] == "'":
                    j += 1; break
                if src[j] == '\n':
                    break
                j += 1
            out.append(src[i:j]); i = j; continue
        if c == '`':
            j = src.find('`', i + 1)
            j = n if j < 0 else j + 1
            raw = src[i + 1:j - 1]
            if looks_like_pawl(raw):
                raw = transform_pawl(raw)
            out.append('`' + raw + '`')
            i = j
            continue
        if c == '"':
            j = i + 1
            while j < n:
                if src[j] == '\\':
                    j += 2; continue
                if src[j] == '"':
                    j += 1; break
                j += 1
            lit = src[i + 1:j - 1]
            dec = decode_go(lit)
            if looks_like_pawl(dec):
                lit = encode_go(transform_pawl(dec))
            out.append('"' + lit + '"')
            i = j
            continue
        out.append(c)
        i += 1
    return ''.join(out)


def migrate_markdown(src: str):
    lines = src.splitlines(keepends=True)
    out = []
    in_fence = False
    fence = ''
    buf = []
    active = False
    for line in lines:
        m = re.match(r'^\s*(`{3,}|~{3,})(.*)', line)
        if m and not in_fence:
            in_fence = True
            fence = m.group(1)
            active = m.group(2).strip().lower() == 'pawl'
            if active:
                buf = []
            out.append(line)
            continue
        if in_fence and re.match(r'^\s*' + re.escape(fence[0]) + '{' + str(len(fence)) + r',}\s*$', line):
            if buf:
                out.append(transform_pawl(''.join(buf)) if active else ''.join(buf))
                buf = []
            in_fence = False
            out.append(line)
            continue
        if in_fence:
            buf.append(line)
        else:
            out.append(line)
    if buf:
        out.append(transform_pawl(''.join(buf)) if active else ''.join(buf))
    return ''.join(out)


def migrate_json_snippets(src: str):
    # snippet bodies are JSON string values containing Pawl code
    return re.sub(r'"([^"\\]*(?:\\.[^"\\]*)*)"',
                  lambda m: '"' + encode_go(transform_pawl(decode_go(m.group(1)))) + '"',
                  src)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('paths', nargs='+')
    ap.add_argument('--apply', action='store_true')
    ap.add_argument('--kind', choices=['go', 'md', 'json'], required=True)
    args = ap.parse_args()
    files = []
    for p in args.paths:
        if os.path.isdir(p):
            for root, _, names in os.walk(p):
                for name in names:
                    if args.kind == 'go' and name.endswith('.go'):
                        files.append(os.path.join(root, name))
                    elif args.kind == 'md' and name.endswith('.md'):
                        files.append(os.path.join(root, name))
                    elif args.kind == 'json' and name.endswith('.json'):
                        files.append(os.path.join(root, name))
        else:
            files.append(p)
    fn = {'go': migrate_go, 'md': migrate_markdown, 'json': migrate_json_snippets}[args.kind]
    changed = 0
    for path in files:
        raw = open(path, 'rb').read()
        bom = raw[:3] == b'\xef\xbb\xbf'
        src = raw.decode('utf-8-sig')
        dst = fn(src)
        if src != dst:
            changed += 1
            if args.apply:
                open(path, 'w', encoding='utf-8-sig' if bom else 'utf-8', newline='').write(dst)
            else:
                print(path)
    print(f'{len(files)} files scanned, {changed} changed')


if __name__ == '__main__':
    main()
