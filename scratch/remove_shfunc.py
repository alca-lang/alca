import os, re
root = r'C:\Users\geekom\Desktop\архив\Pawl'

def edit(path, pairs):
    p = os.path.join(root, path)
    s = open(p, encoding='utf-8-sig', newline='').read()
    for a, b in pairs:
        if a not in s:
            print('MISS', path, repr(a[:70]))
        s = s.replace(a, b)
    open(p, 'w', encoding='utf-8', newline='').write(s)

def drop_func(src, name):
    """Remove a top-level Go func (including preceding comment block)."""
    pat = re.compile(r'(?m)^(?:(?://[^\n]*\n)|(?:\n))*func (?:\([^)]*\) )?' + re.escape(name) + r'\([^\n]*\{\n(?:\n|.*\n)*?^\}\n\n?', re.M)
    new, n = pat.subn('', src, count=1)
    if n == 0:
        print('MISS func', name)
    return new

# token
edit('pkg/token/token.go', [
    ("\t// SHFUNC represents the 'shfunc' keyword for shell-integrated functions.\n\tSHFUNC TokenType = \"shfunc\"\n", ""),
    ('\t"shfunc":    SHFUNC,\n', ''),
    ('case FUNC, SHFUNC, RETURN, IF, ELSE, ELIF, WHILE, FOR, LOOP, FOREACH,', 'case FUNC, RETURN, IF, ELSE, ELIF, WHILE, FOR, LOOP, FOREACH,'),
])
edit('pkg/token/token_test.go', [('\t\t{"shfunc", SHFUNC},\n', '')])
edit('pkg/lexer/lexer_test.go', [
    (' func shfunc return if else', ' func return if else'),
    ('\t\t{token.SHFUNC, "shfunc"},\n', ''),
])

# ast
edit('pkg/ast/ast.go', [
    ('// FuncDeclaration represents a function or shell-integrated function declaration.', '// FuncDeclaration represents a function declaration.'),
    ('\tToken      token.Token     // token.FUNC or token.SHFUNC', '\tToken      token.Token     // token.FUNC'),
    ('\tIsShell    bool            // True if declared as \'shfunc\'\n', ''),
    ('\t} else if fd.IsShell {\n\t\tout.WriteString("shfunc ")\n', ''),
])
edit('pkg/ast/ast_test.go', [('\t\tIsShell: false,\n', '')])
# remove the dedicated shell function test (last test in file)
p = os.path.join(root, 'pkg/ast/ast_test.go')
s = open(p, encoding='utf-8', newline='').read()
s = re.sub(r'(?ms)\nfunc TestShellFunction.*\Z', '\n', s)
open(p, 'w', encoding='utf-8', newline='').write(s)

# parser
edit('pkg/parser/parser.go', [
    ('\tcase token.FUNC, token.SHFUNC, token.TYPE_VOID:', '\tcase token.FUNC, token.TYPE_VOID:'),
    ("\ttok := p.curToken // 'func', 'shfunc', or 'void'\n\tisShell := (tok.Type == token.SHFUNC)\n", "\ttok := p.curToken // 'func' or 'void'\n"),
    ("\tp.nextToken() // past func/shfunc/void\n", "\tp.nextToken() // past func/void\n"),
    ('\t\tIsShell:    isShell,\n', ''),
    ('\t\tcase token.FUNC, token.SHFUNC:', '\t\tcase token.FUNC:'),
    ('\t\t\tif d.IsShell {\n\t\t\t\tp.reportAt(d.Token, "shell functions cannot be class methods", "a method cannot be a shell function")\n\t\t\t\treturn nil\n\t\t\t}\n', ''),
])

# interpreter types
edit('pkg/interpreter/types.go', [
    ('\tSHFUNC_VAL   ValueType = "SHFUNCTION"\n', ''),
    ('\tIsShell    bool\n', ''),
    ('// Type returns SHFUNC_VAL if declared as a shell function, or FUNC_VAL otherwise.\nfunc (v *FuncValue) Type() ValueType {\n\tif v.IsShell {\n\t\treturn SHFUNC_VAL\n\t}\n\treturn FUNC_VAL\n}',
     '// Type returns FUNC_VAL.\nfunc (v *FuncValue) Type() ValueType {\n\treturn FUNC_VAL\n}'),
    ('\tprefix := "func"\n\tif v.IsShell {\n\t\tprefix = "shfunc"\n\t}\n', '\tprefix := "func"\n'),
    ('\t\tIsShell:    v.IsShell,\n', ''),
    ('\tcase FUNC_VAL, SHFUNC_VAL:', '\tcase FUNC_VAL:'),
])

# interpreter main
p = os.path.join(root, 'pkg/interpreter/interpreter.go')
s = open(p, encoding='utf-8', newline='').read()
s = s.replace("\t\t\t\tif field == \"stream\" {\n\t\t\t\t\treturn i.streamFunction(fnVal, args)\n\t\t\t\t}\n", '')
s = s.replace("\t\tif fn.IsShell {\n\t\t\treturn i.executeShellFunc(fn, args), nil\n\t\t}\n\n", '')
s = s.replace('\t\tIsShell: false,\n', '')
s = s.replace('\t\t\tIsShell: node.IsShell,\n', '')
s = s.replace('\tcase "func":\n\t\treturn t == FUNC_VAL || t == SHFUNC_VAL', '\tcase "func":\n\t\treturn t == FUNC_VAL')
for fn in ('executeShellFunc', 'streamFunction', 'executeStreamingShellFunc'):
    s = drop_func(s, fn)
open(p, 'w', encoding='utf-8', newline='').write(s)

# builtins: stream method + streaming runner
p = os.path.join(root, 'pkg/interpreter/builtins.go')
s = open(p, encoding='utf-8', newline='').read()
s = drop_func(s, 'RunShellCommandStreaming')
s = s.replace('\tcase *FuncValue:\n\t\tif method == "stream" {\n\t\t\treturn nil, fmt.Errorf("fn.stream() is not supported by the VM backend; run the program with the interpreter")\n\t\t}\n\t\treturn nil, fmt.Errorf("value of type %s has no methods", obj.Type())',
              '\tcase *FuncValue:\n\t\treturn nil, fmt.Errorf("value of type %s has no methods", obj.Type())')
s = s.replace('\t"bytes"\n', '')
s = s.replace('\t"io"\n', '')
open(p, 'w', encoding='utf-8', newline='').write(s)

# typecheck
edit('pkg/typecheck/check.go', [
    ('if ft.Ret != Void && !blockTerminates(f.Body) && !f.IsShell {', 'if ft.Ret != Void && !blockTerminates(f.Body) {'),
])
p = os.path.join(root, 'pkg/typecheck/expr.go')
s = open(p, encoding='utf-8', newline='').read()
s = re.sub(r'(?m)\tcase \*FuncType:\n\t\tif n.Field == "stream" \{\n(?:\t\t.*\n)*?\t\t\}\n', '', s, count=1)
open(p, 'w', encoding='utf-8', newline='').write(s)

# vm
edit('pkg/vm/compiler_ext.go', [
    ('\t\tif d.IsShell {\n\t\t\treturn unsupported("shell functions (shfunc)", d.Token)\n\t\t}\n', ''),
    ('\tif n.IsShell {\n\t\treturn true, unsupported("shell functions (shfunc)", n.Token)\n\t}\n', ''),
])
edit('pkg/vm/vm.go', [
    ('\t\t\tif field == "stream" {\n\t\t\t\treturn interpreter.NewError(500, "fn.stream() is not supported by the VM backend; run the program with the interpreter")\n\t\t\t}\n', ''),
])

# interpreter tests: drop shell-func tests
p = os.path.join(root, 'pkg/interpreter/interpreter_test.go')
s = open(p, encoding='utf-8', newline='').read()
for fn in ('TestInterpreterShellFunc', 'TestInterpreterStreamingShellFunc', 'TestRunShellCommandStreaming'):
    s = re.sub(r'(?ms)\nfunc ' + fn + r'\(t \*testing\.T\) \{.*?\n\}\n', '\n', s, count=1)
s = s.replace('\t"bytes"\n', '')
open(p, 'w', encoding='utf-8', newline='').write(s)

# example
edit('examples/gui/snake.al', [
    ('shfunc main(cmd: string) -> string | int:', 'func main(cmd: string) -> string | int {'),
])
