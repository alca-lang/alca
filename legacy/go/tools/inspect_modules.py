with open("pkg/typecheck/builtins_table.go") as f:
    text = f.read()

import re
matches = re.findall(r'^\t"([a-zA-Z0-9_]+)":\s*\{', text, re.MULTILINE)
print("Modules in moduleMembers:", len(matches))
print(matches)

# Check moduleAliases
alias_start = text.find("var moduleAliases")
if alias_start != -1:
    print(text[alias_start:alias_start+1000])
