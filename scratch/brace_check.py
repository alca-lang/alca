import sys
p = sys.argv[1]
a, b = int(sys.argv[2]), int(sys.argv[3])
lines = open(p, encoding='utf-8', newline='').read().splitlines()
depth = 0
for i in range(a-1, b):
    ln = lines[i]
    # strip strings crudely
    depth += ln.count('{') - ln.count('}')
    print(i+1, depth, ln)
