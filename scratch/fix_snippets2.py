import json
p = r'C:\Users\geekom\Desktop\архив\Pawl\editors\vscode\snippets\snippets.json'
data = json.load(open(p, encoding='utf-8'))
for entry in data.values():
    body = entry.get('body', [])
    if not isinstance(body, list):
        continue
    net = sum(line.count('{') - line.count('}') for line in body)
    if net > 0:
        body.extend(['}' for _ in range(net)])
with open(p, 'w', encoding='utf-8', newline='') as f:
    json.dump(data, f, indent=2, ensure_ascii=False)
    f.write('\n')
