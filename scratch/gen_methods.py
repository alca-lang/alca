import json
import re


def snake(name):
    s = re.sub(r'(?<!^)(?=[A-Z])', '_', name).lower()
    return s


d = json.load(open('scratch/api.json', encoding='utf-8'))
methods = d['methods']
skip = {'getUpdates', 'getMe', 'sendMessage', 'editMessageText', 'getFile'}

out = []
out.append('// ----------------------------------------------------------------------------')
out.append('// Сгенерированные обёртки всех методов Bot API 10.3')
out.append('//')
out.append('// Формат: telebot.<snake_case>(bot, <обязательные параметры...>, opts?)')
out.append('// Необязательные параметры передаются в opts (map[string, json]).')
out.append('// Локальные файлы в полях InputFile загружаются автоматически.')
out.append('// ----------------------------------------------------------------------------')
out.append('')

count = 0
for name, m in methods.items():
    if name in skip:
        continue
    fields = m.get('fields', [])
    req = [f for f in fields if f.get('required')]
    fn = snake(name)
    doc = ' '.join(m.get('description', []))
    doc = doc.replace('\n', ' ').replace('\\', '/')
    doc = re.sub(r'\s+', ' ', doc).strip()
    if len(doc) > 150:
        doc = doc[:147].rstrip() + '...'
    out.append(f'// {name}: {doc}')
    params = ['bot: Bot'] + [f'{f["name"]}: json' for f in req] + ['opts: map[string, json] = {}']
    out.append(f'pub fn {fn}(' + ', '.join(params) + ') -> json:')
    if req:
        entries = ', '.join(f'"{f["name"]}": {f["name"]}' for f in req)
        out.append(f'    p: map[string, json] = {{{entries}}};')
    else:
        out.append('    p: map[string, json] = {};')
    out.append('    merge_into(p, opts);')
    out.append(f'    return call(bot, "{name}", p);')
    out.append('')
    count += 1

text = '\n'.join(out)
path = 'libs/telebot/telebot.al'
src = open(path, encoding='utf-8').read()
marker = '// === GENERATED METHODS ==='
if marker not in src:
    raise SystemExit('marker not found')
src = src.replace(marker, text)
open(path, 'w', encoding='utf-8', newline='\n').write(src)
print(f'generated {count} wrappers')
