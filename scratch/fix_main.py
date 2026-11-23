p = r'C:\Users\geekom\Desktop\архив\Pawl\cmd\pawl\main.go'
s = open(p, encoding='utf-8', newline='').read()
reps = [
 ('if wd, err {= os.Getwd();\n\terr == nil { }', 'if wd, err := os.Getwd(); err == nil {'),
 ('if out, ok {= session.command(trimmed);\n\t\tok { }', 'if out, ok := session.command(trimmed); ok {'),
 ('for _, arg {= range args { }', 'for _, arg := range args {'),
 ('if r {= recover();\n\t\tr != nil { }', 'if r := recover(); r != nil {'),
 ('if info, statErr {= os.Stat(filename);\n\tstatErr == nil { }', 'if info, statErr := os.Stat(filename); statErr == nil {'),
 ('if err {= os.WriteFile(filename, []byte(formatted), mode);\n\terr != nil { }', 'if err := os.WriteFile(filename, []byte(formatted), mode); err != nil {'),
 ('case "-w", "--write" {', 'case "-w", "--write":'),
 ('case "--migrate-ranges", "--migrate" {', 'case "--migrate-ranges", "--migrate":'),
 ('\t\tdefault {\n\t\t\tfiles = append(files, arg)\n\t\t}', '\t\tdefault:\n\t\t\tfiles = append(files, arg)'),
]
for a, b in reps:
    s = s.replace(a, b)
s = s.replace('{=', ':=')
open(p, 'w', encoding='utf-8', newline='').write(s)
