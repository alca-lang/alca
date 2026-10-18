// ============================================================
// Alca Lang: Демонстрация управления системным и процессным PATH
// ============================================================

use std.env;
use std.path;

println("=== 1. Просмотр текущего process-level PATH ===");
current_path := env.get_path();
println("Всего записей в PATH: " + string(len(current_path)));
println("Первые 3 записи:");
i: int = 0;
for p in current_path:
    if i < 3:
        println("  [" + string(i) + "] " + string(p));
        i = i + 1;

println("\n=== 2. Поиск исполняемых файлов (which) ===");
git_path: string = env.which("git");
if git_path != "":
    println("Найден git: " + git_path);
else:
    println("git не найден в PATH");

go_path: string = path.which("go");
if go_path != "":
    println("Найден go:  " + go_path);

println("\n=== 3. Проверка и динамическое добавление директории в PATH ===");
my_tool_dir := "C:\\MyApp\\bin";
println("Директория в PATH до добавления: " + string(env.has_path(my_tool_dir)));

env.add_path(my_tool_dir);
println("Директория в PATH после add_path: " + string(env.has_path(my_tool_dir)));

println("\n=== 4. Приоритетное добавление (prepend_path) ===");
env.prepend_path(my_tool_dir);
updated := env.get_path();
println("Первый элемент в PATH теперь: " + string(updated[0]));

println("\n=== 5. Удаление директории из PATH ===");
env.remove_path(my_tool_dir);
println("Директория в PATH после remove_path: " + string(env.has_path(my_tool_dir)));

println("\n=== 6. Утилиты std.path для работы со списками путей ===");
sep: string = string(path.list_sep());
println("Системный разделитель путей: " + sep);

sample := "dir1" + sep + "dir2" + sep + "dir3";
parts := path.split_list(sample);
println("split_list: длина " + string(len(parts)));

rejoined: string = string(path.join_list(parts));
println("join_list:  " + rejoined);

println("\n=== 7. Постоянный (Persistent) User PATH ===");
user_paths := env.get_user_path();
println("Постоянных записей в User PATH: " + string(len(user_paths)));
if len(user_paths) > 0:
    println("Первая постоянная запись пользователя: " + string(user_paths[0]));

println("\nГотово! Alca полностью контролирует как локальный, так и постоянный системный PATH.");
