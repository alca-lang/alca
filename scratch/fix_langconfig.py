import json
p = r'C:\Users\geekom\Desktop\архив\Pawl\editors\vscode\language-configuration.json'
d = json.load(open(p, encoding='utf-8'))
kw = r'(?:func|fn|struct|class|enum|interface|if|elif|else|while|for|foreach|loop|match|case|test|ba|threads|tx|watch|mock|try|catch|finally)'
prefix = r'^\s*(?:@[A-Za-z_][A-Za-z0-9_]*(?:\([^)]*\))?\s*)?(?:(?:pub|public|private|const|async)\s+)*'
d['indentationRules']['increaseIndentPattern'] = prefix + kw + r'\b.*\{\s*(?://.*)?$'
d['indentationRules']['decreaseIndentPattern'] = r'^\s*\}\s*$'
for rule in d.get('onEnterRules', []):
    if 'beforeText' in rule and 'func|fn|struct' in rule['beforeText']:
        rule['beforeText'] = prefix + kw + r'\b.*\{\s*(?://.*)?$'
with open(p, 'w', encoding='utf-8', newline='') as f:
    json.dump(d, f, indent=2, ensure_ascii=False)
    f.write('\n')
