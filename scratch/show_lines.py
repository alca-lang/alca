import sys
p = sys.argv[1]
a, b = int(sys.argv[2]), int(sys.argv[3])
lines = open(p, encoding='utf-8', newline='').read().splitlines()
for i in range(a-1, min(b, len(lines))):
    print(i+1, repr(lines[i]))
