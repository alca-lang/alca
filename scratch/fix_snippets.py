p = r'C:\Users\geekom\Desktop\архив\Pawl\editors\vscode\snippets\snippets.json'
s = open(p, encoding='utf-8', newline='').read()
s = s.replace(' {}\\n', ' {\\n').replace('{    }\\n', '{\\n')
open(p, 'w', encoding='utf-8', newline='').write(s)
