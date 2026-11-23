import sys
p = sys.argv[1]
s = open(p, encoding='utf-8-sig').read()
print('len', len(s), 'lines', s.count('\n'), 'literal-backslash-n', s.count('\\n'))
i = s.find("case '\"'")
print('index', i)
print(repr(s[i:i+400]))
