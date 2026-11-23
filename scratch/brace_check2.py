import sys
p = sys.argv[1]
a, b = int(sys.argv[2]), int(sys.argv[3])
lines = open(p, encoding='utf-8', newline='').read().splitlines()
depth = 0
for i in range(a-1, b):
    ln = lines[i]
    # crude: remove rune literals and quoted strings
    import re
    ln2 = re.sub(r"'(\\.|[^'])'", "''", ln)
    ln2 = re.sub(r'"(\\.|[^"])*"', '""', ln2)
    d = ln2.count('{') - ln2.count('}')
    depth += d
    print(i+1, depth, ln)
