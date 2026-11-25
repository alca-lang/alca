# Copyright 2026 Pawl Programming Language
# SPDX-License-Identifier: Apache-2.0

with open("pkg/typecheck/builtins_table.go", "r", encoding="utf-8") as f:
    text = f.read()

# Let's extract moduleMembers:
# It's: var moduleMembers = map[string]map[string]string{ ... }
# var moduleAliases = map[string]string{ ... }
# var globalSigs = map[string]string{ ... }

import re

# Parser for Go map literals
def parse_string_map(src):
    res = {}
    for line in src.splitlines():
        line = line.strip()
        if not line or line.startswith("//"):
            continue
        m = re.match(r'"([^"]+)":\s*"([^"]*)",?', line)
        if m:
            res[m.group(1)] = m.group(2)
    return res

def parse_nested_map(src):
    res = {}
    cur_mod = None
    cur_dict = {}
    for line in src.splitlines():
        trimmed = line.strip()
        if not trimmed or trimmed.startswith("//"):
            continue
        m_head = re.match(r'"([^"]+)":\s*\{', trimmed)
        if m_head:
            cur_mod = m_head.group(1)
            cur_dict = {}
            continue
        if trimmed.startswith("},") or trimmed == "}":
            if cur_mod:
                res[cur_mod] = cur_dict
                cur_mod = None
            continue
        if cur_mod:
            m_item = re.match(r'"([^"]+)":\s*"([^"]*)",?', trimmed)
            if m_item:
                cur_dict[m_item.group(1)] = m_item.group(2)
    return res

start_members = text.find("var moduleMembers = map[string]map[string]string{")
end_members = text.find("var moduleAliases = map[string]string{")
members_src = text[start_members:end_members]
module_members = parse_nested_map(members_src)

start_aliases = end_members
end_aliases = text.find("var globalSigs = map[string]string{")
aliases_src = text[start_aliases:end_aliases]
module_aliases = parse_string_map(aliases_src)

start_globals = end_aliases
globals_src = text[start_globals:]
global_sigs = parse_string_map(globals_src)

print(f"Extracted {len(module_members)} modules, {len(module_aliases)} aliases, {len(global_sigs)} global sigs")

# Now generate builtins_table.cpp
cpp_lines = [
    '// Copyright 2026 Pawl Programming Language',
    '// SPDX-License-Identifier: Apache-2.0',
    '',
    '#include "typecheck/builtins_table.h"',
    '',
    'namespace pawl::typecheck {',
    '',
    'const std::unordered_map<std::string, std::unordered_map<std::string, std::string>>& getModuleMembers() {',
    '    static const std::unordered_map<std::string, std::unordered_map<std::string, std::string>> kMembers = {'
]

for mod, members in sorted(module_members.items()):
    cpp_lines.append(f'        {{"{mod}", {{')
    for name, sig in sorted(members.items()):
        sig_escaped = sig.replace('\\', '\\\\').replace('"', '\\"')
        cpp_lines.append(f'            {{"{name}", "{sig_escaped}"}},')
    cpp_lines.append('        }},')

cpp_lines.extend([
    '    };',
    '    return kMembers;',
    '}',
    '',
    'const std::unordered_map<std::string, std::string>& getModuleAliases() {',
    '    static const std::unordered_map<std::string, std::string> kAliases = {'
])

for alias, canon in sorted(module_aliases.items()):
    cpp_lines.append(f'        {{"{alias}", "{canon}"}},')

cpp_lines.extend([
    '    };',
    '    return kAliases;',
    '}',
    '',
    'const std::unordered_map<std::string, std::string>& getGlobalSigs() {',
    '    static const std::unordered_map<std::string, std::string> kGlobalSigs = {'
])

for name, sig in sorted(global_sigs.items()):
    sig_escaped = sig.replace('\\', '\\\\').replace('"', '\\"')
    cpp_lines.append(f'        {{"{name}", "{sig_escaped}"}},')

cpp_lines.extend([
    '    };',
    '    return kGlobalSigs;',
    '}',
    '',
    '} // namespace pawl::typecheck'
])

with open("compiler/src/typecheck/builtins_table.cpp", "w", encoding="utf-8") as f:
    f.write("\n".join(cpp_lines) + "\n")

print("Generated builtins_table.cpp successfully!")
