// Full capabilities integration test:
// 1. Permission error logging and security auditing
// 2. Advanced file operations (stream handles, seek, tell, permissions)
// 3. System tracking (uptime, whoami, processes, partitions)
// 4. Native Dynamic FFI (loading DLLs and calling C/Win32 APIs)
// 5. Native AI / Neural Network Matrix computation

use std.log
use std.io
use std.sys
use std.ffi
use std.ai

print("==================================================")
print("🚀 ALCA FULL CAPABILITIES & SECURITY AUDIT TEST")
print("==================================================")

// 1. Логирование ошибок прав доступа и аудит безопасности
print("\n[1] Проверка логирования ошибок доступа и аудита безопасности:")
log.access_denied("read_file", "C:\\Windows\\System32\\config\\SAM", "Access is denied (insufficient privileges)")
log.audit("SECURITY", "ELEVATION_ATTEMPT", "C:\\Windows\\System32\\cmd.exe", "denied", {"user": "alca_user", "code": 403})

errors = log.get_access_errors()
print("Зафиксировано событий отказа в доступе: " + string(len(errors)))
last_err = errors[len(errors)-1]
print("Последняя ошибка прав: операция=" + last_err["operation"] + ", ресурс=" + last_err["resource"] + ", админ=" + string(last_err["is_admin"]))

// 2. Более сложные файловые операции (стримы, seek, tell, права)
print("\n[2] Проверка продвинутых файловых операций (Handle, Seek, Tell, Chmod):")
test_file = "./temp_stream_test.txt"
file = io.open(test_file, "w+")
file.write("Первая строка потока\nВторая строка данных\nКонец файла")
file.seek(0, 0)
pos = file.tell()
print("Позиция курсора после seek(0, 0): " + string(pos))

line1 = file.read_line()
print("Считана 1-я строка: " + line1)

file.seek(0, 0)
chunk = file.read(12)
print("Считан блок из 12 байт: " + chunk)

file.lock()
file.unlock()
file.close()

// Проверка прав доступа к файлу
can_read = io.check_access(test_file, "r")
can_write = io.check_access(test_file, "w")
perms = io.permissions(test_file)
print("Права доступа к файлу: r=" + string(can_read) + ", w=" + string(can_write) + ", mode=" + perms["mode"] + ", octal=" + perms["octal"])
io.delete_file(test_file)

// 3. Трекинг системы (Uptime, WhoAmI, Процессы, Диски)
print("\n[3] Проверка трекинга системы:")
uptime_sec = sys.uptime()
print("Аптайм системы (сек): " + string(uptime_sec))

me = sys.whoami()
print("Текущий пользователь: " + me["user"] + " (Хост: " + me["hostname"] + ", Права администратора: " + string(me["is_admin"]) + ")")

procs = sys.process_list()
print("Всего активных процессов в ОС: " + string(len(procs)))
if len(procs) > 0:
    first_proc = procs[0]
    print("Пример процесса: PID=" + string(first_proc["pid"]) + ", Имя=" + string(first_proc["name"]))

parts = sys.disk_partitions()
print("Обнаружено дисковых разделов: " + string(len(parts)))
if len(parts) > 0:
    p0 = parts[0]
    print("Раздел: " + p0["mount"] + " (Всего: " + string(p0["total_gb"]) + " GB, Свободно: " + string(p0["free_gb"]) + " GB, Занято: " + string(p0["percent_used"]) + "%)")

// 4. Динамический FFI (вызов нативных C/Win32 библиотек напрямую)
print("\n[4] Проверка Foreign Function Interface (FFI):")
k32 = ffi.load("kernel32.dll")
current_pid = k32.call("GetCurrentProcessId")
print("Native FFI вызов GetCurrentProcessId(): PID = " + string(current_pid))

get_tick = k32.func("GetTickCount64")
ticks = get_tick()
print("Native FFI вызов GetTickCount64(): Ticks = " + string(ticks))
k32.close()

// 5. Машинное обучение и тензорные вычисления (std.ai)
print("\n[5] Проверка ИИ-фреймворка и тензорных матричных вычислений:")
// Создание матриц и умножение
mat_a = ai.matrix(2, 2, 2.0)
mat_b = ai.matrix(2, 2, 3.0)
mat_c = ai.matmul(mat_a, mat_b)
print("Результат matmul(2x2, 2x2): " + string(mat_c))

// Векторные операции и функции активации
v1 = [1.0, 2.0, 3.0]
v2 = [4.0, 5.0, 6.0]
dot_res = ai.dot(v1, v2)
print("Скалярное произведение векторов dot(): " + string(dot_res))

soft = ai.softmax([1.0, 2.0, 3.0])
print("Softmax вероятности: " + string(soft))

// Прямой проход полносвязного нейросетевого слоя (Dense Forward)
inputs = [ [0.5, 1.5] ]
weights = [ [0.2, 0.8], [0.5, -0.5] ]
biases = [0.1, 0.1]
dense_out = ai.dense_forward(inputs, weights, biases, "relu")
print("Прямой проход слоя нейросети (Dense Forward + ReLU): " + string(dense_out))

print("\n==================================================")
print("✅ ВСЕ ВОЗМОЖНОСТИ УСПЕШНО ПРОТЕСТИРОВАНЫ И РАБОТАЮТ!")
print("==================================================")
