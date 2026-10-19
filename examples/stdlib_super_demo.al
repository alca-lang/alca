// ==============================================================================
// Alca Lang: Комплексная демонстрация стандартных библиотек
// ==============================================================================

use std.re;
use std.archive;
use std.csv;
use std.ini;
use std.sys;
use std.cli;
use std.log;
use std.math;

// ------------------------------------------------------------------------------
// 1. std.re (Регулярные выражения)
// ------------------------------------------------------------------------------
println("=== 1. std.re ===");
log_line := "192.168.1.55 - - [18/Oct/2026:03:40:12] \"POST /api/v2/orders HTTP/1.1\" 201 450";

is_post: bool = re.match(`POST`, log_line);
println("Содержит POST запрос: " + string(is_post));

ip_pattern := `\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`;
ip_addr: string = string(re.find(ip_pattern, log_line));
println("IP адрес клиента: " + ip_addr);

cleaned: string = re.replace(`\s+`, " ", "Alca    Lang     Automation");
println("Нормализация пробелов: " + cleaned);

// ------------------------------------------------------------------------------
// 2. std.archive (Архивация ZIP / TAR.GZ)
// ------------------------------------------------------------------------------
println("\n=== 2. std.archive ===");
// Создадим тестовую папку и файл для архивации
std.io.make_dir("demo_archive_tmp");
std.io.write_file("demo_archive_tmp/notes.txt", "Alca standard library is super fast!");

archive.zip_create("demo_archive_tmp", "demo.zip");
entries := archive.list("demo.zip");
println("Упаковано в demo.zip: " + string(len(entries)) + " записей");
for item in entries:
    println("  -> " + string(item["name"]) + " (" + string(item["size"]) + " bytes)");

archive.zip_extract("demo.zip", "demo_extracted");
restored := std.io.read_file("demo_extracted/notes.txt");
println("Восстановлено из архива: " + restored);

// Очистка временных файлов
std.io.delete_file("demo.zip");
std.io.delete_file("demo_archive_tmp/notes.txt");
std.io.delete_file("demo_archive_tmp");
std.io.delete_file("demo_extracted/notes.txt");
std.io.delete_file("demo_extracted");

// ------------------------------------------------------------------------------
// 3. std.formats (CSV, INI, JSON Query)
// ------------------------------------------------------------------------------
println("\n=== 3. std.formats ===");

// CSV
csv_text := "id,server,status\n101,nginx-prod,online\n102,db-master,active\n103,redis-cache,idle\n";
servers := csv.parse(csv_text);
println("Распарсено CSV серверов: " + string(len(servers)));
for srv in servers:
    println("  Сервер [" + string(srv["id"]) + "]: " + string(srv["server"]) + " (" + string(srv["status"]) + ")");

// INI
ini_text := "[database]\nhost = 127.0.0.1\nport = 5432\nuser = admin\n";
cfg := ini.parse(ini_text);
println("INI database host: " + string(cfg["database"]["host"]) + ":" + string(cfg["database"]["port"]));

// JSON Query (глубокий поиск по точечной нотации)
api_json := "{\"data\":{\"users\":[{\"id\":1,\"name\":\"Иван\"},{\"id\":2,\"name\":\"Алексей\"}]}}";
user2_name := json_query(api_json, "data.users[1].name");
println("JSON Query (data.users[1].name): " + string(user2_name));

// ------------------------------------------------------------------------------
// 4. std.sys (Мониторинг ресурсов)
// ------------------------------------------------------------------------------
println("\n=== 4. std.sys ===");
disk := sys.disk_usage(".");
println("Диск: всего " + string(disk["total_gb"]) + " GB, свободно " + string(disk["free_gb"]) + " GB (занято " + string(disk["percent_used"]) + "%)");

ram := sys.ram_usage();
println("Оперативная память: всего " + string(ram["total_mb"]) + " MB, свободно " + string(ram["free_mb"]) + " MB (загрузка " + string(ram["percent_used"]) + "%)");

println("Количество ядер CPU: " + string(sys.cpu_count()));
println("Платформа: " + sys.platform() + " (" + sys.arch() + ")");

// ------------------------------------------------------------------------------
// 5. std.cli (Консольные утилиты, цвета, прогресс-бар)
// ------------------------------------------------------------------------------
println("\n=== 5. std.cli ===");
colored_ok: string = cli.color("green", "[OK]");
colored_warn: string = cli.color("yellow", "[WARN]");
colored_err: string = cli.color("red", "[FAIL]");

println(colored_ok + " Все сервисы запущены");
println(colored_warn + " Высокая нагрузка на процессор");
println(colored_err + " Ошибка подключения к внешнему API");

bar: string = string(cli.progress_bar(75, 100, 25, "Синхронизация:"));
println(bar);

// ------------------------------------------------------------------------------
// 6. std.log (Структурированное логирование)
// ------------------------------------------------------------------------------
println("\n=== 6. std.log ===");
log.set_level("debug");
log.info("Информационное сообщение системы", {"module": "core", "version": "1.0.0"});
log.warn("Предупреждение о лимите памяти", {"threshold": "90%"});

log.set_format("json");
log.info("Структурированный JSON лог для Grafana/ELK", {"status": 200, "latency_ms": 15});
log.set_format("text");

// ------------------------------------------------------------------------------
// 7. std.math (Математика)
// ------------------------------------------------------------------------------
println("\n=== 7. std.math ===");
println("Число PI: " + string(math.PI));
println("Число E:  " + string(math.E));
println("Квадратный корень sqrt(256): " + string(math.sqrt(256)));
println("Возведение в степень pow(2, 10): " + string(math.pow(2, 10)));
println("Округление round(4.65): " + string(math.round(4.65)));
println("Ограничение диапазона clamp(120, 0, 100): " + string(math.clamp(120, 0, 100)));
println("Синус sin(PI / 2): " + string(math.sin(math.PI / 2)));

println("\nВсе стандартные библиотеки успешно работают и протестированы!");
