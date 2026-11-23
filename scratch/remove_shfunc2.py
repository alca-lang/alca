import os, re
root = r'C:\Users\geekom\Desktop\архив\Pawl'

def sub(path, pattern, repl='', flags=0):
    p = os.path.join(root, path)
    s = open(p, encoding='utf-8-sig', newline='').read()
    s2, n = re.subn(pattern, repl, s, flags=flags)
    if n == 0: print('MISS', path, pattern[:60])
    open(p, 'w', encoding='utf-8', newline='').write(s2)

sub('pkg/lexer/lexer_test.go', r'func shfunc ', 'func ')
sub('pkg/lexer/lexer_test.go', r'\{token\.SHFUNC, "shfunc"\},\n')
sub('pkg/ast/ast_test.go', r'\nfunc TestShellFunction.*\Z', '\n', re.S)
sub('pkg/ast/ast_test.go', r'\n\t\tIsShell:\s+false,', '')
sub('pkg/parser/parser.go', r'\n\t\t\tif p\.curTokenIs\(token\.SHFUNC\) \{\n\t\t\t\tp\.reportAt\([^\n]*\n\t\t\t\}\n')
sub('pkg/interpreter/interpreter.go', r'\n\t\tIsShell:\s+node\.IsShell,')
sub('pkg/interpreter/interpreter.go', r'\n\t\t// Streaming call: fn\.stream\(\)[^\n]*\n\t\tif fnVal, isFn := target\.\(\*FuncValue\); isFn && dot\.Field == "stream" \{\n\t\t\treturn i\.streamFunction\(fnVal, args\)\n\t\t\}\n')
sub('pkg/interpreter/interpreter.go', r'val\.Type\(\) == FUNC_VAL \|\| val\.Type\(\) == SHFUNC_VAL', 'val.Type() == FUNC_VAL')
sub('pkg/vm/compiler_ext.go', r'\n\t\t\tif d\.IsShell \{\n\t\t\t\treturn unsupported\("shell functions \(shfunc\)", d\.Token\)\n\t\t\t\}\n')
sub('pkg/vm/compiler_ext.go', r'\n\t\tif n\.IsShell \{\n\t\t\treturn true, unsupported\("shell functions \(shfunc\)", n\.Token\)\n\t\t\}\n')
sub('examples/gui/snake.al', r'^shfunc ', 'func ', re.M)
