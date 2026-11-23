#!/usr/bin/env python3
"""Migrate Pawl sources from `;` + `:`-blocks to newline-terminated `{}` blocks.

String literals, comments and non-block colons (type annotations, map keys,
URLs, ...) are left untouched.  Indentation is preserved; a `}` line is
inserted where the old INDENT/DEDENT block used to end.
"""
import argparse
import os
import sys

BLOCK_KEYWORDS = {
    "func", "fn", "shfunc", "void", "if", "elif", "else", "for", "foreach",
    "while", "loop", "match", "case", "switch", "default", "struct", "class",
    "enum", "interface", "test", "ba", "threads", "tx", "watch", "mock",
    "try", "catch", "finally", "do",
}
PREFIX_KEYWORDS = {
    "pub", "public", "private", "protected", "const", "static", "async",
    "mut", "await", "override", "abstract",
}


def mask_source(src: str) -> str:
    """Replace every char inside strings/comments with a space, keep newlines."""
    out = list(src)
    n = len(src)
    i = 0

    def blank(a, b):
        for k in range(a, b):
            if out[k] != "\n":
                out[k] = " "

    while i < n:
        c = src[i]
        # line comment
        if c == "/" and i + 1 < n and src[i + 1] == "/":
            j = i + 2
            while j < n and src[j] not in "\r\n":
                j += 1
            blank(i, j)
            i = j
            continue
        # block comment
        if c == "/" and i + 1 < n and src[i + 1] == "*":
            j = i + 2
            while j + 1 < n and not (src[j] == "*" and src[j + 1] == "/"):
                j += 1
            j = min(n, j + 2)
            blank(i, j)
            i = j
            continue
        # triple quoted strings
        if c in "\"'" and src[i:i + 3] == c * 3:
            j = i + 3
            while j + 2 < n and src[j:j + 3] != c * 3:
                j += 1
            j = min(n, j + 3)
            blank(i, j)
            i = j
            continue
        # prefixed strings f" r" fr" etc.
        pref = 0
        if c in "fFrRbB" and i + 1 < n and src[i + 1] in "\"'`":
            pref = 1
        if c in "fFrR" and i + 2 < n and src[i + 1] in "rRbBfF" and src[i + 2] in "\"'":
            pref = 2
        qpos = i + pref
        if qpos < n and src[qpos] in "\"'`":
            q = src[qpos]
            j = qpos + 1
            if q == "`":
                while j < n and src[j] != "`":
                    j += 1
                j = min(n, j + 1)
            else:
                while j < n:
                    if src[j] == "\\":
                        j += 2
                        continue
                    if src[j] == q:
                        j += 1
                        break
                    j += 1
            blank(i, j)
            i = j
            continue
        i += 1
    return "".join(out)


def leading_ws(line: str) -> str:
    out = []
    for ch in line:
        if ch in " \t":
            out.append(ch)
        else:
            break
    return "".join(out)


def indent_width(ws: str) -> int:
    return ws.count("\t") + ws.count(" ") // 4


def first_tokens(masked_line: str):
    """Return list of (word, start_index) for identifier-ish tokens on a line."""
    import re
    return [(m.group(0), m.start()) for m in re.finditer(r"[A-Za-z_][A-Za-z_0-9]*", masked_line)]


def is_block_header(masked_line: str):
    """Return (kind, colon_index or None) where kind is 'colon' or 'brace'."""
    toks = first_tokens(masked_line)
    if not toks:
        return None, None
    idx = 0
    while idx < len(toks) and toks[idx][0] in PREFIX_KEYWORDS:
        idx += 1
    if idx >= len(toks):
        return None, None
    word = toks[idx][0]
    if word.startswith("@") or word not in BLOCK_KEYWORDS:
        return None, None

    code = masked_line.rstrip()
    # strip trailing inline comment already masked -> only whitespace left
    last = code[-1] if code else ""
    # find depth-0 colons
    depth = 0
    colons = []
    for pos, ch in enumerate(masked_line):
        if ch in "([{":
            depth += 1
        elif ch in ")]}":
            depth = max(0, depth - 1)
        elif ch == ":" and depth == 0:
            colons.append(pos)
    if colons:
        return "colon", colons[-1]
    if last == "{":
        return "brace", None
    return None, None


def transform(text: str) -> str:
    masked = mask_source(text)
    src_lines = text.splitlines(keepends=True)
    mask_lines = masked.splitlines(keepends=True)
    # If the file ends with a newline, splitlines(keepends=True) has no extra
    # empty element; otherwise align manually.
    out_lines = []
    stack = []  # list of (indent_width, indent_text, kind)

    def close_blocks(width):
        while stack and width <= stack[-1][0]:
            _, iw_text, kind = stack.pop()
            if kind == "colon":
                out_lines.append(iw_text + "}\n")

    i = 0
    while i < len(src_lines):
        orig = src_lines[i]
        mask = mask_lines[i] if i < len(mask_lines) else "\n"
        stripped = mask.strip()
        ws = leading_ws(orig)
        width = indent_width(ws)

        if stripped:
            close_blocks(width)

        kind, colon_idx = is_block_header(mask)
        line = orig
        line_mask = mask

        # --- convert block colon -> '{' ---
        if kind == "colon" and colon_idx is not None:
            # single-line body: text after colon (ignoring comment) is code
            tail = line_mask[colon_idx + 1:].rstrip("\r\n")
            head = line[:colon_idx].rstrip()
            line = head + " {" + line[colon_idx + 1:]
            line_mask = line_mask[:colon_idx] + " {" + line_mask[colon_idx + 1:]
            if tail.strip():
                # append closing brace just before the inline comment / EOL
                insert_at = len(line.rstrip("\r\n"))
                line = line[:insert_at] + " }" + line[insert_at:]
            else:
                stack.append((width, ws, "colon"))
        elif kind == "brace":
            stack.append((width, ws, "brace"))

        # --- drop semicolons (split statements when needed) ---
        pieces = []
        cur_start = 0
        m_line = line_mask
        pos = 0
        while pos < len(m_line):
            if m_line[pos] == ";":
                before = line[cur_start:pos]
                after_mask = m_line[pos + 1:].rstrip("\r\n")
                if after_mask.strip():
                    pieces.append(before.rstrip() + "\n")
                    cur_start = pos + 1
                    # re-indent the next statement
                    pieces.append(ws)
                else:
                    pieces.append(before)
                    cur_start = pos + 1
            pos += 1
        if cur_start <= len(line):
            pieces.append(line[cur_start:])
        out_lines.append("".join(pieces))
        i += 1

    # close blocks left open at EOF
    while stack:
        _, ws, kind = stack.pop()
        if kind == "colon":
            out_lines.append(ws + "}\n")

    # Drop blank lines that ended up just before a generated closing brace.
    cleaned = []
    for line in out_lines:
        if line.strip() == "}":
            while cleaned and cleaned[-1].strip() == "":
                cleaned.pop()
        cleaned.append(line)

    result = "".join(cleaned)
    return result


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("paths", nargs="+")
    ap.add_argument("--apply", action="store_true", help="write files (default: dry run)")
    ap.add_argument("--ext", default=".al")
    ap.add_argument("--check", action="store_true", help="print files that would change")
    args = ap.parse_args()

    files = []
    for p in args.paths:
        if os.path.isdir(p):
            for root, _, names in os.walk(p):
                for name in names:
                    if name.endswith(args.ext):
                        files.append(os.path.join(root, name))
        else:
            files.append(p)

    changed = 0
    for path in files:
        with open(path, "r", encoding="utf-8-sig", newline="") as f:
            src = f.read()
        dst = transform(src)
        if src != dst:
            changed += 1
            if args.check:
                print(path)
            if args.apply:
                bom = open(path, "rb").read(3) == b"\xef\xbb\xbf"
                with open(path, "w", encoding="utf-8-sig" if bom else "utf-8", newline="") as f:
                    f.write(dst)
    print(f"{len(files)} files scanned, {changed} would change")


if __name__ == "__main__":
    main()
